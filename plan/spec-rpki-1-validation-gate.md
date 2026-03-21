# Spec: rpki-1-validation-gate

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-rpki-0-umbrella |
| Phase | - |
| Updated | 2026-03-04 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rpki-0-umbrella.md` — umbrella spec, architecture decisions
4. `internal/component/bgp/plugins/bgp-adj-rib-in/` — current Adj-RIB-In implementation
5. `internal/component/bgp/plugins/bgp-gr/gr.go` — GR coordination pattern (model)
6. `pkg/plugin/sdk/sdk_engine.go` — DispatchCommand SDK method

## Task

Implement the validation gate coordination primitive: extend bgp-adj-rib-in with a "pending" route state and accept/reject/revalidate commands. When a validation plugin (bgp-rpki, future spec) is loaded and enables validation, received routes are held as pending until the validator issues an accept or reject command. When no validator is loaded, routes flow through unchanged (zero overhead).

This spec implements the infrastructure only — no RTR client, no ROA cache, no actual RPKI validation. Testing uses mock validation via DispatchCommand.

**Parent spec:** `spec-rpki-0-umbrella.md`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor, event dispatch
  → Constraint: UPDATE events delivered in parallel to all subscribers
- [ ] `plan/learned/339-gr-receiving-speaker.md` — GR retain/release pattern
  → Decision: Inter-plugin coordination uses DispatchCommand, dependency ordering guarantees sequencing

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc6811.md` — validation states (Valid, Invalid, NotFound)
  → Constraint: Three states, locally computed, not a BGP attribute

