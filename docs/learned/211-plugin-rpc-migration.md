# 211 — Plugin RPC Migration

## Objective

Convert all internal plugins from raw text-protocol parsing to the SDK callback-based RPC pattern, eliminating hand-rolled command dispatch in each plugin.

## Decisions

- Converted 8 plugins total (bgp-gr, bgp-role, bgp-route-refresh, bgp-softver, bgp-hostname, bgp-rib, bgp-rs, bgp-rr) — bgp-rr was added mid-spec, not in original scope.
- `handleUpdateRouteRPC` retained as bridge: prepends `"bgp peer "` to reconstruct the text command that the reactor expects. This is explicit tech debt — future direct-dispatch will remove this string construction.

## Patterns

- SDK callback pattern (OnConfigure, OnEvent, OnRPC) is dramatically more concise than hand-parsed text protocol — each plugin shrank significantly.
- The RPC bridge pattern (SDK callback → reconstruct text command → existing handler) allows incremental migration without touching reactor dispatch logic.

## Gotchas

- bgp-rr was not in the original spec but was also converted during implementation; the spec was updated retroactively to document this.

## Files

- `internal/component/bgp/plugins/gr/`, `bgp-role/`, `bgp-route-refresh/`, `bgp-softver/`, `bgp-hostname/`, `bgp-rib/`, `bgp-rs/`, `bgp-rr/` — all plugin implementations converted
