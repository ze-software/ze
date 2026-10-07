# Storage SMART Management

## Meta

| Field | Value |
|-------|-------|
| Name | Storage SMART Management |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/core/smart, internal/component/storage |
| Real-path tests | test/plugin/smart-show.ci |
| Docs | docs/guide/configuration.md, docs/architecture/storage/smart-health.md |
| Doc review | 2026-10-07: no exec of smartctl in internal/core/smart or internal/component/storage; rate-of-change state (prevTemp) present in internal/component/storage/manager.go; the device-level sentences are unproven rather than known false |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/core/smart or internal/component/storage |
| Extra criteria | supported: show storage smart values from a real ATA device = test/appliance/smart-ata-device.ci; supported: show storage smart values from a real NVMe device = test/appliance/smart-nvme-device.ci |

## Description

YANG-modeled SMART disk health: auto-enable on detected ATA/NVMe devices, periodic health polling with three-tier temperature alerting (informational warning, rate-of-change warning, critical error) via report bus, scheduled self-tests (short daily, extended weekly with day-of-week constraint), in-progress detection (skips duplicate tests), live status via `show storage smart` (per-device health, temperature, power-on hours, error count, NVMe percent-used/available-spare, self-test schedule). Pure ioctl (no smartctl binary, gokrazy-safe). Config reload updates intervals live. `ze doctor` verifies SMART accessibility when enabled. First NOS with YANG-modeled SMART management. <!-- source: internal/core/smart/ -- ioctl library --> <!-- source: internal/component/storage/ -- Manager, Config --> <!-- source: internal/component/storage/show.go -- show RPC -->
