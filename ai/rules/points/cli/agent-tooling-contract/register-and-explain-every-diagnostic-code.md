---
kind: directive
level: MUST
stage:
---
1. Codes are lower-kebab: `config-parse`, `config-yang-type`, etc.
2. Every code MUST be registered in `internal/core/diagnostic/codes.go` with title, description, and related codes.
3. New validation stages MUST map errors to diagnostic codes, not pass raw strings.
4. The `ze explain` command MUST return an explanation for every registered code.
