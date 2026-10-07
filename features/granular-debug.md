# Granular Debug

## Meta

| Field | Value |
|-------|-------|
| Name | Granular Debug |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/debug, internal/component/debug, internal/core/slogutil |
| Real-path tests | test/plugin/debug-toggle.ci |
| Docs | docs/guide/debugging-tools.md |
| Doc review | 2026-10-07: debug.zefs profile storage read in internal/plugins/debug/profile.go saveProfile, loadProfile, deleteProfile |
| Defect review | 2026-10-07: plan/journal/unwired-feature.md (2026-09-14): set debug active name (apply) has no real-path test, unfixed |
| Extra criteria | supported: a saved profile applied with set debug active name changes the running filter = test/plugin/debug-profile-apply.ci |

## Description

Verb-first debug (set/delete/show/clear, matching VyOS syslog-level config) with per-module flag/direction/scope filtering and named profiles. CLI: `ze set debug module <name>` (enable), `ze delete debug module <name>` (disable), `ze set debug module <name> flag <flag>`, `ze set debug module <name> scope direction <dir>`, `ze set debug module <name> level <level>`. Hierarchical prefixes work (`ze set debug module bgp` covers all bgp.* subsystems). Named profiles: `ze set/delete debug profile name <name>`, `ze show debug profile [name <name>]`, `ze set debug active name <name>` (apply). Stored in `debug.zefs` (separate from config). Each plugin declares its valid flags via the debug YANG registry. Not auto-applied on reboot (safety). `show debug` (YANG-dispatched) queries live daemon state. <!-- source: internal/plugins/debug/debug.go -- runSetModule, runDeleteModule, applyProfile --> <!-- source: internal/plugins/debug/cmd/handlers.go -- show debug live state RPC --> <!-- source: internal/core/slogutil/slogutil.go -- ConfigureFilter, ClearFilter, ActiveFilter --> <!-- source: internal/component/debug/yang/register.go -- RegisterModule, ValidateFlag -->
