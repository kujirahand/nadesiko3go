//go:build darwin

#import <Cocoa/Cocoa.h>
#include <stdlib.h>
#include <string.h>

// allowedFileTypes keeps compatibility with macOS versions before UTType was
// introduced. It is deprecated only on newer SDKs.
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

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

char *gonakoShowOpenPanel(const char *defaultDir, const char *extension) {
    @autoreleasepool {
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
        return [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
    }
}

char *gonakoShowSavePanel(const char *defaultDir, const char *defaultName, const char *extension) {
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
        return [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
    }
}

char *gonakoShowFolderPanel(const char *defaultDir) {
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
        return [panel runModal] == NSModalResponseOK ? gonakoCopyPath(panel.URL) : NULL;
    }
}

void gonakoFreeDialogResult(char *result) {
    free(result);
}

#pragma clang diagnostic pop
