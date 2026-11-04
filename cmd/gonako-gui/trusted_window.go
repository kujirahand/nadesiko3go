package main

// 保護window（S01）の所有loopback serverとfactory。
//
// 保護windowのtrustは、このファイルで作った自前listenerのscheme/host/portの
// 完全一致だけである。plain window（-url/エラー表示）はBindを持たない。
// serverはwindowの寿命に束縛し、native windowのDestroy後にだけcloseする。

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/kujirahand/nadesiko3go/internal/guilib"
	webview "github.com/webview/webview_go"
)

// trustRevokingListener は listener の close より前に native trust を失効させる。
// http.Server.Serve は return 時に listener を close するため、この wrapper が
// port 返却前の revocation を保証する。
type trustRevokingListener struct {
	net.Listener
	once  sync.Once
	owner *ownedServer
}

func (l *trustRevokingListener) Close() error {
	l.once.Do(func() {
		if l.owner != nil {
			l.owner.revokeOnce()
		}
	})
	return l.Listener.Close()
}

type ownedServer struct {
	listener net.Listener
	server   *http.Server
	origin   string

	mu      sync.Mutex
	revoke  func()
	revoked bool
	failed  bool
	closed  bool

	closeOne sync.Once
}

// startOwnedServer は127.0.0.1の動的portでhandlerを配信する。
// Serve の異常終了は failed に記録し、revocation は wrapper の Close が行う。
func startOwnedServer(handler http.Handler) (*ownedServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("ローカルサーバーを起動できません: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	s := &ownedServer{
		server: &http.Server{Handler: loopbackOnly(handler)},
		origin: fmt.Sprintf("http://127.0.0.1:%d", port),
	}
	wrapped := &trustRevokingListener{Listener: listener, owner: s}
	s.listener = wrapped
	go func() {
		err := s.server.Serve(wrapped)
		if err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.failed = true
			s.mu.Unlock()
		}
	}()
	return s, nil
}

// SetRevoker は native trust の失効関数を設定する。Serve が既に失敗/終了して
// いる場合は、windowが生存している間だけ直ちに失効させる。
func (s *ownedServer) SetRevoker(fn func()) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.revoked || s.failed {
		// 既知の native revoker は atomic-only で、Go mutex へ再入しない。
		// lifetime の同期点を曖昧にしないため s.mu 下で完了させる。
		if fn != nil {
			fn()
		}
		return
	}
	s.revoke = fn
}

// LeaseValid は server がまだこのwindowのoriginを所有しているかを返す。
func (s *ownedServer) LeaseValid() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.failed && !s.closed && !s.revoked
}

// revokeOnce は trust を一度だけ失効させる。listener close の前に呼ぶ。
func (s *ownedServer) revokeOnce() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.revoked {
		return
	}
	s.revoked = true
	fn := s.revoke
	s.revoke = nil
	// fn は native atomic store のみ。s.mu 下で完了させ、RevokeAndDetach と
	// native lifetime を同じ同期点に置く。
	if fn != nil {
		fn()
	}
}

// RevokeAndDetach は native window が生存している間に trust を失効させ、
// revoker を外す。late Serve callback が解放済み native pointer を触らない。
func (s *ownedServer) RevokeAndDetach() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fn := s.revoke
	s.revoke = nil
	if !s.revoked {
		s.revoked = true
		if fn != nil {
			// native object が生存している間に atomic revoke を完了する。
			fn()
		}
	}
}

func (s *ownedServer) Close() {
	if s == nil {
		return
	}
	s.closeOne.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		// port を返す前に privilege を失効させる。
		s.revokeOnce()
		_ = s.server.Close()
		_ = s.listener.Close()
	})
}

func (s *ownedServer) URL(path string) string {
	return s.origin + path
}

// ownedPageHandler は、生成の途中で一度だけ確定するHTMLを配信する。
// HTMLを確定するのはウィンドウを作った側のgoroutine、読むのはHTTPサーバーの
// goroutineであり、その間の順序はC（WebViewのイベントループ）とソケット越しに
// しか起きない。Goのrace detectorにはその順序が見えないため、値はmutexで
// 保護して明示的なhappens-beforeを作る。
type ownedPageHandler struct {
	mu   sync.RWMutex
	html string
}

// SetHTML は配信するHTMLを差し替える。Navigateの前に一度だけ呼ぶ。
func (h *ownedPageHandler) SetHTML(html string) {
	h.mu.Lock()
	h.html = html
	h.mu.Unlock()
}

func (h *ownedPageHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	html := h.html
	h.mu.RUnlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// trustedWindow は保護windowとその所有serverをまとめて破棄する。
type trustedWindow struct {
	Window *webview.Protected
	server *ownedServer
}

// newTrustedWindowWithServer は既存serverのoriginで保護windowを作る。
// server が既に失敗/revoke 済みなら window を公開せず破棄する。
func newTrustedWindowWithServer(debug bool, server *ownedServer, settings guilib.WindowSettings) (*trustedWindow, error) {
	w, err := webview.NewProtected(debug, webview.TrustSpec{Origin: server.origin})
	if err != nil {
		return nil, err
	}
	server.SetRevoker(w.RevokeTrust)
	if !server.LeaseValid() {
		server.RevokeAndDetach()
		w.Destroy()
		return nil, fmt.Errorf("ローカルサーバーが起動していません")
	}
	if err := applyWindowSettings(w, settings, false); err != nil {
		server.RevokeAndDetach()
		w.Destroy()
		return nil, err
	}
	// ドロップ経路も保護windowだけに登録する。失敗はwindow破棄へ倒す。
	bindFileDrop(w)
	if err := w.BindErr(); err != nil {
		server.RevokeAndDetach()
		w.Destroy()
		return nil, err
	}
	if !server.LeaseValid() {
		server.RevokeAndDetach()
		w.Destroy()
		return nil, fmt.Errorf("ローカルサーバーが停止しました")
	}
	return &trustedWindow{Window: w, server: server}, nil
}

// Navigate は server の lease が有効なときだけ初回/以後の有効な遷移を行う。
func (t *trustedWindow) Navigate(path string) error {
	if t == nil || t.Window == nil {
		return fmt.Errorf("保護windowがありません")
	}
	if t.server == nil || !t.server.LeaseValid() {
		return fmt.Errorf("ローカルサーバーのleaseが失効しています")
	}
	t.Window.Navigate(t.server.URL(path))
	return nil
}

// Close は native window の生存中に trust を失効させ、その後で window と
// server/port を破棄する。late Serve callback は revoker が外れている。
func (t *trustedWindow) Close() {
	if t == nil {
		return
	}
	if t.server != nil {
		t.server.RevokeAndDetach()
	}
	if t.Window != nil {
		t.Window.Destroy()
		t.Window = nil
	}
	if t.server != nil {
		t.server.Close()
		t.server = nil
	}
}

func (t *trustedWindow) BindErr() error {
	if t == nil || t.Window == nil {
		return fmt.Errorf("保護windowがありません")
	}
	return t.Window.BindErr()
}

// newPlainWindow はBindを持たないwindowを作る。-urlとエラー表示用。
func newPlainWindow(debug bool, settings guilib.WindowSettings) (*webview.Plain, error) {
	w, err := webview.NewPlain(debug)
	if err != nil {
		return nil, err
	}
	if err := applyPlainWindowSettings(w, settings, false); err != nil {
		w.Destroy()
		return nil, err
	}
	return w, nil
}
