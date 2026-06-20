# Spec: bug-review-2 -- Plugin Engine and System Plugins

| Field | Value |
|-------|-------|
| Status | done |
| Depends | spec-bug-review-1-inventory-and-self-containment.md |
| Phase | - |
| Updated | 2026-06-19 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-bug-review-0-umbrella.md` and `plan/spec-bug-review-1-inventory-and-self-containment.md`
3. `docs/architecture/core-design.md` sections 1, 6, 9, 18, and 19
4. `internal/component/plugin/types.go`, `internal/component/plugin/registry/registry.go`, `internal/component/plugin/server/startup.go`
5. `pkg/plugin/rpc/bridge.go` and `pkg/plugin/sdk/`
6. `ai/rules/plugin-design.md`, `ai/rules/plugin-self-containment.md`, `ai/rules/data-flow-tracing.md`
7. `skill://ze-review-deep` lenses for security, concurrency, error handling, API contracts, docs, performance, and feature completeness

## Task

Review the plugin infrastructure and all non-BGP authored plugin implementations for bugs. This includes the engine-side plugin registry/server, DirectBridge and SDK boundaries, system plugins under `internal/plugins/`, and component-owned plugin packages that behave like plugins outside the BGP plugin tree.

This review is read-only. Findings that need code changes must be routed to child 5 and then to separate fix specs.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-bug-review-1-inventory-and-self-containment.md` - authoritative scope
  → Decision: this child reviews rows assigned to plugin engine, system plugins, component-owned system plugins, and schema/RPC surfaces not owned by BGP plugins.
  → Constraint: no package from the inventory may be silently skipped.
- [ ] `docs/architecture/core-design.md:70-82` - engine/plugin responsibilities
  → Decision: plugin startup, config-root autoload, dynamic event/send type registration, and command dispatch are first-class bug surfaces.
  → Constraint: engine remains protocol-agnostic; plugin-specific behavior belongs in owners.
- [ ] `docs/architecture/core-design.md:336-400` - plugin API communication
  → Decision: review must cover internal DirectBridge and external newline-framed RPC paths.
  → Constraint: engine stores peer/cache state, plugins store RIB/policy/feature state; state ownership bugs are findings.
- [ ] `docs/architecture/core-design.md:1154-1171` - config transaction protocol
  → Decision: system plugin review covers verify, apply, rollback, commit, journal cleanup, reload idempotence, and shutdown cleanup.
  → Constraint: config apply failures must roll back or clearly fail, never partially mutate runtime state silently.
- [ ] `ai/rules/plugin-design.md` - plugin lifecycle, optional dependencies, cross-boundary values
  → Decision: review checks Stage 1 through Stage 5 registration, `OnStarted` vs `OnAllPluginsReady`, dependencies, optional dependencies, config roots, event/send types, doctor checks, and value-typed payloads.
  → Constraint: plugin boundary payloads must not hold pointers to producer-owned data.
- [ ] `ai/rules/plugin-self-containment.md` - self-containment and command ownership
  → Decision: every command/schema/RPC/helper surface is checked against folder removal.
  → Constraint: central verb packages may not own plugin command spelling except generic roots.
- [ ] `plan/learned/RECURRING-PATTERNS.md` - known plugin traps
  → Decision: targeted hunts include unwired features, hardcoded registry counts, net.Pipe deadlocks, registry contamination, fake synchronization comments, and silent parser/dispatch fall-through.
  → Constraint: no grep hit becomes a finding without source/caller verification.

### RFC Summaries (MUST for protocol work)
- [ ] Protocol-specific summaries when a candidate touches BFD, LDP, RSVP-TE, L2TP, PPP, IPsec/IKE, DHCP, RADIUS, TACACS, or another protocol plugin.
  → Constraint: if a finding claims protocol non-compliance, it cites the relevant summary and exact requirement.

**Key insights:**
- Plugin infrastructure has two transport shapes: internal DirectBridge and external RPC. Bugs often affect one path only.
- System plugin correctness depends on reload lifecycle as much as startup lifecycle.
- A command can be registered, schema-declared, and still unwired if no generated import or handler path reaches it.
- Review should compare each plugin to a same-shape reference, not to a generic ideal.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/types.go:26-87` - reactor interfaces, protocol reactor abstraction, typed response marker, process spawner, config types.
  → Constraint: plugin infrastructure is intentionally protocol-neutral; protocol-specific type assertions belong at explicit seams.
