# Spec: rfc4271-med-across-as

| Field | Value |
|-------|-------|
| Status | done |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfc4271-med-across-as.md` |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**The requirements.** RFC 4271 Section 5.1.4 carries two obligations ze does not
meet, and two more are exempted only because of them.

| ID | Level | Requirement | State |
|----|-------|-------------|-------|
| `RFC4271-5.1.4-1` | MUST NOT | MULTI_EXIT_DISC received from a neighboring AS MUST NOT be propagated to other neighboring ASes | `{gap}` |
| `RFC4271-5.1.4-4` | MUST | A speaker MUST implement a mechanism, based on local configuration, that allows MULTI_EXIT_DISC to be removed from a route | `{gap}` |
| `RFC4271-5.1.4-2` | MUST | Removal MUST happen before the route is used in Phase 2 of the decision process | `{not-applicable}`, conditional on 5.1.4-4 |
| `RFC4271-9.1.2.2-2` | MUST | If MED is removed before IBGP readvertisement, the optional MED comparison MUST run only among EBGP-learned routes | `{not-applicable}`, conditional on 5.1.4-4 |

**What ze does.** No producer removes MULTI_EXIT_DISC before advertising to
another AS. Both readvertisement encoders write a stored MED unconditionally with
no AS gate (`internal/component/bgp/reactor/peer_rib_routes.go`,
`internal/component/bgp/reactor/reactor_wire.go`), and the forwarding rails
perform community strips only, never an attribute-4 removal.

**Why the two exemptions are not exemptions.** `ai/rules/rfc-compliance.md` names
this shape: a `{not-applicable}` whose reason is that ze has no producer for X at
all is often the violation of a separate MUST requiring X to exist. Both
`{not-applicable}` rows cite the absence of the mechanism 5.1.4-4 requires, so all
four requirements move together.

**MED is NOT the twin of LOCAL_PREF, and copying that shape would be wrong.**
`applyFactsLocalPref` (`internal/component/bgp/reactor/forward_local_pref.go`)
enforces Section 5.1.5 by suppressing LOCAL_PREF for every external peer, and it
runs AFTER the egress filter pass so that `filterapi.LastSetOrSuppress` makes its
Suppress beat a filter's Set. Its comment states the reason: the prohibition is
not a policy a filter may override.

Section 5.1.4 forbids something narrower. A MED that ze itself sets toward a peer
is the attribute's entire purpose, and an operator filter that sets one is
legitimate. What MUST NOT happen is a MED RECEIVED from one neighboring AS being
carried onward to another. So the MED rule suppresses the RECEIVED value while
letting a locally-set one through, which is the reverse of the twin's precedence.
A design that blanket-suppresses attribute 4 on external egress would be
conformant with 5.1.4-1 and would break MED as a feature.

**Why this is release-gating.** MED influences a peer's route selection. Leaking
one AS's MED into another makes a third network choose between paths on a metric
its neighbor never intended it to see, and the leak is silent at both ends. Same
class as the RFC 7911 Path Identifier defect fixed in `8c5bcc191`: an attribute
crossing a boundary it must not cross.

**Not in scope.** MED comparison rules in the decision process beyond
`RFC4271-9.1.2.2-2`, and any change to how MED is used in best-path selection
among routes from one AS.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - both rails reach one egress transform
  → Constraint: a suppression recorded once applies on the live rail and the replay rail alike

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Sections 5.1.4 and 9.1.2.2
  → Constraint: 5.1.4-1 governs a RECEIVED MED propagated to ANOTHER neighboring AS, not MED on external sessions generally

**Key insights:** (minimal context to resume after compaction)
- Two requirements are exempted only because a third is unimplemented.
- The rule is about provenance, not about the destination alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/forward_local_pref.go` - `applyFactsLocalPref`
  records `AttrModSuppress` for a non-IBGP destination, guarded by
  `localPrefAllowedTo`, `payloadHasLocalPref` and `modsTouchLocalPref`. It runs
  after the filter pass so its Suppress beats a filter's Set
  → Decision: the MECHANISM is right for MED, the PRECEDENCE is not. A filter
    that sets MED is the operator originating one, which 5.1.4 permits
  → Constraint: `baseHasLocalPref` is computed once per UPDATE, not per
    destination, because recording unconditionally forces every route to every
    external peer onto the payload-rebuild path and defeats the route-server fast
    path. MED must respect the same cost
  → Constraint: `payloadHasLocalPref` reads the attribute SECTION, not raw
    payload bytes, so an NLRI byte is never mistaken for an attribute. MED needs
    the same care
- [ ] `internal/component/bgp/reactor/peer_rib_routes.go`, `reactor_wire.go` - both
  readvertisement encoders write a stored MED with no AS gate
  → Constraint: confirm both at source; the gap text names them

**Behavior to preserve:**

| Behavior | Where | Why it must not change |
|----------|-------|------------------------|
| A MED ze sets toward a peer reaches that peer | the egress transform | That is what MED is for. Suppressing it breaks the feature while satisfying the letter of 5.1.4-1 |
| An operator filter that sets MED still takes effect | the egress filter pass | Unlike LOCAL_PREF, this is legitimate origination, not an override of a prohibition |
| MED reaches an iBGP destination unchanged | the egress transform | 5.1.4-1 constrains propagation to another NEIGHBORING AS |
| The route-server fast path stays off the rebuild path when nothing changes | `applyFactsLocalPref`'s once-per-UPDATE pattern | Recording unconditionally is the cost that pattern exists to avoid |
| LOCAL_PREF behavior is untouched | `forward_local_pref.go` | The sibling must not regress |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
An UPDATE received from a peer in one AS and advertised to a peer in another, on
the live forward rail or the peer-up replay rail, plus the two readvertisement
encoders.

