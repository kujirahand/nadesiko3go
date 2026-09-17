package format

import (
	"errors"
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/ast"
)

// Options は整形の追加指定。
type Options struct {
	// Colon は、『ここまで』で閉じるブロックを、行末の『:』とインデントで
	// 表すコロン記法に書き換える(#120)。
	Colon bool
}

// ParseFunc は、ソースを構文解析して構文木を返す関数(vm.ParseProgram)。
// formatパッケージ自身は標準命令の一覧を持たないので、呼び出し側から受け取る。
type ParseFunc func(code, filename string) (*ast.Node, error)

// ErrStructureChanged は、整形するとプログラムの構文構造が変わってしまう
// ため、整形を中止したことを表す。
var ErrStructureChanged = errors.New("整形すると構文構造が変わってしまうため中止しました")

// Program はcodeを整形した結果を返す。整形は次の段階に分けて行い、段階ごとに
// 結果を構文解析し直して、構文構造(Structure)が元のプログラムと一致する
// ことを確かめる。
//
//  1. 行の中身の書き方を揃える(空白・全角記号・『×』『÷』。→ normalize)。
//     構造が変わる場合は、値と助詞の間の空白を残して再試行し、それでも
//     だめならこの段階を飛ばす。
//  2. インデントを付け直す(→ Source)。構造が変わる場合はエラーにする。
//  3. opts.Colonなら、ブロックをコロン記法に書き換える。まとめて書き換えて
//     構造が変わる場合は、ブロックを1つずつ試し、変わらないものだけ残す。
//
// codeが構文エラーを含む場合は、parseが返したエラーをそのまま返す。
func Program(code, filename string, parse ParseFunc, opts Options) (string, error) {
	tree, err := parse(code, filename)
	if err != nil {
		return "", err
	}
	want := Structure(tree)
	// verifyは、候補が元と同じ構文構造を持つなら、その構文木を返す。
	verify := func(candidate string) (*ast.Node, error) {
		t, err := parse(candidate, filename)
		if err != nil {
			return nil, fmt.Errorf("整形結果が構文解析できなくなるため中止しました: %w", err)
		}
		if Structure(t) != want {
			return nil, ErrStructureChanged
		}
		return t, nil
	}

	cur, curTree := code, tree

	// 1. 行の中身の書き方を揃える。
	for _, joinJosi := range []bool{true, false} {
		candidate := normalize(cur, filename, joinJosi)
		if candidate == cur {
			break
		}
		if t, err := verify(candidate); err == nil {
			cur, curTree = candidate, t
			break
		}
	}

	// 2. インデントを付け直す。
	if candidate := Source(cur, filename, curTree); candidate != cur {
		t, err := verify(candidate)
		if err != nil {
			return "", err
		}
		cur, curTree = candidate, t
	}

	// 3. コロン記法に書き換える。
	if opts.Colon {
		cur = convertToColon(cur, filename, curTree, verify)
	}
	return cur, nil
}

// convertToColon は、コロン記法に書き換えられるブロックを書き換える。
// 書き換えた結果はインデントも付け直す(行を消しても他の行の深さは
// 変わらないが、念のため同じ整形を通す)。
func convertToColon(code, filename string, tree *ast.Node, verify func(string) (*ast.Node, error)) string {
	blocks := colonBlocks(code, filename, tree)
	if len(blocks) == 0 {
		return code
	}
	try := func(bs []colonBlock) (string, bool) {
		candidate := toColon(code, bs)
		t, err := verify(candidate)
		if err != nil {
			return "", false
		}
		if again := Source(candidate, filename, t); again != candidate {
			if _, err := verify(again); err != nil {
				return "", false
			}
			candidate = again
		}
		return candidate, true
	}
	if result, ok := try(blocks); ok {
		return result
	}
	// まとめて書き換えると構造が変わる場合は、1つずつ試して採用する。
	var accepted []colonBlock
	result := code
	for _, b := range blocks {
		if r, ok := try(append(append([]colonBlock(nil), accepted...), b)); ok {
			accepted = append(accepted, b)
			result = r
		}
	}
	return result
}
