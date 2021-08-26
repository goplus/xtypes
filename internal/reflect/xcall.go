package reflect

import (
	"reflect"
	"unsafe"
)

func fromValue(v reflect.Value) Value {
	return *(*Value)(unsafe.Pointer(&v))
}

func toValue(v Value) reflect.Value {
	return *(*reflect.Value)(unsafe.Pointer(&v))
}

func MethodByName(v reflect.Value, name string) Value {
	return fromValue(v).MethodByName(name)
}

func Call(v Value, args []reflect.Value) []reflect.Value {
	var res []Value
	n := len(args)
	if n == 0 {
		res = v.Call(nil)
	} else {
		a := make([]Value, n, n)
		for i := 0; i < n; i++ {
			a[i] = fromValue(args[i])
		}
		res = v.Call(a)
	}
	nout := len(res)
	if nout == 0 {
		return nil
	}
	r := make([]reflect.Value, nout, nout)
	for i := 0; i < nout; i++ {
		r[i] = toValue(res[i])
	}
	return r
}
