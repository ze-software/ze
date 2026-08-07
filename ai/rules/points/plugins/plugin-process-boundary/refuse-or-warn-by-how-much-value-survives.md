---
kind: directive
level:
stage:
---
- **The call is the plugin's core purpose** (nothing useful happens without it) -> hard refuse: log an error naming the specific call and why, `return 1` before doing anything else. See `internal/plugins/as112/register.go`, `internal/plugins/trafficusage/register.go`, `internal/plugins/flowexport/register.go`.
- **The plugin still provides real value external** (only one feature degrades) -> warn: a `warnIfExternal(isInternal bool)` helper, called once after `sdk.NewWithConn`, logging what breaks and what still works. See `internal/plugins/cos/register.go`, `internal/plugins/ddos/detect/register.go`.
