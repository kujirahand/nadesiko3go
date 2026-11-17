// Package editorlint は、エディタの文法チェックにDocTestの実行を足す。
// WebViewには依存しない。画面側は Check の戻りが Pending なら Poll で結果を受け取る。
package editorlint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kujirahand/nadesiko3go/internal/doctest"
)

// Timeout は1件のDocTestを待つ既定の上限。
// 外部ランタイム（docs/doctest.md）と同じ10秒にする。
// 超えたサンプルは失敗として報告し、後続のサンプルへ進む。
const Timeout = 10 * time.Second

// detailLimit は失敗・省略の詳細を結果へ載せる上限。
// gonako doctest の既定（10件）に合わせる。
const detailLimit = 10

// ParseFunc はコードを構文解析し、エラーがあればそれを返す。
type ParseFunc func(code, filename string) error

// Options は文法チェックとDocTest実行の材料。
// Run が nil なら doctest.Run を使う。Timeout が 0 なら Timeout 定数を使う。
// Sync が true のとき、Check は実行完了まで待って Text を返す。
// 本番のエディタは Sync を false にし、Bindをすぐ返す。
type Options struct {
	// Whole はバッファ全体の構文解析（実行モードの命令セット）。
	Whole ParseFunc
	// Sample は表示結果・GO表示結果の構文解析（内蔵VMの命令セット）。
	Sample ParseFunc
	// Web は WEB表示結果の構文解析（ブラウザ版の命令セット）。
	// nil のときはWEBサンプルの文法を見ない。
	Web     ParseFunc
	Run     func(doctest.Test) doctest.Result
	Timeout time.Duration
	Sync    bool
}

// Result は文法チェックの結果。
type Result struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Text    string `json:"text,omitempty"`
	DocTest *Ref   `json:"doctest,omitempty"`
}

// Ref はDocTest実行を始めたことの通知。結果は Poll で取る。
type Ref struct {
	Pending bool   `json:"pending,omitempty"`
	RunID   uint64 `json:"runId,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// Status は Poll の応答。Done が false のあいだは OK を見ない。
type Status struct {
	Done  bool   `json:"done"`
	OK    bool   `json:"ok"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
}

type job struct {
	mu   sync.Mutex
	done bool
	ok   bool
	text string
}

func (j *job) finish(ok bool, text string) {
	j.mu.Lock()
	j.done = true
	j.ok = ok
	j.text = text
	j.mu.Unlock()
}

func (j *job) snapshot() (done, ok bool, text string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.done, j.ok, j.text
}

var (
	seq  uint64
	jobs sync.Map // uint64 -> *job
)

type outcome struct {
	passed   int
	failed   int
	skipped  int
	failures []string
	skips    []string
	hidden   int
}

// manualDoc はマニュアル（.txt）かどうかを返す。
// DocTestの正本は .txt なので、地の文をプログラムの文法エラーにしない。
// .nako3 や未保存のバッファは、ファイル全体が文法を通ったときだけ
// DocTestを実行する（コメント内に書いたサンプルを想定する）。
func manualDoc(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".txt")
}

func splitTests(code, file string) (run, web, other []doctest.Test) {
	for _, test := range doctest.Extract(code, file) {
		switch test.Label {
		case doctest.LabelCNako, doctest.LabelGOnako:
			run = append(run, test)
		case doctest.LabelWNako:
			web = append(web, test)
		default:
			other = append(other, test)
		}
	}
	return run, web, other
}

