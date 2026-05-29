# Spec: ipc-dispatch-data-raw -- stop double-encoding dispatch-command Data

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-05-29 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/plugin-design.md` - EventBus/DirectBridge, cross-boundary value types
4. `internal/component/plugin/server/dispatch.go`, `pkg/plugin/rpc/types.go`, `internal/component/plugin/types.go`

## Task

The plugin->engine `dispatch-command` RPC carries its payload in `DispatchCommandOutput.Data string`, where the string is *already* a JSON document (`responseToDispatchOutput` does `json.Marshal(resp.Data)` then `string(encoded)`). When the whole `DispatchCommandOutput` is itself marshalled onto the RPC wire, `Data` becomes a JSON string containing JSON: every consumer must decode twice. The same struct overloads `Data` to carry plain (non-JSON) error text on the error path, so a single field means two incompatible encodings.

Goal: make `Data` carry raw JSON exactly once across the IPC boundary, move error text to its own field, and update every consumer so it decodes the payload with a single unmarshal. The user has authorised non-backward-compatible changes to the IPC contract (pre-release).

This was surfaced as a deep-review follow-up during pol-4 (`plan/learned/814-pol-4-explain.md`) and deliberately deferred to a dedicated change validated by a full `make ze-verify`.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] — checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as → Decision: / → Constraint: annotations — these survive compaction. -->
<!-- Track reading progress in session-state.md, not here. -->
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, RPC contract rules
  → Constraint: types crossing the plugin boundary must not carry pointers or ambiguous encodings; a wire field has exactly one meaning.
- [ ] `docs/architecture/api/process-protocol.md` - plugin process RPC framing
  → Constraint: dispatch-command result is framed as a JSON RPC result; `Data` is nested inside that envelope.
- [ ] `ai/rules/json-format.md` - JSON key conventions
  → Constraint: kebab-case keys; `error` and `data` already used elsewhere, reuse the same names.

### RFC Summaries (MUST for protocol work)
- N/A - this is an internal IPC contract, not a wire protocol governed by an RFC.

**Key insights:** (summary of all checkpoint lines — minimal context to resume after compaction)
- `plugin.RawJSON` (`internal/component/plugin/types.go:111`) already exists and marshals raw when the bytes are valid JSON, else as a quoted string. It is the proven primitive; the dispatch hop just fails to use an equivalent on the *wire* type.
- The canonical engine-side `plugin.Response.Data` is a typed `ResponseData` and is NOT double-encoded; only the `DispatchCommandOutput` wire type flattens it to a JSON-string-inside-string.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE writing this spec)
<!-- Same rule: never tick [ ] to [x]. Write → Constraint: annotations instead. -->
- [ ] `pkg/plugin/rpc/types.go` - `DispatchCommandOutput{ Status string; Data string }` (line 270). `Data` doc-comment literally says "JSON-encoded response data".
  → Constraint: `Data string` is the single field used for BOTH a JSON payload (success) and plain error text (error).
- [ ] `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput` (217): success does `string(json.Marshal(resp.Data))`; error does `output.Data = resp.Error` (plain text). `dispatchCommand` (622) returns `(status, data string)`; `handleDispatchCommandDirect` (607) wraps into the struct; runtime path (210) sends it via `conn.SendResult`.
  → Constraint: success and error share one field; the error string is not JSON.
- [ ] `pkg/plugin/sdk/sdk_engine.go` - `Plugin.DispatchCommand` (89) returns `(status, data string, err error)`; slow path unmarshals `DispatchCommandOutput` then returns `out.Data` verbatim to plugin code.
  → Constraint: plugin authors receive `data` as a JSON string and must `json.Unmarshal` it themselves (the double-decode).
- [ ] `internal/component/plugin/server/command.go:741-743`, `system.go:486-488`, `subsystem.go:267-269` - three sites convert `rpcOut.Data`/`out.Data` back into `plugin.RawJSON(...)` on success and treat the same field as `Error:` text on error.
  → Constraint: every engine-side consumer already special-cases "is this JSON or an error string?" by branching on `Status`.
- [ ] `pkg/plugin/rpc/bridge.go:103` - typed DirectBridge fast path returns `(r.Data, r.Err)` (no JSON round-trip), so the fast path does NOT double-encode; only the slow JSON path does.
  → Constraint: the fix must keep the fast/slow paths returning the same Go types so callers are uniform.

**Behavior to preserve:** (unless user explicitly said to change)
- Status values `"done"`/`"error"` (`plugin.StatusDone`/`StatusError`) unchanged.
- DirectBridge fast path semantics (no serialization) unchanged.
- Authorization (`Username: "plugin:" + proc.Name()`) unchanged.
- The typed `plugin.Response.Data` (`ResponseData`) representation is already correct and stays as-is.

**Behavior to change:** (only if user explicitly requested)
- `DispatchCommandOutput.Data` stops being a JSON-string-inside-a-string; it carries raw JSON once.
- Error text moves out of `Data` into a dedicated field.
- `DispatchCommand` SDK return type changes so plugin authors get already-decodable bytes, not a re-encoded string (non-backward-compatible, user-approved).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- A plugin calls `Plugin.DispatchCommand(ctx, command)` (SDK) to invoke a command through the engine's dispatcher.
- Format at entry: plain command string.

### Transformation Path
1. SDK sends `DispatchCommandInput{Command}` over RPC (slow path) or via DirectBridge (fast path).
2. Engine `dispatchCommand` runs the command through the registry, producing a `*plugin.Response` with a typed `ResponseData`.
3. `responseToDispatchOutput` flattens `Response.Data` into `DispatchCommandOutput` (the defect site: `string(json.Marshal(...))`).
4. Engine sends `DispatchCommandOutput` back; the RPC layer marshals the whole struct (second encoding of `Data`).
5. SDK unmarshals the envelope, hands `Data` back to plugin code, which must unmarshal again (third decode).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine <-> Plugin (slow path) | JSON RPC result containing `DispatchCommandOutput` | [ ] |
| Engine <-> Plugin (fast path) | typed DirectBridge channel, no JSON | [ ] |
| Engine dispatcher <-> wire type | `responseToDispatchOutput` flatten/marshal | [ ] |

### Integration Points
- `plugin.RawJSON` (existing) - the encoding primitive to reuse for "embed raw JSON once".
- `directResultResponse` (`dispatch.go`) - direct-path marshal helper that wraps `DispatchCommandOutput`.

### Architectural Verification
- [ ] No bypassed layers (dispatch still flows through the registry)
- [ ] No unintended coupling (no new dependency from `pkg/plugin/rpc` on `internal`)
- [ ] No duplicated functionality (reuse `RawJSON`/`json.RawMessage`, do not invent a new encoder)
- [ ] Zero-copy preserved where applicable (raw bytes passed, not re-stringified)

## Wiring Test (MANDATORY — NOT deferrable)

<!-- BLOCKING: Proves the feature is reachable from its intended entry point. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin calls `DispatchCommand` (slow JSON path) | → | `responseToDispatchOutput` emits raw `Data` + separate `error` | `test/plugin/dispatch-command-single-decode.ci` |
| Plugin calls `DispatchCommand` (success) | → | SDK returns decodable bytes, single unmarshal | `TestDispatchCommandSingleDecode` (dispatch_test.go) |
| Plugin calls `DispatchCommand` (error) | → | error text read from `Error` field, `Data` empty | `TestDispatchCommandErrorField` (dispatch_test.go) |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Plugin dispatches a command returning structured data (slow JSON path) | The `data` field on the wire is raw JSON (not a quoted JSON string); a consumer decodes it with exactly one `json.Unmarshal`. |
| AC-2 | Plugin dispatches a command that errors | Error text is carried in a dedicated `error` field; `data` is empty/omitted; no JSON-parse of the error string is attempted. |
| AC-3 | Plugin dispatches via DirectBridge fast path | Returns the same Go types as the slow path; no serialization round-trip; behavior identical to before. |
| AC-4 | `responseToDispatchOutput` marshal of `resp.Data` fails | Status becomes `error`, error text in the `error` field, `data` omitted. |
| AC-5 | Engine-side consumers (`command.go`, `system.go`, `subsystem.go`) | Read JSON payload from `data` and error text from `error`; no `RawJSON(string-containing-quoted-json)` rewrap remains. |
| AC-6 | External surfaces audit (CLI/mux, web, MCP, gRPC) | Documented finding: confirm whether any external command-result path uses the same `Data string` double-encode; if yes, list each and fix consistently; if no, record the negative result with evidence. |
| AC-7 | Full verification | `make ze-verify` passes (lint + all ze tests, every consumer compiles and behaves). |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchCommandSingleDecode` | `internal/component/plugin/server/dispatch_test.go` | success payload is raw JSON decodable in one pass | |
| `TestDispatchCommandErrorField` | `internal/component/plugin/server/dispatch_test.go` | error text in `error` field, `data` omitted | |
| `TestResponseToDispatchOutputMarshalFail` | `internal/component/plugin/server/dispatch_test.go` | AC-4 marshal-failure routing | |
| `TestDispatchCommandOutputRoundTrip` | `pkg/plugin/rpc/types_test.go` | marshal/unmarshal of the new struct preserves raw JSON without escaping | |
| `TestSDKDispatchCommandReturnsBytes` | `pkg/plugin/sdk/sdk_test.go` | SDK return type carries decodable bytes, not re-encoded string | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs introduced) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `dispatch-command-single-decode` | `test/plugin/dispatch-command-single-decode.ci` | one plugin dispatches a command to the engine and reads structured fields without a second `json.loads` | |

