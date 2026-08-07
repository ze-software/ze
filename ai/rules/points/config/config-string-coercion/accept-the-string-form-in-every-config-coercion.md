---
kind: directive
level: MUST
stage:
---
**Every coercion of a delivered config value MUST accept the string form.** Use a helper with a `case string:` arm (see `internal/plugins/trafficusage/config.go` `cfgBool` and the `case string:` arms in its `toInt`/`toFloat`), shown under Examples.
