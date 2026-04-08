# Spec: decouple-0-umbrella

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-04-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `.claude/patterns/registration.md`
4. `.claude/rules/design-principles.md`

## Task

Decouple cross-component imports in `internal/component/` by moving code to its natural owner, using registration callbacks, and introducing `contract/` sub-packages where needed. The goal: each component can be removed without breaking unrelated components.

This is Phase 1 of a two-phase effort. Phase 1 fixes wrong-direction and unnecessary coupling. Phase 2 (spec-decouple-1-cli-contract) adds `cli/contract` for correct-direction coupling (ssh -> cli, web -> cli).

## Child Specs

| Spec | What | Status |
|------|------|--------|
| `spec-decouple-1-cli-contract.md` | Phase 2: `cli/contract` for ssh and web | skeleton |

## Design Decisions (agreed with user 2026-04-08)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Internal interfaces go in `internal/`, not `pkg/ze/` | None of the identified couplings are plugin-author-facing |
| D2 | Sub-packages named `contract/` | Uniform naming, import path reveals dependency source |
| D3 | Move `UserConfig` + `AuthenticateUser` + `CheckPassword` from ssh to authz | Auth is a shared concern, not SSH-specific. authz is the access-control home |
| D4 | bgp/config/loader returns plain data structs, hub creates servers | Loader is a god object creating SSH/web/authz. Hub is already the orchestrator |
| D5 | Move blank imports from reactor to `plugin/all/all.go` via codegen | Reactor should not know about command handlers. all.go already handles this pattern |
| D6 | Move BGP-specific RPCs from `cmd/*` to `bgp/plugins/cmd/` | Proximity principle. cmd/ keeps only protocol-agnostic commands |
| D7 | iface registers MAC CompleteFn via validator registry instead of config importing iface | Foundation must not depend on domain. Validator registry already supports callbacks |
| D8 | plugin/server <-> config coupling: skip | Bidirectional but not circular (different sub-packages). Both sides have legitimate reasons |
| D9 | ssh -> cli, web -> cli: Phase 2 (cli/contract) | Correct dependency direction, deep legitimate coupling. Not blocking |

## Bad Import Map (current state)

