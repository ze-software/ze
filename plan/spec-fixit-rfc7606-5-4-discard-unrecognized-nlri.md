# Spec: fixit-rfc7606-5-4-discard-unrecognized-nlri -- meet RFC 7606 Section 5.4

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/spec-fixit-rfc7606-5-4-discard-unrecognized-nlri.md` |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7606 Section 5.4: "A BGP speaker advertising support for a typed address
family MUST discard routes with unrecognized NLRI types, unless the relevant
specification for that address family specifies otherwise."

Ze does not do this. Unrecognized typed NLRI is RETAINED and PROPAGATED
opaquely. The requirement is tracked as `RFC7606-5.4-1` and currently carries a
`{gap}` annotation in `rfc/short/rfc7606.md`.

**→ Decision (2026-08-01, Thomas): IMPLEMENT FULL COMPLIANCE.** Discard
unrecognized typed NLRI per Section 5.4, and prove it with an
`RFC requirement:` tagged test.

**This REVERSES the ruling of 2026-07-20** recorded in the `{gap}` annotation.
That ruling is void twice over: `ai/rules/rfc-compliance.md` voided every answer
pointing away from full compliance on 2026-07-27, a week after it was made; and
Thomas was re-asked on 2026-08-01 with the full rationale in front of him and
chose compliance.

The annotation is deliberately NOT edited yet. It describes what the code does
TODAY and is accurate until this spec lands. Removing it first would make the
ledger claim a conformance Ze does not have. Delete it in the same commit as the
behaviour change.

## Why this is not a small change

The retention is EMERGENT, not written. Nothing decides to keep these routes, so
there is no single branch to flip. Two byte-opaque paths never read the route
type. Both were verified against their producing symbol on 2026-08-01:

| Path | Producing symbol | Why it retains |
|------|------------------|----------------|
| RIB insert | `FamilyRIB.Insert` (`internal/component/bgp/plugins/rib/storage/familyrib.go`) | for a non-CIDR family it calls `insertOpaque` with the raw `nlriBytes` and never parses a route type |
| Same-context forward | `buildFwdBody` (`internal/component/bgp/reactor/forward_body.go`) | appends `peerWire.Payload()` verbatim when the ContextID matches, the zero-copy fast path |

`ParseEVPN` / `ParseMVPN` are NOT on either path. Their only non-test caller is
the display/JSON decoder (`internal/component/bgp/plugins/nlri/evpn`), which
emits `parsed: false` rather than dropping. Type recognition therefore does not
happen anywhere a discard decision could be made today, and this spec has to
introduce it.

## The counter-argument that must be answered, not ignored

The 2026-07-20 rationale was substantive. The design must address it rather than
pretend it did not exist:

- Ze has no EVPN forwarding plane (no MAC-VRF, no MAC learning, no DF election),
  so it is only ever a control-plane relay for these routes. Discarding removes
  function from the PEs on either side while improving nothing locally.
- RFC 7606 Section 6 warns that asymmetric dropping inside an AS causes
  "long-lived forwarding loops and black holes". Section 5.4 compliance on a
  pure relay is exactly that asymmetry.
- Precedent cuts both ways. RFC 9552 Section 5.1 uses 5.4's own "unless the
  relevant specification specifies otherwise" clause to require
  preserve-and-propagate for BGP-LS. RFC 7432 never did so for EVPN.
- ExaBGP retains and re-advertises opaquely. GoBGP session-resets on an unknown
  type, which is the reading 5.4 exists to prevent.

**The design question this spec must answer first:** which families does 5.4
actually bind for Ze, given the "unless the relevant specification specifies
otherwise" clause? A blanket discard across every typed family would break
BGP-LS, where RFC 9552 requires the opposite. The likely shape is per-family and
registry-driven, so a family whose specification overrides 5.4 declares that
rather than being special-cased (`ai/rules/plugin-self-containment.md`).

## Task list

- [ ] Re-read RFC 7606 Sections 5.4 and 6 and RFC 9552 Section 5.1 in full
- [ ] Determine per family whether 5.4 binds or the family's own spec overrides
- [ ] Decide where recognition happens, since neither current path parses a type
- [ ] Implement the discard at the owning layer, registry-driven, not a switch
- [ ] `RFC requirement: RFC7606-5.4-1 positive` tagged test, written red first
- [ ] Negative test: a family whose specification overrides 5.4 stays propagated
- [ ] Remove the `{gap}` annotation from `rfc/short/rfc7606.md` in the same commit
- [ ] `make ze-rfc-index`, then re-audit with `/ze-rfc-audit rfc7606`
- [ ] Update the RFC 7606 row in `docs/features/rfc-status.md`, which currently
      discloses the divergence
- [ ] Interop: confirm a real peer's behaviour, since this changes what Ze relays

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7606.md` - Sections 5.4 and 6
  → Constraint: 5.4's "unless the relevant specification specifies otherwise" clause is the whole design question. Section 6 warns that asymmetric dropping inside an AS causes long-lived loops.
