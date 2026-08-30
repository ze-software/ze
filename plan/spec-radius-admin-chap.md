# Spec: radius-admin-chap

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

> **SKELETON.** Created at closure of `radius-admin-backend` per user request
> (deferred assumption A-3: PAP-only MVP). Needs a full `/ze-spec` research pass
> before promotion to `ready`. Section content below is a sketch, not a design.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` -- workflow rules.
3. `plan/learned/NNN-radius-admin-backend.md` -- the PAP admin backend this extends.
4. Source: `internal/component/radius/authenticator.go`, `internal/component/radius/attr.go`, `internal/component/l2tp/plugins/authradius/handler.go`.

## Task

Add **CHAP** (RFC 2865 §5.3: CHAP-Password + CHAP-Challenge) as an admin-login
auth method for the RADIUS admin backend, alongside the existing PAP path in
`radiusAuthenticator.Authenticate`. Deferred from the `radius-admin-backend` MVP.

**Open design question (resolve first):** admin login arrives as username+password
over the SSH/web/MCP password callback; there is no CHAP challenge in that flow.
Determine whether admin CHAP is meaningful for this transport or only for a future
NAS-style front end. If PAP is the only workable method for the callback, this
spec may reduce to "documented as N/A" with the rationale recorded.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/radius.md` -- the shipped PAP admin backend and its config surface.
  → Constraint: CHAP must not change the PAP default or the profile-mapping path.

### RFC Summaries
- [ ] `rfc/short/rfc2865.md` -- CHAP-Password (§5.3), CHAP-Challenge (attr 60).
  → Constraint: CHAP-Password = CHAP-Ident(1) + CHAP-Response(16); response is MD5(ident + secret + challenge).

## Current Behavior (MANDATORY)

**Source files read:** (to be completed during /ze-spec)
- [ ] `internal/component/radius/authenticator.go` -- PAP-only `Authenticate`; where the method branch goes.
- [ ] `internal/component/radius/attr.go` -- `EncodeCHAPPassword` already exists.
- [ ] `internal/component/l2tp/plugins/authradius/handler.go` -- CHAP attr assembly reference (`buildAuthAttrs`).

**Behavior to preserve:** PAP path, profile mapping, chain semantics, L2TP path.

**Behavior to change:** add a configurable admin auth method that emits CHAP attributes.

## Data Flow (MANDATORY)

### Entry Point
- Operator SSH/web/MCP login when the admin auth method is configured as CHAP.

### Transformation Path
1. Resolve auth method (PAP default, CHAP if configured).
2. For CHAP: generate challenge, build CHAP-Password + CHAP-Challenge attributes.
3. Send Access-Request; map Accept/Reject/error as PAP does.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → method selection | new `auth-method` leaf | [ ] |
| Backend → RADIUS server | CHAP attributes on Access-Request | [ ] |

### Integration Points
- `internal/component/radius/authenticator.go` -- method branch.
- `internal/component/radius/yang/ze-radius-conf.yang` -- `auth-method` leaf.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | CHAP is meaningful for the SSH/web/MCP password callback | operator request | Spec becomes N/A | design review | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | CHAP has no challenge in the SSH password flow | design review | Document as N/A or scope to a NAS front end |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `auth-method chap` configured | → | Access-Request carries CHAP-Password | `TestRadiusChapAttributes` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Admin auth method = CHAP | Access-Request carries CHAP-Password + CHAP-Challenge, not User-Password |
| AC-2 | Server Access-Accept | same profile mapping as PAP |
| AC-3 | Method unset | PAP remains the default; no behavior change |

## End-to-End User Stories (MANDATORY)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures CHAP admin auth and logs in | login → CHAP Access-Request → Accept → profiles | functional `.ci` (to be designed) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRadiusChapAttributes` | `internal/component/radius/authenticator_test.go` | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-chap` | `test/plugin/aaa-radius-chap.ci` | CHAP admin login accepted | |

## RFC Documentation
- RFC 2865 §5.3 CHAP-Password; verify `rfc/short/rfc2865.md` covers it (it does after radius-admin-backend).

## Files to Modify
- `internal/component/radius/authenticator.go` -- method branch.
- `internal/component/radius/yang/ze-radius-conf.yang` -- `auth-method` leaf.

## Files to Create
- `test/plugin/aaa-radius-chap.ci`.

## Implementation Steps

### Implementation Phases
1. **Phase: Research** -- resolve the open design question (CHAP over the SSH callback).
2. **Phase: Config** -- `auth-method` enum leaf (pap default, chap).
3. **Phase: Authenticator** -- CHAP branch reusing `EncodeCHAPPassword`.
4. **Phase: Tests + docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | PAP default unchanged; CHAP attributes correct per §5.3 |
| L2TP untouched | no change under `l2tp/plugins/authradius/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secret handling | shared secret never logged |
| Challenge quality | CHAP challenge cryptographically random |

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
- [ ] Open design question resolved
- [ ] `./le verify worktree` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
