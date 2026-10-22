# Spec: BGP explicit final policy disposition

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 1/4 — schema and carrier; independent migrations |
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

## Current Behavior

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
| A-2 | Existing alternative selection can consume final rejection | `queueRecoveryOutcome` already skips rejected candidates | Need repair at writer/recovery boundary | Real-peer alternate and all-rejected cases | unvalidated |
| A-3 | Final-only reload needs reconciliation beyond field copying | `applyHotSwappableSettings` tail has no policy replay | Existing machinery may provide a reusable entry point | Trace and exercise reload withdrawal | unvalidated |
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
| 10 | Test infrastructure | No | Add scenarios using existing harness |
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

### Goal Gates

- [ ] All AC rows have concrete evidence.
- [ ] Scoped package checks and real entry-point smoke are observed.
- [ ] Independent review covers all affected producers/callers.
- [ ] Existing unrelated reds are disclosed, never hidden by weakening tests.
- [ ] Applicable native verification runs through the repository launcher and throwaway verification worktree.

## Design Decisions

| Decision | Alternative | Reason |
|----------|-------------|--------|
| Core-owned final field | Final plugin | Owner approved mandatory structural decision, not optional plugin ordering |
| Validated value carrier | Parallel loose fields or compatibility slices | Preserve coherent snapshots and prevent invalid configured state |
| Preserve raw traversal boundary, then final | New raw-to-text composition | Required safety without unrelated encoding machinery |
| Most-specific final and cumulative filters | Multiple layer-level terminal decisions | Fits current filter inheritance and makes one effective policy decision |

## Owner Decisions

Moved here from `ai/META-DECISIONS.md` on 2026-10-22 on the owner's order
("move"): each is a decision of this spec, not a standing decision about how
agents work. The ids are kept, so the citations above still resolve.

| Id | Date | Decision | Thomas's words | Applied in |
|----|------|----------|----------------|------------|
| M17 | 2026-10-22 | A configured BGP policy must state an explicit final accept or reject disposition, separate from its filters: `filter { import { filters [ CUSTOMERS TAG ]; final accept; } }`. A filter's acceptance continues; rejection stops. At chain exhaustion the final disposition decides. Missing final disposition is a configuration error. Raw-payload modifications must not bypass the final decision | "we should force an explicit final accept or reject on the policy"; "LGTM" approving the syntax and semantics above | this spec |
| M18 | 2026-10-22 | Exception to M8 for implementing M17's explicit final BGP policy disposition: admit this implementation with a cap of 15 while the existing count is 14. The exception does not raise the standing cap for other work | "Authorize a cap exception" selected after being shown 14 active specs, cap 12, and the proposed admission at 15 | `plan/immediate/spec-bgp-explicit-final-policy.md`; session-scoped `ZE_SPEC_WIP_CAP=15` for admission |
| M19 | 2026-10-22 | Authorize the mechanical migration required by M17 in 19 RFC-tagged Go test files and 14 RFC-tagged functional fixtures. The new policy carrier and syntax must preserve existing protocol and wire assertions; this does not authorize weakening assertions or changing RFC claims | "Approve mechanical migrations" selected after being shown the bounded test population and preservation requirement | explicit-final policy migration; RFC approval records for its tagged test units |
| M23 | 2026-10-22 | Keep the RFC-tagged FlowSpec fixture unchanged during M17's implementation. Approve removal of the temporary shared-helper extension; retain the new FlowSpec-only, no-receive-store setup in the untagged policy test. This does not authorize changes to existing RFC assertions or claims | "Keep the RFC fixture unchanged", reconfirmed after asking to repeat the choice | original `internal/component/bgp/reactor/flowspec_rs_wire_test.go` restored; new setup in `policy_reconcile_test.go` |
| M24 | 2026-10-22 | Correct the RFC8654 fatal-length recovery fixture while keeping its preferred-first announcement order. Verify the preferred route remains selected before failure and the alternate's exact attributes reach the recipient after failure. Preserve the single Loc-RIB replacement and no-transient-withdrawal assertions; update the RFC proof claim and discrimination record. M23's FlowSpec fixture remains unchanged | "Keep preferred-first order" selected after the incorrect worse-route-preemption precondition was explained | `TestRFC8654FatalLengthReelectsAlternateBest` in `reactor/rfc8654_rib_recovery_test.go` |
| M29 | 2026-10-22 | Authorize setup-only corrections to simulated capability negotiation in six RFC-tagged fixtures: `rfc7311_aigp_readvertise_test.go`, `flowspec_forward_wire_test.go`, `rfc2545_forward_subnet_test.go`, `rfc4659_link_local_peering_test.go`, `rfc8955_flowspec_forward_mixed_test.go`, and `rfc8955_width_wire_test.go`. Publish the same exact negotiated families and capabilities to peer and session; preserve assertions, RFC claims, and timeouts. M23 remains in force | "Correct simulated negotiation" selected; "continue" after clarification of a stray message | M17 implementation; managed forwarding fixture corrections |
| M30 | 2026-10-22 | Authorize setup-only negotiation corrections in `rfc2545_api_origination_test.go` and `rfc2545_ordinary_joint_subnet_test.go`, affecting six tagged units. Offer the actual IPv6 families on both sides, preserve IPv4 where sibling assertions require it, and retain optional link-local and extended-message capabilities. Assertions, RFC claims and timeouts remain unchanged | "Correct simulated negotiation" selected after the peer/session family mismatch and two affected files were explained | M17 implementation; ordinary IPv6 origination, policy and joint-subnet fixture setup |

A further decision of this spec goes in this table, not in `ai/META-DECISIONS.md`.

## Known Limits

General policy-definition-change re-advertisement remains owned by `plan/immediate/spec-bgp-export-policy-reload-readvertise.md`. This spec must implement final-disposition changes, including removal/restoration, and must not claim that broader definition-reload task is complete.