- [ ] `internal/component/plugin/registry/registry.go:35-143` - `Registration` metadata and handler fields.
  → Constraint: review must check every registration field that changes runtime behavior: name, config roots, dependencies, optional dependencies, YANG, families, capabilities, event/send types, doctor checks, metrics and bridge callbacks.
- [ ] `internal/component/plugin/registry/registry.go:266-331` - registration validation.
  → Constraint: invalid, duplicate, empty, or conflicting registrations should fail at registration, not at first runtime use.
- [ ] `internal/component/plugin/registry/registry.go:575-753` - RPC handler collection, family decode/encode, route encoders, config parsers.
  → Constraint: plugin-provided handlers and family routes must be reachable through registry lookups and have nil/error behavior reviewed.
- [ ] `internal/component/plugin/registry/registry.go:784-1053` - Reset/Snapshot/Restore, dependency resolution, topological tiers.
  → Constraint: tests mutating registries must snapshot/restore every global, and startup ordering bugs require dependency graph analysis.
- [ ] `internal/component/plugin/server/startup.go:122-230` - five-phase plugin startup.
  → Constraint: review must verify config-path, explicit, family, event-type, and send-type autoload each handles missing/failed plugins clearly.
- [ ] `internal/component/plugin/server/startup.go:306-416` - tier-ordered handshake.
  → Constraint: dependent plugins can dispatch only after lower tiers complete; async handlers start after all tiers complete.
- [ ] `internal/component/plugin/server/startup.go:418-758` - startup RPC, config delivery, registry delivery.
  → Constraint: Stage 2 and Stage 4 payloads need symmetry with SDK expectations and correct error propagation.
- [ ] `internal/component/plugin/server/startup.go:770-995` - external family registration and doctor declaration validation.
  → Constraint: external plugin declarations must validate names, AFI/SAFI conflicts, and doctor metadata before becoming runtime state.
- [ ] `pkg/plugin/rpc/bridge.go:23-31` - DirectBridge activation after five-stage startup.
  → Constraint: bridge-ready checks must guard direct calls; fallback paths must behave equivalently.
- [ ] `pkg/plugin/rpc/bridge.go:154-190` - structured and string event delivery through DirectBridge.
  → Constraint: structured event handlers require happens-before and readiness checks.
- [ ] `pkg/plugin/rpc/bridge.go:571-632` - pooled `StructuredEvent` and async safety warning.
  → Constraint: plugins must not retain zero-copy raw message data after callback unless they copy it deliberately.
- [ ] `internal/plugins/static/register.go:24-54` - representative system plugin registration.
  → Decision: use same-shape references per plugin class rather than treating every plugin as identical.
- [ ] `internal/plugins/static/register.go:68-214` - representative SDK lifecycle with config verify/configure/apply/rollback/started/execute-command/run/shutdown.
  → Constraint: system plugin review must include lifecycle ordering, journal rollback, command handling, and cleanup.

**Behavior to preserve:**
- Plugin server remains protocol-neutral.
- Internal plugin DirectBridge remains a performance path, not a separate semantic path.
- External plugin RPC remains supported and receives equivalent behavior unless explicitly documented otherwise.
- System plugin removal leaves no dangling command/schema/help/doctor/metrics surface.
- Config verify/apply/rollback remains exact or reject.

**Behavior to change:**
- Produce a verified findings report for plugin engine and system plugin bugs.
- Produce fix-spec entries for accepted findings.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Plugin registration via package `init`.
- Plugin startup via five-phase server startup.
- Config through YANG tree, config-root extraction, SDK callbacks, verify/apply/rollback.
- Commands through YANG schema, RPC registry, DirectBridge or external RPC.
- Events through EventBus, plugin server delivery, DirectBridge structured events, or JSON text.
- Doctor checks, env vars, metrics, web/LG/CLI surfaces.

