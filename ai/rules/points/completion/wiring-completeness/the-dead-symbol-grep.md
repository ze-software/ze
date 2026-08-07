---
kind: fence
level:
stage:
---
```
grep -rn 'Foo' internal/ cmd/ --include="*.go" | grep -v "_test.go" | grep -v "plan/"
```
