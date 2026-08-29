---
kind: note
level: MUST
stage:
---
Every spec MUST have a metadata table immediately after the `# Spec:` title. This is the source of truth for spec status, parsed by `./le spec status` and validated by `hookValidateSpec` in `internal/le/hookruntime/lifecycle.go`.
