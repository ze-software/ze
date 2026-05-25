# Spec: rbac-audit -- Harden RBAC and Audit Across Every Surface

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 10/10 |
| Updated | 2026-05-25 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `internal/component/aaa/aaa.go` -- AAA interfaces (Authenticator, Authorizer, Accountant)
4. `internal/component/authz/authz.go` -- profile-based RBAC Store
5. `internal/component/plugin/server/command.go:215-383` -- Dispatcher auth + accounting
6. `internal/component/api/engine.go` -- APIEngine auth checker
7. `internal/component/web/auth.go` -- web session + auth middleware
8. `internal/component/web/handler_config.go` -- config handlers (username check only)
9. `internal/component/api/rest/server.go:412-443` -- REST withAuth (no-auth "api" identity)
10. `internal/component/tacacs/accounting.go` -- best-effort queue

## Task

Harden RBAC and audit across every surface of Ze. CLI authorization is strong and
profile-based (`authz.Store.Authorize` via `Dispatcher.isAuthorized`). Web config
handlers check for an authenticated username but do not check RBAC profiles before
editing. REST allows a no-auth "api" identity when no authenticator or token is
configured. TACACS+ accounting is best-effort and can silently drop queued records.

A top NOS needs one audit path for CLI, SSH, web, REST, gRPC, MCP, plugin dispatch,
config commit, rollback, and lifecycle operations.

### Scope

This spec covers three layers:

1. **RBAC enforcement gaps** -- surfaces that authenticate but skip authorization
2. **Unified audit trail** -- structured, append-only local audit log for all mutation surfaces
3. **Accounting reliability** -- make TACACS+ record drops visible and recoverable

### Out of scope

- Fleet audit trail (covered by `spec-fleet-3-audit-trail.md`, depends on fleet registry)
- New AAA backends (RADIUS, LDAP) -- pluggable via existing `aaa.Default.Register`
- Web RBAC UI (profile management page) -- separate spec
- MCP OAuth scope-to-profile mapping -- separate spec

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- Section 19 (component boundaries), aaa/authz components
  -> Decision: authz owns RBAC profiles and local users; aaa owns pluggable backend contract
  -> Constraint: authz imports aaa (not reverse); components communicate through interfaces
- [ ] `docs/architecture/web-interface.md` -- web handler model
  -> Constraint: web uses session-based auth; HTMX requests get OOB fragments
- [ ] `docs/architecture/api/architecture.md` -- API engine as shared backend for REST/gRPC
  -> Decision: all API commands go through APIEngine.Execute/Stream with AuthChecker

### Learned Summaries
- [ ] `plan/learned/390-rbac.md` -- RBAC library built but noted "not wired end-to-end"
  -> Constraint: single authorization chokepoint is Dispatcher.Dispatch()
- [ ] `plan/learned/598-aaa-registry.md` -- pluggable AAA with backend priority ordering
  -> Decision: Bundle.Authorizer is first-non-nil-wins; only Authenticator chains
- [ ] `plan/learned/601-tacacs.md` -- TACACS+ implementation with five deferred ACs (now closed)
  -> Constraint: accounting fires after auth passes, STOP via defer; failures never block

