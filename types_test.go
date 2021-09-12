package xtypes_test

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"image/color"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/xtypes"
)

const filename = "<src>"

func makePkg(src string) (*types.Package, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.DeclarationErrors)
	if err != nil {
		return nil, err
	}
	// use the package name as package path
	conf := types.Config{Importer: importer.Default()}
	return conf.Check(file.Name.Name, fset, []*ast.File{file}, nil)
}

type testEntry struct {
	src, str string
}

// dup returns a testEntry where both src and str are the same.
func dup(s string) testEntry {
	return testEntry{s, s}
}

func two(src, str string) testEntry {
	return testEntry{src, str}
}

var basicTypes = []testEntry{
	// basic
	dup(`bool`),
	dup(`int`),
	dup(`int8`),
	dup(`int16`),
	dup(`int32`),
	dup(`int64`),
	dup(`uint`),
	dup(`uint8`),
	dup(`uint16`),
	dup(`uint32`),
	dup(`uint64`),
	dup(`uintptr`),
	dup(`float32`),
	dup(`float64`),
	dup(`complex64`),
	dup(`complex128`),
	dup(`string`),
	dup(`unsafe.Pointer`),
}

var typesTest = []testEntry{
	dup(`*int`),
	dup(`*string`),
	dup(`[]int`),
	dup(`[]string`),
	dup(`[2]int`),
	dup(`[2]string`),
	dup(`map[int]string`),
	dup(`chan int`),
	dup(`chan string`),
	dup(`struct { x int; y string }`),
	two(`interface{}`, `interface {}`),
	two(`func(x int, y string)`, `func(int, string)`),
	two(`func(fmt string, a ...interface{})`, `func(string, ...interface {})`),
	two(`interface {
		Add(a, b int) int
		Info() string
	}`, `interface { Add(int, int) int; Info() string }`),
	two(`interface {
		Stringer
		Add(a, b int) int
	}`, `interface { Add(int, int) int; String() string }`),
	two(`error`, `interface { Error() string }`),
	two(`func(path *byte, argv **byte, envp **byte) error`, `func(*uint8, **uint8, **uint8) error`),
	two(`[4]struct{ item int; _ [40]byte }`, `[4]struct { item int; _ [40]uint8 }`),
}

func TestTypes(t *testing.T) {
	var tests []testEntry
	tests = append(tests, basicTypes...)
	tests = append(tests, typesTest...)

	for _, test := range tests {
		src := `package p; import "unsafe"; import "fmt"; type _ unsafe.Pointer; type Stringer fmt.Stringer; type T ` + test.src
		pkg, err := makePkg(src)
		if err != nil {
			t.Errorf("%s: %s", src, err)
			continue
		}
		typ := pkg.Scope().Lookup("T").Type().Underlying()
		rt, err := xtypes.ToType(typ, xtypes.NewContext(nil, nil))
		if err != nil {
			t.Errorf("%s: ToType error %v", test.src, err)
		}
		if got := rt.String(); got != test.str {
			t.Errorf("%s: got %s, want %s", test.src, got, test.str)
		}
	}
}

var namedTest = []testEntry{
	two(`package main
	type T int`, `0`),
	two(`package main
	type T byte`, `0x0`),
	two(`package main
	type T func(a **byte, b *int, c string)`, `(main.T)(nil)`),
	two(`package main
	type T struct {
		X int
		Y int
	}
	`, `main.T{X:0, Y:0}`),
	two(`package main
	type T struct {
		_ int
		_ int
		x int
		y int
	}
	`, `main.T{_:0, _:0, x:0, y:0}`),
	two(`package main
	type Point struct {
		X int
		Y int
	}
	type T struct {
		pt Point
	}
	`, `main.T{pt:main.Point{X:0, Y:0}}`),
	two(`package main
	type T struct {
		P map[string]T
	}
	`, `main.T{P:map[string]main.T(nil)}`),
	two(`package main
	type N struct {
		*T
	}
	type T struct {
		*N
	}
	`, `main.T{N:(*main.N)(nil)}`),
	two(`package main
	type T struct {
		*T
	}`, `main.T{T:(*main.T)(nil)}`),
	two(`package main
	type T *T`, `(main.T)(nil)`),
	two(`package main
	type T [2]*T`, `main.T{(*main.T)(nil), (*main.T)(nil)}`),
	two(`package main
	import "errors"
	var T = errors.New("err")
	`, `<nil>`),
	two(`package main
	import "io"
	var T io.Writer
	`, `<nil>`),
}

