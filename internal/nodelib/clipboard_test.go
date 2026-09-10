package nodelib

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"golang.design/x/clipboard"
)

type fakeClipboardDriver struct {
	initErr  error
	readData []byte
	readErr  error
	writeErr error
	written  []byte
}

func (f *fakeClipboardDriver) Init() error {
	return f.initErr
}

func (f *fakeClipboardDriver) Read(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error) {
	return f.readData, f.readErr
}

func (f *fakeClipboardDriver) Write(_ context.Context, _ clipboard.Format, data []byte, _ ...clipboard.Option) (<-chan struct{}, error) {
	f.written = append([]byte(nil), data...)
	return nil, f.writeErr
}

func useFakeClipboard(t *testing.T, fake *fakeClipboardDriver) {
	t.Helper()
	previous := osClipboard
	osClipboard = fake
	t.Cleanup(func() { osClipboard = previous })
}

func TestClipboardCommands(t *testing.T) {
	fake := &fakeClipboardDriver{readData: []byte("こんにちは🌸")}
	useFakeClipboard(t, fake)

	list := New().FuncList()
	change := list["クリップボード変更"]
	if change == nil {
		t.Fatal("『クリップボード変更』が登録されていません")
	}
	if !change.ReturnNone {
		t.Error("『クリップボード変更』が戻り値なしになっていません")
	}
	if len(change.Josi) != 1 || strings.Join(change.Josi[0], ",") != "へ,に" {
		t.Errorf("『クリップボード変更』の助詞 = %#v", change.Josi)
	}

	impls := New().Impls()
	got, err := impls["クリップボード取得"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.ToString(got) != "こんにちは🌸" {
		t.Errorf("取得結果 = %q", value.ToString(got))
	}

	got, err = impls["クリップボード変更"](nil, []value.Value{value.String("変更後📝")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind() != value.KindUndefined {
		t.Errorf("変更結果の種類 = %v", got.Kind())
	}
	if string(fake.written) != "変更後📝" {
		t.Errorf("クリップボードへ書いた値 = %q", fake.written)
	}
}

func TestClipboardChangeSyntax(t *testing.T) {
	funcs := stdlib.NewRegistry(New()).FuncList()
	for _, code := range []string{
		`「値」へクリップボード変更`,
		`「値」にクリップボード変更`,
	} {
		if _, err := parser.ParseSource(code, "main.nako3", funcs); err != nil {
			t.Errorf("%qを解析できません: %v", code, err)
		}
	}
}

func TestClipboardGetEmpty(t *testing.T) {
	useFakeClipboard(t, &fakeClipboardDriver{readErr: clipboard.ErrNoData})

	got, err := New().Impls()["クリップボード取得"](nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.ToString(got) != "" {
		t.Errorf("空のクリップボード = %q", value.ToString(got))
	}
}

func TestClipboardErrors(t *testing.T) {
	t.Run("初期化", func(t *testing.T) {
		useFakeClipboard(t, &fakeClipboardDriver{initErr: errors.New("利用不可")})
		_, err := New().Impls()["クリップボード取得"](nil, nil)
		if err == nil || !strings.Contains(err.Error(), "クリップボードを初期化できません") {
			t.Fatalf("エラー = %v", err)
		}
	})

	t.Run("取得", func(t *testing.T) {
		useFakeClipboard(t, &fakeClipboardDriver{readErr: errors.New("取得失敗")})
		_, err := New().Impls()["クリップボード取得"](nil, nil)
		if err == nil || !strings.Contains(err.Error(), "クリップボードを取得できません") {
			t.Fatalf("エラー = %v", err)
		}
	})

	t.Run("変更", func(t *testing.T) {
		useFakeClipboard(t, &fakeClipboardDriver{writeErr: errors.New("変更失敗")})
		_, err := New().Impls()["クリップボード変更"](nil, []value.Value{value.String("値")})
		if err == nil || !strings.Contains(err.Error(), "クリップボードを変更できません") {
			t.Fatalf("エラー = %v", err)
		}
	})
}
