# Spec: rfc7606-5-1-2-relay-shape

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 3/3 (follow-on 2: the remaining withdraw-only attribute producers) |
| Updated | 2026-08-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `rfc/short/rfc7606.md` (the RFC7606-5.1-2 `{gap}` annotation), `rfc/full/rfc7606.txt` §5.1
4. `internal/component/bgp/reactor/forward_body.go`, `internal/component/bgp/wireu/split.go`

## Task

Close the remaining half of RFC 7606 Section 5.1's second bullet: "An UPDATE message MUST
NOT contain more than one of the following: non-empty Withdrawn Routes field, non-empty
Network Layer Reachability Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI
attribute."

Carved out of `plan/spec-rfc7606-close-gaps.md` (closed 2026-07-20), which implemented five
of that RFC's gaps and narrowed this one. **Ze already ORIGINATES only compliant UPDATEs,
and both splitters now emit one NLRI-bearing field per message.** What remains is the relay
side, and it is a genuine trade rather than an oversight, which is why it gets its own spec
instead of being left as an unexplained annotation.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/wire/mp-nlri-ordering.md` - the Section 5.1 FIRST-bullet divergence
  → Constraint: MP_UNREACH-first / MP_REACH-last ordering is deliberate and must not change.
- [ ] `ai/rules/performance.md` - encoding must not allocate per message
  → Constraint: the zero-copy forward path is the thing at risk here.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7606.md` - the RFC7606-5.1-2 annotation records the current state
  → Constraint: Section 5.1 also says an implementation "MUST still be prepared to receive
    these fields in any position or combination", so RECEIVE-side tolerance must not change.
  → Constraint: the restriction exists "To facilitate the determination of the NLRI field in
    an UPDATE message with a malformed attribute" -- it protects a RECEIVER, and every
    receiver is separately required to cope with any combination anyway.

**Key insights:**
- The obligation binds the sender. Ze is the sender of bytes it relays, even when it did
  not compose them, so forwarding a received mixed shape is within scope of the MUST.
- Ze already satisfies the MUST everywhere it CONSTRUCTS an UPDATE.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody`.
  → Constraint: `:51-65` is the same-context zero-copy path. When the ContextID matches and
    the UPDATE fits, `:64` does `result.rawBodies = append(result.rawBodies, peerWire.Payload())`
    -- a slice-header append, no parse, no copy. The comment at `:48-50` states it is placed
    "before any parse or re-encode" deliberately.
  → Constraint: `:99` appends a re-encoded `destUpdate` whole when it fits. That one IS ze
    constructing an UPDATE -- `fwdUpdateForDestination` rebuilt its sections for the
    destination context -- so it is the weaker of the two justifications for leaving it.
  → Constraint: the oversize branches already split (`:55` via `wireu.SplitWireUpdate`,
    `:93` via `fwdSplitParsedUpdate`), and both splitters are now one-field-per-message. Only
    the two FITS branches are non-compliant.
- [ ] `internal/component/bgp/wireu/split.go` - `buildCombinedUpdates` drains each component
  into its own message (done 2026-07-20).
  → Constraint: `SplitWireUpdate` returns early when the payload fits, so it does not cover
    the fits case.
- [ ] `internal/component/bgp/message/update_split.go` - `splitUpdateWithMP` likewise emits
  one field per chunk (fixed 2026-07-20 in `574e3c596`).
- [ ] Origination is already compliant: `UpdateBuilder.BuildUnicast`
  (`update_build.go`) sets NLRI without WithdrawnRoutes; withdrawals are
  withdraw-only (`peer_rib_routes.go`).

**Behavior to preserve:**
- Receive-side tolerance of any position or combination (Section 5.1 requires it).
- MP_UNREACH before MP_REACH.
- End-of-RIB handling.
- Withdrawals before announcements.
- The zero-copy append at `:64` for UPDATEs that carry a single NLRI-bearing field.

**Behavior to change:**
- A relayed UPDATE that mixes NLRI-bearing fields is split before being sent on.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A received UPDATE being forwarded to a peer, via `buildFwdBody`.

### Transformation Path
1. `buildFwdBody:51` same-context and fits → `:64` verbatim payload append. **Non-compliant
   when the received UPDATE mixed fields.**
2. `buildFwdBody:82` context mismatch → `fwdUpdateForDestination` → `:99` whole emit when it
   fits. **Non-compliant when the rebuilt UPDATE mixes fields.**

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Receive ↔ forward | `*wireu.WireUpdate` shared across peers in the forward loop | [ ] |
| Forward ↔ wire | raw payload append, or parsed `*message.Update` | [ ] |

### Integration Points
- `buildFwdBody` (`reactor/forward_body.go`), its two fits branches.

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality (reuse the existing splitters rather than a third one)
- [ ] Zero-copy preserved where applicable — this is the crux; see R-1
- [ ] Registration over hardcoding — N/A

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Mixed-field UPDATEs are rare in practice | unmeasured | the cost is charged on every forward rather than rarely | count mixed shapes on a real feed before implementing | unvalidated |
| A-2 | The mix/no-mix decision can be made once per RECEIVED UPDATE, not once per peer | the same `*wireu.WireUpdate` pointer is shared across the forward loop and is part of the `bodySlots` cache key (`forward_rs.go`) | the cost multiplies by peer count | read the loop and the cache key | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Splitting relayed UPDATEs costs the zero-copy same-context forward | throughput regression on a route-reflector benchmark | scan once at receive and mark the WireUpdate, so only mixed ones lose zero-copy |
| R-2 | Message counts change, moving the supersede/dedup identity | `fwdSupersedeKey` hit-rate change (`forward_pool.go`) | correctness holds either way; measure the hit rate |
| R-3 | `fwdIsWithdrawal` classification changes for split items | ordering/priority treatment differs | re-check the classifier against the new shapes |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| forward a received UPDATE mixing withdrawn + NLRI, same context | → | `buildFwdBody` sameCtx fits branch | `TestForwardSplitsMixedShapeSameContextThatFits` |
| same, context mismatch | → | `buildFwdBody` re-encode fits branch | `TestForwardSplitsMixedShapeAcrossContextsThatFits` |
| a peer relays a mixed UPDATE through the daemon | → | the whole path, wire to wire | `test/plugin/rfc7606-relay-one-field.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ze forwards a received UPDATE carrying both Withdrawn Routes and NLRI, same context, fits | each emitted UPDATE carries at most one NLRI-bearing field |
| AC-2 | Same, context mismatch (re-encoded `destUpdate`, fits) | same |
| AC-3 | ze receives an UPDATE mixing fields | still accepted, unchanged (Section 5.1 receive-side tolerance) |
| AC-4 | ze forwards an UPDATE with exactly one NLRI-bearing field | the `:64` zero-copy append is still taken, no parse, no extra allocation |
| AC-5 | End-of-RIB | unchanged |
| AC-6 | Any split | withdrawals still precede announcements, MP_UNREACH before MP_REACH |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs ze as a route server; a client sends a mixed-shape UPDATE | receive → enforceRFC7606 (accepts it) → buildFwdBody → split → peer | `test/plugin/rfc7606-relay-one-field.ci`: the receiver gets a withdraw-only UPDATE asserted byte for byte, plus the announce separately |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNLRIBearingFieldCountEveryCombination` | `message/rfc7606_shape_test.go` | all 10 combinations of the four fields | done |
| `TestNLRIBearingFieldCountNoAttributes` / `...TruncatedAttributes` | same | no-attrs and unparseable-attrs handling | done |
| `TestSplitCompliantSplitsMixedUpdateThatFits` | same | AC-1/AC-2 at the splitter | done |
| `TestSplitCompliantPassesThroughCompliantUpdate` | same | AC-4 (identity, not equality) | done |
| `TestSplitCompliantEndOfRIBUntouched` | same | AC-5 | done |
| `TestSplitCompliantStillSplitsOnSize` | same | RFC 8654 size splitting not regressed | done |
| `TestSplitCompliantWithdrawalsPrecedeAnnouncements` | same | AC-6 | done |
| `TestSplitWireUpdateSplitsMixedShapeThatFits` | `wireu/shape_rfc7606_test.go` | AC-1 at the wire splitter, nothing lost | done |
| `TestSplitWireUpdateCompliantShapeUntouched` | same | AC-4, same-pointer return | done |
| `TestWireUpdateMixesNLRIFieldsCachedPerMessage` | same | the verdict is cached per message, not per peer | done |
| `TestWireUpdateMixesNLRIFieldsBoundary` | same | the two-field boundary | done |
| `TestWireUpdateEndOfRIBNotMixed` | same | AC-5 | done |
| `TestWireUpdateMalformedNotMixed` | same | a parse failure is not an invented violation | done |
| `TestForwardSplitsMixedShapeSameContextThatFits` | `reactor/forward_body_rfc7606_test.go` | AC-1 | done |
| `TestForwardSplitMixedShapeWithdrawalsFirst` | same | AC-6 | done |
| `TestForwardCompliantShapeKeepsZeroCopy` | same | AC-4, backing-array identity | done |
| `TestForwardEndOfRIBUnaffected` | same | AC-5 | done |
| `TestForwardSplitsMixedShapeAcrossContextsThatFits` | same | AC-2 | done |
| `TestForwardCompliantShapeAcrossContextsNotSplit` | same | the re-encode path is not pushed into copies | done |
| `BenchmarkMixesNLRIFields` | `wireu/shape_bench_test.go` | the per-destination cost | done |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI-bearing fields per emitted UPDATE | 0..1 | 1 | N/A | 2 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-relay-one-field.ci` | `test/plugin/` | a mixed-shape UPDATE relayed through ze reaches the peer as separate compliant UPDATEs | done: 20/20 green, 4/4 red when either relay branch is reverted |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| relay through ze (configured as a route server) | `test/interop/scenarios/47-rfc7606-relay-shape-frr` | FRR | FRR installs ze's relayed output; discriminates the duplicate-NEXT_HOP defect (RED when reverted); proves the §5.1 split output is accepted | **DELIVERED, 4/4 stable** |