func TestNamed(t *testing.T) {
	var tests []testEntry
	tests = append(tests, namedTest...)

	for _, test := range tests {
		pkg, err := makePkg(test.src)
		if err != nil {
			t.Errorf("%s: %s", test.src, err)
			continue
		}
		ctx := xtypes.NewContext(nil, nil)
		typ := pkg.Scope().Lookup("T").Type()
		rt, err := xtypes.ToType(typ, ctx)
		if err != nil {
			t.Errorf("%s: ToType error %v", test.src, err)
		}
		if got := fmt.Sprintf("%+#v", reflect.New(rt).Elem().Interface()); got != test.str {
			t.Errorf("%s: got %s, want %s", test.src, got, test.str)
		}
	}
}

var methodTest = []testEntry{
	two(`//001
	package main
	import "fmt"
	type T struct {
		X int
		Y int
	}
	func (t T) String() string {
		return fmt.Sprintf("(%v,%v)",t.X,t.Y)
	}
	func (t *T) Set(x int, y int) {
		t.X, t.Y = x, y
	}
	`, `String func(main.T) string
Set func(*main.T, int, int)`),
	two(`//002
	package main
	import "fmt"
	type N struct {
		size int
	}
	func (b N) Size() int {
		return b.size
	}
	func (b *N) SetSize(n int) {
		b.size = n
	}
	type T struct {
		N
		X int
		Y int
	}
	func (t T) String() string {
		return fmt.Sprintf("(%v,%v)",t.X,t.Y)
	}
	func (t *T) Set(x int, y int) {
		t.X, t.Y = x, y
	}
	`, `Size func(main.T) int
String func(main.T) string
Set func(*main.T, int, int)
SetSize func(*main.T, int)`),
	two(`//003
	package main
	import "fmt"
	type N interface {
		Size() int
		SetSize(int)
	}
	type T struct {
		N
		X int
		Y int
	}
	func (t T) String() string {
		return fmt.Sprintf("(%v,%v)",t.X,t.Y)
	}
	func (t *T) Set(x int, y int) {
		t.X, t.Y = x, y
	}
	`, `SetSize func(main.T, int)
Size func(main.T) int
String func(main.T) string
Set func(*main.T, int, int)`),
	two(`//004
	package main
	import "fmt"
	type N struct {
		size int
	}
	func (b N) Size() int {
		return b.size
	}
	func (b *N) SetSize(n int) {
		b.size = n
	}
	type T struct {
		*N
		X int
		Y int
	}
	func (t T) String() string {
		return fmt.Sprintf("(%v,%v)",t.X,t.Y)
	}
	func (t *T) Set(x int, y int) {
		t.X, t.Y = x, y
	}
	func (t T) This() T {
		return t
	}
	`, `SetSize func(main.T, int)
Size func(main.T) int
String func(main.T) string
This func(main.T) main.T
Set func(*main.T, int, int)`),
}

func TestMethod(t *testing.T) {
	var tests []testEntry
	tests = append(tests, methodTest...)

	for _, test := range tests {
		pkg, err := makePkg(test.src)
		if err != nil {
			t.Errorf("%s: %s", test.src, err)
			continue
		}
		ctx := xtypes.NewContext(nil, nil)
		typ := pkg.Scope().Lookup("T").Type()
		rt, err := xtypes.ToType(typ, ctx)
		if err != nil {
			t.Errorf("%s: ToType error %v", test.src, err)
		}
		var infos []string
		skip := make(map[string]bool)
		for i := 0; i < rt.NumMethod(); i++ {
			fn := rt.Method(i)
			infos = append(infos, fmt.Sprintf("%v %v", fn.Name, fn.Type.String()))
			skip[fn.Name] = true
		}
		prt := reflect.PtrTo(rt)
		for i := 0; i < prt.NumMethod(); i++ {
			fn := prt.Method(i)
			if skip[fn.Name] {
				continue
			}
			infos = append(infos, fmt.Sprintf("%v %v", fn.Name, fn.Type.String()))
		}
		if got := strings.Join(infos, "\n"); got != test.str {
			t.Errorf("%s: methods\ngot\n%v\nwant\n%v", test.src, got, test.str)
		}
		if m, ok := rt.MethodByName("This"); ok {
			if m.Type.Out(0) != rt {
				t.Errorf("%s: methods type failed", test.src)
			}
		}
	}
}

var invokeTest = `
package main

import "fmt"

type T struct {
	X int
	Y int
}

func (t *T) Set(x int, y int) {
	t.X, t.Y = x, y
}

func (t T) Add(o T) T {
	return T{t.X+o.X, t.Y+o.Y}
}

func (t T) String() string {
	return fmt.Sprintf("(%v,%v)", t.X,t.Y)
}
`

