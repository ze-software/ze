# Spec: mgmt-version-header-suppress

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

Anchor refresh (2026-07-22 plan review, design unchanged and implementable;
citations below updated in-body): `addSecurityHeaders` now `auth.go`, the
`X-Ze-Version` header write `:361`. The lg cites (`server.go`,
`version.go` `HTTPHeader`) are still exact.

**Notes:** Promoted to ready per user instruction 2026-07-10 (followup-wave impact review session) authorizing conversion to ready.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/web/auth.go` - `addSecurityHeaders`
4. `internal/component/lg/server.go` - the looking-glass header site
5. `internal/core/version/version.go` - `HTTPHeader`

## Task

Ze's web management server and looking-glass server stamp every authenticated
response with a custom `X-Ze-Version` header carrying the full version banner:
release, git commit (with a modified marker), Go version, and OS/arch (for
example `ze/26.04.05 (ac8f5391; go1.26; darwin/arm64)`). This is emitted
unconditionally with no way to turn it off. Leaking a precise build fingerprint
to any client is a minor hardening weakness: it lets an attacker fingerprint the
exact build to target known issues.

Add a management-hardening toggle that suppresses the `X-Ze-Version` header. Both
emitting sites (web + looking-glass) honour it, so a single setting removes the
banner everywhere. The default preserves today's behaviour (header present) so
existing setups are unchanged; operators who want to hide the banner opt in.

Note this is specifically the custom `X-Ze-Version` header. Ze sends no standard
`Server` header (Go's net/http omits it and Ze never sets one), so there is no
`Server` banner to suppress.

## Required Reading

### Architecture Docs
- [ ] `internal/component/web/auth.go` - `addSecurityHeaders` sets the version header alongside the other security headers.
  -> Constraint: gate only the version header; leave the other security headers (frame-options, CSP, HSTS, no-store) untouched.
- [ ] `internal/component/lg/server.go` - the looking-glass sets the same header independently.
  -> Constraint: both sites must consult the same toggle so suppression is complete.
- [ ] `ai/rules/config.md`, `ai/rules/config.md` - YANG vs env var, kebab-case.
  -> Decision: a single kebab-case boolean, resolved from config and consulted at both header sites.

**Key insights:**
- The value is removing a build fingerprint; the change is a guard around two header writes plus one config leaf.
- Keeping the default as-is avoids surprising existing deployments; the hardening is opt-in.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/auth.go` - `addSecurityHeaders` (auth.go) sets `X-Ze-Version` to `version.HTTPHeader()` (auth.go), unconditionally, on authenticated responses.
- [ ] `internal/component/lg/server.go` - sets the same `X-Ze-Version` header (server.go), independently of the web path.
- [ ] `internal/core/version/version.go` - `HTTPHeader` (version.go) builds `ze/<release> (<commit>[+]; <goVer>; <os>/<arch>)`; there is no caller-side option to omit it.

### Post-wave corrections (2026-07-10)

All refs re-verified against current code:

- Line drift corrected in place above (old -> new): `addSecurityHeaders`
  auth.go -> :338 (doc comment :337), `X-Ze-Version` write :316 -> :344;
  lg header write server.go -> :568; `HTTPHeader` version.go ->
  :218-235. Behaviour of all three sites is exactly as described.
- Precision on the lg site: the write at server.go lives inside the
  `securityHeaders` middleware (server.go) which wraps ALL lg
  responses, not only authenticated ones. The suppression guard therefore goes
  in that middleware; "authenticated responses" applies to the web site only.
- Config surface candidates made concrete: the web schema is
  `internal/component/web/yang/ze-web-conf.yang` and the looking-glass schema
  is `internal/component/lg/yang/ze-lg-conf.yang`. Whether the single toggle
  is one shared leaf both servers resolve, or a leaf mirrored into both
  containers, is settled at implement time under R-1's mitigation (ONE logical
  toggle; a functional test hits both servers).
- Unit-test anchors exist: `internal/component/web/auth_test.go`,
  `internal/component/web/integration_test.go`, and
  `internal/component/lg/server_test.go` already reference `X-Ze-Version`, so
  the planned unit tests extend existing files as stated.
- Functional test location corrected everywhere in this spec: `test/ci/` does
  not exist. The test lives at `test/plugin/version-header-suppress.ci`
  (test/plugin already hosts daemon-booting HTTP-surface tests, e.g.
  lg-paginate.ci).

**Behavior to preserve:**
- When suppression is off (the default), the `X-Ze-Version` header is emitted exactly as today.
- The other security headers in `addSecurityHeaders` are unchanged.
- No standard `Server` header is introduced.
- The version string itself and its non-header uses (for example the outbound self-update User-Agent) are unchanged.