**Key insights:**
- The validation gate is a generic coordination primitive — any future plugin could use it (not just RPKI)
- GR's retain/release pattern is the established model: dependent plugin sends command before RIB acts
- Zero overhead when no validator loaded: a single boolean check gates the pending path

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/bgp-adj-rib-in/rib.go` — RIB-In plugin: subscribes to "update direction received" + "state", stores raw routes in seqmap
- [ ] `internal/component/bgp/plugins/bgp-adj-rib-in/register.go` — registration, no dependencies
- [ ] `internal/component/bgp/plugins/bgp-rib/rib_commands.go` — retain/release command handlers (model)
- [ ] `internal/component/bgp/plugins/bgp-gr/gr.go` — DispatchCommand usage pattern
- [ ] `pkg/plugin/sdk/sdk_engine.go` — DispatchCommand, OnExecuteCommand

**Behavior to preserve:**
- Route storage format in bgp-adj-rib-in unchanged for installed routes
- All existing command handlers in bgp-adj-rib-in unchanged
- bgp-rs route forwarding path unchanged
- Event delivery model (parallel for UPDATEs) unchanged

**Behavior to change:**
- bgp-adj-rib-in gains a "pending" route state alongside "installed"
- New commands: `rib enable-validation`, `rib accept-routes`, `rib reject-routes`, `rib revalidate`
- When validation enabled: received routes stored as pending, not immediately installed
- Pending routes have a configurable timeout (fail-open safety valve)

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Validator plugin sends `rib enable-validation` command during startup
- UPDATE events arrive at both bgp-adj-rib-in and the validator plugin in parallel

### Transformation Path

**Route insertion (validation enabled):**
1. bgp-adj-rib-in receives UPDATE event, extracts prefix + attributes
2. Checks `validationEnabled` flag
3. If enabled: stores route with state=Pending, records message-id + timestamp
4. Validator plugin receives same UPDATE, computes validation
5. Validator sends `rib accept-routes <peer> <family> <prefix>` or `rib reject-routes <peer> <family> <prefix>`
6. bgp-adj-rib-in: accept → promotes to Installed (state=Valid or NotFound); reject → discards (or marks Invalid per policy)

**Route insertion (validation NOT enabled):**
1. bgp-adj-rib-in receives UPDATE event, extracts prefix + attributes
2. Checks `validationEnabled` flag — false
3. Stores route as Installed immediately (current behavior, zero overhead)

**Timeout safety valve:**
1. Background goroutine scans pending routes periodically (every 5s)
2. Routes pending longer than timeout (default 30s) are promoted to Installed
3. Timeout logged as warning — indicates validator plugin is unhealthy

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Validator plugin ↔ bgp-adj-rib-in | DispatchCommand: accept/reject/revalidate | [ ] |
| Validator plugin ↔ Engine | DispatchCommand RPC via SDK | [ ] |

### Integration Points
- `bgp-adj-rib-in`: extended with pending state, new commands
- Command dispatcher: new commands registered by bgp-adj-rib-in
- No changes to event delivery, reactor, or FSM

### Architectural Verification
- [ ] No bypassed layers — uses existing DispatchCommand path
- [ ] No unintended coupling — bgp-adj-rib-in checks boolean flag, not validator identity
- [ ] No duplicated functionality — pending state is new, commands are new
- [ ] Zero-copy preserved — route storage format unchanged

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| DispatchCommand("rib enable-validation") | -> | bgp-adj-rib-in sets validationEnabled=true | `TestEnableValidationCommand` |
| UPDATE received + validation enabled | -> | Route stored as pending | `TestRouteStoredAsPending` |
| DispatchCommand("rib accept-routes ...") | -> | Pending route promoted to installed | `TestAcceptRoutesCommand` |
| DispatchCommand("rib reject-routes ...") | -> | Pending route discarded | `TestRejectRoutesCommand` |
| UPDATE received + validation NOT enabled | -> | Route stored immediately as installed | `TestNoValidationPassthrough` |
| Pending route exceeds timeout | -> | Route promoted to installed (fail-open) | `TestPendingRouteTimeout` |
| DispatchCommand("rib revalidate ...") | -> | Installed route re-exported for validation | `TestRevalidateCommand` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rib enable-validation` command received | validationEnabled flag set to true, subsequent routes use pending state |
| AC-2 | UPDATE received, validation enabled | Route stored as Pending with timestamp |
| AC-3 | `rib accept-routes <peer> <family> <prefix>` command | Matching pending route promoted to Installed with validation state |
| AC-4 | `rib reject-routes <peer> <family> <prefix>` command | Matching pending route discarded (not stored) |
| AC-5 | UPDATE received, validation NOT enabled | Route stored immediately as Installed (current behavior) |
| AC-6 | Pending route older than timeout | Route promoted to Installed (fail-open), warning logged |
| AC-7 | `rib revalidate <family> <prefix>` command | Route data sent back to validator for re-validation |
| AC-8 | accept-routes for non-existent pending route | Command returns error, no crash |
| AC-9 | reject-routes for already-installed route | Command returns error, no state change |
| AC-10 | Multiple pending routes for same prefix (path-id) | Each resolved independently by accept/reject |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnableValidation` | `bgp-adj-rib-in/rib_test.go` | Flag toggled by command | |
| `TestPendingRouteStorage` | `bgp-adj-rib-in/rib_test.go` | Route stored as pending when validation enabled | |
| `TestAcceptPendingRoute` | `bgp-adj-rib-in/rib_test.go` | Pending route promoted to installed on accept | |
| `TestRejectPendingRoute` | `bgp-adj-rib-in/rib_test.go` | Pending route discarded on reject | |
| `TestPassthroughWithoutValidation` | `bgp-adj-rib-in/rib_test.go` | Route stored immediately when validation disabled | |
| `TestPendingTimeout` | `bgp-adj-rib-in/rib_test.go` | Pending route auto-promoted after timeout | |
| `TestRevalidateInstalledRoute` | `bgp-adj-rib-in/rib_test.go` | Revalidate command triggers re-export | |
| `TestAcceptNonExistentRoute` | `bgp-adj-rib-in/rib_test.go` | Error returned for unknown pending route | |
| `TestRejectAlreadyInstalled` | `bgp-adj-rib-in/rib_test.go` | Error returned, no state change | |
| `TestMultiplePendingRoutes` | `bgp-adj-rib-in/rib_test.go` | Multiple pending routes resolved independently | |
| `TestValidationStateField` | `bgp-adj-rib-in/rib_test.go` | Validation state stored on route entry | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| validationState | 0-3 | 3 (Invalid) | N/A (uint8) | 4+ (unknown) |
| pendingTimeout | 1-3600 | 3600s | 0s | 3601s |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-rpki-validation-gate` | `test/plugin/rpki-validation-gate.ci` | Plugin sends enable-validation, UPDATE arrives, accept/reject commands resolve routes | |
| `test-rpki-passthrough` | `test/plugin/rpki-passthrough.ci` | Without rpki plugin loaded, routes flow through unchanged | |

