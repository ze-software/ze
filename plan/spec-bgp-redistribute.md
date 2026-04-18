# Spec: bgp-redistribute -- Cross-protocol redistribute egress plugin

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-l2tp-7c-redistribute |
| Phase | - |
| Updated | 2026-04-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `.claude/rules/plugin-design.md` -- plugin architecture, 5-stage protocol, import rules
4. `internal/component/config/redistribute/` -- source registry + evaluator
5. `internal/component/bgp/redistribute/` -- existing intra-BGP `IngressFilter` (to stay untouched)
6. `internal/plugins/sysrib/sysrib.go` -- reference subscriber of `(bgp-rib, best-change)` via EventBus
7. `internal/component/bgp/plugins/rib/rib.go` -- `updateRoute` / `UpdateRoute` egress dispatch
8. `pkg/ze/eventbus.go` + `internal/core/events/events.go` -- typed EventBus handles

## Task

Introduce a new plugin `bgp-redistribute` that implements **vendor-standard
egress redistribution** for non-BGP protocols. Routes emitted by other
protocols (L2TP first; future connected / static / OSPF / ISIS) appear in
BGP UPDATEs to peers when operators configure
`redistribute { import <source> { ... } }`.

This plugin is the **single subscriber** that turns protocol route events
into BGP advertisements. The existing intra-BGP `IngressFilter` (same
`redistribute.Global()` evaluator, different semantics) is unchanged.

Model: routes are events; `redistribute` is a filtered subscription.
- Each protocol emits `(<protocol>, route-change)` batched events on the EventBus.
- bgp-redistribute subscribes, filters via `redistribute.Global().Accept`, dispatches
  `update-route` announce/withdraw per accepted entry.
- Target selector `"*"` -- engine fans out to every up peer. Reactor's existing
  `resolveNextHop()` substitutes each peer's local session address when the
  announce text carries `next-hop self`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- redistribute registry position, plugin isolation
- [ ] `.claude/patterns/plugin.md` -- new plugin structural template
- [ ] `.claude/rules/plugin-design.md` -- proximity principle, import rules, YANG requirement
  -> Constraint: plugins MUST NOT import sibling plugin packages -- use DispatchCommand or text commands
  -> Constraint: Stage-5 subscription callback goes in `OnStarted`; cross-plugin dispatch at startup goes in `OnAllPluginsReady`
  -> Constraint: plugin `Name` uses hyphen (`bgp-redistribute`); subsystem log key uses dot (`bgp.redistribute`)
