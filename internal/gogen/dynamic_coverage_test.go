package gogen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// このファイルは #432 のチェック項目「dynamicFuncCommands への追加漏れ
// 検知」の静的照合テスト。gogen 対象プラグイン (plugins.go の
// pluginConstructors と plugin_system) の実装を go/ast で走査し、
// FindFunc / FindValue / CallFunc / PostFunc / CallCommand / toFunc の
// いずれかへ到達する命令を機械的に洗い出して、dynamicFuncCommands と
// 突き合わせる。
//
// 照合は fail-closed で、例外は理由つきの対応表でだけ認める:
//   - dynamicStateConsumers: コールバックを CommandState 経由で消費する
//     ため走査では検出されるが、対象関数の出所は保存側の命令 (一覧に
//     ある) で決まる命令。新しい消費命令が増えたら、ここへ理由を書いて
//     追加するか、一覧側へ移すかをレビューで判断すること。
//   - dynamicUndetectable: 一覧にあるが、命令自身の実装からは走査で
//     検出できない命令 (保存側と、ONERROR 消費側として #335 で入った
//     取得系)。実装が変わって検出されるようになれば、こちらからは
//     外すこと (テストが二重所属を弾く)。
//
// ヘルパー (toFunc / eachElement / post / copyMergeWithProgress など) は
// 呼び出しの伝播で扱う: ヘルパーを呼ぶ命令は、ヘルパーが動的なら
// その命令も動的とみなす。
//
// 限界: 関数値をマップ・構造体・引数経由で間接的に受け渡す経路は
// 追えない。その形で動的呼び出しを足す場合は、走査が検出できず
// このテストも通ってしまうので、dynamicFuncCommands へ手で足し、
// 検出できない理由を dynamicUndetectable へ書くこと。
// コールバックの引数位置 (第0引数か) は TestDynamicFuncArgPositions が
// 別途固定する。

// sinkMethods は、ユーザー関数や名前解決へ到達する Context メソッド。
var sinkMethods = map[string]bool{
	"FindFunc": true, "FindValue": true, "CallFunc": true,
	"PostFunc": true, "CallCommand": true,
}

// dynamicStateConsumers は「走査で検出されるが一覧には入れない」命令と
// その理由 (対象の出所である保存側の命令)。
var dynamicStateConsumers = map[string]string{
	"ファイルコピー":   "『ファイル処理時』が CommandState へ保存したコールバックを copyMergeWithProgress 経由で消費するだけ",
	"ファイル上書コピー": "同上",
	"ファイル移動":    "『ファイル処理時』の保存コールバックを moveEntry → copyMergeWithProgress 経由で消費するだけ",
	"ファイル上書移動":  "同上",
}

// dynamicUndetectable は「一覧にあるが走査では検出されない」命令と理由。
var dynamicUndetectable = map[string]string{
	"ファイル処理時":     "コールバックを CommandState へ保存する側。消費側は dynamicStateConsumers",
	"AJAX失敗時":     "コールバックを AJAX:ONERROR へ保存する側 (SetSysVar のみで sink が無い)",
	"AJAXテキスト取得":  "#335 で AJAX:ONERROR の消費側として一覧入り。現行実装は ONERROR を直接消費しないが、保存側と対で安全側に残す",
	"AJAX内容取得":    "同上 (AJAXテキスト取得の別名)",
	"AJAX受信":      "同上 (AJAXテキスト取得の別名)",
	"AJAX_JSON取得": "同上",
	"AJAXバイナリ取得":  "同上",
	"POST送信":      "同上",
	"POSTフォーム送信":  "同上",
}

// pkgFunc はパッケージ内の名前つき関数/メソッド/関数変数の本体。
type pkgFunc struct {
	body *ast.BlockStmt // FuncLit の場合はその Body、FuncDecl の場合は Body
	expr ast.Expr       // 関数変数で、本体が式 (別名など) の場合
}

