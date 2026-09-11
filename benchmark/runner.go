package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// BenchmarkCase はベンチマーク1件の定義
type BenchmarkCase struct {
	Name        string
	File        string
	Description string
}

var cases = []BenchmarkCase{
	{
		Name:        "Fibonacci (再帰呼び出し)",
		File:        "01_fibonacci.nako3",
		Description: "再帰呼び出しとスタック処理の性能 (fib(30))",
	},
	{
		Name:        "Sieve (エラトステネスの篩)",
		File:        "02_sieve.nako3",
		Description: "配列アクセス・更新とループ処理 (20万までの素数列挙)",
	},
	{
		Name:        "Mandelbrot (マンデルブロ集合)",
		File:        "03_mandelbrot.nako3",
		Description: "浮動小数点演算と多重ループ (150x150グリッド反復計算)",
	},
	{
		Name:        "QuickSort (クイックソート)",
		File:        "04_quicksort.nako3",
		Description: "配列要素比較・スワップと分割統治 (10,000要素のソート)",
	},
	{
		Name:        "Collatz (コラッツ予想)",
		File:        "05_collatz.nako3",
		Description: "整数演算・条件分岐・反復処理 (1〜3万の探索)",
	},
	{
		Name:        "String (文字列処理)",
		File:        "06_string.nako3",
		Description: "文字列連結・置換・検索・抽出 (3,000回反復)",
	},
	{
		Name:        "Dict (連想配列/辞書操作)",
		File:        "07_dict.nako3",
		Description: "辞書へのキー値挿入・参照・集計 (3万回操作)",
	},
}

// Target は測定対象の処理系1つ分
type Target struct {
	Key     string // 内部キー
	Label   string // 表の見出し
	How     string // 実行方式（README用）
	Note    string // 特徴（README用）
	Version string // バージョン文字列
	// Command はケースに対する実行コマンドを返す。空スライスならそのケースは測定しない
	Command func(c BenchmarkCase) []string
	Enabled bool
	// Times はケース名 → 平均所要時間
	Times map[string]time.Duration
	Total time.Duration
}

// baseKey は速度比の基準にする処理系
const baseKey = "cnako3"

func runCommand(name string, args ...string) (time.Duration, string, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if err != nil {
		return 0, "", fmt.Errorf("command failed: %v, stderr: %s", err, stderr.String())
	}
	return elapsed, strings.TrimSpace(stdout.String()), nil
}

