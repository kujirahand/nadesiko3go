//go:build darwin

#import <Cocoa/Cocoa.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>

// allowedFileTypes keeps compatibility with macOS versions before UTType was
// introduced. It is deprecated only on newer SDKs.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// gonakoOnMainThread は block を必ずメインスレッドで実行し、終わるまで待つ。
//
// NSOpenPanel / NSSavePanel は NSWindow のサブクラスで、生成・表示・runModal の
// すべてがメインスレッド専用。なでしこの『ファイル選択』『ファイル保存選択』
// 『フォルダ選択』は、startNakoFile のようなバックグラウンド実行やイベント
// 処理の goroutine から呼ばれるため、そのままでは AppKit を別スレッドから
// 触ってしまう。
//
// 呼び出し元がすでにメインスレッドならそのまま実行する（runNakoFile の
// ようにUIスレッド上で同期的に走る経路）。dispatch_sync はメインスレッドから
// 呼ぶと自分自身を待ってデッドロックするので使わない。別スレッドからは
// dispatch_async でメインキューへ投げ、セマフォで完了を待つ。
static void gonakoOnMainThread(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
        return;
    }
    dispatch_semaphore_t finished = dispatch_semaphore_create(0);
    dispatch_async(dispatch_get_main_queue(), ^{
        block();
        dispatch_semaphore_signal(finished);
    });
    dispatch_semaphore_wait(finished, DISPATCH_TIME_FOREVER);
#if !__has_feature(objc_arc)
    // MRCビルド（cgo の既定）ではセマフォを明示的に解放する。
    dispatch_release(finished);
#endif
}

static NSURL *gonakoDirectoryURL(const char *path) {
    if (path == NULL || path[0] == '\0') {
        return nil;
    }
    NSString *directory = [NSString stringWithUTF8String:path];
    return directory == nil ? nil : [NSURL fileURLWithPath:directory isDirectory:YES];
}

static NSArray<NSString *> *gonakoAllowedFileTypes(const char *extension) {
    if (extension == NULL || extension[0] == '\0') {
        return nil;
    }
    NSString *value = [NSString stringWithUTF8String:extension];
    if (value == nil) {
        return nil;
    }
    if ([value hasPrefix:@"."]) {
        value = [value substringFromIndex:1];
    }
    return value.length == 0 ? nil : @[value];
}

static char *gonakoCopyPath(NSURL *url) {
    if (url == nil || url.path == nil) {
        return NULL;
    }
    const char *path = [url.path UTF8String];
    return path == NULL ? NULL : strdup(path);
}

// gonakoShowOpenPanel は「ファイルを開く」パネルを出し、選ばれたパスを
// strdup 済みの C 文字列で返す（NULL は取り消し）。解放は呼び出し元が
// gonakoFreeDialogResult で行う。
//
// defaultDir / extension は Go 側が関数の戻りまで保持しているので、ブロックに
// そのまま渡してよい（ブロックの実行完了を待ってから戻る）。
char *gonakoShowOpenPanel(const char *defaultDir, const char *extension) {
    __block char *result = NULL;
    gonakoOnMainThread(^{
        @autoreleasepool {
            // activateIgnoringOtherApps: も AppKit なので、必ずメインスレッドの
            // 内側で呼ぶ。
            [NSApp activateIgnoringOtherApps:YES];
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            panel.title = @"ファイルを開く";
            panel.prompt = @"開く";
            panel.canChooseFiles = YES;
            panel.canChooseDirectories = NO;
            panel.allowsMultipleSelection = NO;
            panel.directoryURL = gonakoDirectoryURL(defaultDir);

            NSArray<NSString *> *types = gonakoAllowedFileTypes(extension);
            if (types != nil) {
                panel.allowedFileTypes = types;
                panel.allowsOtherFileTypes = NO;
            }
            result = [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
        }
    });
    return result;
}

char *gonakoShowSavePanel(const char *defaultDir, const char *defaultName, const char *extension) {
    __block char *result = NULL;
    gonakoOnMainThread(^{
        @autoreleasepool {
            [NSApp activateIgnoringOtherApps:YES];
            NSSavePanel *panel = [NSSavePanel savePanel];
            panel.title = @"名前を付けて保存";
            panel.prompt = @"保存";
            panel.canCreateDirectories = YES;
            panel.extensionHidden = NO;
            panel.directoryURL = gonakoDirectoryURL(defaultDir);

            if (defaultName != NULL && defaultName[0] != '\0') {
                NSString *name = [NSString stringWithUTF8String:defaultName];
                if (name != nil) {
                    panel.nameFieldStringValue = name;
                }
            }
            NSArray<NSString *> *types = gonakoAllowedFileTypes(extension);
            if (types != nil) {
                panel.allowedFileTypes = types;
                panel.allowsOtherFileTypes = NO;
            }
            result = [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
        }
    });
    return result;
}

char *gonakoShowFolderPanel(const char *defaultDir) {
    __block char *result = NULL;
    gonakoOnMainThread(^{
        @autoreleasepool {
            [NSApp activateIgnoringOtherApps:YES];
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            panel.title = @"フォルダを選択";
            panel.prompt = @"選択";
            panel.canChooseFiles = NO;
            panel.canChooseDirectories = YES;
            panel.allowsMultipleSelection = NO;
            panel.canCreateDirectories = YES;
            panel.directoryURL = gonakoDirectoryURL(defaultDir);
            result = [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
        }
    });
    return result;
}

// gonakoFreeDialogResult は gonakoShow*Panel が返した C 文字列を解放する。
// strdup(malloc) しただけなので、どのスレッドから呼んでもよい。
void gonakoFreeDialogResult(char *result) {
    free(result);
}

#pragma clang diagnostic pop
