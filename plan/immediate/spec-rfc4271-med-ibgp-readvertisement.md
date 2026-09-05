# Spec: RFC 4271 MED Removal Before IBGP Readvertisement

**Implementation warning:** The implementer MUST read this spec critically and
verify every behavioral and RFC claim against the current source and RFC 4271
before implementation.

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | protocol |
| Depends | normal BGP RIB-selected route-to-peer egress producer |
| Phase | 5/5 blocked |
| Handoff | verify |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Record and resolve one RFC 4271 Section 9.1.2.2 finding for the common IBGP egress boundary. Ze's public MED removal syntax runs before route selection. A lower-level raw egress replacement can remove MED on the common IBGP egress rail. If a selected route used MED before that replacement, this reopens the route-loop sequence that Section 9.1.2.2 describes.

This work is separate from the website deployment failure. The linked deployment failed on stale RFC 7606 audit and generated-ledger data. MED behavior did not cause that deployment failure.

Closure is blocked: no current runnable normal BGP RIB-selected route-to-peer producer was found. The implemented guard covers the common egress boundary used by route-server and route-reflector forwarding. The unit tests drive `forwardUpdateCore` with a selected source payload. The functional and interop tests drive the same boundary through the route-server rail, so they are proof for that rail, not proof for normal BGP selected-route readvertisement.

## Required Reading

### Architecture Docs

- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - MED provenance, propagation, configured removal, and egress precedence.
  → Decision: Preserve the existing public `del { med; }` behavior and syntax. That work belongs to another session.
  → Constraint: Configured MED removal stays on ingress, before the UPDATE reaches the RIB decision process.
  → Constraint: Any proposed fix must use the existing forward-rail attribute modification path and must not add a second egress pipeline.
- [ ] `docs/architecture/core-design.md` - forward-rail and plugin boundaries.
  → Constraint: The reactor remains the common enforcement point after egress policy. A plugin-specific exception is not acceptable.

### RFC Summaries

- [ ] `rfc/short/rfc4271.md` - requirements RFC4271-5.1.4-2, RFC4271-9.1.2.2-2, and RFC4271-9.1.2.2-3.
  → Constraint: Configured removal must complete before preference and route selection.
  → Constraint: If MED is removed before IBGP readvertisement after it participated in the optional comparison, that comparison must be confined to EBGP-learned routes.
  → Constraint: IBGP-learned MED must still be used in comparisons that reach the MED step. A blanket best-path restriction is not justified by this finding.
- [ ] `rfc/full/rfc4271.txt`, Sections 5.1.4 and 9.1.2.2 - authoritative requirement text and route-loop warning.
  → Decision: Treat the route-loop warning as an ordering invariant: a route must not be selected with MED and then lose MED before IBGP readvertisement unless the decision process implements the RFC's restricted comparison.

**Key insights:**

- The public syntax and import removal path are not the defect.
- The current not-applicable disposition relies on the claim that every MED removal occurs before selection or not at all.
- Raw egress replacement makes that claim unsafe at the common IBGP egress boundary.
- The smallest candidate fix is an IBGP post-policy boundary guard, not a syntax change and not a best-path rewrite.

## Current Behavior

**Source files read:**