// pkgScan は1パッケージ分の走査結果。
type pkgScan struct {
	// funcs は関数名 (メソッドはメソッド名のみ) → 本体の列。同名の
	// メソッド (レシーバ違い) や同名変数があっても上書きで本体を
	// 失わないよう、全部を保持し「どれか1つでも動的なら動的」とする
	funcs  map[string][]*pkgFunc
	regs   map[string]ast.Expr // 命令名 → fn を表す式
	dynFn  map[string]bool     // 動的と判定された関数名
	dynCmd map[string]bool     // 動的と判定された命令名
}

func TestDynamicFuncCommandsCoverage(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	internalDir := filepath.Dir(filepath.Dir(file)) // internal/gogen -> internal
	pkgs := []string{"stdlib"}
	for name := range pluginConstructors {
		pkgs = append(pkgs, name)
	}
	sort.Strings(pkgs)

	discovered := map[string]bool{}
	for _, pkg := range pkgs {
		scan := scanPluginPackage(t, filepath.Join(internalDir, pkg))
		for name := range scan.dynCmd {
			discovered[name] = true
		}
	}

	// 走査が黙って空振りしていないことの最低限の歯止め
	for _, must := range []string{"実行", "配列マップ", "秒後", "ハテナ関数実行", "SQLITE3実行時", "標準入力取得時"} {
		if !discovered[must] {
			t.Errorf("静的走査が %q を検出できていない (走査側の劣化の可能性)", must)
		}
	}

	// 検出された命令は、すべて一覧にあるか、状態消費の例外として
	// 理由が記録されていなければならない (= 追加漏れの検知)
	for name := range discovered {
		if _, ok := dynamicFuncCommands[name]; ok {
			continue
		}
		if _, ok := dynamicStateConsumers[name]; ok {
			continue
		}
		t.Errorf("動的呼び出しへ到達する命令 %q が dynamicFuncCommands にも例外表にもない (追加漏れ)", name)
	}
	// 一覧の命令は、すべて検出されるか、検出できない理由が記録されて
	// いなければならない (= 一覧の腐敗・分類ミスの検知)
	for name := range dynamicFuncCommands {
		if discovered[name] {
			if _, dup := dynamicUndetectable[name]; dup {
				t.Errorf("%q は走査で検出されるようになったので dynamicUndetectable から外すこと", name)
			}
			continue
		}
		if _, ok := dynamicUndetectable[name]; !ok {
			t.Errorf("dynamicFuncCommands の %q が静的走査で検出されず、理由も dynamicUndetectable にない", name)
		}
	}
	// 例外表に、一覧にも検出にも無い名前が残っていないこと
	for name := range dynamicStateConsumers {
		if !discovered[name] {
			t.Errorf("dynamicStateConsumers の %q がもう検出されていない (例外表の掃除が必要)", name)
		}
		if _, ok := dynamicFuncCommands[name]; ok {
			t.Errorf("dynamicStateConsumers の %q が dynamicFuncCommands にもある (二重所属)", name)
		}
	}
	for name := range dynamicUndetectable {
		if _, ok := dynamicFuncCommands[name]; !ok {
			t.Errorf("dynamicUndetectable の %q が dynamicFuncCommands にない (例外表の掃除が必要)", name)
		}
	}
}

// scanPluginPackage は dir のパッケージ (テストファイルを除く) を走査し、
// 動的と判定された命令名を返す。
func scanPluginPackage(t *testing.T, dir string) *pkgScan {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("パッケージディレクトリが読めない: %s: %v", dir, err)
	}
	scan := &pkgScan{
		funcs:  map[string][]*pkgFunc{},
		regs:   map[string]ast.Expr{},
		dynFn:  map[string]bool{},
		dynCmd: map[string]bool{},
	}
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		collectPkgFile(scan, f)
	}
	resolveDynamicFuncs(scan)
	for cmd, expr := range scan.regs {
		if scan.exprIsDynamic(expr, map[string]bool{}) {
			scan.dynCmd[cmd] = true
		}
	}
	return scan
}

