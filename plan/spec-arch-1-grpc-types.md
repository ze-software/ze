# Spec: Domain Request/Response Types at the API Transport Boundary

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/4 |
| Updated | 2026-05-17 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/compatibility.md` - plugin-API contract, internal is free
4. `internal/component/api/types.go` - existing domain types
5. `internal/component/api/engine.go` - engine signature
6. `internal/component/api/grpc/server.go` - gRPC transport (reference)
7. `internal/component/api/rest/server.go` - REST transport (reference)
8. `api/proto/ze.proto` - the gRPC proto surface

## Task

Introduce a typed domain request/response layer between ze's API transports
(gRPC, REST) and `internal/component/api/`. Today every transport handler
extracts wire-format fields inline and calls the engine with positional
primitives; the typed intermediate layer that would let both transports
share parameter shapes does not exist. Add it, so proto types stay confined
to the gRPC transport, JSON decode stays confined to REST, and the engine
sees a single Go-idiomatic request type per method.

Scope: the typed request/response layer between transports and the engine.
Out of scope: rewriting the engine's command dispatch (it still accepts
string commands today; whether it gets typed parameters is an open design
question below).

### Audit (already done for this spec)

| Question | Answer |
|----------|--------|
| Where do `zepb` types appear outside `internal/component/api/grpc/`? | Nowhere. Only `grpc/server.go` and `grpc/server_test.go` import `zepb`. No leaks. |
| Where are domain request types defined in `internal/component/api/`? | Nowhere. `types.go` has `CommandMeta`, `ParamMeta`, `ExecResult`, `AuthContext`. No `ExecuteRequest`, `DescribeCommandRequest`, `ConfigSetRequest`. |
| How do transports call the engine today? | Each handler extracts proto/HTTP fields inline and calls `engine.Execute(auth, command string)` with a string-concatenated command. `ExecResult`, `CommandMeta`, `ErrUnauthorized`, `ErrNotFound` come back as Go types and are re-wrapped per transport. |
| Does the REST transport have the same shape? | Yes. `internal/component/api/rest/server.go` extracts JSON fields inline in each handler and calls the same engine methods. Two parallel boundaries with no shared request type. |
| Is the engine signature Prometheus/proto-aware? | No. `APIEngine.Execute(auth AuthContext, command string) (*ExecResult, error)`. Pure Go. |

So the work is not "plug a leak" (there isn't one). It is "define the domain
request types that both transports already build in an ad-hoc way, and make
the engine take them instead of re-extracted primitives". This is the piece
that is missing, not the piece that is broken.

### Why now, not later

- The gRPC surface just landed (`api/proto/ze.proto`, `api/proto/ze.pb.go`,
  `internal/component/api/grpc/server.go`). The engine is still small, all
  handlers are trivial wrappers, and REST and gRPC have identical call
  shapes. The window for introducing a shared request layer without churn is
  now.
- Every future RPC that gets added without a typed layer is one more parallel
  extraction + call site that has to be reworked later.
- The `Execute(command string)` signature hides the typed structure of
  parameters. Typed RPCs (bgp, config, peer) all flatten to strings at the
  transport boundary. A domain request type for commands that have real
  parameters would catch parameter-order and type mistakes at compile time.

### Out of scope

- Changing the engine dispatch mechanism (string commands vs typed). That is
  a separate, larger question. This spec keeps the current command-string
  dispatch and adds a domain layer over it.
- Generating these types from the proto. Hand-written domain types are the
  point: they decouple from the proto shape.
- Versioning. The plugin-API contract rule in `ai/rules/compatibility.md`
  is about `pkg/`. This spec is `internal/`, so free to change.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API engine design
  → Decision: transports are thin adapters, engine owns logic.
- [ ] `ai/rules/design-principles.md` - "No identity wrappers" rule
  → Constraint: a domain type must transform, not just re-export.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/api/types.go` - `CommandMeta`, `ParamMeta`,
  `ExecResult`, `CallerIdentity`. No request types.
- [ ] `internal/component/api/engine.go` - `Execute`, `ListCommands`,
  `DescribeCommand`, `Stream`. All take primitive args (`string command`,
  `string prefix`, `string path`). Return domain result types.
