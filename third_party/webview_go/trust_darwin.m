// S01: vendor-owned Cocoa navigation guard.
//
// このファイルは third_party/webview_go package に属し、app (cmd/gonako-gui) の
// translation unit に依存しない。vendor consumer / internal/guilib の単独linkでも
// 定義が揃うように、guard実装をvendor側で完結させる。
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

@interface GonakoNavigationGuard : NSObject <WKNavigationDelegate>
@property(nonatomic, copy) NSString *scheme;
@property(nonatomic, copy) NSString *host;
@property(nonatomic, assign) NSInteger port;
@end

@implementation GonakoNavigationGuard

- (void)dealloc {
    // MRC: copy propertyの所有参照を解放する。
    [_scheme release];
    [_host release];
    [super dealloc];
}

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
    // navigationDelegateはweakなので、guardはassociated objectで保持する。
    view.navigationDelegate = guard;
    objc_setAssociatedObject(view, "gonakoNavigationGuard", guard,
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    // newの所有参照を解放し、associated objectのretainだけに残す。
    [guard release];
}