- [ ] `plan/learned/541-policy-framework.md` -- why redistribute was split core/component

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` -- UPDATE message + NEXT_HOP semantics
  -> Constraint: NEXT_HOP MUST be a valid unicast address reachable by the peer; satisfied by existing `resolveNextHop`

### Source files (re-read 2026-04-18)
- [x] `internal/component/bgp/redistribute/filter.go` -- `IngressFilter` uses `redistribute.Global().Accept(route, "")` on UPDATE payload; `Origin="bgp"`, `Source="ibgp"|"ebgp"`.
  -> Constraint: unchanged. Two subscribers (IngressFilter + new plugin) read the same global evaluator.
- [x] `internal/component/config/redistribute/{evaluator,route,registry}.go` -- `Global()` returns `*Evaluator` (atomic.Pointer), `Accept(route, importingProtocol)` RW-locked; `ImportRule{Source, Families}` (empty Families = all); `RegisterSource(RouteSource{Name, Protocol})`; `SourceNames()` returns sorted source names (e.g. `["connected","ebgp","ibgp","l2tp","static"]`).
  -> Constraint: `SourceNames()` returns redistribute *source* names, NOT `(namespace, event-type)` pairs. There is NO registry that maps source name -> typed EventBus handle. The spec's "iterate SourceNames and resolve typed handle" step requires a new mapping to exist.
- [x] `internal/plugins/sysrib/sysrib.go` + `register.go` -- reference pattern. `ConfigureEventBus` callback stores bus in `atomic.Pointer[ze.EventBus]`; `OnStarted` starts `go s.run(ctx)`; `run` calls `ribevents.BestChange.Subscribe(eb, handler)` + `ReplayRequest.Subscribe`; blocks on `<-ctx.Done()`.
  -> Constraint: follow this shape exactly (package-level `loggerPtr`/`eventBusPtr`, `setLogger`/`setEventBus`/`getEventBus`, `OnStarted` spawns one goroutine, subscriptions inside `run`).
- [x] `internal/core/events/typed.go` + `pkg/ze/eventbus.go` -- `events.Register[T](ns, et)` returns `*Event[T]` with `Emit(bus, payload)` and `Subscribe(bus, func(T))`. Subscribe wrapper does `p.(T)` assertion; **mismatch logs a warn and drops silently**. Payload must be a concrete (non-interface) Go type registered at init.
  -> Constraint: to drive a typed handle from outside the engine (Python `emit-event` RPC, external plugin process), payload is a JSON `any` on the bus -- type assertion to `*RouteChangeBatch` FAILS and the event is dropped with a warn. Typed handles + external producer test plugins are incompatible without a raw-bus fallback.
- [x] `internal/component/bgp/plugins/rib/rib.go:499-518` -- `updateRoute(peerSelector, command)` calls `r.plugin.UpdateRoute(ctx, peerSelector, command)` with a 10s context timeout; no return check beyond `logger().Warn` on error.
- [x] `internal/component/plugin/server/dispatch.go:546-594` -- `handleUpdateRouteDirect` prefixes the command with `peer <selector> ` when it is not a known dispatcher prefix; `"*"` fan-out is resolved by the dispatcher across every up peer.
- [x] `internal/component/bgp/format.go:19-106` -- canonical `FormatAnnounceCommand` / `FormatWithdrawCommand` shape.
  -> Constraint: ze's announce command text is `update text origin <origin> [as-path ...] [med N] ... nhop <addr|self> nlri <afi>/<safi> add <prefix>`. Withdraw is `update text nlri <afi>/<safi> del <prefix>`. The spec's earlier claim of `announce <prefix> origin incomplete next-hop self` is WRONG and has been corrected in Data Flow.
- [x] `internal/component/bgp/plugins/cmd/update/update_wire.go:209-211` -- the token `nhop self` sets `bgptypes.NewNextHopSelf()`; reactor's `resolveNextHop(NextHopSelf, fam)` substitutes `p.settings.LocalAddress`.
  -> Constraint: per-peer NEXT_HOP is achieved via `nhop self` in the text command (NOT `next-hop self`); reactor handles substitution.
- [x] `internal/component/l2tp/` -- subsystem exists. `redistribute.go` registers `l2tp` source. `route_observer.go` tracks subscriber addresses but emits NO EventBus events today. No `(l2tp, route-change)` handle is registered.
  -> Constraint: the producer half of this feature does not exist yet. `spec-l2tp-7c-redistribute` is the dependency that adds the observer emission.
- [x] `test/scripts/ze_api.py` -- Python test-plugin SDK. Has `subscribe()` for receiving events; has NO `emit_event()` helper. External plugins can call `emit-event` RPC via the generic path, but the engine converts the JSON payload into a value the typed handle will reject (see typed.go constraint above).
  -> Constraint: a Python fakeproto emitting synthetic `(fakeproto, route-change)` events via `emit-event` RPC will NOT drive a typed `*RouteChangeBatch` subscriber. Wiring tests must either (a) use an in-process Go test fixture that emits directly, (b) have the consumer subscribe via raw `bus.Subscribe` with a JSON payload contract, or (c) wait for a real producer (L2TP-7c) before writing `.ci` wiring tests.

**Key insights (RESEARCH 2026-04-18):**

1. **Producer is absent.** No protocol currently emits `(<protocol>, route-change)` events. L2TP is the first candidate (spec-l2tp-7c, skeleton). Without a producer, bgp-redistribute ships as dead code exercised only by test fixtures.

2. **Command text syntax in the original spec is wrong.** ze's canonical announce command is `update text origin incomplete nhop self nlri <afi>/<safi> add <prefix>`, NOT `announce <prefix> origin incomplete next-hop self`. Withdraw: `update text nlri <afi>/<safi> del <prefix>`. The `nhop self` keyword is what triggers reactor per-peer `LocalAddress` substitution.

3. **Payload shape + handle resolution is undecided.** `RouteChangeBatch`/`RouteChangeEntry` do not exist anywhere in code. `SourceNames()` returns string names with no handle mapping. Three candidate designs:
   - **(A) Shared type + central handle registry.** New `internal/core/redistevents/` package owns `RouteChangeBatch` and a `RegisterProducerHandle(protocol, *Event[*RouteChangeBatch])` registry. Each producer calls `events.Register[*RouteChangeBatch]("<protocol>","route-change")` at init and adds the handle. bgp-redistribute iterates the registry on startup.
   - **(B) Raw bus subscribe-by-name.** bgp-redistribute iterates `redistribute.SourceNames()`, filters non-BGP protocols via `LookupSource`, subscribes via raw `bus.Subscribe(protocol, "route-change", fn(any))` and parses JSON itself. Works with external producers but loses type safety -- violates `plugin-design.md` "Subscribers MUST type-assert via the typed handle".
   - **(C) Single shared handle.** One `events.Register[*RouteChangeBatch]("redistribute","route-change")`; batch carries `Protocol` field. Producers emit on a single (ns, et); subscriber filters by `batch.Protocol`. Simpler registry, but couples all protocols to a shared namespace.
   - Default recommendation: (A). Keeps typed handles and scales to per-protocol registration without coupling producers to each other.

4. **Reference pattern is sysrib.** `ConfigureEventBus` callback + package-level `atomic.Pointer[ze.EventBus]` + `OnStarted` spawns `go s.run(ctx)` + typed handle Subscribe + block on `<-ctx.Done()`.

5. **Plugin placement:** `internal/component/bgp/plugins/redistribute/` is correct -- the plugin emits BGP UPDATEs via `UpdateRoute`, so it is BGP-specific in function. Registering under `internal/plugins/` would hide the BGP coupling.

6. **Wiring tests under typed handles are constrained.** Python `emit-event` cannot drive typed subscribers. Options: (i) in-process Go test fixture, (ii) .ci tests that wait for the real L2TP producer to land, (iii) switch to approach (B) raw bus and accept the type-safety loss.

## Research Decisions (agreed 2026-04-18)

-> Decision: **Handle resolution = approach (A), per-protocol
   producer declaration (value-types-only registry).**
   New package `internal/core/redistevents/` owns the shared payload
   type (`RouteChangeBatch`/`RouteChangeEntry`, value types only, no
   cross-plugin pointers), the numeric `ProtocolID` registry, and a
   "which protocols have a producer" declaration table. Each producer
   calls `events.Register[*RouteChangeBatch](name, "route-change")`
   inside its own package to obtain its LOCAL handle, then calls
   `redistevents.RegisterProducer(id)` to declare existence. Consumer
   enumerates `redistevents.Producers()` on `OnStarted`, filters out
   its own protocol, and for each remaining ID calls
   `events.Register[*RouteChangeBatch](ProtocolName(id), "route-change")`
   in its own package to obtain its OWN local handle, then subscribes.
   No plugin holds a pointer allocated by another plugin. See
   "Protocol registry" table below for the exact surface.
   Rationale: (B) violates `buffer-first.md` by JSON-unmarshaling per
   delivery whenever the producer is an external plugin process
   (5-7 heap allocations per event on the hot path). (C) wastes
   dispatch cycles proportional to total subscribers on the shared
   namespace; scales poorly. (A) matches dispatch cost to registered
   interest and gives an enumerable producer declaration for
   diagnostics, while the value-types-only registry preserves plugin
   isolation (see cross-boundary coupling table below).

-> Decision: **Ship bgp-redistribute standalone with an in-process Go-side
   fake producer.** A minimal test-only internal plugin (`fakeredist`,
   loaded only in `.ci` tests) registers a `RouteChangeBatch` handle via
   the registry and emits synthetic events on a trigger command. BGP
   real-traffic coverage arrives later via `spec-l2tp-7c-redistribute`
   without reopening this spec.

-> Decision: **Command text shape corrected.** Announce:
   `update text origin <origin> nhop <addr|self> nlri <afi>/<safi> add <prefix>`.
   Withdraw: `update text nlri <afi>/<safi> del <prefix>`. `nhop self`
   triggers reactor per-peer `LocalAddress` substitution
   (`internal/component/bgp/plugins/cmd/update/update_wire.go:209-211`
   + `internal/component/bgp/reactor/peer.go:562-591`).

-> Decision: **Event payload is self-contained, no strings, no pointers
   across plugin/component boundaries.**

   **HARD RULE (no exceptions):** payload fields MUST be value types
   only. No `*RouteSource`, no pointer into any registry, no pointer
   into data owned by another plugin or component. Pointers across
   boundaries are rejected the same way a goto is rejected: they create
   non-local coupling invisible at the call site.

   Strings on the hot path are rejected for the same reason they are
   elsewhere in ze (per-event allocation, slow equality compare, escape
   to heap inside slices).

### Payload field types (value types only; no strings, no cross-boundary pointers)

| Type | Field | Representation | Rationale |
|------|-------|----------------|-----------|
| `RouteChangeBatch` | Protocol | numeric `ProtocolID` (uint16), registered once at init in `internal/core/redistevents/` | Self-contained value; integer eq on the hot path; zero cross-boundary pointer. Consumer translates ID back to a name only if it needs one (diagnostics) via a local registry call, not via dereferencing shared state |
| `RouteChangeBatch` | Family | existing `family.Family` value type (AFI uint16 + SAFI uint8) | Matches ze's canonical family form everywhere else; integer eq; no alloc |
| `RouteChangeBatch` | Entries | slice of fixed-size `RouteChangeEntry` value records | Pool-friendly; no pointer escape via string fields |
| `RouteChangeEntry` | Action | uint8 enum -- `ActionUnspecified=0`, `ActionAdd=1`, `ActionRemove=2` | Integer eq; zero value is invalid so corruption is visible |
| `RouteChangeEntry` | Prefix | `netip.Prefix` value type | Fixed size; compares are cheap; no alloc |
| `RouteChangeEntry` | NextHop | `netip.Addr` value type; zero `Addr` means consumer uses reactor's `nhop self` | No boolean flag needed; zero-value sentinel keeps struct fixed-size |
| `RouteChangeEntry` | Metric | uint32 (reserved for future use) | Fixed size |

BLOCKING for implementation:
- NO `string Protocol`, `string Family`, or `string Action` on the batch or entry.
- NO `*RouteSource`, `*anypackage.AnyStruct`, or any pointer field that carries the consumer into another plugin or component's memory.
- The payload is fully self-describing via value types only.

### Protocol registry (new, in `internal/core/redistevents/`)

`ProtocolID` is a uint16 alias. The registry stores VALUE TYPES ONLY --
no handle pointers, no producer-allocated pointers of any kind.
Producers and consumers each build their OWN typed handles locally
using `events.Register[*RouteChangeBatch](name, "route-change")` in
their own package; the `events` registry's duplicate-registration
guard accepts independent Register calls from different packages with
the same `(namespace, eventType, T)` tuple because this is a
CONTRACT guard, not shared mutable state.

| Surface | Stores / returns | Purpose |
|---------|------------------|---------|
| `RegisterProtocol(name string) ProtocolID` | stores `{id, name}` as value fields | Called from producer `init()`. Idempotent. IDs start at 1; 0 is `ProtocolUnspecified` (invalid). Sorted lookup table internally. |
| `RegisterProducer(id ProtocolID)` | marks the ID as "has producer" (a bit, not a pointer) | Called from producer `init()` AFTER the producer's local `events.Register` call. Declares producer existence. |
| `Producers() []ProtocolID` | fresh slice of IDs, value types only | Consumer enumeration at `OnStarted`. |
| `ProtocolName(id ProtocolID) string` | copy of the stored name | Consumer namespace lookup for building its own local handle; also diagnostic output. Called outside the hot path. |
| `ProtocolIDOf(name string) (ProtocolID, bool)` | ID by value | Consumer calls at startup to learn canonical IDs it cares about (e.g. `ProtocolIDOf("bgp")` to filter out its own protocol). |

### Cross-boundary coupling surface after this design

| Surface | Data crossing the boundary | Cross-plugin pointer? |
|---------|----------------------------|----------------------|
| `redistevents` registry | `ProtocolID` (uint16), immutable name strings, "has producer" bit | No. All value types. |
| Bus `Subscribe` call | consumer-owned function closure, passed to the bus | No. Closure is consumer-owned; bus owns the subscription table. |
| Event payload (`*RouteChangeBatch`) | bus-owned pointer, synchronous read-only access during dispatch per `eventbus.go` contract; every field is a value type | No. Pointer comes from the bus, not from another plugin's memory. |
| Type identity | both plugins import `internal/core/redistevents/` for the `RouteChangeBatch` / `RouteChangeEntry` / `RouteAction` / `ProtocolID` type DEFINITIONS | Type definitions are the shared contract, the same way both plugins importing `family.Family` share a type but not state. Not cross-plugin memory access. |

BLOCKING: the registry returns no pointer types. `Producers()` returns
`[]ProtocolID`, not `[]*anything`. Consumer constructs its own handle
locally via `events.Register[*RouteChangeBatch](name, "route-change")`
and subscribes on that handle. Producer does the same in its own
package.

### Pool semantics

Per eventbus.go contract, subscribers run synchronously and MUST NOT
retain the payload. `RouteChangeBatch` is therefore safe to release to a
`sync.Pool` immediately after `handle.Emit` returns. The batch backing
slice (`Entries`) is also recycled. Typical producer lifecycle:
acquire → fill fields → append entries → emit → release.

All Entries element fields are value types (no pointer escape), so the
backing array stays stable in the pool.

### Hot-path allocation target

| Step | Target |
|------|--------|
| Producer acquire/release batch | 0 heap allocations (sync.Pool) |
| Producer populate fields | 0 (value types) |
| Producer append Entries (≤ cap) | 0 |
| `handle.Emit` in-process | 0 (shared pointer fan-out) |
| `handle.Emit` with external subscriber | 1 JSON marshal per Emit (bus-internal, amortized across all external subs) |
| Subscriber typed-handle assertion | 0 |
| `configredist.Accept(route, "bgp")` | 0 (RedistRoute stack-allocated) |
| `b.Family.String()` for evaluator call | 1 string alloc per batch (existing evaluator API takes string) -- SEE DEFERRAL BELOW |
| Entries iteration | 0 (range by pointer via `&b.Entries[i]`) |

### Deferred to follow-up spec

| What | Why deferred | Destination |
|------|--------------|-------------|
| `Evaluator.AcceptFamily(family.Family, ...)` API -- removes the one remaining per-batch `Family.String()` allocation in the consumer hot path | Scope creep: touches `internal/component/config/redistribute/evaluator.go` + all callers including the existing `IngressFilter` path. Landing it here enlarges the change and delays redistribute. Performance impact of a single per-batch string allocation is negligible at steady state | a new spec, to be filed when this spec is closed |

## Open Research Questions (resolved)

All research questions are now closed. Decisions captured in Research
Decisions and Design Alternatives sections above.

1. ~~Fake-producer placement~~ -> **(a) `internal/test/plugins/fakeredist/`**.
   See Alt-5.
2. ~~Explicit nexthop support~~ -> **resolved**: `RouteChangeEntry.NextHop`
   is a `netip.Addr` value; zero-Addr sentinel = consumer emits `nhop
   self`. Non-zero Addr is passed through as explicit `nhop <addr>` in
   the command text. Producers that have no address leave the field
   zero. Field is present from v1; producers opt in when they have a
   value. AC-11 covers this.

## Current Behavior (MANDATORY)

**Source files read:**
- `internal/component/bgp/redistribute/filter.go` -- existing `IngressFilter` drops iBGP/eBGP UPDATEs at reactor if `redistribute.Global()` is non-nil and the source is not matched. Uses `Accept(route, "")` (empty importing protocol, skip loop prevention).
- `internal/component/config/redistribute/evaluator.go` -- `Global() *Evaluator`; `Accept(route, importingProtocol) bool`; atomic swap on reload.
- `internal/component/config/redistribute/route.go` -- `RedistRoute{Origin, Family, Source}` + `ImportRule{Source, Families}`.
- `internal/plugins/sysrib/sysrib.go` -- reference for subscribing to `(bgp-rib, best-change)` via typed EventBus handle; `setEventBus`/`getEventBus` pattern.
- `internal/component/bgp/plugins/rib/rib.go:499-518` -- `updateRoute(peerSelector, command)` path; `plugin.UpdateRoute` RPC; selector `"*"` fans out.
- `internal/component/plugin/server/dispatch.go:546` -- `handleUpdateRouteDirect` dispatches "peer <addr> <cmd>" per matching peer.

**Behavior to preserve:**
- Existing `IngressFilter` keeps its current semantics. No code change in `internal/component/bgp/redistribute/`. Both subscribers (ingress-filter + this new egress plugin) share the same evaluator + rules.
- `sysrib` unchanged (not in the L2TP-redistribute-to-peers path; separately handles FIB selection in the future).
- `bgp rib inject` unchanged.

**Behavior to change:**
- New plugin wired in: `internal/component/bgp/plugins/redistribute/` with its own `register.go`, subscribes to every registered non-BGP protocol event type at Stage-5 ready.
- The `redistribute { import X }` config, previously limited to gating BGP ingress, now also drives egress advertisement when `X.Protocol != "bgp"`.

## Data Flow (MANDATORY)

### Entry Point
- EventBus typed subscription per non-BGP producer. Consumer builds its
  own local typed handle via `events.Register[*RouteChangeBatch](name, "route-change")` in its own package for each enumerated non-BGP
  `ProtocolID`. No handle pointer crosses a plugin boundary.

### Event payload shape (batched, value types only)

| Field on `RouteChangeBatch` | Type | Size | Notes |
|------------------------------|------|------|-------|
| `Protocol` | `redistevents.ProtocolID` (uint16) | 2B | Registered at producer init; 0 = unspecified (invalid) |
| `Family` | `family.Family` (AFI uint16 + SAFI uint8) | 3B padded | Canonical ze family value type |
| `Entries` | `[]RouteChangeEntry` | slice header 24B | Fixed-size elements; pool-friendly backing array |

| Field on `RouteChangeEntry` | Type | Size | Notes |
|-----------------------------|------|------|-------|
| `Action` | `redistevents.RouteAction` (uint8 enum) | 1B | 0=Unspecified (invalid), 1=Add, 2=Remove |
| `Prefix` | `netip.Prefix` | ~24B | Value type |
| `NextHop` | `netip.Addr` | 16B | Zero-value sentinel = consumer emits `nhop self` |
| `Metric` | `uint32` | 4B | Reserved for future use; unused in this spec |

BLOCKING: no string fields, no pointers, no `map[string]any`. The earlier
Metadata field is removed -- free-form maps force per-event heap
allocation and cannot be validated.

### Transformation Path

1. Bus delivers the batch pointer to the consumer's typed-handle wrapper;
   the wrapper performs the type assertion once and calls the handler.
2. Handler filters by protocol: `if b.Protocol == bgpID { return }` (one
   uint16 compare; `bgpID` is learned once at consumer startup via
   `redistevents.ProtocolIDOf("bgp")` and cached in a package-level
   variable).
3. Handler reads `configredist.Global()`. If nil, return.
4. Handler looks up the protocol name once per batch
   (`redistevents.ProtocolName(b.Protocol)`) and builds a `RedistRoute{Origin: name, Source: name, Family: b.Family.String()}` on the stack.
   The `Family.String()` allocation is the only per-batch string
   allocation on the hot path; see Deferred for the follow-up fix.
5. Handler calls `ev.Accept(route, "bgp")`. If false, return.
6. For each entry (range by pointer to avoid copy):
   - Action==Add: build command text `update text origin incomplete nhop
     <self|addr> nlri <fam> add <prefix>`. `<addr>` is used when
     `entry.NextHop` is non-zero; `self` otherwise.
   - Action==Remove: build command text `update text nlri <fam> del <prefix>`.
7. Handler calls `plugin.UpdateRoute(ctx, "*", command)` per entry.
8. Engine dispatcher prefixes `peer <addr>` per up peer; reactor parses
   the text, `nhop self` resolves to the peer's `LocalAddress` via
   existing `resolveNextHop`.
9. Each peer emits an UPDATE with its own NEXT_HOP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol producer -> redistevents registry | value-type registration (`RegisterProtocol`, `RegisterProducer`) | [ ] unit test in `redistevents_test.go` |
| Producer -> bus | producer's local typed handle `Emit(bus, *batch)` | [ ] unit test with in-process fake producer |
| Bus -> consumer | consumer's local typed handle `Subscribe(bus, handler)` | [ ] unit test |
| Consumer -> reactor | `plugin.UpdateRoute(ctx, "*", command)` | [ ] functional test |
| Reactor -> wire | `resolveNextHop` substitutes Self per peer | [ ] functional test with 2 peers |

### Integration Points
- `internal/core/redistevents/` (NEW) -- owns payload types, `ProtocolID`, registry; value-types-only surface.
- `configredist.Global()` evaluator -- read via atomic load; lock-free on the hot path.
- `plugin.UpdateRoute` RPC -- identical path bgp-rib uses today. No changes to the RPC.

### Architectural Verification
- [ ] No direct import of any protocol producer package (l2tp, iface, ...).
- [ ] No import of `bgp/plugins/rib` either -- both plugins call `UpdateRoute` independently.
- [ ] Consumer holds no pointer allocated by another plugin (payload fields are value types; producer handle is NOT exposed by the registry).
- [ ] Plugin disabled = no route redistribution; no effect on bgp-rib or reactor.
- [ ] Evaluator read is lock-free on hot path (atomic.Pointer).
- [ ] Event handle construction is idempotent (`events.Register` accepts duplicate (namespace, eventType, T) calls from different packages).

## Design Alternatives

### Alt-1: Subscription lifecycle

| Option | Behavior | Trade-offs |
|--------|----------|-----------|
| (i) Subscribe once at `OnStarted` to every non-BGP producer found in `redistevents.Producers()` | Simplest. Mirrors sysrib. Evaluator atomic swap handles reload semantics without touching subscriptions. | Assumes producers register at init (before OnStarted). True today: `init()` runs before any plugin starts. |
| (ii) Re-enumerate producers on every config reload | Handles dynamically loaded producers. | Producers don't register at runtime in ze today. Adds complexity without a real use case. |

**Chosen:** (i). Producer registration is init-time by construction. A
runtime-pluggable producer would require plugin-loader changes outside
this spec's scope.

### Alt-2: Dispatch granularity

| Option | Behavior | Trade-offs |
|--------|----------|-----------|
| (a) Per-entry `UpdateRoute` call | One RPC per prefix change. Matches existing bgp-rib/rr/watchdog pattern exactly. | N RPC calls per batch of N entries. Each call is a direct-bridge call in-process (no socket I/O), so overhead is small. |
| (b) Batched `UpdateRoute` using multi-prefix text (`nlri <fam> add <p1> <p2> ...`) | One RPC per accepted-subset-per-family. | Evaluator `Accept` is per-entry; mixed accept/reject within a batch forces splitting. Command-text parser supports multiple prefixes for identical attributes; cross-family mixing is not supported. Complexity > saving. |

**Chosen:** (a). The per-entry pattern is uniform with existing plugins.
If profiling later shows RPC overhead dominates, switch to (b) is
mechanical (group consecutive accepted entries with identical attrs).

### Alt-3: Callback placement

| Option | When | Trade-offs |
|--------|------|-----------|
| (x) `OnStarted(ctx)` spawns `go run(ctx)` | Stage 5 ready. Local goroutine subscribes + blocks on `<-ctx.Done()`. | Matches sysrib exactly. |
| (y) `OnAllPluginsReady` | After all plugins loaded, dispatcher registry frozen. | Only needed when doing cross-plugin `DispatchCommand` at startup. We don't. |

**Chosen:** (x). Per `plugin-design.md` "OnStarted vs OnAllPluginsReady",
subscription callbacks go in `OnStarted`.

### Alt-4: Family.String allocation on hot path

| Option | Behavior | Trade-offs |
|--------|----------|-----------|
| (p) Keep per-batch `Family.String()` for the existing `Evaluator.Accept` API | 1 string alloc per batch. Zero API change. | Small cost at batch cadence. |
| (q) Add `Evaluator.AcceptFamily(family.Family, ...)` now | Zero allocs on hot path. | Touches `configredist.Evaluator` + both subscribers (IngressFilter too). Cross-cutting change. |

**Chosen:** (p) for this spec; (q) deferred to a follow-up spec. The
per-batch allocation cost is negligible at steady state and the
cross-cutting change is scope creep.

### Alt-5: Fake-producer placement

| Option | Location | Trade-offs |
|--------|----------|-----------|
| (a) `internal/test/plugins/fakeredist/` | Fits under existing `internal/test/` namespace (alongside `ci`, `decode`, `peer`, `runner`, `sim`, `syslog`, `testcond`, `tmpfs`). Clearly test-only by virtue of its parent. | Adds a `plugins/` subtree inside `internal/test/`; smaller novelty than a new top-level `internal/testplugins/` would have been. |
| (b) `test/plugin/fakeredist/` | Alongside `.ci` fixtures. | Mixes Go package with non-Go test artifacts; unusual for ze. |
| (c) `internal/plugins/fakeredist/` with a `Private: true` style gate | Same place as real plugins, flagged hidden. | Relies on config hygiene to keep it out of production. Previous precedent in ze for test-only plugins is limited. |

**Chosen:** (a) `internal/test/plugins/fakeredist/`. Parent directory
`internal/test/` already houses test infrastructure; a `plugins/`
subtree there is the smallest-novelty move. Blank-imported only by
the test binary's aggregator (`internal/test/plugins/all/`), never
by `internal/component/plugin/all/all.go`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| fakeredist plugin dispatches `fakeredist emit add ipv4/unicast 10.0.0.1/32` with matching `import fakeredist` rule | -> | producer emits batch -> consumer filter accept -> UpdateRoute announce -> reactor -> UPDATE on wire | `test/plugin/bgp-redistribute-announce.ci` |
| Same trigger, NO matching rule | -> | evaluator rejects -> no UpdateRoute call | `test/plugin/bgp-redistribute-filtered-out.ci` |
| Two peers with distinct local addresses, matching rule | -> | single trigger -> two UPDATEs with distinct NEXT_HOPs | `test/plugin/bgp-redistribute-nexthop-self.ci` |
| `fakeredist emit remove ipv4/unicast 10.0.0.1/32` after an add | -> | UpdateRoute withdraw -> WITHDRAWN_ROUTES on wire | `test/plugin/bgp-redistribute-withdraw.ci` |

Tests load the `fakeredist` test plugin as an internal in-process
plugin (blank-imported by the ze-test binary's plugin-all equivalent).
`fakeredist` registers its `ProtocolID` + typed handle at init,
registers `fakeredist` as a `redistribute.RouteSource`, and exposes a
CLI command that translates a line of `add|remove <family> <prefix>`
into a single-entry batch `Emit`.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin loaded; `fakeredist` producer registered; `redistribute.Global()` nil | No announcements; no panic on events |
| AC-2 | Evaluator has `import fakeredist { family ipv4/unicast; }` rule; add-batch with `/32` IPv4 entry | `update-route` dispatched with selector `*`, command text is `update text origin incomplete nhop self nlri ipv4/unicast add <prefix>` |
| AC-3 | Same as AC-2 but family is `ipv6/unicast`, `/128` entry | Dispatched; command text uses `ipv6/unicast`; `/128` prefix |
| AC-4 | Add-batch for family NOT in the import rule's family list | No dispatch |
| AC-5 | Add-batch for protocol NOT in any import rule | No dispatch |
| AC-6 | Producer whose registered protocol name is `"bgp"` emits a route-change event (hypothetical; BGP is a consumer, not a producer of this event) | **Not subscribed** by this plugin; ProtocolID filter skips the BGP-ID batch if one ever arrives. IngressFilter handles BGP-sourced redistribution separately. |
| AC-7 | Two BGP peers up with distinct local session addresses; accepted event with `NextHop` zero | Each peer's UPDATE carries NEXT_HOP = its own local session address (`resolveNextHop` fires per peer) |
| AC-8 | Remove-batch for a previously announced entry | `update-route` dispatched with `update text nlri <fam> del <prefix>` |
| AC-9 | Config reload adds an import rule while plugin running | Subsequent events matching the new rule are announced; no plugin restart needed |
| AC-10 | Config reload removes an import rule | Previously-accepted events no longer trigger announce; in-flight routes already in Adj-RIB-Out remain (withdraw is only driven by source remove-events) |
| AC-11 | Add-batch with non-zero `NextHop` (e.g., 192.0.2.1) | Command text uses `nhop 192.0.2.1`, not `nhop self`; reactor passes the explicit address through without peer substitution |
| AC-12 | Two batches in close succession on the same (Protocol, Family): (add /32 A, remove /32 A) | Two dispatches in order: announce then withdraw. No reordering, no dedup at this layer. |
| AC-13 | Burst of N=500 accepted add events within one `fakeredist emit-burst` invocation | All 500 events dispatched; no pool-grow warnings; no dropped events; peer receives all 500 UPDATEs (or a compacted subset with identical content if bgp-rib batches them upstream). No timeouts on any `UpdateRoute` call. |
| AC-14 | Metric counters after a known event sequence | `ze_bgp_redistribute_events_received` equals the number of emitted batches; `ze_bgp_redistribute_announcements` equals the number of accepted `add` entries; `ze_bgp_redistribute_withdrawals` equals the number of accepted `remove` entries; `ze_bgp_redistribute_filtered_protocol_total` equals the number of batches filtered by the BGP-protocol skip (batch cadence); `ze_bgp_redistribute_filtered_rule_total` equals the number of entries rejected by the evaluator (entry cadence). All counters monotonically non-decreasing. |

## Failure Mode Analysis

| Failure mode | Trigger | Effect | Mitigation |
|--------------|---------|--------|-----------|
| Bus payload type mismatch | Producer code drift: emits `*WrongType` | `events.Event[T].Subscribe` wrapper logs warn, drops event | Compile-time safe for in-process producers; warn log surfaces publisher/consumer skew in tests |
| Nil EventBus on `OnStarted` | `ConfigureEventBus` never called | `getEventBus() == nil`, plugin logs warn, run returns | Same pattern as sysrib; test coverage |
| Evaluator nil during dispatch | Config reload between subscribe and callback, or plugin starts before config loaded | `configredist.Global()` returns nil; handler returns without dispatch | Read via atomic load; no lock; always safe |
| Reactor UpdateRoute error | RPC timeout, peer list empty, dispatcher error | Handler logs warn (same shape as bgp-rib's `updateRoute`); batch processing continues | 10s context timeout matches bgp-rib; no state mutation on failure |
| Unknown ProtocolID on incoming batch | Corrupted payload (impossible via typed handle; defense in depth) | `ProtocolName(id) == ""`; handler skips the batch with a warn | Zero-value Action is ActionUnspecified which also triggers skip |
| Entries slice aliased across emits (producer bug) | Producer re-uses backing array after Emit returns without copying | Consumer still in synchronous dispatch - safe. If consumer retains, UB. | eventbus.go contract states "MUST treat as read-only". Consumer does not retain. |
| Out-of-order add/remove from producer | Producer emits remove before add (for the same prefix) | Consumer dispatches withdraw of an unannounced prefix; reactor tolerates no-op withdrawal | No state here; idempotent at the wire level. Producer bugs surface in bgp-rib rejection logs |
| NextHop family mismatch (IPv4 addr for IPv6 family) | Producer supplies a wrong NextHop | `resolveNextHop` rejects when explicit NH; `nhop self` path is unaffected | Existing `canUseNextHopFor` check catches this at reactor |
| Reload triggers subscription churn | Config reload re-registers producers (won't happen today; producers are init-only) | No subscribers re-register either | Subscription is OnStarted-only; no reload path |
| Concurrent producer Emits during consumer Subscribe | Consumer starting while producer already running | Bus subscription is thread-safe; events before subscribe are lost | Producer replay is not in scope here. For real-traffic producers (L2TP), replay is a separate concern handled by the producer if needed |
| Many protocols register with same ProtocolID by accident | `RegisterProtocol` returns an existing ID for existing names, so duplicates with different payloads are impossible | N/A | Registry is idempotent on name |
| Family.String allocation pressure | Very high batch rate | 1 alloc per batch; sustained MB/s rates possible under pathological bursts | Deferred to follow-up: `Evaluator.AcceptFamily(family.Family)` |

## Triple Challenge

### Simplicity
- Is this the minimum change that achieves the goal? Yes. The plugin is
  one file + `register.go` + tests. The shared package is one file of
  types + one file of registry + its tests. No abstraction beyond the
  concrete need.
- Alternative considered: reuse `sysrib`'s `best-change` subscription
  instead of a new event type. Rejected because sysrib operates on
  FIB-admin-distance semantics over the system RIB; redistribute operates
  on import-rule semantics over per-protocol route lifecycles. They are
  different filters over different event sources.

### Uniformity
- Same plugin pattern as sysrib (ConfigureEventBus, OnStarted,
  package-level atomic pointers, long-lived goroutine).
- Same command-text API as bgp-rib, rr, watchdog (`update text ... nhop
  self ... nlri <fam> add|del <prefix>`). No new command vocabulary.
- Same registration pattern as `internal/core/family/` (central registry,
  value-type lookups).
- New: `internal/testplugins/` top-level directory. First occupant is
  `fakeredist`. Precedent for future test-only internal plugins.

### Performance
- Per-event heap allocations on the hot path: 0 from the payload
  (batch + entries pooled on producer side); 1 `Family.String()` per
  batch (consumer-side, deferred fix).
- Dispatch cost: typed-handle assertion (1 `p.(T)`), per-batch
  protocol filter (1 uint16 eq), evaluator accept (RW-lock read, 1-N
  import-rule walks), per-entry command build + RPC.
- Zero-copy fan-out: bus delivers the same batch pointer to every
  subscriber synchronously; no copy per subscriber.
- No cross-boundary pointer holds -- registry returns value types, both
  sides build local typed handles, payload is value-typed.
- RPC overhead: each `UpdateRoute` call is a direct-bridge in-process
  call (no socket I/O); function-call overhead, not syscall overhead.
- Burst exercise: `bgp-redistribute-burst.ci` drives 500 sequential
  emits. If the pool seed is correctly sized, `batchPool.New` fires at
  most once at warm-up. If it fires repeatedly, that is a sizing bug
  surfaced by the test.
- Metric assertion: `bgp-redistribute-metrics.ci` catches counter
  drift from reality (a past ze failure mode -- see recurring "metrics
  never asserted" pattern in `rules/memory.md` / mistake log).

## 🧪 TDD Test Plan

### Unit Tests

#### `internal/core/redistevents/` (new package)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterProtocol_Idempotent` | `redistevents_test.go` | Same name twice returns same ID; IDs start at 1 | |
| `TestRegisterProtocol_DistinctNames` | same | Distinct names allocate distinct IDs | |
| `TestProtocolName_Unknown` | same | Unknown ID returns empty string, no panic | |
| `TestProtocolIDOf_Unknown` | same | Unknown name returns 0, false | |
| `TestRegisterProducer_Idempotent` | same | Calling twice for same ID is a no-op | |
| `TestProducers_Snapshot` | same | Returns copied slice of IDs; callers modifying the slice do not affect the registry | |
| `TestRouteAction_ZeroValueInvalid` | same | `ActionUnspecified` is distinct from Add/Remove | |
| `TestBatchPoolReuse` | same | Release clears fields; Acquire returns clean batch | |

