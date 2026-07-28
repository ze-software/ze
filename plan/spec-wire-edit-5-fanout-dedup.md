# Spec: wire-edit-5-fanout-dedup -- one materialisation per policy group, not per destination

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | `plan/spec-wire-edit-3-aspath-fold.md` |
| Phase | 5 |
| Deferral shard | `plan/deferrals/spec-wire-edit-5-fanout-dedup.md` |
| Updated | 2026-07-28 |

Child 5 of `plan/spec-wire-edit-0-umbrella.md`. It restores fan-out sharing, but
upstream of the copy rather than downstream of it.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

One received route fanned out to N destinations does per-destination work N
times. Today's sharing happens too late to help: the forward-body cache keys on
the **materialised wire pointer**.

`fwdBodyCacheKey` (`internal/component/bgp/reactor/reactor_api_forward.go:569`)
is a destination context ID, a `*wireu.WireUpdate` **pointer**, and an extended
flag. It is populated at `internal/component/bgp/reactor/reactor_api_forward.go:828`
and read at `internal/component/bgp/reactor/reactor_api_forward.go:807`, and it is
only active when update groups are enabled
(`internal/component/bgp/reactor/reactor_api_forward.go:580`). The route-server
rail has the same structure with a four-slot array instead of a map
(`internal/component/bgp/reactor/forward_rs.go:276`,
`internal/component/bgp/reactor/forward_rs.go:289`).

Keying on the pointer means two destinations share a body only if they already
produced the *same pointer*. Two destinations in the same policy group, whose
edits are identical, each run `buildModifiedPayload`
(`internal/component/bgp/reactor/reactor_api_forward.go:793`) into their own
per-peer buffer, and only then discover that the results are equal. The copy is
already paid before the cache can help.

**Goal.** Fingerprint the **edit set** and dedup before materialising. Two
destinations over the same base with an equal edit set share one materialisation.
A fan-out of N destinations across G distinct policy groups performs G
materialisations, not N.

The fingerprint is a **hint**, never a decision on its own: a hash collision
would send one destination another destination's wire, so a candidate match is
confirmed by a full edit-set equality check before any sharing happens.

**Non-goal.** No change to what any destination receives. This child changes how
many times identical bytes are produced, never which bytes.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `docs/architecture/encoding-context.md` - per-peer encoding context
  → Constraint: two destinations may share a materialisation only when their destination context IDs match, exactly as the current body cache already requires. Context is part of the identity, not part of the edit set.
- [ ] `docs/architecture/memory/lifetime-contracts.md` - buffer lifetime contracts
  → Constraint: a shared materialisation is referenced by several forward items, so the per-peer pool buffer model does not fit. A shared buffer needs one owner and exactly one return, after the last referencing write completes.
  → Decision: the safest shape is a single owner with reference counting, or a shared buffer drawn from a pool whose return is driven by the last item released, never per item.
- [ ] `ai/rules/buffer-first.md` - encoding writes into pooled bounded buffers
  → Constraint: sharing must not reintroduce an allocation. A shared materialisation still comes from a pool; it just is not the per-peer pool of one destination.
- [ ] `docs/architecture/core-design.md` - update groups and the forward path
  → Constraint: update groups already express "these peers are alike". The fingerprint must not become a second, contradictory grouping concept; it is the mechanism that makes the existing concept pay off.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: a dedup that is wrong is silent and catastrophic. The equality confirmation is not an optimisation guard, it is the correctness guard, and it must never be skipped for speed.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format
  → Constraint: the bytes each peer receives must remain exactly what its own policy produces. Sharing is legal only when the produced bytes are provably identical.
- [ ] `rfc/short/rfc4456.md` - route reflection
  → Constraint: CLUSTER_LIST and ORIGINATOR_ID differ per cluster, so reflector clients in different clusters must never share a materialisation. This is a natural consequence of edit-set equality, and a test must pin it.
