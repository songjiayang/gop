package exec

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/qiniu/x/log"
)

// -----------------------------------------------------------------------------

func init() {
	log.SetOutputLevel(log.Ldebug)
}

func Strcat(a, b string) string {
	return a + b
}

func execStrcat(arity int, p *Context) {
	args := p.GetArgs(2)
	ret := Strcat(args[0].(string), args[1].(string))
	p.Ret(2, ret)
}

func execSprint(arity int, p *Context) {
	args := p.GetArgs(arity)
	s := fmt.Sprint(args...)
	p.Ret(arity, s)
}

func execSprintf(arity int, p *Context) {
	args := p.GetArgs(arity)
	s := fmt.Sprintf(args[0].(string), args[1:]...)
	p.Ret(arity, s)
}

// I is a Go package instance.
var I = NewGoPackage("")

func init() {
	I.RegisterFuncvs(
		I.Funcv("Sprint", fmt.Sprint, execSprint),
		I.Funcv("Sprintf", fmt.Sprintf, execSprintf),
	)
	I.RegisterFuncs(
		I.Func("strcat", Strcat, execStrcat),
	)
	I.RegisterVars(
		I.Var("x", new(int)),
	)
	I.RegisterConsts(
		I.Const("true", reflect.Bool, true),
		I.Const("false", reflect.Bool, false),
		I.Const("nil", ConstUnboundPtr, nil),
	)
	I.RegisterTypes(
		I.Rtype(reflect.TypeOf((*Context)(nil))),
		I.Rtype(reflect.TypeOf((*Code)(nil))),
		I.Rtype(reflect.TypeOf((*Stack)(nil))),
		I.Type("rune", TyRune),
	)
}

func TestVarAndConst(t *testing.T) {
	if ci, ok := I.FindConst("true"); !ok || ci.Value != true {
		t.Fatal("FindConst failed:", ci.Value)
	}
	if ci, ok := I.FindConst("nil"); !ok || ci.Kind != ConstUnboundPtr {
		t.Fatal("FindConst failed:", ci.Kind)
	}
	if addr, ok := I.FindVar("x"); !ok || addr != 0 {
		t.Fatal("FindVar failed:", addr)
	} else {
		if addr.GetInfo().Name != "x" {
			t.Fatal("var.GetInfo failed:", *addr.GetInfo())
		}
	}
}

func TestSprint(t *testing.T) {
	sprint, ok := I.FindFuncv("Sprint")
	if !ok {
		t.Fatal("FindFuncv failed: Sprint")
	}

	code := NewBuilder(nil).
		Push(5).
		Push("32").
		CallGoFuncv(sprint, 2).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); v != "532" {
		t.Fatal("5 `32` sprint != 532, ret =", v)
	}
}

func TestSprintf(t *testing.T) {
	sprintf, ok := I.FindFuncv("Sprintf")
	strcat, ok2 := I.FindFunc("strcat")
	if !ok || !ok2 {
		t.Fatal("FindFunc failed: Sprintf/strcat")
	}
	fmt.Println("sprintf:", sprintf.GetInfo())
	fmt.Println("strcat:", strcat.GetInfo())

	code := NewBuilder(nil).
		Push("Hello, %v, %d, %s").
		Push(1.3).
		Push(1).
		Push("x").
		Push("sw").
		CallGoFunc(strcat).
		CallGoFuncv(sprintf, 4).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); v != "Hello, 1.3, 1, xsw" {
		t.Fatal("format 1.3 1 `x` `sw` strcat sprintf != `Hello, 1.3, 1, xsw`, ret =", v)
	}
}

func TestLargeArity(t *testing.T) {
	sprint, kind, ok := FindGoPackage("").Find("Sprint")
	if !ok || kind != SymbolFuncv {
		t.Fatal("Find failed: Sprint")
	}

	b := NewBuilder(nil)
	ret := ""
	for i := 0; i < bitsFuncvArityMax+1; i++ {
		b.Push("32")
		ret += "32"
	}
	code := b.
		CallGoFuncv(GoFuncvAddr(sprint), bitsFuncvArityMax+1).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); v != ret {
		t.Fatal("32 times(1024) sprint != `32` times(1024), ret =", v)
	}
}

