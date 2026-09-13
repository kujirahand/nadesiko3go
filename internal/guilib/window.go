package guilib

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/value"
)

const (
	minNativeWindowInt = -1 << 31
	maxNativeWindowInt = 1<<31 - 1
)

// MotherWindowHandle は、実行中のgonako-gui本体のウィンドウを表す。
// OSのポインタをなでしこの値として公開しないため、論理ハンドル0を使う。
const MotherWindowHandle = 0

// WindowSettings は「ウィンドウ変更」とindex.jsonで共用する設定である。
// 各Hasフィールドにより、辞書で省略された項目とゼロ値を区別する。
type WindowSettings struct {
	HasSize      bool
	Width        int
	Height       int
	HasPosition  bool
	Center       bool
	X            int
	Y            int
	HasState     bool
	State        string
	HasTitle     bool
	Title        string
	HasResizable bool
	Resizable    bool
}

// WindowInfo は現在のネイティブウィンドウ情報である。
type WindowInfo struct {
	Width     int
	Height    int
	X         int
	Y         int
	State     string
	Title     string
	Resizable bool
}

// WindowController はVMとネイティブウィンドウの境界である。
// 実装側は、必要な処理をWebViewのUIスレッドへ送る責任を持つ。
type WindowController interface {
	Change(handle int, settings WindowSettings) error
	Info(handle int) (WindowInfo, error)
}

func normalizeWindowState(state string) (string, error) {
	state = strings.TrimSpace(state)
	switch state {
	case "通常", "最大化", "最小化", "全画面":
		return state, nil
	default:
		return "", fmt.Errorf("ウィンドウ状態『%s』は指定できません（通常・最大化・最小化・全画面から選んでください）", state)
	}
}

func positiveWindowDimension(number float64, name string) (int, error) {
	if math.IsNaN(number) || math.IsInf(number, 0) || number <= 0 || number > maxNativeWindowInt || number != math.Trunc(number) {
		return 0, fmt.Errorf("ウィンドウの%sには1から%dまでの整数を指定してください", name, maxNativeWindowInt)
	}
	return int(number), nil
}

func integerWindowCoordinate(number float64, name string) (int, error) {
	if math.IsNaN(number) || math.IsInf(number, 0) || number < minNativeWindowInt || number > maxNativeWindowInt || number != math.Trunc(number) {
		return 0, fmt.Errorf("ウィンドウの%sには%dから%dまでの整数を指定してください", name, minNativeWindowInt, maxNativeWindowInt)
	}
	return int(number), nil
}

