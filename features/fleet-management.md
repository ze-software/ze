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
| Real-path tests | test/managed/client-first-boot.ci, test/managed/client-cached-boot.ci, test/managed/config-change-notify.ci, test/managed/config-push-transactional.ci, test/managed/auth-reject.ci, test/managed/per-client-auth.ci |
| Docs | docs/features/fleet-management.md |
| Doc review | 2026-10-07: the row is one line (centralized config distribution over TLS); internal/component/managed carries the TLS client |
| Defect review | 2026-10-07: open: plan/immediate/spec-managed-server-hardening.md (blocked) |
| Extra criteria | supported: two-daemon fetch and notify proof = none yet |

## Description

Centralized config distribution over TLS
