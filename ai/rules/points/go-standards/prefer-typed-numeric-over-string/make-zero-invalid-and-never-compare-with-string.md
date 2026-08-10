---
kind: directive
level: MUST
stage:
---
- Zero MUST mean `Unspecified` / invalid. The enum type MUST be a distinct `uint8`/`uint16` (not assignable from bare integer literal). `String()` is for diagnostics; code MUST NOT use it for comparison.
- Plugin-extensible sets: numeric ID registered at init (see `spec-bgp-redistribute`, `internal/core/family/family.go`).
