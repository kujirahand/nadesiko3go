package nodelib

import (
	"context"
	"errors"
	"fmt"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
	"golang.design/x/clipboard"
)

// clipboardDriver はOSクリップボードをテスト時に差し替えるための境界。
type clipboardDriver interface {
	Init() error
	Read(context.Context, clipboard.Format, ...clipboard.Option) ([]byte, error)
	Write(context.Context, clipboard.Format, []byte, ...clipboard.Option) (<-chan struct{}, error)
}

type systemClipboardDriver struct{}

func (systemClipboardDriver) Init() error {
	return clipboard.Init()
}

func (systemClipboardDriver) Read(ctx context.Context, format clipboard.Format, opts ...clipboard.Option) ([]byte, error) {
	return clipboard.Read(ctx, format, opts...)
}

func (systemClipboardDriver) Write(ctx context.Context, format clipboard.Format, data []byte, opts ...clipboard.Option) (<-chan struct{}, error) {
	return clipboard.Write(ctx, format, data, opts...)
}

var osClipboard clipboardDriver = systemClipboardDriver{}

func clipboardCommands(m map[string]command) {
	m["クリップボード変更"] = command{ // @文字列でOSのクリップボードを変更する // @くりっぷぼーどへんこう
		josi:       [][]string{{"へ", "に"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			if err := osClipboard.Init(); err != nil {
				return value.Undefined(), fmt.Errorf("クリップボードを初期化できません: %w", err)
			}
			if _, err := osClipboard.Write(context.Background(), clipboard.FmtText, []byte(str(a, 0))); err != nil {
				return value.Undefined(), fmt.Errorf("クリップボードを変更できません: %w", err)
			}
			return value.Undefined(), nil
		},
	}

	m["クリップボード取得"] = command{ // @OSのクリップボードの文字列を取得する // @くりっぷぼーどしゅとく
		fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
			if err := osClipboard.Init(); err != nil {
				return value.Undefined(), fmt.Errorf("クリップボードを初期化できません: %w", err)
			}
			data, err := osClipboard.Read(context.Background(), clipboard.FmtText)
			if errors.Is(err, clipboard.ErrNoData) {
				return value.String(""), nil
			}
			if err != nil {
				return value.Undefined(), fmt.Errorf("クリップボードを取得できません: %w", err)
			}
			return value.String(string(data)), nil
		},
	}
}
