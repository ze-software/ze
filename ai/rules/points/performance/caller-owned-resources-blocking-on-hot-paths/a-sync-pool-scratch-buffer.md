---
kind: fence
level:
stage:
---
```go
var scratchPool = sync.Pool{
    New: func() any { return make([]byte, 0, 4096) },
}

func process(data []byte) Result {
    scratch := scratchPool.Get().([]byte)[:0]
    defer scratchPool.Put(scratch)

    // use scratch for intermediate work
    scratch = append(scratch, data...)
    // ... transform scratch ...

    return buildResult(scratch)
}
```
