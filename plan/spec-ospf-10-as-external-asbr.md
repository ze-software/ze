# Spec: ospf-10-as-external-asbr

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-ospf-8-spf-rib.md |
| Phase | - |
| Updated | 2026-06-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-ospf-0-umbrella.md` `## Shared Contracts (canonical)` - "Route install vs redistribution", "Redistribution source", "LSA inventory" (Type 5), "Route preference / path types"; Architecture rows (`redistribute/`, `spf/`); the ospf-10 Metrics rows; the ospf-10 Child-Specs / Dependency rows
4. `docs/research/ospf-implementation-guide.md` §6d (AS-External route computation, lines 412-422), §6e (ABR/ASBR, lines 424-437), trap #7 (E1 vs E2 ordering, lines 1472-1474)
5. `plan/spec-ospf-8-spf-rib.md` - dependency: SPF, route table with path types, and the Loc-RIB FIB-install path (`locrib.Path`, AdminDistance 110) must already exist; this spec adds the external path-type computation and the redistribution wiring on top
6. `plan/spec-isis-11-redistribution.md` - the IS-IS sibling: the `redistevents` producer + `configredist` source/consumer registration pattern OSPF copies verbatim (single source name, `sync.Once mustRegister`, consumer at `OnStarted`)
7. `internal/core/redistevents/events.go` - `RouteChangeBatch`, `RegisterProtocol`, `RegisterProducer`, `Producers()`, `ProtocolName`
8. `internal/component/config/redistribute/{registry.go,consumer.go}` - `RouteSource` / `RegisterSource`; `RedistConsumer` interface / `RegisterConsumer` / `RouteEntry`

## Task

Make a Ze OSPF node an **ASBR** (AS Boundary Router) and wire it into Ze's
protocol-agnostic redistribution framework in **both directions**, exactly as
IS-IS did (ospf is the OSPF analogue of isis-11). Three concerns, all rooted in
RFC 2328 §16.4 and §12.4.4:

- **ASBR status + Type 5 origination.** When OSPF is configured to redistribute
  external routes (or `default-information originate`), the node becomes an
  ASBR: it sets the **E-bit** in its own Router-LSA (so the rest of the AS can
  build a path to it via Type 4 summaries, ospf-9) and maintains the
  `ze_ospf_asbr` gauge. For every redistributed prefix it originates a **Type 5
  AS-External-LSA** carrying Network Mask, the E-bit metric-type flag (E1 or
  E2), a 24-bit metric, a Forwarding Address, and an External Route Tag, using
  the canonical AS-External-LSA body layout from the umbrella Shared Contracts
  "LSA header + body layout". Type 5 LSAs are flooded **AS-wide** but **NOT**
  into stub or NSSA areas (the stub/NSSA filter and the NSSA Type 7 path are
  owned by ospf-11; this spec coordinates with it but does not implement the
  filter logic). When a source route disappears the Type 5 is withdrawn by
  MaxAge-purge (re-originate at LS Age = MaxAge 3600, flood, then drop), the
  umbrella ospf-7 purge mechanism.

