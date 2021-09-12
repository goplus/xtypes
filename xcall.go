package xtypes

import (
	"errors"
	"reflect"

	xcall "github.com/goplus/xtypes/internal/reflect"
)

func FieldAddr(v interface{}, index int) (interface{}, error) {
	x := xcall.ValueOf(v).Elem()
	if !x.IsValid() {
		return nil, errors.New("invalid memory address or nil pointer dereference")
	}
	return x.Field(index).Addr().Interface(), nil
}

func Field(v interface{}, index int) (interface{}, error) {
	x := xcall.ValueOf(v)
	for x.Kind() == xcall.Ptr {
		x = x.Elem()
	}
	if !x.IsValid() {
		return nil, errors.New("invalid memory address or nil pointer dereference")
	}
	return x.Field(index).Interface(), nil
}

func methodByName(v reflect.Value, name string) xcall.Value {
	return xcall.MethodByName(v, name)
}

func callValue(v xcall.Value, args []reflect.Value) []reflect.Value {
	return xcall.Call(v, args)
}
