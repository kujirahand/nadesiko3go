package nodelib

// Issue #374: cp932（x/text の japanese.ShiftJIS）の復号→再エンコードのプロパティテスト。
// 全単一バイトと全二重バイト組み合わせについて、次の2つを確かめる。
//
//  1. 意味の往復: 復号に成功し U+FFFD を含まない入力は、再エンコード→再復号で
//     同じ文字列に戻る（エラーにならない）。
//  2. バイトの往復: 再エンコード結果は元のバイト列と一致する。ただし cp932 には
//     同じ文字に複数のコードが割り当てられた「重複コード」があり、その範囲では
//     エンコーダが正規側のコードを選ぶため一致しない。重複コードは
//     cp932DuplicateCodes に理由つきで明示した範囲だけを例外とする。
//
// 例外の範囲は実測で確定した（x/text のバージョンを上げて例外が増減したら、
// このファイルの例外定義と件数アサートを実測に合わせて更新すること）。

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
)

// cp932DuplicateCodeReason は、バイト列（16進・大文字）が重複コードに当たる場合に
// その理由を返す。重複コードでなければ空文字列を返す。
//
// cp932 の重複コードは、NEC特殊文字・NEC選定IBM拡張・IBM拡張の各領域に、
// 同じ文字に対する別コードが存在することによる。復号はどちらのコードからも
// 同じ文字になり、符号化は片方（正規側）だけを返す。例:
//   - 8790 (≒ U+2252) → 正規側は 81E0
//   - FA40 (Ⅰの小文字 ⅰ U+2170) → 正規側は EEEF、FA4A (Ⅰ U+2160) → 8754
//   - FC40〜FC4B (IBM拡張漢字の一部) → 正規側は EEE0〜EEEB
//   - EEF9 (￢ U+FFE2) → 正規側は 81CA
func cp932DuplicateCodeReason(key string, in []byte) string {
	if len(in) == 2 && (in[0] == 0xFA || in[0] == 0xFB) {
		// FAxx・FBxx は全域が IBM拡張で、復号できる188件すべてが重複コード。
		return "IBM拡張（FA/FB）は NEC側と重複し、再エンコードで正規側のコードになる"
	}
	switch key {
	case "8790", "8791", "8792", "8795", "8796", "8797", "879A", "879B", "879C":
		return "NEC特殊文字（87xxの一部記号）は 81xx 側と重複する"
	case "FC40", "FC41", "FC42", "FC43", "FC44", "FC45", "FC46", "FC47", "FC48", "FC49", "FC4A", "FC4B":
		return "IBM拡張（FC40〜FC4B）は EE側と重複する"
	case "EEF9":
		return "EEF9 (￢) は 81CA と重複する"
	}
	return ""
}

func TestCP932RoundTripProperty(t *testing.T) {
	var (
		singleTotal, singleOK int
		doubleTotal, doubleOK int
		duplicates            int
		unencodable           int
		unexpected            []string
	)
	check := func(in []byte) bool {
		s, err := decodeBytes(in, japanese.ShiftJIS)
		if err != nil || strings.ContainsRune(s, '�') {
			return false
		}
		out, err := encodeString(s, japanese.ShiftJIS)
		if err != nil {
			// 0x80 は制御文字 U+0080 に復号されるが、符号化側に無いため戻せない（実測で確認した唯一の例）。
			if len(in) == 1 && in[0] == 0x80 {
				unencodable++
				return true
			}
			unexpected = append(unexpected, fmt.Sprintf("% X -> %+q -> 再エンコードがエラー: %v", in, s, err))
			return true
		}
		// 意味の往復は例外なく一致すること。
		back, err := decodeBytes(out, japanese.ShiftJIS)
		if err != nil || back != s {
			unexpected = append(unexpected, fmt.Sprintf("% X -> %+q -> % X -> %+q (err=%v)", in, s, out, back, err))
			return true
		}
		if bytes.Equal(out, in) {
			return true
		}
		// バイトが一致しない場合は、明示した重複コードの範囲だけを許す。
		key := fmt.Sprintf("%X", in)
		if reason := cp932DuplicateCodeReason(key, in); reason != "" {
			duplicates++
			return true
		}
		unexpected = append(unexpected, fmt.Sprintf("%s -> %+q -> % X", key, s, out))
		return true
	}

	for b := 0x00; b <= 0xFF; b++ {
		singleTotal++
		if check([]byte{byte(b)}) {
			singleOK++
		}
	}
	leads := []int{}
	for l := 0x81; l <= 0x9F; l++ {
		leads = append(leads, l)
	}
	for l := 0xE0; l <= 0xFC; l++ {
		leads = append(leads, l)
	}
	for _, l := range leads {
		for tr := 0x40; tr <= 0xFC; tr++ {
			if tr == 0x7F {
				continue
			}
			doubleTotal++
			if check([]byte{byte(l), byte(tr)}) {
				doubleOK++
			}
		}
	}

	// テストが空転していないことの確認。
	// リード60種 × トレイル188種 = 11280、単一バイト256種。
	if singleTotal != 256 {
		t.Errorf("単一バイトの総数 got=%d want=256", singleTotal)
	}
	if doubleTotal != 60*188 {
		t.Errorf("二重バイトの総数 got=%d want=%d", doubleTotal, 60*188)
	}
	// 実測（x/text v0.28 系）: 単一バイトは ASCII と半角カナ等で 192、二重バイトは 7724 が復号できる。
	if singleOK < 190 {
		t.Errorf("単一バイトの復号成功数が少なすぎる got=%d", singleOK)
	}
	if doubleOK < 6000 {
		t.Errorf("二重バイトの復号成功数が少なすぎる got=%d", doubleOK)
	}
	// 重複コードの件数も実測で固定する（FA 188 + FB 188 + FC 12 + 87xx 9 + EEF9 1 = 398、
	// 単一バイトには無い）。増減したらマッピング変更として理由を確認すること。
	if duplicates != 398 {
		t.Errorf("重複コードの件数 got=%d want=398（マッピングが変わった可能性があります）", duplicates)
	}
	if unencodable != 1 {
		t.Errorf("復号できるが符号化できない入力の件数 got=%d want=1 (0x80)", unencodable)
	}
	t.Logf("単一バイト %d/%d、二重バイト %d/%d が復号成功（重複コード %d 件）",
		singleOK, singleTotal, doubleOK, doubleTotal, duplicates)

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Errorf("往復しない入力が想定外にあります（%d件）:\n%s",
			len(unexpected), strings.Join(unexpected, "\n"))
	}
}

// x/text の ShiftJIS の代表的な単一バイト対応を固定する。
func TestCP932KnownMappings(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		// 0x5C はバックスラッシュ U+005C、0x7E はチルダ U+007E になる
		// （表示上は ¥ や ‾ に見える環境もあるが、コードポイントは ASCII 側。実測で確認済み）。
		{"0x5Cはバックスラッシュ", []byte{0x5C}, "\\"},
		{"0x7Eはチルダ", []byte{0x7E}, "~"},
		{"0xA1は半角カナ", []byte{0xA1}, "｡"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBytes(tt.in, japanese.ShiftJIS)
			if err != nil {
				t.Fatalf("err=%v", err)
			}
			if got != tt.want {
				t.Errorf("got=%q (%U) want=%q", got, []rune(got), tt.want)
			}
		})
	}
}
