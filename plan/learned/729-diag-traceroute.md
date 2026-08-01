# 729 -- diag-traceroute

## Context

Gokrazy appliances have no `traceroute` binary. Ze already had a `ze traceroute` offline wrapper for the OS tool, but the daemon path (`show traceroute`) was missing. Operators need path tracing from within an SSH CLI session or MCP tool call without exec'ing external binaries.

## Decisions

- Implemented as pure Go raw socket handler (same pattern as ping.go) over using a third-party library, because the codebase already had ICMP echo infrastructure (buildICMPEcho, icmpChecksum) and adding ~170 lines was simpler than evaluating and maintaining a dependency for ~200 LOC of logic.
- Used `golang.org/x/net/ipv4` and `ipv6` PacketConn wrappers for per-hop TTL control over raw syscall, because x/net was already a dependency and provides cross-platform TTL/HopLimit setting.
- Introduced `ttlSetter` interface to abstract IPv4 SetTTL vs IPv6 SetHopLimit behind a single method, over separate code paths, because the doTraceroute loop body is identical for both protocols.
- Added `argTimeout` constant to show.go (alongside existing `argCount`) to satisfy goconst across ping, tcp-check, and traceroute handlers.
- Removed the offline `ze traceroute` OS wrapper (`cmd/ze/diag/`) over keeping both paths, because on gokrazy there is no OS traceroute to wrap, and on regular Linux the user can just run `traceroute` directly. The daemon `show traceroute` is the only path needed.

## Consequences

- The `show traceroute` command works on all platforms with CAP_NET_RAW, matching `show ping`.
- IPv6 traceroute works out of the box via the same code path (protocol detection by `dest.Is6()`).
- No new dependencies added to go.mod.
- The `ttlSetter` interface could be reused if future diagnostic commands need per-packet TTL control.

## Gotchas

- Echo reply ID/seq verification is straightforward (bytes 4-7 of reply), but Time Exceeded verification requires parsing the embedded original IP header (variable IHL) to reach the original ICMP header. Skipped for Time Exceeded; the diagnostic context makes false positives acceptable.
- The `goconst` linter flagged `"timeout"` as a repeated string because `tcpCheckResultTimeout` already existed as a constant with value `"timeout"`, but that constant represents a result status, not a CLI argument keyword. The fix was a new `argTimeout` constant for the CLI argument use case.

## Files

- `internal/component/traceroute/cmd/traceroute.go` (created)
- `internal/component/traceroute/cmd/traceroute_test.go` (created)
- `internal/component/cmd/show/show.go` (modified: RPC registration, argTimeout constant)
- `internal/component/ping/cmd/ping.go` (modified: use argTimeout, add Related comments)
- `internal/plugins/diag/cmd/tcp_check.go` (modified: use argTimeout)
- `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` (modified: traceroute container)
- `internal/plugins/diag/diag.go` (modified: removed tracerouteSpec and RunTraceroute)
- `internal/plugins/diag/register.go` (modified: removed traceroute registration)
- `internal/plugins/diag/diag_test.go` (modified: removed RunTraceroute test)
- `cmd/ze/main.go` (modified: updated comments)
- `ai/patterns/cli-command.md` (modified: updated diag register.go description)
- `test/plugin/show-traceroute.ci` (created)
- `docs/features.md` (updated: core diagnostics count and description)
- `docs/guide/command-reference.md` (updated: traceroute section)
- `docs/architecture/api/commands.md` (updated: traceroute RPC)
- `docs/guide/production-diagnostics.md` (updated: traceroute in quick ref and BGP troubleshooting)
