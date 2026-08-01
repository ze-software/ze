# Spec: hotpath-alloc-round-4 -- Independently-fixable findings from the wire-edit research

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/spec-hotpath-alloc-round-4.md` |
| Updated | 2026-08-01 |

→ Decision (2026-08-01, Thomas): design approved for implementation. Asked where
to start `plan/spec-wire-edit-0-umbrella.md`; he chose this spec's Tier 1 first,
which is the ordering the Task section above already declares load-bearing.
→ Decision (2026-08-01, Thomas): implementation runs on Opus 5 by explicit
operator override of `ai/rules/model-selection.md`, recorded in
`tmp/session/.model-ack-2546e79c-8d57-4803-b856-593a4da12c55`. An independent
review pass is still owed, and this session is not it.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The research behind `plan/spec-wire-edit-0-umbrella.md` produced a list of
concrete defects and costs on the BGP forward path, each verified against its
producing function. Most of them disappear when that umbrella lands, but the
umbrella is a multi-child structural change and some of these items should not
wait for it.

This spec separates the list into two tiers and fixes only the first:

- **Tier 1, fix now.** Items that are correctness problems, or that are small,
  independent, and not made redundant by the umbrella.
- **Tier 2, recorded and deliberately not fixed here.** Items the umbrella
  removes by construction. Fixing them twice wastes the work and creates a merge
  conflict with a spec that rewrites the same functions. They are recorded in
  full so a future session does not re-derive them, following the negative-finding
  precedent set by `plan/spec-perf-next-0-umbrella.md`.

The one live **bug** found in the same research (route-server control communities
leaking when a route carries two or more) is not in either tier: it had its own
spec, `plan/spec-fixit-rs-community-strip-arity.md`, because it is a route leak
rather than a cost. **It shipped on 2026-07-28 in commit `4730deb84`**
(`plan/learned/1280-fixit-rs-community-strip-arity.md`). That commit moved lines in
six of the files this spec cites; the citations here were re-pointed the same day.
A session picking this spec up later must re-verify rather than trust them.

**Ordering (load-bearing).** Tier 1 lands BEFORE `plan/spec-wire-edit-2-edit-apply.md`,
which consumes T1-1 and T1-3 as preconditions (its A-7). T1-3 in particular cannot be
deferred into that child: child 2 grows the accumulator from roughly 150 to roughly 500
inline bytes, so hoisting it out of the destination loop must already be true or child 2
regresses the loop it exists to speed up, and its benchmark baseline mis-attributes the
hoist's win to the fragment model.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs
- [ ] `ai/rules/buffer-first.md` - all wire encoding writes into pooled bounded buffers
  → Constraint: `make([]byte, N)` inside a helper is banned; the buffer comes from a pool or from the caller. Tier 1 item T1-2 is a direct violation.
  → Decision: pool `New` funcs, session buffer creation, cached encoding and result copies to callers are the sanctioned exceptions.
- [ ] `ai/rules/fail-closed-guards.md` - a guard must fail closed or say something
  → Constraint: returning a value the caller reads as "nothing to do" after detecting a failure is the fail-open shape this rule forbids. Tier 1 item T1-1 is exactly that.
- [ ] `plan/spec-perf-next-0-umbrella.md` - the third optimisation round, its methodology and its negative findings
  → Constraint: "Profile before coding" is a blocking gate, not a formality. Campaign 771 rejected three plausible proposals after profiling. The same gate applies to every Tier 1 performance item here (T1-2, T1-3, T1-4).
  → Decision: the umbrella already records `forward_build.go` pool-fallback `make` sites as deliberate design; do not re-open them.
- [ ] `plan/learned/771-performance-optimization-campaign.md` and `plan/learned/859-perf-hot-alloc-reduction.md` - the two prior campaigns
  → Constraint: the remaining gap to BIRD is attributed to architecture (Go garbage collection versus slab allocation, buffered versus in-place parsing, socket-layer write coalescing), not to remaining low-hanging fruit. Do not promise convergence movement from these items.
- [ ] `ai/rules/api-contracts.md` - caller obligations belong in the function comment
  → Constraint: `ModAccumulator.Reset` exists and has no production caller; either it is wired or it is dead code and the dead-code rule applies.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - UPDATE format and attribute ordering
  → Constraint: Section 5 orders attribute type codes ascending on emission. The ordering divergence recorded as T2-7 is a real inconsistency, not a stylistic one.
- [ ] `rfc/short/rfc6793.md` - four-octet ASN handling
  → Constraint: the double attribute scan recorded as T2-3 exists because the slow path must locate AS4_PATH, AGGREGATOR and AS4_AGGREGATOR after deciding the fast path cannot apply.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH
  → Constraint: the cross-context re-encode in T1-2 exists because NLRI framing differs per family and direction, so the allocation cannot simply be removed; it must be pooled.

**Key insights:** (minimal context to resume after compaction)
- T1-1 is a correctness defect, not a cost: an oversize modification currently forwards the route unmodified.
- T1-2 is a straightforward rule violation with a pool already available next to it.
- T1-3 and T1-4 are small and local, and survive the umbrella.
- Everything in Tier 2 is deleted or rewritten by `plan/spec-wire-edit-0-umbrella.md`. Recording is the deliverable, not fixing.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/forward_build.go:58` - `buildModifiedPayload`: sizes the output at `len(payload) + 256` at `:104`; on overflow sets an overflow flag at `:236-239`, releases the buffers and returns nil.
- [ ] `internal/component/bgp/reactor/forward_build.go:263-267` - a second late abort: after all attributes are written, an attribute-length outside 0 to 65535 also releases and returns nil.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:790-798` - the caller: `if modified, bufIdx := buildModifiedPayload(...); modified != nil { ... }` at `:793`. A nil result leaves `peerWire` at its pre-modification value and the route is dispatched.
- [ ] `internal/component/bgp/reactor/filter_ordered.go:208` - the ingress caller falls through here to `return ingressStepResult{accept: true}` with no modified payload; the call it fell out of is `buildModifiedPayload` at `internal/component/bgp/reactor/filter_ordered.go:203`.
- [ ] `internal/component/bgp/reactor/filter_ordered.go:308` - the egress caller falls through here to `return egressStepResult{accept: true}` with no wire override; the call it fell out of is `buildModifiedPayload` at `internal/component/bgp/reactor/filter_ordered.go:303`.
- [ ] `internal/component/bgp/reactor/forward_body.go:158` - `fwdUpdateForDestination`: `buf := make([]byte, len(payload)*2+1024)` for the ASN4 transcode on every cross-context forward, unpooled.
- [ ] `internal/component/bgp/reactor/forward_build.go:392` - `acquireModBuf`: the tiered per-peer pool, then `modBufPool`, then a bare `make`, each site commented `// pool-fallback`. This is the sanctioned pattern the transcode buffer should follow.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:604` and `:632` - the destination loop, with `var mods filterapi.ModAccumulator` declared **inside** it.
- [ ] `internal/component/bgp/reactor/forward_rs.go:313` and `:339` - the route-server rail, with the same per-iteration declaration.
- [ ] `internal/component/bgp/filterapi/filterapi.go:94` - `modAccumulatorInlineBytes` is 64; the accumulator embeds `inline [64]byte`.
- [ ] `internal/component/bgp/filterapi/filterapi.go:119` - `Reset` clears ops, flags, both rewrites and the inline offset. Its only reference in the tree is `internal/component/bgp/filterapi/filterapi_test.go:239`; no production caller exists.
- [ ] `internal/component/bgp/reactor/session_validation.go:108` - `ValidateUpdateRFC7606AddPath` walks the whole attribute section on every received UPDATE.
- [ ] `internal/component/bgp/reactor/session_validation.go:112` - `attribute.AttrFind(pathAttrs, AttrPrefixSID)` walks the same bytes again, immediately after, on every eBGP UPDATE where PrefixSID acceptance is not configured.
- [ ] `internal/component/bgp/wireu/community.go:51` and `:158` - `ParseCommunityPolicy` and `StripControlCommunities`: two complete independent walks of the same attribute section.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go:614-616` - both are called back to back, once per UPDATE, on the route-server path.
- [ ] `internal/component/bgp/reactor/forward_build.go:538` - `groupOpsByCode` returns `[256][]filterapi.AttrOp` by value, 6144 bytes, and heap-allocates one slice per touched code at `:546`.
- [ ] `internal/component/bgp/reactor/forward_build.go:226-230` - untouched attributes are copied one at a time, never coalesced into runs.
- [ ] `internal/component/bgp/wireu/aspath_rewrite.go:95-127` then `:155-195` - two full scans of the attribute section on the slow path.
- [ ] `internal/core/bgp/attribute/wire.go:35`, `:64`, `:133`, `:182` - `sync.RWMutex` taken on every `Get`, `Has` and `GetRaw`, including on single-goroutine read paths.
- [ ] `internal/component/bgp/plugins/filter_community/handler.go:69-70`, `:107`, `:112`, `:165-172` - three allocation families in one handler: a copy of the whole list, an allocating removal loop, and append growth. Line numbers re-pointed 2026-07-28: commit `4730deb84` (`plan/learned/1280-fixit-rs-community-strip-arity.md`) rewrote this handler's Remove branch. It changed the arity contract and added a fail-closed refusal; it did **not** remove any of the three allocation families, so the finding stands.
- [ ] `internal/component/bgp/reactor/forward_build.go:242` - new attributes are appended after all source attributes.
- [ ] `internal/component/bgp/reactor/reactor_api_batch_attr_order_test.go:20-47` - records that the two announce rails once produced different byte strings for one route, chosen by scheduling, and pins ascending type-code order plus rail agreement.

**Behavior to preserve:**
- Every wire byte emitted today for every transform. None of the Tier 1 items may change output bytes, with the single deliberate exception in T1-1 where a route that is currently forwarded unmodified is instead suppressed.
- The tiered buffer acquisition in `acquireModBuf` (`forward_build.go:392`), including its deliberate `make` fallback for oversized payloads, which `plan/spec-perf-next-0-umbrella.md` records as design.
- The buffer lifetime contracts in `docs/architecture/memory/lifetime-contracts.md`; a pooled transcode buffer must have exactly one return point.
- Every existing `.ci` under `test/plugin/`, `test/policy/`, `test/encode/`.

**Behavior to change:**
- An oversize or otherwise unapplicable modification suppresses the route for that destination and logs, instead of forwarding it unmodified.
- The cross-context transcode buffer comes from a pool instead of a fresh allocation.
- The modification accumulator is hoisted above the destination loop and reset per iteration, on both rails.
- The PrefixSID lookup reuses the attribute walk that immediately precedes it.

## Data Flow (MANDATORY)

### Entry Point
- Peer-received UPDATE: framed by the session read loop, validated by `enforceRFC7606` (`session_validation.go:38`), cached, then fanned out through the destination loop.
- Cross-context forward: a destination whose encoding context differs from the source, which forces the re-encode path in `fwdUpdateForDestination` (`forward_body.go:136`).

### Transformation Path
1. Receive and validate. `ValidateUpdateRFC7606AddPath` walks the attribute section (`session_validation.go:108`), then `AttrFind` walks it again for PrefixSID (`:112`). **T1-4** removes the second walk.
2. Per-destination loop entry. A fresh accumulator value is created per iteration (`reactor_api_forward.go:632`, `forward_rs.go:339`). **T1-3** hoists and resets it instead.
3. Filters and reactor rules append modification operations to the accumulator.
4. Cross-context destinations re-encode. `fwdUpdateForDestination` allocates a fresh transcode buffer (`forward_body.go:158`). **T1-2** takes it from a pool.
5. `buildModifiedPayload` applies the operations. On overflow it returns nil and the caller forwards the unmodified route (`forward_build.go:236-239`, `reactor_api_forward.go:785`). **T1-1** suppresses and logs instead.
6. Dispatch to the forward pool and write to TCP.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor to buffer pools | `acquireModBuf` tiering and the read-buffer multiplexers; a pooled transcode buffer joins that discipline | No |
| Reactor to registered attribute handler | unchanged by this spec; the handler contract belongs to `plan/spec-wire-edit-0-umbrella.md` | No |
| Forward loop to forward-pool worker | unchanged; suppression happens before dispatch | No |
| Engine to operator | a new warning and a new counter on the suppression path | No |

### Integration Points
- `acquireModBuf` (`forward_build.go:392`): the existing tiered acquisition the transcode buffer should reuse rather than duplicate.
- `filterapi.ModAccumulator.Reset` (`filterapi.go:119`): already written, currently unreachable from production code; T1-3 is what makes it reachable.
- `enforceRFC7606` (`session_validation.go:38`): owns the single receive-time attribute walk that T1-4 extends.
- The suppression counters already used on the forward path for policy decisions, so a new refusal counter sits alongside them rather than inventing a surface.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, handlers register and the core discovers them; no per-feature field, switch case, or factory added to a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Tier 1: fix in this spec

| ID | Finding | Producer | Kind | Why it does not wait for the umbrella |
|----|---------|----------|------|--------------------------------------|
| T1-1 | An oversize modification abandons every modification, returns nil, and the caller forwards the route **unmodified**. A next-hop-self, a community strip or a private-ASN removal that does not fit is silently not applied. | `forward_build.go:104`, `:236-239`, `:263-267`; callers `reactor_api_forward.go:785`, `filter_ordered.go:203`, `filter_ordered.go:303` | correctness, fail-open | It is a policy failure that can leak whatever the policy exists to strip. The umbrella removes the branch by construction, but that is weeks away. |
| T1-2 | The cross-context ASN4 transcode allocates `len(payload)*2+1024` bytes per forward, unpooled, in a helper. | `forward_body.go:158` | rule violation | `ai/rules/buffer-first.md` bans it, the pooled alternative sits 200 lines away in the same package (`forward_build.go:392`), and the umbrella rewrites the transcode but not this call site's buffer discipline. |
| T1-3 | The modification accumulator is declared inside the destination loop on both rails, so its 64-byte inline arena is re-zeroed once per destination. `Reset` exists for exactly this and has no production caller. | `reactor_api_forward.go:632`, `forward_rs.go:339`, `filterapi.go:94`, `:119` | cost, dead code | The umbrella makes the accumulator larger, which makes per-iteration zeroing worse, so this must be true **before** the umbrella lands or the umbrella regresses the hot path. |
| T1-4 | The PrefixSID lookup walks the attribute section a second time, immediately after the RFC 7606 validator walked it. | `session_validation.go:108` then `:112` | cost | Small, local, and independent of the representation change. |

### T1-1 in detail

Three exit paths in `buildModifiedPayload` return nil after work has begun:
buffer overflow while copying (`:236-239`), an attribute-length result outside
the uint16 range (`:263-267`), and a withdrawn-rewrite longer than 65535
(`:139-143`). All three are indistinguishable at the call site from the
legitimate "no modifications were needed" nil at `:74-76`.

The fix separates the two answers. "Nothing to do" and "I could not do it" must
not share a return value. The caller must suppress the route for that destination
on the second, exactly as it already does for a policy reject, and log a message
naming the peer, the attribute count and the required size.

Note the interaction with `egressStepResult.failed` (`filter_ordered.go:70-77`),
which already exists to distinguish "this step could not run" from "this step
decided to drop the route". T1-1 is the same distinction one layer down, and
should reuse that vocabulary rather than invent a second one.

### T1-3 in detail

Hoisting is not sufficient on its own. `Reset` must clear only the used prefixes
(the operations slice length, the inline offset, the flags and the two rewrites)
and must never re-zero the inline array, or hoisting buys nothing. Read
`filterapi.go:119-125` before changing it: it already has this shape, which is
why T1-3 is a wiring change rather than a rewrite.

## Tier 2: recorded, deliberately not fixed here

Do not implement these in this spec. Each is removed by construction in
`plan/spec-wire-edit-0-umbrella.md`, and fixing them twice creates a conflict
with a spec that rewrites the same functions.

| ID | Finding | Producer | Removed by |
|----|---------|----------|-----------|
| T2-1 | `groupOpsByCode` returns a 256-entry array of slices **by value** (6144 bytes zeroed and copied per modified UPDATE per destination) and heap-allocates one slice per touched attribute code. | `forward_build.go:538`, `:546`; consumed at `:199` and `:243` | umbrella child 2: a flat slot array with a fragment chain has neither the array nor the per-code allocation |
| T2-2 | Untouched attributes are copied one at a time; adjacent untouched runs are never coalesced, so a twelve-attribute UPDATE with one edit performs twelve small copies. | `forward_build.go:226-230` | umbrella child 2: run coalescing is free once a shared span index and a touched bitset exist |
| T2-3 | `rewriteASPathPrepend` scans the whole attribute section twice on the slow path. | `wireu/aspath_rewrite.go:95-127` then `:155-195` | umbrella child 3: the AS-path resolver consumes the shared index |
| T2-4 | `ParseCommunityPolicy` and `StripControlCommunities` each perform a complete independent walk of the same bytes, called back to back. | `wireu/community.go:51`, `:141`; call site `reactor_api_forward.go:614-616` | umbrella child 1: both consume the shared span index |
| T2-5 | `AttributesWire` takes a read-write mutex on every `Get`, `Has` and `GetRaw`, including on single-goroutine read paths. The mutex exists only to guard the lazy index build. | `attribute/wire.go:35`, `:64`, `:133`, `:182`, build at `:291` | umbrella child 1: an eagerly built index makes the object immutable, so the mutex has nothing to guard |
| T2-6 | `genericCommunityHandler` allocates three times per attribute per destination: a copy of the whole list, an allocating removal loop, and append growth. Survived commit `4730deb84`, which rewrote the Remove branch for arity without touching the allocations. | `plugins/filter_community/handler.go:69-70`, `:107`, `:112`, `:165-172` | umbrella child 2: a subset removal becomes base fragments around the holes, with no intermediate buffer |
| T2-7 | New attributes are appended after all source attributes on the forward-modify path, while both announce rails are pinned to ascending type-code order. One route can therefore reach the wire in two different byte orders depending on which path built it. | `forward_build.go:242` versus `reactor_api_batch_attr_order_test.go:20-47` | umbrella child 2: merge-insert places a new attribute at its ascending position |

**Do not re-investigate without new evidence.** If a fresh profile puts one of
these frames at the top before the umbrella lands, that is new evidence: bring it
to the user rather than implementing it here, because the umbrella's child order
may need to change instead.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The three nil-returning failure paths in `buildModifiedPayload` are reachable in production, not merely defensive. | `len(payload)+256` slack (`forward_build.go:104`) against modifications that can grow an attribute arbitrarily, for example a CLUSTER_LIST prepend on a long list or a community add on a near-full list. | If unreachable, T1-1 becomes documentation plus a counter that never fires, which is still worth having but is not urgent. | Land the counter first and observe it in a soak run before changing the behaviour. | unvalidated |
| A-2 | No caller of `buildModifiedPayload` depends on nil meaning "forward unmodified". | The three call sites read nil identically as "skip the modification": `reactor_api_forward.go:785`, `filter_ordered.go:203`, `filter_ordered.go:303`. | A caller that genuinely wants the unmodified route needs an explicit signal rather than a shared nil. | Read all three call sites at implementation and add a test per site. | unvalidated |
| A-3 | The cross-context transcode buffer fits the existing tiered acquisition without a new pool. | `acquireModBuf` (`forward_build.go:392`) already handles sizes above 4096 by falling through to a fresh allocation, which is the same worst case as today. | A new size class is needed; the change grows but stays local. | Benchmark the cross-context forward path before and after, at both standard and extended message sizes. | unvalidated |
| A-4 | Hoisting the accumulator is safe: no operation buffer captured in one iteration is read in a later one. | `Op` stores the caller's slice without copying (`filterapi.go:132-134`) and `OpCopy` copies into the inline arena (`:139`). Both are consumed within the same iteration by `buildModifiedPayload`. | A retained slice would be read after reset, returning another destination's bytes. This is the one way T1-3 can be subtly wrong. | A test that fills the accumulator, resets, refills with fewer operations, and asserts no stale operation survives; plus a debug-build poison of the inline arena on reset. | unvalidated |
| A-5 | The PrefixSID lookup can be folded into the RFC 7606 validator walk without changing the validator's result. | Both read the same `pathAttrs` slice; the PrefixSID check is a presence test with no effect on the validation verdict (`session_validation.go:110-125`). | Keep the second walk; T1-4 drops out with no effect on the rest. | Differential test over the existing RFC 7606 corpus asserting an identical verdict for every input. | unvalidated |
| A-6 | None of the Tier 1 items changes emitted wire bytes, except the deliberate suppression in T1-1. | T1-2 changes only where the buffer comes from, T1-3 only when the value is zeroed, T1-4 only how many times bytes are read. | A byte difference means the change was not what it looked like; stop and re-read. | A golden byte-comparison over a corpus of received UPDATEs times the transform matrix, run before and after each item. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | T1-1 converts a silent leak into visible route drops, surfacing latent oversize cases as a new operational symptom. | The new counter climbs in soak before any behaviour change lands. | Land the counter first, in its own commit, and only change the behaviour once the frequency is known. This is the deliberate ordering in the Implementation Steps. |
| R-2 | A pooled transcode buffer introduces a lifetime bug: the buffer is aliased into a `message.Update` that outlives the call. | Debug-build poison reads under `memguard`. | Follow `acquireModBuf`'s existing contract exactly, including a single return point, and add a debug-tagged test that poisons on return. |
| R-3 | Hoisting the accumulator leaks state between destinations, which produces wrong per-peer wire that unit tests on a single destination cannot see. | A multi-destination golden test diverges on the second destination onward. | A-4's test plus a functional test with three destinations having three different policies. |
| R-4 | This spec and `plan/spec-wire-edit-0-umbrella.md` touch the same functions, so a long-lived branch conflicts. | Merge conflict in `forward_build.go` or `forward_body.go`. | Tier 1 lands first and completely, before umbrella child 2 starts. Tier 2 is the explicit non-overlap boundary. |
| R-5 | Profiling shows none of the Tier 1 performance items near the top, making the work unjustified. | The captured profile itself. | T1-1 is a correctness fix and proceeds regardless. T1-2 is a rule violation and proceeds regardless. T1-3 and T1-4 are dropped if the profile does not support them, and that outcome is recorded here rather than retried. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | T1-1 wrong in the unsafe direction drops routes that should have been forwarded. T1-3 wrong leaks modification state between destination peers, sending one peer another peer's attributes. T1-2 wrong is a use-after-free of a pooled buffer, which reads recycled bytes onto the wire. |
| How is it reverted? | Each item is a separate commit and reverts independently. No configuration, schema or persisted state changes. |
| Who else touches this path? | `plan/spec-wire-edit-0-umbrella.md` rewrites the same functions and must rebase on this spec; its child `plan/spec-wire-edit-2-edit-apply.md` consumes T1-1 and T1-3 as preconditions (its A-7) and must not re-implement them. `plan/spec-perf-next-2-filter-delta-alloc.md` has a deferred Phase B on the same encoders, which belongs to umbrella child 2, not here. `plan/spec-fixit-rs-community-strip-arity.md` already edited `plugins/filter_community/handler.go` in commit `4730deb84`; this spec still does not touch that file (T2-6 is recorded, not fixed). |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer receives a route whose configured modification cannot fit the output buffer | → | `buildModifiedPayload` reports failure, the caller suppresses and logs | `test/plugin/modify-oversize-suppress.ci` |
| A route is forwarded to a peer with a different ASN4 capability | → | `fwdUpdateForDestination` takes a pooled transcode buffer | `test/plugin/asn4-transcode-pooled-buffer.ci` |
| One route is forwarded to three peers with three different policies | → | the hoisted accumulator is reset between destinations | `test/plugin/modify-accumulator-per-peer-isolation.ci` |
| Any received UPDATE carrying PrefixSID from an eBGP peer | → | single-walk PrefixSID detection in `enforceRFC7606` | existing `test/plugin/bgp-prefix-sid-ebgp-discard.ci` if present, otherwise `test/plugin/prefixsid-ebgp-discard-single-walk.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A modification whose result exceeds the available buffer | The route is suppressed for that destination and a warning names the peer, the attribute count and the required size. The route is never forwarded unmodified |
| AC-2 | The same condition, counted | `bgp_update_modify_failed_total` increments, labelled by the reason (overflow, attribute-length range, withdrawn-rewrite size) |
| AC-3 | A modification that legitimately needs no changes | Behaviour is unchanged: the route is forwarded as-is, and no counter increments |
| AC-4 | A cross-context forward at both standard and extended message sizes | The transcode buffer comes from a pool, is returned exactly once, and the emitted bytes are identical to before |
| AC-5 | The same, under a debug build with buffer poisoning enabled | No poison read occurs, proving the buffer is not aliased past its return |
| AC-6 | One route forwarded to three destinations with distinct modification sets | Each destination receives exactly its own modifications; no operation from one destination appears in another |
| AC-7 | The accumulator after reset | No operation, no rewrite and no arena byte from the previous destination is readable |
| AC-8 | A received UPDATE from an eBGP peer carrying PrefixSID, with acceptance not configured | The attribute is discarded exactly as today, with one attribute-section walk instead of two |
| AC-9 | The full RFC 7606 validation corpus | The verdict for every input is identical before and after T1-4 |
| AC-10 | A golden corpus of received UPDATEs times the transform matrix | Emitted bytes are identical before and after every Tier 1 item, except the deliberate suppression in AC-1 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures an export policy that strips private ASNs on a route whose modification would not fit | wire, filter chain, modification apply fails, route suppressed and logged | `test/plugin/modify-oversize-suppress.ci` |
| 2 | Peers with both a four-octet and a two-octet ASN speaker and forwards between them | wire, cross-context re-encode with a pooled buffer, wire | `test/plugin/asn4-transcode-pooled-buffer.ci` |
| 3 | Runs three peers with three different export policies against one route | wire, per-destination accumulator reset, three distinct wires | `test/plugin/modify-accumulator-per-peer-isolation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBuildModifiedPayloadOverflowReportsFailure` | `internal/component/bgp/reactor/forward_build_test.go` | AC-1: overflow is distinguishable from "no modifications needed" at the call site | |
| `TestBuildModifiedPayloadNoModsIsNotFailure` | `internal/component/bgp/reactor/forward_build_test.go` | AC-3: the legitimate empty case is unchanged | |
| `TestForwardSuppressesOnModifyFailure` | `internal/component/bgp/reactor/forward_update_test.go` | AC-1: the caller suppresses rather than dispatching | |
| `TestIngressAndEgressChainSuppressOnModifyFailure` | `internal/component/bgp/reactor/filter_ordered_test.go` | A-2: all three call sites handle the failure, not only the forward one | |
| `TestTranscodeBufferPooled` | `internal/component/bgp/reactor/forward_body_test.go` | AC-4: no allocation on the cross-context path after the first | |
| `TestTranscodeBufferSingleReturn` | `internal/component/bgp/reactor/forward_body_test.go` | AC-5: exactly one return, verified under the debug poison build | |
| `TestAccumulatorResetClearsEverything` | `internal/component/bgp/filterapi/filterapi_test.go` | AC-7, A-4: no operation, rewrite or arena byte survives a reset | |
| `TestAccumulatorResetIsConstantTime` | `internal/component/bgp/filterapi/filterapi_test.go` | reset cost does not scale with the inline capacity, so the umbrella's larger value stays safe | |
| `TestPerDestinationModificationIsolation` | `internal/component/bgp/reactor/forward_update_test.go` | AC-6: three destinations, three distinct modification sets | |
| `TestPrefixSIDSingleWalkSameVerdict` | `internal/component/bgp/reactor/session_validation_test.go` | AC-8, AC-9: identical verdict, one walk | |
| `TestGoldenBytesUnchangedTier1` | `internal/component/bgp/reactor/forward_build_test.go` | AC-10: byte identity across the transform matrix | |
| `BenchmarkForwardCrossContext` | `internal/component/bgp/reactor/forward_update_bench_test.go` | T1-2 before and after, allocations per operation | |
| `BenchmarkForwardPerDestinationLoop` | `internal/component/bgp/reactor/forward_update_bench_test.go` | T1-3 before and after, at fan-out 1, 10 and 100 | |
| `BenchmarkReceiveValidation` | `internal/component/bgp/reactor/hotpath_bench_test.go` | T1-4 before and after | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| modified attribute-section length | 0-65535 | 65535 | N/A | 65536 (suppress and log) |
| withdrawn-rewrite length | 0-65535 | 65535 | N/A | 65536 (suppress and log) |
| output buffer requirement versus capacity | 0-capacity | exactly capacity | N/A | capacity plus one (suppress and log) |
| transcode buffer size | 4-131056 | 131056 (twice the extended body plus slack) | 3 | above the pool class, falls through to a fresh allocation as `acquireModBuf` already does |
| accumulator inline arena | 0-64 | 64 | N/A | 65 (heap spill, unchanged behaviour) |
| destinations per forward | 1-n | n | 0 (no dispatch) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `modify-oversize-suppress` | `test/plugin/modify-oversize-suppress.ci` | a policy modification that cannot fit suppresses the route instead of leaking it unmodified | |
| `asn4-transcode-pooled-buffer` | `test/plugin/asn4-transcode-pooled-buffer.ci` | forwarding between a four-octet and a two-octet speaker produces identical wire with a pooled buffer | |
| `modify-accumulator-per-peer-isolation` | `test/plugin/modify-accumulator-per-peer-isolation.ci` | three peers with three policies each receive only their own modifications | |
| `prefixsid-ebgp-discard-single-walk` | `test/plugin/prefixsid-ebgp-discard-single-walk.ci` | an eBGP route carrying PrefixSID is discarded exactly as before | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-asn4-transcode-pooled` | `test/interop/scenarios/` | FRR | a two-octet-ASN speaker accepts the transcoded UPDATE produced from a pooled buffer, proving no recycled bytes reach the wire | |

## Files to Modify
- `internal/component/bgp/reactor/forward_build.go` - distinguish "could not modify" from "nothing to modify" (T1-1)
- `internal/component/bgp/reactor/reactor_api_forward.go` - suppress on modify failure; hoist the accumulator above the destination loop (T1-1, T1-3)
- `internal/component/bgp/reactor/filter_ordered.go` - suppress on modify failure at both the ingress and egress call sites, reusing the existing `failed` vocabulary (T1-1)
- `internal/component/bgp/reactor/forward_rs.go` - hoist the accumulator on the route-server rail (T1-3)
- `internal/component/bgp/reactor/forward_body.go` - take the transcode buffer from a pool (T1-2)
- `internal/component/bgp/reactor/session_validation.go` - fold the PrefixSID lookup into the existing validation walk (T1-4)
- `internal/component/bgp/filterapi/filterapi.go` - document that `Reset` is the per-destination entry point and that operation buffers must not be retained across it (T1-3)
- `docs/architecture/core-design.md` - record the modify-failure semantics in the modification-accumulator section
- `docs/plugin-development/metrics.md` - document the new counter

## Files to Create
- `test/plugin/modify-oversize-suppress.ci` - T1-1 end to end
- `test/plugin/asn4-transcode-pooled-buffer.ci` - T1-2 end to end
- `test/plugin/modify-accumulator-per-peer-isolation.ci` - T1-3 end to end
- `test/plugin/prefixsid-ebgp-discard-single-walk.ci` - T1-4 end to end

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No configuration surface changes |
| YANG validation constraints | N-A | No new leaves |
| YANG custom validators | N-A | No new leaves |
| CLI commands/flags | No | No new commands |
| CLI grammar (keyword before value) | N-A | No new commands |
| Editor autocomplete | N-A | No new leaves |
| Functional test for new RPC/API | No | No new RPC; the four `.ci` files cover the behaviour |
| Pipe completeness | N-A | No new command output |
| Env var registration | No | No new environment leaves |
| Doctor check for runtime dependencies | No | No new file path, socket, service, port or binary |
| Prometheus counters/metrics | Yes | `bgp_update_modify_failed_total`, labelled by reason (`overflow`, `attr-length-range`, `withdrawn-size`), incremented on every T1-1 suppression |
| BGP family surface (new SAFI / capability / attribute) | No | No new SAFI, capability or attribute code |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Correctness and cost work on existing paths |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | The handler contract is untouched here; that belongs to `plan/spec-wire-edit-0-umbrella.md` |
| 6 | Has a user guide page? | No | The suppression is an internal failure mode surfaced through logs and a counter |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC behaviour changes; T1-4 changes how many times bytes are read, not the verdict |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: the modification accumulator is per destination and reset between destinations, and a failed modification suppresses |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` for `bgp_update_modify_failed_total` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors naming `forward_build.go`, `forward_body.go`, `filterapi.go` and `session_validation.go` and correct any stale claim |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No user-facing syntax in this area |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- profile, then land observability before any behaviour change
   - Tests: `TestGoldenBytesUnchangedTier1` pinning current output; the new counter registered and asserted present
   - Files: `reactor/forward_build.go` (counter increment on each nil-returning failure path, behaviour unchanged), metrics registration
   - Verify: `make ze-perf-bench PERF_DUT=ze PPROF=1` captured into `tmp/perf-run/pprof` per `plan/spec-perf-next-0-umbrella.md`; for T1-3 and T1-4, locate their frames in the profile and record the result. If a frame is absent near the top, STOP and present the evidence before implementing that item. The counter runs in soak so A-1 is answered with numbers, not a guess