- [ ] `rfc/short/rfc7947.md` - route server
  → Constraint: RS clients differ by which control communities are stripped and by whether AS_PATH is modified, so the route-server rail is where fan-out is largest and where equality classes are most valuable.
- [ ] `rfc/short/rfc8654.md` - extended message
  → Constraint: the extended-message flag already participates in the current cache key and must participate in the identity here too, since it changes the split behaviour downstream.

**Key insights:** (minimal context to resume after compaction)
- The current cache cannot dedup upstream of the copy because its key **is** the result of the copy.
- Both forward rails have the same cache with different storage: a map on the API rail, a four-slot array on the route-server rail (`internal/component/bgp/reactor/forward_rs.go:289`).
- Dedup is gated on update groups being enabled today, so this child inherits an existing on-off switch rather than needing a new one.
- The zero-copy passthrough for an unmodified same-context forward (`internal/component/bgp/reactor/forward_body.go:72`) already costs nothing, so dedup must not disturb it: an empty edit set has no materialisation to share.
- Child 3 deletes the eBGP wire caches. Between child 3 and this child, a large eBGP fan-out does per-destination work that those caches used to share, which is why this child follows directly.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:569` - `fwdBodyCacheKey`: destination context ID, the materialised `*wireu.WireUpdate` pointer, and the extended flag.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:580` - `groupsEnabled`: the cache exists only when update groups are enabled, and `fwdBodyCache` is allocated at `internal/component/bgp/reactor/reactor_api_forward.go:583`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:807` - the lookup, which jumps straight to dispatch on a hit.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:818` - `buildFwdBody`, the work a hit skips, plus the `cacheKey` store at `internal/component/bgp/reactor/reactor_api_forward.go:828`.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:793` - `buildModifiedPayload`: the per-destination materialisation that happens **before** the cache is consulted, so a hit never saves it.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:800` - the `fwdItem` construction, carrying `peerBufIdx` and the pool reference that decide when the buffer is returned.
- [ ] `internal/component/bgp/reactor/forward_rs.go:276` - the route-server rail's own `fwdBodyCacheKey`, with `var bodySlots [4]fwdBodyCacheSlot` at `internal/component/bgp/reactor/forward_rs.go:289`, a `groupsEnabled` lookup at `internal/component/bgp/reactor/forward_rs.go:452` and a `bodySlotCount < len(bodySlots)` store at `internal/component/bgp/reactor/forward_rs.go:476`.
- [ ] `internal/component/bgp/reactor/forward_body.go:37` - `buildFwdBody`: the same-context branch appends `peerWire.Payload()` with no copy at `internal/component/bgp/reactor/forward_body.go:72`, and the cross-context branch re-encodes.
- [ ] `internal/component/bgp/reactor/forward_build.go:392` - `acquireModBuf`: the per-peer pool first, then the shared pool, then a bare allocation. The per-peer pool is what a shared materialisation cannot use unchanged.
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go:228` - `applyFactsNextHop`, plus `applyFactsSendCommunity` at `internal/component/bgp/reactor/peer_forward_facts.go:246`: the two producers whose output is identical for every peer in a group, which is what makes equality classes large in practice.
- [ ] `internal/component/bgp/reactor/recent_cache.go:73` - `maxEntries`: the soft limit on cached received updates, which bounds how long a base and any shared materialisation can live.

**Behavior to preserve:**
- The exact bytes each destination receives. Sharing changes the number of materialisations, never the content.
- The zero-copy passthrough for an unmodified same-context forward (`internal/component/bgp/reactor/forward_body.go:72`).
- The existing update-groups gate: when groups are disabled, behaviour is unchanged.
- The forward-item lifecycle: every buffer has exactly one return, after the last write that references it.
- Reflector clients in different clusters receive different CLUSTER_LIST bytes; RS clients with different community policies receive different community bytes.
- Both rails keep working, including the route-server rail's bounded storage discipline.
- Every existing expectation under `test/plugin/`, `test/policy/` and `test/encode/`.

**Behavior to change:**
- The dedup key moves from the materialised wire pointer to the edit set fingerprint plus the destination context and extended flag.
- Dedup happens before `buildModifiedPayload` rather than after it.
- A shared materialisation is owned once and released once, replacing the per-peer buffer index for shared entries.
- A candidate fingerprint match is confirmed by full edit-set equality before any sharing.

## Data Flow (MANDATORY)

### Entry Point
- A received UPDATE published as an immutable base, fanned out to many destinations in one forward call.
- Each destination's forward facts, export policy chain and in-process egress filters, which together determine that destination's edit set.

### Transformation Path
1. The destination loop builds the edit set for this peer, exactly as `plan/spec-wire-edit-2-edit-apply.md` and `plan/spec-wire-edit-3-aspath-fold.md` define.
2. **Proposed:** compute the edit set's fingerprint, covering every slot, every fragment, the arena contents, the AS-path intent and the NLRI and withdrawn overrides.
3. **Proposed:** look up the identity (base pointer, destination context ID, extended flag, fingerprint). On a candidate hit, confirm full edit-set equality.
4. On a confirmed hit, reuse the existing materialisation and skip both `buildModifiedPayload` (`internal/component/bgp/reactor/reactor_api_forward.go:793`) and `buildFwdBody` (`internal/component/bgp/reactor/reactor_api_forward.go:818`).
5. On a miss, materialise once into a shared buffer, build the body, and record it under the identity.
6. Dispatch to the forward pool. Each item references the shared materialisation; the buffer is returned when the last referencing item is released.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Edit set to fingerprint | every field that can affect output bytes is hashed; nothing that cannot is | No |
| Fingerprint to equality confirmation | a candidate match is a hint; the full comparison is what authorises sharing | No |
| Shared materialisation to many forward items | one owner, one return, after the last referencing write | No |
| Forward loop to forward-pool workers | several workers may write the same bytes concurrently; the buffer is read-only for all of them | No |

### Integration Points
- The edit set from `plan/spec-wire-edit-2-edit-apply.md` gains a fingerprint and an equality operation.
- `fwdBodyCacheKey` on both rails is replaced by the new identity, so both rails share one dedup implementation instead of two.
- The update-groups gate stays where it is, so the feature has an existing off switch.
- `buildFwdBody` (`internal/component/bgp/reactor/forward_body.go:37`) is unchanged; it is simply called fewer times.
- `wireu.SplitWireUpdate` still runs after materialisation, and a shared materialisation splits once for every destination that shares it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Fingerprinting pays off from a fan-out of two upward. | Hashing a few hundred bytes of edit set replaces a payload copy of 100 to 4000 bytes plus a pool-buffer acquisition. | Gate dedup on a measured fan-out threshold, or drop the child entirely: children 1 to 4 stand alone. | `BenchmarkFanoutDedup` comparing per-destination cost with dedup on and off at fan-out 1, 2, 10 and 100. | unvalidated |
| A-2 | Destinations in one update group usually produce equal edit sets. | The facts-driven producers (`internal/component/bgp/reactor/peer_forward_facts.go:228`, `internal/component/bgp/reactor/peer_forward_facts.go:246`) read per-peer settings that a group shares by construction. | The equality classes are small and the win is small; A-1's benchmark already measures that case. | Instrumented run recording the distribution of equality-class sizes over a real fan-out. | unvalidated |
| A-3 | Every field that can change output bytes is reachable from the edit set, so the fingerprint is complete. | The edit set is the sole input to the writer after `plan/spec-wire-edit-2-edit-apply.md`, other than the base, context and extended flag, all of which are in the identity. | A field outside the fingerprint means two destinations can share bytes that should differ, which is the catastrophic case. This must be found by construction, not by test. | A field-by-field audit of the edit set against the writer's inputs, plus a property test that mutates each field and asserts the fingerprint changes. | unvalidated |
| A-4 | A shared materialisation can be released correctly without per-peer buffer indices. | The current lifecycle returns the buffer from the item that owns it (`internal/component/bgp/reactor/reactor_api_forward.go:800`), which is a one-to-one model. | Fall back to materialising per destination and sharing only the parsed body, which is what the current cache does. | A soak run under the debug buffer poison, plus a test that releases items out of order. | unvalidated |
| A-5 | Splitting a shared materialisation for several destinations is safe and cheap. | `SplitWireUpdate` returns the identical pointer when no split is needed, so the common case is free, and it reads its input without mutating it. | Materialise separately whenever a split is required. | A test that fans out an oversized UPDATE to several destinations in one equality class. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A fingerprint collision sends one destination another destination's wire. This is silent, and it is the worst failure in the whole umbrella. | A property test that constructs colliding inputs and asserts the equality check rejects them. | The fingerprint is a hint only. Full edit-set equality is checked before any sharing, unconditionally, with no fast-path bypass. |
| R-2 | An edit-set field is added later and not added to the fingerprint, silently widening the equality class. | A test that fails when a new field is added without being hashed. | The fingerprint is derived from an explicit field list that the equality check shares, so a new field that is compared but not hashed, or hashed but not compared, is a test failure. |
| R-3 | Shared buffer lifetime is harder than per-peer lifetime, and this is the exact area a prior fix had to repair. | Debug-build poison reads and out-of-order release tests. | A-4 is a gate. If the lifecycle cannot be made simple, share only the body and keep per-destination materialisation. |
| R-4 | Dedup disturbs the zero-copy passthrough, turning a free forward into a hashed one. | The fast-path `.ci` files and a benchmark at fan-out 1. | An empty edit set short-circuits before any fingerprint is computed: there is nothing to share. |
| R-5 | Reflector clients in different clusters, or RS clients with different community policies, are wrongly grouped. | Dedicated tests for both. | Their edit sets differ in CLUSTER_LIST and community bytes respectively, so equality separates them. The tests pin that rather than trusting it. |
| R-6 | The win does not appear in the existing perf baseline, which is a single-peer convergence run with almost no fan-out. | The baseline shows nothing either way. | The benchmark in A-1 is the proof, not the baseline. This is stated up front so a flat baseline is not read as a failed change. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A destination receives another destination's UPDATE: wrong next-hop, wrong communities, a CLUSTER_LIST from another cluster, or a route that policy should have suppressed for it. This is a silent, targeted mis-advertisement and the peer has no way to detect it. |
| How is it reverted? | Single commit revert, and the update-groups gate means it can also be turned off at runtime. Once a peer has accepted another peer's routes the wire effect is not revertible from our side. |
| Who else touches this path? | `plan/spec-wire-edit-3-aspath-fold.md` deletes the caches this child replaces; `plan/spec-perf-next-0-umbrella.md` records the profile-before-coding gate that applies here. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| One route fanned out to many peers in two policy groups | → | fingerprint plus equality, one materialisation per group | `test/plugin/wire-edit-fanout-dedup.ci` |
| One route fanned out to reflector clients in two different clusters | → | edit sets differ in CLUSTER_LIST, so no sharing occurs | `TestReflectorClustersNeverShare` |
| One route fanned out to RS clients with different community policies | → | edit sets differ, so no sharing occurs | existing `test/plugin/bgp-rs-community-strip-multi.ci` |
| A route forwarded unchanged to a same-context peer | → | empty edit set short-circuits before any fingerprint | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | One route fanned out to N destinations falling into G distinct edit sets | Exactly G materialisations occur, not N |
| AC-2 | Two destinations whose edit sets are equal | Both receive byte-identical UPDATEs, and only one materialisation is performed |
| AC-3 | Two destinations whose edit sets differ in any single field | No sharing occurs, and each receives its own bytes |
| AC-4 | A constructed fingerprint collision between two unequal edit sets | The equality check rejects the candidate and no sharing occurs |
| AC-5 | A new field added to the edit set without being added to the fingerprint | A test fails |
| AC-6 | Reflector clients in two different clusters | Each receives its own CLUSTER_LIST; no sharing |
| AC-7 | A destination whose edit set is empty | No fingerprint is computed and the zero-copy passthrough is unchanged |
| AC-8 | A shared materialisation referenced by several forward items released out of order | The buffer is returned exactly once, after the last release, with no poison read |
| AC-9 | An oversized UPDATE shared by several destinations | Splitting behaves exactly as today for each of them |
| AC-10 | Update groups disabled | Behaviour is identical to the pre-child behaviour, including the number of materialisations |
| AC-11 | Fan-out at 1, 2, 10 and 100 | The per-destination cost with dedup on is no worse at 1 and 2, and measurably better at 10 and 100; A-1 is resolved with numbers |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a route server with two peer groups and one hundred peers | wire, per-destination edit sets, two materialisations, TCP | `test/plugin/wire-edit-fanout-dedup.ci` |
| 2 | Runs a route reflector with clients in two clusters | wire, differing edit sets, no sharing, TCP | `TestReflectorClustersNeverShare` |
| 3 | Runs a route server whose clients have different community policies | wire, differing edit sets, no sharing, TCP | existing `test/plugin/bgp-rs-community-strip-multi.ci` |
| 4 | Runs a plain iBGP mesh with no policy | wire, empty edit sets, zero-copy passthrough untouched | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` |
| 5 | Disables update groups | wire, no dedup, behaviour identical to before | existing `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFingerprintEqualForEqualEdits` | `internal/component/bgp/filterapi/fingerprint_test.go` | AC-2: equal edit sets hash equal, including generate-slot parameters | |
| `TestFingerprintDiffersForEveryField` | `internal/component/bgp/filterapi/fingerprint_test.go` | AC-3, AC-5, A-3: mutating any single field changes the fingerprint | |
| `TestFingerprintFieldListMatchesEquality` | `internal/component/bgp/filterapi/fingerprint_test.go` | AC-5, R-2: the hashed field list and the compared field list are the same list | |
| `TestCollisionRejectedByEquality` | `internal/component/bgp/filterapi/fingerprint_test.go` | AC-4, R-1: a forced collision does not authorise sharing | |
| `TestEmptyEditSetSkipsFingerprint` | `internal/component/bgp/filterapi/fingerprint_test.go` | AC-7: no hashing on the zero-copy path | |
| `TestFanoutMaterialisesOncePerGroup` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-1: G materialisations for G equality classes | |
| `TestSharedMaterialisationBytesIdentical` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-2: sharing peers receive identical bytes | |
| `TestReflectorClustersNeverShare` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-6: CLUSTER_LIST differences separate destinations | |
| `TestSharedBufferReleasedOnce` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-8, A-4: out-of-order release still returns exactly once | |
| `TestSharedMaterialisationSplits` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-9, A-5: an oversized shared UPDATE splits per destination as today | |
| `TestGroupsDisabledNoDedup` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-10: the existing gate still turns everything off | |
| `BenchmarkFanoutDedup` | `internal/component/bgp/reactor/forward_dedup_bench_test.go` | AC-11, A-1: per-destination cost with dedup on and off at 1, 2, 10, 100 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| fan-out destinations per forward call | 1-n | whatever the peer set holds | 0 (nothing to forward) | N/A |
| distinct equality classes per forward call | 1-n | equal to the destination count | 0 (only when there are no destinations) | N/A |
| references to one shared materialisation | 1-n | the size of its equality class | 0 (the entry would not exist) | N/A |
| fingerprint width | 64 bits | 64 bits | 32 bits (collision rate too high to be a useful hint) | N/A |
| route-server rail cached bodies | 0-4 slots today | 4 | N/A | 5 (must not silently drop; see Known Limitations) |
| UPDATE body length shared | 4-65516 | 65516 | 3 | 65517 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `wire-edit-fanout-dedup` | `test/plugin/wire-edit-fanout-dedup.ci` | two policy groups, many peers, correct per-group wire | |
| `bgp-rs-community-strip-multi` | existing `test/plugin/bgp-rs-community-strip-multi.ci` | RS clients with different policies are not grouped | |
| `bgp-rs-fastpath-ebgp-shared` | existing `test/plugin/bgp-rs-fastpath-ebgp-shared.ci` | the zero-copy fast path is untouched | |
| `bgp-rs-fastpath-ibgp-identity` | existing `test/plugin/bgp-rs-fastpath-ibgp-identity.ci` | the identity forward is untouched | |
| `bgp-rs-reactor-fastpath-fallback` | existing `test/plugin/bgp-rs-reactor-fastpath-fallback.ci` | the fallback path with groups disabled is unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-wire-edit-fanout-groups` | `test/interop/scenarios/` | BIRD and GoBGP together | two real peers in different policy groups each receive their own correct UPDATE from one shared forward call | |

## Files to Modify
- `internal/component/bgp/filterapi/filterapi.go` - the edit set gains a fingerprint and a full equality operation, derived from one shared field list
- `internal/component/bgp/reactor/reactor_api_forward.go` - dedup moves ahead of materialisation; the pointer-keyed body cache retires
- `internal/component/bgp/reactor/forward_rs.go` - the route-server rail uses the same dedup instead of its own four-slot cache
- `internal/component/bgp/reactor/forward_build.go` - a shared materialisation is owned once and released once
- `docs/architecture/core-design.md` - the update-groups and forward-path section
- `docs/architecture/memory/lifetime-contracts.md` - the shared-materialisation ownership rule

## Files to Create
- `internal/component/bgp/filterapi/fingerprint.go` - the field list, the fingerprint and the equality check
- `internal/component/bgp/filterapi/fingerprint_test.go` - equality, completeness and collision coverage
- `internal/component/bgp/reactor/forward_dedup.go` - the per-call identity table and the shared-materialisation lifecycle
- `internal/component/bgp/reactor/forward_dedup_test.go` - fan-out, cluster separation, release and split coverage
- `internal/component/bgp/reactor/forward_dedup_bench_test.go` - the fan-out benchmark
- `test/plugin/wire-edit-fanout-dedup.ci` - per-group materialisation

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | The update-groups leaf already exists and is the gate |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; `test/plugin/wire-edit-fanout-dedup.ci` covers the behaviour |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_materialisations_total` and `bgp_update_dedup_hits_total`, so the equality-class distribution in A-2 is observable in production, plus `bgp_update_dedup_collisions_total` so a rejected candidate is never silent |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The update-groups feature already exists; this changes how it pays off |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | No emitted byte changes |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | RFC 4456 and RFC 7947 behaviour is preserved, not changed |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` for the dedup position, `docs/architecture/memory/lifetime-contracts.md` for shared ownership |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for the three counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `reactor_api_forward.go`, `forward_rs.go` and `forward_build.go`, and correct each stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The update-groups configuration is unchanged |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- count materialisations before changing anything
   - Tests: `TestFanoutMaterialisesOncePerGroup`, written first in its counting form so it records today's N-per-N behaviour
   - Files: `internal/component/bgp/reactor/forward_dedup_test.go`, plus the materialisation counter
   - Verify: the counter exists, the test records today's number, and A-2's distribution becomes measurable
2. **Phase: fingerprint and equality** -- one field list, two consumers
   - Tests: `TestFingerprintEqualForEqualEdits`, `TestFingerprintDiffersForEveryField`, `TestFingerprintFieldListMatchesEquality`, `TestCollisionRejectedByEquality`, `TestEmptyEditSetSkipsFingerprint`
   - Files: `internal/component/bgp/filterapi/fingerprint.go`
   - Verify: AC-3, AC-4, AC-5 and AC-7 pass; A-3 resolved by the field-by-field audit
3. **Phase: shared materialisation lifecycle** -- one owner, one return
   - Tests: `TestSharedBufferReleasedOnce`, `TestSharedMaterialisationSplits`
   - Files: `internal/component/bgp/reactor/forward_build.go`, `internal/component/bgp/reactor/forward_dedup.go`
   - Verify: AC-8 and AC-9 pass; A-4 and A-5 resolved; debug poison reports nothing
4. **Phase: dedup on the API rail** -- move the decision ahead of the copy
   - Tests: `TestFanoutMaterialisesOncePerGroup`, `TestSharedMaterialisationBytesIdentical`, `TestReflectorClustersNeverShare`, `TestGroupsDisabledNoDedup`
   - Files: `internal/component/bgp/reactor/reactor_api_forward.go`
   - Verify: AC-1, AC-2, AC-6 and AC-10 pass; the pointer-keyed cache is gone
5. **Phase: dedup on the route-server rail** -- one implementation, both rails
   - Tests: the existing route-server suites plus `test/plugin/wire-edit-fanout-dedup.ci`
   - Files: `internal/component/bgp/reactor/forward_rs.go`
   - Verify: the four-slot cache is gone and the rail's bounded storage concern is addressed explicitly
6. **Phase: measure**
   - Tests: `BenchmarkFanoutDedup`
   - Files: `internal/component/bgp/reactor/forward_dedup_bench_test.go`
   - Verify: AC-11 passes; A-1 resolved with numbers at 1, 2, 10 and 100. If the numbers do not support the change, this child is dropped and children 1 to 4 stand
7. **Phase: documentation, counters and interop**
   - Tests: the interop scenario
   - Files: the doc targets above, `test/interop/scenarios/`
   - Verify: two real peers in different groups each receive their own correct UPDATE

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every edit-set field appears in both the fingerprint and the equality check |
| Feature completeness | Every user story has a passing test, including both no-sharing stories |
| Correctness | Sharing occurs only after full equality; the fingerprint alone never authorises anything; disabled groups behave exactly as before |
| Naming | The identity, fingerprint and equality-class names follow `ai/rules/naming.md` and do not collide with the update-groups vocabulary |
| Data flow | Dedup sits ahead of materialisation on both rails; the two rails share one implementation |
| Registration over hardcoding | The dedup table is generic over edit sets; no per-policy or per-plugin case is added to the forward loop |
| Rule: `docs/architecture/memory/lifetime-contracts.md` | A shared buffer has exactly one owner and one return, after the last referencing write |
| Rule: `ai/rules/fail-closed-guards.md` | The equality check has no bypass, and a rejected candidate is counted rather than silent |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| G materialisations for G groups | `go test ./internal/component/bgp/reactor/ -run TestFanoutMaterialisesOncePerGroup -v` |
| Collisions cannot authorise sharing | `go test ./internal/component/bgp/filterapi/ -run TestCollisionRejectedByEquality -v` |
| Field list completeness | `go test ./internal/component/bgp/filterapi/ -run TestFingerprintFieldListMatchesEquality -v` |
| Pointer-keyed body cache removed | `grep -rn "fwdBodyCacheKey" internal/component/bgp/reactor/` returns nothing |
| Shared buffers returned exactly once | `go test -race ./internal/component/bgp/reactor/ -run TestSharedBufferReleasedOnce` |
| Fan-out numbers | `go test ./internal/component/bgp/reactor/ -bench BenchmarkFanoutDedup -benchmem` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fingerprint collision | A collision would send one destination another destination's wire. The fingerprint must be a hint confirmed by a full equality check, with no fast path that skips the confirmation under load |
| Cross-peer information leakage | Sharing is the only mechanism in this umbrella that can move one peer's bytes to another. Every equality field must be justified, and any field omitted from the comparison is a leak |
| Attacker-influenced grouping | A peer cannot choose another peer's policy, so it cannot force itself into another's equality class. Confirm that no attacker-controlled attribute value participates in the identity in a way that could |
| Resource exhaustion | The per-call identity table is bounded by the destination count of one forward call, which is bounded by the peer count. It must not outlive the call |
| Concurrency | Several forward-pool workers read one shared buffer. It must be read-only for all of them, and freed only after the last write completes |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Two destinations receive the same bytes when they should differ | STOP. This is the catastrophic case. Do not tune, find the missing equality field |
| A poison read or a double return appears | STOP. Fall back to sharing only the body, per A-4 |
| Lint failure | Fix inline; if architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The benchmark shows no win | Drop this child. Children 1 to 4 stand alone, and A-1 says so in advance |
| Interop peer receives another peer's route | STOP and present immediately. This is a correctness incident, not a test failure |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The existing cache is not too small, it is too late. Keying on the materialised pointer means the copy has already been paid by the time the cache can answer. Moving the key upstream is the whole change.
- Fingerprinting is cheap relative to what it replaces only because the edit set is small and the payload is not. That ratio is what A-1 measures, and it is the reason this child follows the edit-set work rather than preceding it.
- The dangerous half of this child is not the hashing, it is the equality. A hash is an index; the comparison is the authorisation. Keeping those two roles distinct in the code is what makes a collision harmless.
- Deriving both the fingerprint and the equality check from one explicit field list is what stops the two drifting apart when a field is added later. A field added to one and not the other is the failure mode that a future change would otherwise introduce silently.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fingerprint the edit set | keep keying on the materialised wire pointer | The pointer key can only dedupe destinations that already produced the same pointer, which is after the copy it exists to avoid |
| The fingerprint is a hint, confirmed by full equality | trust the hash | A collision would send one destination another destination's wire, silently and undetectably from the peer's side |
| One field list feeding both the hash and the comparison | write them independently | Independent implementations drift, and the drift is silent and dangerous |
| Reuse the existing update-groups gate | add a new configuration leaf | The feature already has an on-off switch and an existing meaning of "these peers are alike" |
| One dedup implementation for both rails | keep the map on one rail and the four-slot array on the other | Two implementations of the same idea is what let the route-server rail cap silently at four |
| Empty edit set short-circuits | fingerprint everything uniformly | The zero-copy passthrough must stay free, and there is nothing to share when nothing is edited |

## Known Limitations

- Dedup is per forward call. A route fanned out in two separate calls materialises twice, which matches the current cache's lifetime.
- The route-server rail's current four-slot cache silently stops caching beyond four bodies. This child replaces it, and the replacement must log what it drops rather than capping silently, per the umbrella's no-silent-caps rule.
- The win does not appear in the existing single-peer convergence baseline. The fan-out benchmark is the proof, and A-1 is the gate that decides whether this child lands at all.
- Sharing applies to the materialised body. Splitting still runs per destination, so an oversized UPDATE shared by many peers still splits many times.
- This child assumes children 1 to 4 have landed. On its own it has nothing to fingerprint.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.

| RFC | Section | Requirement | Site after this child |
|-----|---------|-------------|----------------------|
| 4271 | 4.3 | each peer receives the UPDATE its own policy produced | the equality check in `internal/component/bgp/filterapi/fingerprint.go` |
| 4456 | 8 | ORIGINATOR_ID and CLUSTER_LIST are per-cluster, so cross-cluster sharing is forbidden | the cluster separation test and the equality fields |
| 7947 | 2.2.2 | route-server clients differ in AS_PATH treatment and community stripping | the equality fields covering the AS-path intent and community slots |
| 8654 | - | the extended-message flag changes downstream splitting and is part of the identity | the identity in `internal/component/bgp/reactor/forward_dedup.go` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate; `ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-wire-edit-5-fanout-dedup.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-wire-edit-5-fanout-dedup.md` only (commit A preserves the spec in history)
