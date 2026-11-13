# Spec: bgp-best-route-advertised-to-peers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Priority | P1 |
| Release reason | Routing correctness of a shipped capability: under the default configuration a Ze node between two BGP peers passes on only the routes it originates, where RFC 4271 Section 9.2 has a speaker advertise the routes its Decision Process chose |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` (a best route advertised toward an eBGP peer under the default `next-hop auto` needs the rewrite that spec owns) |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Bucket: `plan/release/fixes/`. `plan/README.md` puts incorrect shipped
behaviour there, routing defects named, and `post-release/features/` holds
additional capabilities and absent RFC features. Ze ships a BGP speaker that
implements RFC 4271, and the lab below shows the default configuration failing
to do what Section 9.2 makes the Update-Send process do. That is a defect of
the shipped routing behaviour, not an extra capability, so it is P1 routing
correctness.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The owner's ruling, 2026-11-13. The deployment-user-journeys session asked:

> "Should Ze pass best routes between BGP peers by default, as FRR and BIRD
> do? Today plain transit needs `bgp-rs` attached to every peer, and it
> forwards every received UPDATE with nothing arbitrating against
> redistribution. If yes, that is a new spec; if no, the guides now say what
> to configure and I leave it."

Thomas answered: "absoletely" (absolutely, yes).

Goal, in his terms: Ze passes the best route it elected between its BGP peers
by default, as an ordinary BGP speaker such as FRR or BIRD does, without the
operator loading or attaching a route-server plugin. The ruling is recorded in
the design page that owns the subject,
`docs/architecture/rib-transition.md`, "Owner ruling: best routes reach the
other peers by default".

This spec is a skeleton. It records the ruling, the evidence, the mechanisms
that already do part of the job, and the choices a design must make. It makes
none of them. The framing in the question above ("nothing arbitrating against
redistribution") is the asking session's suspicion; this spec checks it under
Existing Mechanisms and keeps it as an open question.

### Observed evidence

| Evidence | What it shows | Where |
|----------|---------------|-------|
| netlab reference topology, before commit `ef1a3cf33d` | the render attached only `bgp-rib` and `redistribute-orchestrator` to each peer; r1 never told r2 about r3's loopback, and three of six sourced pings passed | commit `ef1a3cf33d` message; `plan/handover/deployment-user-journeys-2026-11-13/README.md`, "Progress after the migration" |
| the workaround in commit `ef1a3cf33d` | the render now loads and attaches `bgp-rs` and `bgp-adj-rib-in` on every peer, and sets `next-hop self` toward an external peer; all six pings then passed | `contrib/netlab/ze/ze.j2` |
| the user guide | "When `bgp-rs` is not loaded, received routes are not forwarded to other peers. The `bgp-rib` plugin stores routes and performs best-path selection, but does not re-advertise them." | `docs/guide/route-reflection.md`, "Without Route Reflection" |
| the architecture page | "`bgp-rib` forwards nothing between peers" | `docs/architecture/core-design.md`, the paragraph on a changed grant on a route-consuming process |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/rib-transition.md` - the design this ruling changes: the engine caches bytes, a plugin decides forwarding
  → Decision: the owner ruled that best routes reach the other peers by default ("Owner ruling" section)
  → Constraint: the engine holds no policy and no Loc-RIB (`ai/doors/bgp-engine.md`, "Where it sits"); a design that moves best path into the reactor departs from that page and needs a discussion with Thomas first
- [ ] `docs/architecture/core-design.md` - "Forwarding Path", "BGP RIB Best-Path Tracking", "The Two Deliveries a Redistribution Rule Derives"
  → Constraint: three forwarding paths exist today (the reactor RS fast path, the text `cached` path, the typed `ForwardCached` path), and the last two end in the shared `ForwardUpdate` core
- [ ] `docs/architecture/rib/forward-handle.md` - the zero-copy handle a Loc-RIB change carries, whose intended consumer is an RS/RR fast path
- [ ] `docs/guide/route-reflection.md` - "Forwarding and Destination Election", "Plugin Bindings", "Without Route Reflection"
- [ ] `docs/architecture/plugin/plugin-system.md` - the process boundary: any design must work with the deciding plugin forked

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - rows RFC4271-9.2-6, RFC4271-9.2-7, RFC4271-9.2-9, RFC4271-9.2-2, RFC4271-9.2-3
  → Constraint: RFC4271-9.2-7 carries no gap annotation, though under the default configuration no newly installed route reaches another peer; the design phase checks whether that row's claim holds and corrects the summary if it does not
