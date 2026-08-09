# SMART Disk Health Management

YANG-modeled SMART management: auto-enable, periodic health polling, three-tier
temperature alerting, self-test scheduling, and live status through `show
storage smart`. `docs/architecture/host/smart.md` covers the host inventory's
per-device view, which delegates to the same core library.

<!-- source: internal/core/smart/smart.go -- Info, ParseNVMeBuf, NvmeNamespace -->
<!-- source: internal/core/smart/smart_linux.go -- ATA and NVMe passthrough ioctls -->
<!-- source: internal/component/storage/manager.go -- Manager, poll loop, thresholds, self-test scheduling -->
<!-- source: internal/component/storage/config.go -- Config, TemperatureConfig, SelfTestConfig -->
<!-- source: internal/component/storage/discover_linux.go -- sysfs device enumeration -->
<!-- source: internal/component/storage/show.go -- ze-show:storage-smart -->

## The decisions

**The ioctl library is `internal/core/smart/`, not inside the storage
component.** The host detector also needs `Detect`, for `ze host show storage`.
Core placement avoids a component-to-component import. The host files stay as
one-line wrappers that delegate to `core/smart.Detect`, which preserves the
detector's testdata-mode routing with zero duplication.

**Pure ioctl. No smartctl dependency.** `HDIO_DRIVE_CMD` for ATA and admin
passthrough for NVMe. A gokrazy image carries no smartctl.

**Storage has no `register.go`.** It is a system service, not a plugin. The hub
creates it, starts it, reconfigures it and stops it directly, matching the
archive scheduler. Plugin-style registration would add indirection for nothing.

**The show RPC reads an `atomic.Pointer[storage.Manager]` the hub sets.** When
the pointer is nil, meaning no `storage { smart { ... } }` in config, the
handler returns the stable error `storage SMART management not configured`. The
error is part of the contract and has its own functional test.

**Scheduling fails open.** An invalid time-of-day or an unrecognized day name
matches always. An operator who mistypes a time gets self-tests at any time
rather than never.

**Health errors dedup through a `healthReported` flag.** A SMART-failing device
raises one error per failure episode and clears it on recovery, instead of
flooding while the failure persists.

A doctor check warns when `storage.smart.enabled` is true and no enumerated
sysfs device is SMART-accessible.

## Traps in the device protocols

**ATA self-test in progress is data offset 363, bits 7:4, value 0x0F.** Any
other non-zero value there means completed-with-error, not in-progress.

**The NVMe self-test log is page 0x06.** Page 0x02 is SMART/Health. They are
separate Get Log Page calls.

**Crossing the critical temperature threshold must clear the informational
`temp-high` warning explicitly.** Otherwise the same device carries stale
warnings at two severity levels at once.

**A removed device must have its warnings cleared.** The poll tracks the devices
it has seen and clears the warnings of any that disappeared, or the alerts
outlive the hardware.
