# Spec: API Engine (Shared Backend)

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-04-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-api-1-rest.md` - REST transport
3. `plan/spec-api-2-grpc.md` - gRPC transport
4. `internal/component/mcp/tools.go` - MCP auto-generation pattern (reference implementation)
5. `internal/component/plugin/server/command.go` - dispatcher

## Task

Build a shared API engine that both REST and gRPC transports use. The engine owns:
command discovery, command execution, streaming, config sessions, auth, and schema
generation. Transports are thin adapters -- they marshal to/from their wire format
and call engine functions. They cannot add logic, filter commands, or diverge.

Inspired by osvbng's `BNGService` (see `docs/research/comparison/osvbng.md`) but
designed around ze's existing command registry and YANG metadata.

## Spec Set

| Spec | Purpose |
|------|---------|
| `spec-api-0-umbrella.md` | Shared engine, API contract, types (this file) |
| `spec-api-1-rest.md` | REST/HTTP transport + OpenAPI generation |
| `spec-api-2-grpc.md` | gRPC transport + proto generation |

Implementation order: engine first, then either transport (independent of each other).

## Required Reading

### Architecture Docs
- [ ] `internal/component/mcp/tools.go` - MCP tool auto-generation from command registry
  -> Decision: commands grouped by longest common prefix, YANG params become typed schema
  -> Constraint: YANG type mapping: uint* -> integer, boolean -> boolean, rest -> string
- [ ] `internal/component/mcp/handler.go` - MCP HTTP handler pattern
  -> Constraint: dispatch via serverDispatcher, responses wrapped in standard envelope
- [ ] `internal/component/plugin/server/command.go` - dispatcher dispatch logic
  -> Constraint: longest-prefix match for builtins, then plugin registry fallback
  -> Constraint: peer selector extraction for "peer addr command" pattern
- [ ] `internal/component/plugin/server/command_registry.go` - registered command metadata
  -> Constraint: Name, Description, Args, Completable, Timeout per command
- [ ] `internal/component/config/yang/rpc.go` - YANG RPC metadata extraction
  -> Constraint: RPCMeta has Module, Name, Description, Input LeafMeta list, Output LeafMeta list
  -> Constraint: LeafMeta has Name, Type, Description, Mandatory
- [ ] `cmd/ze/hub/mcp.go` - how MCP integrates dispatcher + YANG params
  -> Constraint: buildParamMap() maps CLI command path to YANG RPC input params
- [ ] `docs/research/comparison/osvbng.md` - osvbng gRPC API design

**Key insights:**
- MCP already auto-generates typed tools from command registry + YANG -- same pattern for REST/gRPC
- Dispatcher.Dispatch() is the single execution point for all interfaces
- Response envelope: serial, status, partial, data
- Auth via dispatcher.IsAuthorized()
- Config editing via Editor (separate from command dispatch)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/mcp/tools.go` - command grouping, schema generation
- [ ] `internal/component/plugin/server/command.go` - Command struct, dispatch
- [ ] `internal/component/config/yang/rpc.go` - RPCMeta, LeafMeta extraction
- [ ] `cmd/ze/hub/mcp.go` - buildParamMap, serverDispatcher, serverCommandLister

**Behavior to preserve:**
- All existing interfaces (SSH, Web, MCP, LG) continue unchanged
- Command dispatch via plugin dispatcher (same handlers, same behavior)
- YANG-based parameter metadata
- Auth via existing authz system
- Standard response envelope format

**Behavior to change:**
- Extract command discovery + dispatch bridge from MCP into shared engine
- Add schema generation for OpenAPI and gRPC reflection
- Add config session management as first-class engine capability
- Add streaming as first-class engine capability (SSE and gRPC server-stream)

## Design: API Engine

### Location

New component at `internal/component/api/`.

### Engine Interface

The engine exposes these capabilities. Both transports call these and nothing else.

