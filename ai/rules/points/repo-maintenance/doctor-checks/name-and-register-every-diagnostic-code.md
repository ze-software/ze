---
kind: directive
level: MUST
stage:
---
- **All doctor codes MUST use the `doctor-` prefix: `doctor-<component>-<condition>`.**
- **Every new code MUST be registered in `internal/core/diagnostic/codes.go` with title, description, and examples. The code MUST be explainable via `ze explain`.**
