---
kind: directive
level: MUST NOT
stage:
---
**These patterns MUST NOT be used to read a config value:**
- `if b, ok := v.(bool); ok { cfg.Enabled = b }`: `v` is `"true"`, the assertion fails, `cfg.Enabled` keeps its `false` default. For a boolean `enabled` gate this disables the **entire feature** with no error, no panic, no log line.
- a `toInt`/`toFloat` helper whose type switch handles `int`/`int64`/`float64` but has no `case string:` arm returns `(0, false)` for every string, so the operator's configured value is silently ignored and the default is used.
