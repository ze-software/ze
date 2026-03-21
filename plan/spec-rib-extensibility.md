# Spec: rib-extensibility

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/attribute/community.go` - hardcoded community switch
4. `internal/component/bgp/plugins/rib/rib_commands.go` - hardcoded command switch
5. `internal/component/bgp/plugins/rib/storage/routeentry.go` - Stale + LLGRStale flags
6. `internal/component/plugin/registry/registry.go` - Registration struct, Dependencies

## Task

Refactor three hardcoded patterns into plugin-extensible registries so that GR, LLGR, and future plugins can extend RIB behavior without modifying core code:

1. **Community name registry** -- plugins register well-known community names at init(), replacing the hardcoded switch in `community.go:String()`
2. **Generic stale mechanism** -- replace `Stale bool` + `LLGRStale bool` with a single `StaleLevel uint8` that plugins set via RIB commands, with best-path comparing levels (lower = more preferred)
3. **RIB command registration** -- plugins register command handlers with the RIB at startup, replacing the hardcoded switch in `rib_commands.go:handleCommand()`

Constraint: LLGR must initialize after RIB when both are loaded, so it can register its commands and stale level against the RIB's extension points.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - plugin registration, dependency ordering
  -> Constraint: plugins register via init(), Dependencies field controls load order
  -> Decision: registry is a leaf package with no plugin imports
- [ ] `docs/architecture/plugin/rib-storage-design.md` - RIB storage, RouteEntry
  -> Constraint: RouteEntry fields are per-route metadata, not pooled

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc1997.md` - BGP communities (well-known values)
  -> Constraint: well-known communities are in 0xFFFF0000-0xFFFFFFFF range
- [ ] `rfc/short/rfc4724.md` - GR stale marking
  -> Constraint: stale routes treated normally in best-path (no depreference)
- [ ] `rfc/short/rfc9494.md` - LLGR stale marking
  -> Constraint: LLGR-stale routes treated as least preferred

