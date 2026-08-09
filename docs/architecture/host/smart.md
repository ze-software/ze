# SMART disk health

The host inventory reports SMART health for each block device it detects. The
data comes from direct ATA and NVMe ioctls in `internal/core/smart/`. Ze runs no
external tool for it, so a host needs no `smartmontools` package.

<!-- source: internal/component/host/smart_linux.go -- detectSMART, the per-device delegation -->
<!-- source: internal/core/smart/smart.go -- Info, ParseNVMeBuf -->
<!-- source: internal/core/smart/smart_linux.go -- the ATA and NVMe passthrough ioctls -->
<!-- source: internal/component/host/storage_linux.go -- where SMART joins a block device -->

## Decisions

- **The host files are a delegation, not an implementation.** `detectSMART` is
  one call to `smart.Detect`. The library lives in `internal/core/` because the
  storage component needs it too, and a core package avoids a
  component-to-component import.
- **Detection is Linux only, and the type is not.** `smart.Info` is declared in
  the untagged file, so a caller compiles everywhere. The ioctls live behind
  `//go:build linux`, and storage detection returns `ErrUnsupported` off Linux,
  which makes the detection path unreachable there.
- **A device that cannot report SMART is marked unavailable, not nil.** Nil
  means detection was not attempted. The unavailable flag with its note means
  the device cannot answer. A caller that cannot tell those apart cannot alert
  correctly.

## Related

- `../storage/smart-health.md` for the YANG-modeled management surface: polling,
  temperature thresholds, self-test scheduling, and `show storage smart`
- `inventory.md` for the storage section this extends