**Key insights:**
- Dispatcher.Dispatch() is the single authorization chokepoint covering SSH, CLI, API, MCP
- Web config mutations bypass the dispatcher entirely (direct EditorManager calls)
- REST no-auth path sets username="api" with full admin access
- TACACS+ accounting drops are logged at WARN but not surfaced to operators
- Config commit goes through EditorManager.Commit(), not through the dispatcher
- The aaa.Authorizer interface returns bool, not authz.Action

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/authz/authz.go` -- profile-based RBAC with Store.Authorize()
  -> Constraint: Store.Authorize fail-closed when user assignments exist
- [ ] `internal/component/plugin/server/command.go:215-383` -- Dispatcher with authorizer + accountant fields
  -> Constraint: isAuthorized nil-safe (nil = allow all); BeginAccounting nil-safe
- [ ] `internal/component/api/engine.go` -- APIEngine.Execute/Stream check auth before dispatch
  -> Constraint: auth checker is a function, not an interface; nil = skip
- [ ] `internal/component/api/rest/server.go:412-443` -- REST withAuth sets "api" default
  -> Constraint: no authenticator AND no token = "api" identity with full access
- [ ] `internal/component/web/handler_config.go:93-190` -- HandleConfigSet checks username != ""
  -> Constraint: username presence is the only check; no profile/RBAC check
- [ ] `internal/component/web/auth.go` -- AuthMiddleware sets username in context
  -> Constraint: web knows nothing about authz.Store or profiles after login
- [ ] `internal/component/web/handler.go` -- Tier enum (TierView, TierConfig, TierAdmin)
  -> Decision: URL tiers exist in the type system but are not enforced against profiles
- [ ] `internal/component/tacacs/accounting.go:74-84` -- queue full drops with WARN log
  -> Constraint: queue is 64-deep; drops silently when full or stopped
- [ ] `internal/component/web/cli.go:181-214` -- HandleCLICommand dispatches through dispatcher
  -> Decision: web CLI bar goes through dispatcher (has auth + accounting)
- [ ] `internal/component/mcp/handler.go` -- MCP dispatches through CommandDispatcher(cmd, username, remoteAddr)
  -> Decision: MCP commands go through dispatcher (has auth + accounting)

**Behavior to preserve:**
- `authz.Store.Authorize` fail-closed semantics when user assignments configured
- Dispatcher single-chokepoint architecture for CLI/SSH/API/MCP commands
- TACACS+ accounting never blocks command execution
- Web session-based auth with 24h TTL
- REST/gRPC constant-time token comparison
- MCP OAuth flow and bearer token auth
- Config transaction protocol (verify/apply/commit/rollback events)

**Behavior to change:**
- Web config handlers (set, add, delete, rename, commit, discard) must check RBAC profiles before mutating
- REST no-auth mode must not silently grant admin access
- TACACS+ accounting drops must be counted and surfaceable
- Config commit must emit an audit record with actor identity
- Lifecycle operations (daemon reload, restart) must emit audit records

## Data Flow (MANDATORY)

### Entry Point
- User request arrives at one of: SSH, web HTTP, REST HTTP, gRPC, MCP JSON-RPC, local CLI

### Transformation Path
1. **Transport authentication** -- SSH: password/pubkey via aaa.Authenticator; Web: session cookie or basic auth; REST/gRPC: bearer token or per-user authenticator; MCP: bearer token or OAuth
2. **Identity extraction** -- username + remoteAddr placed into request context (different mechanisms per transport)
3. **Authorization check** -- CLI/SSH/API/MCP: Dispatcher.isAuthorized(ctx, command, readOnly); Web config: MISSING (only username != "" check)
4. **Command execution** -- Dispatcher.Dispatch() or EditorManager.SetValue/Commit/etc.
5. **Accounting record** -- Dispatcher.BeginAccounting() wraps execution with START/STOP; Web config: MISSING

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Transport -> Identity | Context values (web: ctxKeyUsername, REST: usernameKey, gRPC: metadata) | [ ] |
| Identity -> Authorization | Dispatcher.isAuthorized() or APIEngine.auth() | [ ] |
| Authorization -> Accounting | Dispatcher.BeginAccounting() wraps handler | [ ] |
| Web -> EditorManager | Direct method calls (SetValue, Commit, etc.) | [ ] |

### Integration Points
- `aaa.Authorizer` interface -- consumed by Dispatcher, needs to be consumed by web
- `aaa.Accountant` interface -- consumed by Dispatcher, needs to be consumed by web + audit log
- `authz.Store.Authorize` -- underlying RBAC engine behind aaa.Authorizer
- `EditorManager` -- web config mutation entry point (no auth hooks currently)
- Config transaction events (VerifyEvent, CommittedEvent, RollbackEvent) -- could carry actor

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Web POST /config/set/ with read-only profile user | -> | RBAC check in web config handler | `TestWebConfigSetRBACDeny` |
| Web POST /config/commit/ with read-only profile user | -> | RBAC check in web config commit | `TestWebConfigCommitRBACDeny` |
| REST no-auth mode request | -> | Restricted "api" identity | `TestRESTNoAuthRestricted` |
| Config commit via web | -> | Audit record emitted | `TestWebConfigCommitAuditRecord` |
| TACACS+ queue full | -> | Drop counter incremented | `TestTacacsAccountingDropCounter` |
| Config commit via any surface | -> | Audit record with actor | `test/plugin/audit-config-commit.ci` |
| Web config set with RBAC deny | -> | 403 response | `test/web/rbac-web-config-deny.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Web user with read-only profile POSTs to /config/set/ | 403 Forbidden; config unchanged |
| AC-2 | Web user with read-only profile POSTs to /config/add/ | 403 Forbidden; config unchanged |
| AC-3 | Web user with read-only profile POSTs to /config/delete/ | 403 Forbidden; config unchanged |
| AC-4 | Web user with read-only profile POSTs to /config/rename/ | 403 Forbidden; config unchanged |
| AC-5 | Web user with read-only profile POSTs to /config/commit/ | 403 Forbidden; config unchanged |
| AC-6 | Web user with read-only profile POSTs to /config/discard/ | 403 Forbidden; config unchanged |
| AC-7 | Web user with admin profile POSTs to /config/set/ | 200 OK; config changed (existing behavior preserved) |
| AC-8 | REST request with no authenticator AND no token configured | "api" identity gets read-only access (not admin) |
| AC-9 | Config commit via any surface (CLI, web, API) | Audit record emitted with: timestamp, actor, surface, action="config-commit", summary of changes |
| AC-10 | Config rollback/discard via any surface | Audit record emitted with: timestamp, actor, surface, action="config-discard" |
| AC-11 | Daemon reload (SIGHUP or command) | Audit record emitted with: timestamp, actor, surface, action="daemon-reload" |
| AC-12 | TACACS+ accounting queue drops N records | Drop count accessible via `ze show aaa accounting` or equivalent |
| AC-13 | Audit log queried via `ze show audit` | Returns structured audit records, filterable by time range and action type |
| AC-14 | Audit log persists across daemon restarts | Stored to disk, survives restart |
| AC-15 | Web user with view-only access GETs /show/ pages | 200 OK (read access unaffected by RBAC enforcement) |
| AC-16 | Failed authentication attempts (all surfaces) | Audit record emitted with: timestamp, source IP, surface, action="auth-fail", username attempted |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWebConfigSetRBACDeny` | `internal/component/web/handler_config_test.go` | AC-1: read-only profile denied on set | |
| `TestWebConfigAddRBACDeny` | `internal/component/web/handler_config_test.go` | AC-2: read-only profile denied on add | |
| `TestWebConfigDeleteRBACDeny` | `internal/component/web/handler_config_test.go` | AC-3: read-only profile denied on delete | |
| `TestWebConfigRenameRBACDeny` | `internal/component/web/handler_config_test.go` | AC-4: read-only profile denied on rename | |
| `TestWebConfigCommitRBACDeny` | `internal/component/web/handler_config_test.go` | AC-5: read-only profile denied on commit | |
| `TestWebConfigDiscardRBACDeny` | `internal/component/web/handler_config_test.go` | AC-6: read-only profile denied on discard | |
| `TestWebConfigSetAdminAllowed` | `internal/component/web/handler_config_test.go` | AC-7: admin profile allowed on set | |
| `TestRESTNoAuthReadOnly` | `internal/component/api/rest/server_test.go` | AC-8: no-auth "api" gets read-only | |
| `TestAuditRecordConfigCommit` | `internal/component/audit/audit_test.go` | AC-9: commit produces audit record | |
| `TestAuditRecordConfigDiscard` | `internal/component/audit/audit_test.go` | AC-10: discard produces audit record | |
| `TestAuditRecordDaemonReload` | `internal/component/audit/audit_test.go` | AC-11: reload produces audit record | |
| `TestTacacsDropCounter` | `internal/component/tacacs/accounting_test.go` | AC-12: drop counter increments | |
| `TestAuditQueryTimeRange` | `internal/component/audit/audit_test.go` | AC-13: query by time range | |
| `TestAuditPersistence` | `internal/component/audit/audit_test.go` | AC-14: records survive write + read cycle | |
| `TestWebShowUnaffectedByRBAC` | `internal/component/web/handler_config_test.go` | AC-15: view tier unblocked | |
| `TestAuditAuthFailRecord` | `internal/component/audit/audit_test.go` | AC-16: auth-fail recorded | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Audit log max entries | 100-100000 | 100000 | 99 | 100001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rbac-web-config-deny` | `test/web/rbac-web-config-deny.ci` | Read-only web user cannot edit config | |
| `audit-config-commit` | `test/plugin/audit-config-commit.ci` | Config commit produces audit entry visible via CLI | |
| `audit-auth-fail` | `test/plugin/audit-auth-fail.ci` | Failed SSH login produces audit entry | |
| `rest-no-auth-readonly` | `test/plugin/rest-no-auth-readonly.ci` | REST without token is limited to read-only | |

