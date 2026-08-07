---
kind: note
level:
stage:
---
`scripts/checks/config_string_coercion.go` (`make ze-config-coercion-check`, wired into `ze-verify`) parses every `internal/**/config.go` and fails on a type switch whose cases include a numeric/bool type but not `string`, or a direct type assertion to a numeric/bool type.
