# 値モデル

なでしこの値はJavaScriptの値です。ここを最初に決めないと全部が揺れます。

```go
// internal/value
type Kind uint8

const (
    KindUndefined Kind = iota
    KindNull            // なでしこの「空」
    KindBool
    KindNumber          // float64。JSのnumberと同じ
    KindString          // Goネイティブの文字列(UTF-8)
    KindArray
    KindDict            // 挿入順を保持する
    KindFunc
)

type Value struct {
    data unsafe.Pointer // 文字列・配列・辞書・関数で共用。GC追跡対象
    num  float64        // 数値・真偽値
    aux  uintptr        // 文字列のUTF-8バイト長
    kind Kind
}
```

64bit環境では32バイトです。文字列は `data` に `unsafe.StringData` の返す
バッキング配列へのポインタ、`aux` にバイト長を持たせ、`unsafe.String` で
再構築します。`data` を `uintptr` にしてはいけません。`unsafe.Pointer` のまま
保持することで、文字列を含む参照先がGCの追跡対象になります。

決めておくこと。

- **数値は `float64` 一本**。整数型を別に持つとJSとの差が出る。
  `9007199254740993` が `9007199254740992` になるのも含めて互換
- **文字列はGoネイティブ（UTF-8）**。UTF-16互換層は作らない（→ `docs/compat.md`）
- **辞書は挿入順を保持**する。Goの `map` は順序を保証しないので、
  `map[string]*Value` に加えてキーの順序を保つスライスを持つ
- 配列は**疎（穴あき）になりうる**。`A=[1]` に `A[3]=9` を代入したときの
  中間要素は `undefined` であって `null` ではない
- 暗黙の型変換（`0` と `"0"` の比較、`+` が加算か連結か）は
  **JSの規則をそのまま移植**する。ここは自分で考えず、差分fixtureに従う
- **日時に専用の型を作らない。** 現行TS版の `plugin_system_datetime` は
  日時を**文字列**で表す（`今日` → `"YYYY/MM/DD"`、`今` → `"HH:mm:ss"`、
  `日時差` などが扱うのは `"YYYY/MM/DD HH:mm:ss"` 形式）。`システム時間` は
  UNIX秒の数値。したがって `KindDate` は不要で、`stdlib/datetime` は
  **文字列と数値の変換だけ**を行う。SPEC.md の値表現に `{"t":"date"}` があるのは
  `Date` オブジェクトが値として漏れた場合の保険であり、`plugin_system` の範囲では
  出現しない（`expected/*.json` に1件もない）
- **多倍長整数は当面サポートしない**。SPEC.md の `{"t":"bigint"}` も
  現状の期待値には出現しない。必要になった時点で `KindBigInt` を足す

## Host API

VMと外界の境界です。**Goのポインタやmapを直接公開しません。**

```go
// internal/host
type Host interface {
    Print(s string)                       // 『表示』の出力先
    Now() time.Time                       // 日時（テストで固定できるように）
    Env() Env                             // ファイル・OS・プロセス・ネットワーク
    Timer() Timer                         // イベントキューへの登録
}
```

CUI版は `os` / `net` / `io` を、GUI版はWebViewへの橋渡しを、
差分fixture実行時は**出力を集めるだけの実装**を差します。
外部境界では整数handleと明示的なValue APIを使い、GC境界を跨がせません。
