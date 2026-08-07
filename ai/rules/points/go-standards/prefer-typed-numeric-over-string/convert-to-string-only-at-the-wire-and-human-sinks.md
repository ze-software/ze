---
kind: directive
level:
stage:
---
- Two sinks only: external wire (`MarshalText`/`UnmarshalText`) and human output (`String()` returning interned literal or registry name).
- Banned in `String()`: `fmt.Sprintf`, `strconv.Itoa`, `strconv.FormatUint`, `string([]byte{...})`, `strings.Builder`, `+`.
- `fmt.Sprintf` bypasses `AppendTo`/`WriteTo` -- cold paths only.
- Canonical impl: `internal/core/family/family.go`.
