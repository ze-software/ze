# 695 -- host-3-smart

## Context

The host inventory detection (`ze host show storage`) reported block device metadata (size, model, serial, transport, rotational) but had no SMART health data. Operators monitoring disk health had to run smartctl separately. SMART detection was deferred from spec-host-0-inventory because it adds an external tool dependency (smartmontools).

## Decisions

- **Map-based JSON extraction over struct tags** for parsing smartctl output. smartctl emits snake_case keys which conflict with ze's kebab-case JSON convention enforced by the `check-json-kebab` hook. Using `map[string]json.RawMessage` extraction avoids struct tags entirely.
- **Cross-platform parsing, linux-only exec.** `parseSMARTJSON` lives in `smart.go` (no build tag) because it is pure JSON logic. `detectSMART` lives in `smart_linux.go` because it uses `exec.LookPath` and `exec.CommandContext`. No `smart_other.go` stub needed because `DetectStorage` itself returns `ErrUnsupported` on non-Linux, so `detectSMART` is never reached.
- **Nil SmartInfo when smartctl absent** rather than an error. Missing smartctl is a normal deployment state (not all hosts install smartmontools). An error would pollute the inventory's Errors array for expected conditions.
- **Unavailable flag for non-SMART devices** rather than nil. Devices that exist but lack SMART support (USB drives, virtual disks) get `SmartInfo{Unavailable: true, UnavailableNote: "..."}` so the caller can distinguish "not attempted" (nil) from "device cannot report" (Unavailable).

## Consequences

- `ze host show storage` now includes SMART data per device when smartctl is installed.
- Existing storage tests and JSON output are backward-compatible (Smart is omitempty).
- SMART detection adds ~100ms per device (smartctl exec). For hosts with many block devices this could be noticeable.

## Gotchas

- smartctl exit status bit 0x02 means "device does not support SMART" but smartctl still produces valid JSON output. The parser must handle non-zero exit with partial JSON.
- The 10-second timeout per device prevents a hung smartctl from blocking inventory detection indefinitely.

## Files

- `internal/component/host/inventory.go` -- SmartInfo type, Smart field on StorageDevice
- `internal/component/host/smart.go` -- parseSMARTJSON and JSON extraction helpers (cross-platform)
- `internal/component/host/smart_linux.go` -- detectSMART exec wrapper (linux-only)
- `internal/component/host/storage_linux.go` -- wires detectSMART into readBlockDevice
- `internal/component/host/smart_test.go` -- 5 tests covering healthy, unsupported, SATA with errors, invalid JSON, JSON serialization
- `docs/guide/command-reference.md` -- storage section updated with SMART fields