- [ ] `rfc/short/rfc9552.md` - Section 5.1
  → Constraint: uses 5.4's own escape clause to REQUIRE preserve-and-propagate for BGP-LS. A blanket discard would violate this.
- [ ] `rfc/short/rfc7432.md` - EVPN
  → Constraint: never invoked 5.4's escape clause, which is why EVPN was the disclosed divergence rather than conformance by another route.

### Architecture Docs
- [ ] `ai/rules/rfc-compliance.md`
  → Decision: the 2026-07-20 divergence ruling is VOID. Do not cite it as authority.
- [ ] `ai/rules/plugin-self-containment.md`
  → Constraint: per-family behaviour is registry-driven; no family spelled in a central switch.

**Key insights:** (minimal context to resume after compaction)
- Retention is EMERGENT, not written. There is no branch to flip.
- Neither retaining path parses a route type, so recognition must be introduced.
- A blanket discard breaks BGP-LS. The answer is per-family.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-01 against the producing symbol)
- [ ] `internal/component/bgp/plugins/rib/storage/familyrib.go` - `FamilyRIB.Insert`: for a non-CIDR family calls `insertOpaque` with raw `nlriBytes`; never parses a route type.
- [ ] `internal/component/bgp/reactor/forward_body.go` - `buildFwdBody`: appends `peerWire.Payload()` verbatim on a ContextID match.
- [ ] `internal/component/bgp/plugins/nlri/evpn` - `ParseEVPN`: only non-test caller is the display/JSON decoder, which emits `parsed: false` rather than dropping.

**Behavior to preserve:** every family whose own specification overrides 5.4, BGP-LS foremost, keeps propagating unchanged.

**Behavior to change:** an unrecognized NLRI type in a family that 5.4 binds is discarded instead of retained and propagated.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

The central question is WHERE recognition happens, since neither retaining path
parses a route type today.

### Entry Point
- Peer-received UPDATE carrying a typed-family NLRI (EVPN, MVPN, BGP-LS, MUP,
  RTC), validated by `enforceRFC7606` then delivered to the RIB and the forward
  path.

### Transformation Path
1. Receive and validate. `enforceRFC7606` walks the attribute section. It does
   NOT parse NLRI route types.
2. RIB insert. `FamilyRIB.Insert` sends a non-CIDR family to `insertOpaque`,
   keyed on the raw NLRI bytes. **Proposed:** a recognition point here, or
   earlier, that a family can bind to.
3. Forward. `buildFwdBody` appends the payload verbatim on a ContextID match.
   A route discarded at step 2 never reaches this.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine to RIB | raw NLRI bytes, family-keyed | No |
| RIB to forward path | opaque entry, never type-parsed | No |
| Family registry to the discard decision | per-family predicate, registry-driven | No |

### Integration Points
- `internal/core/family` - the registry a per-family 5.4 binding should hang off,
  rather than a switch in a central package.
- `FamilyRIB.Insert` - the non-CIDR branch that currently retains.
- `rfc/short/rfc7606.md` and `docs/features/rfc-status.md` - both disclose the
  divergence and must be updated when it ends.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: no per-family spelling in a core/shared package (`ai/rules/plugin-self-containment.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | 5.4 does not bind every typed family; at least BGP-LS overrides it. | RFC 9552 Section 5.1 invokes 5.4's escape clause explicitly. | A blanket discard is correct and simpler. | Re-read RFC 9552 Section 5.1. | unvalidated |
| A-2 | Discarding is reachable without parsing every NLRI on the hot path. | Recognition can be a per-family registry predicate rather than a full parse. | The cost lands on the receive path and must be measured. | Design, then a receive-path benchmark. | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Over-broad discard drops BGP-LS or MVPN routes an operator relies on. | An interop scenario loses routes it used to relay. | Per-family opt-in, defaulting to today's behaviour until a family is ruled on. |
| R-2 | Section 6's asymmetric-dropping warning materialises: Ze drops inside an AS while its neighbours propagate. | Loops or black holes in an interop topology with mixed implementations. | This is the substance of the reversed 2026-07-20 ruling. Raise to Thomas if the design cannot avoid it. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Routes an operator expects relayed are silently dropped. Over-broad application breaks BGP-LS, where RFC 9552 requires propagation. Under-broad leaves the MUST unmet. |
| How is it reverted? | Single commit revert, but a peer that stopped receiving routes will have withdrawn them downstream, so the control-plane effect outlives the revert. |
| Who else touches this path? | `plan/spec-wire-edit-0-umbrella.md` child 1 rewrites the receive-time attribute walk, and this spec adds a decision to that same path. Sequence them. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends a route with an unrecognized NLRI type in a family 5.4 binds | → | the discard decision | (fill during design) |
| A peer sends an unrecognized BGP-LS NLRI type | → | the override path, route still propagated | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An unrecognized NLRI type in a family RFC 7606 Section 5.4 binds | The route is discarded, not installed and not propagated |
| AC-2 | An unrecognized NLRI type in a family whose specification overrides 5.4 (BGP-LS, RFC 9552 Section 5.1) | The route is retained and propagated, exactly as today |
| AC-3 | Every currently recognized NLRI type | Behavior is unchanged; no route that is relayed today stops being relayed |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Peers with a speaker sending an EVPN route type Ze does not recognize | receive, recognition, discard | (fill during design) |
| 2 | Runs BGP-LS with a peer sending a newer NLRI type | receive, override, propagate | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill during design) | | AC-1, tagged `RFC requirement: RFC7606-5.4-1 positive` | |
| (fill during design) | | AC-2, the override path | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-54-discard-unrecognized-nlri` | `test/plugin/rfc7606-54-discard-unrecognized-nlri.ci` | a peer sends an unrecognized typed NLRI in a family 5.4 binds; the route is NOT relayed to any other peer | |
| `rfc7606-54-bgpls-override-propagates` | `test/plugin/rfc7606-54-bgpls-override-propagates.ci` | a peer sends an unrecognized BGP-LS NLRI type; the route IS still relayed, per RFC 9552 Section 5.1 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| (fill during design) | `test/interop/scenarios/` | (fill) | a real peer's view of the changed relay behaviour | |

## Files to Modify

(fill during design)

## Files to Create

(fill during design)

## Implementation Steps

1. **Phase: design** -- answer the per-family binding question BEFORE any code
2. (fill during design)

## Design Insights

- The retention nobody wrote is harder to remove than a retention someone chose:
  there is no branch to flip, so compliance means ADDING a decision point to a
  path that is currently, deliberately, byte-opaque and zero-copy.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Implement full 5.4 compliance | keep the disclosed divergence | Thomas, 2026-08-01, re-asked with the full counter-argument in front of him. The 2026-07-20 ruling is void under the 2026-07-27 directive. |

## Known Limitations

- Until this lands, `rfc/short/rfc7606.md` still carries the `{gap}` annotation
  and `docs/features/rfc-status.md` still discloses the divergence. Both are
  accurate descriptions of current behaviour and must not be edited early.

## RFC Documentation (Scope: protocol)

| RFC | Section | Requirement | Site |
|-----|---------|-------------|------|
| 7606 | 5.4 | discard routes with unrecognized NLRI types unless the family's specification says otherwise | (fill during design) |
| 9552 | 5.1 | BGP-LS invokes 5.4's escape clause and requires preserve-and-propagate | (fill during design) |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] `make ze-verify` passes
- [ ] `make ze-rfc-check` passes with the `{gap}` removed
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-fixit-rfc7606-5-4-discard-unrecognized-nlri.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm` this spec
