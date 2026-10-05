package nodelib

import (
	"testing"

	"github.com/kujirahand/nadesiko3go/internal/parser"
	"github.com/kujirahand/nadesiko3go/internal/stdlib"
	"github.com/kujirahand/nadesiko3go/internal/value"
)

type fakeWindowDriver struct {
	list      []windowInfo
	activated int64
	movedTo   [2]int
	w, h      int
	resizedTo [2]int
}

func (f *fakeWindowDriver) List() ([]windowInfo, error) { return f.list, nil }
func (f *fakeWindowDriver) Activate(h int64) error      { f.activated = h; return nil }
func (f *fakeWindowDriver) Move(_ int64, x, y int) error {
	f.movedTo = [2]int{x, y}
	return nil
}
func (f *fakeWindowDriver) Size(int64) (int, int, error) { return f.w, f.h, nil }
func (f *fakeWindowDriver) Resize(_ int64, w, h int) error {
	f.resizedTo = [2]int{w, h}
	return nil
}

func TestWindowCommands(t *testing.T) {
	fake := &fakeWindowDriver{
		list: []windowInfo{{10, "メモ帳"}, {20, "無題 - メモ帳"}, {30, "電卓"}},
		w:    640, h: 480,
	}
	prev := osWindow
	osWindow = fake
	t.Cleanup(func() { osWindow = prev })

	impls := New().Impls()
	num := func(f float64) value.Value { return value.Number(f) }

	if got, _ := impls["窓ハンドル検索"](nil, []value.Value{value.String("メモ帳")}); value.ToNumber(got) != 10 {
		t.Errorf("完全一致 = %v", value.ToNumber(got))
	}
	if got, _ := impls["窓ハンドル検索"](nil, []value.Value{value.String("無題")}); value.ToNumber(got) != 20 {
		t.Errorf("部分一致 = %v", value.ToNumber(got))
	}
	if got, _ := impls["窓ハンドル検索"](nil, []value.Value{value.String("なし")}); value.ToNumber(got) != 0 {
		t.Errorf("未検出 = %v", value.ToNumber(got))
	}

	if _, err := impls["窓アクティブ"](nil, []value.Value{num(30)}); err != nil || fake.activated != 30 {
		t.Errorf("窓アクティブ: err=%v activated=%d", err, fake.activated)
	}
	if _, err := impls["窓アクティブ"](nil, []value.Value{num(0)}); err == nil {
		t.Error("ハンドル0はエラーになるべき")
	}

	pos := value.ArrayValue(value.NewArray(num(100), num(200)))
	if _, err := impls["窓位置移動"](nil, []value.Value{num(10), pos}); err != nil || fake.movedTo != [2]int{100, 200} {
		t.Errorf("窓位置移動: err=%v moved=%v", err, fake.movedTo)
	}
	if _, err := impls["窓位置移動"](nil, []value.Value{num(10), num(1)}); err == nil {
		t.Error("配列でない座標はエラーになるべき")
	}

	got, err := impls["窓ハンドルサイズ取得"](nil, []value.Value{num(10)})
	if err != nil {
		t.Fatal(err)
	}
	arr, _ := got.Array()
	if arr.Len() != 2 || value.ToNumber(arr.Get(0)) != 640 || value.ToNumber(arr.Get(1)) != 480 {
		t.Errorf("サイズ取得 = %v", arr.Values())
	}

	size := value.ArrayValue(value.NewArray(num(800), num(600)))
	if _, err := impls["窓ハンドルサイズ設定"](nil, []value.Value{num(10), size}); err != nil || fake.resizedTo != [2]int{800, 600} {
		t.Errorf("サイズ設定: err=%v resized=%v", err, fake.resizedTo)
	}
	zero := value.ArrayValue(value.NewArray(num(0), num(600)))
	if _, err := impls["窓ハンドルサイズ設定"](nil, []value.Value{num(10), zero}); err == nil {
		t.Error("幅0はエラーになるべき")
	}

	got, _ = impls["窓列挙"](nil, nil)
	list, _ := got.Array()
	if list.Len() != 3 {
		t.Fatalf("列挙の件数 = %d", list.Len())
	}
	d, _ := list.Get(1).Dict()
	title, _ := d.Get("タイトル")
	handle, _ := d.Get("ハンドル")
	if value.ToString(title) != "無題 - メモ帳" || value.ToNumber(handle) != 20 {
		t.Errorf("列挙の要素 = %v", d)
	}
}

func TestWindowCommandSyntax(t *testing.T) {
	funcs := stdlib.NewRegistry(New()).FuncList()
	for _, code := range []string{
		`H=「メモ帳」の窓ハンドル検索`,
		`Hの窓アクティブ`,
		`HをAに窓位置移動`,
		`S=Hの窓ハンドルサイズ取得`,
		`HをAに窓ハンドルサイズ設定`,
		`L=窓列挙`,
	} {
		if _, err := parser.ParseSource(code, "main.nako3", funcs); err != nil {
			t.Errorf("%qを解析できません: %v", code, err)
		}
	}
}
