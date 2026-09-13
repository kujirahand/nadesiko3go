// Package wasmrt は、ブラウザ(WebAssembly)でなでしこのコア機能だけを動かす実行環境です。
//
// ファイル・OS・プロセスを扱う nodelib や単一ファイル梱包(bundle)は取り込まず、
// plugin_system 相当の stdlib と、純粋な計算だけの mathlib・csvlib に絞ります。
// syscall/js には依存しないので、ネイティブ環境でもテストできます。
// JavaScriptとの橋渡しは cmd/gonako-wasm が受け持ちます。
package wasmrt

import (
	"io"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/csvlib"
	"github.com/kujirahand/nadesiko3go/internal/mathlib"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// DefaultFilename は、ファイル名を指定しなかったときにエラー表示で使う名前です。
const DefaultFilename = "main.nako3"

// Host はブラウザ向けの vm.Host です。表示内容を溜めつつ、呼び出し側へも通知します。
type Host struct {
	// OnPrint は『表示』のたびに1行分(改行なし)を受け取ります。nil なら通知しません。
	OnPrint func(line string)
	// OnWrite は『連続無改行表示』などの改行しない出力を受け取ります。nil なら通知しません。
	OnWrite func(s string)
	// Dialog は『言う』などのダイアログ要求を受け取ります。nil なら未対応として扱い、
	// 命令側が『表示』に切り替えます。
	Dialog func(kind, message string) (answer string, accepted, supported bool, err error)
	// CmdArgs はプログラムに渡す引数です。
	CmdArgs []string

	// ExitCode は『終了』で指定された値、Exited は『終了』が呼ばれたかどうかです。
	ExitCode int
	Exited   bool

	out strings.Builder
}

var _ vm.Host = (*Host)(nil)

func (h *Host) Print(s string) {
	h.out.WriteString(s)
	h.out.WriteByte('\n')
	if h.OnPrint != nil {
		h.OnPrint(s)
	}
}

func (h *Host) Write(s string) {
	h.out.WriteString(s)
	if h.OnWrite != nil {
		h.OnWrite(s)
	}
}

// ReadLine は読み込む先がないので常に EOF を返します。
func (h *Host) ReadLine() (string, error) { return "", io.EOF }

func (h *Host) Exit(code int) {
	h.ExitCode = code
	h.Exited = true
}

func (h *Host) Args() []string { return h.CmdArgs }

// ReadResource は梱包リソースを持たないので常に見つかりません。
func (h *Host) ReadResource(string) ([]byte, bool) { return nil, false }

func (h *Host) Now() time.Time { return time.Now() }

// ShowDialog は vm が任意で呼ぶダイアログの窓口です。
func (h *Host) ShowDialog(kind, message string) (string, bool, bool, error) {
	if h.Dialog == nil {
		return "", false, false, nil
	}
	return h.Dialog(kind, message)
}

// Output はこれまでに出力された内容をすべて返します。
func (h *Host) Output() string { return h.out.String() }

// Registry はブラウザ版で使える命令の一覧を作ります。
// コンパイルと実行で同じレジストリを使う必要があるので、実行ごとに1つ作って使い回します。
func Registry() *stdlib.Registry {
	return stdlib.NewRegistry(mathlib.New(), csvlib.New())
}

// Run はプログラムをコンパイルして実行します。『秒待機』は実時間で待ちます。
// なでしこのエラーは *errs.NakoError で返ります。
func Run(code, filename string, h *Host) error {
	if filename == "" {
		filename = DefaultFilename
	}
	return vm.RunWithHostAndRegistry(code, filename, Registry(), h)
}
