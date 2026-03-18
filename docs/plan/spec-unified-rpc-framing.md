# Spec: unified-rpc-framing

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-03-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/wire-format.md` - current NUL-delimited JSON-RPC wire format
4. `docs/architecture/api/ipc_protocol.md` - current IPC protocol (serial, events, commands)
5. `pkg/plugin/rpc/conn.go` - current Conn implementation
6. `pkg/plugin/rpc/mux.go` - current MuxConn implementation

## Task

Unify the plugin RPC wire format so that both JSON-mode and text-mode plugins use a single framing:

```
#<id> <verb> [<json-payload>]\n
```

Where `<verb>` is a method name (requests) or `ok`/`error` (responses), and `<json-payload>` is always JSON.

This eliminates:
- NUL-delimited framing (replaced by newline)
- ID embedded inside JSON body (moved to `#id` prefix)
- `ok`/`error` status embedded inside JSON body (moved to verb position)
- Two separate multiplexer implementations (`MuxConn` + `TextMuxConn`)
- Two separate startup code paths (`startup.go` + `startup_text.go`, `subsystem.go` + `subsystem_text.go`)
- Text-mode handshake format (`text.go` 561 lines of format/parse)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/wire-format.md` - current NUL-delimited JSON-RPC wire format
  -> Constraint: current format uses NUL delimiter, ID inside JSON, `result`/`error` keys for status
- [ ] `docs/architecture/api/ipc_protocol.md` - current IPC protocol (serial convention, events, commands)
  -> Constraint: IPC protocol already uses `#serial` prefix and newline delimiters for text commands
  -> Decision: text-mode plugin handshake section documents current text format (to be replaced)
- [ ] `docs/architecture/api/process-protocol.md` - ExaBGP external process protocol (historical reference)
  -> Constraint: external processes use newline-delimited communication
- [ ] `.claude/rules/plugin-design.md` - plugin protocol stages and SDK
  -> Constraint: 5-stage startup protocol must be preserved (same semantics, new framing)
  -> Constraint: DirectBridge bypasses framing entirely (unaffected)

