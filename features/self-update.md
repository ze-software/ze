# Self-Update

## Meta

| Field | Value |
|-------|-------|
| Name | Self-Update |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/config/system, internal/plugins/update-cmd, cmd/ze/update_serve.go |
| Real-path tests | test/ui/update-serve.ci |
| Docs | docs/guide/self-update.md |
| Doc review | 2026-10-07: selfupdate.go carries the FNV spread; the backend files named in the anchors exist |
| Defect review | 2026-10-07: open: plan/pre-release/spec-selfupdate-manifest-authenticity.md (manifest not signed) |
| Extra criteria | supported: functional tests for check and download and apply and rollback = none yet; supported: gokrazy A/B update proof = none yet |

## Description

Platform-aware update backend. Normal Linux uses Ze self-update or passive version checking with SHA-256 verified download, atomic binary replacement via rename, `.prev` hard-link rollback, deterministic spread scheduling (FNV-1a per device+version), maintenance windows, server-side pause, and persisted update history. Gokrazy appliances report `backend: gokrazy-ab` through the gokrazy backend and keep the manual `update system firmware {check,download,apply,restart,rollback}` command family wired; unsupported operations report that system image updates are managed by gokrazy. Minimal (no-tag) builds return an explicit `self-update unavailable in minimal build` response. `ze update serve` standalone server remains available on Ze-managed platforms to distribute artifacts. Config: `system { update-check { auto-apply true; spread 1800; maintenance-window { start 02:00; end 06:00 }; restart { time 03:00 } } }`. <!-- source: internal/component/config/system/backend.go -- UpdateBackend interface and active backend --> <!-- source: internal/component/config/system/backend_ze_distro.go -- zeBackend delegates to UpdateChecker/selfUpdater --> <!-- source: internal/component/config/system/backend_ze_appliance.go -- stripped backend unsupported status --> <!-- source: internal/component/config/system/backend_gokrazy.go -- gokrazyBackend status and unsupported operations --> <!-- source: cmd/ze/update_serve.go -- standalone update server with enhanced manifest --> <!-- source: gokrazy/ze/config.json -- PackageConfig.GoBuildTags --> <!-- source: internal/plugins/update-cmd/cmd/firmware.go -- update system firmware CLI handlers --> <!-- source: internal/component/cmd/update/yang/register.go -- update schema registration -->
