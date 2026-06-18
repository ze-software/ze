# Spec: isis-11-redistribution

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-isis-9-spf-rib.md |
| Phase | - |
| Updated | 2026-06-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-isis-0-umbrella.md` (row isis-11, AC-7/AC-8) and `plan/spec-isis-9-spf-rib.md` (SPF/origination must exist)
4. `internal/component/config/redistribute/registry.go` - `RouteSource`, `RegisterSource`
5. `internal/component/config/redistribute/consumer.go` - `RedistConsumer` interface, `RegisterConsumer`, `RouteEntry`
6. `internal/component/bgp/redistribute/{bgp.go,consumer.go}` - the source-registration and consumer template to mirror
7. `internal/component/bgp/plugins/redistribute_egress/register.go` (lines ~53-75) - consumer registered at OnStarted
8. `internal/plugins/connected/connected.go` - connected-prefix producer (RegisterSource + observe-and-emit)

## Task

Wire IS-IS into Ze's protocol-agnostic redistribution framework in **both
directions**, so an IS-IS node can mesh with the BGP engine (and with
connected / static routes) the way operators of vendor routers expect.

This is the "mesh with BGP via redistribution" link from the umbrella
(AC-7 and AC-8). It has two halves:

- **Producer side (IS-IS routes out to other protocols).** Registering the
  config-side `RouteSource` is NOT sufficient for BGP to receive IS-IS routes:
  the source registry only feeds the `redistribute-source` YANG validator and
  editor completion. To actually deliver routes, IS-IS MUST register as a
  `redistevents` PRODUCER, because the redistribute-orchestrator
  (`internal/component/bgp/plugins/redistribute_egress/redistribute.go`)
  subscribes ONLY to the producers returned by `redistevents.Producers()`.
  That producer wiring has four explicit, mandatory parts, mirroring
  `internal/plugins/connected/events/events.go`:
  1. `redistevents.RegisterProtocol("isis")` to allocate a `ProtocolID`.
  2. `redistevents.RegisterProducer(id)` so the protocol appears in
     `redistevents.Producers()` (without this the orchestrator never
     subscribes and no IS-IS route reaches BGP).
  3. A typed event handle
     `events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)`
     built locally in the IS-IS events package (value-typed payload; no
     handle pointer crosses a boundary).
  4. EMIT a `redistevents.RouteChangeBatch` on the typed handle whenever SPF
     route changes (add / remove) for a level, with `Protocol` set to the
     allocated IS-IS `ProtocolID`.
  Together with the SINGLE config source `isis` (registered via
  `redistribute.RegisterSource(redistribute.RouteSource{Name: "isis", Protocol: "isis", Description: "IS-IS SPF routes"})`, which returns an error and is wrapped in a `sync.Once` `mustRegister` exactly like BGP `RegisterBGPSources`), this lets
  `redistribute { destination bgp { import isis } }` pull IS-IS SPF routes
  into BGP via the `BGPConsumer`. There is ONE source name `isis`, not
  per-level `isis-l1`/`isis-l2`: `redistevents.RouteChangeBatch` has no
  level/source-name field, the orchestrator derives the source solely from
  `ProtocolName(b.Protocol)` (`redistribute_egress/redistribute.go:180-198`),
  and the loop-prevention evaluator matches on the consumer's importing name
  (`route.Origin == importingProtocol`, `config/redistribute/route.go:34-40`).
  A single `isis` keeps self-import auto-rejected and matches the single
  admin distance (umbrella Shared Contracts "Redistribution source"). This `redistevents` producer path is for
  REDISTRIBUTION (to the orchestrator, then the BGP consumer) ONLY; it is
  separate from FIB install. `redistevents` NEVER installs routes to the
  kernel; the kernel install of IS-IS routes is owned by spec-isis-9 via
  Loc-RIB insertion (`locrib.Path`), an entirely separate path. THIS spec
  makes IS-IS SPF results visible to the **redistribution framework** so the
  BGP consumer can advertise them, and registers the source names so the
  `redistribute-source` YANG validator and editor completion accept them.

- **Consumer side (other protocols' routes injected into IS-IS LSPs).**
  Implement the `RedistConsumer` interface (Name `isis`, `InjectRoute`,
  `WithdrawRoute`) and register it via `configredist.RegisterConsumer` from
  the IS-IS component's `OnStarted`. Injected routes (from `connected`,
  `static`, BGP) become Extended IP Reachability entries (TLV 135 for IPv4;
  IPv6 TLV 236 lands via spec-isis-12) in IS-IS LSPs, using the canonical
  TLV 135 / 236 entry layout from `plan/spec-isis-0-umbrella.md`
  `## Shared Contracts (canonical)`: TLV 135 (IPv4) carries a 4-byte metric
  (32-bit `PrefixMetric`), a control byte holding ONLY the up/down bit (0x80)
  + sub-TLV-present (0x40) + 6-bit prefix length, then `ceil(len/8)` prefix
  bytes. TLV 135 has NO external bit (RFC 5305 sec 4); the external (X 0x20)
  bit exists only on IPv6 TLV 236 (owned by isis-12). For IPv4 redistribution,
  set the up/down bit ONLY when a redistributed prefix is leaked to a lower
  level (RFC 2966); the up/down bit does NOT mark external origin, and there is
  no wire flag distinguishing external from internal reachability in TLV 135.
  Then trigger LSP re-origination and SPF.

This spec is purely redistribution. The FIB / kernel install of IS-IS SPF
routes is a separate path owned by spec-isis-9 (Loc-RIB insertion via
`locrib.Path` -> sysrib `OnChange` -> fibkernel); neither `redistevents` nor
the consumer `InjectRoute`/`WithdrawRoute` programs the kernel here.

Also in scope: IS-IS advertising its own enabled-interface prefixes
(especially passive interfaces) into its LSPs as connected-prefix
reachability, and ensuring the `redistribute` YANG model accepts `isis` as
both a `destination` protocol and the single `source` name.

