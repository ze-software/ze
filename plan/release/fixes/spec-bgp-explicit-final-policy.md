# Spec: BGP explicit final policy disposition

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P0 |
| Release reason | Explicit final policy must not be bypassed by accepted filter rewrites. |
| Scope | config |
| Depends | - |
| Phase | 4/4 — focused repair review, verification and commit |
| Handoff | - |
| Updated | 2026-10-22 |

## Task

Implement owner decision M17. A configured BGP import or export policy has an ordered filters list and an explicit final accept or reject disposition. Filter acceptance continues; rejection stops. A route surviving the filters receives the final disposition. No attribute modification, including a full raw-payload rewrite, grants acceptance or bypasses the final decision. Reject configurations missing the final disposition before they become live.

The owner approved separate filters and final fields inside each import/export container, then explicitly requested implementation. M18 authorizes admission at a cap of 15 for this work only. Peers with no policy configured retain their behavior. Migrate existing policy configurations, generators, fixtures, and examples to explicit final accept where they formerly relied on implicit acceptance. Remove obsolete syntax rather than maintain a compatibility path.

## Scope Boundaries

| In scope | Out of scope |
|----------|--------------|
| Import/export schema, parsing, inheritance, runtime carrier and final core decision | New compound predicates or a second policy engine |
| Every affected policy consumer and producer, including generated config and migration | Changing route ranking |
| Empty configured policies versus no configured policy | Changing absent-policy defaults |
| Raw modifications, reload withdrawals, client-specific alternative selection | Unrelated protocol changes |
| Functional and real-peer proof; affected docs and independent review | Closing unrelated active specs |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Configured import/export policy with a valid final accept or reject | Configuration loads and both directions use the chosen disposition |
| AC-2 | Configured policy missing final, unknown final, or obsolete list-only syntax | Configuration fails with the offending path and corrective syntax; running config remains unchanged |
| AC-3 | Passing filters followed by final reject | Route is not imported or advertised |
| AC-4 | Rejecting filter followed by final accept | Rejection remains terminal |
| AC-5 | Attribute delta or full raw-payload modification followed by final reject | Modified route is not accepted or advertised |
| AC-6 | Modified route followed by final accept | Intended modification reaches the consumer without unrelated attribute loss |
| AC-7 | No configured policy, including peers with automatic mandatory/default filters | Existing behavior remains unchanged |
| AC-8 | Explicit empty policy with final accept or reject | Final action still decides; emptiness does not erase policy presence |
| AC-9 | Policy inherited across BGP, group, peer, or dynamic-peer configuration | Filter ordering and existing inheritance remain coherent, with one effective explicit final disposition |
| AC-10 | Live export policy changes from accept to reject | Previously advertised routes are withdrawn; withdrawals are not blocked by final reject |
| AC-11 | Preferred candidate rejected by destination export policy while an eligible alternate exists | Existing client-specific alternative selection advertises the alternate; no eligible candidate withdraws prior advertisement |
| AC-12 | Existing config producers, fixtures and documentation | New syntax is emitted consistently, with previous implicit acceptance made explicit; no obsolete execution path remains |
| AC-13 | Peer/policy display or configured-policy dry run, including empty and automatic-only policies | Output distinguishes configured final from automatic-only filtering, and dry-run final verdict agrees with forwarding |

## Implementation Phases

| Phase | Work | Dependency |
|-------|------|------------|
| 1 | Schema and validated policy carrier, config resolution, inheritance and plugin JSON readers | Shared contract below |
| 2 | Core import/export final disposition, runtime state, recovery and final-only reload reconciliation | Shared contract below; implemented alongside phase 1 in disjoint files |
| 3 | Migrate producers, fixtures and documentation; author functional and real-peer proof | Shared syntax; disjoint migrations run alongside phases 1/2 |
| 4 | Independent review, scoped checks, commit and closure | All behavior and proof |

## Risks & Assumptions

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Empty chain erases explicit final reject | Length-based fast path accepts | Carry policy presence independently of filter count |
| R-2 | Raw modification bypasses final | Early terminal return advertises a route rejected by final | Apply the same final decision to raw and delta outcomes |
| R-3 | Final-only reload appears unchanged | Existing route remains at remote peer | Include disposition in equality, swap, replay, and withdrawal accounting |
| R-4 | Old syntax survives in a generator or fixture | Config load rejects generated content | Inventory producers as well as literal examples; exercise full relevant suites |
| R-5 | Other sessions modify shared BGP paths | File changes during implementation | Preserve foreign hunks and assign explicit edit ownership |

## Required Reading

| Page | Decision or constraint |
|------|------------------------|
| "Owner Decisions" below, M17/M18 | Owner approved syntax, semantics, implementation and this admission exception |
| `docs/guide/configuration.md`, Prefix-List Filter and Route Attribute Modifier | Preserve filter names, cumulative ordering, and modifications; migrate examples |
| `docs/architecture/core-design.md`, Policy Filter Chain | Keep mandatory/default/plugin ordering and the existing RPC boundary |
| `docs/architecture/config/yang-config-design.md` | Use presence containers and schema-native enum/mandatory validation; preserve structural deactivation |
| `docs/architecture/config/syntax.md` | Change original-tree extraction rather than rely on generic resolved map merging |
| `ai/doors/system.md`, `bgp-engine.md`, `config-yang.md`, `bgp-plugins.md`, `le-tooling.md` | Respect registration, subsystem ownership, external plugins, and native checks |
| `docs/contributing/ze-go-style.md`, `ai/rules/config.md`, `testing.md`, `performance.md`, `user-facing-errors.md` | Validated state, no hot-path allocation, consumer-visible assertions and exact corrective errors |

## Pre-implementation Baseline

