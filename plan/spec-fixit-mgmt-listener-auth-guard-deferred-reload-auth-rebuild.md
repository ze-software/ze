# Spec: fixit-mgmt-listener-auth-guard-deferred-reload-auth-rebuild

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/hub/listener_migrate.go` - `ReloadListeners`, the SIGHUP reload path
4. `plan/spec-fixit-mgmt-listener-auth-guard.md` - the source spec (if still on disk)

## Task

Make an authentication-mode change take effect on a SIGHUP config reload. Today a
running management server keeps the auth mode it was constructed with; only its
listen addresses are migrated.

**Provenance:** deferred from `plan/spec-fixit-mgmt-listener-auth-guard.md` Known
Limitations, recorded 2026-07-17. The source spec confirmed AC-7's boot-plus-migration
address guard as its shipped scope and left the auth rebuild here.

**The limitation is verified, not inherited.** `ReloadListeners`
(`cmd/ze/hub/listener_migrate.go:77-117`) builds its change set exclusively from
addresses:

| Service | What reload reads | Producer |
|---------|-------------------|----------|
| web | `endpointsToAddrs(webCfg.Servers)` | `listener_migrate.go:80-86` |
| lg | `endpointsToAddrs(lgCfg.Servers)` | `listener_migrate.go:88-94` |
| mcp | `endpointsToAddrs(mcpCfg.Servers)` | `listener_migrate.go:96-102` |
| rest / grpc | `apiListenToAddrs(apiCfg.REST / .GRPC)` | `listener_migrate.go:104-117` |

Every path funnels into `buildChange(name, srv, newAddrs)`
(`listener_migrate.go:174-189`), whose only per-service input is `newAddrs`, and the
diff it drives (`listenerDiff`, `:190-213`) compares address lists. No auth field is
read, compared, or applied, so a reload cannot rebuild a server's auth mode.

**Why it was not simply widened:** the source spec's AC-7 stops a running
*unauthenticated* listener from being migrated onto a non-loopback address, which is
the security-relevant half and needs no rebuild. Turning auth on for an
already-running server is a lifecycle change (servers are constructed once), which is
a different and larger piece of work.

**Scope note:** the source spec also parks gNMI token-over-plaintext (token set, no
TLS still boots; the guard enforces authentication, not transport secrecy). Decide
when picking this up whether that belongs here or in its own spec; it is a distinct
concern (secrecy vs identity) and is NOT currently claimed by this file.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
- [ ] `docs/architecture/api/architecture.md` - management listener construction and lifecycle
  → Constraint: [fill when picked up]

**Key insights:**
- Reload today is address-migration only; auth is fixed at construction.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
- [ ] `cmd/ze/hub/listener_migrate.go` (250L) - `ReloadListeners` (`:77-117`) migrates listen addresses only; `buildChange` (`:174-189`) takes `newAddrs` and nothing else; `listenerDiff` (`:190-213`) compares address lists.
  → Constraint: the `Reconfigurable` interface is the seam every service is driven through; an auth rebuild either extends it or replaces the server instance.

**Behavior to preserve:**
- AC-7's address guard from the source spec: a running unauthenticated listener must not migrate to a non-loopback address.
- Rollback on partial failure (`rollbackAppliedListeners`, `listener_migrate.go:162-173`).

**Behavior to change:**
- An auth-mode change in the reloaded config must take effect, rather than being silently ignored until restart.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- SIGHUP → config reload → `ReloadListeners(ctx, tree)` (`cmd/ze/hub/listener_migrate.go:77`)

### Transformation Path
1. Extract per-service config from the tree (`ExtractWebConfig` / `ExtractLGConfig` / `ExtractMCPConfig` / `ExtractAPIConfig`)
2. Build the change set — today addresses only (`buildChange`, `:174-189`)
3. Diff against running listeners (`listenerDiff`, `:190-213`)
4. Apply, with rollback on failure (`rollbackAppliedListeners`, `:162-173`)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ hub | `zeconfig.Extract*Config` | [ ] |
| Hub ↔ each service | `Reconfigurable` | [ ] |

### Integration Points
- `Reconfigurable` implementations for web, lg, mcp, rest, grpc - each would need to accept an auth-mode change.

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Registration over hardcoding — no new per-service switch in a shared package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Rebuilding a server in place can preserve in-flight connections acceptably | not yet investigated | design changes to drain-and-replace | prototype + functional test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An auth rebuild that fails halfway leaves a server unauthenticated | rollback test | extend `rollbackAppliedListeners` to cover auth, fail closed (`ai/rules/fail-closed-guards.md`) |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| SIGHUP with a changed auth mode | → | `ReloadListeners` auth path | [fill when picked up] |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config reloaded with auth turned on for a running server | The server enforces auth after reload, without a restart |
| AC-2 | Auth rebuild fails midway | Rollback leaves no server less authenticated than before |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadListenersAuthChange` | `cmd/ze/hub/listener_migrate_test.go` | an auth-mode change is applied on reload | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| [fill when picked up] | `test/plugin/*.ci` | operator turns auth on and SIGHUPs; the listener demands auth | |

### Future (if deferring any tests)
- None yet; this is a skeleton.

## Files to Modify
- `cmd/ze/hub/listener_migrate.go` - carry auth mode through the change set

## Implementation Steps

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — a failing test proving an auth change on reload is currently ignored
2. **Phase: [fill when picked up]**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