Package: `internal/component/isis/redistribute/`. Depends on spec-isis-9
(SPF and LSP origination must already exist; the consumer adds reachability
TLVs to originated LSPs, the source exposes SPF results).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `internal/component/config/redistribute/registry.go` - source registry
  -> Constraint: `RouteSource{Name, Protocol, Description}`; re-register with same name+protocol is a no-op, different protocol is an error. IS-IS registers a SINGLE source `isis` (Protocol `isis`), mirroring BGP's `RegisterBGPSources` `sync.Once` pattern in `internal/component/bgp/redistribute/bgp.go`. No per-level source names (the event payload carries no level; see umbrella "Redistribution source")
- [ ] `internal/component/config/redistribute/consumer.go` - consumer registry + interface
  -> Constraint: `RedistConsumer` is `Name() string`, `InjectRoute(ctx, family.Family, RouteEntry)`, `WithdrawRoute(ctx, family.Family, prefix string)`. `RouteEntry{Prefix, NextHop}` carries only strings (value-typed, no cross-boundary pointers). One consumer per protocol; re-register is rejected
  -> Constraint: register at `OnStarted`, not `init` and not `OnAllPluginsReady` (the consumer only needs the engine handle, not other plugins). This follows the learned explicit-destination redistribution decision: runtime registration, not schema coupling
- [ ] `internal/component/bgp/redistribute/consumer.go` - consumer template
  -> Constraint: `InjectRoute` / `WithdrawRoute` MUST log on failure (never `_, _, _ =`). IS-IS consumer logs LSP-origination failures the same way
- [ ] `internal/plugins/connected/connected.go` - producer pattern
  -> Decision: source registration via a `sync.Once` `registerSources()`; value-typed `RouteEntry` at the boundary; reference-count prefixes if multiple addresses map to one prefix (per 685-redist-producers)
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md`
  -> Constraint: all IS-IS redistribution code lives under `internal/component/isis/redistribute/`; no `isis` spelling appears in the generic `config/redistribute` package
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md`
  -> Constraint: redistribute config is YANG; source/destination name is kebab-case (`isis`)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc5305.md` - Extended IP Reachability (TLV 135), wide metrics
  -> Constraint: injected (redistributed) routes are originated as TLV 135 Extended IP Reachability entries with a 32-bit metric; TLV 135 has only the up/down bit (RFC 5305 sec 4), NOT an external bit; up/down semantics per RFC 2966
- [ ] `rfc/short/rfc2966.md` - up/down bit for domain-wide prefix distribution
  -> Constraint: routes redistributed into IS-IS and leaked down (L2 -> L1) set the up/down bit so they are never re-advertised back up, preventing loops
- [ ] `rfc/short/rfc1195.md` - IS-IS for IP, IP Reachability semantics
  -> Constraint: RFC 1195 distinguished internal (TLV 128) vs external (TLV 130) IP reachability, but Ze originates the unified RFC 5305 TLV 135 instead, which has NO external bit; redistributed routes are plain TLV 135 entries (up/down bit only); metric handling per RFC 5305

**Key insights:** (minimal context to resume after compaction)
- Redistribution framework is protocol-agnostic and already shipped: source registry, consumer registry + `RedistConsumer`, YANG `redistribute { destination <proto> { import <source> } }`. IS-IS is a NEW single source (`isis`) and a NEW consumer (`isis`).
- BGP already produces (`bgp`/`ibgp`/`ebgp` sources) and consumes (`BGPConsumer`). IS-IS mirrors both.
- Value-typed events / `RouteEntry` only at the boundary; no pointers cross plugin/component boundaries.
- Consumer registered at `OnStarted`; source names registered via `sync.Once`.
- The kernel install of IS-IS routes is spec-isis-9's job, done via Loc-RIB insertion (`locrib.Path` -> sysrib -> fibkernel), NOT via `redistevents`. THIS spec is purely redistribution: it emits IS-IS SPF routes as `redistevents.RouteChangeBatch` to the redistribute-orchestrator (for BGP) and injects other protocols' routes into IS-IS LSPs; `redistevents` never installs to the FIB.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/component/config/redistribute/registry.go` - source registry exists; `RegisterSource` / `SourceNames` / `LookupSource`; `RouteSource` already lists `isis` as an example protocol in its doc comment but no IS-IS source is registered today
  -> Constraint: `RegisterSource` takes a `RouteSource` struct (NOT a string) and returns an error: `RegisterSource(RouteSource{Name: "isis", Protocol: "isis", Description: "IS-IS SPF routes"})`. IS-IS must call it once (single source), wrapped in a `sync.Once` `mustRegister` that logs on error, exactly like BGP `RegisterBGPSources` (`internal/component/bgp/redistribute/bgp.go:16-40`). Until it does, the `redistribute-source` validator rejects the name
- [ ] `internal/component/config/redistribute/consumer.go` - consumer registry exists; `RedistConsumer` interface, `RegisterConsumer`, `LookupConsumer`; no IS-IS consumer registered today
  -> Constraint: IS-IS must implement and register a `RedistConsumer` named `isis`
- [ ] `internal/component/bgp/redistribute/{bgp.go,consumer.go}` - BGP registers its sources via `RegisterBGPSources` (`sync.Once`) and consumes via `BGPConsumer` (translates `RouteEntry` to a text `update-route` command); `internal/component/bgp/plugins/redistribute_egress/register.go` registers the BGP consumer at `OnStarted`
  -> Constraint: IS-IS mirrors this exact shape; the IS-IS consumer translates `RouteEntry` into an LSP reachability TLV instead of a text command
- [ ] `internal/plugins/connected/connected.go` - connected source registered via `sync.Once`; observes `interface addr-added/removed`, ref-counts prefixes, emits `redistevents.RouteChangeBatch`
  -> Constraint: IS-IS connected-prefix advertisement reuses this idea but writes into IS-IS LSPs (own enabled/passive interfaces), not the redistevents bus