### Transformation Path
1. A route carrying MULTI_EXIT_DISC arrives from a neighboring AS. Its provenance
   is what the rule turns on, so that fact must survive to egress.
2. The egress transform asks, per destination, whether this destination is a
   different neighboring AS from the one the MED came from.
3. Where it is, the RECEIVED attribute 4 is suppressed.
4. A locally-set MED, whether from configuration or from an egress filter, is
   written normally.
5. A local configuration surface can request removal independently, which is the
   mechanism 5.1.4-4 requires, and it runs before Phase 2 of the decision process.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| AS to AS | a received attribute 4 does not cross | No |
| Live rail to replay rail | one suppression, both rails | No |
| Config to egress | the removal mechanism is operator-driven | No |

### Integration Points
- `applyFactsLocalPref` (`internal/component/bgp/reactor/forward_local_pref.go`) - the mechanism to reuse, with different precedence.
- The two readvertisement encoders named above.

### Architectural Verification
- **Registration over hardcoding.** The destination's AS and the route's source AS both come from state the session registry already holds. No core or shared package spells a peer name, and nothing re-enumerates what that registry carries (`ai/rules/plugins.md`, `ai/rules/evidence.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|--------------------------------|----------|--------------|--------|
| A-1 | The egress transform can tell a RECEIVED MED from a locally-set one | `peerForwardFacts` already carries per-destination facts and the filter accumulator records ops | the rule cannot be expressed at this altitude and the design moves upstream | read `peerForwardFacts` and the accumulator | confirmed. Both rails relay another speaker's UPDATE, so a MED in the source payload is received by construction. A locally-set one arrives as an accumulator Set op or as the export chain's wire override, and the override is told apart by comparing its value against the source |
| A-2 | The source AS of a route survives to egress | the forward rails already key on the source peer | provenance must be threaded, which enlarges the change | read what the rails carry about the source | not needed. `forwardSourceInfo` carries `isIBGP` and no source AS, and the rule does not need one: refusing an OPTIONAL attribute toward the AS it came from is conformant, so the destination test is "external and not an RS client". Nothing is threaded upstream |
| A-3 | Suppressing attribute 4 uses the accumulator LOCAL_PREF uses | `AttrModSuppress` is recorded by `applyFactsLocalPref` | MED needs its own path, which would be a second mechanism | read the accumulator's API | confirmed. `mods.Op(4, AttrModSuppress, nil)`, applied by `genericAttrSetHandler` |
| A-4 | No existing test asserts a received MED survives to another AS | the gap says no producer removes it, so a test may pin the leak | that test is wrong and is corrected, not worked around | grep the encode and plugin suites for a MED expectation on eBGP egress | confirmed, with one near miss. `forward_dedup_test.go` uses MED as its base-identity needle, and its destinations are iBGP (`LocalAS == PeerAS == 65000`), so the fixture is untouched |
| A-5 | RFC 7947 Section 2.2.3 exempts a route server client, and that exemption is load-bearing | "Contrary to Section 5.1.4 of [RFC4271] ... this attribute SHOULD be propagated to other route server clients"; `RFC7947-x-3` is a gated MUST in Ze's ledger | a blanket external strip closes one gated MUST by breaking another | read RFC 7947 Section 2.2.3 and `rfc/short/rfc7947.md` | confirmed. `medPropagationAllowedTo` gates on `facts.rsClient` |

### Risks
| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Blanket-suppressing MED on external egress, satisfying 5.1.4-1 while breaking MED as a feature | a peer stops seeing the MED ze is configured to send | the test is provenance plus destination, never destination alone |
| R-2 | Overriding an operator filter that sets MED, by copying the twin's precedence | a policy chain's MED never reaches the wire | the suppression targets the received value; a filter Set is legitimate origination |
| R-3 | Forcing every external route onto the payload-rebuild path | forward throughput regresses on a route server | follow the once-per-UPDATE pattern the twin uses |
| R-4 | The propagation rule and the configured mechanism become two suppression points | two places decide whether attribute 4 is written | one suppression point, two reasons to trigger it |

## Blast Radius

Every eBGP session ze advertises to. MED is wire-visible and changes which path a
neighbor selects, so a wrong suppression is as operator-visible as the leak. The
LOCAL_PREF sibling is the control: it has shipped this mechanism without incident.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | Feature Code | Test |
|-------------|--------------|------|
| Route from AS X readvertised to AS Y | wire → egress transform → received MED suppressed | `test/plugin/med-not-propagated-across-as.ci` |
| Ze's own MED toward a peer | config → egress transform → MED written | `test/plugin/med-locally-set-reaches-peer.ci` |
| Operator configures MED removal | config → removal mechanism → egress | `test/plugin/med-removal-configured.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A route with MED received from AS X, readvertised to a peer in AS Y | the UPDATE toward AS Y carries no MULTI_EXIT_DISC |
| AC-2 | Ze configured to send its own MED to a peer in AS Y | that MED is written, because 5.1.4-1 governs a RECEIVED value only |
| AC-3 | An egress filter that SETS MED on a route toward AS Y | the filter's value is written. Unlike LOCAL_PREF, this is legitimate origination and must not be overridden |
| AC-4 | The same received route readvertised to an iBGP peer | MED is unchanged, because iBGP is not another neighboring AS |
| AC-5 | An operator configures MED removal for a route or peer | MED is removed regardless of destination, which is the mechanism 5.1.4-4 requires |
| AC-6 | A route whose MED was removed, entering the decision process | removal happened before Phase 2 (`RFC4271-5.1.4-2`) |
| AC-7 | MED removed before iBGP readvertisement | the optional MED comparison runs only among eBGP-learned routes (`RFC4271-9.1.2.2-2`) |
| AC-8 | The same route on the live rail and the peer-up replay rail | both carry the same attribute set |
| AC-9 | A route to an external peer that needs no MED change | it does not enter the payload-rebuild path, so the route-server fast path is preserved |
| AC-10 | LOCAL_PREF behavior | unchanged, proven by its existing tests still passing |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with two transit providers and readvertises between them | wire → egress transform → received MED suppressed | `med-not-propagated-across-as.ci` |
| 2 | Sets a MED toward one provider to steer inbound traffic | config → egress transform → MED written | `med-locally-set-reaches-peer.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestForwardSuppressesReceivedMEDToAnotherAS` | `internal/component/bgp/reactor/forward_med_test.go` | AC-1 | [ ] |
| `TestForwardWritesLocallySetMED` | same | AC-2 | [ ] |
| `TestForwardKeepsFilterSetMED` | same | AC-3 | [ ] |
| `TestForwardKeepsMEDWithinAS` | same | AC-4 | [ ] |
| `TestMEDRemovalMechanismIsConfigurable` | same | AC-5 | [ ] |
| `TestForwardMEDStaysOffRebuildPathWhenUnchanged` | same | AC-9 | [ ] |
| `TestLocalPrefSuppressionUnchanged` | `forward_local_pref_test.go` | AC-10 | [ ] |

### Boundary Tests (numeric inputs)
| Test | Input | Expected |
|------|-------|----------|
| received MED at 0 | 0 | suppressed toward another AS; 0 is a value, not an absence |
| received MED at max | 2^32-1 | suppressed the same way |

### Functional Tests
| Test | File | Validates |
|------|------|-----------|
| a received MED does not cross an AS boundary | `test/plugin/med-not-propagated-across-as.ci` | AC-1 |
| a locally-set MED reaches the peer | `test/plugin/med-locally-set-reaches-peer.ci` | AC-2 |
| the configured removal mechanism works | `test/plugin/med-removal-configured.ci` | AC-5 |

### Interop Tests (Scope: protocol)
| Test | Peer | Validates |
|------|------|-----------|
| a peer daemon in another AS receives no propagated MED, and does receive ze's own | FRR or BIRD | AC-1 and AC-2 judged by a foreign RIB |

## Files to Modify

| File | Change |
|------|--------|
| a sibling beside `internal/component/bgp/reactor/forward_local_pref.go` | the MED rule, reusing the accumulator with its own precedence |
| `internal/component/bgp/reactor/peer_rib_routes.go` | the readvertisement encoder stops writing a received MED across an AS boundary |
| `internal/component/bgp/reactor/reactor_wire.go` | the second encoder, same change |
| `rfc/short/rfc4271.md` | 5.1.4-1 and 5.1.4-4 lose their `{gap}`; 5.1.4-2 and 9.1.2.2-2 lose their mechanism-absence `{not-applicable}` |
| `docs/features/rfc-status.md` | the RFC 4271 row's Remaining count and prose |

## Files to Create

| File | Purpose |
|------|---------|
| the MED rule and its unit tests | AC-1 through AC-5, AC-9 |
| the three `.ci` named above | AC-1, AC-2, AC-5 |
| an interop scenario | AC-1 and AC-2 against a foreign RIB |

### Integration Checklist

| Surface | Answer |
|---------|--------|
| Functional test for new behavior | Yes, the three `.ci` above |
| Interop test | Yes, protocol scope |
| YANG schema and validation | Yes, if the removal mechanism needs a config leaf. `ai/patterns/config-option.md` governs its shape |
| RFC status ledger | Yes, `rfc/short/rfc4271.md` and `docs/features/rfc-status.md` |
| CLI, completion, env var, doctor, metrics | N-A unless the removal mechanism adds an operator surface, in which case the CLI and completion rows apply |

### Documentation Update Checklist (BLOCKING)

| Category | Answer |
|----------|--------|
| 3. Config syntax | Yes, if a removal leaf is added: `docs/guide/configuration.md` |
| 9. RFC compliance | Yes. `rfc/short/rfc4271.md`, `docs/features/rfc-status.md` |
| 12. Internal architecture | Yes. The forward-rail doc gains the MED rule beside the LOCAL_PREF one, including why their precedence differs |
| 16. Source anchors | Yes. Grep `docs/` for the changed reactor files |
| all others | N-A. No API, plugin SDK, wire-format or comparison change |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the three `.ci` exist and fail; the decision point is reached and suppresses nothing yet.
2. **Phase: Provenance** - establish that the egress transform can tell a received MED from a locally-set one, and that the source AS survives to egress (A-1, A-2). **If it cannot, STOP and report: the design moves upstream and that is a decision, not an implementation detail.**
3. **Phase: The propagation rule** - AC-1, AC-2, AC-3, AC-4. Suppress the received value, write a locally-set one, leave a filter's Set alone, leave iBGP alone.
4. **Phase: The configured mechanism** - AC-5. 5.1.4-4 requires a mechanism driven by local configuration, independent of the propagation rule.
5. **Phase: Ordering and cost** - AC-6, AC-7, AC-9.
6. **Phase: Rails and the sibling** - AC-8, AC-10.
7. **Phase: Ledger** - two `{gap}` and two `{not-applicable}` replaced by tagged proof, `make ze-rfc-check` green.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Provenance, not destination alone | The test is "received from a different neighboring AS", never "destination is external" (R-1) |
| Precedence is NOT the twin's | A filter that sets MED wins; a filter that sets LOCAL_PREF does not. Both behaviors are correct and they differ (R-2) |
| One mechanism | MED and LOCAL_PREF share the accumulator. A second suppression path is the failure (`ai/rules/no-layering.md`) |
| Fast path preserved | A route needing no MED change does not enter the payload rebuild (R-3, AC-9) |
| Zero is a value | A received MED of 0 is suppressed like any other value |
| Exemptions retired | 5.1.4-2 and 9.1.2.2-2 no longer cite a missing mechanism |
| A test that pinned the leak | If one exists it is corrected, and the correction is stated (`ai/rules/rfc-compliance.md`) |
| Registration over hardcoding | The AS relationship comes from negotiated session state, not an enumeration |

### End-to-End User Stories (filled)

The two rows above are the filled set.

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| A received MED does not cross an AS boundary | the `.ci` passes and fails when the suppression is reverted |
| A locally-set MED still reaches the peer | its own `.ci`, which the blanket-suppression mistake would fail |
| The removal mechanism exists and is configurable | `RFC4271-5.1.4-4` carries a tagged test |
| All four requirements carry real dispositions | `grep -n "5.1.4\|9.1.2.2" rfc/short/rfc4271.md` shows no `{gap}` and no mechanism-absence `{not-applicable}` |
| The ledger agrees | `make ze-rfc-check` green |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Information leakage | MED reveals a neighbor's internal metric. Leaking it across an AS boundary is the defect, so confirm no other egress path re-introduces it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Provenance is not available at egress | STOP and report. The design moves upstream |
| A test asserts a received MED survives to another AS | That test is wrong. Correct it and say so |
| 3 fix attempts failed | STOP. Report all three approaches. Ask the user |

## Design Insights

- Two requirements were exempted because a third was unimplemented. An exemption
  resting on an absence is a claim about the absence, not about the rule.
- A sibling obligation is not a template. LOCAL_PREF and MED sit one section
  apart and their precedence against an operator filter is opposite, because one
  prohibits an attribute outright and the other prohibits only relaying someone
  else's value.
- A third RFC governs the same attribute. RFC 7947 Section 2.2.3 names Section
  5.1.4 and reverses it for a route server client, and `RFC7947-x-3` is already a
  gated MUST here. Closing 5.1.4-1 without that gate would have closed one
  obligation by breaking another, with `TestReactorForwardRSTransparent` as the
  only warning.

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Reuse the LOCAL_PREF mechanism, not its precedence | One accumulator keeps the rails agreeing; the differing precedence follows from what each RFC section prohibits |
| The propagation rule and the configured mechanism trigger one suppression | Two suppression points would let them disagree (R-4) |

## Known Limitations

| Limitation | Why it is accepted |
|------------|-------------------|
| MED comparison rules beyond 9.1.2.2-2 are untouched | Out of scope; changing best-path selection is a separate decision |

## RFC Documentation (Scope: protocol)

`rfc/short/rfc4271.md`, Sections 5.1.4 and 9.1.2.2. Four requirements:
`RFC4271-5.1.4-1`, `RFC4271-5.1.4-4`, `RFC4271-5.1.4-2`, `RFC4271-9.1.2.2-2`.

## Checklist

### Goal Gates (MUST pass)
- [ ] `make ze-rfc-check` green with all four requirements carrying real dispositions
- [ ] `make ze-verify` green, or scoped evidence with attribution

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Review Gate 0 BLOCKER / 0 ISSUE

## Implementation Summary

### What Was Implemented

Two mechanisms, one suppression point.

The propagation rule, RFC 4271 Section 5.1.4's MUST NOT, landed in commit
`f9b42734e`. `applyFactsMED` (`internal/component/bgp/reactor/forward_med.go`)
records one `filterapi.AttrModSuppress` on attribute 4 when the source payload
carried a metric and the destination is a different neighboring AS. It turns on
provenance: a metric ze sets, and a metric an egress filter sets, both reach the
peer. `medPropagationAllowedTo` reads `facts.rsClient`, so RFC 7947 Section 2.2.3
keeps a route-server client's metric. Both forward rails call it
(`reactor_api_forward.go`, `forward_rs.go`).

The configured removal, Section 5.1.4's MUST, is this closure's work.
`ExtractMEDRemoveOps` (`internal/component/bgp/reactor/filter_delta.go`) converts
the `med-remove` directive into the SAME suppression, so the two triggers cannot
disagree about what a removal does to the wire. It is called from ONE site, the
ingress chain (`runIngressPolicyChain`, `filter_ordered.go`), and that omission
on the export side is the conformance boundary: Section 5.1.4 requires the
removal before Decision Process phases 1 and 2, and Section 9.1.2.2 states that
comparing on a metric and then advertising the route without it causes route
loops. The operator surface is the `med-remove` boolean leaf of a modify policy's
set block (`yang/ze-filter-modify.yang`), parsed by `parseModifyDefs`
(`filter_modify/config.go`) and emitted by `appendMEDRemove`
(`filter_modify/filter_modify.go`), which refuses the directive on an export
chain and logs the RFC reason. `validateNoConflict` refuses `med-remove` beside
`set med`, `increment med` and `decrement med`.

`faMEDRemove` joins the filter-attribute enum (`filter_chain.go`) as a valueless
token beside `atomic-aggregate`, so it survives `applyFilterDelta` and
`formatFilterAttrs` without consuming the token after it.

The Review Gate added three things the first implementation did not have.
`medPresentOnWire` (`filter_delta.go`) keeps a route with no metric off the
payload-rebuild path, reading the WIRE because the filter text never names
MULTI_EXIT_DISC (Mistake Log). `filterAttrs.merge` (`filter_chain.go`) makes the
CHAIN's order decide between `med` and `med-remove`, so a later filter that sets
the metric cancels an earlier removal. `computeWireChanges` (`policy_dryrun.go`)
takes the direction, so `ze bgp policy dry-run` reports the removal on an import
chain and does not promise one on an export chain.

### Bugs Found/Fixed
- None in the product beyond the two RFC gaps this spec exists to close. Three
  defects were FOUND and journalled rather than fixed here, because none blocks
  this spec's goal: two in `plan/journal/test-against-broken-path.md` (a
  `ze-test peer` `conn_map` delivery defect, and `relay-withdraw-nexthop-self.ci`
  going red under another session's commit `480897faf`), and one in
  `plan/journal/reference-checked-claim-unchecked.md` (a stale anchor in
  `ai/digests/web.md` left by the templ migration `aa43dd4cc`, fixed here because
  it reddened `make ze-doc-test`).

### Documentation Updates
- `docs/architecture/bgp/egress-attribute-rules.md` -- the third trigger, why it
  is not on the egress rail, and a Proof table that gained an Interop column.
- `docs/guide/plugins.md` -- the `med-remove` paragraph, plus source anchors for
  `appendMEDRemove` and `ExtractMEDRemoveOps`.
- `docs/guide/configuration.md` -- the `med-remove` row in the modify set-leaf
  table, the import-only constraint, the mutual exclusion, and two source anchors.
- `docs/features/rfc-status.md` -- the RFC 4271 row already carries the MED
  coverage prose (committed with `f9b42734e`).
- `make ze-doc-test` exit 0.

### Deviations from Plan
- The spec's "Files to Modify" table named `peer_rib_routes.go` and
  `reactor_wire.go`, the two readvertisement encoders. Neither changed. They write
  a metric ze itself set, which Section 5.1.4 permits; the received-metric case
  reaches the wire through the forward rails, and one suppression there covers
  both. Recorded as a deviation rather than a gap: the encoders were read, and
  the rule they would have needed does not apply to what they emit.
- The removal mechanism landed on the modify policy's set block rather than on a
  per-peer leaf. A modify policy is already the operator's route-attribute
  surface, and a second surface would have been a second suppression point (R-4).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2 assumed the source AS must be threaded to egress | The rule needs no source AS. Refusing an OPTIONAL attribute toward the AS it came from is conformant, so the test is "external and not an RS client" | reading `forwardSourceInfo`, which carries `isIBGP` and no AS | A-2 recorded `not needed`; no provenance plumbing was written |
| approach | The review's "gate the removal on the route carrying a metric" fix first read the metric from the parsed filter TEXT (`modAttrs.has(faMED)`), on the reasoning that `AppendUpdateForFilter` renders every attribute it is given | The filter text never names MULTI_EXIT_DISC. `appendSingleAttr` (`internal/component/bgp/reactor/filter_format.go`) switches on `*attribute.MED` while `knownAttrParsers` builds the value form `attribute.MED`, so the case cannot match. The text gate refused EVERY configured removal | the new interop scenario, `test/interop/scenarios/61-med-remove-configured-gobgp`, which went red while every unit test stayed green: each one writes its update text by hand with `med N` in it | the gate moved to the wire (`medPresentOnWire`, `internal/component/bgp/reactor/filter_delta.go`), the unit test now asserts the text does NOT carry the metric, and the renderer defect is journalled in `plan/journal/silent-fall-through.md` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `RFC4271-5.1.4-1` MUST NOT propagate a received MED to another AS | Done | `applyFactsMED`, `internal/component/bgp/reactor/forward_med.go` | `{gap}` retired in `rfc/short/rfc4271.md`; both polarities tagged |
| `RFC4271-5.1.4-4` MUST implement a configuration-driven removal | Done | `ExtractMEDRemoveOps`, `internal/component/bgp/reactor/filter_delta.go`; `appendMEDRemove`, `internal/component/bgp/plugins/filter_modify/filter_modify.go` | `{gap}` retired; the leaf is `med-remove` |
| `RFC4271-5.1.4-2` removal MUST precede Decision Process phases 1 and 2 | Done | the ingress call site, `runIngressPolicyChain`, `internal/component/bgp/reactor/filter_ordered.go` | `{not-applicable}` retired; the rewritten payload replaces the WireUpdate before dispatch |
| `RFC4271-9.1.2.2-2` restricted MED comparison when MED is removed before IBGP readvertisement | Changed | `rfc/short/rfc4271.md`, the disposition text | Still `{not-applicable}`, on a NEW ground: the condition never holds because every removal ze performs happens BEFORE the comparison. **This classification is an OPEN QUESTION for the owner** (`ai/rules/rfc-compliance.md`): a classification that lowers what Ze owes is his to make. It was not changed by this closure and it is not settled by it |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestForwardSuppressesReceivedMEDToAnotherAS`, `test/plugin/med-not-propagated-across-as.ci`, `test/interop/scenarios/60-med-across-as-gobgp/` | |
| AC-2 | Done | `TestForwardWritesLocallySetMED`, `test/plugin/med-locally-set-reaches-peer.ci` | |
| AC-3 | Done | `TestForwardKeepsFilterSetMED` | accumulator Set and policy-chain override |
| AC-4 | Done | `TestForwardSuppressesReceivedMEDToAnotherAS`, subtest `internal-destination-keeps-it` | |
| AC-5 | Done | `TestMEDRemovalMechanismIsConfigurable`, `TestParseModifyDefsMEDRemove`, `TestMEDRemoveNeedsAMetricToRemove`, `TestMEDRemoveObeysTheChainOrder`, `test/plugin/med-removal-configured.ci`, `test/parse/modify-config.ci`, `test/interop/scenarios/61-med-remove-configured-gobgp/` | |
| AC-6 | Done | `test/plugin/med-removal-before-decision.ci`, `test/plugin/med-removal-export-refused.ci`, `TestHandleFilterUpdateMEDRemoveIsImportOnly` | |
| AC-7 | Changed | the re-derived `RFC4271-9.1.2.2-2` disposition | see the Requirements row above: the classification is open with the owner |
| AC-8 | Done | one rule, both rails; the peer-up replay reaches `forwardUpdateCore` through `RelayStoredRoute` (`TestRelayStoredRouteForwardsThroughForwardRail`) | |
| AC-9 | Done | `TestForwardMEDStaysOffRebuildPathWhenUnchanged` | |
| AC-10 | Done | `forward_local_pref_test.go` and `test/plugin/local-pref-strip-ebgp.ci` still green | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestForwardSuppressesReceivedMEDToAnotherAS` | Done | `internal/component/bgp/reactor/forward_med_test.go` | boundary cases 0 and 2^32-1 |
| `TestForwardWritesLocallySetMED` | Done | same | |
| `TestForwardKeepsFilterSetMED` | Done | same | |
| `TestForwardKeepsMEDWithinAS` | Changed | same | folded into `TestForwardSuppressesReceivedMEDToAnotherAS`'s `internal-destination-keeps-it` subtest, so one UPDATE carries both polarities |
| `TestMEDRemovalMechanismIsConfigurable` | Done | same | |
| `TestForwardMEDStaysOffRebuildPathWhenUnchanged` | Done | same | |
| `TestLocalPrefSuppressionUnchanged` | Done | `forward_local_pref_test.go` | pre-existing, still green |
| the three `.ci` | Done | `test/plugin/med-not-propagated-across-as.ci`, `med-locally-set-reaches-peer.ci`, `med-removal-configured.ci` | plus `med-removal-before-decision.ci`, which the plan did not name |
| interop | Done | `test/interop/scenarios/60-med-across-as-gobgp/` | GoBGP as the witness, FRR as the second receiver |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| a sibling beside `forward_local_pref.go` | Done | `internal/component/bgp/reactor/forward_med.go` |
| `internal/component/bgp/reactor/peer_rib_routes.go` | Changed | not modified; see Deviations |
| `internal/component/bgp/reactor/reactor_wire.go` | Changed | not modified; see Deviations |
| `rfc/short/rfc4271.md` | Done | committed with `f9b42734e` |
| `docs/features/rfc-status.md` | Done | committed with `f9b42734e` |

### Audit Summary
- **Total items:** 28
- **Done:** 23
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 5 (two encoders not needed, one unit test folded, one AC and one
  requirement whose disposition is open with the owner)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A MULTI_EXIT_DISC received from one neighboring AS does not reach another | interop | `test/interop/scenarios/60-med-across-as-gobgp/` PASS: GoBGP shows no Med on the three relayed prefixes. Discrimination measured: with `applyFactsMED`'s `mods.Op` removed, GoBGP printed `{Med: 100}` on all three and FRR printed Metric 100 |
| A metric ze itself sets still reaches the peer | interop + functional | the same scenario shows `{Med: 42}` on the prefix ze originates; `test/plugin/med-locally-set-reaches-peer.ci` |
| The RS-client exemption survives the strip | functional | `test/plugin/med-not-propagated-across-as.ci`: one UPDATE, an rs-client receiver keeping the metric byte-identical and a plain eBGP receiver losing it |
| An operator can remove MULTI_EXIT_DISC from a route by configuration | functional + interop | `test/plugin/med-removal-configured.ci`, byte-exact: the receiver is an RS CLIENT, which the automatic strip deliberately spares, so only the configured removal can explain the absence. `test/interop/scenarios/61-med-remove-configured-gobgp/` PASS: GoBGP is an INTERNAL peer, where the automatic rule never fires, and of two routes on one session the one the policy names arrives with no Med while the one it does not keeps Med 100 |
| The mechanism is refused where the RFC forbids it | functional | `test/plugin/med-removal-export-refused.ci`: the same definition on an EXPORT chain leaves the metric on the wire and logs why. Mutation-measured: with the direction guard removed from `appendMEDRemove`, `TestHandleFilterUpdateMEDRemoveIsImportOnly/export_refused` and `/export_keeps_the_rest` both go red, which is the same producer the `.ci` rejects `delta=med-remove` for |
| The removal happens before the decision process weighs the route | functional | `test/plugin/med-removal-before-decision.ci`: the stored Adj-RIB-In `attr-hex` holds no MULTI_EXIT_DISC attribute |
| The fast path is preserved | unit | `TestForwardMEDStaysOffRebuildPathWhenUnchanged`: `buildModifiedPayload` returns nil, so the route stays zero-copy |

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| MED comparison rules of the decision process beyond `RFC4271-9.1.2.2-2` (Known Limitations) | cancelled | The spec's own Known Limitations put this outside what it governs, and no obligation is left unproven by it: `RFC4271-9.1.2.2-1` and `RFC4271-9.1.2.2-3` both carry real dispositions in `rfc/short/rfc4271.md`. The row's destination named this spec, which closes here, so it is cancelled rather than re-homed |
| `RFC4271-5.1.4-4` plus `5.1.4-2` and `9.1.2.2-2`, `med-removal-configured.ci` and the interop scenario (phases 4 to 7) | done | 5.1.4-4 and 5.1.4-2 carry tagged proof; `test/plugin/med-removal-configured.ci` and `test/interop/scenarios/60-med-across-as-gobgp/` exist and pass. `9.1.2.2-2` keeps a `{not-applicable}` on a re-derived ground, which is the open question carried to the owner |

The shard holds no live row after this closure. It is COMMITTED rather than removed: `deferral_shard_removal_problems` reads the shard as HEAD holds it, so a removal in the same commit that resolves the rows is refused, and a shard outliving its source spec with terminal rows is the recorded end state.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc4271-med-across-as-ca112cd4-8337-4992-b4e1-e0d7bbff5820.md`, 29 files hash-pinned |
| `review_gate.py check` | clean |
| Rounds | 3. Round 1 ran two lenses in parallel and found five ISSUEs; round 2 judged the fixes and found one more, a real product defect; round 3 judged that fix and found none |
| Reviewer lenses used | wiring + logic + removed-behaviour + RFC conformance (lens A); test discrimination + functional coverage + security/allocation + doc drift (lens B); round 2 and round 3 over the fixes only |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The configured removal recorded a suppression for a route carrying no metric, so every such route on a `med-remove` import chain took the payload-rebuild path for a byte that is not there. Its sibling `applyFactsMED` refuses the same cost | `ExtractMEDRemoveOps`, `internal/component/bgp/reactor/filter_delta.go` | `medRemoveHasWork` in the same file, asked by both call sites. Mutation-measured: removing either arm turns `TestMEDRemoveNeedsAMetricToRemove` red |
| 2 | ISSUE | A `med-remove` from any filter beat a `med N` from a LATER filter in the same chain: the directive was converted after `textDeltaToModOps` and `filterapi.LastSetOrSuppress` is last-wins, so the operator's second filter was silently discarded | `runIngressPolicyChain`, `internal/component/bgp/reactor/filter_ordered.go` | `filterAttrs.clear` and the trailing rule in `filterAttrs.merge` (`filter_chain.go`): a delta that SETS the metric cancels an earlier removal, so the CHAIN's order decides. `TestMEDRemoveObeysTheChainOrder` covers both orders |
| 3 | ISSUE | `ze bgp policy dry-run` told the operator the metric survives an import policy that removes it, and would have promised a removal on an export chain | `computeWireChanges`, `internal/component/bgp/reactor/policy_dryrun.go` | the function takes the validated `direction` and converts the directive on import alone. `TestComputeWireChangesAS4Path/med_remove_is_reported_on_import_and_not_on_export` |
| 4 | ISSUE | No interop scenario covered the configured mechanism. `ai/rules/interop-and-goal-validation.md` puts policy in the required set, and scenario 60 proves the automatic rule only | `test/interop/scenarios/` | `test/interop/scenarios/61-med-remove-configured-gobgp/`: GoBGP as an INTERNAL peer, where the automatic rule never fires, and two routes on one session told apart by the policy's match container |
| 5 | ISSUE | The export refusal was user-visible behaviour proven only by a unit row | `appendMEDRemove`, `internal/component/bgp/plugins/filter_modify/filter_modify.go` | `test/plugin/med-removal-export-refused.ci`. Mutation-measured on the same producer: dropping the direction guard turns `TestHandleFilterUpdateMEDRemoveIsImportOnly/export_refused` and `/export_keeps_the_rest` red, which is the `delta=med-remove` line the `.ci` rejects |
| 6 | ISSUE | The first fix for finding 1 read the metric from the filter TEXT, and the text never names MULTI_EXIT_DISC, so it refused every removal. Its replacement then read the wire ALONE, which misses a metric a filter earlier in the same chain set | `medRemoveHasWork`, `internal/component/bgp/reactor/filter_delta.go` | the gate reads both. The text half was caught by scenario 61 going red while every unit test stayed green (Mistake Log); the wire-only half by round 2, and it has a subtest that drives the ordered chain on a metric-less payload |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/reactor/forward_med.go` | Yes | `ls -la` 7.7K |
| `internal/component/bgp/reactor/forward_med_test.go` | Yes | `ls -la` 20K |
| `test/plugin/med-not-propagated-across-as.ci` | Yes | `ls -la` 6.1K |
| `test/plugin/med-locally-set-reaches-peer.ci` | Yes | `ls -la` 2.9K |
| `test/plugin/med-removal-configured.ci` | Yes | `ls -la` 5.6K |
| `test/plugin/med-removal-before-decision.ci` | Yes | `ls -la` 6.4K |
| `test/plugin/med-removal-export-refused.ci` | Yes | `make ze-plugin-test` ran it as index 326, PASS |
| `test/interop/scenarios/60-med-across-as-gobgp/` | Yes | `ls -la` 7 files: ze.conf, inject.msg, inject-args, gobgp.toml, frr.conf, announce.py, check.py |
| `test/interop/scenarios/61-med-remove-configured-gobgp/` | Yes | 5 files: ze.conf, gobgp.toml, inject.msg, inject-args, check.py. `make ze-interop-test INTEROP_SCENARIO=61-med-remove-configured-gobgp` PASS |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | a received MED is suppressed toward another AS | `grep -rn "RFC4271-5.1.4-1" internal test` names `forward_med_test.go` five times and `test/plugin/med-not-propagated-across-as.ci` twice, both polarities |
| AC-5 | the removal is configurable | the same grep for `RFC4271-5.1.4-4` names `filter_modify/modify_test.go` (`TestParseModifyDefsMEDRemove`), `forward_med_test.go` (`TestMEDRemovalMechanismIsConfigurable`) and `test/plugin/med-removal-configured.ci` |
| AC-6 | the removal precedes phases 1 and 2 | the same grep for `RFC4271-5.1.4-2` names `forward_med_test.go`, `filter_modify/modify_test.go` (`TestHandleFilterUpdateMEDRemoveIsImportOnly`) and `test/plugin/med-removal-before-decision.ci` |
| AC-9 | the fast path is preserved | `TestForwardMEDStaysOffRebuildPathWhenUnchanged` asserts `buildModifiedPayload` returns nil |
| AC-10 | LOCAL_PREF unchanged | `git diff --stat -- internal/component/bgp/reactor/forward_local_pref.go` is empty |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Route from AS X readvertised to AS Y | `test/plugin/med-not-propagated-across-as.ci` | Yes. One source frame, two external receivers told apart by `rs-client`, byte-exact expectations on both |
| Ze's own MED toward a peer | `test/plugin/med-locally-set-reaches-peer.ci` | Yes. The announce rail, byte-exact |
| Operator configures MED removal | `test/plugin/med-removal-configured.ci` | Yes. `modify DROP-MED { set { med-remove true } }` on the source peer's IMPORT chain; the receiver is an rs-client, so the automatic strip cannot explain the absence |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | both rails relay another speaker's UPDATE, so a MED in the source payload is received by construction; a locally-set one arrives as an accumulator Set or as the export chain's wire override |
| A-2 | broken (not needed) | `forwardSourceInfo` carries `isIBGP` and no source AS. The destination test is "external and not an RS client", which is conformant, so nothing was threaded upstream. Mistake Log row above |
| A-3 | confirmed | `mods.Op(4, filterapi.AttrModSuppress, nil)`, applied by `genericAttrSetHandler` |
| A-4 | confirmed | `forward_dedup_test.go` uses MED as its base-identity needle, and its destinations are iBGP (`LocalAS == PeerAS == 65000`), so no fixture pinned the leak |
| A-5 | confirmed | `medPropagationAllowedTo` gates on `facts.rsClient`; `RFC7947-x-3` keeps both polarities |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config syntax (`docs/guide/configuration.md`, the `med-remove` row) | the leaf exists in `yang/ze-filter-modify.yang` as `type boolean; default "false"`, and `setBlockAllowedKeys` accepts it (`filter_modify/config.go`) | Yes |
| Plugin guide (`docs/guide/plugins.md`, the `med-remove` paragraph) | the import-only claim is produced by `appendMEDRemove` (`filter_modify/filter_modify.go`) and by the single ingress call site (`filter_ordered.go`); the mutual exclusion is produced by `validateNoConflict` (`filter_modify/config.go`) | Yes |
| Internal architecture (`docs/architecture/bgp/egress-attribute-rules.md`) | the "not on this rail" claim is produced by the absence of an export-side `ExtractMEDRemoveOps` call; `grep -rn "ExtractMEDRemoveOps" internal` returns the definition and one call site | Yes |
| RFC compliance (`docs/features/rfc-status.md`) | already committed with `f9b42734e`; the row's MED prose cites `applyFactsMED`, `ExtractMEDRemoveOps` and `appendMEDRemove`, and this commit is what puts the last two in the tree | Yes |
| CLI / API / plugin SDK / wire format / comparison table | no handler, RPC, SDK type or wire encoding changed; the directive travels in the existing filter text | N-A |

## Core Insight

A conformance boundary can live in an OMISSION. What keeps ze on the right side
of RFC 4271 Section 9.1.2.2 is not a guard that rejects: it is that
`ExtractMEDRemoveOps` has one call site and the export converter does not know
the directive. `appendMEDRemove`'s refusal is the operator-facing half and can be
deleted without changing a byte on the wire. A future change that unifies the
ingress and egress converters would remove the obligation without touching
anything that looks like a check, and the tests that would catch it are
`TestHandleFilterUpdateMEDRemoveIsImportOnly` and the zero-op assertion inside
`TestMEDRemovalMechanismIsConfigurable`.