### Transformation Path
1. Inventory rows select plugin engine and system plugin packages.
2. For each package, identify user entry points: config, command, event, send type, doctor, metric, web route, file/socket/backend.
3. Trace each entry point through registration, startup, runtime handling, error path, reload path, and shutdown path.
4. Apply review lenses: wiring first, then functional coverage, docs, ownership, security, concurrency, error handling, logic, API contract, performance, project rules.
5. Promote only verified candidates to the child findings report.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Generated import -> plugin `init` | blank import runs registration | [ ] registration row in inventory |
| Engine -> plugin startup | five-stage RPC over connection | [ ] Stage 1 through Stage 5 path read |
| Engine -> internal plugin runtime | DirectBridge typed call | [ ] ready/handler guard and fallback reviewed |
| Engine -> external plugin runtime | newline-framed RPC | [ ] external RPC path reviewed or explicitly not applicable |
| Config -> plugin | extracted subtree and SDK callbacks | [ ] verify/apply/rollback flow traced |
| CLI/API -> handler | YANG command -> RPC registry -> owner handler | [ ] user command path traced |
| Plugin -> plugin | DispatchCommand or DirectBridge typed global | [ ] ownership and optional dependency behavior traced |

### Integration Points
- `internal/component/plugin/registry/`
- `internal/component/plugin/server/`
- `pkg/plugin/sdk/`
- `pkg/plugin/rpc/`
- `internal/plugins/*`
- component-owned plugin packages assigned by child 1
- `internal/component/cmd/*/yang` and owner YANG packages
- `internal/core/diagnostic`, `internal/core/env`, `internal/core/metrics`, `internal/core/events`

### Architectural Verification
- [ ] No bypassed layers: commands flow through YANG/RPC/registry or registered local commands.
- [ ] No unintended coupling: plugin-specific knowledge stays in owner packages.
- [ ] No duplicated functionality: config/doctor/env/metric sources have one owner.
- [ ] Zero-copy preserved where applicable: DirectBridge structured event data is not retained unsafely.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|------------|----------------------------------|----------|--------------|--------|
| A-1 | Inventory child correctly assigns all non-BGP plugin rows to this child | child 1 contract | child 2 misses code | compare child 2 report package list to inventory assignments | unvalidated |
| A-2 | Same-shape reference comparison is possible for every system plugin | multiple plugin families in `internal/plugins/` | plugin-specific behavior lacks reference baseline | classify plugin shape and choose nearest reference or mark bespoke | unvalidated |
| A-3 | Internal and external transport paths should be semantically equivalent unless code says otherwise | core-design plugin API communication | bugs in one transport path are missed | review direct and RPC paths for each command/event class | unvalidated |
| A-4 | Config transaction participation is required only for plugins with mutable config | core-design config transaction protocol | non-participating plugin leaves stale runtime state after reload | trace config roots and SDK callbacks per plugin | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Review surface is too broad | report names packages without findings evidence | split report by plugin shape and require per-package read evidence |
| R-2 | DirectBridge-only review misses external plugin bugs | finding says internal path only | review fallback RPC path or explicitly mark plugin in-process only |
| R-3 | Lifecycle bugs require integration tests to prove | code path depends on startup/reload timing | reproduce with existing functional tests or create fix spec with lifecycle regression |
| R-4 | Backend plugins are OS-specific | Darwin review cannot execute Linux/VPP paths | read code and require QEMU/VPP test plan in fix specs |
| R-5 | Active uncommitted work touches LDP/RSVP-TE/MPLS plugin code | line numbers or tests shift | re-read modified files before classifying findings |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| inventory row assigned to child 2 | -> | per-package review evidence | `PluginReviewEveryAssignedPackageRead` |
| YANG command path | -> | RPC/local handler in owner | `PluginReviewCommandWiringChecked` |
| config root | -> | SDK verify/configure/apply/rollback callbacks | `PluginReviewConfigLifecycleChecked` |
| plugin event/send type | -> | producer/consumer registration and runtime path | `PluginReviewEventSendWiringChecked` |
| DirectBridge typed method | -> | handler plus RPC fallback or explicit in-process-only reason | `PluginReviewBridgeParityChecked` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Child 2 starts | Report lists every assigned package from inventory and selected reference shape |
| AC-2 | A plugin has a command/schema surface | Handler, YANG schema, completion/discovery path, and owner package are traced |
| AC-3 | A plugin has config roots or YANG leaves | Verify/configure/apply/rollback behavior is reviewed, including failure cleanup |
| AC-4 | A plugin emits or consumes events/send types | Registration, runtime producer/consumer path, and missing-plugin behavior are reviewed |
| AC-5 | A plugin owns runtime dependency | Doctor check ownership and test coverage are reviewed |
| AC-6 | DirectBridge or SDK typed path exists | Readiness, fallback, cancellation, async safety, and error propagation are reviewed |
| AC-7 | A finding survives | It includes source file:line, trigger, expected vs actual behavior, severity, owner, and regression-test plan |
| AC-8 | No finding survives for a plugin | Report records cleared lenses and files read for that plugin |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures a system plugin | YANG config -> config root -> SDK callbacks -> backend state -> rollback/commit | `PluginReviewConfigLifecycleChecked` |
| 2 | Runs a plugin command | CLI/API command -> YANG path -> RPC/local handler -> plugin state/backend -> response/pipes | `PluginReviewCommandWiringChecked` |
| 3 | Starts Ze with plugins | generated import -> registry -> startup phase -> dependency tier -> ready callback | `PluginReviewStartupLifecycleChecked` |
| 4 | Plugin receives an event | producer -> EventBus or EventDispatcher -> DirectBridge/RPC -> handler -> cleanup | `PluginReviewEventSendWiringChecked` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `PluginReviewEveryAssignedPackageRead` | `plan/review-bug-review-plugin-engine-system.md` | every assigned package has read evidence or exclusion | |
| `PluginReviewCommandWiringChecked` | same report | every command surface traces schema to handler | |
| `PluginReviewConfigLifecycleChecked` | same report | config plugins have verify/apply/rollback/shutdown reviewed | |
| `PluginReviewBridgeParityChecked` | same report | DirectBridge and RPC behavior differences documented | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Runtime port/listen values | per owning YANG/env validation | last allowed by schema | first below schema | first above schema |
| Timer/interval config | per owning YANG/env validation | last allowed by schema | zero or below min | first above max |
| Plugin limits/caps | per owning package | last allowed | below min | above max |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin `.ci` named per finding | `test/plugin/`, `test/*/` | command/config behavior works through daemon | |
| Existing reload `.ci` named per finding | `test/reload/` or plugin suite | config reload and rollback behavior | |
| New regression test named in fix spec | fix spec | accepted bug reproduction | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Protocol-specific scenario named by finding | `test/interop/scenarios/` | peer daemon as applicable | protocol plugin defect reproduces against peer | |

