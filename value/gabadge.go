package value

//
// (memo) 現状、うまく動いてない。全く使われていない。
//

// gabadgeMaxValue : max value
const gabadgeMaxValue = 1024 * 1024

// GabadgeMan : ゴミ集め
type GabadgeMan struct {
	valueList *TArray
}

// GabadgeMan 唯一のオブジェクト
var objGabadgeMan = &GabadgeMan{
	valueList: NewTArray(),
}

// GetGabadgeMan : ゴミ集め用オブジェクトを返す
func GetGabadgeMan() *GabadgeMan {
	return objGabadgeMan
}

// NewValue : 新規Valueを返す
func (p *GabadgeMan) NewValue() *Value {
	v := p.valueList.Pop()
	if v == nil {
		v = &Value{
			Type:  Null,
			Value: nil,
		}
	} else {
		v.Type = Null
		v.Value = nil
		v.Tag = 0
		v.IsConst = false
	}
	return v
}

// AddValue : ゴミ集めにオブジェクトを返す
func (p *GabadgeMan) AddValue(v *Value) {
	if p.valueList.Length() > gabadgeMaxValue {
		print("[NAKO3 DEBUG MESSAGE] over gabadgeMaxValue")
		v = nil
		return // ゴミ集めしない
	}
	v.Clear()
	p.valueList.Append(v)
}

// Free : ゴミ集めにオブジェクトを返す
func Free(v *Value) {
	if v == nil {
		return
	}
	objGabadgeMan.AddValue(v)
}