- [ ] `internal/component/api/config_session.go` - `Enter`, `Set`, `Delete`,
  `Diff`, `Commit`, `Discard`. All take `(username, sessionID, path, value)`
  style primitive args.
- [ ] `internal/component/api/grpc/server.go` - nine handlers, each extracts
  proto fields inline and calls the engine with primitives. Result converted
  back to proto via `execResultToProto`, `commandMetaToProto`.
- [ ] `internal/component/api/rest/server.go` - parallel set of handlers,
  each extracts JSON/URL fields inline and calls the same engine methods.

**Behavior to preserve:**
- Every RPC / REST endpoint produces the same wire-format output.
- The `api.APIEngine` interface remains the sole entry point for transports
  (no direct dispatcher access).
- `api.ExecResult`, `api.CommandMeta`, `api.ParamMeta`, `api.CallerIdentity`
  remain the return/context types of the engine.

**Behavior to change:**
- Add domain request types mirroring the parameter shape of each engine
  method.
- Move the ad-hoc field extraction out of transport handlers into typed
  `from<Transport>Request` conversion helpers.
- Change the engine method signatures to take the domain request type
  instead of positional primitives (Design Decision 1).

## Data Flow (MANDATORY)

### Entry Point
- gRPC: `*zepb.CommandRequest` arrives at `zeServiceImpl.Execute`
- REST: HTTP request body + URL arrives at `RESTServer.handleExecute`
- Both reach `api.APIEngine.Execute`.

### Transformation Path
1. Transport receives wire-format request (proto or HTTP JSON)
2. Transport calls convert helper: `fromProtoExecuteRequest` or `fromRESTExecuteRequest` producing `*api.ExecuteRequest`
3. Transport calls `engine.Execute(ctx, req)` with the domain request
4. Engine returns `*api.ExecResult` (unchanged return type)
5. Transport calls `toProtoExecuteResponse` or writes JSON directly
6. Transport writes the wire-format response

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| proto to domain | helper per method in `internal/component/api/grpc/convert.go` | [ ] |
| JSON/URL to domain | helper per method in `internal/component/api/rest/convert.go` | [ ] |
| domain to engine | engine methods take `*api.<Method>Request` | [ ] |