// ParseWindowSettings は、なでしこの設定辞書を共通表現へ変換する。
func ParseWindowSettings(v value.Value) (WindowSettings, error) {
	dict, ok := v.Dict()
	if !ok || dict == nil {
		return WindowSettings{}, errors.New("『ウィンドウ変更』の設定には辞書を指定してください")
	}

	var settings WindowSettings
	if item, ok := getWindowSetting(dict, "サイズ", "size"); ok {
		array, ok := item.Array()
		if !ok || array == nil || array.Len() != 2 {
			return WindowSettings{}, errors.New("ウィンドウの『サイズ』には[幅, 高さ]を指定してください")
		}
		width, ok := array.Get(0).Number()
		if !ok {
			return WindowSettings{}, errors.New("ウィンドウの幅には数値を指定してください")
		}
		height, ok := array.Get(1).Number()
		if !ok {
			return WindowSettings{}, errors.New("ウィンドウの高さには数値を指定してください")
		}
		var err error
		settings.Width, err = positiveWindowDimension(width, "幅")
		if err != nil {
			return WindowSettings{}, err
		}
		settings.Height, err = positiveWindowDimension(height, "高さ")
		if err != nil {
			return WindowSettings{}, err
		}
		settings.HasSize = true
	}

	if item, ok := getWindowSetting(dict, "位置", "position"); ok {
		if item.Kind() == value.KindString {
			if value.ToString(item) != "中央" {
				return WindowSettings{}, errors.New("ウィンドウの『位置』には『中央』または[X, Y]を指定してください")
			}
			settings.Center = true
		} else {
			array, ok := item.Array()
			if !ok || array == nil || array.Len() != 2 {
				return WindowSettings{}, errors.New("ウィンドウの『位置』には『中央』または[X, Y]を指定してください")
			}
			x, ok := array.Get(0).Number()
			if !ok {
				return WindowSettings{}, errors.New("ウィンドウのX座標には数値を指定してください")
			}
			y, ok := array.Get(1).Number()
			if !ok {
				return WindowSettings{}, errors.New("ウィンドウのY座標には数値を指定してください")
			}
			var err error
			settings.X, err = integerWindowCoordinate(x, "X座標")
			if err != nil {
				return WindowSettings{}, err
			}
			settings.Y, err = integerWindowCoordinate(y, "Y座標")
			if err != nil {
				return WindowSettings{}, err
			}
		}
		settings.HasPosition = true
	}

	if item, ok := getWindowSetting(dict, "状態", "state"); ok {
		state, err := normalizeWindowState(value.ToString(item))
		if err != nil {
			return WindowSettings{}, err
		}
		settings.HasState = true
		settings.State = state
	}
	if item, ok := getWindowSetting(dict, "タイトル", "title"); ok {
		settings.HasTitle = true
		settings.Title = value.ToString(item)
	}
	if item, ok := getWindowSetting(dict, "サイズ変更可", "resizable"); ok {
		resizable, ok := item.Bool()
		if !ok {
			return WindowSettings{}, errors.New("ウィンドウの『サイズ変更可』には真偽値を指定してください")
		}
		settings.HasResizable = true
		settings.Resizable = resizable
	}
	return settings, nil
}

func getWindowSetting(dict *value.Dict, names ...string) (value.Value, bool) {
	for _, name := range names {
		if item, ok := dict.Get(name); ok {
			return item, true
		}
	}
	return value.Undefined(), false
}

// DecodeWindowSettings はindex.jsonを「ウィンドウ変更」と同じ規則で読む。
func DecodeWindowSettings(data []byte) (WindowSettings, error) {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return WindowSettings{}, fmt.Errorf("index.jsonをJSONとして読めません: %w", err)
	}
	converted, err := jsonWindowValue(raw)
	if err != nil {
		return WindowSettings{}, err
	}
	settings, err := ParseWindowSettings(converted)
	if err != nil {
		return WindowSettings{}, fmt.Errorf("index.jsonのウィンドウ設定が不正です: %w", err)
	}
	return settings, nil
}

func jsonWindowValue(raw any) (value.Value, error) {
	switch item := raw.(type) {
	case nil:
		return value.Null(), nil
	case bool:
		return value.Bool(item), nil
	case float64:
		return value.Number(item), nil
	case string:
		return value.String(item), nil
	case []any:
		items := make([]value.Value, len(item))
		for i, child := range item {
			converted, err := jsonWindowValue(child)
			if err != nil {
				return value.Undefined(), err
			}
			items[i] = converted
		}
		return value.ArrayValue(value.NewArray(items...)), nil
	case map[string]any:
		dict := value.NewDict()
		for key, child := range item {
			converted, err := jsonWindowValue(child)
			if err != nil {
				return value.Undefined(), err
			}
			dict.Set(key, converted)
		}
		return value.DictValue(dict), nil
	default:
		return value.Undefined(), fmt.Errorf("index.jsonに扱えない値があります")
	}
}

func windowInfoValue(info WindowInfo) value.Value {
	dict := value.NewDict()
	dict.Set("サイズ", value.ArrayValue(value.NewArray(value.Number(float64(info.Width)), value.Number(float64(info.Height)))))
	dict.Set("位置", value.ArrayValue(value.NewArray(value.Number(float64(info.X)), value.Number(float64(info.Y)))))
	dict.Set("状態", value.String(info.State))
	dict.Set("タイトル", value.String(info.Title))
	dict.Set("サイズ変更可", value.Bool(info.Resizable))
	return value.DictValue(dict)
}
