# Fleet Management

## Meta

| Field | Value |
|-------|-------|
| Name | Fleet Management |
| Page | docs/features/fleet-management.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/managed |
| Real-path tests | test/managed/client-first-boot.ci, test/managed/client-cached-boot.ci, test/managed/client-cached-boot-hub-down.ci, test/managed/client-first-start-from-init.ci, test/managed/config-change-notify.ci, test/managed/config-push-transactional.ci, test/managed/auth-reject.ci, test/managed/per-client-auth.ci, test/managed/fleet-status.ci, test/managed/fleet-hub-editor-declared.ci, test/managed/fleet-hold-keeps-local-edit.ci, test/managed/fleet-hold-clears-on-match.ci, test/managed/fleet-diverged-doctor.ci, test/managed/fleet-disable-severs.ci, test/managed/fleet-enable-rejoins.ci, test/managed/fleet-freeze-refuses-commit.ci, test/managed/fleet-divergence-upload.ci, test/managed/fleet-divergence-adopt.ci, test/managed/fleet-divergence-hub-restart.ci, test/managed/fleet-divergence-orphan.ci, test/managed/fleet-divergence-revert.ci, test/managed/fleet-divergence-revert-void.ci |
| Docs | docs/features/fleet-management.md |
| Doc review | 2026-10-07: the row is one line (centralized config distribution over TLS); internal/component/managed carries the TLS client |
| Defect review | 2026-10-07: open: plan/release/fixes/spec-managed-server-hardening.md (blocked) |
| Extra criteria | supported: two-daemon fetch and notify proof = none yet |

## Description

Centralized config distribution over TLS
