# Spec: fixit-authz-admin-fallthrough

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
3. `docs/guide/operator-access-rbac.md`, `docs/guide/tacacs.md` - the operator contract this spec must not break
4. `internal/component/authz/authz.go` (`Store.Authorize`), `cmd/ze/hub/main_servers.go` (`usersFromZefsDB`), `internal/component/plugin/server/server.go` (`wrapHandler`)
5. `plan/learned/1121-negative-test-must-fail-for-its-reason.md` - how this area's tests hid two bypasses

## Task

`authz.Store.Authorize` has three paths that end in allow-all. Each is deliberate,
documented, and asserted by a test, so none is a bug on its own. Together they mean a
box with `system.authorization` profiles configured can still authorize an
authenticated user as admin.

The crux is `hasUsers` (`authz.go:343`), which is `len(s.assignments) > 0` and serves as
the proxy for "is RBAC in use". Assignments come only from
`system.authentication.user[*].profile` (`bgp/config/loader.go:301-308`), so a
TACACS-only box has profiles but no assignments: `hasUsers` is false, and every
authenticated user whose profiles are not login-resolved lands on the admin default.

This spec decides what should happen instead, without locking an operator out of a
router. It is a design decision with bricking risk, not a patch: the obvious fix (give
the bootstrap admin an explicit assignment) flips `hasUsers` to always-true, which
changes a second site and would deny the internal RPC path.

