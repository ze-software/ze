---
kind: table
level:
stage:
---
| Pattern | Mechanism | Use when |
|---------|-----------|----------|
| Async broadcast (one-to-many) | EventBus (`pkg/ze/eventbus.go`) | A component notifies zero or more listeners about a state change. No return value needed. Example: `(l2tp, session-down)`, `(bgp-rib, best-change)`. |
| Sync request/response (one-to-one) | DirectBridge typed handler | Core calls a plugin function with typed args and waits for a typed result. Example: `ForwardCached`, `DispatchCommand`, `EmitEvent`. |
| Structured event delivery | DirectBridge `DeliverStructured` | Engine delivers pre-parsed event data to internal plugins (zero JSON). Example: `StructuredEvent` for BGP UPDATEs. |
| Text command dispatch | `DispatchCommand` (via bridge or pipe) | Plugin sends a text command to the engine's command registry. Slow path for ad-hoc or external callers. |
