# Spec: radius-admin-interop-freeradius

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

> **SKELETON.** Created at closure of `radius-admin-backend` (interop deferral,
> user-approved 2026-07-08). Needs a full `/ze-spec` research pass before
> promotion to `ready`. Section content below is a sketch, not a design.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` -- workflow rules; `ai/rules/interop-and-goal-validation.md`.
3. `plan/learned/NNN-radius-admin-backend.md` -- the admin backend under test.
4. `test/interop/interop.py`, `test/interop/Dockerfile.*`, `test/interop/daemons` -- the existing Docker interop harness (BGP-peer-only today).

## Task

Add a FreeRADIUS interop scenario proving the RADIUS admin backend authenticates
against a real RADIUS server: Access-Request/Accept/Reject plus Filter-Id →
profile mapping. Deferred from `radius-admin-backend` because the interop harness
(`test/interop/interop.py`) is a Docker-based BGP-peer harness with no RADIUS
category; this is a distinct infrastructure build.

**Why deferred, not dropped:** the functional `.ci` tests
(`test/plugin/aaa-radius-admin.ci`, `aaa-radius-fallback.ci`) already drive the
real RADIUS wire protocol end-to-end via the production `radius` package, and the
admin backend reuses the same `radius.Client` the L2TP subscriber path already
exercises against real servers. This spec adds third-party-server compatibility
confidence (FreeRADIUS quirks), not new wire coverage.

## Required Reading

### Architecture Docs
- [ ] `test/interop/interop.py` -- daemon container lifecycle, config injection, check runner.
  → Constraint: today it only models BGP peers (FRR/BIRD/GoBGP); a RADIUS daemon type is new.
- [ ] `docs/guide/radius.md` -- the admin backend config surface to drive.

### RFC Summaries
- [ ] `rfc/short/rfc2865.md` -- Access-Request/Accept/Reject, Filter-Id (§5.11).

## Current Behavior (MANDATORY)

**Source files read:** (to be completed during /ze-spec)
- [ ] `test/interop/interop.py` -- how a scenario's daemon container is built, started, and probed.
- [ ] `test/interop/scenarios/bgp-ebgp-ipv4-frr/` -- reference scenario layout (ze.conf + peer.conf + check.py).
- [ ] `internal/component/radius/authenticator.go` -- the backend the scenario validates.

**Behavior to preserve:** all existing BGP interop scenarios; the admin backend unchanged.

**Behavior to change:** add a `Dockerfile.freeradius`, a RADIUS daemon type in the harness, and a scenario.

## Data Flow (MANDATORY)

### Entry Point
- `test/interop/interop.py` runs the new `NN-radius-admin-freeradius` scenario.

### Transformation Path
1. Start a FreeRADIUS container with a users file (admin/testpass, Filter-Id=admin).
2. Start Ze with `system/authentication/radius` pointed at the container.
3. Drive an admin login; assert Accept + profile, then a wrong password Reject.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ FreeRADIUS | Docker container + users config | [ ] |
| Ze backend ↔ FreeRADIUS | real Access-Request/Accept/Reject over UDP | [ ] |

### Integration Points
- `test/interop/Dockerfile.freeradius` (new).
- `test/interop/interop.py` -- RADIUS daemon type.
- `test/interop/scenarios/NN-radius-admin-freeradius/` (new).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | interop.py can be extended to a non-BGP daemon type without deep surgery | harness read | Larger refactor needed | design review | unvalidated |
| A-2 | Docker + a FreeRADIUS image are available in the interop environment | existing Docker harness | Cannot run the scenario | check CI/docker availability | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | interop.py is too BGP-specific to extend cleanly | reading interop.py | Add a minimal parallel non-BGP runner path |
| R-2 | FreeRADIUS config drift (version quirks) | scenario flakiness | Pin the image tag; document the users file |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `interop.py` runs the scenario | → | Ze admin login against FreeRADIUS | `NN-radius-admin-freeradius/check.py` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Valid admin creds vs FreeRADIUS | Access-Accept; login succeeds; Filter-Id → profile |
| AC-2 | Wrong password vs FreeRADIUS | Access-Reject; login denied; no local fallthrough (or documented) |
| AC-3 | FreeRADIUS container down | infra error; local fallback (mirrors `aaa-radius-fallback.ci`) |

## End-to-End User Stories (MANDATORY)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs the interop suite | harness → FreeRADIUS + Ze → admin login accepted | `NN-radius-admin-freeradius` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| n/a (interop scenario, not a unit) | -- | -- | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-admin.ci` | `test/plugin/aaa-radius-admin.ci` | Existing mock-server functional baseline the interop scenario extends | shipped |
| `NN-radius-admin-freeradius` | `test/interop/scenarios/NN-radius-admin-freeradius/` | Ze admin login against real FreeRADIUS | |

## RFC Documentation
- RFC 2865 already summarized in `rfc/short/rfc2865.md` (Filter-Id row added by radius-admin-backend).

## Files to Modify
- `internal/component/radius/authenticator.go` -- any real-server compatibility fix the interop reveals (e.g. an attribute FreeRADIUS requires that the mock did not). Interop testing exists to surface exactly these; expect small backend adjustments, not a rewrite.
- `test/interop/interop.py` -- add a RADIUS daemon type / scenario support.
- `test/interop/daemons` (or a new config) -- FreeRADIUS runtime config if needed.

## Files to Create
- `test/interop/Dockerfile.freeradius`.
- `test/interop/scenarios/NN-radius-admin-freeradius/{ze.conf,freeradius/,check.py}`.

## Implementation Steps

### Implementation Phases
1. **Phase: Harness** -- add a FreeRADIUS daemon type to interop.py; `Dockerfile.freeradius`.
2. **Phase: Scenario** -- ze.conf + FreeRADIUS users; check.py asserts Accept/Reject/profile.
3. **Phase: CI wiring + docs** -- register the scenario; update `docs/functional-tests.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | Accept/Reject/profile assertions match the backend semantics |
| No BGP regression | existing interop scenarios still pass |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret handling | FreeRADIUS shared secret confined to the test fixture |

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 demonstrated
- [ ] Docker availability confirmed
- [ ] interop suite passes
- [ ] `./le verify current mode full` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
