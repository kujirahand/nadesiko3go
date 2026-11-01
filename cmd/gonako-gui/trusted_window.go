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

type ownedServer struct {
	listener net.Listener
	server   *http.Server
	origin   string
	closeOne sync.Once
}

// startOwnedServer は127.0.0.1の動的portでhandlerを配信する。
func startOwnedServer(handler http.Handler) (*ownedServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("ローカルサーバーを起動できません: %w", err)
	}
	server := &http.Server{Handler: loopbackOnly(handler)}
	go func() { _ = server.Serve(listener) }()
	port := listener.Addr().(*net.TCPAddr).Port
	return &ownedServer{
		listener: listener,
		server:   server,
		origin:   fmt.Sprintf("http://127.0.0.1:%d", port),
	}, nil
}

func (s *ownedServer) Close() {
	if s == nil {
		return
	}
	s.closeOne.Do(func() {
		_ = s.server.Close()
		_ = s.listener.Close()
	})
}

func (s *ownedServer) URL(path string) string {
	return s.origin + path
}

// trustedWindow は保護windowとその所有serverをまとめて破棄する。
type trustedWindow struct {
	Window *webview.Protected
	server *ownedServer
}

// newTrustedWindow は新しいloopback serverと保護windowを作る。
func newTrustedWindow(debug bool, handler http.Handler, settings guilib.WindowSettings) (*trustedWindow, error) {
	server, err := startOwnedServer(handler)
	if err != nil {
		return nil, err
	}
	tw, err := newTrustedWindowWithServer(debug, server, settings)
	if err != nil {
		server.Close()
		return nil, err
	}
	return tw, nil
}

// newTrustedWindowWithServer は既存serverのoriginで保護windowを作る。
func newTrustedWindowWithServer(debug bool, server *ownedServer, settings guilib.WindowSettings) (*trustedWindow, error) {
	w, err := webview.NewProtected(debug, webview.TrustSpec{Origin: server.origin})
	if err != nil {
		return nil, err
	}
	if err := applyWindowSettings(w, settings, false); err != nil {
		w.Destroy()
		return nil, err
	}
	// ドロップ経路も保護windowだけに登録する。失敗はwindow破棄へ倒す。
	bindFileDrop(w)
	if err := w.BindErr(); err != nil {
		w.Destroy()
		return nil, err
	}
	return &trustedWindow{Window: w, server: server}, nil
}

// Close はnative windowを先に破棄し、callbackが止まってからportを返す。
func (t *trustedWindow) Close() {
	if t == nil {
		return
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

// onePageHandler は生成HTMLを1ページだけ配信する。
func onePageHandler(html string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	})
}
