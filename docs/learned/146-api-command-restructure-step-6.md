# 146 — API Command Restructure Step 6: Event Subscription

## Objective

Replace config-driven event routing (`PeerProcessBinding.Receive*` fields) with an API-driven subscription model (`subscribe bgp event update`, etc.), enabling plugins to dynamically choose which events they receive.

## Decisions

- Chose parallel routing over replacement: config-driven and API-driven routing work simultaneously, with deduplication so a process never receives the same event twice. This preserves backward compatibility for existing plugins that use `process { receive { update; } }` config.
- Spec planned to remove `Receive*` fields; implementation kept them to avoid breaking the existing config-based workflow.
- Default direction is `both` (empty string in matching); events without direction concept (state, negotiated) use empty string consistently.

## Patterns

- Subscription matching: namespace → event type → direction → peer filter, evaluated in that order. Direction filter skips events that have no direction concept.
- `SubscriptionManager` is per-Server, not per-Process — one manager for all clients, keyed by `*Process`.
- Sent UPDATE messages needed `AttrsWire` creation inline from UPDATE body bytes (RFC 4271 §4.3 parsing) to supply wire bytes for subscribers in JSON mode.

## Gotchas

- `OnMessageSent` was initially formatting sent messages using the received formatter — producing incorrect `"type":"received"` in sent message events. Discovered during session 2 fixes.
- Mutex pattern in reactor: capturing peer slice before unlock/relock was required to avoid holding read lock while iterating and writing.
- Dead `seen` map in `GetMatching()` was removed — map keys are already unique by definition, so dedup was a no-op.

## Files

- `internal/component/plugin/subscribe.go` — new, subscription types, manager, handlers
- `internal/component/plugin/server.go` — parallel routing through both config and subscription paths
- `internal/component/plugin/session.go` — added `sendCtxID` for encoding context on sent messages
