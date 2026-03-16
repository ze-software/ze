# 321 — Allocation Reduction: Direct Wire Access for In-Process Delivery

## Objective

Eliminate text serialize→deserialize overhead for DirectBridge consumers (bgp-rs in-process). Engine delivers `*RawMessage` directly; bgp-rs reads wire bytes using existing types instead of parsing formatted text strings.

## Decisions

- Chose `rpc.StructuredUpdate{PeerAddress, Event: *RawMessage}` as transport — one field in `EventDelivery.Event` carries both peer address and raw message rather than adding two separate fields
- `prefixBytesToKey` placed inline in `bgp-rs/server.go` — only one consumer, no reason to export to `nlri/` package
- `walkNLRIsAllocating` fallback for non-unicast families — `NLRIIterator` is zero-alloc for unicast; non-unicast uses `NLRIs()` which allocates, acceptable for those families
- Text path (`formatMessageForSubscription`) completely unchanged — fork-mode external plugins continue receiving text events

## Patterns

- Ze lazy pattern: pass `*RawMessage` (raw bytes reference), let consumer call `MPReachWire.Family()` (3-byte read) and `NLRIIterator.Next()` (offset cursor) directly — no intermediate struct, no pre-parsing
- `MPReachWire.Family()` = 3-byte read for forwarding decisions; `NLRIs()` = expensive; `NLRIIterator()` = zero-alloc — always use iterator for NLRI walking
- `EventDelivery.PeerAddress` reuses address string already computed in `events.go` for subscription matching — zero additional allocation
- Import cycle fix: moved `_ "...all"` blank import from `inprocess.go` to `cmd/ze/main.go`

## Gotchas

- Two previous attempts abandoned before the correct design: (1) eager `StructuredEvent` pre-computed `FilterResult` including NLRIs — violated lazy-first (N→1 when answer is N→0-until-needed); (2) `UpdateHandle` wrapper struct with accessor methods — identity wrapper, ze pattern is pass raw data and call existing wire type methods directly
- `RawMessage.IsAsyncSafe()` may return false for received UPDATEs (bytes reference reusable TCP buffer) — must create owned copy before async delivery via `eventChan`
- Unit tests for internal wire extraction functions were skipped — existing functional tests (`rib-withdrawal.ci`, `rib-reconnect.ci`) provide end-to-end coverage

## Files

- `internal/component/bgp/server/events.go` — all three event functions rewritten
- `internal/component/bgp/plugins/rs/server.go` — structured path fully rewritten with wire types
- `internal/component/plugin/process_delivery.go` — `Event any` field added to `EventDelivery`
- `pkg/plugin/rpc/bridge.go` — `StructuredUpdate` type added
- `internal/component/plugin/process.go` — `HasStructuredHandler()` added
- `internal/component/bgp/format/structured.go` — deleted (eager approach)
