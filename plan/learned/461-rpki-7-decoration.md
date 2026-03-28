# 461 -- RPKI Event Decoration

## Objective

Enable plugins to receive BGP UPDATE events enriched with RPKI validation state, without adding complexity to the engine or blocking UPDATE delivery.

## Decisions

- **Separate events, not enriched events:** rpki plugin emits a standalone validation event rather than rebuilding the UPDATE with extra fields. This avoids JSON cloning in the hot path, engine withholding, and round-trip blocking.
- **Union helper in SDK, not in engine:** Correlation is a consumer concern. The engine stays simple (just event routing). The SDK `Union` type correlates two async streams by (peer, message ID).
- **Generic emit-event RPC:** `ze-plugin-engine:emit-event` is not rpki-specific. Any plugin can emit any registered event type. Opens the door for future enrichment sources (FlowSpec classification, community tagging) without engine changes.
- **Unavailable as object, not string:** rpki events use `{"status":"unavailable"}` (object) rather than `"unavailable"` (bare string). Consumers always get an object, avoiding type-switching.
- **Rejected alternative: engine-managed decoration chains.** Attractive abstraction but creates tight coupling between UPDATE delivery and decorator latency (pending state, timeouts, chain tracking, JSON rebuild per UPDATE).

## Patterns

- **Lazy emission:** rpki plugin only emits validation events if any subscriber has subscribed to the `rpki` event type. Zero overhead for non-rpki consumers.
- **Correlation key = peer + message ID:** Events already carry `message.id` (uint64). No new ID scheme needed. Union uses `peer|msgID` composite string key.
- **Struct-based JSON building:** `emit.go` uses `json.Marshal` on typed structs rather than string concatenation. Prevents injection and makes structure visible.
- **Ordered eviction in Union:** insertion-order slice + index map for O(1) eviction of oldest entries when max-pending reached. Timeout sweep runs on configurable interval with WaitGroup shutdown.
- **Self-delivery prevention:** emit-event handler skips the emitting plugin's own subscriptions to prevent event loops.

## Gotchas

- The unavailable event format changed during implementation: spec said bare string `"unavailable"`, implementation uses `{"status":"unavailable"}` object. Object is better (consistent type for consumers), but this was a deviation.
- Union `Stop()` must be called to prevent goroutine leaks from the sweep timer. Documented in API but easy to forget.
- Three functional tests were implemented (rpki-event-valid, rpki-event-multi, rpki-event-unavailable). The three union functional tests (rpki-union-join, rpki-union-passthrough, rpki-union-timeout) were not created -- union is tested at the unit level only (9 tests in `union_test.go`).

## Files

- `pkg/plugin/sdk/union.go` -- Union helper: correlates two event streams by message ID
- `pkg/plugin/sdk/union_test.go` -- 9 union tests (both-arrive, primary-first, secondary-first, timeout, flush, max-pending, correlation-key, stop, concurrent)
- `pkg/plugin/sdk/sdk_engine.go` -- EmitEvent SDK method
- `pkg/plugin/rpc/types.go` -- EmitEventInput/Output RPC types
- `internal/component/bgp/plugins/rpki/emit.go` -- rpki event JSON building (struct-based)
- `internal/component/bgp/plugins/rpki/emit_test.go` -- 7 emit tests
- `internal/component/bgp/plugins/rpki/rpki.go` -- emit integration into validation worker
- `internal/component/plugin/events.go` -- EventRPKI constant added to ValidBgpEvents
- `internal/component/plugin/server/dispatch.go` -- emit-event RPC handler
- `internal/component/plugin/server/subscribe.go` -- whitespace validation for subscriptions
- `internal/core/ipc/schema/ze-plugin-engine.yang` -- emit-event YANG schema
- `test/plugin/rpki-event-valid.ci` -- functional: valid prefix emits validation event
- `test/plugin/rpki-event-multi.ci` -- functional: three prefixes with independent states
- `test/plugin/rpki-event-unavailable.ci` -- functional: no RTR server produces unavailable