- [ ] `internal/component/config/redistribute/yang/ze-redistribute-conf.yang` - `destination` is a YANG `list` keyed by `protocol`; `import.source` carries `ze:validate "redistribute-source"`; `protocol` (destination) has NO validator (consumers register after YANG parse, validated at runtime)
  -> Constraint: no YANG schema change is needed to add `isis` as a destination (it is a free-form list key validated at runtime); `isis` becomes a valid source purely by registering it with the source registry, which feeds the `redistribute-source` validator/completion

**Behavior to preserve:**
- BGP, connected, static, L2TP redistribution sources and the BGP consumer remain independent and functional
- `redistribute { destination bgp { import ibgp/ebgp/connected/static } }` semantics unchanged
- Loop-prevention in the evaluator (`route.Origin == importingProtocol`) unchanged; IS-IS as a new consumer name participates the same way
- SPF and the spec-isis-9 Loc-RIB install path (`locrib.Path` -> sysrib -> fibkernel) unchanged; this spec adds a redistribution read path (SPF routes out via `redistevents`) and a TLV-injection write path (routes in to LSPs) on top, and touches neither the FIB nor `redistevents`-to-kernel (there is no such path)

**Behavior to change:**
- One new redistribution source `isis` (Protocol `isis`)
- One new redistribution consumer `isis`
- IS-IS LSP origination gains externally-injected reachability TLVs and own connected-interface prefixes
- The `redistribute` config now accepts `destination isis { ... }` and `import isis`

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Producer direction:** IS-IS SPF results (from spec-isis-9) become available to the redistribution framework under the single source name `isis`; a `redistribute { destination bgp { import isis } }` rule causes the BGP consumer to advertise them (both L1 and L2 routes; level is not a redistribution selector in v1).
- **Consumer direction:** routes arrive at `RedistConsumer.InjectRoute` / `WithdrawRoute` (called by the redistribute orchestrator) for sources `connected`, `static`, `bgp` when a `redistribute { destination isis { import <source> } }` rule is configured.
- **Connected-prefix direction:** IS-IS enumerates its own enabled (and passive) interface prefixes at circuit-up.

### Transformation Path
1. **Source + producer registration:** IS-IS component init / OnStarted calls `RegisterSource(RouteSource{Name: "isis", Protocol: "isis", Description: ...})` (struct arg, error return, `sync.Once` `mustRegister` wrapper) so the name is known to the framework and the `redistribute-source` validator. Separately and mandatorily, the IS-IS events package registers IS-IS as a `redistevents` PRODUCER: `redistevents.RegisterProtocol("isis")` allocates a `ProtocolID`, `redistevents.RegisterProducer(id)` puts it in `redistevents.Producers()`, and `events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)` builds the typed handle. Source registration alone is NOT enough: the orchestrator subscribes only to producers from `redistevents.Producers()`, so without `RegisterProducer` no IS-IS route ever reaches BGP.
2. **Producer read / emit:** when `import isis` is configured for some destination, IS-IS EMITS its SPF route changes (both levels) as `redistevents.RouteChangeBatch` (on the typed handle, `Protocol` = the single IS-IS `ProtocolID`) to the redistribute-orchestrator (`internal/component/bgp/plugins/redistribute_egress/redistribute.go`), which has subscribed because IS-IS is in `redistevents.Producers()`; the orchestrator resolves the source name as `ProtocolName(Protocol)` = `isis`. This is the REDISTRIBUTION path only; it does not touch the FIB (kernel install is the separate isis-9 Loc-RIB path).
3. **BGP consume:** orchestrator dispatches IS-IS routes to `BGPConsumer.InjectRoute` -> text `update-route` -> reactor -> BGP RIB -> BGP UPDATE.
4. **Consumer register:** IS-IS `OnStarted` calls `RegisterConsumer(isisConsumer)`.
5. **IS-IS consume (inject):** orchestrator calls `isisConsumer.InjectRoute(ctx, fam, RouteEntry)` -> IS-IS adds an Extended IP Reachability entry (TLV 135 IPv4; TLV 236 IPv6 via isis-12) using the canonical TLV 135 / 236 layout from the umbrella Shared Contracts, with a FIXED default redistribution metric applied IS-IS-side (the generic `RouteEntry` carries no metric, so v1 uses a single code constant -- NOT a config leaf), to the local LSP set -> LSP re-origination -> flooding -> SPF on peers. For IPv4 (TLV 135) the only wire flag is the up/down bit, set only when the prefix is later leaked to a lower level (RFC 2966); TLV 135 has no external bit. For IPv6 (TLV 236, isis-12) the external bit (X) is set for redistributed prefixes.
6. **IS-IS consume (withdraw):** `isisConsumer.WithdrawRoute(ctx, fam, prefix)` -> remove the reachability entry -> re-originate LSP -> peers re-run SPF and withdraw the route from their kernel.
7. **Connected advertisement:** IS-IS originates its own enabled-interface prefixes (passive interfaces especially) as internal reachability in its LSPs.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IS-IS engine <-> redistribute source registry | `RegisterSource` value type | [ ] |
| redistribute orchestrator <-> IS-IS consumer | `RedistConsumer.InjectRoute/WithdrawRoute`, value-typed `RouteEntry` | [ ] |
| IS-IS engine <-> BGP | source registry (producer) + existing `BGPConsumer` (consumer) | [ ] |
| Injected route <-> LSP | new TLV 135 (236 for v6) reachability entry, triggers re-origination | [ ] |
| Config tree <-> consumer | `destination isis { import <source> }` parsed, dispatched at runtime | [ ] |

### Integration Points
- New package `internal/component/isis/redistribute/` (`source.go`, `consumer.go`)
- `RegisterSource` (registry.go) and `RegisterConsumer` (consumer.go) in the generic framework
- IS-IS LSP origination (spec-isis-6) to add/remove reachability TLVs
- IS-IS SPF (spec-isis-9) to expose routes per level as the producer
- `redistribute` YANG validator / completion (source names) -- no schema change, registry-driven

