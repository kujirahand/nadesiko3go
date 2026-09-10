package commanddoc

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebResult はマニュアルサイトの検索結果1件。
type WebResult struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// searchURL は kona3 (マニュアルのWikiエンジン) の検索フォームの送信先。
const searchURL = docBaseURL + "index.php?FrontPage&search"

// webTimeout はマニュアルサイトへの問い合わせの制限時間。
const webTimeout = 15 * time.Second

// resultLinkRe は検索結果のリンク（<li><a href="URL">タイトル</a></li>）を拾う。
var resultLinkRe = regexp.MustCompile(`(?s)<li><a href="([^"]+)"[^>]*>(.*?)</a></li>`)

// resultBlockRe は検索フォームより後ろの「結果」の部分だけを切り出す。
// メニューにもリンクがあるので、この範囲に絞らないと拾ってしまう。
var resultBlockRe = regexp.MustCompile(`(?s)<!-- result -->(.*?)</div>\s*</div>`)

// SearchWeb はマニュアルサイト(https://nadesi.com/v3/doc/)をキーワードで検索する。
// キーワードを複数与えた場合はスペースでつないで1つの検索語として送る。
func SearchWeb(keywords []string) ([]WebResult, error) {
	form := url.Values{}
	form.Set("a_mode", "search")
	form.Set("a_key", strings.Join(keywords, " "))

	client := &http.Client{Timeout: webTimeout}
	res, err := client.PostForm(searchURL, form)
	if err != nil {
		return nil, fmt.Errorf("マニュアルサイトへ接続できません: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("マニュアルサイトの応答が異常です: %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("マニュアルサイトの応答を読み込めません: %w", err)
	}
	return parseWebResults(string(body)), nil
}

// parseWebResults は検索結果のHTMLからリンクの一覧を取り出す。
func parseWebResults(body string) []WebResult {
	block := resultBlockRe.FindStringSubmatch(body)
	if len(block) < 2 {
		return nil
	}
	var results []WebResult
	for _, m := range resultLinkRe.FindAllStringSubmatch(block[1], -1) {
		link := html.UnescapeString(m[1])
		title := strings.TrimSpace(html.UnescapeString(stripTags(m[2])))
		if link == "" || title == "" {
			continue
		}
		results = append(results, WebResult{Title: title, URL: link})
	}
	return results
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

func stripTags(s string) string {
	return tagRe.ReplaceAllString(s, "")
}