### Integration Points
- `api.APIEngine` methods - signatures change from primitives to request structs
- `api.ConfigSessionManager` methods - signatures change from positional strings to request structs
- `grpc/server.go` handlers - call convert helpers instead of inline extraction
- `rest/server.go` handlers - call convert helpers instead of inline extraction
- Existing tests (`grpc/server_test.go`, `rest/server_test.go`, `engine_test.go`, `config_session_test.go`) - must be updated for new signatures

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gRPC Execute | → | `fromProtoExecuteRequest` then `engine.Execute` | `TestGRPCExecuteUsesDomainType` |
| REST POST /execute | → | `fromRESTExecuteRequest` then `engine.Execute` | `TestRESTExecuteUsesDomainType` |
| gRPC SetConfig | → | `fromProtoConfigSetRequest` then `sessions.Set` | `TestGRPCSetConfigUsesDomainType` |
| REST PUT /config/sessions/:id | → | `fromRESTConfigSetRequest` then `sessions.Set` | `TestRESTSetConfigUsesDomainType` |
| Unit round-trip | → | `toProtoExecuteResponse(fromProtoExecuteRequest(x))` | `TestExecuteRequestRoundTrip` |
| Functional gRPC | → | full gRPC execute path | `test/plugin/grpc-execute.ci` |
| Functional REST | → | full REST execute path | `test/plugin/rest-execute.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any file imports `zepb` (`codeberg.org/thomas-mangin/ze/api/proto`) | Only `internal/component/api/grpc/*.go` is allowed. Enforce via a lint/grep test. |
| AC-2 | Engine method called from a transport | The call passes a single `*api.<Method>Request` pointer, not positional primitives. |
| AC-3 | Field renamed in `ze.proto` | At most one conversion helper file changes. Engine, REST, other handlers untouched. |
| AC-4 | New gRPC handler added | The handler is three lines: convert request, call engine, convert response. |
| AC-5 | `ConfigSetRequest` typed as `{ SessionID, Path, Value }` | Parameter-order bugs at the call site fail compile, not runtime. |
| AC-6 | Both transports call the same engine method | Both go through the same domain request shape; no divergence. |
| AC-7 | REST handler body | Extracts fields from `http.Request`, constructs domain request, calls engine. No engine call with naked primitives. |
| AC-8 | Unit tests | Can call engine methods without constructing proto. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExecuteRequestRoundTrip` | `internal/component/api/grpc/convert_test.go` | proto to domain to proto preserves fields | |
| `TestExecuteRequestFromREST` | `internal/component/api/rest/convert_test.go` | JSON to domain preserves fields | |
| `TestEngineExecuteWithDomainRequest` | `internal/component/api/engine_test.go` | Engine takes domain request | |
| `TestGRPCExecuteHandler` | `internal/component/api/grpc/server_test.go` | Handler is a thin wrapper around conversion + engine call | |
| `TestRESTExecuteHandler` | `internal/component/api/rest/server_test.go` | Same for REST | |
| `TestConfigSetRequestPositionalSafety` | `internal/component/api/config_session_test.go` | Compile-time type safety of `{SessionID, Path, Value}` vs positional args | |
| `TestBuildCommand` | `internal/component/api/requests_test.go` | Shared BuildCommand helper handles params correctly | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

No numeric inputs in request types (all string fields).

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-grpc-execute-typed` | `test/plugin/grpc-execute.ci` | gRPC Execute still returns the same wire-format output | |
| `test-rest-execute-typed` | `test/plugin/rest-execute.ci` | REST POST /execute still returns the same JSON shape | |

### Lint Test (MANDATORY)
| Test | Location | Validates |
|------|----------|-----------|
| Import lint | `internal/component/api/grpc/proto_leak_test.go` | `zepb` imports only under `internal/component/api/grpc/` |

## Files to Modify
- `internal/component/api/engine.go` - update method signatures to take request structs
- `internal/component/api/config_session.go` - update method signatures to take request structs
- `internal/component/api/engine_test.go` - update tests for new signatures
- `internal/component/api/config_session_test.go` - update tests for new signatures
- `internal/component/api/grpc/server.go` - call engine with domain types via convert helpers
- `internal/component/api/grpc/server_test.go` - update tests for new handler shape
- `internal/component/api/rest/server.go` - call engine with domain types via convert helpers
- `internal/component/api/rest/server_test.go` - update tests for new handler shape

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/grpc-execute.ci`, `test/plugin/rest-execute.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No (wire-format unchanged) | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md` - document the domain-type layer |

## Files to Create
- `internal/component/api/requests.go` - typed request structs
- `internal/component/api/requests_test.go` - BuildCommand tests
- `internal/component/api/grpc/convert.go` - proto to/from domain helpers
- `internal/component/api/grpc/convert_test.go` - round-trip tests
- `internal/component/api/grpc/proto_leak_test.go` - import lint enforcement
- `internal/component/api/rest/convert.go` - JSON/URL to domain helpers
- `internal/component/api/rest/convert_test.go` - conversion tests
- `test/plugin/grpc-execute.ci` - functional test for gRPC execute
- `test/plugin/rest-execute.ci` - functional test for REST execute

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. /ze-review gate | Review Gate section |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 6. Critical review | Critical Review Checklist below |
| 7. Fix issues | Fix every issue from critical review |
| 8. Re-verify | Re-run stage 5 |
| 9. Repeat 6-8 | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Re-verify | Re-run stage 5 |
| 13. Present summary | Summary report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Domain request types** - Define all request structs in `requests.go` and the shared `BuildCommand` helper
   - Tests: `TestBuildCommand`
   - Files: `internal/component/api/requests.go`, `internal/component/api/requests_test.go`
   - Verify: tests fail (no impl) then pass

2. **Phase: Engine signature migration** - Change `APIEngine` and `ConfigSessionManager` methods to accept request structs
   - Tests: `TestEngineExecuteWithDomainRequest`, `TestConfigSetRequestPositionalSafety`
   - Files: `internal/component/api/engine.go`, `internal/component/api/config_session.go`, `internal/component/api/engine_test.go`, `internal/component/api/config_session_test.go`
   - Verify: all existing tests updated and passing with new signatures

3. **Phase: gRPC convert helpers + handler update** - Add `grpc/convert.go` and rewrite handlers to use it
   - Tests: `TestExecuteRequestRoundTrip`, `TestGRPCExecuteHandler`, proto leak lint test
   - Files: `internal/component/api/grpc/convert.go`, `internal/component/api/grpc/convert_test.go`, `internal/component/api/grpc/proto_leak_test.go`, `internal/component/api/grpc/server.go`, `internal/component/api/grpc/server_test.go`
   - Verify: tests fail then pass, handlers are three-line pattern

4. **Phase: REST convert helpers + handler update** - Add `rest/convert.go` and rewrite handlers to use it
   - Tests: `TestExecuteRequestFromREST`, `TestRESTExecuteHandler`
   - Files: `internal/component/api/rest/convert.go`, `internal/component/api/rest/convert_test.go`, `internal/component/api/rest/server.go`, `internal/component/api/rest/server_test.go`
   - Verify: tests fail then pass, handlers are three-line pattern

5. **Functional tests** - Create .ci tests covering end-to-end paths
   - Files: `test/plugin/grpc-execute.ci`, `test/plugin/rest-execute.ci`

6. **Full verification** - `make ze-verify` (lint + all ze tests except fuzz)

7. **Complete spec** - Fill audit tables, write learned summary to `plan/learned/NNN-grpc-domain-types.md`, delete spec from `plan/`. BLOCKING: summary is part of the commit, not a follow-up.

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-8 has implementation with file:line |
| Correctness | Engine Execute still produces identical ExecResult for same inputs; no behavior change |
| Naming | Request types use `<Method>Request` pattern; package qualifier disambiguates from proto |
| Data flow | Conversion happens only in convert.go files; no inline extraction remains in handlers |
| No identity wrappers | Every domain type adds value (typed fields, parameter-order safety, or shared BuildCommand) |
| Proto containment | `zepb` imports exist only in `internal/component/api/grpc/` |
| Three-line handlers | Every gRPC/REST handler follows: convert, call engine, convert response |
| Old code deleted | No remnants of inline extraction in handlers; buildCommand moved to shared location |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `requests.go` with all request structs | `ls internal/component/api/requests.go` |
| `grpc/convert.go` with proto-domain helpers | `ls internal/component/api/grpc/convert.go` |
| `rest/convert.go` with HTTP-domain helpers | `ls internal/component/api/rest/convert.go` |
| Engine takes request structs | `grep -n 'func.*APIEngine.*Request' internal/component/api/engine.go` |
| ConfigSessionManager takes request structs | `grep -n 'func.*ConfigSessionManager.*Request' internal/component/api/config_session.go` |
| No zepb outside grpc/ | `grep -rn 'zepb' internal/component/api/ --include="*.go" \| grep -v grpc/` returns empty |
| BuildCommand shared helper | `grep -n 'func BuildCommand' internal/component/api/requests.go` |
| Proto leak lint test | `ls internal/component/api/grpc/proto_leak_test.go` |
| Functional tests exist | `ls test/plugin/grpc-execute.ci test/plugin/rest-execute.ci` |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | BuildCommand must reject whitespace in keys/values (carried over from existing buildCommand) |
| No new trust boundaries | Convert helpers do not add auth checks; auth remains in interceptors/middleware |
| No information leakage | Error messages from convert helpers do not expose internal state |
| No injection via params | Parameter key/value validation prevents command injection via whitespace splitting |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural, revisit design |
| Functional test fails | Check AC; if AC wrong, revisit design; if AC correct, fix implementation |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Design Decisions

Resolved during `/ze-design` session (2026-05-13).

| # | Question | Decision | Rationale |
|---|----------|----------|-----------|
| 1 | Engine signature: rewrite to take `*Request` structs, or keep primitives? | All engine methods take `*Request` structs | Uniformity, AC-4 three-line handlers, positional safety for config session methods (e.g. can't swap Path and Value) |
| 2 | Naming: `<Method>Request` vs `<Method>Params` vs verb-first? | `<Method>Request` (e.g. `api.ExecuteRequest`, `api.ConfigSetRequest`) | Package qualifier disambiguates from proto names (`zepb.CommandRequest` vs `api.ExecuteRequest`); matches engine method names |
| 3 | `Execute` gets typed `Params map[string]string` or pre-flattened `Command string`? | Pre-flattened `Command string`; shared `api.BuildCommand` helper deduplicates the flatten logic | Dispatcher is string-based, consumers send CLI-style strings, no structured dispatch planned. Spec explicitly scopes out dispatcher changes |
| 4 | REST: full convert-helper pattern (`rest/convert.go`) or keep inline decode? | Full convert helpers in `rest/convert.go`, mirroring `grpc/convert.go` | Structural symmetry between transports; AC-4 uniform three-line handlers; conversion logic independently testable |
| 5 | `CallerIdentity`: embedded in request struct or separate parameter? | Embedded as `Caller CallerIdentity` field in every request struct | Consistent with Decision 1 (request struct is the single arg); keeps handlers at three lines; convert helper is the right place to inject transport-sourced identity |

### Resulting Engine Signatures

- `APIEngine.Execute(ctx, *ExecuteRequest) (*ExecResult, error)` - single request struct with Caller and Command
- `APIEngine.ListCommands(*ListCommandsRequest) []CommandMeta` - request struct with Caller and Prefix
- `APIEngine.DescribeCommand(*DescribeCommandRequest) (CommandMeta, error)` - request struct with Caller and Path
- `APIEngine.Stream(ctx, *StreamRequest) (<-chan string, func(), error)` - request struct with Caller and Command
- `ConfigSessionManager.Set(*ConfigSetRequest) error` - request struct with Username, SessionID, Path, Value
- `ConfigSessionManager.Delete(*ConfigDeleteRequest) error` - request struct with Username, SessionID, Path
- `ConfigSessionManager.Diff(*ConfigDiffRequest) (string, error)` - request struct with Username, SessionID
- `ConfigSessionManager.Commit(*ConfigCommitRequest) error` - request struct with Username, SessionID
- `ConfigSessionManager.Discard(*ConfigDiscardRequest) error` - request struct with Username, SessionID

### Resulting Handler Pattern (both transports)

gRPC Execute: convert proto request + caller identity from context, call engine.Execute, convert result to proto response, return with grpcError.

REST Execute: call fromRESTExecuteRequest(r), handle parse error, call engine.Execute, write result with writeResult.

### Shared Helper

`BuildCommand(command string, params map[string]string) (string, error)` - Moved from duplicated implementations in `grpc/server.go:buildCommand` and `rest/server.go` inline loop. Called by transport convert helpers before setting `ExecuteRequest.Command`.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The audit would find proto leaks outside the gRPC package | There are no leaks; the problem is the missing domain layer, not a violation of a boundary | Pre-spec grep for `zepb.` | Re-framed the spec from "plug leaks" to "add the missing layer" |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- **The problem is the missing typed layer, not a boundary violation.**
  Ze's physical boundary is already clean; `zepb` does not leak. But every
  transport handler reinvents the parameter extraction for the same engine
  call. The domain type is the thing that would stop that from being
  reinvented.
- **The gRPC / REST parallelism is the best evidence for the spec.** Every
  engine method has two nearly-identical transport wrappers that exist
  because neither transport can share code via a typed request.

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement)

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

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /implement)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] AC-1..AC-8 demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] No `zepb` imports outside `internal/component/api/grpc/`
- [ ] Engine methods take domain request types
- [ ] Architecture docs updated

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No identity wrappers (the domain type MUST add value)
- [ ] No premature abstraction (three-plus call sites? yes, every RPC)
- [ ] Single responsibility (conversion only)
- [ ] Explicit > implicit

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Lint test proves no proto leakage outside `grpc/`
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING)
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-grpc-domain-types.md`