### Interop Tests (MANDATORY for protocol features)
<!-- Skip with justification for non-protocol features. -->
- N/A - internal IPC contract, no peer daemon involved. Justification: this is a Ze-internal plugin RPC type, not a network wire protocol.

### Future (if deferring any tests)
- None planned. AC-6 may convert external-surface findings into follow-up items in their own specs if the same anti-pattern exists there (deferral requires a named destination spec).

## Files to Modify
<!-- MUST include feature code, not only test files -->
- `pkg/plugin/rpc/types.go` - change `DispatchCommandOutput.Data` to raw-JSON type, add `Error` field.
- `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput` (raw data + error field), `dispatchCommand` return type, `handleDispatchCommandDirect`, `directResultResponse` usage.
- `pkg/plugin/sdk/sdk_engine.go` - `DispatchCommand` return type + slow-path decode.
- `pkg/plugin/sdk/sdk_types.go` - alias unchanged but verify it still compiles against the new struct.
- `internal/component/plugin/server/command.go` (741-743) - read `data`/`error` fields.
- `internal/component/plugin/server/system.go` (486-488) - same.
- `internal/component/plugin/server/subsystem.go` (267-269) - same.
- `pkg/plugin/rpc/bridge.go` - verify fast-path return types stay aligned.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/dispatch-command-single-decode.ci` |
| Pipe completeness | No (no new command output) | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` (dispatch-command result shape: `data` raw JSON + `error`) |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` / `ai/rules/plugin-design.md` if the SDK `DispatchCommand` signature is documented |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (DispatchCommandOutput shape) |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |

## Files to Create
- `test/plugin/dispatch-command-single-decode.ci` - functional test proving single-decode.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-verify` |
| 7-10. Critical review loop | Critical Review Checklist |
| 11. Deliverables review | Deliverables Checklist |
| 12. Security review | Security Review Checklist |
| 13. Re-verify | `make ze-verify` |
| 14. Present summary | Executive Summary |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — change the wire type + add failing tests
   - Tests: `TestDispatchCommandOutputRoundTrip`, `TestDispatchCommandSingleDecode` (initially failing)
   - Files: `pkg/plugin/rpc/types.go`
   - Verify: tests compile and fail because producers/consumers still use the old shape.