**Command Discovery:**

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| ListCommands | filter (optional prefix) | list of CommandInfo | All available commands with metadata |
| DescribeCommand | command path | CommandInfo with params | Full metadata for one command |
| Complete | partial command | list of completions | Tab-completion candidates |

CommandInfo carries: name, description, read-only flag, parameter list (name, type, description, required), group prefix.

**Command Execution:**

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| Execute | command string, params map, auth context | Response | One-shot command execution |
| Stream | command string, params map, auth context | channel of Response | Streaming (monitor, subscribe) |

Response carries: status (done/error/partial), data (any), error message.

**Config Sessions:**

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| ConfigGetRunning | auth context | config string | Running config |
| ConfigEnter | auth context | session ID | Start candidate session |
| ConfigSet | session ID, path, value | success/error | Modify candidate |
| ConfigDelete | session ID, path | success/error | Delete from candidate |
| ConfigDiff | session ID | list of changes | Preview pending changes |
| ConfigCommit | session ID, message | success/error | Apply atomically |
| ConfigDiscard | session ID | success | Throw away candidate |

Config sessions wrap the existing Editor, exposed as API operations.

**Schema Generation:**

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| OpenAPISchema | - | OpenAPI 3.1 JSON | Auto-generated from command registry |
| CommandSchema | command path | JSON Schema | Parameter schema for one command |

### Engine Internals

The engine wraps existing infrastructure:

| Engine function | Backed by |
|----------------|-----------|
| ListCommands | dispatcher.Commands() + dispatcher.Registry().All() + buildParamMap() |
| Execute | dispatcher.Dispatch(ctx, command) |
| Stream | dispatcher.Dispatch(ctx, "monitor ...") with streaming response |
| ConfigGetRunning | Read config file (same as `ze config show`) |
| ConfigEnter/Set/Commit | Editor instance per session |
| OpenAPISchema | Generated at startup from CommandInfo list |
| Complete | dispatcher.Complete(ctx, partial) |

### Non-Divergence Contract

Both transports MUST:

1. Call engine methods only -- no direct dispatcher access
2. Use engine types for request/response -- marshal to/from their wire format
3. Expose all commands the engine lists -- no filtering, no subsetting
4. Use engine auth checks -- no transport-specific auth logic
5. Use engine schema -- OpenAPI generated from same metadata as proto reflection

If a command works in REST, it works in gRPC, and vice versa. Always.

### YANG Config

New module `ze-api-conf.yang` with container `api` containing:

| Path | Type | Default | Description |
|------|------|---------|-------------|
| api.rest.enabled | boolean | false | Enable REST API |
| api.rest.ip | zt:ip-address | 0.0.0.0 | REST listen address |
| api.rest.port | zt:port | 8080 | REST listen port |
| api.rest.cors-origin | string | (empty) | CORS allowed origin |
| api.grpc.enabled | boolean | false | Enable gRPC API |
| api.grpc.ip | zt:ip-address | 0.0.0.0 | gRPC listen address |
| api.grpc.port | zt:port | 50051 | gRPC listen port |
| api.grpc.tls-cert | string | (empty) | TLS certificate path |
| api.grpc.tls-key | string | (empty) | TLS key path |

Both use ze:listener extension for port conflict detection. Both disabled by default.

### Auth Model

Same as existing: auth context carries username + permissions. Engine checks
dispatcher.IsAuthorized() before executing. Read-only commands allowed for
read-only users. Config mutations require write permission.

| Transport | Auth mechanism |
|-----------|---------------|
| REST | Bearer token in Authorization header, or session cookie |
| gRPC | Bearer token in metadata, or mTLS |

Token validation via existing authz component.

## Data Flow (MANDATORY)

### Entry Point
- REST: HTTP request -> router -> handler -> engine method
- gRPC: RPC call -> service impl -> engine method