Found while fixing two real bypasses (60e35c0d5, 701cbaaa3, 0544b274d). Those are
committed; this is the residue deliberately left alone.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/guide/operator-access-rbac.md` - the documented operator contract for profiles
  → Decision: `admin` is kept as a recovery account and is defined as a config user with a profile, so the documented setup has `hasUsers` true and unassigned users fail closed.
  → Constraint: any change must keep a documented recovery path to a box whose authorization config is wrong or partial.
- [ ] `docs/guide/tacacs.md` - priv-lvl mapping and fallback semantics
  → Constraint: an unmapped priv-lvl rejects the login (AC-18), so a TACACS user reaching authorization should already have resolved profiles.
  → Decision: with `authorization true` the TACACS server decides per command and local profiles are only the unreachable-server fallback; `strict-fallback true` makes that case deny.
- [ ] `ai/rules/no-workarounds-for-missing-behavior.md` - three tests assert today's behavior
  → Constraint: changing them is a deliberate behavior change needing user approval, not test weakening.

### RFC Summaries (MUST for protocol work)
- [ ] N/A - no protocol wire behavior. TACACS+ (RFC 8907) is involved only through
  already-implemented authorization; this spec changes no packet.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- The documented RBAC setup is safe today: it defines config users with profiles, so `hasUsers` is true and unassigned users are denied.
- The exposure is the supported-but-undocumented shape: profiles configured, no config users (TACACS-only, or profiles staged before users).
- Recovery access is the constraint that makes this hard. A network OS that locks out its operator needs console or physical access to recover.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `internal/component/authz/authz.go` - `Store.Authorize` (:339). Three terminal paths reach allow-all.
  → Constraint: `hasUsers := len(s.assignments) > 0` (:343) is the only signal for "RBAC is in use", and it counts config assignments only.
  → Decision: the godoc at :336-338 already states the intended contract — "When user assignments are configured, empty username and unassigned users are denied (fail closed). When no assignments exist, all access is allowed." The code matches its comment; the comment is what this spec questions.
  → Decision: `HasProfiles` (:301) and `HasUserAssignments` (:330) already exist as separate predicates, so O-1 needs no new accessor — only a decision about which one means "RBAC is in use".
- [ ] `internal/component/bgp/config/loader.go` - `extractAuthzConfig` (:267) builds the store; assignments come only from `system.authentication.user[*].profile` (:301-308). `ValidateAuthzConfig` (:215) rejects undefined profile references for users (:248-254) and, since 0544b274d, for `tacacs-profile`.
  → Constraint: any profile name reaching the store from config is guaranteed to resolve; validation is what keeps S-3 unreachable.
- [ ] `cmd/ze/hub/main_servers.go` - `usersFromZefsDB` (:115) returns the `ze init` bootstrap admin as `UserConfig{Name, Hash}` with **no Profiles** (:131).
  → Constraint: the break-glass admin can never have an assignment, so it always lands on S-2. This is what makes S-2 load-bearing.
  → Decision: `meta/instance/admin-disabled` (:116-118) already lets an operator suppress the bootstrap admin entirely, so an explicit "recovery identity" concept partly exists and is a natural hook for O-3.
- [ ] `internal/component/plugin/server/server.go` - `wrapHandler` (:121) builds a `CommandContext` with no Username (:123-127); the comment at :139-140 states identity must be injected by the transport layer.
  → Constraint: RPC handlers authorize with an empty username, so S-1 is a live path, not theoretical.
- [ ] `internal/component/aaa/login_profiles.go` - records profiles resolved at authentication; `authz.go:352-384` consumes them when there is no config assignment, filtered to names the store defines.
  → Decision: a TACACS user's profiles arrive here, so S-1..S-3 are now only reachable by users with *no* resolved profiles.

The three paths, as they stand:

| Site | Condition | Result | Test asserting it | Reachable by |
|------|-----------|--------|-------------------|--------------|
| S-1 `authz.go:345-350` | `username == ""` and `!hasUsers` | `Allow` | `TestStoreAuthorizeNoAuth` ("empty username (no auth configured) allows all") | RPC handlers via `wrapHandler`, which pass no username |
| S-2 `authz.go:385-391` | named user, no assignment, no login-resolved profiles, and `!hasUsers` | `BuiltinAdminProfile` = allow-all | `TestStoreAuthorizeNoProfiles` ("PREVENTS: users locked out when no profile assigned") | the `ze init` bootstrap admin on any box with no config users; any authenticated user on a TACACS-only box whose profiles did not resolve |
| S-3 `authz.go:428-430` | assignment exists but no referenced profile resolves | `BuiltinAdminProfile` = allow-all | `TestStoreProfileNotFound` ("user assigned a non-existent profile gets admin default") | not reachable from config today: `ValidateAuthzConfig` rejects undefined references, and 701cbaaa3 filters login-resolved names to known ones. Reachable only through the direct `Store` API |

**Behavior to preserve:** (unless user explicitly said to change)
- The documented setup in `docs/guide/operator-access-rbac.md` keeps working: profiles plus config users with assignments, unassigned users denied.
- A box whose config defines no authorization at all stays fully permissive.
- The `ze init` bootstrap admin retains a usable path to a box, including one whose authorization config is wrong or partial. This is the recovery account.
- Internal RPC dispatch (`wrapHandler`, empty username) keeps working on a box with RBAC configured.
- TACACS+ priv-lvl mapping continues to govern commands (60e35c0d5), and an unresolvable mapping continues to fail closed (701cbaaa3).

**Behavior to change:** (only if user explicitly requested)
- None yet. The change is the subject of this spec and needs a decision at the DESIGN gate. Candidate directions are in Key Design Decisions; none is approved.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- A command arrives on a surface: SSH exec/session, REST/gRPC, web, MCP, or an internal RPC handler.
- Format at entry: a command string plus a caller identity (username, remote address). The internal RPC path supplies no username.

### Transformation Path
1. Authentication resolves the identity and, for local/TACACS users, a profile-name list (`aaa` chain; `login_profiles.go` records it).
2. The surface builds a `CommandContext` carrying username and remote address (`plugin/server/command.go`), or omits the username on the `wrapHandler` RPC path.
3. `Dispatcher.isAuthorized` (`plugin/server/command.go:503`) calls the configured `aaa.Authorizer`.
4. `authz.StoreAuthorizer` forwards to `Store.Authorize` (`authz/register.go`).
5. `Store.Authorize` selects profiles: config assignment, else login-resolved names filtered to known profiles, else one of the three fall-throughs above.
6. The chosen profile's run/edit section decides; a refusal surfaces as `plugin.UnauthorizedMessage`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Authentication ↔ Authorization | profile names recorded at login, read by the store | [ ] |
| Config ↔ Store | `extractAuthzConfig` builds profiles and assignments | [ ] |
| Transport ↔ Dispatcher | `CommandContext.Username`; empty on the RPC path | [ ] |
| zefs ↔ AAA | `usersFromZefsDB` supplies the bootstrap admin credential, without profiles | [ ] |

### Integration Points
- `aaa.Bundle.Authorizer` / `liveAAABundleAuthorizer` - resolves the live bundle per call, so a store change takes effect on reload without re-wiring.
- `tacacs.TacacsAuthorizer` - falls back to the local store on server error, so any store change also changes TACACS fallback behavior.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding — new commands, CLI/monitor views, families, and handlers register via the existing registry and the core discovers them; no new per-feature field, switch case, or factory is added to a core/shared package (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The bootstrap admin from `ze init` never carries profiles, so it always reaches S-2 | `cmd/ze/hub/main_servers.go:131` returns `UserConfig{Name, Hash}` only | If it can carry profiles, S-2 stops being the recovery path and may be removable outright | Read the producer; add a test asserting the zefs user's Profiles are empty | unvalidated |
| A-2 | Denying at S-2 would lock the bootstrap admin out of a box that has profiles but no config users | Follows from A-1 plus `authz.go:385-391` | The lockout risk evaporates and the fix is a one-line flip | Functional test: profiles, no config users, authenticate as the zefs admin, run a command | unvalidated |
| A-3 | Assigning the bootstrap admin a profile makes `hasUsers` true on every box | `hasUsers := len(s.assignments) > 0` (`authz.go:343`) counts any assignment | The S-1 coupling disappears and the sites can be fixed independently | Read `extractAuthzConfig`; add a store test | unvalidated |
| A-4 | S-1 (empty username) is a live path, not dead code | `plugin/server/server.go:123-127` builds a context with no username; :139-140 says identity is injected by the transport | If dead, S-1 can be denied outright with no consequence | Instrument or test `wrapHandler` dispatch on an RBAC-configured box | unvalidated |
| A-5 | S-3 is unreachable from config | `ValidateAuthzConfig` (`loader.go:215`) rejects undefined references for users and tacacs-profile; 701cbaaa3 filters login names | If reachable, a typo grants admin and this becomes urgent rather than defensive | Grep every `AssignProfiles`/`AddProfile` caller for a path that skips validation | unvalidated |
| A-6 | An unmapped TACACS priv-lvl rejects the login, so TACACS users reaching authorization have resolved profiles | `docs/guide/tacacs.md` ("unmapped priv-lvl rejects the login (AC-18)") | TACACS users could reach S-2 and get admin, making this urgent | Read the tacacs authenticator's handlePass; add a test for an unmapped level | unvalidated |
| A-7 | `admin-disabled` is the existing operator control for suppressing the recovery account | `cmd/ze/hub/main_servers.go:116-118` returns `errAdminDisabledInZefs` | O-3 needs a different hook for naming the recovery identity | Read the flag's consumers and docs | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-0 | The three sites are load-bearing in ways a reader may not expect, so a future session "cleans them up" without reading this spec | A diff flips a fall-through to Deny with no spec reference | The Current Behavior table names who reaches each site; R-3 covers the tests |
| R-1 | A stricter default locks an operator out of a live router; recovery needs console or physical access | A functional test authenticating the bootstrap admin against a profiles-only config starts failing | Keep an always-available recovery identity; make the deny path explicit and logged so the cause is visible in the daemon log |
| R-2 | Making the bootstrap admin an explicit assignment flips `hasUsers` true everywhere, so S-1 starts denying internal RPC dispatch | RPC-driven features (plugin commands, web tools) fail with the access-control refusal on boxes that previously worked | Decide S-1 on its own terms (inject identity at the RPC boundary, or exempt it explicitly) before touching S-2 |
| R-3 | The three tests asserting today's behavior get "fixed" to match new code without a decision, silently changing the security contract | A diff editing `TestStoreAuthorizeNoAuth`/`NoProfiles`/`ProfileNotFound` without a spec reference | Any change to those tests cites this spec and the approved decision in its commit body |
| R-4 | Fixing only S-2 leaves S-1 as an equivalent hole via the RPC path | An RPC-dispatched command succeeds for an identity the SSH path would refuse | Treat S-1 and S-2 as one decision; do not ship a partial |
| ~~R-5~~ | ~~The `bgp/config` authz tests do not run (they need the ssh build tag), so a regression here is invisible to that package's suite~~ | ~~`TestExtractAuthzConfig_*` and `TestValidateAuthzConfig_*` fail on "unknown field in authentication: user"~~ | **Superseded: this risk does not exist.** The tests run and pass under the real gate. The reds were phantoms from running bare `go test` without Ze's feature tags — see `ai/rules/bash-output.md` "Bare `go test` Lies". With `-tags "ze_core ... ze_ssh"` the package is `ok`, and `make ze-verify` never reported it. Recorded in the Mistake Log below. |
| R-5b | Unit coverage for this area is *not* the weak spot, so the temptation is to trust it entirely; but `Store.Authorize` decides for surfaces (ssh, RPC, web) that unit tests do not exercise | A store-level test passes while an operator is still refused, or still admitted, on a real box | Keep the `.ci` rows: they cover the surface path, which is where 60e35c0d5 and 701cbaaa3 actually bit |
| R-6 | The godoc at `authz.go:336-338` documents the current contract, so changing behavior without rewriting it leaves a lying comment | Reviewer reads the comment and believes the old rule | Update the godoc in the same commit as the behavior |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SSH command from the zefs bootstrap admin, config has profiles but no config users | → | `Store.Authorize` S-2 branch | (fill during design — a `test/plugin/*.ci` proving the decided behavior, allow or deny) |
| SSH command from an authenticated user with no resolved profiles on a TACACS-only config | → | `Store.Authorize` S-2 branch | (fill during design) |
| Internal RPC dispatch with no username on an RBAC-configured box | → | `Store.Authorize` S-1 branch | (fill during design — must prove the decided behavior does not break plugin RPC) |

## Acceptance Criteria

<!-- Written against the problem, not a chosen solution. The DESIGN gate fixes the expected column. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config defines `system.authorization` profiles and no config users; an authenticated user with no resolved profiles runs a command | (fill during design) A decided, documented outcome that is not "silently admin". |
| AC-2 | Same config; the `ze init` bootstrap admin runs a command | The recovery account reaches the box by a documented path. |
| AC-3 | Internal RPC dispatch (no username) on a box with profiles and config users | Behaves as today: plugin RPC keeps working, and the reason is explicit rather than incidental. |
| AC-4 | A store whose assignment names only undefined profiles (direct API) | Fails closed rather than granting admin. |
| AC-5 | The documented setup in `docs/guide/operator-access-rbac.md` | Unchanged: assigned users get their profile, unassigned users are denied. |
| AC-6 | Whatever outcome AC-1 fixes | The daemon log states which rule decided, so an operator can tell "denied by profile" from "denied because no profile applied". |
| AC-7 | The godoc on `Store.Authorize` and the two operator guides | Describe the rule the code actually implements after the change. |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Stages authorization profiles before adding config users, then logs in as the bootstrap admin | ssh auth → aaa chain → dispatcher → `Store.Authorize` S-2 | (fill during design) |
| 2 | Runs a TACACS-only box where a user authenticates but resolves no profiles | tacacs authen → login profiles (empty) → `Store.Authorize` S-2 | (fill during design) |
| 3 | Uses a plugin feature that dispatches an RPC with no username | plugin → `wrapHandler` → `isAuthorized` → `Store.Authorize` S-1 | (fill during design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStoreAuthorizeNoAuth` | `internal/component/authz/authz_test.go` | existing S-1 contract; changes only if S-1 changes | |
| `TestStoreAuthorizeNoProfiles` | `internal/component/authz/authz_test.go` | existing S-2 contract; changes only if S-2 changes | |
| `TestStoreProfileNotFound` | `internal/component/authz/authz_test.go` | existing S-3 contract; changes only if S-3 changes | |
| (fill during design) | `internal/component/authz/authz_test.go` | the decided behavior for each of S-1..S-3 | |
| (fill during design) | `cmd/ze/hub/main_servers_test.go` | the bootstrap admin's profile shape (A-1) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A — this spec adds no numeric input | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (fill during design) | `test/plugin/*.ci` | profiles configured, no config users, bootstrap admin runs a command | |
| (fill during design) | `test/plugin/*.ci` | profiles configured, no config users, authenticated user with no resolved profiles | |
| `rbac-ssh-only-enforced` | `test/plugin/rbac-ssh-only-enforced.ci` | existing: profiles apply on an ssh-only daemon (must keep passing) | |
| `tacacs-readonly`, `tacacs-author` | `test/plugin/` | existing: priv-lvl mapping governs commands (must keep passing) | |
| `authz-deny`, `authz-allow`, `authz-default` | `test/plugin/` | existing: profile allow/deny contract (must keep passing) | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A — no wire protocol behavior changes | - | - | - | - |

### Future (if deferring any tests)
- None planned. `.ci` coverage is mandatory here because `Store.Authorize` decides for surfaces the unit tests do not exercise (R-5b), not because the unit tests are broken — they are not (see the superseded R-5).

## Files to Modify
- `internal/component/authz/authz.go` - the three fall-throughs in `Store.Authorize`, and its godoc (R-6)
- `internal/component/authz/authz_test.go` - the three tests asserting today's behavior
- `cmd/ze/hub/main_servers.go` - only if the decision gives the bootstrap admin an explicit profile
- `internal/component/plugin/server/server.go` - only if the decision injects an identity on the RPC path
- `docs/guide/operator-access-rbac.md` - the operator-facing rule for a user with no applicable profile
- `docs/guide/tacacs.md` - if the TACACS-only shape's outcome changes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] | none expected; a reserved profile name would need a decision on whether it is config-visible |
| CLI commands/flags | [ ] | none expected |
| Functional test for new RPC/API | [ ] | `test/plugin/*.ci` — mandatory, see R-5b |
| Doctor check for runtime dependencies | [ ] | consider a check that warns when profiles exist but nothing assigns them (the exposed shape) |
| Prometheus counters/metrics | [ ] | consider counting refusals by reason, to make AC-6 observable |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/operator-access-rbac.md`, `docs/guide/tacacs.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | only if the RPC identity path changes |
| 9 | RFC behavior implemented, changed, or newly proven? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [ ] | `docs/plugin-development/metrics.md` if refusal counters are added |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | [ ] | N/A |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | grep `docs/` for `source: internal/component/authz/authz.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | [ ] | `docs/guide/operator-access-rbac.md` profile examples |

## Files to Create
- `test/plugin/*.ci` - (fill during design) functional coverage for the decided behavior

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. Full verification | `make ze-verify` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section |
| 14. Present summary + close | Executive Summary Report; two-commit closure |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Validate the assumptions before designing** — A-1..A-7 are cheap to settle by reading plus one probe each, and A-2/A-4 decide whether there is a lockout risk at all.
   - Tests: probes only
   - Files: none
   - Verify: every A-N row is `confirmed` or `broken`
2. **Phase: Decide** — present the options in Key Design Decisions with the validated assumptions; get approval. STOP here without it.
3. **Phase: Wiring (MANDATORY FIRST once approved)** — (fill during design) register the entry points from the Wiring Test table and write failing wiring tests
4. **Phase: (fill during design)** — the decided behavior, TDD per site
5. **Functional tests** → the `.ci` rows above; unit coverage alone is insufficient here because the decision serves surfaces the store tests never drive (R-5b)
6. **Full verification** → `make ze-verify`
7. **Complete spec** → learned summary + two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | The bootstrap admin still reaches a box whose authorization config is wrong |
| Correctness | Internal RPC dispatch still works on an RBAC-configured box |
| Data flow | The decision is made in `Store.Authorize` only; surfaces are unchanged |
| Naming | Any new "RBAC is in use" signal is named for what it means, not for what it counts |
| Rule: no-layering | No second authorization notion added alongside the store |
| Observability | A refusal states which rule decided (AC-6) |
| Comment truth | The `Store.Authorize` godoc matches the implemented rule (R-6) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Each of S-1..S-3 has a decided, tested outcome | `go test ./internal/component/authz -run TestStoreAuthorize` |
| Recovery path documented | grep `docs/guide/operator-access-rbac.md` for the recovery rule |
| Existing RBAC/TACACS functional tests still pass | `ze-test bgp plugin rbac-ssh-only-enforced tacacs-readonly tacacs-author authz-deny` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Fail-closed default | No remaining path where "nothing applied" means allow-all |
| Recovery vs exposure | The recovery identity is explicit, named, and logged — not an accidental consequence of a counter being zero |
| Information disclosure | A refusal does not reveal profile contents to an unauthorized caller |
| Downgrade | A config reload cannot transiently widen a session's authority |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior → RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural → DESIGN phase |
| Functional test fails | Check AC; if AC wrong → DESIGN; if AC correct → IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The three fall-throughs were fail-open bugs to fix | Each is deliberate, documented, and asserted by a test; S-2 is the break-glass path for the `ze init` admin, which carries no profiles | Read `usersFromZefsDB` (`main_servers.go:131`) and the three tests' stated intent before changing anything | The reported "fix these" scope collapsed to one real gap (tacacs-profile validation, fixed in 0544b274d); the rest became this spec |
| `bgp/config` had 10 broken tests that "never run" (needing the ssh build tag), and `plugin/all` had 3 failing golden snapshots — both reported as pre-existing, and R-5 written into this spec on that basis | Neither is broken. Both pass under Ze's feature tags: `go test -tags "ze_core ... ze_ssh"` returns `ok`. `make ze-verify` never reported either package — the reds existed only in bare `go test` runs, where `all_ze_ssh.go` (`//go:build ze_ssh`) never registers the ssh YANG module, so `system.authentication.user` becomes an unknown field | Re-ran with the tags after noticing commit fd72df184, "rules: bare go test drops feature tags and fakes reds", which landed mid-session and describes this exact trap | R-5 superseded. The claim also reached the user repeatedly ("the blind spot that let the authz bugs hide") and the body of commit 0544b274d, which calls them "10 pre-existing failures". That commit's conclusion still holds — the failure set was identical before and after, so the change added none — but its premise is a phantom |

**Why the baseline "confirmed" it.** The `git archive f9dc6132b` check ran bare `go test`
too, so it reproduced the same tags-less mistake and returned an identical failure set.
Identical-to-baseline proved only "not caused by my change"; it could never prove "real",
because both sides shared the defect. `ai/rules/bash-output.md` names this: *"a bare run
there reproduces your own mistake and 'confirms' a red that does not exist."*

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Flip the fall-throughs to Deny | Would lock the bootstrap admin out of a box with profiles but no config users (A-1/A-2) | Decide the recovery identity first; this spec |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| Calling a tested, deliberate behavior a "bug" before reading its test's stated intent | 1 (this spec) | Partly covered by `ai/rules/no-fabrication.md` (read the producer); the gap was reading the producer of the *value* but not of the *intent* | Note in the learned summary if it recurs |

## Design Insights
- `hasUsers` conflates two different questions: "did the operator configure RBAC?" (profiles exist) and "did the operator assign anyone?" (assignments exist). All three paths key off the second while meaning the first. Naming those apart is probably the whole fix, and both predicates already exist (`HasProfiles` :301, `HasUserAssignments` :330) — they are simply not used here.
- The bootstrap admin is an *implicit* super-user: it is admin because nothing matched, not because anything says so. Making it explicit is what would let the defaults become strict.
- `admin-disabled` (`main_servers.go:116-118`) shows the recovery account is already a first-class concept at the zefs layer; authorization simply never learned about it.

## Core Insight
A permissive default that exists to prevent lockout is not the same thing as a bug — but it becomes one the moment a second, unrelated shape (a TACACS-only box) reaches the same branch. The fix is not to flip the default; it is to give the recovery case a name, so the default no longer has to carry it.

## Key Design Decisions
<!-- None approved. These are the options to present at the DESIGN gate. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| (undecided) | **O-1** Split the signal: keep the fall-throughs but key them on `HasProfiles()` instead of `hasUsers`, so a box with profiles never silently grants admin | Smallest change, and both predicates already exist; denies the bootstrap admin on a profiles-only box unless O-3 lands first |
| (undecided) | **O-2** Leave S-1/S-2 as they are; document the shape and add a doctor check plus startup warning when profiles exist with no assignments | Zero lockout risk; leaves the exposure in place and relies on the operator noticing |
| (undecided) | **O-3** Give the bootstrap admin an explicit reserved profile at load (hooking where `admin-disabled` already lives), then make the defaults strict | Most principled; flips `hasUsers` true everywhere, so S-1 must be settled first (R-2) |
| (undecided) | **O-4** Inject an explicit internal identity at the RPC boundary so S-1 stops needing an empty-username rule | Removes the S-1/S-2 coupling; touches the plugin server contract |

## Known Limitations
- This spec deliberately does not decide. It records what is true, what breaks if changed naively, and what must be settled first.
- S-3 is defensive only: unreachable from config today. Fixing it is cheap and safe, but on its own it closes nothing an operator can hit.
- The exposed shape (profiles with no assignments) is not currently documented as supported or unsupported. Deciding that is arguably prior to deciding the code.

## RFC Documentation

N/A — no protocol behavior. No `// RFC` annotations expected.

## Implementation Summary

### What Was Implemented
- Nothing yet — `skeleton`.

### Bugs Found/Fixed
- Not in this spec. Related bugs already fixed and committed: ssh-only daemons ignoring profiles and TACACS priv-lvl mapping not governing commands (60e35c0d5), an unresolvable mapping granting admin (701cbaaa3), `tacacs-profile` references unvalidated (0544b274d).

### Documentation Updates
- None yet.

### Deviations from Plan
- None yet.

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
| A box with profiles configured never silently authorizes a user as admin | functional test | (fill during implementation) |
| The recovery account still reaches a misconfigured box | functional test | (fill during implementation) |
| Internal RPC dispatch is unaffected | functional test | (fill during implementation) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | (not yet run) | | |

### Fixes applied
- (none yet)

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
- [ ] AC-1..AC-N all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

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
- [ ] Critical Review passes — all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
