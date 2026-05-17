# Spec: ASPA Policy Enforcement

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-bgp-2-aspa (done) |
| Phase | - |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rpki/rpki.go` - plugin entry point
4. `internal/component/bgp/plugins/rpki/aspa_verify.go` - ASPA verification algorithm
5. `internal/component/bgp/plugins/rpki/rpki_config.go` - config parsing
6. `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` - YANG schema

## Task

Add policy enforcement for ASPA path verification results, mirroring the existing RPKI origin validation policy. Currently ASPA results are informational only (included in the event JSON as `"aspa-state"` but never triggering accept/reject). This spec adds configurable actions for ASPA states, allowing operators to reject routes whose AS_PATH fails ASPA verification.

The model follows the existing `rpki/policy/invalid-action` and `rpki/policy/not-found-action` pattern. Two new config leaves:

- `rpki/policy/aspa-invalid-action` (default: `log-only`) - action for ASPA Invalid routes
- `rpki/policy/aspa-unknown-action` (default: `accept`) - action for ASPA Unknown routes

Default is `log-only` for invalid (conservative: ASPA is a newer standard with incomplete deployment) rather than `reject` (which is the ROA origin validation default). Operators explicitly opt into enforcement.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - adj-rib-in validation gate
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - existing policy dispatch in dispatchValidation()

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9582.md` - RTR v2 ASPA PDU format
- [ ] `rfc/short/draft-ietf-sidrops-aspa-verification.md` - ASPA verification algorithm