// collectPkgFile は1ファイルから、名前つき関数と命令登録を集める。
func collectPkgFile(scan *pkgScan, f *ast.File) {
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			// メソッドはレシーバを無視して名前で引く (p.execCallback 等)
			scan.funcs[fd.Name.Name] = append(scan.funcs[fd.Name.Name], &pkgFunc{body: fd.Body})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// m["命令名"] = <式> の形 (stdlib / nodelib)
			if len(node.Lhs) == 1 && len(node.Rhs) == 1 {
				if name, ok := indexStringKey(node.Lhs[0]); ok {
					scan.regs[name] = node.Rhs[0]
				}
				// 名前つき関数変数 (ajaxSendFn := func... など)。
				// 関数でない代入 (fn, ok := toFunc(...) の fn 等) も
				// ここへ入るが、式の解決で関数として解決できるもの
				// だけが効くので害はない
				if id, ok := node.Lhs[0].(*ast.Ident); ok {
					scan.funcs[id.Name] = append(scan.funcs[id.Name], &pkgFunc{expr: node.Rhs[0]})
				}
			}
		case *ast.KeyValueExpr:
			// "命令名": <式> の形 (sqlitelib のマップリテラル)
			if key, ok := node.Key.(*ast.BasicLit); ok && key.Kind == token.STRING {
				if name, err := strconv.Unquote(key.Value); err == nil {
					if _, dup := scan.regs[name]; !dup {
						scan.regs[name] = node.Value
					}
				}
			}
		}
		return true
	})
}

// indexStringKey は m["名前"] の形から名前を取り出す。
func indexStringKey(expr ast.Expr) (string, bool) {
	idx, ok := expr.(*ast.IndexExpr)
	if !ok {
		return "", false
	}
	lit, ok := idx.Index.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return name, true
}

// resolveDynamicFuncs は、sink へ到達する名前つき関数を固定点まで伝播する。
func resolveDynamicFuncs(scan *pkgScan) {
	for changed := true; changed; {
		changed = false
		for name, bodies := range scan.funcs {
			if scan.dynFn[name] {
				continue
			}
			for _, fn := range bodies {
				if fn.body != nil && scan.bodyIsDynamic(fn.body) {
					scan.dynFn[name] = true
					changed = true
					break
				}
				if fn.expr != nil && scan.exprIsDynamic(fn.expr, map[string]bool{}) {
					scan.dynFn[name] = true
					changed = true
					break
				}
			}
		}
	}
}

// bodyIsDynamic は関数本体が sink 呼び出し、または動的な名前つき関数の
// 呼び出しを含むかを返す。
func (scan *pkgScan) bodyIsDynamic(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			if sinkMethods[fun.Sel.Name] {
				found = true
				return false
			}
			// メソッド呼び出し (p.execCallback の呼び出し側など)
			if scan.dynFn[fun.Sel.Name] {
				found = true
				return false
			}
		case *ast.Ident:
			if fun.Name == "toFunc" || scan.dynFn[fun.Name] {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// exprIsDynamic は命令の fn を表す式が、動的な本体へ解決できるかを返す。
// seen は別名解決の循環止め。
func (scan *pkgScan) exprIsDynamic(expr ast.Expr, seen map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.FuncLit:
		return scan.bodyIsDynamic(e.Body)
	case *ast.CompositeLit:
		// command{..., fn: <式>} の形は fn フィールドをたどる
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "fn" {
					return scan.exprIsDynamic(kv.Value, seen)
				}
			}
		}
		return false
	case *ast.Ident:
		if e.Name == "toFunc" || scan.dynFn[e.Name] {
			return true
		}
		for _, fn := range scan.funcs[e.Name] {
			if scan.pkgFuncIsDynamic(e.Name, fn, seen) {
				return true
			}
		}
		return false
	case *ast.SelectorExpr:
		// p.execCallback のようなメソッド値
		if scan.dynFn[e.Sel.Name] {
			return true
		}
		// m["別名"].fn の形は、参照先の命令をたどる
		if name, ok := indexStringKey(e.X); ok && e.Sel.Name == "fn" {
			return scan.commandIsDynamic(name, seen)
		}
		return false
	case *ast.CallExpr:
		// setTimer(false) のような「関数を返す関数」の呼び出し
		if id, ok := e.Fun.(*ast.Ident); ok {
			for _, fn := range scan.funcs[id.Name] {
				if scan.pkgFuncIsDynamic(id.Name, fn, seen) {
					return true
				}
			}
		}
		return false
	case *ast.IndexExpr:
		// m["別名"] の形の別名代入
		if name, ok := indexStringKey(e); ok {
			return scan.commandIsDynamic(name, seen)
		}
		return false
	}
	return false
}

