# Spec: redist-explicit-dest

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-05-13 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/config/redistribute/schema/ze-redistribute-conf.yang` - YANG schema
4. `internal/component/config/loader_redistribute.go` - config extraction
5. `internal/component/bgp/config/loader_create.go` - initRedistribute caller

## Task

Add a destination-protocol nesting level to the `redistribute` config block
and introduce a `RedistConsumer` interface so protocols register as
redistribution destinations rather than each writing a bespoke consumer plugin.

Config changes from `redistribute { import l2tp { ... } }` to
`redistribute { bgp { import l2tp { ... } } }`.

The top-level `redistribute` container remains, preserving web UI navigation
(`/show/redistribute/`), ConfigRoots auto-start, and forward compatibility
with the VRF spec's cross-VRF redistribution (which also lives at top level).

A shared redistribution orchestrator replaces the current
`bgp-redistribute-egress` plugin. It subscribes to all producers, evaluates
per-destination rules, and dispatches to the registered consumer. Adding a
new destination protocol (OSPF, ISIS) becomes: implement `RedistConsumer`,
register it, add a YANG destination key.

### Consumer Interface

Defined in `internal/component/config/redistribute/consumer.go`, mirroring
the existing `RouteSource` / `RegisterSource` pattern in `registry.go`:

- `RedistConsumer` interface: `Name() string`, `InjectRoute(ctx, family, entry)`,
  `WithdrawRoute(ctx, family, prefix)`
- `RegisterConsumer(consumer)`, `LookupConsumer(name)`, `ConsumerNames()`

### Orchestrator

A single `redistribute-orchestrator` plugin replaces `bgp-redistribute-egress`:
- Subscribes to all non-self producers via EventBus (same subscribe loop as today)
- For each batch, iterates registered consumers
- Per consumer: evaluates that consumer's import rules via the evaluator
- Dispatches accepted entries to the matching `RedistConsumer`

BGP registers its consumer at startup (wrapping the existing `UpdateRoute`
dispatch logic from `bgp-redistribute-egress`). Future protocols do the same.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - redistribution evaluator architecture
  → Decision: global singleton evaluator, set by BGP config loader at startup
  → Constraint: evaluator is thread-safe (RWMutex), rules swapped atomically on reload
- [ ] `ai/patterns/config-option.md` - YANG leaf/container/list patterns
  → Constraint: YANG groupings reusable across modules via `uses`
- [ ] `ai/patterns/registration.md` - plugin ConfigRoots auto-start
  → Constraint: ConfigRoots must match ConfiguredPaths entries for auto-load

### RFC Summaries (MUST for protocol work)
- N/A (config restructuring, no protocol wire changes)

**Key insights:**
- Evaluator is a global singleton; no change to its API needed
- ConfigRoots `"redistribute"` stays unchanged (container remains at top level)
- The ingress filter calls Accept(route, "") with empty importingProtocol; this works unchanged
- Cross-module YANG `uses` with prefix is already used in 9+ modules (e.g. `uses zt:listener`)
- `ExtractRedistributeRules` is called only from `bgp/config/loader_create.go:41`
- `SetGlobal` is called only from `bgp/config/loader_create.go:47`
- Web UI workbench has `/show/redistribute/` nav entry and routing guard; stays unchanged
- VRF spec envisions cross-VRF redistribution at top level too; compatible

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/redistribute/schema/ze-redistribute-conf.yang` - top-level `redistribute` container with `list import { key source; leaf-list family }`
  → Constraint: `ze:hidden true` on the container (not yet fully visible)
  → Constraint: `ze:validate "redistribute-source"` and `ze:validate "registered-address-family"` on leaves
- [ ] `internal/component/config/loader_redistribute.go` - `ExtractRedistributeRules(tree)` reads `tree.GetContainer("redistribute").GetListOrdered("import")`
  → Constraint: returns nil (no error) when container absent or empty
  → Constraint: exact-or-reject: unknown source or family returns error
- [ ] `internal/component/config/redistribute/evaluator.go` - global singleton, `Accept(route, importingProtocol)` checks rules
  → Constraint: `SetGlobal()` / `Global()` atomic pointer swap; thread-safe
- [ ] `internal/component/config/redistribute/route.go` - `RedistRoute{Origin, Family, Source}`, `ImportRule{Source, Families}`, `Evaluate()`, loop prevention
  → Constraint: loop prevention: `route.Origin == importingProtocol` rejects
- [ ] `internal/component/config/redistribute/registry.go` - source registry: `RegisterSource()`, `LookupSource()`, `SourceNames()`
  → Decision: no destination registry exists; sources only
