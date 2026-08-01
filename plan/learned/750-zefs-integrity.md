# 750 -- zefs-integrity

## Context

ZeFS had no mechanism to detect or recover from store corruption. The netcapstring format validated structure on decode (magic, framing, key validity) but could not pinpoint which entry was damaged, recover valid entries from a partially corrupt file, or detect silent bit rot between writes. The capacity-aware framing was designed for in-place updates but flush() always did a full re-encode, wasting that design.

## Decisions

- Chose per-record CRC32c in the netcapstring header over a whole-file checksum stored as a key, because the container is itself a netcapstring so per-record CRC gives both entry-level pinpointing and whole-file coverage in one mechanism.
- Chose CRC32c (Castagnoli) over SHA-256 because the threat model is accidental corruption, not adversarial tampering, and CRC32c is hardware-accelerated on arm64/amd64.
- Chose in-place pwrite for value changes within capacity over always doing atomic temp+rename, because it finally delivers on the netcapstring capacity design intent. Full rewrite is still used when layout changes (add/remove entries, capacity growth).
- Chose to export `EncodeNetcapstring` (was lowercase) to support the `ze data encode` CLI command.
- Repair writes to a new file, never overwrites the corrupt original.

## Consequences

- The netcapstring header format changed: `<number>:<cap>:<used>:<crc>\n`. Header is 9 bytes larger per netcapstring. No backward compatibility needed (pre-production).
- `decodeNetcapstringRef` now verifies CRC on every decode. Corrupt data is rejected at load time, never silently served.
- `flush()` has two paths: `flushInPlace` (pwrite dirty entries + container CRC) and `flushFull` (atomic temp+rename). Decision is based on dirty tracking fields (dirty set, added list, layoutChanged flag).
- `ze data check` and `ze data repair` provide CLI tooling for operators. `ze doctor` includes store integrity in its readiness checks.
- In-place pwrite is not atomic. A crash mid-write produces CRC-detectable corruption. This is a weaker durability guarantee than temp+rename, acceptable because CRC detection + repair provides a recovery path.

## Gotchas

- The auto-linter hook blocks `fmt.Sprintf` in non-test code. CLI output to stdout/stderr must use `fmt.Fprintf(os.Stdout/Stderr, ...)` and suppress `errcheck` with `//nolint:errcheck // CLI output`.
- The `nilerr` linter flags returning `nil` error when an `err` variable is in scope and non-nil. Check/Repair intentionally return partial reports with `nil` error (the report describes the corruption). Requires `//nolint:nilerr` with justification.
- Container CRC must be computed after all entries are written (it covers the entry bytes). The encode path writes a placeholder CRC, writes entries, then patches the CRC.

## Files

- `pkg/zefs/netcapstring.go` -- CRC32c in header format (write/read/verify)
- `pkg/zefs/store.go` -- dirty tracking, in-place flush path, container CRC
- `pkg/zefs/check.go` -- Check(), Repair(), CheckReport, RepairReport
- `pkg/zefs/pwrite_unix.go` -- pwriteRegions for in-place writes
- `pkg/zefs/pwrite_other.go` -- fallback stub for non-unix
- `internal/component/config/storage/cli/cmd_integrity.go` -- check, repair, encode CLI commands
- `internal/component/doctor/doctor.go` -- store integrity check
- `docs/architecture/zefs-format.md` -- format documentation updated
