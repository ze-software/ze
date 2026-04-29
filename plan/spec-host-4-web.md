# Spec: host-4-web

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-host-0-inventory (done), spec-web-3-foundation (in-progress) |
| Phase | - |
| Updated | 2026-04-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/host/inventory.go` - Inventory struct
4. `internal/component/web/` - web handler patterns
5. `internal/component/cmd/show/host.go` - existing `show host *` RPC registration

## Task

Add a web UI panel for host inventory. Displays hardware information (CPU, memory,
NICs, storage, thermal, DMI, kernel) in the operator workbench with auto-refresh
via SSE. Follows the workbench component patterns from spec-web-3-foundation.

**Origin:** deferred from spec-host-0-inventory (item 7 in Deferrals table, dated 2026-04-18).
Reason for deferral: depends on inventory shipping and web foundation; orthogonal to detection.

## Required Reading

### Architecture Docs
- [ ] `internal/component/web/` - handler patterns, HTMX templates
- [ ] `internal/component/cmd/show/host.go` - `show host *` RPC pattern
- [ ] `plan/spec-web-3-foundation.md` - workbench component patterns
- [ ] `plan/spec-web-2-operator-workbench.md` - workbench shell

**Key insights:**
- Skeleton: to be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/web/` - existing page handlers
- [ ] `internal/component/cmd/show/host.go` - `show host *` RPCs
- [ ] `internal/component/host/inventory.go` - Inventory struct

**Behavior to preserve:**
- Existing `show host *` RPCs unchanged
- Web handler patterns consistent with other pages
- Inventory detection unchanged

**Behavior to change:**
- Add web page for host inventory
- Add SSE stream for live thermal/NIC data

## Data Flow (MANDATORY)

### Entry Point
- HTTP GET `/host` or `/system/host` (TBD based on web-3 navigation structure)
- SSE stream for live updates

### Transformation Path
1. HTTP request arrives at host page handler
2. Handler calls `host.Detect()` or `host.DetectSection()`
3. Inventory data rendered into HTMX template
4. SSE stream pushes thermal/NIC updates on change

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| HTTP ↔ Host detection | Handler calls Detect | [ ] |
| SSE ↔ Inventory diff | Change events trigger SSE push | [ ] |

### Integration Points
- Web handler registry - register host page
- Workbench navigation - add host entry
- SSE infrastructure - push inventory changes

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| GET /host | → | handleHostPage | `test/web/web-host-page.ci` |
| SSE /host/events | → | handleHostSSE | `test/web/web-host-sse.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Navigate to host inventory page | All inventory sections displayed |
| AC-2 | CPU section expanded | Per-core details visible |
| AC-3 | NIC section | Per-NIC table with carrier status color-coded |
| AC-4 | Thermal alarm active | Red indicator on thermal section |
| AC-5 | SSE stream connected | Thermal and NIC data update without page reload |
| AC-6 | Non-Linux host | Graceful "unsupported platform" message |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestHostPageHandler` | `internal/component/web/handler_host_test.go` | AC-1 | skeleton |
| `TestHostPageUnsupported` | `internal/component/web/handler_host_test.go` | AC-6 | skeleton |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-host-page` | `test/web/web-host-page.ci` | GET /host returns HTML with inventory sections | skeleton |
| `web-host-sse` | `test/web/web-host-sse.ci` | SSE stream delivers thermal update | skeleton |

### Future (if deferring any tests)
- None planned

## Files to Create
- `internal/component/web/handler_host.go` - page handler
- `internal/component/web/handler_host_test.go` - tests
- `internal/component/web/templates/host.html` - HTMX template
- `test/web/web-host-page.ci` - functional test
- `test/web/web-host-sse.ci` - functional test

## Files to Modify
- `internal/component/web/` - register host route in router
- Navigation template - add host entry

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/web/*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/guide/web-ui.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/web-ui.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4-13. | Standard flow |

### Implementation Phases

1. **Phase: Page Handler** -- HTTP handler, template, route registration
   - Tests: `TestHostPageHandler`, `TestHostPageUnsupported`
   - Files: `handler_host.go`, `templates/host.html`
   - Verify: tests fail → implement → tests pass

2. **Phase: SSE Stream** -- live updates for thermal/NIC
   - Tests: functional SSE test
   - Files: `handler_host.go` (SSE endpoint)
   - Verify: SSE delivers updates

3. **Phase: Navigation** -- add host entry to workbench nav
   - Tests: functional page test
   - Files: navigation template
   - Verify: host page reachable from nav

4. **Full verification** → `make ze-verify`

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All sections render, SSE delivers updates |
| Naming | Routes follow existing web conventions |
| Data flow | HTTP → handler → Detect → template |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| handler_host.go exists | `ls internal/component/web/handler_host.go` |
| Template exists | `ls internal/component/web/templates/host.html` |
| All tests pass | `go test ./internal/component/web/...` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| XSS | Template escaping for inventory strings (model names, serial numbers) |
| Input validation | No user input in inventory detection path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| 3 fix attempts fail | STOP. Ask user. |

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

## RFC Documentation

N/A

## Implementation Summary

### What Was Implemented
- Skeleton

### Bugs Found/Fixed
- None

### Documentation Updates
- None

### Deviations from Plan
- None

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
- **Total items:** -
- **Done:** -
- **Partial:** -
- **Skipped:** -
- **Changed:** -

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- Skeleton

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
- [ ] AC-1..AC-N all demonstrated
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
