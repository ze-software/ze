# Spec: unified-cli

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/3 |
| Updated | 2026-03-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/bgp/cmd_plugin.go` - current plugin CLI entry point
4. `internal/component/ssh/ssh.go` - SSH server, `execMiddleware`
5. `pkg/plugin/sdk/sdk.go` - `NewWithConn`, `Plugin.Run()` (5-stage startup)
6. `internal/component/plugin/server/startup.go` - `handleProcessStartupRPC` (engine-side 5-stage)

## Task

Replace `ze bgp plugin cli` with an interactive plugin debug shell. Purpose: let developers manually test plugin code by speaking the 5-stage plugin protocol against a running daemon.

The debug shell connects via SSH (auth already handled), asks the developer brief questions about handshake parameters with sensible defaults, performs the handshake over the SSH channel, then drops into interactive command mode for runtime debugging.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - 5-stage plugin protocol
  -> Constraint: wire format is `#<id> <verb> [<json>]\n`, newline-delimited
  -> Constraint: stages are barrier-synchronized via `StartupCoordinator` for normal plugins
  -> Decision: debug shell sessions run with `coordinator == nil` (barriers skipped)
- [ ] `docs/architecture/api/text-format.md` - post-stage-5 event format
  -> Constraint: text events are one line per event, `bye` signals shutdown

**Key insights:**
- SSH sessions implement io.ReadCloser + io.WriteCloser, can be wrapped as plugin transport directly via `rpc.NewConn(sess, sess)`
- `handleProcessStartupRPC()` works with `coordinator == nil` -- barriers skipped, ad-hoc sessions work
- `rpc.NewConn` accepts `io.ReadCloser`/`io.WriteCloser` (changed in spec-exabgp-bridge-muxconn)
- Q&A must happen locally (terminal) BEFORE SSH session opens -- MuxConn framing and human prompts cannot share one stream

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/bgp/cmd_plugin.go` (106L) - current plugin CLI: connects via SSH, creates `cli.NewCommandModel()` with `PluginCompleter`, one-shot commands via `sshclient.ExecCommand`
- [ ] `internal/component/cli/completer_plugin.go` (74L) - `PluginCompleter` with 10 SDK methods
- [ ] `internal/component/ssh/ssh.go` (498L) - `execMiddleware` dispatches one-shot and streaming commands. Interactive sessions go to `teaHandler`.
- [ ] `pkg/plugin/sdk/sdk.go` (349L) - `NewWithConn(name, conn)` creates plugin from net.Conn. `Plugin.Run()` executes all 5 stages then enters event loop.
- [ ] `internal/component/plugin/server/startup.go` (527L) - `handleProcessStartupRPC()` handles engine-side 5-stage protocol. Works with `coordinator == nil`.
- [ ] `internal/component/plugin/process/process.go` (688L) - Process lifecycle. `SetConn` bypasses rawConn/InitConns.

**Behavior to preserve:**
- Plugin SDK method completions (all 10 methods in `PluginCompleter`)
- SSH credential loading and error handling
- Normal plugin connections via TLS unaffected
- All existing SSH command dispatch unaffected

**Behavior to change:**
- `ze bgp plugin cli` becomes the debug shell (single flow with Q&A defaults)
- SSH server detects `plugin protocol` exec command, upgrades to bidirectional plugin transport
- Engine-side creates ad-hoc Process from SSH channel, runs 5-stage handshake + runtime handler
- CLI-side asks stage parameter questions (with defaults), then uses SDK to drive handshake
- After handshake, interactive command mode with PluginCompleter

## Data Flow (MANDATORY)

### Entry Point
1. User runs `ze bgp plugin cli`
2. CLI asks Q&A locally (terminal stdin/stdout) with defaults
3. CLI opens persistent SSH session with exec command `plugin protocol`
4. Engine-side `execMiddleware` detects `plugin protocol`, calls PluginProtocolFunc
5. Engine wraps SSH channel in `rpc.NewConn(sess, sess)`, creates ad-hoc Process
6. Engine runs `handleProcessStartupRPC(proc)` with coordinator == nil

### Transformation Path
1. CLI constructs registration/capabilities JSON from Q&A answers
2. SDK `Plugin.Run()` sends stages 1, 3, 5 (plugin-initiated)
3. Engine responds with stages 2, 4 (engine-initiated: configure, share-registry)
4. After stage 5: engine launches runtime command handler
5. CLI enters interactive mode: user types SDK methods, sent as MuxConn RPCs
6. `bye` triggers clean shutdown on both sides

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| CLI -> daemon | SSH persistent session | [ ] |
| Plugin protocol | MuxConn RPCs over SSH channel | [ ] |
| Interactive commands | MuxConn RPCs over same channel | [ ] |

### Integration Points
- `cmd/ze/bgp/cmd_plugin.go` -- Q&A, persistent SSH session, SDK handshake, interactive mode
- `internal/component/ssh/ssh.go` -- detect `plugin protocol` in `execMiddleware`
- `internal/component/plugin/server/server.go` -- `HandleAdHocPluginSession` creates Process and runs handshake

### Architectural Verification
- [ ] No bypassed layers -- uses standard 5-stage plugin protocol, SSH provides auth
- [ ] No unintended coupling -- CLI uses public SDK, engine uses existing Process/startup infrastructure
- [ ] No duplicated functionality -- reuses SDK handshake logic and existing `handleProcessStartupRPC`
- [ ] Zero-copy preserved -- text protocol, no wire encoding involved

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze bgp plugin cli` | -> | SSH connect + 5-stage negotiation + interactive mode | `test/plugin/plugin-cli-debug.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-9 | `ze bgp plugin cli` with all defaults | Connects via SSH, performs 5-stage handshake with default registration, enters interactive command mode with plugin SDK completions |
| AC-10 | `ze bgp plugin cli` with custom answers | Developer provides custom families. Handshake uses those values. |
| AC-11 | Interactive mode after handshake | Developer can type SDK methods, see responses, and `bye` to disconnect cleanly |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAdHocProcessHandshake` | `internal/component/plugin/server/adhoc_test.go` | Ad-hoc Process from io.ReadCloser/io.WriteCloser completes 5-stage handshake with coordinator == nil | |
| `TestAdHocProcessRuntime` | `internal/component/plugin/server/adhoc_test.go` | After handshake, runtime commands dispatched correctly | |
| `TestNewWithIO` | `pkg/plugin/sdk/sdk_test.go` | NewWithIO creates plugin that can send/receive RPCs | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no new numeric inputs | | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `plugin-cli-debug` | `test/plugin/plugin-cli-debug.ci` | `ze bgp plugin cli` with defaults completes handshake and enters interactive mode | |

