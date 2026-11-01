//go:build unix

package bundle_test

// A02 のピークメモリ計測。RSS のピークはプロセス全体で単調増加するため、
// 各サイズを独立した子プロセスで測る。通常の go test では動かない。
//
//	A02_PERF=1 go test ./internal/bundle -run '^TestA02PerfSamples$' -v
//
// 子プロセスは同じテストバイナリを A02_PERF_CHILD=<MiB> 付きで起動する。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/bundle"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// a02Sample は子プロセス1回分の計測結果。
type a02Sample struct {
	MiB             int    `json:"mib"`
	Run             int    `json:"run"`
	PeakRSSMiB      int64  `json:"peak_rss_mib"`
	PeakRSSDeltaMiB int64  `json:"peak_rss_delta_mib"`
	TotalAllocMiB   int64  `json:"total_alloc_mib"`
	WallMS          int64  `json:"wall_ms"`
	Readback        string `json:"readback"`
}

func TestA02PerfSamples(t *testing.T) {
	if os.Getenv("A02_PERF") != "1" {
		t.Skip("A02_PERF=1 のときだけ計測する")
	}
	for _, mib := range []int{8, 32, 64} {
		for run := 1; run <= 2; run++ {
			sample := runA02Child(t, mib, run)
			encoded, err := json.Marshal(sample)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("A02_SAMPLE %s\n", encoded)
		}
	}
}

func runA02Child(t *testing.T, mib, run int) a02Sample {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestA02PerfChild$")
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("A02_PERF_CHILD=%d", mib),
		fmt.Sprintf("A02_PERF_RUN=%d", run),
	)
	output, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatalf("%dMiB run%d: 子プロセスが期限を超えました", mib, run)
	}
	if err != nil {
		t.Fatalf("%dMiB run%d: 子プロセスが失敗しました: %v", mib, run, err)
	}
	var sample a02Sample
	for _, line := range splitLines(string(output)) {
		const marker = "A02_SAMPLE "
		if len(line) > len(marker) && line[:len(marker)] == marker {
			if err := json.Unmarshal([]byte(line[len(marker):]), &sample); err != nil {
				t.Fatalf("%dMiB run%d: 計測結果を読めません: %v", mib, run, err)
			}
		}
	}
	if sample.MiB != mib || sample.Run != run {
		t.Fatalf("%dMiB run%d: 計測結果がありません: %q", mib, run, output)
	}
	return sample
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// TestA02PerfChild は A02_PERF_CHILD で指定された大きさの梱包を1回行い、
// ピーク RSS と TotalAlloc の増分、読み戻しの一致を出力する。
func TestA02PerfChild(t *testing.T) {
	mibText := os.Getenv("A02_PERF_CHILD")
	if mibText == "" {
		t.Skip("親の計測からのみ起動する")
	}
	mib, err := strconv.Atoi(mibText)
	if err != nil || mib <= 0 {
		t.Fatalf("A02_PERF_CHILD が不正です: %q", mibText)
	}
	run, _ := strconv.Atoi(os.Getenv("A02_PERF_RUN"))

	dir := t.TempDir()
	size := int64(mib) << 20
	runtimePath := filepath.Join(dir, "runtime")
	writeZeros(t, runtimePath, size)

	resources := filepath.Join(dir, "res")
	if err := os.Mkdir(resources, 0o755); err != nil {
		t.Fatal(err)
	}
	resourcePath := filepath.Join(resources, "payload.bin")
	wantHash := writeRandom(t, resourcePath, size)

	prog, err := vm.CompileProgram("1を表示", "main.nako3")
	if err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(dir, "app")
	var before, after runtime.MemStats
	var ruBefore, ruAfter syscall.Rusage
	runtime.ReadMemStats(&before)
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruBefore); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = bundle.BuildSpec(out, runtimePath, bundle.Spec{
		Kind:        bundle.KindProgram,
		Program:     prog,
		Name:        "main.nako3",
		ResourceDir: resources,
		Flat:        true,
	})
	wall := time.Since(started)
	if err != nil {
		t.Fatalf("梱包に失敗しました: %v", err)
	}
	runtime.ReadMemStats(&after)
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ruAfter); err != nil {
		t.Fatal(err)
	}

	readback := "ok"
	packed, err := bundle.Open(out)
	if err != nil {
		t.Fatalf("読み戻しに失敗しました: %v", err)
	}
	data, ok := packed.ReadResource("payload.bin")
	if !ok || sha256.Sum256(data) != wantHash {
		readback = "resource-mismatch"
	}
	if packed.Program == nil || packed.Name != "main.nako3" {
		readback = "program-mismatch"
	}
	if err := packed.Close(); err != nil {
		t.Fatal(err)
	}
	if readback != "ok" {
		t.Fatalf("読み戻しが一致しません: %s", readback)
	}

	peak := maxRSSBytes(&ruAfter)
	peakBefore := maxRSSBytes(&ruBefore)
	sample := a02Sample{
		MiB:             mib,
		Run:             run,
		PeakRSSMiB:      peak >> 20,
		PeakRSSDeltaMiB: (peak - peakBefore) >> 20,
		TotalAllocMiB:   int64(after.TotalAlloc-before.TotalAlloc) >> 20,
		WallMS:          wall.Milliseconds(),
		Readback:        readback,
	}
	encoded, err := json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("A02_SAMPLE %s\n", encoded)
}

// maxRSSBytes は getrusage の Maxrss をバイトへ直す。32bit Linux では
// Maxrss が int32 なので、比較の前に int64 へ広げる。単位は OS ごとに違い、
// darwin/ios はバイト、illumos/solaris はページ、それ以外は KiB
// (runtime/pprof/pprof_rusage.go と同じ扱い)。
func maxRSSBytes(ru *syscall.Rusage) int64 {
	switch runtime.GOOS {
	case "darwin", "ios":
		return int64(ru.Maxrss)
	case "illumos", "solaris":
		return int64(ru.Maxrss) * int64(syscall.Getpagesize())
	default:
		return int64(ru.Maxrss) * 1024
	}
}

func writeZeros(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	chunk := make([]byte, 1<<20)
	for written := int64(0); written < size; written += int64(len(chunk)) {
		n := int64(len(chunk))
		if remaining := size - written; remaining < n {
			n = remaining
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
	}
}

func writeRandom(t *testing.T, path string, size int64) [32]byte {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hash := sha256.New()
	chunk := make([]byte, 1<<20)
	for written := int64(0); written < size; written += int64(len(chunk)) {
		n := int64(len(chunk))
		if remaining := size - written; remaining < n {
			n = remaining
		}
		if _, err := io.ReadFull(rand.Reader, chunk[:n]); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		hash.Write(chunk[:n])
	}
	var sum [32]byte
	copy(sum[:], hash.Sum(nil))
	return sum
}