func TestLargeSlice(t *testing.T) {
	b := NewBuilder(nil)
	ret := []string{}
	for i := 0; i < bitsFuncvArityMax+1; i++ {
		b.Push("32")
		ret = append(ret, "32")
	}
	code := b.
		MakeArray(reflect.SliceOf(TyString), bitsFuncvArityMax+1).
		MakeArray(reflect.SliceOf(TyString), -1).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, ret) {
		t.Fatal("32 times(1024) mkslice != `32` times(1024) slice, ret =", v)
	}
}

func TestLargeArray(t *testing.T) {
	b := NewBuilder(nil)
	ret := [bitsFuncvArityMax + 1]string{}
	for i := 0; i < bitsFuncvArityMax+1; i++ {
		b.Push("32")
		ret[i] = "32"
	}
	code := b.
		MakeArray(reflect.ArrayOf(bitsFuncvArityMax+1, TyString), bitsFuncvArityMax+1).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, ret) {
		t.Fatal("32 times(1024) mkslice != `32` times(1024) slice, ret =", v)
	}
}

func TestMap(t *testing.T) {
	code := NewBuilder(nil).
		Push("Hello").
		Push(3.2).
		Push("xsw").
		Push(1.0).
		MakeMap(reflect.MapOf(TyString, TyFloat64), 2).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, map[string]float64{"Hello": 3.2, "xsw": 1.0}) {
		t.Fatal("expected: {`Hello`: 3.2, `xsw`: 1}, ret =", v)
	}
}

func TestMapComprehension(t *testing.T) {
	typData := reflect.MapOf(TyString, TyInt)
	key := NewVar(TyString, "k")
	val := NewVar(TyInt, "v")
	f := NewForPhrase(key, val, typData)
	c := NewComprehension(reflect.MapOf(TyInt, TyString))
	code := NewBuilder(nil).
		MapComprehension(c).
		Push("Hello").
		Push(3).
		Push("xsw").
		Push(1).
		MakeMap(typData, 2).
		ForPhrase(f).
		DefineVar(key, val).
		LoadVar(val).
		LoadVar(key).
		EndForPhrase(f).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, map[int]string{3: "Hello", 1: "xsw"}) {
		t.Fatal(`expected: {3: "Hello", 1: "xsw"}, ret =`, v)
	}
}

func TestMapComprehensionFilter(t *testing.T) {
	typData := reflect.MapOf(TyString, TyInt)
	key := NewVar(TyString, "k")
	val := NewVar(TyInt, "v")
	f := NewForPhrase(key, val, typData)
	c := NewComprehension(reflect.MapOf(TyInt, TyString))
	code := NewBuilder(nil).
		MapComprehension(c).
		Push("Hello").
		Push(3).
		Push("xsw").
		Push(1).
		MakeMap(typData, 2).
		ForPhrase(f).
		DefineVar(key, val).
		LoadVar(val).
		Push(2).
		BuiltinOp(Int, OpLE).
		FilterForPhrase(f).
		LoadVar(val).
		LoadVar(key).
		EndForPhrase(f).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, map[int]string{1: "xsw"}) {
		t.Fatal(`expected: {1: "xsw"}, ret =`, v)
	}
}

func TestListComprehension(t *testing.T) {
	typData := reflect.ArrayOf(4, TyInt)
	x := NewVar(TyInt, "x")
	f := NewForPhrase(nil, x, typData)
	c := NewComprehension(reflect.SliceOf(TyInt))
	code := NewBuilder(nil).
		ListComprehension(c).
		Push(1).
		Push(3).
		Push(5).
		Push(7).
		MakeArray(typData, 4).
		ForPhrase(f).
		DefineVar(x).
		LoadVar(x).
		LoadVar(x).
		BuiltinOp(Int, OpMul).
		EndForPhrase(f).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, []int{1, 9, 25, 49}) {
		t.Fatal(`expected: [1, 9, 25, 49], ret =`, v)
	}
}

