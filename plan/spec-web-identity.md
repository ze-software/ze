# Spec: web-identity -- Router Identity and Multi-Router Web Management

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-web-0-umbrella |
| Phase | - |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/` - web server implementation
4. `internal/component/ssh/` - CLI session management

## Task

Two related problems:

### 1. Router Identity Display

When managing multiple routers, users need to know which router they are currently working on. The router identity (router-id, hostname, or a configured display name) must be prominently visible in both the CLI prompt and the web interface header/breadcrumb.

Today there is no persistent visual indicator of which router instance the user is connected to. This creates risk of misconfiguration when operators manage multiple routers in separate tabs or terminals.

### 2. Multi-Router Peer Selection (Web Fleet View)

The web interface should be usable across the network to manage multiple ze instances. A peer/router selector in the web UI would let operators switch between routers without opening separate browser tabs. This requires:

- A way to discover or configure the list of ze instances (peer routers)
- A selector in the web interface to switch the active router
- Clear visual distinction of which router is active
- Session state preserved per router (pending edits, context path)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/web-interface.md` - web server structure and rendering
- [ ] `docs/architecture/config/syntax.md` - config tree and YANG schema
- [ ] `docs/guide/web-interface.md` - user-facing web documentation

### RFC Summaries (MUST for protocol work)
- Not applicable (web UI feature, not protocol work)

**Key insights:**
- To be filled during design phase

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/fragment.go` - finder columns, breadcrumb, layout data
- [ ] `internal/component/web/render.go` - template rendering, layout structure
- [ ] `internal/component/ssh/session.go` - CLI prompt format
- [ ] `internal/component/config/system/` - system config (hostname candidate)

**Behavior to preserve:**
- To be filled during design phase

**Behavior to change:**
- CLI prompt: add router identity
- Web header/breadcrumb: add router identity
- Web layout: add router/peer selector

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config file: router identity from `system { name }` or `bgp { router-id }`
- Fleet config: list of remote ze instances from config or discovery

### Transformation Path
1. Config parsing extracts router identity
2. Identity passed to web renderer and CLI session
3. Fleet list loaded at startup or on demand
4. Web proxy forwards requests to selected remote ze instance

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Web renderer | Identity string passed to layout data | [ ] |
| Config -> CLI prompt | Identity string in prompt prefix | [ ] |
| Web proxy -> Remote ze | HTTPS forwarding with auth | [ ] |

### Integration Points
- To be filled during design phase

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config with system name | -> | Web header shows name | To be defined |
| Config with system name | -> | CLI prompt shows name | To be defined |
| Fleet config with peers | -> | Web selector shows peers | To be defined |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Router has configured display name | CLI prompt shows name (e.g., `core-01[bgp]#`) |
| AC-2 | Router has configured display name | Web header shows name prominently |
| AC-3 | No display name configured | Falls back to router-id |
| AC-4 | Two browser tabs to different routers | Each tab shows different identity clearly |
| AC-5 | Fleet config lists 3 ze instances | Web selector shows all 3 |
| AC-6 | User switches router in web selector | View updates to selected router's config |
| AC-7 | User has pending edits on router A, switches to B | Edits on A are preserved when switching back |
| AC-8 | Remote ze instance is unreachable | Selector shows it as offline |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| To be defined during design | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Display name length | 1-255 | 255 chars | empty | 256 chars |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| To be defined during design | | | |

### Future (if deferring any tests)
- Dynamic peer discovery (requires BGP integration)
- Side-by-side multi-router view

## Files to Modify
- `internal/component/web/fragment.go` - add identity to layout data
- `internal/component/web/render.go` - render identity in header
- `internal/component/web/templates/page/layout.html` - identity display element
- `internal/component/ssh/session.go` - identity in CLI prompt
- `internal/component/bgp/schema/ze-bgp-conf.yang` - display name leaf (or system config)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] | system name leaf or bgp display name |
| CLI commands/flags | [ ] | prompt format |
| Editor autocomplete | [ ] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [ ] | To be determined |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | |
| 5 | Plugin added/changed? | [ ] | |
| 6 | Has a user guide page? | [ ] | `docs/guide/web-interface.md` |
| 7 | Wire format changed? | [ ] | |
| 8 | Plugin SDK/protocol changed? | [ ] | |
| 9 | RFC behavior implemented? | [ ] | |
| 10 | Test infrastructure changed? | [ ] | |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | |

## Files to Create
- To be determined during design phase

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Router Identity Config** -- add display name to system/bgp config
   - Tests: to be defined
   - Files: YANG schema, config resolution
   - Verify: tests fail -> implement -> tests pass
2. **Phase: CLI Identity** -- show router name in CLI prompt
   - Tests: to be defined
   - Files: SSH session, CLI prompt
   - Verify: tests fail -> implement -> tests pass
3. **Phase: Web Identity** -- show router name in web header
   - Tests: to be defined
   - Files: web fragment, layout template
   - Verify: tests fail -> implement -> tests pass
4. **Phase: Fleet Config** -- configure list of remote ze instances
   - Tests: to be defined
   - Files: YANG schema, config parsing
   - Verify: tests fail -> implement -> tests pass
5. **Phase: Web Router Selector** -- selector UI for switching routers
   - Tests: to be defined
   - Files: web templates, JS, proxy handler
   - Verify: tests fail -> implement -> tests pass

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Identity source priority correct (name > hostname > router-id) |
| Naming | Config leaf names follow ze conventions |
| Data flow | Identity flows from config to both CLI and web |
| Security | Fleet proxy does not leak credentials between routers |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Router identity in CLI prompt | SSH to ze, check prompt |
| Router identity in web header | Open web, check header |
| Router selector in web | Open web, check selector |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Display name sanitized for HTML (XSS) |
| Auth forwarding | Fleet proxy does not forward credentials to wrong router |
| TLS | Fleet proxy uses TLS to remote ze instances |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Open Questions

| # | Question |
|---|----------|
| Q-1 | Should identity be a new config leaf (e.g., `system { name "core-01" }`) or reuse router-id? |
| Q-2 | Should the fleet view be a proxy (one web server forwards to others) or direct (browser connects to each)? |
| Q-3 | Should router discovery be static config only, or also support dynamic discovery (e.g., via BGP peer list)? |
| Q-4 | Should the web UI support side-by-side views of two routers? |
| Q-5 | How should the CLI show identity -- in the prompt prefix, or as a status line? |

## Design Notes (to explore during design phase)

### Identity Source Priority

| Priority | Source | Example |
|----------|--------|---------|
| 1 | Configured display name | `system { name "core-01" }` |
| 2 | System hostname | from OS |
| 3 | Router-ID | `1.2.3.4` |

### Fleet Architecture Options

| Option | Pros | Cons |
|--------|------|------|
| Proxy mode (one web server) | Single URL, unified auth | Added complexity, latency |
| Direct mode (browser to each) | Simple, no middleman | Multiple tabs, separate auth per router |
| Hybrid (discovery via one, connect direct) | Best of both | More moving parts |

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

Not applicable (web UI feature, not protocol work).

## Implementation Summary

### What Was Implemented
- To be filled

### Bugs Found/Fixed
- To be filled

### Documentation Updates
- To be filled

### Deviations from Plan
- To be filled

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] Summary included in commit