#### `internal/component/bgp/plugins/redistribute/` (consumer plugin)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubscribe_SkipsOwnProtocol` | `redistribute_test.go` | On OnStarted, consumer skips `ProtocolIDOf("bgp")` when iterating `Producers()` | |
| `TestSubscribe_NonBGPProducers` | same | Consumer subscribes via its own local `events.Register` handle for each non-BGP producer | |
| `TestHandleBatch_AcceptedAddDispatches` | same | Accepted add entries produce `UpdateRoute` calls with canonical text `update text origin incomplete nhop self nlri <fam> add <prefix>` | |
| `TestHandleBatch_ExplicitNextHop` | same | Non-zero `entry.NextHop` produces `nhop <addr>` instead of `nhop self` | |
| `TestHandleBatch_RejectedAddNoop` | same | Rejected entries: zero UpdateRoute calls | |
| `TestHandleBatch_RemoveDispatches` | same | Remove entries use `update text nlri <fam> del <prefix>` | |
| `TestHandleBatch_NoEvaluator_Noop` | same | `configredist.Global()==nil`: no dispatches, no panic | |
| `TestHandleBatch_ReloadApplies` | same | Evaluator atomic swap mid-run: next call uses new rules | |
| `TestHandleBatch_BGPSourceSkipped` | same | Batch with `Protocol == ProtocolIDOf("bgp")` is filtered out at handler entry | |
| `TestHandleBatch_UnknownProtocol` | same | Batch with unregistered ProtocolID is skipped with a warn, no dispatch | |
| `TestCommandText_AllFamilies` | same | Table-driven: IPv4 /32, IPv6 /128, /24, /64 render the expected text | |

