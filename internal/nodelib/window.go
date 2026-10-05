package nodelib

import (
	"fmt"
	"math"
	"strings"

	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

// @ウィンドウ操作

// windowInfo は外部アプリのウィンドウ1つ分の情報。
type windowInfo struct {
	Handle int64  // OSごとのウィンドウ識別子（Windows: HWND / Linux: X11のウィンドウID / macOS: pid*1000+番号）
	Title  string // ウィンドウのタイトル
}

// windowDriver はOS別のウィンドウ操作の実体。テストで差し替えられるよう変数にしている。
type windowDriver interface {
	List() ([]windowInfo, error)
	Activate(h int64) error
	Move(h int64, x, y int) error
	Size(h int64) (w, hgt int, err error)
	Resize(h int64, w, hgt int) error
}

var osWindow windowDriver = systemWindowDriver{}

// findWindowHandle はタイトルが一致するウィンドウのハンドルを返す。
// 完全一致を優先し、なければ部分一致の最初のものを返す。見つからなければ0。
func findWindowHandle(list []windowInfo, title string) int64 {
	if title == "" {
		return 0
	}
	for _, w := range list {
		if w.Title == title {
			return w.Handle
		}
	}
	for _, w := range list {
		if strings.Contains(w.Title, title) {
			return w.Handle
		}
	}
	return 0
}

// pairArg は [X,Y] や [W,H] の配列引数を2つの整数にする。
func pairArg(cmd string, v value.Value) (int, int, error) {
	arr, ok := v.Array()
	if !ok || arr.Len() < 2 {
		return 0, 0, fmt.Errorf("%s: 引数は[X,Y]のように2要素の配列で指定してください", cmd)
	}
	a, b := value.ToNumber(arr.Get(0)), value.ToNumber(arr.Get(1))
	if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
		return 0, 0, fmt.Errorf("%s: 配列の要素は数値で指定してください", cmd)
	}
	return int(math.Round(a)), int(math.Round(b)), nil
}

// handleArg はウィンドウハンドルの引数を検証して返す。
func handleArg(cmd string, a []value.Value) (int64, error) {
	n := value.ToNumber(argAt(a, 0))
	if math.IsNaN(n) || n <= 0 {
		return 0, fmt.Errorf("%s: ウィンドウハンドルが不正です（『窓ハンドル検索』で取得してください）", cmd)
	}
	return int64(n), nil
}

// windowCommands はウィンドウ操作の命令群。実体はOS別の window_*.go にある。
func windowCommands(m map[string]command) {
	m["窓ハンドル検索"] = command{ // @タイトルSのウィンドウを検索してハンドルを返す(見つからなければ0) // @まどはんどるけんさく
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			list, err := osWindow.List()
			if err != nil {
				return value.Undefined(), fmt.Errorf("窓ハンドル検索: %w", err)
			}
			return value.Number(float64(findWindowHandle(list, str(a, 0)))), nil
		},
	}
	m["窓アクティブ"] = command{ // @ウィンドウハンドルHのウィンドウをアクティブにする // @まどあくてぃぶ
		josi:       [][]string{{"の", "を"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			h, err := handleArg("窓アクティブ", a)
			if err != nil {
				return value.Undefined(), err
			}
			if err := osWindow.Activate(h); err != nil {
				return value.Undefined(), fmt.Errorf("窓アクティブ: %w", err)
			}
			return value.Undefined(), nil
		},
	}
	m["窓位置移動"] = command{ // @ウィンドウハンドルHのウィンドウを配列[X,Y]に移動する // @まどいちいどう
		josi:       [][]string{{"を", "の"}, {"に", "へ"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			h, err := handleArg("窓位置移動", a)
			if err != nil {
				return value.Undefined(), err
			}
			x, y, err := pairArg("窓位置移動", argAt(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			if err := osWindow.Move(h, x, y); err != nil {
				return value.Undefined(), fmt.Errorf("窓位置移動: %w", err)
			}
			return value.Undefined(), nil
		},
	}
	m["窓ハンドルサイズ取得"] = command{ // @ウィンドウハンドルHのウィンドウのサイズを配列[W,H]で返す // @まどはんどるさいずしゅとく
		josi: [][]string{{"の", "を"}},
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			h, err := handleArg("窓ハンドルサイズ取得", a)
			if err != nil {
				return value.Undefined(), err
			}
			w, hgt, err := osWindow.Size(h)
			if err != nil {
				return value.Undefined(), fmt.Errorf("窓ハンドルサイズ取得: %w", err)
			}
			return value.ArrayValue(value.NewArray(value.Number(float64(w)), value.Number(float64(hgt)))), nil
		},
	}
	m["窓ハンドルサイズ設定"] = command{ // @ウィンドウハンドルHのウィンドウのサイズを配列[W,H]に変更する // @まどはんどるさいずせってい
		josi:       [][]string{{"を", "の"}, {"に", "へ"}},
		returnNone: true,
		fn: func(_ stdlib.Context, a []value.Value) (value.Value, error) {
			h, err := handleArg("窓ハンドルサイズ設定", a)
			if err != nil {
				return value.Undefined(), err
			}
			w, hgt, err := pairArg("窓ハンドルサイズ設定", argAt(a, 1))
			if err != nil {
				return value.Undefined(), err
			}
			if w <= 0 || hgt <= 0 {
				return value.Undefined(), fmt.Errorf("窓ハンドルサイズ設定: 幅と高さは1以上で指定してください")
			}
			if err := osWindow.Resize(h, w, hgt); err != nil {
				return value.Undefined(), fmt.Errorf("窓ハンドルサイズ設定: %w", err)
			}
			return value.Undefined(), nil
		},
	}
	m["窓列挙"] = command{ // @ウィンドウの一覧を[{ハンドル,タイトル}]の配列で返す // @まどれっきょ
		fn: func(_ stdlib.Context, _ []value.Value) (value.Value, error) {
			list, err := osWindow.List()
			if err != nil {
				return value.Undefined(), fmt.Errorf("窓列挙: %w", err)
			}
			items := make([]value.Value, len(list))
			for i, w := range list {
				d := value.NewDict()
				d.Set("ハンドル", value.Number(float64(w.Handle)))
				d.Set("タイトル", value.String(w.Title))
				items[i] = value.DictValue(d)
			}
			return value.ArrayValue(value.NewArray(items...)), nil
		},
	}
}
