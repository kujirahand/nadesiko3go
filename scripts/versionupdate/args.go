// Package versionupdate は version-update スクリプトの引数解析ロジックを提供する。
// scripts/version-update.go が //go:build ignore を必要とするため、
// 通常の go test ./... で実行できるように別パッケージに切り出している。
package versionupdate

import "fmt"

// ParsedArgs はコマンドライン引数の解析結果を表す。
type ParsedArgs struct {
	Check      bool
	Nadesiko   string
	Stable     string
	Positional []string
}

// ParseArgs は位置引数の前後どちらにあってもフラグを解析する。
// 標準の flag.Parse() は最初の位置引数で停止するため、使用例
// 「go run ./scripts/version-update.go 3.8.2 --nadesiko 3.9.0」を
// 正しく解析できるようにするためのカスタムパーサー。
func ParseArgs(args []string) (ParsedArgs, error) {
	var result ParsedArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--check":
			result.Check = true
		case "--nadesiko":
			if i+1 >= len(args) {
				return result, fmt.Errorf("--nadesiko の後に値が必要です")
			}
			i++
			result.Nadesiko = args[i]
		case "--stable":
			if i+1 >= len(args) {
				return result, fmt.Errorf("--stable の後に値が必要です")
			}
			i++
			result.Stable = args[i]
		case "--":
			// 終端マーカー以降はすべて位置引数
			result.Positional = append(result.Positional, args[i+1:]...)
			return result, nil
		default:
			if len(arg) > 2 && arg[:2] == "--" {
				return result, fmt.Errorf("未知のフラグです: %s", arg)
			}
			result.Positional = append(result.Positional, arg)
		}
	}
	return result, nil
}