- [ ] `internal/component/bgp/plugins/redistribute_egress/register.go` - `ConfigRoots: ["redistribute"]`
  → Constraint: auto-starts when top-level "redistribute" container present in config
- [ ] `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - calls `configredist.Global()` and `Accept(route, "bgp")`
  → Constraint: hardcodes `bgpProtocolName = "bgp"` as importingProtocol
- [ ] `internal/component/bgp/plugins/redistribute_ingress/filter.go` - calls `redistribute.Global()` and `Accept(route, "")`
  → Constraint: passes empty string as importingProtocol (no loop check, source/family only)
- [ ] `internal/component/bgp/config/loader_create.go` - `initRedistribute(tree)` calls `config.ExtractRedistributeRules(tree)` with full config tree
  → Constraint: called during reactor creation; non-fatal on error
- [ ] `internal/component/web/workbench_sections.go` - `/show/redistribute/` nav entry and routing guard
  → Constraint: references `"redistribute"` as first path segment -- UNCHANGED by this spec

**Behavior to preserve:**
- Evaluator API: `Accept(route, importingProtocol) bool` signature unchanged
- Loop prevention logic unchanged
- Source registry unchanged
- Ingress filter behavior unchanged (reads same evaluator, same semantics)
- exact-or-reject validation on source names and family names
- Net effect on BGP peers identical: same UPDATEs sent, same timing

**Behavior to change:**
- `bgp-redistribute-egress` plugin replaced by shared `redistribute-orchestrator`
- BGP registers a `RedistConsumer` that wraps its `UpdateRoute` dispatch
- Consumer registry added parallel to source registry
- All existing test scenarios still pass (announce, withdraw, filtered-out, metrics, burst, nhop-self, explicit-nhop, L2TP variants)
- Config shape: `redistribute { import ... }` becomes `redistribute { bgp { import ... } }`
- YANG: add destination-protocol list/container between `redistribute` and `import`
- Config loader: reads one level deeper (destination key then import list)
- Plugin ConfigRoots: unchanged (`"redistribute"` still matches)
- Web UI: unchanged (`/show/redistribute/` stays valid)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file parsed into Tree structure (unchanged)
- Top-level `redistribute` container gains destination key nesting

### Transformation Path
1. Config file -> Tree -> `tree.GetContainer("redistribute").GetContainer("bgp").GetListOrdered("import")` (one extra level)
2. `ExtractRedistributeRules(tree)` updated to iterate destination keys, extract rules per destination
3. `redistribute.SetGlobal(redistribute.NewEvaluator(rules))` (unchanged)
4. `redistribute.Global().Accept(route, "bgp")` in egress plugin (unchanged)
5. `redistribute.Global().Accept(route, "")` in ingress filter (unchanged)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file -> Tree | YANG-driven parsing | [ ] |
| Tree -> Evaluator | `ExtractRedistributeRules` | [ ] |
| Evaluator -> Plugin | `redistribute.Global()` atomic pointer | [ ] |

### Integration Points
- `initRedistribute(tree)` in `bgp/config/loader_create.go` - reads one level deeper in tree
- `ExtractRedistributeRules` in `loader_redistribute.go` - handles destination key nesting
- `ze-redistribute-conf.yang` - adds destination list/container wrapping the existing import list

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `redistribute { bgp { import fakeredist { ... } } }` config | -> | `ExtractRedistributeRules` -> evaluator -> orchestrator -> BGP consumer -> `UpdateRoute` | `bgp-redistribute-announce.ci` |
| `redistribute { bgp { import l2tp { ... } } }` config | -> | evaluator -> orchestrator -> BGP consumer -> UPDATE | `redistribute-l2tp-announce.ci` |
| `redistribute { bgp { import ebgp { ... } } }` config parse | -> | YANG validation passes | `redistribute-family-filter.ci` |
| `redistribute { bgp { import unknown { ... } } }` config parse | -> | YANG validation passes (runtime reject) | `redistribute-invalid-source.ci` |
| BGP startup | -> | `RegisterConsumer(bgpConsumer)` -> consumer registry | `TestRegisterConsumer` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config `redistribute { bgp { import l2tp { family [ ipv4/unicast ] } } }` | Parses without error; evaluator accepts L2TP IPv4 routes into BGP |
| AC-2 | Config with flat `redistribute { import ... }` (old shape, no destination key) | YANG validation rejects (import list no longer directly under redistribute) |
| AC-3 | No `redistribute` block at all | Evaluator is nil; all routes pass ingress filter; no egress redistribution |
| AC-4 | Orchestrator plugin with `redistribute { bgp { ... } }` present | Plugin auto-starts via ConfigRoots `"redistribute"` |
| AC-5 | All 13 existing .ci functional tests | Pass with updated config shape and orchestrator dispatch |
| AC-6 | All 2 existing parse tests | Pass with updated config shape |
| AC-7 | `ze-redistribute-conf.yang` has destination-protocol container wrapping import list | Future protocols add their own destination key without schema changes |
| AC-8 | BGP registers `RedistConsumer` at startup | `LookupConsumer("bgp")` returns the BGP consumer |
| AC-9 | `ConsumerNames()` called | Returns registered consumer names (at least `"bgp"`) |
| AC-10 | Orchestrator receives route batch from L2TP producer | Routes dispatched to BGP consumer's `InjectRoute`, same UPDATE on wire as before |
| AC-11 | Orchestrator receives remove batch | Routes dispatched to BGP consumer's `WithdrawRoute`, same withdrawal on wire |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterConsumer` | `internal/component/config/redistribute/consumer_test.go` | Consumer registry add/lookup | |
| `TestLookupConsumer` | `internal/component/config/redistribute/consumer_test.go` | Lookup by name, missing name | |
| `TestConsumerNames` | `internal/component/config/redistribute/consumer_test.go` | Sorted names list | |
| `TestBGPConsumerInjectRoute` | `internal/component/bgp/redistribute/consumer_test.go` | BGP consumer produces correct announce command | |
| `TestBGPConsumerWithdrawRoute` | `internal/component/bgp/redistribute/consumer_test.go` | BGP consumer produces correct withdraw command | |
| `TestExtractRedistributeRules` (existing, updated) | `internal/component/config/loader_redistribute_test.go` | Config extraction with destination key | |
| `TestEvaluatorAccept` (existing) | `internal/component/config/redistribute/evaluator_test.go` | Evaluator unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
- N/A (no new numeric inputs)

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All 13 `test/plugin/bgp-redistribute-*.ci` and `test/plugin/redistribute-l2tp-*.ci` | `test/plugin/` | Config shape updated, same behavior | |
| Both `test/parse/redistribute-*.ci` | `test/parse/` | Config shape updated, YANG validates | |

