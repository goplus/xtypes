package xtypes_test

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
		rt, err := xtypes.ToType(typ, nil)
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
		ctx := xtypes.NewContext(nil)
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
	two(`package main
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
	two(`package main
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
	two(`package main
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
	two(`package main
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
		ctx := xtypes.NewContext(nil)
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
			t.Errorf("%s: methods: got %v, want %v", test.src, got, test.str)
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
	ctx := xtypes.NewContext(func(nt types.Type, name string) func(args []reflect.Value) []reflect.Value {
		if !types.Identical(typ, nt) {
			t.Fatal("error identical")
		}
		switch name {
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
	})
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