### Transformation Path
1. Transport receives request in its wire format (JSON or protobuf)
2. Transport extracts: command, params, auth context
3. Transport calls engine method with ze-native types
4. Engine validates auth, builds command string, calls dispatcher
5. Dispatcher executes, returns Response
6. Engine returns Response to transport
7. Transport marshals Response to wire format (JSON or protobuf)

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Transport -> Engine | Engine interface methods (typed Go calls) | [ ] |
| Engine -> Dispatcher | dispatcher.Dispatch(ctx, command) | [ ] |
| Engine -> Editor | Editor instance per config session | [ ] |
| Engine -> YANG | yang.ExtractRPCs() at startup for schema | [ ] |

### Integration Points
- dispatcher.Dispatch() - existing, the single execution point for commands
- dispatcher.Commands() - existing, lists builtin commands with metadata
- dispatcher.Registry().All() - existing, lists plugin commands
- buildParamMap() - existing in MCP, extracted into engine
- Editor - existing, wraps config edit lifecycle
- authz - existing, checks authorization

### Architectural Verification
- [ ] No bypassed layers (transports never call dispatcher directly)
- [ ] No unintended coupling (REST and gRPC independent of each other)
- [ ] No duplicated functionality (engine is the only command bridge)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Engine.Execute("bgp summary") | -> | dispatcher.Dispatch called, response returned | TestEngineExecuteDispatch |
| Engine.ListCommands() | -> | all dispatcher + plugin commands returned | TestEngineListCommandsComplete |
| Engine.ConfigEnter + Set + Commit | -> | Editor commit applied | TestEngineConfigSession |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Engine.ListCommands() called | Returns all commands from dispatcher + plugin registry with YANG params |
| AC-2 | Engine.Execute("bgp summary") | Returns same output as SSH CLI "bgp summary" |
| AC-3 | Engine.Execute with unauthorized user | Returns auth error, command not executed |
| AC-4 | Engine.Stream("bgp monitor") | Returns channel that delivers BGP events |
| AC-5 | Engine.ConfigEnter + Set + Commit | Config applied, same as `ze config edit` workflow |
| AC-6 | Engine.OpenAPISchema() | Returns valid OpenAPI 3.1 JSON describing all commands |
| AC-7 | New YANG command added to a plugin | Automatically appears in ListCommands, OpenAPISchema, and both transports |
| AC-8 | REST and gRPC execute same command | Identical data in response (wire format differs, content identical) |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestEngineListCommands | api/engine_test.go | All commands returned with metadata | |
| TestEngineExecuteDispatch | api/engine_test.go | Execute calls dispatcher, returns response | |
| TestEngineExecuteUnauthorized | api/engine_test.go | Auth check rejects, command not run | |
| TestEngineStream | api/engine_test.go | Stream returns channel with events | |
| TestEngineConfigSession | api/config_session_test.go | Enter/Set/Commit lifecycle works | |
| TestEngineConfigDiscard | api/config_session_test.go | Discard throws away changes | |
| TestOpenAPISchemaValid | api/schema_test.go | Generated schema validates as OpenAPI 3.1 | |
| TestCommandSchemaMatchesYANG | api/schema_test.go | Schema params match YANG RPC input | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs in engine interface.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-api-engine | test/plugin/test-api-engine.ci | Engine executes commands through dispatcher | |

### Future (if deferring any tests)
- Property test: random command sequences produce identical output through REST and gRPC

## Files to Modify
- `cmd/ze/hub/main.go` - wire API engine into startup (after dispatcher + YANG loaded)

