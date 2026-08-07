---
kind: fence
level:
stage:
---
```go
newNS, err := netns.NewNamed(name)
if err != nil {
    t.Skipf("requires CAP_NET_ADMIN: %v", err)
}
```