### Interop Tests (MANDATORY for protocol features)
N/A -- no wire protocol changes.

### Future (if deferring any tests)
- Web RBAC UI for profile management (separate spec, user-approved)
- MCP OAuth scope-to-profile mapping (separate spec)
- RADIUS/LDAP backend integration tests (when backends implemented)

## Files to Modify

- `internal/component/web/handler_config.go` -- add RBAC checks to all config mutation handlers
- `internal/component/web/auth.go` -- carry profile info in session context (or expose authorizer)
- `internal/component/web/handler.go` -- enforce tier-to-profile mapping
- `internal/component/api/rest/server.go` -- restrict no-auth "api" identity to read-only
- `internal/component/tacacs/accounting.go` -- add drop counter (atomic uint64)
- `internal/component/plugin/server/command.go` -- emit audit records for lifecycle commands
- `cmd/ze/hub/infra_setup.go` -- wire audit log into hub lifecycle

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | [x] | `internal/component/audit/schema/ze-audit-conf.yang` |
| CLI commands/flags | [x] | `cmd/ze/audit/main.go` (`ze show audit`) |
| CLI grammar (action before identifier) | [x] | `show audit` follows existing pattern |
| Editor autocomplete | [x] | YANG-driven (automatic if YANG updated) |
| Functional test for new RPC/API | [x] | `test/plugin/audit-config-commit.ci` |
| Doctor check for runtime dependencies | [ ] | N/A -- no external dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` -- add audit log feature |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` -- audit section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` -- `show audit` |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/security.md` or new `docs/guide/audit.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` -- audit row |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` -- audit component |

## Files to Create

- `internal/component/audit/audit.go` -- audit log engine (append-only, structured records)
- `internal/component/audit/store.go` -- disk persistence (JSON-lines file)
- `internal/component/audit/query.go` -- query/filter by time, action, actor
- `internal/component/audit/schema/ze-audit-conf.yang` -- YANG schema for audit config
- `internal/component/audit/schema/embed.go` -- YANG embed
- `internal/component/audit/schema/register.go` -- YANG init registration
- `cmd/ze/audit/main.go` -- `ze show audit` CLI command
- `test/web/rbac-web-config-deny.ci` -- web RBAC functional test
- `test/plugin/audit-config-commit.ci` -- audit record functional test
- `test/plugin/audit-auth-fail.ci` -- auth-fail audit functional test
- `test/plugin/rest-no-auth-readonly.ci` -- REST no-auth restriction test

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Wiring phase | Wiring Test table -- register entry points, write failing wiring tests |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register audit component, RBAC middleware for web
   - Tests: `TestWebConfigSetRBACDeny`, `TestAuditRecordConfigCommit` (wiring stubs)
   - Files: `internal/component/audit/audit.go`, web handler RBAC skeleton
   - Verify: wiring tests fail because logic is a stub

2. **Phase: Web RBAC enforcement** -- add authorization checks to all web config handlers
   - Tests: `TestWebConfigSetRBACDeny`, `TestWebConfigAddRBACDeny`, `TestWebConfigDeleteRBACDeny`, `TestWebConfigRenameRBACDeny`, `TestWebConfigCommitRBACDeny`, `TestWebConfigDiscardRBACDeny`, `TestWebConfigSetAdminAllowed`, `TestWebShowUnaffectedByRBAC`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/auth.go`
   - Verify: AC-1 through AC-7, AC-15

