# 1143 -- ze support (tech-support bundle)

## Context

Ze had `ze doctor` for health checks but no way to package system state into a single
artifact for support cases. Network operators need to share diagnostic state with
remote support (or other engineers) without running dozens of commands manually.
Surveyed 10 NOS vendors (Cumulus, Cisco, Juniper, SONiC, Arista, VyOS, Nokia,
MikroTik, FRR, OpenWrt) to identify best-in-class patterns before designing.

## Decisions

- Chose `ze support` as a standalone root command in SectionSystem, over putting it
  under `ze generate`, because it parallels `ze doctor` (both are system-level,
  offline, noun-form commands).
- Chose Cumulus-style named module system over Cisco-style subsystem variants,
  because Ze's registration pattern (single map, derive everything) maps directly.
- Chose privacy-by-default (redact passwords/secrets) over Nokia-style encryption,
  because operators need to read the bundle for self-service troubleshooting.
- Chose JSON-per-module archive structure over flat text dumps, because no NOS vendor
  offers machine-parseable per-module output. This is Ze's primary differentiator.
- Deferred logs/routing/firewall/vpp/ipsec/plugins modules to follow-up work,
  because each needs Linux-specific tooling or daemon RPC infrastructure.

## Consequences

- Adding a new support module requires one line in `moduleRegistry` (modules.go).
  Help text, validation, and --list-modules all derive automatically.
- The module collector interface (`func(*CollectOptions) (any, error)`) makes it
  trivial to add daemon-dependent modules later with graceful degradation.
- Config sanitization (`sanitize.go`) is reusable for any future feature that needs
  to strip sensitive values from config trees.
- The `--since` flag is parsed and threaded through CollectOptions but has no
  consumer until the logs module is added.

## Gotchas

- Pre-existing build breaks (FormatSchemaStamp signature change, RecoverConfig
  signature change) blocked compilation and had to be worked around.
- The linter flags type assertions in tests as errcheck violations; need helper
  functions (mustMap/mustSlice) for safe assertions.
- `disk_unix.go` uses `syscall.Statfs_t` which has platform-specific field types
  (`Fsid.Val` is `[2]int32` on Darwin, may differ on other Unix variants).

## Files

- `internal/plugins/support/register.go` -- command registration
- `internal/component/support/support.go` -- Run(), collect(), archive creation, module collectors
- `internal/component/support/modules.go` -- module registry, filtering, listing
- `internal/component/support/sanitize.go` -- config tree sanitization
- `internal/component/support/disk_unix.go` -- disk usage collection (Unix)
- `internal/component/support/support_test.go` -- 12 unit tests
- `internal/component/support/sanitize_test.go` -- 5 sanitization tests
- `cmd/ze/main.go` -- blank import for registration