## Files to Create
- `internal/component/api/engine.go` - API engine: discovery, execute, stream, auth
- `internal/component/api/config_session.go` - config session manager (wraps Editor)
- `internal/component/api/schema.go` - OpenAPI + JSON Schema generation from YANG
- `internal/component/api/types.go` - shared request/response types
- `internal/component/api/engine_test.go` - unit tests
- `internal/component/api/config_session_test.go` - config session tests
- `internal/component/api/schema_test.go` - schema generation tests
- `internal/component/api/schema/ze-api-conf.yang` - YANG config for api listeners
- `test/plugin/test-api-engine.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | Yes | `internal/component/api/schema/ze-api-conf.yang` |
| CLI commands/flags | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | `test/plugin/test-api-engine.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - REST/gRPC API |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` - api section |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/` - new API engine doc |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/api.md` - API usage guide |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - programmatic API |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` - API engine |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | make ze-lint and make ze-unit-test and make ze-functional-test |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Engine types and interface** -- define CommandInfo, Response, engine interface
   - Tests: TestEngineListCommands, TestEngineExecuteDispatch, TestEngineExecuteUnauthorized
   - Files: api/types.go, api/engine.go, api/engine_test.go
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Streaming** -- implement Stream method with channel-based delivery
   - Tests: TestEngineStream
   - Files: api/engine.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Config session manager** -- wrap Editor for API use
   - Tests: TestEngineConfigSession, TestEngineConfigDiscard
   - Files: api/config_session.go, api/config_session_test.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: Schema generation** -- OpenAPI from YANG metadata
   - Tests: TestOpenAPISchemaValid, TestCommandSchemaMatchesYANG
   - Files: api/schema.go, api/schema_test.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase: YANG config** -- api.rest and api.grpc listener config
   - Files: ze-api-conf.yang, register.go, embed.go

6. **Phase: Wire into startup** -- create engine during hub startup
   - Files: cmd/ze/hub/main.go
   - Tests: test-api-engine.ci

7. **Full verification** -> make ze-verify
8. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Engine returns identical data for same command regardless of caller |
| Naming | Types follow ze conventions (CommandInfo, not APICommand) |
| Data flow | Transport -> Engine -> Dispatcher, never Transport -> Dispatcher |
| Rule: no-layering | buildParamMap extracted from MCP, not duplicated |
| Non-divergence | Both transports tested against same engine instance |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Engine interface in engine.go | grep "ListCommands\|Execute\|Stream\|Config" api/engine.go |
| Types in types.go | grep "CommandInfo\|Response" api/types.go |
| OpenAPI generation in schema.go | grep "OpenAPISchema" api/schema.go |
| Config sessions in config_session.go | grep "ConfigEnter\|ConfigCommit" api/config_session.go |
| YANG config | ls internal/component/api/schema/ze-api-conf.yang |
| Functional test | ls test/plugin/test-api-engine.ci |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Command strings sanitized before dispatch |
| Auth enforcement | Every engine method checks auth before execution |
| Session management | Config sessions timeout, cleaned up on disconnect |
| Rate limiting | Consider per-client request limits (future work, note in design) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
| Lint failure | Fix inline |
| Functional test fails | Check AC |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### Engine is a component, not a library

The API engine lives in `internal/component/api/`, not `pkg/`. It is internal to ze.
External consumers use REST or gRPC -- never import the engine directly.

### Config sessions are engine-managed, not transport-managed

Each transport calls ConfigEnter() and gets a session ID. The engine manages session
lifecycle (timeouts, cleanup on disconnect). This prevents REST and gRPC from implementing
session management differently.

### Schema generated at startup, not on each request

The command registry is stable after startup (plugins register during init). The OpenAPI
schema is generated once and cached. Adding a command requires a restart -- same as today
for SSH/Web/MCP.

### Generic execute + typed config, not typed everything

Most commands use the generic Execute(command, params) path. Only config operations
get typed methods because config sessions have lifecycle (enter/set/commit/discard) that
does not fit a single execute call. Peer/RIB/system commands go through generic execute.

REST adds RESTful convenience routes (GET /peers) but they map to generic execute internally.

### Response format is JSON

The engine returns structured data. Both transports serialize it:
- REST: JSON directly
- gRPC: JSON bytes in a bytes data field (not per-command proto messages)

This ensures content is always identical. If we later want typed proto responses per
command, the engine can generate them from YANG -- but that is future work.

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
N/A -- internal architecture, not protocol work.

## Implementation Summary

### What Was Implemented
- [pending]

### Bugs Found/Fixed
- [pending]

### Documentation Updates
- [pending]

### Deviations from Plan
- [pending]

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
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
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-api-engine.md`
- [ ] Summary included in commit