**Key insights:**
- RPKI origin validation rejects via `adj-rib-in reject-routes` command dispatched from `dispatchValidation()`
- ASPA verification runs in `handleEvent()` / `handleStructuredUpdate()`, after origin validation
- The `validationRequest` struct carries per-prefix state to the worker
- ASPA state applies to the entire UPDATE (shared AS_PATH), not per-prefix
- A route that is ROA Valid but ASPA Invalid should still be rejectable
- `RPKIPlugin` has `aspaEnabled` atomic.Bool gating ASPA computation

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` - handleEvent dispatches origin validation to adj-rib-in; ASPA state computed but only emitted in event JSON; dispatchValidation() at line 517 sends reject/accept
- [ ] `internal/component/bgp/plugins/rpki/rpki_config.go` - parses policy/invalid-action, policy/not-found-action; ASPAValidation bool
- [ ] `internal/component/bgp/plugins/rpki/aspa_verify.go` - verifyASPA returns ASPAValid/ASPAInvalid/ASPAUnknown
- [ ] `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` - policy container with invalid-action and not-found-action
- [ ] `internal/component/bgp/plugins/rpki/aspa_tracker.go` - tracks routes for re-validation; Revalidate() returns changed routes

**Behavior to preserve:**
- Origin validation continues to work exactly as before
- ASPA event emission (`"aspa-state"` in JSON) remains unchanged
- When `aspa-validation false`, no ASPA policy is applied
- Route tracking and re-validation on cache change remain unchanged
- The adj-rib-in validation gate holds routes for origin validation

**Behavior to change:**
- ASPA Invalid routes can now trigger reject (currently always accepted regardless of ASPA state)
- ASPA Unknown routes can now trigger reject (currently always accepted)
- New config options under `rpki/policy/` for ASPA actions
- `handleASPAChange()` (line 697) must dispatch reject on re-validation when policy demands it

## Data Flow (MANDATORY)

### Entry Point
- BGP UPDATE arrives from peer, processed by handleEvent() or handleStructuredUpdate()
- ASPA state already computed as `aspaState` variable

### Transformation Path
1. handleEvent()/handleStructuredUpdate() computes ASPA state (already done)
2. ASPA state is carried into the per-prefix validation dispatch
3. In dispatchValidation(): if origin says accept but ASPA policy says reject, override to reject
4. adj-rib-in receives the final decision

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin -> adj-rib-in | `adj-rib-in reject-routes` command via DispatchCommand | [ ] |
| Config -> Plugin | OnConfigure JSON with new policy fields | [ ] |

### Integration Points
- `validationRequest` struct (line 79) carries per-prefix state; needs ASPA state added
- `dispatchValidation()` (line 517) makes the accept/reject decision; needs ASPA override logic
- `handleASPAChange()` (line 697) handles re-validation; needs to dispatch reject for newly-invalid routes

### Architectural Verification
- [ ] No bypassed layers (uses existing adj-rib-in gate)
- [ ] No unintended coupling (ASPA policy is in the same plugin as origin policy)
- [ ] No duplicated functionality (extends existing dispatch, doesn't recreate)
- [ ] Zero-copy preserved where applicable

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Config `rpki/policy/aspa-invalid-action reject` | -> | ASPA reject dispatch | `rpki-aspa-policy-reject.ci` |
| Config `rpki/policy/aspa-unknown-action reject` | -> | ASPA unknown reject | `rpki-aspa-policy-unknown-reject.ci` |
| Config `rpki/policy/aspa-invalid-action log-only` | -> | No reject, event emitted | `rpki-aspa-policy-logonly.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `aspa-invalid-action reject` + route with ASPA Invalid path | Route rejected (adj-rib-in reject-routes), not installed in RIB |
| AC-2 | `aspa-invalid-action log-only` + route with ASPA Invalid path | Route accepted, ASPA state logged and in event JSON |
| AC-3 | `aspa-invalid-action accept` + route with ASPA Invalid path | Route accepted, ASPA state in event JSON |
| AC-4 | `aspa-unknown-action reject` + route with ASPA Unknown path | Route rejected |
| AC-5 | `aspa-unknown-action accept` (default) + ASPA Unknown | Route accepted |
| AC-6 | `aspa-validation false` + any aspa-invalid-action | No ASPA verification runs, routes accepted based on origin only |
| AC-7 | Route is ROA Valid but ASPA Invalid + `aspa-invalid-action reject` | Route rejected (ASPA overrides origin accept) |
| AC-8 | ASPA cache changes, route re-validated, new state is Invalid + reject policy | Route withdrawn on re-validation |
| AC-9 | Config parsing: new YANG leaves present and valid | parseRPKIConfig returns correct policy values |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseRPKIConfigASPAPolicy` | `rpki_config_test.go` | New policy fields parsed correctly | |
| `TestParseRPKIConfigASPAPolicyDefaults` | `rpki_config_test.go` | Defaults: invalid=log-only, unknown=accept | |
| `TestASPAPolicyReject` | `rpki_test.go` | ASPA Invalid + reject -> reject command dispatched | |
| `TestASPAPolicyLogOnly` | `rpki_test.go` | ASPA Invalid + log-only -> no reject, event emitted | |
| `TestASPAPolicyOverridesOrigin` | `rpki_test.go` | ROA Valid + ASPA Invalid + reject -> route rejected | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-aspa-policy-reject` | `test/plugin/rpki-aspa-policy-reject.ci` | ASPA Invalid route rejected when policy=reject | |
| `rpki-aspa-policy-unknown-reject` | `test/plugin/rpki-aspa-policy-unknown-reject.ci` | ASPA Unknown route rejected when unknown-action=reject | |
| `rpki-aspa-policy-logonly` | `test/plugin/rpki-aspa-policy-logonly.ci` | ASPA Invalid route accepted when policy=log-only | |

## Files to Modify
- `internal/component/bgp/plugins/rpki/rpki_config.go` - add ASPAInvalidAction, ASPAUnknownAction to rpkiConfig
- `internal/component/bgp/plugins/rpki/rpki.go` - integrate ASPA policy decision into dispatch flow
- `internal/component/bgp/plugins/rpki/aspa_tracker.go` - on re-validation state change, apply policy
- `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` - add aspa-invalid-action, aspa-unknown-action leaves
- `docs/guide/rpki.md` - document new policy options

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new leaves) | Yes | `internal/component/bgp/plugins/rpki/schema/ze-rpki.yang` |
| CLI commands/flags | No | existing `rpki status` suffices |
| Editor autocomplete | Yes | YANG-driven (automatic if YANG updated) |
| Functional test for new policy | Yes | `test/plugin/rpki-aspa-policy-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/rpki.md`, wiki `rpki.md` |
| 5 | Plugin behavior changed? | Yes | wiki `bgp-rpki.md` |
| 11 | Affects daemon comparison? | No | |

