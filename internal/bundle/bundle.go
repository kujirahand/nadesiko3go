// Package bundle packs a compiled program and its resources onto the end of
// the runtime executable, so that the result is one file to hand over
// (AGENTS.md §10).
//
//	[ gonako ランタイム本体 ][ ペイロード(zip) ][ フッタ(マジック+長さ) ]
//
// The runtime reads its own tail at startup. Without a footer it behaves as an
// ordinary command; with one it runs the program it carries.
package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ir"
)

// magic marks a bundled executable. The trailing digit is the container
// format's version, which is separate from the IR version inside.
const magic = "GONAKOBUNDLE1"

// footerSize is the magic plus the eight bytes holding the payload length.
const footerSize = len(magic) + 8

// programEntry is where the compiled program lives inside the payload.
const programEntry = "program.ir.json"

// resourcePrefix is the folder resources are stored under.
const resourcePrefix = "resources/"

// ErrNoBundle reports a file with nothing appended to it. It is the ordinary
// case for the plain runtime, not a failure.
var ErrNoBundle = errors.New("バンドルが見つかりません")

// Bundle is what a packed executable carries.
type Bundle struct {
	Program *ir.Program
	// Name is the source file the program was built from, for error messages.
	Name string

	reader *zip.Reader
	file   *os.File
}

// Close releases the executable the bundle was read from.
func (b *Bundle) Close() error {
	if b.file == nil {
		return nil
	}
	return b.file.Close()
}

// ReadResource reads a bundled resource by its path relative to the resource
// folder. It reports false when the bundle has no such file.
func (b *Bundle) ReadResource(name string) ([]byte, bool) {
	if b == nil || b.reader == nil {
		return nil, false
	}
	f, err := b.reader.Open(resourcePrefix + path.Clean(strings.TrimPrefix(name, "./")))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Resources lists the bundled resource paths, for `gonako build --list`.
func (b *Bundle) Resources() []string {
	if b == nil || b.reader == nil {
		return nil
	}
	var names []string
	for _, f := range b.reader.File {
		if strings.HasPrefix(f.Name, resourcePrefix) {
			names = append(names, strings.TrimPrefix(f.Name, resourcePrefix))
		}
	}
	return names
}

// Build writes a bundled executable: the runtime, then the payload, then the
// footer.
//
// runtimePath names the runtime to build on. Passing one built for another
// platform is how cross-platform packaging works — appending bytes needs no Go
// toolchain on the machine doing it.
func Build(outPath, runtimePath string, prog *ir.Program, name, resourceDir string) error {
	runtime, err := readRuntime(runtimePath)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("出力ファイル『%s』を作れません: %w", outPath, err)
	}
	defer out.Close()

	if _, err := out.Write(runtime); err != nil {
		return fmt.Errorf("ランタイムを書き出せません: %w", err)
	}

	payload, err := buildPayload(prog, name, resourceDir)
	if err != nil {
		return err
	}
	if _, err := out.Write(payload); err != nil {
		return fmt.Errorf("ペイロードを書き出せません: %w", err)
	}

	footer := make([]byte, footerSize)
	copy(footer, magic)
	binary.BigEndian.PutUint64(footer[len(magic):], uint64(len(payload)))
	if _, err := out.Write(footer); err != nil {
		return fmt.Errorf("フッタを書き出せません: %w", err)
	}
	return nil
}

// readRuntime reads a runtime executable, dropping any payload it already
// carries so that building twice does not stack them up.
func readRuntime(runtimePath string) ([]byte, error) {
	data, err := os.ReadFile(runtimePath)
	if err != nil {
		return nil, fmt.Errorf("ランタイム『%s』を読み込めません: %w", runtimePath, err)
	}
	if size, ok := payloadSize(data); ok {
		return data[:len(data)-footerSize-int(size)], nil
	}
	return data, nil
}

// payloadSize reads the footer, reporting how long the payload is.
func payloadSize(data []byte) (uint64, bool) {
	if len(data) < footerSize {
		return 0, false
	}
	footer := data[len(data)-footerSize:]
	if string(footer[:len(magic)]) != magic {
		return 0, false
	}
	size := binary.BigEndian.Uint64(footer[len(magic):])
	if size > uint64(len(data)-footerSize) {
		return 0, false
	}
	return size, true
}

