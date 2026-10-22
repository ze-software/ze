# Configuration Reload

## Meta

| Field | Value |
|-------|-------|
| Name | Configuration Reload |
| Page | docs/features/config-reload.md |
| Kind | daemon |
| Scope | partial |
| Scope gaps | reload safety that depends on component-specific journals, privileged dataplane rollback evidence |
| Level | experimental |
| Components | cmd/ze/hub/main_reload.go, internal/component/plugin/server |
| Real-path tests | test/reload/reload-add-peer.ci, test/reload/reload-remove-peer.ci, test/reload/reload-bad-config.ci, test/reload/reload-no-change.ci, test/reload/tx-protocol-rollback.ci, test/reload/tx-bgp-rollback.ci, test/reload/tx-protocol-sighup.ci, test/reload/bgp-bfd-added-live.ci, test/reload/bgp-bfd-removed-live.ci, test/reload/bgp-bfd-profile-edit-live.ci, test/reload/bgp-bfd-dynamic-toggle-live.ci, test/reload/bgp-bfd-shared-session-widens.ci, test/reload/bgp-rpki-attach-live.ci, test/reload/bgp-attach-receive-only-live.ci, test/reload/bgp-attach-grant-change-live.ci, test/reload/bgp-attach-no-route-refresh-restarts.ci, test/reload/bgp-attach-sent-direction-restarts.ci, test/reload/bgp-attach-dynamic-receive-only-live.ci, test/reload/bgp-dynamic-filter-swap-live.ci, test/plugin/attach-process-reload.ci |
| Docs | docs/features/config-reload.md |
| Doc review | 2026-10-07: the row names its own unproven subset; reload functional tests under test/reload cover candidate promotion and rollback |
| Defect review | 2026-10-07: open: plan/journal/rollback-forgets-partial-apply.md row 7, plan/release/validation/spec-bgp-reload-rich-peer-no-bounce.md |

## Description

Live reload via SIGHUP with automatic reconciliation. Reload stages edited config as a candidate version and promotes it to active only after runtime reload succeeds. Plugin-server transactions, config-provider roots, subsystem reload, and changed external plugin replacement roll back on failure; remaining reload safety depends on component-specific journals and privileged dataplane evidence.