### Future (if deferring any tests)
- Plugin CLI chaos/timeout tests -- deferred to advanced fault injection spec

## Files to Modify

- `cmd/ze/bgp/cmd_plugin.go` -- rewrite: local Q&A, persistent SSH session, SDK handshake, interactive mode
- `cmd/ze/internal/ssh/client/client.go` -- add persistent bidirectional SSH session support
- `internal/component/ssh/ssh.go` -- add `plugin protocol` detection in `execMiddleware` + PluginProtocolFunc
- `internal/component/plugin/server/server.go` -- add `HandleAdHocPluginSession` method
- `pkg/plugin/sdk/sdk.go` -- add `NewWithIO` constructor for non-net.Conn transports

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | SSH command, not YANG RPC |
| CLI commands/flags | Yes | `cmd/ze/bgp/cmd_plugin.go` |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | Yes | `test/plugin/plugin-cli-debug.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- plugin debug shell |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` -- `ze bgp plugin cli` rewrite |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md` -- plugin debugging |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | No | N/A |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `internal/component/plugin/server/adhoc.go` -- ad-hoc plugin session handler
- `test/plugin/plugin-cli-debug.ci` -- functional test for plugin debug shell

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-verify` |
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

1. **Phase: Engine-side ad-hoc session** -- SSH detection + Process handshake over SSH channel
   - Tests: `TestAdHocProcessHandshake`, `TestAdHocProcessRuntime`
   - Files: `internal/component/plugin/server/adhoc.go`, `internal/component/ssh/ssh.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: SDK + CLI** -- `NewWithIO`, persistent SSH, Q&A, interactive mode
   - Tests: `TestNewWithIO`
   - Files: `pkg/plugin/sdk/sdk.go`, `cmd/ze/bgp/cmd_plugin.go`, `cmd/ze/internal/ssh/client/client.go`
   - Verify: tests fail -> implement -> tests pass

3. **Functional tests + docs + learned summary**
   - Create `test/plugin/plugin-cli-debug.ci`
   - Write documentation updates
   - Write learned summary to `plan/learned/`
   - Full verification: `make ze-verify`

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-9, AC-10, AC-11 all implemented with file:line |
| Correctness | 5-stage handshake completes in correct order over SSH channel |
| Security | Only authenticated SSH users can enter plugin protocol mode |
| Data flow | Uses standard plugin protocol, no shortcuts |
| Rule: no-layering | Normal plugin connections via TLS unaffected |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `ze bgp plugin cli` with defaults works | `test/plugin/plugin-cli-debug.ci` passes |
| Default SSH command mode still works | existing SSH/CLI tests pass |
| Normal plugin connections unaffected | existing plugin tests pass |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Authentication | Only authenticated SSH users can enter plugin protocol mode |
| Input validation | Q&A answers must produce valid registration JSON |
| Resource limits | SSH session timeout applies to plugin protocol mode |
| Name collision | Ad-hoc plugin name must not conflict with real plugin names |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| `plugin protocol` not detected | Phase 1 -- execMiddleware detection |
| Handshake fails over SSH | Phase 1 -- Process/conn wiring |
| Q&A produces invalid registration | Phase 2 -- JSON construction |
| Interactive commands not dispatched | Phase 2 -- runtime handler wiring |
| Normal plugins break | Phase 1 -- regression |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Need TLS for plugin CLI sessions | SSH channel works directly as transport | User pointed out SSH already handles auth | Eliminated TLS/acceptor complexity |
| Need separate auto/manual modes | One flow with Q&A defaults covers both | User feedback | Simpler UX |
| Purpose was end-user plugin simulation | Purpose is developer debugging tool | User clarification | Different design priorities |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| TLS with prepare-session RPC | Overly complex | SSH channel as direct plugin transport |
| Separate auto/manual modes | Unnecessary complexity | Single flow with Enter-for-defaults |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- SSH sessions satisfy io.ReadCloser + io.WriteCloser so `rpc.NewConn(sess, sess)` works directly
- `handleProcessStartupRPC()` works with `coordinator == nil` -- key enabler for ad-hoc sessions
- Q&A must happen BEFORE SSH session opens (MuxConn and prompts cannot share same stream)
- `Process.SetConn()` bypasses rawConn/InitConns -- allows ad-hoc Process creation without net.Conn

## RFC Documentation

N/A -- no protocol changes. Plugin protocol is internal, not RFC-governed.

## Implementation Summary

### What Was Implemented
- Engine-side `HandleAdHocPluginSession` in `server/adhoc.go` -- creates ad-hoc Process from io.ReadCloser/io.WriteCloser, runs 5-stage handshake + runtime commands
- SSH server `plugin protocol` detection in `execMiddleware` with `PluginProtocolFunc` callback
- `sdk.NewWithIO(name, reader, writer)` constructor for non-net.Conn transports
- `sshclient.OpenProtocolSession` for persistent bidirectional SSH sessions
- Rewritten `cmdPluginCLI` with local Q&A, persistent SSH session, SDK handshake, interactive command mode

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/features.md` -- added plugin debug shell entry
- `docs/guide/command-reference.md` -- updated `ze bgp plugin` commands
- `docs/guide/plugins.md` -- added Debugging Plugins section

