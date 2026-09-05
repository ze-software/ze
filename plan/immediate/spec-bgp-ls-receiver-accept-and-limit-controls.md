# Spec: bgp-ls-receiver-accept-and-limit-controls

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | spec-bgp-ls-receiver-fault-management |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the RFC asks for.** RFC 9552 Section 8.2.3 states three receiver-side
configuration obligations, of which two bind a BGP-LS receiver. The first:
"An implementation SHOULD allow the operator to specify neighbors to which
Link-State NLRIs will be advertised and from which Link-State NLRIs will be
accepted." The second: "An implementation SHOULD allow the operator to specify
the maximum number of Link-State NLRIs stored in a router's Routing Information
Base (RIB)." `rfc/short/rfc9552.md` carries them as `RFC9552-8.2.3-1` and
`RFC9552-8.2.3-3`, both SHOULD-level, both unannotated and untested today.

**What ze does instead.** Ze has no per-neighbor BGP-LS accept list and no
Link-State RIB ceiling. An operator who wants to refuse BGP-LS from one peer
writes a `bgp/policy/family-filter` instance naming the `bgp-ls` family with
action `remove` and references it from that peer's import chain
(`parseFamilyFilters`, `internal/component/bgp/plugins/filter_family/config.go`;
`handleFilterUpdate`, `handler.go`). That is a hand-written all-or-nothing drop,
not the accept list Section 8.2.3 describes. The declared BGP-LS role that
`plan/immediate/spec-bgp-ls-receiver-fault-management.md` designs is also
all-or-nothing: it is what Section 8.2.6's MUST asks for, and it is less than
Section 8.2.3 suggests. That spec is at Status `design`, so the role does not
exist in the tree yet either.

**Why the row waited, and why the wait is over.** The deferral this spec
replaces (`plan/deferrals/bgp-ls-receiver-fault-management.md`, 2026-08-26) made
the RIB ceiling wait on `RFC9552-8.2.6-1`, whose `{gap}` said the bgp-ls prefix
maximum was compared against a count that `countPrefixEntries` derived from a
CIDR walk that never parses a type-length Link-State NLRI. That defect is
settled. `forEachPrefixEntry`
(`internal/component/bgp/reactor/session_prefix.go`) now asks
`nlrisplit.Get(familyFromKey(fk))` first and walks the section with the
per-family splitter, so a bgp-ls section is counted by its own framing, and the
`RFC9552-8.2.6-1` row of `rfc/short/rfc9552.md` carries no `{gap}` today. The
precondition the row named is met, so the two SHOULDs can be scoped.

**What the work is.** Two operator-facing controls, each its own config surface:

1. **The accept list.** Which neighbors ze accepts Link-State NLRIs from,
   declared per peer or per group, so an operator names the accepted set rather
   than writing a filter for every peer that must be refused. The design has to
   decide how this relates to the declared BGP-LS role the fault-management
   spec adds, because both govern import of one family and two mechanisms that
   answer the same question are one mechanism too many (`ai/rules/simplicity.md`).
2. **The RIB ceiling.** A maximum number of Link-State NLRIs held in the RIB.
   The per-peer, per-family prefix maximum ze already enforces
   (`applyPrefixCheck` and `applyInstalledPrefixSections`,
   `internal/component/bgp/reactor/session_prefix.go`) is counted at message
   time against one session. Section 8.2.3 asks about what the RIB stores,
   across peers, which is a different number with a different owner.

