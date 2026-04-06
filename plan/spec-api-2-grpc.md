# Spec: gRPC API

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-api-0-umbrella |
| Phase | - |
| Updated | 2026-04-05 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-api-0-umbrella.md` - shared engine (MUST read first)
3. `internal/component/api/engine.go` - engine interface (created by umbrella spec)
4. `internal/component/api/types.go` - shared types

## Task

Implement a gRPC transport over the shared API engine defined in `spec-api-0-umbrella.md`.
The transport exposes all engine commands via two gRPC services: a generic ZeService for
command execution and a typed ZeConfigService for config session management.

This is a thin adapter: all logic lives in the engine. The gRPC layer handles protobuf
marshaling, server-streaming, TLS, and gRPC reflection.

## Spec Set

| Spec | Purpose |
|------|---------|
| `spec-api-0-umbrella.md` | Shared engine (dependency) |
| `spec-api-1-rest.md` | REST/HTTP transport + OpenAPI |
| `spec-api-2-grpc.md` | gRPC transport (this file) |

## Required Reading

### Architecture Docs
- [ ] `plan/spec-api-0-umbrella.md` - engine interface, types, non-divergence contract
  -> Decision: engine owns all logic, transport is marshal/unmarshal only
  -> Constraint: transports must not call dispatcher directly
  -> Constraint: transports must expose all commands engine lists
- [ ] `docs/research/comparison/osvbng.md` - osvbng gRPC design (BNGService + HAPeerService)
  -> Decision: ze uses generic execute + typed config, not typed-per-command
  -> Constraint: response data is JSON bytes, not typed proto messages per command

**Key insights:**
- Engine provides ListCommands, Execute, Stream, Config*, OpenAPISchema
- gRPC layer routes RPCs to engine methods and marshals protobuf
- Server-streaming for monitor/subscribe commands
- gRPC reflection for tooling (grpcurl, grpcui)
- New dependency: google.golang.org/grpc (requires user approval)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `plan/spec-api-0-umbrella.md` - engine design
- [ ] `go.mod` - current dependencies (no gRPC yet)

**Behavior to preserve:**
- All existing interfaces unchanged
- Engine is the single source of truth
- No new third-party deps without user approval

**Behavior to change:**
- New gRPC listener (configurable port, default 50051)
- Proto-based service definitions
- gRPC reflection enabled for tooling
- New dependency: google.golang.org/grpc and google.golang.org/protobuf

## Design: gRPC Services

### ZeService (generic command execution)

| RPC | Type | Input | Output | Engine call |
|-----|------|-------|--------|-------------|
| Execute | unary | CommandRequest | CommandResponse | Engine.Execute |
| Stream | server-stream | CommandRequest | stream CommandResponse | Engine.Stream |
| ListCommands | unary | ListCommandsRequest | ListCommandsResponse | Engine.ListCommands |
| DescribeCommand | unary | DescribeCommandRequest | CommandDescription | Engine.DescribeCommand |
| Complete | unary | CompleteRequest | CompleteResponse | Engine.Complete |

**Message types:**

| Message | Fields |
|---------|--------|
| CommandRequest | command (string), params (map string to string) |
| CommandResponse | status (string: done/error/partial), data (bytes, JSON), error (string) |
| ListCommandsRequest | prefix (string, optional filter) |
| ListCommandsResponse | commands (repeated CommandInfo) |
| CommandInfo | name (string), description (string), read_only (bool), params (repeated ParamInfo) |
| ParamInfo | name (string), type (string), description (string), required (bool) |
| DescribeCommandRequest | path (string) |
| CommandDescription | info (CommandInfo) |
| CompleteRequest | partial (string) |
| CompleteResponse | completions (repeated string) |

### ZeConfigService (typed config session management)

| RPC | Type | Input | Output | Engine call |
|-----|------|-------|--------|-------------|
| GetRunningConfig | unary | Empty | ConfigResponse | Engine.ConfigGetRunning |
| EnterSession | unary | Empty | SessionResponse | Engine.ConfigEnter |
| SetConfig | unary | ConfigSetRequest | ConfigSetResponse | Engine.ConfigSet |
| DeleteConfig | unary | ConfigDeleteRequest | ConfigDeleteResponse | Engine.ConfigDelete |
| DiffSession | unary | SessionRequest | DiffResponse | Engine.ConfigDiff |
| CommitSession | unary | CommitRequest | CommitResponse | Engine.ConfigCommit |
| DiscardSession | unary | SessionRequest | DiscardResponse | Engine.ConfigDiscard |

**Message types:**

| Message | Fields |
|---------|--------|
| ConfigResponse | config_yaml (string) |
| SessionResponse | session_id (string) |
| ConfigSetRequest | session_id (string), path (string), value (string) |
| ConfigSetResponse | success (bool), message (string) |
| ConfigDeleteRequest | session_id (string), path (string) |
| ConfigDeleteResponse | success (bool), message (string) |
| SessionRequest | session_id (string) |
| DiffResponse | added (repeated DiffLine), deleted (repeated DiffLine), modified (repeated DiffLine) |
| DiffLine | path (string), value (string) |
| CommitRequest | session_id (string), commit_msg (string) |
| CommitResponse | success (bool), message (string) |
| DiscardResponse | success (bool) |

### gRPC Details

| Concern | Approach |
|---------|----------|
| Auth | Bearer token in metadata (authorization key); interceptor validates before handler |
| TLS | Optional, configured via YANG (api.grpc.tls-cert, api.grpc.tls-key) |
| Reflection | gRPC server reflection enabled for grpcurl/grpcui tooling |
| Errors | gRPC status codes: Unauthenticated (no token), PermissionDenied (insufficient), NotFound (unknown command), Internal (dispatcher error) |
| Streaming | Server-streaming for Stream RPC; client receives CommandResponse messages until context cancelled |
| Proto package | ze.api.v1 |

## Data Flow (MANDATORY)

### Entry Point
- gRPC request arrives at gRPC listener (configured port, default 50051)

### Transformation Path
1. gRPC server receives RPC call with protobuf message
2. Auth interceptor extracts Bearer token from metadata, validates
3. Service handler unmarshals protobuf to engine-native types
4. Handler calls engine method (Execute, Stream, ConfigSet, etc.)
5. Engine returns Response (ze-native types)
6. Handler marshals Response to protobuf CommandResponse
7. gRPC response sent

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gRPC -> Service handler | protobuf RPC | [ ] |
| Service handler -> Engine | Engine interface methods | [ ] |
| Engine -> Dispatcher | (internal to engine, not gRPC's concern) | [ ] |

### Integration Points
- API Engine (from spec-api-0-umbrella) - the only backend
- authz component - token validation via interceptor
- YANG config - listener address/port/TLS

### Architectural Verification
- [ ] No bypassed layers (gRPC never calls dispatcher directly)
- [ ] No unintended coupling (gRPC independent of REST)
- [ ] No duplicated functionality (all logic in engine)
- [ ] Zero-copy preserved where applicable (N/A)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| ZeService.ListCommands RPC | -> | Engine.ListCommands | test-grpc-api-commands.ci |
| ZeService.Execute with "bgp summary" | -> | Engine.Execute | test-grpc-api-execute.ci |
| ZeConfigService.EnterSession + SetConfig + CommitSession | -> | Engine config session | test-grpc-api-config.ci |
| ZeService.Stream with "bgp monitor" | -> | Engine.Stream | TestGRPCStream |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | ZeService.ListCommands() | Returns all commands with metadata (same as REST /api/v1/commands) |
| AC-2 | ZeService.Execute("bgp summary") | Returns CommandResponse with same data as REST execute |
| AC-3 | RPC without auth metadata | Returns gRPC Unauthenticated status |
| AC-4 | ZeService.Stream("bgp monitor") | Server streams CommandResponse messages |
| AC-5 | ZeConfigService full lifecycle | EnterSession + SetConfig + CommitSession applies config |
| AC-6 | gRPC reflection query | Services and methods discoverable via grpcurl |
| AC-7 | Same command via REST and gRPC | Identical data content in responses |
| AC-8 | Unknown command in Execute | Returns gRPC NotFound status |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestGRPCListCommands | api/grpc/server_test.go | ListCommands returns all commands | |
| TestGRPCExecute | api/grpc/server_test.go | Execute returns response | |
| TestGRPCExecuteUnauthorized | api/grpc/server_test.go | Missing auth returns Unauthenticated | |
| TestGRPCStream | api/grpc/server_test.go | Server-streaming delivers events | |
| TestGRPCConfigSession | api/grpc/server_test.go | Config lifecycle via gRPC | |
| TestGRPCReflection | api/grpc/server_test.go | Reflection enabled, services listed | |
| TestGRPCUnknownCommand | api/grpc/server_test.go | NotFound for unknown command | |
| TestGRPCRESTEquivalence | api/grpc/equivalence_test.go | Same command returns identical data via both transports | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A -- no numeric inputs in gRPC layer.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| test-grpc-api-commands | test/plugin/test-grpc-api-commands.ci | grpcurl list commands | |
| test-grpc-api-execute | test/plugin/test-grpc-api-execute.ci | grpcurl execute command | |
| test-grpc-api-config | test/plugin/test-grpc-api-config.ci | grpcurl config session | |

### Future (if deferring any tests)
- mTLS authentication test
- Load testing with concurrent streams

## Files to Modify
- `cmd/ze/hub/main.go` - start gRPC server after engine is created
- `go.mod` - add google.golang.org/grpc, google.golang.org/protobuf (requires user approval)

## Files to Create
- `api/proto/ze.proto` - proto3 service definitions for ZeService and ZeConfigService
- `api/proto/ze.pb.go` - generated protobuf code
- `api/proto/ze_grpc.pb.go` - generated gRPC code
- `internal/component/api/grpc/server.go` - gRPC server, service implementations
- `internal/component/api/grpc/auth.go` - auth interceptor
- `internal/component/api/grpc/server_test.go` - unit tests
- `internal/component/api/grpc/equivalence_test.go` - REST/gRPC equivalence test
- `test/plugin/test-grpc-api-commands.ci` - functional test
- `test/plugin/test-grpc-api-execute.ci` - functional test
- `test/plugin/test-grpc-api-config.ci` - functional test

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No (config in umbrella spec) | |
| CLI commands/flags | No | |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | test/plugin/test-grpc-api-*.ci |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - gRPC API |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/grpc.md` - gRPC service reference |
| 6 | Has a user guide page? | Yes | `docs/guide/api.md` - gRPC usage examples (grpcurl) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - gRPC API |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + umbrella spec |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | make ze-verify |
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