### Future (if deferring any tests)
- None

## Files to Modify
- `internal/component/config/redistribute/schema/ze-redistribute-conf.yang` - add destination-protocol list wrapping import list
- `internal/component/config/loader_redistribute.go` - traverse destination key, return per-destination rules
- `internal/component/bgp/config/loader_create.go` - pass destination key "bgp" to extractor, register BGP consumer
- `internal/component/bgp/plugins/redistribute_egress/redistribute.go` - extract dispatch logic into BGP consumer; replace plugin with orchestrator or remove
- `internal/component/bgp/plugins/redistribute_egress/register.go` - rename plugin to `redistribute-orchestrator` or retire
- 13 `.ci` functional test files - wrap `import ...` inside `bgp { ... }` under `redistribute`
- 2 `.ci` parse test files - same config shape change
- `docs/guide/configuration.md` - update config examples
- `docs/features.md` - update config examples
- `docs/guide/l2tp.md` - update config examples if present
- `docs/architecture/core-design.md` - update architecture description

**Unchanged files (verified by critical review):**
- `internal/component/bgp/schema/ze-bgp-conf.yang` - no change needed
- `internal/component/web/workbench_sections.go` - `/show/redistribute/` stays valid
- `internal/component/config/redistribute/evaluator.go` - API unchanged
- `internal/component/config/redistribute/route.go` - types unchanged
- `internal/component/config/redistribute/registry.go` - source registry unchanged
- `internal/component/bgp/plugins/redistribute_ingress/filter.go` - runtime unchanged

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [x] | `ze-redistribute-conf.yang` only |
| CLI commands/flags | [ ] | N/A |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | Existing tests, updated config |
| Web UI | [ ] | N/A (unchanged) |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/l2tp.md`, `docs/guide/static-routes.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |

## Files to Create
- `internal/component/config/redistribute/consumer.go` - `RedistConsumer` interface, consumer registry (`RegisterConsumer`, `LookupConsumer`, `ConsumerNames`)
- `internal/component/config/redistribute/consumer_test.go` - registry unit tests
- `internal/component/bgp/redistribute/consumer.go` - BGP consumer implementation wrapping `UpdateRoute` dispatch

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Max 2 review passes |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Consumer interface + registry** -- define `RedistConsumer` interface and consumer registry in `internal/component/config/redistribute/consumer.go`
   - Tests: `TestRegisterConsumer`, `TestLookupConsumer`, `TestConsumerNames`
   - Files: `consumer.go`, `consumer_test.go`
   - Verify: registry tests pass

