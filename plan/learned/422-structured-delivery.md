# 422 -- Structured Event Delivery for Internal Plugins

## Context

Internal plugins (rib, adj-rib-in, rs, gr, rpki, watchdog, persist) received BGP events as JSON text strings via DirectBridge. The engine formatted `PeerInfo` + `RawMessage` into JSON text (`formatMessageForSubscription`), and each plugin parsed that JSON back into Go structs (`ParseEvent` -- 5+ `json.Unmarshal` calls, 172 allocs per UPDATE). For a full table (800k prefixes), this added significant latency and GC pressure. The engine already had the data in structured form -- the JSON round-trip was pure waste.

## Decisions

- **Chose `StructuredEvent` with metadata fields + `RawMessage` pointer** over delivering `FilterResult` (which forces eager `wire.All()` parsing of every attribute). Each plugin reads only what it needs via lazy `AttrsWire.Get()` accessors.
- **Extended structured delivery to all message types** (not just UPDATE) over keeping the `isUpdate` guard. This means all internal plugins handle all their subscribed event types via `OnStructuredEvent`, eliminating JSON parsing for state events too.
- **Kept `OnEvent` as fallback** alongside `OnStructuredEvent` for external plugin compatibility and for event types where wire decoding requires the `format` package (import cycle: `format` imports `plugin`, so plugins can't import `format`). RS and GR handle OPEN/refresh via text OnEvent for this reason.
- **Pooled `StructuredEvent`** (same pattern as the replaced `StructuredUpdate`) to eliminate per-event heap allocation on the hot path.
- **Deleted `StructuredUpdate`** entirely (no-layering rule) rather than keeping both types.

## Consequences

- UPDATE delivery to internal plugins is 415x faster and uses 86x fewer allocations (benchmark: 196 ns/op vs 81,400 ns/op, 2 allocs vs 172).
- Rib plugin reads raw attribute bytes directly from `AttrsWire.Packed()` and NLRI bytes from `WireUpdate.NLRI()` -- no hex encode/decode round-trip for pool storage.
- RPKI plugin parses only AS_PATH (1 attribute) from `AttrsWire.Get()` instead of 5+ `json.Unmarshal` calls.
- RS now handles OPEN structurally using `capability.Parse()` directly (avoiding the `bgp/format` import cycle). GR handles OPEN structurally by walking raw capability bytes for codes 64 (GR) and 71 (LLGR) via `message.UnpackOpen` (no `capability.Parse` or `format` import needed).
- `adj-rib-in` structured UPDATE handler builds a `bgp.Event` from wire sections -- still hex-encodes for the existing `handleReceived` storage logic, but eliminates the JSON round-trip (engine doesn't format, plugin doesn't `ParseEvent`).

## Gotchas

- Registering `OnStructuredEvent` makes `HasStructuredHandler()` return true, causing the engine to deliver ALL events as `StructuredEvent` (not just UPDATEs). Any event type the handler doesn't process is silently dropped -- the `OnEvent` handler does NOT fire for those events. This caused a critical bug during development where UPDATE events were dropped for rib/adj-rib-in/rpki/persist.
- `goimports` cannot auto-resolve aliased imports (`bgptypes`, `bgpctx`). These must be added manually after every edit, or the linter removes them and the next build fails.
- `bgp/format` imports `component/plugin`, creating a cycle if any plugin imports `format`. Solved for RS by using `capability.Parse()` directly. GR keeps OPEN on text path.
- NLRI wire-to-prefix conversion needs a stack-allocated `[16]byte` buffer (not `make([]byte)`) to avoid per-prefix heap allocation. Applied across rib, adj-rib-in, rpki, persist.
- Pre-existing `NewServer` signature change (added error return) broke 14 test files across the codebase. These had to be fixed as part of this work to unblock lint.

## Files

- `pkg/plugin/rpc/bridge.go` -- `StructuredEvent` type + pool (replaced `StructuredUpdate`)
- `pkg/plugin/rpc/bridge_test.go` -- pool and delivery tests
- `internal/component/bgp/server/events.go` -- engine dispatch builds `StructuredEvent`
- `internal/component/bgp/server/events_bench_test.go` -- JSON vs structured benchmark
- `internal/component/bgp/plugins/rib/rib_structured.go` -- wire-level UPDATE handling (new file)
- `internal/component/bgp/plugins/rib/rib.go` -- `OnStructuredEvent` registration
- `internal/component/bgp/plugins/rs/server.go` -- migrated from `StructuredUpdate`
- `internal/component/bgp/plugins/adj_rib_in/rib.go` -- structured state + UPDATE
- `internal/component/bgp/plugins/gr/gr.go` -- structured state
- `internal/component/bgp/plugins/rpki/rpki.go` -- structured UPDATE (AS_PATH from wire)
- `internal/component/bgp/plugins/watchdog/watchdog.go` -- structured state
- `internal/component/bgp/plugins/persist/server.go` -- structured state + UPDATE + OPEN
- `docs/architecture/api/process-protocol.md` -- documented structured delivery path
- `ai/rules/plugin-design.md` -- StructuredEvent field table
- `docs/architecture/core-design.md` -- updated receive path diagram