3. **Phase: REST no-auth hardening** -- restrict "api" identity to read-only when no auth configured
   - Tests: `TestRESTNoAuthReadOnly`
   - Files: `internal/component/api/rest/server.go`
   - Verify: AC-8

4. **Phase: Audit log engine** -- structured append-only audit log with disk persistence
   - Tests: `TestAuditRecordConfigCommit`, `TestAuditRecordConfigDiscard`, `TestAuditRecordDaemonReload`, `TestAuditQueryTimeRange`, `TestAuditPersistence`, `TestAuditAuthFailRecord`
   - Files: `internal/component/audit/audit.go`, `store.go`, `query.go`
   - Verify: AC-9, AC-10, AC-11, AC-14, AC-16

5. **Phase: Audit CLI + YANG** -- `ze show audit` command and YANG config
   - Tests: functional test `audit-config-commit.ci`
   - Files: `cmd/ze/audit/main.go`, YANG schema files
   - Verify: AC-13

6. **Phase: TACACS+ drop counter** -- expose accounting drop statistics
   - Tests: `TestTacacsDropCounter`
   - Files: `internal/component/tacacs/accounting.go`
   - Verify: AC-12

7. **Phase: Integration wiring** -- connect audit log to all surfaces
   - Tests: all functional tests
   - Files: `cmd/ze/hub/infra_setup.go`, dispatcher, web handlers
   - Verify: end-to-end audit coverage

