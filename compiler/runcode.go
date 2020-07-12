package compiler

import "github.com/kujirahand/nadesiko3go/value"

// TCodeFunc : バイトコードの実行関数
type TCodeFunc = func(p *TCompiler, code *TCode) (*value.Value, error)

var codeFuncTable = []TCodeFunc{
	// OPCODE
	rcNOP,
	rcFileInfo,
	rcMoveR,
	rcConstO,
	rcExString,
	rcSetGlobal,
	rcGetGlobal,
	rcSetLocal,
	rcGetLocal,
	rcFindVar,
	rcSetSore,
	rcIncReg,
	rcIncLocal,
	rcDecLocal,
	rcGetLocalNAdd,
	rcGetLocalNSub,
	rcJump,
	rcJumpTo,
	rcJumpIfTrue,
	rcJumpLabel,
	rcJumpLabelIfTrue,
	rcDefLabel,
	rcAdd,
	rcSub,
	rcMul,
	rcDiv,
	rcMod,
	rcGt,
	rcGtEq,
	rcLt,
	rcLtEq,
	rcEqEq,
	rcNtEq,
	rcExp,
	rcAnd,
	rcOr,
	rcNotReg,
	rcLength,
	rcNewArray,
	rcSetArrayElem,
	rcAppendArray,
	rcGetArrayElem,
	rcGetArrayElemI,
	rcNewHash,
	rcSetHash,
	rcForeach,
	rcCallFunc,
	rcCallUserFunc,
	rcReturn,
	rcPrint,
}

// NOP : 何もしない
func rcNOP(p *TCompiler, code *TCode) (*value.Value, error) {
	return nil, nil
}

// FileInfo A, B : fileno:A, lineno:B
func rcFileInfo(p *TCompiler, code *TCode) (*value.Value, error) {
	p.FileNo = code.A
	p.Line = code.B
	return nil, nil
}

// MoveR A,B : R[A] = R[B]
func rcMoveR(p *TCompiler, code *TCode) (*value.Value, error) {
	p.regSet(code.A, p.regGet(code.B))
	return nil, nil
}

// ConstO A,B : R[A] = CONSTS[B]
func rcConstO(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.Consts.Get(code.B).Clone()
	p.regSet(code.A, v)
	return v, nil
}

// ExString A,B : R[A] = ExString(CONSTS[B])
func rcExString(p *TCompiler, code *TCode) (*value.Value, error) {
	p.regSet(code.A, p.runExString(p.Consts.Get(code.B).ToString()))
	return nil, nil
}

// SetGlobal A,B : Vars[ CONSTS[A] ] = R[B]
func rcSetGlobal(p *TCompiler, code *TCode) (*value.Value, error) {
	g := p.sys.Scopes.GetGlobal()
	varV := g.GetByIndex(code.A)
	varV.SetValue(p.regGet(code.B))
	return varV, nil
}

// GetGlobal A,B : R[A] = scope.values[ CONSTS[B] ]
func rcGetGlobal(p *TCompiler, code *TCode) (*value.Value, error) {
	g := p.sys.Scopes.GetGlobal()
	p.regSet(code.A, g.GetByIndex(code.B))
	return p.regGet(code.A), nil
}

// SetLocal A, B: scope.values[A] = R[B]
func rcSetLocal(p *TCompiler, code *TCode) (*value.Value, error) {
	varV := p.scope.GetByIndex(code.A)
	if varV == nil { // はじめての代入なら値を生成
		varV = value.NewNullPtr()
		p.scope.SetByIndex(code.A, varV)
	}
	valV := p.regGet(code.B)
	varV.SetValue(valV)
	// println("SetLocal=@", valV.ToJSONString())
	// fmt.Printf("%#v\n", valV)
	return varV, nil
}

// GetLocal A,B : R[A] = Scope[B]
func rcGetLocal(p *TCompiler, code *TCode) (*value.Value, error) {
	p.regSet(code.A, p.scope.GetByIndex(code.B))
	lastValue := p.regGet(code.A)
	// println("@@value=", lastValue.ToJSONString())
	return lastValue, nil
}

// FindVar A, B : R[A] = FindVar(CONSTS[B])
func rcFindVar(p *TCompiler, code *TCode) (*value.Value, error) {
	name := p.Consts.Get(code.B).ToString()
	v, _ := p.sys.Scopes.Find(name)
	p.regSet(code.A, v)
	return v, nil
}