**Not in scope.** Section 8.2.3's producer-side SHOULDs (the advertisement rate
limit, the abstracted topologies, and the 4096-byte BGP-LS UPDATE size limit)
have no producer to limit, because ze originates no BGP-LS. They belong to
`plan/pre-release/spec-bgp-ls-origination-and-the-scheduled-marker.md` Phase 2,
where the shard's second row already sends them.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-bgpls.md` - how a Link-State NLRI is framed and how it reaches the RIB
  → Decision: [fill during research]
  → Constraint: [fill during research]

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc9552.md` - rows `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3`
  → Constraint: both are SHOULD-level, so `./le rfc check` does not gate them; the proof owed is a tagged test, not a gate pass

**Key insights:** (minimal context to resume after compaction)
- The counting defect that blocked this work is fixed: `forEachPrefixEntry` dispatches through `nlrisplit`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `forEachPrefixEntry` asks `nlrisplit.Get` for the family's splitter and counts a bgp-ls section by its own framing; `countPrefixEntries` is the nil-callback form. The maximum this feeds is per peer and per family, checked as the message is read, and it is not a ceiling on what the RIB holds.
- [ ] `internal/component/bgp/plugins/filter_family/config.go` - `parseFamilyFilters` reads the `family-filter` instances an operator writes; this is the only surface today by which a peer's BGP-LS can be refused.
- [ ] `rfc/short/rfc9552.md` - `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3` carry no annotation and no test; `RFC9552-8.2.6-1` carries no `{gap}`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A peer with no BGP-LS configuration keeps accepting and propagating BGP-LS exactly as today.
- The per-peer, per-family prefix maximum keeps its message-time semantics.

**Behavior to change:** (only what the user asked for)
- Add an operator-declared accept list for Link-State NLRIs.
- Add an operator-declared ceiling on Link-State NLRIs stored in the RIB.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A BGP UPDATE carrying MP_REACH_NLRI or MP_UNREACH_NLRI with AFI 16388 arrives on an established session, in wire bytes.
- Operator configuration arrives as a YANG-modeled config tree at config-apply.

### Transformation Path
1. Wire parsing and per-family splitting in `internal/core/bgp/nlri/nlrisplit/`.
2. Per-peer prefix accounting in `internal/component/bgp/reactor/session_prefix.go`.
3. Import policy in `internal/component/bgp/plugins/filter_family/`.
4. RIB storage in `internal/component/bgp/plugins/rib/`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ reactor | the accept list and the ceiling arrive as config sections at apply | No |
| Reactor ↔ RIB plugin | the ceiling counts what the RIB stores, not what one session read | No |

### Integration Points
- `applyPrefixCheck` and `applyInstalledPrefixSections` (`internal/component/bgp/reactor/session_prefix.go`) - the existing per-peer limit the ceiling must not duplicate.
- The declared BGP-LS role of `plan/immediate/spec-bgp-ls-receiver-fault-management.md` - the accept list must extend it rather than compete with it.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A RIB-wide Link-State ceiling has an owner that can count across peers | the RIB plugin holds the stored set | the ceiling has to be approximated per peer, which is not what Section 8.2.3 asks | reading the RIB plugin during research | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The accept list and the declared BGP-LS role become two answers to one question | a config that sets both and disagrees | design the accept list as the general form and the role as its declaration, or fold one into the other |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | BGP-LS from a wanted neighbor is dropped, or a topology silently stops filling |
| How is it reverted? | single commit revert; both controls default to unset |
| Who else touches this path? | `plan/immediate/spec-bgp-ls-receiver-fault-management.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| a peer absent from the declared accept list sends a Link-State NLRI | → | the import path that drops it | `TestUnlistedNeighborsLinkStateNLRIIsNotAccepted` |
| the RIB holds the declared maximum of Link-State NLRIs and one more arrives | → | the ceiling check | `TestLinkStateRIBCeilingRefusesTheNextNLRI` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a peer named in the accept list sends BGP-LS | the NLRI is accepted, as today |
| AC-2 | a peer not named in the accept list sends BGP-LS | the NLRI is not accepted, and the operator can see why |
| AC-3 | no accept list is configured | every peer is accepted from, as today |
| AC-4 | the configured RIB maximum is reached | further Link-State NLRIs are refused and the operator is told |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnlistedNeighborsLinkStateNLRIIsNotAccepted` | `internal/component/bgp/plugins/nlri/ls/accept_test.go` | AC-2 | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->
| `TestLinkStateRIBCeilingRefusesTheNextNLRI` | `internal/component/bgp/plugins/rib/ls_ceiling_test.go` | AC-4 | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-ls-accept-list` | `test/decode/bgp-ls-accept-list.ci` | the operator names the neighbors BGP-LS is accepted from | | <!-- doc-links: ignore (file this skeleton plans and has not created yet) -->

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-ls-accept-list-frr` | `test/interop/scenarios/` | FRR | a real BGP-LS speaker outside the accept list is refused | | <!-- doc-links: ignore (directory this skeleton plans and has not created yet) -->

## Files to Modify
- `internal/component/bgp/reactor/session_prefix.go` - the per-peer limit the ceiling must not duplicate
- `rfc/short/rfc9552.md` - `RFC9552-8.2.3-1` and `RFC9552-8.2.3-3` gain tagged tests

## Files to Create
- [named at design]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | [answered at design] |
| CLI grammar (keyword before value) | | [answered at design] |
| Functional test for new RPC/API | | [answered at design] |
| BGP family surface (new SAFI / capability / attribute) | | [answered at design] |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | | [answered at design] |
| 9 | RFC behavior implemented, changed, or newly proven? | | `rfc/short/rfc9552.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the config surface and write the failing wiring tests
   - Tests: the two names in the Wiring Test table
   - Files: [named at design]
   - Verify: the entry point exists and the wiring test fails because the control is a stub
2. **Phase: [named at design]**

## Known Limitations
- Section 8.2.3's producer-side SHOULDs stay out of scope while ze originates no BGP-LS.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Current Behavior and Data Flow sections completed

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Interop tests for protocol features (or N-A with a reason)
