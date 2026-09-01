package nodelib

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// osCommands adds the commands that talk to the operating system and to the
// person running the program.
func osCommands(m map[string]command) {
	// --- 標準入出力 ---

	m["尋"] = command{josi: [][]string{{"と", "を"}}, fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
		// プロンプトは改行しない。入力が同じ行に続くため。
		if prompt := str(a, 0); prompt != "" {
			ctx.Write(prompt)
		}
		line, err := ctx.ReadLine()
		if err != nil {
			return value.Undefined(), errors.New("『尋』命令で標準入力が読めません。")
		}
		// 数値に見えるものは数値として返す。TS版と同じ。
		if n := value.ToNumber(value.String(strings.TrimSpace(line))); strings.TrimSpace(line) != "" && !isNaN(n) {
			return value.Number(n), nil
		}
		return value.String(line), nil
	}}
	m["文字尋"] = command{josi: [][]string{{"と", "を"}}, fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
		if prompt := str(a, 0); prompt != "" {
			ctx.Write(prompt)
		}
		line, err := ctx.ReadLine()
		if err != nil {
			return value.Undefined(), errors.New("『文字尋』命令で標準入力が読めません。")
		}
		return value.String(line), nil
	}}

	// --- プロセス ---

	m["終了"] = command{returnNone: true, fn: func(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
		ctx.Exit(0)
		return value.Undefined(), nil
	}}
	m["強制終了"] = command{josi: [][]string{{"で", "の"}}, returnNone: true,
		fn: func(ctx stdlib.Context, a []value.Value) (value.Value, error) {
			ctx.Exit(int(value.ToNumber(argAt(a, 0))))
			return value.Undefined(), nil
		}}

	m["コマンドライン"] = command{fn: func(ctx stdlib.Context, _ []value.Value) (value.Value, error) {
		args := ctx.Args()
		items := make([]value.Value, len(args))
		for i, s := range args {
			items[i] = value.String(s)
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	}}

	m["環境変数取得"] = command{josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			return value.String(os.Getenv(str(a, 0))), nil
		}}
	m["環境変数一覧取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		d := value.NewDict()
		for _, kv := range os.Environ() {
			if name, v, ok := strings.Cut(kv, "="); ok {
				d.Set(name, value.String(v))
			}
		}
		return value.DictValue(d), nil
	}}

	m["OS取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		return value.String(runtime.GOOS), nil
	}}
	m["OSアーキテクチャ取得"] = command{fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
		return value.String(runtime.GOARCH), nil
	}}

	// --- 外部コマンド ---

	m["コマンド実行待機"] = command{josi: [][]string{{"を", "の"}}, fn: runCommand}
}

// runCommand runs a shell command and returns what it printed.
//
// The command goes through the platform's shell, as plugin_node does, so that
// a script can use pipes and redirection.
func runCommand(_ stdlib.Context, a []value.Value) (value.Value, error) {
	line := str(a, 0)
	if line == "" {
		return value.String(""), nil
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", line)
	} else {
		cmd = exec.Command("sh", "-c", line)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		// 出力は返しつつ、失敗したことは伝える
		return value.String(string(out)), errors.New("コマンド『" + line + "』の実行に失敗しました。" + err.Error())
	}
	return value.String(string(out)), nil
}

func argAt(args []value.Value, i int) value.Value {
	if i < 0 || i >= len(args) {
		return value.Undefined()
	}
	return args[i]
}

func isNaN(f float64) bool { return f != f }
