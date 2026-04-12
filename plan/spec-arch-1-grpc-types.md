# Spec: Domain Request/Response Types at the API Transport Boundary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/compatibility.md` - plugin-API contract, internal is free
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
- Versioning. The plugin-API contract rule in `.claude/rules/compatibility.md`
  is about `pkg/`. This spec is `internal/`, so free to change.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - API engine design
  → Decision: transports are thin adapters, engine owns logic.
- [ ] `.claude/rules/design-principles.md` - "No identity wrappers" rule
  → Constraint: a domain type must transform, not just re-export.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/api/types.go` - `CommandMeta`, `ParamMeta`,
  `ExecResult`, `AuthContext`. No request types.
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
- `api.ExecResult`, `api.CommandMeta`, `api.ParamMeta`, `api.AuthContext`
  remain the return/context types of the engine.

**Behavior to change:**
- Add domain request types mirroring the parameter shape of each engine
  method.
- Move the ad-hoc field extraction out of transport handlers into typed
  `from<Transport>Request` conversion helpers.
- (Optional, decide during design) Change the engine method signatures to
  take the domain request type instead of positional primitives.

## Data Flow

### Entry Point
- gRPC: `*zepb.CommandRequest` → `zeServiceImpl.Execute`
- REST: HTTP request body + URL → `RESTServer.handleExecute`
- Both reach `api.APIEngine.Execute`.

### Transformation Path (target shape)
1. Transport receives wire-format request
2. Transport calls `fromProtoExecuteRequest(*zepb.CommandRequest)` (or
   `fromRESTExecuteRequest(http.Request)`) → `api.ExecuteRequest`
3. Transport calls `engine.Execute(ctx, req api.ExecuteRequest)`
4. Engine returns `*api.ExecuteResponse` (or keeps returning `*ExecResult`
   if we decide to keep the name)
5. Transport calls `toProtoExecuteResponse(*api.ExecuteResponse)` (or the
   REST equivalent)
6. Transport writes the wire-format response

### Boundaries
| Boundary | How | Verified |
|----------|-----|----------|
| proto ↔ domain | helper per method in `internal/component/api/grpc/convert.go` | [ ] |
| JSON/URL ↔ domain | helper per method in `internal/component/api/rest/convert.go` | [ ] |
| domain ↔ engine | engine methods take `*api.<Method>Request` | [ ] |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| gRPC Execute | → | `fromProtoExecuteRequest` → `engine.Execute` | `TestGRPCExecuteUsesDomainType` |
| REST POST /execute | → | `fromRESTExecuteRequest` → `engine.Execute` | `TestRESTExecuteUsesDomainType` |
| gRPC SetConfig | → | `fromProtoConfigSetRequest` → `sessions.Set` | `TestGRPCSetConfigUsesDomainType` |
| REST PUT /config/sessions/:id | → | `fromRESTConfigSetRequest` → `sessions.Set` | `TestRESTSetConfigUsesDomainType` |
| Unit round-trip | → | `toProtoExecuteResponse(fromProtoExecuteRequest(x))` | `TestExecuteRequestRoundTrip` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any file imports `zepb` (`codeberg.org/thomas-mangin/ze/api/proto`) | Only `internal/component/api/grpc/*.go` is allowed. Enforce via a lint/grep test. |
| AC-2 | Engine method called from a transport | The call passes a single `*api.<Method>Request` pointer, not positional primitives. |
| AC-3 | Field renamed in `ze.proto` | At most one conversion helper file changes. Engine, REST, other handlers untouched. |
| AC-4 | New gRPC handler added | The handler is three lines: `req := fromProtoFooRequest(pb); resp, err := s.engine.Foo(ctx, req); return toProtoFooResponse(resp), err`. |
| AC-5 | `ConfigSetRequest` typed as `{ SessionID, Path, Value }` | Parameter-order bugs at the call site fail compile, not runtime. |
| AC-6 | Both transports call the same engine method | Both go through the same domain request shape; no divergence. |
| AC-7 | REST handler body | Extracts fields from `http.Request`, constructs domain request, calls engine. No engine call with naked primitives. |
| AC-8 | Unit tests | Can call engine methods without constructing proto. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExecuteRequestRoundTrip` | `internal/component/api/grpc/convert_test.go` | proto → domain → proto preserves fields | |
| `TestExecuteRequestFromREST` | `internal/component/api/rest/convert_test.go` | JSON → domain preserves fields | |
| `TestEngineExecuteWithDomainRequest` | `internal/component/api/engine_test.go` | Engine takes domain request | |
| `TestGRPCExecuteHandler` | `internal/component/api/grpc/server_test.go` | Handler is a thin wrapper around conversion + engine call | |
| `TestRESTExecuteHandler` | `internal/component/api/rest/server_test.go` | Same for REST | |
| `TestConfigSetRequestPositionalSafety` | `internal/component/api/config_session_test.go` | Compile-time type safety of `{SessionID, Path, Value}` vs positional args | |

### Lint Test (MANDATORY)
| Test | Location | Validates |
|------|----------|-----------|
| Import lint | `tools/lint/proto_leak_test.go` (or equivalent) | `zepb` imports only under `internal/component/api/grpc/` |

### Functional Tests
| Test | Location | End-User Scenario |
|------|----------|-------------------|
| `test-grpc-execute-typed` | `test/plugin/grpc-execute.ci` | gRPC Execute still returns the same wire-format output |
| `test-rest-execute-typed` | `test/plugin/rest-execute.ci` | REST POST /execute still returns the same JSON shape |

## Files to Modify
- `internal/component/api/types.go` - add request types (or move to new `requests.go`)
- `internal/component/api/engine.go` - update method signatures
- `internal/component/api/config_session.go` - update method signatures
- `internal/component/api/grpc/server.go` - call engine with domain types
- `internal/component/api/rest/server.go` - call engine with domain types

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 4 | API/RPC added/changed? | No (wire-format unchanged) | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md` - document the domain-type layer |

## Files to Create
- `internal/component/api/requests.go` - typed request structs
- `internal/component/api/grpc/convert.go` - proto ↔ domain helpers
- `internal/component/api/grpc/convert_test.go`
- `internal/component/api/rest/convert.go` - JSON/URL ↔ domain helpers
- `internal/component/api/rest/convert_test.go`

## Open Design Questions

These are decided at `/ze-spec` time, not now.

1. **Engine signature change or optional?** Option A: rewrite every engine
   method to take `*api.<Method>Request`. Option B: keep the primitive
   signatures, add the domain types only at the transport side. Option A
   pushes type safety all the way to the engine; Option B is smaller churn.

2. **`*Request` struct name collision.** `api.ConfigSetRequest` shadows the
   proto `zepb.ConfigSetRequest` semantically. Both exist, one for each
   layer. Naming: keep them identical (`api.ConfigSetRequest` vs
   `zepb.ConfigSetRequest`) or rename the domain side to something like
   `api.SetConfigParams` to avoid confusion in reviews.

3. **`Execute` stays string-based or gets typed parameters?** Today the
   engine flattens typed params into a `"command key value key value"`
   string. Pattern #7 would argue for `ExecuteRequest { Command string;
   Params map[string]any }`. Deciding this affects how many downstream
   consumers (dispatcher, YANG validators, authorization) have to change.

4. **REST: is a middle conversion layer worth it?** REST is a thinner
   format (JSON decode directly into a Go struct), so the "convert at the
   edge" pattern has less payoff than on the gRPC side where proto types
   carry unwanted ergonomics. Decide whether REST handlers get the full
   convert helper or keep their inline decode.

5. **Where does `AuthContext` get constructed?** Today each transport
   builds it from its own auth metadata. If the domain request carries
   `Auth AuthContext` as an embedded field, the engine signature
   simplifies to `engine.Execute(ctx, req)`.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| The audit would find proto leaks outside the gRPC package | There are no leaks — the problem is the missing domain layer, not a violation of a boundary | Pre-spec grep for `zepb.` | Re-framed the spec from "plug leaks" to "add the missing layer" |

## Design Insights

- **The problem is the missing typed layer, not a boundary violation.**
  Ze's physical boundary is already clean — `zepb` does not leak. But every
  transport handler reinvents the parameter extraction for the same engine
  call. The domain type is the thing that would stop that from being
  reinvented.
- **The gRPC / REST parallelism is the best evidence for the spec.** Every
  engine method has two nearly-identical transport wrappers that exist
  because neither transport can share code via a typed request.

## Implementation Summary

### What Was Implemented
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
- [ ] AC-1..AC-8 demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-verify` passes
- [ ] No `zepb` imports outside `internal/component/api/grpc/`
- [ ] Engine methods take domain request types (or explicit decision to stay
      primitive, captured in learned summary)

### Design
- [ ] No identity wrappers — the domain type MUST add value (typed fields,
      validation, or parameter-order safety)
- [ ] No premature abstraction (three-plus call sites? yes, every RPC)
- [ ] Single responsibility (conversion only)
- [ ] Explicit > implicit

### TDD
- [ ] Tests written
- [ ] Tests FAIL then PASS
- [ ] Lint test proves no proto leakage outside `grpc/`

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-grpc-domain-types.md`
