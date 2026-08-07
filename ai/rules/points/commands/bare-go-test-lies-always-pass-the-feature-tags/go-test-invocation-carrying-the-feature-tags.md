---
kind: fence
level:
stage:
---
```
go test -tags "ze_core $(awk '$1 ~ /^ze_/ {print $1}' feature-gates.txt | sort -u | tr '\n' ' ')" ./internal/component/foo/
```
