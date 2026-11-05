// 対象はファイル名の _linux サフィックスで Linux に絞る。
// このファイルは GTK に依存しない。gcc 単体でもコンパイルでき、
// AddressSanitizer / ThreadSanitizer のハーネスで検証できる。
#include "drop_buffer_linux.h"

#include <pthread.h>
#include <stdlib.h>
#include <string.h>

typedef struct GonakoDropItem {
	struct GonakoDropItem *next;
	char *data; // マーカー項目では NULL
	size_t length;
} GonakoDropItem;

// プロセスと共に生きる単一のキュー。終了時に解放はしない(プロセス終了で
// OS が回収する)。残った項目があっても、解放済み参照や二重解放にはならない。
static pthread_mutex_t gonakoDropQueueMutex = PTHREAD_MUTEX_INITIALIZER;
static GonakoDropItem *gonakoDropQueueHead = NULL;
static GonakoDropItem *gonakoDropQueueTail = NULL;

int gonakoDropQueuePush(char *data, size_t length) {
	GonakoDropItem *item = (GonakoDropItem *)malloc(sizeof(*item));
	if (item == NULL) {
		return 0;
	}
	item->next = NULL;
	item->data = data;
	item->length = length;

	pthread_mutex_lock(&gonakoDropQueueMutex);
	if (gonakoDropQueueTail != NULL) {
		gonakoDropQueueTail->next = item;
	} else {
		gonakoDropQueueHead = item;
	}
	gonakoDropQueueTail = item;
	pthread_mutex_unlock(&gonakoDropQueueMutex);
	return 1;
}

char *gonakoTakeDroppedFiles(int *length) {
	*length = 0;

	pthread_mutex_lock(&gonakoDropQueueMutex);
	GonakoDropItem *item = gonakoDropQueueHead;
	if (item == NULL) {
		pthread_mutex_unlock(&gonakoDropQueueMutex);
		return NULL;
	}
	gonakoDropQueueHead = item->next;
	if (gonakoDropQueueHead == NULL) {
		gonakoDropQueueTail = NULL;
	}
	pthread_mutex_unlock(&gonakoDropQueueMutex);

	// ここから先、item と data はこの呼び出しだけのもの。他スレッドが
	// 触れることはないので、ロックの外で受け渡してよい。
	char *data = item->data;
	*length = (int)item->length;
	free(item);
	return data;
}

// NUL 区切りのバイト列を区切りごとに数える。空の区切りは数えない。
static int gonakoCountSegments(const char *buf, size_t length) {
	int count = 0;
	size_t start = 0;
	for (size_t i = 0; i < length; i++) {
		if (buf[i] == '\0') {
			if (i > start) {
				count++;
			}
			start = i + 1;
		}
	}
	if (start < length) {
		count++;
	}
	return count;
}

typedef struct {
	const char *ptr;
	size_t len;
} GonakoSegment;

// 区切りを取り出す。segs には gonakoCountSegments と同じ数だけ入る。
static void gonakoFillSegments(const char *buf, size_t length, GonakoSegment *segs) {
	int n = 0;
	size_t start = 0;
	for (size_t i = 0; i < length; i++) {
		if (buf[i] == '\0') {
			if (i > start) {
				segs[n].ptr = buf + start;
				segs[n].len = i - start;
				n++;
			}
			start = i + 1;
		}
	}
	if (start < length) {
		segs[n].ptr = buf + start;
		segs[n].len = length - start;
	}
}

// パスのベース名部分を指すビューを返す。
static GonakoSegment gonakoBasename(GonakoSegment seg) {
	size_t start = 0;
	for (size_t i = 0; i < seg.len; i++) {
		if (seg.ptr[i] == '/') {
			start = i + 1;
		}
	}
	seg.ptr += start;
	seg.len -= start;
	return seg;
}

static int gonakoSegmentEqual(GonakoSegment a, GonakoSegment b) {
	return a.len == b.len && memcmp(a.ptr, b.ptr, a.len) == 0;
}