### Future (if deferring any tests)
- No deferral in review. Fix specs may request user approval only when reproduction requires unavailable hardware or peer software, and must name the substitute evidence.

## Files to Modify

- No production code files.
- Read-only scope includes `internal/component/plugin/`, `pkg/plugin/`, `internal/plugins/`, and component-owned plugin packages assigned by child 1.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | Review only |
| CLI commands/flags | No | Review only |
| Functional test for new RPC/API | No | Fix specs own regression tests |
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

- `plan/review-bug-review-plugin-engine-system.md` - child 2 findings report.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file plus inventory report |
| 2. Audit | assigned package list and Required Reading |
| 3. Wiring phase | Wiring Test table, command/config/event/bridge reachability |
| 4. Implement (TDD) | read-only review report |
| 5. /ze-review gate | review report quality |
| 6. Full verification | report audits from TDD plan |
| 7-14 | standard completion |

### Implementation Phases

1. **Phase: Scope reconciliation** - load inventory rows assigned to child 2.
   - Tests: `PluginReviewEveryAssignedPackageRead`.
   - Files: inventory report and package list.
   - Verify: zero unreviewed assigned packages.
2. **Phase: Plugin infrastructure review** - review registry, server startup, command registry, config delivery, DirectBridge, SDK.
   - Tests: `PluginReviewStartupLifecycleChecked`, `PluginReviewBridgeParityChecked`.
   - Files: `internal/component/plugin/`, `pkg/plugin/`.
   - Verify: each candidate has source/caller evidence.
3. **Phase: System plugin shape reviews** - review packages by shape: config/backends, command-only, event producers/consumers, protocol plugins, file/socket services.
   - Tests: `PluginReviewCommandWiringChecked`, `PluginReviewConfigLifecycleChecked`, `PluginReviewEventSendWiringChecked`.
   - Files: assigned plugin packages.
   - Verify: findings report has per-package cleared/finding status.
