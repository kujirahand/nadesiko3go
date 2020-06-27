package value

const gabadgeMaxValue = 1024 * 1024

// GabadgeMan : ゴミ集め
type GabadgeMan struct {
	valueList *TArray
}

// GetGabadgeMan : ゴミ集め用オブジェクトを返す
func GetGabadgeMan() *GabadgeMan {
	p := GabadgeMan {
		valueList: NewTArray(),
	}
	return &p
}

// NewValue : 新規Valueを返す
func (p *GabadgeMan)NewValue() *Value {
	v := p.valueList.Pop()
	if v == nil {
		v = &Value{
			Type: Null,
			Value: nil,
			Tag: 0,
			IsConst: false,
		}
	}
	return v
}

// AddValue : ゴミ集めにオブジェクトを返す
func (p *GabadgeMan)AddValue(v *Value) {
	if p.valueList.Length() > gabadgeMaxValue {
		v = nil
		return // ゴミ集めしない
	}
	p.valueList.Append(v)
}

// Free : ゴミ集めにオブジェクトを返す
func Free(v *Value) {
	if v == nil {
		return
	}
	ga := GetGabadgeMan()
	ga.AddValue(v)
}