- [ ] `rfc/short/rfc8212.md` - RFC8212-3-1, RFC8212-3-2
- [ ] `rfc/short/rfc4456.md`, `rfc/short/rfc7947.md` - the reflector and route server roles that must keep working

### RFC text (read from `rfc/full/rfc4271.txt` and `rfc/full/rfc8212.txt`)

| RFC section | Sentence, byte-exact | Why it decides |
|-------------|----------------------|----------------|
| RFC 4271 Section 9.1 | "The output of the Decision Process is the set of routes that will be advertised to peers; the selected routes will be stored in the local speaker's Adj-RIBs-Out, according to policy." | the elected route is what a speaker advertises |
| RFC 4271 Section 9.1.3 | "All routes in the Loc-RIB are processed into Adj-RIBs-Out according to configured policy." | dissemination is of Loc-RIB routes, the elected ones |
| RFC 4271 Section 9.1.3 | "This policy MAY exclude a route in the Loc-RIB from being installed in a particular Adj-RIB-Out." | exclusion is policy, per destination |
| RFC 4271 Section 9.1.3 | "When the updating of the Adj-RIBs-Out and the Routing Table is complete, the local BGP speaker runs the Update-Send process of 9.2." | Phase 3 drives Update-Send |
| RFC 4271 Section 9.2 | "The Update-Send process is responsible for advertising UPDATE messages to all peers." | the default this ruling restores |
| RFC 4271 Section 9.2 | "When a BGP speaker receives an UPDATE message from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the routing information contained in that UPDATE message to other internal peers (unless the speaker acts as a BGP Route Reflector [RFC2796])." | iBGP split horizon, RFC4271-9.2-6 |
| RFC 4271 Section 9.2 | "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." | RFC4271-9.2-7 |
| RFC 4271 Section 5.1.3, item 1 | "When sending a message to an internal peer, if the route is not locally originated, the BGP speaker SHOULD NOT modify the NEXT_HOP attribute unless it has been explicitly configured to announce its own IP address as the NEXT_HOP." | next hop toward iBGP; the eBGP default is owned by the Depends spec |
| RFC 8212 Section 3 | "Routes SHALL NOT be added to an Adj-RIB-Out associated with an EBGP peer if no explicit Export Policy has been applied." | a default that advertises toward eBGP meets this MUST |