// SetSore A : Sore = R[A]
func rcSetSore(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.regGet(code.A)
	p.scope.Set("それ", v)
	return v, nil
}

// IncReg A : R[A]++
func rcIncReg(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.NewIntPtr(p.regGet(code.A).ToInt() + 1)
	p.regSet(code.A, v)
	return v, nil
}

// IncLocal A,B : Scope[A]+=B
func rcIncLocal(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.scope.GetByIndex(code.A)
	v.SetInt(v.ToInt() + code.B)
	return v, nil
}

// DecLocal A,B : Scope[A]-=B
func rcDecLocal(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.scope.GetByIndex(code.A)
	v.SetInt(v.ToInt() - code.B)
	return v, nil
}

// Jump A : PC += A
func rcJump(p *TCompiler, code *TCode) (*value.Value, error) {
	p.move(code.A)
	p.isJump = true
	return nil, nil
}

// JumpTo A :  PC = A
func rcJumpTo(p *TCompiler, code *TCode) (*value.Value, error) {
	// 事前にラベルをJUMPに変換している
	return nil, p.RuntimeError("システムエラー: ラベル未解決")
}

// JumpIfTrue A, B: if A then PC += B
func rcJumpIfTrue(p *TCompiler, code *TCode) (*value.Value, error) {
	expr := p.regGet(code.A)
	if expr != nil && expr.ToBool() {
		p.move(code.B)
		p.isJump = true
	}
	return nil, nil
}

// JumpLabel A : PC = LABELS[A]
func rcJumpLabel(p *TCompiler, code *TCode) (*value.Value, error) {
	// 事前にラベルをJUMPに変換している
	return nil, p.RuntimeError("システムエラー: ラベル未解決")
}

// JumpLabelIfTrue A B : if A then PC = LABELS[B]
func rcJumpLabelIfTrue(p *TCompiler, code *TCode) (*value.Value, error) {
	// 事前にJumpIfTrueに変換している
	return nil, p.RuntimeError("システムエラー: ラベル未解決")
}

// DefLabel A : LABELS[A] = addr
func rcDefLabel(p *TCompiler, code *TCode) (*value.Value, error) {
	return nil, nil
}

