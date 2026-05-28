# Spec: gNMI Component

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 7/11 |
| Updated | 2026-05-28 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/architecture.md` - API architecture
4. `internal/component/api/grpc/server.go` - existing gRPC transport
5. `internal/component/api/engine.go` - API engine pattern
6. `internal/component/config/tree.go` - config tree operations
7. `internal/component/api/config_session.go` - config session manager

## Task

Add gNMI (gRPC Network Management Interface) support to Ze as a new component
`internal/component/gnmi/`. gNMI is the industry-standard protocol for
YANG-modeled network device management over gRPC, used by automation platforms
(Ansible/Napalm, OpenConfig, network controllers) to read/write config and
stream telemetry.

Ze already has YANG-modeled config, a gRPC server, and config session management.
gNMI maps naturally onto these existing primitives.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - existing API transport pattern
  -> Decision: transports are thin adapters over shared engine; proto types confined to transport package
  -> Constraint: all logic lives in the engine, never in the transport
- [ ] `docs/architecture/core-design.md` - component architecture
  -> Constraint: components register at startup via init(), core discovers through registries
- [ ] `ai/patterns/registration.md` - registration model
  -> Constraint: blank import -> init() -> Registry.Register(...)

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc7950.md` - YANG 1.1 modeling language (already exists)
  -> Constraint: gNMI paths are YANG schema paths; path encoding must match Ze's YANG modules
- [ ] gNMI specification (openconfig/reference, not an IETF RFC)
  -> Constraint: four RPCs (Capabilities, Get, Set, Subscribe); paths use origin/elem encoding

