// drop_buffer_linux.c の検証ハーネス。go test からは実行されず、
// C 側共有キューのメモリ安全性と競合を単体で確認するために置く。
//
// 実行方法(リポジトリのルートで):
//   gcc -Icmd/gonako-gui -g -O1 -pthread -fsanitize=address \
//     cmd/gonako-gui/testdata/drop_buffer_harness.c \
//     cmd/gonako-gui/drop_buffer_linux.c -o /tmp/drop_harness_asan
//   /tmp/drop_harness_asan multi && /tmp/drop_harness_asan ordered && /tmp/drop_harness_asan match
//   gcc -Icmd/gonako-gui -g -O1 -pthread -fsanitize=thread \
//     cmd/gonako-gui/testdata/drop_buffer_harness.c \
//     cmd/gonako-gui/drop_buffer_linux.c -o /tmp/drop_harness_tsan
//   setarch x86_64 -R /tmp/drop_harness_tsan multi
//   setarch x86_64 -R /tmp/drop_harness_tsan ordered
//   setarch x86_64 -R /tmp/drop_harness_tsan match
// (ThreadSanitizer は実行環境によっては setarch -R が必要)
//
// 解放後参照・二重解放・リークは AddressSanitizer が、データ競合は
// ThreadSanitizer が検出する。どちらも検出があれば異常終了する。
#include "drop_buffer_linux.h"

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define PRODUCERS 4
#define PER_PRODUCER 2000
#define TOTAL (PRODUCERS * PER_PRODUCER)

static pthread_mutex_t checkMu = PTHREAD_MUTEX_INITIALIZER;
static int seen[PRODUCERS][PER_PRODUCER];
static int received = 0;
static int errors = 0;

static void *producer(void *arg) {
	long p = (long)arg;
	char buf[64];
	for (int i = 0; i < PER_PRODUCER; i++) {
		int n = snprintf(buf, sizeof(buf), "/p%ld/%d", p, i);
		// 実ペイロードと同じく末尾の NUL まで含める。
		if (!gonakoDropQueuePushCopy(buf, (size_t)n + 1)) {
			fprintf(stderr, "push failed p=%ld i=%d\n", p, i);
			exit(2);
		}
		// ときどき「パスなし」マーカーも積む。
		if (i % 500 == 499) {
			if (!gonakoDropQueuePushCopy(NULL, 0)) {
				fprintf(stderr, "marker push failed\n");
				exit(2);
			}
		}
	}
	return NULL;
}

static void *consumer(void *arg) {
	(void)arg;
	for (;;) {
		pthread_mutex_lock(&checkMu);
		int done = received >= TOTAL;
		pthread_mutex_unlock(&checkMu);
		if (done) return NULL;

		int length = -1;
		char *data = gonakoTakeDroppedFiles(&length);
		if (data == NULL) {
			if (length != 0) {
				fprintf(stderr, "empty take set length=%d\n", length);
				exit(2);
			}
			continue; // 空、またはマーカーを取り出した
		}
		int p = -1, i = -1;
		if (sscanf(data, "/p%d/%d", &p, &i) != 2 || p < 0 || p >= PRODUCERS || i < 0 || i >= PER_PRODUCER) {
			fprintf(stderr, "corrupt payload: %p len=%d\n", (void *)data, length);
			exit(2);
		}
		if (length != (int)strlen(data) + 1) {
			fprintf(stderr, "length mismatch: %d vs %zu\n", length, strlen(data) + 1);
			exit(2);
		}
		pthread_mutex_lock(&checkMu);
		if (seen[p][i]) {
			fprintf(stderr, "duplicate payload p=%d i=%d\n", p, i);
			errors++;
		}
		seen[p][i] = 1;
		received++;
		pthread_mutex_unlock(&checkMu);
		free(data); // Go 側と同じく、取り出した側が1回だけ free する
	}
}

// 順序確認用: consumer が1本のときは producer ごとの単調増加も保証される。
static void *orderedConsumer(void *arg) {
	(void)arg;
	int localLast[PRODUCERS];
	for (int p = 0; p < PRODUCERS; p++) localLast[p] = -1;
	int got = 0;
	while (got < TOTAL) {
		int length = 0;
		char *data = gonakoTakeDroppedFiles(&length);
		if (data == NULL) continue;
		int p = -1, i = -1;
		if (sscanf(data, "/p%d/%d", &p, &i) != 2) {
			fprintf(stderr, "corrupt payload\n");
			exit(2);
		}
		if (i <= localLast[p]) {
			fprintf(stderr, "order broken: p=%d %d after %d\n", p, i, localLast[p]);
			exit(2);
		}
		localLast[p] = i;
		free(data);
		got++;
	}
	return NULL;
}

