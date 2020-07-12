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
	p.reg = p.scope.Reg
	return p.runCode()
}

func (p *TCompiler) runCode() (*value.Value, error) {
	lastValue := value.NewNullPtr()
	p.isJump = false
	for p.isLive() {
		code := p.peek()
		f := codeFuncTable[code.Type]
		if f == nil {
			println("[system error]" + fmt.Sprintf("Undefined code: %s", p.ToString(code)))
		}
		//println("*RUN=", p.index, p.ToString(code))
		res, err := f(p, code)
		if err != nil {
			return nil, err
		}
		lastValue = res
		// println("\t\tres=", lastValue.ToString())
		// println("\t@@Lvl=", p.scope.Level, "|", p.scope.ToStringRegs())
		// println("\t@@Lvl=", p.scope.Level, "|", p.scope.ToStringValues())
		if p.isJump {
			p.isJump = false
			continue
		}
		p.moveNext()
	}
	return lastValue, nil
}

func (p *TCompiler) runExString(s string) *value.Value {
	res := ""
	varName := ""
	isEx := false
	src := []rune(s)
	i := 0
	for i < len(src) {
		c := src[i]
		if !isEx {
			if c == '{' {
				isEx = true
				i++
				continue
			}
			res += string(c)
			i++
			continue
		}
		if c == '}' {
			isEx = false
			v, _ := p.sys.Scopes.Find(varName)
			if v != nil {
				res += v.ToString()
			}
			i++
			continue
		}
		varName += string(c)
		i++
		continue
	}
	return value.NewStrPtr(res)
}

func (p *TCompiler) runForeach(code *TCode) (*value.Value, error) {
	// FOREACH isContinue:A expr:B counter:C
	A, B, C := code.A, code.B, code.C
	exprV := p.regGet(B)
	i := p.regGet(C).ToInt()
	clen := exprV.Length()
	condB := (i < clen)
	var lastValue *value.Value = nil
	if condB {
		if exprV.Type == value.Array {
			elemV := exprV.ArrayGet(i)
			p.sys.Scopes.SetTopVars("それ", elemV)
			p.sys.Scopes.SetTopVars("対象", elemV)
			lastValue = elemV
		} else if exprV.Type == value.Hash {
			keys := exprV.HashKeys()
			k := keys[i]
			// println("foreack,k=", k, "/", len(keys), "=", clen)
			v := exprV.HashGet(k)
			p.sys.Scopes.SetTopVars("それ", v)
			p.sys.Scopes.SetTopVars("対象", v)
			p.sys.Scopes.SetTopVars("対象キー", value.NewStrPtr(k))
			lastValue = v
		} else {
			condB = false
		}
	}
	p.regSet(C, value.NewIntPtr(i+1))
	condV := value.NewBoolPtr(!condB)
	p.regSet(A, condV)
	return lastValue, nil
}

func (p *TCompiler) runCallFunc(code *TCode) (*value.Value, error) {
	// get func
	funcV := p.Consts.Get(code.B)
	// argV := p.regGet(code.C)
	// args := argV.Value.(value.TArray)
	if funcV.Type == value.UserFunc {
		return nil, p.RuntimeError("[SYSTEM ERROR:ユーザー関数をシステム関数として呼んだ]")
	}
	// call system func
	argCount := len(p.sys.JosiList[funcV.Tag])
	fn := funcV.Value.(value.TFunction)
	// args
	args := value.NewTArray()
	for i := 0; i < argCount; i++ {
		v := p.ValueStack.Pop()
		args.Append(v)
	}
	args.Reverse()
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
	p.sys.Scopes.Close()
	p.scope = p.sys.Scopes.GetTopScope()
	p.reg = p.scope.Reg
	// Set Result
	p.regSet(retReg, retValue)
	p.scope.Set("それ", retValue)
	// println("RETURN,reg=", p.reg.ToJSONString(), "/Back=", retAddr)
	return retAddr, retValue
}

func (p *TCompiler) procCallUserFunc(code *TCode) int {
	// get func
	label := p.Labels[code.B]
	// argIndex := code.C
	// oldScope := p.scope
	// open scope
	scope := p.sys.Scopes.Open()
	p.scope = scope
	p.reg = scope.Reg
	// 登録する順番に注意
	scope.Set("それ", value.NewNullPtr())
	scope.Reg.Set(metaRegReturnAddr, value.NewIntPtr(p.index+1))
	scope.Reg.Set(metaRegReturnValue, value.NewIntPtr(code.A))
	// 引数分だけPOPして、ローカル変数に登録
	plen := len(label.argNames)
	for i := 0; i < plen; i++ {
		v := p.ValueStack.Pop()
		name := label.argNames[plen-i-1]
		scope.Set(name, v)
	}
	/*
		// 変数を登録する
		for i, name := range label.argNames {
			v := oldScope.Reg.Get(argIndex + i)
			scope.Set(name, v)
		}
	*/
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

// --- レジスタ操作 ---

func (p *TCompiler) regSet(index int, val *value.Value) {
	p.reg.Set(index, val)
	// println("[REG]SET = " + p.reg.ToString())
}

func (p *TCompiler) regGet(index int) *value.Value {
	// println("[REG]GET = " + p.reg.ToString())
	return p.reg.Get(index)
}

func (p *TCompiler) regTop() int {
	return p.scope.Index
}

func (p *TCompiler) regNext() int {
	i := p.scope.Index
	p.scope.Index++
	return i
}

func (p *TCompiler) regBack() int {
	p.scope.Index--
	return p.scope.Index
}

func (p *TCompiler) loopBegin(continueLabel, breakLabel *TCode) {
	// backup
	p.loopLabels = append(p.loopLabels, p.continueLabel, p.breakLabel)
	// new value
	p.continueLabel = continueLabel
	p.breakLabel = breakLabel
}
func (p *TCompiler) loopEnd() {
	// recover value
	labelCount := len(p.loopLabels)
	if labelCount >= 2 {
		p.continueLabel = p.loopLabels[labelCount-2]
		p.breakLabel = p.loopLabels[labelCount-1]
		// pop 2 items
		p.loopLabels = p.loopLabels[0 : labelCount-2]
	}
	//println("@@@", len(p.loopLabels))
}
