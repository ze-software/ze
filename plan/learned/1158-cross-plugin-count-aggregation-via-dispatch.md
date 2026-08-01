# cross-plugin-count-aggregation-via-dispatch

`show bgp summary` needed per-peer route counts, but those counts are owned by the
`bgp-rib` plugin (Adj-RIB-In/Out sizes), and `cmd/peer` must not import it
(`ai/rules/plugin-self-containment.md`). The reusable shape: an aggregating
command handler reads another plugin's state by RUNTIME COMMAND DISPATCH, keyed
by a string constant, never a Go import.

## The pattern

`handleBgpSummary` (`cmd/peer/summary.go`) calls
`ctx.Dispatcher().ForwardToPlugin(ctx, "show bgp rib status", nil, "")`, gets a
`plugin.RawJSON` response, unmarshals the per-peer `route-counts` map, and merges
it into each peer row. The RIB plugin owns the numbers and exposes them in its own
`status()` output; the caller only aggregates. This is the same shape
`cmd/rib/rib.go` already used to proxy `show bgp rib` commands.

Three properties that make it correct:

- **String-keyed, not imported.** `cmdRibStatus = "show bgp rib status"` is a
  const. `make ze-plugin-boundary-check` stays green because there is no
  compile-time edge from `cmd/peer` to `bgp/plugins/rib`.
- **Best-effort by construction.** `ForwardToPlugin` returns `ErrUnknownCommand`
  when the plugin is not loaded (`command.go:795-801`). The merge treats a nil
  result as "omit the keys," so the summary still renders on a build without the
  RIB. Do NOT fake a 0 for an absent owner — an omitted key and a real 0 mean
  different things to a consumer.
- **The response is `plugin.RawJSON` (a string type), not a Go map.** A plugin
  forward round-trips through JSON over the `net.Pipe` (`routeToProcess` sets
  `Data: plugin.RawJSON(rpcOut.Data)`, `command.go:895`). Parse it with
  `json.Unmarshal([]byte(raw), ...)`. Internal BGP plugins are in-process
  goroutines but still communicate ONLY by RPC over a pipe — there are no shared
  Go objects to read directly.

## Testability: split the seam

The dispatch itself cannot be faked in a `cmd/peer` unit test (the test server has
a real dispatcher but no plugin processes, so `ForwardToPlugin` always returns
`ErrUnknownCommand`). So factor the logic into pure functions and test those:
`parseRibRouteCounts([]byte) map` and `mergeRibRouteCounts(row, addr, counts)` are
unit-tested directly; the "no RIB -> omit" degradation is tested through the real
handler with the default context; and the ONE remaining seam (the live dispatch +
JSON round-trip) is proven by a functional `.ci` with a real peer and the RIB
loaded. Unit-testing the parse+merge and `.ci`-testing the wire is the split that
gives full coverage without a fake plugin process.

## Honest zero: do not fabricate a count you cannot produce

Alice-LG reads `routes_filtered`. Ze drops import-rejected routes at the reactor
gate (`reactor_notify.go:449`) and never stores them (`project-knowledge.md`: "No
filtered/noexport route tracking"). The temptation was to derive `filtered =
received - accepted`; that conflates policy rejects with loop/limit/malformed
drops and would report a number that is wrong rather than absent. The honest move
is to leave `filtered` at 0 (matching the existing `handleAPIRoutesFiltered`
stub) and document why. `ai/rules/no-fabrication.md` and
`no-workarounds-for-missing-behavior.md` both point the same way: an absent value
stays absent.

## Trap: the "obvious" source can race

The first design took `received` from the reactor's pre-policy `prefixCounts`
(`session_prefix.go`). But `Peers()` reads under `r.mu.RLock` while the session
loop writes `prefixCounts` without that lock — a data race. Making it safe needed
new hot-path atomics + session->peer plumbing, for a signal that is imprecise
(filtered=0) and already on the `ze_bgp_prefix_count` gauge. The lower-risk design
took all counts from ONE owner (the RIB, whose sizes are exact and already
per-peer), accepting that `received == accepted`. When a counter you want to
surface lives on a hot path guarded by a different lock than your reader, prefer a
source that already exposes it safely over adding cross-goroutine plumbing.

## Traps for the next agent

- `bgp-rib` only populates its per-peer Adj-RIB-In (`bgpPeers`) when wired
  `receive [ update ]`. Many `.ci` configs give it `receive [ state ]` only, so
  its `route-counts` would be empty there. The functional test must wire
  `receive [ update state ]`.
- There are TWO RIB-ish plugins: `bgp-rib` (`plugins/rib`, the main RIB with
  bgpPeers + ribOut) and `bgp-adj-rib-in` (`plugins/adj_rib_in`, raw-hex replay
  storage). The summary counts come from `bgp-rib`; do not confuse the two.

## Files

None recorded.
