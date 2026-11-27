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
| Real-path tests | test/managed/client-first-boot.ci, test/managed/client-cached-boot.ci, test/managed/client-cached-boot-hub-down.ci, test/managed/client-first-start-from-init.ci, test/managed/config-change-notify.ci, test/managed/config-push-transactional.ci, test/managed/auth-reject.ci, test/managed/per-client-auth.ci, test/managed/fleet-status.ci, test/managed/fleet-hub-editor-declared.ci, test/managed/fleet-hold-keeps-local-edit.ci, test/managed/fleet-hold-clears-on-match.ci, test/managed/fleet-diverged-doctor.ci, test/managed/fleet-disable-severs.ci, test/managed/fleet-enable-rejoins.ci, test/managed/fleet-freeze-refuses-commit.ci, test/managed/fleet-divergence-upload.ci, test/managed/fleet-divergence-adopt.ci, test/managed/fleet-divergence-hub-restart.ci, test/managed/fleet-divergence-orphan.ci, test/managed/fleet-divergence-revert.ci, test/managed/fleet-divergence-revert-void.ci, test/managed/fleet-config-change-live.ci, test/managed/fleet-devices-connect.ci, test/managed/fleet-device-detail.ci, test/managed/fleet-devices-restart.ci, test/managed/fleet-devices-declared.ci, test/managed/fleet-devices-reload-add.ci, test/managed/fleet-devices-reload-remove.ci, test/managed/fleet-devices-label.ci, test/managed/fleet-devices-diverged.ci, test/managed/fleet-reload-listener-verified.ci, test/managed/fleet-reload-certificate-rotated.ci, test/managed/fleet-certificate-refused-doctor.ci, test/managed/fleet-inventory-report.ci, test/managed/fleet-health-report.ci, test/managed/fleet-history-reconnect.ci, test/managed/fleet-history-cap.ci, test/managed/fleet-alarm-lifecycle.ci, test/managed/fleet-alarm-history.ci |
| Docs | docs/features/fleet-management.md |
| Doc review | 2026-10-07: the row is one line (centralized config distribution over TLS); internal/component/managed carries the TLS client |
| Defect review | 2026-11-20: spec-managed-server-hardening closed with a clean review; its two guard-added-to-one-half-of-a-pair.md rows (the unguarded push and the unbounded fetch of internal/component/managed) are marked fixed; the one open journal row naming internal/component/managed (guard-added-to-one-half-of-a-pair.md, 2026-08-22, `readLine` strips a trailing carriage return) is owned by plan/release/fixes/spec-daemon-runtime-one-off-defects.md |
| Extra criteria | supported: two-daemon fetch and notify proof, a client configuration imported on a running hub applied by its connected device on the same session = test/managed/fleet-config-change-live.ci |

## Description

Centralized configuration distribution over TLS with a hub registry of managed device sessions, configuration state, hardware inventory, component health, bounded dual-resolution metric history, and hub-derived fleet alarms with bounded episode history.