#### `internal/test/plugins/fakeredist/` (test-only producer)
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInit_RegistersProtocol` | `fakeredist_test.go` | init() registers protocol name and producer | |
| `TestCommand_EmitAdd` | same | `fakeredist emit add ipv4/unicast 10.0.0.1/32` builds a one-entry batch and calls `RouteChange.Emit` | |
| `TestCommand_EmitRemove` | same | `fakeredist emit remove ipv4/unicast 10.0.0.1/32` same with Action=Remove | |
| `TestCommand_EmitBurst` | same | `fakeredist emit-burst 500 add ipv4/unicast 10.0.0.0/24` builds 500 sequential single-entry batches and emits them; no allocations beyond the pool-seeded capacity on the happy path | |
| `TestCommand_BadArgs` | same | Invalid family / prefix / burst count returns an error status, no Emit | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `ProtocolID` | uint16 0-65535 | 1 (first allocated), 65535 (max) | 0 (`ProtocolUnspecified`) | overflow beyond uint16 is impossible (type-bounded) |
| `RouteAction` | enum 0-2 | 1 (Add), 2 (Remove) | 0 (`ActionUnspecified`) | ≥3 rejected by handler (warn + skip) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-redistribute-announce` | `test/plugin/bgp-redistribute-announce.ci` | fakeredist plugin emits matching add event; peer receives UPDATE with synthesized attrs | |
| `bgp-redistribute-filtered-out` | `test/plugin/bgp-redistribute-filtered-out.ci` | fakeredist emits event without matching rule; peer receives nothing | |
| `bgp-redistribute-nexthop-self` | `test/plugin/bgp-redistribute-nexthop-self.ci` | Two peers, distinct local addrs; each UPDATE carries its own NEXT_HOP | |
| `bgp-redistribute-withdraw` | `test/plugin/bgp-redistribute-withdraw.ci` | fakeredist emits remove after add; peer receives WITHDRAWN_ROUTES | |
| `bgp-redistribute-explicit-nhop` | `test/plugin/bgp-redistribute-explicit-nhop.ci` | fakeredist emit with explicit NextHop; peer receives UPDATE with that NEXT_HOP, not the peer's LocalAddress | |
| `bgp-redistribute-burst` | `test/plugin/bgp-redistribute-burst.ci` | `fakeredist emit-burst 500 add ipv4/unicast 10.0.0.0/24` drives 500 sequential emissions; peer receives all 500 UPDATEs (or equivalent batched form); no pool-grow warnings; no UpdateRoute timeouts | |
| `bgp-redistribute-metrics` | `test/plugin/bgp-redistribute-metrics.ci` | Known event sequence (N accepted adds, M filtered, K removes); `curl` the Prometheus endpoint or parse `ze metrics show` output; assert each of the four counters matches the expected value | |

