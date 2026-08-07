---
kind: fence
level:
stage:
---
```go
// save (best-effort; no-op when no blob store is registered)
data, _ := json.Marshal(snapshot)
_, _ = statestore.Put(zefs.KeyDDoSDetectBaseline.Pattern, data)

// restore
if data, ok := statestore.Get(zefs.KeyDDoSDetectBaseline.Pattern); ok {
    _ = json.Unmarshal(data, &snapshot) // keep version/sanity guards
}
```
