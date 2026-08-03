# Spec: radius-chap-eap-admin

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/guide/radius.md`, `docs/features.md` (the RADIUS admin AAA row)
4. `internal/component/radius/authenticator.go` (`Authenticate`), `internal/component/radius/aaa.go`

## Task

CHAP/EAP admin authentication, and admin-session accounting, for the RADIUS **admin**
AAA path (operator login via SSH, web, MCP) — distinct from the L2TP subscriber RADIUS
path, which has its own auth plugins.

**This spec RECORDS A KNOWN GAP. It does not report a defect.** `docs/features.md`
marks RADIUS admin AAA `Partial` **by design** and already states, in the row itself:
"CHAP/EAP admin auth and admin-session accounting are follow-up work." Nothing is
broken. PAP-only is the shipped, documented, intended scope, and this spec exists only
so the follow-up has a home rather than living in a deferrals row.

**This is the lowest-priority of the four specs opened on 2026-07-16.** It is demand-driven:
open it when an operator needs CHAP or EAP. Until then it is a placeholder.

Verified at the producer on 2026-07-16: `radiusAuthenticator.Authenticate`
(`internal/component/radius/authenticator.go`) is PAP-only. Its doc comment at
`:85-86` says "performs PAP (RFC 2865 User-Password) authentication", and the
Access-Request it builds at `:106-111` carries exactly four attributes —
`AttrUserName`, `AttrUserPassword`, `AttrServiceType`, `AttrNASIdentifier` —
plus `AttrNASIPAddress` at `:113` when an IPv4 source is set. There is no
CHAP-Password attribute, no EAP-Message attribute, and no accounting packet anywhere
in the file.

Points to complete:

| # | Point |
|---|-------|
| 1 | CHAP admin authentication (RFC 2865 Section 5.3 CHAP-Password, Section 5.40 CHAP-Challenge) |
| 2 | EAP admin authentication (RFC 3579 EAP-Message / Message-Authenticator, multi-round-trip) |
| 3 | Admin-session accounting (RFC 2866 Accounting-Request Start/Stop for operator logins) |
| 4 | Config surface deciding which method is used, and the fallback order between methods |
| 5 | Update `docs/features.md`, whose row currently names all of this as follow-up work |

### Open Questions

| ID | Question | Status |
|----|----------|--------|
| O-1 | Does CHAP fit ze's admin login at all? CHAP needs the cleartext password (or a reversibly stored one) at the server, and SSH/web/MCP hand ze a cleartext password at login time, so it may fit — but this must be checked against the actual `aaa.AuthRequest` contract, not assumed | Open |
| O-2 | EAP is multi-round-trip; `aaa.Authenticator` is a single `Authenticate(request) (result, error)` call (`authenticator.go`). Does the interface admit an EAP conversation, or does EAP need a different seam? **This is the likely blocker and should be answered before anything else.** | Open |
| O-3 | Should admin accounting reuse the L2TP RADIUS accounting client, as auth already reuses the L2TP RADIUS client, or is an operator session too different from a subscriber session? | Open |
| O-4 | Is there a real operator asking for this? If not, the spec stays skeleton. | Open |

## Required Reading

### Architecture Docs
- [ ] `docs/guide/radius.md` - the admin RADIUS guide as shipped
  → Constraint: the guide documents PAP-only; any change here changes the guide.
- [ ] `docs/features.md` - the RADIUS admin AAA row
  → Decision: `Partial` is deliberate, not a defect. The row already names CHAP/EAP/accounting as follow-up work, which is why this is a skeleton and not a fixit.
- [ ] `ai/rules/config.md` - YANG vs env var for any new method selector
  → Constraint: an auth-method selector is operator-facing config, so YANG, not env var.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc2865.md` - Access-Request/Accept/Reject; User-Password (5.2), CHAP-Password (5.3), CHAP-Challenge (5.40)
  → Constraint: today only Section 5.2 User-Password is implemented (`authenticator.go`).
