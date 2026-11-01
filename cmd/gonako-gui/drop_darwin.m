//go:build darwin

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include <stdlib.h>
#include <string.h>

// 直近のドロップのパス。NUL区切りのUTF-8で持つ。
static NSData *gonakoDroppedPaths = nil;
static IMP gonakoOriginalPerformDrag = NULL;

static BOOL gonakoPerformDragOperation(id self, SEL cmd, id<NSDraggingInfo> info) {
    NSArray *urls = [info.draggingPasteboard
        readObjectsForClasses:@[[NSURL class]]
        options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    NSMutableData *joined = [NSMutableData data];
    for (NSURL *url in urls) {
        const char *path = url.path.UTF8String;
        if (path == NULL) continue;
        [joined appendBytes:path length:strlen(path) + 1];
    }
    // cgoのObjective-CはARCなしでビルドされるので、保持と解放は手で行う。
    // 保持しないと、次のautoreleaseプールで解放されて不正なメモリを指す。
    @synchronized ([NSApplication class]) {
        [gonakoDroppedPaths release];
        gonakoDroppedPaths = joined.length > 0 ? [joined copy] : nil;
    }
    // WebKit本来の処理(JavaScriptのdropイベント)はその後に続く。
    return ((BOOL (*)(id, SEL, id))gonakoOriginalPerformDrag)(self, cmd, info);
}

// WKWebViewのドロップ処理に割り込み、フルパスを記録してから元の処理へ渡す。
void gonakoInstallFileDropHook(void) {
    if (gonakoOriginalPerformDrag != NULL) return;
    Class cls = NSClassFromString(@"WKWebView");
    if (cls == Nil) return;
    Method method = class_getInstanceMethod(cls, @selector(performDragOperation:));
    if (method == NULL) return;
    gonakoOriginalPerformDrag = method_getImplementation(method);
    method_setImplementation(method, (IMP)gonakoPerformDragOperation);
}

// 記録したパスを取り出して消す。呼び出し側がfreeする。
char *gonakoTakeDroppedFiles(int *length) {
    char *out = NULL;
    *length = 0;
    @synchronized ([NSApplication class]) {
        if (gonakoDroppedPaths != nil && gonakoDroppedPaths.length > 0) {
            *length = (int)gonakoDroppedPaths.length;
            out = malloc(gonakoDroppedPaths.length);
            if (out != NULL) memcpy(out, gonakoDroppedPaths.bytes, gonakoDroppedPaths.length);
            else *length = 0;
        }
        [gonakoDroppedPaths release];
        gonakoDroppedPaths = nil;
    }
    return out;
}

// S01: 保護window向けのexact origin navigation guard。
//
// delegateは実SDKのWKNavigationDelegate型を使う。trust originの設定に成功した
// windowだけに導入し、plain/-url windowには付けない。page-worldのmessage受信側も
// frameInfoを確認するため、これは境界のcommit阻止側を担う。
#import <WebKit/WebKit.h>

@interface GonakoNavigationGuard : NSObject <WKNavigationDelegate>
@property(nonatomic, copy) NSString *scheme;
@property(nonatomic, copy) NSString *host;
@property(nonatomic, assign) NSInteger port;
@end

@implementation GonakoNavigationGuard

- (BOOL)allowsURL:(NSURL *)url {
    if (url == nil) return NO;
    NSURLComponents *components =
        [NSURLComponents componentsWithURL:url resolvingAgainstBaseURL:NO];
    if (components == nil || components.scheme == nil ||
        components.host == nil || components.port == nil) {
        return NO;
    }
    if (![components.scheme.lowercaseString isEqualToString:self.scheme]) {
        return NO;
    }
    if (![components.host.lowercaseString isEqualToString:self.host]) {
        return NO;
    }
    return components.port.integerValue == self.port;
}

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationAction:(WKNavigationAction *)action
                  decisionHandler:(void (^)(WKNavigationActionPolicy))handler {
    // nil targetFrameはnew-window要求である。trusted URLでも作成しない。
    if (action.targetFrame == nil) {
        handler(WKNavigationActionPolicyCancel);
        return;
    }
    handler([self allowsURL:action.request.URL] ? WKNavigationActionPolicyAllow
                                                : WKNavigationActionPolicyCancel);
}

- (void)webView:(WKWebView *)webView
    decidePolicyForNavigationResponse:(WKNavigationResponse *)response
                    decisionHandler:(void (^)(WKNavigationResponsePolicy))handler {
    handler([self allowsURL:response.response.URL]
                ? WKNavigationResponsePolicyAllow
                : WKNavigationResponsePolicyCancel);
}

@end

void gonakoInstallNavigationGuard(void *wkwebview, const char *scheme,
                                  const char *host, int port) {
    WKWebView *view = (__bridge WKWebView *)wkwebview;
    if (view == nil) return;
    GonakoNavigationGuard *guard = [GonakoNavigationGuard new];
    guard.scheme = [NSString stringWithUTF8String:scheme];
    guard.host = [NSString stringWithUTF8String:host];
    guard.port = port;
    // navigationDelegateはweakなので、guardはviewのassociated objectで保持する。
    objc_setAssociatedObject(view, "gonakoNavigationGuard", guard,
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    view.navigationDelegate = guard;
}
