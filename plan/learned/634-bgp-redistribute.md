# 634 -- bgp-redistribute

## Context

Operators of vendor-style routers expect `redistribute { import <protocol> }`
to advertise routes from non-BGP protocols (L2TP sessions, connected
prefixes, future static / OSPF / ISIS) into BGP UPDATEs. Ze's existing
`redistribute` block was an intra-BGP ingress ACL only -- the same evaluator
gated received UPDATEs from `ibgp` / `ebgp` sources but had no producer-side
counterpart for non-BGP protocols. The goal was to ship the egress half
without disturbing the ingress filter, and to do it in a way that lets
future protocol producers join the registry without coupling to BGP
internals.

## Decisions

- **Per-protocol producer declaration in `internal/core/redistevents/`** over
  shared single-namespace bus or raw JSON-payload subscriptions. The shared
  registry stores VALUE TYPES ONLY (`ProtocolID uint16`, name string,
  has-producer bit) -- no handle pointers cross plugin boundaries, and both
  producer and consumer call `events.Register[*RouteChangeBatch](name,
  redistevents.EventType)` in their own packages to obtain LOCAL handles.
- **Value-typed payload only** -- `RouteChangeBatch{Protocol ProtocolID,
  AFI uint16, SAFI uint8, Entries []RouteChangeEntry}`,
  `RouteChangeEntry{Action RouteAction, Prefix netip.Prefix, NextHop
  netip.Addr, Metric uint32}`. Strings on the hot path were rejected
  (per-event allocation); pointers across plugin boundaries were rejected
  (non-local coupling). Pool-friendly: sync.Pool with seeded EntriesCap=64.
- **Standalone shipping with an in-process Go fake producer** rather than
  waiting for the real L2TP-7c producer. `internal/test/plugins/fakeredist/`
  registers as both a redistribute source and a redistevents producer, and
  exposes `fakeredist emit` / `fakeredist emit-burst` CommandDecls for `.ci`
  drive scripts. Real-traffic coverage arrives via spec-l2tp-7c without
  reopening this spec.
- **Canonical command text reuses ze's existing announce parser:**
  `update text origin incomplete nhop <self|addr> nlri <fam> add <prefix>`
  for adds, `update text nlri <fam> del <prefix>` for withdraws. The
  `nhop self` token triggers the reactor's per-peer `LocalAddress`
  substitution (`internal/component/bgp/reactor/peer.go:562-591`); explicit
  next-hops pass through verbatim.
- **`OnStarted` (not `OnAllPluginsReady`)** for subscription -- consumer
  does not DispatchCommand to other plugins at startup, only subscribes to
  the bus.
- **fakeredist is also blank-imported from production
  `internal/component/plugin/all/all.go`** because `.ci` tests run the
  production `bin/ze` binary. The runtime cost is one registry entry; no
  goroutine until invoked.

## Consequences

- Future protocol producers (L2TP, connected, static, OSPF, ISIS) can join
  by adding two init-time calls: `events.Register[*RouteChangeBatch]
  (<name>, redistevents.EventType)` to obtain a local typed handle, and
  `redistevents.RegisterProducer(redistevents.RegisterProtocol(<name>))` to
  appear in the consumer's enumeration. No bgp-redistribute changes needed.
- The bus delivery is zero-copy in-process: a `*RouteChangeBatch` pointer
  fans out to every subscriber synchronously per the EventBus contract;
  only external plugin processes pay the JSON marshal cost (none today
  for redistribute).
- The shared evaluator now has TWO consumers: BGP `IngressFilter`
  (intra-BGP ACL, source ibgp/ebgp) and `bgp-redistribute` (cross-protocol
  egress, source non-BGP). One config block governs both behaviors.
- Per-protocol metrics (`ze_bgp_redistribute_events_received`,
  `_announcements`, `_withdrawals`, `_filtered_protocol_total`,
  `_filtered_rule_total`) make it possible to detect filter drift without
  reading logs.

## Gotchas

- **`events.Register` is idempotent on (namespace, eventType, T)** -- both
  producer (init time) and consumer (run time) calling it for the same
  protocol is by design. The events typeRegistry rejects mismatched T but
  accepts repeats of the same T from different packages.
- **`Family.String()` allocates once per batch** in the consumer hot path
  for the evaluator API. Deferred to a follow-up spec
  (`Evaluator.AcceptFamily(family.Family, ...)`); negligible at steady
  state, surfaces under sustained MB/s burst.
- **The `redistevents` payload deliberately stores AFI / SAFI as raw
  uint16/uint8** rather than `family.Family` so the package stays a
  zero-internal-coupling leaf. Producers and consumers translate at the
  boundary.
- **fakeredist appears in `ze plugin list` in production** -- not ideal
  spec-wise but unavoidable given `.ci` runs production `bin/ze`. The
  description marks it as test-only.
- **The auto-linter strips fresh imports between edits** when the new
  symbols are not yet referenced. When threading new imports through, add
  the consuming usage in the same Edit or expect the next Edit to
  re-introduce the import.
- **Switch statements with a `default:` arm trigger the silent-ignore
  hook**. Use chained `if` for action / status enumerants instead.

## Open Question (deferred to future spec)

After the implementation landed, the user pointed out that the design they
originally had in mind was a **central-store model**: the engine owns a
shared per-protocol RIB store, producers register / push routes into it,
consumers (bgp-redistribute, FIB plugins, looking glass) read by reference
from the store. The spec's "Failed Approaches" rejected
`Source *redistribute.RouteSource` as a "pointer into another plugin's
memory" -- correctly -- but conflated that with engine-owned shared
state, which is a different shape and would have been acceptable. The
spec therefore never compared event-bus vs central-store; it took the
event-bus model as given.

What shipped is correct against the spec. If a future workload (a second
consumer beyond bgp-redistribute, e.g. cross-protocol show, fib-source
attribution, looking-glass topology) makes the lack of a single source
of truth painful, the right move is a fresh `spec-protorib-0` that
introduces the engine-owned per-protocol store. Under that direction:
`redistevents` reduces to a notification-only event package, the typed-
handle subscription glue in bgp-redistribute-egress is replaced by store
reads, fakeredist becomes a store-push driver. The text `UpdateRoute`
boundary to the reactor stays.

## Files

- `internal/core/redistevents/{events,registry,pool,redistevents_test}.go`
- `internal/component/bgp/plugins/redistribute/{redistribute,format,register,redistribute_test,metrics_test}.go`
- `internal/test/plugins/fakeredist/{fakeredist,register,fakeredist_test}.go`
- `internal/test/plugins/all/all.go`
- `internal/component/plugin/all/{all,all_test}.go`
- `test/plugin/bgp-redistribute-{announce,filtered-out,nexthop-self,withdraw,explicit-nhop,burst,metrics}.ci`
- `docs/{features,guide/plugins,guide/configuration,comparison,architecture/core-design,functional-tests}.md`