2. **Phase: T1-1, distinguish failure from no-op**
   - Tests: `TestBuildModifiedPayloadOverflowReportsFailure`, `TestBuildModifiedPayloadNoModsIsNotFailure`, `TestForwardSuppressesOnModifyFailure`, `TestIngressAndEgressChainSuppressOnModifyFailure`, `test/plugin/modify-oversize-suppress.ci`
   - Files: `reactor/forward_build.go`, `reactor/reactor_api_forward.go`, `reactor/filter_ordered.go`, `docs/architecture/core-design.md`
   - Verify: the `.ci` fails before the change with the unmodified route on the wire; all three call sites suppress; the golden corpus is otherwise unchanged
3. **Phase: T1-2, pool the transcode buffer**
   - Tests: `TestTranscodeBufferPooled`, `TestTranscodeBufferSingleReturn`, `BenchmarkForwardCrossContext`, `test/plugin/asn4-transcode-pooled-buffer.ci`
   - Files: `reactor/forward_body.go`
   - Verify: allocations per cross-context forward drop to zero after warm-up; no poison read under a debug build; emitted bytes identical
4. **Phase: T1-3, hoist and reset the accumulator on both rails**
   - Tests: `TestAccumulatorResetClearsEverything`, `TestAccumulatorResetIsConstantTime`, `TestPerDestinationModificationIsolation`, `BenchmarkForwardPerDestinationLoop`, `test/plugin/modify-accumulator-per-peer-isolation.ci`
   - Files: `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`, `filterapi/filterapi.go`
   - Verify: both rails hoisted; three-destination isolation holds; A-4 confirmed by the reset test plus debug-build arena poisoning