### Deviations from Plan
- ~~`.ci` functional test not created -- requires full daemon infrastructure (SSH server + plugin server wired together). Unit tests prove the core via net.Pipe. A `.ci` test belongs in a follow-up when the daemon wiring for `SetPluginProtocolFunc` is committed.~~ Resolved: `test/plugin/plugin-cli-debug.ci` created and passing (commit ca62e83d).

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Plugin debug shell | ✅ Done | `cmd/ze/bgp/cmd_plugin.go` | Q&A + SSH + handshake + interactive |
| SSH as transport | ✅ Done | `internal/component/ssh/ssh.go:463` | `plugin protocol` detection |
| Engine-side handshake | ✅ Done | `internal/component/plugin/server/adhoc.go` | `HandleAdHocPluginSession` |
| SDK for non-net.Conn | ✅ Done | `pkg/plugin/sdk/sdk.go:114` | `NewWithIO` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-9 | ✅ Done | `TestAdHocProcessHandshake` + `plugin-cli-debug.ci` | 5-stage handshake completes over net.Pipe and via SSH |
| AC-10 | ✅ Done | `cmdPluginCLI` Q&A flow | Custom families parsed from Q&A |
| AC-11 | ✅ Done | `TestAdHocProcessRuntime` + `plugin-cli-debug.ci` | dispatch-command works after handshake, .ci verifies via SSH |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAdHocProcessHandshake` | ✅ Pass | `server/adhoc_test.go` | |
| `TestAdHocProcessRuntime` | ✅ Pass | `server/adhoc_test.go` | |
| `TestNewWithIO` | ✅ Pass | `server/adhoc_test.go` | |
| `plugin-cli-debug.ci` | ✅ Pass | `test/plugin/plugin-cli-debug.ci` | 5-stage handshake + dispatch via SSH |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/bgp/cmd_plugin.go` | ✅ Modified | Rewritten |
| `cmd/ze/internal/ssh/client/client.go` | ✅ Modified | `OpenProtocolSession` added |
| `internal/component/ssh/ssh.go` | ✅ Modified | `PluginProtocolFunc` + detection |
| `internal/component/plugin/server/server.go` | 🔄 Changed | `HandleAdHocPluginSession` placed in `adhoc.go` instead |
| `pkg/plugin/sdk/sdk.go` | ✅ Modified | `NewWithIO` added |
| `internal/component/plugin/server/adhoc.go` | ✅ Created | |
| `test/plugin/plugin-cli-debug.ci` | ✅ Created | Passes: 5-stage handshake + dispatch via SSH |

### Audit Summary
- **Total items:** 12
- **Done:** 11
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`HandleAdHocPluginSession` in `adhoc.go` instead of `server.go`)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/server/adhoc.go` | Yes | Created |
| `internal/component/plugin/server/adhoc_test.go` | Yes | Created |
| `test/plugin/plugin-cli-debug.ci` | Yes | Created, passes (5.0s) |
| `plan/learned/484-unified-cli.md` | Yes | Created |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-9 | Handshake completes | `TestAdHocProcessHandshake` PASS |
| AC-10 | Custom families | `cmd_plugin.go:129` parses families from Q&A |
| AC-11 | Runtime dispatch | `TestAdHocProcessRuntime` PASS |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze bgp plugin cli` | `test/plugin/plugin-cli-debug.ci` | Passes: 5-stage handshake + dispatch-command via SSH |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-9, AC-10, AC-11 demonstrated
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