- [ ] `internal/component/bgp/reactor/reactor_notify.go` - applies ordered ingress policy, replaces the received `WireUpdate`, then caches and dispatches the post-ingress payload to the RIB.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - converts the public MED removal directive only on ingress; its egress path accepts raw policy replacement.
- [ ] `internal/component/bgp/reactor/filter_delta.go` - converts configured MED removal to an attribute suppression operation.
- [ ] `internal/component/bgp/plugins/filter_modify/filter_modify.go` - emits configured MED removal for import and refuses that directive for export.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - extracts MED from the stored, post-ingress route for best-path comparison.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - compares MED before the EBGP-over-IBGP decision step when routes share the neighboring AS condition.
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - runs egress policy, selects a raw override as the destination base, calls `applyFactsMED`, rebuilds, and sends.
- [ ] `internal/component/bgp/reactor/forward_rs.go` - runs in-process egress filters, calls the same MED enforcement point, rebuilds, and sends.
- [ ] `internal/component/bgp/reactor/forward_med.go` - returns without a MED operation for internal destinations. It does not inspect whether a raw egress replacement removed MED.
- [ ] `internal/component/bgp/reactor/forward_med_test.go` - proves configured ingress removal and normal internal propagation, but does not exercise post-selection raw removal.
- [ ] `internal/component/bgp/reactor/forward_dedup_test.go` - uses a changed MED as a raw-override identity marker. A future IBGP MED-preservation guard would invalidate that fixture even though the dedup behavior remains correct.
- [ ] `test/plugin/med-removal-export-refused.ci` - proves the public export directive is refused. It does not cover lower-level raw replacement.
- [ ] `test/interop/scenarios/bgp-med-across-as-gobgp/` - proves normal MED propagation behavior against GoBGP. It does not discriminate the post-selection removal finding.
- [ ] `test/interop/scenarios/bgp-med-remove-configured-gobgp/` - proves configured ingress removal against GoBGP. It does not discriminate the post-selection removal finding.

**Behavior to preserve:**

- Preserve the public `modify NAME { del { med; } }` syntax and all parser, formatter, help, and documentation work for that syntax.
- Preserve configured MED removal on ingress before route selection.
- Preserve normal MED comparison, including RFC4271-9.1.2.2-3 behavior for IBGP-learned routes.
- Preserve the external-peer suppression required by RFC 4271 Section 5.1.4.
- Preserve the RFC 7947 route-server-client exception.
- Preserve a valid destination-specific MED Set made by egress policy. This finding concerns removal, not value replacement.
- Preserve the zero-copy path when the selected and final egress payloads both contain MED or both omit it.
- Preserve the forward dedup test's base-identity contract, but use a non-MED marker if the proposed guard makes MED unsuitable as fixture data.

**Behavior to change:**

- Prevent a raw egress replacement from removing MED at the common IBGP egress boundary when the source payload carries MED.
- Keep in-process accumulator MED suppression out of this implementation unless a current producer is proved reachable.
- Replace the unapproved not-applicable disposition only after discriminating positive and negative proof exists.

## Data Flow

### Entry Point

- A peer sends an UPDATE that can contain MULTI_EXIT_DISC.
- The reactor stores the UPDATE as wire bytes plus lazy attribute indexes.

### Transformation Path

1. `notifyMessageReceiver` runs ordered ingress policy.
2. Configured `del { med; }` is converted on ingress and rebuilds the payload before RIB dispatch.
3. The RIB stores the post-ingress payload and reads its MED during best-path selection.
4. A selected route enters `forwardUpdateCore` or `reactorForwardRS` for a destination peer.
5. Egress policy can return a whole-payload raw replacement. Current registered in-process filters do not append MED suppression.
6. `applyFactsMED` runs after egress policy and before final payload rebuild.
7. The rebuilt UPDATE is written to the destination peer.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ingress to RIB | Post-ingress `WireUpdate` dispatch | Yes, producer read |
| RIB selection to reactor forwarding | Selected stored route enters a forward rail | Yes, producer read |
| External policy to reactor | Raw payload replacement | Yes, producer read |
| In-process filter to reactor | `ModAccumulator` attribute operations; no current MED suppression producer | Yes, producer read |
| Reactor to peer wire | Final rebuild after `applyFactsMED` | Yes, producer read |

### Integration Points

