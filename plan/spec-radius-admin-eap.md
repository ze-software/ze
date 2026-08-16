# Spec: radius-admin-eap

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-08 |

> **SKELETON.** Created at closure of `radius-admin-backend` per user request
> (deferred assumption A-3: PAP-only MVP). Needs a full `/ze-spec` research pass
> before promotion to `ready`. Section content below is a sketch, not a design.
> EAP is substantially larger than CHAP; this may split into its own multi-phase spec.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file.
2. `.claude/rules/planning.md` -- workflow rules.
3. `plan/learned/NNN-radius-admin-backend.md` -- the PAP admin backend this extends.
4. Source: `internal/component/radius/authenticator.go`, `internal/component/radius/packet.go`, `internal/component/radius/dict.go`.

## Task

Add **EAP** (RFC 3579, RADIUS support for EAP; Access-Challenge round trips,
EAP-Message + Message-Authenticator attributes) as an admin-login auth method
for the RADIUS admin backend. Deferred from the `radius-admin-backend` MVP.

**Open design question (resolve first):** EAP is a multi-round-trip method with
Access-Challenge; the current admin backend is single-shot (Access-Request →
Accept/Reject). EAP requires a state machine and a way to relay EAP frames
between the login client and the RADIUS server. Determine which EAP methods are
in scope (EAP-TLS, EAP-TTLS, PEAP, EAP-MSCHAPv2) and how EAP frames reach the
backend from the SSH/web/MCP transport (which does not natively carry EAP). This
likely needs a transport-level design decision before any RADIUS code.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/radius.md` -- the shipped PAP admin backend.
  → Constraint: EAP must not change the PAP default or the profile-mapping path.

### RFC Summaries
- [ ] `rfc/short/rfc2865.md` -- base RADIUS; Access-Challenge (code 11).
- [ ] `rfc/short/rfc3579.md` -- RADIUS EAP (EAP-Message attr 79, Message-Authenticator attr 80). Create via `/ze-rfc` if missing.
  → Constraint: every EAP Access-Request/Challenge MUST carry a Message-Authenticator (HMAC-MD5); the client already has `VerifyMessageAuthenticator` (`packet.go`).

## Current Behavior (MANDATORY)

**Source files read:** (to be completed during /ze-spec)
- [ ] `internal/component/radius/authenticator.go` -- single-shot PAP; EAP needs a challenge loop.
- [ ] `internal/component/radius/packet.go` -- `VerifyMessageAuthenticator`, `CodeAccessChallenge`.
- [ ] `internal/component/radius/dict.go` -- `AttrMessageAuthenticator` (80); EAP-Message (79) is NOT yet defined.

**Behavior to preserve:** PAP path, profile mapping, chain semantics, L2TP path.

**Behavior to change:** add an EAP method with Access-Challenge round trips.

## Data Flow (MANDATORY)

### Entry Point
- Operator login when the admin auth method is configured as EAP (transport relay TBD).

### Transformation Path
1. Resolve auth method (PAP default, EAP if configured).
2. EAP: relay EAP frames in EAP-Message; loop on Access-Challenge until Accept/Reject.
3. Verify Message-Authenticator on every server response.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Login transport ↔ EAP frames | relay mechanism (TBD) | [ ] |
| Backend ↔ RADIUS server | EAP-Message + Message-Authenticator | [ ] |

### Integration Points
- `internal/component/radius/authenticator.go` -- EAP state machine.
- `internal/component/radius/dict.go` -- EAP-Message attribute.
- Login transport (SSH/web/MCP) -- EAP relay (design decision).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The login transport can relay EAP frames | none yet | EAP admin auth is infeasible over this transport | design review | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | EAP scope explodes (methods, TLS, state machine) | design review | Scope to one method (e.g. EAP-MSCHAPv2) or defer entirely |
| R-2 | No EAP transport in SSH/web/MCP login | design review | May require a new front end; document as out of scope |

## Wiring Test (MANDATORY)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `auth-method eap` configured | → | Access-Request carries EAP-Message + Message-Authenticator | `TestRadiusEapFirstRequest` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Admin auth method = EAP | Access-Request carries EAP-Message + Message-Authenticator |
| AC-2 | Server Access-Challenge | backend relays and continues the EAP exchange |
| AC-3 | Final Access-Accept | same profile mapping as PAP |
| AC-4 | Method unset | PAP remains the default; no behavior change |

## End-to-End User Stories (MANDATORY)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures EAP admin auth and logs in | login → EAP exchange → Accept → profiles | functional `.ci` (to be designed) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRadiusEapFirstRequest` | `internal/component/radius/authenticator_test.go` | AC-1 | |
| `TestRadiusEapChallengeLoop` | `internal/component/radius/authenticator_test.go` | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `aaa-radius-eap` | `test/plugin/aaa-radius-eap.ci` | EAP admin login accepted | |

## RFC Documentation
- RFC 3579 (RADIUS EAP): ensure `rfc/short/rfc3579.md` exists and covers EAP-Message + Message-Authenticator.

## Files to Modify
- `internal/component/radius/authenticator.go` -- EAP state machine.
- `internal/component/radius/dict.go` -- EAP-Message attribute (79).
- `internal/component/radius/yang/ze-radius-conf.yang` -- `auth-method` leaf (shared with CHAP spec).

## Files to Create
- `test/plugin/aaa-radius-eap.ci`.

## Implementation Steps

### Implementation Phases
1. **Phase: Research** -- resolve the EAP-transport-relay design question and method scope.
2. **Phase: Attributes** -- EAP-Message; reuse Message-Authenticator.
3. **Phase: State machine** -- Access-Challenge loop.
4. **Phase: Tests + docs.**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | PAP default unchanged; Message-Authenticator verified on every response |
| L2TP untouched | no change under `l2tp/plugins/authradius/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Message-Authenticator | present + verified on all EAP packets (RFC 3579) |
| Method downgrade | EAP cannot be downgraded to a weaker method silently |

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
- [ ] AC-1..AC-4 demonstrated
- [ ] EAP transport-relay design question resolved
- [ ] `make ze-standard-test` passes

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
