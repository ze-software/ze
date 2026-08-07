---
kind: directive
level:
stage:
---
1. Codes are lower-kebab: `config-parse`, `config-yang-type`, etc.
2. Every code must be registered in `internal/core/diagnostic/codes.go` with title, description, and related codes.
3. New validation stages must map errors to diagnostic codes, not pass raw strings.
4. The `ze explain` command must return an explanation for every registered code.
