---
kind: directive
level: MUST
stage:
---
- Code MUST convert to string only at two sinks: external wire (`MarshalText`/`UnmarshalText`) and human output (`String()` returning interned literal or registry name).
- `String()` MUST NOT use: `fmt.Sprintf`, `strconv.Itoa`, `strconv.FormatUint`, `string([]byte{...})`, `strings.Builder`, `+`.
- `fmt.Sprintf` bypasses `AppendTo`/`WriteTo`, so it MUST stay on cold paths only.
- Canonical impl: `internal/core/family/family.go`.