8. **Functional tests** -- create after feature works
9. **Full verification** -- `make ze-verify`
10. **Complete spec** -- fill audit tables, write learned summary, delete spec

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | RBAC deny returns correct HTTP status (403 for web, ErrUnauthorized for API) |
| Naming | Audit record fields use kebab-case JSON keys |
| Data flow | Web config mutations go through RBAC before EditorManager; audit records emitted after successful mutations |
| CLI grammar | `show audit` follows action-before-identifier |
| Rule: no-layering | No duplicate authorization logic; web calls same aaa.Authorizer as dispatcher |
| Rule: fail-closed | No-auth mode defaults to restricted, not admin |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| Web RBAC enforcement | `grep -r 'Authorize\|authoriz' internal/component/web/handler_config.go` |
| REST no-auth restriction | `grep 'read.*only\|ReadOnly' internal/component/api/rest/server.go` |
| Audit log package | `ls internal/component/audit/audit.go` |
| Audit CLI command | `ls cmd/ze/audit/main.go` |
| YANG schema | `ls internal/component/audit/schema/ze-audit-conf.yang` |
| TACACS+ drop counter | `grep 'drop.*count\|Drop' internal/component/tacacs/accounting.go` |
| Functional tests | `ls test/web/rbac-web-config-deny.ci test/plugin/audit-config-commit.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | Audit query time range parameters validated; no injection in filter strings |
| Authorization bypass | Verify no web config handler path skips RBAC check (grep for direct EditorManager calls) |
| Fail-closed | REST no-auth defaults to read-only; web with no profiles denies config mutations |
| Audit tampering | Audit log file permissions (0600); no user-facing delete/truncate API |
| Information leakage | Auth-fail audit records do not include passwords; audit queries require authenticated user |
| Rate limiting | Auth-fail audit records bounded (no disk exhaustion from brute-force) |
| TOCTOU | RBAC check and config mutation are not separated by async boundary |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
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

### Key Design Decision: Web RBAC via Authorizer Injection

The web config handlers currently check `username == ""` for authentication. RBAC
enforcement should NOT duplicate the authorization logic. Instead, inject the same
`aaa.Authorizer` (backed by `authz.Store`) that the Dispatcher uses. Each config
mutation handler maps to a synthetic command string for authorization:

| Web handler | Synthetic command | ReadOnly |
|-------------|-------------------|----------|
| HandleConfigSet | `config set` | false |
| HandleConfigAdd | `config add` | false |
| HandleConfigDelete | `config delete` | false |
| HandleConfigRename | `config rename` | false |
| HandleConfigCommit | `config commit` | false |
| HandleConfigDiscard | `config discard` | false |
| HandleConfigView | `show config` | true |
| HandleConfigChanges | `show config changes` | true |
| HandleConfigAddForm | `show config` | true |

This reuses the existing profile matching (prefix, regex, word boundary) without
any new authorization primitives.

### Key Design Decision: Audit Log as Component

The audit log is a new component (`internal/component/audit/`) following the
registration pattern. It exposes a simple `Record(entry AuditEntry)` interface.
The hub wires it to surfaces via hooks (web: middleware, dispatcher: similar to
accounting, config: transaction events).

Audit entries are structured:

| Field | Type | Description |
|-------|------|-------------|
| timestamp | time.Time | When the action occurred |
| actor | string | Username (or "api", "system") |
| remote-addr | string | Source IP:port |
| surface | string | "ssh", "web", "rest", "grpc", "mcp", "cli", "system" |
| action | string | "config-commit", "config-discard", "daemon-reload", "auth-fail", etc. |
| detail | string | Action-specific detail (command, change summary, etc.) |
| outcome | string | "success", "denied", "error" |

Storage: JSON-lines file under the config directory. Configurable max entries
with oldest-pruned-on-overflow (ring buffer semantics). Not syslog, not a
database -- follows the same self-contained philosophy as ZeFS.

### Key Design Decision: REST No-Auth Restriction

When no authenticator and no token are configured, the "api" identity currently
gets full admin access. Change: the "api" identity gets a read-only profile by
default. Operators who want unauthenticated write access must explicitly configure
`api.rest.unauthenticated-write true` in YANG config. This is a behavior change
but a security improvement; the current default is dangerous for any
non-development deployment.

## RFC Documentation

N/A -- no protocol-level RFCs involved.

## Implementation Summary

### What Was Implemented
- Web config RBAC checks use the existing `aaa.Authorizer` interface before direct editor mutations. Synthetic commands cover `config set`, `config add`, `config delete`, `config rename`, `config commit`, `config discard`, `config rollback`, and `config save`.
- Web terminal config mode now runs the same authorization mapping before mutating a draft and records successful terminal commit, discard, and rollback actions.
- REST and gRPC no-auth mode now admits the default `api` identity as read-only. Command execution uses command metadata to reject writes, and direct config-session endpoints reject writes before mutation.
- `pluginserver.IsReadOnlyPath` now treats `daemon status` as read-only while daemon lifecycle commands such as `daemon reload` are write operations.
- `internal/component/audit` provides a local structured audit recorder with in-memory retention, JSON-lines persistence, and query filters for action, actor, surface, since, until, and count.
- Audit records are emitted for config commit/discard from web, web terminal, REST, gRPC, and SSH/CLI model paths, daemon reload through dispatcher/SIGHUP/managed push paths, and failed auth from web, REST, gRPC, SSH, and MCP.
- `show audit` exposes audit records online through the show command registry and `ze-cli-show-cmd.yang`.
- TACACS+ accounting now counts dropped START/STOP records, and `show aaa accounting` exposes `dropped-records`.
- Hub wiring creates one audit log, registers the show provider, and passes the recorder into web, API, SSH, MCP, dispatcher, reload, and managed commit paths.
- Functional coverage was added for REST no-auth read-only behavior, audit commit/discard, auth-fail auditing, audit persistence/reload auditing, web RBAC denial, and TACACS+ accounting visibility.

### Bugs Found/Fixed
- REST and gRPC direct config-session endpoints bypassed command dispatch, so API read-only enforcement had to guard session create/set/delete/commit/discard directly.
- `daemon ...` was previously classified as read-only as a prefix. This would have let no-auth API callers run lifecycle mutations once read-only mode relied on command metadata.
- Web terminal mode bypassed normal web config handlers, so adding RBAC only to `handler_config.go` was insufficient.
- Audit detail for commit/discard must be captured before the mutation because commit/discard clears the candidate diff.
- TACACS+ queue drops were only WARN logs; operators had no runtime counter.

### Documentation Updates
- `docs/guide/audit.md` documents local audit storage, actions, surfaces, and `show audit` filters.
- `docs/guide/README.md` links the audit guide.
- `docs/guide/api.md` documents no-auth API read-only behavior.
- `docs/guide/configuration.md` documents web/API RBAC effects and audit coverage.
- `docs/guide/tacacs.md` documents `show aaa accounting` and `dropped-records`.
- `docs/features.md` and `docs/guide/command-reference.md` were updated earlier in the branch for audit and show command visibility.
- `docs/architecture/core-design.md` documents the audit component boundary and allowed imports. Only the audit-related hunks are part of this spec closure.

### Deviations from Plan
- `show audit` was implemented as an online show RPC in `internal/component/cmd/show/`, not as a new offline `cmd/ze/audit/main.go`, because the existing command tree and pipe handling already cover operator queries.
- No audit YANG config schema was added. The implementation uses the default hub-owned audit path next to the config file, with memory-only behavior for empty or `-` config paths.
- The functional web RBAC test lives at `test/plugin/rbac-web-config-deny.ci`, not `test/web/rbac-web-config-deny.ci`, because it needs daemon/plugin orchestration.
- The spec mentioned a possible `api.rest.unauthenticated-write true` escape hatch. It was not added; no-auth API writes now fail closed instead of adding a new insecure knob.
- The implementation also covered gRPC no-auth hardening and auth-fail audit even where individual early test rows named only REST or SSH.
- Full verification is not clean in the recorded evidence. Targeted unit and functional suites passed, then `make ze-verify-changed` passed changed Go package checks but failed later in broader functional verification. The user later requested commit-only with no more tests.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| RBAC enforcement gaps | Implemented | `internal/component/web/handler_config.go`, `internal/component/web/cli_terminal.go`, `internal/component/api/engine.go`, `internal/component/api/rest/server.go`, `internal/component/api/grpc/server.go` | Web direct mutations and API no-auth paths are guarded before mutation. |
| Unified audit trail | Implemented | `internal/component/audit/`, `cmd/ze/hub/audit.go`, `internal/component/cmd/show/audit.go` | One hub-owned recorder is wired to transport and lifecycle surfaces. |
| Accounting reliability | Implemented | `internal/component/tacacs/accounting.go`, `internal/component/cmd/show/aaa.go` | Drops are counted and exposed as `dropped-records`. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Implemented | `TestWebConfigSetRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web set returns 403 and draft remains unchanged. |
| AC-2 | Implemented | `TestWebConfigAddRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web add returns 403. |
| AC-3 | Implemented | `TestWebConfigDeleteRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web delete returns 403 and value remains. |
| AC-4 | Implemented | `TestWebConfigRenameRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web rename returns 403. |
| AC-5 | Implemented | `TestWebConfigCommitRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web commit returns 403 and draft remains pending. |
| AC-6 | Implemented | `TestWebConfigDiscardRBACDeny`, `test/plugin/rbac-web-config-deny.ci` | Read-only web discard returns 403 and draft remains pending. |
| AC-7 | Implemented | `TestWebConfigSetAdminAllowed` | Admin profile set path still mutates draft. |
| AC-8 | Implemented | `TestEngineReadOnlyCallerDeniedWrite`, `TestRESTNoAuthReadOnly`, `TestGRPCNoAuthReadOnly`, `test/plugin/rest-no-auth-readonly.ci` | No-auth API reads work; write commands and config sessions fail. |
| AC-9 | Implemented | `TestWebConfigCommitAuditRecord`, `TestTerminalModeCommitAuditRecord`, `TestRESTConfigCommitAuditRecord`, `TestGRPCConfigCommitAuditRecord`, CLI audit tests, `test/plugin/audit-config-commit.ci` | Commit records include actor, surface, outcome, and diff/detail. |
| AC-10 | Implemented | `TestWebConfigDiscardAuditRecord`, `TestTerminalModeDiscardAuditRecord`, `TestRESTConfigDiscardAuditRecord`, `TestGRPCConfigDiscardAuditRecord`, CLI audit tests, `test/plugin/audit-config-commit.ci` | Discard/rollback records use `config-discard`. |
| AC-11 | Implemented | `TestDispatcherDaemonReloadAuditRecord`, `test/plugin/audit-persistence.ci` | Dispatcher and daemon reload paths record `daemon-reload`. |
| AC-12 | Implemented | `TestTacacsAccountingDropCounter`, `TestHandleShowAAAAccounting`, `test/plugin/tacacs-acct.ci` | Drop counter is queryable through `show aaa accounting`. |
| AC-13 | Implemented | `TestHandleShowAuditFilters`, `test/plugin/audit-config-commit.ci` | `show audit` supports action and time-range filters. |
| AC-14 | Implemented | `internal/component/audit` persistence tests, `test/plugin/audit-persistence.ci` | JSON-lines audit file is loaded after restart. |
| AC-15 | Implemented | `TestWebShowUnaffectedByRBAC`, `test/plugin/rbac-web-config-deny.ci` | `/show/` remains available to view-only users. |
| AC-16 | Implemented | `TestLoginHandlerAuthFailureAuditRecord`, `TestRESTAuthFailureAuditRecord`, `TestGRPCAuthFailureAuditRecord`, `TestSSHAuthFailureAuditRecord`, MCP auth tests, `test/plugin/audit-auth-fail.ci` | Failed auth records include attempted actor, remote address, surface, and denied outcome. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Web RBAC unit tests | Implemented | `internal/component/web/handler_config_test.go` | Covers set/add/delete/rename/commit/discard/admin/view paths. |
| API read-only unit tests | Implemented | `internal/component/api/engine_test.go`, `internal/component/api/rest/server_test.go`, `internal/component/api/grpc/server_test.go` | Covers command and config-session denial. |
| Audit engine unit tests | Implemented | `internal/component/audit/audit_test.go` | Covers record, query, retention, and persistence behavior. |
| Show command tests | Implemented | `internal/component/cmd/show/show_test.go` | Covers `show audit` filters and `show aaa accounting`. |
| TACACS+ drop counter test | Implemented | `internal/component/tacacs/accounting_test.go` | Covers queue-full drop count. |
| Auth-fail unit tests | Implemented | `internal/component/web/auth_test.go`, `internal/component/api/rest/server_test.go`, `internal/component/api/grpc/server_test.go`, `internal/component/ssh/ssh_test.go`, `internal/component/mcp/*_test.go` | Covers all implemented auth surfaces. |
| Functional tests | Implemented | `test/plugin/rest-no-auth-readonly.ci`, `test/plugin/audit-config-commit.ci`, `test/plugin/audit-auth-fail.ci`, `test/plugin/audit-persistence.ci`, `test/plugin/rbac-web-config-deny.ci`, `test/plugin/tacacs-acct.ci` | Selected functional suite passed 6/6 in recorded evidence. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/web/handler_config.go` | Implemented | Web mutation RBAC and web config commit/discard audit. |
| `internal/component/web/auth.go` | Implemented | Failed login/basic-auth audit. |
| `internal/component/web/handler.go` | Not changed | Route tier model did not need profile mapping for this scope. |
| `internal/component/api/rest/server.go` | Implemented | Read-only caller enforcement, config-session guards, REST audit. |
| `internal/component/api/grpc/server.go` | Implemented | gRPC parity with REST for read-only and audit. |
| `internal/component/tacacs/accounting.go` | Implemented | Atomic drop counter. |
| `internal/component/plugin/server/command.go` | Implemented | Daemon reload audit and lifecycle read-only classification. |
| `cmd/ze/hub/infra_setup.go` | Implemented | Audit recorder and AAA accounting provider wiring. |
| `internal/component/audit/audit.go` | Implemented | Entry, constants, recorder, memory log. |
| `internal/component/audit/store.go` | Implemented | JSON-lines persistence. |
| `internal/component/audit/query.go` | Implemented | Query filters. |
| `internal/component/audit/schema/*` | Not added | Deviated: audit is not user-configured yet. |
| `cmd/ze/audit/main.go` | Not added | Deviated: online `show audit` was implemented under show RPC. |
| `test/web/rbac-web-config-deny.ci` | Not added | Deviated: functional test lives under `test/plugin/`. |

### Audit Summary
- **Total items:** 16 acceptance criteria, 3 task requirements, 13 planned file groups.
- **Implemented:** 16 acceptance criteria, 3 task requirements.
- **Partial:** full repo verification evidence is not clean; targeted verification passed.
- **Skipped:** no acceptance criterion was intentionally skipped.
- **Changed:** audit command location, audit config schema, functional test location, and no-auth write escape hatch are documented in Deviations.

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Web RBAC enforcement | functional test | `test/plugin/rbac-web-config-deny.ci` passed in selected functional suite |
| REST no-auth hardening | functional test | `test/plugin/rest-no-auth-readonly.ci` |
| Unified audit trail | functional test | `test/plugin/audit-config-commit.ci`, `test/plugin/audit-persistence.ci`, and `test/plugin/audit-auth-fail.ci` |
| TACACS+ drop visibility | unit and functional test | `TestTacacsAccountingDropCounter`, `test/plugin/tacacs-acct.ci` |
| Auth-fail auditing | functional test | `test/plugin/audit-auth-fail.ci` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | No separate `/ze-review` report was run in this recorded session. | Whole spec | Manual audit and targeted tests were recorded below. |
| 2 | ISSUE | Full `make ze-verify-changed` did not finish cleanly after targeted tests passed. | Verification | Do not claim full verification; user requested no more tests before commit. |

### Fixes applied
- Web terminal mode received RBAC and audit coverage after identifying it as a bypass of `handler_config.go`.
- API direct config-session endpoints received read-only guards after identifying that they bypass command dispatch.
- `daemon` read-only classification was narrowed to avoid treating lifecycle mutations as reads.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Not re-run. | Whole spec | User requested commit-only with no additional tests. |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [x] All NOTEs recorded above

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/audit/audit.go` | Yes | File exists in working tree. |
| `internal/component/audit/query.go` | Yes | File exists in working tree. |
| `internal/component/audit/store.go` | Yes | File exists in working tree. |
| `cmd/ze/hub/audit.go` | Yes | File exists in working tree. |
| `internal/component/cmd/show/audit.go` | Yes | File exists in working tree. |
| `internal/component/cmd/show/aaa.go` | Yes | File exists in working tree. |
| `docs/guide/audit.md` | Yes | File exists in working tree. |
| `test/plugin/rest-no-auth-readonly.ci` | Yes | File exists in working tree. |
| `test/plugin/audit-config-commit.ci` | Yes | File exists in working tree and was already tracked at this checkpoint. |
| `test/plugin/audit-auth-fail.ci` | Yes | File exists in working tree. |
| `test/plugin/audit-persistence.ci` | Yes | File exists in working tree. |
| `test/plugin/rbac-web-config-deny.ci` | Yes | File exists in working tree. |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-7, AC-15 | Web RBAC protects mutation and preserves show access. | Unit tests in `internal/component/web/handler_config_test.go`; functional `test/plugin/rbac-web-config-deny.ci`. |
| AC-8 | No-auth REST/gRPC callers are read-only. | `TestRESTNoAuthReadOnly`, `TestGRPCNoAuthReadOnly`, `test/plugin/rest-no-auth-readonly.ci`. |
| AC-9..AC-11, AC-13..AC-14 | Audit records are written and queryable for config/lifecycle actions. | `internal/component/audit/*_test.go`, `TestHandleShowAuditFilters`, `test/plugin/audit-config-commit.ci`, `test/plugin/audit-persistence.ci`. |
| AC-12 | TACACS+ drops are counted and shown. | `TestTacacsAccountingDropCounter`, `TestHandleShowAAAAccounting`, `test/plugin/tacacs-acct.ci`. |
| AC-16 | Auth failures are audited across surfaces. | Web/REST/gRPC/SSH/MCP auth-fail tests and `test/plugin/audit-auth-fail.ci`. |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Web read-only profile POST to config mutation endpoints | `test/plugin/rbac-web-config-deny.ci` | Selected functional suite passed. |
| REST no-auth read and write attempts | `test/plugin/rest-no-auth-readonly.ci` | Selected functional suite passed. |
| REST config commit/discard audit queried through show command | `test/plugin/audit-config-commit.ci` | Selected functional suite passed. |
| SSH auth failure audit queried through show command | `test/plugin/audit-auth-fail.ci` | Selected functional suite passed. |
| Audit persistence and daemon reload audit across restart | `test/plugin/audit-persistence.ci` | Selected functional suite passed. |
| TACACS+ accounting and `show aaa accounting` | `test/plugin/tacacs-acct.ci` | Selected functional suite passed. |

### Verification Evidence

| Command | Result |
|---------|--------|
| `go test ./internal/component/api/... ./internal/component/plugin/server ./internal/component/audit ./internal/component/web ./internal/component/cli ./internal/component/tacacs ./internal/component/cmd/show ./internal/component/mcp ./internal/component/ssh ./cmd/ze/hub` | Passed in recorded evidence. |
| `go run ./cmd/ze-test bgp plugin -p 1 -t 60s -v rest-no-auth-readonly audit-config-commit audit-auth-fail rbac-web-config-deny audit-persistence tacacs-acct` | Passed, 6/6. |
| `make ze-verify-changed` | Changed Go package checks passed; broader functional verification failed later. Do not treat this as full verification passed. |
| Additional tests after commit request | Not run. User requested no tests, commit only. |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-16 all demonstrated
- [x] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [x] Feature code integrated (`internal/*`, `cmd/*`)
- [x] Integration completeness proven end-to-end with selected functional tests
- [x] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added
- [x] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [x] Tests written
- [ ] Tests FAIL (paste output)
- [x] Tests PASS (targeted evidence recorded above)
- [x] Boundary tests for all numeric inputs
- [x] Functional tests for end-to-end behavior
- [x] Interop tests for protocol features (N/A, no wire protocol changes)
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [x] Partial/Skipped items have user approval for no additional tests before commit
- [x] Implementation Summary filled
- [x] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [x] Write learned summary to `plan/learned/780-rbac-audit.md`
- [x] **Summary included in commit plan** -- two-commit closure requested by user: implementation plus completed spec first, learned summary plus spec deletion second.
