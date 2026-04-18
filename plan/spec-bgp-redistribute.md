# Spec: bgp-redistribute -- Cross-protocol redistribute egress plugin

| Field | Value |
|-------|-------|
| Status | design |
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

## Open Research Questions (block DESIGN gate)

1. Fake-producer placement. Two candidates:
   - (a) `internal/testplugins/fakeredist/` (new top-level test plugin
     package; blank-import guarded by a build tag or only wired in test
     binaries).
   - (b) `test/plugin/fakeredist/` (alongside `.ci` fixtures but still a
     Go package; unusual location).
   - (c) `internal/plugins/fakeredist/` with `Private: true` style gating,
     relying on config to not load it in production.
2. Explicit nexthop support (resolved): `RouteChangeEntry.NextHop` is a
   `netip.Addr` value; zero `Addr` sentinel means the consumer emits
   `nhop self` and relies on reactor per-peer substitution. Non-zero
   `Addr` from the producer is passed through as an explicit `nhop
   <addr>` in the command text. Producers that have no address to supply
   simply leave the field zero. No follow-up spec needed; the
   field is present from v1 and producers opt in when they have a value.

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
- EventBus `(<protocol>, route-change)` typed handle, subscribed at plugin startup.

### Event payload shape (batched)
| Field | Type | Notes |
|-------|------|-------|
| `Protocol` | string | "l2tp", "connected", ... (matches `RouteSource.Name`) |
| `Family` | string | "ipv4/unicast", "ipv6/unicast" |
| `Entries` | `[]RouteChangeEntry` | One batch per family per emission |

| `RouteChangeEntry` field | Type | Notes |
|---|---|---|
| `Action` | enum | `add` / `remove` |
| `Prefix` | netip.Prefix | |
| `NextHop` | netip.Addr (optional) | For locally-originated: invalid/zero -> egress synthesizes `next-hop self`. Non-zero is ignored for non-BGP sources (egress always uses self). |
| `Metric` | uint32 | Reserved for future use (not consulted in this spec) |
| `Metadata` | map[string]any | Free-form; ignored for non-BGP sources in this spec |

### Transformation Path
1. Engine delivers a batch to bgp-redistribute's callback.
2. For each entry: build `RedistRoute{Origin: batch.Protocol, Source: batch.Protocol, Family: batch.Family}`.
3. Consult `redistribute.Global().Accept(route, "bgp")`. If nil evaluator or Accept=false: skip.
4. For accepted `add`: build command text `announce <prefix> origin incomplete next-hop self` and call `plugin.UpdateRoute(ctx, "*", command)`.
5. For accepted `remove`: command text `withdraw <prefix>`; same dispatch path.
6. Engine resolves `"*"` to every up BGP peer, prepends `peer <addr> ` (`dispatch.go:566`), dispatcher hands the final text to the per-peer reactor, which parses the announce + runs `resolveNextHop(Self, family) -> conn.LocalAddr().IP` per peer.
7. Each peer gets an UPDATE with its own NEXT_HOP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Protocol producer -> this plugin | typed EventBus handle | [ ] unit test with fake bus |
| This plugin -> reactor | `update-route` RPC (text command) | [ ] functional test |
| Reactor -> wire | `resolveNextHop` substitutes Self per peer | [ ] functional test with 2 peers |