func TestListComprehensionFilter(t *testing.T) {
	typData := reflect.ArrayOf(4, TyInt)
	x := NewVar(TyInt, "x")
	f := NewForPhrase(nil, x, typData)
	c := NewComprehension(reflect.SliceOf(TyInt))
	code := NewBuilder(nil).
		ListComprehension(c).
		Push(1).
		Push(3).
		Push(5).
		Push(7).
		MakeArray(typData, 4).
		ForPhrase(f).
		DefineVar(x).
		LoadVar(x).
		Push(3).
		BuiltinOp(Int, OpGT). // x > 3
		FilterForPhrase(f).
		LoadVar(x).
		LoadVar(x).
		BuiltinOp(Int, OpMul).
		EndForPhrase(f).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, []int{25, 49}) {
		t.Fatal(`expected: [25, 49], ret =`, v)
	}
}

func TestMapComprehension2(t *testing.T) {
	typData := reflect.SliceOf(TyInt)
	i := NewVar(TyInt, "i")
	x := NewVar(TyInt, "x")
	f := NewForPhrase(i, x, typData)
	c := NewComprehension(reflect.MapOf(TyInt, TyInt))
	code := NewBuilder(nil).
		MapComprehension(c).
		Push(1).
		Push(3).
		Push(5).
		Push(7).
		MakeArray(typData, 4).
		ForPhrase(f).
		DefineVar(i, x).
		LoadVar(x).
		LoadVar(x).
		BuiltinOp(Int, OpMul).
		LoadVar(i).
		EndForPhrase(f).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, map[int]int{1: 0, 9: 1, 25: 2, 49: 3}) {
		t.Fatal(`expected: {1: 0, 9: 1, 25: 2, 49: 3}, ret =`, v)
	}
}

func TestListComprehensionEx(t *testing.T) {
	typData := reflect.SliceOf(TyInt)
	a := NewVar(TyInt, "a")
	b := NewVar(TyInt, "b")
	fa := NewForPhrase(nil, a, typData)
	fb := NewForPhrase(nil, b, typData)
	c := NewComprehension(typData)
	code := NewBuilder(nil).
		ListComprehension(c).
		Push(5).
		Push(6).
		Push(7).
		MakeArray(typData, 3).
		ForPhrase(fb).
		DefineVar(b).
		Push(1).
		Push(2).
		Push(3).
		Push(4).
		MakeArray(typData, 4).
		ForPhrase(fa).
		DefineVar(a).
		LoadVar(a).
		Push(1).
		BuiltinOp(Int, OpGT). // a > 1
		FilterForPhrase(fa).
		LoadVar(a).
		LoadVar(b).
		BuiltinOp(Int, OpMul).
		EndForPhrase(fa).
		EndForPhrase(fb).
		EndComprehension(c).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); !reflect.DeepEqual(v, []int{10, 15, 20, 12, 18, 24, 14, 21, 28}) {
		t.Fatal(`expected: [10, 15, 20, 12, 18, 24, 14, 21, 28], ret =`, v)
	}
}

func TestZero(t *testing.T) {
	code := NewBuilder(nil).
		Zero(TyFloat64).
		Push(3.2).
		BuiltinOp(Float64, OpAdd).
		Resolve()

	ctx := NewContext(code)
	ctx.Exec(0, code.Len())
	if v := checkPop(ctx); v != 3.2 {
		t.Fatal("0 + 3.2 != 3.2, ret =", v)
	}
}

func TestType(t *testing.T) {
	typ, ok := I.FindType("Context")
	if !ok {
		t.Fatal("FindType failed: Context not found")
	}
	fmt.Println(typ)

	typ, ok = FindGoPackage("").FindType("rune")
	if !ok || typ != TyRune {
		t.Fatal("FindType failed: rune not found")
	}
	fmt.Println(typ)
}