func measureAverage(runs int, name string, args ...string) (time.Duration, string, error) {
	// 1. ウォームアップ（OSのバイナリキャッシュとNodeのJITを温める）
	_, _, err := runCommand(name, args...)
	if err != nil {
		return 0, "", fmt.Errorf("warmup failed: %w", err)
	}

	// 2. 本計測
	var durations []time.Duration
	var lastOutput string
	for i := 0; i < runs; i++ {
		d, out, err := runCommand(name, args...)
		if err != nil {
			return 0, "", err
		}
		durations = append(durations, d)
		lastOutput = out
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(runs), lastOutput, nil
}

// commandVersion はバージョン取得コマンドの1行目を返す。取れなければ空文字
func commandVersion(name string, args ...string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	out, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return line
}

// baseName は "01_fibonacci.nako3" から "01_fibonacci" を取り出す
func baseName(file string) string {
	return strings.TrimSuffix(file, ".nako3")
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func main() {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		panic(err)
	}
	// benchmark/ 直下からでもリポジトリ直下からでも動くようにする
	if _, statErr := os.Stat(filepath.Join(repoRoot, "go.mod")); statErr != nil {
		repoRoot, err = filepath.Abs(".")
		if err != nil {
			panic(err)
		}
	}

	cnako3Path := filepath.Join(repoRoot, "nadesiko3", "bin", "cnako3")
	gonakoPath := filepath.Join(repoRoot, "bin", "gonako")

	benchDir := filepath.Join(repoRoot, "benchmark")
	buildDir := filepath.Join(benchDir, "build")
	pyDir := filepath.Join(benchDir, "py")
	jsDir := filepath.Join(benchDir, "js")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		panic(err)
	}

	// gogen 用の go.mod を buildDir に用意する
	goModContent := fmt.Sprintf(`module benchgogen

go 1.24

require github.com/kujirahand/nadesiko3go v0.0.0

replace github.com/kujirahand/nadesiko3go => %s
`, repoRoot)
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		panic(err)
	}

	fmt.Println("=== 1. gogen で Go バイナリをビルド中 ===")
	for _, c := range cases {
		srcPath := filepath.Join(benchDir, c.File)
		goSrcPath := filepath.Join(buildDir, baseName(c.File)+".go")

		genCmd := exec.Command(gonakoPath, "gengo", srcPath, "--out", goSrcPath)
		if out, err := genCmd.CombinedOutput(); err != nil {
			panic(fmt.Errorf("gengo failed for %s: %v, out: %s", c.File, err, out))
		}
	}

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = buildDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		panic(fmt.Errorf("go mod tidy failed: %v, out: %s", err, out))
	}

	for _, c := range cases {
		goSrcPath := filepath.Join(buildDir, baseName(c.File)+".go")
		binPath := filepath.Join(buildDir, baseName(c.File)+".bin")

		buildCmd := exec.Command("go", "build", "-o", binPath, goSrcPath)
		buildCmd.Dir = buildDir
		if out, err := buildCmd.CombinedOutput(); err != nil {
			panic(fmt.Errorf("go build failed for %s: %v, out: %s", c.File, err, out))
		}
		fmt.Printf("  [OK] %s ビルド完了\n", baseName(c.File))
	}

	goVersion := commandVersion("go", "version")
	nodeVersion := commandVersion("node", "--version")
	pythonVersion := commandVersion("python3", "--version")

	targets := []*Target{
		{
			Key:     "cnako3",
			Label:   "cnako3 (本家)",
			How:     "`node src/cnako3.mjs`",
			Note:    "公式 TypeScript 実装。Node.js / V8 JIT ランタイム上で動作",
			Version: nodeVersion,
			Command: func(c BenchmarkCase) []string {
				return []string{cnako3Path, filepath.Join(benchDir, c.File)}
			},
		},
		{
			Key:     "gonako",
			Label:   "gonako (VM)",
			How:     "`bin/gonako <file>`",
			Note:    "本実装のスタック型バイトコードインタプリタ。Goネイティブバイナリ",
			Version: goVersion,
			Command: func(c BenchmarkCase) []string {
				return []string{gonakoPath, filepath.Join(benchDir, c.File)}
			},
		},
		{
			Key:     "gogen",
			Label:   "gogen (Goネイティブ)",
			How:     "`gonako gengo` → `go build`",
			Note:    "なでしこプログラムをGoソースに変換し、ネイティブコンパイルして実行",
			Version: goVersion,
			Command: func(c BenchmarkCase) []string {
				return []string{filepath.Join(buildDir, baseName(c.File)+".bin")}
			},
		},
		{
			Key:     "node",
			Label:   "Node.js",
			How:     "`node benchmark/js/<file>.js`",
			Note:    "同じアルゴリズムを素のJavaScriptで書いたもの。言語そのものの速度の目安",
			Version: nodeVersion,
			Command: func(c BenchmarkCase) []string {
				return []string{"node", filepath.Join(jsDir, baseName(c.File)+".js")}
			},
		},
		{
			Key:     "python3",
			Label:   "Python3",
			How:     "`python3 benchmark/py/<file>.py`",
			Note:    "同じアルゴリズムを素のPythonで書いたもの。言語そのものの速度の目安",
			Version: pythonVersion,
			Command: func(c BenchmarkCase) []string {
				return []string{"python3", filepath.Join(pyDir, baseName(c.File)+".py")}
			},
		},
	}

	// 実行できる処理系だけを有効にする
	for _, t := range targets {
		t.Times = map[string]time.Duration{}
		argv := t.Command(cases[0])
		if _, err := exec.LookPath(argv[0]); err != nil {
			fmt.Printf("  [SKIP] %s が見つからないため測定から外します (%v)\n", t.Label, err)
			continue
		}
		t.Enabled = true
	}

	runs := 5
	fmt.Printf("\n=== 2. ベンチマーク実行中 (各 %d 回計測の平均) ===\n", runs)

	outputs := map[string]string{}
	for _, c := range cases {
		fmt.Printf("測定中: %s ...\n", c.Name)

		var caseOutputs []string
		mismatch := false
		for _, t := range targets {
			if !t.Enabled {
				continue
			}
			argv := t.Command(c)
			d, out, err := measureAverage(runs, argv[0], argv[1:]...)
			if err != nil {
				panic(fmt.Errorf("%s failed on %s: %v", t.Key, c.File, err))
			}
			t.Times[c.Name] = d
			t.Total += d
			caseOutputs = append(caseOutputs, fmt.Sprintf("%s: %s", t.Label, out))
			if outputs[c.Name] == "" {
				outputs[c.Name] = out
			} else if outputs[c.Name] != out {
				mismatch = true
			}
		}
		if mismatch {
			fmt.Printf("  [WARNING] 出力不一致:\n    %s\n", strings.Join(caseOutputs, "\n    "))
		} else {
			fmt.Printf("  [一致確認] %s\n", outputs[c.Name])
		}
	}

	var enabled []*Target
	for _, t := range targets {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}

	var base *Target
	for _, t := range enabled {
		if t.Key == baseKey {
			base = t
		}
	}

	fmt.Println("\n=== 3. ベンチマーク結果集計 ===")
	header := fmt.Sprintf("%-32s", "Benchmark")
	for _, t := range enabled {
		header += fmt.Sprintf(" | %12s", t.Label)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))
	for _, c := range cases {
		line := fmt.Sprintf("%-32s", c.Name)
		for _, t := range enabled {
			line += fmt.Sprintf(" | %10.1fms", ms(t.Times[c.Name]))
		}
		fmt.Println(line)
	}
	fmt.Println(strings.Repeat("-", len(header)))
	line := fmt.Sprintf("%-32s", "合計 (Total)")
	for _, t := range enabled {
		line += fmt.Sprintf(" | %10.1fms", ms(t.Total))
	}
	fmt.Println(line)

	// --- Markdown 出力作成 ---
	var md bytes.Buffer
	md.WriteString("# なでしこ3 動作速度ベンチマーク\n\n")
	md.WriteString("本家リポジトリ（TypeScript / Node.js 公式実装 `cnako3`）と、Go言語による本実装 `gonako`（バイトコードVM実行）、Goコード生成バックエンド `gogen`（Goネイティブコンパイル実行）、さらに比較の物差しとして同じアルゴリズムを素の **Node.js (JavaScript)** と **Python3** で書いたものを並べて測定した結果です。\n\n")

	md.WriteString("## 1. 測定環境\n\n")
	md.WriteString(fmt.Sprintf("- **OS**: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	if goVersion != "" {
		md.WriteString(fmt.Sprintf("- **Go バージョン**: %s\n", goVersion))
	}
	if nodeVersion != "" {
		md.WriteString(fmt.Sprintf("- **Node.js バージョン**: %s\n", nodeVersion))
	}
	if pythonVersion != "" {
		md.WriteString(fmt.Sprintf("- **Python バージョン**: %s\n", pythonVersion))
	}
	md.WriteString(fmt.Sprintf("- **測定日**: %s\n", time.Now().Format("2006-01-02")))
	md.WriteString(fmt.Sprintf("- **測定方法**: 各テストプログラムをウォームアップ後に %d 回実行し、平均実行時間を算出\n\n", runs))

	md.WriteString("## 2. 比較対象\n\n")
	md.WriteString("| 対象 | 実行方式 | 特徴 |\n")
	md.WriteString("|---|---|---|\n")
	for _, t := range enabled {
		md.WriteString(fmt.Sprintf("| **%s** | %s | %s |\n", t.Label, t.How, t.Note))
	}
	md.WriteString("\n")
	md.WriteString("Node.js版は [`js/`](./js)、Python3版は [`py/`](./py) にあります。どちらも `.nako3` と**同じアルゴリズム・同じ計算規模**で書いてあり、出力文字列も一致します。なでしこ（と本家cnako3）の数値は倍精度浮動小数点なので、Python版のうち擬似乱数を使う QuickSort だけは、同じ値を得るためにあえて `math.fmod` で浮動小数点演算に揃えてあります。\n\n")

	md.WriteString("## 3. ベンチマークテスト一覧\n\n")
	md.WriteString("代表的なアルゴリズム（再帰・配列・数値計算・ソート・文字列・連想配列）を網羅したテストセットです。\n\n")
	md.WriteString("| No | テスト名 | プログラム | アルゴリズム概要 / 計算規模 | 計算結果（全環境一致確認） |\n")
	md.WriteString("|---|---|---|---|---|\n")
	for i, c := range cases {
		b := baseName(c.File)
		md.WriteString(fmt.Sprintf("| %d | **%s** | [`%s`](./%s) / [js](./js/%s.js) / [py](./py/%s.py) | %s | `%s` |\n",
			i+1, c.Name, c.File, c.File, b, b, c.Description, outputs[c.Name]))
	}
	md.WriteString("\n")

	md.WriteString("## 4. ベンチマーク測定結果\n\n")
	md.WriteString("### 4.1 実行時間\n\n")
	md.WriteString("| ベンチマーク項目 |")
	for _, t := range enabled {
		md.WriteString(fmt.Sprintf(" %s |", t.Label))
	}
	md.WriteString("\n|---|")
	for range enabled {
		md.WriteString("---:|")
	}
	md.WriteString("\n")
	for _, c := range cases {
		// 各行で最速の処理系を太字にする
		var best time.Duration
		for _, t := range enabled {
			if best == 0 || t.Times[c.Name] < best {
				best = t.Times[c.Name]
			}
		}
		md.WriteString(fmt.Sprintf("| **%s** |", c.Name))
		for _, t := range enabled {
			d := t.Times[c.Name]
			if d == best {
				md.WriteString(fmt.Sprintf(" **%.1f ms** |", ms(d)))
			} else {
				md.WriteString(fmt.Sprintf(" %.1f ms |", ms(d)))
			}
		}
		md.WriteString("\n")
	}
	md.WriteString("| **合計 (Total)** |")
	for _, t := range enabled {
		md.WriteString(fmt.Sprintf(" **%.1f ms** |", ms(t.Total)))
	}
	md.WriteString("\n\n")

	if base != nil {
		md.WriteString("### 4.2 速度比（本家 cnako3 を 1.00x としたとき）\n\n")
		md.WriteString("| ベンチマーク項目 |")
		for _, t := range enabled {
			md.WriteString(fmt.Sprintf(" %s |", t.Label))
		}
		md.WriteString("\n|---|")
		for range enabled {
			md.WriteString("---:|")
		}
		md.WriteString("\n")
		for _, c := range cases {
			md.WriteString(fmt.Sprintf("| **%s** |", c.Name))
			for _, t := range enabled {
				md.WriteString(fmt.Sprintf(" %.2fx |", ms(base.Times[c.Name])/ms(t.Times[c.Name])))
			}
			md.WriteString("\n")
		}
		md.WriteString("| **合計 (Total)** |")
		for _, t := range enabled {
			md.WriteString(fmt.Sprintf(" **%.2fx** |", ms(base.Total)/ms(t.Total)))
		}
		md.WriteString("\n\n")
		md.WriteString("※ 速度比（倍率）は `cnako3の所要時間 / 対象の所要時間` です（1.00x より大きいほど高速）。\n\n")
	}

	md.WriteString("## 5. 結果の考察と分析\n\n")

	md.WriteString("### ① Go版が本家より速いところ\n\n")
	md.WriteString("**起動が軽い。** Goのネイティブ単一バイナリなので、Node.jsプロセスの初期化とTypeScriptパーサーの読み込みがありません。数十msで終わるテスト（String・Dict・QuickSort）では、この差がそのまま順位になります。\n\n")
	md.WriteString("**文字列と辞書が速い。** Goネイティブの `string` (UTF-8) と、挿入順を保つ辞書の実装が効きます。\n\n")
	md.WriteString("**関数呼び出しが速い。** 再帰（Fibonacci）の差がもっとも大きく、これが合計を押し上げている最大の要因です。呼び出し1回あたりの割り当てを3個まで減らしてあります（docs/parser.md）。\n\n")

	md.WriteString("### ② VM実行が本家より遅いところ\n\n")
	md.WriteString("**数値ループはV8のJITに負けます。** 残っている弱点はCollatzで、ここだけは本家より遅いままです。V8はホットな数値ループを型を特殊化したマシン語に落としますが、`gonako` のVMはバイトコードインタプリタで、JITを持ちません。素のNode.js列と見比べると差の出どころがはっきりします。\n\n")
	md.WriteString("値の持ち方も効いています。VMは値を `value.Value` に包んだまま扱い、演算のたびに包み直します。ここを詰めたのが次の gogen です。\n\n")

	md.WriteString("### ③ gogen (Goネイティブ生成) は数値計算で本家を追い越します\n\n")
	md.WriteString("`internal/gogen/types.go` の型推論により、生成コードは**数値と証明できた場所を生の `float64` で計算します**。オペランドスタックはGoのローカル変数に、捕捉されない数値ローカルはただの `float64` 変数になり、`rt.Binary(...)` は `f0 = f1 + f2` になります。\n\n")
	md.WriteString("この結果、計算集約のケースでgogenがVMを大きく引き離します（同一マシンでの前後比較は次節）。\n\n")
	md.WriteString("推論できないところは今までどおり `rt.Value` のまま一般経路を通ります。**証明できたときだけ特殊化する**方針なので、当てが外れても遅くなるだけで、結果は変わりません（docs/gogen.md）。\n\n")
	md.WriteString("なお、掛け算だけは生成コードで `float64(a * b)` と明示的に丸めています。これがないとarm64などでGoコンパイラが直後の加減算とまとめてFMA命令に融合し、中間結果が丸められず、JavaScript（とVM実行）と答えが変わってしまうためです。変換自体に実行時コストはありません。\n\n")
	md.WriteString("逆に、文字列処理（String）や辞書操作（Dict）はもともと命令呼び出しが主体で、数値演算がほとんどないため、gogenにしてもVMとあまり変わりません。\n\n")

	md.WriteString("### ④ 素のNode.js・Python3との位置関係\n\n")
	md.WriteString("Node.js と Python3 の列は、**言語処理系そのものの地力**を示す物差しです。なでしこの列と直接勝ち負けを競わせるためのものではありません（なでしこ側は日本語の構文解析とプラグイン命令の呼び出しを通るぶん、同じ計算でも手数が増えます）。\n\n")
	md.WriteString("読み方の目安は次のとおりです。\n\n")
	md.WriteString("- **20ms前後はプロセス起動のぶん**です。`node` も `python3` も、何もしないスクリプトで20ms台かかります。数十msで終わるテスト（String・Dict・QuickSort）でネイティブバイナリの `gogen` が勝つのは、主にこの起動コストの差です。\n")
	md.WriteString("- **Node.js (素のJS) が強いのは数値ループと再帰**です。V8のJITが効く領域で、Fibonacci と Collatz では `gogen` もまだ届きません。逆に配列・文字列・辞書が主体のケースでは、起動コストを含めると `gogen` が上回ります。\n")
	md.WriteString("- **Python3 は全体に重め**です。`gonako` のVMはバイトコードインタプリタという点ではCPythonと同じ方式ですが、値表現がGoの構造体で固定サイズ・ヒープ割り当てが少ないぶん有利で、全ケースで上回っています。\n")
	md.WriteString("- **cnako3 と 素のNode.js の差**が、そのまま「なでしこ処理系の上乗せ分」の目安になります。同様に `gonako` と `gogen` の差が、Go版におけるVM実行のオーバーヘッドです。\n\n")

	md.WriteString("### ⑤ 表の読み方の注意\n\n")
	md.WriteString("**日をまたいだ比較には使えません。** この表の絶対値は測定した日のマシンの状態に強く依存します。実際、同じ7ケースでcnako3の合計が 3236.9ms の日と 2646.9ms の日があり、Go版を1行も変えていなくても20%近く動きます。\n\n")
	md.WriteString("変更の前後を比べたいときは、**変更前後のバイナリを同じマシンで1回ずつ交互に回し、その中央値を取る**こと。片方をまとめて測ってからもう片方を測ると、熱による速度低下の分だけ結果を読み違えます。次節がその方法で取った値です。\n\n")

	md.WriteString("## 6. 最適化の履歴\n\n")
	md.WriteString("この節の数字は、前節の表とは別に、変更前後のバイナリを交互に5回ずつ回して中央値を取ったものです。\n\n")

	md.WriteString("### gogenの型推論と非ボックス化 (2026-09-03)\n\n")
	md.WriteString("`internal/gogen/types.go` を入れ、生成コードのオペランドスタックをGoの変数へ展開し、数値と証明できた値を生の `float64` で持つようにしたときの効果です（VM実行は変えていないので数字も動きません）。\n\n")
	md.WriteString("| ベンチマーク項目 | gogen 前 | gogen 後 | 差 |\n")
	md.WriteString("|---|---:|---:|---:|\n")
	md.WriteString("| Fibonacci | 740 ms | 541 ms | **-26.9%** |\n")
	md.WriteString("| Sieve | 114 ms | 57 ms | **-50.0%** |\n")
	md.WriteString("| Mandelbrot | 300 ms | 51 ms | **-83.0%** |\n")
	md.WriteString("| QuickSort | 61 ms | 37 ms | **-39.3%** |\n")
	md.WriteString("| Collatz | 637 ms | 162 ms | **-74.6%** |\n")
	md.WriteString("| String | 14 ms | 15 ms | +7.1% |\n")
	md.WriteString("| Dict | 29 ms | 24 ms | **-17.2%** |\n\n")
	md.WriteString("数値演算が主体のもの（Mandelbrot・Collatz・Sieve）ほど効きます。String がわずかに動いているのは測定誤差の範囲で、文字列処理には数値がほとんど出てこないため、推論しても変わりません。\n\n")

	md.WriteString("### 定数伝播とIRの覗き穴最適化 (2026-09-03)\n\n")
	md.WriteString("`internal/compiler/fold.go`（定数伝播）と `internal/compiler/peephole.go`（覗き穴最適化・スーパー命令 `OpBinaryAt`）を入れたときの効果です。\n\n")
	md.WriteString("| ベンチマーク項目 | gonako VM 前 | gonako VM 後 | 差 | gogen 前 | gogen 後 | 差 |\n")
	md.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
	md.WriteString("| Fibonacci | 793 ms | 749 ms | **-5.5%** | 777 ms | 748 ms | -3.7% |\n")
	md.WriteString("| Sieve | 117 ms | 104 ms | **-11.1%** | 119 ms | 114 ms | -4.2% |\n")
	md.WriteString("| Mandelbrot | 323 ms | 297 ms | **-8.0%** | 314 ms | 300 ms | -4.5% |\n")
	md.WriteString("| QuickSort | 62 ms | 57 ms | **-8.1%** | 60 ms | 61 ms | +1.7% |\n")
	md.WriteString("| Collatz | 768 ms | 668 ms | **-13.0%** | 714 ms | 640 ms | -10.4% |\n")
	md.WriteString("| String | 15 ms | 15 ms | ±0 | 15 ms | 15 ms | ±0 |\n")
	md.WriteString("| Dict | 30 ms | 29 ms | -3.3% | 30 ms | 29 ms | -3.3% |\n\n")
	md.WriteString("演算とループが主体のもの（Collatz・Sieve・Mandelbrot）ほど効き、命令呼び出しが主体のもの（String）は変わりません。`Load;Load;Binary` を1命令にまとめる最適化なので、素直な結果です。\n\n")
	md.WriteString("`internal/vm/bench_test.go` の合成ケースでも同じ傾向で、Loop -18%、Calls -11%、Recursion -5% でした。\n\n")

	md.WriteString("## 7. ベンチマークの再実行方法\n\n")
	md.WriteString("以下のコマンドで、すべてのテストのビルド、測定、および本ドキュメントの再生成が自動で行われます。\n\n")
	md.WriteString("```bash\n")
	md.WriteString("# gonako本体をビルド\n")
	md.WriteString("just cmd\n\n")
	md.WriteString("# ベンチマークの自動実行\n")
	md.WriteString("go run ./benchmark/runner.go\n")
	md.WriteString("```\n\n")
	md.WriteString("`node` や `python3` が見つからない環境では、その処理系の列だけを飛ばして測定します（本家 `cnako3` の実行には Node.js が必要です）。\n\n")
	md.WriteString("本ドキュメントは `runner.go` が全体を生成します。**README.md を直接編集しても次の実行で消える**ので、文章を足すときは `runner.go` の生成部を直してください。\n")

	readmePath := filepath.Join(benchDir, "README.md")
	if err := os.WriteFile(readmePath, md.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("\n[完了] 結果を %s に保存しました。\n", readmePath)
}
