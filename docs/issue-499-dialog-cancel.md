# issue #499: ダイアログ打ち切りの仕様明文化と unload フックの一度化

[issue #499](https://github.com/kujirahand/nadesiko3go/issues/499) は、PR #494（ダイアログ打ち切り）のレビューで残った小項目である。#494 のマージは妨げない。コメントは付いていない。本文の4項目を、現行コードに照らして整理する。

関連: #494（打ち切りの実装）、#492（`lockExecWithin`。マージ後に `gui_session.go` で衝突し、メンテナが `cancelPendingDialogs` → `lockExecWithin` の順で併存させた）、#488（macOS 実機確認の側）、#183（外部 URL へ特権 Bind を出さない）。

## 背景

gonako-gui の HTML ダイアログ（`言う` / `尋ねる` / `二択` など）は、WebView の UI スレッドを塞がないようプログラムを非同期で動かし、応答が来るまで待つ。待ちのあいだ実行はロックを握ったままになる。

- Go 側 VM の実行（`guiSession`）は `execMu`
- wnako3 の命令ブリッジ（`commandBridge`）は `b.mu`

応答が来なくなると、次の実行や次の命令呼び出しがそのロックに阻まれる。ページの再読み込み・移動、ウィンドウ終了、新しい実行の開始で、待ちをエラーとして打ち切るのが #494 の中核である。

レビュー（Terra / Sonnet / 実コード検証）の結論は次のとおり。

- bridge と guiSession の中核に High / Medium / Low の指摘は残っていない
- JS の配線に、`beforeunload` と `pagehide` の二重発火と、将来の「ページに留まる確認」による誤打ち切りの理論的可能性があった
- 実コード検証では、留まる確認を出す `beforeunload` ハンドラは無く、誤打ち切りは現行コードでは成立しない。二重発火の2回目は待機 0 件で、実害は未確認
- それ以外は仕様を文章にすること

#494 は #492 との衝突を解消したあとのツリーで、`go test ./cmd/gonako-gui` とダイアログ関連の `-race` が合格している、と issue 本文にある。

## 現状分析

### 打ち切りの出入り口

| きっかけ | guiSession | commandBridge |
|---|---|---|
| 新しい実行の開始 | `startWithWindow` / `startCompiled` / `runWithWindowInDir` / `runCompiled` が予約や `lockExecWithin` の前に `cancelPendingDialogs` | 新しい実行では呼ばない（ブリッジは命令単位） |
| ページの離脱 | JS が `cancelNakoDialogs` | JS が `cancelGonakoDialogs` |
| ウィンドウ終了 | `main.go` / `bundled.go` / `window_run.go` / `wnako3_run.go` | 同じ終了経路 |

JS のフックは2か所にある。

- `cmd/gonako-gui/ui/gonako-loader.js` … エディタ・`-dir`・wnako3 画面へ注入するローダー
- `cmd/gonako-gui/ui/bundled/app.js` … 梱包アプリの実行画面

どちらも `beforeunload` と `pagehide` の両方で `cancelNakoDialogs` と `cancelGonakoDialogs` を呼ぶ。Bind が無いときは何もしない。`cancelNakoDialogs` は特権ブリッジなので、外部 URL（`-url`）には公開されない（#183）。`installWNako3` の `cancelGonakoDialogs` も `targetURL == ""` のときだけ登録される。

エディタの `Cmd+R` / `Ctrl+R` は `ui/app.js` で `preventDefault` したうえで `runCode()` である。再読み込みではない。

### 世代番号（置き換え）

`cancelPendingDialogs` は待ちを走査するだけではない。`dialogEpoch` を1進める。

実行（またはブリッジの `call`）は開始時の世代を覚える。ダイアログを登録する直前に現在の世代と比べ、違えば待ちに入らず `errDialogCanceled`（「ダイアログの待機が中断されました」）を返す。

したがって新しい実行の効果は2段になる。

1. いま応答待ちのダイアログを即時に打ち切る
2. 実行中の run がこの後に出そうとするダイアログも、世代不一致で打ち切る

予約（`reserveRun`）は打ち切りの後なので、新しい実行自身の世代は進んだ後の値になる。新しい実行が後から出すダイアログは、別の打ち切りが来るまで有効である。

この「走査の直後に登録されるダイアログ」を取りこぼさないことが、世代を進める理由である。待機 0 件の呼び出しでも世代は進む。進めるのをやめると、走査と登録の競合でダイアログが取り残され、`execMu` / `b.mu` を握ったまま戻らなくなる。

2回目の呼び出しは、既に打ち切ったダイアログをもう一度エラーにはしない（登録が空なら `cancelDialog` は false、ブリッジ側は map から消えている）。戻り値は 0 になりうる。世代だけはもう一度進む。Go 側はパニックしないが、呼び出し回数について冪等ではない。

利用者のキャンセル（`accepted=false` かつ `err=nil`、空文字や偽）と、打ち切り（必ず非 nil のエラーで VM を止める）は別である。

### 30分タイムアウト

放置されたダイアログはエラー終了する。上限は変数で、テストが短縮できる。

- `guiDialogTimeout`（`gui_session.go`）… Go 側 VM。既定 30 分
- `bridgeDialogTimeout`（`bridge.go`）… 命令ブリッジ。既定 30 分

タイムアウトと応答・打ち切りが同時でも、登録を外した側が一つに決まる。タイムアウト文面は「ダイアログの応答がありません（…以内に応答がなく、ページの移動などで打ち切られた可能性があります）」。

`guiDialogTimeout` は実装と `TestDialogTimeoutReturnsError` がある。利用者向けの `docs/gonako-gui.md` には、ブリッジ側の 30 分（`docs/wnako3-bridge.md`）と比べて書かれていなかった。

### `registerDialog` の `session == nil`

`reserveRun` は本番の非同期実行に必ず `session` と予約時の `dialogEpoch` を入れる。`session == nil` の分岐は、テストが `guiAsyncRun` を単体で作り、世代チェック無しで打ち切りと応答の競合を見るためのものである。本番経路はこの分岐に頼らない。

### 実害の有無（issue 本文の検証と一致）

- 留まる確認を出す `beforeunload` は、ローダーにも梱包画面にもエディタの `app.js` にも無い。`Cmd+R` の `preventDefault` はキー入力の実行ショートカットであり、離脱確認ではない
- 二重発火の2回目は、1回目が待ちを空にしたあとなので打ち切り件数は 0。ページが本当に死ぬ離脱では、世代がもう一度進んでも新しいダイアログは登録されない。実害はコード上も説明がつく
- 世代が進むこと自体は、無待機の2回目でも起きる。JS が同じ離脱で2回呼ぶと、その間に登録されたダイアログまで打ち切る。離脱中は実害になりにくいが、呼び出しは1回にまとめる方が世代の意味が読みやすい

## 課題

issue の4項目は次の課題に対応する。

1. **unload の二重発火**。`beforeunload` と `pagehide` が同じ離脱で両方走る。Go を「何回呼んでも同じ」とコメントするだけでは、世代が進む事実とずれる
2. **留まる確認を将来足したときの誤打ち切り**。`beforeunload` はブラウザが「留まりますか」を出す前に走る。利用者が留まるを選んでも、フックは既にダイアログを打ち切っている。現行コードではこのハンドラが無いので未成立。条件をコード上に残さないと、後から足した人が踏む
3. **置き換えと 30 分タイムアウトが文章になっていない**。挙動は実装とテストにある。読んだ人が「いま出ているダイアログだけ打ち切る」「guiSession はタイムアウトしない」と読めてしまう
4. **`session == nil` が本番のフォールバックに見える**。テスト専用であることをコメントしていない
5. **macOS 実機の再実行・再読み込み**。自動テストは世代とタイムアウトと、JS にフックの文字列があることまで。WebView が実際にどの順で `beforeunload` / `pagehide` を出すかは未確認。エディタの `Cmd+R` は実行なので、再読み込みの確認手順に使えない

## 解決方針の選択肢と比較

### A. JS で離脱1回につき1回だけ呼ぶ（採用）

`unloadDialogsCanceled` を立て、2回目は Go を呼ばない。世代が同じ離脱で2回進まない。Go のロックとエラーの契約は変えない。

### B. Go が冪等だとコメントするだけ（不採用）

2回目がパニックしないのは本当である。世代は進むので「冪等」とは書けない。コメントだけにすると、二重発火が残る。

### C. 待機 0 件のときは世代を進めない（不採用）

無待機の再呼び出しで世代が変わらなくなる。走査が空振りした直後に登録されるダイアログを、世代で落とせなくなる。#494 が塞いだデッドロックが戻る。

### D. 打ち切りを `pagehide` だけにする（見送り）

`pagehide` は離脱が確定してから発火するので、留まる確認との相性がよい。一方、WebView（WKWebView / WebView2 / WebKitGTK）が実際の閉じる・再読み込みで `pagehide` を必ず出すかは、この環境では確認できない。`beforeunload` を外すと、出さない実装で打ち切りが消え、デッドロックが戻る。#488 の実機確認のあとに判断する。

### E. 大きな設計変更（見送り）

ダイアログ待ちをロックの外へ出す、タイムアウトを利用者が設定できるようにする、などは #499 の範囲を超える。現行の 30 分と世代による置き換えを仕様として固定する。

## 推奨案

A を実装し、B の代わりに「2回目はパニックしないが世代は進む。JS が1回にまとめる」と Go 側へ書く。留まる確認の条件は JS のコメントに残し、足すときは打ち切りを `pagehide` だけへ移す、と書く。

仕様として固定するのは次の3点である。

1. 新しい実行の開始は、いまのダイアログと、その run が後から出すダイアログの両方を打ち切る（置き換え）
2. `guiSession` の放置ダイアログは `guiDialogTimeout`（30分）でエラー終了する。ブリッジの `bridgeDialogTimeout` も同じ
3. `registerDialog` の `session == nil` はテスト専用

D と macOS 実機確認は残件とする。

## 実装手順・タスク分解

今回実施するもの:

1. `gonako-loader.js` と `ui/bundled/app.js` に `unloadDialogsCanceled` と、留まる確認のコメントを入れる
2. `guiSession.cancelPendingDialogs` と `commandBridge.cancelPendingDialogs` に、置き換え・世代は毎回進む・JS が1回にまとめる、を書く
3. `guiDialogTimeout` に、放置はエラー終了で利用者のキャンセルとは別、と書く
4. `registerDialog` に、`session == nil` はテスト専用、と書く
5. 置き換えを `TestNewRunCancelsDialogsTheOldRunEmitsLater` で固定する。unload フックのテストに、一度化フラグと留まる確認の文言が含まれることを足す
6. `docs/gonako-gui.md` と `docs/wnako3-bridge.md` に、利用者とブリッジの読者が辿れる短い仕様を足す

見送り:

- `beforeunload` を外して `pagehide` のみにする
- タイムアウト値の変更、待ちをロックの外へ出す設計
- macOS 実機での操作確認

## テスト方針

既存で覆っているもの:

- ダイアログ待ちのまま新しい実行を始めると、待ちがエラー終了し、2つ目が完了する（`TestNewRunCancelsPendingDialogAndReleasesExecMu`）
- ウィンドウ閉じ相当の `cancelPendingDialogs` で `execMu` が解放される。2回目は 0 件（`TestCancelPendingDialogsUnblocksWaitingRun`）
- 打ち切りはキャンセル回答にならない（`TestCanceledDialogReturnsErrorNotCancelAnswer`）
- resolve と cancel の競合（`TestResolveAndCancelRaceIsSafe`）
- 打ち切りの直後に登録されるダイアログは待ちに入らない（`TestDialogRegisteredAfterCancellationFails`）
- タイムアウトがエラーになる（`TestDialogTimeoutReturnsError`。上限を 10ms に縮めて実行）
- ブリッジ側の打ち切り・競合・タイムアウト（`bridge_cancel_test.go`）
- バンドルページとローダーに `beforeunload` / `pagehide` / 両方の Bind 名がある

今回足すもの:

- 打ち切りのあとで予約した新しい実行のダイアログは応答でき、予約前の世代の run が後から出すダイアログはエラーになる
- 両 JS に `unloadDialogsCanceled` と「留まる確認」の文言がある（生成したバンドル HTML と、埋め込みローダーのソース）

実行コマンド:

```bash
go test ./cmd/gonako-gui -count=1 -timeout 180s \
  -run 'TestNewRunCancelsDialogsTheOldRunEmitsLater|TestDialogTimeoutReturnsError|TestDialogRegisteredAfterCancellationFails|TestBundledPageCancelsPendingDialogsOnUnload|TestLoaderCancelsPendingDialogsOnUnload|TestNewRunCancelsPendingDialogAndReleasesExecMu|TestCancelPendingDialogsUnblocksWaitingRun'
```

競合は既存の `-race` 対象（`TestResolveAndCancelRaceIsSafe` と `bridge_cancel_test.go`）のままにする。今回の差分はコメントと、世代を読む順序が既にある経路のテスト追加と、JS の早期 return である。

実機で見ること（今回は未実施）は「未解決の論点」に書く。

## 未解決の論点

1. **macOS 実機（#488 側）**。この作業環境は Linux であり、WKWebView の発火順は確認していない。見るものは2つに分ける。
   - **再実行**: エディタでダイアログを出したまま実行する。`Cmd+R` / `Ctrl+R` / F5 / 実行ボタンはどれも `runCode()` なので、置き換えの確認に使える。期待は、出ているダイアログが消え、実行中のプログラムがエラー終了し、新しい実行の結果が出ること
   - **再読み込み**: エディタの `Cmd+R` は再読み込みではない（`preventDefault` して実行する）。開発者ツールから `location.reload()` する、別ウィンドウの梱包アプリや wnako3 実行で WebView の再読み込みを使う、など手順を分ける。期待は、待ちがエラーで解け、再読み込み後の実行がデッドロックしないこと。`beforeunload` と `pagehide` の両方が出るか、片方だけかも合わせて見る
2. **留まる確認を足すとき**。打ち切りを `pagehide` だけに移す。移す前に、対象の WebView が閉じる・再読み込みで `pagehide` を出すことを実機で確認する。確認が取れるまでは `beforeunload` も残す（今回の推奨）
3. **外部 URL**。`-url` では `cancelNakoDialogs` が無く、フックは空振りする。外部ページにダイアログ待ちの特権実行は載らない、という #183 の境界はそのままにする
4. **タイムアウトの 30 分**。利用者を急かさないための値で、設定項目にはしない。変える場合は `guiDialogTimeout` と `bridgeDialogTimeout` を揃える