2. **Phase: BGP consumer** -- implement BGP consumer wrapping `UpdateRoute` dispatch, extracted from `bgp-redistribute-egress`
   - Tests: `TestBGPConsumerInjectRoute`, `TestBGPConsumerWithdrawRoute`
   - Files: `internal/component/bgp/redistribute/consumer.go`
   - Verify: unit tests pass; consumer produces same commands as current `formatAnnounce`/`formatWithdraw`

3. **Phase: Orchestrator** -- refactor `bgp-redistribute-egress` into a generic orchestrator that dispatches to registered consumers
   - Tests: existing `bgp-redistribute-announce.ci` (same behavior, different internal path)
   - Files: `redistribute_egress/redistribute.go`, `redistribute_egress/register.go`
   - Verify: functional tests pass with orchestrator dispatching to BGP consumer

4. **Phase: YANG refactor** -- add destination-protocol list inside `redistribute` container, wrapping the existing `import` list
   - Tests: YANG parse tests (`redistribute-family-filter.ci`, `redistribute-invalid-source.ci`)
   - Files: `ze-redistribute-conf.yang`
   - Verify: config with `redistribute { bgp { import ... } }` parses; old flat shape rejected

5. **Phase: Config loader** -- update `ExtractRedistributeRules` to traverse destination key before reading import list
   - Tests: `TestExtractRedistributeRules` (update existing unit tests for new tree shape)
   - Files: `loader_redistribute.go`, `loader_create.go`
   - Verify: unit tests pass with new tree structure

6. **Phase: Test configs** -- update all 15 .ci files to wrap import inside `bgp { }` under `redistribute`
   - Tests: all 13 functional tests + 2 parse tests
   - Files: all .ci files listed in scope
   - Verify: `make ze-functional-test` passes

7. **Phase: Documentation** -- update all docs referencing the old config shape
   - Files: docs listed in Documentation Update Checklist
   - Verify: grep for old shape returns zero hits in docs

8. **Full verification** -- `make ze-verify`
9. **Complete spec** -- Fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Old flat `redistribute { import ... }` shape no longer parses (must have destination key) |
| Naming | YANG destination list key follows ze naming convention |
| Data flow | Config extraction traverses destination key before import list |
| Rule: no-layering | Old flat import list directly under redistribute is removed |
| Rule: grep-for-old-shape | No `.ci` file has `redistribute {` immediately followed by `import` (must have `bgp {` in between) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `RedistConsumer` interface defined | `grep "RedistConsumer" internal/component/config/redistribute/consumer.go` |
| Consumer registry functions | `grep "RegisterConsumer\|LookupConsumer\|ConsumerNames" internal/component/config/redistribute/consumer.go` |
| BGP consumer registered | `grep "RegisterConsumer" internal/component/bgp/redistribute/consumer.go` |
| Orchestrator dispatches to consumers | `grep "LookupConsumer\|InjectRoute\|WithdrawRoute" internal/component/bgp/plugins/redistribute_egress/redistribute.go` |
| YANG destination container | `grep "list\|container" ze-redistribute-conf.yang` shows destination level |
| ConfigRoots unchanged | `grep '"redistribute"' redistribute_egress/register.go` |
| Web UI unchanged | `grep "show/redistribute" workbench_sections.go` |
| All functional tests pass | `make ze-functional-test` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Source name and family validation unchanged (exact-or-reject) |
| Config injection | No new user-controlled strings in paths or commands |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check config shape in .ci file |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

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

- Root-level approach chosen over protocol-owns-its-config because: web UI `/show/redistribute/`
  stays valid, ConfigRoots unchanged, forward compatible with VRF spec's cross-VRF redistribution
  (also top-level), and fewer files to modify.
- Consumer interface chosen over per-protocol consumer plugins: eliminates duplicated
  subscribe/filter/dispatch loop. Adding OSPF as a destination becomes one interface
  implementation + one `RegisterConsumer` call + one YANG destination key.
- Global evaluator singleton will need to become per-VRF when VRF lands (spec-vrf-7). This is a
  pre-existing concern unaffected by this spec. The consumer registry has the same shape
  (package-level, process-wide) and will need the same VRF treatment.
- Cross-module YANG `uses` with prefix is proven (9+ existing modules use `uses zt:listener`).
  goyang resolves it during `modules.Process()`.

## RFC Documentation
- N/A (no protocol wire changes)

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-redist-explicit-dest.md`
- [ ] Summary included in commit