### Future (if deferring any tests)
- None.

## Files to Modify
- `internal/component/bgp/reactor/forward_body.go` - both fits branches
- `internal/component/bgp/message/update_split.go` - `SplitCompliant` + extracted `splitByShape`
- `internal/component/bgp/wireu/split.go` - shape fast path + `mpUnreachHasNLRI` EoR guard
- `internal/component/bgp/wireu/wire_update.go` - cached `MixesNLRIFields`
- `internal/component/bgp/reactor/reactor_api_batch.go` - `stripAttribute`; NEXT_HOP / MP_REACH dedup (bug found via interop)
- `rfc/short/rfc7606.md` - remove the RFC7606-5.1-2 `{gap}` once met
- `rfc/audit/rfc7606.json`, `docs/features/rfc-status.md` - gaps 3 → 2
- `docs/architecture/wire/mp-nlri-ordering.md` - enforcement points
- `test/interop/interop.py` - injector-sidecar support
- `docs/architecture/testing/interop.md`, `ai/INDEX.md` - discovery for the sidecar
- `ai/INSTRUCTIONS.md` + `ai/rules/completion.md` (new), `ai/rules/interop-and-goal-validation.md` - ethos rules (owner directive)

### BGP Family Checklist (if new SAFI / capability / attribute)
N/A — no new family, capability or attribute.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | no config surface |
| CLI commands/flags | No | - |
| Doctor check | No | - |
| Prometheus counters | Maybe | a split-on-relay counter would make R-1 measurable in production |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 7 | Wire format changed? | Yes | `docs/architecture/wire/` |
| 9 | RFC behavior implemented? | Yes | `rfc/short/rfc7606.md`, `docs/features/rfc-status.md` |
| others | | No | |

## Files to Create
- `test/plugin/rfc7606-relay-one-field.ci`
- `internal/component/bgp/message/rfc7606_shape.go` (+ `_test.go`)
- `internal/component/bgp/wireu/shape_rfc7606_test.go`, `shape_bench_test.go`, `split_eor_test.go`
- `internal/component/bgp/reactor/forward_body_rfc7606_test.go`, `reactor_api_batch_dedup_test.go`
- `test/interop/scenarios/47-rfc7606-relay-shape-frr/` (ze.conf, frr.conf, inject.msg, inject-args, check.py)
- `ai/rules/completion.md`
- `plan/learned/1225-rfc7606-relay-shape.md`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify-changed` |
| 6-9. Review + fix | Critical Review Checklist |
| 10-12. Deliverables, security, docs | below |
| 13. /ze-review gate | Review Gate |
| 14. Present + close | two-commit closure |

### Implementation Phases

1. **Phase: measure first** — count mixed-shape UPDATEs on a representative feed and record
   the number here. A-1 is unvalidated and the whole cost/benefit turns on it.
2. **Phase: receive-side decision** — determine once per received UPDATE whether it mixes
   fields, cached on the WireUpdate, so the per-peer loop pays nothing in the common case.
3. **Phase: split on relay** — reuse `SplitWireUpdate` / `message.Splitter`.
4. **Phase: disclosure** — drop the `{gap}`, update the rfc-status row and the audit verdict.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Receive tolerance | Section 5.1's "MUST still be prepared to receive" is untouched |
| Ordering | withdrawals before announcements; MP_UNREACH before MP_REACH |
| Zero-copy | the single-field common case still takes the `:64` append with no parse |
| No third splitter | the existing two are reused |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Relay emits one field per UPDATE | the new `.ci` plus a reactor unit test |
| Gate reflects it | `grep -c "{gap" rfc/short/rfc7606.md` returns 2 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Amplification | a peer sending mixed-shape UPDATEs forces ze to split on every relay; bound the cost |
| Allocation | splitting must not allocate per peer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Throughput regression | back to DESIGN: the receive-side decision is not doing its job |
| Existing forward test turns red | STOP: a relay regression is a blocker |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| An allocation assertion can prove the shape verdict is cached | neither side allocates, so `AllocsPerRun == 0` held with the cache deleted | mutation testing: the cache-removal mutant survived | a test that claimed to pin the cache proved nothing. Second occurrence of this shape in two specs (1224's `Enabled()` guard test) |
| A fixture carrying all four fields exercises the two-field threshold | `> 1` becoming `> 2` still splits a four-field UPDATE | the threshold mutant survived inside the wireu package | the boundary was pinned only by a test in another package |
| The frequency of mixed UPDATEs decides whether this is worth doing (A-1) | it only decides that if the check is charged per relay; caching makes the question moot | writing the benchmark to settle A-1 | the spec's first implementation phase was unnecessary |
| Declaring the peers' End-of-RIB in the `.ci` makes the ordering deterministic | it opts INTO the race: an unmatched EOR is silently accepted, but a declared one competes with the real message for the same rule | two flaky failures on an unchanged tree | cost two debugging rounds before reading `checker.go` |
| A green plugin `.ci` suite means a peer would accept the output | `ze-peer` asserts only the bytes it was told to expect; FRR applies RFC 7606 and discarded every relayed UPDATE over a duplicate NEXT_HOP | building the interop scenario | a real duplicate-NEXT_HOP defect had been invisible to the whole `.ci` suite -- now fixed |
| A passing interop scenario proves the fix works | the first scenario PASSED with all three fixes reverted -- FRR accepts an unsplit mix by RFC, and the live-forward path never duplicates NEXT_HOP, so nothing under test was exercised | reverting all fixes and re-running, at the owner's prompting that ze "is not ALWAYS a route server" | a vacuous interop test was about to ship as evidence. Redesigned to route the asserted prefix through the REPLAY path, where the duplicate-NEXT_HOP defect is a real FRR rejection; now RED when the fix is reverted. Rule added: `interop-and-goal-validation.md` "Prove the test discriminates" |
| Ze IS a route server (implied by the scenario's framing) | Ze is a general BGP speaker; route-server is one CONFIGURED mode, and `no bgp enforce-first-as` on FRR is correct RS-client config (RFC 7947), not a workaround | owner correction | scenario comments and `ze.conf`/`frr.conf` reworded to say "configured as a route server" |
| Parking the interop scenario in `tmp/` with an offer to drop it was an acceptable way to handle a pre-existing blocker | it was not: the owner's standard is that a non-interoperating daemon is worthless, and "pre-existing" is not out of scope once your work depends on the path | owner rejection, twice | the blocker was fixed at source and the scenario delivered; rules added (`completion.md`) so this cannot recur |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The MUST protects a receiver's ability to locate the NLRI field in an UPDATE with a
  malformed attribute. Every receiver is independently required to cope with any
  combination, so the practical benefit of splitting on relay is smaller than the letter
  suggests. That is an argument about priority, not about whether the obligation applies.

## Core Insight

Ze is the sender of bytes it relays, even when it did not compose them. "We only forward
what we received" is a policy choice, not an exemption from a sender-side MUST.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Carved into its own spec rather than left as an unexplained annotation | leaving the `{gap}` in place with no home | the trade is real and needs measuring first; a spec is where that belongs |

## Known Limitations
- Until this lands, `rfc/short/rfc7606.md` carries the RFC7606-5.1-2 `{gap}` and
  `docs/features/rfc-status.md` discloses it.

## RFC Documentation

Add `// RFC 7606 Section 5.1: "<quoted requirement>"` above the enforcing code.