- `runEgressPolicyChainASN4` supplies a raw destination base.
- Current registered in-process filters supply accumulated operations, but none supplies MED suppression.
- `applyFactsMED` is the shared post-policy MED enforcement point on received-route forward rails.
- `buildModifiedPayload` applies the final attribute operation for the destination wire.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Both received-route forward rails call `applyFactsMED` after egress policy |
| No unintended coupling | Yes | Candidate fix stays in reactor forwarding and does not add RIB knowledge to plugins |
| No duplicated functionality | Yes | Candidate fix extends the existing MED enforcement point |
| Zero-copy preserved where applicable | Conditional | Design must add no operation when final MED presence is already correct |
| Registration over hardcoding | N-A | No command, family, capability, plugin, or handler is added |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | A raw egress replacement that omits MED is reachable on the common IBGP egress boundary | `runEgressPolicyChainASN4` and `forwardUpdateCore` producers | The raw-path guard is unnecessary | `TestRFC4271IBGPReadvertisementRejectsRawMEDRemoval` through `forwardUpdateCore`; functional and interop tests through the route-server rail | confirmed for the forward rails |
| A-2 | A current in-process egress filter can append final MED suppression | Registered egress filters do not record `AttrMED` suppression today | Accumulator suppression has no current producer to enforce | Add coverage only after a focused wiring test names the producer | broken for current source |
| A-3 | A destination-specific MED Set is permitted and must survive | Existing Section 5.1.4 provenance design and `modsSetMED` behavior | Restoring the selected value could override valid policy | `TestIBGPEgressMEDSetWins` | confirmed |
| A-4 | A current runnable normal BGP producer forwards a RIB-selected route through raw egress policy to a peer | `bgp-rib` records selected Loc-RIB state but does not itself send that selected route to peers; route-server and route-reflector plugins forward cached UPDATEs | RFC disposition cannot become proven for normal BGP selected-route readvertisement | MEDRIBProofScout and NormalBGPForwardScout source traces; scenario 62 remains route-server proof only | broken for current source |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A blanket preservation rule overrides a valid egress MED Set | A test that sets a new MED receives the selected value instead | Guard only absence after the last MED operation; do not compare values |
| R-2 | A guard synthesizes MED after configured ingress removal | A source without MED gains attribute 4 on IBGP egress | Require source presence before recording any Set |
| R-3 | The change alters EBGP suppression or route-server behavior | Existing Section 5.1.4 tests change result | Confine the new branch to internal destinations before existing EBGP logic |
| R-4 | The change forces every internal UPDATE onto the rebuild path | Materialization counters rise when no removal occurs | Record no operation when final MED presence is already correct |
| R-5 | A unit-only proof misses a bypass on a forwarding rail | No raw-path functional or interop observation | Drive `forwardUpdateCore`, then add discriminating wire-visible proof |
| R-6 | The RFC disposition is changed before proof discriminates | RFC gate is green after deleting the enforcement | Mutation-check the tagged test before removing the annotation |
| R-7 | Work overlaps the separate MED syntax session | Diff includes parser, formatter, modify policy syntax, help, or syntax docs | Treat every syntax-related file as excluded from this spec |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | IBGP peers can receive a different MED-presence state from the one used during selection, or valid egress policy can be overridden. Either can change routing decisions. |
| How is it reverted? | Revert the isolated implementation commit. No config migration is permitted. |
| Who else touches this path? | The separate session that changes public MED removal syntax. Its files and behavior are excluded. Forward dedup tests also use raw overrides and require fixture-only compatibility. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Raw egress policy response for an IBGP destination | → | `forwardUpdateCore` then `applyFactsMED`; `reactorForwardRS` then `applyFactsMED` | `TestRFC4271IBGPReadvertisementRejectsRawMEDRemoval`, `test/plugin/med-ibgp-post-selection-removal.ci`, scenario 62 |
| Source payload with no MED | → | `applyFactsMED` no-op path | `TestRFC4271IBGPReadvertisementDoesNotSynthesizeMED` |
| Current in-process egress filters | → | No MED suppression producer | No implementation test; add one only if a producer is proved reachable |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A selected source payload contains MED and a raw egress replacement omits MED for an IBGP destination | The final wire UPDATE contains the selected MED |
| AC-2 | The post-ingress source payload has no MED | IBGP egress does not add MED |
| AC-3 | Egress policy records a valid MED Set after the source is selected | The final wire UPDATE contains the policy value; the guard does not replace it |
| AC-4 | The destination is an ordinary EBGP peer | Existing received-MED suppression is unchanged |
| AC-5 | The destination is an RFC 7947 route-server client | Existing MED transparency is unchanged |
| AC-6 | No MED removal occurs at the common egress boundary | No MED operation is added and the zero-copy path remains available |
| AC-7 | RFC4271-9.1.2.2-2 remains disclosed as a gap until normal BGP selected-route readvertisement has a runnable producer and discriminating proof | The route-server proof does not change the RFC disposition. A future positive tagged test for the normal producer fails when the guard is removed. The negative tagged tests fail under an overbroad guard, such as MED synthesis or Set override. |
| AC-8 | The existing raw-base dedup fixture runs with the guard | It still proves unequal bases do not share materialization without using MED as the base marker |
| AC-9 | The change is reviewed | The diff contains no MED syntax, parser, formatter, help, or public policy documentation change |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives an EBGP route with MED and readvertises it over IBGP after raw egress policy | Unit path: selected source payload to egress policy to peer wire. Functional and interop path: route-server rail to egress policy to peer wire. Normal BGP selected-route peer egress is blocked on a missing producer. | `TestRFC4271IBGPReadvertisementRejectsRawMEDRemoval`, `test/plugin/med-ibgp-post-selection-removal.ci`, and scenario 62 prove the common boundary and route-server rail only |
| 2 | Removes MED with the existing import syntax | Import policy to pre-selection payload to route-server relay to IBGP wire | Existing `med-removal-before-decision.ci` and scenario 61 cover Section 5.1.4 only |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRFC4271IBGPReadvertisementRejectsRawMEDRemoval` | `internal/component/bgp/reactor/forward_med_test.go` | AC-1 and raw-policy wiring | written and passing |
| `TestRFC4271IBGPReadvertisementSkipsWithdrawOnlyBase` | `internal/component/bgp/reactor/forward_med_test.go` | no rebuild for withdrawal-only bases | written and passing |
| `TestRFC4271IBGPReadvertisementDoesNotSynthesizeMED` | `internal/component/bgp/reactor/forward_med_test.go` | AC-2 | written and passing |
| `TestIBGPEgressMEDSetWins` | `internal/component/bgp/reactor/forward_med_test.go` | AC-3 | written and passing |
| `TestDifferentBaseSameEditSetNeverShares` | `internal/component/bgp/reactor/forward_dedup_test.go` | AC-8 after fixture-only marker change | updated and passing |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MED | 0 to 4294967295 | 4294967295 | N-A | N-A |
| MED presence | present or absent | present with value 0 | N-A | N-A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `med-ibgp-post-selection-removal` | `test/plugin/med-ibgp-post-selection-removal.ci` | A reachable raw egress policy attempts removal on the route-server egress rail; the internal peer still receives MED | written and pending rerun |
| `med-removal-before-decision` | `test/plugin/med-removal-before-decision.ci` | Existing public import removal remains pre-selection | existing |

### Interop Tests

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-med-ibgp-post-selection-removal-gobgp` | `test/interop/scenarios/bgp-med-ibgp-post-selection-removal-gobgp/` | GoBGP | A foreign IBGP peer receives MED after a reachable raw egress removal attempt on the route-server rail; reverting the guard removes MED and fails the check. This is not normal BGP selected-route readvertisement proof. | written and pending rerun |
| `bgp-med-remove-configured-gobgp` | `test/interop/scenarios/bgp-med-remove-configured-gobgp/` | GoBGP | Existing ingress removal remains absent and is not synthesized | existing |