1. **Phase: Proto definitions** -- write ze.proto, generate Go code
   - Files: api/proto/ze.proto, generated pb.go files
   - Verify: protoc generates without errors
   - Note: requires user approval for new go.mod dependencies

2. **Phase: gRPC server + auth interceptor** -- listener, TLS, auth
   - Tests: TestGRPCExecuteUnauthorized
   - Files: grpc/server.go, grpc/auth.go
   - Verify: tests fail -> implement -> tests pass

3. **Phase: ZeService implementation** -- Execute, Stream, ListCommands, Complete
   - Tests: TestGRPCListCommands, TestGRPCExecute, TestGRPCStream, TestGRPCUnknownCommand
   - Files: grpc/server.go
   - Verify: tests fail -> implement -> tests pass

4. **Phase: ZeConfigService implementation** -- config session RPCs
   - Tests: TestGRPCConfigSession
   - Files: grpc/server.go
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Reflection + equivalence** -- enable reflection, verify REST/gRPC equivalence
   - Tests: TestGRPCReflection, TestGRPCRESTEquivalence
   - Files: grpc/server.go, grpc/equivalence_test.go
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Wire into startup** -- start gRPC server from hub
   - Files: cmd/ze/hub/main.go
   - Tests: functional tests (test-grpc-api-*.ci)