static void pushJoined(const char *joined, size_t length) {
	if (!gonakoDropQueuePushCopy(joined, length)) {
		fprintf(stderr, "push failed in match scenario\n");
		exit(2);
	}
}

// want は期待するペイロード全体(NUL 区切り、末尾の NUL まで含む)。
// wantLength はそのバイト数。want が NULL なら「取れないこと」を期待する。
static void expectEvent(const char *names, int namesLength, const char *want, size_t wantLength) {
	int length = -1;
	char *data = gonakoTakeDroppedFilesForEvent(names, namesLength, &length);
	if (want == NULL) {
		if (data != NULL) {
			fprintf(stderr, "match scenario: expected NULL, got %p\n", (void *)data);
			exit(2);
		}
		return;
	}
	if (data == NULL || length != (int)wantLength || memcmp(data, want, wantLength) != 0) {
		fprintf(stderr, "match scenario: payload mismatch (length=%d, want %zu)\n", length, wantLength);
		exit(2);
	}
	free(data);
}

// 名前による照合の確認: 失われたイベントの項目を飛ばして自分の項目を取る、
// 一致しないイベントは何も取らない、マーカーが対応をずらさない。
static int matchScenario(void) {
	pushJoined("/x/a.txt", strlen("/x/a.txt") + 1);
	const char bc[] = "/y/b.txt\0/y/c.txt";
	pushJoined(bc, sizeof(bc)); // 末尾の NUL まで含む
	if (!gonakoDropQueuePushCopy(NULL, 0)) {
		fprintf(stderr, "marker push failed\n");
		exit(2);
	}
	pushJoined("/z/d.txt", strlen("/z/d.txt") + 1);

	// 先の a.txt のイベントが失われていても、b/c のイベントは自分の取る。
	// その際 a.txt は残りとして捨てられる。
	const char namesBC[] = "c.txt\0b.txt";
	expectEvent(namesBC, (int)sizeof(namesBC) - 1, bc, sizeof(bc));
	// 捨てられた a.txt はもう取れない。
	expectEvent("a.txt", 5, NULL, 0);
	// 一致しないイベントは何も取らない(先頭のマーカーだけ消費される)。
	expectEvent("unknown.txt", 11, NULL, 0);
	// マーカーに阻まれず d.txt が取れる。
	const char d[] = "/z/d.txt";
	expectEvent("d.txt", 5, d, sizeof(d));
	// 空になった。
	int length = 99;
	if (gonakoTakeDroppedFiles(&length) != NULL || length != 0) {
		fprintf(stderr, "match scenario: queue should be empty\n");
		exit(2);
	}
	printf("OK mode=match\n");
	return 0;
}

int main(int argc, char **argv) {
	if (argc > 1 && strcmp(argv[1], "match") == 0) {
		return matchScenario();
	}
	int orderedMode = argc > 1 && strcmp(argv[1], "ordered") == 0;

	int length = 99;
	if (gonakoTakeDroppedFiles(&length) != NULL || length != 0) {
		fprintf(stderr, "empty queue should return NULL/0\n");
		return 2;
	}

	pthread_t prod[PRODUCERS];
	for (long p = 0; p < PRODUCERS; p++) {
		pthread_create(&prod[p], NULL, producer, (void *)p);
	}
	pthread_t cons;
	if (orderedMode) {
		pthread_create(&cons, NULL, orderedConsumer, NULL);
		for (int p = 0; p < PRODUCERS; p++) pthread_join(prod[p], NULL);
		pthread_join(cons, NULL);
	} else {
		pthread_t cons2[3];
		pthread_create(&cons, NULL, consumer, NULL);
		for (int i = 0; i < 3; i++) pthread_create(&cons2[i], NULL, consumer, NULL);
		for (int p = 0; p < PRODUCERS; p++) pthread_join(prod[p], NULL);
		pthread_join(cons, NULL);
		for (int i = 0; i < 3; i++) pthread_join(cons2[i], NULL);
		if (errors > 0 || received != TOTAL) {
			fprintf(stderr, "received=%d errors=%d\n", received, errors);
			return 2;
		}
	}
	gonakoDropQueueClear();
	printf("OK mode=%s total=%d\n", orderedMode ? "ordered" : "multi", TOTAL);
	return 0;
}
