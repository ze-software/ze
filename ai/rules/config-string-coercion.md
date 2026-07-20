# Config String Coercion

**When:** Writing or reviewing a plugin/component `config.go` that reads YANG leaf values out of the delivered config into a typed `Config` struct.
**Severity:** advisory

## The problem

The plugin config framework delivers every YANG leaf value to a plugin's
`ParseConfig` as a JSON **string** -- `"true"`, `"50000"`, `"3.5"` -- never the
native JSON type. A hand-written parser that coerces a config value with a
native-type assertion always fails the assertion on that string and silently
falls back to the leaf's **default**:

- `if b, ok := v.(bool); ok { cfg.Enabled = b }` -- `v` is `"true"`, the
  assertion fails, `cfg.Enabled` keeps its `false` default. For a boolean
  `enabled` gate this disables the **entire feature** with no error, no panic,
  no log line.
- a `toInt`/`toFloat` helper whose type switch handles `int`/`int64`/`float64`
  but has no `case string:` -- returns `(0, false)` for every string, so the
  operator's configured value is silently ignored and the default is used.

Confirmed real instance: `ddos-detect` never ran in any daemon -- `enabled`
parsed `false` from the string `"true"`, so the detector never subscribed to
the rate feed and never fired (session 6503). The BPS/persistence/confidence
code was correct; it was never reached.

## The rule

Every coercion of a delivered config value MUST accept the string form. Use a
helper with a `case string:` arm (see `internal/plugins/trafficusage/config.go`
`cfgBool` and the `case string:` arms in its `toInt`/`toFloat`):

```go
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		if pb, err := strconv.ParseBool(strings.TrimSpace(b)); err == nil {
			return pb, true
		}
	}
	return false, false
}
```

Never `v.(bool)` / `v.(float64)` directly on a config value, and never a
numeric/bool type switch without a `case string:` arm.

## The mechanical check

`scripts/checks/config_string_coercion.go` (`make ze-config-coercion-check`,
wired into `ze-verify`) parses every `internal/**/config.go` and fails on a
type switch whose cases include a numeric/bool type but not `string`, or a
direct type assertion to a numeric/bool type. Add an allowlist entry -- with a
stated reason -- only for a genuine non-config coercion. The companion
`--selftest` proves the AST detection fires on isolated fixtures.