func TestInvoke(t *testing.T) {
	pkg, err := makePkg(invokeTest)
	if err != nil {
		t.Errorf("invoke: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("T").Type()
	ctx := xtypes.NewContext(func(mtyp reflect.Type, method *types.Func) func(args []reflect.Value) []reflect.Value {
		switch method.Name() {
		case "Set":
			return func(args []reflect.Value) []reflect.Value {
				v := args[0].Elem()
				v.Field(0).Set(args[1])
				v.Field(1).Set(args[2])
				return nil
			}
		case "String":
			return func(args []reflect.Value) []reflect.Value {
				v := args[0]
				r := fmt.Sprintf("(%v,%v)", v.Field(0).Int(), v.Field(1).Int())
				return []reflect.Value{reflect.ValueOf(r)}
			}
		case "Add":
			return func(args []reflect.Value) []reflect.Value {
				v := args[0]
				o := args[1]
				r := reflect.New(v.Type()).Elem()
				r.Field(0).SetInt(v.Field(0).Int() + o.Field(0).Int())
				r.Field(1).SetInt(v.Field(1).Int() + o.Field(1).Int())
				return []reflect.Value{r}
			}
		}
		return nil
	}, nil)
	rt, err := xtypes.ToType(typ, ctx)
	if err != nil {
		t.Errorf("invoke: ToType error %v", err)
	}
	v := reflect.New(rt)
	v.MethodByName("Set").Call([]reflect.Value{reflect.ValueOf(100), reflect.ValueOf(200)})
	if r := v.MethodByName("String").Call(nil); len(r) != 1 || fmt.Sprint(r[0].Interface()) != "(100,200)" {
		t.Errorf("error call String: %v", r)
	}
	o := reflect.New(rt)
	o.MethodByName("Set").Call([]reflect.Value{reflect.ValueOf(10), reflect.ValueOf(20)})
	if r := v.MethodByName("Add").Call([]reflect.Value{o.Elem()}); len(r) != 1 || fmt.Sprint(r[0].Interface()) != "(110,220)" {
		t.Errorf("error call Add: %v", r)
	}
}

func TestPtrElem(t *testing.T) {
	pkg, err := makePkg("package main; type T *T")
	if err != nil {
		t.Errorf("elem: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("T").Type()
	rt, err := xtypes.ToType(typ, xtypes.NewContext(nil, nil))
	if err != nil {
		t.Errorf("elem: ToType error %v", err)
	}
	ptr := reflect.PtrTo(rt)
	if !ptr.AssignableTo(rt) || !rt.AssignableTo(ptr) {
		t.Errorf("elem: AssignableTo error %v", rt)
	}
}

func TestArrayElem(t *testing.T) {
	pkg, err := makePkg("package main; type T []T")
	if err != nil {
		t.Errorf("elem: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("T").Type()
	rt, err := xtypes.ToType(typ, xtypes.NewContext(nil, nil))
	if err != nil {
		t.Errorf("elem: ToType error %v", err)
	}
	ptr := rt.Elem()
	if !ptr.AssignableTo(rt) || !rt.AssignableTo(ptr) {
		t.Errorf("elem: AssignableTo error %v", rt)
	}
}

func TestFunc(t *testing.T) {
	pkg, err := makePkg("package main; type T func(string) T")
	if err != nil {
		t.Errorf("func: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("T").Type()
	rt, err := xtypes.ToType(typ, xtypes.NewContext(nil, nil))
	if err != nil {
		t.Errorf("func: ToType error %v", err)
	}
	out := rt.Out(0)
	if out != rt {
		t.Errorf("func: out error %v", out.Kind())
	}
}

func TestInterface(t *testing.T) {
	pkg, err := makePkg(`package main
	type T interface {
		a(s string) T
		b(s string) string
	}`)
	if err != nil {
		t.Errorf("func: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("T").Type()
	rt, err := xtypes.ToType(typ, xtypes.NewContext(nil, nil))
	if err != nil {
		t.Errorf("func: ToType error %v", err)
	}
	out := rt.Method(0).Type.Out(0)
	if out != rt {
		t.Errorf("func: out error %v", out.Kind())
	}
}

var structTest = `
package main

import "fmt"

type T struct {
	X int
	Y int
}

func (t *T) Set(x int, y int) {
	t.X, t.Y = x, y
}

func (t T) Add(o T) T {
	return T{t.X+o.X, t.Y+o.Y}
}

func (t T) String() string {
	return fmt.Sprintf("(%v,%v)", t.X,t.Y)
}

type Stringer interface {
	String() string
}

var t1 struct{ T }
var t2 struct{ *T }
var t3 struct{ Stringer }
`

func TestEmbbed(t *testing.T) {
	pkg, err := makePkg(structTest)
	if err != nil {
		t.Errorf("embbed: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("t1").Type()
	ctx := xtypes.NewContext(nil, nil)
	rt, err := xtypes.ToType(typ, ctx)
	if err != nil {
		t.Errorf("embbed: ToType error %v", err)
	}
	if n := rt.NumMethod(); n != 2 {
		t.Errorf("embbed: num method %v", n)
	}
	typ = pkg.Scope().Lookup("t2").Type()
	rt, err = xtypes.ToType(typ, ctx)
	if err != nil {
		t.Errorf("embbed: ToType error %v", err)
	}
	if n := rt.NumMethod(); n != 3 {
		t.Errorf("embbed: num method %v", n)
	}
	typ = pkg.Scope().Lookup("t3").Type()
	rt, err = xtypes.ToType(typ, ctx)
	if err != nil {
		t.Errorf("embbed: ToType error %v", err)
	}
	if n := rt.NumMethod(); n != 1 {
		t.Errorf("embbed: num method %v", n)
	}
}

var dddTest = `
package main

type T []T

func ln(args ...T) int { return len(args) }

func (*T) Sum(args ...int) int { return 1 }

type U struct {
	*T
}

func (u *U) Test() {
}
`

func TestDDD(t *testing.T) {
	pkg, err := makePkg(dddTest)
	if err != nil {
		t.Errorf("ddd: makePkg error %s", err)
	}
	typ := pkg.Scope().Lookup("U").Type()
	ctx := xtypes.NewContext(nil, nil)
	rt, err := xtypes.ToType(typ, ctx)
	if err != nil {
		t.Errorf("ddd: ToType error %v", err)
	}
	if n := rt.NumMethod(); n != 1 {
		t.Errorf("ddd: num method %v", n)
	}
	if n := reflect.PtrTo(rt).NumMethod(); n != 2 {
		t.Errorf("ddd: num method %v", n)
	}
}

var multipleTest = `
package main

type T struct {
	X int
	Y int
}

func init() {
	type T struct {
		s string
	}
	b := &T{"hello"}
	println(b)
}

func init() {
	type T *T
	var c T
	println(c)
}

func main() {
	a := &T{100,200}
	println(a)
}

`

func lookupObject(scope *types.Scope, name string) (types.Object, bool) {
	if obj := scope.Lookup(name); obj != nil {
		return obj, true
	}
	for i := 0; i < scope.NumChildren(); i++ {
		if obj, ok := lookupObject(scope.Child(i), name); ok {
			return obj, true
		}
	}
	return nil, false
}

func TestMultiple(t *testing.T) {
	pkg, err := makePkg(multipleTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	a, ok := lookupObject(pkg.Scope(), "a")
	if !ok {
		t.Error("not found object a")
	}
	b, ok := lookupObject(pkg.Scope(), "b")
	if !ok {
		t.Error("not found object b")
	}
	c, ok := lookupObject(pkg.Scope(), "c")
	if !ok {
		t.Error("not found object b")
	}
	ctx := xtypes.NewContext(nil, nil)
	ta, err := xtypes.ToType(a.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	tb, err := xtypes.ToType(b.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	tc, err := xtypes.ToType(c.Type(), ctx)
	if ta.Elem() == tb.Elem() || ta.Elem() == tc.Elem() || tb.Elem() == tc.Elem() {
		t.Error("must diffrent type")
	}
	if s := fmt.Sprintf("%#v", reflect.New(ta.Elem()).Interface()); s != "&main.T{X:0, Y:0}" {
		t.Error("bad type", s)
	}
	if s := fmt.Sprintf("%#v", reflect.New(tb.Elem()).Interface()); s != `&main.T{s:""}` {
		t.Error("bad type", s)
	}
	if s := fmt.Sprintf("%#v", reflect.New(tc.Elem()).Elem().Interface()); s != `(main.T)(nil)` {
		t.Error("bad type", s)
	}
}

var basicInterfaceTest = `
package main

var i interface{}
var e error
var b []byte
`

func TestBasicInterface(t *testing.T) {
	pkg, err := makePkg(basicInterfaceTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	i := pkg.Scope().Lookup("i")
	e := pkg.Scope().Lookup("e")
	b := pkg.Scope().Lookup("b")
	ctx := xtypes.NewContext(nil, nil)
	typ, err := xtypes.ToType(i.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if typ != reflect.TypeOf((*interface{})(nil)).Elem() {
		t.Errorf("to interface{} error %v", err)
	}
	typ, err = xtypes.ToType(e.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if typ != reflect.TypeOf((*error)(nil)).Elem() {
		t.Errorf("to error interface error %v", err)
	}
	typ, err = xtypes.ToType(b.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if typ != reflect.TypeOf((*[]byte)(nil)).Elem() {
		t.Errorf("to []byte error %v", err)
	}
}

var extTest = `
package main

import "image/color"

var r = &color.RGBA{255,0,0,255}
`

func TestExtType(t *testing.T) {
	pkg, err := makePkg(extTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	r := pkg.Scope().Lookup("r")
	ctx := xtypes.NewContext(nil, func(name *types.TypeName) (reflect.Type, bool) {
		if name.Type().String() == "image/color.RGBA" {
			return reflect.TypeOf((*color.RGBA)(nil)).Elem(), true
		}
		return nil, false
	})
	typ, err := xtypes.ToType(r.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if typ != reflect.TypeOf((*color.RGBA)(nil)) {
		t.Error("to ext type color.RGBA failed")
	}
}

var jsonTest = `
package main

import "encoding/json"

var err json.MarshalerError
`

func TestJson(t *testing.T) {
	pkg, err := makePkg(jsonTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	r := pkg.Scope().Lookup("err")
	ctx := xtypes.NewContext(nil, nil)
	typ, err := xtypes.ToType(r.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if n := typ.NumField(); n != 3 {
		t.Errorf("num field error %v", n)
	}
}

var sliceElemTest = `
package main

type Scope struct {
	s     string
	child []*Scope
}
var s *Scope
`

func TestSliceElem(t *testing.T) {
	pkg, err := makePkg(sliceElemTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	r := pkg.Scope().Lookup("s")
	ctx := xtypes.NewContext(nil, nil)
	typ, err := xtypes.ToType(r.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if elem := typ.Elem().Field(1).Type.Elem(); elem != typ {
		t.Errorf("child ptr error %#v != %#v", elem, typ)
	}
}

var implementsTest = `
package main

type Object interface {
	Name() string
	id() int
}

type Named struct {
	name string
}

func (m Named) Name() string {
	return m.name
}

func (m Named) id() int {
	return 0
}

var v Object
var n Named


`

func TestImplement(t *testing.T) {
	pkg, err := makePkg(implementsTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	v := pkg.Scope().Lookup("v")
	ctx := xtypes.NewContext(nil, nil)
	ityp, err := xtypes.ToType(v.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if n := ityp.NumMethod(); n != 2 {
		t.Errorf("num method %v", n)
	}
	r := pkg.Scope().Lookup("n")
	typ, err := xtypes.ToType(r.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if n := typ.NumMethod(); n != 1 {
		t.Errorf("num method %v", n)
	}
	if n := reflect.PtrTo(typ).NumMethod(); n != 1 {
		t.Errorf("num method %v", n)
	}
	if !typ.Implements(ityp) {
		t.Error("bad typ Implements")
	}
	if !reflect.PtrTo(typ).Implements(ityp) {
		t.Error("bad ptr typ Implements")
	}
	// if !reflectx.Implements(ityp, typ) {
	// 	t.Error("bad typ Implements")
	// }
	// if !reflectx.Implements(ityp, reflect.PtrTo(typ)) {
	// 	t.Error("bad ptr typ Implements")
	// }
}

var typesObjectTest = `
package main

import "go/types"

var v types.Object
var n *types.TypeName
`

func TestTypesObject(t *testing.T) {
	pkg, err := makePkg(typesObjectTest)
	if err != nil {
		t.Errorf("makePkg error %s", err)
	}
	v := pkg.Scope().Lookup("v")
	ctx := xtypes.NewContext(nil, nil)
	ityp, err := xtypes.ToType(v.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if n := ityp.NumMethod(); n != reflect.TypeOf((*types.Object)(nil)).Elem().NumMethod() {
		t.Errorf("num method %v", n)
	}
	r := pkg.Scope().Lookup("n")
	typ, err := xtypes.ToType(r.Type(), ctx)
	if err != nil {
		t.Errorf("ToType error %v", err)
	}
	if !typ.Implements(ityp) {
		t.Error("bad typ Implements")
	}
}
