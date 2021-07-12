# xtypes

Package xtypes provides `go/types` extended utilities.
Converting `types.Type` into `reflect.Type`.

```
ctx := xtypes.NewContext()
rt1, err := xtypes.ToType(typ1,ctx)
rt2, err := xtypes.ToType(typ2,ctx)
ctx.UpdateAllMethods()
```
