# Spec: rfc4271-med-across-as

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/rfc4271-med-across-as.md` |
| Updated | 2026-08-14 |

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

## Review Gate

### Run 1
| Severity | Finding | Location | Fixed by |
|----------|---------|----------|----------|
| | | | |