### Future (if deferring any tests)
- Per-peer redistribute config (different import rules per peer) -- out of scope here; separate spec when per-peer YANG lands.
- Intra-BGP egress redistribution (`redistribute ibgp` as egress advertisement vs current ingress-ACL semantics) -- out of scope; the current IngressFilter keeps its meaning.
- Policy transformations (set-localpref, community-tag on redistribute) -- out of scope; route-map/policy is a separate feature.
- `Evaluator.AcceptFamily(family.Family, ...)` to remove the per-batch `Family.String()` allocation -- deferred to follow-up spec.

## Files to Modify
- `internal/component/plugin/all/all.go` -- blank import of the new `bgp-redistribute` plugin package
- `cmd/ze-test/` binary's plugin-all equivalent (or a new `internal/test/plugins/all/` that test binaries blank-import) -- wire in `fakeredist` for test builds only

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Core payload/registry package | [x] NEW | `internal/core/redistevents/` (types, ProtocolID registry, pool) |
| Consumer plugin registration | [x] NEW | `internal/component/bgp/plugins/redistribute/register.go` |
| Consumer plugin logic | [x] NEW | `internal/component/bgp/plugins/redistribute/redistribute.go` |
| Test-only producer | [x] NEW | `internal/test/plugins/fakeredist/` (package + register + CLI) |
| Blank import in production `all.go` | [x] | `internal/component/plugin/all/all.go` (`bgp-redistribute` only, NOT `fakeredist`) |
| Blank import in test `all.go` | [x] | test-binary plugin-all (`fakeredist` here, never in production) |
| YANG schema | [ ] no (reads existing `redistribute` container; no new leaves) | -- |
| CLI commands (production) | [ ] no | -- |
| CLI commands (test fixture) | [x] | `fakeredist` registers `fakeredist emit ...` via CommandDecl for `.ci` driving |
| EventBus subscription | [x] | consumer plugin, via local `events.Register` handle per enumerated producer |
| Functional tests | [x] | seven `.ci` files above (announce, filtered-out, nexthop-self, withdraw, explicit-nhop, burst, metrics) |
| bgp-redistribute status/metrics | [x] | Prometheus counters: `ze_bgp_redistribute_events_received` (batch cadence), `ze_bgp_redistribute_announcements` (accepted-add entry cadence), `ze_bgp_redistribute_withdrawals` (accepted-remove entry cadence), `ze_bgp_redistribute_filtered_protocol_total` (BGP-protocol skip, batch cadence), `ze_bgp_redistribute_filtered_rule_total` (evaluator reject, entry cadence) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] yes | `docs/features.md` -- redistribute egress for non-BGP sources |
| 2 | Config syntax changed? | [ ] no (existing `redistribute` keyword, new semantic for non-BGP) | -- |
| 3 | CLI command added/changed? | [ ] no | -- |
| 4 | API/RPC added/changed? | [ ] no (uses existing update-route) | -- |
| 5 | Plugin added/changed? | [x] | `docs/guide/plugins.md` -- list `bgp-redistribute` |
| 6 | Has a user guide page? | [x] | `docs/guide/configuration.md` (redistribute section) -- clarify two semantics (ingress ACL for bgp sources, egress announce for non-bgp) |
| 7 | Wire format changed? | [ ] no | -- |
| 8 | Plugin SDK/protocol changed? | [ ] no | -- |
| 9 | RFC behavior implemented? | [ ] no | -- |
| 10 | Test infrastructure changed? | [x] | `docs/functional-tests.md` -- in-process test-only plugin pattern via `internal/testplugins/`; first occupant is `fakeredist` |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- BGP redistribute parity for non-BGP sources |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- new cross-protocol route-change event surface in `internal/core/redistevents/`; bgp-redistribute as egress subscriber |

