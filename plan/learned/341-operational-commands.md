# 341 — Operational Commands

## Objective

Implement missing operational CLI commands that a BGP operator expects: populated peer statistics, BGP summary, peer capabilities query, soft clear, enriched RIB show with attribute detail and filtering, and config diff.

## Decisions

- Peer statistics use atomic counters (`peerCounters` with `atomic.Uint64`/`Uint32`) — lock-free increment from hot paths, snapshot via `Stats()` method
- New handlers live in `bgp_summary.go` with `SummaryRPCs()` registration pattern — follows existing handler file structure
- RIB attribute formatting dereferences pool handles at display time (lazy) via `enrichRouteMapFromEntry()` — keeps storage zero-copy
- Config diff operates on resolved `map[string]any` after `ResolveBGPTree()` — compares effective config, not raw text
- Family/prefix filters parsed from `args` parameter via `parseShowFilters()` — distinguishes family (starts with letter + `/`) from prefix (starts with digit + `/`)

## Patterns

- **Handler registration:** Each handler file provides a `*RPCs()` function returning `[]RPCRegistration`; `register.go` aggregates via `BgpHandlerRPCs()`
- **Clock injection:** All time operations must use `clock.Clock` interface, not `time.Now()` or `time.Since()` — `time.Since(t)` hides a `time.Now()` call internally
- **Pool handle dereference:** `RouteEntry` stores `attrpool.Handle` values; formatting functions call `pool.Get(handle)` to retrieve raw wire bytes, then parse for display
- **Config diff pipeline:** Load → Parse → `ResolveBGPTree()` → `DiffMaps(old, new)` → `ConfigDiff` with `Added`/`Removed`/`Changed` maps using dotted key paths
- **Parse test runner requires `stdin=config`:** `.ci` tests in `test/parse/` must include a `stdin=config` block even for CLI commands; the runner validates config syntax

## Gotchas

- `time.Since(estAt)` calls `time.Now()` internally — broke chaos simulation clock. Fix: `clock.Now().Sub(estAt)`
- Parse test runner interprets `expect=exit:code=N` as config validation result, not CLI exit code — can't test non-zero exit codes for CLI errors in parse tests
- `DiffPair` struct needed explicit JSON tags (`json:"old"`, `json:"new"`) for `--json` output — Go's default would use uppercase field names
- Plugin functional tests (bgp-summary.ci, etc.) require a running daemon with established peers — too complex for parse test runner, deferred

## Files

- `internal/component/bgp/reactor/peer_stats.go` — atomic counters + `PeerStats` struct
- `internal/component/bgp/reactor/reactor_api.go` — populated stats, `PeerNegotiatedCapabilities()`, clock fix
- `internal/component/bgp/handler/bgp_summary.go` — 3 handlers: summary, capabilities, clear soft
- `internal/component/bgp/plugins/bgp-rib/rib_attr_format.go` — attribute formatting from pool handles
- `internal/component/bgp/plugins/bgp-rib/rib_commands.go` — family/prefix filtering, 3-arg handleCommand
- `cmd/ze/config/cmd_diff.go` — config diff CLI handler
- `internal/component/plugin/types.go` — `PeerCapabilitiesInfo`, extended `ReactorIntrospector`
- `internal/component/bgp/types/reactor.go` — `SoftClearPeer` on `BGPReactor` interface