// Check はバッファを文法チェックし、DocTestがあれば実行する。
// DocTestが無ければ whole の結果だけを返す。
// 文法エラーがあるときはサンプルを実行しない。
func Check(code, filename, originalName string, opt Options) Result {
	run, web, other := splitTests(code, filename)
	if len(run) == 0 && len(web) == 0 && len(other) == 0 {
		return wholeResult(opt.Whole, code, filename)
	}

	// プログラムファイルは、全体の文法が先。地の文とサンプルが混ざる
	// マニュアル（.txt）だけ、全体を1本のプログラムとしては解析しない。
	if !manualDoc(originalName) {
		if res := wholeResult(opt.Whole, code, filename); !res.OK {
			return res
		}
	}

	// JS実行など、内蔵VMが最初から実行しないサンプルは文法エラーにしない。
	// doctest.Run と同じく、実行前に省略する。
	run, skipped := omitBuiltIn(run)
	if msg := sampleErrors(run, filename, opt.Sample); msg != "" {
		return Result{OK: false, Error: msg}
	}
	// WEB表示結果はブラウザ版の命令で解析する。構造の壊れたサンプルは
	// 他のDocTestも実行しない。
	if msg := sampleErrors(web, filename, opt.Web); msg != "" {
		return Result{OK: false, Error: msg}
	}

	if len(run) == 0 {
		return Result{OK: true, Text: formatText(skipped, len(web), len(other))}
	}
	runner, timeout := opt.runner()
	if opt.Sync {
		got := execute(run, runner, timeout)
		got.passed += skipped.passed
		got.failed += skipped.failed
		got.skipped += skipped.skipped
		got.skips = append(skipped.skips, got.skips...)
		text := formatText(got, len(web), len(other))
		res := Result{OK: got.failed == 0, Text: text}
		if !res.OK {
			res.Error = text
		}
		return res
	}
	id := start(run, skipped, runner, timeout, len(web), len(other))
	return Result{
		OK: true,
		DocTest: &Ref{
			Pending: true,
			RunID:   id,
			Total:   len(run),
		},
	}
}

func wholeResult(parse ParseFunc, code, filename string) Result {
	if parse == nil {
		return Result{OK: true}
	}
	if err := parse(code, filename); err != nil {
		return Result{OK: false, Error: err.Error()}
	}
	return Result{OK: true}
}

func (opt Options) runner() (func(doctest.Test) doctest.Result, time.Duration) {
	run := opt.Run
	if run == nil {
		run = doctest.Run
	}
	timeout := opt.Timeout
	if timeout == 0 {
		timeout = Timeout
	}
	return run, timeout
}

// sampleLineRe はエラー文面の「(N行目)」を取り出す。
var sampleLineRe = regexp.MustCompile(`\((\d+)行目\)`)

// sampleErrors は各サンプルを構文解析し、エラーをファイル上の行番号へ直して連結する。
// サンプルの1行目は『{{{#nako3』の次の行なので、表示上の行番号に
// ブロック開始行（1始まり）を足すとファイル上の行になる。
// 期待結果の後ろにもコードがあるサンプルでは、後半の行がずれることがある。
func sampleErrors(tests []doctest.Test, filename string, parse ParseFunc) string {
	if parse == nil {
		return ""
	}
	var b strings.Builder
	for _, test := range tests {
		if err := parse(test.Code, filename); err != nil {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(shiftLine(err.Error(), test.Line))
		}
	}
	return b.String()
}

func shiftLine(msg string, blockLine int) string {
	if blockLine <= 0 {
		return msg
	}
	return sampleLineRe.ReplaceAllStringFunc(msg, func(part string) string {
		m := sampleLineRe.FindStringSubmatch(part)
		if m == nil {
			return part
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return part
		}
		return fmt.Sprintf("(%d行目)", n+blockLine)
	})
}

// execute はサンプルを順に実行する。
// 1件が制限時間を超えても、そこで全体を止めずに次のサンプルへ進む。
func execute(tests []doctest.Test, run func(doctest.Test) doctest.Result, timeout time.Duration) outcome {
	var out outcome
	for _, test := range tests {
		result := runOne(test, run, timeout)
		switch {
		case result.Skipped:
			out.skipped++
			if len(out.skips) < detailLimit {
				out.skips = append(out.skips, fmt.Sprintf("%s:%d %s", test.File, test.Line, result.SkipReason))
			}
		case result.OK:
			out.passed++
		default:
			out.failed++
			if len(out.failures) < detailLimit {
				out.failures = append(out.failures, doctest.FormatFailure(test, result, ""))
			} else {
				out.hidden++
			}
		}
	}
	return out
}

