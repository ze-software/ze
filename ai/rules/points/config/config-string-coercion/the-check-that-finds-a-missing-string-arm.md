---
kind: note
level:
stage:
---
`internal/le/configcoercion/configcoercion.go` (`./le config-coercion check`, wired into `./le verify current mode full`) parses every `internal/**/config.go` and fails on a type switch whose cases include a numeric/bool type but not `string`, or a direct type assertion to a numeric/bool type.
