---
kind: directive
level: MAY
stage:
---
- `internal/component/plugin/`, `internal/component/bgp/`, `internal/component/config/`, `cmd/ze/` -> registry
- NLRI decoding: `registry.NLRIDecoder(family)` -> `func(hex) (json, error)`
- NLRI encoding: `registry.NLRIEncoder(family)` -> `func(args) (hex, error)`
- Plugin `register.go` and `internal/component/plugin/all/all.go` blank imports MAY be used
- Schema imports (`<plugin>/yang/`) MAY be used (data, not logic)
- Test imports MAY be used