5. **Phase: T1-4, single receive-time walk**
   - Tests: `TestPrefixSIDSingleWalkSameVerdict`, `BenchmarkReceiveValidation`, `test/plugin/prefixsid-ebgp-discard-single-walk.ci`
   - Files: `reactor/session_validation.go`
   - Verify: identical verdict over the full RFC 7606 corpus; one walk instead of two
6. **Phase: hand off Tier 2** -- confirm the boundary rather than implement it
   - Tests: none; this phase produces a record
   - Files: `plan/spec-wire-edit-0-umbrella.md` cross-reference check
   - Verify: every T2 row is claimed by a named umbrella child, and none of them was touched by phases 2 to 5. `git diff --stat` must show no change to `wireu/aspath_rewrite.go`, `wireu/community.go`, `attribute/wire.go` or `plugins/filter_community/handler.go`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and every Tier 1 item has both a unit test and a `.ci` |
| Feature completeness | The three modify-failure call sites are all handled, not only the forward one; both rails are hoisted, not only the general one |
| Correctness | "Could not modify" and "nothing to modify" are distinct at every call site; a hoisted accumulator carries nothing between destinations; the pooled buffer has exactly one return point |
| Naming | The counter reason labels are stable, lower-case and hyphenated; the warning names the peer, the attribute count and the required size, per `ai/rules/error-messages.md` |
| Data flow | T1-4 does not change the RFC 7606 verdict for any input; T1-2 does not change emitted bytes; T1-3 does not change per-destination output |
| Rule: `ai/rules/buffer-first.md` | No bare `make` remains in an encoding helper on this path, other than the sanctioned pool fallbacks already recorded as design |
| Rule: `ai/rules/fail-closed-guards.md` | No path returns a value a caller can read as success after detecting a failure |
| Rule: `ai/rules/fix-dont-record.md` | Tier 2 is a deliberate hand-off to a named destination spec, not a deferral into prose. Every row names its umbrella child |
| Scope boundary | `git diff --stat` shows no change to any Tier 2 file |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Profile captured before coding | `ls tmp/perf-run/pprof` and the frame locations recorded in this spec's Design Insights |
| Modify failures are observable | `grep -rn "bgp_update_modify_failed_total" internal/` returns the registration and every increment site |
| An oversize modification suppresses | `make ze-functional-test TEST=modify-oversize-suppress` |
| The test genuinely catches the leak | revert the caller change only and confirm the `.ci` turns red with the unmodified route on the wire |
| The transcode buffer is pooled | `grep -n "make(\[\]byte" internal/component/bgp/reactor/forward_body.go` returns nothing on the transcode path |
| Both rails hoist the accumulator | `grep -n "var mods filterapi.ModAccumulator" internal/component/bgp/reactor/` returns no match inside a loop body |
| `Reset` has a production caller | `grep -rn "mods.Reset()" internal/` returns both rails |
| Tier 2 untouched | `git diff --stat` lists none of the four Tier 2 files |
| Full gate | `make ze-verify` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-open risk | T1-1 is the fix for one. Verify no remaining path returns a caller-visible success after a detected failure, including the withdrawn-rewrite size check at `forward_build.go:139-143` |
| Cross-tenant leakage | T1-3 is the highest-risk item in this spec: a hoisted accumulator that is not fully reset sends one destination peer another peer's attributes. Treat it as an isolation boundary, not a performance tweak |
| Use after free | T1-2 introduces a pooled buffer whose bytes are aliased into a parsed `message.Update`. Verify the return point against the contracts in `docs/architecture/memory/lifetime-contracts.md` and prove it with a debug-build poison test |
| Resource exhaustion | The counter added in phase 1 is incremented on a peer-influenceable path. Confirm its label set is closed (three fixed reasons) so a peer cannot drive unbounded label cardinality |
| Log volume | The T1-1 warning fires per destination per failing UPDATE. Confirm it is rate-limited or that the failure is rare enough not to be a logging denial of service |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| The profile does not show a Tier 1 performance frame | STOP for T1-3 or T1-4 and present the evidence. T1-1 and T1-2 proceed regardless: one is correctness, the other a rule violation |
| A new `.ci` passes before its fix | The test does not reproduce the behaviour. Back to that phase; do not proceed |
| The golden corpus reports a byte difference | The change was not what it looked like. Stop and re-read the producing function |
| A poison read appears under the debug build | R-2. The pooled buffer is aliased past its return; revert T1-2 and re-derive the lifetime |
| A multi-destination test diverges after the first destination | R-3. The accumulator reset is incomplete; treat as an isolation bug, not a test bug |
| A Tier 2 file appears in the diff | Scope violation. Revert that hunk; it belongs to the umbrella |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The two answers "nothing to modify" and "I could not modify" have shared one nil return since the progressive build was written. The reactor already has vocabulary for exactly this distinction one layer up: `egressStepResult.failed` (`filter_ordered.go:70-77`) exists so that a filter plugin timeout is not reported as a policy decision. T1-1 is the same idea applied one layer down, and should borrow the vocabulary rather than invent a second one.
- `ModAccumulator.Reset` (`filterapi.go:119`) was written for the per-destination loop and never wired to it. Both rails declare a fresh value per iteration instead. That is cheap today at 64 inline bytes and becomes expensive under `plan/spec-wire-edit-0-umbrella.md`, which grows the value roughly eightfold. T1-3 must therefore land **before** the umbrella, or the umbrella regresses the hot path it exists to improve.
- Tier 2 is not a deferral. Each row names the umbrella child that removes it by construction, which is what `ai/rules/fix-dont-record.md` and `ai/rules/deferral-tracking.md` require of anything written down instead of fixed.
- The single largest cost on this path is not in either tier: it is that five separate scanners re-derive the same attribute offsets per UPDATE. That is structural and is the umbrella's first child.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two tiers, only the first implemented | fix everything found | Seven of eleven findings are removed by construction in a spec that rewrites the same functions. Fixing them twice wastes the work and guarantees a conflict. |
| Land the counter before the behaviour change | change the behaviour and observe afterwards | A-1 is unvalidated: nobody knows how often the modify-failure paths fire. Changing behaviour first turns an unknown frequency into an unknown number of route drops. |
| Reuse `egressStepResult.failed` vocabulary | a new error type for modify failure | The distinction already exists one layer up for the same reason. A second vocabulary for the same concept is the coupling this codebase's rules exist to prevent. |
| T1-3 before the umbrella | let the umbrella hoist the accumulator itself | The umbrella grows the value, so hoisting must already be true or the umbrella regresses the path. Ordering is the point of the item. |
| Keep `acquireModBuf`'s existing tiering for the transcode buffer | a dedicated transcode pool | A new pool is a new lifetime to get wrong. The existing tiering already handles the oversized case identically to today. |
| No convergence-movement promise | claim a throughput win | Both prior campaigns attribute the remaining gap to architecture, not to fruit of this size. The benchmarks are the proof, and per-item they may be flat. |