### Architectural Verification
- [ ] No bypassed layers (inject -> LSP origination -> flooding -> SPF; not direct route push)
- [ ] No unintended coupling (no `isis` spelling in generic `config/redistribute`; only registration calls)
- [ ] No duplicated functionality (reuses source/consumer registries and the BGP consumer; does not re-implement redistribution)
- [ ] Zero-copy / value-typed preserved (`RouteEntry`/`RouteSource` are value types; no pointers cross the boundary)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The generic `redistribute` framework accepts `isis` as a destination with no YANG schema change (free-form list key, runtime-validated) | `ze-redistribute-conf.yang` (no validator on `protocol`); learned explicit-destination redistribution decision | Need a YANG/validator edit | parse `destination isis { ... }` config in a unit/functional test | unvalidated |
| A-2 | `isis` becomes a valid source purely by `RegisterSource`, and the `redistribute-source` validator picks it up | `registry.go` `SourceNames`/`LookupSource`; connected/BGP precedent | Need validator wiring | config with `import isis` validates after registration | unvalidated |
| A-3 | The `RedistConsumer` `InjectRoute`/`WithdrawRoute` (string `Prefix`/`NextHop`) is sufficient to originate a TLV 135 reachability entry, with the IS-IS consumer applying a FIXED default metric (a code constant, no config leaf) since `RouteEntry` carries none | `consumer.go` `RouteEntry` has no Metric field | If a configurable/per-route metric is needed, extend the generic `RouteEntry` (metric field) or add a YANG leaf -- both are future work, not v1 | consumer test asserting the TLV uses the fixed default metric | unvalidated |
| A-4 | A single `isis` producer exporting both L1 and L2 SPF routes is sufficient for "mesh with BGP" (no per-level redistribution selector in v1) | `RouteChangeBatch` has no level field; orchestrator derives source from `ProtocolName(Protocol)` only | If operators need per-level selection, add a level field to `RouteChangeBatch` (future) | producer test: both an L1 and an L2 route reach BGP via `import isis` | unvalidated |
| A-5 | Registering the consumer at `OnStarted` is early enough for the orchestrator to dispatch (orchestrator depends on consumers being present before config apply) | `redistribute_egress/register.go:59-66`; learned explicit-destination registration decision | Re-order to a different SDK hook | functional test: configured import reaches the consumer | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Redistributed routes leaked L2 -> L1 without the up/down bit cause a routing loop | loop in mixed L1L2 topology with redistribution | Enforce RFC 2966 up/down bit on injected/leaked reachability; interop test |
| R-2 | Consumer registered but never invoked | configured import silently does nothing | Wiring test asserts a connected/static/BGP route appears in an IS-IS LSP end-to-end |
| R-3 | Silent error discard in Inject/Withdraw | failed LSP re-origination invisible | Log every failure; assert a metric/counter |
| R-4 | Inject/withdraw churn re-originates LSPs too aggressively | LSP sequence-number burn under flapping source | Debounce re-origination; batch reachability changes per origination cycle |
| R-5 | Operators expect per-level redistribution (`isis-l1`/`isis-l2`) and the single `isis` source surprises them | support question / config rejected | Document the single-source v1 model in the guide and Known Limitations; per-level is future work needing a `RouteChangeBatch` level field |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| IS-IS events package imported (producer wiring) | -> | `RegisterProtocol("isis")` + `RegisterProducer(id)` -> IS-IS in `redistevents.Producers()` -> orchestrator subscribes | `TestISISProducerRegistered` (asserts `Producers()` contains the IS-IS `ProtocolID` and `ProtocolIDOf("isis")` resolves) |
| config `redistribute { destination bgp { import isis } }`, IS-IS SPF route present | -> | IS-IS source `isis` registered AND IS-IS emits `RouteChangeBatch` -> orchestrator -> `BGPConsumer.InjectRoute` -> BGP RIB | `TestISISRedistSourceToBGP` + `test/isis/isis-redist-bgp.ci` (IS-IS route appears in BGP) |
| config `redistribute { destination isis { import connected } }`, a connected prefix exists | -> | `isisConsumer.InjectRoute` -> TLV 135 in local LSP -> re-origination | `TestISISRedistConsumerConnected` + `test/isis/isis-redist-bgp.ci` (connected route appears in an IS-IS LSP) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `redistribute { destination bgp { import isis } }` with an IS-IS route present | The IS-IS route is advertised by BGP (appears in the BGP RIB / UPDATE) |
| AC-2 | `redistribute { destination bgp { import isis } }` with both an L1-only and an L2-only IS-IS route present | BOTH routes are advertised by BGP via the single `isis` source (level is not a redistribution selector in v1; per-level selection is documented future work) |
| AC-3 | `redistribute { destination isis { import connected } }` with a connected prefix | The connected prefix appears as an Extended IP Reachability (TLV 135) entry in the local IS-IS LSP and in peers' RIBs |
| AC-4 | `redistribute { destination isis { import static } }` with a static route | The static prefix appears as a TLV 135 entry in the local IS-IS LSP |
| AC-5 | `redistribute { destination isis { import bgp } }` with a BGP-learned route | The BGP prefix appears as a TLV 135 entry in the local IS-IS LSP; the up/down bit is 0 on first injection (TLV 135 has NO external bit -- RFC 5305 sec 4) and is set to 1 only if/when the prefix is later leaked to a lower level |
| AC-6 | An injected route is withdrawn (source removes it) | `WithdrawRoute` removes the TLV entry, the LSP is re-originated, and peers withdraw the route from their kernel |
| AC-7 | An IS-IS route is withdrawn (SPF removes it) while imported into BGP | The route is withdrawn from BGP (withdrawal propagates producer-side) |
| AC-8 | IS-IS configured with a passive interface | The passive interface's prefix is advertised into the LSP as internal reachability without forming an adjacency on it |
| AC-9 | `import isis` configured before the source is registered, then registered | After registration the config validates and routes flow (registration-order tolerant) |
| AC-10 | `redistribute { destination isis { import isis } }` (self-import) | Loop-prevention rejects redistributing IS-IS into IS-IS (origin `isis` == importing protocol `isis`); the single source name is what makes this auto-rejection work |
| AC-11 | The IS-IS events package is imported (producer wiring loaded) | `redistevents.Producers()` contains the IS-IS `ProtocolID`, `redistevents.ProtocolIDOf("isis")` resolves, and the orchestrator subscribes to IS-IS. Registering only the config `RouteSource` (without `RegisterProducer`) does NOT make IS-IS appear in `Producers()` and no route reaches BGP |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Redistributes IS-IS into BGP (umbrella AC-7) | SPF route -> `isis` source -> orchestrator -> `BGPConsumer.InjectRoute` -> reactor -> BGP RIB | `TestISISRedistSourceToBGP`, `test/isis/isis-redist-bgp.ci` |
| 2 | Redistributes connected into IS-IS (umbrella AC-8) | connected prefix -> orchestrator -> `isisConsumer.InjectRoute` -> TLV 135 -> LSP re-origination -> flooding -> peer SPF/RIB | `TestISISRedistConsumerConnected`, `test/isis/isis-redist-bgp.ci` |
| 3 | Redistributes static into IS-IS | static prefix -> orchestrator -> `isisConsumer.InjectRoute` -> TLV 135 in LSP | `TestISISRedistConsumerStatic`, `test/isis/isis-redist-bgp.ci` |
| 4 | Redistributes BGP into IS-IS | BGP route -> orchestrator -> `isisConsumer.InjectRoute` -> TLV 135 entry (up/down bit on down-level leak; no external bit) | `TestISISRedistConsumerBGP`, `test/isis/isis-redist-bgp.ci` |
| 5 | Advertises a passive interface prefix | circuit-up -> connected-prefix enumeration -> internal reachability TLV in LSP | `TestISISConnectedAdvertise` |