- **AS-External route computation (RFC 2328 §16.4).** Compute external routes
  from Type 5 LSAs (Type 7 after ospf-11 translation is out of scope here):
  resolve the originating ASBR via the intra/inter-area route table (skip if
  unreachable); for **E1** (metric type 1, E-bit = 0) cost =
  distance-to-ASBR + advertised metric; for **E2** (metric type 2, E-bit = 1)
  cost = advertised metric only, tie-broken by the distance to the forwarding
  address. **E1 is always preferred over E2 regardless of metric** (trap #7);
  externals rank below intra-area and inter-area (umbrella "Route preference /
  path types"). Forwarding-address resolution: FA `0.0.0.0` means forward via
  the ASBR itself; a non-zero FA must itself be reachable via an intra/inter-area
  route, else the LSA is skipped. The winning external route is published as a
  single `locrib.Path` per prefix (AdminDistance 110), the OSPF-internal
  preference already resolved (umbrella contract; the FIB-install mechanism
  itself is ospf-8, reused unchanged).

- **`default-information originate`.** Conditionally (a default exists in the
  RIB) or `always`, originate a Type 5 default (`0.0.0.0/0`) with the configured
  metric/metric-type, also making the node an ASBR.

- **Redistribution wiring (the SEPARATE path, NOT FIB install).** Register OSPF
  as a SINGLE redistribution **source** named `ospf` (producer side: OSPF SPF
  routes out to BGP) and as a **consumer** named `ospf` (connected / static /
  BGP routes in, becoming Type 5 AS-External LSAs). This is the
  `redistevents` + `configredist` framework, identical in shape to isis-11; it
  NEVER installs to the kernel (that is ospf-8's Loc-RIB path).

Package: `internal/component/ospf/redistribute/` (producer + source + consumer)
and `internal/component/ospf/spf/` (the §16.4 external computation, the
`ase`/external stage of SPF, extending the ospf-8 SPF). Depends on ospf-8 (SPF,
the route table with path types, and the Loc-RIB install path must exist; this
spec adds the external path type and the redistribution read/write paths).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `plan/spec-ospf-0-umbrella.md` `## Shared Contracts (canonical)` - the binding contracts
  -> Constraint: "Route install vs redistribution" -- THIS spec owns the `redistevents` producer + the `configredist` source/consumer (redistribution path); it does NOT own FIB install (ospf-8 owns `locrib.Path` -> sysrib -> fibkernel). `redistevents` never installs to the kernel
  -> Constraint: "Redistribution source" -- register exactly ONE source/protocol named `ospf` (`redistevents.RegisterProtocol("ospf")` + `RegisterProducer`, plus `configredist.RegisterSource(RouteSource{Name: "ospf", Protocol: "ospf", ...})` in a `sync.Once mustRegister`); emit `RouteChangeBatch{Protocol = the ospf ProtocolID}`. No per-area / per-path-type source names; the orchestrator derives the source from `ProtocolName(b.Protocol)`; the generic loop-prevention check (`route.Origin == importingProtocol`) auto-rejects OSPF self-import. A single `ospf` source matches the single admin distance (110)
  -> Constraint: "LSA inventory" -- Type 5 AS-External-LSA: LS ID = external network address, Adv. Router = ASBR self, Scope = AS (not flooded into stub/NSSA), originated by ospf-10
  -> Constraint: "Route preference / path types" -- intra > inter > E1 > E2; E1 cost = path-to-ASBR + advertised metric, E2 cost = advertised metric only (tie-broken by path-to-forwarding-address). OSPF resolves this INTERNALLY and publishes one winning `locrib.Path` per prefix with AdminDistance 110 regardless of path type
- [ ] `docs/research/ospf-implementation-guide.md` §6d / §6e / trap #7 - external computation, ASBR behaviour, E1/E2 ordering
  -> Decision: follow the §16.4 algorithm: resolve ASBR vertex, compute E1/E2 cost by metric type, redirect to a non-zero forwarding address (re-resolved through the route table), install path-type external-1/external-2 with the route tag
  -> Constraint: trap #7 -- E1 always wins over E2 regardless of metric; a comparison using metric as primary key and type as tiebreaker is WRONG. Path type is the primary key, cost the secondary
  -> Constraint: §6e -- an ASBR sets the E flag in its Router-LSA; ABR and ASBR flags are independent; `default-information originate` originates a Type 5 for `0.0.0.0/0`
- [ ] `plan/spec-isis-11-redistribution.md` - the sibling redistribution pattern (verbatim mirror)
  -> Decision: producer wiring has four mandatory parts (`RegisterProtocol`, `RegisterProducer`, typed `events.Register[*RouteChangeBatch]` handle, EMIT on SPF change); single source name; consumer at `OnStarted`; source via `sync.Once`
  -> Constraint: `RegisterProducer` is mandatory -- registering only the config `RouteSource` does NOT put OSPF in `redistevents.Producers()`, so the orchestrator never subscribes and no OSPF route reaches BGP
- [ ] `plan/spec-ospf-8-spf-rib.md` - dependency (SPF, route table with path types, Loc-RIB install)
  -> Constraint: the external stage runs AFTER intra-area (§16.1) and inter-area (§16.2/§16.3, ospf-9) so the ASBR and forwarding-address lookups resolve against an already-built route table; reuse the ospf-8 `locrib.Path` install seam unchanged (no second FIB path)
- [ ] `ai/rules/plugin-self-containment.md`, `ai/rules/registration-dispatch.md` - self-contained component, registration not switch
  -> Constraint: all OSPF redistribution code lives under `internal/component/ospf/redistribute/`; no `ospf` spelling appears in the generic `config/redistribute` or `core/redistevents` packages
- [ ] `ai/rules/config-surface.md`, `ai/rules/config-naming.md`, `ai/patterns/config-option.md` - YANG vs env, kebab-case
  -> Constraint: `redistribute` wiring and `default-information originate` are YANG leaves in `ze-ospf-conf.yang` (schema owner ospf-4), kebab-case; the `redistribute` source/destination name is `ospf`

### RFC Summaries (MUST for protocol work; created via `/ze-rfc` at implementation time)
- [ ] `rfc/short/rfc2328.md` - OSPF v2: §12.4.4 (AS-External-LSA origination), §16.4 (external route computation), §16.4.1 (E1/E2 examples), §3.6 (Type 5 not flooded into stub)
  -> Constraint: §16.4 -- E1 cost = X + Y (distance to ASBR/FA + advertised metric), E2 cost = advertised metric, E2 tie-break by the cost to the forwarding address; type-1 always preferred over type-2; both rank below internal (intra/inter) routes
  -> Constraint: §12.4.4 -- Forwarding Address `0.0.0.0` directs traffic to the advertising ASBR; the E-bit (high bit of the metric field) selects metric type; the External Route Tag is opaque to OSPF and carried unchanged
- [ ] `rfc/short/rfc9129.md` - OSPF YANG model (the redistribute / default-information leaf shapes; schema owned by ospf-4)

**Key insights:** (minimal context to resume after compaction)
- Two distinct paths: FIB install is ospf-8 (`locrib.Path` -> sysrib -> fibkernel); THIS spec is purely (a) Type 5 origination + §16.4 external computation and (b) redistribution wiring via `redistevents` (out) and `configredist` consumer (in). `redistevents` never touches the kernel.
- Single source/consumer name `ospf` (no per-area/per-path-type names) -- this is what makes self-import auto-rejection work and matches the single admin distance 110.
- Path type is the PRIMARY external key: E1 always beats E2 (trap #7); externals always rank below intra/inter; OSPF resolves preference internally and publishes one `locrib.Path` per prefix.
- ASBR = E-bit set in own Router-LSA + `ze_ospf_asbr` gauge; the Type 5 store is AS-wide (umbrella), withdrawn via MaxAge-purge when the source disappears. Stub/NSSA Type 5 suppression and Type 7 are ospf-11's job (coordinate, do not implement here).
- Owns exactly four metrics: `ze_ospf_asbr`, `ze_ospf_external_lsas`, `ze_ospf_redist_injected_total{source}`, `ze_ospf_redist_withdrawn_total{source}`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write -> Constraint: annotations instead. -->
- [ ] `internal/core/redistevents/events.go` - `RouteChangeBatch`, `RegisterProtocol`, `RegisterProducer`, `Producers()`, `ProtocolName`; value-typed pooled payload; the producer registry the orchestrator subscribes to
  -> Constraint: the OSPF events package must call `RegisterProtocol("ospf")` then `RegisterProducer(id)` so OSPF appears in `Producers()`; emit `RouteChangeBatch` with `Protocol` = the allocated ProtocolID. The payload has no path-type/area field, so a single `ospf` stream carries all SPF route changes
- [ ] `internal/component/config/redistribute/registry.go` - `RouteSource{Name, Protocol, Description}`; `RegisterSource` (error return; re-register same name+protocol is a no-op, different protocol is an error); `SourceNames` / `LookupSource` feed the `redistribute-source` YANG validator + completion
  -> Constraint: register a SINGLE `RouteSource{Name: "ospf", Protocol: "ospf", Description: "OSPF SPF routes"}` in a `sync.Once mustRegister` that logs on error, exactly like BGP `RegisterBGPSources` / IS-IS `RegisterISISSources`
- [ ] `internal/component/config/redistribute/consumer.go` - `RedistConsumer` (`Name() string`, `InjectRoute(ctx, family.Family, RouteEntry)`, `WithdrawRoute(ctx, family.Family, prefix string)`); `RegisterConsumer` (one per protocol, re-register rejected); `RouteEntry{Prefix, NextHop}` value-typed
  -> Constraint: implement and register a `RedistConsumer` named `ospf` at `OnStarted`; `InjectRoute`/`WithdrawRoute` MUST log on failure (never `_, _, _ =`); idempotent re-register across SDK reconnect (use the `ReregisterConsumer` pattern from isis-11 if a plain register conflicts)
- [ ] `internal/component/isis/redistribute/{events/events.go,source.go,consumer.go}` - the sibling implementation OSPF mirrors (single source, `sync.Once`, producer emit on SPF change, consumer translates `RouteEntry` to a reachability advertisement)
  -> Constraint: the OSPF consumer translates `RouteEntry` into a **Type 5 AS-External-LSA** (not an IS-IS TLV); the producer emit reads the ospf-8 SPF output
- [ ] `internal/component/ospf/spf/` (ospf-8 output) - the route table with path types and the `locrib.Path` install seam
  -> Constraint: the §16.4 external stage runs after intra/inter; reuse the existing install seam; do NOT add a second FIB path
- [ ] `internal/component/config/redistribute/yang/ze-redistribute-conf.yang` - `destination` is a free-form `list` keyed by `protocol` (runtime-validated, no validator on the key); `import.source` carries `ze:validate "redistribute-source"`
  -> Constraint: no generic-redistribute YANG change is needed for `destination ospf`; `ospf` becomes a valid source purely by `RegisterSource`. The `default-information originate` and `redistribute` ENROLMENT leaves live under the `ospf` container (`ze-ospf-conf.yang`, schema owner ospf-4)

**Behavior to preserve:**
- BGP, LDP, RSVP-TE, IS-IS, static, connected redistribution sources and the BGP consumer remain independent and functional; `redistribute { destination bgp { import ... } }` semantics unchanged
- Loop-prevention in the evaluator (`route.Origin == importingProtocol`) unchanged; OSPF as a new source/consumer name `ospf` participates the same way
- The ospf-8 SPF intra/inter route table and the Loc-RIB install path (`locrib.Path` -> sysrib -> fibkernel) are unchanged; this spec adds the external path type on top and touches neither the FIB nor a `redistevents`-to-kernel path (there is none)
- The umbrella metrics contract: this spec registers ONLY its four owned series; no other ospf metric is touched here

**Behavior to change:**
- One new redistribution source `ospf` (Protocol `ospf`) and one new consumer `ospf`
- OSPF SPF gains an AS-External (§16.4) computation stage producing external-1/external-2 routes
- OSPF Router-LSA gains the E-bit when the node is an ASBR; a new AS-wide Type 5 store; `default-information originate`
- The `ospf` config container gains `redistribute` enrolment and `default-information originate` leaves (schema owner ospf-4; this spec consumes them)

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- **Consumer (in):** connected / static / BGP routes arrive at `RedistConsumer.InjectRoute` / `WithdrawRoute` (called by the redistribute orchestrator) when `redistribute { destination ospf { import <source> } }` is configured.
- **Producer (out):** OSPF SPF route changes (from ospf-8) become available to the framework under the single source `ospf`; `redistribute { destination bgp { import ospf } }` causes the BGP consumer to advertise them.
- **External computation (in-protocol):** received Type 5 AS-External-LSAs (from peer ASBRs, flooded AS-wide by ospf-7) enter the §16.4 external stage of SPF.
- **Default origination:** `default-information originate [always]` config (and, when conditional, presence of a default in the RIB) triggers a Type 5 for `0.0.0.0/0`.

### Transformation Path
1. **Source + producer registration:** OSPF init calls `RegisterSource(RouteSource{Name: "ospf", Protocol: "ospf", Description: ...})` (`sync.Once mustRegister`) so the name reaches the `redistribute-source` validator; the OSPF events package registers the `redistevents` PRODUCER (`RegisterProtocol("ospf")` -> ProtocolID, `RegisterProducer(id)` -> in `Producers()`, `events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)` typed handle). Source registration alone is insufficient: the orchestrator subscribes only to `Producers()`.
2. **Consumer inject (in -> Type 5):** orchestrator calls `ospfConsumer.InjectRoute(ctx, fam, RouteEntry)` -> the node becomes an ASBR (E-bit set in Router-LSA, re-originated; `ze_ospf_asbr` = 1) -> originate a Type 5 AS-External-LSA (network mask from the prefix, E-bit/metric-type and 24-bit metric from config defaults, forwarding address, route tag) into the AS-wide store -> ospf-7 floods it AS-wide (suppressed in stub/NSSA by ospf-11) -> `ze_ospf_external_lsas` and `ze_ospf_redist_injected_total{source}` bump.
3. **Consumer withdraw (in):** `ospfConsumer.WithdrawRoute(ctx, fam, prefix)` -> MaxAge-purge the Type 5 (re-originate at LS Age 3600, flood, drop) -> `ze_ospf_redist_withdrawn_total{source}` bumps; when the last external is gone, clear the E-bit and `ze_ospf_asbr` = 0.
4. **External SPF computation (§16.4):** on an LSDB/route-table change, for each Type 5 (and translated Type 7, ospf-11): resolve the ASBR via the intra/inter route table (skip if unreachable); compute E1 (X = path-to-ASBR + metric) or E2 (metric only, FA-distance tiebreak); redirect to a non-zero forwarding address (re-resolved through the route table); pick the winner by path type then cost (E1 > E2 > nothing); publish one `locrib.Path` per prefix (AdminDistance 110, path-type resolved internally) via the ospf-8 install seam.
5. **Producer emit (out):** on SPF route change, OSPF EMITS `redistevents.RouteChangeBatch{Protocol = ospf ProtocolID}` on the typed handle to the orchestrator (`redistribute_egress/redistribute.go`), which resolves the source as `ProtocolName(Protocol)` = `ospf` and dispatches to `BGPConsumer.InjectRoute` -> text `update-route` -> BGP RIB. ActionRemove withdraws.
6. **Default origination:** `default-information originate` -> a Type 5 for `0.0.0.0/0` (conditional on a RIB default unless `always`) in the AS-wide store; withdrawn via MaxAge when the condition lapses.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| OSPF engine <-> redistribute source registry | `RegisterSource` value type | [ ] (`TestOSPFRegisterSource`) |
| OSPF events <-> redistevents producer registry | `RegisterProtocol` + `RegisterProducer`; `Producers()` | [ ] (`TestOSPFProducerRegistered`) |
| redistribute orchestrator <-> OSPF consumer | `RedistConsumer.InjectRoute/WithdrawRoute`, value-typed `RouteEntry` | [ ] (`TestOSPFRedistConsumer*`) |
| OSPF engine <-> BGP | source registry (producer) + existing `BGPConsumer` | [ ] unit (`TestOSPFRedistSourceToBGP`); live RIB via `ospf-redist-frr` (Linux/QEMU) |
| Injected route <-> Type 5 LSA | new AS-External-LSA in the AS-wide store, triggers flooding | [ ] (`TestOSPFRedistConsumerConnected`) |
| External LSA <-> route table | §16.4 external stage publishes `locrib.Path` (path type resolved internally) | [ ] (`TestOSPFExternalE1PreferredOverE2`, `TestOSPFExternalForwardingAddress`) |
| Config tree <-> consumer / default-origination | `destination ospf { import <source> }`, `default-information originate` parsed, dispatched at runtime | [ ] (`test/ospf/ospf-redist-bgp.ci`) |

### Integration Points
- New package `internal/component/ospf/redistribute/` (`events/events.go`, `source.go`, `consumer.go`)
- `RegisterSource` (registry.go), `RegisterProducer` (redistevents), `RegisterConsumer` (consumer.go) in the generic frameworks (registration only; no generic-package OSPF spelling)
- OSPF Type 5 origination + AS-wide store (this spec) feeding ospf-7 flooding
- OSPF SPF external stage in `internal/component/ospf/spf/` extending the ospf-8 SPF and reusing its `locrib.Path` install seam
- OSPF Router-LSA E-bit origination (ospf-7 origination hook) and `default-information originate`
- `redistribute` YANG validator / completion (source names) -- no generic schema change, registry-driven; `ospf` container leaves owned by ospf-4

### Architectural Verification
- [ ] No bypassed layers (inject -> Type 5 origination -> ospf-7 flooding -> peer SPF; external computation -> `locrib.Path` via the ospf-8 seam; not a direct route push, not a second FIB path)
- [ ] No unintended coupling (no `ospf` spelling in generic `config/redistribute` or `core/redistevents`; OSPF independent of IS-IS; stub/NSSA filtering left to ospf-11)
- [ ] No duplicated functionality (reuses source/consumer registries, the BGP consumer, and the ospf-8 install seam; does not re-implement redistribution or FIB install)
- [ ] Zero-copy / value-typed preserved (`RouteEntry`/`RouteSource` value types; no pointers cross the boundary; Type 5 stored as raw bytes + metadata per the umbrella lazy-LSDB contract)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The generic `redistribute` framework accepts `ospf` as a destination with no generic YANG change (free-form list key, runtime-validated), and `ospf` becomes a valid source purely by `RegisterSource` | `ze-redistribute-conf.yang` (no validator on `protocol`); isis-11 confirmed A-1/A-2 | Need a validator/schema edit | parse `destination ospf { import ... }` + `import ospf` in a unit/functional test | unvalidated |
| A-2 | `RouteEntry` (string `Prefix`/`NextHop`, no metric) is sufficient to originate a Type 5; the consumer applies a FIXED default metric + metric type (code constants / `ospf` container defaults, not per-route) | `consumer.go` `RouteEntry` has no Metric; isis-11 A-3 precedent | Need a per-route metric: extend generic `RouteEntry` or add a YANG leaf (future) | consumer test asserting the Type 5 uses the default metric/metric-type | unvalidated |
| A-3 | A single `ospf` producer exporting all SPF routes is sufficient for "mesh with BGP" (no per-area / per-path-type redistribution selector in v1) | `RouteChangeBatch` has no area/path-type field; orchestrator derives source from `ProtocolName` only; isis-11 A-4 | Per-area/path-type selection needs a payload field (future) | producer test: intra + inter routes both reach BGP via `import ospf` | unvalidated |
| A-4 | Registering the consumer at `OnStarted` (idempotent across SDK reconnect) is early enough for the orchestrator to dispatch | isis-11 A-5 (`ReregisterConsumer`); `redistribute_egress/register.go` | Re-order to a different SDK hook | functional test: configured import reaches the consumer | unvalidated |
| A-5 | The §16.4 external stage can run after the ospf-8 intra/inter route table is built and reuse its `locrib.Path` install seam unchanged | ospf-8 dependency; guide §6d ("runs after Phase 1+2") | The install seam needs a path-type-aware extension | external-route install test against a multi-LSA LSDB | unvalidated |
| A-6 | The 24-bit metric field of the AS-External-LSA never overflows for redistributed metrics (default and configured) | umbrella body layout (3-byte metric); RFC 2328 §12.4.4 | Need clamping/validation at 0xFFFFFF | boundary test on the metric field | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | E1/E2 ordering implemented with metric as primary key (trap #7) -> E2 wrongly beats E1 | a low-metric E2 route installed over a higher-cost E1 | Path type is the PRIMARY comparison key; explicit `TestOSPFExternalE1PreferredOverE2` with a low-cost E2 and a high-cost E1 |
| R-2 | Non-zero forwarding address not re-resolved through the route table -> black-holed externals | traffic to an external prefix dropped though the FA is reachable elsewhere | §12.4.4 FA rule: FA 0.0.0.0 -> via ASBR; non-zero FA must be intra/inter-reachable else skip the LSA; `TestOSPFExternalForwardingAddress` |
| R-3 | Type 5 leaked into a stub/NSSA area (umbrella forbids) | adjacency/flooding mismatch with a stub peer | Coordinate with ospf-11: this spec floods AS-wide via ospf-7, which applies the stub/NSSA scope filter; assert no Type 5 in a stub area's LSDB |
| R-4 | Source route disappears but the Type 5 is not withdrawn -> stale external advertised AS-wide | external route never withdrawn on peers | MaxAge-purge on `WithdrawRoute` (re-originate at 3600, flood, drop); `TestOSPFRedistConsumerWithdraw` |
| R-5 | Consumer registered but never invoked, or registering only the source (not the producer) | configured `import ospf` silently does nothing | `RegisterProducer` mandatory; wiring tests assert `Producers()` contains OSPF AND a connected route reaches a Type 5 end-to-end |
| R-6 | E-bit / `ze_ospf_asbr` not cleared when the last external is withdrawn -> node falsely advertised as ASBR | a Type 4 summary for a non-ASBR | Clear the E-bit and re-originate the Router-LSA when the external set empties; assert `ze_ospf_asbr` returns to 0 |
| R-7 | Self-import (`destination ospf { import ospf }`) re-imports OSPF routes -> loop | OSPF external for an OSPF-learned prefix | Single source/consumer name `ospf` makes `route.Origin == importingProtocol` auto-reject; `TestOSPFRedistSelfImportRejected` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| OSPF events package imported (producer wiring) | -> | `RegisterProtocol("ospf")` + `RegisterProducer(id)` -> OSPF in `redistevents.Producers()` -> orchestrator subscribes | `TestOSPFProducerRegistered` (asserts `Producers()` contains the OSPF ProtocolID and `ProtocolIDOf("ospf")` resolves) |
| config `redistribute { destination bgp { import ospf } }`, an OSPF SPF route present | -> | source `ospf` registered AND OSPF emits `RouteChangeBatch` -> orchestrator -> `BGPConsumer.InjectRoute` -> BGP RIB | `TestOSPFRedistSourceToBGP` + `test/ospf/ospf-redist-bgp.ci` (OSPF route appears in BGP) |
| config `redistribute { destination ospf { import connected } }`, a connected prefix exists | -> | `ospfConsumer.InjectRoute` -> Type 5 AS-External-LSA in the AS-wide store -> ospf-7 flooding; node becomes ASBR (E-bit) | `TestOSPFRedistConsumerConnected` + `test/ospf/ospf-redist-bgp.ci` (connected route appears as a Type 5 LSA) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `redistribute { destination bgp { import ospf } }` with an OSPF SPF route present | The OSPF route is advertised by BGP (appears in the BGP RIB / UPDATE) via the single `ospf` source |
| AC-2 | `redistribute { destination bgp { import ospf } }` with both an intra-area and an inter-area OSPF route present | BOTH routes are advertised by BGP via the single `ospf` source (area/path-type is not a redistribution selector in v1; per-area selection is documented future work) |
| AC-3 | `redistribute { destination ospf { import connected } }` with a connected prefix | The prefix is originated as a Type 5 AS-External-LSA (network mask, default metric/metric-type, forwarding address, route tag), the node sets the Router-LSA E-bit, `ze_ospf_asbr` = 1, and the LSA is flooded AS-wide |
| AC-4 | `redistribute { destination ospf { import static } }` / `{ import bgp }` | The static / BGP prefix is originated as a Type 5 AS-External-LSA in the AS-wide store |
| AC-5 | An injected route is withdrawn (source removes it) | `WithdrawRoute` MaxAge-purges the Type 5 (re-originate at LS Age 3600, flood, drop); peers withdraw the external; `ze_ospf_redist_withdrawn_total{source}` bumps |
| AC-6 | The last external/redistributed route is withdrawn | The Router-LSA E-bit is cleared, the Router-LSA is re-originated, and `ze_ospf_asbr` returns to 0 |
| AC-7 | An OSPF SPF route imported into BGP is withdrawn by SPF | The route is withdrawn from BGP (an `ActionRemove` `RouteChangeBatch` propagates producer-side) |
| AC-8 | LSDB has a Type 5 E1 (metric type 1) and a Type 5 E2 (metric type 2) for the SAME prefix, with the E2 cost LOWER than the E1 cost | The E1 route is installed (path type external-1), NOT the lower-cost E2 (trap #7: E1 always preferred over E2 regardless of metric) |
| AC-9 | A Type 5 E1 LSA whose ASBR is reachable at cost X with advertised metric Y | The external route is installed with cost X + Y and path type external-1; if the ASBR is unreachable the LSA is skipped (no route) |
| AC-10 | A Type 5 E2 LSA, and separately a Type 5 with a non-zero forwarding address | E2 cost = advertised metric only (tie-broken by the cost to the FA); a non-zero forwarding address is used as the nexthop target and re-resolved through the route table; FA `0.0.0.0` forwards via the ASBR; an unreachable non-zero FA causes the LSA to be skipped |
| AC-11 | An external OSPF route and an intra/inter-area route exist for the same prefix | The intra/inter-area route is preferred (externals rank below internal); OSPF publishes one winning `locrib.Path` per prefix with AdminDistance 110 |
| AC-12 | `default-information originate` (conditional) with a default present in the RIB, and `default-information originate always` | A Type 5 for `0.0.0.0/0` is originated (conditionally only when a RIB default exists; `always` unconditionally); the node becomes an ASBR |
| AC-13 | `redistribute { destination ospf { import ospf } }` (self-import) | Loop-prevention rejects redistributing OSPF into OSPF (origin `ospf` == importing protocol `ospf`); the single source/consumer name makes this auto-rejection work |
| AC-14 | The OSPF events package is imported (producer wiring loaded) | `redistevents.Producers()` contains the OSPF ProtocolID and `ProtocolIDOf("ospf")` resolves; registering only the config `RouteSource` (without `RegisterProducer`) does NOT make OSPF appear in `Producers()` |
| AC-15 | A redistributed metric at and above the 24-bit field width | The AS-External-LSA metric field carries up to `0xFFFFFF`; values above are clamped/rejected (no silent overflow) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Redistributes OSPF into BGP | SPF route -> `ospf` source -> orchestrator -> `BGPConsumer.InjectRoute` -> reactor -> BGP RIB | `TestOSPFRedistSourceToBGP`, `test/ospf/ospf-redist-bgp.ci` |
| 2 | Redistributes connected into OSPF | connected prefix -> orchestrator -> `ospfConsumer.InjectRoute` -> Type 5 AS-External-LSA -> ospf-7 flooding -> peer SPF | `TestOSPFRedistConsumerConnected`, `test/ospf/ospf-redist-bgp.ci` |
| 3 | Redistributes static / BGP into OSPF | static/BGP prefix -> `ospfConsumer.InjectRoute` -> Type 5 in the AS-wide store | `TestOSPFRedistConsumerStatic`, `TestOSPFRedistConsumerBGP` |
| 4 | Receives a Type 5 from a peer ASBR | flooded Type 5 -> §16.4 external stage -> E1/E2 cost -> winning `locrib.Path` -> sysrib -> kernel | `TestOSPFExternalE1PreferredOverE2`, `TestOSPFExternalForwardingAddress`, `test/ospf/ospf-redist-bgp.ci` |
| 5 | Originates a default route | `default-information originate [always]` -> Type 5 for `0.0.0.0/0` -> ASBR | `TestOSPFDefaultInformationOriginate` |

## 🧪 TDD Test Plan

Every AC has at least one exact named test below (AC mapping in the rightmost column).

### Unit Tests
| Test | File | Validates | Covers AC | Status |
|------|------|-----------|-----------|--------|
| `TestOSPFProducerRegistered` | `internal/component/ospf/redistribute/events/events_test.go` | producer wiring: `RegisterProtocol("ospf")` + `RegisterProducer(id)` put OSPF in `redistevents.Producers()`; `ProtocolIDOf("ospf")` resolves; registering only the `RouteSource` does NOT add it to `Producers()` | AC-14 | |
| `TestOSPFRegisterSource` | `internal/component/ospf/redistribute/source_test.go` | single source `ospf` (Protocol `ospf`) registered; idempotent; `LookupSource` finds it; no per-area names | AC-1 | |
| `TestOSPFRedistSourceToBGP` | `internal/component/ospf/redistribute/source_test.go` | the `ospf` source emits a `RouteChangeBatch` (`Protocol` = the ospf ProtocolID) reaching the BGP consumer; BOTH an intra-area and an inter-area route are exported (single source) | AC-1, AC-2 | |
| `TestOSPFRedistSourceWithdrawToBGP` | `internal/component/ospf/redistribute/source_test.go` | SPF-removed route emits an `ActionRemove` `RouteChangeBatch`; route withdrawn from BGP | AC-7 | |
| `TestOSPFRedistConsumerConnected` | `internal/component/ospf/redistribute/consumer_test.go` | `InjectRoute` for `connected` originates a Type 5 AS-External-LSA with the default metric/metric-type; node becomes ASBR (E-bit, `ze_ospf_asbr`=1) | AC-3 | |
| `TestOSPFRedistConsumerStatic` | `internal/component/ospf/redistribute/consumer_test.go` | `InjectRoute` for `static` originates a Type 5 | AC-4 | |
| `TestOSPFRedistConsumerBGP` | `internal/component/ospf/redistribute/consumer_test.go` | `InjectRoute` for `bgp` originates a Type 5 | AC-4 | |
| `TestOSPFRedistConsumerWithdraw` | `internal/component/ospf/redistribute/consumer_test.go` | `WithdrawRoute` MaxAge-purges the Type 5 (LS Age 3600, flood, drop); `ze_ospf_redist_withdrawn_total{source}` bumps | AC-5 | |
| `TestOSPFRedistConsumerName` | `internal/component/ospf/redistribute/consumer_test.go` | `Name()` returns `ospf`; registered once | AC-3, AC-4 | |
| `TestOSPFASBRBitClearedOnEmpty` | `internal/component/ospf/redistribute/consumer_test.go` | withdrawing the last external clears the Router-LSA E-bit, re-originates the Router-LSA, and `ze_ospf_asbr` returns to 0 | AC-6 | |
| `TestOSPFRedistConsumerLogsFailure` | `internal/component/ospf/redistribute/consumer_test.go` | Type 5 origination failure is logged, not swallowed (regression guard) | AC-3..AC-5 | |
| `TestOSPFRedistSelfImportRejected` | `internal/component/ospf/redistribute/consumer_test.go` | `destination ospf { import ospf }` is a no-op (origin `ospf` == importing `ospf`) | AC-13 | |
| `TestOSPFExternalE1PreferredOverE2` | `internal/component/ospf/spf/external_test.go` | for the same prefix, a high-cost E1 wins over a low-cost E2 (trap #7: path type is the primary key) | AC-8 | |
| `TestOSPFExternalE1Cost` | `internal/component/ospf/spf/external_test.go` | E1 cost = distance-to-ASBR + advertised metric; unreachable ASBR -> LSA skipped (no route) | AC-9 | |
| `TestOSPFExternalE2Cost` | `internal/component/ospf/spf/external_test.go` | E2 cost = advertised metric only; tie-broken by the cost to the forwarding address | AC-10 | |
| `TestOSPFExternalForwardingAddress` | `internal/component/ospf/spf/external_test.go` | FA `0.0.0.0` -> nexthop via the ASBR; non-zero FA used as the nexthop target and re-resolved; unreachable non-zero FA -> LSA skipped | AC-10 | |
| `TestOSPFExternalBelowInternal` | `internal/component/ospf/spf/external_test.go` | for the same prefix, an intra/inter-area route is preferred over any external; one winning `locrib.Path` (AdminDistance 110) | AC-11 | |
| `TestOSPFDefaultInformationOriginate` | `internal/component/ospf/redistribute/consumer_test.go` | conditional: Type 5 `0.0.0.0/0` only when a RIB default exists; `always`: unconditional; node becomes ASBR | AC-12 | |
| `TestOSPFRedistRegistrationOrder` | `internal/component/ospf/redistribute/source_test.go` | registration-order tolerance: producer registered before AND after the consumer/orchestrator both deliver routes | AC-1 | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| AS-External-LSA metric (24-bit field) | 0..16777215 (`0xFFFFFF`) | 16777215 | N/A | clamp/reject above 16777215 (no silent overflow) |
| Metric type (E-bit) | 1 (E1) / 2 (E2) | 2 | N/A (only 1 or 2) | N/A |
| External Route Tag (32-bit, opaque) | 0..4294967295 | 4294967295 | N/A | N/A (full 32-bit, carried unchanged) |
| Forwarding Address | any IPv4; `0.0.0.0` = via ASBR | non-zero must be intra/inter-reachable | N/A | non-zero-but-unreachable -> LSA skipped |
| Source name | `ospf` only (single source) | n/a | unregistered name rejected | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ospf-redist-bgp` | `test/ospf/ospf-redist-bgp.ci` | config surface: `import ospf` into BGP and `destination ospf { import connected/static/bgp }` validate; `default-information originate` validates; self-import is a no-op (AC-1/AC-3/AC-4/AC-12/AC-13 config layer). Live route flow is unit-tested + the Linux-pending `ospf-redist-frr` scenario | |

### Interop Tests (MANDATORY for protocol features)
<!-- Type 5 origination and §16.4 computation are wire-affecting; interop verifies FRR accepts the externals and computes the same routes. -->
The FRR redistribution/external interop scenarios are MANDATORY and not
deferrable, but are OWNED by ospf-13 (umbrella "Test + interop wiring": FRR
`ospfd` interop including redistribution is ospf-13). This spec contributes the
redistribution scenario inputs; ospf-13 runs them under Linux/QEMU.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ospf-redist-frr` | `test/interop/scenarios/` (run by ospf-13) | FRR ospfd | FRR installs OSPF externals redistributed by Ze (connected/static/BGP -> Type 5, E1/E2 + forwarding address honoured) and Ze computes FRR-originated Type 5 routes identically | scenario inputs here; execution Linux/QEMU-pending, owned by ospf-13 |

### Future (if deferring any tests)
- Per-area / per-path-type redistribution selection (needs a `RouteChangeBatch` area/path-type field) -- documented future work, cross-reference only.
- NSSA Type 7 -> Type 5 translation route computation is owned by ospf-11 (this spec computes Type 5 externals only).

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files -->
- `internal/component/ospf/register.go` - call `RegisterConsumer` (idempotent / `Reregister`) at `OnStarted` and `RegisterSource(RouteSource{Name: "ospf", Protocol: "ospf", Description: ...})` (`sync.Once mustRegister`, at init for `ze config validate`); import the new `redistribute/events` package so its producer registration runs and OSPF appears in `redistevents.Producers()`
- `internal/component/ospf/spf/...` - add the AS-External (§16.4) computation stage (E1/E2 cost, forwarding-address resolution, path-type ordering) after intra/inter; publish the winning `locrib.Path` via the existing ospf-8 install seam (no second FIB path)
- `internal/component/ospf/lsdb/...` - Type 5 AS-External-LSA origination + the AS-wide store (umbrella: Type 5 lives AS-wide, not per-area); MaxAge-purge on withdraw; Router-LSA E-bit set/clear hook (ASBR status)
- `internal/component/config/redistribute/...` - NO source/consumer code change expected; only confirm a registration call site. Modify ONLY if a registration hook or validator wiring is missing
- `docs/guide/ospf.md`, `docs/guide/configuration.md`, `docs/comparison.md` - OSPF redistribution + `default-information originate` rows (also tracked in ospf-13)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new config) | Yes | `redistribute` enrolment + `default-information originate` leaves under the `ospf` container in `ze-ospf-conf.yang` (schema OWNED by ospf-4; this spec consumes them). `destination ospf` itself is a runtime-validated free-form list key (A-1) -- no generic-redistribute schema change |
| YANG validation constraints | Yes | `default-information` metric `range 0..16777215`, metric-type `enumeration {1,2}`, route-tag `range 0..4294967295` (native constraints; defined with the ospf-4 schema) |
| YANG custom validators | No | `redistribute-source` validator picks up `ospf` from the source registry (A-1); no new custom validator |
| CLI commands/flags | No | reuses `redistribute` config; `show ip ospf database` (external) + `show ip ospf border-routers` reflect Type 5 / ASBR (ospf-13) |
| CLI grammar (action before identifier) | No | n/a -- config block, not a verb |
| Editor autocomplete | No | `ospf` appears via the source registry `CompleteFn` |
| Functional test for new RPC/API | Yes | `test/ospf/ospf-redist-bgp.ci` |
| Pipe completeness | No | n/a -- no new show command in this spec |
| Doctor check for runtime dependencies | No | none new (no new socket/path/binary) |
| Prometheus counters/metrics | Yes | this spec OWNS and registers exactly `ze_ospf_asbr` (gauge), `ze_ospf_external_lsas` (gauge), `ze_ospf_redist_injected_total{source}` (counter), `ze_ospf_redist_withdrawn_total{source}` (counter) per the umbrella "Metrics (canonical)" table. Per-owner registration here; ospf-13 only scrapes/asserts |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (OSPF ASBR / external redistribution / `default-information originate`) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (`destination ospf`, `import ospf`, `default-information originate`) |
| 3 | CLI command added/changed? | No | external/ASBR views are owned by ospf-13 |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | OSPF is a component (ospf-4), not a plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/ospf.md` (redistribution + external routes section) |
| 7 | Wire format changed? | Yes | `docs/architecture/wire/ospf.md` (Type 5 AS-External-LSA body, E1/E2, forwarding address, route tag) |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc2328.md` (§12.4.4, §16.4) |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (`test/ospf/ospf-redist-bgp.ci`) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (OSPF<->BGP redistribution, external routes) |
| 12 | Internal architecture changed? | No | covered by ospf-0/ospf-8 rows |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` (the four ospf-10 counters/gauges) |
| 15 | Registered plugin/event/command/capability changed? | Yes | `docs/plugin-overview.md` (new redistribute source `ospf`, consumer `ospf`) |
| 16 | Any changed source file referenced by doc source anchors? | No | grep `docs/` at completion |
| 17 | Existing docs show examples for this area? | No | verify `redistribute` examples at completion |

## Files to Create
- `internal/component/ospf/redistribute/events/events.go` - the `redistevents` PRODUCER wiring (mirrors `internal/component/isis/redistribute/events/events.go`): `Namespace = "ospf"`, `ProtocolID = redistevents.RegisterProtocol(Namespace)`, `redistevents.RegisterProducer(ProtocolID)`, typed handle `RouteChange = events.Register[*redistevents.RouteChangeBatch](Namespace, redistevents.EventType)`. Without this file no OSPF route reaches the orchestrator
- `internal/component/ospf/redistribute/events/events_test.go` - `TestOSPFProducerRegistered`
- `internal/component/ospf/redistribute/source.go` - `RegisterSource(RouteSource{Name: "ospf", Protocol: "ospf", Description: ...})` (`sync.Once mustRegister`) + the producer read that EMITS `RouteChangeBatch` on the typed handle for ospf-8 SPF route changes
- `internal/component/ospf/redistribute/source_test.go` - `TestOSPFRegisterSource`, `TestOSPFRedistSourceToBGP`, `TestOSPFRedistSourceWithdrawToBGP`, `TestOSPFRedistRegistrationOrder`
- `internal/component/ospf/redistribute/consumer.go` - `RedistConsumer` impl (Name `ospf`, `InjectRoute`, `WithdrawRoute`) translating `RouteEntry` into a Type 5 AS-External-LSA; ASBR E-bit set/clear; MaxAge-purge on withdraw; `default-information originate`
- `internal/component/ospf/redistribute/consumer_test.go` - `TestOSPFRedistConsumerConnected/Static/BGP/Withdraw/Name/LogsFailure`, `TestOSPFASBRBitClearedOnEmpty`, `TestOSPFRedistSelfImportRejected`, `TestOSPFDefaultInformationOriginate`
- `internal/component/ospf/spf/external.go` - the §16.4 AS-External route computation (E1/E2 cost, forwarding-address resolution, path-type ordering), publishing the winning `locrib.Path` via the ospf-8 seam
- `internal/component/ospf/spf/external_test.go` - `TestOSPFExternalE1PreferredOverE2`, `TestOSPFExternalE1Cost`, `TestOSPFExternalE2Cost`, `TestOSPFExternalForwardingAddress`, `TestOSPFExternalBelowInternal`
- `test/ospf/ospf-redist-bgp.ci` - functional test: OSPF route into BGP and connected/static/BGP routes into Type 5 LSAs; `default-information originate`; self-import no-op

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + `spec-ospf-8-spf-rib.md` |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8-14. | Standard flow |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register the `redistevents` producer (`RegisterProtocol` + `RegisterProducer` + typed `RouteChange` handle), register the source `ospf`, register a stub consumer; write failing wiring tests
   - Tests: `TestOSPFProducerRegistered`, `TestOSPFRegisterSource`, `TestOSPFRedistConsumerName`
   - Files: `redistribute/events/events.go`, `redistribute/source.go`, `redistribute/consumer.go` (Name + stub Inject/Withdraw), `register.go` (import events; `RegisterConsumer` at `OnStarted`; source at init)
   - Verify: OSPF in `redistevents.Producers()` and `ProtocolIDOf("ospf")` resolves; source `ospf` in `SourceNames`; consumer in `ConsumerNames`; wiring tests fail because Inject/Withdraw are stubs
2. **Phase: Consumer inject/withdraw -> Type 5 + ASBR** -- translate `RouteEntry` into a Type 5 AS-External-LSA in the AS-wide store; set the Router-LSA E-bit (ASBR), `ze_ospf_asbr`; MaxAge-purge on withdraw; clear the E-bit when empty
   - Tests: `TestOSPFRedistConsumerConnected/Static/BGP/Withdraw`, `TestOSPFASBRBitClearedOnEmpty`, `TestOSPFRedistConsumerLogsFailure`
   - Files: `consumer.go`, `lsdb/` (Type 5 origination + AS-wide store + Router-LSA E-bit hook)
   - Verify: injected prefix -> Type 5 with default metric/metric-type; withdraw MaxAge-purges; E-bit clears on empty; failures logged
3. **Phase: AS-External route computation (§16.4)** -- E1/E2 cost, forwarding-address resolution, path-type ordering (E1 > E2 > none, below internal); publish one `locrib.Path` per prefix via the ospf-8 seam
   - Tests: `TestOSPFExternalE1PreferredOverE2`, `TestOSPFExternalE1Cost`, `TestOSPFExternalE2Cost`, `TestOSPFExternalForwardingAddress`, `TestOSPFExternalBelowInternal`
   - Files: `spf/external.go`
   - Verify: E1 beats E2 regardless of metric (trap #7); FA rules honoured; unreachable ASBR/FA skipped; external below internal
4. **Phase: default-information originate** -- conditional + `always` Type 5 for `0.0.0.0/0`; ASBR status
   - Tests: `TestOSPFDefaultInformationOriginate`
   - Files: `consumer.go` (or a small `default.go`), `lsdb/`
   - Verify: conditional default only with a RIB default; `always` unconditional
5. **Phase: Producer read -> BGP + loop prevention** -- EMIT `RouteChangeBatch` (`Protocol` = ospf ProtocolID) on SPF change; reject self-import; registration-order tolerance
   - Tests: `TestOSPFRedistSourceToBGP`, `TestOSPFRedistSourceWithdrawToBGP`, `TestOSPFRedistSelfImportRejected`, `TestOSPFRedistRegistrationOrder`
   - Files: `source.go`, `events/events.go`, SPF read path (ospf-8 output)
   - Verify: intra + inter routes reach `BGPConsumer.InjectRoute`; ActionRemove withdraws; `destination ospf { import ospf }` is a no-op; producer registered before/after both deliver
6. **Functional tests** -- `test/ospf/ospf-redist-bgp.ci` (both directions, default-origination, self-import no-op)
7. **RFC refs** -- add `// RFC 2328 Section 16.4` / `Section 12.4.4` comments above the external computation and Type 5 origination
8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- audit tables, learned summary, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Feature completeness | Both redistribution directions work end-to-end; §16.4 external computation matches RFC; parity with the IS-IS redistribution sibling and with FRR external handling |
| Correctness | E1 always beats E2 (path type primary key, trap #7); E1 cost = X+Y, E2 cost = metric only (FA-distance tiebreak); FA 0.0.0.0 -> via ASBR, non-zero FA re-resolved/skip-if-unreachable; externals below internal; metric within 24 bits |
| Naming | single source/consumer name `ospf`; config kebab-case; no per-area names |
| Data flow | inject -> Type 5 -> ospf-7 flooding -> peer SPF; external computation -> `locrib.Path` via the ospf-8 seam; producer reads SPF and emits one `ospf` batch stream; no direct route push; no `ospf` spelling in generic redistribute/redistevents; no second FIB path |
| CLI grammar | n/a (config block) |
| Doctor checks | none new |
| YANG validation | `default-information` metric/metric-type/route-tag have native constraints; `ospf` validates via the source registry |
| Prometheus counters | exactly the four owned series defined and registered here (none from other owners) |
| Rule: plugin-self-containment | all OSPF redistribution code under `internal/component/ospf/redistribute/` (+ the SPF external stage under `spf/`) |
| Rule: no silent error discard | Inject/Withdraw and Type 5 origination log every failure |
| Rule: stub/NSSA boundary | this spec floods Type 5 AS-wide via ospf-7; the stub/NSSA scope filter and Type 7 stay with ospf-11 (no scope-filter code here) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| OSPF redistevents producer registered | `grep -rn 'RegisterProtocol\|RegisterProducer' internal/component/ospf/redistribute/events/`; `TestOSPFProducerRegistered` asserts OSPF in `redistevents.Producers()` |
| OSPF redistribute source registered | `grep -r 'RegisterSource' internal/component/ospf/redistribute/`; source in `SourceNames` test |
| OSPF redistribute consumer registered | `grep -r 'RegisterConsumer' internal/component/ospf/`; consumer in `ConsumerNames` test |
| Type 5 origination + §16.4 external computation | `ls internal/component/ospf/spf/external.go`; `go test ./internal/component/ospf/spf/ -run External` PASS |
| Source + consumer files | `ls internal/component/ospf/redistribute/{source,consumer}.go` |
| Owned metrics registered | `grep -rn 'ze_ospf_asbr\|ze_ospf_external_lsas\|ze_ospf_redist_injected_total\|ze_ospf_redist_withdrawn_total' internal/component/ospf/` |
| Functional test | `ls test/ospf/ospf-redist-bgp.ci` |
| Import/export paths proven | run `ospf-redist-bgp.ci`; assert OSPF route in BGP and connected/static/BGP routes as Type 5 LSAs |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | `RouteEntry.Prefix`/`NextHop` parsed/validated before forming a Type 5; metric clamped to 24 bits; reject malformed prefixes/forwarding addresses |
| Loop prevention | self-import rejected (origin `ospf` == importing `ospf`); externals never re-redistributed into OSPF |
| Resource exhaustion | bound Type 5 origination rate (debounce re-origination); cap the AS-external store growth under source flap |
| Error leakage | log inject/withdraw/origination failures at warn without leaking secrets |
| Spoofing / scope | Type 5 must NOT leak into stub/NSSA areas (ospf-11 scope filter); a non-zero forwarding address must be validated reachable before use to avoid black-holing |

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
- Like isis-11, "redistribution into OSPF" is a consumer that ORIGINATES an LSA (Type 5 instead of an IS-IS TLV) and "redistribution out of OSPF" is one named source (`ospf`) emitting SPF route changes; the framework does all dispatch and loop prevention. The OSPF-specific novelty is the §16.4 external computation (E1/E2 ordering + forwarding-address resolution) and the ASBR E-bit lifecycle.

## Core Insight
<!-- Optional: the single most important design revelation from this work. -->
The hard part is not plumbing (the source/consumer registries are protocol-agnostic
and already ship) but two RFC-correctness points concentrated here: path type is the
PRIMARY external comparison key (E1 always beats E2 regardless of metric, trap #7),
and the forwarding address governs the real nexthop (0.0.0.0 = via the ASBR, non-zero
must be independently reachable). Getting these right is the whole spec; the
redistribution wiring is a verbatim mirror of isis-11.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Single `ospf` source/consumer | Per-area or per-path-type sources (`ospf-intra`/`ospf-e1`) | `RouteChangeBatch` has no area/path-type field; the orchestrator derives the source from `ProtocolName(Protocol)`, and loop-prevention matches `route.Origin == importingProtocol`. Multiple IDs would break self-import auto-rejection (AC-13) and the single admin distance. Single `ospf` is the only cleanly-implementable choice; per-area is future work needing a payload field |
| Consumer originates a Type 5 into the AS-wide store | Direct SPF/RIB injection | Reuses ospf-7 flooding + the §16.4 read path; peers learn via normal OSPF, not a side channel; matches RFC 2328 §12.4.4 |
| Path type is the primary external comparison key | metric primary, type as tiebreaker | RFC 2328 §16.4 / trap #7: E1 always wins over E2 regardless of metric because E1 includes the internal cost to the ASBR; metric-first is a known bug |
| §16.4 stage reuses the ospf-8 `locrib.Path` install seam | a separate external FIB path | One FIB path total (umbrella "Route install vs redistribution"); OSPF resolves intra/inter/E1/E2 preference internally and publishes one Path per prefix |
| Stub/NSSA scope filter + Type 7 left to ospf-11 | filter here | Umbrella assigns stub/NSSA suppression and Type 7 to ospf-11; this spec floods Type 5 AS-wide via ospf-7 and coordinates, avoiding duplicated scope logic |

## Known Limitations
- Single redistribution source `ospf` (no per-area / per-path-type selection). All SPF routes are exported under `ospf`; per-area selection is future work needing an area/path-type field on `redistevents.RouteChangeBatch`.
- `RouteEntry` carries no metric, so the OSPF consumer applies a FIXED default external metric + metric type (code constants / `ospf` container defaults, NOT per-route). A per-route metric or route-map is future work.
- Route-map / policy-based redistribution filtering beyond source selection is out of scope (the existing flat evaluator governs source/family selection only).
- NSSA Type 7 origination and Type 7 -> Type 5 translation route computation are owned by ospf-11; this spec computes Type 5 externals only. The stub/NSSA Type 5 scope filter lives in ospf-11/ospf-7.
- Per-path-type admin distance vs other protocols (e.g. external OSPF vs eBGP) is not modelled: `locrib.Path` has no path-type field, so all OSPF routes carry AdminDistance 110 (umbrella contract); per-path-type distance is future work needing a `locrib.Path` protoType field.

## RFC Documentation

Add `// RFC 2328 Section 16.4: "<quoted requirement>"` above the external
computation (E1/E2 cost, E1-preferred-over-E2, forwarding-address resolution),
and `// RFC 2328 Section 12.4.4` above Type 5 AS-External-LSA origination.
MUST document: E1 cost = path-to-ASBR + advertised metric; E2 cost = advertised
metric only (FA-distance tiebreak); E1 always preferred over E2; externals rank
below intra/inter; FA 0.0.0.0 = via ASBR, non-zero FA must be intra/inter-reachable;
the E-bit metric-type encoding; the 24-bit metric range; the opaque route tag.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, with source anchors named, or "None" with grep evidence]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| OSPF routes into BGP | unit + functional config test | `TestOSPFRedistSourceToBGP`, `test/ospf/ospf-redist-bgp.ci`; live RIB via `ospf-redist-frr` (ospf-13, Linux/QEMU) |
| connected/static/BGP into OSPF as Type 5 | unit + functional config test | `TestOSPFRedistConsumerConnected/Static/BGP`, `test/ospf/ospf-redist-bgp.ci` |
| RFC 2328 §16.4 external computation (E1/E2, forwarding address) | unit test | `TestOSPFExternalE1PreferredOverE2`, `TestOSPFExternalE1Cost`, `TestOSPFExternalE2Cost`, `TestOSPFExternalForwardingAddress`, `TestOSPFExternalBelowInternal` |
| ASBR status + `default-information originate` | unit test | `TestOSPFASBRBitClearedOnEmpty`, `TestOSPFDefaultInformationOriginate` |
| FRR accepts/computes the externals | interop test (owned by ospf-13) | `ospf-redist-frr` scenario; execution Linux/QEMU-pending |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/ospf/redistribute/`, `internal/component/ospf/spf/external.go`)
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
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (out-of-scope honoured)
- [ ] Single responsibility per file (source vs consumer vs external)
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling (no `ospf` spelling in generic redistribute/redistevents)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-ospf-10-as-external-asbr.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ospf-10-as-external-asbr.md`

## Cross-References
- `plan/spec-ospf-0-umbrella.md` -- umbrella; `## Shared Contracts (canonical)` "Route install vs redistribution", "Redistribution source", "LSA inventory" (Type 5), "Route preference / path types"; Architecture rows (`redistribute/`, `spf/`); the ospf-10 Metrics rows; the ospf-10 Child-Specs / Dependency rows
- **Two distinct paths (do not conflate):** FIB / kernel install is ospf-8 (SPF routes -> Loc-RIB `locrib.Path` -> sysrib `OnChange` -> fibkernel); THIS spec is purely (a) Type 5 origination + §16.4 external computation and (b) redistribution (OSPF SPF out via `redistevents`; connected/static/BGP in via the `ospf` `RedistConsumer` writing Type 5 LSAs). `redistevents` never installs to the FIB
- `plan/spec-ospf-8-spf-rib.md` -- dependency; SPF, the route table with path types, and the Loc-RIB FIB-install path must exist first; this spec adds the external path type and reuses the install seam
- `plan/spec-ospf-9-inter-area-abr.md` -- sibling; Type 4 ASBR-summaries (so other areas reach this ASBR) and the inter-area route table the §16.4 ASBR lookup consults
- `plan/spec-ospf-11-stub-nssa.md` -- sibling; owns Type 5 suppression in stub/NSSA areas and NSSA Type 7 origination + Type 7 -> Type 5 translation; this spec coordinates (floods Type 5 AS-wide via ospf-7) but does not implement the scope filter
- `plan/spec-isis-11-redistribution.md` -- the IS-IS sibling whose `redistevents` producer + `configredist` source/consumer pattern this spec mirrors verbatim
