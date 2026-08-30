---
kind: directive
level: MUST NOT
stage:
---
**A config value MUST NOT be asserted directly with `v.(bool)` or `v.(float64)`, and a numeric or bool type switch MUST NOT omit a `case string:` arm.** `./le config coercion check` (`internal/le/config/coercion/configcoercion.go`, wired into `./le verify current mode full`) refuses both shapes across every `internal/**/config.go`. Why every delivered value is a JSON string is `docs/architecture/config/yang-config-design.md`.