## Known Limitations

- Tier 2 is not fixed here. If `plan/spec-wire-edit-0-umbrella.md` is abandoned or deferred, every Tier 2 row becomes unowned and must be re-homed rather than silently dropped.
- T1-4 folds one duplicate walk into another. The remaining three duplicate walks (`wireu/aspath_rewrite.go`, `wireu/community.go`, `attribute/wire.go`) are Tier 2 and stay.
- The counter added in phase 1 is the only visibility into how often the modify-failure paths fire. Before that soak result exists, A-1 is a belief.
- No claim is made about the throughput regression measured against the 2026-06-05 baseline. That is a bisect, not an optimisation spec.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. This
spec adds no new RFC enforcement; it preserves existing enforcement while
changing costs and failure reporting. The requirements that must survive
unchanged, each at the site it already lives:

| RFC | Section | Requirement | Site that must not change behaviour |
|-----|---------|-------------|------------------------------------|
| 7606 | 3, 5.3 | attribute-discard and treat-as-withdraw classification | `session_validation.go:108`, unchanged verdict under T1-4 |
| 6793 | 4.2.2 | AS4_PATH obligation on ASN4 transcode | `forward_body.go:159`, unchanged output under T1-2 |
| 4271 | 4.3 | total path attribute length is a two-octet field | `forward_build.go:263-267`, whose out-of-range abort becomes a suppression under T1-1 |
| 8669 | 4 | discard PrefixSID from eBGP unless configured to accept | `session_validation.go:110-125`, unchanged verdict under T1-4 |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
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
- [ ] Learned summary written to `plan/learned/NNN-hotpath-alloc-round-4.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/spec-hotpath-alloc-round-4.md` only (commit A preserves the spec in history)
