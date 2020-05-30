package compiler

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/value"
)

const (
	metaRegReturnAddr  = 0
	metaRegReturnValue = 1
)

// RuntimeError : 実行時エラーを生成
func (p *TCompiler) RuntimeError(msg string) error {
	e := fmt.Errorf("[実行時エラー] (%d) %s", p.Line, msg)
	return e
}

// Run : 実行する
func (p *TCompiler) Run() (*value.Value, error) {
	p.moveToTop()
	p.scope = p.sys.Scopes.GetTopScope()
	p.reg = &p.scope.Reg
	return p.runCode()
}

func (p *TCompiler) runCode() (*value.Value, error) {
	var lastValue *value.Value = nil
	for p.isLive() {
		code := p.peek()
		A, B, C := code.A, code.B, code.C
		// println("RUN=", p.index, p.ToString(code))
		switch code.Type {
		case ConstO:
			p.regSet(A, p.Consts[B])
		case MoveR:
			p.regSet(A, p.regGet(B))
		case SetLocal:
			varV := p.scope.GetByIndex(A)
			varV.SetValue(p.regGet(B))
			lastValue = varV
		case GetLocal:
			p.regSet(A, p.scope.GetByIndex(B))
			lastValue = p.regGet(A)
		case SetGlobal:
			g := p.sys.Scopes.GetGlobal()
			varV := g.GetByIndex(A)
			varV.SetValue(p.regGet(B))
			lastValue = varV
		case GetGlobal:
			g := p.sys.Scopes.GetGlobal()
			p.regSet(A, g.GetByIndex(B))
			lastValue = p.regGet(A)
		// Calc
		case Add:
			v := value.Add(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Sub:
			v := value.Sub(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Mul:
			v := value.Mul(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Div:
			v := value.Div(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Mod:
			v := value.Mod(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case EqEq:
			v := value.EqEq(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case NtEq:
			v := value.NtEq(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Gt:
			v := value.Gt(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case GtEq:
			v := value.GtEq(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case Lt:
			v := value.Lt(p.regGet(B), p.regGet(C))
			p.regSet(A, &v)
			lastValue = &v
		case LtEq:
			bv := p.regGet(B)
			cv := p.regGet(C)
			v := value.LtEq(bv, cv)
			p.regSet(A, &v)
			lastValue = &v
		case IncReg:
			v := p.regGet(A)
			v.SetInt(v.ToInt() + 1)
		case IncLocal:
			v := p.scope.GetByIndex(A)
			v.SetInt(v.ToInt() + 1)
		case NotReg:
			p.regGet(A).SetBool(!p.regGet(A).ToBool())
		// label
		case DefLabel:
			//nop
		case Jump:
			p.move(code.A)
			continue
		case JumpIfTrue:
			expr := p.regGet(A)
			if expr != nil && expr.ToBool() {
				p.move(B)
				continue
			}
		case NewArray:
			a := value.NewValueArray()
			p.regSet(A, &a)
		case AppendArray:
			a := p.regGet(A)
			if a.Type != value.Array {
				return nil, p.RuntimeError("[SYSTEM] AppendArray")
			}
			a.ArrayAppend(p.regGet(B))
		case CallFunc:
			res, err := p.runCallFunc(code)
			if err != nil {
				return nil, err
			}
			p.regSet(A, res)
			lastValue = res
		case CallUserFunc:
			cur := p.procUserCallFunc(code)
			p.moveTo(cur)
			continue
		case Return:
			cur, ret := p.procReturn(code)
			if cur < 0 { // プログラム終了
				return lastValue, nil
			}
			p.moveTo(cur)
			lastValue = ret
			continue
		default:
			println("[system error]" + fmt.Sprintf("Undefined code: %s", p.ToString(code)))
		}
		p.moveNext()
	}
	return lastValue, nil
}

func (p *TCompiler) runCallFunc(code *TCode) (*value.Value, error) {
	// get func
	funcV := p.Consts[code.B]
	argV := p.regGet(code.C)
	if funcV.Type == value.UserFunc {
		return nil, p.RuntimeError("[SYSTEM ERROR:ユーザー関数をシステム関数として呼んだ]")
	}
	// args
	args := argV.Value.(value.TArray)
	// call system func
	fn := funcV.Value.(value.TFunction)
	res, err := fn(args)
	if err != nil {
		return nil, p.RuntimeError("関数実行中のエラー。" + err.Error())
	}
	p.regSet(code.A, res)
	p.scope.Set("それ", res)
	return res, nil
}

func (p *TCompiler) procReturn(code *TCode) (int, *value.Value) {
	if p.scope == p.sys.Global {
		// プログラム終了を表す
		return -1, nil
	}
	retValue := p.regGet(code.A)
	retAddr := p.regGet(metaRegReturnAddr).ToInt()
	retReg := p.regGet(metaRegReturnValue).ToInt()
	// Close Scope
	/*
		for i, v := range p.sys.Scopes.Items {
			println("@@[REG]", i, "=", v.Reg.ToJSONString())
		}
	*/
	// println("--- close scope ---")
	p.sys.Scopes.Close()
	p.scope = p.sys.Scopes.GetTopScope()
	p.reg = &(p.scope.Reg)
	// Set Result
	p.regSet(retReg, retValue)
	// println("RETURN,reg=", p.reg.ToJSONString(), "/Back=", retAddr)
	return retAddr, retValue
}

func (p *TCompiler) procUserCallFunc(code *TCode) int {
	// get func
	label := p.Labels[code.B]
	argV := p.regGet(code.C)
	// open scope
	scope := p.sys.Scopes.Open()
	p.scope = scope
	p.reg = &scope.Reg
	// 登録する順番に注意
	scope.Set("それ", value.NewValueNullPtr())
	scope.Reg.Set(metaRegReturnAddr, value.NewValueIntPtr(p.index+1))
	scope.Reg.Set(metaRegReturnValue, value.NewValueIntPtr(code.A))
	// 変数を登録する
	if argV != nil && argV.Type == value.Array {
		args := argV.Value.(value.TArray)
		for i, v := range args {
			name := label.argNames[i]
			scope.Set(name, v)
		}
	}
	cur := label.addr
	return cur
}

// --- index(カーソル)の移動 ---
func (p *TCompiler) peek() *TCode {
	return p.Codes[p.index]
}

func (p *TCompiler) move(n int) {
	p.index += n
}

func (p *TCompiler) moveTo(n int) {
	p.index = n
}

func (p *TCompiler) moveNext() {
	p.index++
}

func (p *TCompiler) isLive() bool {
	return p.index < len(p.Codes)
}

func (p *TCompiler) moveToTop() {
	p.index = 0
	p.length = len(p.Codes)
}

func (p *TCompiler) regSet(index int, val *value.Value) {
	p.reg.Set(index, val)
	// println("[REG]SET = " + p.reg.ToString())
}

func (p *TCompiler) regGet(index int) *value.Value {
	// println("[REG]GET = " + p.reg.ToString())
	return p.reg.Get(index)
}