| Source read | Producer and observed behavior |
|-------------|--------------------------------|
| `internal/component/bgp/config/redistribution.go` | `extractFilterChain` reads direction leaf-lists, preserves inactive members; `concatFilters` accumulates refs and loses empty-policy presence |
| `internal/component/bgp/config/peers.go` | Original global/group/peer layers accumulate in order; refs are validated, canonicalized, and automatic loop filters prepended |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Global and peer-fields schema declare import/export leaf-lists; grouping supplies group and peer contexts |
| `internal/component/bgp/reactor/filter_chain.go` | `PolicyFilterChain` accepts on exhaustion; raw modification ends traversal and returns before a final decision |
| `internal/component/bgp/reactor/filter_ordered.go` | Ingress and egress skip empty/inactive chains; both accept valid raw replacements |
| `internal/component/bgp/reactor/peer_initial_sync.go` | `defaultOriginateFilterAccepts` is an internal single-filter check, not a configured policy |
| `internal/component/bgp/reactor/peer_settings_apply.go` | Hot swap copies only filter slices; no final-policy reconciliation at the apply tail |
| `internal/component/bgp/reactor/relay_recovery.go` | `queueRecoveryOutcome` already retries ranked candidates after genuine policy rejection |

Preserve automatic loop filters, absent policies, early rejection, error-vs-policy rejection, raw-terminal filter traversal, plugin RPC contracts, source encoding, and destination-specific recovery. Change only the user-policy terminal decision and the configuration/operational state needed to carry it.

## Shared Implementation Contract

| Item | Contract |
|------|----------|
| Carrier owner | Phase 1 owns a validated value type in `filterapi`, shared by configuration and runtime |
| Carrier state | Immutable filter references plus final disposition; absence remains distinct from an empty configured policy. Invalid configured-without-final state is refused at construction/validation |
| Runtime field names | Replace `ImportFilters` and `ExportFilters` with `ImportPolicy` and `ExportPolicy`; remove obsolete fields and update every caller |
| Filter order | Preserve global, group, peer concatenation and inactive-member state |
| Final precedence | The most-specific present direction policy supplies final. An absent child inherits. Every explicitly present raw direction container must supply its own final; inheritance cannot repair an omission |
| Automatic filters | Retain automatic refs without inventing a configured operator policy or requiring final where none was configured |
| Raw rewrite | Ends filter traversal as today, then receives final accept/reject. No new raw-to-text pipeline or plugin API |
| Fast paths | Final-only reject executes without RPC or text formatting and cannot enter the policy-agnostic RS fast path |
| Internal prefilter | Default origination's single-filter predicate has an explicit internal accept outcome; it is not an operator policy |
| Reload | Final-only export change must reconcile advertised state without requiring a new source UPDATE or session reset; preserve writer serialization and candidate recovery |
| Withdrawal | Final rejection must not strand previously advertised state by swallowing the withdrawal needed to remove it |
| Operational output | Peer/policy display and dry-run must report/use the effective final instead of claiming acceptance after filter exhaustion |

## Data Flow

1. Operator configuration enters existing YANG parsing, validation and transactional apply.
2. Each original direction presence container is validated before inheritance; ordered filter refs and final produce a valid policy value.
3. Global/group/peer composition retains cumulative filters and the most-specific final; automatic filters join without losing presence.
4. Static/dynamic peer settings, coherent snapshots and forwarding facts carry the policy value.
5. Existing core ordered import/export step runs filters and applies the final decision; existing writer/recovery translates export rejection into correct destination state.
6. Reload publishes the new policy and reconciles affected existing advertisements; rollback restores the old value and state.

| Boundary | Contract |
|----------|----------|
| Config to reactor | Validated policy values, including presence and final, not loose slices |
| Reactor to plugin | Existing filter-update RPC unchanged; final is core-owned |
| Reactor to writer/recovery | Genuine policy rejection is distinct from filter execution failure |

### Architectural Verification

| Check | Evidence / obligation |
|-------|-----------------------|
| No bypassed layers | Use existing ordered policy steps and final writer; review every active-filter fast path |
| No unintended coupling | No reactor import of a plugin implementation; no plugin-specific final handler |
| No duplicated functionality | Reuse selection, modifications, config schema and filter registries |
| Allocation behavior | Value carrier, immutable snapshots; no per-route allocation for final-only policy |
| Registration outbound/inbound | YANG declares configuration and completion; final is a finite core disposition, not an enumerated plugin feature |

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | Raw rewrite may retain terminal filter traversal | Existing chain contract and raw-terminal test; owner requires final enforcement, not new composition | Would require a separate encoding design | Main and runtime scout read chain producer | confirmed |
| A-2 | Existing alternative selection can consume final rejection | `queueRecoveryOutcome` already skips rejected candidates | Need repair at writer/recovery boundary | `bgp-policy-final-reload` passed 80 loaded invocations, including alternate selection and all-rejected removal; this does not certify the separate unfinished RS election cutover | confirmed |
| A-3 | Final-only reload needs reconciliation beyond field copying | The original hot-swap tail had no policy replay | Existing machinery may provide a reusable entry point | The real reload scenario withdrew and restored retained routes without a new source UPDATE or a session reset | confirmed |
| A-4 | Restoration uses retained route sources; one-shot API announcements remain producer-owned | `Peer` route-queuing contract delegates persistence to external API programs; `AnnounceNLRIBatch` sends established-session requests directly and initial-sync drains consumed operations | Adding desired-route storage would be a separate API ownership change | Main read the injection producer, queue drains, and API architecture replay contract | confirmed |

## Blast Radius

| Question | Answer |
|----------|--------|
| Failure cost | Route leakage, incorrect rejection, stale advertisements, invalid generated configurations |
| Recovery | Correct the policy/configuration and reload; no destructive Git operation |
| Concurrent work | Existing BGP replacement and plugin-reload sessions; preserve their hunks and coordinate file ownership |

## Wiring Test

| Entry point | Feature path | Test |
|-------------|--------------|------|
| Config validation | Direction presence and required final | `TestPolicyFinalConfiguration` |
| Received and forwarded UPDATE | Core chain and writer | `TestPolicyFilterChainFinalDisposition` and `test/plugin/policy-final-disposition.ci` |
| Config reload | Hot swap to final reject and withdrawal | `test/reload/bgp-policy-final-reload.ci` |

## End-to-End User Stories

| User action | Observable proof |
|-------------|------------------|
| Configure final accept/reject on import/export | Allowed route reaches peer; rejected route does not; established-session positive control |
| Omit final or supply obsolete syntax | Validation fails before live state changes, with corrective syntax |
| Modify route then reject | Neither delta-modified nor raw-rewritten announcement escapes |
| Change export final while session remains up | Remote route is withdrawn without source update; retained received/configured/plugin-owned routes restore when acceptance returns. One-shot API producers must reannounce because Ze holds no desired-route source for them |
| Apply destination-specific restrictions | Existing alternate is advertised; no eligible alternate removes the old route |