- [ ] `rfc/short/rfc2866.md` - Accounting-Request Start/Stop (summary exists, verified 2026-07-16)
  → Constraint: admin-session accounting means Start on login and Stop on logout, which needs a logout signal the auth path does not have.
- [ ] RFC 3579 (RADIUS support for EAP; EAP-Message, Message-Authenticator) - **no summary exists under `rfc/short/` (verified 2026-07-16). Run `/ze-rfc` to create it BEFORE any EAP work**, per `ai/rules/planning.md` Pre-Spec Verification.
  → Constraint: RFC 3579 requires Message-Authenticator on every EAP-bearing packet.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- PAP-only is intended and documented, not an oversight. This spec is demand-driven.
- The single-shot `aaa.Authenticator` interface is the structural obstacle for EAP (O-2); CHAP and accounting are additive by comparison.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/radius/authenticator.go` - `radiusAuthenticator.Authenticate`, doc at `:85-86` states PAP. Builds an Access-Request with `AttrUserName`, `AttrUserPassword`, `AttrServiceType` (`serviceTypeLogin`), `AttrNASIdentifier`, plus `AttrNASIPAddress` when an IPv4 source exists. Sends via `a.client.SendToServers` under an `authBudget` timeout. Switches on `resp.Code` at `:132`; `mapProfiles` at `:187`. No CHAP-Password, no EAP-Message, no Accounting-Request
- [ ] `internal/component/radius/aaa.go` - `radiusBackend.Build`, registers into the `aaa` chain at priority 50
- [ ] `internal/component/radius/config.go` - `ExtractConfig`; `default-profile` GetSlice at `:103`
- [ ] `internal/component/radius/yang/ze-radius-conf.yang` - `system.authentication.radius` config surface; no auth-method leaf today

**Behavior to preserve:** (unless user explicitly said to change)
- PAP remains the default. An existing PAP deployment must keep working with no config change.
- The password is passed cleartext into the packet and XOR-hidden per-server inside `Exchange` (RFC 2865 Section 5.2), per the comment at `:102-105`. It never reaches the wire in the clear.
- The explicit-reject vs unreachable distinction (`:126-130` returns a plain error on infra failure so the chain falls back to local bcrypt; Access-Reject returns `ErrAuthRejected` so the chain STOPS). This is load-bearing: a wrong password must not fall through to local bcrypt, an unreachable server must.
- An Access-Accept resolving to no profile is rejected, not authorized (`:133-149` and the reasoning recorded there). Any new method MUST route through the same profile check, not around it.
- Chain priority 50 (ahead of TACACS+ 100, local 200).
- `authBudget` clamping.

**Behavior to change:** (only if user explicitly requested)
- None yet. This spec is a placeholder until an operator needs CHAP or EAP (O-4).

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An operator login attempt over SSH, the web UI, or MCP, arriving as an `aaa.AuthRequest` carrying a username and a cleartext password.

### Transformation Path
1. The `aaa` chain calls backends in priority order; the RADIUS backend is registered at priority 50 by `radiusBackend.Build` (`internal/component/radius/aaa.go`)
2. `radiusAuthenticator.Authenticate` (`authenticator.go`) generates a random Request Authenticator
3. It builds an Access-Request carrying User-Name and **User-Password** (`:106-111`) — the PAP-only step this spec would extend
4. `Client.SendToServers` XOR-hides the password per-server per RFC 2865 Section 5.2 and performs ordered failover with per-server timeout and retries
5. On Access-Accept, `mapProfiles` maps the configured reply attribute (default Filter-Id, or Class) to profile names; an empty result is a rejection
6. On Access-Reject, `ErrAuthRejected` stops the chain; on infra failure, a plain error lets it fall through to local bcrypt

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Login surface ↔ aaa chain | `aaa.AuthRequest` / `aaa.AuthResult`, single-shot — the EAP obstacle (O-2) | [ ] |
| aaa ↔ RADIUS client | `Client.SendToServers`, shared with the L2TP subscriber path | [ ] |
| RADIUS ↔ authz | profile names via `mapProfiles` → `aaa.RecordLoginProfiles` → `authz.Store` | [ ] |

### Integration Points
- `aaa.Default.Register` - the registration seam; a new method must not add a switch case to the chain.
- The L2TP RADIUS client (reused today for retransmit/failover/Response-Authenticator verification) - the natural home for CHAP/EAP attribute encoding, and possibly for accounting (O-3).
- `mapProfiles` - every method must route through it, so the no-profile rejection cannot be bypassed.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — a new auth method registers rather than adding a per-method switch case to the aaa chain or a shared core struct (`ai/rules/plugins.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PAP-only is intended, not an oversight | `docs/features.md` states "CHAP/EAP admin auth and admin-session accounting are follow-up work"; the row is marked `Partial` | This is a defect spec, not a follow-up spec, and priority rises | Read `docs/features.md` | unvalidated |
| A-2 | `aaa.AuthRequest` carries a cleartext password, so CHAP can compute its response locally | `authenticator.go` puts `request.Password` straight into the packet | CHAP does not fit this seam at all (O-1) | Read the `aaa.AuthRequest` definition | unvalidated |
| A-3 | The single-shot `Authenticate` signature cannot carry an EAP conversation | `authenticator.go` returns a final result in one call | EAP is additive and this spec is smaller than feared (O-2) | Read the `aaa.Authenticator` interface | unvalidated |
| A-4 | No operator needs this today | Nothing in the deferrals row or `docs/features.md` names a requester | Priority rises and the spec leaves skeleton | Ask Thomas (O-4) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A new method bypasses the no-profile rejection and re-opens the admin escalation that rule closed | An Accept with no Filter-Id yields a working admin login | Route every method through `mapProfiles`; test the empty-profile case per method |
| R-2 | Adding EAP forces a change to the shared `aaa.Authenticator` interface, hitting TACACS+ and local backends | The design starts editing `aaa` core types | Treat an interface change as its own spec with its own approval |
| R-3 | Speculative implementation with no operator behind it violates "no speculative features" | Nobody can name the deployment | Keep this spec `skeleton` until O-4 is answered yes |
| R-4 | CHAP requires the RADIUS server to hold a reversible password, which weakens the deployment it is meant to strengthen | Operators asked for CHAP believing it is strictly safer than PAP | Document the trade-off in `docs/guide/radius.md`: PAP over a shared-secret-hidden transport vs CHAP needing reversible storage |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Operator logs in over SSH with the RADIUS auth method set to chap | → | `radiusAuthenticator.Authenticate` builds a CHAP-Password Access-Request | `test/plugin/radius-admin-chap.ci` |
| Operator session starts and ends | → | the admin accounting Start/Stop emitter | `test/plugin/radius-admin-accounting.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Auth method configured as CHAP; operator logs in with a correct password | Access-Request carries CHAP-Password (RFC 2865 Section 5.3) and CHAP-Challenge (Section 5.40), not User-Password; login succeeds |
| AC-2 | Auth method configured as CHAP; wrong password | Access-Reject → `ErrAuthRejected` → chain STOPS, no local bcrypt fallthrough (preserved behavior) |
| AC-3 | Any new method; Access-Accept resolving to no profile | Rejected, exactly as PAP is today — the no-profile rejection is not bypassed |
| AC-4 | No auth method configured | PAP, unchanged. An existing deployment needs no config edit |
| AC-5 | Operator session starts, then ends | Accounting-Request Start then Stop, with the session correlated between them |
| AC-6 | RADIUS servers unreachable, any method | Plain error → chain falls through to local bcrypt (preserved behavior; the operator is not locked out) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator logs into the CLI over SSH against a CHAP-configured RADIUS server | SSH → aaa chain → radius backend (priority 50) → CHAP Access-Request → Accept → mapProfiles → authz | `test/plugin/radius-admin-chap.ci` |
| 2 | Auditor reads RADIUS accounting and sees who logged into the router and when | login → Accounting-Request Start; logout → Stop | `test/plugin/radius-admin-accounting.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthenticateCHAPRequestShape` | `internal/component/radius/authenticator_test.go` | AC-1: CHAP-Password + CHAP-Challenge present, User-Password absent | |
| `TestAuthenticateCHAPRejectStopsChain` | `internal/component/radius/authenticator_test.go` | AC-2: `ErrAuthRejected` on Access-Reject | |
| `TestAuthenticateAnyMethodEmptyProfileRejected` | `internal/component/radius/authenticator_test.go` | AC-3: the no-profile rejection holds per method | |
| `TestAuthenticateDefaultsToPAP` | `internal/component/radius/authenticator_test.go` | AC-4: unconfigured stays PAP | |
| `TestAdminAccountingStartStop` | `internal/component/radius/accounting_test.go` | AC-5: Start/Stop correlation | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| CHAP Ident (RFC 2865 Section 5.3, first octet of CHAP-Password) | 0-255 | 255 | N/A | N/A (single octet) |
| CHAP-Password total length | 17 octets fixed (1 Ident + 16 response) | 17 | 16 | 18 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `radius-admin-chap` | `test/plugin/radius-admin-chap.ci` | Operator logs in via CHAP against a test RADIUS server | |
| `radius-admin-accounting` | `test/plugin/radius-admin-accounting.ci` | Operator login/logout produces Start/Stop | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-radius-admin-chap` | `test/interop/scenarios/` | FreeRADIUS | A real server accepts ze's CHAP Access-Request | |