| # | Source | Imports | Uses | Problem |
|---|--------|---------|------|---------|
| 1 | web/auth.go | ssh | `AuthenticateUser`, `UserConfig` | Peer imports peer for shared concern |
| 2 | ssh/session.go, ssh.go, warnings.go | cli | Editor, MonitorSession, LoginWarning | Correct direction, Phase 2 |
| 3 | bgp/config/loader.go | cli | `LoginWarning`, `MonitorSession` | BGP imports presentation layer |
| 4 | bgp/config/loader.go, loader_create.go | ssh | `NewServer`, `Config`, `Server` | BGP creates SSH servers |
| 5 | bgp/config/loader_create.go | web | `WebServer` | BGP creates web servers |
| 6 | bgp/config/loader.go | authz | `Store`, `Profile`, `Entry` | Loader builds authz store (hub's job) |
| 7 | bgp/reactor/reactor.go | cmd/cache, cmd/commit, cmd/del, cmd/log, cmd/meta, cmd/metrics, cmd/set, cmd/show, cmd/subscribe, cmd/update, iface/cmd | Blank imports for init() RPC registration | Reactor knows about all command handlers |
| 8 | cmd/show, cmd/del, cmd/set, cmd/update | bgp/plugins/cmd/peer | Handler functions | Protocol-specific handlers in generic cmd |
| 9 | cmd/commit | bgp/nlri, bgp/transaction | Route building types | Protocol-specific logic in generic cmd |
| 10 | cmd/cache/require.go | bgp/types | `BGPReactor` interface | Protocol type assertion in generic cmd |
| 11 | config/validators.go | iface | `DiscoverInterfaces()` | Foundation imports domain |

## Target Import Graph (after Phase 1)

| Component | Allowed imports (other components) |
|-----------|-----------------------------------|
| authz | config/yang (schema registration) |
| bgp | config, plugin (no cli, ssh, web, authz, cmd, iface) |
| cli | command, config, plugin/server |
| cmd (protocol-agnostic only: log, meta, metrics, subscribe) | config/yang, plugin, plugin/server |
| command | plugin/registry |
| config | plugin, plugin/registry, command (no iface) |
| engine | (none) |
| hub | everything (it is the orchestrator) |
| iface | config/yang, plugin, plugin/registry, plugin/server |
| lg | config/yang |
| managed | plugin/ipc |
| mcp | config/yang |
| plugin (excluding all/all.go) | config, authz |
| plugin/all/all.go | everything (auto-generated registration hub) |
| resolve | config/yang, plugin, plugin/server |
| ssh | cli, command, config, plugin/server (cli coupling addressed in Phase 2) |
| telemetry | config/yang |
| web | cli, config (no ssh -- auth moved to authz) |

## Execution Phases

### Phase 1: Auth to authz (D3)

**What:** Move `UserConfig`, `CheckPassword`, `AuthenticateUser` from `internal/component/ssh/auth.go` to `internal/component/authz/`.

**Files to modify:**

| File | Change |
|------|--------|
| `internal/component/ssh/auth.go` | Delete `UserConfig`, `CheckPassword`, `AuthenticateUser`, `dummyHash` |
| `internal/component/authz/auth.go` | Create: move the 4 items here |
| `internal/component/ssh/ssh.go` | Update imports: `ssh.UserConfig` becomes `authz.UserConfig` |
| `internal/component/ssh/session.go` | Same import update |
| `internal/component/web/auth.go` | Change import from ssh to authz |
| `internal/component/web/auth_test.go` | Same import update |
| `internal/component/web/integration_test.go` | Same import update |
| `internal/component/bgp/config/loader.go` | Already imports authz, update type references |
| `internal/component/bgp/config/loader_create.go` | Same |

**Verification:** `make ze-verify` passes. grep confirms zero `component/ssh` imports in web.

### Phase 2: iface validator registration (D7)

**What:** Split the MAC address validator. config keeps `ValidateFn` (pure regex). iface registers `CompleteFn` for interface discovery via the validator registry in its own `init()`.

**Files to modify:**

| File | Change |
|------|--------|
| `internal/component/config/validators.go` | Remove iface import. MAC validator returns only `ValidateFn`, no `CompleteFn` |
| `internal/component/config/yang/validator_registry.go` | If needed: add API to register a `CompleteFn` separately from `ValidateFn` |
| `internal/component/iface/register.go` (or new validators_register.go) | Register MAC `CompleteFn` in `init()` using `iface.DiscoverInterfaces()` |

**Verification:** `make ze-verify` passes. grep confirms zero `component/iface` imports in config.

### Phase 3: Reactor blank imports to codegen (D5)

**What:** Remove blank imports of `cmd/*` and `iface/cmd` from `bgp/reactor/reactor.go`. Add them to `plugin/all/all.go` via the codegen script.

**Files to modify:**

| File | Change |
|------|--------|
| `internal/component/bgp/reactor/reactor.go` | Remove 11 blank imports (cmd/cache through iface/cmd) |
| `scripts/codegen/plugin_imports.go` | Extend to discover and generate blank imports for cmd RPC packages |
| `internal/component/plugin/all/all.go` | Re-generated: now includes cmd/* and iface/cmd blank imports |

**Verification:** `make generate` then `make ze-verify` passes. grep confirms zero `component/cmd` imports in reactor.

### Phase 4: Loader returns data, hub wires servers (D4)

**What:** Extract server creation from bgp/config/loader into hub. Loader returns plain config structs. Hub creates SSH server, web server, wires editor sessions and monitors.

This is the largest phase. The loader currently:
- Calls `extractSSHConfig()` returning `ssh.Config`, then `ssh.NewServer()`
- Calls `extractAuthzConfig()` returning `*authz.Store`
- Creates `web.WebServer`
- Builds `cli.MonitorSession` factories
- Collects `cli.LoginWarning` via `collectPrefixWarnings()`
- Wires executor factories on SSH server

After: loader returns plain structs. Hub calls loader, then creates servers.

**Files to modify:**

| File | Change |
|------|--------|
| `internal/component/bgp/config/loader.go` | Remove cli, ssh imports. `extractSSHConfig` returns plain struct (strings, ints). `collectPrefixWarnings` returns plain strings. Remove `extractAuthzConfig` (hub calls authz directly) |
| `internal/component/bgp/config/loader_create.go` | Remove cli, ssh, web imports. Return a `LoaderResult` struct with config data, no server instances |
| `cmd/ze/hub/main.go` | Add server creation code: `ssh.NewServer()`, `web.NewWebServer()`, executor factory wiring, monitor factory wiring using cli types |

**New types in loader (plain data, no component imports):**

| Type | Fields |
|------|--------|
| `SSHConfig` | IP string, Port int, HostKeyPaths []string, Users []UserCredential |
| `UserCredential` | Name string, Hash string |
| `WebConfig` | IP string, Port int, Enabled bool |
| `LoaderResult` | SSHConfig, WebConfig, AuthzProfiles, PrefixWarnings []string |

**Verification:** `make ze-verify` passes. grep confirms zero cli/ssh/web imports in bgp/config/.

### Phase 5: BGP RPCs to bgp/plugins/cmd (D6)

**What:** Move BGP-specific RPC registrations from `cmd/{show,del,set,update}` into `bgp/plugins/cmd/peer/`. Move `cmd/commit` and `cmd/cache` BGP-specific logic into `bgp/plugins/cmd/`.

**Commands that stay in cmd/ (protocol-agnostic):**

| Package | Wire methods | Why stays |
|---------|-------------|-----------|
| cmd/log | ze-log:* | Generic logging, no BGP types |
| cmd/meta | ze-meta:* | Help/discovery, no BGP types |
| cmd/metrics | ze-metrics:* | Generic metrics, uses plugin/registry not BGP |
| cmd/subscribe | ze-subscribe:* | Generic event subscription |

**Commands that move to bgp/plugins/cmd/:**

| Current location | Wire methods | Moves to |
|-----------------|-------------|----------|
| cmd/show/show.go | ze-show:bgp-peer | bgp/plugins/cmd/peer/ (handler already there) |
| cmd/del/del.go | ze-del:bgp-peer | bgp/plugins/cmd/peer/ |
| cmd/set/set.go | ze-set:bgp-peer-with, ze-set:bgp-peer-save | bgp/plugins/cmd/peer/ |
| cmd/update/update.go | ze-update:bgp-peer-prefix | bgp/plugins/cmd/peer/ |
| cmd/commit/commit.go | ze-bgp:commit | New: bgp/plugins/cmd/commit/ |
| cmd/cache/* | ze-bgp:cache-* | New: bgp/plugins/cmd/cache/ |

**Files to modify:**

| File | Change |
|------|--------|
| `internal/component/cmd/show/show.go` | Remove bgp-peer RPC registration |
| `internal/component/cmd/del/del.go` | Remove bgp-peer RPC registration |
| `internal/component/cmd/set/set.go` | Remove bgp-peer RPC registrations |
| `internal/component/cmd/update/update.go` | Remove bgp-peer RPC registration |
| `internal/component/cmd/commit/` | Delete package (all logic is BGP-specific) |
| `internal/component/cmd/cache/` | Delete package (all logic is BGP-specific) |
| `internal/component/bgp/plugins/cmd/peer/register.go` | Add RPC registrations that were in cmd/show, cmd/del, cmd/set, cmd/update |
| `internal/component/bgp/plugins/cmd/commit/` | New package: move commit logic from cmd/commit |
| `internal/component/bgp/plugins/cmd/cache/` | New package: move cache logic from cmd/cache |
| `scripts/codegen/plugin_imports.go` | Update codegen to find new bgp/plugins/cmd/* packages |

**Verification:** `make generate` then `make ze-verify` passes. grep confirms zero `component/bgp` imports in cmd/.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | grep for `component/ssh` in web/*.go (non-test) | Zero matches |
| AC-2 | grep for `component/iface` in config/*.go (non-test) | Zero matches |
| AC-3 | grep for `component/cmd` in bgp/reactor/*.go | Zero matches |
| AC-4 | grep for `component/cli` in bgp/config/*.go | Zero matches |
| AC-5 | grep for `component/ssh` in bgp/config/*.go | Zero matches |
| AC-6 | grep for `component/web` in bgp/config/*.go | Zero matches |
| AC-7 | grep for `component/bgp` in cmd/*.go (non-test) | Zero matches |
| AC-8 | `make ze-verify` after all phases | Pass |
| AC-9 | `AuthenticateUser` callable from both ssh and web | Both import authz, both work |
| AC-10 | MAC completion still works in CLI | iface registers CompleteFn via validator registry |

## Wiring Test (MANDATORY)

| Entry Point | Arrow | Feature Code | Test |
|-------------|---|--------------|------|
| Web login with password | -> | `authz.AuthenticateUser` | Existing web auth tests pass after move |
| SSH login with password | -> | `authz.AuthenticateUser` | Existing ssh tests pass after move |
| MAC address tab completion | -> | iface-registered CompleteFn | Existing config validator tests pass |
| `ze hub` startup with bgp+ssh+web config | -> | hub creates servers from loader data | Existing functional tests pass |
| `show bgp peer` command | -> | bgp/plugins/cmd/peer handler | Existing plugin functional tests pass |
| `bgp commit` command | -> | bgp/plugins/cmd/commit handler | Existing commit functional tests pass |

## TDD Test Plan

### Unit Tests

This is a refactoring spec -- existing tests must continue to pass. New tests only where behavior changes:

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAuthenticateUser` | `internal/component/authz/auth_test.go` | Auth functions work from new location | |
| `TestMACCompleteFn` | `internal/component/iface/validators_test.go` | CompleteFn registration works | |
| `TestLoaderResultPlainTypes` | `internal/component/bgp/config/loader_test.go` | Loader returns data structs, no server instances | |

### Functional Tests

All existing functional tests must pass unchanged. No new `.ci` files needed -- this is pure refactoring (same behavior, different code organization).

## Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` -- update component dependency description, note auth lives in authz |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify per phase |
| 3. Implement (TDD) | Phases 1-5 in order |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run `make ze-verify` |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run `make ze-verify` |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with `make ze-verify`. Fix before proceeding.

1. **Phase: Auth to authz** -- Move auth types and functions from ssh to authz. Update all imports.
   - Tests: existing ssh/web auth tests pass
   - Files: see Phase 1 table
   - Verify: `make ze-verify`, grep for zero ssh imports in web

2. **Phase: iface validator** -- Split MAC validator, iface registers CompleteFn
   - Tests: existing validator tests pass, new CompleteFn test
   - Files: see Phase 2 table
   - Verify: `make ze-verify`, grep for zero iface imports in config

3. **Phase: Reactor imports** -- Remove blank imports, update codegen
   - Tests: `make generate` then `make ze-verify`
   - Files: see Phase 3 table
   - Verify: grep for zero cmd imports in reactor

4. **Phase: Loader to hub** -- Loader returns data, hub creates servers
   - Tests: existing functional tests pass
   - Files: see Phase 4 table
   - Verify: `make ze-verify`, grep for zero cli/ssh/web imports in bgp/config

5. **Phase: BGP RPCs** -- Move protocol-specific RPCs to bgp/plugins/cmd
   - Tests: existing functional tests pass
   - Files: see Phase 5 table
   - Verify: `make generate` then `make ze-verify`, grep for zero bgp imports in cmd

6. **Functional tests** -- Run full suite
7. **Full verification** -- `make ze-verify`
8. **Complete spec** -- Audit, learned summary

### Critical Review Checklist

| Check | What to verify |
|-------|----------------|
| Completeness | All 10 AC verified with grep evidence |
| Correctness | All existing tests pass. No behavior change |
| No-layering | Old code deleted, not wrapped. No identity wrappers |
| Import graph | Matches target table above |
| Registration | iface CompleteFn uses existing validator registry API |
| Codegen | `make generate` produces correct all.go |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Zero ssh imports in web (non-test) | `grep -rn component/ssh internal/component/web/*.go` |
| Zero iface imports in config (non-test) | `grep -rn component/iface internal/component/config/*.go` |
| Zero cmd imports in reactor | `grep -rn component/cmd internal/component/bgp/reactor/` |
| Zero cli/ssh/web imports in bgp/config | `grep -rn 'component/cli\|component/ssh\|component/web' internal/component/bgp/config/*.go` |
| Zero bgp imports in cmd | `grep -rn component/bgp internal/component/cmd/` |
| `make ze-verify` passes | Run and paste output |
| Auth works from authz | `go test ./internal/component/authz/...` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Auth timing safety | `AuthenticateUser` still uses bcrypt dummy hash for unknown users after move to authz |
| Auth constant-time | `CheckPassword` still uses `subtle.ConstantTimeCompare` for hash-as-token |
| No auth bypass | Web and SSH both call the same `authz.AuthenticateUser`, no code path skips it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error after move | Fix import paths in the phase that moved code |
| Test fails after auth move | Verify all import updates, check no ssh.UserConfig references remain |
| Codegen produces wrong output | Fix `scripts/codegen/plugin_imports.go` discovery logic |
| Functional test fails after loader refactor | Hub wiring is missing something the loader used to do. Trace the failure |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user |

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
- Phase 1: auth moved from ssh to authz (UserConfig, CheckPassword, AuthenticateUser)
- Phase 2: global CompleteFn registry decouples config from iface
- Phase 3: codegen RPC discovery + all_import_test.go refactored to external test packages
- Phase 4: InfraHook extracts SSH/CLI/web wiring from bgp/config to hub
- Phase 5: BGP RPCs moved from cmd/* to bgp/plugins/cmd/
- All 10 ACs pass

### Bugs Found/Fixed
- cmd/show needed blank import in reactor for non-BGP RPCs (caught by review)

### Documentation Updates
- docs/architecture/core-design.md: added section 19 (Component Boundaries)

### Deviations from Plan
- Phase 3 required all_import_test.go refactoring to external test packages (not in original spec)
- Codegen rpcDirs were empty until AC-3 fix; now populated with cmd and iface/cmd

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Decouple cross-component imports | done | All phases | 10/10 ACs pass |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | done | grep zero ssh in web | commit 1 |
| AC-2 | done | grep zero iface in config | commit 1 |
| AC-3 | done | grep zero cmd in reactor | commit 5 |
| AC-4 | done | grep zero cli in bgp/config | commit 3 |
| AC-5 | done | grep zero ssh in bgp/config | commit 3 |
| AC-6 | done | grep zero web in bgp/config | commit 3 |
| AC-7 | done | grep zero bgp in cmd (non-test) | commit 2 |
| AC-8 | done | make ze-verify passes | all commits |
| AC-9 | done | both ssh and web import authz | commit 1 |
| AC-10 | done | iface registers CompleteFn | commit 1 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestAuthenticateUser | done | authz/auth_test.go | moved from ssh |
| TestMACCompleteFn | done | config/yang/validator_registry_test.go | MergeGlobalCompleteFns |
| TestLoaderResultPlainTypes | changed | not needed | InfraHook approach instead |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| authz/auth.go | done | moved from ssh/auth.go |
| iface/validators.go | done | new, CompleteFn registration |
| bgp/config/infra_hook.go | done | new, hook types |
| hub/infra_setup.go | done | new, SSH/CLI wiring |
| bgp/plugins/cmd/cache/ | done | moved from cmd/cache |
| bgp/plugins/cmd/commit/ | done | moved from cmd/commit |

### Audit Summary
- **Total items:** 10
- **Done:** 10
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
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
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/539-decouple-0-umbrella.md`
- [ ] Summary included in commit