## 🧪 TDD Test Plan

Every AC has at least one exact named test below (AC mapping in the rightmost
column).

| Test | File | Validates | Covers AC | Status |
|------|------|-----------|-----------|--------|
| `TestISISProducerRegistered` | `internal/component/isis/redistribute/events/events_test.go` | producer wiring: `RegisterProtocol("isis")` + `RegisterProducer(id)` put IS-IS in `redistevents.Producers()`; `ProtocolIDOf("isis")` resolves; registering only the config `RouteSource` does NOT add it to `Producers()` | AC-11 | |
| `TestISISRegisterSource` | `internal/component/isis/redistribute/source_test.go` | single source `isis` registered with Protocol `isis`; idempotent; `LookupSource` finds it; no `isis-l1`/`isis-l2` names | AC-1 | |
| `TestISISRedistSourceToBGP` | `internal/component/isis/redistribute/source_test.go` | the `isis` source emits a `RouteChangeBatch` (`Protocol` = the isis ProtocolID) that reaches the BGP consumer; BOTH an L1-only and an L2-only route are exported (single source, no per-level selector) | AC-1, AC-2 | |
| `TestISISRedistSourceWithdrawToBGP` | `internal/component/isis/redistribute/source_test.go` | when SPF removes an imported IS-IS route, an `ActionRemove` `RouteChangeBatch` is emitted and the route is withdrawn from BGP (producer-side withdraw propagation) | AC-7 | |
| `TestISISRedistConsumerConnected` | `internal/component/isis/redistribute/consumer_test.go` | `InjectRoute` for a `connected` source originates a TLV 135 reachability entry in the local LSP with the FIXED default redistribution metric (no config leaf) (connected import) | AC-3 | |
| `TestISISRedistConsumerStatic` | `internal/component/isis/redistribute/consumer_test.go` | `InjectRoute` for a `static` source originates a TLV 135 entry in the local LSP (static import) | AC-4 | |
| `TestISISRedistConsumerBGP` | `internal/component/isis/redistribute/consumer_test.go` | `InjectRoute` for a `bgp` source originates a TLV 135 entry with up/down bit 0 on first injection (TLV 135 has no external bit) (BGP import) | AC-5 | |
| `TestISISRedistConsumerWithdraw` | `internal/component/isis/redistribute/consumer_test.go` | `WithdrawRoute` removes the entry and re-originates the LSP (consumer-side withdraw propagation) | AC-6 | |
| `TestISISRedistConsumerName` | `internal/component/isis/redistribute/consumer_test.go` | `Name()` returns `isis`; registered once | AC-3, AC-4, AC-5 | |
| `TestISISRedistConsumerUpDownBit` | `internal/component/isis/redistribute/consumer_test.go` | a redistributed TLV 135 entry has up/down bit 0 on first injection and 1 only when leaked to a lower level (RFC 2966); TLV 135 carries NO external bit (RFC 5305 sec 4) -- the codec exposes no external flag for IPv4 | AC-5 | |
| `TestISISRedistConsumerLogsFailure` | `internal/component/isis/redistribute/consumer_test.go` | LSP-origination failure is logged, not swallowed (regression guard) | AC-3..AC-6 | |
| `TestISISConnectedAdvertise` | `internal/component/isis/redistribute/source_test.go` | enabled/passive interface prefixes appear as internal reachability without an adjacency | AC-8 | |
| `TestISISRedistRegistrationOrder` | `internal/component/isis/redistribute/source_test.go` | registration-order tolerance: producer registered before AND after the consumer/orchestrator both deliver routes (run subtests with each order) | AC-9 | |
| `TestISISRedistSelfImportRejected` | `internal/component/isis/redistribute/consumer_test.go` | self-import rejection: `destination isis { import isis }` is a no-op; IS-IS does not re-import its own routes (origin `isis` == importing protocol `isis`) | AC-10 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Redistribution prefix metric (TLV 135, 32-bit) | 0..4294967295 | 4294967295 | N/A | N/A (full 32-bit field; SPF excludes values >= MAX_PATH_METRIC 0xFE000000, but the TLV field itself is not 24-bit capped) |
| Default redistribution metric (FIXED code constant, no config leaf in v1) | a single constant in 0..4294967295 (32-bit TLV 135 field) | the constant | N/A | N/A (configurable/per-route metric is future work, not v1) |
| Up/down bit | 0..1 | 1 | N/A | N/A |
| Source name | `isis` only (single source) | n/a | unregistered name rejected | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-redist-bgp` | `test/isis/isis-redist-bgp.ci` | IS-IS route appears in BGP (AC-1), connected/static/BGP imports appear in IS-IS LSPs (AC-3/AC-4/AC-5) | |

### Interop Tests (MANDATORY for protocol features)
<!-- Redistribution into LSP TLVs is wire-affecting; interop verifies FRR accepts the external reachability. -->
The `isis-redist-frr` scenario is MANDATORY and not deferrable (consistent with
the spec-isis-0 umbrella "Test + interop wiring" requirement and spec-isis-13,
which owns the FRR redistribution interop scenarios).

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-redist-frr` (MANDATORY) | `test/interop/scenarios/` | FRR isisd | FRR installs IS-IS reachability redistributed by Ze (connected/static/BGP -> TLV 135, up/down bit honoured) | |