### Future (if deferring any tests)
- EAP tests are blocked on O-2 (whether the `aaa.Authenticator` seam admits a multi-round-trip conversation at all). Do not plan EAP tests before that is answered.

## Files to Modify
- `internal/component/radius/authenticator.go` - `Authenticate` gains method selection; the Access-Request build at `:106-111` gains CHAP/EAP attributes
- `internal/component/radius/config.go` - `ExtractConfig` reads the new method leaf
- `internal/component/radius/yang/ze-radius-conf.yang` - an auth-method leaf under `system.authentication.radius`, with an `enumeration` (`ai/patterns/config-option.md`: maximum native validation)
- `internal/component/radius/aaa.go` - only if the backend registration shape must change
- `docs/features.md` - `:87` RADIUS admin AAA row, which currently names this work as follow-up
- `docs/guide/radius.md` - the method selector and the CHAP reversible-storage trade-off (R-4)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | `internal/component/radius/yang/ze-radius-conf.yang` — read `ai/rules/config.md` and `ai/rules/config.md` |
| YANG validation constraints | [ ] | The method leaf MUST be an `enumeration`, not a bare `type string` |
| Editor autocomplete | [ ] | Automatic for a YANG enum leaf |
| Functional test for new RPC/API | [ ] | `test/plugin/radius-admin-chap.ci` |
| Doctor check for runtime dependencies | [ ] | `doctor-radius-admin-unreachable` exists (`internal/component/radius/doctor.go`); check whether a method mismatch deserves its own diagnostic |
| Prometheus counters/metrics | [ ] | `ze_radius_*` metrics exist for the L2TP path; decide whether admin auth/accounting needs its own counters |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/radius.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | `rfc/short/rfc2865.md`, `docs/features/rfc-status.md` |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | `docs/features.md` already anchors `internal/component/radius/authenticator.go -- radiusAuthenticator.Authenticate, mapProfiles` |

