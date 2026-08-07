---
kind: fence
level:
stage:
---
```go
var fooDispatcher = newFooDispatcher()

func newFooDispatcher() *subdispatch.Dispatcher {
    d := subdispatch.New("foo", "Foo operations")
    d.Register("bar", runBar, subdispatch.SubMeta{Desc: "Do bar"})
    d.Register("baz", runBaz, subdispatch.SubMeta{Desc: "Do baz"})
    return d
}

func runFoo(args []string) int {
    return fooDispatcher.Dispatch(args)
}
```
