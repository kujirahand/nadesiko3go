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
//     1回だけ gonakoDropQueuePush を呼ぶ。push が成功した時点で data の
//     所有権はキューへ移り、他のドロップの data を解放することは決してない。
//   - 取得側は gonakoTakeDroppedFiles で先頭の項目を1つ取り出す。pointer と
//     length の読み取りと取り外しは mutex の内側で不可分に行う。取り出した
//     data の所有権は呼び出し側へ移り、呼び出し側がちょうど1回 free する。
//   - キューはドロップ順を保持する。取得はイベント処理側でドロップ順に
//     行われるため、各イベントは自分のドロップのパスを受け取る。
//   - data==NULL かつ length==0 の項目は「URIドロップだったが変換できる
//     パスがなかった」ことを表すマーカー。取得側では空と区別がつかず
//     何も受け取らないが、キューが1つ進むことで後続ドロップとの対応が
//     ずれない。

// data の所有権を受け取る。成功したら 1、失敗(項目の確保失敗)なら 0 を
// 返し、その場合の所有権は呼び出し側に残る(呼び出し側が free する)。
int gonakoDropQueuePush(char *data, size_t length);

// 先頭項目の data を返す。*length にその長さを入れる。キューが空、または
// 先頭がマーカー項目のときは NULL を返し *length は 0 になる。
char *gonakoTakeDroppedFiles(int *length);

// 検証用: data をコピーして項目を積む。GTK コールバックを通さずに
// キューの挙動を確認するために使う。
int gonakoDropQueuePushCopy(const char *data, size_t length);

// 検証用: 残っている項目をすべて解放して空にする。記録側と取得側の
// どちらも動いていないときにだけ呼ぶ。
void gonakoDropQueueClear(void);

#endif