## Files to Create
- `internal/component/radius/accounting.go` - admin-session accounting emitter (point 3), if O-3 says not to reuse the L2TP client
- `test/plugin/radius-admin-chap.ci`
- `test/plugin/radius-admin-accounting.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify; answer O-1..O-4 first |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Answer the open questions** — O-4 first (is anyone asking?), then O-2 (does the seam admit EAP?)
   - Tests: none
   - Files: this spec
   - Verify: if O-4 is no, the spec stays `skeleton` and work stops here. That is the expected outcome today
2. **Phase: Wiring (MANDATORY FIRST)** — YANG method leaf + failing wiring test
   - Tests: `TestAuthenticateDefaultsToPAP`
   - Files: `yang/ze-radius-conf.yang`, `config.go`
   - Verify: the leaf is settable and defaults to PAP
3. **Phase: CHAP** — RFC 2865 Section 5.3 / 5.40 attributes
   - Tests: `TestAuthenticateCHAPRequestShape`, `TestAuthenticateCHAPRejectStopsChain`, `TestAuthenticateAnyMethodEmptyProfileRejected`
   - Files: `authenticator.go`
   - Verify: red → implement → green; PAP tests stay green
4. **Phase: Admin accounting** — RFC 2866 Start/Stop
   - Tests: `TestAdminAccountingStartStop`
   - Files: `accounting.go` or the reused L2TP client
   - Verify: Stop needs a logout signal; confirm one exists before starting
5. **Phase: EAP** — only if O-2 says the seam admits it
   - Tests: (fill after O-2)
   - Files: (fill after O-2)
   - Verify: (fill after O-2)
6. **Functional tests** → the two `.ci`
7. **RFC refs** → `// RFC 2865 Section 5.3` / `// RFC 2866` / `// RFC 3579` comments
8. **Full verification** → `make ze-verify`
9. **Complete spec** → learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The reject-vs-unreachable distinction survives every method (AC-2, AC-6) |
| Security | No method logs or leaks the password; CHAP-Challenge is random per request |
| Naming | YANG uses kebab-case (`ai/rules/config.md`) |
| YANG validation | The method leaf is an `enumeration`, not a bare `type string` |
| Registration over hardcoding | Methods do not become a switch case in an aaa core struct (`ai/rules/plugins.md`) |
| Rule: no-partial-completion | If EAP is dropped for O-2, that is a scope change needing user approval, not a silent omission |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| PAP still default | `TestAuthenticateDefaultsToPAP` passes |
| `docs/features.md` no longer calls this follow-up work | `grep -n "follow-up work" docs/features.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Server-supplied EAP-Message length and Message-Authenticator must be validated before use (RFC 3579) |
| Credential handling | CHAP-Challenge must come from a CSPRNG, like the Request Authenticator does today (`authenticator.go`) |
| Privilege escalation | AC-3: no method may return success with an empty profile set. This is the escalation `:133-149` was written to close |
| Error leakage | Reject vs unreachable must stay distinguishable to ze and indistinguishable to the attacker |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails behavior mismatch | Re-read source from Current Behavior |
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

- A documented `Partial` with the gap named in the feature row is the healthy shape: the limitation is discoverable by an operator before deployment, not after. This spec exists to give the follow-up a home, not to fix a bug.

## Core Insight
(fill during design)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- PAP-only, by design and documented at `docs/features.md`. PAP over RADIUS hides the password with the shared secret per RFC 2865 Section 5.2 rather than leaving it cleartext on the wire, so PAP-only is a real limitation but not an unguarded one.
- No admin-session accounting: RADIUS records no operator login/logout for ze today.

## RFC Documentation

Add `// RFC 2865 Section 5.3: "<quoted requirement>"` (CHAP-Password), `// RFC 2865 Section 5.40` (CHAP-Challenge), `// RFC 3579` (EAP-Message, Message-Authenticator), `// RFC 2866` (Accounting Start/Stop) above enforcing code.
MUST document: validation rules, error conditions, state transitions, any MUST/MUST NOT.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| CHAP admin login works against a real server | interop test | (fill during implementation) |
| Admin sessions are accounted | functional test | (fill during implementation) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (fill during implementation) | file:line | (fill during implementation) |

### Fixes applied
- (fill during implementation)

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

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
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
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md`
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-radius-chap-eap-admin.md` only
