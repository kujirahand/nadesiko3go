// Package format はなでしこ3のソースコードを整形する。構文解析済みの
// ブロック構造から各行のインデントを計算し直し、行末の空白を落とし、
// ファイルの末尾には必ず改行を1つだけ付ける。行についてそれ以外を
// 書き換えることはない。
//
// これだけではプログラムの意味を保つとは限らない。ソース自身がインデントで
// 構文を表すことがあり(『!インデント構文』や、行末が『:』で終わる記法)、その
// 場合パーサーはデデント先の行の位置を借りて『ここまで』トークンを合成する
// ため、ブロックの本当の範囲を構文木から読み取れなくなる。呼び出し側は
// Structure(整形前)とStructure(整形後)を比較し、一致しない結果は捨てなければ
// ならない。`gonako format`は一致しない結果を書き戻しも表示もせずに拒否する。
package format

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/ast"
	"github.com/kujirahand/nadesiko3go/internal/lexer"
	"github.com/kujirahand/nadesiko3go/internal/prepare"
)

// Unit はインデント1段につき挿入する空白。
const Unit = "    "

// Source は、既に構文解析済みの構文木(vm.ParseProgramが返すもの)を使って
// codeを整形する。treeはcode自身を解析したものでなければならない。そうで
// なければ、木が持つ行番号がcodeと噛み合わない。
//
// 複数行の文字列リテラルや範囲コメント(『...』・"..."・/*...*/)は、複数の
// 物理行にまたがる箇所ではバイト単位でそのまま残す。それらの行はインデント
// の段数を紐づけられる文を持たず、行頭の空白を書き換えればリテラルの中身を
// 書き換えてしまうからである。ただし開始行・終了行のうち、リテラルの外側に
// あたる部分(開き引用符・閉じ引用符と同じ行にある他のコード)は整形する。
func Source(code, filename string, tree *ast.Node) string {
	depths := lineDepths(tree, filename) // ast.Node.Line(0始まり) -> 深さ
	spans := protectedSpans(code, filename)
	lines := splitPhysicalLines(code)
	// 末尾の空行はすべて落とし、末尾には改行を1つだけ付ける。保護範囲は実際の
	// トークンの範囲であり空行にはかからないので、この後の行番号との対応に
	// 影響しない。
	for len(lines) > 0 && strings.TrimRight(lines[len(lines)-1].text, " \t　") == "" {
		lines = lines[:len(lines)-1]
	}

	// 出力全体で使う改行コード。元ファイルにCRLFが1つでもあれば統一してCRLF
	// にする。保護範囲(複数行リテラル・範囲コメントの内部)の改行だけは、この
	// 統一に従わせず元の綴りのまま残す。
	newline := "\n"
	if strings.Contains(code, "\r\n") {
		newline = "\r\n"
	}

	// startAt/endAtは、複数行にまたがる保護範囲がその行で始まる・終わること
	// を示す。inside[i]は、行i全体が保護範囲の内部(開始行・終了行を除く)に
	// あることを示す。
	startAt := map[int]protectedSpan{}
	endAt := map[int]protectedSpan{}
	inside := make([]bool, len(lines))
	for _, sp := range spans {
		startAt[sp.startLine] = sp
		endAt[sp.endLine] = sp
		for l := sp.startLine + 1; l < sp.endLine && l < len(inside); l++ {
			inside[l] = true
		}
	}

	depth := 0
	out := make([]string, len(lines))
	rawEnding := make([]bool, len(lines)) // 直後の改行を、統一newlineではなく元の綴りで出す

	reindent := func(text string, lineNo int) string {
		trimmed := strings.TrimRight(text, " \t　")
		_, skip := lexer.CountIndent(trimmed)
		runes := []rune(trimmed)
		if skip > len(runes) {
			skip = len(runes)
		}
		content := string(runes[skip:])
		if content == "" {
			return ""
		}
		lineDepth := depth + 1
		if d, ok := depths[lineNo]; ok {
			depth = d
			lineDepth = d
		}
		return strings.Repeat(Unit, lineDepth) + content
	}

	for i, pl := range lines {
		endSp, hasEnd := endAt[i]
		startSp, hasStart := startAt[i]
		runes := []rune(pl.text)
		switch {
		case inside[i]:
			// リテラル・範囲コメントの内部の行はそのまま。
			out[i] = pl.text
			rawEnding[i] = true
		case hasEnd && hasStart:
			// 1つの行に、手前のリテラル・範囲コメントの閉じ記号と、次の
			// リテラルの開き記号が両方ある(範囲コメントの直後に文字列が
			// 始まるなど、非常に稀なケース)。閉じ記号までと開き記号から
			// 先はそのまま残し、その間(行の途中なのでインデントは付け
			// 直さない)だけ末尾の空白を落とす。
			endCol := clampCol(endSp.endCol, len(runes))
			startCol := clampCol(startSp.startCol, len(runes))
			if startCol < endCol {
				startCol = endCol
			}
			middle := strings.TrimRight(string(runes[endCol:startCol]), " \t　")
			out[i] = string(runes[:endCol]) + middle + string(runes[startCol:])
			rawEnding[i] = true
		case hasStart:
			col := clampCol(startSp.startCol, len(runes))
			out[i] = reindent(string(runes[:col]), i) + string(runes[col:])
			rawEnding[i] = true
		case hasEnd:
			col := clampCol(endSp.endCol, len(runes))
			// 閉じ記号より後ろに続くコードは、行頭ではないのでインデントは
			// 付け直さず、末尾の空白だけ落とす。
			out[i] = string(runes[:col]) + strings.TrimRight(string(runes[col:]), " \t　")
		default:
			out[i] = reindent(pl.text, i)
		}
	}

	var b strings.Builder
	for i, s := range out {
		b.WriteString(s)
		if i == len(out)-1 {
			b.WriteString(newline) // 末尾の改行が無くても必ず1つ付ける
			break
		}
		if rawEnding[i] {
			b.WriteString(lines[i].ending)
		} else {
			b.WriteString(newline)
		}
	}
	return b.String()
}

