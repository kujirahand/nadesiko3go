// Package format reformats なでしこ3 source code: it recomputes each line's
// indentation from the parsed block structure, trims trailing whitespace, and
// ensures the file ends with exactly one newline. It rewrites nothing else
// about a line.
//
// That is not on its own enough to keep a program's meaning, because
// indentation can itself be the syntax — 『!インデント構文』, or a line ending
// in 『:』 — and there the parser synthesises 『ここまで』 tokens that borrow a
// neighbouring line's number, so a body's real extent cannot be recovered
// from the tree. Callers must compare Structure(before) against
// Structure(after) and discard a result that does not match; `gonako format`
// refuses to write or print one.
package format

import (
	"fmt"
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

	// 改行コードは元のファイルに合わせる。Windowsで書かれたファイルを
	// 整形しただけで全行が変更扱いになるのを避ける。
	newline := "\n"
	if strings.Contains(code, "\r\n") {
		newline = "\r\n"
	}
	normalized := strings.ReplaceAll(code, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	// 末尾が改行なら、strings.Splitが作る空文字列の要素は行ではないので外す。
	// 出力には必ず改行を1つ付けるので、元の末尾が改行だったかは問わない。
	if len(lines) > 0 && lines[len(lines)-1] == "" {
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

	result := strings.Join(out, newline)
	if len(out) > 0 {
		result += newline
	}
	return result
}

// Structure renders a syntax tree as text — node types, names and literal
// values, and how they nest, with every source position left out. Two
// sources with the same Structure are the same program, differently laid
// out. Names and values are part of it so that a change inside a literal —
// the body of a multi-line string, say — shows up too, and not only a change
// in the shape of the tree.
//
// Reindenting is not always meaning-preserving: a source can express its
// blocks through indentation itself (『!インデント構文』, or a line ending in
// 『:』), and there the parser synthesises 『ここまで』 tokens that borrow a
// neighbouring line's number, so the block structure cannot be read back off
// the tree line by line. Comparing the Structure of the formatted source
// against the original catches that, and anything else a future change might
// get wrong, before the result reaches a file.
func Structure(tree *ast.Node) string {
	var b strings.Builder
	var walk func(n *ast.Node)
	walk = func(n *ast.Node) {
		if n == nil {
			b.WriteString("_")
			return
		}
		fmt.Fprintf(&b, "%s|%s|%s|%v(", n.Type, n.Name, n.Josi, n.Value)
		for _, child := range n.Blocks {
			walk(child)
			b.WriteString(",")
		}
		for _, arg := range n.Args {
			walk(arg)
			b.WriteString(";")
		}
		b.WriteString(")")
	}
	walk(tree)
	return b.String()
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

	// set records that something at this depth sits on the line. Several
	// constructs can claim one line, and the shallowest of them is the one
	// the line must be indented to, because a line starts at the level of
	// the outermost thing on it:
	//
	//   - 『もし〜ならば』の行を終端する改行は、そのブロック本文の最初の要素
	//     としても現れる。行はもし文のものなので、深い方ではなく浅い方。
	//   - インデント構文では、パーサーがデデント先の行の位置に『ここまで』と
	//     改行を合成する。その行には外側の本物の文があり、そちらが正しい。
	set := func(line, depth int) {
		if depth < 0 {
			depth = 0
		}
		if current, ok := depths[line]; ok && current <= depth {
			return
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
	// for nested bodies to place: an ast.Block child is a body one level
	// deeper (walkBlock handles it), and a chained 『違えば、もし』else-if or a
	// 『条件分岐』case label sits on its own line at the same depth as the
	// branch itself.
	//
	// Not every ast.Block is a body: 『AをXして、Bを表示』comma chaining (連文)
	// is wrapped as one too, purely so the parser can hand it around as a
	// single node. Such a chain needs no telling apart, because it lives on
	// the line it was invoked from, which its own statement has already
	// claimed at the shallower depth — and the shallowest claim on a line
	// wins.
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
			walkBlock(n, depth+1)
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
