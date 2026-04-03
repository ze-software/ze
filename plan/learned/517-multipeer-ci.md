# Learned: multipeer-ci

## What Was Done

Fixed multi-peer `.ci` test infrastructure so ze actually connects to all
peers, not just the first one.

### Root Cause

Two existing multi-peer tests (`role-otc-egress-stamp`, `community-strip`)
used `--port $PORT2` for the second ze-peer on 127.0.0.2. But
`applyPortOverride` (`peers.go:438`) sets ALL peer ports to the
`ze_bgp_tcp_port` value (`$PORT`). Ze tried to connect to 127.0.0.2:$PORT
but the sink peer was on 127.0.0.2:$PORT2. Connection never established.
Tests passed because the Python plugin validated adj-rib-in (source peer),
not the sink peer.

### Fix

All ze-peers use `--port $PORT` on different loopback IPs (127.0.0.1,
127.0.0.2). The port override correctly sets all peer ports to `$PORT`.

### Per-Process Output Tracking

The runner previously shared a single `syncWriter` and `strings.Builder`
across all ze-peer background processes. The `syncWriter.found` flag is
permanent -- once the first peer prints "listening on", `WaitFor` returns
true immediately for the second peer. Fixed by giving each ze-peer its own
`peerOutput` struct (syncWriter + stderr builder + process pointer).

Also fixed `peerProc` tracking: previously only the last ze-peer was
assigned to `peerProc`, so the runner only waited for one peer. Now waits
for all peer processes.

### Platform Portability

Linux routes 127.0.0.0/8 to lo automatically. macOS/FreeBSD only have
127.0.0.1 by default. Added `loopback_linux.go` (no-op) and
`loopback_darwin.go` (`SIOCAIFADDR` ioctl on lo0) to add aliases.

## Key Decision

Differentiate peers by IP, not port. In BGP, peers are identified by IP.
Test infrastructure mirrors this. The `ze_bgp_tcp_port` override sets a
uniform port for all peers, which is correct when all peers listen on the
same port.

## Mistakes

- Original spec proposed `$PORT3`/`$PORT4`, port suppression option, and
  conditional port allocation. All unnecessary once the IP-based approach
  was identified.
- Assumed existing multi-peer tests worked. They passed but the second peer
  was never connected to.

## Files Changed

- `internal/test/runner/runner_exec.go` -- `peerOutput` struct, per-process tracking
- `internal/test/runner/loopback_linux.go` -- no-op (new)
- `internal/test/runner/loopback_darwin.go` -- SIOCAIFADDR ioctl (new)
- `internal/test/runner/loopback_test.go` -- unit tests (new)
- `test/plugin/role-otc-egress-stamp.ci` -- `$PORT2` -> `$PORT`
- `test/plugin/community-strip.ci` -- `$PORT2` -> `$PORT`
- `docs/architecture/testing/ci-format.md` -- multi-peer pattern docs