func clampCol(col, max int) int {
	if col < 0 {
		return 0
	}
	if col > max {
		return max
	}
	return col
}

// Structure は構文木を、SourceMap(行・列・オフセット)を除いた意味のある
// フィールドすべてとその入れ子関係だけで表したテキストにする。2つのソースの
// Structureが一致すれば、レイアウトが違うだけの同じプログラムだと言える。
//
// インデントの付け替えは、それだけでは意味を保つとは限らない。ソース自身が
// インデントで構文を表すことがあり(『!インデント構文』や、行末が『:』で終わる
// 記法)、その場合パーサーはデデント先の行の位置を借りて『ここまで』トークンを
// 合成するため、ブロックの本当の範囲を構文木から行単位で読み取れない。
// 整形前後のStructureを比較し、一致しなければ結果を捨てることで、この場合と、
// 将来の変更が引き起こしうる未知の破壊の両方を検出できる。
//
// ast.EOL は改行そのもの、つまりレイアウトの情報であり意味には関与しない。
// 末尾に改行があるかどうかだけで(ルートのBlocksの末尾に)EOLノードの有無が
// 変わってしまうため、比較対象から除く。そうしないと、末尾改行の有無を
// 揃えるというSourceの仕様そのものによって、あらゆる整形が拒否されてしまう。
func Structure(tree *ast.Node) string {
	var b strings.Builder
	var walk func(n *ast.Node)
	walkList := func(nodes []*ast.Node, sep string) {
		for _, n := range nodes {
			if n != nil && n.Type == ast.EOL {
				// EOL自体を飛ばすだけでなく、区切り文字も出さない。そうしない
				// と要素数の違い(末尾改行の有無でEOLが1個増減する)が区切り文字
				// の個数として残ってしまう。
				continue
			}
			walk(n)
			b.WriteString(sep)
		}
	}
	walk = func(n *ast.Node) {
		if n == nil {
			b.WriteString("_")
			return
		}
		if n.Type == ast.EOL {
			return
		}
		fmt.Fprintf(&b, "%s|%s|%s|%s|%v|%s|%s|%s|%s|%d|%t|%t|%t|%t|%t|%t(",
			n.Type, n.Name, n.Josi, n.RawJosi, n.Value, n.Operator, n.VarType,
			n.Word, n.LoopDirection, n.CaseCount,
			n.IsExport, n.AsyncFn, n.Setter, n.CheckInit, n.FlagDown, n.FlagUp)
		if len(n.Options) > 0 {
			keys := make([]string, 0, len(n.Options))
			for k := range n.Options {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&b, "%s=%t;", k, n.Options[k])
			}
		}
		b.WriteString("[")
		walkList(n.Blocks, ",")
		b.WriteString("][")
		walkList(n.Index, ",")
		b.WriteString("][")
		walkList(n.Args, ",")
		b.WriteString("][")
		walkList(n.Names, ",")
		b.WriteString("])")
	}
	walk(tree)
	return b.String()
}