// 項目のパスのベース名の集合が、イベントのファイル名の集合と一致するか。
// どちらの側も重複を許し、順序は見ない(多重集合として比べる)。
static int gonakoEntryMatchesNames(GonakoDropItem *item, const char *names, size_t namesLength) {
	int entryCount = gonakoCountSegments(item->data, item->length);
	int nameCount = gonakoCountSegments(names, namesLength);
	if (entryCount == 0 || entryCount != nameCount) {
		return 0;
	}
	GonakoSegment *entrySegs = (GonakoSegment *)malloc(sizeof(GonakoSegment) * (size_t)entryCount);
	char *used = (char *)calloc((size_t)entryCount, 1);
	if (entrySegs == NULL || used == NULL) {
		free(entrySegs);
		free(used);
		return 0;
	}
	gonakoFillSegments(item->data, item->length, entrySegs);
	for (int i = 0; i < entryCount; i++) {
		entrySegs[i] = gonakoBasename(entrySegs[i]);
	}

	int matched = 1;
	size_t start = 0;
	for (size_t i = 0; i <= namesLength && matched; i++) {
		if (i != namesLength && names[i] != '\0') {
			continue;
		}
		if (i > start) {
			GonakoSegment name;
			name.ptr = names + start;
			name.len = i - start;
			name = gonakoBasename(name);
			int found = 0;
			for (int j = 0; j < entryCount; j++) {
				if (!used[j] && gonakoSegmentEqual(entrySegs[j], name)) {
					used[j] = 1;
					found = 1;
					break;
				}
			}
			if (!found) {
				matched = 0;
			}
		}
		start = i + 1;
	}
	free(entrySegs);
	free(used);
	return matched;
}

char *gonakoTakeDroppedFilesForEvent(const char *names, int namesLength, int *length) {
	*length = 0;
	if (names == NULL || namesLength <= 0) {
		return NULL;
	}

	pthread_mutex_lock(&gonakoDropQueueMutex);
	GonakoDropItem *cur = gonakoDropQueueHead;
	while (cur != NULL) {
		if (cur->data != NULL && gonakoEntryMatchesNames(cur, names, (size_t)namesLength)) {
			// 一致した項目より前は、対応するイベントが失われた残り。
			// 後続の対応を保つため、ここでまとめて捨てる。
			GonakoDropItem *stale = gonakoDropQueueHead;
			while (stale != cur) {
				GonakoDropItem *next = stale->next;
				free(stale->data);
				free(stale);
				stale = next;
			}
			gonakoDropQueueHead = cur->next;
			if (gonakoDropQueueHead == NULL) {
				gonakoDropQueueTail = NULL;
			}
			pthread_mutex_unlock(&gonakoDropQueueMutex);

			char *data = cur->data;
			*length = (int)cur->length;
			free(cur);
			return data;
		}
		cur = cur->next;
	}
	// 一致がなければ、先頭のマーカーだけを消費して返すものはない。
	// 実項目は将来のイベントのものかもしれないので残す。
	if (gonakoDropQueueHead != NULL && gonakoDropQueueHead->data == NULL) {
		GonakoDropItem *marker = gonakoDropQueueHead;
		gonakoDropQueueHead = marker->next;
		if (gonakoDropQueueHead == NULL) {
			gonakoDropQueueTail = NULL;
		}
		free(marker);
	}
	pthread_mutex_unlock(&gonakoDropQueueMutex);
	return NULL;
}

int gonakoDropQueuePushCopy(const char *data, size_t length) {
	char *copy = NULL;
	if (length > 0) {
		copy = (char *)malloc(length);
		if (copy == NULL) {
			return 0;
		}
		memcpy(copy, data, length);
	}
	if (!gonakoDropQueuePush(copy, length)) {
		free(copy);
		return 0;
	}
	return 1;
}

void gonakoDropQueueClear(void) {
	pthread_mutex_lock(&gonakoDropQueueMutex);
	GonakoDropItem *item = gonakoDropQueueHead;
	gonakoDropQueueHead = NULL;
	gonakoDropQueueTail = NULL;
	pthread_mutex_unlock(&gonakoDropQueueMutex);

	while (item != NULL) {
		GonakoDropItem *next = item->next;
		free(item->data);
		free(item);
		item = next;
	}
}