### Future (if deferring any tests)
- RTR integration tests deferred to spec-rpki-2-rtr-client.md
- End-to-end RPKI validation tests deferred to spec-rpki-3-origin-validation.md

## Files to Modify
- `internal/component/bgp/plugins/bgp-adj-rib-in/rib.go` — add pending state, validation flag, timeout goroutine
- `internal/component/bgp/plugins/bgp-adj-rib-in/rib_commands.go` — new file: accept/reject/revalidate/enable-validation handlers

## Files to Create
- `internal/component/bgp/plugins/bgp-adj-rib-in/rib_validation.go` — pending route management, timeout scanner
- `internal/component/bgp/plugins/bgp-adj-rib-in/rib_validation_test.go` — unit tests
- `test/plugin/rpki-validation-gate.ci` — functional test
- `test/plugin/rpki-passthrough.ci` — functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [ ] No — commands registered at plugin level | N/A |
| CLI commands/flags | [ ] No — validation is automatic when plugin loaded | N/A |
| Plugin SDK docs | [ ] Yes — document new commands | `.claude/rules/plugin-design.md` |
| Functional test for new commands | [x] Yes | `test/plugin/rpki-validation-gate.ci` |

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Write unit tests** for pending state, accept/reject, timeout, passthrough → Review: edge cases? Boundary tests?
2. **Run tests** → Verify FAIL (paste output). Fail for RIGHT reason?
3. **Implement rib_validation.go** — pending route map, timeout scanner, validation state field → Minimal code to pass
4. **Implement rib_commands.go** — enable-validation, accept-routes, reject-routes, revalidate handlers
5. **Extend rib.go** — check validationEnabled flag in handleReceived, route to pending or installed
6. **Run tests** → Verify PASS (paste output). All pass? Any flaky?
7. **Write functional tests** → rpki-validation-gate.ci, rpki-passthrough.ci
8. **Verify all** → `make ze-test` (lint + all ze tests including fuzz + exabgp)
9. **Critical Review** → All 6 checks from `rules/quality.md` must pass
10. **Complete spec** → Fill audit tables, write learned summary

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Step 3/4 (fix syntax/types) |
| Test fails wrong reason | Step 1 (fix test) |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |

## Command Syntax

### rib enable-validation

Sent by validator plugin during startup. Enables the pending route state in bgp-adj-rib-in.

| Field | Type | Description |
|-------|------|-------------|
| (no args) | — | Toggles validationEnabled to true |

Response: `{"status":"ok"}`

### rib accept-routes

Sent by validator plugin after validation completes.

| Field | Type | Description |
|-------|------|-------------|
| peer | string | Peer IP address |
| family | string | Address family ("ipv4/unicast") |
| prefix | string | NLRI prefix ("10.0.0.0/24") |
| state | uint8 | Validation state: 1=Valid, 2=NotFound |

Response: `{"status":"ok"}` or `{"status":"error","error":"no pending route for ..."}`

### rib reject-routes

Sent by validator plugin after validation determines invalidity.

| Field | Type | Description |
|-------|------|-------------|
| peer | string | Peer IP address |
| family | string | Address family |
| prefix | string | NLRI prefix |

Response: `{"status":"ok"}` or `{"status":"error","error":"no pending route for ..."}`

### rib revalidate

Sent by validator plugin when ROA cache changes. Requests bgp-adj-rib-in to re-export route data for re-validation.

| Field | Type | Description |
|-------|------|-------------|
| family | string | Address family |
| prefix | string | NLRI prefix (or "*" for all) |

Response: JSON with route data for each matching installed route.

## Pending Route Data Structure

Each pending route stores:

| Field | Type | Description |
|-------|------|-------------|
| peerAddr | string | Source peer IP |
| family | string | Address family |
| prefix | string | NLRI prefix |
| rawRoute | RawRoute | Existing route data (wire bytes, attributes) |
| receivedAt | time.Time | When the route was received |
| validationState | uint8 | 0=Pending, 1=Valid, 2=NotFound, 3=Invalid |

Pending routes are stored in a separate map from installed routes. On accept, the route is moved to the installed map. On reject, it is discarded. On timeout, it is moved to installed with state=NotValidated.

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

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]