// physicalLine は改行文字そのものを取り除いた1行分のテキストと、その行の
// 直後にあった改行の綴り("\n"・"\r\n"・"\r"、最終行で改行がなければ空文字)を
// 保持する。改行の綴りを残すのは、保護範囲(複数行リテラル・範囲コメントの
// 内部)の改行だけは、ファイル全体で統一する改行コードに合わせず元のまま
// 出力するため(CRLFとLFが混在するファイルで、リテラルの中身を壊さない)。
type physicalLine struct {
	text   string
	ending string
}

// splitPhysicalLines は、CRLF・CR・LFのいずれも改行として認める点を除けば
// ただの行分割で、ast.Node.Lineの番号付けと同じルールになる(internal/prepare
// も同じ3種類を改行として正規化する)。
func splitPhysicalLines(code string) []physicalLine {
	runes := []rune(code)
	var lines []physicalLine
	lineStart := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '\r' && r != '\n' {
			continue
		}
		text := string(runes[lineStart:i])
		ending := string(r)
		if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
			ending = "\r\n"
			i++
		}
		lines = append(lines, physicalLine{text: text, ending: ending})
		lineStart = i + 1
	}
	if lineStart < len(runes) || len(lines) == 0 {
		lines = append(lines, physicalLine{text: string(runes[lineStart:]), ending: ""})
	}
	return lines
}

// protectedSpan は、複数の物理行にまたがる文字列リテラル・範囲コメントが
// 元のソース上で占める範囲を、開始・終了それぞれの物理行番号(0-based、
// ast.Node.Lineと同じ体系)とその行内でのrune位置(0-based)で表す。endCol・
// endLineが指す位置そのものはリテラルの外(半開区間の終端)。
type protectedSpan struct {
	startLine, startCol int
	endLine, endCol     int
}

