# Spec: bug-review-0 -- Plugin and Core Bug Review (Umbrella)

| Field | Value |
|-------|-------|
| Status | done |
| Depends | - |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `ai/rules/planning.md` - spec workflow and completion gates
3. `skill://ze-review` and `skill://ze-review-deep` - review method and lenses
4. `skill://ze-hunt` and `plan/learned/RECURRING-PATTERNS.md` - recorded bug classes
5. `ai/rules/plugin-design.md`, `ai/rules/plugin-self-containment.md`, `ai/patterns/registration.md` - plugin boundary invariants
6. `docs/architecture/core-design.md` - BGP engine, plugin server, data flow, memory model
7. Child specs: `spec-bug-review-1-inventory-and-self-containment.md` through `spec-bug-review-5-verification-and-fix-backlog.md`

## Task

Run a systematic bug review of every in-tree plugin we authored and the core engine that wires them together. This is a review program, not a fix program. The output is evidence-backed findings and follow-up fix specs for confirmed or plausible defects.

The review set covers:

| Area | Child spec | Scope |
|------|------------|-------|
| Inventory and ownership | `spec-bug-review-1-inventory-and-self-containment.md` | Authoritative list of compiled plugin packages, schema-only plugin surfaces, RPC command packages, and ownership/removal rules |
| Plugin engine and system plugins | `spec-bug-review-2-plugin-engine-and-system-plugins.md` | `internal/component/plugin/`, `pkg/plugin/`, `internal/plugins/`, component-owned plugin packages, lifecycle/config/RPC/doctor/YANG surfaces |
| BGP core engine | `spec-bug-review-3-bgp-engine-core.md` | FSM/session, message/wire/capability/attribute, reactor, filter chains, forward cache, pools, route building |
| BGP plugins and protocol codecs | `spec-bug-review-4-bgp-plugins-and-protocol-codecs.md` | `internal/component/bgp/plugins/`, NLRI families, RIB/RS/RR/RPKI/GR/BMP/filter plugins, BGP plugin registrations |
| Verification and fix backlog | `spec-bug-review-5-verification-and-fix-backlog.md` | Deduped findings, reproduction/regression-test plans, fix-spec creation, final evidence |

The review must not report grep hits as bugs. Every finding needs source evidence, a reachable trigger, expected vs actual behavior, and a proposed regression test. If a defect requires code changes, create or update a separate fix spec. Do not fix code while executing the review specs unless the user explicitly selects that finding for implementation.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - canonical engine/plugin architecture
  → Decision: review must follow the actual data flow: peer FSM -> wire layer -> reactor -> EventDispatcher -> plugin server, with RIB state in plugins rather than the reactor.
  → Constraint: internal plugins may use DirectBridge after readiness; external plugin paths still use newline-framed YANG RPC, so both paths need coverage.
- [ ] `ai/rules/plugin-design.md` - plugin boundaries and registration contracts
  → Decision: plugin review is organized around registration, YANG/RPC ownership, value-typed boundaries, optional dependency behavior, and startup phase ordering.
  → Constraint: infrastructure must not import plugin implementations directly except blank-import aggregators and schema imports.
- [ ] `ai/rules/plugin-self-containment.md` - removal test
  → Decision: every plugin surface is reviewed with the folder-removal invariant: deleting a plugin and its blank import removes only that plugin's features.
  → Constraint: generic command/schema/help packages may carry selector scope, but not plugin command spelling or plugin-owned handlers.
- [ ] `ai/patterns/registration.md` - registration mechanisms
  → Decision: the inventory child derives coverage from registries and generated aggregators, not from memory or documentation.
  → Constraint: every registered surface must have one authoritative owner and a consistency check or review evidence.
- [ ] `ai/rules/data-flow-tracing.md` - full data-flow review requirement
  → Constraint: each child spec must trace entry points, transformations, boundary crossings, and integration points before accepting or clearing a finding.
- [ ] `plan/learned/RECURRING-PATTERNS.md` - known traps
  → Decision: review includes targeted hunts for silent parser fall-through, unwired features, hardcoded registry counts, nil-nil returns, fake synchronization, net.Pipe deadlocks, and registry contamination.