## Test Plan

| Layer | Tests and assertion |
|-------|---------------------|
| Config | Required enum, presence boundary, malformed global policy with no peers, parent omission not repaired by child, empty policy, inheritance, dynamic templates, deactivation, automatic refs |
| Core unit | Accept/reject precedence, raw/delta outcomes, empty/inactive policies, filter failures and teardown preservation, final-only policy without API, default-originate predicate |
| Lifecycle unit | Coherent snapshots, final-only swap, rollback, removal to absent, RS fast-path exclusion, dry-run parity |
| Functional | New policy-final-disposition and bgp-policy-final-reload tests; migrate existing parse/plugin/reload/CLI fixtures |
| Real peers | Existing policy import/export and modification interop scenarios plus explicit final contrasts; raw loopback BGP probe may supplement but not replace permanent functional proof |
| Numeric boundaries | N-A: no numeric setting added; final has exactly two valid values |
| Discrimination | A deliberately accepting final executor must fail the final-reject wire assertion; syntax-only failure is not evidence of runtime discrimination |

## Files to Modify

| Owner | Population |
|-------|------------|
| Config/type | `internal/component/bgp/filterapi`, `internal/component/bgp/config`, `internal/component/bgp/yang/ze-bgp-conf.yang`, necessary registered configuration validation, plugin JSON consumers in `filter_family`, `filter_irr`, `filter_path_asn` |
| Runtime | `internal/component/bgp/reactor` policy chain, ordered steps, forwarding facts, dynamic settings, hot swap, dry run, operational consumers in `internal/component/bgp/plugins/cmd` |
| Migration | All executable policy syntax under `test/`, configuration/CLI unit fixtures, authored demos, and producer-generated config; do not rewrite role export lists or redistribution policy |
| Documentation | Core design, config syntax, configuration/plugin/policy/deactivation/IRR/peering guides, affected API policy descriptions, comparison and live website examples |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/bgp/filterapi/policy.go` | Validated policy value if existing file cannot hold it clearly |
| `test/plugin/policy-final-disposition.ci` | End-to-end final semantics |
| `test/reload/bgp-policy-final-reload.ci` | Established-session final-only reload |

### Integration Checklist

| Point | Applies | File or reason |
|-------|---------|----------------|
| YANG schema | Yes | Global and peer-fields policy direction containers |
| YANG constraints | Yes | Presence, mandatory final and enumeration; no default final |
| Custom validation | Yes if needed | Existing registered BGP validation boundary; refuse invalid original layers even without peers |
| CLI commands/flags | No | Existing config editor derives the new tree |
| CLI grammar | Yes | New nested config paths in examples, fixtures and producer arrays |
| Editor autocomplete | Yes | Derived schema enum and filters leaf-list; retain inactive member editing |
| Functional test | Yes | New plugin/reload scenarios |
| Pipe completeness | No | No new command/output pipe |
| Env registration | No | No runtime environment setting |
| Doctor/dependency | Yes for existing diagnostics only | Filter reference paths move; no new runtime dependency |
| Metrics | No | Existing policy statistics; no new counter needed for a disposition |
| BGP family | No | No new SAFI, capability or attribute |

### Documentation Update Checklist

| # | Surface | Applies | Action |
|---|---------|---------|--------|
| 1 | Feature declaration | Yes | Update existing BGP policy feature description if it describes the old contract |
| 2 | Config syntax | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI reference | Yes where examples use old paths | Migrate only affected config command examples |
| 4 | API/RPC | No RPC change | Operational final output description updated where exposed |
| 5 | Plugin guide | Yes | `docs/guide/plugins.md`; distinguish filter result from final policy result |
| 6 | User guides | Yes | Policy, peering, role, deactivation, IRR and redistribution examples |
| 7 | Wire format | No | Existing UPDATE format unchanged |
| 8 | SDK/protocol | No protocol change | Existing process-protocol policy semantics clarified |
| 9 | RFC behavior | No new obligation claimed | Preserve protocol withdrawal and filter behavior; changed tagged proofs owe existing discrimination workflow |
| 10 | Test infrastructure | Yes, proof conventions | Existing harness retained; docs/functional-tests.md describes native event, wire-fence and FRR attribute oracles |
| 11 | Comparison | Yes | `docs/comparison.md`, live website policy comparison |
| 12 | Architecture | Yes | Core policy chain, inheritance and raw traversal boundary |
| 13 | Route metadata | No | No new metadata key |
| 14 | Metrics | No | No metric declaration |
| 15 | Registration/inventory | No plugin added | Existing schema discovery remains authoritative |
| 16 | Source anchors | Yes | Derive affected pages from changed producer paths and update corresponding claims |
| 17 | Examples | Yes | Migrate executable/current examples; leave historical records identified as history |

### Discovery

The existing configuration and policy guide/index entries remain the discovery surface. The YANG schema owns the two final values and editor completion. Configuration and wire assertions prevent implicit acceptance from returning.

### Critical Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Fail-closed boundary | No configured policy can lack final; no length/inactive/raw fast path bypasses reject |
| State coherence | Filters and final come from the same settings generation |
| Inheritance | Original layers validate independently; cumulative order and most-specific final are explicit |
| Withdrawals | Remote state is removed on final rejection and final-only reload |
| Recovery | Rejected candidate does not hide a permitted alternate |
| Migration | Every active producer emits nested filters and explicit final; absence remains absence |

### Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| Config-to-core implementation | Scoped config/filterapi/reactor checks plus real daemon entry point |
| Operator and generator migration | Relevant parse, plugin, reload and editor suites |
| Policy wire behavior | Real-peer positive and negative route assertions |
| Independent review | Fresh reviewer over affected producers and caller population |

### Security Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Policy bypass | Missing final, inactive filters, raw rewrite, error-accept and nil API cannot convert final reject into acceptance |
| Live state | Invalid config does not alter running policy; permitted-to-denied transition removes old advertisements |
| Resource behavior | Final-only decision requires no RPC, heap buffer or per-route worker |

## Goal Validation

Evidence files below are under session
`tmp/session/2026-10-22-644a4e78-0122-4eeb-8642-eb50638a1641/scratch/`;
stress captures are under `tmp/stress-repro/`. These are observed results, not
a claim that the final verification gate has passed.

| Goal / AC | Concrete evidence | Status |
|-----------|-------------------|--------|
| Explicit import/export final, AC-1 | `policy-final-disposition` passed 80 invocations; `job-policy-final-frr-restored-98ed6c29.log` records the FRR scenario pass | real-peer path proven |
| Refuse incomplete or obsolete policy before apply, AC-2 | `TestPolicyFinalConfiguration`, `TestPolicyOriginalTreeValidation`, `TestPolicySetSyntaxRefusals` and `TestPolicyRefusalDoesNotPublishEnvironment` passed in the scoped package run; `policyFinalReloadScenario` refuses missing final then proves a newly received route still uses the live accept policy | configuration and live-state proof |
| Passing filters cannot bypass reject, AC-3 | Matrix passing-reject case; accepting-final mutant leaks `10.71.2.0/24` in `job-policy-final-runtime-discrimination-f2c38740.log` | discriminated on wire |
| Early rejection remains terminal, AC-4 | Matrix early-reject case and both real configured-policy dry-run directions | real-peer and command proof |
| Delta/raw rewrite cannot bypass reject, AC-5 | Matrix delta-reject/raw-reject cases; FRR accepting-final mutant fails route absence in `job-policy-final-frr-discrimination-f6d0cc35.log` | discriminated on wire |
| Preserve accepted modifications and unrelated attributes, AC-6 | Matrix import-attribute and sink-wire assertions; `TestFilterSubjectReadsUnmodifiableAttributes` covers native plugin dispatch and consumer bytes on import, originated export and managed forwarded export | observed behavioral proof |
| Preserve absent/automatic-only behavior, AC-7 | `TestPolicyConstruction`, `TestPolicyInactiveContainerIsAbsent`, dynamic-template checks and existing no-export-policy acceptance checks | scoped checks passed |
| Empty/inactive final-only policies, AC-8 | Empty matrix cases; `TestPolicyFinalWithoutActiveFilters`, `TestPolicyFinalNilAPIRails` and `TestPolicyFinalOnlyDecisionDoesNotAllocate` passed in race/count-20 runs | no-RPC/no-allocation path proven |
| Original-layer validation and inheritance, AC-9 | `TestPolicyInheritance`, `TestPolicyDynamicTemplateFinal`, original-tree validation and migrated dynamic-peer fixtures | scoped and functional checks passed |
| Withdraw/restore on final-only reload, AC-10 | Reload scenario passed 80 invocations; accepting-final mutant retains forbidden routes in `job-policy-final-reload-discrimination-da796aa7.log`; restored scenario passes. Actual eviction and staged-ID collision reds (`job-policy-replay-eviction-red-373dda8c.log`, `job-policy-staged-id-collision-red-899af56a.log`) are repaired; `job-policy-replay-identity-final-race20-a0d38ba2.log` passes the final targeted race/count-20 population, including replay, recipient identity/lifetime and unknown-withdrawal allocation controls | real-peer reload and targeted replay repair proven; final native gate pending |
| Destination-specific alternate and all-rejected removal, AC-11 | `policyFinalAlternate` in the same reload scenario, with exact received state and unchanged sessions | this policy path proven; separate RS WIP remains open |
| Migrate active producers and examples, AC-12 | All 120 fixtures promoted after 24 groups of 80 invocations; `policy-stress80-passed-groups.json` and the promotion inventories identify them | migration complete; final gate pending |
| Display effective final and dry-run agreement, AC-13 | `policy-operational-smoke/logs/669-policy-final-disposition/client-stderr.log` records 16 configured import/export display and dry-run observations. `policy-automatic-display-smoke/logs/669-policy-final-disposition/client-stderr.log:257` records the real automatic-only import chain with filter AUTO and no invented final; its complete matrix run passed in `job-policy-automatic-display-smoke-253c2e96.log` | live commands proven |

### Goal Gates

- [x] All AC rows have concrete evidence.
- [x] Scoped package checks and real entry-point smoke are observed.
- [x] Independent review covers all affected producers/callers.
- [x] Existing unrelated reds are disclosed, never hidden by weakening tests.
- [ ] Applicable native verification runs through the repository launcher and throwaway verification worktree.

## Design Decisions

| Decision | Alternative | Reason |
|----------|-------------|--------|
| Core-owned final field | Final plugin | Owner approved mandatory structural decision, not optional plugin ordering |
| Validated value carrier | Parallel loose fields or compatibility slices | Preserve coherent snapshots and prevent invalid configured state |
| Preserve raw traversal boundary, then final | New raw-to-text composition | Required safety without unrelated encoding machinery |
| Most-specific final and cumulative filters | Multiple layer-level terminal decisions | Fits current filter inheritance and makes one effective policy decision |

## Owner Decisions

Preserved here as spec-specific owner approvals, including decisions copied
from `ai/META-DECISIONS.md`; this record does not claim those global rows were
removed. M30 and M31 below are this spec's approval labels, not the unrelated
current global M30/M31 release and layout decisions. The original ids remain
so the implementation evidence and citations still resolve.

| Id | Date | Decision | Thomas's words | Applied in |
|----|------|----------|----------------|------------|
| M17 | 2026-10-22 | A configured BGP policy must state an explicit final accept or reject disposition, separate from its filters: `filter { import { filters [ CUSTOMERS TAG ]; final accept; } }`. A filter's acceptance continues; rejection stops. At chain exhaustion the final disposition decides. Missing final disposition is a configuration error. Raw-payload modifications must not bypass the final decision | "we should force an explicit final accept or reject on the policy"; "LGTM" approving the syntax and semantics above | this spec |
| M18 | 2026-10-22 | Exception to M8 for implementing M17's explicit final BGP policy disposition: admit this implementation with a cap of 15 while the existing count is 14. The exception does not raise the standing cap for other work | "Authorize a cap exception" selected after being shown 14 active specs, cap 12, and the proposed admission at 15 | this spec; session-scoped `ZE_SPEC_WIP_CAP=15` for admission |
| M19 | 2026-10-22 | Authorize the mechanical migration required by M17 in 19 RFC-tagged Go test files and 14 RFC-tagged functional fixtures. The new policy carrier and syntax must preserve existing protocol and wire assertions; this does not authorize weakening assertions or changing RFC claims | "Approve mechanical migrations" selected after being shown the bounded test population and preservation requirement | explicit-final policy migration; RFC approval records for its tagged test units |
| M23 | 2026-10-22 | Keep the RFC-tagged FlowSpec fixture unchanged during M17's implementation. Approve removal of the temporary shared-helper extension; retain the new FlowSpec-only, no-receive-store setup in the untagged policy test. This does not authorize changes to existing RFC assertions or claims | "Keep the RFC fixture unchanged", reconfirmed after asking to repeat the choice | original `internal/component/bgp/reactor/flowspec_rs_wire_test.go` restored; new setup in `policy_reconcile_test.go` |
| M24 | 2026-10-22 | Correct the RFC8654 fatal-length recovery fixture while keeping its preferred-first announcement order. Verify the preferred route remains selected before failure and the alternate's exact attributes reach the recipient after failure. Preserve the single Loc-RIB replacement and no-transient-withdrawal assertions; update the RFC proof claim and discrimination record. M23's FlowSpec fixture remains unchanged | "Keep preferred-first order" selected after the incorrect worse-route-preemption precondition was explained | `TestRFC8654FatalLengthReelectsAlternateBest` in `reactor/rfc8654_rib_recovery_test.go` |
| M29 | 2026-10-22 | Authorize setup-only corrections to simulated capability negotiation in six RFC-tagged fixtures: `rfc7311_aigp_readvertise_test.go`, `flowspec_forward_wire_test.go`, `rfc2545_forward_subnet_test.go`, `rfc4659_link_local_peering_test.go`, `rfc8955_flowspec_forward_mixed_test.go`, and `rfc8955_width_wire_test.go`. Publish the same exact negotiated families and capabilities to peer and session; preserve assertions, RFC claims, and timeouts. M23 remains in force | "Correct simulated negotiation" selected; "continue" after clarification of a stray message | M17 implementation; managed forwarding fixture corrections |
| M30 | 2026-10-22 | Authorize setup-only negotiation corrections in `rfc2545_api_origination_test.go` and `rfc2545_ordinary_joint_subnet_test.go`, affecting six tagged units. Offer the actual IPv6 families on both sides, preserve IPv4 where sibling assertions require it, and retain optional link-local and extended-message capabilities. Assertions, RFC claims and timeouts remain unchanged | "Correct simulated negotiation" selected after the peer/session family mismatch and two affected files were explained | M17 implementation; ordinary IPv6 origination, policy and joint-subnet fixture setup |
| M31 | 2026-10-22 | Authorize the one-line setup migration in `rfc7911_forward_path_id_gen_test.go`, `TestForwardPathIDFiniteNamespace`: seed each reserved identifier with reference count 1 instead of an empty marker when the allocator gains recipient-ownership accounting. Preserve all assertions and RFC claims | "Approve the one-line setup change" selected after the observed cache-eviction replay failure and identifier-reservation fix were explained | M17 implementation; ADD-PATH identifier lifetime across retained replay |

A further decision of this spec goes in this table, not in `ai/META-DECISIONS.md`.

## Known Limits

General policy-definition-change re-advertisement remains owned by `plan/release/fixes/spec-bgp-export-policy-reload-readvertise.md`. This spec must implement final-disposition changes, including removal/restoration, and must not claim that broader definition-reload task is complete.

## Implementation Summary

### What Was Implemented

- A validated `filterapi.Policy` carries ordered references and a core-owned final. Original global, group and peer direction containers require their own final before inheritance or apply; cumulative references and the most-specific final survive static and dynamic settings publication. Absence and automatic-only filtering remain distinct from configured empty policies.
- The shared chain, ingress, forwarded and originated egress, native-RS exclusion, final writer and dry-run enforce the same terminal decision. Accept continues; reject, execution failure and teardown remain terminal; raw rewrites terminate traversal but still receive final. Empty/inactive final-only decisions use neither RPC nor text rendering. Final rejection preserves explicit withdrawals without allocating a replacement for an unchanged pure withdrawal.
- Export-final transitions reconcile the existing session. Rejection joins admitted writers before withdrawing advertised keys. Acceptance regenerates configured sources and replays retained sources under the actual authorized owner, with publication, replay and live-work ordering. Static and dynamic transactions restore the previous policy after reconciliation failure and retire uncertain sessions if the inverse fails.
- Recipient-held ingress identity and counted allocator reservations preserve ADD-PATH identity beyond source-cache eviction, allocator wrap, reannouncement and same-UPDATE collisions. Recipient slots, source mappings and temporary write proposals release their own reservations independently. Unknown withdrawals do not mint identities or scan recipient routes.

### Bugs Found/Fixed

- Empty/inactive and raw-terminal acceptance bypasses are covered by `TestPolicyFinalNilAPIRails`, `TestPolicyFilterChainFinalDisposition` and the permanent eight-epoch wire matrix.
- A prefiltered announcement racing reject publication is covered by `TestPolicyFinalWriterRejectsOldPrefilteredAnnouncement` and `TestPolicyFinalPausedWriterCannotEscapeReject`.
- Static removal during accepting-policy removal, failed static/dynamic restoration and FlowSpec-only restoration have direct lifecycle tests in `policy_reconcile_test.go`.
- Stale replay versus a same-session withdrawal, including actual cache gap eviction, is covered by the three replay tests in `policy_reconcile_test.go`. Six `policy_replay_identity_test.go` tests cover recipient identity, collisions, local replacement, clear/failure/session replacement and PATHS-LIMIT refusal cleanup. The final targeted race/count-20 result is in the existing Goal Validation table, not a claim of a green full reactor run.
- Wire-oracle field names now come from native full events, FRR MED is read at route-path level, and simulated IPv6 capability setup uses actual bilateral negotiation. These fixture repairs preserve consumer assertions; they do not certify unrelated route-server election work.

### Documentation Updates

The configuration and policy guides, feature/comparison pages, live website examples and authored producers use nested `filters` plus explicit final. Durable producer anchors cover `PolicyFilterChain`, `Policy.AcceptsAtEnd`, `extractFilterPolicy`, `concatPolicies`, `reconcileExportFinal`, `policyReplayOwner`, `flushPolicyReplay`, `requeuePolicyReplay`, `bindSourcePath` and recipient reservation release. Independent closure review also corrected stale `ImportFilters`/`ExportFilters` names, the obsolete extraction anchor, an overbroad mixed-withdrawal statement, a reload statement that omitted export-final reconciliation, and promoted fixtures still called drafts. Historical handover provenance uses the bare spec stem. Native documentation verification remains parent-owned and is not claimed here.

### Deviations from Plan

The final-only reload requirement needed live replay ordering and recipient identity lifetime repairs, not only a policy field swap. They reuse the existing writer, allocator, ownership table and forwarding queue; no second route store or selector was introduced. Owner-approved tagged-fixture changes and their exact limits are recorded in Owner Decisions. The in-tree full reactor result also exposes the separate unfinished RS election cutover; this feature does not absorb or certify it.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | A global-ID lookup appeared sufficient for held replay withdrawals | Cache gap eviction can remove that mapping while the recipient still owns the advertised ID | Actual eviction red and source review | Recipient ingress index plus counted source, recipient and staged reservations; exact wire/lifetime tests |
| approach | Reserving only collisions left same-UPDATE proposals vulnerable | A later path can take an earlier proposed ID after eviction and wrap | Combined collision red emitted IDs 1,0,0 | Reserve every new proposal until recording/refusal; combined-UPDATE regression |
| approach | Handwritten event JSON appeared to prove the reload oracle | Native events use `peer.remote.address` and `update.nlri` | Real reload and native event producer | Native full-event oracle test plus exact received attributes |
| assumption | Synthetic IPv6 setup represented a negotiated session | Peer and Session family intersections differed | Full reactor result and producer review | Owner-approved setup corrections; assertions and claims retained |
| escalation | A dedicated peer address appeared to isolate an allocation test | Literal and registered SourceIDs can share the process-global allocator | Repeated targeted population | Isolated allocator with real cleanup and zero-state checks; `plan/journal/test-shares-a-process-global.md` |
| approach | Updating the policy chapter left old names and reload claims elsewhere | The generic reload chapter and plugin guide also describe the changed producers | Independent source-citer and legacy-symbol review | Corrected durable field/extractor names, reload and withdrawal semantics, and direct producer anchors |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Mandatory explicit final and original-layer refusal | Done | `internal/component/bgp/yang/ze-bgp-conf.yang:344`; `internal/component/bgp/config/redistribution.go:31,69` | No implicit configured final or legacy compatibility branch |
| Cumulative inheritance, empty/absent distinction and automatic filters | Done | `internal/component/bgp/config/redistribution.go:178`; `internal/component/bgp/filterapi/policy.go` | Dynamic templates retain the validated value |
| Core final across every affected live rail | Done | `internal/component/bgp/reactor/filter_chain.go:247`; `filter_ordered.go:147,275,432`; `session_write.go` | Error/teardown and raw traversal preserved |
| Final-only reconciliation, withdrawals and alternatives | Done | `internal/component/bgp/reactor/policy_reload.go:28`; `peer_settings_apply.go:329`; `relay_recovery.go` | Actual replay-owner grant, writer ordering and rollback; no route-ranking change |
| Stable replay identity with bounded additional accounting | Done | `internal/component/bgp/reactor/session_ownership.go:335`; `adj_rib_out.go:292,311,357`; `forward_path_id.go` | Index proportional to recipient slots; temporary bindings proportional to one UPDATE; existing overflow is not a hard-bounded queue |
| Operational final, syntax/producers and durable documentation | Done | `internal/component/bgp/reactor/reactor_api.go:273`; `policy_dryrun.go:25,102`; `internal/component/bgp/plugins/cmd/policy/handler.go:29` | Optional final for automatic-only output; full migration inventory and observed smoke in Goal Validation |

### Acceptance Criteria

The thirteen-row Goal Validation table above is the single AC-to-observed-result ledger. This audit adds the independently read source boundaries rather than duplicating its test logs.

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Schema, `NewPolicy`, original-tree extraction, ordered runtime | Both directions |
| AC-2 | Done | Original-tree validator before environment publication; schema-derived parser corrections | Invalid reload probes a new route under the unchanged live policy |
| AC-3 | Done | Chain exhaustion final and final writer | Passing filter cannot grant acceptance |
| AC-4 | Done | Terminal reject branches in live and traced chain | No final-accept override |
| AC-5 | Done | Raw terminal branch applies final; delta exhaustion applies final | Same wire matrix rejects both |
| AC-6 | Done | Native SDK behavior and wire attribute consumers | Unmodifiable subject attributes remain visible |
| AC-7 | Done | Zero/InternalPolicy state and automatic-filter prepend | Live AUTO output omits final |
| AC-8 | Done | No-active-reference branches and explicit policy presence | No RPC/text/final-decision allocation |
| AC-9 | Done | Original-layer validation, `concatPolicies`, dynamic resolution | Parent omissions are not repaired by children |
| AC-10 | Done | Reconciliation, withdrawal projection, writer joins and replay identity | Targeted repair evidence, not full-tree certification |
| AC-11 | Done | Existing rejected-candidate retry and exact alternate wire oracle | Separate RS election cutover remains outside this verdict |
| AC-12 | Done | Candidate producer/test migrations, removed legacy runtime fields and migrated docs | Historical mentions and rejection fixtures are not active legacy syntax |
| AC-13 | Done | `policyInfo`, chain/peer handlers and `tracePolicyFilterChain` | Empty, automatic, raw and withdrawal outputs distinguished |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Configuration validity and inheritance | Done | `internal/component/bgp/config/policy_test.go:17`; `internal/component/config/parser_diagnostics_test.go:13` | Direct original trees, set syntax, deactivation and environment nonpublication |
| Core final semantics and resources | Done | `internal/component/bgp/reactor/policy_final_test.go:25,59,76,242,340` | Positive controls, terminal failures, nil API and allocations |
| Lifecycle, rollback and replay | Done | `internal/component/bgp/reactor/policy_reconcile_test.go:153,180,222,306,340,480,622,687`; `policy_replay_identity_test.go` | Actual wire and final resource state |
| Functional entry points and alternates | Done | `test/plugin/policy-final-disposition.ci`; `test/reload/bgp-policy-final-reload.ci` | Registered external observers, real sessions, completed reloads and received-wire fences |
| Foreign-peer and semantic discrimination | Done | `internal/le/interoplab/bgp/check_policy_final.go:14,79`; `test/interop/scenarios/bgp-policy-final-frr` | FRR route presence/absence, attributes, no source update or reconnect; observed mutants in Goal Validation |
| Operational parity | Done | `internal/component/bgp/reactor/policy_dryrun_test.go`; `internal/component/bgp/plugins/cmd/policy/handler_test.go:139` | Actual command smoke complements isolated adapters |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| Config/type/schema and JSON-reader population | Done | Actual feature diff and producing functions inspected |
| Runtime, reload, writer and operational population | Done | Actual callers inspected, including late recipient identity repair |
| Executable migration and fixture population | Done | Canonical inventory and promoted fixtures; prior behavior made explicit |
| Durable docs, comparison and website population | Done | Producer-backed prose and source-citer review |
| Three planned new files | Done | `filterapi/policy.go` and both permanent `.ci` files read directly |

### Audit Summary

- **Total items:** 30 (six requirement rows, thirteen AC rows, six test rows, five file-population rows).
- **Done:** 30 implementation items, with verification scope explicitly limited to observed evidence.
- **Partial:** 0 implementation items.
- **Skipped:** 0 implementation items.
- **Changed:** 0 omitted/replaced requirements; supporting repairs are described under Deviations.
- Native committed-tree verification, review recording and closure execution remain open gates, not missing implementation hidden by a Done label.

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| No in-scope product behavior omitted | All thirteen ACs have source and observed evidence | This spec retains its native verification and closure gates until parent completion |
| General filter-definition re-advertisement | Explicitly outside final-transition scope | `plan/release/fixes/spec-bgp-export-policy-reload-readvertise.md` |
| Separate RS competing-route election cutover | Pre-existing unfinished work; twelve full-reactor failures are not claimed fixed | `plan/release/fixes/spec-bgp-rs-replacement-on-withdrawal.md`; `plan/handover/12-rs-replacement-phase-3-remaining.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Not recorded: canonical spec location, final proof metadata and parent-owned verification must settle first |
| `./le spec review check` | Not run by closure reviewer; parent-owned |
| Rounds | 2: full independent pass, then scoped review of its fixes and affected callers; earlier implementation/repair lenses are not counted as formal closure rounds |
| Reviewer lenses used | Deliverables, all ACs, security/trust boundaries, concurrency/lifetime, Go style, documentation/source citers, generic final review |
| Scope | Candidate inventory `policy-pending-files.json`: 372 paths at intake, including 155 Go paths (137 tracked and 18 then-new); actual feature diff reviewed, not unrelated checkout hunks. All changed Go in that population was read, with producing functions and affected callers expanded. Additional already-landed fixture/FRR producers and native protocol summaries were read where needed |
| Independent verdict | CLEAN independent source review: 0 BLOCKER, 0 ISSUE remaining in the reviewed feature/fix scope; changed-Go style pass completed. Native proof freshness, committed-tree verification and the final hash-pinned artifact remain pending; this is not completed closure or a green full-tree claim |

