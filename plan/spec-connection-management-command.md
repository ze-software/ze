# Spec: an operator command to list, re-check and close live connections

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-05 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze has no command that shows the connections the daemon is serving, and no way
for an operator to close one. The work is that command: it lists the live SSH
sessions and SSE streams, re-checks each against the running configuration, and
closes the ones the operator chooses.

This is Thomas's ruling of 2026-08-08, given verbatim while closing
`spec-fixit-web-auth-deleted-user-survives-reload`: "removing a user should only
affect new SSH connection as you may be editing your own user, we should instead
have a command to management existing connections and close them/re-check them
and allowing to close them and this should be a different spec".

So a connection open at the moment a user is removed outliving the removal is
the DECIDED behavior, not a defect. It is decided on both surfaces:

- `(*Server).Reload` (`internal/component/ssh/ssh.go`) returns nil and does
  nothing. Its comment says SSH server config changes require a restart and
  there is no hot reload, so no session is torn down.
- `(*Store).authorize` (`internal/component/authz/authz.go`), which
  `(*Store).Authorize` delegates to, reads the store under a read lock. A reload
  does not rebuild the assignments map for a session already open.
- `(*EventBroker).ServeHTTP` (`internal/component/web/sse.go`) authenticates at
  connect, calls `Subscribe`, then blocks in a select on the request context,
  the client's done channel and its event channel for the life of the
  connection.

What is owed is the TOOL that makes cutting a live connection a deliberate
operator act. A new request after removal is already refused on every surface,
so nothing is wrong on the wire today. The row was triaged on 2026-08-30 as an
improvement rather than a release defect.

The SSE half is the real work. `sseClient` (`internal/component/web/sse.go`)
carries a `ch` and a `done` channel and nothing else, and `Subscribe` builds it
from no arguments, so the broker holds no identity for any subscriber. It cannot
drop one subscriber's stream today because it cannot name one. Giving a
subscriber an identity, and deciding what that identity is on a surface whose
authentication happens in middleware above the broker, is the design question
this spec has to answer before any command can be written.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/` - the command surface a new verb registers into
  → Decision: <to be filled>
- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/ssh/ssh.go` - `(*Server).Reload` returns nil with no work; its comment states SSH config changes need a restart, so no live session is re-checked or torn down
- [ ] `internal/component/web/sse.go` - `sseClient` holds only an event channel and a done channel; `Subscribe` takes no arguments, so the broker has no per-subscriber identity, and `(*EventBroker).ServeHTTP` blocks on the request context for the life of the stream
- [ ] `internal/component/authz/authz.go` - `(*Store).Authorize` delegates to `(*Store).authorize`, which reads the store under a read lock and fails closed on an empty identity; it is consulted per command, not per connection

**Behavior to preserve:** (unless the user explicitly said to change it)
- a user removal affects NEW connections only, unless the operator closes a live one deliberately (owner ruling, 2026-08-08)

**Behavior to change:** (only what the user asked for)
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- an operator command listing, re-checking and closing connections, typed in the CLI
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI command ↔ SSH server | <to be filled> | No |
| CLI command ↔ web event broker | <to be filled> | No |

### Integration Points
- `internal/component/ssh/ssh.go` - the SSH server holds the live sessions
- `internal/component/web/sse.go` - the event broker holds the live streams

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | <to be filled> | <to be filled> | <to be filled> | <to be filled> | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | an operator closes their own session and loses the CLI they are typing in | <to be filled> | <to be filled> |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | live operator sessions are dropped |
| How is it reverted? | <to be filled> |
| Who else touches this path? | `spec-login-identity-for-looking-glass-and-gnmi`, `spec-managed-server-hardening` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the connection list command | → | <to be filled> | <to be filled> |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | an operator lists connections while an SSH session and an SSE stream are open | both are listed with the identity that opened them |
| AC-2 | an operator closes a named connection | that connection ends and no other connection is disturbed |
| AC-3 | an operator re-checks connections after a user is removed | the removed user's connections are reported as no longer authorized |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | removes a user, then closes that user's live session | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| <to be filled> | <to be filled> | <to be filled> | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| <to be filled> | `test/` | <to be filled> | |

## Files to Modify
- `internal/component/web/sse.go` - give a subscriber an identity the broker can name
- `internal/component/ssh/ssh.go` - expose the live sessions and a close path

## Files to Create
- <to be filled>

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | | <to be filled> |
| CLI grammar (keyword before value) | | <to be filled> |
| Pipe completeness | | <to be filled> |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | | `docs/guide/command-reference.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- <to be filled>
2. **Phase: <to be filled>**

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Security | closing a connection is authorized, and an operator cannot close a connection they may not see |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| <to be filled> | <to be filled> |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization | the close verb is a write, and an unauthorized caller is refused |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Test fails on behavior mismatch | Re-read the source in Current Behavior |

## Known Limitations
- <to be filled>

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] `./le verify worktree` passes