**Key insights:**
- GR and LLGR have different stale semantics: GR-stale routes compete normally, LLGR-stale routes are least-preferred. A uint8 level captures this ordering.
- The community switch in `community.go` couples the attribute package to plugin-specific knowledge (LLGR_STALE, NO_LLGR). Plugins should own their community names.
- The RIB command switch grows with every GR/LLGR feature. Registration makes the RIB a framework that plugins extend.
- Plugin Dependencies already control init order. Adding bgp-rib to bgp-gr's Dependencies (already done) ensures RIB is available when GR registers commands.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/attribute/community.go` - Community type is uint32. String() has a switch with 6 well-known communities (NO_EXPORT, NO_ADVERTISE, NO_EXPORT_SUBCONFED, NOPEER, LLGR_STALE, NO_LLGR). Adding a new well-known community requires editing this file.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go` - handleCommand is a switch with 17 cases. Adding a new RIB command requires editing this file. Each case delegates to a method on RIBManager.
- [ ] `internal/component/bgp/plugins/rib/storage/routeentry.go` - RouteEntry has Stale bool (GR) and LLGRStale bool (LLGR) as separate fields. Adding another stale mechanism would require a third bool.
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` - ComparePair has Step 0 (LLGR depreference) hardcoded. A generic stale level would replace this with a single comparison.
- [ ] `internal/component/plugin/registry/registry.go` - Registration has Dependencies field. Server resolves dependencies and starts plugins in order. bgp-gr already depends on bgp-rib.

**Behavior to preserve:**
- All existing community String() output unchanged
- All existing RIB commands work identically
- GR stale marking behavior (routes compete normally in best-path)
- LLGR stale depreference (least preferred)
- Plugin dependency ordering

**Behavior to change:**
- Community String() reads from a registry instead of a switch
- RIB commands dispatched via registered handlers instead of a switch
- RouteEntry uses StaleLevel instead of Stale + LLGRStale bools
- Best-path compares StaleLevel instead of checking LLGRStale bool
- bgp-gr plugin registers its community names, stale levels, and RIB commands at init

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Plugin init() registers community names, stale levels, and RIB command handlers
- RIB plugin starts, makes its command registry available
- GR/LLGR plugin starts (after RIB), registers commands against RIB's registry

### Transformation Path
1. Community name registry: `attribute.RegisterCommunityName(0xFFFF0006, "LLGR_STALE")` called in bgp-gr init()
2. Community.String() looks up registered name, falls back to ASN:value format
3. RIB command registry: `rib.RegisterCommand("enter-llgr", handler)` called during bgp-gr Stage 2/3
4. RIB handleCommand() looks up handler in registry, dispatches
5. Stale level: `rib mark-stale` sets StaleLevel=1 (GR), `rib enter-llgr` sets StaleLevel=2 (LLGR)
6. Best-path: ComparePair compares StaleLevel (0 < 1 < 2, lower is more preferred)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin init -> Community registry | Function call during init() | [ ] |
| Plugin startup -> RIB command registry | Inter-plugin registration during handshake | [ ] |
| RIB command dispatch -> Registered handler | Function lookup + call | [ ] |
| Best-path -> StaleLevel | Direct field comparison on Candidate | [ ] |

### Integration Points
- `attribute/community.go` - String() uses registry lookup
- `registry/registry.go` - Registration may gain a WellKnownCommunities field
- `rib_commands.go` - handleCommand uses registered handlers
- `storage/routeentry.go` - StaleLevel replaces Stale + LLGRStale
- `bestpath.go` - ComparePair compares StaleLevel

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Design

### D-1: Community Name Registry

| Component | Detail |
|-----------|--------|
| Location | `internal/component/bgp/attribute/community.go` |
| API | `RegisterCommunityName(value Community, name string)` |
| Storage | `map[Community]string` (package-level, populated during init) |
| Thread safety | Populated only during init(), read-only after. No mutex needed. |
| Fallback | Unknown communities still format as `ASN:value` |
| RFC 1997 built-ins | NO_EXPORT, NO_ADVERTISE, NO_EXPORT_SUBCONFED, NOPEER remain as constants and are pre-registered (they're RFC 1997, not plugin-specific) |

Plugins register in their `register.go`:

| Plugin | Communities |
|--------|-------------|
| bgp-gr | LLGR_STALE (0xFFFF0006), NO_LLGR (0xFFFF0007) |
| (future) | Any well-known community the plugin needs named |

### D-2: Generic Stale Levels

| Level | Meaning | Best-Path Behavior | Set By |
|-------|---------|-------------------|--------|
| 0 | Fresh (not stale) | Normal comparison | Default / route refresh |
| 1 | GR-stale | Normal comparison (RFC 4724: no depreference) | `rib mark-stale` (bgp-gr) |
| 2 | LLGR-stale | Least preferred (RFC 9494: depreference) | `rib enter-llgr` (bgp-gr) |

**RouteEntry change:** Replace `Stale bool` + `LLGRStale bool` with `StaleLevel uint8`.

**Best-path change:** ComparePair Step 0 becomes: if `a.StaleLevel != b.StaleLevel`, lower level wins. Level 0 (fresh) beats level 1 (GR-stale) beats level 2 (LLGR-stale). Between equal levels, normal tiebreaking applies.

**Note:** GR-stale (level 1) and fresh (level 0) both use normal best-path comparison per RFC 4724. To preserve this, the comparator should only depreference at level >= 2. Or more precisely: level 0 and 1 are treated as equivalent in best-path (both "normal"), level 2+ is "least preferred".

Revised comparison:

| a.StaleLevel | b.StaleLevel | Result |
|-------------|-------------|--------|
| 0 or 1 | 0 or 1 | Normal tiebreaking (GR-stale competes normally per RFC 4724) |
| 0 or 1 | >= 2 | a wins (normal beats LLGR-stale) |
| >= 2 | 0 or 1 | b wins |
| >= 2 | >= 2 | Lower level wins, then normal tiebreaking |

This is equivalent to: "is either candidate at LLGR-stale level or above? If so, the one with lower level wins. Otherwise, normal comparison."

**Stale level constants** should be registered by plugins, not hardcoded in the RIB:

| Constant | Value | Registered By |
|----------|-------|--------------|
| `StaleLevelFresh` | 0 | RIB (built-in) |
| `StaleLevelGR` | 1 | bgp-gr plugin |
| `StaleLevelLLGR` | 2 | bgp-gr plugin |

The RIB only knows that level 0 is fresh and level >= `depreferenceThreshold` (default 2) triggers depreference. Plugins register their levels.

### D-3: RIB Command Registration

| Component | Detail |
|-----------|--------|
| Location | `internal/component/bgp/plugins/rib/rib_commands.go` |
| API | `RegisterCommand(name string, handler CommandHandler)` |
| Handler type | `func(r *RIBManager, selector string, args []string) (string, string, error)` |
| When | During plugin startup (Stage 2-3), after RIB is running |
| Thread safety | Registration during single-threaded startup phase; dispatch during event loop (read-only) |

The handleCommand switch becomes:

1. Check built-in commands (rib status, rib show, rib help, etc.)
2. Check registered commands from plugins
3. Fail on unknown

**Init order:** bgp-gr has `Dependencies: ["bgp-rib"]`. The plugin server starts bgp-rib first, which makes its command registry available. When bgp-gr starts, it registers its GR/LLGR commands during Stage 2 (OnConfigure) or via a new Stage-1 hook.

**Alternative approach:** Instead of runtime registration, use the existing Registration struct. Add a `RIBCommands map[string]func(...)` field to Registration. The RIB plugin reads all registrations at startup and builds its command table. This avoids runtime registration entirely -- everything is declared in init().

| Approach | Pros | Cons |
|----------|------|------|
| Runtime registration (RIB API) | Explicit, plugins actively register | Requires init ordering, more complex |
| Registration struct field | Follows existing pattern, all in init() | RIB must scan all registrations at startup |

**Decision:** Use the Registration struct approach. It follows the existing pattern (NLRI decoders/encoders are already registered this way). The RIB reads `registry.All()` at startup to build its command table.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin init with WellKnownCommunities | -> | Community.String() returns registered name | Unit test `TestCommunityRegistryLookup` |
| Plugin Registration with RIBCommands | -> | RIB dispatches to registered handler | Unit test `TestRIBCommandRegistration` |
| rib mark-stale sets StaleLevel=1 | -> | RouteEntry.StaleLevel == 1 | Unit test `TestMarkStaleSetsLevel` |
| rib enter-llgr sets StaleLevel=2 | -> | Best-path deprioritizes level 2 | Unit test `TestBestPathStaleLevel` |
| Config with GR + LLGR | -> | Full flow through registration | `test/plugin/graceful-restart.ci` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin registers community name via Registration | Community.String() returns registered name |
| AC-2 | Plugin registers community name for existing built-in | Registration fails (built-ins are immutable) |
| AC-3 | Unknown community value | String() returns ASN:value format (fallback) |
| AC-4 | Plugin registers RIB command via Registration field | RIB dispatches command to registered handler |
| AC-5 | Duplicate RIB command registration | Registration fails with error |
| AC-6 | `rib mark-stale` | RouteEntry.StaleLevel set to GR level (1) |
| AC-7 | `rib enter-llgr` | RouteEntry.StaleLevel set to LLGR level (2) |
| AC-8 | Best-path with StaleLevel 0 vs 2 | Level 0 wins |
| AC-9 | Best-path with StaleLevel 1 vs 1 | Normal tiebreaking (GR-stale competes normally) |
| AC-10 | Best-path with StaleLevel 1 vs 2 | Level 1 wins (GR-stale beats LLGR-stale) |
| AC-11 | bgp-gr plugin loads after bgp-rib | GR/LLGR commands registered, stale levels available |
| AC-12 | Community constants still importable | CommunityLLGRStale, CommunityNoLLGR remain as typed constants |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommunityRegistryLookup` | `attribute/community_test.go` | Registered community names returned by String() | |
| `TestCommunityRegistryBuiltinProtected` | `attribute/community_test.go` | Cannot override built-in community names | |
| `TestCommunityRegistryFallback` | `attribute/community_test.go` | Unknown community formats as ASN:value | |
| `TestRIBCommandRegistration` | `rib/rib_commands_test.go` | Registered command handler called | |
| `TestRIBCommandDuplicate` | `rib/rib_commands_test.go` | Duplicate registration rejected | |
| `TestStaleLevelDefault` | `rib/storage/stale_test.go` | New RouteEntry has StaleLevel=0 | |
| `TestStaleLevelGR` | `rib/rib_gr_test.go` | mark-stale sets StaleLevel to GR level | |
| `TestStaleLevelLLGR` | `rib/rib_gr_test.go` | enter-llgr sets StaleLevel to LLGR level | |
| `TestBestPathStaleLevel` | `rib/bestpath_test.go` | Level 0 beats 2, level 1 ties with 1, level 1 beats 2 | |
| `TestPurgeByStaleLevel` | `rib/rib_gr_test.go` | purge-stale works with StaleLevel field | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| StaleLevel | 0-255 | 255 | N/A | N/A (uint8) |
| Community value | 0-0xFFFFFFFF | 0xFFFFFFFF | N/A | N/A (uint32) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing GR tests | `test/plugin/graceful-restart*.ci` | GR still works with new stale mechanism | |
| Existing LLGR tests | `test/parse/graceful-restart-llgr.ci` | LLGR config still parses | |

