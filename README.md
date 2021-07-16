# xtypes

Package xtypes provides `go/types` extended utilities.
Converting `types.Type` into `reflect.Type`.

```
ctx := xtypes.NewContext(nil)
t, err := xtypes.ToType(typ,ctx)
```