### Integration Points
- `internal/core/events/` (or each protocol's events subpackage) -- typed route-change handle per protocol. L2TP adds its handle in spec-l2tp-7c.
- `redistribute.Global()` evaluator -- read-only on the hot path.
- `plugin.UpdateRoute` RPC -- identical path bgp-rib uses today.

### Architectural Verification
- [ ] No direct import of any protocol producer package (l2tp, iface, ...)
- [ ] No import of `bgp/plugins/rib` either -- both plugins call `UpdateRoute` independently
- [ ] Plugin disabled = no route redistribution; no effect on bgp-rib or reactor
- [ ] Evaluator read is lock-free on hot path (atomic.Pointer)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Test-only fixture emits `(fakeproto, route-change)` add-entry with matching `import` rule | -> | bgp-redistribute filter accept -> UpdateRoute announce -> reactor -> UPDATE on wire | `test/plugin/bgp-redistribute-announce.ci` |
| Same event but NO matching rule | -> | filter rejects -> no UpdateRoute call | `test/plugin/bgp-redistribute-filtered-out.ci` |
| Two peers with distinct local addresses, matching rule | -> | single event -> two UPDATEs with distinct NEXT_HOPs | `test/plugin/bgp-redistribute-nexthop-self.ci` |
| Remove entry after add | -> | UpdateRoute withdraw -> WITHDRAWN_ROUTES on wire | `test/plugin/bgp-redistribute-withdraw.ci` |

Tests use a minimal Python test-plugin inside `tmpfs=*.run` that obtains an
SDK handle to the EventBus and emits synthetic `(fakeproto, route-change)`
batches. `fakeproto` is registered as a `RouteSource` by the same fixture at
startup.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin loaded; `(fakeproto, route-change)` handle registered; `redistribute.Global()` nil | No announcements; no panic on events |
| AC-2 | Evaluator has `import fakeproto { family ipv4/unicast; }` rule; add-batch with `/32` IPv4 entry | `update-route` dispatched with selector `*`, command starts `announce <prefix> origin incomplete next-hop self` |
| AC-3 | Same as AC-2 but family is `ipv6/unicast` | Dispatched; `/128` IPv6 command |
| AC-4 | Add-batch for family NOT in the import rule's family list | No dispatch |
| AC-5 | Add-batch for source NOT in any import rule | No dispatch |
| AC-6 | Source whose `RouteSource.Protocol == "bgp"` emits a route-change event | **Not handled** by this plugin; existing IngressFilter path handles BGP-sourced redistribute |
| AC-7 | Two BGP peers up with distinct local session addresses; accepted event | Each peer's UPDATE carries NEXT_HOP = its own local session address |
| AC-8 | Remove-batch for a previously announced entry | `update-route` dispatched with `withdraw <prefix>` |
| AC-9 | Config reload adds an import rule while plugin running | Subsequent events matching the new rule are announced; no plugin restart needed |
| AC-10 | Config reload removes an import rule | Previously-accepted events no longer trigger announce; in-flight routes already in Adj-RIB-Out remain (withdraw is only driven by source remove-events) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubscribe_BGPSources_Ignored` | `internal/component/bgp/plugins/redistribute/redistribute_test.go` | Plugin skips BGP-protocol sources at subscription time | |
| `TestSubscribe_NonBGPSources_Handled` | same | Plugin subscribes to each registered non-BGP source | |
| `TestHandleBatch_AcceptedAddDispatches` | same | Accepted add entries produce UpdateRoute calls with correct command text | |
| `TestHandleBatch_RejectedAddDoesNothing` | same | Rejected entries: zero UpdateRoute calls | |
| `TestHandleBatch_RemoveDispatchesWithdraw` | same | Remove entries use `withdraw` command | |
| `TestHandleBatch_NoEvaluator_Noop` | same | Global()==nil: no dispatches | |
| `TestHandleBatch_ReloadApplies` | same | Evaluator swap mid-run: next call uses new rules | |
| `TestAttrSynthesis` | same | Synthesized announce text carries `origin incomplete`, no `aspath`, `next-hop self` | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| n/a (no numeric surface; command text is string) | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-redistribute-announce` | `test/plugin/bgp-redistribute-announce.ci` | Fixture emits matching route event; peer receives UPDATE with synthesized attrs | |
| `bgp-redistribute-filtered-out` | `test/plugin/bgp-redistribute-filtered-out.ci` | Fixture emits event without matching rule; peer receives nothing | |
| `bgp-redistribute-nexthop-self` | `test/plugin/bgp-redistribute-nexthop-self.ci` | Two peers, distinct local addrs; each UPDATE carries its own NEXT_HOP | |
| `bgp-redistribute-withdraw` | `test/plugin/bgp-redistribute-withdraw.ci` | Remove event after add produces WITHDRAWN_ROUTES | |

### Future (if deferring any tests)
- Per-peer redistribute config (different import rules per peer) -- out of scope here; separate spec when per-peer YANG lands.
- Intra-BGP egress redistribution (`redistribute ibgp` as egress advertisement vs current ingress-ACL semantics) -- out of scope; the current IngressFilter keeps its meaning.
- Policy transformations (set-localpref, community-tag on redistribute) -- out of scope; route-map/policy is a separate feature.

## Files to Modify
- `internal/component/plugin/all/all.go` -- blank import of the new plugin package

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Plugin registration | [x] | `internal/component/bgp/plugins/redistribute/register.go` |
| YANG schema | [ ] no (reads existing `redistribute` container; no new leaves) | -- |
| CLI commands | [ ] no | -- |
| EventBus subscription | [x] | new plugin, uses existing typed-handle infrastructure |
| Functional tests | [x] | four `.ci` files above |
| bgp-redistribute status/metrics | [x] | simple Prometheus counters: `ze_bgp_redistribute_events_received`, `ze_bgp_redistribute_announcements`, `ze_bgp_redistribute_withdrawals`, `ze_bgp_redistribute_filtered_total` |

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
| 10 | Test infrastructure changed? | [x] | `docs/functional-tests.md` -- fixture-producer pattern (Python emits events via SDK) |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- BGP redistribute parity for non-BGP sources |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- redistribute now has egress subscriber |

## Files to Create
- `internal/component/bgp/plugins/redistribute/redistribute.go`
- `internal/component/bgp/plugins/redistribute/register.go`
- `internal/component/bgp/plugins/redistribute/redistribute_test.go`
- `test/plugin/bgp-redistribute-announce.ci`
- `test/plugin/bgp-redistribute-filtered-out.ci`
- `test/plugin/bgp-redistribute-nexthop-self.ci`
- `test/plugin/bgp-redistribute-withdraw.ci`

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
1. **Phase: plugin skeleton** -- `register.go` + empty `RunEngine`; registry test count bumped; blank import added. `make generate` + `TestAllPluginsRegistered`.
2. **Phase: subscribe to non-BGP protocol route-change handles** -- iterate `redistribute.SourceNames()`, filter out `Protocol == "bgp"`, resolve typed handle, subscribe on `OnStarted` (or `OnAllPluginsReady`). Unit tests with mock bus.
3. **Phase: handleBatch core** -- filter via evaluator, synthesize command text, dispatch via `plugin.UpdateRoute`. Unit tests cover every AC branch.
4. **Phase: metrics** -- four counters; register via ConfigureMetrics.
5. **Phase: functional tests** -- four `.ci` files; fixture-producer plugin emits synthetic events.
6. **Phase: docs** -- update listed files; add `<!-- source: -->` anchors.
7. **Phase: learned summary** -- write on completion.
8. **Full verification** -- `make ze-verify-fast`.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation + test |
| Correctness | Evaluator consulted per entry; BGP-source protocols skipped at subscription |
| Naming | Command text matches reactor's parser (`announce <prefix> origin incomplete next-hop self`) |
| Data flow | Plugin reads EventBus, writes UpdateRoute; no other couplings |
| Rule: no-layering | No parallel redistribute path; the existing IngressFilter unchanged |
| Rule: plugin-design | No import of sibling plugin packages; DispatchCommand for cross-plugin if ever needed |
| Rule: buffer-first | Command text assembled by append on a scratch buffer (RFC 4271 max line length); no fmt.Sprintf on hot path |
| Rule: goroutine-lifecycle | Subscription callback is long-lived (EventBus callback); no per-event goroutines |

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
