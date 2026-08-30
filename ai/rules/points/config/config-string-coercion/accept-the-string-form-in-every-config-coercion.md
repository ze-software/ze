---
kind: directive
level: MUST
stage:
---
**Every coercion of a delivered config value MUST accept the string form.** Use a helper with a `case string:` arm; `cfgBool` and the `case string:` arms of `toInt` and `toFloat` in `internal/plugins/trafficusage/config.go` are the reference, and the worked shape is `docs/architecture/config/yang-config-design.md`.