### Round scopes and outcomes

- **Run 1:** Full actual feature diff in the intake candidate population, expanded producing functions/callers, all thirteen ACs, trust and resource boundaries, changed-Go style and durable documentation. Found C-1–C-4 below; no product correctness or security defect.
- **Run 2:** C-3's replay lifecycle and guard corrections plus callers, C-4's stress assertion disposition and actual scenario smoke, and the prose/citer repairs from C-1/C-2. Independently read corrected producers, retained behavioral tests, the weakening record and actual profile-loader output. Result: 0 BLOCKER, 0 ISSUE in scope; no always-in-scope product defect found. Canonical spec edits were initially held during layout migration. The parent then authorized matching closure records in both existing copies, preserving their current headers and owner edits; canonical commit scope remains a parent decision.

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| C-1 | ISSUE | Durable docs retained removed policy field/extractor names, omitted export-final reconciliation from reload prose, and overgeneralized successful withdrawal projection | `docs/architecture/core-design.md`; `docs/guide/plugins.md` | Closure prose corrections grounded in producers |
| C-2 | ISSUE | New replay/dry-run/recipient-identity claims lacked direct producing-function anchors; promoted fixtures still called drafts | API architecture/process protocol, structural forwarding, functional-test docs | Closure source-anchor and terminology corrections |
| C-3 | ISSUE | Reciprocal replay-release MUST contract and terminating compound guards | `policy_reload.go`; `forward_pool.go`; `check_policy_final.go` | Implementation owner corrected the guards and paired contract; closure reviewer read the changed producers, including error-path hold release |
| C-4 | ISSUE | Stress harness policy-reference assertion removals not covered by test-weakening explanation | `internal/le/test/integration/stress_test.go`; `test/weakened/0cf1e0e8.md` | Owner removed the incidental source-text test, retained workflow/query/capture assertions and documented the exact loss. Native real-loader smoke resolves injector import STRESS-IMPORT and receiver export STRESS-EXPORT with final accept; no million-route run claimed |

