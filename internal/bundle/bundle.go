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
	"crypto/rand"
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

// manifestEntry says what kind of application the payload is. A payload
// written before manifests existed has none, and is a compiled program.
const manifestEntry = "manifest.json"

// resourcePrefix is the folder resources are stored under.
const resourcePrefix = "resources/"

// The kinds of application a bundle can carry.
const (
	// KindProgram runs a compiled なでしこ program on startup.
	KindProgram = "nako3"
	// KindHTML opens a bundled HTML file in a WebView window.
	KindHTML = "html"
)

// manifest is the payload's description of itself.
type manifest struct {
	Kind string `json:"kind"`
	// Entry is the start page inside the resources, for KindHTML.
	Entry string `json:"entry,omitempty"`
	// Title is the window title the runtime should use.
	Title string `json:"title,omitempty"`
}

// ErrNoBundle reports a file with nothing appended to it. It is the ordinary
// case for the plain runtime, not a failure.
var ErrNoBundle = errors.New("バンドルが見つかりません")

// Bundle is what a packed executable carries.
type Bundle struct {
	Program *ir.Program
	// Name is the source file the program was built from, for error messages.
	Name string
	// Kind is KindProgram or KindHTML.
	Kind string
	// Entry is the start page inside the resources, for KindHTML.
	Entry string
	// Title is the window title the runtime should use, if it opens one.
	Title string

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

// ResourceFS exposes the bundled resources as a file system, so that a
// runtime can serve them to a WebView with http.FileServer.
func (b *Bundle) ResourceFS() (fs.FS, error) {
	if b == nil || b.reader == nil {
		return nil, errors.New("バンドルが開かれていません")
	}
	return fs.Sub(b.reader, strings.TrimSuffix(resourcePrefix, "/"))
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

// Spec describes the application to pack.
type Spec struct {
	// Kind is KindProgram or KindHTML. Empty means KindProgram.
	Kind string
	// Program is the compiled program, required for KindProgram.
	Program *ir.Program
	// Name is the source file the program was built from, for error messages.
	Name string
	// Entry is the start page inside the resources, required for KindHTML.
	Entry string
	// Title is the window title the runtime should use, if it opens one.
	Title string
	// ResourceDir is the folder to pack, or "" for none.
	ResourceDir string
	// Flat stores the resources without the folder's own name in front, so
	// that a folder packed whole keeps the paths its files had inside it.
	Flat bool
	// IncludeSymlink は、フォルダ外を指すシンボリックリンクも
	// リンク先を辿って梱包する。既定ではそうしたリンクを拒否する
	// (Issue #187、オプション自体は Issue #263)
	IncludeSymlink bool
	// Warn は梱包中の警告を受け取る。リンクが範囲外を指す・壊れている・
	// 隠しファイルを指す・同じフォルダを複数リンクが参照しているときに呼ぶ
	// (Issue #263 のレビュー指定とレビュー指摘)。
	// nil なら黙る。
	Warn func(format string, args ...any)
	// Skip names files that must not be packed, by absolute path. The
	// executable being written into the folder it packs is the usual case.
	Skip map[string]bool
}

// Build writes a bundled executable carrying a compiled program.
//
// runtimePath names the runtime to build on. Passing one built for another
// platform is how cross-platform packaging works — appending bytes needs no Go
// toolchain on the machine doing it.
func Build(outPath, runtimePath string, prog *ir.Program, name, resourceDir string) error {
	return BuildSpec(outPath, runtimePath, Spec{
		Kind:        KindProgram,
		Program:     prog,
		Name:        name,
		ResourceDir: resourceDir,
	})
}

// BuildSpec writes a bundled executable: the runtime, then the payload, then
// the footer.
func BuildSpec(outPath, runtimePath string, spec Spec) error {
	if spec.Kind == "" {
		spec.Kind = KindProgram
	}
	switch spec.Kind {
	case KindProgram:
		if spec.Program == nil {
			return errors.New("同梱するプログラムがありません")
		}
	case KindHTML:
		if spec.Entry == "" {
			return errors.New("開始ページが指定されていません")
		}
	default:
		return fmt.Errorf("知らない種類のアプリです: %s", spec.Kind)
	}

	runtime, err := readRuntime(runtimePath)
	if err != nil {
		return err
	}

	// 一時ファイルがリソースに混入しないよう、先にペイロードを完成させる。
	payload, err := buildPayload(spec)
	if err != nil {
		return err
	}

	// 同じフォルダに書き出し、完成した場合だけ既存出力と置き換える。
	oldInfo, err := os.Stat(outPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("出力ファイル『%s』を確認できません: %w", outPath, err)
	}
	mode := os.FileMode(0o755)
	if oldInfo != nil {
		mode = oldInfo.Mode().Perm()
	}
	out, err := createOutputTemp(filepath.Dir(outPath), mode)
	if err != nil {
		return fmt.Errorf("出力ファイル『%s』を作れません: %w", outPath, err)
	}
	tempPath := out.Name()
	defer func() {
		_ = out.Close()
		// 成功後はRename済みで一時パスは存在しない。失敗時の残骸だけ削除する。
		_ = os.Remove(tempPath)
	}()

	if _, err := out.Write(runtime); err != nil {
		return fmt.Errorf("ランタイムを書き出せません: %w", err)
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
	// 既存出力は権限を保持する。新規出力は作成時のumaskを適用した権限を使う。
	if oldInfo != nil {
		if err := out.Chmod(oldInfo.Mode().Perm()); err != nil {
			return fmt.Errorf("出力ファイルの権限を設定できません: %w", err)
		}
	}
	if err := out.Sync(); err != nil {
		return fmt.Errorf("出力ファイルを同期できません: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("出力ファイルを閉じられません: %w", err)
	}
	if err := os.Rename(tempPath, outPath); err != nil {
		return fmt.Errorf("出力ファイル『%s』を置き換えられません: %w", outPath, err)
	}
	return nil
}

// createOutputTemp は指定された権限とumaskを反映した一時ファイルを排他的に作る。
func createOutputTemp(dir string, mode os.FileMode) (*os.File, error) {
	for range 10 {
		name := filepath.Join(dir, ".gonako-build-"+rand.Text())
		out, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if os.IsExist(err) {
			continue
		}
		return out, err
	}
	return nil, errors.New("一時ファイル名が重複しました")
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

// buildPayload zips the manifest and the program together with the resource
// folder.
func buildPayload(spec Spec) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := addJSON(zw, manifestEntry, manifest{
		Kind:  spec.Kind,
		Entry: spec.Entry,
		Title: spec.Title,
	}); err != nil {
		return nil, fmt.Errorf("マニフェストを書き出せません: %w", err)
	}

	if spec.Program != nil {
		if err := addJSON(zw, programEntry, bundledProgram{Name: spec.Name, Program: spec.Program}); err != nil {
			return nil, fmt.Errorf("IRを書き出せません: %w", err)
		}
	}

	if spec.ResourceDir != "" {
		warn := spec.Warn
		if warn == nil {
			warn = func(string, ...any) {}
		}
		if err := addResources(zw, spec.ResourceDir, spec.Flat, spec.Skip, spec.IncludeSymlink, warn); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// addJSON writes one JSON entry into the payload.
func addJSON(zw *zip.Writer, name string, v any) error {
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(encoded)
	return err
}

// addResources copies a folder into the payload, keeping the folder name the
// build was given.
//
// A program that read 『images/a.png』 during development asks for the same
// path once packed, so the prefix has to survive (AGENTS.md §10). Packing a
// whole folder as an application is the exception: there the program already
// runs from inside the folder, so flat is what keeps its paths working.
func addResources(zw *zip.Writer, dir string, flat bool, skip map[string]bool, includeSymlink bool, warn func(string, ...any)) error {
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	prefix := ""
	if !flat {
		prefix = resourcePrefixFor(dir, root)
	}
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("リソース『%s』を読み込めません: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("リソース『%s』はフォルダではありません", dir)
	}
	// リンク先の実体がフォルダ内にあるか判定するため、フォルダ自身も
	// シンボリックリンクを解決したパスにしておく (macOSの/tmp等でも
	// 正しく比較できるように)。ルート自身がフォルダへのリンクでも、
	// 実体側を歩いて中身を梱包する。
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("リソース『%s』を読み込めません: %w", dir, err)
	}

	// skip はリンク未解決のパスで来ることがあるので、解決後の形でも
	// 照合できるようにしておく
	skipPaths := make(map[string]bool, len(skip))
	for k := range skip {
		skipPaths[k] = true
		// 未作成の出力先も、親フォルダのリンクを解決したパスで除外する。
		if parent, err := filepath.EvalSymlinks(filepath.Dir(k)); err == nil {
			if absolute, err := filepath.Abs(filepath.Join(parent, filepath.Base(k))); err == nil {
				skipPaths[absolute] = true
			}
		}
		// EvalSymlinksが失敗する（パスが存在しないなど）場合は、
		// 解決前のパスのみで照合する
		if resolved, err := filepath.EvalSymlinks(k); err == nil {
			skipPaths[resolved] = true
		}
	}

	// フォルダ外リンクを辿る際の再帰防止。現在辿っている実体フォルダを保持する
	visited := map[string]bool{}
	// 同じ外部フォルダが複数のリンクから参照されたときの警告用。
	// visited と違い一度梱包したら消さない (Issue #263 レビュー指摘)
	packed := map[string]bool{}

	return filepath.Walk(realRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// 「.git」のような隠しフォルダは、丸ごと梱包すると邪魔なので飛ばす
		if name := fi.Name(); strings.HasPrefix(name, ".") && p != realRoot {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// シンボリックリンクは、リンク先の実体がフォルダ内にあるかを
		// 確かめてから扱う。外を指すリンクは、既定では外部のファイルを
		// 配布バイナリへ取り込んでしまうので拒否する (Issue #187)。
		// --include-symlink を指定したときはリンク先を辿って梱包し、
		// 警告を出す (Issue #263 のレビュー指定)
		resolved := ""
		if fi.Mode()&os.ModeSymlink != 0 {
			// 出力先は完成まで作らないため、リンク先が未作成でも除外を優先する。
			if pointsToSkippedPath(p, skipPaths) {
				return nil
			}
			r, err := filepath.EvalSymlinks(p)
			if err != nil {
				// 壊れたリンク (先がない)。オプション時は警告出して飛ばす
				if !includeSymlink {
					return fmt.Errorf("リソース内のリンク『%s』を解決できません: %w", p, err)
				}
				warn("リンク『%s』は先が存在しないのでスキップしました", p)
				return nil
			}
			outside := !withinDir(realRoot, r)
			if outside && !includeSymlink {
				return fmt.Errorf("リソース内のリンク『%s』がフォルダ外の『%s』を指しているため梱包できません", p, r)
			}
			// 警告は梱包に進むリンクに対してだけ出す。エラーになる
			// 判断より前に出すとノイズになる (Issue #263 レビュー指摘)
			if hiddenTarget(r) {
				warn("リンク『%s』は隠しファイル『%s』を指しています", p, r)
			}
			if outside {
				warn("リンク『%s』はフォルダ外の『%s』を指していますが、梱包します", p, r)
				rel, err := filepath.Rel(realRoot, p)
				if err != nil {
					return err
				}
				return addLinkTarget(zw, prefix, rel, p, r, realRoot, skipPaths, visited, packed, warn)
			}
			resolved = r
			target, err := os.Stat(p)
			if err != nil {
				return err
			}
			if target.IsDir() {
				// フォルダへのリンクは実体側が別途走査されるので、
				// 別名での重複梱包を避けて飛ばす
				return nil
			}
		}
		if fi.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(realRoot, p)
		if err != nil {
			return err
		}
		// 出力先の実行ファイル自身を巻き込まない。歩いているのは実体側の
		// パスなので、指定されたときのパスの形やリンクの実体でも照合する
		if skipPaths[p] || skipPaths[filepath.Join(root, rel)] || (resolved != "" && skipPaths[resolved]) {
			return nil
		}
		return writeFileEntry(zw, resourcePrefix+prefix+filepath.ToSlash(rel), p)
	})
}

// pointsToSkippedPath は多段リンクを辿り、未作成の除外対象も判定する。
// 循環や解決エラーは通常のリンク検証に任せる。
func pointsToSkippedPath(p string, skip map[string]bool) bool {
	visited := map[string]bool{}
	for range 255 {
		parent, err := filepath.EvalSymlinks(filepath.Dir(p))
		if err != nil {
			return false
		}
		p, err = filepath.Abs(filepath.Join(parent, filepath.Base(p)))
		if err != nil || visited[p] {
			return false
		}
		if skip[p] {
			return true
		}
		visited[p] = true
		target, err := os.Readlink(p)
		if err != nil {
			return false
		}
		if filepath.IsAbs(target) {
			p = target
		} else {
			p = filepath.Join(filepath.Dir(p), target)
		}
	}
	return false
}

// addLinkTarget はリンク p の先を辿り、リンクの相対パス linkRel の
// 位置に実体として梱包する (--include-symlink、Issue #263)。
// 先がファイルならその内容をそのまま同名のファイルへ、フォルダなら
// 配下をまとめて linkRel のフォルダとして書き込む。visited は現在
// 辿っている実体フォルダで、リンクが環状につながって同じフォルダへ
// 再び到達したらエラーにする (Issue #263 のレビュー指定)。
// packed はこれまでにリンク先として梱包した実体フォルダで、同じ
// フォルダを二度目に参照したときは警告を出して梱包は続ける
// (Issue #263 レビュー指摘)。realRoot はリソースフォルダの実体で、
// 再帰途中でフォルダ外を指すリンクを見つけたときの警告に使う
// (Issue #263 レビュー指摘)。
// prefix と skipPaths は addResources から受け継ぐものと同じ。
func addLinkTarget(zw *zip.Writer, prefix, linkRel, p, realTarget, realRoot string, skipPaths map[string]bool, visited, packed map[string]bool, warn func(string, ...any)) error {
	target, err := os.Stat(p)
	if err != nil {
		return err
	}
	if !target.IsDir() {
		if skipPaths[realTarget] {
			return nil
		}
		return writeFileEntry(zw, resourcePrefix+prefix+filepath.ToSlash(linkRel), realTarget)
	}
	if visited[realTarget] {
		// 循環リンク (Issue #263 のレビュー指定ではエラー)
		return fmt.Errorf("シンボリックリンク『%s』が循環しているため梱包できません", p)
	}
	visited[realTarget] = true
	defer delete(visited, realTarget)
	if packed[realTarget] {
		warn("フォルダ『%s』は複数のリンクから参照されているため、内容が重複して梱包されます", p)
	}
	packed[realTarget] = true
	return filepath.Walk(realTarget, func(tp string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// フォルダを歩くときと同じ規則で、隠しファイルは飛ばす
		if name := fi.Name(); strings.HasPrefix(name, ".") && tp != realTarget {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		sub, err := filepath.Rel(realTarget, tp)
		if err != nil {
			return err
		}
		entryRel := filepath.Join(linkRel, sub)
		if fi.Mode()&os.ModeSymlink != 0 {
			r, err := filepath.EvalSymlinks(tp)
			if err != nil {
				warn("リンク『%s』は先が存在しないのでスキップしました", tp)
				return nil
			}
			if hiddenTarget(r) {
				warn("リンク『%s』は隠しファイル『%s』を指しています", tp, r)
			}
			if !withinDir(realRoot, r) {
				// 再帰先で見つけたフォルダ外リンクも、トップレベルと
				// 同じ規則で警告を出す (Issue #263 レビュー指摘)
				warn("リンク『%s』はフォルダ外の『%s』を指していますが、梱包します", tp, r)
			} else {
				target, err := os.Stat(tp)
				if err != nil {
					return err
				}
				if target.IsDir() {
					// フォルダ内のフォルダへのリンクは、実体側が別途
					// 歩かれるので重複梱包を避けて飛ばす (トップレベルと同じ)
					return nil
				}
			}
			return addLinkTarget(zw, prefix, entryRel, tp, r, realRoot, skipPaths, visited, packed, warn)
		}
		if skipPaths[tp] {
			return nil
		}
		return writeFileEntry(zw, resourcePrefix+prefix+filepath.ToSlash(entryRel), tp)
	})
}

// hiddenTarget はリンク先の実体の名前が隠しファイル (. から始まる) かを
// 返す。
func hiddenTarget(realTarget string) bool {
	return strings.HasPrefix(filepath.Base(realTarget), ".")
}

// writeFileEntry は p の中身を、ペイロード内の name というエントリへ
// 書き込む。
func writeFileEntry(zw *zip.Writer, name, p string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// withinDir は resolved が dir の中にあるかを返す。どちらも実体パス
// (EvalSymlinks済み) で渡すこと。
// dir と resolved が同じ場合（rel == "."）は許可する。リソースルート
// 自身がシンボリックリンクの場合、realRoot と resolved が同じになる
// 可能性があるため。
func withinDir(dir, resolved string) bool {
	rel, err := filepath.Rel(dir, resolved)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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

	// マニフェストのないバンドルは、マニフェストができる前に作られた
	// プログラム同梱の実行ファイル。
	mf := manifest{Kind: KindProgram}
	if entry, err := reader.Open(manifestEntry); err == nil {
		data, readErr := io.ReadAll(entry)
		entry.Close()
		if readErr != nil {
			f.Close()
			return nil, readErr
		}
		if err := json.Unmarshal(data, &mf); err != nil {
			f.Close()
			return nil, fmt.Errorf("バンドルのマニフェストを読めません: %w", err)
		}
	}

	packed := &Bundle{Kind: mf.Kind, Entry: mf.Entry, Title: mf.Title, reader: reader, file: f}
	if mf.Kind == KindProgram {
		prog, name, err := readProgram(reader)
		if err != nil {
			f.Close()
			return nil, err
		}
		packed.Program = prog
		packed.Name = name
	}
	return packed, nil
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