**Key insights:**
- Ze's existing gRPC transport (`internal/component/api/grpc/`) serves the Ze-specific proto; gNMI is a separate, standardized proto service that should be its own component
- The config tree already supports Get/Set/Delete/Walk, which maps to gNMI Get/Set
- ConfigSessionManager already provides transactional config editing (Enter, Set, Delete, Commit, Discard), which maps to gNMI Set (replace/update/delete are batched in one RPC)
- Ze's YANG modules define the schema; gNMI Capabilities reports them
- `github.com/openconfig/goyang` is already a dependency
- gRPC (google.golang.org/grpc) is already vendored

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/api/grpc/server.go` - gRPC server with auth interceptor, TLS, multi-listener binding
- [ ] `internal/component/api/engine.go` - shared API engine: Executor, CommandSource, AuthChecker, StreamSource
- [ ] `internal/component/api/config_session.go` - ConfigSessionManager: Enter(username), Set(ConfigSetRequest), Delete(ConfigDeleteRequest), Diff, Commit, Discard
- [ ] `internal/component/config/tree.go` - config data: Get, Set, Delete, GetContainer, GetList, Walk; concurrent-safe with per-node mutex
- [ ] `internal/component/config/yang/loader.go` - YANG schema loader, DefaultLoader()
- [ ] `api/proto/ze.proto` - existing Ze API proto (ZeService + ZeConfigService)
- [ ] `internal/component/web/sse.go` - EventBroker for live streaming
- [ ] `cmd/ze/hub/main_servers.go` - server startup orchestration

**Behavior to preserve:**
- Existing gRPC API (`ZeService`, `ZeConfigService`) unchanged
- Config tree operations and semantics
- YANG schema loading and validation
- TLS/auth patterns from existing gRPC server

**Behavior to change:**
- None. This is a new component alongside existing API transports.

## Data Flow (MANDATORY)

### Entry Point
- gNMI client connects to gRPC listener (separate port from Ze API)
- Requests arrive as gNMI proto messages (gnmi.GetRequest, gnmi.SetRequest, gnmi.SubscribeRequest)

### Transformation Path
1. gRPC interceptor: TLS termination, auth (bearer token or cert-based)
2. gNMI server handler: parse gNMI path elements into Ze config tree path
3. Path translation: gNMI `origin/elem[key=val]` -> Ze config path segments
4. Operation dispatch:
   - **Capabilities**: enumerate Ze's YANG modules from the loader
   - **Get**: read from running config tree; return as JSON_IETF or proto-encoded TypedValue
   - **Set**: create ConfigSession via Enter(username), apply replace/update/delete ops via Set/Delete, Commit
   - **Subscribe**: register listener for config changes or telemetry samples

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| gNMI proto <-> Ze config path | Path translator (gnmi.Path -> []string) | [ ] |
| gNMI Set <-> ConfigSessionManager | Enter/Set/Delete/Commit | [ ] |
| gNMI Get <-> Config tree | Tree.Get / tree walker | [ ] |
| gNMI Subscribe <-> Event bus | Config change notifications | [ ] |

### Integration Points
- `internal/component/config.Tree` - read/write config state
- `internal/component/api.ConfigSessionManager` - transactional config edits (Enter, Set, Delete, Commit, Discard)
- `internal/component/config/yang.DefaultLoader()` - YANG module enumeration
- `internal/component/web.EventBroker` (or bus equivalent) - change notifications for Subscribe

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| gRPC listener on configured port | -> | gnmi.Server.Capabilities() | `TestGNMICapabilitiesWiring` |
| gNMI GetRequest | -> | gnmi.Server.Get() -> config tree read | `TestGNMIGetWiring` |
| gNMI SetRequest | -> | gnmi.Server.Set() -> config session commit | `TestGNMISetWiring` |
| gNMI SubscribeRequest | -> | gnmi.Server.Subscribe() -> change stream | `TestGNMISubscribeWiring` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | gNMI Capabilities RPC | Returns supported YANG models (name, version, organization) and supported encodings (JSON_IETF, PROTO) |
| AC-2 | gNMI Get for a known config path (e.g., `/bgp/neighbor[address=X]/description`) | Returns correct TypedValue with the config leaf value |
| AC-3 | gNMI Get for a container path | Returns the subtree as JSON_IETF |
| AC-4 | gNMI Get for nonexistent path | Returns gRPC NOT_FOUND status |
| AC-5 | gNMI Set with update operation | Config leaf is updated; commit applies the change |
| AC-6 | gNMI Set with replace operation on a container | Entire container replaced with new content |
| AC-7 | gNMI Set with delete operation | Config path removed; commit applies |
| AC-8 | gNMI Set with mixed update+delete in one request | All operations applied atomically via single session commit |
| AC-9 | gNMI Subscribe ONCE mode | Returns current state snapshot, then closes stream |
| AC-10 | gNMI Subscribe STREAM mode with ON_CHANGE | Client receives updates when config changes |
| AC-11 | Auth: unauthenticated client rejected | gRPC UNAUTHENTICATED status when no valid credentials |
| AC-12 | TLS: server presents TLS cert | Connections use TLS (reuse PKI cert pattern from web/api) |
| AC-13 | YANG path with list keys (e.g., `neighbor[address=10.0.0.1]`) | Path correctly resolved to list entry in config tree |
| AC-14 | Component disabled when not imported | No gRPC listener, no binary size impact when gnmi package not blank-imported |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPathTranslation` | `internal/component/gnmi/path_test.go` | gNMI path <-> Ze config path conversion | |
| `TestPathWithListKeys` | `internal/component/gnmi/path_test.go` | List key extraction from gNMI PathElem | |
| `TestCapabilitiesResponse` | `internal/component/gnmi/capabilities_test.go` | YANG model enumeration | |
| `TestGetLeafValue` | `internal/component/gnmi/get_test.go` | Leaf read from config tree | |
| `TestGetContainer` | `internal/component/gnmi/get_test.go` | Subtree serialization as JSON_IETF | |
| `TestGetNotFound` | `internal/component/gnmi/get_test.go` | Missing path returns error | |
| `TestSetUpdate` | `internal/component/gnmi/set_test.go` | Update operation through session | |
| `TestSetReplace` | `internal/component/gnmi/set_test.go` | Replace operation through session | |
| `TestSetDelete` | `internal/component/gnmi/set_test.go` | Delete operation through session | |
| `TestSetAtomic` | `internal/component/gnmi/set_test.go` | Mixed ops in single transaction | |
| `TestSubscribeOnce` | `internal/component/gnmi/subscribe_test.go` | Snapshot delivery then close | |
| `TestSubscribeStream` | `internal/component/gnmi/subscribe_test.go` | Change event delivery | |
| `TestAuthInterceptor` | `internal/component/gnmi/auth_test.go` | Reject unauthenticated requests | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| gNMI path depth | 1-64 | 64 elements | 0 (empty path) | 65+ (reject) |
| Subscribe sample_interval | 0-max int64 | max int64 ns | N/A (0 = on-change) | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-gnmi-capabilities` | `test/gnmi/capabilities.ci` | Client discovers YANG models | |
| `test-gnmi-get-config` | `test/gnmi/get-config.ci` | Client reads running config via gNMI | |
| `test-gnmi-set-config` | `test/gnmi/set-config.ci` | Client modifies config via gNMI Set | |
| `test-gnmi-subscribe` | `test/gnmi/subscribe.ci` | Client subscribes to config changes | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `gnmi-gnmic-get` | `test/interop/scenarios/` | gnmic (OpenConfig CLI) | Standard gNMI client can Get config | |
| `gnmi-gnmic-set` | `test/interop/scenarios/` | gnmic | Standard gNMI client can Set config | |
| `gnmi-gnmic-subscribe` | `test/interop/scenarios/` | gnmic | Standard gNMI client can Subscribe | |

### Future (if deferring any tests)
- Telemetry Subscribe SAMPLE mode (periodic counters) -- requires telemetry collector integration, separate spec
- POLL subscription mode -- lower priority, ONCE and STREAM cover primary use cases

## Files to Modify
- `internal/component/api/config_session.go` - possibly expose session creation for gnmi (or use existing factory)
- `cmd/ze/hub/main_servers.go` - add gNMI server startup (or gnmi registers itself)
- `go.mod` / `go.sum` - add `github.com/openconfig/gnmi` dependency

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [x] | `internal/component/gnmi/schema/` - gnmi listener config under `service/gnmi/` |
| YANG validation constraints | [x] | Port range, TLS mode enum |
| YANG custom validators | [ ] | N/A |
| CLI commands/flags | [x] | `show service gnmi` status command |
| CLI grammar (action before identifier) | [x] | `show service gnmi` |
| Editor autocomplete | [x] | YANG enum for encoding, TLS mode |
| Functional test for new RPC/API | [x] | `test/gnmi/*.ci` |
| Pipe completeness | [x] | `show service gnmi` output through ApplyPipes |
| Env var registration | [x] | `ze.gnmi.enabled`, `ze.gnmi.listen` |
| Doctor check for runtime dependencies | [ ] | N/A (no external deps beyond gRPC) |
| Prometheus counters/metrics | [x] | `gnmi_requests_total`, `gnmi_subscribe_active`, `gnmi_errors_total` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [x] | `docs/features.md` |
| 2 | Config syntax changed? | [x] | `docs/guide/configuration.md` - service/gnmi section |
| 3 | CLI command added/changed? | [x] | `docs/guide/command-reference.md` - show service gnmi |
| 4 | API/RPC added/changed? | [x] | `docs/architecture/api/commands.md` - gNMI section |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [x] | `docs/guide/gnmi.md` |
| 7 | Wire format changed? | [ ] | N/A (standard gNMI proto) |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A (gNMI is OpenConfig spec, not IETF RFC) |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [x] | `docs/comparison.md` - gNMI support |
| 12 | Internal architecture changed? | [ ] | N/A |
| 13 | Route metadata keys added/changed? | [ ] | N/A |
| 14 | Prometheus counters added/changed? | [x] | `docs/architecture/telemetry/` |

## Files to Create
- `internal/component/gnmi/server.go` - gNMI gRPC server (Capabilities, Get, Set, Subscribe handlers)
- `internal/component/gnmi/path.go` - gNMI path <-> Ze config path translation
- `internal/component/gnmi/get.go` - Get RPC: config tree read + TypedValue encoding
- `internal/component/gnmi/set.go` - Set RPC: config session create/modify/commit
- `internal/component/gnmi/subscribe.go` - Subscribe RPC: ONCE, STREAM modes
- `internal/component/gnmi/capabilities.go` - Capabilities RPC: YANG model enumeration
- `internal/component/gnmi/auth.go` - auth interceptor (reuse pattern from api/grpc)
- `internal/component/gnmi/register.go` - component registration via init()
- `internal/component/gnmi/schema/gnmi.yang` - YANG schema for service/gnmi config
- `internal/component/gnmi/path_test.go` - path translation tests
- `internal/component/gnmi/get_test.go` - Get handler tests
- `internal/component/gnmi/set_test.go` - Set handler tests
- `internal/component/gnmi/subscribe_test.go` - Subscribe handler tests
- `internal/component/gnmi/capabilities_test.go` - Capabilities tests
- `internal/component/gnmi/auth_test.go` - auth tests
- `test/gnmi/capabilities.ci` - functional test
- `test/gnmi/get-config.ci` - functional test
- `test/gnmi/set-config.ci` - functional test
- `test/gnmi/subscribe.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7. Critical review | Critical Review Checklist below |
| 8. Fix issues | Fix every issue from critical review |
| 9. Re-verify | Re-run stage 6 |
| 10. Repeat 7-9 | Until clean |
| 11. Deliverables review | Deliverables Checklist below |
| 12. Security review | Security Review Checklist below |
| 13. Re-verify | Re-run stage 6 |
| 14. Present summary | Executive Summary Report |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- register component, start gRPC listener, stub handlers
   - Tests: `TestGNMICapabilitiesWiring`
   - Files: `register.go`, `server.go` (skeleton), YANG schema
   - Verify: gRPC listener binds; Capabilities returns empty response; wiring test fails because real logic is stub

2. **Phase: Path Translation** -- gNMI path <-> Ze config path bidirectional conversion
   - Tests: `TestPathTranslation`, `TestPathWithListKeys`
   - Files: `path.go`, `path_test.go`
   - Verify: paths with containers, lists, keys correctly translated

3. **Phase: Capabilities** -- enumerate YANG modules
   - Tests: `TestCapabilitiesResponse`
   - Files: `capabilities.go`, `capabilities_test.go`
   - Verify: returns Ze YANG modules with correct metadata

4. **Phase: Get** -- read config tree via gNMI
   - Tests: `TestGetLeafValue`, `TestGetContainer`, `TestGetNotFound`
   - Files: `get.go`, `get_test.go`
   - Verify: leaf values, container subtrees, missing paths all handled

5. **Phase: Set** -- modify config via gNMI
   - Tests: `TestSetUpdate`, `TestSetReplace`, `TestSetDelete`, `TestSetAtomic`
   - Files: `set.go`, `set_test.go`
   - Verify: all three operations work individually and combined in one request

6. **Phase: Subscribe** -- streaming config changes
   - Tests: `TestSubscribeOnce`, `TestSubscribeStream`
   - Files: `subscribe.go`, `subscribe_test.go`
   - Verify: ONCE returns snapshot then closes; STREAM delivers changes

7. **Phase: Auth + TLS** -- authentication and encryption
   - Tests: `TestAuthInterceptor`
   - Files: `auth.go`, `auth_test.go`
   - Verify: unauthenticated requests rejected; TLS works

8. **Functional tests** -- end-to-end from gNMI client perspective
9. **gNMI spec refs** -- gNMI specification section references in comments
10. **Full verification** -- `make ze-verify`
11. **Complete spec** -- audit, learned summary, two commits

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Path translation bidirectional; Set atomicity; Subscribe cleanup on disconnect |
| Naming | YANG paths use kebab-case; gNMI PathElem names match Ze YANG module names |
| Data flow | Get reads from running tree only; Set goes through ConfigSessionManager only |
| CLI grammar | `show service gnmi` -- action before identifier |
| Doctor checks | N/A (no external runtime deps) |
| YANG validation | Port leaf has `range "1..65535"`; TLS mode is enum |
| Prometheus counters | `gnmi_requests_total{rpc}`, `gnmi_subscribe_active`, `gnmi_errors_total{rpc,code}` |
| Rule: no-layering | gNMI server is independent of Ze API server; no wrapping |
| Rule: exact-or-reject | Path translation must be exact; unknown paths return NOT_FOUND |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `internal/component/gnmi/` package exists | `ls internal/component/gnmi/` |
| gNMI proto dep in go.mod | `grep openconfig/gnmi go.mod` |
| register.go with init() | `grep 'func init' internal/component/gnmi/register.go` |
| YANG schema for service/gnmi | `ls internal/component/gnmi/schema/` |
| All unit tests pass | `go test ./internal/component/gnmi/...` |
| Functional tests exist | `ls test/gnmi/*.ci` |
| `show service gnmi` CLI command | `grep 'show service gnmi' internal/component/gnmi/` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | gNMI path depth bounded; SetRequest value size bounded; no path traversal |
| Auth bypass | Every RPC goes through auth interceptor; no unauthenticated code path |
| TLS enforcement | Default to TLS; plaintext only with explicit config flag |
| Resource exhaustion | Subscribe client count bounded; per-client event buffer bounded |
| Error leakage | gRPC status codes; no internal paths or stack traces in error messages |

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
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Separate component (`internal/component/gnmi/`) not a transport under `api/` | Add gNMI as third transport in `api/` alongside grpc/rest | gNMI is a standardized protocol with its own proto service definition, authentication model, and subscription semantics. It is not another transport for Ze's custom API. Keeping it separate avoids coupling and allows independent lifecycle (optional import). |
| Own gRPC server instance (separate port) | Share gRPC server with Ze API, register gNMI service on same server | Separate ports allow independent TLS/auth config. Automation tools expect gNMI on a known port (typically 9339). Ze API has its own auth model. Separate servers keep each cleanly configurable. |
| Reuse ConfigSessionManager for Set | Direct tree manipulation | ConfigSessionManager provides atomicity, validation, and commit/discard semantics that gNMI Set requires. Reimplementing would duplicate logic. |
| JSON_IETF as primary encoding | PROTO encoding first | JSON_IETF is human-readable and most widely supported by gNMI clients (gnmic, Ansible, etc.). PROTO can be added later. |

## Known Limitations
- Telemetry SAMPLE mode (periodic counter streaming) deferred to a follow-up spec; requires deeper telemetry collector integration
- POLL subscription mode deferred; ONCE and STREAM cover the primary use cases
- gNMI path target field (multi-target) not supported initially; single-device only
- No gNMI Extension support initially

## RFC Documentation

gNMI is not an IETF RFC. Reference the OpenConfig gNMI specification:
- `// gNMI Specification Section 3.2: Capabilities RPC` above capability handler
- `// gNMI Specification Section 3.3: Get RPC` above get handler
- `// gNMI Specification Section 3.4: Set RPC` above set handler
- `// gNMI Specification Section 3.5: Subscribe RPC` above subscribe handler

## Implementation Summary

### What Was Implemented
- gNMI gRPC server with all four RPCs: Capabilities, Get, Set, Subscribe
- Bearer token auth interceptor (unary + stream)
- TLS support via cert/key pair
- gNMI path <-> Ze config path translation with list key support
- Capabilities: enumerates Ze YANG modules from loader
- Get: leaf values (StringVal), containers (JSON_IETF), NOT_FOUND for missing
- Set: update/replace/delete via ConfigSessionManager, atomic commit
- Subscribe: ONCE (snapshot + sync), STREAM (ChangeNotifier broadcast)
- Env vars: ze.gnmi.enabled, ze.gnmi.listen, ze.gnmi.token
- Hub wiring: startup when enabled, clean shutdown
- 19 tests (unit + integration) with -race

### Bugs Found/Fixed
- time.After(0) in select with channel send races (picks randomly); fixed with default:

### Documentation Updates
- None yet (deferred to follow-up)

### Deviations from Plan
- Functional .ci tests deferred: gNMI tests are Go integration tests through live gRPC, not .ci format
- Interop tests with gnmic deferred: requires gnmic binary in test infrastructure
- YANG schema for service/gnmi/ config deferred: currently env-var-only configuration
- show service gnmi CLI command deferred: requires YANG schema first
- Prometheus counters deferred: requires metrics registry wiring

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| gNMI clients can discover Ze's YANG models | functional test | `test-gnmi-capabilities` |
| gNMI clients can read running config | functional test | `test-gnmi-get-config` |
| gNMI clients can modify config | functional test | `test-gnmi-set-config` |
| gNMI clients can subscribe to changes | functional test | `test-gnmi-subscribe` |
| Standard gNMI client (gnmic) works | interop test | `gnmi-gnmic-get`, `gnmi-gnmic-set` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [pending]

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
- [ ] AC-1..AC-14 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] gNMI spec section comments added
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
- [ ] Interop tests for protocol features
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/803-gnmi.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-gnmi.md`