// protectedSpans は、複数行にわたる文字列リテラル・範囲コメントをすべて
// 見つける。これによりSourceは、その内部には手を付けずに、開始・終了行で
// リテラルと同居する他のコードだけを整形できる。ParseSourceが構文解析の
// 前段で通す字句解析の最初の段階(prepareしてからTokenize)だけを再実行して
// おり、フルパイプラインは通さない。インデント構文の変換やrequireの解決は
// 失敗したりトークン列を書き換えたりする可能性があり、ここでは無関係な
// うえ、このベストエフォートな処理が何も見つけられない要因を増やすだけ
// だからである。
//
// Tokenizeはprepare後のテキストに対して行うので、トークンのOffsetはcode
// ではなくそのテキストへの添字である。prepare.NewlineOriginalOffsetsは、
// その添字を元のcode自身のrune offsetへ戻す(全角記号の折り畳みは文字の
// 位置を変えないが、改行の正規化だけは変える。このマッピングはちょうど
// それを打ち消す)。そこから得られる範囲は、ast.Node.Lineと同じ数え方の
// code自身の物理行で表されるので、Sourceはこれ以上何も導出し直さずに
// その範囲を特定できる。
//
// Tokenizeは、フルパイプラインなら受け付けるはずの入力でも失敗することが
// ある(フルパイプラインは先にインデント構文の変換を通し、一部のソースは
// それに依存している)。ここで失敗した場合は単に何も保護しないだけであり、
// formatはvm.ParseProgram自体が成功した後にしか動かないので、それと整合する。
func protectedSpans(code, filename string) []protectedSpan {
	prepared := prepare.Text(prepare.Convert(code))
	tokens, err := lexer.Tokenize(prepared, 0, filename)
	if err != nil {
		return nil
	}
	preparedRunes := []rune(prepared)
	toOriginal := prepare.NewlineOriginalOffsets(code)
	origRunes := []rune(code)
	origLineStart := originalLineStarts(origRunes)

	var spans []protectedSpan
	for _, tok := range tokens {
		switch tok.Type {
		case lexer.TypeString, lexer.TypeStringEx, lexer.TypeRangeComment:
		default:
			continue
		}
		start, end := tok.Offset, tok.Offset+tok.Length
		if start < 0 || end > len(preparedRunes) || start >= end {
			continue
		}
		origStart := originalOffset(toOriginal, start)
		origEnd := originalOffset(toOriginal, end)
		startLine, startCol := lineColOf(origLineStart, origStart)
		endLine, endCol := lineColOf(origLineStart, origEnd)
		if startLine == endLine {
			continue // 単一行に収まるリテラルは、インデントの付け替え対象のまま
		}
		spans = append(spans, protectedSpan{
			startLine: startLine, startCol: startCol,
			endLine: endLine, endCol: endCol,
		})
	}
	return spans
}

// originalLineStarts は、CRLF・CR・LFのいずれも改行として、各物理行の先頭が
// runesの何文字目かを返す(splitPhysicalLinesと同じ数え方)。
func originalLineStarts(runes []rune) []int {
	starts := []int{0}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '\r' && r != '\n' {
			continue
		}
		if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
			i++
		}
		starts = append(starts, i+1)
	}
	return starts
}

// originalOffset は、prepare後のテキストでのrune位置iを元のソースのrune
// 位置に変換する。iがtoOriginalの範囲を超える(トークンがファイル末尾で
// 終わる)場合は、元のソースの末尾の1つ先を返す。
func originalOffset(toOriginal []int, i int) int {
	if i < len(toOriginal) {
		return toOriginal[i]
	}
	if len(toOriginal) == 0 {
		return 0
	}
	return toOriginal[len(toOriginal)-1] + 1
}

// lineColOf は、lineStarts(originalLineStartsが返すもの)を使って、元の
// ソース上のrune位置offsetがどの物理行(0-based)の何文字目(0-based)かを返す。
func lineColOf(lineStarts []int, offset int) (line, col int) {
	// 行数が大きくないので線形探索で十分。
	line = 0
	for line+1 < len(lineStarts) && lineStarts[line+1] <= offset {
		line++
	}
	return line, offset - lineStarts[line]
}

