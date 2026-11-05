#ifndef GONAKO_DROP_BUFFER_LINUX_H
#define GONAKO_DROP_BUFFER_LINUX_H

#include <stddef.h>

// Linux のファイルドロップで、GTK コールバック(UIスレッド)が記録したパスを
// Go のイベント処理(ワーカースレッド)へ渡すための FIFO キュー。
// 実装は drop_buffer_linux.c に置く(ヘッダに static 実装を置くと、cgo の
// 翻訳単位ごとに状態が分かれるおそれがあるため)。
//
// 所有権とスレッドの前提:
//   - 記録側は GTK の drag-data-received コールバックで、ドロップのたびに
//     gonakoDropQueuePush を呼ぶ。push が成功した時点で data の所有権は
//     キューへ移り、他のドロップの data を解放することは決してない。
//   - 取得側はワーカースレッドから呼ばれ、項目の取り外しと pointer、
//     length の読み取りは mutex の内側で不可分に行う。取り出した data の
//     所有権は呼び出し側へ移り、呼び出し側がちょうど1回 free する。
//     取得同士は Go の画面セッションが直列化する前提で、記録だけが
//     非同期に割り込む。
//   - イベントとドロップの対応は「ファイル名の集合」で取る。記録は UI
//     スレッドで即座に行われるが、イベント処理は遅れたり、実行中のため
//     拒否されて失われたりする。単純な先入れ先出しでは、失われた
//     イベントの項目が残って後続のイベントが1つずつずれるため、
//     gonakoTakeDroppedFilesForEvent が名前の一致する項目を探して取り、
//     それより前の項目(対応するイベントが失われた残り)を捨てる。
//   - data==NULL かつ length==0 の項目は「URIドロップだったが変換できる
//     パスがなかった」ことを表すマーカー。名前を持たないので照合では
//     一致せず、後続の照合で「前の残り」として捨てられるか、照合に
//     失敗した取得で先頭にあるときだけ消費される。
//   - 照合の限界: 同名ファイルが別ディレクトリにあり、先のドロップの
//     項目が残りとして残っていると、後のイベントが古い方のパスを取る
//     ことがある(イベントの喪失と同名が重なった場合のみ、1回分)。
//     また、JS 側で置換・正規化されるファイル名(不正な UTF-8 や
//     NFC/NFD の差など)は照合に失敗し、その回のパスは取れない
//     (JavaScript が知っているファイル名のまま処理され、壊れはしない)。

// data の所有権を受け取る。成功したら 1、失敗(項目の確保失敗)なら 0 を
// 返し、その場合の所有権は呼び出し側に残る(呼び出し側が free する)。
int gonakoDropQueuePush(char *data, size_t length);

// 先頭項目の data を返す。*length にその長さを入れる。キューが空、または
// 先頭がマーカー項目のときは NULL を返し *length は 0 になる。
char *gonakoTakeDroppedFiles(int *length);

// イベントが知っているファイル名(names を NUL でつないだもの、末尾の
// NUL は不要)を使い、パスのベース名の集合が一致する項目を探して返す。
// 一致した項目より前の項目は解放して捨てる。一致がなければ、先頭が
// マーカー項目のときだけそれを捨て、NULL を返す。返した data の所有権は
// 呼び出し側へ移る。
char *gonakoTakeDroppedFilesForEvent(const char *names, int namesLength, int *length);

// 検証用: data をコピーして項目を積む。GTK コールバックを通さずに
// キューの挙動を確認するために使う。
int gonakoDropQueuePushCopy(const char *data, size_t length);

// 検証用: 残っている項目をすべて解放して空にする。記録側と取得側の
// どちらも動いていないときにだけ呼ぶ。
void gonakoDropQueueClear(void);

#endif