// runOne は1件を制限時間つきで実行する。
//
// ホストは doctest.Run と同じもので、ダイアログには対応しない。
// 『尋』はすぐEOFになり、『言う』はダイアログを出さず、『秒待』は仮想時間で進む。
// 命令数の上限に達しても継続確認のダイアログは出さず、エラーで終わる
// （ホストが確認に対応していないため）。
// 制限時間を超えたあとも、裏の実行は命令数の上限まで残ることがある。
// 外部プロセスの停止までは行わない。
func runOne(test doctest.Test, run func(doctest.Test) doctest.Result, timeout time.Duration) doctest.Result {
	if timeout <= 0 {
		return run(test)
	}
	ch := make(chan doctest.Result, 1)
	go func() {
		ch <- run(test)
	}()
	select {
	case result := <-ch:
		return result
	case <-time.After(timeout):
		return doctest.Result{Err: fmt.Errorf("制限時間%sを超えたため中断しました", timeoutPhrase(timeout))}
	}
}

func timeoutPhrase(d time.Duration) string {
	if d > 0 && d%time.Second == 0 {
		return fmt.Sprintf("%d秒", int(d/time.Second))
	}
	return d.String()
}

func start(tests []doctest.Test, skipped outcome, run func(doctest.Test) doctest.Result, timeout time.Duration, web, other int) uint64 {
	id := atomic.AddUint64(&seq, 1)
	j := &job{}
	jobs.Store(id, j)
	go func() {
		got := execute(tests, run, timeout)
		got.skipped += skipped.skipped
		got.skips = append(skipped.skips, got.skips...)
		j.finish(got.failed == 0, formatText(got, web, other))
	}()
	return id
}

// omitBuiltIn は内蔵VMが実行しないサンプルを、実行リストから外して省略結果にする。
func omitBuiltIn(tests []doctest.Test) ([]doctest.Test, outcome) {
	var run []doctest.Test
	var skipped outcome
	for _, test := range tests {
		reason := doctest.SkipReason(test.Code)
		if reason == "" {
			run = append(run, test)
			continue
		}
		skipped.skipped++
		if len(skipped.skips) < detailLimit {
			skipped.skips = append(skipped.skips, fmt.Sprintf("%s:%d %s", test.File, test.Line, reason))
		}
	}
	return run, skipped
}

// Poll はDocTest実行の経過を返す。完了を一度返したら、その実行は捨てる。
func Poll(id uint64) Status {
	v, ok := jobs.Load(id)
	if !ok {
		return Status{Done: true, OK: false, Error: "DocTestの実行結果が見つかりません。"}
	}
	j := v.(*job)
	done, success, text := j.snapshot()
	if !done {
		return Status{Done: false}
	}
	jobs.Delete(id)
	res := Status{Done: true, OK: success, Text: text}
	if !success {
		res.Error = text
	}
	return res
}

func formatText(got outcome, web, other int) string {
	var b strings.Builder
	b.WriteString("文法エラーはありません")
	ran := got.passed + got.failed + got.skipped
	if ran == 0 && web == 0 && other == 0 {
		return b.String()
	}
	b.WriteString("\n\n")
	if ran > 0 {
		b.WriteString("DocTest: ")
		b.WriteString(joinCounts(got.passed, got.failed, got.skipped))
		b.WriteByte('\n')
	}
	for _, failure := range got.failures {
		b.WriteByte('\n')
		b.WriteString(failure)
		b.WriteByte('\n')
	}
	if got.hidden > 0 {
		fmt.Fprintf(&b, "\nほか%d件の失敗は省略しました。\n", got.hidden)
	}
	for _, skip := range got.skips {
		fmt.Fprintf(&b, "省略: %s\n", skip)
	}
	if web > 0 {
		fmt.Fprintf(&b, "WEB表示結果のDocTest %d件は、ブラウザ版での実行が必要なため省略しました。\n", web)
	}
	if other > 0 {
		fmt.Fprintf(&b, "表示結果・GO表示結果以外のDocTest %d件は、この文法チェックでは実行しません。\n", other)
	}
	return strings.TrimRight(b.String(), "\n")
}

func joinCounts(passed, failed, skipped int) string {
	parts := []string{fmt.Sprintf("%d件成功", passed)}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d件失敗", failed))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d件省略", skipped))
	}
	return strings.Join(parts, "、")
}