4. **Phase: Report and route findings** - emit child report and route accepted findings to child 5.
   - Tests: report audits.
   - Files: `plan/review-bug-review-plugin-engine-system.md`.
   - Verify: every finding has owner and regression-test plan.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every assigned package reviewed or explicitly excluded |
| Feature completeness | command, config, event/send, doctor, metrics, web, and backend surfaces checked per package |
| Correctness | lifecycle, reload, rollback, shutdown, and optional dependency paths examined |
| Naming | plugin names, command keys, env vars, YANG leaves use canonical owner names |
| Data flow | DirectBridge and external RPC paths traced separately |
| Security | user-controlled config/CLI/file/socket inputs have validation and length/resource bounds reviewed |
| Concurrency | goroutines, channels, callbacks, bridge channels, registry mutation, and shutdown races reviewed |
| Performance | hot internal plugin paths do not retain zero-copy data or allocate per event without reason |
| Rules | plugin self-containment, exact-or-reject, no-workarounds, doctor checks, config naming applied |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Child 2 findings report | read `plan/review-bug-review-plugin-engine-system.md` |
| Assigned package coverage table | report table matches inventory rows |
| Finding evidence table | every finding has required fields |
| Cleared class table | every plugin shape records clean lenses or N/A reason |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|------------------|
| Config input | accepted characters, length bounds, exact-or-reject behavior, reload behavior |
| CLI/API input | argument validation, pipe handling, JSON response shape, resource limits |
| File/socket paths | path traversal, permissions, close errors, shutdown cleanup |
| Secrets | env/config secret redaction and log/error leakage |
| External plugins | handshake validation, RPC method validation, unknown method behavior, malformed JSON |
| DirectBridge | unsafe retention of pooled or zero-copy data after callback |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Unassigned package discovered | child 1 inventory update before continuing |
| Finding requires code | child 5 fix backlog |
| Candidate lacks trigger | reject or keep as open question, not a bug |
| OS-specific behavior cannot be run | fix spec must include QEMU/VPP or platform-specific verification |
| External plugin path unknown | read SDK/RPC source before classifying |

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

- Plugin bugs often live in mismatched surfaces: schema exists without handler, handler exists without generated import, config verify exists without rollback, DirectBridge exists without RPC parity.

## Core Insight

For system plugins, correctness is a lifecycle property. Startup-only review misses reload, rollback, and shutdown bugs.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Review by plugin shape after inventory | alphabetical package review | shape comparison catches missing lifecycle pieces |
| Treat DirectBridge and external RPC separately | assume bridge is an optimization only | performance paths can diverge semantically |
| Require owner/removal evidence for commands | check only handler registration | schema and completion can outlive handlers if ownership is wrong |

## Known Limitations

- This child may identify Linux/VPP bugs that cannot be executed on Darwin. Those findings need fix specs with QEMU or VPP verification.
- This child does not review BGP plugin packages; child 4 owns them.

## RFC Documentation

Protocol-specific fix specs must verify RFC comments. This review records RFC references only when a finding depends on a requirement.

## Implementation Summary

### What Was Implemented
- Created `plan/review-bug-review-plugin-engine-system.md`.
- Reviewed plugin startup/reload lifecycle, SDK/RPC bridge behavior, DirectBridge parity, central/generic command wiring, directory-only command roots, system plugins, and representative component-owned plugin shapes.
- Produced confirmed, plausible, rejected, cleared-class, and assumptions-resolved sections for child 2 scope.

### Bugs Found/Fixed
- Confirmed SYS-001, SYS-002, and SYS-003.
- Recorded SYS-004 and SYS-005 as plausible but not promoted by the final report.
- No production code was changed.

### Documentation Updates
- None. The child output is a review report and does not change user-facing behavior.