**Key insights:**
- The Decision Process exists (`bgp-rib`), and forwarding exists (`bgp-rs`, `bgp-rr`, the reactor fast path). What is missing is that the elected route reaches the other peers without the operator adding a route-server plugin.
- `bgp-rs` forwards received UPDATEs, not the Loc-RIB; a destination without ADD-PATH defers to the `bgp-rib` election only where sources contest a prefix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/rib_bestchange.go` - `publishBestChanges` emits the elected-route changes as one batch on the event bus, then calls the BGP redistribution bridge directly
- [ ] `internal/component/bgp/plugins/rib/forward_tracker.go` - `forwardStateTracker` records the prefixes whose Loc-RIB best is a BGP path and reads the `ForwardHandle` bytes; it sends nothing, and its comment names the RS/RR fast path as the intended consumer
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - `batchForwardUpdateSkipped` batches cached message ids toward the peers the reactor fast path did not serve
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - the forward loop suppresses an iBGP source toward an iBGP destination unless one of them is a reflector client (`isReflected`)
- [ ] `internal/component/bgp/reactor/session_ownership.go` - `oweAnnouncement` and `adjOutContests`: the destination writer detects a second owner for a key and, while a recovery owner receives the source's UPDATEs, owes the key to the RIB election
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `precomputeNextHop` maps `NextHopAuto` and `NextHopUnchanged` to the same no-op mode
- [ ] `internal/core/redistevents/registry.go` - `WouldLoop` returns true when source and destination protocol names are equal, so BGP is never redistributed into BGP
- [ ] `internal/component/bgp/redistribute/producer.go` - `EmitBestChange` turns `bgp-rib` best changes into redistribution route changes for other protocols

**Behavior to preserve:**
- A configuration that attaches `bgp-rs` or `bgp-rr` keeps its route server or reflector behaviour (RFC 7947, RFC 4456), the reactor fast path included.
- iBGP split horizon (RFC 4271 Section 9.2) and the reflector rules in the forward loop.
- The destination writer's per-key ownership rules (a withdrawal from another source never erases a replacement or a locally originated route).
- Redistribution of BGP into other protocols, and of other protocols into BGP.

**Behavior to change:**
- Under the default configuration, the best route Ze elected reaches its other BGP peers, subject to policy and the RFC rules above, with no route-server plugin loaded or attached by the operator. How is the design question.

## Existing Mechanisms (MANDATORY)

Every row below was read at the producing code or its page on 2026-11-13. The
last column is an open design question, not a choice.

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| `bgp-rib` election and best-change batch | `rib/rib_bestchange.go` `publishBestChanges`; `core-design.md` "BGP RIB Best-Path Tracking" | elects the best path per prefix and publishes the changes; subscribers include BMP Loc-RIB monitoring (`bmp/bmp_locrib.go`) and flow-export enrichment (`flowexport/enrichbgp.go`), and the BGP redistribution bridge is called directly; neither `bgp-rs` nor `bgp-rr` subscribes | open: extend it as the source of what is advertised (Loc-RIB driven, RFC 4271 Section 9.1.3), or keep forwarding UPDATE driven and only defer to it |
| Loc-RIB mirror and `ForwardHandle` | `rib/forward_tracker.go` `forwardStateTracker`; `docs/architecture/rib/forward-handle.md` | carries the elected route's received wire bytes zero-copy to a change subscriber; the tracker only records state, behind `request bgp rib fastpath enable` | open: candidate carrier for advertising the elected route's bytes without re-encoding; its comment already names an RS/RR fast path as the intended consumer |
| `bgp-rs` forwarding | `rs/server_forward.go` `batchForwardUpdateSkipped`; `docs/guide/route-reflection.md` "Forwarding and Destination Election" | forwards each received UPDATE by cache id to the other peers it is attached to; a destination without ADD-PATH defers to the `bgp-rib` election only where sources contest a prefix | open: make the default advertiser a mode of it, keep it as the RFC 7947 role only, or replace its transit use; it must keep working for route servers |
| Reactor RS fast path | `reactor/forward_rs.go` `reactorForwardRS`; `core-design.md` "Forwarding Path" | forwards inline on the read goroutine for peers with `rs-fast-path`, handing back to `bgp-rs` what it cannot serve | open: whether a default advertiser reuses this rail; preserve it for route servers |
| Forward loop rules | `reactor/reactor_api_forward.go` (`isReflected`, the eBGP AS_PATH prepend, the egress chain) | applies iBGP split horizon, reflector rules, AS_PATH prepend and egress policy per destination | extends: any advertiser reaches the destination through these rules, never a second copy of them |
| Destination writer ownership | `reactor/session_ownership.go` `oweAnnouncement`, `adjOutContests` | tracks the owner of each advertised key and owes a contested key to the RIB election while the recovery owner receives the source's UPDATEs | open: this is the arbitration point the asking session said was missing. Whether a contest between a forwarded owner and a locally originated (redistributed) owner reaches an election was not traced; the design traces it |
| `bgp-rr` | `internal/component/bgp/plugins/rr` (registered as `bgp-rr`, "Route Reflector") | reflects iBGP routes under RFC 4456 when attached | does not apply as the default (reflection is a configured role), but must keep working beside it |
| Redistribution | `internal/core/redistevents/registry.go` `WouldLoop`; `internal/plugins/redistribute`; `bgp/redistribute/producer.go` `EmitBestChange` | carries routes between protocols and refuses BGP into BGP | does not apply as the carrier (BGP into BGP is a loop by its own invariant); open: how a redistributed route and an elected BGP route for the same prefix toward the same peer are arbitrated |
| Derived process binding | `core-design.md` "The Two Deliveries a Redistribution Rule Derives" | a redistribution rule derives the `attach process` binding a peer needs, so the operator does not type it | open: precedent for deriving the bindings a default advertiser needs instead of the operator attaching a plugin |
| Config-root auto-load | `core-design.md`, protocol-agnostic plugin loading; `bgp-rib` auto-loads under its registry name | starts a plugin the config implies without a `plugin` block | open: precedent for loading the advertiser by default |
| Filter insertions | `rpc.FilterInsertion`; `ai/doors/bgp-plugins.md` "Boundaries" | a plugin inserts the filter-list members its config implies | open: whether the default's policy steps (RFC 8212 export default, split horizon) become declared members or stay where they are |
| RFC 8212 import default | `rfc/short/rfc8212.md` RFC8212-3-1; `bgp-rib` source policy | `bgp-rib` enforces the default import rejection | open: where the export half (RFC8212-3-2) is enforced for routes the default advertises toward eBGP |
| Next-hop default | `plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md` | owns the `next-hop auto` rewrite toward eBGP that `precomputeNextHop` lacks | dependency: an elected route advertised to eBGP under the default is wrong until it lands |

## Open Design Questions (put to Thomas before `ready`)

| # | Choice | Options seen so far | Evidence still owed |
|---|--------|---------------------|---------------------|
| Q-1 | Which component advertises the elected route | `bgp-rib` itself; `bgp-rs` in a new mode; a new advertiser plugin; the reactor (departs from `rib-transition.md`) | FRR's and BIRD's deciding functions, read and quoted at the lab's versions |
| Q-2 | How it relates to `bgp-rs` and `bgp-rr`, which must keep working | the default yields to an attached route server or reflector per peer; both coexist under one arbiter; one replaces the other | the config forms in the repository that attach each today |
| Q-3 | Arbitration between an elected BGP route and a redistributed route for the same prefix toward the same peer | the destination writer's ownership check; the daemon Loc-RIB's cross-protocol selection; an explicit precedence | a trace of `adjOutContests` with a locally originated owner |
| Q-4 | iBGP split horizon and route reflection under the default | the forward loop's existing `isReflected` rule; a rule in the advertiser | none beyond reading the chosen advertiser |
| Q-5 | RFC 8212 export default toward eBGP | refuse without an explicit export policy, as the RFC says; a configuration option, which RFC 8212 Section 3 permits ("A BGP speaker MAY provide a configuration option to deviate from the following updated behaviors.") | how Ze enforces RFC8212-3-2 today, read at the producer |
| Q-6 | What happens to configs that attach `bgp-rs` for plain transit today (the netlab render, guides, `.ci` bodies, interop scenarios) | convert them in the same work, as the pre-release rule on behaviour changes requires; leave route-server configs as they are | a census of the repository's users |
| Q-7 | The plugin process boundary | the advertiser works with it forked: what crosses is a self-contained value, and the cache ack stays cumulative and FIFO | `ai/doors/system.md`, "A plugin can run in another process" |
| Q-8 | Advertising from the Loc-RIB versus forwarding received UPDATEs | Loc-RIB driven (the RFC 4271 Section 9.1.3 wording) sends only the winner; UPDATE driven sends every received path and needs the election to suppress losers | allocation figures on the forward path (time figures only on a known host) |
| Q-9 | Replay to a peer that comes up later (today `bgp-rs` reads `bgp-adj-rib-in` for it) | replay from the Loc-RIB; keep `bgp-adj-rib-in` | RFC 4271 Section 9.1.3 event c ("when a new BGP speaker connection has been established") |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: an UPDATE received from one BGP peer.
- Config: the default configuration, with no route-server plugin loaded or attached.

### Transformation Path
1. To be designed. Today: receive, cache, dispatch to `bgp-rib`, which elects and publishes a best change that no forwarder reads.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | to be designed (Q-1, Q-7) | No |

### Changed-Wire Census

**Search evidence:** to be filled at design, once Q-1 fixes which wire carries the advertisement.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- To be designed.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at design |
| No unintended coupling (components stay isolated) | No | to be filled at design |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled at design, against Existing Mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | No | to be filled at design |
| Registration over hardcoding, outbound | No | to be filled at design |
| Registration over hardcoding, inbound | No | to be filled at design |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No forwarder consumes the `bgp-rib` best-change batch | grep on 2026-11-13: no reference in `internal/component/bgp/plugins/rs` or `rr` | an existing consumer already advertises and the defect is elsewhere | design re-reads every subscriber | unvalidated |
| A-2 | The netlab failure is the default behaviour, not a render mistake | commit `ef1a3cf33d`: pings failed with `bgp-rib` alone and passed with `bgp-rs` attached | the spec fixes the wrong layer | a `.ci` with two peers and no route-server plugin, red at HEAD | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two advertisers send the same prefix to the same peer (the default and an attached `bgp-rs`, or the default and redistribution) | flapping or duplicate announcements in a `.ci` with both | Q-2 and Q-3 settle one owner per destination key before the design is approved |
| R-2 | The default advertises toward eBGP routes an operator never meant to export | routes seen on an eBGP peer with no export policy | Q-5: RFC 8212 Section 3 governs |
| R-3 | Route server and reflector regressions | `bgp-rs-*` and reflector `.ci` and interop reds | preserve both roles and run their suites |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes leaked to or withheld from peers on every Ze node with more than one BGP peer |
| How is it reverted? | To be decided at design; it changes a default |
| Who else touches this path? | `spec-bgp-next-hop-auto-rewrites-for-ebgp.md`; route server and reflector work under `internal/component/bgp/plugins/rs` and `rr`; the reactor forward path |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from iBGP peer A, default config, eBGP peer B | → | the advertiser chosen at Q-1 | `test/plugin/bgp-best-route-transit-default.ci` (planned name) |

## Acceptance Criteria

Only the criteria the ruling and the RFC text ground. The design adds the rest.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Default config, no route-server plugin configured; peer A (iBGP) announces a prefix, peer B is eBGP with an export policy that accepts it | B receives the prefix with the path Ze elected | none: the ruling makes this the default |
| AC-2 | The same, with A eBGP and B iBGP | B receives the elected route | the import policy on A may still refuse it (RFC 8212 Section 3) |
| AC-3 | A and B both iBGP, no reflection configured | the route stays in Ze's Loc-RIB, and B is sent only what RFC 4271 Section 9.2 allows | A's route is not sent to B: Section 9.2 forbids re-distribution to other internal peers |
| AC-4 | The elected route is withdrawn and no replacement exists | every peer that was sent it receives a withdrawal | none: RFC 4271 Section 9.2, RFC4271-9.2-7 |
| AC-5 | A config that attaches `bgp-rs` or `bgp-rr` | route server and reflector behaviour as at HEAD | the default's own advertisement toward those peers, as Q-2 decides |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator configures a node between an iBGP and an eBGP peer and adds no plugin | wire → `bgp-rib` election → advertiser → forward rules → wire | `test/plugin/bgp-best-route-transit-default.ci` (planned name) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| to be named at design | the advertiser's package | AC-1 to AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input known at skeleton | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-best-route-transit-default` (planned) | `test/plugin/` | a node between two peers passes the elected route on with no route-server plugin | |

