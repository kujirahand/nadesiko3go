// S01: vendor-owned Cocoa navigation guard.
//
// このファイルは third_party/webview_go package に属し、app (cmd/gonako-gui) の
// translation unit に依存しない。vendor consumer / internal/guilib の単独linkでも
// 定義が揃うように、guard実装をvendor側で完結させる。
#import <WebKit/WebKit.h>
#import <objc/runtime.h>

extern int gonakoNativeTrustAllowsURL(void *engine, const char *url);

@interface GonakoNavigationGuard : NSObject <WKNavigationDelegate>
@property(nonatomic, assign) void *engine;
@property(nonatomic, copy) NSString *scheme;
@property(nonatomic, copy) NSString *host;
@property(nonatomic, assign) NSInteger port;
@end

@implementation GonakoNavigationGuard

- (void)dealloc {
    // deallocは、view側のteardownに伴う解放と、gonakoDetachNavigationGuardに
    // よる明示的な解放の両方から来る。前者の時点ではviewは自分のdeallocを
    // 終えており、weakなnavigationDelegateもview側で無効化されている。後者では
    // 呼び出し側が先にdelegateを外している。どちらの場合もviewへメッセージは
    // 送らず、engineの生ポインタだけを捨てる（viewのdealloc中にviewを触ると、
    // 解放済みの内部状態に触れてしまう）。
    _engine = NULL;
    // MRC: copy propertyの所有参照を解放する。
    [_scheme release];
    [_host release];
    [super dealloc];
}

- (BOOL)allowsURL:(NSURL *)url {
    if (url == nil) return NO;
    // native trustがrevoke済みなら、copied originが一致していてもdenyする。
    // Aもmessage guardと同じ失効点へ追随する。
    const char *text = url.absoluteString.UTF8String;
    if (self.engine == NULL || text == NULL ||
        !gonakoNativeTrustAllowsURL(self.engine, text)) {
        return NO;
    }
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
    // S01: 保護ウィンドウの現行仕様（意図した挙動であり、未実装ではない）。
    // - このdelegateはmain frameとsub frameの両方のnavigation actionに来る。
    //   ここではtargetFrameの別を見ず、要求URLのoriginだけで判定する。したがって
    //   外部originのsub frame（動画埋め込み等のiframe）は読み込めない。
    //   about:blank / about:srcdoc / data: / blob: のchildの扱いはCocoa実機での
    //   確認待ちである。ブリッジのメッセージ判定はURLではなく
    //   WKFrameInfo.securityOriginのtuple照合なので、HTML仕様上 about:blank や
    //   trustedページが生成した blob: が親originを継承し、securityOriginが
    //   trusted tupleを報告すれば許可されうる。継承originは親と同じ権限なので、
    //   権限昇格にはならず、セキュリティ上の穴ではない。
    // - new-window（targetFrame == nil、target=_blank / window.open / mailto: 等）は
    //   常にcancelする。別ウィンドウに特権Bindを渡さないための措置で、外部ブラウザへ
    //   委ねる経路はこのguardには持たせていない。Go側のopenExternalURL
    //   （http/httpsのみ）が、アプリが用意した安全な外部遷移の経路である。
    // - 「main frameだけ判定する」への緩和は意図的に見送っている。緩和するには
    //   GTK/Windowsのmain/sub判別を実機で確認する必要があり、未検証の挙動変更は
    //   行わない。スコープを狭める場合は、このコメントとdocsの記載を同時に直すこと。
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

static char gonakoNavigationGuardKey;

void gonakoInstallNavigationGuard(void *wkwebview, void *engine,
                                  const char *scheme, const char *host,
                                  int port) {
    WKWebView *view = (__bridge WKWebView *)wkwebview;
    if (view == nil) return;
    GonakoNavigationGuard *guard = [GonakoNavigationGuard new];
    guard.engine = engine;
    guard.scheme = [NSString stringWithUTF8String:scheme];
    guard.host = [NSString stringWithUTF8String:host];
    guard.port = port;
    // navigationDelegateはweakなので、guardはassociated objectで保持する。
    view.navigationDelegate = guard;
    objc_setAssociatedObject(view, &gonakoNavigationGuardKey, guard,
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
    // newの所有参照を解放し、associated objectのretainだけに残す。
    [guard release];
}

// 保護windowの破棄時に、WKWebViewがまだ生存しているうちにdelegateを外す。
// engineの生ポインタはここで捨てるので、engineの解放後にguardがnavigation
// callbackを受けて解放済みメモリを読むことはない。guardはassociated objectが
// 保持しているため、最後にassociated objectを外して解放する。
void gonakoDetachNavigationGuard(void *wkwebview) {
    WKWebView *view = (__bridge WKWebView *)wkwebview;
    if (view == nil) return;
    GonakoNavigationGuard *guard = objc_getAssociatedObject(
        view, &gonakoNavigationGuardKey);
    if (guard == nil) return;
    // 順序: viewは生存中なのでdelegateを先に外す。次にengineを無効化してから
    // associated objectを外す（この解放でdeallocが走る）。
    view.navigationDelegate = nil;
    guard.engine = NULL;
    objc_setAssociatedObject(view, &gonakoNavigationGuardKey, nil,
                             OBJC_ASSOCIATION_RETAIN_NONATOMIC);
}
