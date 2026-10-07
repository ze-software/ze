# Modular Deployment

## Meta

| Field | Value |
|-------|-------|
| Name | Modular Deployment |
| Kind | daemon |
| Scope | partial |
| Scope gaps | BGP-off plugin functional coverage (plan/spec-fixit-bgp-off-plugin-functional.md), idempotent deferred reapply at startup (plan/immediate/spec-startup-deferred-reapply-idempotency.md) |
| Level | experimental |
| Components | internal/component/plugin/server/startup_autoload.go |
| Real-path tests | test/reload/reload-add-bgp.ci, test/reload/reload-remove-bgp.ci |
| Doc review | 2026-10-07: autoLoadForNewConfigPaths and autoStopForRemovedConfigPaths exist in startup_autoload.go; no user page links this row |
| Defect review | 2026-10-07: open: plan/spec-fixit-bgp-off-plugin-functional.md, plan/immediate/spec-startup-deferred-reapply-idempotency.md |
| Extra criteria | supported: a user page describing config-driven loading = none yet |

## Description

Config-driven plugin loading: BGP, interfaces, and FIB load only when their config section is present. Add or remove subsystems at runtime via config reload (SIGHUP). Required config-root autoload failures fail closed, reload diffs restart same-name plugins when their definition changes, and changed external plugin replacements are pre-started before the old handler is removed. <!-- source: internal/component/plugin/server/startup_autoload.go -- autoLoadForNewConfigPaths, autoStopForRemovedConfigPaths -->
