package guilib

import (
	"strings"
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

type fakeWindowController struct {
	handle   int
	settings WindowSettings
	info     WindowInfo
}

func (f *fakeWindowController) Change(handle int, settings WindowSettings) error {
	f.handle = handle
	f.settings = settings
	return nil
}

func (f *fakeWindowController) Info(handle int) (WindowInfo, error) {
	f.handle = handle
	return f.info, nil
}

func TestWindowCommandsAndMotherConstant(t *testing.T) {
	controller := &fakeWindowController{info: WindowInfo{
		Width: 640, Height: 480, X: -100, Y: 20,
		State: "通常", Title: "テスト", Resizable: true,
	}}
	plugin := NewWithScreenAndWindow(NewScreen(), controller)
	funcs := plugin.FuncList()
	if mother := funcs["母艦"]; mother == nil || mother.Type != "const" || mother.Value != MotherWindowHandle {
		t.Fatalf("母艦定数が正しく登録されていません: %#v", mother)
	}
	if funcs["ウィンドウ変更"] == nil || funcs["ウィンドウ取得"] == nil {
		t.Fatal("ウィンドウ操作命令が登録されていません")
	}

	dict := value.NewDict()
	dict.Set("サイズ", value.ArrayValue(value.NewArray(value.Number(640), value.Number(480))))
	dict.Set("位置", value.String("中央"))
	dict.Set("状態", value.String("最大化"))
	dict.Set("タイトル", value.String("アプリ"))
	dict.Set("サイズ変更可", value.Bool(false))
	if _, err := plugin.Impls()["ウィンドウ変更"](nil, []value.Value{
		value.DictValue(dict), value.Number(MotherWindowHandle),
	}); err != nil {
		t.Fatalf("ウィンドウ変更に失敗しました: %v", err)
	}
	if controller.handle != MotherWindowHandle || !controller.settings.HasSize || controller.settings.Width != 640 || controller.settings.Height != 480 {
		t.Fatalf("サイズ設定が渡されていません: %#v", controller.settings)
	}
	if !controller.settings.HasPosition || !controller.settings.Center || controller.settings.State != "最大化" {
		t.Fatalf("位置または状態が渡されていません: %#v", controller.settings)
	}
	if !controller.settings.HasTitle || controller.settings.Title != "アプリ" || !controller.settings.HasResizable || controller.settings.Resizable {
		t.Fatalf("タイトルまたはサイズ変更可が渡されていません: %#v", controller.settings)
	}

	got, err := plugin.Impls()["ウィンドウ取得"](nil, []value.Value{value.Number(MotherWindowHandle)})
	if err != nil {
		t.Fatalf("ウィンドウ取得に失敗しました: %v", err)
	}
	gotDict, ok := got.Dict()
	if !ok {
		t.Fatalf("戻り値が辞書ではありません: %v", got)
	}
	if state, _ := gotDict.Get("状態"); value.ToString(state) != "通常" {
		t.Fatalf("状態が違います: %s", value.ToString(state))
	}
	if pos, _ := gotDict.Get("位置"); value.ToString(pos) == "" {
		t.Fatal("位置が返されていません")
	}
}

func TestDecodeWindowSettings(t *testing.T) {
	settings, err := DecodeWindowSettings([]byte(`{
		"サイズ":[640,480], "位置":[-20,30], "状態":"通常",
		"タイトル":"見本", "サイズ変更可":false
	}`))
	if err != nil {
		t.Fatalf("index.jsonを解釈できません: %v", err)
	}
	if !settings.HasSize || settings.Width != 640 || settings.Height != 480 || settings.X != -20 || settings.Y != 30 {
		t.Fatalf("設定値が違います: %#v", settings)
	}
	if !settings.HasResizable || settings.Resizable {
		t.Fatalf("サイズ変更可が違います: %#v", settings)
	}
}

func TestDecodeWindowSettingsRejectsInvalidValues(t *testing.T) {
	for _, source := range []string{
		`{"サイズ":[0,480]}`,
		`{"サイズ":[2147483648,480]}`,
		`{"位置":"右上"}`,
		`{"位置":[-2147483649,0]}`,
		`{"状態":"閉じる"}`,
		`{"サイズ変更可":"false"}`,
		`{"サイズ変更可":1}`,
	} {
		if _, err := DecodeWindowSettings([]byte(source)); err == nil {
			t.Fatalf("不正な設定を受理しました: %s", source)
		}
	}

	plugin := NewWithScreen(NewScreen())
	dict := value.NewDict()
	dict.Set("サイズ", value.ArrayValue(value.NewArray(value.Number(640), value.Number(480))))
	_, err := plugin.Impls()["ウィンドウ変更"](nil, []value.Value{value.DictValue(dict), value.Number(0)})
	if err == nil || !strings.Contains(err.Error(), "gonako-gui") {
		t.Fatalf("ウィンドウ未接続時のエラーが違います: %v", err)
	}
}

func TestDecodeWindowSettingsAcceptsNativeIntegerLimits(t *testing.T) {
	settings, err := DecodeWindowSettings([]byte(`{
		"サイズ":[2147483647,1],
		"位置":[-2147483648,2147483647],
		"サイズ変更可":true
	}`))
	if err != nil {
		t.Fatalf("ネイティブ整数の境界値を受理できません: %v", err)
	}
	if settings.Width != maxNativeWindowInt || settings.X != minNativeWindowInt || settings.Y != maxNativeWindowInt {
		t.Fatalf("境界値が変わっています: %#v", settings)
	}
}

func TestWindowCommandsRejectOutOfRangeHandle(t *testing.T) {
	plugin := NewWithScreenAndWindow(NewScreen(), &fakeWindowController{})
	if _, err := plugin.Impls()["ウィンドウ取得"](nil, []value.Value{value.Number(2147483648)}); err == nil {
		t.Fatal("32ビット範囲外のウィンドウハンドルを受理しました")
	}
}