## Files to Modify

- `internal/component/bgp/reactor/forward_med.go` - candidate common egress boundary, only after design approval.
- `internal/component/bgp/reactor/forward_med_test.go` - tagged positive, negative, and policy-preservation proof.
- `internal/component/bgp/reactor/forward_dedup_test.go` - fixture marker only, if the guard invalidates the current MED marker.
- `docs/architecture/bgp/egress-attribute-rules.md` - replace the not-applicable argument with the implemented boundary and proof.
- `docs/features/bgp-protocol.md`, `docs/features.md`, `docs/guide/configuration.md` - check source-anchor claims that name `forward_med.go`; update only stale claims.
- `docs/features/rfc-status.md` - update the RFC 4271 row after proof passes.
- `rfc/short/rfc4271.md`, `rfc/requirements/rfc4271.md`, `ai/RFC-REQUIREMENTS.md` - remove the disposition and regenerate ledgers only after proof passes.

## Files to Create

- `test/plugin/med-ibgp-post-selection-removal.ci` - functional raw producer-to-wire proof through the existing route-server rail.
- `test/interop/scenarios/bgp-med-ibgp-post-selection-removal-gobgp/` - discriminating foreign-peer proof through the same route-server rail.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | No config or syntax change is permitted |
| YANG validation constraints | N-A | No YANG leaf changes |
| YANG custom validators | N-A | No validation changes |
| CLI commands/flags | N-A | No CLI change |
| CLI grammar | N-A | Existing MED syntax is excluded |
| Editor autocomplete | N-A | Existing MED syntax is excluded |
| Functional test for new RPC/API | N-A | No new RPC or API; the producer-to-wire proof is in the Functional Tests table |
| Pipe completeness | N-A | No command output |
| Env var registration | N-A | No environment option |
| Doctor check for runtime dependencies | N-A | No runtime dependency |
| Prometheus counters/metrics | N-A | No new observable counter |
| BGP family surface | N-A | No SAFI, capability, or attribute type is added |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Correctness fix only |
| 2 | Config syntax changed? | No | Syntax work is explicitly excluded |
| 3 | CLI command added/changed? | No | No CLI change |
| 4 | API/RPC added/changed? | No | No API change |
| 5 | Plugin added/changed? | No | No plugin contract change |
| 6 | Has a user guide page? | No | No user workflow change |
| 7 | Wire format changed? | Yes | `docs/architecture/bgp/egress-attribute-rules.md`; wire presence is corrected |
| 8 | Plugin SDK/protocol changed? | No | Existing raw filter and accumulator API contracts remain |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md`, `rfc/requirements/rfc4271.md`, `ai/RFC-REQUIREMENTS.md`, `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | Conditional | Update `docs/functional-tests.md` only if a new test producer is added |
| 11 | Affects daemon comparison? | No | Best-path comparison is preserved |
| 12 | Internal architecture changed? | Yes | `docs/architecture/bgp/egress-attribute-rules.md` |
| 13 | Route metadata keys added/changed? | No | No metadata change |
| 14 | Prometheus counters added/changed? | No | No metrics change |
| 15 | Registered inventory changed? | Conditional | Only if a new test plugin or interop scenario enters an inventory |
| 16 | Changed source referenced by existing anchors? | Yes | `docs/architecture/bgp/egress-attribute-rules.md`, `docs/features/bgp-protocol.md`, `docs/features.md`, and `docs/guide/configuration.md` anchor `forward_med.go` |
| 17 | Existing docs show config examples? | Preserve | Do not edit the other session's MED syntax examples |

