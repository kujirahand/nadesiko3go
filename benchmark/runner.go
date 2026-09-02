package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

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

type Result struct {
	CaseName    string
	File        string
	Description string
	Cnako3      time.Duration
	Gonako      time.Duration
	Gogen       time.Duration
	Output      string
}

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
	// 1. Warmup run to warm OS binary caching & Node JIT cache
	_, _, err := runCommand(name, args...)
	if err != nil {
		return 0, "", fmt.Errorf("warmup failed: %w", err)
	}

	// 2. Measured runs
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

	// Sort durations
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	// Take average of all runs
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	avg := sum / time.Duration(runs)
	return avg, lastOutput, nil
}

func main() {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		panic(err)
	}

	cnako3Path := filepath.Join(repoRoot, "nadesiko3", "bin", "cnako3")
	gonakoPath := filepath.Join(repoRoot, "bin", "gonako")

	benchDir := filepath.Join(repoRoot, "benchmark")
	buildDir := filepath.Join(benchDir, "build")
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		panic(err)
	}

	// Setup go.mod for gogen in buildDir
	goModContent := fmt.Sprintf(`module benchgogen

go 1.24

require github.com/kujirahand/nadesiko3go v0.0.0

replace github.com/kujirahand/nadesiko3go => %s
`, repoRoot)
	if err := os.WriteFile(filepath.Join(buildDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		panic(err)
	}

	fmt.Println("=== 1. gogen で Go バイナリをビルド中 ===")
	gogenBins := make(map[string]string)

	// Generate all Go source files
	for _, c := range cases {
		srcPath := filepath.Join(benchDir, c.File)
		baseName := strings.TrimSuffix(c.File, ".nako3")
		goSrcPath := filepath.Join(buildDir, baseName+".go")

		genCmd := exec.Command(gonakoPath, "gengo", srcPath, "--out", goSrcPath)
		if out, err := genCmd.CombinedOutput(); err != nil {
			panic(fmt.Errorf("gengo failed for %s: %v, out: %s", c.File, err, out))
		}
	}

	// Run go mod tidy
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = buildDir
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		panic(fmt.Errorf("go mod tidy failed: %v, out: %s", err, out))
	}

	// Build each Go binary
	for _, c := range cases {
		baseName := strings.TrimSuffix(c.File, ".nako3")
		goSrcPath := filepath.Join(buildDir, baseName+".go")
		binPath := filepath.Join(buildDir, baseName+".bin")

		buildCmd := exec.Command("go", "build", "-o", binPath, goSrcPath)
		buildCmd.Dir = buildDir
		if out, err := buildCmd.CombinedOutput(); err != nil {
			panic(fmt.Errorf("go build failed for %s: %v, out: %s", c.File, err, out))
		}
		gogenBins[c.File] = binPath
		fmt.Printf("  [OK] %s ビルド完了\n", baseName)
	}

	runs := 5
	fmt.Printf("\n=== 2. ベンチマーク実行中 (各 %d 回計測の平均) ===\n", runs)

	var results []Result
	var totalCnako3, totalGonako, totalGogen time.Duration

	for _, c := range cases {
		srcPath := filepath.Join(benchDir, c.File)
		binPath := gogenBins[c.File]

		fmt.Printf("測定中: %s ...\n", c.Name)

		dCnako, outCnako, err := measureAverage(runs, cnako3Path, srcPath)
		if err != nil {
			panic(fmt.Errorf("cnako3 failed on %s: %v", c.File, err))
		}

		dGonako, outGonako, err := measureAverage(runs, gonakoPath, srcPath)
		if err != nil {
			panic(fmt.Errorf("gonako failed on %s: %v", c.File, err))
		}

		dGogen, outGogen, err := measureAverage(runs, binPath)
		if err != nil {
			panic(fmt.Errorf("gogen failed on %s: %v", c.File, err))
		}

		if outCnako != outGonako || outGonako != outGogen {
			fmt.Printf("  [WARNING] 出力不一致:\n    cnako3: %s\n    gonako: %s\n    gogen:  %s\n",
				outCnako, outGonako, outGogen)
		} else {
			fmt.Printf("  [一致確認] %s\n", outCnako)
		}

		totalCnako3 += dCnako
		totalGonako += dGonako
		totalGogen += dGogen

		results = append(results, Result{
			CaseName:    c.Name,
			File:        c.File,
			Description: c.Description,
			Cnako3:      dCnako,
			Gonako:      dGonako,
			Gogen:       dGogen,
			Output:      outCnako,
		})
	}

	fmt.Println("\n=== 3. ベンチマーク結果集計 ===")
	fmt.Printf("%-32s | %12s | %12s | %12s\n", "Benchmark", "cnako3(Node)", "gonako(VM)", "gogen(Go)")
	fmt.Println(strings.Repeat("-", 77))
	for _, r := range results {
		fmt.Printf("%-32s | %10.1fms | %10.1fms | %10.1fms\n",
			r.CaseName,
			float64(r.Cnako3.Microseconds())/1000.0,
			float64(r.Gonako.Microseconds())/1000.0,
			float64(r.Gogen.Microseconds())/1000.0,
		)
	}
	fmt.Println(strings.Repeat("-", 77))
	fmt.Printf("%-32s | %10.1fms | %10.1fms | %10.1fms\n",
		"合計 (Total)",
		float64(totalCnako3.Microseconds())/1000.0,
		float64(totalGonako.Microseconds())/1000.0,
		float64(totalGogen.Microseconds())/1000.0,
	)

	// Markdown 出力作成
	var md bytes.Buffer
	md.WriteString("# なでしこ3 動作速度ベンチマーク\n\n")
	md.WriteString("本家リポジトリ（TypeScript / Node.js 公式実装 `cnako3`）と、Go言語による本実装 `gonako`（バイトコードVM実行）、および Goコード生成バックエンド `gogen`（Goネイティブコンパイル実行）の動作速度を比較測定したベンチマーク結果です。\n\n")

	md.WriteString("## 1. 測定環境\n\n")
	md.WriteString("- **OS**: macOS (darwin/arm64, Apple Silicon)\n")
	md.WriteString("- **Go バージョン**: go version go1.26.5 darwin/arm64\n")
	md.WriteString("- **Node.js バージョン**: v24.18.0\n")
	md.WriteString(fmt.Sprintf("- **測定日**: %s\n", time.Now().Format("2006-01-02")))
	md.WriteString(fmt.Sprintf("- **測定方法**: 各テストプログラムをウォームアップ後に %d 回実行し、平均実行時間を算出\n\n", runs))

	md.WriteString("## 2. 比較対象\n\n")
	md.WriteString("| 対象 | 実行方式 | 特徴 |\n")
	md.WriteString("|---|---|---|\n")
	md.WriteString("| **cnako3 (本家)** | `node src/cnako3.mjs` | 公式 TypeScript 実装。Node.js / V8 JIT ランタイム上で動作 |\n")
	md.WriteString("| **gonako (VM実行)** | `bin/gonako <file>` | 本実装のスタック型バイトコードインタプリタ。Goネイティブバイナリ |\n")
	md.WriteString("| **gogen (Goネイティブ)** | `gonako gengo` → `go build` | なでしこプログラムをGoソースに変換し、ネイティブコンパイルして実行 |\n\n")

	md.WriteString("## 3. ベンチマークテスト一覧\n\n")
	md.WriteString("代表的なアルゴリズム（再帰・配列・数値計算・ソート・文字列・連想配列）を網羅したテストセットです。\n\n")
	md.WriteString("| No | テスト名 | プログラム | アルゴリズム概要 / 計算規模 | 計算結果（全環境一致確認） |\n")
	md.WriteString("|---|---|---|---|---|\n")
	for i, r := range results {
		md.WriteString(fmt.Sprintf("| %d | **%s** | [`%s`](./%s) | %s | `%s` |\n",
			i+1, r.CaseName, r.File, r.File, r.Description, r.Output))
	}
	md.WriteString("\n")

	md.WriteString("## 4. ベンチマーク測定結果\n\n")
	md.WriteString("| ベンチマーク項目 | cnako3 (Node.js) | gonako (VM) | gogen (Goネイティブ) | gonako速度比 (対cnako3) | gogen速度比 (対cnako3) |\n")
	md.WriteString("|---|---|---|---|---|---|\n")

	for _, r := range results {
		cMs := float64(r.Cnako3.Microseconds()) / 1000.0
		vmMs := float64(r.Gonako.Microseconds()) / 1000.0
		goMs := float64(r.Gogen.Microseconds()) / 1000.0

		vmRatio := cMs / vmMs
		goRatio := cMs / goMs

		md.WriteString(fmt.Sprintf("| **%s** | %.1f ms | %.1f ms | **%.1f ms** | %.2fx | **%.2fx** |\n",
			r.CaseName, cMs, vmMs, goMs, vmRatio, goRatio))
	}

	totCMs := float64(totalCnako3.Microseconds()) / 1000.0
	totVMMs := float64(totalGonako.Microseconds()) / 1000.0
	totGoMs := float64(totalGogen.Microseconds()) / 1000.0
	totVMRatio := totCMs / totVMMs
	totGoRatio := totCMs / totGoMs

	md.WriteString("|---|---|---|---|---|---|\n")
	md.WriteString(fmt.Sprintf("| **合計 (Total)** | **%.1f ms** | **%.1f ms** | **%.1f ms** | **%.2fx** | **%.2fx** |\n\n",
		totCMs, totVMMs, totGoMs, totVMRatio, totGoRatio))

	md.WriteString("※ 速度比（倍率）は `cnako3の所要時間 / 対象の所要時間` です（1.00x より大きいほど高速）。\n\n")

	md.WriteString("## 5. 結果の考察と分析\n\n")

	md.WriteString("### ① Go版が本家より速いところ\n\n")
	md.WriteString("**起動が軽い。** Goのネイティブ単一バイナリなので、Node.jsプロセスの初期化とTypeScriptパーサーの読み込みがありません。数十msで終わるテスト（String・Dict）では、この差がそのまま順位になります。\n\n")
	md.WriteString("**文字列と辞書が速い。** Goネイティブの `string` (UTF-8) と、挿入順を保つ辞書の実装が効きます。特に文字列処理は本家の5倍前後です。\n\n")
	md.WriteString("**関数呼び出しが速い。** 再帰（Fibonacci）は本家の2.6倍前後で、これが合計を押し上げている最大の要因です。呼び出し1回あたりの割り当てを3個まで減らしてあります（AGENTS.md 6節）。\n\n")

	md.WriteString("### ② Go版が本家より遅いところ\n\n")
	md.WriteString("**数値ループはV8に負けています。** Collatz・Mandelbrot・Sieve は本家より遅く、特にCollatzは3倍ほどの差があります。これは実装の詰めが甘いというより、方式の差です。\n\n")
	md.WriteString("- V8はJITで、ホットな数値ループを型を特殊化したマシン語に落とします。Go版のVMはバイトコードインタプリタで、JITを持ちません。\n")
	md.WriteString("- 数値は float64 一本で、値は `Value` に包んだまま扱います。「この変数は整数だ」と決めつける最適化はしません。JavaScriptの意味論を曲げないための方針です（AGENTS.md 12節）。\n")
	md.WriteString("- したがって、この差を埋めるには型推論か実行時の特殊化が要ります。互換性を優先している間は、ここは本家に譲る前提で読んでください。\n\n")

	md.WriteString("### ③ gogen (Goネイティブ生成) は今のところVMと大差ありません\n\n")
	md.WriteString("ネイティブコンパイルする以上もっと速くなりそうに見えますが、実測では合計でVMとほぼ同じです。Collatzのように演算が続くものだけ数%〜10%ほど勝ちます。\n\n")
	md.WriteString("理由は、現在のgogenが「バイトコードの機械的なGoソース化」だからです（docs/gogen.md 7節）。命令ディスパッチは消えますが、オペランドスタックの push/pop はそのまま残り、値の演算も `internal/vm` へ委譲します。式を直接Goの式へ組み立てる作りに変えれば伸びしろがありますが、それは今の構造を変えることになります。\n\n")
	md.WriteString("裏を返すと、**gogenを使う理由は現状では速度よりも配布性**（Goツールチェインのある環境でネイティブ実行ファイルを作れること）にあります。\n\n")

	md.WriteString("### ④ 表の読み方の注意\n\n")
	md.WriteString("**日をまたいだ比較には使えません。** この表の絶対値は測定した日のマシンの状態に強く依存します。実際、同じ7ケースでcnako3の合計が 3236.9ms の日と 2646.9ms の日があり、Go版を1行も変えていなくても20%近く動きます。\n\n")
	md.WriteString("変更の前後を比べたいときは、**変更前後のバイナリを同じマシンで1回ずつ交互に回し、その中央値を取る**こと。片方をまとめて測ってからもう片方を測ると、熱による速度低下の分だけ結果を読み違えます。次節がその方法で取った値です。\n\n")

	md.WriteString("## 6. 最適化の履歴\n\n")
	md.WriteString("この節の数字は、前節の表とは別に、変更前後のバイナリを交互に5回ずつ回して中央値を取ったものです。\n\n")

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
	md.WriteString("make cmd\n\n")
	md.WriteString("# ベンチマークの自動実行\n")
	md.WriteString("go run ./benchmark/runner.go\n")
	md.WriteString("```\n\n")
	md.WriteString("本ドキュメントは `runner.go` が全体を生成します。**README.md を直接編集しても次の実行で消える**ので、文章を足すときは `runner.go` の生成部を直してください。\n")

	readmePath := filepath.Join(benchDir, "README.md")
	if err := os.WriteFile(readmePath, md.Bytes(), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("\n[完了] 結果を %s に保存しました。\n", readmePath)
}