// buildPayload zips the program together with the resource folder.
func buildPayload(prog *ir.Program, name, resourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	encoded, err := json.Marshal(bundledProgram{Name: name, Program: prog})
	if err != nil {
		return nil, fmt.Errorf("IRを書き出せません: %w", err)
	}
	w, err := zw.Create(programEntry)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(encoded); err != nil {
		return nil, err
	}

	if resourceDir != "" {
		if err := addResources(zw, resourceDir); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// addResources copies a folder into the payload, keeping the folder name the
// build was given.
//
// A program that read 『images/a.png』 during development asks for the same
// path once packed, so the prefix has to survive (AGENTS.md §10).
func addResources(zw *zip.Writer, dir string) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	prefix := resourcePrefixFor(dir, root)
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("リソース『%s』を読み込めません: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("リソース『%s』はフォルダではありません", dir)
	}

	return filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(resourcePrefix + prefix + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
}

// resourcePrefixFor decides what folder name the resources keep.
//
// A folder inside the build's working directory keeps the path the program
// would have used while being developed. One from elsewhere keeps just its
// own name, since there is no relative path that would make sense.
func resourcePrefixFor(dir, root string) string {
	prefix := filepath.Clean(dir)
	if filepath.IsAbs(prefix) {
		prefix = filepath.Base(root)
		if cwd, err := os.Getwd(); err == nil {
			if rel, err := filepath.Rel(cwd, root); err == nil && !strings.HasPrefix(rel, "..") {
				prefix = rel
			}
		}
	}
	prefix = filepath.ToSlash(prefix)
	if prefix == "." {
		return ""
	}
	return prefix + "/"
}

// bundledProgram is the payload's program entry.
type bundledProgram struct {
	Name    string      `json:"name"`
	Program *ir.Program `json:"program"`
}

// Open reads the bundle appended to an executable. It reports ErrNoBundle when
// there is none, which is how the plain runtime tells it is not packed.
func Open(execPath string) (*Bundle, error) {
	f, err := os.Open(execPath)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	total := info.Size()
	if total < int64(footerSize) {
		f.Close()
		return nil, ErrNoBundle
	}

	footer := make([]byte, footerSize)
	if _, err := f.ReadAt(footer, total-int64(footerSize)); err != nil {
		f.Close()
		return nil, err
	}
	if string(footer[:len(magic)]) != magic {
		f.Close()
		return nil, ErrNoBundle
	}
	size := int64(binary.BigEndian.Uint64(footer[len(magic):]))
	start := total - int64(footerSize) - size
	if size <= 0 || start < 0 {
		f.Close()
		return nil, ErrNoBundle
	}

	reader, err := zip.NewReader(io.NewSectionReader(f, start, size), size)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("バンドルを読めません: %w", err)
	}

	prog, name, err := readProgram(reader)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &Bundle{Program: prog, Name: name, reader: reader, file: f}, nil
}

func readProgram(reader *zip.Reader) (*ir.Program, string, error) {
	entry, err := reader.Open(programEntry)
	if err != nil {
		return nil, "", fmt.Errorf("バンドルにプログラムが入っていません: %w", err)
	}
	defer entry.Close()

	data, err := io.ReadAll(entry)
	if err != nil {
		return nil, "", err
	}
	var packed bundledProgram
	if err := json.Unmarshal(data, &packed); err != nil {
		return nil, "", fmt.Errorf("バンドルのプログラムを読めません: %w", err)
	}
	if packed.Program == nil {
		return nil, "", errors.New("バンドルのプログラムが空です")
	}
	// IRのバージョンが違うバイナリは、黙って動かさず拒否する (AGENTS.md §6)
	if err := packed.Program.Validate(); err != nil {
		return nil, "", fmt.Errorf("バンドルのプログラムが使えません: %w", err)
	}
	return packed.Program, packed.Name, nil
}

// FS presents the bundled resources as a read-only file system, so that code
// that already speaks fs.FS can read them.
type FS struct{ bundle *Bundle }

// Open implements fs.FS.
func (f FS) Open(name string) (fs.File, error) {
	if f.bundle == nil || f.bundle.reader == nil {
		return nil, fs.ErrNotExist
	}
	return f.bundle.reader.Open(resourcePrefix + path.Clean(name))
}

// FS returns the resources as a file system.
func (b *Bundle) FS() FS { return FS{bundle: b} }
