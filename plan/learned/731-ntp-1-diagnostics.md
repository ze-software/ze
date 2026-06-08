# 731 -- ntp-1-diagnostics

## Context

The NTP plugin queried NTP servers and stepped the clock but discarded all response metadata.
Operators had no visibility into NTP sync status, server reachability, or clock offset.
The clock was always stepped (Settimeofday) even for small offsets, causing time jumps that
disrupted log ordering and timer-based protocols.

## Decisions

- Chose `pluginserver.RegisterRPCs()` from the NTP plugin's `init()` over a leaf state package
  or registry-based provider, because 53 other packages already use this pattern and it avoids
  the show package importing the NTP plugin directly.
- Chose `registry.SetNTPSyncProvider()` for `show system date` enrichment over importing
  the NTP plugin, following the existing `SetMetricsRegistry`/`GetMetricsRegistry` pattern.
- Chose `Adjtimex` with `ADJ_OFFSET` for slew over `ClockAdjtime`, because `syscall.Adjtimex`
  is available in the stdlib without `golang.org/x/sys/unix`.
- Chose `decideClockAction()` as a pure function returning slew/step/reject over inlining the
  logic, to make the decision testable independent of syscalls.
- Query all servers per cycle over picking one random server, to populate per-server diagnostics.

## Consequences

- `show system ntp` and `show system ntp peers` provide full NTP observability.
- `show system date` now includes `ntp-synced`, `ntp-source`, `ntp-offset` when NTP is enabled.
- `slew-threshold` config leaf (default 128ms) controls slew/step boundary; 0 disables slew.
- Per-server reach bitmap (8-bit shift register) tracks reachability history.
- Future `monitor ntp` can read the same `globalState` atomic pointer.

## Gotchas

- `clock_other.go` (non-Linux stub) must also define `slewClock` or compilation fails on macOS.
- `fmt.Sprintf` is banned on non-error paths; used `strconv.Itoa` + string concatenation instead.
- The lint hook fires on every intermediate edit; all new functions and types must be used by
  the end of the same edit or the next immediate edit.

## Files

- `internal/plugins/ntp/state.go` (new) -- syncState, serverState, reachShift, selectBestServer
- `internal/plugins/ntp/ntp.go` -- doSync rewritten to query all servers, slew/step decision
- `internal/plugins/ntp/register.go` -- RPC registration, show handlers, NTP sync provider
- `internal/plugins/ntp/clock_linux.go` -- slewClock via Adjtimex
- `internal/plugins/ntp/clock_other.go` -- slewClock stub
- `internal/plugins/ntp/yang/ze-ntp-conf.yang` -- slew-threshold leaf
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` -- ntp containers under system
- `internal/component/cmd/show/system.go` -- handleShowSystemDate NTP enrichment
- `internal/component/plugin/registry/registry.go` -- NTP sync provider registration
- `test/plugin/show-system-ntp.ci` -- functional test