**Key insights:**
- The IPC protocol (`ipc_protocol.md`) already uses `#serial` + newline. The plugin RPC protocol (`wire-format.md`) uses NUL + embedded ID. This spec unifies them.
- Text-mode auto-detection (first byte `{` vs letter) becomes unnecessary -- all plugins use the same format.
- DirectBridge for internal plugins is unaffected -- it bypasses framing entirely.
- The streaming protocol (`more`/`continues` fields) moves to the verb layer or stays in the JSON payload.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/framing.go` (109 lines) - NUL-delimited FrameReader/FrameWriter with `splitNUL`
- [ ] `pkg/plugin/rpc/message.go` (75 lines) - Request/RPCResult/RPCError types with `ID json.RawMessage` field
- [ ] `pkg/plugin/rpc/conn.go` (382 lines) - Sequential JSON-RPC: NextID(), CallRPC(), ReadRequest(), SendResult/Error/OK with ID matching
- [ ] `pkg/plugin/rpc/mux.go` (170 lines) - Concurrent JSON-RPC multiplexer: readLoop does json.Unmarshal to extract ID, routes via sync.Map
- [ ] `pkg/plugin/rpc/text_mux.go` (157 lines) - Concurrent text-mode multiplexer: readLoop extracts `#id` prefix via strings.Cut, routes via sync.Map
- [ ] `pkg/plugin/rpc/text_conn.go` (189 lines) - Line-based text framing for handshake stages
- [ ] `pkg/plugin/rpc/text.go` (561 lines) - Text format/parse for all 5 startup stages (registration, capabilities, config, registry, ready)
- [ ] `pkg/plugin/rpc/batch.go` (74 lines) - Pooled batch delivery frame construction, uses uint64 ID directly
- [ ] `pkg/plugin/rpc/bridge.go` (137 lines) - DirectBridge for internal plugins (bypasses framing)
- [ ] `pkg/plugin/sdk/sdk.go` (392 lines) - Plugin SDK: stores separate Conn/TextConn, MuxConn/TextMuxConn, textMode flag, drives 5-stage startup
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` (423 lines) - Event loop: reads Socket B requests, dispatches callbacks via req.ID
- [ ] `pkg/plugin/sdk/sdk_engine.go` (135 lines) - Plugin-to-engine RPC delegation (branches on text vs JSON)
- [ ] `pkg/plugin/sdk/sdk_text.go` (258 lines) - Text-mode 5-stage startup and post-startup text RPC
- [ ] `pkg/plugin/sdk/sdk_types.go` (86 lines) - Re-exported RPC type aliases
- [ ] `internal/component/plugin/server/startup.go` (660 lines) - Engine-side 5-stage handshake (JSON mode)
- [ ] `internal/component/plugin/server/startup_text.go` (340 lines) - Engine-side 5-stage handshake (text mode)
- [ ] `internal/component/plugin/server/subsystem.go` (860 lines) - Runtime Socket A dispatch (JSON mode)
- [ ] `internal/component/plugin/server/subsystem_text.go` (420 lines) - Runtime Socket A dispatch (text mode)
- [ ] `internal/component/plugin/server/dispatch.go` (600 lines) - RPC method dispatch, handlers receive req.ID
- [ ] `internal/core/ipc/message.go` (56 lines) - MapResponse: serial string to json.RawMessage ID conversion

**Behavior to preserve:**
- 5-stage startup protocol semantics (declare, configure, capabilities, registry, ready)
- Barrier synchronization (all plugins complete each stage before advancing)
- DirectBridge bypass for internal plugins
- Streaming protocol (`more`/`continues` semantics)
- All existing RPC methods and their input/output types
- Batch event delivery performance (pooled buffers)
- All existing functional tests in `test/plugin/*.ci`

**Behavior to change:**
- NUL delimiter replaced by newline
- ID field moved from inside JSON body to `#id` line prefix
- Response status (`ok`/`error`) moved from inside JSON to verb position on the line
- Text-mode handshake format (line-per-field) replaced by JSON payloads
- Text-mode auto-detection removed (single protocol)
- Two multiplexer implementations merged into one
- Two startup code paths merged into one
- Two dispatch code paths merged into one

## Data Flow (MANDATORY)

### Entry Point
- Plugin RPC messages enter via Unix socket pairs (Socket A: plugin-initiated, Socket B: engine-initiated)
- Current format: `{"method":"...", "params":{...}, "id":N}\0`
- New format: `#N method {"params":{...}}\n`

### Transformation Path

1. **Framing layer** reads a line (up to `\n`), extracts `#id` prefix and verb via `strings.Cut`
2. **Routing layer** (MuxConn) uses `#id` to route responses to waiting callers. No JSON parsing needed.
3. **Dispatch layer** uses verb to determine handler. For requests: verb is the method name. For responses: verb is `ok` or `error`.
4. **Payload layer** parses JSON payload only when the handler needs it (lazy).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> Plugin (Socket A) | `#id verb json\n` lines over Unix socket | [ ] |
| Engine <-> Plugin (Socket B) | `#id verb json\n` lines over Unix socket | [ ] |
| Engine <-> Plugin (DirectBridge) | Direct Go function calls (unchanged) | [ ] |

### Integration Points
- `internal/core/ipc/message.go` MapResponse -- converts plugin Response to wire format (must output new format)
- `internal/component/plugin/ipc/rpc.go` PluginConn -- wrapper around rpc.Conn (delegates to new API)
- `test/plugin/*.ci` -- functional tests that exercise RPC dispatch

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin startup (JSON plugin connects) | -> | 5-stage handshake with new framing | `test/plugin/cli-run.ci` |
| Plugin command dispatch | -> | Runtime RPC with `#id` routing | `test/plugin/cli-run-command.ci` |
| Plugin event delivery | -> | Batch events with new framing | `test/plugin/api-subscribe.ci` |
| Plugin NLRI encode/decode | -> | Codec RPC with `#id` routing | `test/plugin/api-raw.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin sends `#1 ze-plugin-engine:declare-registration {"families":["ipv4/unicast"]}\n` | Engine parses correctly, responds `#1 ok\n` or `#1 ok {"status":"done"}\n` |
| AC-2 | Engine sends `#2 ze-plugin-callback:deliver-batch {"events":[...]}\n` | Plugin receives and dispatches events correctly |
| AC-3 | MuxConn routes concurrent responses | Response `#42 ok {...}\n` delivered to caller that sent request `#42` |
| AC-4 | Malformed line (missing `#id`, no verb) | Error logged, connection not closed |
| AC-5 | `text.go`, `text_conn.go`, `text_mux.go` deleted | No text-mode framing code remains |
| AC-6 | `startup_text.go`, `subsystem_text.go` deleted | No text-mode startup/dispatch code remains |
| AC-7 | `Request.ID`, `RPCResult.ID`, `RPCError.ID` fields removed | ID is framing-only, not in JSON body |
| AC-8 | All existing `test/plugin/*.ci` functional tests pass | No regression |
| AC-9 | Streaming responses use `#id ok {"continues":true, ...}\n` | Streaming semantics preserved |
| AC-10 | Batch delivery uses pooled buffers | No performance regression in event delivery hot path |
| AC-11 | SDK `callEngineRaw()` tries Bridge -> MuxConn (no text branch) | Single code path for all plugins |

## New Wire Format (Reference)

### Request (plugin-to-engine or engine-to-plugin)

```
#<id> <method> <json>\n
#<id> <method>\n                    (no payload)
```

Examples:
```
#1 ze-plugin-engine:declare-registration {"families":["ipv4/unicast"],"commands":[]}\n
#2 ze-plugin-callback:configure {"sections":[{"root":"bgp","data":{}}]}\n
#7 ze-plugin-callback:deliver-batch {"events":["...","..."]}\n
```

### Successful Response

```
#<id> ok <json>\n
#<id> ok\n                          (no payload)
```

### Error Response

```
#<id> error <json>\n
#<id> error\n                       (no payload)
```

Error payload: `{"message":"human-readable text"}` or `{"code":"error-identity","message":"human-readable text"}`

### Streaming Response

```
#<id> ok {"continues":true, "result":{...}}\n     (intermediate)
#<id> ok {"result":{...}}\n                        (final)
```

### ID Rules

| Rule | Detail |
|------|--------|
| Format | Decimal integer, monotonically increasing |
| Scope | Per-connection (each socket pair has independent sequence) |
| Generation | Atomic uint64 counter |
| Uniqueness | Within connection lifetime |

### Framing Rules

| Rule | Detail |
|------|--------|
| Delimiter | `\n` (0x0A) |
| Encoding | UTF-8 |
| Max line size | 16 MB |
| JSON payload | Always compact (single-line, no unescaped newlines) |
| Empty payload | Verb followed immediately by `\n` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestFrameReaderLines` | `pkg/plugin/rpc/framing_test.go` | Newline-delimited frame reading |  |
| `TestFrameWriterLines` | `pkg/plugin/rpc/framing_test.go` | Newline-terminated frame writing |  |
| `TestConnCallRPC` | `pkg/plugin/rpc/conn_test.go` | `#id method json\n` request/response round-trip |  |
| `TestConnReadRequest` | `pkg/plugin/rpc/conn_test.go` | Parse `#id method [json]` from line |  |
| `TestConnSendResult` | `pkg/plugin/rpc/conn_test.go` | Write `#id ok [json]\n` |  |
| `TestConnSendError` | `pkg/plugin/rpc/conn_test.go` | Write `#id error [json]\n` |  |
| `TestMuxConcurrentRouting` | `pkg/plugin/rpc/mux_test.go` | Multiple concurrent requests routed by `#id` |  |
| `TestMuxOrphanedResponse` | `pkg/plugin/rpc/mux_test.go` | Orphaned `#id` response logged, not crash |  |
| `TestMuxMalformedLine` | `pkg/plugin/rpc/mux_test.go` | Missing `#`, no verb, etc. handled gracefully |  |
| `TestBatchFrame` | `pkg/plugin/rpc/batch_test.go` | Pooled batch frame uses new format |  |
| `TestSDKStartup` | `pkg/plugin/sdk/sdk_test.go` | 5-stage startup with new framing |  |
| `TestSDKEventLoop` | `pkg/plugin/sdk/sdk_test.go` | Event dispatch with new framing |  |
| `TestMapResponse` | `internal/core/ipc/message_test.go` | Serial-to-`#id` conversion in new format |  |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ID | 1 to 2^64-1 | 18446744073709551615 | 0 (no `#0`) | N/A (wraps) |
| Line length | 1 to 16 MB | 16777216 bytes | N/A | 16777217 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Plugin startup + command | `test/plugin/cli-run.ci` | Plugin starts, registers, becomes ready | |
| Plugin command dispatch | `test/plugin/cli-run-command.ci` | User runs plugin command, gets response | |
| Event subscription | `test/plugin/api-subscribe.ci` | Plugin subscribes, receives events | |
| Raw passthrough | `test/plugin/api-raw.ci` | Plugin sends raw BGP message | |
| Peer operations | `test/plugin/api-peer-list.ci` | Plugin lists peers | |
| All existing test/plugin/*.ci | `test/plugin/` | All existing functional tests pass unchanged | |

### Future (if deferring any tests)
- Performance benchmarks comparing NUL vs newline framing throughput -- deferred because functional correctness is the priority and the change is expected to be neutral or positive

## Files to Modify

### Core RPC (`pkg/plugin/rpc/`)

| File | Change |
|------|--------|
| `framing.go` | Replace NUL splitting with newline splitting. `FrameReader` uses `bufio.Scanner` default line splitting. `FrameWriter` appends `\n` instead of `\0` |
| `message.go` | Remove `ID` field from `Request`, `RPCResult`, `RPCError`. Add `LineParsed` type with `ID uint64`, `Verb string`, `Payload []byte` for parsed line representation |
| `conn.go` | Rewrite `NextID()` to return `uint64`. Rewrite `CallRPC`/`ReadRequest`/`SendResult`/`SendError`/`SendOK` to use `#id verb json\n` format. Remove `callMu` serialization (MuxConn handles concurrency). `CallBatchRPC` uses new format |
| `mux.go` | Rewrite `readLoop` to extract `#id` via `strings.Cut` (no `json.Unmarshal`). Response channel type changes. Absorb `TextMuxConn` functionality (already identical after framing unification) |
| `batch.go` | Rewrite `WriteBatchFrame` to produce `#id ze-plugin-callback:deliver-batch {"events":[...]}\n` |
| `bridge.go` | No change (bypasses framing) |

### SDK (`pkg/plugin/sdk/`)

| File | Change |
|------|--------|
| `sdk.go` | Remove `textMode` flag, `textConnA`, `textConnB`, `textMux` fields. Single `Conn`/`MuxConn` for all plugins. Remove text-mode branching from startup sequence |
| `sdk_dispatch.go` | Update `serveOne`/`eventLoop`/`dispatchCallback` to use new Conn API (ID from framing, not from `req.ID`) |
| `sdk_engine.go` | Remove text vs JSON branching in `callEngine`/`callEngineRaw` |
| `sdk_types.go` | Update re-exported types (remove ID from Request/RPCResult/RPCError) |
| `sdk_callbacks.go` | No change (function pointer storage) |

### Engine Server (`internal/component/plugin/server/`)

| File | Change |
|------|--------|
| `startup.go` | Update all `ReadRequest`/`SendResult`/`SendError`/`SendOK` calls to new API. Absorb text-mode startup logic (now same format) |
| `subsystem.go` | Update dispatch loop to use new Conn API. Absorb text-mode dispatch logic |
| `dispatch.go` | Update handler signatures (ID from framing, not `req.ID`) |
| `command.go` | Update dispatch with new response format |
| `pending.go` | Update if subscription storage uses ID type |
| `reload.go` | Update RPC calls |
| `server.go` | Minor -- startup coordinator unchanged |
| `system.go` | Update handler dispatch |

### IPC Support (`internal/core/ipc/`)

| File | Change |
|------|--------|
| `message.go` | Rewrite `MapResponse` for new format (serial becomes `#id` prefix) |

### Architecture Docs

| File | Change |
|------|--------|
| `docs/architecture/api/wire-format.md` | Rewrite for new `#id verb json\n` format |
| `docs/architecture/api/ipc_protocol.md` | Update "Wire Format" and "Text Mode Plugin Handshake" sections |

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| RPC count in architecture docs | No | - |
| CLI commands/flags | No | - |
| CLI usage/help text | No | - |
| API commands doc | No | - |
| Plugin SDK docs | Yes | `.claude/rules/plugin-design.md` (update "Text mode" references) |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | Existing `test/plugin/*.ci` must pass |

## Files to Delete

| File | Lines | Reason |
|------|-------|--------|
| `pkg/plugin/rpc/text_mux.go` | 157 | Merged into `mux.go` |
| `pkg/plugin/rpc/text_conn.go` | 189 | Replaced by unified `Conn` with newline framing |
| `pkg/plugin/rpc/text.go` | 561 | Text handshake format/parse no longer needed (JSON payloads) |
| `pkg/plugin/sdk/sdk_text.go` | 258 | Text-mode startup absorbed into `sdk.go` |
| `internal/component/plugin/server/startup_text.go` | 340 | Text-mode startup absorbed into `startup.go` |
| `internal/component/plugin/server/subsystem_text.go` | 420 | Text-mode dispatch absorbed into `subsystem.go` |

**Total deleted: ~1,925 lines across 6 files**

## Files to Create

- None -- all changes are modifications to existing files or deletions

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Delete, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
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

1. **Phase: Core framing** -- Replace NUL framing with newline framing, add `#id verb payload` line parsing
   - Tests: `TestFrameReaderLines`, `TestFrameWriterLines`, `TestConnCallRPC`, `TestConnReadRequest`, `TestConnSendResult`, `TestConnSendError`
   - Files: `framing.go`, `message.go`, `conn.go`
   - Verify: tests fail -> implement -> tests pass (`go test ./pkg/plugin/rpc/...`)

2. **Phase: Unified mux** -- Merge TextMuxConn into MuxConn using `#id` prefix extraction
   - Tests: `TestMuxConcurrentRouting`, `TestMuxOrphanedResponse`, `TestMuxMalformedLine`
   - Files: `mux.go`, delete `text_mux.go`
   - Verify: tests fail -> implement -> tests pass (`go test ./pkg/plugin/rpc/...`)

3. **Phase: Batch delivery** -- Update batch frame construction for new format
   - Tests: `TestBatchFrame`
   - Files: `batch.go`
   - Verify: tests fail -> implement -> tests pass (`go test ./pkg/plugin/rpc/...`)

4. **Phase: SDK unification** -- Remove text-mode branching, single code path for all plugins
   - Tests: `TestSDKStartup`, `TestSDKEventLoop`
   - Files: `sdk.go`, `sdk_dispatch.go`, `sdk_engine.go`, `sdk_types.go`, delete `sdk_text.go`
   - Verify: tests fail -> implement -> tests pass (`go test ./pkg/plugin/sdk/...`)

5. **Phase: Engine server unification** -- Merge text-mode startup and dispatch into main files
   - Tests: existing server tests
   - Files: `startup.go`, `subsystem.go`, `dispatch.go`, `command.go`, delete `startup_text.go`, delete `subsystem_text.go`
   - Verify: tests fail -> implement -> tests pass (`go test ./internal/component/plugin/server/...`)

6. **Phase: Delete text handshake** -- Remove text.go, text_conn.go, update IPC message.go
   - Tests: remove text-specific tests, update `message_test.go`
   - Files: delete `text.go`, delete `text_conn.go`, update `internal/core/ipc/message.go`
   - Verify: `go test ./pkg/plugin/rpc/... ./internal/core/ipc/...`

7. **Phase: Architecture docs** -- Update wire-format.md and ipc_protocol.md
   - Files: `docs/architecture/api/wire-format.md`, `docs/architecture/api/ipc_protocol.md`

8. **Functional tests** -> Run all `test/plugin/*.ci` tests, fix any regressions
9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | `#id` extraction handles edge cases (large IDs, malformed lines) |
| Naming | Method names unchanged (`ze-plugin-engine:declare-registration` etc.) |
| Data flow | Framing layer never parses JSON; dispatch layer never handles `#id` routing |
| Rule: no-layering | All 6 deleted files are fully gone, no remnant imports or dead references |
| Rule: buffer-first | Batch delivery still uses pooled buffers |
| Rule: compatibility | No backwards-compat shims (Ze has no users) |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `text_mux.go` deleted | `ls pkg/plugin/rpc/text_mux.go` fails |
| `text_conn.go` deleted | `ls pkg/plugin/rpc/text_conn.go` fails |
| `text.go` deleted | `ls pkg/plugin/rpc/text.go` fails |
| `sdk_text.go` deleted | `ls pkg/plugin/sdk/sdk_text.go` fails |
| `startup_text.go` deleted | `ls internal/component/plugin/server/startup_text.go` fails |
| `subsystem_text.go` deleted | `ls internal/component/plugin/server/subsystem_text.go` fails |
| No `textMode` in SDK | `grep -r textMode pkg/plugin/sdk/` returns nothing |
| No `TextMuxConn` type | `grep -r TextMuxConn pkg/plugin/` returns nothing |
| No `TextConn` type | `grep -r TextConn pkg/plugin/` returns nothing |
| No `splitNUL` function | `grep -r splitNUL pkg/plugin/` returns nothing |
| No `json:"id"` in RPC messages | `grep 'json:"id"' pkg/plugin/rpc/message.go` returns nothing |
| All functional tests pass | `make ze-functional-test` exits 0 |
| `wire-format.md` updated | `grep '#<id>' docs/architecture/api/wire-format.md` finds new format |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | Malformed `#id` lines (non-numeric ID, missing verb, oversized lines) must not crash or leak memory |
| Max line size | 16 MB limit enforced by `bufio.Scanner` buffer cap, same as current NUL framing |
| JSON injection | Newlines in JSON payloads: compact JSON never contains unescaped `\n`, verify `json.Marshal` output is single-line |
| Resource exhaustion | Large ID values (uint64 max) handled without overflow in `strings.Cut` parsing |

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

### Failed Approaches
| Approach | Why abandoned | Replacement |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |

## Design Insights

- Compact JSON (output of `json.Marshal`) never contains unescaped newlines, so `\n` is a safe frame delimiter for JSON payloads
- The `#id` prefix extraction is the same operation for both request routing and response routing -- `strings.Cut` on space after `#`
- The verb position (`method` for requests, `ok`/`error` for responses) means the dispatch layer can determine message type without JSON parsing
- The framing layer becomes truly protocol-agnostic: it reads lines and extracts `#id` + verb. It does not know or care whether the payload is JSON, text, or empty

## RFC Documentation

Not applicable -- this is internal protocol, not BGP wire format.

## Implementation Summary

### What Was Implemented
- (to be filled after implementation)

### Bugs Found/Fixed
- (to be filled after implementation)

### Documentation Updates
- (to be filled after implementation)

### Deviations from Plan
- (to be filled after implementation)

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
- [ ] AC-1..AC-11 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`pkg/plugin/rpc/*`, `pkg/plugin/sdk/*`, `internal/component/plugin/server/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated (`wire-format.md`, `ipc_protocol.md`)
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Write learned summary to `docs/learned/NNN-<name>.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
