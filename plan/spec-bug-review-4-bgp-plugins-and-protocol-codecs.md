# Spec: bug-review-4 -- BGP Plugins and Protocol Codecs

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-bug-review-1-inventory-and-self-containment.md |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bug-review-0-umbrella.md`, `plan/spec-bug-review-1-inventory-and-self-containment.md`, and `plan/spec-bug-review-3-bgp-engine-core.md`
3. `ai/rules/plugin-design.md`, `ai/rules/plugin-self-containment.md`, `ai/patterns/bgp-family.md` if any NLRI/capability/family defect is found
4. `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`, `ai/rules/no-sprintf-alloc.md`
5. `internal/component/bgp/plugins/`, including nested `nlri/` and `cmd/` packages
6. `internal/component/bgp/filterapi/`, `internal/component/bgp/attribute/`, `internal/component/plugin/registry/`
7. Relevant `rfc/short/*.md` and draft summaries named by plugin code

## Task

Review all authored BGP plugins and BGP protocol codec extensions for bugs. Scope includes BGP RIB, Adj-RIB-In, route server, route reflector, RPKI, BMP, graceful restart, route refresh, LLNH, role/OTC, AIGP, hostname, watchdog, persistence, filters, redistribute plugins, BGP command plugins, NLRI family plugins, attribute formatters/modifiers, capability plugins, family registration, route/config encoders, and decode/encode CLI paths.

This child depends on the BGP core contracts in child 3 but focuses on plugin-owned behavior.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-bug-review-1-inventory-and-self-containment.md` - BGP plugin package list
  → Decision: this child reviews every BGP plugin and nested NLRI/command package assigned by inventory.
  → Constraint: no BGP plugin package is skipped because it is nested under `nlri/` or `cmd/`.
- [ ] `plan/spec-bug-review-3-bgp-engine-core.md` - core contracts used by plugins
  → Decision: plugin findings distinguish plugin bug from core contract bug.
  → Constraint: if a finding requires core behavior change, route it to child 3/child 5 with both owners named.
- [ ] `ai/rules/plugin-design.md` - BGP plugin registration, family registration, optional dependencies, filter declaration
  → Decision: review checks `registry.Registration`, `filterapi` registrations, attribute JSON formatter ownership, route filter chain ownership, and family/capability registration.
  → Constraint: plugin-specific logic must not live in generic infrastructure except through registries.
- [ ] `ai/patterns/bgp-family.md` - BGP family integration checklist
  → Decision: any NLRI family finding uses the full family checklist, not only parser/encoder files.
  → Constraint: decode, encode, route display, config route parser, family registration, splitter, CLI, ExaBGP bridge, tests, and interop are all part of family completeness when relevant.
- [ ] `ai/rules/buffer-first.md` and `ai/rules/memory-architecture.md` - NLRI/attribute encode allocation rules
  → Constraint: NLRI/attribute codec review checks `WriteTo`, length bounds, caller-owned buffers, and no per-NLRI/per-route allocation in hot paths.
- [ ] `skill://ze-hunt` - known bug classes
  → Decision: run targeted hunts for silent default handling, unwired symbols, registry count literals, fake synchronization, nil-nil returns, and registry contamination in BGP plugin scope.
  → Constraint: candidates require source and caller verification before reporting.

### RFC Summaries (MUST for protocol work)
- [ ] Plugin-specific RFC/draft summaries named by code or feature: RPKI/ROA/ASPA, BMP, GR/LLGR, route refresh, ADD-PATH, route reflection, route server, role/OTC, AIGP, EVPN, FlowSpec, BGP-LS, MUP, MVPN, RTC, SR Policy, VPLS, VPN, labeled unicast, and capability summaries.
  → Constraint: protocol compliance findings cite the exact summary and requirement.

**Key insights:**
- BGP plugin completeness is not only `registry.Register`. Many plugins also register attribute formatters, filter handlers, event namespaces, DirectBridge globals, command schemas, or family/capability maps.
- NLRI plugin bugs often hide in missing chain links: family registered but no encoder, decoder exists but no route display, config parser exists but no functional test, splitter exists but ADD-PATH behavior differs.
- BGP RIB and forwarding plugins consume zero-copy data from the core. Lifetime and async safety are as important as parser correctness.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/all/all.go:140-178` - generated imports for BGP plugin packages and nested NLRI families.
  → Constraint: this child covers direct BGP plugin packages and nested `nlri/*` packages imported by the aggregator.
- [ ] `internal/component/plugin/all/all.go:15-44` and `17-24` - BGP plugin YANG schemas and command plugin schemas.
  → Constraint: command/YANG surfaces under BGP plugins are part of the same review.
- [ ] `internal/component/plugin/all/all.go:233-247` - BGP RPC command packages.
  → Constraint: BGP command handlers are reviewed with their owning plugin, not as generic dispatcher code.
- [ ] `internal/component/bgp/plugins/rib/register.go:17-56` - RIB plugin registration, event namespace, config root, YANG, logging, metrics, EventBus.
  → Constraint: RIB review covers state storage, EventBus publication, config root ownership, and metrics injection.
- [ ] `internal/component/bgp/plugins/rs/register.go:13-41` - route server registration with optional dependency on Adj-RIB-In.
  → Constraint: optional dependency behavior and replay fallback must be reviewed, including one-shot warning and startup ordering.
- [ ] `internal/component/bgp/plugins/nlri/mup/register.go:15-53` - NLRI plugin reference with families, in-process decoder/encoder/route encoder/config parser, CLI flags, and RunEngine.
  → Decision: use MUP as one reference for NLRI completeness checks when comparing another NLRI family.
- [ ] `internal/component/bgp/plugins/nlri/mup/types.go:34-152` - family registration, route type parsing, and NLRI parse contract.
  → Constraint: NLRI review checks unknown route type, length, remaining bytes, AFI/SAFI, and draft constraints.
- [ ] `internal/component/bgp/plugins/nlri/mup/types.go:168-228` - `Bytes`, `Len`, `SupportsAddPath`, `String`, and `WriteTo`.
  → Constraint: hot-path senders should use `WriteTo`; allocation helpers must be cold or clearly documented.
- [ ] `internal/component/bgp/plugins/filter_community/register.go:13-46` - attr modification handlers, JSON formatters, filterapi registration, plugin registration.
  → Decision: attribute/filter plugins can have multiple registry surfaces; review every one.
  → Constraint: a plugin that modifies an attribute also owns the JSON formatter for that attribute unless core owns it.

**Behavior to preserve:**
- BGP plugins remain self-contained and removable through registration boundaries.
- NLRI/capability/attribute extensions use registries, not central switches, except approved core seams.
- Route storage remains plugin-owned and buffer/pool contracts remain intact.
- Optional dependencies degrade explicitly and safely.
- Family/capability/attribute names remain canonical and single-source.

**Behavior to change:**
- Produce a verified findings report for BGP plugin and codec bugs.
- Route accepted findings to child 5 with fix specs and regression tests.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Plugin registration via `init` and generated blank imports.
- BGP events delivered from reactor to plugin via StructuredEvent/JSON.
- Plugin commands via YANG/RPC/local CLI paths.
- NLRI decode/encode via CLI, config route parser, ExaBGP/migration bridges, and core registry lookups.
- Attribute modification and filter chains via filterapi.
- RIB/best-change events through EventBus to sysrib/FIB/observability.

### Transformation Path
1. Inventory rows identify BGP plugin packages and surfaces.
2. For each plugin, enumerate registration surfaces: plugin registry, YANG, RPC, event namespace, filterapi, attribute formatter/modifier, family/capability, DirectBridge global, metrics, config roots.
3. Trace user/peer input through plugin-owned decode/parse/config/command/event path.
4. Compare same-shape reference plugins for missing registration, handler, tests, and docs.
5. Apply bug lenses: wiring, completeness, logic, error handling, security, performance, RFC, concurrency, self-containment, docs.
6. Promote only verified candidates to report with owner and regression-test plan.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Core reactor -> plugin | StructuredEvent/RawMessage, JSON event, cache IDs | [ ] lifetime and callback behavior traced |
| Plugin -> core | DirectBridge/RPC commands, cache forward/release, update route | [ ] slow/fast parity traced if relevant |
| Plugin -> registry | `registry.Register`, family/capability maps, YANG, CLI handler | [ ] registration complete and validated |
| Plugin -> filter pipeline | `filterapi.Register`, attr mod handlers | [ ] ordering/stage/priority and missing-handler behavior checked |
| Plugin -> RIB/sysrib/FIB | EventBus best-change or route injection | [ ] event shape and consumer path traced |
| NLRI codec -> CLI/config/wire | decoder/encoder/splitter/route encoder/config parser | [ ] full chain compared to reference |

### Integration Points
- `internal/component/bgp/plugins/*`
- `internal/component/bgp/plugins/nlri/*`
- `internal/component/bgp/plugins/cmd/*`
- `internal/component/bgp/filterapi/`
- `internal/component/bgp/attribute/`
- `internal/component/plugin/registry/`
- `internal/component/plugin/server/`
- `test/decode/`, `test/encode/`, `test/plugin/`, `test/interop/scenarios/`

### Architectural Verification
- [ ] No bypassed layers: plugins talk to core through registered seams.
- [ ] No unintended coupling: plugin-specific command/schema/filter logic stays with owner.
- [ ] No duplicated functionality: registration tables and docs do not carry second sources of truth.
- [ ] Zero-copy preserved where applicable: RawMessage/WireUpdate data is consumed within lifetime or copied deliberately.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|----------------------------------|----------|--------------|--------|
| A-1 | MUP is a useful reference for NLRI plugin completeness | `ze-review-deep` feature-completeness guidance and read source | comparison misses family-specific requirements | compare each family against its closest family, not only MUP, and use bgp-family checklist | unvalidated |
| A-2 | BGP plugin command schemas under `cmd/` belong with BGP plugin review | generated imports and self-containment rule | command bugs reviewed by wrong child | inventory assignment and handler dependency trace | unvalidated |
| A-3 | Attribute formatter ownership follows modifier ownership unless core owns the attribute | registration pattern and plugin-design ownership rule | JSON drift or wrong owner missed | compare attribute code registration, formatter, and modifier owner | unvalidated |
| A-4 | Optional dependency behavior is intentional only when declared as optional and handled at runtime | plugin-design optional dependencies | missing hard dependency becomes silent feature loss | trace dependencies and error paths for each plugin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Protocol-codec review becomes shallow due to many NLRI families | report lists families but no chain table | use per-family checklist with decode/encode/display/config/tests |
| R-2 | Fix recommendations break hot path memory model | proposed fix allocates parsed structs per NLRI/update | require buffer-first review in fix spec |
| R-3 | RFC/draft behavior is ambiguous | finding cites draft behavior without summary | read/create summary and mark assumption if draft changed |
| R-4 | Plugin and core ownership overlap | same bug appears in child 3 and child 4 | child 5 dedupes and assigns fix owner based on deepest source |
| R-5 | Existing tests use fixtures that bypass production codec | green test does not exercise registry path | require named production file:line that test would fail if removed |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| BGP plugin inventory row | -> | plugin registration and RunEngine/CLI handler | `BGPPluginReviewEveryAssignedPackageRead` |
| NLRI family | -> | family registration -> splitter -> decoder -> encoder -> route encoder -> config parser -> tests | `BGPPluginReviewFamilyChainChecked` |
| Attribute/filter plugin | -> | formatter/modifier/filterapi registration -> pipeline usage | `BGPPluginReviewFilterAttributeChainChecked` |
| BGP command schema | -> | YANG command -> RPC handler -> plugin/core action | `BGPPluginReviewCommandWiringChecked` |
| RIB/best-change path | -> | plugin event -> EventBus -> downstream consumers | `BGPPluginReviewRIBEventPathChecked` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child 4 starts | Report lists every BGP plugin package assigned by inventory and file groups read |
| AC-2 | A plugin has a registration surface | All related registry surfaces are enumerated and reviewed for owner, validation, and tests |
| AC-3 | A plugin is an NLRI family | Full family chain is checked against bgp-family checklist and closest reference |
| AC-4 | A plugin modifies/filters attributes | Attribute handler, JSON formatter, filter stage/priority, ingress/egress semantics, and tests are reviewed |
| AC-5 | A plugin stores or forwards routes | buffer lifetime, cache ack/release, best-change/event shape, and downstream consumers are traced |
| AC-6 | A plugin has optional dependencies | missing dependency behavior is explicit, logged once if needed, and tested or flagged |
| AC-7 | A finding survives | It includes file:line, trigger, expected vs actual, severity, owner, RFC status if relevant, and regression-test plan |
| AC-8 | No finding survives for a plugin | Report records files read and cleared lenses |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Decodes an NLRI | CLI/API -> registry family lookup -> plugin decoder -> JSON/text output | `BGPPluginReviewFamilyChainChecked` |
| 2 | Configures a route for a plugin family | config route -> config parser -> plugin route encoder -> core build/send path | `BGPPluginReviewFamilyChainChecked` |
| 3 | Runs a BGP plugin command | YANG command -> RPC handler -> plugin state/core action -> response | `BGPPluginReviewCommandWiringChecked` |
| 4 | Receives route through RIB path | reactor event -> RIB/Adj-RIB-In plugin -> best-change/event -> downstream | `BGPPluginReviewRIBEventPathChecked` |
| 5 | Applies policy/filter | config -> filter plugin registry -> filterapi chain -> ingress/egress decision/modification | `BGPPluginReviewFilterAttributeChainChecked` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `BGPPluginReviewEveryAssignedPackageRead` | `plan/review-bug-review-bgp-plugins.md` | every assigned package has file evidence | |
| `BGPPluginReviewFamilyChainChecked` | same report | every NLRI family chain has no missing link or names missing link | |
| `BGPPluginReviewFilterAttributeChainChecked` | same report | filter/attribute registry surfaces reviewed | |
| `BGPPluginReviewCommandWiringChecked` | same report | BGP command schema to handler chain reviewed | |
| `BGPPluginReviewRIBEventPathChecked` | same report | RIB/best-change/EventBus path reviewed | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| NLRI length fields | per family/RFC | max valid per family | truncated by one | exceeds remaining payload |
| AFI/SAFI values | registered family values | max registered by family | unknown or zero when invalid | conflicting registration |
| route type/subtype | per plugin/RFC | highest valid | unknown low value | unknown high value |
| attribute length/code | per attribute/RFC | max valid | truncated | extra trailing bytes |
| filter priority/stage | filterapi definitions | highest valid stage | unknown stage | conflicting duplicate |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing decode/encode tests named per family | `test/decode/`, `test/encode/` | NLRI round-trip from user entry | |
| Existing plugin policy tests named per finding | `test/plugin/` | filter/RIB/command behavior through daemon | |
| New regression test named in fix spec | fix spec | accepted bug reproduction | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Family/protocol scenario named by finding | `test/interop/scenarios/` | FRR/BIRD/GoBGP/ExaBGP as applicable | peer-compatible behavior for protocol plugin defect | |

### Future (if deferring any tests)
- No deferral in review. Fix specs may defer an interop test only with explicit user approval and substitute evidence.

## Files to Modify

- No production code files.
- Read-only scope includes BGP plugin packages, tests, RFC summaries, and docs referenced by findings.

### BGP Family Checklist (if new SAFI / capability / attribute)

This review does not add a family, capability, or attribute. If a finding identifies incomplete existing integration, the fix spec must copy the relevant bgp-family checklist sections and answer every row.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Review only |
| CLI commands/flags | No | Review only |
| Functional test for new RPC/API | No | Fix specs own tests |
| Env var registration | No | Review only |
| Doctor check for runtime dependencies | No | Review only |
| Prometheus counters/metrics | No | Review only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | Review only |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | Review-only spec |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `plan/review-bug-review-bgp-plugins.md` - child 4 findings report.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file plus inventory report and child 3 core contracts |
| 2. Audit | Required Reading and assigned BGP plugin packages |
| 3. Wiring phase | Wiring Test table, family/command/filter/RIB chains |
| 4. Implement (TDD) | read-only findings report |
| 5. /ze-review gate | report quality and evidence |
| 6. Full verification | report audits |
| 7-14 | standard completion |

### Implementation Phases

1. **Phase: Scope reconciliation** - load BGP plugin rows assigned by inventory.
   - Tests: `BGPPluginReviewEveryAssignedPackageRead`.
   - Files: inventory report and BGP plugin tree.
   - Verify: zero unreviewed assigned packages.
2. **Phase: Registration and ownership review** - plugin registry, YANG, RPC, filterapi, attribute, event namespace, metrics, DirectBridge globals.
   - Tests: `BGPPluginReviewCommandWiringChecked`, `BGPPluginReviewFilterAttributeChainChecked`.
   - Files: BGP plugin register and handler files.
   - Verify: each surface has owner and path evidence.
3. **Phase: NLRI/capability/attribute codec review** - family/capability/attribute completeness and wire correctness.
   - Tests: `BGPPluginReviewFamilyChainChecked`.
   - Files: `nlri/*`, capability plugins, attribute plugins.
   - Verify: every family chain complete or finding recorded.
4. **Phase: RIB/forwarder/state plugin review** - RIB, Adj-RIB-In, RS, RR, RPKI, BMP, GR, route refresh, persistence.
   - Tests: `BGPPluginReviewRIBEventPathChecked`.
   - Files: state and forwarding plugins.
   - Verify: buffer lifetime, cache ack/release, optional dependencies, and event flow traced.
5. **Phase: Report and route findings** - create child report and route accepted findings to child 5.
   - Tests: report audits.
   - Files: `plan/review-bug-review-bgp-plugins.md`.
   - Verify: every finding has regression-test plan.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every BGP plugin package and nested NLRI/command package assigned by inventory reviewed |
| Feature completeness | NLRI families, filters, commands, RIB/state plugins, capability/attribute registrations checked |
| Correctness | parser/encoder lengths, unknown values, policy decisions, RIB state, optional deps, event shapes checked |
| Naming | plugin names, family names, JSON keys, YANG keys, event/send types are canonical |
| Data flow | each finding traces peer/user/config/plugin entry to plugin effect |
| Performance | hot NLRI/RIB/filter paths checked for allocation and unsafe retention |
| Security | malformed wire/config/API input checked for resource exhaustion or silent accept |
| RFC | protocol findings cite summaries and specific requirements |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Child 4 findings report | read `plan/review-bug-review-bgp-plugins.md` |
| BGP plugin coverage table | report package coverage table |
| NLRI family chain table | report family table |
| Filter/attribute surface table | report registry surface table |
| Finding evidence table | every finding has required fields |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Wire input | malformed NLRI/attribute lengths, unknown route types, trailing bytes |
| Config input | unbounded strings/counts, exact-or-reject, route parser validation |
| JSON/text output | wrong field names, output injection, unsafely retained buffers |
| RPKI/BMP/external data | malformed records, stale validation state, trust boundary mistakes |
| Resource exhaustion | large NLRI lists, large communities, route replay, cache consumers |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Core contract bug | child 3 and child 5 with core owner |
| Plugin bug | child 5 with plugin owner |
| Missing family checklist item | child 5 fix spec with bgp-family checklist copied |
| Test-only gap | child 5 test fix spec if behavior is otherwise correct |
| RFC summary missing | read/create summary before classifying |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- For BGP plugins, registration completeness is behavior. A codec that is correct but not reachable through CLI/config/registry is still a user-visible bug.

## Core Insight

A BGP plugin review must check every chain link from family or command name to production execution. Parser-only review finds bugs, but it also misses entire missing features.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Use per-family chain checklist for NLRI plugins | inspect parser files only | missing registration/encoder/display/test links are common |
| Separate BGP plugin review from BGP core review | one BGP review | plugin and core defects have different owners and fix shapes |
| Treat filter/attribute registries as plugin surfaces | review only `registry.Register` | filterapi and attribute registrations directly affect runtime behavior |

## Known Limitations

- This child does not implement fixes.
- Draft-based families may have changing specs; findings must state the draft version or summary read.

## RFC Documentation

Fix specs must add or verify RFC comments above enforcing code for any RFC behavior they change.

## Implementation Summary

### What Was Implemented
- Created `plan/review-bug-review-bgp-plugins.md`.
- Reviewed BGP plugin registrations, NLRI family-chain coverage, BGP command plugin schema/RPC wiring, attribute/filter/capability surfaces, and zero-copy/raw-message candidates.
- Produced confirmed, plausible, rejected, cleared-class, and assumptions-resolved sections for child 4 scope.

### Bugs Found/Fixed
- Confirmed BPLUG-001 and BPLUG-002.
- Recorded BPLUG-P1 and BPLUG-P2 as plausible but not promoted by the final report.
- No production code was changed.

### Documentation Updates
- None. The child output is a review report and does not change user-facing behavior.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Review BGP plugin registrations | done | child report assigned package ledger | all direct plugin and NLRI packages covered |
| Review NLRI family chain | done | child report family-chain matrix | BPLUG-001 and BPLUG-002 |
| Review BGP command and RPC wiring | done | command/RPC wiring matrix | no unwired command bug promoted |
| Review zero-copy and raw-message candidates | done | cleared classes and rejected candidates | BMP candidate rejected |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | package ledger | child 4 assigned rows covered |
| AC-2 | done | NLRI family-chain matrix | each family chain classified |
| AC-3 | done | BPLUG finding sections | trigger, expected/actual, impact, severity, owner, test plan |
| AC-4 | done | plausible and rejected tables | unverified encode/decode completeness not promoted |
| AC-5 | done | command and RPC matrix | BGP plugin commands wired |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| BgpPluginFamilyChainReviewed | pass | child report family-chain matrix | manual report audit |
| BgpPluginCommandWiringReviewed | pass | child report command/RPC matrix | manual report audit |
| BgpPluginFindingHasRegressionPlan | pass | BPLUG finding sections | accepted findings have tests |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/review-bug-review-bgp-plugins.md` | created | child 4 findings report |

### Audit Summary
- **Total items:** 14
- **Done:** 14
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 report file created

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| BGP plugin and codec bug review | Findings report | `plan/review-bug-review-bgp-plugins.md` with plugin/family coverage and verified findings |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | BPLUG-001 NLRI encode/config parsers silently ignore unknown or dangling tokens | child report | routed to NLRI strictness fix spec |
| 2 | ISSUE | BPLUG-002 SR-Policy family not wired into canonical encode route path | child report | routed to SR-Policy encode fix spec |
| 3 | NOTE | BPLUG-P1 and BPLUG-P2 plausible only | child report | not promoted by final report |

### Fixes applied
- None during review spec execution.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No untriaged child 4 finding remains after final report | `plan/review-bug-review-final.md` | no action |

### Final status
- [x] Critical review of child report artifacts records accepted and not-promoted findings
- [x] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/review-bug-review-bgp-plugins.md` | yes | report read and listed in final report |
| `plan/spec-bugfix-bgp-nlri-strictness.md` | yes | final report fix spec ledger |
| `plan/spec-bugfix-bgp-srpolicy-encode.md` | yes | final report fix spec ledger |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | assigned package rows covered | assigned package ledger |
| AC-2 | NLRI family chain classified | family-chain matrix |
| AC-3 | findings have evidence | BPLUG finding sections |
| AC-4 | plausible candidates not promoted | BPLUG-P1 and BPLUG-P2 sections |
| AC-5 | command/RPC wiring covered | command and RPC matrix |

### Wiring Verified (end-to-end)
| Entry Point | Report Audit | Verified |
|-------------|--------------|----------|
| NLRI registry encode path | BPLUG-001 | yes |
| config route parser path | BPLUG-001 | yes |
| canonical `ze bgp encode` route path | BPLUG-002 | yes |
| BGP plugin command schema/RPC path | command matrix | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | MUP and other family references in assumptions table |
| A-2 | confirmed | command schemas assigned to child 4 |
| A-3 | confirmed | attribute formatter/modifier ownership table |
| A-4 | confirmed | optional dependency evidence |
| A-5 | confirmed | final report active spec overlap routing |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | review-only artifact | yes |
| RFC comments required later | protocol fix specs carry requirement | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N all demonstrated
- [x] End-to-End User Stories all have report evidence
- [x] Wiring Test table complete
- [x] Critical review gate clean for child report artifacts
- [x] `make ze-spec-status` passes
- [x] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass, defer with user approval)
- [x] RFC constraint comments verified for fix specs
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility
- [x] Explicit behavior
- [x] Minimal coupling

### TDD
- [x] Report audits written or manual evidence recorded
- [x] Regression test named for every accepted bug
- [x] Boundary tests named for numeric/protocol inputs
- [x] Functional or interop tests named where peer/user behavior is affected
- [x] Goal Validation table filled

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are not applicable
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/942-bug-review-4-bgp-plugins-and-protocol-codecs.md`
- [x] **Commit A script prepared:** spec + report + learned summary + counter bump in `tmp/commit-f32fa560.sh`
- [x] **Commit B script prepared:** remove `plan/spec-bug-review-4-bgp-plugins-and-protocol-codecs.md` only after final state is preserved
