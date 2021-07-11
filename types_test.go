package xtypes_test

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
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
		ctx := xtypes.NewContext()
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