**Behavior to change:**
- When suppression is enabled, neither the web server nor the looking-glass emits `X-Ze-Version`.

## Data Flow (MANDATORY)

### Entry Point
- Config: a management-hardening boolean (for example `hide-version`) resolved into the web and looking-glass server settings.

### Transformation Path
1. Config resolve produces the toggle value for the servers.
2. On each authenticated response, `addSecurityHeaders` checks the toggle before setting `X-Ze-Version`.
3. The looking-glass response builder makes the same check before its header write.
4. When the toggle is on, the header is omitted; all other headers and body are unchanged.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree <-> server settings | the toggle resolved into web + lg config | [ ] |
| Settings <-> response headers | the header write is guarded by the toggle | [ ] |
| Version producer <-> header | `HTTPHeader` called only when not suppressed | [ ] |

### Integration Points
- Config surface - the `hide-version` leaf (web/service level or a shared management-hardening container).
- `internal/component/web/auth.go` - guard the header write.
- `internal/component/lg/server.go` - guard the header write.

### Architectural Verification
- [ ] No bypassed layers (both header writers consult the resolved toggle)
- [ ] No unintended coupling (only the version header is gated; other headers untouched)
- [ ] No duplicated functionality (one toggle, consulted at both sites, not two independent flags)
- [ ] Registration over hardcoding - the behaviour is config-driven, not a compile-time constant.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Both header sites can read a resolved config toggle | auth.go, server.go (refs refreshed 2026-07-22); note `addSecurityHeaders` takes only the ResponseWriter, so the toggle reaches it via its callers or a package-level setting | plumb the toggle into the relevant struct | trace both servers' config during audit | unvalidated |
| A-2 | `X-Ze-Version` is the only response-header version leak | grep found only these two sites; no `Server` header | another surface leaks the banner | grep all response-header writers | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | One site suppressed but the other still leaks | header still present on one server | one shared toggle, functional test hits both servers |
| R-2 | Suppressing breaks a client that parses the header | a tool depends on `X-Ze-Version` | default off; opt-in only |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `hide-version true`, request the web UI | -> | response omits `X-Ze-Version` | `TestWebVersionHeaderSuppressed` |
| `hide-version true`, request the looking-glass | -> | response omits `X-Ze-Version` | `TestLGVersionHeaderSuppressed` |
| default (off), request either server | -> | `X-Ze-Version` present as today | `TestVersionHeaderPresentByDefault` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `hide-version true`, web response | no `X-Ze-Version` header |
| AC-2 | `hide-version true`, looking-glass response | no `X-Ze-Version` header |
| AC-3 | default (unset/false), web response | `X-Ze-Version` present, value unchanged |
| AC-4 | suppression on | the other security headers are still present |
| AC-5 | any config | no standard `Server` header appears |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | hardens the box by hiding the build banner | config `hide-version` -> both servers omit `X-Ze-Version` | `test/plugin/version-header-suppress.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWebVersionHeaderSuppressed` | `internal/component/web/auth_test.go` | web omits the header when suppressed | |
| `TestLGVersionHeaderSuppressed` | `internal/component/lg/server_test.go` | looking-glass omits the header when suppressed | |
| `TestVersionHeaderPresentByDefault` | `internal/component/web/auth_test.go` | default behaviour unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hide-version | boolean | true/false | n/a | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `version-header-suppress` | `test/plugin/version-header-suppress.ci` | header present by default, absent when hidden, on both servers | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local HTTP-header hardening; no protocol peer | - | - | not a protocol feature | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- Config surface (web/service YANG) - add the `hide-version` leaf
- `internal/component/web/auth.go` - guard the `X-Ze-Version` write
- `internal/component/lg/server.go` - guard the `X-Ze-Version` write

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/plugin/version-header-suppress.ci` - functional test
- (unit tests in existing `_test.go`)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** - add the `hide-version` leaf (unused) + a failing `test/plugin/version-header-suppress.ci`.
2. **Phase: Toggle plumbing** - resolve the leaf into the web + lg server settings.
3. **Phase: Guard the header writes** - gate both `X-Ze-Version` sites on the toggle.
   - Tests: `TestWebVersionHeaderSuppressed`, `TestLGVersionHeaderSuppressed`, `TestVersionHeaderPresentByDefault`
4. **Functional** - both servers, default and suppressed.
5. **Full verification** -> `make ze-verify`
6. **Complete spec** -> audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | only the version header gated; other headers intact; default preserved |
| Both paths | web and looking-glass both honour the toggle |
| Registration over hardcoding | one config-driven toggle, no compile-time constant |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
- (fill during implementation)

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
- [ ] AC-1..AC-5 demonstrated
- [ ] End-to-End User Stories: working path + passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