## Files to Create
- `internal/core/redistevents/events.go` -- type definitions (`RouteChangeBatch`, `RouteChangeEntry`, `RouteAction`, `ProtocolID`) + constants
- `internal/core/redistevents/registry.go` -- `RegisterProtocol`, `RegisterProducer`, `Producers`, `ProtocolName`, `ProtocolIDOf`
- `internal/core/redistevents/pool.go` -- `AcquireBatch` / `ReleaseBatch`
- `internal/core/redistevents/redistevents_test.go`
- `internal/component/bgp/plugins/redistribute/redistribute.go` -- consumer plugin logic (subscribe, handleBatch, command builder)
- `internal/component/bgp/plugins/redistribute/register.go` -- consumer plugin registration
- `internal/component/bgp/plugins/redistribute/redistribute_test.go`
- `internal/testplugins/fakeredist/fakeredist.go` -- test-only in-process producer
- `internal/testplugins/fakeredist/register.go`
- `internal/testplugins/fakeredist/fakeredist_test.go`
- `internal/testplugins/all/all.go` -- test-binary blank-import aggregator (mirrors `internal/component/plugin/all/all.go` pattern, test-only)
- `test/plugin/bgp-redistribute-announce.ci`
- `test/plugin/bgp-redistribute-filtered-out.ci`
- `test/plugin/bgp-redistribute-nexthop-self.ci`
- `test/plugin/bgp-redistribute-withdraw.ci`
- `test/plugin/bgp-redistribute-explicit-nhop.ci`
- `test/plugin/bgp-redistribute-burst.ci`
- `test/plugin/bgp-redistribute-metrics.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify / Create |
| 3. Implement (TDD) | Phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-verify-fast` |
| 6-12 | Standard flow |