### Future (if deferring any tests)
- IPv6 redistribution (TLV 236) interop is owned by spec-isis-12; cross-reference only.

## Files to Modify
- `internal/component/isis/register.go` - call `RegisterConsumer` at `OnStarted` and `RegisterSource(RouteSource{Name: "isis", Protocol: "isis", Description: ...})` (single source, struct arg + error return, `sync.Once` `mustRegister`); ensure the new `internal/component/isis/redistribute/events` package is imported so its producer registration (`RegisterProtocol` + `RegisterProducer`) runs and IS-IS appears in `redistevents.Producers()`
- `internal/component/isis/lsdb/...` - LSP origination accepts injected reachability + connected-interface prefixes (depends on spec-isis-6 origination hooks)
- `internal/component/isis/spf/...` - expose per-level SPF routes to the producer read path (depends on spec-isis-9)
- `internal/component/config/redistribute/...` - NO source/consumer code change expected; only confirm a registration call site exists. Modify ONLY if a registration hook or validator wiring is missing
- `docs/guide/isis.md`, `docs/guide/configuration.md`, `docs/comparison.md` - IS-IS redistribution rows (also tracked in isis-13)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | No | none -- `destination isis` is a runtime-validated free-form list key (A-1) |
| YANG validation constraints | No | `redistribute-source` validator picks up `isis` from the source registry (A-2); no new leaves |
| YANG custom validators | No | none new; existing `redistribute-source` `CompleteFn` enumerates registered sources |
| CLI commands/flags | No | reuses `redistribute` config; `show isis database/route` reflect injected reachability (isis-13) |
| CLI grammar (action before identifier) | No | n/a -- config block, not a verb |
| Editor autocomplete | No | `isis` appears via the source registry `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/isis/isis-redist-bgp.ci` |
| Pipe completeness | No | n/a -- no new show command in this spec |
| Doctor check for runtime dependencies | No | none new (no new socket/path) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers `ze_isis_redist_injected_total{source,afi}`, `ze_isis_redist_withdrawn_total{source,afi}`, `ze_isis_redist_inject_failures_total{source}`, `ze_isis_lsp_reoriginations_total{level}` (per the umbrella "Metrics (canonical)" table). Per-owner registration here, NOT in isis-13 (isis-13 only scrapes/asserts) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (IS-IS redistribution both directions) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`destination isis`, `import isis`) |
| 3 | CLI command added/changed? | No | none in this spec |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | IS-IS is a component (isis-4), not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/isis.md` (redistribution section) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/isis.md` (TLV 135 reachability use; up/down bit, no external bit) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc5305.md`, `rfc/short/rfc2966.md`, `rfc/short/rfc1195.md` |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (`test/isis/isis-redist-bgp.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (IS-IS<->BGP redistribution) |
| 12 | Internal architecture changed? | No | covered by isis-0/isis-4 component rows |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (IS-IS redistribution counters) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (new redistribute source `isis`, consumer `isis`) |
| 16 | Any changed source file referenced by doc source anchors? | No | grep `docs/` at completion |
| 17 | Existing docs show examples for this area? | No | verify `redistribute` examples at completion |

## Files to Create
- `internal/component/isis/redistribute/events/events.go` - the `redistevents` PRODUCER wiring (mirrors `internal/plugins/connected/events/events.go`): `Namespace = "isis"`, `ProtocolID = redistevents.RegisterProtocol(Namespace)`, `redistevents.RegisterProducer(ProtocolID)`, and the typed handle `RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)`. Without this file no IS-IS route reaches the orchestrator (it subscribes only to `redistevents.Producers()`).
- `internal/component/isis/redistribute/events/events_test.go` - `TestISISProducerRegistered`: after import, `redistevents.Producers()` contains the IS-IS `ProtocolID`, and `redistevents.ProtocolIDOf("isis")` resolves to it
- `internal/component/isis/redistribute/source.go` - `RegisterSource(RouteSource{Name: "isis", Protocol: "isis", Description: ...})` (single, struct arg + error return, `sync.Once` `mustRegister`) + producer read that EMITS `redistevents.RouteChangeBatch` on the typed handle for SPF route changes (both levels) + connected-prefix advertisement
- `internal/component/isis/redistribute/source_test.go` - `TestISISRegisterSource`, `TestISISRedistSourceToBGP`, `TestISISRedistSourceWithdrawToBGP`, `TestISISConnectedAdvertise`, `TestISISRedistRegistrationOrder`
- `internal/component/isis/redistribute/consumer.go` - `RedistConsumer` impl (Name `isis`, `InjectRoute`, `WithdrawRoute`) translating `RouteEntry` to TLV 135 reachability
- `internal/component/isis/redistribute/consumer_test.go` - `TestISISRedistConsumerConnected`, `*Static`, `*BGP`, `*Withdraw`, `*Name`, `*UpDownBit`, `*LogsFailure`, `TestISISRedistSelfImportRejected`
- `test/isis/isis-redist-bgp.ci` - functional test: IS-IS route into BGP and connected/static/BGP routes into IS-IS LSPs
- `test/interop/scenarios/isis-redist-frr/` - MANDATORY FRR interop for redistributed reachability (not deferrable; consistent with the spec-isis-0 umbrella "Test + interop wiring" and spec-isis-13)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `spec-isis-9-spf-rib.md` |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register the `redistevents` producer (RegisterProtocol + RegisterProducer + typed RouteChange handle), register source names and a stub consumer; write failing wiring tests
   - Tests: `TestISISProducerRegistered`, `TestISISRegisterSource`, `TestISISRedistConsumerName`
   - Files: `internal/component/isis/redistribute/events/events.go` (RegisterProtocol + RegisterProducer + `events.Register[*RouteChangeBatch]`), `internal/component/isis/redistribute/source.go` (`RegisterSource(RouteSource{Name: "isis", Protocol: "isis", ...})`), `consumer.go` (Name + stub Inject/Withdraw), `internal/component/isis/register.go` (import the events package; RegisterConsumer at OnStarted)
   - Verify: IS-IS in `redistevents.Producers()` and `ProtocolIDOf("isis")` resolves; source `isis` visible in `SourceNames`; consumer in `ConsumerNames`; wiring tests fail because Inject/Withdraw are stubs
2. **Phase: Consumer inject/withdraw -> LSP TLV** -- translate `RouteEntry` into TLV 135 reachability, trigger re-origination
   - Tests: `TestISISRedistConsumerConnected`, `TestISISRedistConsumerStatic`, `TestISISRedistConsumerBGP`, `TestISISRedistConsumerWithdraw`, `TestISISRedistConsumerUpDownBit`, `TestISISRedistConsumerLogsFailure`
   - Files: `consumer.go`, IS-IS LSP origination hook (`lsdb/`)
   - Verify: injected prefix appears as a TLV 135 entry; withdraw removes it; up/down bit set only on down-level leak (no external bit on TLV 135); failures logged
3. **Phase: Connected-prefix advertisement** -- enumerate own enabled/passive interface prefixes into LSPs
   - Tests: `TestISISConnectedAdvertise`
   - Files: `source.go`, circuit/interface hookup
   - Verify: passive-interface prefix advertised as internal reachability without an adjacency
4. **Phase: Producer read -> BGP** -- expose IS-IS SPF routes (both levels) to the orchestrator/BGP consumer by EMITTING `redistevents.RouteChangeBatch` (`Protocol` = the single isis ProtocolID) on the typed handle
   - Tests: `TestISISRedistSourceToBGP`, `TestISISRedistSourceWithdrawToBGP`
   - Files: `source.go`, `redistribute/events/events.go`, SPF read path (spec-isis-9 output)
   - Verify: both an L1 and an L2 route reach `BGPConsumer.InjectRoute` via the single `isis` source; an `ActionAdd` batch reaches BGP and an `ActionRemove` batch withdraws from BGP
5. **Phase: Loop-prevention + up/down bit** -- reject IS-IS self-import; set up/down bit on leaked/external reachability
   - Tests: `TestISISRedistSelfImportRejected`, `TestISISRedistRegistrationOrder`; functional check
   - Files: `consumer.go`, origination
   - Verify: `destination isis { import isis }` is a no-op (IS-IS does not re-import its own routes); L2->L1 leaked routes set the up/down bit; producer registered before/after the consumer both deliver routes
6. **Functional tests** -- `test/isis/isis-redist-bgp.ci` covering IS-IS export plus connected/static/BGP imports
7. **RFC refs** -- add `// RFC 5305 Section ...` / `// RFC 2966 Section ...` comments above the TLV and up/down enforcing code
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Both redistribution directions work end-to-end (no broken link); matches BGP's producer+consumer parity |
| Correctness | TLV 135 reachability encoded per RFC 5305; up/down bit per RFC 2966; metric within range |
| Naming | single source name `isis`; consumer name `isis`; config kebab-case; no `isis-l1`/`isis-l2` |
| Data flow | inject -> LSP origination -> flooding -> SPF; producer reads SPF (both levels) and emits one `isis` batch stream; no direct route push, no isis spelling in generic redistribute |
| CLI grammar | n/a (config block) |
| Doctor checks | none new |
| YANG validation | `isis` validates via the source registry; `destination isis` accepted at runtime |
| Prometheus counters | inject/withdraw/re-origination/failure counters defined and registered |
| Rule: plugin-self-containment | all IS-IS redistribution code under `internal/component/isis/redistribute/` |
| Rule: no silent error discard | Inject/Withdraw log every failure (regression guard) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| IS-IS redistevents producer registered | `grep -rn 'RegisterProtocol\|RegisterProducer' internal/component/isis/redistribute/events/` ; `TestISISProducerRegistered` asserts IS-IS in `redistevents.Producers()` |
| IS-IS redistribute source registered | `grep -r 'RegisterSource' internal/component/isis/redistribute/` ; source in `SourceNames` test |
| IS-IS redistribute consumer registered | `grep -r 'RegisterConsumer' internal/component/isis/` ; consumer in `ConsumerNames` test |
| Source + consumer files | `ls internal/component/isis/redistribute/{source,consumer}.go` |
| Functional test | `ls test/isis/isis-redist-bgp.ci` |
| Import/export paths proven | run `isis-redist-bgp.ci`; assert IS-IS route in BGP and connected/static/BGP routes in LSPs |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `RouteEntry.Prefix`/`NextHop` parsed/validated before forming a TLV; reject malformed prefixes |
| Loop prevention | self-import rejected; up/down bit enforced on leaked/external reachability (no re-advertise loop) |
| Resource exhaustion | cap injected reachability per LSP; bound re-origination rate (debounce) |
| Error leakage | log inject/withdraw failures at warn without leaking sensitive data |
| Spoofing | TLV 135 (IPv4) has no external bit, so redistributed routes are not wire-distinguishable from internal reachability; the mitigation is that they carry the originating system's LSP identity and the up/down bit on leak. For IPv6 (TLV 236, isis-12) the external bit IS set. Do not fabricate an IPv4 external marking the protocol lacks |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- Redistribution into IS-IS is "a consumer that originates TLVs" and out of IS-IS is "one named source (`isis`) emitting SPF route changes"; the framework does all dispatch. The only IS-IS-specific work is TLV 135 origination + up/down bit + the single-source SPF route exposure.

