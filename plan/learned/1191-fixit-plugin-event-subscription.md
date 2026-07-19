# Learned: fixit-plugin-event-subscription

Spec: `plan/spec-fixit-plugin-event-subscription.md` (Gap A namespace, Gap B envelope, Gap C wildcard).

## What the change does
An external plugin can now (A) subscribe to a non-bgp event namespace at startup,
(B) opt into an `{namespace,event,payload}` envelope so events sharing a payload
type are discriminable, and (C) use `"*"` as a startup event to subscribe to every
event type of a namespace. All additive to the frozen `pkg/plugin` SDK.

## Non-obvious findings / traps
- **Two delivery formatters; only one was broken.** BGP events are formatted by
  `bgp/server/events.go` `formatMessageForSubscription` and delivered directly —
  they never enter `deliverEvent`/`payloadToJSON`. Everything else (EmitEngineEvent
  -> IPsec etc.) goes through `deliverEvent`. Gap B is confined to `deliverEvent`,
  so the BGP hot path is structurally untouched (no perf risk).
- **EventTypeID is GLOBAL, not per-namespace** (`internal/core/events/events.go`:
  single `eventTypeIDs` map, IDs from 1). So `"sa-up"` has ONE id across namespaces;
  `Subscription.Matches` gates on BOTH namespace AND event-type equality. That is why
  a control subscription in the default namespace does not receive a same-named event
  emitted in another namespace — the discriminator is the namespace, and the unit test
  asserts exactly that.
- **Gap C must use the server-package `allEventTypes()` (event_monitor.go), NOT
  `events.AllEventTypes()`.** The bgp namespace registers `events.DirectionSent`
  ("sent") as a pseudo event type; `allEventTypes()` filters it via
  `excludedFromMonitor`. Expanding `"*"` with the raw map would create a dead
  subscription for "sent".
- **Envelope must ride INSIDE the delivered string.** `deliver-batch` carries a JSON
  array of STRINGS and batching is the default path (`process/delivery.go`), so a
  sibling field on `DeliverEventInput` would only reach the single-event path. The
  envelope is a marshaled `rpc.EventEnvelope` string; both wire paths stay
  string-typed and byte-transparent.
- **Lazy-marshal-once preserved.** `deliverEvent` still marshals the bare payload
  exactly once; the envelope is built at most once per emit and ONLY when a matching
  proc has `Envelope()==true`. Zero opt-in subscribers = byte-identical work to before
  (guarded by `TestBgpStartupSubscriptionUnchanged` asserting BYTES, not counts).
- **Fail-closed on unknown namespace.** An explicit but unregistered namespace warns
  and skips the whole subscribe block (`resolveSubscriptionNamespace` returns ok=false)
  rather than resolving to `NamespaceUnknown` (0) and registering a silently-dead sub —
  the exact Gap A/C failure mode the spec calls out.
- **SetStartupSubscriptions left byte-identical.** New `SetStartupSubscriptionsIn`
  does NOT delegate through the existing (locked) method — that would re-enter the
  non-reentrant mutex and deadlock. It builds the input directly. `SetEnvelope`
  mirrors `SetEncoding` (lazy-allocs the input if unset).

## Out of scope (latent, left as-is)
- `RegisterPluginEventTypes` (`internal/component/plugin/resolve.go`) still hardcodes
  the default namespace for plugin-DECLARED event types — the next plugin declaring a
  non-bgp `Registration.EventTypes` entry hits the same class of bug. Spec R-6; the
  sibling `spec-fixit-reject-fence-observability` chose the counter, so no collision.
- `parseEventString` vs `ParseSubscription` duplication/drift not unified (spec D-6).

## Verification boundary (this was a parked background job)
Every AC's MECHANISM proven by scoped unit tests through the real entry points
(registerSubscriptions / EmitEngineEvent / deliver-batch wire / SDK setters). The
end-to-end SDK-fork `.ci` fixtures (spec Wiring/Functional rows) were NOT authored/run
— they need a live env. See `tmp/drain-fixit-plugin-event-subscription.md`.
