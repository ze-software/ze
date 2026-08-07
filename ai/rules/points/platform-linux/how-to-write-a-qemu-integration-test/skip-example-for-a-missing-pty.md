---
kind: fence
level:
stage:
---
```go
master, slave, err := pty.Open()
if err != nil {
    t.Skipf("cannot open pty: %v", err)
}
```
