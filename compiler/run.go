package compiler

import (
	"fmt"

	"github.com/kujirahand/nadesiko3go/value"
)

const (
	metaKeyReturnAddr = "__ReturnAddr"
	metaKeyReturnReg  = "__ReturnReg"
)

// RuntimeError : 実行時エラーを生成
func (p *TCompiler) RuntimeError(msg string) error {
	e := fmt.Errorf("[実行時エラー] (%d) %s", p.Line, msg)
	return e
}

// Run : 実行する
func (p *TCompiler) Run() (*value.Value, error) {
	p.moveToTop()
	var lastValue *value.Value = nil
	for p.isLive() {
		code := p.peek()
		A, B, C := code.A, code.B, code.C
		println("RUN=", p.index, p.ToString(code))
		switch code.Type {
		case ConstO:
			p.Reg[A] = p.Consts[B]
			println("ConstO", A, B, "Reg[", A, "]=", p.Reg[A].ToString())
		case MoveR:
			p.Reg[A] = p.Reg[B]
		case SetLocal:
			scope := p.sys.Scopes.GetTopScope()
			varV := scope.GetByIndex(A)
			varV.SetValue(p.Reg[B])
			lastValue = varV
		case GetLocal:
			scope := p.sys.Scopes.GetTopScope()
			p.Reg[A] = scope.GetByIndex(B)
			lastValue = p.Reg[A]
			//println("GetLocal", A, "Reg[A]=", p.Reg[A].ToString())
		case SetGlobal:
			g := p.sys.Scopes.GetGlobal()
			varV := g.GetByIndex(A)
			varV.SetValue(p.Reg[B])
			lastValue = varV
		case GetGlobal:
			g := p.sys.Scopes.GetGlobal()
			p.Reg[A] = g.GetByIndex(B)
			lastValue = p.Reg[A]
		// Calc
		case Add:
			v := value.Add(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
			// println("Add", A, B, C, "Reg[", A, "]=", v.ToString())
		case Sub:
			v := value.Sub(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case Mul:
			v := value.Mul(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
			// println("Mul", A, B, C, "Reg[", A, "]=", v.ToString())
		case Div:
			v := value.Div(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case Mod:
			v := value.Mod(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case EqEq:
			v := value.EqEq(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case NtEq:
			v := value.NtEq(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case Gt:
			v := value.Gt(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
			//println("Gt", A, B, C, "Reg[", A, "]=", v.ToString(), "B=", p.Reg[B].ToString(), "C=", p.Reg[C].ToString())
		case GtEq:
			v := value.GtEq(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case Lt:
			v := value.Lt(p.Reg[B], p.Reg[C])
			p.Reg[A] = &v
			lastValue = &v
		case LtEq:
			bv := p.Reg[B]
			cv := p.Reg[C]
			v := value.LtEq(bv, cv)
			p.Reg[A] = &v
			lastValue = &v
			println("LtEq", A, B, C, "Reg[", A, "]=", v.ToString(), "B=", bv.ToString(), "C=", cv.ToString())
		case IncReg:
			v := p.Reg[A]
			v.SetInt(v.ToInt() + 1)
		case IncLocal:
			scope := p.sys.Scopes.GetTopScope()
			v := scope.GetByIndex(A)
			v.SetInt(v.ToInt() + 1)
		case NotReg:
			p.Reg[A].SetBool(!p.Reg[A].ToBool())
			println("NotReg", A, "=", p.Reg[A].ToString())
		// label
		case DefLabel:
			//nop
		case Jump:
			p.move(code.A)
			continue
		case JumpIfTrue:
			expr := p.Reg[A]
			if expr != nil && expr.ToBool() {
				p.move(B)
				// println("JUMP +", B)
				continue
			}
		case NewArray:
			a := value.NewValueArray()
			p.Reg[A] = &a
		case AppendArray:
			a := p.Reg[A]
			if a.Type != value.Array {
				return nil, p.RuntimeError("[SYSTEM] AppendArray")
			}
			a.ArrayAppend(p.Reg[B])
			println("append", a.ToJSONString())
		case CallFunc:
			res, err := p.runCallFunc(code)
			if err != nil {
				return nil, err
			}
			p.Reg[A] = res
			lastValue = res
		case CallUserFunc:
			cur := p.procUserCallFunc(code)
			p.moveTo(cur)
			continue
		case Return:
			cur := p.procReturn(code)
			p.moveTo(cur)
		default:
			println("[system error]" + fmt.Sprintf("Undefined code: %s", p.ToString(code)))
		}
		p.next() // next code
	}
	return lastValue, nil
}

func (p *TCompiler) runCallFunc(code *TCode) (*value.Value, error) {
	// get func
	funcV := p.Consts[code.B]
	argV := p.Reg[code.C]
	if funcV.Type == value.UserFunc {
		return nil, p.RuntimeError("[SYSTEM ERROR:ユーザー関数をシステム関数として呼んだ]")
	}
	// args
	args := argV.Value.(value.TArray)
	println("argV=" + argV.ToJSONString())
	// call system func
	fn := funcV.Value.(value.TFunction)
	res, err := fn(args)
	if err != nil {
		return nil, p.RuntimeError("関数実行中のエラー。" + err.Error())
	}
	p.Reg[code.A] = res
	p.sys.Scopes.SetTopVars("それ", res)
	return res, nil
}

func (p *TCompiler) procReturn(code *TCode) int {
	scope := p.sys.Scopes.GetTopScope()
	if scope == p.sys.Global {
		println("[SYSTEMエラー] スコープが壊れています")
	}
	retValue := p.Reg[code.A]
	retAddr := scope.Get(metaKeyReturnAddr).ToInt()
	retReg := scope.Get(metaKeyReturnReg).ToInt()
	p.sys.Scopes.Close()
	p.Reg[retReg] = retValue
	return retAddr
}

func (p *TCompiler) procUserCallFunc(code *TCode) int {
	// get func
	label := p.Labels[code.B]
	argV := p.Reg[code.C]
	// open scope
	scope := p.sys.Scopes.Open()
	scope.Set(metaKeyReturnAddr, value.NewValueIntPtr(p.index+1))
	scope.Set(metaKeyReturnReg, value.NewValueIntPtr(code.A))
	scope.Set("それ", value.NewValueNullPtr())
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

func (p *TCompiler) next() {
	p.index++
}

func (p *TCompiler) isLive() bool {
	return p.index < len(p.Codes)
}

func (p *TCompiler) moveToTop() {
	p.index = 0
	p.length = len(p.Codes)
}