### Interop Tests (Scope: protocol)
Owed against both FRR and BIRD. Before a check expects either to accept or
print a route, read that behaviour in its source at the lab's version and name
the function.

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-best-route-transit-frr` (planned name) | `test/interop/scenarios/` | FRR | FRR on one side learns, through Ze, the route FRR on the other side announced, with no route-server plugin on Ze; red when the change is reverted | |
| `bgp-best-route-transit-bird` (planned name) | `test/interop/scenarios/` | BIRD 3 | the same with BIRD on both sides | |

## Files to Modify
- To be decided at design (Q-1). Docs known now: `docs/architecture/rib-transition.md`, `docs/architecture/core-design.md`, `docs/guide/route-reflection.md` ("Without Route Reflection"), `docs/guide/plugins.md`, and `contrib/netlab/ze/ze.j2` (Q-6).

## Files to Create
- `test/plugin/bgp-best-route-transit-default.ci` - functional test
- the two interop scenarios above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | to decide | Q-5 may add an option |
| YANG validation constraints | to decide | with any option |
| YANG custom validators | to decide | with any option |
| CLI commands/flags | to decide | design |
| CLI grammar (keyword before value) | to decide | design |
| Editor autocomplete | to decide | design |
| Functional test for new RPC/API | to decide | design |
| Pipe completeness | to decide | design |
| Env var registration | to decide | design |
| Doctor check for runtime dependencies | to decide | design |
| Doctor code per problem class | to decide | design |
| Prometheus counters/metrics | to decide | design |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | to decide | design |
| 2 | Config syntax changed? | to decide | design |
| 3 | CLI command added/changed? | to decide | design |
| 4 | API/RPC added/changed? | to decide | design |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md` |
| 7 | Wire format changed? | to decide | design |
| 8 | Plugin SDK/protocol changed? | to decide | design |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` (RFC4271-9.2-7), `rfc/short/rfc8212.md` |
| 10 | Test infrastructure changed? | to decide | design |
| 11 | Affects daemon comparison? | to check | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/rib-transition.md`, `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | to decide | design |
| 14 | Prometheus counters added/changed? | to decide | design |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | to decide | design |
| 16 | Any changed source file referenced by existing doc source anchors? | to derive | `./le spec citation anchors spec plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md` at design |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the guides that attach `bgp-rs` for transit |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the functional test above, red at HEAD
2. Further phases at design.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | one owner per destination key whatever plugins are attached (Q-2, Q-3) |
| Data flow | the advertiser reaches peers through the existing forward rules, not a copy of them |
| Rule: `ai/rules/no-layering.md` | transit through `bgp-rs` is not kept beside the default as a second path for the same job unless Q-2 decides so |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Default transit | `bgp-best-route-transit-default.ci` passes, red when reverted |
| Interop | both scenarios pass, red recorded |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Route leak | nothing reaches an eBGP peer without an export policy unless Q-5 decides otherwise |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- To be written at design.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Best routes reach the other peers by default (owner ruling, 2026-11-13) | keep transit behind `bgp-rs` and document it | Thomas answered "absoletely" to the question quoted under Task |

## Known Limitations

- None yet.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, `./le test functional plugin`, and the two interop scenarios through `./le test integration interop`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