2. **Phase: Producer** — update `responseToDispatchOutput`, `dispatchCommand`, direct handler
   - Tests: `TestResponseToDispatchOutputMarshalFail`, `TestDispatchCommandErrorField`
   - Files: `internal/component/plugin/server/dispatch.go`
   - Verify: producer emits raw `data` + separate `error`.
3. **Phase: SDK consumer** — update `DispatchCommand` return type + slow-path decode
   - Tests: `TestSDKDispatchCommandReturnsBytes`
   - Files: `pkg/plugin/sdk/sdk_engine.go`, `pkg/plugin/sdk/sdk_types.go`
   - Verify: plugin authors get decodable bytes once.
4. **Phase: Engine-side consumers** — `command.go`, `system.go`, `subsystem.go`, `bridge.go`
   - Files: those four
   - Verify: error read from `error`, JSON from `data`; no double-rewrap remains; `make ze-lint-changed` clean.
5. **Phase: External-surface audit (AC-6)** — grep CLI/mux, web, MCP, gRPC command-result paths
   - Verify: each surface either confirmed clean (evidence) or fixed consistently; record findings in Implementation Summary.
6. **Functional tests** → `test/plugin/dispatch-command-single-decode.ci`.
7. **Full verification** → `make ze-verify`.
8. **Complete spec** → audit tables + learned summary; two commits (A: code+spec+learned+counter; B: `git rm` spec).

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | success uses `data` only; error uses `error` only; no field overload remains |
| Naming | wire keys `data` / `error` kebab-case per json-format.md |
| Data flow | typed `Response.Data` unchanged; only the wire flatten changed |
| Rule: no-layering | old `string(json.Marshal(...))` flatten fully removed, not wrapped |
| Rule: cross-boundary value types | new field carries bytes/raw JSON, no pointer crosses the boundary |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| `DispatchCommandOutput` carries raw JSON + error field | `grep -n "Data " pkg/plugin/rpc/types.go` shows raw-JSON type |
| No `string(.*json.Marshal` in dispatch producer | `grep -n "json.Marshal" internal/component/plugin/server/dispatch.go` |
| Functional test exists | `ls test/plugin/dispatch-command-single-decode.ci` |
| All consumers updated | `grep -rn "rpcOut.Data\|out.Data" internal/component/plugin/server` reviewed |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | malformed `data` bytes from a plugin must not crash the engine; decode errors handled, not panicked |
| Resource | no unbounded allocation introduced by the new field (size is the existing payload) |
| Error leakage | error text routed to `error` field, not silently dropped or mixed into `data` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Consumer behavior mismatch | Re-read Current Behavior for that consumer |
| Functional test fails | Check AC-1/AC-2; if AC wrong → DESIGN |
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
<!-- LIVE — write IMMEDIATELY when you learn something -->

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add a dedicated `error` field; keep `data` for JSON only | Keep one overloaded field and marshal the error string as JSON | One field, one meaning. An overloaded field is exactly what forced every consumer to branch on `Status`. |
| Carry raw JSON in `data` (raw-message semantics) | Keep `string` and document "decode twice" | Single-decode is the whole point; documenting the foot-gun does not remove it. |
| Change the SDK `DispatchCommand` signature | Keep `string` return for source-compat | User authorised non-backward-compatible IPC change pre-release; aligning the type removes the foot-gun at the source. |

## Known Limitations
- Scope is the plugin<->engine `dispatch-command` RPC. External surfaces are audited (AC-6) and only fixed if they share the anti-pattern; broader response-envelope redesign is out of scope.

## RFC Documentation
- N/A (internal IPC, no RFC).

## Implementation Summary

### What Was Implemented
- [filled during implementation]

### Bugs Found/Fixed
- [filled during implementation]

### Documentation Updates
- [filled during implementation]

### Deviations from Plan
- [filled during implementation]

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
| Payload decoded exactly once across IPC | functional test | `test/plugin/dispatch-command-single-decode.ci` reads fields without a second `json.loads` |
| Error text no longer mixed into a JSON field | unit test | `TestDispatchCommandErrorField` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed / deferred / acknowledged |

### Fixes applied
- [per BLOCKER/ISSUE]

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
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes
- [ ] Feature code integrated
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes

### Quality Gates (SHOULD pass)
- [ ] RFC constraint comments added (N/A)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-ipc-dispatch-data-raw.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-ipc-dispatch-data-raw.md`
