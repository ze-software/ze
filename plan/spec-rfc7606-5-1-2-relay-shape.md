# Spec: rfc7606-5-1-2-relay-shape

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | - |
| Updated | 2026-07-20 |

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
