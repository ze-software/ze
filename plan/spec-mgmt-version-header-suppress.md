# Spec: mgmt-version-header-suppress

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-09-05 |

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
- [ ] `docs/architecture/web-interface.md` - the HTTPS server rendering YANG-driven config views with HTMX components
- [ ] `internal/component/web/auth.go` - `addSecurityHeaders` sets the version header alongside the other security headers.
  -> Constraint: gate only the version header; leave the other security headers (frame-options, CSP, HSTS, no-store) untouched.
- [ ] `internal/component/lg/server.go` - the looking-glass sets the same header independently.
  -> Constraint: both sites must consult the same toggle so suppression is complete.
- [ ] `docs/architecture/config/syntax.md` - the design document `internal/component/config/apply_env.go` declares, which is where the `environment` block's syntax is stated.
  -> Constraint: the page defers the block's leaf list to `docs/architecture/config/environment-block.md`, so the new leaf is documented there and `syntax.md` needs no edit.
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
| A-1 | Both header sites can read a resolved config toggle | auth.go, server.go (refs refreshed 2026-07-22); note `addSecurityHeaders` takes only the ResponseWriter, so the toggle reaches it via its callers or a package-level setting | plumb the toggle into the relevant struct | trace both servers' config during audit | confirmed -- both packages already import `internal/core/version`, so the predicate `version.HTTPHeaderHidden` reaches both sites with no signature change |
| A-2 | `X-Ze-Version` is the only response-header version leak | grep found only these two sites; no `Server` header | another surface leaks the banner | grep all response-header writers | confirmed at closure -- `version.HTTPHeader()` has five non-test callers: the two response-header writers (`web/auth.go`, `lg/server.go`) and three outbound User-Agent writers (`config/system/selfupdate.go` twice, `config/system/update.go`), none of which is a response header. `X-Ze-Version` is written at those two sites alone, and no `Header().Set("Server"` exists anywhere. The implementation session's cell named three callers, having missed the two in `selfupdate.go`; the conclusion is unchanged |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | One site suppressed but the other still leaks | header still present on one server | one shared toggle, functional test hits both servers |
| R-2 | Suppressing breaks a client that parses the header | a tool depends on `X-Ze-Version` | default off; opt-in only |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `hide-version true`, request the web UI | -> | `addSecurityHeaders` (`internal/component/web/auth.go`) returns before the header write | `TestWebVersionHeaderSuppressed` (`internal/component/web/auth_test.go`) |
| `hide-version true`, request the looking-glass | -> | `securityHeaders` (`internal/component/lg/server.go`) skips the header write | `TestLGVersionHeaderSuppressed` (`internal/component/lg/server_test.go`) |
| default (off), request either server | -> | `X-Ze-Version` present as today | `TestVersionHeaderPresentByDefault` (web), `TestLGVersionHeaderPresentByDefault` (lg) |
| `environment { hide-version true; }` in a config file | -> | `ApplyEnvConfig` sets `ze.hide-version`, `version.HTTPHeaderHidden` answers true | `TestApplyEnvConfigHideVersion` (`internal/component/config/apply_env_test.go`) |

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
| `TestWebVersionHeaderSuppressed` | `internal/component/web/auth_test.go` | web omits the header when suppressed, and keeps the other four | PASS |
| `TestLGVersionHeaderSuppressed` | `internal/component/lg/server_test.go` | looking-glass omits the header when suppressed, and keeps the other four | PASS |
| `TestVersionHeaderPresentByDefault` | `internal/component/web/auth_test.go` | default behaviour unchanged | PASS |
| `TestLGVersionHeaderPresentByDefault` | `internal/component/lg/server_test.go` | default behaviour unchanged on the looking glass | PASS |
| `TestHTTPHeaderHidden` | `internal/core/version/version_test.go` | the toggle reads the env key, and unset means false | PASS |
| `TestApplyEnvConfigHideVersion` | `internal/component/config/apply_env_test.go` | the YANG leaf reaches the env key and the predicate | PASS |
| `TestApplyEnvConfigHideVersionAbsent` | `internal/component/config/apply_env_test.go` | a config that writes no leaf leaves the banner on | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hide-version | boolean | true/false | n/a | n/a |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `version-header-suppress` | `test/plugin/version-header-suppress.ci` | header present by default, absent when hidden, on both servers of one daemon | PASS (`ze-test bgp plugin --pattern version-header-suppress`, 6.3s) |
| `environment-hide-version` | `test/parse/environment-hide-version.ci` | `environment { hide-version true; }` validates | PASS (`ze-test bgp parse --pattern environment-hide-version`, 575ms) |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - local HTTP-header hardening; no protocol peer | - | - | not a protocol feature | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/hub/yang/ze-hub-conf.yang` - the `hide-version` leaf, a top-level leaf under `environment` beside `pprof`, because the toggle is daemon-wide rather than owned by either server
- `internal/core/version/version.go` - `EnvKeyHideVersion`, its env registration, and `HTTPHeaderHidden`
- `internal/component/config/apply_env.go` - the `envPlumbingTable` row that carries the leaf into the env key
- `internal/component/web/auth.go` - guard the `X-Ze-Version` write in `addSecurityHeaders`
- `internal/component/lg/server.go` - guard the `X-Ze-Version` write in `securityHeaders`
- `internal/test/fixture/plugin_fixture_09_shell.go` - `waitForLog09` named the looking glass in an error it now answers for any needle

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] yes | `docs/guide/configuration.md` |

## Files to Create
- `test/plugin/version-header-suppress.ci` - functional test (drafted at `test/draft/plugin/`)
- `test/parse/environment-hide-version.ci` - the leaf parses (drafted at `test/draft/parse/`)
- `internal/test/fixture/plugin_fixture_version_header_suppress.go` - the scenario body
- `internal/test/fixture/register_version_header_suppress.go` - its registration
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
5. **Full verification** -> `./le verify current mode full`
6. **Complete spec** -> audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | only the version header gated; other headers intact; default preserved |
| Both paths | web and looking-glass both honour the toggle |
| Registration over hardcoding | one config-driven toggle, no compile-time constant |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-2's evidence cell said `version.HTTPHeader` has three non-test callers | It has five: two response-header writers and three outbound User-Agent writers, the two in `internal/component/config/system/selfupdate.go` having been missed | `git grep -n "version.HTTPHeader()"` at closure | A-2's cell corrected in place. The conclusion it supports, that `X-Ze-Version` is written at two sites only, is unchanged |

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| `gopls references` on `version.HTTPHeader` had enumerated every caller | The count recorded in A-2 was two short | re-run at closure | none on the design: both missed callers write an outbound `User-Agent`, not a response header |

## Design Insights
<!-- LIVE -->

- **The toggle is daemon-wide, not per-server.** `hide-version` is a top-level
  leaf under `environment` in `ze-hub-conf.yang`, beside `pprof`, rather than a
  leaf mirrored into `environment/web` and `environment/looking-glass`. R-1 asks
  for ONE logical toggle, and one leaf reaching one env key read by one
  predicate makes that structural: there is no second value for the two servers
  to disagree about.
- **The env key constant is exported and the plumbing table NAMES it.**
  `env.Set` ends the process on an unregistered key, so a row in
  `envPlumbingTable` whose key no linked package registered is a startup crash
  the first time an operator writes the leaf. `internal/component/config` now
  imports `internal/core/version` for `version.EnvKeyHideVersion`, so the import
  that carries the spelling carries the registration with it. The registration
  sits beside the reader, as `internal/core/privilege` does for `ze.user`.
- **`HTTPHeader` still always answers the banner.** Returning an empty string
  when the operator hid it would be a value no caller can tell from a build that
  carries no version, so the decision is a separate named predicate the two
  writers consult (`ai/rules/principles.md`, "a zero value is never an answer").
- **The `.ci` runner asserts REQUEST headers only.** `httpCheck`
  (`internal/test/runner/record.go`) carries `Headers` for the request and
  asserts status, a body substring and a body file on the response; it has no
  response-header assertion. So the functional test drives a compiled fixture
  (`ze-test fixture plugin/version-header-suppress`) that reads the header set
  in Go, which is the shape `lg-tls-default-on.ci` already uses for a two-config
  daemon comparison. Extending the runner was considered and rejected: the
  fixture route needs no new directive, no new documentation and no new parser.
- **Both `.ci` files RAN and passed, and both are promoted.** The daemon flavor
  of the tree compiles only in windows: through this session, other sessions'
  in-flight edits broke `internal/component/config/validators.go`, then
  `internal/component/l2tp/ppp/ra_send.go` and
  `internal/component/bgp/reactor/reactor_api_batch.go`, then
  `internal/component/bgp/plugins/cmd/commit/commit.go`, then
  `internal/exabgp/bridge/bridge_attribute.go`, then an untracked
  `internal/component/ike/ipsec/spd_policy.go` declaring a second package name in
  that directory. None of them is this spec's code. Both tests were run in a
  window where the tree did build, and both passed.
- **The functional test's RED half is observed, through the daemon.** With both
  guards removed and `ze` rebuilt, `version-header-suppress.ci` failed on the
  assertion the feature exists for: `ZE-OBSERVER-FAIL: the hardened web server
  sent X-Ze-Version: "ze/26.09.06 (3522fc9db485+; go1.27.0; linux/amd64)", want no
  banner`. The `+` on that commit id is the rebuilt binary carrying the mutation,
  so the run is not a cached verdict. The default half stayed green in the same
  run (`OK: both servers send the version banner by default`), so the test
  discriminates on the guard alone rather than on the daemon starting. Restoring
  both guards returned it to PASS in 15.1s. The unit pair shows the same break:
  `TestWebVersionHeaderSuppressed` fails with `Should be empty, but was ze/dev
  (go1.27.0; linux/amd64)` and `TestLGVersionHeaderSuppressed` beside it.

## Implementation Summary
### What Was Implemented
- One YANG leaf, `environment/hide-version` (boolean, default false), in
  `internal/component/hub/yang/ze-hub-conf.yang`.
- `version.EnvKeyHideVersion` and `version.HTTPHeaderHidden`
  (`internal/core/version/version.go`), plus the `env.MustRegister` for the key.
- The `envPlumbingTable` row that carries the leaf to the key
  (`internal/component/config/apply_env.go`).
- A guard at each of the two header writers: `addSecurityHeaders`
  (`internal/component/web/auth.go`) and `securityHeaders`
  (`internal/component/lg/server.go`). Neither touches any other header.
- Seven unit tests over the three boundaries, and two `.ci` files with the
  compiled fixture behind the first.

### Acceptance Criteria Evidence
| AC | Producing code | Evidence |
|----|----------------|----------|
| AC-1 | `addSecurityHeaders`, `internal/component/web/auth.go` | `TestWebVersionHeaderSuppressed` PASS; `version-header-suppress.ci` PASS ("OK: hide-version takes the banner off both servers") |
| AC-2 | `securityHeaders`, `internal/component/lg/server.go` | `TestLGVersionHeaderSuppressed` PASS; the same `.ci` line |
| AC-3 | the `false` default of `HTTPHeaderHidden`, `internal/core/version/version.go` | `TestVersionHeaderPresentByDefault`, `TestLGVersionHeaderPresentByDefault`, `TestApplyEnvConfigHideVersionAbsent` PASS; `.ci` line "OK: both servers send the version banner by default" |
| AC-4 | both guards touch the version header alone | `TestWebVersionHeaderSuppressed` and `TestLGVersionHeaderSuppressed` assert each surviving header by value; `.ci` line "OK: the other security headers survive suppression" |
| AC-5 | no `Server` header is written anywhere | `gopls references` finds no `Header().Set("Server"`; both unit tests and the `.ci` line "OK: neither server sends a Server banner" assert its absence |

### Bugs Found/Fixed
- `waitForLog09` (`internal/test/fixture/plugin_fixture_09_shell.go`) answered
  "looking glass never announced a listener" for any needle it timed out on. The
  new fixture waits on the web banner first, so that message named the wrong
  server. The error now quotes the needle and the log path.

### Documentation Updates
- `docs/features.md` and `docs/features/web-interface.md` -- the toggle on the
  web interface row, with the anchor
  `<!-- source: internal/core/version/version.go -- HTTPHeaderHidden -->`.
- `docs/guide/configuration.md` -- the `hide-version` leaf in the `environment`
  block, landed by `170e0139ea`.
- `docs/guide/environment-variables.md` -- the `ze.hide-version` row and the
  config form that sets it.
- `docs/guide/web-interface.md` and `docs/guide/looking-glass.md` -- what the
  banner carries and how to keep it off each server.
- `docs/architecture/config/environment-block.md` and
  `docs/architecture/config/environment.md` -- the leaf, its env key and its
  default in the two tables that enumerate the block.
- `docs/architecture/web-interface.md` -- the security-header section.
- `docs/architecture/config/syntax.md` needs no edit: it states the block's
  syntax and defers the leaf list to `environment-block.md`, which carries the
  new row.

### Deviations from Plan
- The spec left open whether the toggle is one shared leaf or a leaf mirrored
  into the web and looking-glass containers. It is one top-level leaf under
  `environment`, beside `pprof`, so the two servers have no second value to
  disagree about (Design Insights, first bullet).
- The plan named `internal/component/web/integration_test.go` as an existing
  anchor. The new web tests went in `auth_test.go` beside `addSecurityHeaders`
  instead, and `integration_test.go` is unchanged.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/mgmt-version-header-suppress-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md` (14 files, verdict=clean, model=claude-opus-5) |
| `review check` | OK: 6 code files, clean, hashes match |
| Rounds | 1 |
| Reviewer lenses used | wiring + logic, security + edge cases, style + simplicity |

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | `addSecurityHeaders` returns early when the banner is hidden, so a header appended after that point would also disappear. `securityHeaders` in the looking glass wraps the write instead, which has no such edge | `internal/component/web/auth.go` `addSecurityHeaders` | acknowledged: the doc comment above the guard states the intent, and the version write is the last statement today |
| 2 | NOTE | The two header tests spell `"ze.hide-version"` as a literal where `version.EnvKeyHideVersion` exists | `internal/component/web/auth_test.go`, `internal/component/lg/server_test.go` | acknowledged: a rename turns `TestWebVersionHeaderSuppressed` and `TestLGVersionHeaderSuppressed` red rather than green, so the copy cannot pass silently |
| 3 | NOTE | An unrecognized value in the env var falls back to the default, so the banner stays on. `env.GetBool` warns on stderr and this is its behavior for every boolean key, not this change's | `internal/core/env/env.go` `GetBool` | acknowledged: shared helper semantics, and the YANG type is boolean so the config path cannot reach it |

### Fixes applied
- None. Run 1 found no BLOCKER and no ISSUE.

### Run 2+ (re-runs until clean)
<!-- No re-run: run 1 changed no code, so there is nothing for a second pass to read. -->

### Final status
- Run 1 shows 0 BLOCKER, 0 ISSUE. The three NOTEs are recorded above.
- Automated pre-checks: `./le repository check` reports 2 issues, both in other sessions' files (`filter_delta.go` `ExtractRemovePrivateASOps`, `internal/le/rfc/carriers.go` `CarrierRank`). `./le commit audit base origin/main` reports 11 `[WEAKENED]` findings, none in this spec's files.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 demonstrated
- [ ] End-to-End User Stories: working path + passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

## Progress, 2026-09-06

`3522fc9db4` carries the feature, `5b15df5e06` the amendment, and `170e0139ea`
the configuration guide.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A management-hardening toggle suppresses `X-Ze-Version` | Done | `hide-version` leaf, `internal/component/hub/yang/ze-hub-conf.yang`; `HTTPHeaderHidden`, `internal/core/version/version.go` | one leaf, one env key, one predicate |
| Both emitting sites honor it | Done | `addSecurityHeaders`, `internal/component/web/auth.go`; `securityHeaders`, `internal/component/lg/server.go` | `git grep "X-Ze-Version"` finds no third writer |
| The default preserves today's behavior | Done | `HTTPHeaderHidden`, `internal/core/version/version.go`, which passes `false` to `env.GetBool` | the YANG default is false as well |
| No standard `Server` header is introduced | Done | nowhere | `git grep 'Header().Set("Server"'` answers nothing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWebVersionHeaderSuppressed` over `addSecurityHeaders` | PASS at closure |
| AC-2 | Done | `TestLGVersionHeaderSuppressed` over `securityHeaders` | PASS at closure |
| AC-3 | Done | `TestVersionHeaderPresentByDefault`, `TestLGVersionHeaderPresentByDefault`, `TestApplyEnvConfigHideVersionAbsent` | PASS at closure |
| AC-4 | Done | both suppression tests assert every other security header by value | PASS at closure |
| AC-5 | Done | both suppression tests and both default tests assert `Server` is absent | PASS at closure |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestWebVersionHeaderSuppressed` | Done | `internal/component/web/auth_test.go` | PASS |
| `TestVersionHeaderPresentByDefault` | Done | `internal/component/web/auth_test.go` | PASS |
| `TestLGVersionHeaderSuppressed` | Done | `internal/component/lg/server_test.go` | PASS |
| `TestLGVersionHeaderPresentByDefault` | Done | `internal/component/lg/server_test.go` | PASS |
| `TestHTTPHeaderHidden` | Done | `internal/core/version/version_test.go` | PASS, four sub-cases |
| `TestApplyEnvConfigHideVersion` | Done | `internal/component/config/apply_env_test.go` | PASS |
| `TestApplyEnvConfigHideVersionAbsent` | Done | `internal/component/config/apply_env_test.go` | PASS |
| `version-header-suppress` | Done | `test/plugin/version-header-suppress.ci` | PASS on 2026-09-06, 15.1s, with its red half observed through the rebuilt daemon. Not re-run at closure: the plugin suite has no single-test selector and its budget is 1500s |
| `environment-hide-version` | Done | `test/parse/environment-hide-version.ci` | PASS on 2026-09-06, 575ms |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/hub/yang/ze-hub-conf.yang` | Done | the leaf, with `ze:help` |
| `internal/core/version/version.go` | Done | `EnvKeyHideVersion`, `env.MustRegister`, `HTTPHeaderHidden` |
| `internal/component/config/apply_env.go` | Done | the `envPlumbingTable` row naming the constant |
| `internal/component/web/auth.go` | Done | the guard in `addSecurityHeaders` |
| `internal/component/lg/server.go` | Done | the guard in `securityHeaders` |
| `internal/test/fixture/plugin_fixture_09_shell.go` | Done | `waitForLog09` names the needle it waited for |
| `test/plugin/version-header-suppress.ci` | Done | created |
| `test/parse/environment-hide-version.ci` | Done | created |
| `internal/test/fixture/plugin_fixture_version_header_suppress.go` | Done | created |
| `internal/test/fixture/register_version_header_suppress.go` | Done | created |

### Audit Summary
- **Total items:** 27
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 2, both recorded in Deviations from Plan

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator can stop Ze publishing its exact build over HTTP | functional | `test/plugin/version-header-suppress.ci` drives one daemon twice over the same configuration text apart from the leaf, and reads the header set of both servers with a real HTTP client. Its assertion line "OK: hide-version takes the banner off both servers" |
| One setting removes the banner everywhere, so no server is left leaking | functional plus source | the same `.ci` reads the web server and the looking glass in each run. `git grep -n "X-Ze-Version" -- '*.go'` answers two production writers, `addSecurityHeaders` and `securityHeaders`, and both consult `HTTPHeaderHidden` |
| Existing deployments are unchanged unless they opt in | functional plus unit | the `.ci` line "OK: both servers send the version banner by default", and `TestApplyEnvConfigHideVersionAbsent`, which writes an `environment` block with another leaf and asserts the predicate stays false |
| The hardening takes no other header down with it | functional plus unit | the `.ci` line "OK: the other security headers survive suppression", and both suppression unit tests, which assert each surviving header by value |
| The test discriminates: it would fail if the guards were gone | recorded red | with both guards removed and `ze` rebuilt, the `.ci` failed with `ZE-OBSERVER-FAIL: the hardened web server sent X-Ze-Version: "ze/26.09.06 (3522fc9db485+; go1.27.0; linux/amd64)", want no banner`, while the default half stayed green. Recorded by `5b15df5e06` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every acceptance criterion, test and file in the plan is implemented and verified | n/a |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/version-header-suppress.ci` | yes | `ls` at closure listed it |
| `test/parse/environment-hide-version.ci` | yes | `ls` at closure listed it |
| `internal/test/fixture/plugin_fixture_version_header_suppress.go` | yes | `ls` at closure listed it |
| `internal/test/fixture/register_version_header_suppress.go` | yes | `ls` at closure listed it |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | the web server omits the banner when hidden | `TestWebVersionHeaderSuppressed` PASS in the closure run of `internal/component/web` |
| AC-2 | the looking glass omits it | `TestLGVersionHeaderSuppressed` PASS in the closure run of `internal/component/lg` |
| AC-3 | the default keeps it | `TestVersionHeaderPresentByDefault`, `TestLGVersionHeaderPresentByDefault` and `TestApplyEnvConfigHideVersionAbsent` all PASS in the same closure run |
| AC-4 | the other security headers survive | the two suppression tests above assert five and four header values respectively, and both passed |
| AC-5 | no `Server` banner appears | `git grep -n 'Header().Set("Server"' -- '*.go'` answers nothing, and all four header tests assert the absence |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `hide-version true`, request the web UI | `test/plugin/version-header-suppress.ci` | yes: the file runs `ze-test fixture plugin/version-header-suppress`, and `versionHeaderSuppress` writes the leaf, starts the daemon and reads `/show/` over HTTPS |
| `hide-version true`, request the looking glass | `test/plugin/version-header-suppress.ci` | yes: the same fixture reads `/api/looking-glass/status` in the same run |
| default (off), request either server | `test/plugin/version-header-suppress.ci` | yes: the fixture's first run writes no leaf and asserts the `ze/` banner on both servers |
| `environment { hide-version true; }` in a config file | `test/parse/environment-hide-version.ci` | yes: the file pipes that block into `ze config validate -` and expects exit 0 |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `addSecurityHeaders` and `securityHeaders` both call `version.HTTPHeaderHidden()`, and neither signature changed |
| A-2 | confirmed | `git grep -n "version.HTTPHeader()" -- '*.go'` answers five production callers, of which exactly two write a response header. Both are guarded |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| New user-facing feature (`docs/features.md`, `docs/features/web-interface.md`) | the paragraph names `HTTPHeaderHidden` and carries the source anchor for it | yes, at HEAD |
| Config syntax (`docs/guide/configuration.md`, `docs/architecture/config/environment-block.md`, `docs/architecture/config/environment.md`) | the leaf, the env key `ze.hide-version` and the default `false` match `ze-hub-conf.yang` and `EnvKeyHideVersion` | yes, at HEAD |
| Env var (`docs/guide/environment-variables.md`) | the row matches the `env.MustRegister` entry in `internal/core/version/version.go` | yes, at HEAD |
| Guides for the two servers (`docs/guide/web-interface.md`, `docs/guide/looking-glass.md`) | each names the leaf and states that no other header changes, which both guards satisfy | yes, at HEAD |
| Architecture (`docs/architecture/web-interface.md`) | the security-header section names the toggle | yes, at HEAD |
| CLI reference, API/RPC, plugin SDK, wire format, RFC status, comparison table | `git grep -n "hide-version" -- docs/` answers only the pages above, and the change adds no command, no RPC, no wire field and no protocol behavior | yes, not applicable |
| `./le doc check verify` | not run at closure. It is red across the BGP command surface and `../gh-pages/` for reasons this spec did not produce, and re-running it would report those | not run |