### Future (if deferring any tests)
- None

## Files to Modify

- `internal/component/bgp/attribute/community.go` - Add registry, update String()
- `internal/component/bgp/plugins/rib/rib_commands.go` - Add command registry, update handleCommand
- `internal/component/bgp/plugins/rib/storage/routeentry.go` - StaleLevel replaces Stale + LLGRStale
- `internal/component/bgp/plugins/rib/bestpath.go` - StaleLevel comparison
- `internal/component/bgp/plugins/gr/register.go` - Register communities, commands, stale levels
- `internal/component/bgp/plugins/gr/gr.go` - Use StaleLevel in commands
- `internal/component/bgp/plugins/gr/gr_state.go` - Use StaleLevel constants
- `internal/component/plugin/registry/registry.go` - WellKnownCommunities + RIBCommands fields

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | N/A |
| RPC count in architecture docs | [ ] | N/A |
| CLI commands/flags | [ ] | N/A |
| CLI usage/help text | [ ] | N/A |
| API commands doc | [ ] | N/A |
| Plugin SDK docs | [x] | `.claude/rules/plugin-design.md` (Registration fields) |
| Editor autocomplete | [ ] | N/A |
| Functional test for new RPC/API | [ ] | Existing tests cover behavior |

## Files to Create

- None (refactor, not new feature)

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Community Registry** -- add RegisterCommunityName, update String()
   - Tests: `TestCommunityRegistryLookup`, `TestCommunityRegistryBuiltinProtected`, `TestCommunityRegistryFallback`
   - Files: `attribute/community.go`, `attribute/community_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: StaleLevel** -- replace Stale + LLGRStale with StaleLevel uint8
   - Tests: `TestStaleLevelDefault`, `TestStaleLevelGR`, `TestStaleLevelLLGR`, `TestBestPathStaleLevel`
   - Files: `storage/routeentry.go`, `bestpath.go`, `rib_commands.go`, `gr.go`
   - Verify: tests fail -> implement -> tests pass
   - Note: all existing Stale/LLGRStale usage must be migrated in this phase

3. **Phase: RIB Command Registry** -- add command registration, update handleCommand
   - Tests: `TestRIBCommandRegistration`, `TestRIBCommandDuplicate`
   - Files: `rib_commands.go`, `registry/registry.go`, `gr/register.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Plugin Registration** -- bgp-gr registers communities, levels, commands
   - Tests: verify bgp-gr init() registers everything
   - Files: `gr/register.go`
   - Verify: existing GR/LLGR tests still pass

