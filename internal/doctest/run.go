package doctest

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/errs"
	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// Result is what running one sample produced.
type Result struct {
	OK         bool
	Skipped    bool
	SkipReason string
	Actual     string
	Err        error
}

// omittedCommands are commands intentionally outside the Go backend. The JS
// commands depend on the TypeScript runtime's JavaScript environment, and the
// self-evaluation commands are explicitly omitted from this implementation.
// Report these separately so they are never mistaken for executed tests.
var omittedCommands = []struct {
	marker string
	name   string
}{
	{marker: "JS実行", name: "JS実行"},
	{marker: "JS関数実行", name: "JS関数実行"},
	{marker: "JSメソッド実行", name: "JSメソッド実行"},
	{marker: "JS:", name: "JSコード"},
	{marker: "をナデシコ", name: "ナデシコ"},
	{marker: "ナデシコ続ける", name: "ナデシコ続"},
	{marker: "ナデシコする", name: "ナデシコ"},
}

func omittedReason(code string) string {
	for _, command := range omittedCommands {
		if strings.Contains(code, command.marker) {
			return "Go版で省略する命令: " + command.name
		}
	}
	if strings.Contains(code, "プラグイン一覧取得") {
		return "実行環境で一覧が異なる命令: プラグイン一覧取得"
	}
	return ""
}

// logHost collects what a sample printed, the way 『表示ログ』 does. A prompt
// written without a newline is not part of the log.
type logHost struct {
	log     strings.Builder
	pending strings.Builder
}

func (h *logHost) Print(s string) {
	h.log.WriteString(h.pending.String())
	h.pending.Reset()
	h.log.WriteString(s)
	h.log.WriteByte('\n')
}

func (h *logHost) Write(s string)                     { h.pending.WriteString(s) }
func (h *logHost) ReadLine() (string, error)          { return "", io.EOF }
func (h *logHost) Exit(int)                           {}
func (h *logHost) Args() []string                     { return nil }
func (h *logHost) ReadResource(string) ([]byte, bool) { return nil, false }
func (h *logHost) Now() time.Time                     { return time.Now() }

// Run executes one sample and compares what it printed with what the manual
// says it prints.
func Run(test Test) Result {
	if test.Runtime == WNako {
		return Result{Err: errors.New("WEB表示結果のDocTestはブラウザ版の実行が必要です。")}
	}
	if reason := omittedReason(test.Code); reason != "" {
		return Result{Skipped: true, SkipReason: reason}
	}
	host := &logHost{}
	if err := vm.RunWithHost(test.Code, test.File, host); err != nil {
		return Result{Err: err}
	}
	actual := trimTrailing(host.log.String())
	return Result{OK: actual == test.Expect, Actual: actual}
}

// Failure names why a sample did not pass, so that a run over the whole manual
// can be summarised rather than read line by line.
type Failure int

const (
	// FailMismatch is a sample that ran but printed something else.
	FailMismatch Failure = iota
	// FailMissingCommand is a sample using a command this implementation does
	// not have yet.
	FailMissingCommand
	// FailSyntax is a sample the parser rejected.
	FailSyntax
	// FailRuntime is any other error while running.
	FailRuntime
)

func (f Failure) String() string {
	switch f {
	case FailMismatch:
		return "表示結果が違う"
	case FailMissingCommand:
		return "未対応の命令"
	case FailSyntax:
		return "構文エラー"
	}
	return "実行時エラー"
}

// missingCommandMarkers are the phrases an error carries when the cause is a
// command this implementation does not have.
//
// An unknown command is not a word the parser can resolve, so it surfaces as a
// syntax error rather than a runtime one. Sorting those out matters, because
// the two ask for completely different work.
var missingCommandMarkers = []string{
	"はまだ実装されていません",
	"が見つかりません。",
	"が解決していません",
	"未解決の単語があります",
}

// Classify says why a result failed.
func Classify(r Result) Failure {
	if r.Err == nil {
		return FailMismatch
	}
	msg := r.Err.Error()
	for _, marker := range missingCommandMarkers {
		if strings.Contains(msg, marker) {
			return FailMissingCommand
		}
	}
	var nakoErr *errs.NakoError
	if errors.As(r.Err, &nakoErr) && (nakoErr.Kind == errs.Syntax || nakoErr.Kind == errs.Lexer) {
		return FailSyntax
	}
	return FailRuntime
}

// FormatFailure builds the report the spec describes: where it was, what ran,
// what was expected, what happened, and which lines differ.
func FormatFailure(test Test, r Result, root string) string {
	where := shortPath(test.File, root) + ":" + strconv.Itoa(test.Line)
	var b strings.Builder

	if r.Err != nil {
		fmt.Fprintf(&b, "[DocTest失敗] %s のサンプルコードが実行エラーになりました。\n", where)
		b.WriteString("--- 実行したコード ---\n" + indent(test.Code) + "\n")
		b.WriteString("--- エラー内容 ---\n" + indent(r.Err.Error()) + "\n")
		b.WriteString("マニュアルのサンプルコードを修正するか、期待する表示結果を見直してください。")
		return b.String()
	}

	fmt.Fprintf(&b, "[DocTest失敗] %s の表示結果が期待と異なります。\n", where)
	b.WriteString("--- 実行したコード ---\n" + indent(test.Code) + "\n")
	b.WriteString("--- 期待した表示結果 ---\n" + indent(test.Expect) + "\n")
	b.WriteString("--- 実際の表示結果 ---\n" + indent(r.Actual) + "\n")
	b.WriteString("--- 違いのある行 ---\n" + indent(diffLines(test.Expect, r.Actual)) + "\n")
	marker := "### 表示結果:"
	if test.Runtime == WNako {
		marker = "### WEB表示結果:"
	}
	fmt.Fprintf(&b, "マニュアルの「%s」の記述か、サンプルコードのどちらかを修正してください。", marker)
	return b.String()
}

// diffLines lists only the lines where the two differ.
func diffLines(expect, actual string) string {
	e := strings.Split(expect, "\n")
	a := strings.Split(actual, "\n")
	var out []string
	for i := 0; i < len(e) || i < len(a); i++ {
		want, got := lineAt(e, i), lineAt(a, i)
		if want == got {
			continue
		}
		out = append(out, fmt.Sprintf("%d行目: 期待=%s / 実際=%s", i+1, quote(want), quote(got)))
	}
	return strings.Join(out, "\n")
}

func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "\x00" // 行がないことを、空行と区別する印
}

func quote(s string) string {
	if s == "\x00" {
		return `"(行なし)"`
	}
	return strconv.Quote(s)
}

// shortPath makes a path relative to the repository root when it is inside it.
func shortPath(file, root string) string {
	if root == "" {
		return file
	}
	rel, err := filepath.Rel(root, file)
	if err != nil || strings.HasPrefix(rel, "..") {
		return file
	}
	return rel
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
