package nodelib_test

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/vm"
)

// TestExitEscapesTry は『終了』『強制終了』がエラー監視に捕まらず、ハンドラも
// 後続の処理も実行しないことを確かめる (#337)。
func TestExitEscapesTry(t *testing.T) {
	tests := []struct {
		name string
		code string
		// out は標準出力の期待値（contains が空のときの完全一致）
		out string
		// contains は完全一致できない出力（エラーメッセージを含む）の部分一致
		contains []string
		exited   bool
		exitCode int
	}{
		{
			name:   "1_エラー監視内の終了",
			code:   "エラー監視\n  終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:    "",
			exited: true,
		},
		{
			name:   "2_エラー監視内でユーザー定義関数が終了",
			code:   "●止める処理とは\n  終了\nここまで\nエラー監視\n  止める処理を実行\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:    "",
			exited: true,
		},
		{
			name:   "3_入れ子のエラー監視内の終了",
			code:   "エラー監視\n  エラー監視\n    終了\n  エラーならば\n    「内側」を表示\n  ここまで\nエラーならば\n  「外側」を表示\nここまで\n「後続」を表示\n",
			out:    "",
			exited: true,
		},
		{
			name:     "4_エラー監視内の強制終了",
			code:     "エラー監視\n  5で強制終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:      "",
			exited:   true,
			exitCode: 5,
		},
		{
			// 『止める処理』のような名前は関数名の正規化で『止処理』になり、
			// 文字列で指定する『実行』では引けないので、正規化されない名前を使う。
			name:   "5_実行命令で呼んだ関数の中で終了",
			code:   "●おわり処理とは\n  終了\nここまで\nエラー監視\n  「おわり処理」を実行\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:    "",
			exited: true,
		},
		{
			name:   "6_エラー監視内の待機中にタイマーが終了",
			code:   "エラー監視\n  0.01秒後には\n    終了\n  ここまで\n  1秒待つ\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:    "",
			exited: true,
		},
		{
			name:   "7_main終了後の単発コールバック内の終了",
			code:   "0.01秒後には\n  終了\nここまで\n0.02秒後には\n  「CB2」を表示\nここまで\n「main終わり」を表示\n",
			out:    "main終わり\n",
			exited: true,
		},
		{
			name:     "8_対照_通常のエラーは捕捉される",
			code:     "エラー監視\n  「E」でエラー発生\nエラーならば\n  「捕捉された:」&エラーメッセージを表示\nここまで\n「後続」を表示\n",
			contains: []string{"捕捉された:", "E", "後続"},
			exited:   false,
		},
		{
			name:   "9_対照_終了前の表示は残る",
			code:   "エラー監視\n  「A」を表示\n  終了\nエラーならば\n  「捕捉された」を表示\nここまで\n「後続」を表示\n",
			out:    "A\n",
			exited: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			host := vm.NewCUIHost(&out, strings.NewReader(""), nil)
			err := vm.RunProgram(tc.code, "main.nako3", host)
			if err != nil {
				t.Fatalf("エラーが返った: %#v", err)
			}
			if tc.contains != nil {
				for _, s := range tc.contains {
					if !strings.Contains(out.String(), s) {
						t.Errorf("出力 %q に %q が含まれない", out.String(), s)
					}
				}
			} else if out.String() != tc.out {
				t.Errorf("出力 = %q, want %q", out.String(), tc.out)
			}
			if host.Exited != tc.exited || host.ExitCode != tc.exitCode {
				t.Errorf("Exited=%v ExitCode=%d, want %v/%d", host.Exited, host.ExitCode, tc.exited, tc.exitCode)
			}
		})
	}
}
