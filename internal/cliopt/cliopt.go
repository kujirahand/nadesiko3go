// Package cliopt は、gonako と gonako-cui で共通するコマンドライン
// オプションの解釈をまとめたパッケージ。
package cliopt

// TakeLongtimeError は実行対象より前に置かれた --longtime-error を取り除く。
// ファイル名より後ろの引数はプログラムへの引数なので取り除かない。
func TakeLongtimeError(args []string) (rest []string, longtimeError bool) {
	if len(args) == 0 {
		return args, false
	}
	if args[0] == "--longtime-error" {
		return args[1:], true
	}
	if (args[0] == "run" || args[0] == "-e") && len(args) >= 2 && args[1] == "--longtime-error" {
		rest = append([]string{args[0]}, args[2:]...)
		return rest, true
	}
	return args, false
}