Security/style source audit: no newly peer-reachable panic found. Registration panic denotes a static duplicate-registration bug; live and dry-run enum panics receive normalized internal actions, with malformed/unknown RPC outcomes failed closed. New raw-key slicing follows native length/identity validation; configuration diagnostic path slicing is bounded by `ListAddressSpan`. New recipient index entries follow installed slots; scratch reservations survive both filtering passes and release on refusal, success and clear/failure. No per-route goroutine or independent desired-route cache was added. Copies are limited to retained ownership/private wire changes and mixed-withdrawal projection; unchanged final-only and pure-withdrawal paths retain their no-extra-allocation contract. Existing forward overflow is explicitly not a fixed-size queue; replay reuses its congestion/shutdown lifecycle and has a two-minute control-plane deadline. This review does not relabel that pre-existing resource design as hard bounded.

Additional real-peer evidence supplied during closure: `job-policy-addpath-final-frr-6c28ad6b.log:8-10` records `bgp-addpath-readvertise-collision-frr` passing in 4m43s and `1 passed, 0 failed`. The outer runner nevertheless exited 1 after Docker refused image cleanup. The parent subsequently observed the referenced container absent. This is an unattributed harness-cleanup outcome, not a failing product assertion or a green overall command; it is recorded in `plan/journal/lifetime-ends-before-its-last-reader.md`.

