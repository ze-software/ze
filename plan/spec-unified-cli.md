# Spec: unified-cli

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/bgp/cmd_plugin.go` - current plugin CLI entry point
4. `pkg/plugin/sdk/sdk.go` - `NewFromTLSEnv`, `Plugin.Run()` (5-stage startup)
5. `internal/component/plugin/ipc/tls.go` - `PluginAcceptor`, `handleConn()`, `WaitForPlugin()`
6. `internal/component/plugin/process/process.go` - `startExternal()` fork + TLS connect-back

## Completed Work

Phases 1-5 of the original spec are implemented:

| Phase | What | Key files |
|-------|------|-----------|
| 1. Relocate package | `config/editor/` -> `cli/`, rename `editor` -> `cli` | `internal/component/cli/*.go` (41 files) |
| 2. Unify `ze cli` | Deleted own model, uses `cli.NewCommandModel()` | `cmd/ze/cli/main.go` (393L) |
| 3. Unify SSH | Deleted `SessionModel`, creates `cli.Model` with optional editor | `internal/component/ssh/session.go` (102L) |
| 4. Ctrl+Arrow scroll | `tea.KeyCtrlUp`/`KeyCtrlDown` page scrolling | `internal/component/cli/model.go:527` |
| 5. Plugin CLI (partial) | `ze bgp plugin cli` connects via SSH, uses `PluginCompleter` | `cmd/ze/bgp/cmd_plugin.go` (105L), `completer_plugin.go` (74L) |

Design decisions from completed work:
- Nil editor pattern: `hasEditor()` guards ~20 code paths for command-only mode
- Plugin CLI connects via SSH (`sshclient.ExecCommand`), not direct socket
- SSH sessions get full edit mode when ConfigPath + Storage configured, command-only fallback otherwise

## Task

Add auto and manual 5-stage plugin negotiation modes to `ze bgp plugin cli`.

Currently `ze bgp plugin cli` connects via SSH and enters command mode. Two additional modes would let users simulate a real text-mode plugin's 5-stage handshake:

- **Auto mode:** Connect via TLS, perform the 5-stage handshake automatically using the SDK, then enter interactive command mode
- **Manual mode:** Same TLS connection, but show each stage in the TUI for user review/editing before sending

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: text mode auto-detected from first byte on Socket A
  -> Decision: stages use newline-separated lines terminated by blank line
- [ ] `docs/architecture/api/text-format.md` - post-stage-5 event format
  -> Constraint: text events are one line per event, `bye` signals shutdown

### Related Learned Summaries
- [ ] `plan/learned/380-ssh-server.md` - SSH server implementation
  -> Constraint: SSH uses Wish middleware chain

**Key insights:**
- TLS plugin infrastructure exists: `PluginAcceptor` in `ipc/tls.go`, `NewFromTLSEnv` in `sdk.go`
- External plugins connect via TLS, authenticate with `#0 auth {"token":"...","name":"..."}`, engine responds `#0 ok`
- SDK `Plugin.Run()` handles all 5 stages automatically (`sdk.go:217-259`)
- **Gap:** `PluginAcceptor.handleConn()` (`tls.go:345-349`) drops connections for names nobody called `WaitForPlugin()` for. The engine only calls `WaitForPlugin` for plugins it launched. An unsolicited CLI connection authenticates but gets closed.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/bgp/cmd_plugin.go` (105L) - current plugin CLI: connects via SSH, creates `cli.NewCommandModel()` with `PluginCompleter`, executes commands via `sshclient.ExecCommand`
- [ ] `internal/component/cli/completer_plugin.go` (74L) - `PluginCompleter` with 10 SDK methods (update-route, dispatch-command, subscribe-events, etc.)
- [ ] `pkg/plugin/sdk/sdk.go` - `NewFromTLSEnv()` reads env vars, dials TLS, authenticates, returns plugin. `Plugin.Run()` executes all 5 stages then enters event loop.
- [ ] `internal/component/plugin/ipc/tls.go` - `PluginAcceptor`: TLS listener, auth via `Authenticate()`, routes by name via `WaitForPlugin()`. `handleConn()` at line 345 does `pending.LoadAndDelete(name)` -- if no waiter, closes connection.
- [ ] `internal/component/plugin/process/process.go` - `startExternal()` forks plugin, passes TLS host/port/token via env, calls `acceptor.WaitForPlugin(ctx, name)` with 30s timeout.
- [ ] `internal/component/plugin/manager/manager.go` - `ensureAcceptor()` creates TLS listener + token when external plugins configured.

**Behavior to preserve:**
- `ze bgp plugin cli` default mode (SSH connection, command mode with plugin completions)
- Plugin SDK method completions (all 10 methods in `PluginCompleter`)
- SSH credential loading and error handling
- `PluginAcceptor` behavior for normal plugin connections (engine-launched)

**Behavior to change:**
- Add `auto` and `manual` flags to `ze bgp plugin cli`
- Auto mode: connect via TLS as a plugin, perform 5-stage handshake, enter interactive mode
- Manual mode: same connection, show each stage interactively for review
- `PluginAcceptor` must accept unsolicited plugin connections (not just pre-registered names)

## Data Flow (MANDATORY)

### Entry Point -- Auto Mode
1. User runs `ze bgp plugin cli auto`
2. CLI queries daemon (via SSH) for hub address + token, OR reads env vars
3. CLI calls `sdk.NewFromTLSEnv("cli-session")` -- dials TLS, authenticates
4. Acceptor routes connection to a handler (new: dynamic accept path)
5. Engine-side creates plugin session for this connection
6. SDK `Plugin.Run()` performs 5-stage handshake automatically
7. After stage 5: enters interactive command mode with `PluginCompleter`

### Entry Point -- Manual Mode
1. Same connection setup as auto (steps 1-5)
2. Each stage shown in TUI viewport with the JSON payload
3. User reviews/edits, presses Enter to send
4. Engine responds, next stage shown
5. After all 5 stages: enters interactive command mode

### Transformation Path
1. CLI connects to hub TLS listener, authenticates with token
2. Acceptor routes to dynamic handler (new infrastructure)
3. Engine creates plugin session (ProcessManager or equivalent)
4. Stage 1: plugin sends `ze-plugin-engine:declare-registration`
5. Stage 2: engine sends `ze-plugin-callback:configure`
6. Stage 3: plugin sends `ze-plugin-engine:declare-capabilities`
7. Stage 4: engine sends `ze-plugin-callback:share-registry`
8. Stage 5: plugin sends `ze-plugin-engine:ready`
9. Interactive: user types SDK methods, sent as MuxConn RPCs

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> hub | TLS connection + `#0 auth` | [ ] |
| Plugin protocol stages | MuxConn RPCs (`#N method json`) | [ ] |
| Interactive commands | MuxConn RPCs via `callEngine` | [ ] |

### Integration Points
- `cmd/ze/bgp/cmd_plugin.go` -- add auto/manual dispatch, TLS connection
- `internal/component/plugin/ipc/tls.go` -- accept unsolicited connections
- `internal/component/plugin/process/` or `server/` -- create session for ad-hoc plugins
- `pkg/plugin/sdk/sdk.go` -- `NewFromTLSEnv` already works, `Plugin.Run()` already works

### Architectural Verification
- [ ] No bypassed layers -- uses standard TLS auth + 5-stage plugin protocol
- [ ] No unintended coupling -- CLI uses public SDK, not internal engine types
- [ ] No duplicated functionality -- reuses SDK handshake logic for auto mode
- [ ] Zero-copy preserved -- text protocol, no wire encoding involved

## Design Decision Needed

The `PluginAcceptor` currently drops connections for unrecognized plugin names (`tls.go:345-349`). Options:

| Approach | How it works | Pros | Cons |
|----------|-------------|------|------|
| **A. Dynamic accept callback** | Add `OnUnexpectedPlugin(func(name, conn))` to `PluginAcceptor`. When no `WaitForPlugin` waiter exists, call the callback instead of closing. | Minimal change to acceptor. Engine decides what to do. | Acceptor gains new responsibility. |
| **B. Pre-register via SSH RPC** | CLI sends "prepare-plugin-session" RPC over SSH. Engine calls `WaitForPlugin("cli-session-XXXX")`. CLI then connects via TLS with that name. | No acceptor changes. Uses existing `WaitForPlugin` flow. | Two-step connection (SSH then TLS). |
| **C. Accept-all mode** | Add a channel-based catch-all to `PluginAcceptor` for any name not in `pending`. | Simple. | Weaker security -- any authenticated connection accepted. |

**Recommendation:** Approach B is safest and changes the least. The CLI already connects via SSH, so querying for hub credentials + registering the name is one extra RPC before the TLS connection.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze bgp plugin cli auto` | -> | TLS connect + 5-stage negotiation + cli.Model | `test/plugin/plugin-cli-auto.ci` |
| `ze bgp plugin cli manual` | -> | TLS connect + interactive stages + cli.Model | `test/plugin/plugin-cli-manual.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-9 | `ze bgp plugin cli auto` | Connects to daemon via TLS, performs 5-stage negotiation automatically, enters interactive command mode with plugin SDK completions |
| AC-10 | `ze bgp plugin cli manual` | Connects to daemon via TLS, shows each stage message in viewport, user edits/confirms with Enter, after all 5 stages enters interactive mode |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginCLIAutoNegotiation` | `cmd/ze/bgp/cmd_plugin_test.go` | Auto mode completes all 5 stages via SDK | |
| `TestPluginCLIManualStageDisplay` | `cmd/ze/bgp/cmd_plugin_test.go` | Manual mode shows stage content, waits for user input | |
| `TestAcceptorDynamicPlugin` | `internal/component/plugin/ipc/tls_test.go` | Acceptor handles unsolicited plugin connection (approach-dependent) | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-cli-auto` | `test/plugin/plugin-cli-auto.ci` | `ze bgp plugin cli auto` completes negotiation and enters interactive mode | |
| `plugin-cli-manual` | `test/plugin/plugin-cli-manual.ci` | `ze bgp plugin cli manual` shows stages, user confirms each | |

### Future (if deferring any tests)
- Plugin CLI chaos/timeout tests -- deferred to advanced fault injection spec

## Files to Modify

- `cmd/ze/bgp/cmd_plugin.go` -- add auto/manual flags, TLS connection, SDK usage
- `internal/component/plugin/ipc/tls.go` -- accept unsolicited connections (approach-dependent)
- Engine-side plugin session creation (approach-dependent, likely `internal/component/plugin/server/` or `manager/`)

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Maybe | If approach B: new RPC for "prepare-plugin-session" |
| CLI commands/flags | Yes | `cmd/ze/bgp/cmd_plugin.go` -- add auto/manual |
| Editor autocomplete | No | N/A |
| Functional test | Yes | `test/plugin/plugin-cli-auto.ci`, `test/plugin/plugin-cli-manual.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` -- plugin CLI auto/manual modes |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` -- auto/manual flags |
| 4 | API/RPC added/changed? | [ ] | Depends on approach |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | `docs/guide/plugins.md` -- plugin debugging |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | Depends on approach |

## Files to Create

- `test/plugin/plugin-cli-auto.ci` -- functional test for auto negotiation mode
- `test/plugin/plugin-cli-manual.ci` -- functional test for manual negotiation mode

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

1. **Phase: Acceptor dynamic routing** -- enable `PluginAcceptor` to handle unsolicited connections (approach TBD)
   - Tests: `TestAcceptorDynamicPlugin`
   - Files: `internal/component/plugin/ipc/tls.go`, possibly `server/` or `manager/`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Auto negotiation** -- wire `ze bgp plugin cli auto` to connect via TLS and run 5-stage handshake
   - Tests: `TestPluginCLIAutoNegotiation`
   - Files: `cmd/ze/bgp/cmd_plugin.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Manual negotiation** -- wire `ze bgp plugin cli manual` to show stages interactively
   - Tests: `TestPluginCLIManualStageDisplay`
   - Files: `cmd/ze/bgp/cmd_plugin.go`
   - Verify: tests fail -> implement -> tests pass

4. **Functional tests** -- create .ci tests for both modes
5. **Full verification** -- `make ze-verify`
6. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-9 and AC-10 both implemented with file:line |
| Correctness | 5-stage handshake completes in correct order |
| Security | Unsolicited connections still require valid token |
| Data flow | Uses standard plugin protocol, no shortcuts |
| Rule: no-layering | If acceptor behavior changed, old path still works for normal plugins |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze bgp plugin cli auto` works | `test/plugin/plugin-cli-auto.ci` passes |
| `ze bgp plugin cli manual` works | `test/plugin/plugin-cli-manual.ci` passes |
| Default SSH mode still works | existing `test/plugin/cli-*.ci` tests pass |
| Normal plugin connections unaffected | existing plugin tests pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Authentication | Unsolicited connections still require valid hub token |
| Input validation | Manual mode edits must not produce malformed protocol messages |
| Resource limits | Connection timeout for incomplete handshakes |
| Name collision | Ad-hoc plugin name must not conflict with real plugin names |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Connection closed after auth | Phase 1 -- acceptor not routing unsolicited connections |
| Handshake timeout | Phase 2 -- engine-side session not created |
| Manual mode display wrong | Phase 3 -- viewport content formatting |
| Normal plugins break | Phase 1 -- regression in acceptor routing |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Daemon has no plugin connection path | TLS infrastructure exists (`ipc/tls.go`, `sdk.go`). `PluginAcceptor` handles auth + routing. Gap is only that unsolicited names are dropped. | Reading `handleConn()` line 345-349 | Spec was incorrectly marked blocked |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The TLS plugin connection infrastructure is fully built. `PluginAcceptor` in `ipc/tls.go` handles TLS 1.3 connections, auth via `#0 auth {"token":"...","name":"..."}`, and routes by name.
- The specific gap is `handleConn()` line 345: `pending.LoadAndDelete(name)` returns false for names not pre-registered via `WaitForPlugin()`. The engine only calls `WaitForPlugin` for plugins it launches.
- `sdk.NewFromTLSEnv()` + `Plugin.Run()` handle the entire client-side flow (TLS dial, auth, 5-stage handshake, event loop).
- The original spec incorrectly stated `pkg/plugin/rpc/text_mux.go` and `pkg/plugin/sdk/sdk_text.go` as key files. These do not exist. The actual plugin protocol uses `rpc.MuxConn` (in `pkg/plugin/rpc/conn.go`) with `#N method json` framing.

## RFC Documentation

N/A -- no protocol changes. Plugin protocol is internal, not RFC-governed.

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
- [ ] AC-9 and AC-10 demonstrated
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
