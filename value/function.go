package value

// TFunction : 関数型の型
type TFunction func(args *TArray) (*Value, error)

// DefArgs : 関数引数の助詞一覧
type DefArgs [][]string

// TFuncValue : Valueの関数型の型
type TFuncValue struct {
	Name     string
	Args     DefArgs
	Tag      int
	LinkFunc TFunction
	LinkNode interface{}
}

// NewFunc : 関数オブジェクトを生成
func NewFunc(name string, args DefArgs, v TFunction) Value {
	f := TFuncValue{
		Name:     name,
		Args:     args,
		LinkFunc: v,
	}
	return Value{Type: Function, Value: f}
}

// GetFuncLink : Value から TFunction を得る
func GetFuncLink(v *Value) TFunction {
	if v.Type == Function {
		return v.Value.(TFuncValue).LinkFunc
	}
	return nil
}

// NewUserFunc : ユーザー定義関数を生成
func NewUserFunc(name string, args DefArgs, v interface{}) Value {
	f := TFuncValue{
		Name:     name,
		Args:     args,
		LinkNode: v,  // Link to Node.TNodeDefFunc
		Tag:      -1, // ArgsList Link
	}
	return Value{Type: UserFunc, Value: f}
}

// GetUserFuncLink : Value から LinkNode を得る
func GetUserFuncLink(v *Value) interface{} {
	if v.Type == UserFunc {
		return v.Value.(TFuncValue).LinkNode
	}
	return nil
}

// GetFuncTag : Get tag id
func GetFuncTag(v *Value) int {
	if v.Type == UserFunc || v.Type == Function {
		f := v.Value.(TFuncValue)
		return f.Tag
	}
	println("*** ERROR ***")
	panic(-1)
	// return -1
}

// SetFuncTag : set tag
func SetFuncTag(v *Value, tag int) {
	if v.Type == UserFunc || v.Type == Function {
		f := v.Value.(TFuncValue)
		f.Tag = tag
		return
	}
	println("*** ERROR ***", valueTypeStr[v.Type])
	panic(-1)
}
