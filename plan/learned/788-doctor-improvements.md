# 788 -- Doctor Improvements

## Context

After spec-doctor-coverage filled the first set of missing doctor checks, this spec addressed doctor architecture and diagnostic accuracy: reusing schema-driven listener discovery, running semantic validators from doctor, correcting feature trigger semantics, and adding checks for second-order dependencies discovered during the coverage audit. The goal was to prevent the same class of coverage gaps from returning.

## Decisions

- Chose `RegisterListenerDefault` + `CollectListenersWithDefaults` over modifying the YANG compiler to propagate refine defaults, because the registration pattern is self-contained and doesn't require compiler changes across all consumers.
- Chose schema-driven listener discovery with hardcoded fallback over pure hardcoded extraction, because the schema path covers new ze:listener services automatically (Prometheus, plugin-hub, WireGuard) without adding service-specific branches.
- Chose a provider pattern (`diagnostic.RegisterDoctorProvider`) for `show doctor` over importing `cmd/ze/doctor` from `internal/component/cmd/show`, to keep the dependency direction clean (cmd -> internal, not the reverse).
- Chose UDP SNTP probe for NTP server reachability over TCP connect (which would hit a closed port), matching the daemon's own `checkClockSkew` probe pattern.
- Chose injectable function variables (`httpHead`, `probeWritable`, `ntpServerReachable`) over interface-based mocking for test isolation, consistent with the existing doctor test patterns.

## Consequences

- New ze:listener services added via YANG are automatically covered by doctor when schemas are loaded; no doctor code change is needed.
- Services with empty server lists and registered defaults are still probed by doctor.
- The dependency inventory test (`TestDoctorDependencyInventory`) with its `expectedTotal` constant forces conscious handling of new dependencies.
- `show doctor` is available from SSH CLI sessions, returning the same DoctorResult structure as `ze doctor --json`.

## Gotchas

- `config.YANGSchema()` succeeds with an empty schema when YANG modules aren't registered (unit test context). The schema-driven listener path must check `len(DiscoverListenerServices) > 0` and fall back to hardcoded extraction, otherwise unit tests that build config trees manually get no listener checks.
- The Ze YANG compiler does not process `refine` statements. `LeafNode.Default` is always empty for `uses zt:listener` children. This affects all schema-driven default lookups, not just listener defaults.
- NTP is a client in Ze, not a server. The readiness check should probe configured NTP servers (UDP/123 SNTP), not try to bind UDP/123. Doctor does bind UDP/123 for the NTP listener check (separate from reachability).
- `probeWritableDir` must call `os.Remove` before checking `Close()` error, not after, to avoid leaking temp files.

## Files

- `internal/component/doctor/doctor.go` -- schema-driven listener collection, HTTP probes, writable destinations, update-check/archive checks
- `internal/component/doctor/checks_linux.go` -- NTP clock privilege (CAP_SYS_TIME), VPP DPDK sysfs/VFIO checks
- `internal/component/doctor/checks_other.go` -- stubs for Linux-only checks
- `internal/component/doctor/doctor_test.go` -- tests for all new checks, dependency inventory test
- `internal/component/doctor/register.go` -- doctor provider registration
- `internal/core/diagnostic/codes.go` -- 5 new diagnostic codes
- `internal/core/diagnostic/doctor_provider.go` -- provider pattern for show doctor
- `internal/component/config/listener.go` -- RegisterListenerDefault, CollectListenersWithDefaults
- `internal/component/config/listener_defaults.go` -- builtin listener defaults
- `internal/component/doctor/cmd/show.go` -- show doctor RPC handler
- `internal/component/cmd/show/doctor_test.go` -- show doctor tests
- `internal/component/cmd/show/show.go` -- ze-show:doctor RPC registration
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- show doctor YANG entry