## Files to Create
- `test/plugin/rpki-aspa-policy-reject.ci` - functional test for ASPA reject policy
- `test/plugin/rpki-aspa-policy-unknown-reject.ci` - functional test for ASPA unknown reject
- `test/plugin/rpki-aspa-policy-logonly.ci` - functional test for ASPA log-only

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |

### Implementation Phases

1. **Phase: YANG + Config Parsing** - add new leaves, parse them
   - Tests: `TestParseRPKIConfigASPAPolicy`, `TestParseRPKIConfigASPAPolicyDefaults`
   - Files: `ze-rpki.yang`, `rpki_config.go`, `rpki_config_test.go`
   - Verify: config correctly parsed with new fields

2. **Phase: Policy Dispatch** - integrate ASPA state into accept/reject flow
   - Tests: `TestASPAPolicyReject`, `TestASPAPolicyLogOnly`, `TestASPAPolicyOverridesOrigin`
   - Files: `rpki.go`
   - Verify: ASPA Invalid/Unknown routes dispatched correctly based on policy
   - Design: carry aspaState in `validationRequest`; in `dispatchValidation()`, if origin says accept but ASPA policy says reject, override to reject. The ASPA state is per-UPDATE (shared AS_PATH), so it applies uniformly to all prefixes in the UPDATE.

3. **Phase: Re-validation Policy** - apply policy on ASPA cache change
   - Tests: functional test verifying withdrawal on re-validation
   - Files: `rpki.go` (`handleASPAChange`), `aspa_tracker.go`
   - Verify: route withdrawn when ASPA state changes to Invalid with reject policy

4. **Functional tests** - end-to-end .ci tests
5. **Documentation** - update docs/guide/rpki.md, wiki
6. **Full verification** - `make ze-verify`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | ASPA reject overrides origin accept; order cannot cause accept-after-reject race |
| Naming | YANG leaves: `aspa-invalid-action`, `aspa-unknown-action` (kebab-case) |
| Data flow | Policy decision flows through existing adj-rib-in command interface |
| Rule: no-layering | No new command protocol; reuses existing `adj-rib-in reject-routes` |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| YANG schema with new leaves | `grep aspa-invalid-action schema/ze-rpki.yang` |
| Config parsing | `go test -run TestParseRPKIConfigASPAPolicy` |
| Policy dispatch | `go test -run TestASPAPolicyReject` |
| Functional tests | `ze-test bgp plugin --test rpki-aspa-policy-reject` |
| Docs updated | `grep aspa-invalid-action docs/guide/rpki.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | New enum values must be validated (only reject/log-only/accept) |
| Resource exhaustion | No new unbounded structures (policy is a static config enum) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| ASPA reject races with origin accept | Redesign dispatch ordering |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Insights

Key design question: how does ASPA reject interact with the per-prefix origin validation dispatch?

Origin validation dispatches per-prefix (each NLRI gets its own accept/reject via `validationRequest`). ASPA state applies to the entire UPDATE (it verifies the AS_PATH, which is shared across all NLRIs). The interaction:

- For each prefix: if origin says reject, reject (ASPA state irrelevant)
- For each prefix: if origin says accept but ASPA policy says reject, send reject instead
- This means ASPA reject overrides any origin accept

Implementation: add `aspaState uint8` to `validationRequest`. In `dispatchValidation()`, after the existing origin-based decision, check if ASPA policy demands reject. If so, override the accept to reject. This keeps the change minimal and localized to the existing dispatch path.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Docs updated

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling
