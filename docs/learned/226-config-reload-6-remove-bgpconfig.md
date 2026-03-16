# 226 — Config Reload 6: Remove BGPConfig

## Objective

Eliminate the typed `BGPConfig` intermediate struct from the config pipeline, replacing it with direct `map[string]any` passing from `ResolveBGPTree()` through to the reactor.

## Decisions

- `bgp.go` was NOT fully deleted — route config types (`StaticRouteConfig`, etc.) and `FamilyMode` remain because they are used by the route pipeline, which was out of scope for this change.
- `peers.go` and `resolve.go` were created as new files during the refactor — not listed in the original plan, but the concern split made them necessary for modularity.
- `api_sync.go` nil channel bug: `SetAPIProcessCount(0)` left `startupComplete` as nil because the zero case bypassed channel initialization. Fixed by always initializing the channel.
- Double file read in the coordinator path is a conscious correctness-over-optimization trade-off: reading the file twice (once for verify, once for apply) ensures both phases see the same on-disk state at their respective times.

## Patterns

- When eliminating a typed intermediate struct, check all consumers — some may rely on fields that live in the struct for reasons unrelated to the primary concern being removed.

## Gotchas

- The nil channel bug in `api_sync.go` was subtle: `SetAPIProcessCount(0)` looks like a valid no-op but left the system unable to signal startup completion, causing hangs in tests.
- Import cycle prevents static route parsing from moving to the reactor package — this is an ongoing architectural constraint.

## Files

- `internal/component/config/resolve.go` — ResolveBGPTree() producing map[string]any (new file)
- `internal/component/config/peers.go` — peer parsing from map[string]any (new file)
- `internal/component/bgp/reactor/api_sync.go` — nil channel bug fix
