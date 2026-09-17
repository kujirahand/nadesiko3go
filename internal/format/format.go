// Package format reformats なでしこ3 source code: it recomputes each line's
// indentation from the parsed block structure, trims trailing whitespace, and
// ensures the file ends with exactly one newline. It never rewrites anything
// else about a line, so a program that still runs the same way before
// formatting still runs the same way after.
package format

import (
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Unit is the whitespace inserted per indentation level.
const Unit = "    "

// Source reformats code using its already-parsed syntax tree (as returned by
// vm.ParseProgram). tree must have been parsed from code itself, or the line
// numbers it carries will not line up.
//
// Lines inside a multi-line string literal or range comment (『...』・"..."・
// /*...*/ spanning more than one line) are left byte-for-byte alone: they
// carry no statement of their own to hang an indentation level off, and
// rewriting their leading whitespace would rewrite the literal's contents.
func Source(code, filename string, tree *ast.Node) string {
	depths := lineDepths(tree)
	protected := protectedLines(code, filename)

	normalized := strings.ReplaceAll(code, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	// strings.Splitは末尾の改行の後に空文字列の要素を作る。最終行として扱わず、
	// 出力の末尾に改行を1つ付けるかどうかの判定にだけ使う。
	hadTrailingNewline := len(lines) > 0 && lines[len(lines)-1] == ""
	if hadTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	depth := 0
	out := make([]string, len(lines))
	for i, line := range lines {
		// ast.Node.Lineは0始まりの行番号なので、そのままスライス添字と対応する。
		if protected[i] {
			out[i] = line
			continue
		}
		trimmed := strings.TrimRight(line, " \t　")
		content := strings.TrimLeft(trimmed, " \t　")
		if content == "" {
			out[i] = ""
			continue
		}
		// 行がどの文にも対応しない場合(複数行にわたる配列・辞書リテラルの
		// 続き行など)は、直前の文より1段深く続きとして扱う。
		lineDepth := depth + 1
		if d, ok := depths[i]; ok {
			depth = d
			lineDepth = d
		}
		out[i] = strings.Repeat(Unit, lineDepth) + content
	}

	result := strings.Join(out, "\n")
	if len(out) > 0 {
		result += "\n"
	}
	return result
}

// protectedLines finds every physical line that a multi-line string literal
// or range comment spans, so Source can leave it untouched. It re-runs only
// the lexer's first stage (the same prepare+tokenize pair ParseSource uses
// before the parser proper) rather than the full pipeline: indentation-syntax
// conversion and requires resolution can fail or rewrite the token stream in
// ways irrelevant here, and would only add ways for this best-effort pass to
// come back empty. prepare folds individual characters and never touches a
// string or comment body nor the line count (internal/prepare), so a token's
// Line still names the same physical line in the original code.
//
// Tokenizing can fail on input the full pipeline would still accept (that
// pipeline runs indentation conversion first, which some sources depend on).
// A failure here just means no lines are protected, matching how format
// already only runs after vm.ParseProgram itself succeeded.
func protectedLines(code, filename string) map[int]bool {
	protected := map[int]bool{}
	prepared := prepare.Text(prepare.Convert(code))
	tokens, err := lexer.Tokenize(prepared, 0, filename)
	if err != nil {
		return protected
	}
	runes := []rune(prepared)
	for _, tok := range tokens {
		switch tok.Type {
		case lexer.TypeString, lexer.TypeStringEx, lexer.TypeRangeComment:
		default:
			continue
		}
		start, end := tok.Offset, tok.Offset+tok.Length
		if start < 0 || end > len(runes) || start > end {
			continue
		}
		span := strings.Count(string(runes[start:end]), "\n")
		if span == 0 {
			continue // 単一行のリテラルはインデントの付け替え対象のまま
		}
		for l := tok.Line; l <= tok.Line+span; l++ {
			protected[l] = true
		}
	}
	return protected
}

// lineDepths maps each physical line number (0-based, matching ast.Node.Line)
// that carries a statement to its indentation depth, derived from the block
// structure the parser already worked out: every construct that requires a
// matching 『ここまで』(もし・くり返す・関数定義など) hands its body to the
// parser as an ast.Block, and that block records both where its content
// starts and where its closing keyword (ここまで・違えば・エラーならば) sits.
//
// A statement that itself stands in for a whole branch without an ast.Block
// around it — 『違えば、もし』chaining onto another もし on the same branch,
// or a one-line 『もし〜ならば〜』then/else — is not a body one level deeper;
// it is just what that branch's line contains, so it is walked at the same
// depth as the branch itself.
func lineDepths(root *ast.Node) map[int]int {
	depths := map[int]int{}
	set := func(line, depth int) {
		if _, already := depths[line]; already {
			// 最初にその行へ到達した時点の深さを採用する。行を辿る順序は必ず
			// 外側の構文(もし・くり返すなど)が先、内側の中身が後になるので、
			// 最初の書き込みが常にその行の正しい所属先を表す。
			// (例: 『もし〜ならば』行の直後の改行はブロック本文の最初の要素
			// としても現れるが、本来はもし文自身と同じ行にすぎない)
			return
		}
		if depth < 0 {
			depth = 0
		}
		depths[line] = depth
	}

	var scan func(n *ast.Node, depth int)
	var walkBlock func(block *ast.Node, bodyDepth int)

	// walkBlock places every statement in an ast.Block at bodyDepth — always,
	// since anything reached through block.Blocks is a statement by
	// construction, not a value — then looks inside each one for further
	// nested bodies. It places the block's own closing keyword's line one
	// level shallower.
	walkBlock = func(block *ast.Node, bodyDepth int) {
		if block == nil {
			return
		}
		for _, stmt := range block.Blocks {
			if stmt == nil {
				continue
			}
			set(stmt.Line, bodyDepth)
			scan(stmt, bodyDepth)
		}
		if block.End != nil {
			set(block.End.Line, bodyDepth-1)
		}
	}

	// scan looks inside a statement (or a branch already at its own depth)
	// for nested bodies to place: an ast.Block child spanning more than one
	// physical line is a body one level deeper (walkBlock handles it); a
	// chained 『違えば、もし』else-if, or a 『条件分岐』case label, sits on its
	// own line at the same depth as the branch itself; a same-line
	// 『AをXして、Bを表示』comma chain (連文, which the parser also wraps as an
	// ast.Block purely to hand it around as one node) has nothing to
	// reindent, since its whole body lives on the one line it was invoked
	// from.
	//
	// Anything else — condition expressions, call arguments, array and
	// object literals — is a value, not a statement, and is left alone: an
	// expression that happens to span several physical lines (a multi-line
	// array literal, say) has no statement of its own on those lines to hang
	// a depth off, so Source treats them as a continuation of whatever
	// statement came before instead. (A control-flow body embedded deeper
	// inside such a value, e.g. an anonymous function literal passed as a
	// call argument, is a known gap: its own body will not be reindented.)
	scan = func(n *ast.Node, depth int) {
		if n == nil {
			return
		}
		switch n.Type {
		case ast.Block:
			if n.End != nil && n.End.Line > n.Line {
				walkBlock(n, depth+1)
			} else {
				for _, child := range n.Blocks {
					scan(child, depth)
				}
			}
		case ast.If:
			set(n.Line, depth)
			for _, child := range n.Blocks {
				scan(child, depth)
			}
		case ast.Switch:
			set(n.Line, depth)
			// Blocks = [値, 既定ブロック, 条件1, ブロック1, 条件2, ブロック2, ...]
			for i, child := range n.Blocks {
				if child == nil {
					continue
				}
				if i >= 2 && i%2 == 0 {
					set(child.Line, depth) // 「〇〇のとき」の行
					continue
				}
				scan(child, depth)
			}
		default:
			// くり返す・間・反復・関数定義など、他の構文の本体もここを通って
			// 見つかる: 本体はどれもBlocksのどこかにあるast.Blockなので、
			// 深さを設定せずに子をたどるだけでBlockケースまで届く。式の途中
			// (配列・辞書リテラルの要素など)をここで行の深さに関連付けない
			// ことで、複数行リテラルを直前の文の続きとして扱えるようにする。
			for _, child := range n.Blocks {
				scan(child, depth)
			}
		}
	}

	walkBlock(root, 0)
	return depths
}