## Implementation Steps

1. **Phase: Wiring**
   - Prove the raw removal path through `forwardUpdateCore`.
   - Do not add accumulator suppression enforcement unless a current producer is proved reachable.
2. **Phase: Discriminating tests**
   - Add positive and negative RFC-tagged tests.
   - Remove the proposed enforcement and confirm the positive test fails for the intended wire difference.
3. **Phase: Minimal enforcement**
   - Extend the existing post-policy MED enforcement point only.
   - Preserve a later MED Set, pre-selection absence, EBGP suppression, route-server transparency, and the no-op path.
4. **Phase: Functional and interop proof**
   - Exercise a reachable raw post-selection removal producer.
   - Observe the final MED state from a foreign IBGP peer.
5. **Phase: RFC disposition and documentation**
   - Keep RFC4271-9.1.2.2-2 disclosed as a gap until a normal BGP selected-route-to-peer producer exists and has discriminating proof.
   - Regenerate the requirement ledgers after any RFC disposition change.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | The raw producer is proved reachable on the common egress boundary, and accumulator suppression has a current producer before any enforcement is added |
| Feature completeness | Each applicable forward rail enforces the same boundary; normal BGP selected-route egress remains blocked on a missing producer |
| Correctness | Only MED absence is restored; a valid later Set is preserved |
| Naming | No public MED syntax or identifier changes |
| Data flow | Unit proof uses the selected post-ingress payload as source and the base as the post-policy destination payload. Functional and interop proof use the route-server rail only |
| RFC compliance | Tagged tests prove the common egress guard without weakening RFC4271-9.1.2.2-3. They do not prove normal BGP selected-route readvertisement |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Reachability proof for the raw egress producer | Focused wiring test through `forwardUpdateCore`; route-server rail proof only for functional and interop tests |
| RFC-tagged positive and negative tests | `./le rfc check` plus mutation run for the common egress guard; future normal producer needs its own proof |
| No MED syntax overlap | Scoped changed-file review excludes syntax, parser, formatter, help, and public syntax docs |
| Reactor package behavior | `go test -race ./internal/component/bgp/reactor` |
| Changed Go quality | `./le changed scope` |
| Functional wire behavior | Focused plugin suite target for the new `.ci` on the route-server rail |
| Foreign-peer behavior | Focused GoBGP interop scenario on the route-server rail |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Raw policy input | Malformed raw replacements continue to fail closed; the guard must not turn malformed payloads into accepted UPDATEs |
| Resource use | No allocation or rebuild when the final MED presence is already correct |
| Policy authority | The guard must not override a valid MED Set or alter unrelated attributes |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A claimed producer is unreachable | Research; remove that branch from the design |
| A normal BGP selected-route peer egress producer is missing | Block closure; keep RFC4271-9.1.2.2-2 disclosed as a gap |
| A valid MED Set is overwritten | Design; narrow enforcement to absence only |
| Pre-selection absence gains MED | Implementation; require source presence |
| EBGP or route-server tests change | Implementation; restore existing branch behavior |
| Interop test passes after enforcement removal | Test design; the scenario is vacuous |
| RFC4271-9.1.2.2-3 loses proof | Design; do not alter best-path comparison |

