# 1269 -- hostload contention single source

## Context

Two loose threads from the netlink/QEMU recovery tail (handover 21). First, the
`ze-qemu-debug` and `ze-qemu-shell` targets built the DUT `$(ZE_QEMU_BIN)` with
`ze_core zetest ze_distro` only -- missing `ze_setup $(ZE_FEATURES)`. `zetest`
pulls in the test-only `fakeddos` plugin, whose YANG imports `ze-ddos-detect-conf`
(owned by `ze_ddos`, a member of `$(ZE_FEATURES)`), so every config load through
those targets died "no such module: ze-ddos-detect-conf". This is the same defect
class as [[1258-qemu-gate-ran-a-stripped-daemon]], which fixed it on
`ze-qemu-all-test` / `ze-netns-qemu-test` but left these two behind, blocking
validation of `ospf-interface-runtime` (33) and `ospf-route-daemon` (66).

Second, contention detection was implemented in TWO places that had already
drifted: `internal/test/runner/hostload.go` (`Contended()` requires
`LoadAvg1 > CPUs` AND concurrency) and `scripts/status/verify_run.go`
(`contendedWarning()` fired on process concurrency ALONE, no load gate), with the
proc-count and digit-parse helpers copy-pasted between them.

## Decisions

- Aligned both QEMU DUT builds with `internal/test/runner` `TestBuildTags`
  (`runner.go:50`) / line 257: `ze_core zetest ze_distro ze_setup $(ZE_FEATURES)`.
  Made the in-recipe comment reference the sibling targets by NAME, not line
  number -- inserting the comment had itself shifted the line numbers a reviewer
  then flagged as stale.
- Extracted the contention logic into a new leaf package `internal/core/hostload`
  as the single source of truth (over leaving two copies, or importing the heavy
  `internal/test/runner` into the status tool). The runner now aliases it
  (`type HostLoad = hostload.Load`, preserving the failure-group JSON), and
  `verify_run.go` calls `hostload.Snapshot().Contended()` -- so the status tool
  stops warning "contended" on a quiet many-core box where the runner (correctly)
  said otherwise.
- Added a `!linux && !darwin` fallback (`readLoadAvg1` returns 0) so the leaf
  package builds on any GOOS; verified this keeps `scripts/status` windows-
  buildable, which it was before and would otherwise have regressed.

## Consequences

- `verify_run`'s contended warning is now load-gated: fewer false positives, and
  the two surfaces cannot silently disagree again.
- The DUT-tag drift has now bitten three of the five DUT build lines. The durable
  fix (not taken here) is a single shared make variable, e.g.
  `ZE_QEMU_DUT_TAGS := ze_core zetest ze_distro ze_setup $(ZE_FEATURES) $(ZE_TAGS)`,
  used by all five so a fourth recurrence is structurally impossible.
- `ospf-interface-runtime` (33) and `ospf-route-daemon` (66) both PASS under
  `make ze-qemu-debug` after the fix (QEMU-validated).

## Gotchas

- `zetest` is enough to pull `fakeddos` in (no `ze_ddos` build tag on fakeddos), so
  a DUT with `zetest` but without `ze_ddos` fails EVERY config load, not just
  ddos-related ones -- the failure looks unrelated to the feature under test.
- `ze-test <suite>` numeric ids are 1-based ordinals over the sorted `.ci` glob and
  renumber when a file is added/renamed; select by `--pattern <name>` (substring on
  id/name/path) for stability. `-p` is the PARALLEL flag, not pattern.
- Two detectors that "look the same" drift silently: the load-gate divergence
  existed unnoticed until the extraction forced one definition.

## Files

- `mk/test-integration.mk` -- `ze-qemu-debug` / `ze-qemu-shell` DUT tags + comment
- `internal/core/hostload/` -- new leaf package (impl + platform splits + test)
- `internal/test/runner/hostload.go` -- alias + near-timeout classifier only
- `internal/test/runner/hostload_{linux,darwin}.go` -- deleted (moved to core)
- `scripts/status/verify_run.go` -- `contendedWarning` uses `hostload.Snapshot`