### Implementation Phases

1. **Phase 1: core `redistevents` package.**
   - Create `internal/core/redistevents/events.go` with `ProtocolID`, `RouteAction`, `RouteChangeBatch`, `RouteChangeEntry`. Tests FAIL (no registry yet).
   - Add `registry.go` with `RegisterProtocol`, `RegisterProducer`, `Producers`, `ProtocolName`, `ProtocolIDOf`. Tests PASS.
   - Add `pool.go` with `AcquireBatch` / `ReleaseBatch`. Pool-reuse test PASS.
   - Gate: `go test ./internal/core/redistevents/...` green.
2. **Phase 2: consumer plugin skeleton.**
   - Create `internal/component/bgp/plugins/redistribute/register.go` with empty `RunEngine`.
   - Blank-import in `internal/component/plugin/all/all.go`.
   - Bump `TestAllPluginsRegistered` expected count.
   - `make generate` + unit test green.
3. **Phase 3: consumer subscribe + handleBatch.**
   - Implement `run(ctx)`: enumerate `Producers()`, skip BGP-ID, build local typed handles, subscribe. Unit tests with a bus stub FAIL then PASS.
   - Implement `handleBatch`: evaluator lookup, RedistRoute construction, per-entry dispatch via `plugin.UpdateRoute`. Unit tests for every AC branch FAIL then PASS.
   - Implement command-text builder: offset-writes into a scratch buffer, string conversion at the UpdateRoute boundary only. Unit tests for all command-text variants PASS.
4. **Phase 4: test-only producer `fakeredist`.**
   - Create `internal/test/plugins/fakeredist/` with registration, typed-handle, and `CommandDecl` for `fakeredist emit add|remove <family> <prefix> [<nexthop>]`.
   - Create `internal/testplugins/all/all.go` blank-import aggregator.
   - Wire `internal/testplugins/all` into `cmd/ze-test/` (or equivalent test binary).
   - Unit tests for fakeredist command parsing + emission PASS.
5. **Phase 5: functional `.ci` tests.**
   - Write the seven `.ci` files (announce, filtered-out, nexthop-self, withdraw, explicit-nhop, burst, metrics).
   - Add a `fakeredist emit-burst <N> <add|remove> <family> <prefix>` CommandDecl to the test producer; each invocation emits N single-entry batches sequentially with prefix auto-incremented from the base.
   - Burst test asserts all N UPDATEs are received at the peer (or, if bgp-rib batches them before the reactor, that the aggregate prefix count matches N).
   - Metrics test drives a known event sequence and scrapes `ze metrics show` (or the Prometheus `/metrics` endpoint) to assert counter values.
   - Run via `bin/ze-test bgp plugin N`. Validate each AC's wire effect against `ze-peer` output.