// Add A,B,C : R[A] = R[B] + R[C]
func rcAdd(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Add(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Sub A,B,C : R[A] = R[B] - R[C]
func rcSub(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Sub(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Mul A,B,C : R[A] = R[B] * R[C]
func rcMul(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Mul(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Div A,B,C : R[A] = R[B] * R[C]
func rcDiv(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Div(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Mod A,B,C : R[A] = R[B] % R[C]
func rcMod(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Mod(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Gt A,B,C : R[A] = R[B] > R[C]
func rcGt(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Gt(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// GtEq A,B,C : R[A] = R[B] >= R[C]
func rcGtEq(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.GtEq(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Lt A,B,C : R[A] = R[B] < R[C]
func rcLt(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Lt(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// LtEq A,B,C : R[A] = R[B] <= R[C]
func rcLtEq(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.LtEq(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// EqEq A,B,C : R[A] = R[B] == R[C]
func rcEqEq(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.EqEq(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// NtEq A,B,C : R[A] = R[B] != R[C]
func rcNtEq(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.NtEq(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Exp A,B,C : R[A] = R[B] ^ R[C]
func rcExp(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Exp(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// And A,B,C : R[A] = R[B] && R[C]
func rcAnd(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.And(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// Or A,B,C : R[A] = R[B] || R[C]
func rcOr(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Or(p.regGet(code.B), p.regGet(code.C))
	p.regSet(code.A, v)
	return v, nil
}

// NotReg A : R[A] = !R[A]
func rcNotReg(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.Not(p.regGet(code.A))
	p.regSet(code.A, v)
	return v, nil
}

// Length A B : R[A] = Len(R[B])
func rcLength(p *TCompiler, code *TCode) (*value.Value, error) {
	vb := p.regGet(code.B)
	va := value.NewIntPtr(vb.Length())
	p.regSet(code.A, va)
	return va, nil
}

// NewArray A : R[A] = NewArray
func rcNewArray(p *TCompiler, code *TCode) (*value.Value, error) {
	a := value.NewArrayPtr()
	p.regSet(code.A, a)
	return a, nil
}

// SetArrayElem A B : R[A].Value = R[B]
func rcSetArrayElem(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.regGet(code.A)
	if v != nil {
		v.SetValue(p.regGet(code.B))
		return v, nil
	}
	return nil, nil
}

// AppendArray A B : (R[A]).append(R[B])
func rcAppendArray(p *TCompiler, code *TCode) (*value.Value, error) {
	a := p.regGet(code.A)
	b := p.regGet(code.B)
	a.Append(b)
	return nil, nil
}

// GetArrayElem A B C : R[A] = (R[B])[ R[C] ]
func rcGetArrayElem(p *TCompiler, code *TCode) (*value.Value, error) {
	var v *value.Value = nil
	b := p.regGet(code.B)
	c := p.regGet(code.C)
	if b.Type == value.Array {
		idx := c.ToInt()
		v = b.ArrayGet(idx)
		if v == nil { // 値がなければ作る
			v = value.NewNullPtr()
			b.ArraySet(idx, v)
		}
		p.regSet(code.A, v)
	} else if b.Type == value.Hash {
		v = b.HashGet(c.ToString())
		p.regSet(code.A, v)
	}
	return v, nil
}

// GetArrayElemI A B C : R[A] = (R[B])[ C ]
func rcGetArrayElemI(p *TCompiler, code *TCode) (*value.Value, error) {
	var v *value.Value = nil
	rb := p.regGet(code.B)
	if rb.Type == value.Array {
		v := rb.ArrayGet(code.C)
		p.regSet(code.A, v)
	}
	return v, nil
}

// NewHash A : R[A] = NewHash
func rcNewHash(p *TCompiler, code *TCode) (*value.Value, error) {
	v := value.NewHashPtr()
	p.regSet(code.A, v)
	return v, nil
}

// SetHash A B C : (R[A]).HashSet(CONSTS[B], C)
func rcSetHash(p *TCompiler, code *TCode) (*value.Value, error) {
	h := p.regGet(code.A)
	key := p.Consts.Get(code.B).ToString()
	if h != nil {
		h.HashSet(key, p.regGet(code.C))
	}
	// println(h.ToJSONString())
	return h, nil
}

// Foreach A B C : FOREACH isContinue:A expr:B counter:C -> それ|対象|対象キーの値を更新
func rcForeach(p *TCompiler, code *TCode) (*value.Value, error) {
	return p.runForeach(code)
}

// CallFunc A B C : R[A] = call(fn=CONSTS[B], argStart=R[C])
func rcCallFunc(p *TCompiler, code *TCode) (*value.Value, error) {
	res, err := p.runCallFunc(code)
	if err != nil {
		return nil, err
	}
	p.regSet(code.A, res)
	// println("call=", res.ToString())
	return res, nil
}

// CallUserFunc A B C : R[A] = call(fn=LABELS[B], argStart=R[C])
func rcCallUserFunc(p *TCompiler, code *TCode) (*value.Value, error) {
	cur := p.procCallUserFunc(code)
	p.moveTo(cur)
	p.isJump = true
	return nil, nil
}

// Retruen A : return R[A]
func rcReturn(p *TCompiler, code *TCode) (*value.Value, error) {
	cur, ret := p.procReturn(code)
	p.moveTo(cur)
	p.isJump = true
	return ret, nil
}

// Print A : PRINT R[A] for DEBUG
func rcPrint(p *TCompiler, code *TCode) (*value.Value, error) {
	println("[PRINT]", p.regGet(code.A).ToString())
	return nil, nil
}

// GetLocalNAddInt A, B, C : R[A] = Scope[B] + C
func rcGetLocalNAdd(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.scope.GetByIndex(code.B)
	v2 := value.Add(v, value.NewIntPtr(code.C))
	p.regSet(code.A, v2)
	return v2, nil
}

// GetLocalNSubInt A, B, C : R[A] = Scope[B] - C
func rcGetLocalNSub(p *TCompiler, code *TCode) (*value.Value, error) {
	v := p.scope.GetByIndex(code.B)
	v2 := value.Sub(v, value.NewIntPtr(code.C))
	p.regSet(code.A, v2)
	return v2, nil
}
