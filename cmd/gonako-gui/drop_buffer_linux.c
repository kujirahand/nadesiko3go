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
