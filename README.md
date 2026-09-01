# nadesiko3go

日本語プログラミング言語「なでしこ3」のGo言語実装です。
現行のTypeScript版を置き換えるものではなく、インストール不要で配布しやすい
CUI/GUIバックエンドを目標にしています。

現在は段階0です。値モデル、rune基準の文字列ヘルパ、直列化可能IR、Host API、
差分fixture runnerの骨格を用意しています。言語処理系はまだ未実装です。

## 開発

```bash
# 本家の差分fixtureをGo側へ同期
make sync-compat

# テスト
make test

# 全ケースを未実装結果として出力
go run ./cmd/gonako compat run --cases ./testdata/compat/cases --out ./out

# 本家のoracleと照合（段階0では不一致が正常、未実行が0件ならrunnerは正常）
(cd nadesiko3 && npm run compat:check -- ../out)
```

詳細な設計と開発上の制約は [AGENTS.md](./AGENTS.md) を参照してください。