5. **Full verification** -- `make ze-verify`

6. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All three registries working, bgp-gr uses them |
| Correctness | StaleLevel comparison matches RFC 4724 (GR=normal) and RFC 9494 (LLGR=depreference) |
| Naming | Registry functions follow existing naming patterns |
| Data flow | Registration at init() time, lookup at runtime (read-only, no locks needed) |
| Rule: no-layering | Old Stale + LLGRStale fields fully removed, not kept alongside StaleLevel |
| Rule: plugin-design | Registration struct fields, not direct imports |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Community registry | grep for RegisterCommunityName in `community.go` |
| StaleLevel on RouteEntry | grep for StaleLevel in `routeentry.go` |
| No Stale bool | grep confirms Stale bool removed from RouteEntry |
| No LLGRStale bool | grep confirms LLGRStale bool removed from RouteEntry |
| RIB command registry | grep for RegisterCommand or RIBCommands in registry |
| bgp-gr registers communities | grep for RegisterCommunityName in `gr/register.go` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Registry overflow | Community registry is bounded by uint32 key space (finite) |
| Command name collision | Duplicate registration detected and rejected |
| StaleLevel manipulation | Only settable via RIB commands (plugin boundary), not wire data |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

- Community registry follows the same pattern as NLRI decoder/encoder registration: init()-time, read-only at runtime, no locks
- StaleLevel uint8 is intentionally wider than needed (3 values today) to leave room for future stale mechanisms without another refactor
- RIB command registration via Registration struct avoids runtime dependency on init order for the command table itself (init() populates Registration, RIB reads at startup)

## RFC Documentation

Add `// RFC 4724 Section 4.2: stale routes compete normally (StaleLevel 1)` and `// RFC 9494: LLGR-stale routes least preferred (StaleLevel 2)` above level constants.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled)

### Documentation Updates
- (to be filled)

### Deviations from Plan
- (to be filled)

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
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-rib-extensibility.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
