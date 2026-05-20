# Spec: zefs-remote-credentials

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/9 |
| Updated | 2026-05-20 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/init/main.go` - writes SSH credentials to zefs
4. `cmd/ze/internal/ssh/client/client.go` - reads SSH credentials from zefs
5. `pkg/zefs/store.go` - BlobStore API

## Task

Restructure zefs SSH credential storage from flat `meta/ssh/*` to per-service `meta/ssh/<host>/<port>/*`. Add `ze remote` CLI for managing remote daemon credentials. Update all consumers.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/zefs-format.md` - ZeFS file format
  -> Constraint: netcapstring framing, key hierarchy
- [ ] `docs/architecture/system-architecture.md` - SSH client/server design
  -> Decision: CLI tools connect to daemon via SSH

### Related Learned Summaries
- [ ] `plan/learned/380-ssh-server.md` - SSH server implementation

**Key insights:**
- Current keys: `meta/ssh/username`, `meta/ssh/password`, `meta/ssh/host`, `meta/ssh/port`
- `ze init` writes these, `LoadCredentials` reads them, `loader.go` reads username/password for config SSH auth
- `meta/ssh/default` stores `<host>/<port>` pointer to the active credential set
- `ze init` writes credentials AND sets the default pointer
- `LoadCredentials` without `--remote` follows the default pointer
- Key hierarchy uses host+port as composite key (same host may run multiple daemons)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/init/main.go` - writes `meta/ssh/{username,password,host,port}` during bootstrap. Password is bcrypt-hashed. Reads from stdin (piped or interactive).
- [ ] `cmd/ze/internal/ssh/client/client.go` - `LoadCredentials()` reads from zefs. Env vars `ze.ssh.host`, `ze.ssh.port`, `ze.ssh.password` override zefs values. Defaults: host=127.0.0.1, port=2222.
- [ ] `internal/component/bgp/config/loader.go:1010` - reads `meta/ssh/username` and `meta/ssh/password` for config SSH authentication.
- [ ] `pkg/zefs/store.go` - `BlobStore` with `ReadFile(key)` and `WriteFile(key, data, perm)`. Keys are path-like strings.
- [ ] `cmd/ze/internal/ssh/client/client_test.go` - tests with `meta/ssh/*` keys
- [ ] `cmd/ze/init/main_test.go` - tests writing and reading `meta/ssh/*` keys

**Consumers of `meta/ssh/*`:**

| File | Keys used | Operation |
|------|-----------|-----------|
| `cmd/ze/init/main.go:27-30` | all four | write |
| `cmd/ze/internal/ssh/client/client.go:139-165` | all four | read |
| `internal/component/bgp/config/loader.go:1010-1014` | username, password | read |
| `cmd/ze/internal/ssh/client/client_test.go` | all four | test fixtures |
| `cmd/ze/init/main_test.go` | all four | test assertions |
| `internal/component/config/storage/storage_test.go:729` | username | test fixture |
| `pkg/zefs/store_test.go:4558` | password | test fixture |

**Behavior to preserve:**
- `ze init` interactive/piped credential input
- Env var overrides (`ze.ssh.host`, `ze.ssh.port`, `ze.ssh.password`)
- Bcrypt password hashing
- All existing CLI tools that use `LoadCredentials`

**Behavior to change:**
- Key scheme: `meta/ssh/<host>/<port>/{username,password}` instead of `meta/ssh/{username,password,host,port}`
- `ze init` writes to `meta/ssh/127.0.0.1/2222/*` by default (or specified host/port)
- `LoadCredentials` resolves host+port, then reads `meta/ssh/<host>/<port>/*`
- Port defaults to 2222 when not specified
- New `ze remote` CLI for managing remote credentials
- `loader.go` reads from new key paths

## Data Flow (MANDATORY)

### Entry Point -- `ze init`
1. User runs `ze init` (or `ze init --host 10.0.1.5 --port 2222`)
2. Reads username, password from stdin
3. Writes to `meta/ssh/<host>/<port>/username`, `meta/ssh/<host>/<port>/password`
4. Writes `<host>/<port>` to `meta/ssh/default`
5. Default host: `127.0.0.1`, default port: `2222`

### Entry Point -- `ze remote add`
1. User runs `ze remote add <host> [--port N] [--user name]`
2. Prompts for password (or reads from stdin)
3. Writes to `meta/ssh/<host>/<port>/username`, `meta/ssh/<host>/<port>/password`

### Entry Point -- `ze remote default`
1. User runs `ze remote default <host> [--port N]`
2. Writes `<host>/<port>` to `meta/ssh/default`
3. All subsequent commands without `--remote` target this daemon

### Entry Point -- `LoadCredentials`
1. If `--remote` flag: use that host:port directly
2. Else if env `ze.ssh.host`/`ze.ssh.port`: use those
3. Else: read `meta/ssh/default` -> get `<host>/<port>`
4. Read `meta/ssh/<host>/<port>/password` (env `ze.ssh.password` overrides)
5. Read `meta/ssh/<host>/<port>/username`
6. Return `Credentials{Host, Port, Username, Auth}`

### Entry Point -- CLI tools with `--remote`
1. User runs `ze cli --remote 10.0.1.5:2222` (or `ze show --remote 10.0.1.5:2222`)
2. Looks up `meta/ssh/10.0.1.5/2222/*` in zefs
3. Connects to that remote daemon instead of localhost

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> zefs | `BlobStore.ReadFile` / `WriteFile` | [ ] |
| CLI -> daemon | SSH with credentials from zefs | [ ] |

### Integration Points
- `cmd/ze/init/main.go` - update key constants
- `cmd/ze/internal/ssh/client/client.go` - update `ReadCredentials` key paths
- `internal/component/bgp/config/loader.go` - update credential read path
- All CLI tools accepting `--remote` flag

### Architectural Verification
- [ ] No bypassed layers -- all credential access through zefs
- [ ] No unintended coupling -- host resolution in `LoadCredentials` only
- [ ] No duplicated functionality -- single credential read path
- [ ] Zero-copy preserved -- N/A (config/credential layer)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze init` | -> | writes `meta/ssh/<host>/<port>/*` + sets default | `test/parse/init-credentials.ci` |
| `ze remote add <host> --port <port>` | -> | writes `meta/ssh/<host>/<port>/*` | `test/parse/remote-add.ci` |
| `ze remote default <host> --port <port>` | -> | writes `meta/ssh/default` | `test/parse/remote-default.ci` |
| `ze remote list` | -> | reads `meta/ssh/*/*/` + default | `test/parse/remote-list.ci` |
| `ze cli --remote <host>:<port>` | -> | `LoadCredentials(<host>, <port>)` | `test/plugin/cli-remote.ci` |
| `ze cli` (no --remote) | -> | `LoadCredentials()` follows default | `test/plugin/cli-default.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze init` with defaults | Writes credentials to `meta/ssh/127.0.0.1/2222/{username,password}` and sets `meta/ssh/default` to `127.0.0.1/2222` |
| AC-2 | `ze remote add 10.0.1.5 --user admin` (no `--port`) | Writes to `meta/ssh/10.0.1.5/2222/{username,password}` (default port) |
| AC-3 | `LoadCredentials()` with no flags/env, default points to `127.0.0.1/2222` | Follows default pointer, reads from `meta/ssh/127.0.0.1/2222/*` |
| AC-4 | `LoadCredentials()` with `ze.ssh.host=10.0.1.5`, `ze.ssh.port=2223` | Env overrides default pointer, reads from `meta/ssh/10.0.1.5/2223/*` |
| AC-5 | `ze remote add 10.0.1.5 --port 2223 --user admin` | Writes credentials to `meta/ssh/10.0.1.5/2223/{username,password}` |
| AC-6 | `ze remote list` | Lists all host:port entries with stored credentials, marks default |
| AC-7 | `ze remote remove 10.0.1.5 --port 2223` | Deletes `meta/ssh/10.0.1.5/2223/*` |
| AC-8 | `ze remote default 10.0.1.5 --port 2223` | Sets `meta/ssh/default` to `10.0.1.5/2223` |
| AC-9 | `ze cli --remote 10.0.1.5:2223` | Connects to 10.0.1.5:2223 using stored credentials |
| AC-10 | `ze cli --remote unknown-host:2222` | Error: no credentials for host:port |
| AC-11 | `loader.go` SSH auth | Follows default pointer, reads credentials from resolved path |
| AC-12 | `ze remote default` with no credentials stored for target | Error: no credentials for host:port |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReadCredentialsPerService` | `cmd/ze/internal/ssh/client/client_test.go` | Reads from `meta/ssh/<host>/<port>/*` | |
| `TestReadCredentialsDefaultPort` | `cmd/ze/internal/ssh/client/client_test.go` | Uses default port 2222 when not specified | |
| `TestReadCredentialsEnvOverride` | `cmd/ze/internal/ssh/client/client_test.go` | Env host+port selects different credential set | |
| `TestInitWritesPerService` | `cmd/ze/init/main_test.go` | Writes to `meta/ssh/127.0.0.1/2222/*` | |
| `TestRemoteAdd` | `cmd/ze/remote/main_test.go` | Writes remote credentials | |
| `TestRemoteList` | `cmd/ze/remote/main_test.go` | Lists hosts | |
| `TestRemoteRemove` | `cmd/ze/remote/main_test.go` | Deletes remote credentials | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `init-credentials` | `test/parse/init-credentials.ci` | `ze init` writes per-host credentials | |
| `remote-add` | `test/parse/remote-add.ci` | `ze remote add` stores remote credentials | |
| `remote-list` | `test/parse/remote-list.ci` | `ze remote list` shows stored hosts | |

### Future (if deferring any tests)
- `cli-remote` functional test -- requires two daemons running, may need test infrastructure

## Files to Modify

- `cmd/ze/init/main.go` - update key constants to `meta/ssh/<host>/<port>/*`
- `cmd/ze/internal/ssh/client/client.go` - update `ReadCredentials` to per-service lookup
- `internal/component/bgp/config/loader.go:1010-1014` - update credential key paths
- `cmd/ze/internal/ssh/client/client_test.go` - update test fixtures
- `cmd/ze/init/main_test.go` - update test assertions
- `internal/component/config/storage/storage_test.go:729` - update test fixture
- `pkg/zefs/store_test.go:4558` - update test fixture

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A |
| CLI commands/flags | Yes | `cmd/ze/remote/main.go` -- new subcommand |
| CLI `--remote` flag | Yes | `cmd/ze/cli/main.go`, `cmd/ze/show/main.go`, `cmd/ze/run/main.go`, etc. |
| Editor autocomplete | No | N/A |
| Functional test | Yes | `test/parse/init-credentials.ci`, `test/parse/remote-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` -- remote daemon management |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` -- `ze remote`, `--remote` flag |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/remote-management.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- `cmd/ze/remote/main.go` -- `ze remote` subcommand (add, list, remove)
- `test/parse/init-credentials.ci` -- init writes per-host credentials
- `test/parse/remote-add.ci` -- remote add stores credentials
- `test/parse/remote-list.ci` -- remote list shows hosts

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Credential read** -- update `ReadCredentials` to read `meta/ssh/<host>/<port>/*`
   - Tests: `TestReadCredentialsPerService`, `TestReadCredentialsDefaultPort`, `TestReadCredentialsEnvOverride`
   - Files: `client.go`, `client_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Init update** -- update `ze init` to write `meta/ssh/<host>/<port>/*`
   - Tests: `TestInitWritesPerService`
   - Files: `init/main.go`, `init/main_test.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Loader update** -- update `loader.go` credential reads
   - Tests: existing loader tests
   - Files: `loader.go`
   - Verify: tests pass

4. **Phase: `ze remote` CLI** -- add, list, remove subcommands
   - Tests: `TestRemoteAdd`, `TestRemoteList`, `TestRemoteRemove`
   - Files: new `cmd/ze/remote/main.go`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: `--remote` flag** -- add to CLI tools
   - Tests: functional tests
   - Files: `cmd/ze/cli/main.go` and other CLI entry points
   - Verify: functional tests pass

6. **Phase: Test fixture updates** -- update remaining test fixtures
   - Files: `storage_test.go`, `store_test.go`

7. **Functional tests** -> create .ci tests
8. **Full verification** -> `make ze-verify`
9. **Complete spec** -> audit, learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All AC-N implemented with file:line |
| Default port | Omitting port uses 2222 everywhere (AC-2) |
| Security | Passwords remain bcrypt-hashed, never logged |
| No regressions | All existing CLI tools work with new key scheme |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze init` writes per-service keys | `test/parse/init-credentials.ci` passes |
| `ze remote add` works | `test/parse/remote-add.ci` passes |
| `ze remote list` works | `test/parse/remote-list.ci` passes |
| Default port behavior | `TestReadCredentialsDefaultPort` passes |
| Existing tools unbroken | `make ze-verify` passes |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Password storage | Bcrypt hash, never plaintext in zefs |
| Password display | `ze remote list` never shows passwords |
| Credential leakage | No passwords in logs, error messages, or test output |
| File permissions | zefs database file permissions unchanged |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Old tests fail with new keys | Phase 1 -- key path construction wrong |
| `ze init` writes wrong path | Phase 2 -- key constant wrong |
| `loader.go` can't find credentials | Phase 3 -- key path mismatch |
| `ze remote` parse error | Phase 4 -- flag parsing |
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

## RFC Documentation

N/A -- no protocol changes.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

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
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
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

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