The actual profile-loader smoke, `job-policy-profile-config-resolved-smoke-dee1cc59.log:2-3`, prints canonical `bgp-filter-modify:STRESS-IMPORT` and `bgp-filter-modify:STRESS-EXPORT` references with final accept. The parent reports its native run passed in 7.02s. This proves the actual scenario's resolved configuration, not a million-route benchmark or replacement of all profile behavior tests.

Native proof renewal remains incomplete: the ADD-PATH batch completed 19 of 21 Go renewals, then its buffered interop observation was killed after 1801 seconds of log silence. The parent restored the exact `forward_path_id.go::generate` body from the native saved overlay. No interrupted observation counts as proof. `plan/journal/concurrent-session-corruption.md` records the separate tooling isolation/progress defect; its repair, the last two record operations and freshness regeneration are parent-owned prerequisites to a final artifact. This separate blocking recorder fix is not another policy product review round. Full committed-tree verification and both closure commits remain pending.

## Pre-Commit Verification

This independent pass reads sources and existing native evidence; it runs no commands that build, test, lint, format, generate, commit or remove files. Parent-owned native checks and committed-tree evidence remain required under current pre-release rules.

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/filterapi/policy.go` | Yes | Direct source read, including validated constructor and absent/internal semantics |
| `test/plugin/policy-final-disposition.ci` | Yes | Direct read of all 154 lines: eight epochs, positive wire fences and registered driver |
| `test/reload/bgp-policy-final-reload.ci` | Yes | Direct read of all 119 lines: three sessions, invalid reload and alternate wire state |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Both directions use validated final | Read schema grouping at `ze-bgp-conf.yang:344` and `filterapi/policy.go`; enum has only accept/reject and no default |
| AC-2 | Invalid original policy cannot apply | Read `validateFilterPolicies`, loader ordering, `TestPolicyRefusalDoesNotPublishEnvironment` and missing-final live reload probe |
| AC-3 | Passing filter cannot bypass reject | Read `PolicyFilterChain` exhaustion and final writer recheck |
| AC-4 | Earlier rejection remains terminal | Read reject branches in `PolicyFilterChain` and `tracePolicyFilterChain` |
| AC-5 | Delta/raw modification cannot grant acceptance | Read raw-terminal final branch and the eight fixture wire epochs |
| AC-6 | Accepted modifications preserve other attributes | Read native SDK subject test and `policyFinalImportedAttributes`/sink attribute assertions |
| AC-7 | Absent and automatic-only policies preserve behavior | Read zero/InternalPolicy construction, prepend caller and optional-final serializer |
| AC-8 | Empty/inactive policies still decide | Read no-active-reference runtime branches and allocation assertions in `policy_final_test.go:242` |
| AC-9 | Original layers validate; effective final is most specific | Read `parsePolicyDisposition`, `validateFilterPolicies`, `concatPolicies` and dynamic settings migration |
| AC-10 | Reject withdraws; acceptance restores retained state | Read writer barriers, replay gate, recipient index and lifetime tests; read final targeted race log ending in PASS |
| AC-11 | Genuine policy rejection permits alternate selection | Read recovery candidate loop and `policyFinalAlternate`, including every-announcement attributes and unchanged source counters |
| AC-12 | Active producers and runtime callers migrated | Read all candidate changed Go and durable prose diffs; legacy-symbol search found only historical/test names plus prose corrected in this pass |
| AC-13 | Display/dry-run agree with live final | Read `policyInfo`, command handlers and `PolicyDryRun`, including raw, mixed withdrawal, error and teardown branches |

The existing Goal Validation table remains the sole observed-result ledger.
These are independent source reads and existing evidence, not new native runs.

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Configuration validator and actual policy reload | `test/reload/bgp-policy-final-reload.ci` | `policyFinalReloadScenario` removes final, requires failed completion, then injects a fresh route and observes unchanged acceptance |
| Import and originated export | `test/plugin/policy-final-disposition.ci` | `register_policy_final.go` dispatches the external observer; parsed-tree edits, source EOR/quiesce, exact received bytes and same-socket permitted fences |
| Final-only withdrawal/restoration and destination alternate | `test/reload/bgp-policy-final-reload.ci` | Echo endpoint is excluded from receive-store/election input; every announcement's original attributes are checked; source counters and connection counts stay fixed |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Raw terminal branch in `PolicyFilterChain` applies the same final without new raw-to-text traversal |
| A-2 | confirmed | Existing recovery retries genuinely rejected candidates; exact alternate fixture proves this policy path only, not the separate RS cutover |
| A-3 | confirmed | Hot-swap reconciliation and same-session real reload evidence require more than field copying |
| A-4 | confirmed | Restoration uses existing retained/configured sources; one-shot API producer remains responsible for reannouncement |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Feature, configuration, CLI examples, plugins, user guides and live examples | Mandatory presence schema; original-tree validator; cumulative extraction; core chain and final writer | Source review complete; native doc gate pending |
| API/RPC and SDK/protocol | Existing filter-update RPC remains unchanged; `PolicyInfo` and operational handlers expose optional configured final | Source review complete; no new transport message claimed |
| Architecture, inheritance, raw traversal, reload and identity | `PolicyFilterChain`, `concatPolicies`, `reconcileExportFinal`, `bindSourcePath`, recipient slot lifecycle | Source review complete; direct anchors corrected |
| Functional-test infrastructure descriptions | Actual `.ci` files, fixture observers, native full-event oracle and FRR path decoder | Source review complete; existing harness reused, proof conventions documented |
| Wire/RFC | No new wire field, family or capability; registered codecs and existing ADD-PATH rules retained | RFC2545 3-3 remains PARTIAL and 3-6 remains UNIMPLEMENTED; parent owns pending proof freshness |
| Route metadata, metrics and plugin inventory | Reviewed changed declarations add no metadata key, metric or plugin registration | No corresponding new declaration to document; final is a core policy value |

## Core Insight

A final policy decision is only effective when its configuration presence, final writer and retained-route lifecycle agree. A source cache cannot be the lifetime authority for an identifier that a remote recipient still holds; the recipient's existing advertisement inventory must preserve that identity until it is removed.
