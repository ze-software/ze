---
kind: fence
level:
stage:
---
```go
// At init time (cold path): string -> ID
var familyByName = map[string]family.Family{}

// At runtime (hot path): ID -> data
var ribByFamily = map[family.Family]*RIB{}
```