7. **Full verification** -> make ze-verify
8. **Complete spec** -> Fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | All RPCs call engine methods, never dispatcher directly |
| Naming | Proto package ze.api.v1, Go package matches |
| Data flow | gRPC -> Engine -> Dispatcher (never gRPC -> Dispatcher) |
| Rule: no-layering | No duplicate command logic in gRPC handlers |
| Non-divergence | Equivalence test proves identical output with REST |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Proto file | ls api/proto/ze.proto |
| Generated code | ls api/proto/ze.pb.go api/proto/ze_grpc.pb.go |
| gRPC server | grep "grpc.NewServer" api/grpc/server.go |
| Auth interceptor | grep "UnaryInterceptor\|StreamInterceptor" api/grpc/auth.go |
| Reflection | grep "reflection.Register" api/grpc/server.go |
| Equivalence test | grep "TestGRPCRESTEquivalence" api/grpc/equivalence_test.go |
| Functional tests | ls test/plugin/test-grpc-api-*.ci |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Auth enforcement | Interceptor validates token on every RPC |
| TLS | Warn if TLS not configured (log at startup) |
| Input validation | Command strings from proto sanitized |
| Stream cleanup | Server-stream goroutines cleaned up on client disconnect |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
| Proto generation error | Fix proto syntax |
| Functional test fails | Check AC |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

### Response data is JSON bytes, not typed proto messages

CommandResponse carries data as bytes (JSON-encoded). This ensures identical content
with REST. Typed proto responses per command would require a proto message for every
command output format -- unsustainable with 80+ commands. JSON bytes is the pragmatic choice.

### Two services, not one

ZeService handles generic command execution. ZeConfigService handles typed config
session management. Separation keeps proto definitions clean and allows clients to
import only what they need.

### gRPC reflection enabled by default

Reflection lets grpcurl and grpcui discover services without the proto file. This is
essential for usability. The security trade-off is acceptable because the auth interceptor
still protects all RPCs.

### Proto package ze.api.v1

Future breaking changes would use ze.api.v2. Non-breaking changes (new RPCs, new fields)
stay in v1. This matches the REST /api/v1/ versioning.

### New dependency requires user approval

google.golang.org/grpc is not in go.mod today. Per rules/go-standards.md, new third-party
imports require asking the user. This is flagged in Phase 1.

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
N/A.

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
- [ ] Write learned summary to `plan/learned/NNN-api-grpc.md`
- [ ] Summary included in commit