// pkgFuncIsDynamic は名前つき関数1件が動的かを返す。
func (scan *pkgScan) pkgFuncIsDynamic(name string, fn *pkgFunc, seen map[string]bool) bool {
	if scan.dynFn[name] {
		return true
	}
	if seen[name] {
		return false
	}
	seen[name] = true
	if fn.body != nil {
		return scan.bodyIsDynamic(fn.body)
	}
	if fn.expr != nil {
		return scan.exprIsDynamic(fn.expr, seen)
	}
	return false
}

// commandIsDynamic は別名経由で命令名から動的かをたどる。
func (scan *pkgScan) commandIsDynamic(name string, seen map[string]bool) bool {
	if scan.dynCmd[name] {
		return true
	}
	key := "cmd:" + name
	if seen[key] {
		return false
	}
	seen[key] = true
	expr, ok := scan.regs[name]
	if !ok {
		return false
	}
	return scan.exprIsDynamic(expr, seen)
}

// TestDynamicFuncArgPositions は、分類 dynamicFuncDirect の前提である
// 「コールバックは必ず第0引数 (または保存済み状態) から来る」を固定する。
// toFunc / FindFunc / FindValue / CallCommand の各呼び出しについて、
// 対象の値の出所を AST 上で分類し、第0引数・AJAX:ONERROR・
// CommandState の保存値・ヘルパー (toFunc/post) の引数のいずれかで
// あることを確認する。第1引数以降を toFunc する命令が増えたら、
// ここの分類が "other" になって落ちる。
// 限界: ヘルパー内部で param と判定された値については、その引数を
// 渡す呼び出し側 (post の呼び出し元など) を別途ここで検査している
// 範囲でのみ保証する。
func TestDynamicFuncArgPositions(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	internalDir := filepath.Dir(filepath.Dir(file))

	type record struct {
		where string // 呼び出しの種類 (toFunc/FindFunc/FindValue/CallCommand/post/onerror-store/state-store)
		class string // arg0 / onerror / state / param / other
	}
	var records []record

	for _, pkg := range []string{"stdlib", "nodelib", "sqlitelib"} {
		dir := filepath.Join(internalDir, pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			for _, decl := range f.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				params := map[string]bool{}
				if fd.Type.Params != nil {
					for _, field := range fd.Type.Params.List {
						for _, n := range field.Names {
							params[n.Name] = true
						}
					}
				}
				origins := map[string]string{}
				var classOf func(expr ast.Expr) string
				classOf = func(expr ast.Expr) string {
					switch ex := expr.(type) {
					case *ast.Ident:
						if c, ok := origins[ex.Name]; ok {
							return c
						}
						if params[ex.Name] {
							return "param"
						}
						return "other"
					case *ast.CallExpr:
						if id, ok := ex.Fun.(*ast.Ident); ok && (id.Name == "arg" || id.Name == "argAt") {
							if len(ex.Args) == 2 {
								if lit, ok := ex.Args[1].(*ast.BasicLit); ok && lit.Value == "0" {
									return "arg0"
								}
							}
							return "other"
						}
						if sel, ok := ex.Fun.(*ast.SelectorExpr); ok {
							switch sel.Sel.Name {
							case "Get":
								// 辞書/配列の要素取得は、入れ物の出所を引き継ぐ
								// (ハテナのパイプライン配列の要素など)
								return classOf(sel.X)
							case "String":
								// value.String(...) のリテラル生成は定数扱い
								// (ハテナの既定パイプライン名など)
								if id, ok := sel.X.(*ast.Ident); ok && id.Name == "value" {
									return "const"
								}
								return "other"
							case "ToString":
								if len(ex.Args) == 1 {
									return classOf(ex.Args[0])
								}
							case "SysVar":
								if len(ex.Args) == 1 {
									if lit, ok := ex.Args[0].(*ast.BasicLit); ok && lit.Value == `"AJAX:ONERROR"` {
										return "onerror"
									}
								}
							case "CommandState":
								return "state"
							}
						}
						return "other"
					}
					return "other"
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.AssignStmt:
						if len(node.Rhs) == 1 {
							if len(node.Lhs) == 1 {
								if id, ok := node.Lhs[0].(*ast.Ident); ok {
									origins[id.Name] = classOf(node.Rhs[0])
								}
							} else if call, ok := node.Rhs[0].(*ast.CallExpr); ok {
								// names, ok := pipeline.Array() のような
								// 複数代入は、セレクタの対象の出所を引き継ぐ
								if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
									for _, lhs := range node.Lhs {
										if id, ok := lhs.(*ast.Ident); ok {
											origins[id.Name] = classOf(sel.X)
										}
									}
								}
							}
						}
					case *ast.CallExpr:
						switch fun := node.Fun.(type) {
						case *ast.Ident:
							if fun.Name == "toFunc" && len(node.Args) == 2 {
								records = append(records, record{"toFunc", classOf(node.Args[1])})
							}
							if fun.Name == "post" && len(node.Args) >= 2 {
								records = append(records, record{"post", classOf(node.Args[1])})
							}
						case *ast.SelectorExpr:
							switch fun.Sel.Name {
							case "FindFunc", "FindValue":
								if len(node.Args) == 1 {
									records = append(records, record{fun.Sel.Name, classOf(node.Args[0])})
								}
							case "CallCommand":
								if len(node.Args) == 2 {
									records = append(records, record{"CallCommand", classOf(node.Args[0])})
								}
							case "SetSysVar":
								if len(node.Args) == 2 {
									if lit, ok := node.Args[0].(*ast.BasicLit); ok && lit.Value == `"AJAX:ONERROR"` {
										records = append(records, record{"onerror-store", classOf(node.Args[1])})
									}
								}
							case "SetCommandState":
								if len(node.Args) == 2 {
									if lit, ok := node.Args[0].(*ast.BasicLit); ok && lit.Value == `"__fileProcessCallback"` {
										records = append(records, record{"state-store", classOf(node.Args[1])})
									}
								}
							}
						}
					}
					return true
				})
			}
		}
	}

	if len(records) == 0 {
		t.Fatal("引数位置の記録が1件も無い (走査が空振りしている)")
	}
	allowed := map[string]map[string]bool{
		// toFunc の対象は、第0引数か、保存済み状態 (ONERROR/ファイル処理) のみ
		"toFunc": {"arg0": true, "onerror": true, "state": true},
		// FindFunc は第0引数由来か、ヘルパー (toFunc/post) の引数経由のみ
		"FindFunc": {"arg0": true, "param": true},
		// FindValue は『JSオブジェクト取得』の第0引数由来のみ
		"FindValue": {"arg0": true},
		// CallCommand の対象名は CommandState のパイプライン (ハテナ)
		// か、その既定値の定数のみ
		"CallCommand": {"state": true, "const": true},
		// post ヘルパーへ渡るのは呼び出し側命令の第0引数のみ
		"post": {"arg0": true},
		// 保存側2件は、どちらも第0引数を保存する
		"onerror-store": {"arg0": true},
		"state-store":   {"arg0": true},
	}
	seen := map[string]int{}
	for _, r := range records {
		seen[r.where]++
		if !allowed[r.where][r.class] {
			t.Errorf("%s の対象の出所が %q (想定外: コールバックの引数位置が変わった可能性)", r.where, r.class)
		}
	}
	for _, must := range []string{"toFunc", "FindFunc", "FindValue", "CallCommand", "post", "onerror-store", "state-store"} {
		if seen[must] == 0 {
			t.Errorf("%s の呼び出し/保存が検出できていない (走査側の劣化の可能性)", must)
		}
	}
}
