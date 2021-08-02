/*
 Copyright 2020 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package xtypes provides `go/types` extended utilities. for example,
// converting `types.Type` into `reflect.Type`.

package xtypes

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"reflect"
	"unsafe"

	"github.com/goplus/reflectx"
)

var basicTypes = [...]reflect.Type{
	types.Bool:          reflect.TypeOf(false),
	types.Int:           reflect.TypeOf(0),
	types.Int8:          reflect.TypeOf(int8(0)),
	types.Int16:         reflect.TypeOf(int16(0)),
	types.Int32:         reflect.TypeOf(int32(0)),
	types.Int64:         reflect.TypeOf(int64(0)),
	types.Uint:          reflect.TypeOf(uint(0)),
	types.Uint8:         reflect.TypeOf(uint8(0)),
	types.Uint16:        reflect.TypeOf(uint16(0)),
	types.Uint32:        reflect.TypeOf(uint32(0)),
	types.Uint64:        reflect.TypeOf(uint64(0)),
	types.Uintptr:       reflect.TypeOf(uintptr(0)),
	types.Float32:       reflect.TypeOf(float32(0)),
	types.Float64:       reflect.TypeOf(float64(0)),
	types.Complex64:     reflect.TypeOf(complex64(0)),
	types.Complex128:    reflect.TypeOf(complex128(0)),
	types.String:        reflect.TypeOf(""),
	types.UnsafePointer: reflect.TypeOf(unsafe.Pointer(nil)),
}

var (
	tyEmptyInterface = reflect.TypeOf((*interface{})(nil)).Elem()
	tyErrorInterface = reflect.TypeOf((*error)(nil)).Elem()
)

var (
	// ErrUntyped error
	ErrUntyped = errors.New("untyped type")
	// ErrUnknownArrayLen error
	ErrUnknownArrayLen = errors.New("unknown array length")
)

func ToTypeList(tuple *types.Tuple, ctx Context) (list []reflect.Type, err error) {
	for i := 0; i < tuple.Len(); i++ {
		t, err := ToType(tuple.At(i).Type(), ctx)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return
}

func ToType(typ types.Type, ctx Context) (reflect.Type, error) {
	switch t := typ.(type) {
	case *types.Basic:
		if kind := t.Kind(); kind >= types.Bool && kind <= types.UnsafePointer {
			return basicTypes[kind], nil
		} else if kind == types.UntypedBool {
			return basicTypes[types.Bool], nil
		} else if kind == types.UntypedString {
			return basicTypes[types.String], nil
		}
		return nil, ErrUntyped
	case *types.Pointer:
		elem, err := ToType(t.Elem(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown pointer elem type - %w", err)
		}
		return reflect.PtrTo(elem), nil
	case *types.Slice:
		elem, err := ToType(t.Elem(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown slice elem type - %w", err)
		}
		return reflect.SliceOf(elem), nil
	case *types.Array:
		elem, err := ToType(t.Elem(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown array elem type - %w", err)
		}
		n := t.Len()
		if n < 0 {
			return nil, ErrUnknownArrayLen
		}
		return reflect.ArrayOf(int(n), elem), nil
	case *types.Map:
		key, err := ToType(t.Key(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown map key type - %w", err)
		}
		elem, err := ToType(t.Elem(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown map elem type - %w", err)
		}
		return reflect.MapOf(key, elem), nil
	case *types.Chan:
		elem, err := ToType(t.Elem(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown chan elem type - %w", err)
		}
		return reflect.ChanOf(toChanDir(t.Dir()), elem), nil
	case *types.Struct:
		return toStructType(t, ctx)
	case *types.Named:
		return toNamedType(t, ctx)
	case *types.Interface:
		return toInterfaceType(t, ctx)
	case *types.Signature:
		in, err := ToTypeList(t.Params(), ctx)
		if err != nil {
			return nil, err
		}
		out, err := ToTypeList(t.Results(), ctx)
		if err != nil {
			return nil, err
		}
		return reflect.FuncOf(in, out, t.Variadic()), nil
	}
	return nil, fmt.Errorf("unknown type %v", typ)
}

var (
	sigMap = make(map[string]reflect.Type)
)

func toChanDir(d types.ChanDir) reflect.ChanDir {
	switch d {
	case types.SendRecv:
		return reflect.BothDir
	case types.SendOnly:
		return reflect.SendDir
	case types.RecvOnly:
		return reflect.RecvDir
	}
	return 0
}

// toStructType converts a types.Struct to reflect.Type.
func toStructType(t *types.Struct, ctx Context) (typ reflect.Type, err error) {
	n := t.NumFields()
	flds := make([]reflect.StructField, n)
	for i := 0; i < n; i++ {
		flds[i], err = toStructField(t.Field(i), t.Tag(i), ctx)
		if err != nil {
			return nil, err
		}
	}
	typ = reflectx.StructOf(flds)
	typ, _ = toMethodSet(t, typ, ctx)
	//ctx.UpdateType(typ, fnUpdate)
	return typ, nil
}

func toStructField(v *types.Var, tag string, ctx Context) (fld reflect.StructField, err error) {
	name := v.Name()
	typ, err := ToType(v.Type(), ctx)
	if err != nil {
		err = fmt.Errorf("unknown struct field `%s` type - %w", name, err)
		return
	}
	fld = reflect.StructField{
		Name:      name,
		Type:      typ,
		Tag:       reflect.StructTag(tag),
		Anonymous: v.Anonymous(),
	}
	if !token.IsExported(name) {
		fld.PkgPath = v.Pkg().Path()
	}
	return
}

func toMethodSet(t types.Type, styp reflect.Type, ctx Context) (reflect.Type, func() error) {
	methods := IntuitiveMethodSet(t)
	numMethods := len(methods)
	if numMethods == 0 {
		return styp, nil
	}
	var mcount, pcount int
	for i := 0; i < numMethods; i++ {
		sig := methods[i].Type().(*types.Signature)
		pointer := isPointer(sig.Recv().Type())
		if !pointer {
			mcount++
		}
		pcount++
	}
	typ := reflectx.NewMethodSet(styp, mcount, pcount)
	fn := func() error {
		var ms []reflectx.Method
		for i := 0; i < numMethods; i++ {
			fn := methods[i].Obj().(*types.Func)
			sig := methods[i].Type().(*types.Signature)
			pointer := isPointer(sig.Recv().Type())
			mtyp, err := ToType(sig, ctx)
			if err != nil {
				return fmt.Errorf("named methods `%s.%s` - %w", t, fn.Name(), err)
			}
			var mfn func(args []reflect.Value) []reflect.Value
			if ctx != nil {
				mfn = ctx.LookupMethod(fn)
			}
			ms = append(ms, reflectx.MakeMethod(fn.Name(), methods[i].Obj().Pkg().Path(), pointer, mtyp, mfn))
		}
		return reflectx.SetMethodSet(typ, ms, false)
	}
	fn()
	return typ, fn
}

func toNamedType(t *types.Named, ctx Context) (reflect.Type, error) {
	name := t.Obj()
	if name.Pkg() == nil {
		if name.Name() == "error" {
			return tyErrorInterface, nil
		}
		return ToType(t.Underlying(), ctx)
	}
	if ctx != nil {
		if t, ok := ctx.FindType(name); ok {
			return t, nil
		}
	}
	utype, err := ToType(t.Underlying(), ctx)
	if err != nil {
		return nil, fmt.Errorf("named type `%s` - %w", name.Name(), err)
	}
	typ := reflectx.NamedTypeOf(name.Pkg().Path(), name.Name(), utype)
	var fnUpdate func() error
	if typ.Kind() != reflect.Interface {
		typ, fnUpdate = toMethodSet(t, typ, ctx)
	}
	ctx.UpdateType(name, typ, fnUpdate)
	return typ, nil
}

func isPointer(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Pointer)
	return ok
}

func toInterfaceType(t *types.Interface, ctx Context) (reflect.Type, error) {
	n := t.NumMethods()
	if n == 0 {
		return tyEmptyInterface, nil
	}
	ms := make([]reflect.Method, n)
	for i := 0; i < n; i++ {
		fn := t.Method(i)
		mtyp, err := ToType(fn.Type(), ctx)
		if err != nil {
			return nil, fmt.Errorf("unknown interface method `%v` `%s` type - %w", t, fn.Name(), err)
		}
		ms[i] = reflect.Method{
			Name: fn.Name(),
			Type: mtyp,
		}
		if pkg := fn.Pkg(); pkg != nil {
			ms[i].PkgPath = pkg.Path()
		}
	}
	return reflectx.InterfaceOf(nil, ms), nil
}

// Context interface
type Context interface {
	FindType(name *types.TypeName) (reflect.Type, bool)
	UpdateType(name *types.TypeName, typ reflect.Type, fnUpdateMethods func() error)
	LookupMethod(method *types.Func) func(args []reflect.Value) []reflect.Value
}

type typeScope struct {
	rtype map[reflect.Type]reflect.Type // pre_type => type
}

type context struct {
	scope        map[*types.Scope]*typeScope
	ntype        map[reflect.Type](func() error) // type => update_methods
	findMethod   func(method *types.Func) func(args []reflect.Value) []reflect.Value
	findTypeName func(name *types.TypeName) (reflect.Type, bool)
}

func NewContext(
	findMethod func(method *types.Func) func(args []reflect.Value) []reflect.Value,
	findTypeName func(name *types.TypeName) (reflect.Type, bool),
) Context {
	ctx := &context{
		scope:        make(map[*types.Scope]*typeScope),
		ntype:        make(map[reflect.Type](func() error)),
		findMethod:   findMethod,
		findTypeName: findTypeName,
	}
	if ctx.findMethod == nil {
		ctx.findMethod = func(method *types.Func) func(args []reflect.Value) []reflect.Value {
			return nil
		}
	}
	if ctx.findTypeName == nil {
		ctx.findTypeName = func(name *types.TypeName) (reflect.Type, bool) {
			return nil, false
		}
	}
	return ctx
}

func (t *context) LookupMethod(method *types.Func) func(args []reflect.Value) []reflect.Value {
	return t.findMethod(method)
}

func (t *typeScope) FindType(name *types.TypeName) (reflect.Type, bool) {
	for k, v := range t.rtype {
		if k.PkgPath() == name.Pkg().Path() && k.Name() == name.Name() {
			if v != nil {
				return v, true
			}
			return k, true
		}
	}
	typ := reflectx.NamedTypeOf(name.Pkg().Path(), name.Name(), tyEmptyInterface)
	t.rtype[typ] = nil
	return typ, false
}

func typeId(typ reflect.Type) string {
	var id string
	if path := typ.PkgPath(); path != "" {
		id = path + "."
	}
	return id + typ.Name()
}

func (t *typeScope) UpdateType(typ reflect.Type) {
	rmap := make(map[string]reflect.Type)
	for k, v := range t.rtype {
		if k.PkgPath() == typ.PkgPath() && k.Name() == typ.Name() {
			t.rtype[k] = typ
			v = typ
		}
		if v != nil {
			rmap[typeId(k)] = v
		}
	}
	for _, v := range t.rtype {
		if v != nil {
			reflectx.ReplaceType(v.PkgPath(), v, rmap)
		}
	}
}

func (t *context) findScope(parent *types.Scope) *typeScope {
	scope, ok := t.scope[parent]
	if !ok {
		scope = &typeScope{make(map[reflect.Type]reflect.Type)}
		t.scope[parent] = scope
	}
	return scope
}
func (t *context) FindType(name *types.TypeName) (reflect.Type, bool) {
	if typ, ok := t.findTypeName(name); ok {
		return typ, true
	}
	return t.findScope(name.Parent()).FindType(name)
}

func (t *context) UpdateType(name *types.TypeName, typ reflect.Type, fnUpdateMethods func() error) {
	t.findScope(name.Parent()).UpdateType(typ)
	if fnUpdateMethods != nil {
		t.ntype[typ] = fnUpdateMethods
		for _, fn := range t.ntype {
			fn()
		}
	}
}

// golang.org/x/tools/go/types/typeutil.IntuitiveMethodSet
func IntuitiveMethodSet(T types.Type) []*types.Selection {
	isPointerToConcrete := func(T types.Type) bool {
		ptr, ok := T.(*types.Pointer)
		return ok && !types.IsInterface(ptr.Elem())
	}

	var result []*types.Selection
	mset := types.NewMethodSet(T)
	if types.IsInterface(T) || isPointerToConcrete(T) {
		for i, n := 0, mset.Len(); i < n; i++ {
			result = append(result, mset.At(i))
		}
	} else {
		// T is some other concrete type.
		// Report methods of T and *T, preferring those of T.
		pmset := types.NewMethodSet(types.NewPointer(T))
		for i, n := 0, pmset.Len(); i < n; i++ {
			meth := pmset.At(i)
			if m := mset.Lookup(meth.Obj().Pkg(), meth.Obj().Name()); m != nil {
				meth = m
			}
			result = append(result, meth)
		}
	}
	return result
}