- [ ] `ai/rules/buffer-first.md`, `ai/rules/memory-architecture.md`, `ai/rules/no-sprintf-alloc.md` - performance and allocation invariants
  → Constraint: BGP engine and BGP plugin hot paths are reviewed for caller-owned buffers, pool lifecycle, lazy parsing, and no per-event string or byte allocations.

### RFC Summaries (MUST for protocol work)
- [ ] Relevant `rfc/short/*.md` and `iso/short/*.md` summaries for code touched by a child finding
  → Constraint: protocol findings must cite the specific RFC or state that the defect is implementation-internal rather than RFC-driven.

**Key insights:**
- The source of review scope is generated code and registries: `internal/component/plugin/all/all.go`, plugin registry registrations, RPC command registrations, and YANG module registrations.
- Review is split by boundary, not by directory alone. The same bug class appears differently in plugin startup, BGP forwarding, system plugin config, and protocol codecs.
- The most common project failure is unwired or partially wired behavior. Wiring review runs first in every child.
- Fixes are intentionally outside this umbrella. The review closes only when findings are deduped, verified, and each accepted issue has a fix spec with a regression test plan.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/all/all.go:10-273` - generated aggregator blank-imports infrastructure schema packages, plugin packages, event namespace packages, and RPC command packages.
  → Constraint: review inventory must include both plugin packages and schema/RPC surfaces because user-visible bugs can live in either half.
- [ ] `docs/architecture/core-design.md:70-82` - engine supervises startup, plugin manager handles process lifecycle, EventDispatcher handles data delivery, dynamic event/send types register at startup.
  → Constraint: review must cover startup phases, dynamic event/send registration, config-root autoload, and reactor-optional behavior.
- [ ] `docs/architecture/core-design.md:336-400` - plugin API communication and what engine stores versus plugin stores.
  → Constraint: review must separately exercise text update commands, raw wire command paths, JSON event delivery, DirectBridge, cache-forward IDs, and plugin-owned route storage.
- [ ] `docs/architecture/core-design.md:496-600` - receive/API/forwarding path summary, including RS fast path, slow text path, and DirectBridge path.
  → Constraint: a BGP forwarding finding is not cleared until all three forwarding shapes are considered.
- [ ] `plan/spec-cross-plugin-switch-audit.md:18-115` - existing skeleton for cross-plugin switch review.
  → Decision: this review set does not replace that spec; it references it where switch placement and self-containment overlap.
- [ ] `ai/rules/plugin-design.md:155-230` - five-stage protocol, OnStarted vs OnAllPluginsReady, registration fields, optional dependencies.
  → Constraint: startup bugs must be reviewed against phase ordering and dependency semantics, not just compile-time imports.
- [ ] `plan/learned/RECURRING-PATTERNS.md:106-135` - unwired feature trap.
  → Constraint: every child review starts with user-entry reachability.

**Behavior to preserve:**
- Review does not modify production code.
- Existing active specs remain independent; findings against active work are reported with explicit spec cross-references.
- Generated files stay generated. Any fix spec must modify canonical sources and regenerate as required.
- The plugin removal invariant remains the authority for ownership decisions.
- BGP hot paths keep zero-copy and buffer-first constraints.

**Behavior to change:**
- The project gains a structured review backlog and fix-spec queue for plugin/core bugs.
- Future review work uses the child specs rather than ad-hoc directory scans.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Review input enters from source code, generated registries, current specs, learned bug classes, automated checks, tests, and optional reviewer subagent reports.
- Finding input format is a candidate defect with source site, trigger path, affected user operation, expected behavior, actual behavior, severity, and regression-test plan.

### Transformation Path
1. Inventory child derives the authoritative plugin/core scope from generated imports, directory registries, and runtime registration points.
2. Area children run deterministic checks and focused review lenses over their assigned boundaries.
3. Main session verifies each candidate by reading source, tracing callers or user entry points, and rejecting ungrounded claims.
4. Verification child dedupes findings, maps them to owners, and creates follow-up fix specs for accepted issues.
5. Final report summarizes clean areas, confirmed issues, plausible issues, rejected candidates, and next fix order.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Source code -> review candidate | search, AST/LSP, direct read, automated check output | [ ] Candidate cites file:line and code path |
| Candidate -> verified finding | caller/data-flow trace by main session | [ ] Finding includes trigger and expected vs actual behavior |
| Finding -> fix backlog | separate fix spec or explicit rejection | [ ] Each accepted issue has a spec or owner decision |
| Plugin -> engine | registration, RPC, DirectBridge, events, config, command dispatch | [ ] Child 2 evidence |
| Wire/FSM/reactor -> plugin | WireUpdate, cache IDs, StructuredEvent, JSON/text | [ ] Child 3 and child 4 evidence |

### Integration Points
- `internal/component/plugin/all/all.go` - generated inventory anchor.
- `internal/component/plugin/registry/` and `internal/component/plugin/server/` - plugin registration, startup, command dispatch, DirectBridge, subscriptions.
- `internal/component/bgp/` - BGP engine source of wire, reactor, session, capability, filter, and forwarding behavior.
- `internal/component/bgp/plugins/` - BGP plugin behavior and protocol extension ownership.
- `internal/plugins/` and selected `internal/component/*` packages - system plugin behavior and runtime backends.
- `test/`, `rfc/short/`, `docs/architecture/`, `plan/learned/` - evidence and expectation sources.

### Architectural Verification
- [ ] No bypassed layers: findings trace the intended path, not a convenient test-only path.
- [ ] No unintended coupling: ownership violations are classified against plugin self-containment.
- [ ] No duplicated functionality: repeated bug shapes are routed to shared checks or one owner.
- [ ] Zero-copy preserved where applicable: review does not recommend allocation-heavy fixes for BGP paths.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|----------------------------------|----------|--------------|--------|
| A-1 | In-tree authored plugin scope is the generated aggregator plus in-repo plugin directories, excluding vendor and generated schema glue unless the glue exposes user-visible surface | `all.go` generated imports and AGENTS architecture inventory | Review misses plugin code compiled through a different path | Inventory child compares `all.go`, directories, registry calls, and command/YANG registries | unvalidated |
| A-2 | Review and fixes should be separate work units | user asked for specs and project review skills are read-only | Review hides code changes and creates unverified partial fixes | Child specs mark production edits out of scope and fix specs required for accepted findings | unvalidated |
| A-3 | Existing active specs may overlap this review, especially plugin migration and MPLS/LDP/RSVP-TE work | current selected specs and modified working tree | Review duplicates or contradicts active implementation work | Inventory child records overlapping specs and routes findings by owner/spec | unvalidated |
| A-4 | Automated checks catch only mechanical classes and cannot replace manual data-flow verification | `ze-review` and `ze-hunt` instructions | False confidence from green gates | Verification child requires source-read proof per finding | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Scope is too large and review becomes shallow | findings lack trigger paths or file:line evidence | split by child specs and reject unverified candidates |
| R-2 | Review reports pre-existing known issues without prioritization | many low-value notes, no fix owner | classify severity, impact, owner, and fix-spec order in child 5 |
| R-3 | Active uncommitted user work changes while review runs | source lines differ between read and verification | re-read before verifying each finding, treat changed files as user work |
| R-4 | Security/concurrency/performance lenses produce duplicates | same defect reported across children | final dedupe keeps highest severity and strongest evidence |
| R-5 | Findings turn into fixes without regression tests | fix spec has no test plan | child 5 blocks fix-spec creation unless regression evidence is named |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| User selects umbrella | -> | child specs execute in dependency order | `spec-bug-review-5` checks every child findings report exists |
| Generated plugin aggregator | -> | inventory table of plugin/schema/RPC packages | `TestBugReviewInventoryCoverage` or equivalent report check in child 1 |
| Child finding candidate | -> | main-session verification path | `BugReviewFindingHasTrigger` report audit in child 5 |
| Accepted finding | -> | follow-up fix spec with regression plan | `BugReviewFixSpecCompleteness` report audit in child 5 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The review starts | Inventory child produces an authoritative table covering generated plugin imports, schema imports, RPC command imports, directory-only candidates, and exclusions with reasons |
| AC-2 | Each area child finishes | A findings report exists with candidates, confirmed findings, plausible findings, rejected candidates, cleared classes, and files actually read |
| AC-3 | A candidate is reported as a bug | Report includes source file:line, reachable trigger, expected vs actual behavior, impact, severity, owner, and regression-test plan |
| AC-4 | A grep or automated check finds a candidate | Candidate is not promoted until surrounding code and relevant callers/data flow are read |
| AC-5 | A child reviews plugin code | It applies wiring, functional coverage, docs, ownership, lifecycle, config reload, error handling, security, concurrency, performance, and rules lenses |
| AC-6 | A child reviews protocol or BGP code | It applies RFC, wire-format, capability-context, zero-copy, pool lifecycle, interop, and edge-case lenses |
| AC-7 | Findings overlap across children | Verification child dedupes them and keeps the strongest evidence and highest severity |
| AC-8 | A finding is accepted for fixing | A separate fix spec is created or updated with failing regression test plan before any code fix starts |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Requests full plugin/core bug review | umbrella -> inventory child -> area children -> verification child -> final report | `BugReviewAllChildrenAccounted` report audit |
| 2 | Reviews a reported bug | finding table -> source evidence -> trigger path -> proposed regression test -> fix spec | `BugReviewFindingHasEvidence` report audit |
| 3 | Selects a finding for implementation later | accepted finding -> separate fix spec -> TDD regression -> implementation | fix spec AC table names the finding ID |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBugReviewInventoryCoverage` | `plan/review-bug-review-inventory.md` or companion script if created | every generated import category is represented in child coverage | |
| `BugReviewFindingHasEvidence` | `plan/review-bug-review-final.md` audit table | every finding has file:line, trigger, expected vs actual behavior, owner, and test plan | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | Review orchestration adds no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `BugReviewAllChildrenAccounted` | `plan/review-bug-review-final.md` | user can see every child spec result and uncovered area count is zero | |
| `BugReviewFixSpecCompleteness` | `plan/review-bug-review-final.md` | accepted issues have fix specs or explicit rejected/deferred status | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Existing BGP interop scenarios named by child findings | `test/interop/scenarios/` | FRR/BIRD/GoBGP/ExaBGP as applicable | protocol findings are proven against peer behavior before fixing | |

### Future (if deferring any tests)
- No deferral inside this review program. If a regression test cannot be written, the finding must explain why and the fix spec must carry that risk explicitly.

## Files to Modify

- No production code files. Review specs are read-only against code.
- `tmp/session/selected-spec` is not changed by this umbrella unless the user selects a child for implementation.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Review only |
| CLI commands/flags | No | Review only |
| Functional test for new RPC/API | No | Review only; fix specs own regression tests |
| Env var registration | No | Review only |
| Doctor check for runtime dependencies | No | Review only |
| Prometheus counters/metrics | No | Review only |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Review plan only |
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
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | Review does not change registration |
| 16 | Any changed source file is referenced by existing doc source anchors? | No | Review-only specs |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Files to Create

- `plan/spec-bug-review-1-inventory-and-self-containment.md` - child spec
- `plan/spec-bug-review-2-plugin-engine-and-system-plugins.md` - child spec
- `plan/spec-bug-review-3-bgp-engine-core.md` - child spec
- `plan/spec-bug-review-4-bgp-plugins-and-protocol-codecs.md` - child spec
- `plan/spec-bug-review-5-verification-and-fix-backlog.md` - child spec
- `plan/review-bug-review-inventory.md` - implementation-time review report, created by child 1
- `plan/review-bug-review-final.md` - implementation-time final report, created by child 5

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file plus all child specs |
| 2. Audit | Required Reading, Current Behavior, child scope tables |
| 3. Wiring phase | Wiring Test table, inventory first |
| 4. Implement (TDD) | Execute review phases, not production code |
| 5. /ze-review gate | Review Gate section checks the review output itself |
| 6. Full verification | Run report audits and any child-specific gates |
| 7. Critical review | Critical Review Checklist below |
| 8-14 | Standard completion, with no production-code claim unless separate fix specs run |

### Implementation Phases

Each phase ends with a self-critical review of the evidence quality.

1. **Phase: Inventory first** - execute `spec-bug-review-1-inventory-and-self-containment.md` and create the inventory report.
   - Tests: `BugReviewInventoryCoverage` report audit.
   - Files: generated import aggregator, plugin directories, registry calls, YANG/RPC registries.
   - Verify: uncovered compiled plugin count is zero, exclusions have reasons.
2. **Phase: Area reviews** - execute child specs 2, 3, and 4. They may run in parallel after inventory is stable.
   - Tests: child findings report audits.
   - Files: child-specific source files.
   - Verify: every child has read-file evidence and candidate triage.
3. **Phase: Final verification and backlog** - execute child 5.
   - Tests: `BugReviewFindingHasEvidence`, `BugReviewFixSpecCompleteness`.
   - Files: findings reports and new fix specs.
   - Verify: every accepted finding is routed to a fix spec; every rejected candidate states the proof.
4. **Functional tests** - run only where a child finding needs reproduction before classification.
5. **Full verification** - run `make ze-spec-status` and child-specific report checks.
6. **Complete spec** - fill audit tables and write learned summary only after the review program completes.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every child spec has output and every output is referenced by final report |
| Feature completeness | Review scope covers plugin aggregator imports, in-tree plugin directories, plugin engine core, BGP engine core, and BGP plugins |
| Correctness | No candidate is reported without source evidence, trigger, expected behavior, actual behavior, and owner |
| Naming | Finding IDs use stable child prefixes: INV, SYS, PENG, BENG, BPLUG, FINAL |
| Data flow | Every finding traces a user or peer entry path, not a test-only path |
| Rule: plugin-self-containment | Ownership findings cite the folder-removal invariant |
| Rule: buffer-first | BGP hot-path findings do not propose allocation-heavy fixes |
| Rule: no-partial-completion | No child marked complete with uncovered scope |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Umbrella and five child specs exist | `make ze-spec-status` lists all six files with valid metadata |
| Inventory report exists | read `plan/review-bug-review-inventory.md` and verify uncovered count |
| Area findings reports exist | read child report references from final report |
| Final report exists | read `plan/review-bug-review-final.md` |
| Accepted findings have fix specs | final report table maps every accepted finding ID to a `plan/spec-*.md` file |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Review input trust | Do not execute untrusted plugin code or external commands as part of review without explicit need |
| Sensitive data | Do not paste secrets from configs, env vars, certificates, or private keys into findings reports |
| Resource use | Avoid whole-tree broad commands when source tools can scope by directory |
| External peer evidence | For interop or wire captures, store minimal reproducer, not large raw private configs |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Inventory cannot derive scope | child 1, update derivation method before area reviews |
| Candidate lacks trigger | reject or downgrade to open question, do not report as bug |
| Finding requires code change | create separate fix spec, do not edit code under review spec |
| Child discovers out-of-scope subsystem | record as explicit exclusion or add a child spec with user approval |
| Review report contradicts source | re-read source, update report, record mistake |

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

- Review by generated registration boundary is safer than review by directory name. Some user-visible surfaces are schema-only or RPC-only packages.
- Fixes are deliberately excluded from review execution to keep evidence clean and avoid untested partial patches.

## Core Insight

A plugin/core review is only systematic if it starts from reachability and ownership. A bug that cannot be reached by a user or peer is dead code; a plugin feature that survives folder removal is owned in the wrong place.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Split review into inventory, system/plugin engine, BGP engine, BGP plugins, final verification | One huge review spec | Smaller scopes make evidence auditable and allow parallel review after inventory |
| Keep review read-only | Fix while reviewing | Fixes need TDD regression specs; mixing review and fixes hides incomplete work |
| Use generated aggregator as an inventory anchor | Manual list from docs | Generated imports are what actually compile into the binary |
| Require fix specs for accepted findings | Add a loose task list | Specs enforce ACs, test plans, and ownership |

## Known Limitations

- This spec set does not fix defects. It creates a verified backlog and fix specs.
- The first pass reviews current source state. Active uncommitted work may shift line numbers and must be re-read during implementation.
- External third-party plugins are out of scope unless they are vendored or registered by this repository.

## RFC Documentation

No RFC comments are added by the review program. Protocol fix specs must add or verify RFC comments when they implement or change RFC behavior.

## Implementation Summary

### What Was Implemented
- Executed the full bug-review program: inventory, child area reviews, final dedupe, final report, and accepted-finding fix specs.
- Created `plan/review-bug-review-inventory.md`, `plan/review-bug-review-plugin-engine-system.md`, `plan/review-bug-review-bgp-engine.md`, `plan/review-bug-review-bgp-plugins.md`, and `plan/review-bug-review-final.md`.
- Created eight follow-up bugfix specs for accepted findings: plugin lifecycle rollback, DirectBridge panic, BGP message validation before delivery, BGP forward split context, BGP reactor startup cleanup, BGP next-hop allocation, BGP NLRI strictness, and SR-Policy encode wiring.
- Kept review execution read-only against production code.

### Bugs Found/Fixed
- Found 10 accepted production bug findings and converted each to a fix spec.
- Did not fix production code under this review spec by design. Fixes belong to the generated bugfix specs.
- Recorded 4 plausible findings not promoted and 2 inventory observations.

### Documentation Updates
- Created review and fix-spec artifacts only.
- No user-facing documentation, config, CLI, API, wire format, or registration behavior changed.

### Deviations from Plan
- Accepted BENG-005 into a fix spec as an allocation-confirming performance issue because the source path is concrete and the spec gates implementation on allocation proof.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Execute inventory first | done | `plan/review-bug-review-inventory.md` | generated import and registry scope reconciled |
| Execute child reviews | done | three child report files | plugin/system, BGP engine, BGP plugins |
| Deduplicate accepted findings | done | `plan/review-bug-review-final.md` | one accepted-finding table and rejected/plausible tables |
| Create fix specs | done | eight `plan/spec-bugfix-*.md` files | every accepted finding mapped to a fix spec |
| Keep production code unchanged | done | artifact set | review-only deliverables |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | inventory report | generated imports, directories, registries, and exclusions covered |
| AC-2 | done | child reports | each area report has candidates, confirmed findings, plausible findings, rejected candidates, and cleared classes |
| AC-3 | done | child report finding sections | accepted findings include file/line evidence, trigger, expected/actual, impact, severity, owner, and test plan |
| AC-4 | done | final report rejected/plausible tables | grep/search candidates were not promoted without surrounding proof |
| AC-5 | done | child 2 report | lifecycle, config reload, DirectBridge, command wiring, security, and ownership lenses applied |
| AC-6 | done | child 3 and 4 reports | RFC, wire, capability context, zero-copy, pool lifecycle, interop, and edge cases applied |
| AC-7 | done | final report | dedupe table maps findings to root-cause fix specs |
| AC-8 | done | fix spec ledger | every accepted finding has a follow-up fix spec with tests |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestBugReviewInventoryCoverage` | pass | `plan/review-bug-review-inventory.md` audit tests | manual report audit |
| `BugReviewFindingHasEvidence` | pass | child reports and final report | every accepted finding has evidence and trigger |
| `BugReviewAllChildrenAccounted` | pass | `plan/review-bug-review-final.md` | all child reports listed |
| `BugReviewFixSpecCompleteness` | pass | final report fix spec ledger | all accepted findings map to specs |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/spec-bug-review-1-inventory-and-self-containment.md` | done | child spec updated |
| `plan/spec-bug-review-2-plugin-engine-and-system-plugins.md` | done | child spec updated |
| `plan/spec-bug-review-3-bgp-engine-core.md` | done | child spec updated |
| `plan/spec-bug-review-4-bgp-plugins-and-protocol-codecs.md` | done | child spec updated |
| `plan/spec-bug-review-5-verification-and-fix-backlog.md` | done | child spec updated |
| `plan/review-bug-review-inventory.md` | created | inventory report |
| `plan/review-bug-review-final.md` | created | final report |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** review specs, reports, and bugfix specs only

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Systematic bug review of authored plugins and core engine | Review reports plus fix specs | `plan/review-bug-review-final.md` names every child report and accepted finding route |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | SYS-001 accepted | child 2 report | routed to `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` |
| 2 | BLOCKER | BENG-001 accepted | child 3 report | routed to `plan/spec-bugfix-bgp-message-validation-before-delivery.md` |
| 3 | BLOCKER | BENG-002 accepted | child 3 report | routed to `plan/spec-bugfix-bgp-forward-split-context.md` |
| 4 | ISSUE | remaining accepted SYS, BENG, and BPLUG findings | final report | routed to fix specs |

### Fixes applied
- None during review spec execution. Fixes are separate follow-up specs.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | ArtifactReview found closure specs and early fix-spec audits still open | `plan/review-bug-review-final.md` | closure sections and fix-spec audits completed before final handoff |

### Final status
- [x] Critical review of review artifacts records accepted findings and fix-spec routing
- [x] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/review-bug-review-inventory.md` | yes | inventory report read and listed in final report |
| `plan/review-bug-review-plugin-engine-system.md` | yes | child 2 report read and listed in final report |
| `plan/review-bug-review-bgp-engine.md` | yes | child 3 report read and listed in final report |
| `plan/review-bug-review-bgp-plugins.md` | yes | child 4 report read and listed in final report |
| `plan/review-bug-review-final.md` | yes | final report created |
| `plan/spec-bugfix-*.md` accepted-finding specs | yes | final report fix spec ledger names all eight |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | inventory complete | `plan/review-bug-review-inventory.md` reports in-scope unassigned count 0 |
| AC-2 | child reports complete | `plan/review-bug-review-final.md` source artifacts table |
| AC-3 | findings have evidence | accepted findings table and child report finding sections |
| AC-4 | unverified candidates not promoted | plausible and rejected candidate tables |
| AC-5 | plugin/system lenses applied | child 2 wiring and coverage tables |
| AC-6 | BGP/protocol lenses applied | child 3 and 4 matrices |
| AC-7 | findings deduped | final report accepted findings table |
| AC-8 | fix specs created | final report fix spec ledger |

### Wiring Verified (end-to-end)
| Entry Point | Report Audit | Verified |
|-------------|--------------|----------|
| generated plugin aggregator | `BugReviewInventoryAllImportsAccounted` | yes |
| child review outputs | `FinalReviewAllChildReportsLoaded` | yes |
| accepted finding route | `FinalReviewAcceptedFindingsHaveFixSpecs` | yes |
| rejected candidates | `FinalReviewRejectedCandidatesHaveProof` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | inventory report uses generated imports plus directory and registry reconciliation |
| A-2 | confirmed | production fixes were not made, separate fix specs created |
| A-3 | confirmed | final report records active spec overlap routing |
| A-4 | confirmed | child reports cite source-read proof and final report rejects unsupported candidates |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | review-only artifacts, no behavior change | yes |
| Protocol comments required later | fix specs require RFC comments where protocol enforcement changes | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N all demonstrated
- [x] End-to-End User Stories: every story has a working path and a passing report audit
- [x] Wiring Test table complete, every row has a concrete verification name
- [x] Critical review gate clean for review artifacts
- [x] `make ze-spec-status` passes
- [x] Integration completeness proven by final report
- [x] Documentation Update Checklist answered Yes/No with source evidence
- [x] Critical Review passes
- [x] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass, defer with user approval)
- [x] RFC constraint comments verified for protocol fix specs
- [x] Implementation Audit complete
- [x] Mistake Log escalation reviewed

### Design
- [x] No premature abstraction
- [x] No speculative features
- [x] Single responsibility per child spec
- [x] Explicit behavior
- [x] Minimal coupling

### TDD
- [x] Report audits written or explicitly replaced by manual evidence
- [x] Regression tests named for every accepted bug
- [x] Functional tests named where user-visible behavior is affected
- [x] Interop tests named for protocol findings
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are not applicable
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/938-bug-review-0-umbrella.md`
- [x] **Commit A script prepared:** specs + reports + learned summary + counter bump in `tmp/commit-f32fa560.sh`
- [x] **Commit B script prepared:** remove `plan/spec-bug-review-0-umbrella.md` and child specs after learned summaries preserve final state
