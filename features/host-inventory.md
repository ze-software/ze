# Host Inventory

## Meta

| Field | Value |
|-------|-------|
| Name | Host Inventory |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/host, internal/plugins/host-cmd, internal/plugins/host |
| Real-path tests | test/parse/cli-host-all-show.ci, test/parse/cli-host-cpu-show.ci, test/parse/cli-host-nic-show.ci, test/parse/cli-host-dmi-show.ci, test/parse/cli-host-memory-show.ci, test/parse/cli-host-thermal-show.ci, test/parse/cli-host-storage-show.ci, test/parse/cli-host-kernel-show.ci, test/parse/cli-host-platform-show.ci, test/parse/cli-host-bogus-show.ci |
| Docs | docs/guide/command-reference.md |
| Doc review | 2026-10-07: the source anchors (inventory.go, show_host.go, host register.go offline fallback) exist; command-reference.md documents show host |
| Defect review | 2026-10-07: no open spec or journal row found naming host inventory |

## Description

Structured hardware inventory for ISP fleet monitoring: CPU (vendor, topology, hybrid P/E layout, scaling driver, frequencies, throttle counts), physical NICs (driver, PCI IDs, link speed, queue counts, firmware, rings), DMI board identity, memory with ECC counters, hwmon thermal sensors + per-CPU throttle, block devices with NVMe firmware, kernel release/cmdline/microcode/arch flags. Read-only sysfs/procfs, no daemon required. Single `show host cpu`/`nic`/... command served by the daemon when reachable, falling back to the same in-process detection when no daemon is running; JSON by default for pipeline consumption. <!-- source: internal/component/host/inventory.go -- Inventory struct, Detector, DetectCPU/NICs/DMI/Memory/Thermal/Storage/Kernel/Host --> <!-- source: internal/plugins/host-cmd/cmd/show_host.go -- online `show host *` RPCs --> <!-- source: internal/plugins/host/register.go -- offline fallback via registry.RegisterOfflineFallback -->