## Design Insights

- A public directive can be correctly confined while a lower-level raw policy mechanism bypasses that confinement.
- The selected source payload and the final destination payload are already available at one shared enforcement point.
- Presence, not numeric value, is the narrow property this finding requires.
- The linked website failure and this protocol finding must remain separate work.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Preserve MED presence at the common IBGP egress boundary | Restrict all MED comparison to EBGP routes; reject every raw policy payload that omits MED | The boundary fix addresses the observed ordering defect without changing route selection or the policy API |
| Preserve a later MED Set | Restore the exact selected value after every egress change | The RFC finding concerns removal; overriding valid policy would expand scope |
| Exclude syntax changes | Extend or alter `del { med; }` | Syntax is owned by another session and is not part of the finding |
| Require discriminating interop | Rely on existing scenarios 60 and 61 | Existing scenarios do not fail when the post-selection guard is absent |

## Known Limitations

- The raw post-selection producer is identified for the implemented common egress paths: functional test `med-ibgp-post-selection-removal` and interop scenario 62 both drive a raw export filter through the route-server rail after route selection.
- No current normal BGP RIB-selected route-to-peer egress producer was found. `bgp-rib` records selected Loc-RIB state, but route-server and route-reflector plugins are the current peer egress producers.
- This spec does not change best-path MED comparison.
- This spec does not change any MED configuration syntax.
- This spec does not address the website deployment failure.

## RFC Documentation

If implementation proceeds, add the RFC 4271 Section 9.1.2.2 requirement comment directly above the enforcing internal-egress branch. Do not add an RFC comment to syntax or import-removal code that does not enforce this boundary.

## Checklist

### Goal Gates

- [ ] AC-1 through AC-9 demonstrated for normal BGP selected-route readvertisement
- [ ] Every user story has a working normal BGP path and a passing test
- [x] Wiring Test table complete for the common egress boundary
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [x] Feature code integrated through the applicable forward rail
- [x] Integration and Documentation checklists revalidated for the route-server rail
- [x] Architectural Verification revalidated
- [x] Every assumption confirmed or broken

### TDD

- [x] Tests written for the common egress boundary and route-server rail
- [x] Tests FAIL without enforcement on the common egress boundary
- [x] Tests PASS with enforcement on the common egress boundary
- [x] MED boundary values covered
- [x] Functional test covers the route-server rail
- [x] Interop test covers the route-server rail and discriminates

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section
- [ ] Independent review gate is clean
- [ ] Commit implementation, tests, docs, and spec as one focused change
- [ ] Remove the spec in a separate closure commit
