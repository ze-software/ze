# Backend-Aware CLI Completion

## Meta

| Field | Value |
|-------|-------|
| Name | Backend-Aware CLI Completion |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/cli/completer.go, internal/component/command/completer.go |
| Real-path tests | test/editor/completion/backend-filter.et |
| Docs | docs/guide/configuration.md |
| Doc review | 2026-10-07: every source anchor resolves (deriveBackends and backendAllowed in internal/component/cli/completer.go, backendAllowed and SetActiveBackends in internal/component/command/completer.go) |
| Defect review | 2026-10-07: audit found no open immediate spec against the completers; journal rows naming a Component, not each re-verified here: concurrent-session-corruption.md:18, rule-written-after-the-surface-it-binds.md:18, silent-fall-through.md:22, validated-value-discarded-by-its-caller.md:14 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

CLI auto-completion filters options based on the active backend. Config editor mode (set/delete/edit/show) and operational command mode (show/clear/monitor) hide nodes annotated with `ze:backend` when the active backend is not in the annotation's list. Backend names are derived from the config tree at each tree change. Same `ze:backend` annotations used by commit-time validation, applied earlier at completion time. <!-- source: internal/component/cli/completer.go -- backendAllowed, deriveBackends --> <!-- source: internal/component/command/completer.go -- backendAllowed, SetActiveBackends -->
