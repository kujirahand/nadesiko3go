package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/commanddoc"
)

// docResult は --json で出力する検索結果。AIが扱いやすいよう、
// 検索語・命令・Web検索結果を1つのオブジェクトにまとめる。
type docResult struct {
	Keywords []string               `json:"keywords"`
	Count    int                    `json:"count"`
	Commands []commanddoc.Command   `json:"commands"`
	Web      []commanddoc.WebResult `json:"web,omitempty"`
	WebError string                 `json:"web_error,omitempty"`
}

// searchDoc は `gonako doc KEYWORD` を処理する。命令一覧(JSON)からの検索を
// 既定とし、--web を付けるとマニュアルサイトの検索結果も加える。
func searchDoc(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("doc", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var commandOnly, web, asJSON bool
	flags.BoolVar(&commandOnly, "command", false, "命令一覧から検索する")
	flags.BoolVar(&commandOnly, "c", false, "命令一覧から検索する (--commandの短縮形)")
	flags.BoolVar(&web, "web", false, "Webのマニュアルも検索する")
	flags.BoolVar(&web, "w", false, "Webのマニュアルも検索する (--webの短縮形)")
	flags.BoolVar(&asJSON, "json", false, "結果をJSONで出力する")
	limit := flags.Int("limit", 20, "表示する件数の上限 (0で全件)")

	keywords, rest := splitKeywords(args, "limit")
	if err := flags.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	keywords = append(keywords, flags.Args()...)
	if len(keywords) == 0 {
		return errors.New("検索するキーワードを指定してください (例: gonako doc 秒待)")
	}

	list, err := commanddoc.Commands()
	if err != nil {
		return err
	}
	found := commanddoc.Search(list, keywords)
	total := len(found)
	if *limit > 0 && len(found) > *limit {
		found = found[:*limit]
	}

	result := docResult{Keywords: keywords, Count: total, Commands: found}
	if result.Commands == nil {
		result.Commands = []commanddoc.Command{}
	}
	// --command は既定の動作なので、--web と一緒に指定されていなければ何もしない。
	if web {
		results, err := commanddoc.SearchWeb(keywords)
		if err != nil {
			// Webは繋がらないこともある。命令の検索結果は出したいので、
			// エラーは知らせるだけにして続ける。
			result.WebError = err.Error()
		}
		result.Web = results
	}

	if asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(result)
	}
	writeDocText(stdout, stderr, result, web, total-len(found))
	return nil
}

// writeDocText は人が読む形式で検索結果を書き出す。
func writeDocText(stdout, stderr io.Writer, result docResult, web bool, omitted int) {
	keyword := strings.Join(result.Keywords, " ")
	if len(result.Commands) == 0 {
		fmt.Fprintf(stdout, "『%s』に一致する命令はありませんでした。\n", keyword)
	} else {
		fmt.Fprintf(stdout, "『%s』に一致する命令: %d件\n\n", keyword, result.Count)
	}
	for _, cmd := range result.Commands {
		fmt.Fprintf(stdout, "■ %s%s\n", cmd.Name, formatOrigin(cmd))
		if cmd.Template != "" && cmd.Template != cmd.Name {
			fmt.Fprintf(stdout, "  書式: %s\n", cmd.Template)
		}
		if cmd.Yomi != "" {
			fmt.Fprintf(stdout, "  読み: %s\n", cmd.Yomi)
		}
		if cmd.Desc != "" {
			fmt.Fprintf(stdout, "  説明: %s\n", cmd.Desc)
		}
		fmt.Fprintf(stdout, "  マニュアル: %s\n", cmd.DocURL)
		if cmd.File != "" {
			fmt.Fprintf(stdout, "  ソース: %s:%d\n", cmd.File, cmd.Line)
		}
		fmt.Fprintln(stdout)
	}
	if omitted > 0 {
		fmt.Fprintf(stdout, "(ほか %d件は省略しました。--limit 0 で全件表示します)\n\n", omitted)
	}

	if !web {
		return
	}
	if result.WebError != "" {
		fmt.Fprintf(stderr, "Web検索に失敗しました: %s\n", result.WebError)
		return
	}
	if len(result.Web) == 0 {
		fmt.Fprintf(stdout, "Webのマニュアルに『%s』の該当ページはありませんでした。\n", keyword)
		return
	}
	fmt.Fprintf(stdout, "Webのマニュアル (%d件):\n", len(result.Web))
	for _, page := range result.Web {
		fmt.Fprintf(stdout, "  %s\n    %s\n", page.Title, page.URL)
	}
}

// formatOrigin は命令の出どころ（プラグイン名・分類）を丸カッコで返す。
func formatOrigin(cmd commanddoc.Command) string {
	var parts []string
	if cmd.Plugin != "" {
		parts = append(parts, cmd.Plugin)
	}
	if cmd.Category != "" {
		parts = append(parts, cmd.Category)
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, " / ") + ")"
}

// splitKeywords はフラグでない引数（＝キーワード）を取り除いて返す。
// flagパッケージは最初の非フラグ引数で解析をやめるので、
// `gonako doc 秒待 --web` のような書き方に対応するために先に分けておく。
func splitKeywords(args []string, valueFlags ...string) (keywords, rest []string) {
	valueExpected := false
	for _, a := range args {
		switch {
		case valueExpected:
			valueExpected = false
		case strings.HasPrefix(a, "-") && a != "-":
			name := strings.TrimLeft(strings.Split(a, "=")[0], "-")
			valueExpected = !strings.Contains(a, "=") && containsString(valueFlags, name)
		default:
			keywords = append(keywords, a)
			continue
		}
		rest = append(rest, a)
	}
	return keywords, rest
}