// lineDepths は、文を持つ物理行番号(0始まり、ast.Node.Lineと同じ体系)を
// そのインデント深さへ対応付ける。これはパーサーが既に組み立てたブロック
// 構造から導く: 対応する『ここまで』を必要とする構文(もし・繰り返す・
// 関数定義など)はどれも自分の本体をast.Blockとしてパーサーに渡し、その
// ブロックは本体の始まりと、閉じるキーワード(ここまで・違えば・
// エラーならば)の位置の両方を記録している。
//
// 分岐そのものを、それを囲むast.Blockを持たずに表す文もある――同じ分岐上
// で別の もし に繋がる『違えば、もし』チェーンや、1行で書く『もし〜ならば
// 〜』のthen/elseがそれである。これらは1段深い本体ではなく、その分岐の行
// の内容そのものなので、分岐自身と同じ深さで辿る。
//
// filenameは整形しているファイル自身のパスで、Sourceが受け取ったfilename
// と同じ値を渡す。『!「file」を取込』で展開されたノードは、取込先ファイル
// 自身の行番号(そちらも0始まり)を持ったまま木に混ざり込むため、filename
// と異なるNode.Fileを持つノードは深さの計算から除外する。除外しないと、
// 取込先ファイルのトップレベルの行番号が、本体側の同じ行番号のブロック内
// の文と衝突し、「最も浅い深さを採用する」規則によって本体側の字下げが
// 消されてしまう(取込先の行が本体の行より浅くなりがちなため)。
func lineDepths(root *ast.Node, filename string) map[int]int {
	depths := map[int]int{}

	// setは、この深さの何かがその行にあることを記録する。1つの行を複数の
	// 構文が主張することがあり、そのうち最も浅いものがその行を実際に
	// 合わせるべき深さになる。行は、その行にある最も外側のものの深さで
	// 始まるからである:
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

	// walkBlockは、ast.Blockの中のすべての文をbodyDepthに置く――
	// block.Blocksを通って辿り着くものは、構造上必ず値ではなく文だから、
	// これは常に成り立つ――そのうえで、それぞれの内側を見て入れ子の本体を
	// さらに探す。ブロック自身を閉じるキーワードの行は、1段浅い深さに置く。
	//
	// stmt.File・block.End.Fileがfilenameと異なる場合はスキップする:
	// 『!「file」を取込』で展開されたノードは、取込先ファイル自身の行番号
	// (そちらも0始まりなので本体の行番号と衝突しうる)を持ったまま木に
	// 混ざり込むため、それらは整形対象(filename)の行ではないと分かって
	// いなければならない。
	walkBlock = func(block *ast.Node, bodyDepth int) {
		if block == nil {
			return
		}
		for _, stmt := range block.Blocks {
			if stmt == nil || stmt.File != filename {
				continue
			}
			set(stmt.Line, bodyDepth)
			scan(stmt, bodyDepth)
		}
		if block.End != nil && block.End.File == filename {
			set(block.End.Line, bodyDepth-1)
		}
	}

	// scanは、文(またはすでに自分の深さが決まっている分岐)の内側を見て、
	// 置くべき入れ子の本体を探す: 子がast.Blockなら1段深い本体(walkBlockが
	// 処理する)。同じ分岐上で別の もし に繋がる『違えば、もし』チェーンや、
	// 『条件分岐』のcaseラベルは、その分岐自身と同じ深さの、それ自身の行に
	// 置く。
	//
	// すべてのast.Blockが本体というわけではない: 『AをXして、Bを表示』の
	// ようなコンマでの連文も、パーサーが1つのノードとして受け渡すためだけ
	// にast.Blockとして包まれる。この連文をわざわざ見分ける必要はない――
	// それが呼び出された行そのものに収まっており、その行はすでにその文
	// 自身によって、より浅い深さで確保されているからである。1つの行を
	// 複数の構文が主張するときは、最も浅い主張が勝つ。
	//
	// それ以外――条件式、呼び出しの引数、配列・辞書リテラル――は値であって
	// 文ではないので、そのまま何もしない: 複数の物理行にまたがる式(たとえば
	// 複数行の配列リテラル)自身は、その行に紐づける文を持たないので、
	// Sourceはそれらを直前の文の続きとして扱う。(そのような値の奥に埋め
	// 込まれた制御構文の本体、たとえば呼び出しの引数として渡した無名関数
	// の本体は、既知の制限としてインデントの再計算対象にならない。)
	scan = func(n *ast.Node, depth int) {
		if n == nil || n.File != filename {
			// walkBlockから呼ぶ場合はここに来る前に確認済みだが、If・Switch
			// などが子として持つノードが取込先ファイル由来になることは
			// 通常ないものの、念のためここでも確認する。
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
		case ast.JSONArray, ast.JSONObj:
			// 配列・辞書リテラルの閉じ記号(]・})がある物理行は、その文自身と
			// 同じ深さになる。子は要素なので深さは設定せず、直前の文より
			// 1段深い続きとして扱わせる。
			if n.End != nil {
				set(n.End.Line, depth)
			}
			for _, child := range n.Blocks {
				scan(child, depth)
			}
		default:
			// 繰り返す・間・反復・関数定義など、他の構文の本体もここを通って
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