6. **Phase 6: metrics.**
   - Register `ze_bgp_redistribute_events_received` (batch cadence), `_announcements` (entry cadence), `_withdrawals` (entry cadence), `_filtered_protocol_total` (batch cadence), `_filtered_rule_total` (entry cadence) via `ConfigureMetrics`.
   - Unit tests verify each counter increments at the expected cadence for a known sequence.
7. **Phase 7: docs.**
   - Update files listed in Documentation Update Checklist. Add `<!-- source: -->` anchors.
8. **Phase 8: learned summary.**
   - Write on completion per TEMPLATE.md format.
9. **Full verification** -- `make ze-verify-fast`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation + test |
| Correctness | Evaluator consulted per batch; BGP-ID filtered at handler entry; `ActionUnspecified` and unknown ProtocolID skipped with warn |
| Naming | Command text exactly matches ze's canonical `update text origin <o> [attrs...] nhop <addr\|self> nlri <afi>/<safi> add <prefix>` / `update text nlri <afi>/<safi> del <prefix>`. Round-trip test: parser accepts what the builder emits |
| Data flow | Plugin reads EventBus, writes UpdateRoute; no direct import of any producer plugin; no import of `bgp/plugins/rib` |
| No cross-boundary pointers | Event payload carries only value types; registry returns value types; consumer builds its own local handle |
| Rule: no-layering | No parallel redistribute path; existing IngressFilter unchanged |
| Rule: plugin-design | No import of sibling plugin packages; `OnStarted` (not `OnAllPluginsReady`) since no cross-plugin dispatch at startup |
| Rule: buffer-first | Command text assembled by offset writes on a scratch buffer (RFC 4271 max line length). `strings.Builder` is acceptable here (matches `internal/component/bgp/format.go`), but `fmt.Sprintf` is forbidden on the hot path |
| Rule: goroutine-lifecycle | Subscription callback is long-lived; no per-event goroutines; `run(ctx)` blocks on `<-ctx.Done()` |
| Rule: self-documenting | Command text in code references `internal/component/bgp/format.go` FormatAnnounceCommand / FormatWithdrawCommand as the canonical shape |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | `grep -n 'bgp-redistribute' internal/component/plugin/all/all.go` |
| Subscribes only to non-BGP sources | `grep -n 'Protocol.*bgp' internal/component/bgp/plugins/redistribute/redistribute.go` |
| Dispatches via UpdateRoute | `grep -n 'UpdateRoute' internal/component/bgp/plugins/redistribute/redistribute.go` |
| Announce text shape | grep for `"announce "` in redistribute.go |
| All `.ci` files exist | `ls test/plugin/bgp-redistribute-*.ci` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Protocol name / family / prefix validated before passing to command builder |
| Command injection | Command text parameters are typed (netip.Prefix, family.Family); no raw string interpolation from untrusted sources |
| Evaluator race | Evaluator read via `atomic.Pointer.Load`; concurrent reload safe |
| Resource exhaustion | Batch size bounded by producer (L2TP: one entry per session event); no unbounded accumulation inside plugin |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test asserts announce text but reactor parser rejects it | Re-read reactor's announce parser; adjust command text syntax |
| Two peers get same NEXT_HOP in the nhop-self test | `resolveNextHop` not firing; trace update-route -> reactor dispatch -> announce parse |
| `update-route` returns "no peers matched" | Test peer not up yet; wait for peer-up event before dispatch |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A Loc-RIB / PeerRIB flag was needed | Existing `types.RouteNextHop{Self}` + `resolveNextHop` already resolve per-peer; egress needs only a new subscriber | Design discussion | Avoided RIB-storage change |
| `bgp rib inject` could be refactored into the production path | Adj-RIB-In semantics + operator-supplied nhop are wrong for locally-originated routes | Design discussion | Kept CLI as debug tool; new egress path added |
| `redistribute` was unambiguous | Existing ze `redistribute` = ingress ACL; vendor-standard = egress advertise. Both are subscribers of the same evaluator. | Design discussion | Both semantics coexist; new plugin is egress, existing IngressFilter keeps ingress |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Merge into `bgp-rib` plugin | Violates single-responsibility; bgp-rib already owns Adj-RIB-In/best-path/Adj-RIB-Out | New plugin `bgp-redistribute` |
| Per-peer subscriber instance | Current config is global; per-peer structure buys nothing until per-peer YANG lands | Single subscriber, per-peer fan-out via `UpdateRoute "*"` |
| Payload field `Source *redistribute.RouteSource` (pointer into shared registry) | Cross-boundary pointer: consumer plugin would hold a pointer into data registered by the producer's init, even though the backing lives in shared core. User rejected: "NO POINTER. YOU ARE NOT ALLOWED TO POKE INTO OTHER PLUGIN OR COMPONENT CODE." | Numeric `ProtocolID uint16` registered in `internal/core/redistevents/`; name lookup via local value-returning API |
| Registry returning `map[ProtocolID]*Event[*RouteChangeBatch]` handles | Same class of violation: handle pointers are producer-allocated; consumer iterating the registry would hold pointers across the boundary | Registry returns value types only (`[]ProtocolID`, `(ProtocolID, bool)`, `string`). Consumer and producer each build their own local typed handle via `events.Register`, which is idempotent on the (namespace, eventType, T) contract |
| Approach (B) raw `bus.Subscribe` with JSON payload | Per-delivery JSON unmarshal when producer is external plugin process -- 5-7 heap allocations per event violates `buffer-first.md` | Approach (A): typed handle per protocol |
| Approach (C) single shared `(redistribute, route-change)` namespace | Dispatches every event to every subscriber on that one key; scales as subscribers × protocols | Approach (A): per-protocol key; subscribers receive only their registered interests |
| Original command text `announce <prefix> origin incomplete next-hop self` | Syntax not recognized by ze's announce parser; the canonical form is `update text origin <o> ... nhop <addr\|self> nlri <afi>/<safi> add <prefix>` | Adopt the canonical form as shown in `internal/component/bgp/format.go` |
| Python `emit-event` fake producer for `.ci` tests | External plugin JSON payload arrives as `json.RawMessage`; typed-handle subscribers fail the assertion and drop with a warn | In-process `internal/test/plugins/fakeredist/` producer with a `fakeredist emit` CommandDecl driven from `.ci` |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- Routes-as-events is the unifying abstraction. `redistribute` is a subscription filter, not an injection mechanism. This matches the existing sysrib+EventBus precedent.
- The evaluator runs inside the egress plugin, not inside the producer. Producers are protocol-neutral and have no redistribute knowledge.
- `update-route "*"` is a clean fan-out primitive; the plugin issues one call per accepted entry and the engine takes care of per-peer dispatch + per-peer nhop resolution.

## RFC Documentation

- RFC 4271 S5.1.3 NEXT_HOP -- satisfied via the reactor's `resolveNextHop(Self, family)` substitution. The plugin does not encode NEXT_HOP bytes; it requests `self` resolution.
- RFC 4271 S5.1.2 AS_PATH -- synthesized empty for locally-originated routes; per-peer path prepending (eBGP) happens in the reactor's outbound pipeline.

## Implementation Summary

### What Was Implemented
- (to be filled)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   |          |         |          |        |

### Fixes applied
- (to be filled)

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-verify-fast` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