## Implementation Summary

### What Was Implemented
- `message.NLRIBearingFieldCount(withdrawn, attrs, nlri)` in a new `message/rfc7606_shape.go`:
  the single definition of how many of the four fields an UPDATE carries. Both the parsed and
  the wire path call it, so neither grows its own notion of "mixed".
- `(*message.Update).MixesNLRIFields()` and `(*wireu.WireUpdate).MixesNLRIFields()`. The wire
  one caches the verdict behind a `sync.Once`, matching that type's existing lazy-field style.
- `Splitter.SplitCompliant`: splits on shape as well as size. `Split` keeps its size-only fast
  path, and both share the extracted `splitByShape`.
- `SplitWireUpdate`'s fast path now also asks about shape. It has exactly one non-test caller
  (the relay), so changing it in place needed no new entry point.
- `buildFwdBody` splits on shape in BOTH branches; `fwdSplitParsedUpdate` calls `SplitCompliant`.

### Bugs Found/Fixed
- **Duplicate NEXT_HOP on the route-server replay/re-advertise path (FIXED).** Surfaced by the
  interop scenario: FRR installed ZERO of ze's relayed routes and logged `BGP attribute type 3
  appears twice in a message - discard attribute` (RFC 7606 Section 3(g)). Root cause:
  `buildWireModeUpdate` (reactor_api_batch.go) inserted a NEXT_HOP unconditionally while
  `writeMandatoryAttrs` had already copied the full stored attribute block -- NEXT_HOP included.
  The route server's replay-on-peer-up re-encodes stored routes through this path (`update hex
  attr set <attrs> nhop set <nh> ...`, adj_rib_in/rib.go), so every replayed IPv4 route
  carried NEXT_HOP twice; same defect for MP families (a second MP_REACH_NLRI). Fix:
  `stripAttribute` removes any pre-existing NEXT_HOP / MP_REACH before the builder writes the
  authoritative one; an unset next-hop errors out before the builder (`peer.resolveNextHop` ->
  `ErrNextHopUnset`, nexthop.go), so replacing never loses a next-hop. Three unit tests plus
  the interop scenario, which fails when the fix is reverted.
- **Splitting could MANUFACTURE an End-of-RIB (FIXED).** Independent review: an UPDATE mixing
  IPv4 withdrawn routes with an empty MP_UNREACH (AFI/SAFI only) split into a standalone
  MP_UNREACH message, byte-identical to an RFC 4724 multiprotocol EoR marker, ending a
  restarting peer's deferral early. Fix: `mpUnreachHasNLRI` (wireu/split.go) drops an empty
  MP_UNREACH from the split (it withdraws nothing); a genuine standalone EoR is never mixed and
  never reaches the loop. Two tests in `split_eor_test.go`.

### Documentation Updates
- `docs/architecture/wire/mp-nlri-ordering.md`: new "Where requirement 2 is enforced" section
  with the four enforcement points, the caching rationale and three source anchors. The
  heading "Ze is half-compliant with RFC 7606" was wrong once requirement 2 was met and now
  reads "Ze meets requirement 2 and diverges on half of requirement 1".
- `docs/features/rfc-status.md`: RFC 7606 row from three gaps to two.
- `rfc/short/rfc7606.md`: the RFC7606-5.1-2 `{gap}` annotation removed.
- `rfc/audit/rfc7606.json`: verdict re-judged to `implemented` with the nine tests bound.
- `ai/rules/completion.md` (new) + `ai/INSTRUCTIONS.md`: owner directive after this session --
  never park a blocker, never reduce coverage to reach green.
- `ai/rules/interop-and-goal-validation.md`: a new "Prove the test discriminates" section, from
  the vacuity mistake below.

### Deviations from Plan
- **Phase 1 ("measure first") was not performed as written, and A-1 was dissolved rather than
  measured.** The plan assumed the cost of checking shape would be charged per relay, making
  the frequency of mixed UPDATEs decisive. Caching the verdict per received message removes
  that dependency: a compliant UPDATE costs one bool read (3.3ns against 51.7ns recomputed,
  both allocation-free, `BenchmarkMixesNLRIFields`) and keeps the zero-copy forward. Measuring
  a real feed would not change any decision here, so it was not worth a feed capture.
- **The interop scenario went from "parked/blocked" to DELIVERED after owner intervention.**
  An earlier draft of this spec parked the scenario in `tmp/` because a pre-existing duplicate
  -NEXT_HOP defect blocked it, and offered to drop the deliverable. The owner rejected that
  ("a BGP daemon that cannot interoperate is NOTHING") and the rules were changed
  (`ai/rules/completion.md`). The blocking defect was then fixed at its source (above), and the
  scenario is now `test/interop/scenarios/47-rfc7606-relay-shape-frr`, passing and discriminating.
- **Scope grew by one fix and one harness feature, both endorsed by the no-park directive:**
  the NEXT_HOP de-duplication (reactor_api_batch.go) and an injector-sidecar addition to
  `test/interop/interop.py` (a `ze-test peer` sidecar that drives ze with wire bytes no
  conforming daemon emits).
- A pre-existing observation, not pursued and not blocking: with LOCAL_PREF present on an
  eBGP-learned route, ze puts its attribute-discard marker (type 253) on the wire -- the same
  question `bgp-rs-fastpath-ebgp-shared.ci` already raises. The fixture avoids LOCAL_PREF so it
  does not probe this; it is genuinely out of scope for a Section 5.1 spec.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A relayed UPDATE carries at most one NLRI-bearing field | done | `reactor/forward_body.go,:101` | both fits branches |
| Reuse the existing splitters, no third one | done | `wireu/split.go`, `message/update_split.go` | `SplitCompliant` shares `splitByShape` with `Split` |
| Receive-side tolerance unchanged | done | nothing on the receive path was touched | the reactor suite passes |
| Zero-copy preserved for compliant UPDATEs | done | `TestForwardCompliantShapeKeepsZeroCopy` | asserts backing-array identity |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | `TestForwardSplitsMixedShapeSameContextThatFits`, `rfc7606-relay-one-field.ci` | both fail if the branch is reverted |
| AC-2 | done | `TestForwardSplitsMixedShapeAcrossContextsThatFits` | |
| AC-3 | done | `rfc7606-relay-one-field.ci`, whose source peer is not reset | receive path untouched |
| AC-4 | done | `TestForwardCompliantShapeKeepsZeroCopy`, `TestSplitWireUpdateCompliantShapeUntouched` | identity, not equality |
| AC-5 | done | `TestForwardEndOfRIBUnaffected`, `TestWireUpdateEndOfRIBNotMixed`, `TestSplitCompliantEndOfRIBUntouched` | |
| AC-6 | done | `TestForwardSplitMixedShapeWithdrawalsFirst`, `TestSplitCompliantWithdrawalsPrecedeAnnouncements` | NOT asserted by the `.ci`; see its header for why |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Unit tests | done, 19 plus 1 benchmark | three packages | all listed in the TDD table |
| Boundary test | done | `TestWireUpdateMixesNLRIFieldsBoundary`, `TestNLRIBearingFieldCountEveryCombination` | 0 and 1 compliant, 2 not |
| Functional `.ci` | done | `test/plugin/rfc7606-relay-one-field.ci` | 20/20 green, 4/4 red on revert |
| Interop scenario | done | `test/interop/scenarios/47-rfc7606-relay-shape-frr` | passing, stable, discriminates the NEXT_HOP dedup fix |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `reactor/forward_body.go` | changed | both fits branches, plus `fwdSplitParsedUpdate` |
| `wireu/split.go` | changed | fast path asks about shape |
| `message/update_split.go` | changed | `SplitCompliant` plus the extracted `splitByShape` |
| `message/rfc7606_shape.go` | added | not in the plan: the plan assumed each splitter would decide for itself, which would have meant two definitions of "mixed" |
| `wireu/wire_update.go` | changed | not in the plan: the cached verdict lives with the other lazy fields |
| `rfc/short/rfc7606.md`, `rfc/audit/rfc7606.json`, `docs/features/rfc-status.md` | changed | gap removed, verdict re-judged, disclosure updated |
| `docs/architecture/wire/mp-nlri-ordering.md` | changed | enforcement points documented |
| `test/plugin/rfc7606-relay-one-field.ci` | added | as planned |

### Audit Summary
- **Total items:** 6 ACs, 4 task requirements, 4 test categories
- **Done:** 6 ACs, 4 requirements, 3 test categories
- **Partial:** none
- ~~**Skipped:** the interop scenario -- **NOT owner-approved, so the spec stays open**~~ (stale, corrected 2026-07-22 during plan review: the interop scenario was subsequently owner-endorsed and DELIVERED -- `test/interop/scenarios/47-rfc7606-relay-shape-frr` exists and ran 4/4 stable; see the Interop Tests table above, the Findings/Resolution section, and `plan/learned/1225-rfc7606-relay-shape.md`. Nothing in this spec remains open; awaiting two-commit closure)
- **Changed:** two files added that the plan did not list, both to avoid a second definition
  of "mixed"; documented above

## Follow-On: withdraw-only relay shape (2026-08-04)

Homed here by `plan/deferrals/fixit-otc-src-role-meta-fallback.md`, which this
spec's ownership of relayed UPDATE shape made the destination for.

### The defect

`(*ASPathEdit).recordPrepend` (`internal/component/bgp/wireu/aspath_slot.go`)
built `&attribute.ASPath{}` from nothing when the forwarded payload carried no
AS_PATH, and emitted it. `forwardUpdateCore`
(`internal/component/bgp/reactor/reactor_api_forward.go`) drove it on
`facts.isEBGP` alone. A source withdrawal of `attrLen=0000` therefore reached an
eBGP peer as `attrLen=0009` -- AS_PATH present, ORIGIN and NEXT_HOP absent.

RFC 4271 Section 4.3 says such a message "will not include path attributes or
Network Layer Reachability Information". Section 6.3 says "If any of the
well-known mandatory attributes are not present, then the Error Subcode MUST be
set to Missing Well-known Attribute." Section 6.3's tolerance clause covers "an
UPDATE message that contains correct path attributes, but no NLRI", and a lone
AS_PATH is not a correct set. Measured against FRR 10.3.1 on 2026-08-04:

> [EC 33554482] 172.30.0.2 Missing well-known attribute NEXT_HOP.
>
> [EC 33554455] 172.30.0.2(Unknown) rcvd UPDATE with errors in attr(s)!! Withdrawing route.

So every withdrawal Ze relayed to an eBGP peer was refused, and the withdrawn
route stayed live there.

### The fix, and the layer

`ASPathEdit.Record` now routes to `recordTranscode` when the intent's Prepend is
empty OR `PayloadAdvertisesNLRI` says the payload carries no reachable NLRI.
`recordPrepend`, the only frame that can create an AS_PATH, is then unreachable
for a withdraw-only UPDATE.

**Why `Record` and not `forwardUpdateCore`.** Three reasons, in order of weight.

1. `Record` is the only frame that holds the WHOLE payload. `recordPrepend` sees
   just the attribute section, so it cannot see the IPv4 NLRI field at all, and
   the guard would have needed a second argument threaded in for it.
2. Two callers compose the same intent -- `forwardUpdateCore` and
   `reactorForwardRS` (`reactor/forward_rs.go`). A predicate at the driver is two
   copies today and a third the next time somebody records an intent.
3. RFC 4271 Section 5.1.2's prepend clause is conditioned on "when a given BGP
   speaker advertises the route to an external peer". That condition is a
   property of the PAYLOAD, not of the destination, so it belongs beside the
   payload rather than beside the peer.

Landing on `recordTranscode` rather than returning early is deliberate: an
AS_PATH or AGGREGATOR that rode along on a withdraw-only UPDATE is still
re-encoded at the destination's AS number width (RFC 6793 Section 4.2.2), and
one that was never there is never invented.

`wireu.PayloadAdvertisesNLRI` is the single definition. `role/otc.go`'s
`payloadAdvertisesNLRI` -- the RFC 9234 Section 5 stamping gate, which asks the
identical question -- now delegates to it, and its duplicate byte-walk plus the
`hasAttr` helper it needed are deleted.

### Tests

| Test | File | Proves |
|------|------|--------|
| `TestPayloadAdvertisesNLRIShapes` | `wireu/advertise_test.go` | 10 shapes: withdraw-only, MP_UNREACH-only, EoR, NLRI, MP_REACH, mixed, three truncations |
| `TestASPathSlotPrependOnlyWhenAdvertising` | same | the SAME intent prepends over an advertising payload and records nothing over a withdraw-only one; RFC4271-5.1.2-3 both polarities |
| `TestASPathSlotWithdrawOnlyStillTranscodes` | same | dropping the prepend does not drop the RFC 6793 width transcode |
| `TestMessageIsEOR` (12 rows) + `...RejectsNonUpdate` | `internal/test/peer/message_eor_test.go` | RFC 4724 Section 2 by content, not by length |
| `role-otc-fwd-withdraw.ci` | `test/plugin/` | the relayed withdrawal is byte-identical to the source's: `attrLen=0000` |
| `52-relay-withdraw-shape-frr` | `test/interop/scenarios/` | FRR 10.3.1 ACCEPTS the relayed withdrawal, and prepends still reach advertisements |

### Mutation evidence (all measured, 2026-08-04)

| Mutant | Result |
|--------|--------|
| `Record`: drop the guard (`len(in.Prepend) == 0`) | 3 unit subtests RED; `role-otc-fwd-withdraw.ci` RED with `+ AS_PATH: [65000] (unexpected)`; interop 52 RED at the positive -- the relayed EoR is stamped, stops being a marker, and the injector never releases |
| `Record`: narrow the guard to `len(payload) == 4` | interop 52 RED at the NEGATIVE with FRR's two error lines quoted above |
| `Message.IsEOR`: back to the length test | `role-otc-rs-withdraw-eor.ci` fails as `TYPE: unknown` with no diff instead of naming `+ ATTR_35: 0000fde8 (unexpected)` |
| `role.payloadAdvertisesNLRI`: forced open | interop 51 RED with FRR's two error lines; `role-otc-rs-withdraw-eor.ci` RED on the withdrawal |
| `role.payloadAdvertisesNLRI`: open for the EoR only | `role-otc-rs-withdraw-eor.ci` RED on the EoR with `+ ATTR_35: 0000fde8 (unexpected)` |

### Fixtures corrected, not assertions

Eight test fixtures asserted a prepend over a payload carrying no NLRI, so they
were withdraw-only UPDATEs claiming to be advertisements. Each gained an NLRI;
no assertion changed. Seven are in `wireu/aspath_slot_test.go` (via
`probeAdvertisedNLRI`); the eighth is `TestReactorForwardRSEBGPPrepend`
(`reactor/forward_rs_test.go`), which is RFC-tagged and carries an
`rfc-test-change-approved:` marker.

### Found while doing this, NOT fixed here

**Rows 1, 2 and 3 are FIXED by "Follow-On 2" below (2026-08-04), by one guard in
`planAttr` rather than the three separate ones this table anticipated.** Rows 4
and 5 stand as written. The paragraph and table are kept unedited so the reasoning
that homed the work here stays readable.

All three were confirmed by independent review, and none is reached on the rail
this spec owns. The first two are the same defect CLASS as the one fixed above:
a per-destination rule that adds an attribute without asking whether the UPDATE
advertises anything. They want one "advertises" bit hoisted into
`forwardUpdateCore` and read by every adding site, not three separate guards, and
that is a spec of its own with its own interop witnesses.

| Finding | Producer | Why not here |
|---------|----------|--------------|
| Next-hop-self stamps a lone NEXT_HOP onto a relayed withdrawal AND onto a relayed legacy End-of-RIB. `applyFactsNextHop` (`reactor/peer_forward_facts.go`) records `Op(3, Set)` whenever `nhMode` is not `nhModeNone`, and `genericAttrSetHandler` (`reactor/filter_delta_handlers.go`) CREATES the attribute when the payload has no source value | `forwardUpdateCore` (`reactor/reactor_api_forward.go`) | reached only when the operator configures a next-hop rewrite, which the eBGP default path does not. `role-otc-fwd-withdraw.ci` asserts the relayed withdrawal is byte-identical, which holds because `nhMode` is `nhModeNone` there. Needs its own interop witness with next-hop-self configured |
| RFC 4456 reflection injects ORIGINATOR_ID and CLUSTER_LIST onto a relayed withdraw-only UPDATE and onto a relayed End-of-RIB; `originatorIDHandler` and `clusterListHandler` both create when the attribute is absent | same, gated on `srcInfo.isIBGP && !facts.isEBGP` | the iBGP route-reflector rail, which this spec does not touch. Confirmed NOT reachable on the plain eBGP rail |
| The egress community tag records `AttrModAdd`, and `genericCommunityHandler` (`bgp/plugins/filter_community/handler.go`) emits from an Add with no source value | `applyEgressFilter` (`bgp/plugins/filter_community/egress.go`) | a plugin's own policy surface, a third producer again |
| `tryShift` (`wireu/aspath_slot.go`) returns before `recordAS4Path`, so the fast prepend path carries a received AS4_PATH onward between two NEW speakers (RFC 6793 Section 4.1 "MUST NOT be carried"). Pre-existing, on the ADVERTISING rail, and untouched by this change; the withdraw-only half of the same hole IS fixed here, by `recordWithdrawOnly` | `ASPathEdit.tryShift` | the fast path cannot express the AS4_PATH merge it would need for the both-OLD case, so closing it is a redesign of that path rather than a guard |
| An RS-Client DESTINATION (RFC 9234 wire role 2) is proven nowhere. `role-otc-rs-withdraw-eor.ci`'s dest declares wire role 3, which `roleNames` (`role/role.go`) calls customer; egress rule 1's third named destination role rests on the Customer case reading as representative | `role/otc.go` | RFC 9234 scope, not relay shape. Recorded in that `.ci`'s header |

## Follow-On 2: the remaining withdraw-only attribute producers (2026-08-04)

Closes three rows of the table above, the one titled "Found while doing this,
NOT fixed here". A fourth producer came from the review of this change.

### The defect, restated once for all four

A per-destination egress rule CREATED a path attribute on a relayed UPDATE that
advertises no reachable NLRI. Four producers did it, each on a different rail:

| Producer | File | Reached when |
|----------|------|--------------|
| `applyFactsNextHop` -- `Op(3, Set)`, `Op(14, Set)` | `reactor/peer_forward_facts.go` | the operator configures a next-hop rewrite |
| `forwardUpdateCore` / `reactorForwardRS` -- `Op(9, Set)`, `Op(10, Prepend)`, landing on `originatorIDHandler` and `clusterListHandler` | `reactor/reactor_api_forward.go`, `reactor/forward_rs.go`, `reactor/filter_delta_handlers.go` | the iBGP route-reflector rail (RFC 4456 Section 8) |
| the egress community tag, landing on `genericCommunityHandler` | `bgp/plugins/filter_community/egress.go` | a route server strips or adds control communities |
| a policy chain's text delta on any code, landing on `genericAttrSetHandler` | `reactor/filter_ordered.go` | an import or export policy sets an attribute |

RFC 4271 Section 4.3 states the shape. An UPDATE that withdraws only "will not
include path attributes or Network Layer Reachability Information".

Section 6.3 makes the result a wire error. "If any of the well-known mandatory
attributes are not present, then the Error Subcode MUST be set to Missing
Well-known Attribute."
An RFC 4724 Section 2 End-of-RIB stops being a marker the moment one attribute
lands on it.

### The layer, and why it is NOT `forwardUpdateCore`

The design first proposed was one `advertises` bit hoisted into
`forwardUpdateCore` and threaded to the recording sites. The code says otherwise,
for four reasons in order of weight.

1. **`forwardUpdateCore` is one of FIVE drivers.** `buildModifiedPayload` is
   reached from `reactor_api_forward.go`, `forward_rs.go`, `filter_ordered.go`
   twice (import and export chains) and `reactor_api_batch.go` (stale
   re-advertise). A bit at one driver covers one rail.
2. **A driver cannot tell CREATE from MODIFY.** Refusing to RECORD the operation
   also cancels the legitimate rewrite of an attribute that rode along on the
   withdrawal, which is exactly the distinction `79b46ef60` was careful to keep
   (`recordTranscode` still re-encodes an AS_PATH that was already there).
   `src == nil` is known only inside `planAttr`.
3. **One producer lives in a plugin package.** `genericCommunityHandler`
   (`filter_community`) never sees a reactor-local bit.
4. **The bit must describe the body the rebuild WRITES, not the one it reads.**
   An export chain that denies every prefix passes `nlriOverride = []byte{}`, so
   a source-only reading calls an emptying body an advertisement.

The guard therefore sits in `planAttr` (`reactor/forward_build.go`), the single
place a handler is called. It reads `advertiseGate`, a lazy memoized wrapper
over `wireu.PayloadAdvertisesNLRI`. That is still the one definition.
`buildModifiedPayload` reports "nothing to apply" when every code was refused, so
a relayed withdrawal keeps the zero-copy forward instead of paying a copy for a
change that did not happen.

### Tests

| Test | File | Proves |
|------|------|--------|
| `TestRelayCreatesNoAttributeOnABodyAdvertisingNothing` | `reactor/forward_build_withdraw_shape_test.go` | 4 producers x 3 bodies (withdraw-only, legacy EoR, MP_UNREACH-only), each paired with an advertising control |
| `TestRelayStillRewritesAnAttributeAWithdrawalCarries` | same | the create/modify boundary: an AS_PATH riding along is still transcoded |
| `TestRelayCreatesNoAttributeWhenEveryPrefixIsFiltered` | same | the `nlriOverride` shape, both polarities |
| `TestForwardNextHopSelfLeavesAWithdrawalUntouched` | same | the whole rail through `ForwardUpdate`, byte-identical withdrawal and EoR, plus a rewritten advertisement |
| `TestForwardReflectionLeavesAWithdrawalUntouched` | same | the same, on the iBGP RFC 4456 rail |
| `TestAdvertiseGateAgreesWithPayloadAdvertisesNLRI` | same | the gate and `wireu.PayloadAdvertisesNLRI` answer identically over seven shapes, so the O(1) shortcut cannot drift from the definition |
| `TestRelayCreatesTheAttributeWhenAnOverrideAddsNLRI` | same | an override that ADDS NLRI to a withdraw-only source makes the body an advertisement, so the attribute lands (review finding 2) |
| `TestPolicyChainCreatesNoAttributeOnAWithdrawal` | `reactor/filter_ordered_test.go` | the policy-chain entry point, both chains, plus an advertising control (review finding 3) |
| `relay-withdraw-nexthop-self.ci` | `test/plugin/` | the VERIFY tier: the running daemon relays a withdrawal to a `next-hop self` peer byte-identical, and rewrites the advertisement's next-hop on the same session |
| `53-relay-withdraw-nexthop-self-frr` | `test/interop/scenarios/` | FRR 10.3.1 accepts the relayed withdrawal with next-hop-self configured, and the advertisement's next-hop IS rewritten |
| `54-relay-withdraw-reflector-frr` | same | the iBGP witness: FRR accepts the reflected withdrawal, and the reflected advertisement DOES carry ORIGINATOR_ID and CLUSTER_LIST |

### Mutation evidence (all measured, 2026-08-04)

| Mutant | Result |
|--------|--------|
| `planAttr`: `if false && src == nil && !gate.advertises()` | 22 subtests RED across all four producers and both entry-point rails. Withdrawal `0004180a00000000` becomes `0004180a00000007400304c0000201` (a lone NEXT_HOP); legacy EoR `00000000` becomes `00000007400304c0000201`; the reflected withdrawal gains `8009040a000002 800a040a000001` |
| `planAttr`: drop the `src == nil` half, so the gate blocks modification too | `TestRelayStillRewritesAnAttributeAWithdrawalCarries` RED: "a rewrite of a PRESENT attribute must still happen on a withdrawal" |
| `advertiseGate.advertises`: disable the `nlriOverride` branch | `...WhenEveryPrefixIsFiltered/every-prefix-denied` RED: the rebuilt attrs are `{0x1: ORIGIN, 0x3: NEXT_HOP}`, "should not contain 0x3" |
| `buildModifiedPayload`: disable the nothing-planned early return | 12 subtests RED on `assert.Nil(result)`: the relay pays a byte-identical copy and loses zero-copy |
| `planAttr`: drop the guard, run interop 53 | RED at the NEGATIVE, positive still green (next-hop-self engaged). FRR 10.3.1: `[EC 33554482] 172.30.0.2 Missing well-known attribute AS_PATH.` and `[EC 33554455] 172.30.0.2(Unknown) rcvd UPDATE with errors in attr(s)!! Withdrawing route.` |
| `advertiseGate.advertises`: `|| len(g.payload) != 4`, so only a legacy EoR is guarded, run interop 54 **as first drafted** | **SURVIVED.** FRR accepted a withdrawal carrying ORIGINATOR_ID and CLUSTER_LIST without a word: its mandatory-attribute check fires only once NEXT_HOP or MP_REACH_NLRI is present. The scenario was REBUILT around a byte-exact witness -- see below |
| `planAttr`: drop the guard, run the rebuilt interop 54 | RED, and the failure names the stamped bytes: the reflected End-of-RIB arrives as `0000000E800904AC1E0003800A04AC1E0002` -- ORIGINATOR_ID plus CLUSTER_LIST, no NLRI, no longer an RFC 4724 Section 2 marker |

**Scenario 54 was measured vacuous once, then rebuilt.** Its first draft put FRR
on the receiving side. It asserted that FRR raised no attribute error over the
reflected withdrawal. The mutant above proved that assertion cannot fail. The
reason is structural in FRR, not a fixture accident.

FRR is now the SOURCE. It originates the prefix, and `check.py` drives
`no network` to withdraw it. The receiving witness is a raw `ze-test peer`, and it
asserts the reflected withdrawal byte for byte. Scenario 53 keeps the
complementary claim, that a conforming receiver ACCEPTS what ze relays, on the
rail where FRR can see the difference.

### Run 5: independent review of Follow-On 2 (2026-08-04)

Two independent reviewer subagents on Opus 5, neither of which wrote the code: A
over the production change, B over the tests and interop. Zero BLOCKER survives;
every ISSUE was fixed and mutation-verified. Artifact:
`tmp/review/relay-withdraw-attribute-gate-<session>.md`.

| # | Severity | Finding | Action |
|---|----------|---------|--------|
| 1 | BLOCKER | `RFC6793-4.2.2-1 positive` on `TestRelayStillRewritesAnAttributeAWithdrawalCarries` is a false compliance claim. The test hand-builds the narrowed AS_PATH and never runs `ASPathEdit.recordTranscode`, so it is a tautology with respect to that id | fixed: tag removed with a marker recording why. `ze-rfc-check` green at 3251 tags |
| 2 | ISSUE | The gate corrected only the EMPTYING direction of `nlriOverride`. A NON-EMPTY override can ADD NLRI to a withdraw-only source, so the built body advertised while the policy's attribute went missing, under `modifyFailureNone` | fixed: both directions. `TestRelayCreatesTheAttributeWhenAnOverrideAddsNLRI`, mutant-killed |
| 3 | ISSUE | The policy-chain driver was never given a withdraw-only body, so producer 4 was proven only at the helper | fixed: `TestPolicyChainCreatesNoAttributeOnAWithdrawal`, both chains plus a control. Mutant stamps `4005 04 000000c8` |
| 4 | ISSUE | Every verify-tier proof was a unit test; both daemon witnesses were nightly interop | fixed: `test/plugin/relay-withdraw-nexthop-self.ci`. Mutant reddens it with `+ NEXT_HOP: 127.0.0.2 (unexpected)` |
| 5 | ISSUE | The gate re-parsed the header and re-walked the attribute section `spans` already indexes, per destination | fixed: reads `attrEnd` plus `spans.Find`, so it costs no walk. `TestAdvertiseGateAgreesWithPayloadAdvertisesNLRI` pins it to the shared predicate over seven shapes |
| 6 | ISSUE | `decideStaleReadvertise`'s comment was falsified by the new early return | fixed: names both producers of that nil and why neither reaches it |
| 7 | MINOR | Two comments each claimed to be "the ONE legitimate nil" | fixed |
| 8 | MINOR | Three sites claimed FRR says "Missing well-known attribute NEXT_HOP"; measured was AS_PATH | fixed at all three, with the scenario named |
| 9 | NOTE | An op removing MP_REACH beside an empty `nlriOverride` would let a create land on a body ending with no NLRI | accepted: no production producer removes code 14 |

### Fixtures corrected, not assertions

Fifteen fixtures in three reactor test files relayed an UPDATE with NO NLRI, and
asserted that an egress rule ADDED an attribute to it. They were withdraw-only
UPDATEs claiming to be advertisements, the shape `aspath_slot_test.go` carried
before `79b46ef60`. Each gained an NLRI prefix, and no assertion changed.

`TestProgressiveBuildWithdrawnPreserved` is the one exception: it is a genuine
withdrawal, so its modification moved from CREATING an OTC attribute to
REWRITING the ORIGIN the fixture already carried. The withdrawn-bytes assertion
it exists for is untouched. None of the fifteen is RFC-tagged.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Relayed UPDATEs carry one NLRI-bearing field | functional `.ci` | `test/plugin/rfc7606-relay-one-field.ci` asserts the withdraw-only frame byte for byte: 20/20 green, and 4/4 FAIL with the mixed frame when either relay branch is reverted |
| Same, at the unit level | mutation | 8 mutants, all killed: wire fast path, `SplitCompliant` degenerated to `Split`, MP attributes uncounted, both `> 1` thresholds, the cache, and each of the two reactor branches |
| The zero-copy path survives for single-field UPDATEs | identity assertion plus benchmark | `TestForwardCompliantShapeKeepsZeroCopy` compares `&body[0]` with `&result.rawBodies[0][0]`; `TestSplitWireUpdateCompliantShapeUntouched` asserts the same pointer comes back; `BenchmarkMixesNLRIFields` measures 3.3ns per destination |
| The check is paid per message, not per peer | timing ratio plus benchmark | `TestWireUpdateMixesNLRIFieldsCachedPerMessage` fails when the cache is removed (warm 3ns becomes 41ns); the first version of that test asserted allocations and proved nothing, since neither side allocates |
| Interop | FRR | scenario 47: FRR installs both the replayed route (discriminates the NEXT_HOP dedup -- RED without it) and the live-split announce (proves §5.1 split output accepted). Split-vs-unsplit is not peer-discriminable (RFC 7606 third bullet); that lives in unit + `.ci` |
| Gate | ze-rfc-check | green, 2543 tags resolved, up from 2535 |
| Lint | ze-lint-changed | 0 findings in the three changed packages |
| No regression | package tests | `internal/component/bgp/...` 81 packages ok; the one red is the known missing-build-tag artifact in `bgp/config`, green with the ze_core plus ze_ssh tags |

## Review Gate

Author mutation testing first; independent review recorded under Run 2.

### Run 1 (author mutation pass)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `TestWireUpdateMixesNLRIFieldsCached` asserted zero allocations to prove the cache existed. Removing the cache entirely still passed, because neither side allocates. A test that claims to pin a property it cannot observe is worse than no test | `wireu/shape_rfc7606_test.go` | fixed: replaced with a cold/warm timing ratio, verified to fail when the cache is removed |
| 2 | ISSUE | The mixed fixture carries all four fields, so an off-by-one making the check fire only at three or more survived inside the wireu package; it was killed only by a reactor test in another package | same | fixed: `TestWireUpdateMixesNLRIFieldsBoundary` pins 1 against 2 |
| 3 | NOTE | The `.ci` header claimed AC-6 ordering, which a `.ci` cannot assert: `Checker.check` matches an arriving message against every pending rule, not just the head | `test/plugin/rfc7606-relay-one-field.ci` | fixed: claim removed with the reason recorded; ordering is pinned by unit tests |
| 4 | NOTE | The `.ci` was flaky at roughly 75%, and the closest existing test (`bgp-rs-reactor-fastpath.ci`) fails about 1 in 6 on an unmodified tree for the same reason | same | fixed: dropping `rs-fast-path` gave 20/20 while keeping 4/4 on the mutant; the extra frame is described in the header and left to its own investigation |

### Fixes applied
- All four, each re-verified by re-running the mutant that motivated it.

### Run 2 (independent review of the §5.1 relay-split change)
One subagent over the whole diff. Findings and outcomes:
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | splitting a mixed UPDATE whose MP_UNREACH is AFI/SAFI-only manufactured an RFC 4724 End-of-RIB | `wireu/split.go` | fixed: `mpUnreachHasNLRI` guard; two tests in `split_eor_test.go`, the manufacture test RED without the guard |
| 2 | MINOR | stale comment (`split.go`) whose premise the fast-path change falsified | same | fixed as part of the guard comment |
| 3 | NOTE | duplicate MP attributes counted but the parsed splitter cannot honour a second one | `message/rfc7606_shape.go` | accepted: unreachable from the receive path (session-reset on `mpReachCount > 1`); the wire path handles it |
| 4 | NOTE | AC-4 "no parse" is destination 2..N only; the benchmark measures the warm side | `wire_update.go` | accepted; corrected the claim to "one bool read for destinations 2..N" understanding |

### Run 3 (independent review of the two found-via-interop fixes)
One subagent over the NEXT_HOP-dedup and EoR fixes. No BLOCKER, no ISSUE.
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | MINOR | the strip-safety comment cited only `ErrNextHopUnset`; the real invariant also depends on callers, because `resolveNextHop` (peer.go) deliberately passes an invalid explicit next-hop through without erroring (a fail-open guard, pinned by `TestResolveNextHop_ExplicitInvalid`) | `reactor_api_batch.go` | fixed: added a local `nextHop.IsValid()` fail-closed guard at the strip (leaves the block untouched for an invalid next-hop rather than dropping the stored NEXT_HOP), a discriminating test (`...InvalidNextHopKeepsBlock`, mutant-killed with a MED-after-NEXT_HOP fixture), and an accurate comment. `resolveNextHop`'s deliberate behavior left unchanged |
| 2 | NOTE | MP_REACH re-origination cannot preserve an RFC 2545 link-local next-hop | same | pre-existing, not introduced here; out of scope |
| 3 | NOTE | 4K build buffer vs 64K ExtendedMessage ceiling | same | pre-existing; the strip only lowers occupancy |

### Run 4 (independent review of the withdraw-only relay-shape follow-on, 2026-08-04)

Two independent reviewer subagents on Opus 5, neither of which wrote the code:
A over the production change, B over the tests and test infrastructure. Zero
BLOCKER. Every ISSUE fixed and mutation-verified. Artifact:
`tmp/review/withdraw-only-relay-shape-<session>.md`.

| # | Severity | Finding | Action |
|---|----------|---------|--------|
| 1 | ISSUE | Routing a withdraw-only UPDATE to `recordTranscode` lost the RFC 6793 Section 4.1 AS4_PATH drop that `recordPrepend` performed through `recordAS4Path`. `recordTranscode` returns at `SrcASN4 == DstASN4` before reaching it, which is right for the RFC 7947 route-server rail and wrong for an EBGP peer, so a received AS4_PATH would travel between two NEW speakers | fixed: `recordWithdrawOnly` wraps `recordTranscode` and adds the equal-width drop, gated on `DstASN4` so an OLD destination still gets the attribute it is meant to have. `TestASPathSlotWithdrawOnlyDropsAS4Path`, two subtests; the `if false &&` mutant reddens the NEW-NEW one |
| 2 | ISSUE | Three new comments attributed the prepend to "RFC 4271 Section 9.1.2", which is Phase 2 Route Selection. The clause quoted is Section 5.1.2 b. The file header carried the same miscitation before this change | fixed: all five sites in `wireu/aspath_slot.go` and the one in `role/otc.go`; `grep "9\.1\.2"` over both returns nothing |
| 3 | ISSUE | The same defect CLASS survives on three other operation sites, so "no attribute is synthesized onto a withdraw-only relay" is not universally true | homed, not fixed: each is a different producer on a rail this spec does not own, and none is reached on the eBGP default path. See "Found while doing this, NOT fixed here" above, where each producing handler is now named |
| 4 | ISSUE (hot path) | `PayloadAdvertisesNLRI` re-reads the two length fields `aspathAttrSection` just read, and walks the TLVs for an MP-only advertisement although `BuildSpanIndex` ran two statements earlier | accepted: allocation-free, and O(1) for native IPv4 NLRI because the trailing-NLRI test answers first. A second spans-based implementation would re-create the duplicate definition this change removed, and `WireUpdate.MixesNLRIFields` answers a different question over a different type |
| 5 | ISSUE | `TestASPathSlotRSClientSkipsPrepend` kept a payload with no NLRI while the other seven fixtures gained one, so its RFC 7947 Section 2.2.2 assertion passed through the new withdraw-only branch: deleting the `len(in.Prepend) == 0` half of the condition left it green | fixed: same one-line fixture change, reason recorded in the test. The `if false &&` mutant now reddens both subtests |
| 6 | ISSUE | Scenario 51's rewritten negative changed KIND, not only regex. "The receiver raised no attribute error" is narrower than "no OTC Attribute was added", and it carried no measured mutant | fixed: the docstring records both measured mutants with FRR's verbatim output, states what the assertion does and does NOT observe, and names `role-otc-fwd-withdraw.ci` as what closes the remainder. The tag text now states the observation, not the inference |
| 7 | MINOR | `checker_match_test.go` still described the deleted length rule | fixed |
| 8 | NOTE | Dead branch in the new `IsEOR` (`len(m.Body) < 2+withdrawnLen+2` is unreachable) | fixed: removed with the reason stated |
| 9 | NOTE | `internal/test/peer.Message.IsEOR` duplicates the semantics of production `wireu.WireUpdate.IsEOR` and the two can drift; `isBareMPUnreach` requires exactly one MP_UNREACH while `(*Update).IsEndOfRIBAnyFamily` accepts a run | accepted: the test peer decodes without importing the production wire package, and no producer emits the multi-attribute form |

Reported clean by both reviewers: the predicate's agreement with the role walk it
replaced on every input including malformed ones; MP_REACH, End-of-RIB, mixed and
route-server behavior; fail-closed guard quality; the `IsEOR` narrowing (nothing
real is refused, and Ze's own `message.BuildEOR` output still classifies as an
End-of-RIB); non-vacuity of all four new tests; both `inject.msg` barriers; and
all eleven `rfc-test-change-approved:` markers, none of which weakens.

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE -- all ISSUEs fixed and mutation-verified; NOTEs recorded
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `message/rfc7606_shape.go` + `_test.go` | yes | `git status` shows both as new |
| `wireu/shape_rfc7606_test.go`, `wireu/shape_bench_test.go` | yes | same |
| `reactor/forward_body_rfc7606_test.go` | yes | same |
| `test/plugin/rfc7606-relay-one-field.ci` | yes | runs under `ze-test bgp plugin rfc7606-relay-one-field` |
| `test/interop/scenarios/47-rfc7606-relay-shape-frr` (+ `interop.py` injector sidecar) | yes | 4/4 green; RED when the NEXT_HOP dedup is reverted |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | mixed same-context UPDATE that fits is split | reactor test green; `.ci` asserts the withdraw-only frame byte for byte and fails 4/4 on revert |
| AC-2 | same across contexts | `TestForwardSplitsMixedShapeAcrossContextsThatFits`, red before the change |
| AC-3 | receive unchanged | no receive-path file was modified; reactor suite green |
| AC-4 | zero-copy kept | `require.Same(t, &body[0], &result.rawBodies[0][0])` |
| AC-5 | EoR untouched | three tests, one per layer |
| AC-6 | withdrawals first | two unit tests; deliberately not claimed by the `.ci` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| a peer relays a mixed UPDATE through the daemon | `test/plugin/rfc7606-relay-one-field.ci` | yes: 20/20 green, 4/4 red with the relay branches reverted and the binary rebuilt |
| every new exported symbol has a non-test caller | - | `NLRIBearingFieldCount` called by both `MixesNLRIFields` implementations and by tests; `(*Update).MixesNLRIFields` by `SplitCompliant` and `buildFwdBody:101`; `(*WireUpdate).MixesNLRIFields` by `SplitWireUpdate:40` and `buildFwdBody:60`; `SplitCompliant` by `fwdSplitParsedUpdate` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | **dissolved, not confirmed** | the assumption only mattered if the check were charged per relay. Caching removes that: 3.3ns per destination against 51.7ns recomputed, allocation-free either way. No feed measurement was taken, and none would change a decision here |
| A-2 | confirmed | the forward loop passes the same `*wireu.WireUpdate` to `buildFwdBody` per destination, and the verdict is cached behind `shapeOnce` on that value; `TestWireUpdateMixesNLRIFieldsCachedPerMessage` fails when the cache is removed |
| R-1 | mitigated as planned | the single-field common case keeps the zero-copy append, asserted by pointer identity in two packages |
| R-2 | accepted, no change needed | `fwdSupersedeKey` (`forward_pool.go`) is a content hash over all bodies; splitting changes the key but not its meaning, and the oversize path already produced multiple bodies |
| R-3 | resolved, no change needed | `fwdIsWithdrawal` (`forward_pool.go`) scans every body and returns false if ANY carries an announcement. A mixed UPDATE classified as an announcement before, and its split halves still do |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| rfc-status says two gaps for RFC 7606 | matches `grep -c '{gap' rfc/short/rfc7606.md` = 2 | yes |
| the audit verdict names the tests that prove it | `rfc/audit/rfc7606.json` binds 8 tagged units; `make ze-rfc-check` green at 2543 tags | yes |
| mp-nlri-ordering.md enforcement table | each row traced to the function named; three source anchors added | yes |
| `check_doc_links.py` | all corpus path references resolve | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only