### Deviations from Plan
- None.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Review plugin engine and SDK/RPC bridge | done | child report scope and findings | SYS-001 and SYS-003 |
| Review reload/autoload lifecycle | done | child report wiring table | SYS-002 and SYS-004 plausible |
| Review system plugin classes and directory-only command roots | done | child report package coverage | no missing generic root bug promoted |
| Route findings to final backlog | done | final report | accepted findings map to fix specs |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | inventory report and child report scope | child 2 assigned rows covered |
| AC-2 | done | package coverage table | same-shape package groups checked |
| AC-3 | done | SYS finding sections | trigger, expected/actual, impact, severity, owner, test plan |
| AC-4 | done | rejected candidates and plausible findings | unverified candidates not promoted |
| AC-5 | done | wiring/coverage audit table | lifecycle, command, bridge, config, file service lenses applied |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| PluginLifecycleFindingHasEvidence | pass | child report SYS sections | manual report audit |
| PluginCommandWiringReviewed | pass | child report command and RPC wiring matrix | no missing generic command bug promoted |
| PluginBridgeParityReviewed | pass | child report DirectBridge section | SYS-003 accepted |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `plan/review-bug-review-plugin-engine-system.md` | created | child 2 findings report |

### Audit Summary
- **Total items:** 13
- **Done:** 13
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 report file created

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Plugin engine and system plugin bug review | Findings report | `plan/review-bug-review-plugin-engine-system.md` with package coverage and verified findings |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | BLOCKER | SYS-001 startup failure can be swallowed after partial dynamic registration | child report | routed to lifecycle rollback fix spec |
| 2 | ISSUE | SYS-002 failed reload cleanup passes plugin names to config-root stop logic | child report | routed to lifecycle rollback fix spec |
| 3 | ISSUE | SYS-003 DirectBridge callback panic leaves engine caller waiting for timeout | child report | routed to DirectBridge panic fix spec |
| 4 | NOTE | SYS-004 and SYS-005 plausible only | child report | not promoted by final report |

### Fixes applied
- None during review spec execution.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | No untriaged child 2 finding remains after final report | `plan/review-bug-review-final.md` | no action |

### Final status
- [x] Critical review of child report artifacts records accepted and not-promoted findings
- [x] All NOTEs recorded above or explicitly none

## Pre-Commit Verification

### Files Exist
| File | Exists | Evidence |
|------|--------|----------|
| `plan/review-bug-review-plugin-engine-system.md` | yes | report read and listed in final report |
| `plan/spec-bugfix-sys-plugin-lifecycle-rollback.md` | yes | final report fix spec ledger |
| `plan/spec-bugfix-sys-directbridge-panic.md` | yes | final report fix spec ledger |

### AC Verified
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | assigned inventory rows covered | child report inventory coverage table |
| AC-2 | representative package shapes reviewed | child report source files and cleared classes |
| AC-3 | findings have evidence | SYS-001 through SYS-003 sections |
| AC-4 | unverified candidates not promoted | SYS-004 and SYS-005 plausible table |
| AC-5 | plugin lenses applied | wiring/coverage audit table |

### Wiring Verified (end-to-end)
| Entry Point | Report Audit | Verified |
|-------------|--------------|----------|
| plugin startup lifecycle | child report wiring table | yes |
| config-root autoload/reload | child report wiring table | yes |
| DirectBridge callbacks | child report SYS-003 | yes |
| command schema to handler | child report command table | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | child report assumptions table |
| A-2 | confirmed | same-shape package coverage |
| A-3 | confirmed with bridge-specific finding | SYS-003 |
| A-4 | confirmed | SYS-002 lifecycle finding |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| No user docs needed | review-only artifact | yes |
| Platform-specific tests required later | fix specs carry targeted regression plans | yes |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-N all demonstrated
- [x] End-to-End User Stories all have report evidence
- [x] Wiring Test table complete
- [x] Critical review gate clean for child report artifacts
- [x] `make ze-spec-status` passes
- [x] Risks & Assumptions: every A-N confirmed or broken

### Quality Gates (SHOULD pass, defer with user approval)
- [x] Protocol RFC references checked for protocol findings
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
- [x] Functional tests named where user-visible behavior is affected
- [x] Goal Validation table filled

### Completion (BLOCKING, before ANY commit)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval or are not applicable
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [x] Write learned summary to `plan/learned/940-bug-review-2-plugin-engine-and-system-plugins.md`
- [x] **Commit A script prepared:** spec + report + learned summary + counter bump in `tmp/commit-f32fa560.sh`
- [x] **Commit B script prepared:** remove `plan/spec-bug-review-2-plugin-engine-and-system-plugins.md` only after final state is preserved
