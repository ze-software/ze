# Spec: login-warnings

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-23 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/component/ssh/session.go` -- createSessionModel, initial statusMessage
3. `internal/component/ssh/ssh.go` -- SSH server teaHandler, SetExecutorFactory pattern
4. `internal/component/cli/model.go` -- NewModel/NewCommandModel, statusMessage, messageLines()
5. `internal/component/bgp/config/loader.go:550-577` -- SSH wiring in post-start hook
6. `internal/component/bgp/reactor/session_prefix.go` -- IsPrefixDataStale()

→ Decision: warnings flow via closure injection (SetLoginWarnings), same pattern as SetExecutorFactory
→ Decision: LoginWarning type lives in cli package (consumed by model rendering)
→ Decision: no registry -- single closure composes all providers (YAGNI)
→ Decision: wiring test is Go unit test, not .ci (SSH BubbleTea sessions not .ci-testable)
→ Constraint: ssh/session.go already imports cli package -- no new import cycle risk

## Task

Warning system for SSH login. When an operator connects via SSH, ze checks
for conditions requiring attention and displays warnings in the welcome area.
Each warning includes a message and an actionable command to resolve it.

First provider: prefix data staleness (N peers have stale prefix data).

## Design

### LoginWarning Type

| Field | Type | Purpose |
|-------|------|---------|
| Message | string | Human-readable warning, e.g. "3 peer(s) have stale prefix data" |
| Command | string | Actionable command to resolve, e.g. "ze update bgp peer * prefix" |

Defined in `cli` package (where rendering happens). SSH server references via `cli.LoginWarning`.

### Warning Injection (Closure Pattern)

~~Providers register on the plugin server (which has the reactor reference).
Each subsystem calls `server.RegisterWarningProvider(fn)` during init.~~
Superseded: plugin server is not accessible from SSH server. Use closure injection instead.

The SSH server receives a `LoginWarningsFunc` via `SetLoginWarnings(fn)`,
following the established pattern used by `SetExecutorFactory`,
`SetStreamingExecutorFactory`, `SetShutdownFunc`, and `SetRestartFunc`.

The daemon's post-start hook in `loader.go` creates the closure, capturing
the reactor reference. The closure calls `reactor.Peers()`, iterates peers,
and builds warnings from any staleness conditions found.

This approach:
1. Follows the existing SSH wiring pattern exactly
2. Keeps SSH server decoupled from plugin server and reactor
3. Avoids a registry abstraction for a single provider (YAGNI)
4. Future providers: add more checks inside the same closure in loader.go

### LoginWarningsFunc Type

A function type `func() []LoginWarning` stored on SSH Server.
Called during `createSessionModel` to collect warnings for the session.
Returns nil when no warnings exist.

### CLI Display

At SSH session creation, `createSessionModel` calls the warnings function.
Warnings are rendered below the welcome text using `warningLineStyle`
(yellow on dark background, already defined in model.go).

Display format:

    Welcome to ze, thomas!

    warning: 3 peer(s) have stale prefix data
      run: ze update bgp peer * prefix

    ze>

Multiple warnings render as consecutive blocks separated by blank lines.

### Prefix Staleness Provider Logic

1. Call `reactor.Peers()` to get all peer snapshots
2. For each peer, check `PrefixUpdated` field using `IsPrefixDataStale(updated, now)`
   (already exists in `reactor/session_prefix.go`)
3. Count peers with stale prefix data
4. If count > 0, return one LoginWarning with count and command
5. If count == 0, return nil

### Future Providers

| Provider | Condition | Command |
|----------|-----------|---------|
| Prefix staleness | PrefixUpdated > 6 months | `ze update bgp peer * prefix` |
| RPKI cache | Cache age > threshold | `ze update rpki` (future) |
| Software version | New ze version available | `ze update check` (future) |

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `internal/component/ssh/session.go` -- createSessionModel builds cli.Model for SSH session. Two paths: editor-capable (NewModel with Editor) or command-only (NewCommandModel). Editor path sets statusMessage to "welcome to ze!" or "welcome to ze, <user>!". Command-only path sets no statusMessage. Both paths wire cmdCompleter, executor, shutdownFn, restartFn.
- [x] `internal/component/ssh/ssh.go` -- teaHandler creates session via createSessionModel(username). SSH Server struct holds executorFactory, streamingExecutorFactory, shutdownFunc, restartFunc -- all injected via Set* methods. Server has no reference to plugin server or reactor.
- [x] `internal/component/cli/model.go` -- NewModel sets statusMessage to welcome string. NewCommandModel sets no statusMessage. messageLines() in model_render.go checks for "welcome" prefix to apply welcomeStyle. warningLineStyle already defined (yellow on dark). Model.statusMessage is the temporary message shown above viewport.
- [x] `internal/component/bgp/config/loader.go:550-577` -- post-start hook wires SSH: SetExecutorFactory (closure over apiServer + username), SetStreamingExecutorFactory, SetShutdownFunc, SetRestartFunc. This is where SetLoginWarnings wiring belongs.
- [x] `internal/component/plugin/types.go` -- PeerInfo has PrefixUpdated string field (ISO date YYYY-MM-DD). ReactorIntrospector interface has Peers() method returning []PeerInfo.
- [x] `internal/component/bgp/reactor/session_prefix.go` -- IsPrefixDataStale(updated string, now time.Time) bool. Checks if timestamp is older than 6 months. Already tested (session_prefix_test.go).

**Behavior to preserve:**
- Existing welcome message format ("welcome to ze!" / "welcome to ze, <user>!")
- SSH session creation speed (warning check must be fast -- Peers() is O(n), n=peer count)
- CLI model initialization (NewModel, NewCommandModel unchanged signatures)
- warningLineStyle appearance
- messageLines() priority: error > status > idle

**Behavior to change:**
- SSH Server gets new SetLoginWarnings method (closure injection)
- createSessionModel calls warnings func, passes results to model
- Model stores and renders login warnings below welcome message
- loader.go post-start hook wires warnings closure

## Data Flow (MANDATORY)

### Entry Point
1. Daemon starts, reactor launches, post-start hook fires in loader.go
2. Hook creates warnings closure capturing reactor reference
3. Hook calls `sshSrv.SetLoginWarnings(fn)` (alongside SetExecutorFactory)
4. Operator connects via SSH
5. `teaHandler()` calls `createSessionModel(username)`
6. `createSessionModel` calls stored `loginWarningsFunc()` to collect warnings
7. Warnings passed to Model (stored on Model struct)
8. Model renders warnings in `messageLines()` when statusMessage has "welcome" prefix
9. Operator sees warnings on first render

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> Closure | loader.go captures reactor, calls Peers() | [ ] |
| SSH Server -> loginWarningsFunc | Stored via SetLoginWarnings, called in createSessionModel | [ ] |
| Warnings -> CLI Model | Stored on Model struct, rendered in messageLines() | [ ] |

## Wiring Test (MANDATORY)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| createSessionModel with injected loginWarningsFunc | -> | Warning rendered in model View() | internal/component/ssh/session_test.go:TestCreateSessionModelWithWarnings |
| NewModel/NewCommandModel with warnings | -> | Warnings appear in messageLines() output | internal/component/cli/model_test.go:TestModelDisplaysLoginWarnings |

Note: SSH BubbleTea sessions are not testable via `.ci` framework (no PTY simulation).
Go unit tests verify the wiring from createSessionModel through to model rendering.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | SSH login with 3 stale peers | Welcome shows "3 peer(s) have stale prefix data" and "ze update bgp peer * prefix" |
| AC-2 | SSH login with no stale peers | No warning in welcome |
| AC-3 | SSH login with no peers configured | No warning |
| AC-4 | Warning includes actionable command | Message includes `run: ze update bgp peer * prefix` |
| AC-5 | loginWarningsFunc is nil (pre-reactor-start) | No crash, no warnings, normal welcome |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestLoginWarningsStalePeers | internal/component/ssh/session_test.go | createSessionModel with stale peers returns model with warnings (AC-1) | [ ] |
| TestLoginWarningsNoneStale | internal/component/ssh/session_test.go | createSessionModel with fresh peers returns model without warnings (AC-2) | [ ] |
| TestLoginWarningsNoPeers | internal/component/ssh/session_test.go | createSessionModel with no peers returns model without warnings (AC-3) | [ ] |
| TestLoginWarningsNilFunc | internal/component/ssh/session_test.go | createSessionModel with nil loginWarningsFunc returns normal model (AC-5) | [ ] |
| TestModelDisplaysLoginWarnings | internal/component/cli/model_test.go | Model with loginWarnings renders them in View() output (AC-1, AC-4) | [ ] |
| TestModelNoLoginWarnings | internal/component/cli/model_test.go | Model without loginWarnings renders normal welcome (AC-2) | [ ] |
| TestPrefixStalenessWarning | internal/component/ssh/session_test.go | Staleness closure with mixed peers returns correct count | [ ] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| (none) | N/A | SSH BubbleTea sessions not testable via .ci framework | N/A |

## Files to Modify

- `internal/component/ssh/ssh.go` -- add loginWarningsFunc field, SetLoginWarnings method
- `internal/component/ssh/session.go` -- createSessionModel calls loginWarningsFunc, passes to model
- `internal/component/cli/model.go` -- add loginWarnings field to Model, accept in NewModel/NewCommandModel, render in messageLines()
- `internal/component/bgp/config/loader.go` -- wire SetLoginWarnings in post-start hook

## Files to Create

- `internal/component/cli/warnings.go` -- LoginWarning type definition
- `internal/component/ssh/warnings.go` -- LoginWarningsFunc type, prefixStalenessWarnings helper

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- login warnings |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin SDK changed? | No | |
| 6 | User guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK rules? | No | |
| 9 | RFC behavior? | No | |
| 10 | Test infrastructure? | No | |
| 11 | Comparison table? | No | |
| 12 | Internal architecture? | No | |

## Implementation Steps

### Implementation Phases

| Phase | What |
|-------|------|
| 1 | LoginWarning type in cli package, LoginWarningsFunc type in ssh package |
| 2 | Model accepts and renders login warnings (TDD: write tests, then implement) |
| 3 | SSH Server SetLoginWarnings + createSessionModel wiring (TDD) |
| 4 | Prefix staleness closure + loader.go wiring (TDD) |

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Performance | Warning check fast (Peers() is O(n), n=peer count, no network calls) |
| No crash | Nil loginWarningsFunc, empty peers, nil reactor -- all safe |
| Display | Warnings visible, styled with warningLineStyle, include command |
| Pattern consistency | SetLoginWarnings follows same pattern as SetExecutorFactory |
| No import cycles | ssh imports cli (already does), cli defines LoginWarning |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Warnings displayed at login | TestModelDisplaysLoginWarnings in model_test.go |
| No warnings when fresh | TestLoginWarningsNoneStale in session_test.go |
| Staleness detection correct | TestPrefixStalenessWarning in session_test.go |
| Nil func safe | TestLoginWarningsNilFunc in session_test.go |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| No sensitive data in warnings | Warning messages are operational, not credential-related |
| Provider execution bounded | No network calls in warning closure, only in-memory Peers() |
| No injection | Warning messages are hardcoded format strings, not user-supplied |

### Failure Routing

| Failure | Route To |
|---------|----------|
| loginWarningsFunc panics | Recover in createSessionModel, log error, continue without warnings |
| SSH session creation slow | Profile warning function execution |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| SSH server has access to plugin server | SSH server is decoupled, uses closure injection | Read ssh.go and loader.go | Redesigned from registry to closure pattern |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| WarningProvider registry on plugin server | SSH server cannot access plugin server (no import, no reference) | Closure injection via SetLoginWarnings, wired in loader.go |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Implementation Summary

### What Was Implemented

### Deviations from Plan

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
- **Partial:**
- **Skipped:**
- **Changed:**

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Critical Review passes

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary
- [ ] Summary included in commit