## Core Insight
The mesh-with-BGP link is almost entirely registration plus a TLV write: the
protocol-agnostic source/consumer registries already handle dispatch, loop
prevention, and config. IS-IS contributes one source name (`isis`), one
consumer that writes Extended IP Reachability into its own LSPs, and a read of
SPF output. The novelty is concentrated in correct up/down marking (RFC 5305
sec 4 / RFC 2966) -- and in NOT inventing an external bit that TLV 135 does not
have -- not in plumbing.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Single `isis` source | Two sources `isis-l1`/`isis-l2` (per level) | `RouteChangeBatch` has no level/source field; the orchestrator derives the source from `ProtocolName(Protocol)` and loop-prevention matches `route.Origin == importingProtocol`. Two protocol IDs would break self-import auto-rejection (AC-10). Single `isis` is the only cleanly-implementable choice and matches the single admin distance; per-level is future work needing a payload level field |
| Consumer writes TLV 135 into local LSPs | Direct SPF/RIB injection | Reuses LSP origination + flooding; peers learn via normal IS-IS, not a side channel |
| Register consumer at `OnStarted`, sources via `sync.Once` | init-time consumer | Consumer needs engine handle; matches BGP egress plugin and learned explicit-destination guidance |
| No YANG schema change for `destination isis` | New schema/validator | `destination.protocol` is a runtime-validated free-form list key (learned explicit-destination decision) |
| Up/down bit on leaked routes (TLV 135 has no external bit) | Inventing an external flag for IPv4 | RFC 5305 sec 4 defines only up/down + S in the TLV 135 control octet; the up/down bit (RFC 2966) is set on L2->L1 leak for loop prevention. The external bit exists only on IPv6 TLV 236 (isis-12) |

## Known Limitations
- Single redistribution source `isis` (no per-level `isis-l1`/`isis-l2` selection). Both L1 and L2 SPF routes are exported under `isis`. Per-level redistribution selection is future work: it needs a level field added to `redistevents.RouteChangeBatch` (or a payload that lets the orchestrator distinguish the level), because today the orchestrator derives the source purely from `ProtocolName(Protocol)`.
- TLV 135 (IPv4) carries NO external bit (RFC 5305 sec 4): redistributed IPv4 routes are ordinary TLV 135 entries with only the up/down bit (set on down-level leak). The external (X) bit is IPv6-only (TLV 236, isis-12).
- IPv4 only here (TLV 135); IPv6 redistribution (TLV 236) is spec-isis-12.
- `RouteEntry` carries no metric, so the IS-IS consumer applies a FIXED default redistribution metric (a code constant, NOT a YANG leaf -- consistent with "no new config leaves" in the Integration Checklist). A configurable or per-route metric is future work needing either a metric field on the generic `RouteEntry` or a new YANG leaf; neither is in v1.
- Route-map / policy-based redistribution filtering beyond source selection is out of scope (the existing flat evaluator governs source/family selection only).

## RFC Documentation

Add `// RFC 5305 Section X.Y: "<quoted requirement>"` above TLV 135 reachability
origination, and `// RFC 2966 Section X.Y` above up/down-bit enforcement.
MUST document: that TLV 135 has no external bit (only up/down + sub-TLV-present
in the control octet, RFC 5305 sec 4), up/down bit set/clear rules (RFC 2966),
the 32-bit prefix-metric range, and the no-re-advertise loop-prevention rule.

## Implementation Summary

### What Was Implemented
- [To be filled]

### Bugs Found/Fixed
- [To be filled]

### Documentation Updates
- [To be filled]

### Deviations from Plan
- [To be filled]

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

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| IS-IS routes into BGP (umbrella AC-7) | functional test | `test/isis/isis-redist-bgp.ci` (AC-1) |
| connected into IS-IS LSPs (umbrella AC-8) | functional test | `test/isis/isis-redist-bgp.ci` (AC-3) |
| static / BGP into IS-IS LSPs (umbrella AC-8) | functional test | `test/isis/isis-redist-bgp.ci` plus `TestISISRedistConsumerStatic` (AC-4) and `TestISISRedistConsumerBGP` (AC-5) |
| Redistributed reachability (both directions) accepted by FRR | interop test | `isis-redist-frr` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/isis/redistribute/`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features (out-of-scope honoured)
- [ ] Single responsibility per file (source vs consumer)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no isis spelling in generic redistribute)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-isis-11-redistribution.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-isis-11-redistribution.md`

## Cross-References
- `plan/spec-isis-0-umbrella.md` -- umbrella, AC-7 / AC-8 (mesh with BGP via redistribution), row isis-11; `## Shared Contracts (canonical)` "Route install vs redistribution" and the canonical TLV 135 / 236 entry layout
- **Two distinct paths (do not conflate):** FIB / kernel install is isis-9 (SPF routes -> Loc-RIB `locrib.Path` -> sysrib `OnChange` -> fibkernel); THIS spec is purely redistribution (IS-IS SPF out via `redistevents` to the orchestrator, and connected/static/BGP in via the `isis` `RedistConsumer` writing LSP TLVs). `redistevents` never installs to the FIB.
- `plan/spec-isis-9-spf-rib.md` -- dependency; SPF, LSP origination, and the Loc-RIB FIB-install path (NOT `redistevents`) must exist first
- `plan/spec-isis-6-lsdb.md` -- LSP origination hooks the consumer writes TLV 135 reachability into
- `plan/spec-isis-12-ipv6.md` -- IPv6 redistribution (TLV 236) builds on this spec's IPv4 path
