# 592 -- Operational Commands

## Context

Ze lacked basic operational commands that every network OS provides: show uptime, show interface brief/counters, resolve ping/traceroute, and rib best-path reason narration. These are read-only queries that operators need for day-to-day troubleshooting without leaving the CLI.

## Decisions

- All commands registered via the standard `pluginserver.RPCRegistration` pattern with YANG `ze:command` nodes for tab-completion.
- ping/traceroute delegate to system binaries via `exec.CommandContext` (no raw socket implementation), with fixed command names (not user-controlled) to prevent injection.
- `rib best reason` uses a separate `SelectBestExplain` function that re-runs the comparison with narration, over instrumenting the hot-path `SelectBest`. The reason terminal is CLI-only; the hot path stays allocation-free.
- JSON output shape for reason: per-prefix entry with candidate list and pairwise step array. Each step names incumbent, challenger, winner, deciding step, and human-readable reason string.
- show interface dispatches to `iface.GetInterface`/`iface.ListInterfaces` (existing OS abstraction), not a new backend.

## Consequences

- `rib best reason` is the first CLI command that exposes the RFC 4271 Section 9.1.2 decision process to operators. Useful for debugging why a particular path won.
- ping/traceroute have 15s/30s context timeouts. No rate limiting, which is fine for operator CLI but would need throttling if exposed via API.
- show uptime nil-guards both CommandContext and Reactor for safety before daemon is fully started.

## Gotchas

- The spec was stale: claimed rib best reason was not started, but it was fully implemented with `BestStep` enum, `BestPathExplanation` struct, `SelectBestExplain`, `bestReasonTerminal`, and 11 unit+pipeline tests.
- ping/traceroute have zero unit tests because `execCommand` is a package-level function (not injectable). Testing requires actually running system ping. The .ci test (`resolve-ping.ci`) covers the dispatch path.
- The `comparePairWithReason` function is the single source of truth for both `ComparePair` (hot path, discards narrative) and `SelectBestExplain` (CLI path, records narrative). No risk of divergence.
- The `bestReasonTerminal` uses a candidate stash mechanism: `newBestSource` populates a `map[string][]*Candidate` during iteration so the reason terminal can re-run explanation without re-acquiring the RIB lock.

## Files

- `internal/component/cmd/show/show.go` -- handleShowUptime, handleShowInterface, showInterfaceBrief
- `internal/component/resolve/cmd/resolve.go` -- handlePing, handleTraceroute
- `internal/component/bgp/plugins/rib/bestpath.go` -- BestStep, SelectBestExplain, comparePairWithReason
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` -- bestReasonTerminal, candidatesByKey stash
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- uptime, interface YANG nodes
- `internal/component/resolve/cmd/schema/ze-resolve-cmd.yang` -- ping, traceroute YANG nodes
- `test/plugin/bestpath-reason.ci`, `test/plugin/resolve-ping.ci`
