# Spec: codec-callback-passthrough -- carry struct through OnDecodeNLRI, marshal once

| Field | Value |
|-------|-------|
| Status | complete |
| Depends | - |
| Phase | 4/4 |
| Updated | 2026-05-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/plugin/sdk/sdk_callbacks.go:135-172` - OnDecodeNLRI, OnDecodeCapability, OnEncodeNLRI
4. `internal/component/plugin/registry/registry.go:76-79,617-624` - InProcessNLRIDecoder type and DecodeNLRIByFamily
5. `pkg/plugin/rpc/types.go:168-170` - DecodeNLRIOutput struct

## Task

The NLRI decode callbacks follow the same double-marshal pattern that `spec-dispatch-response-passthrough` fixed for `OnExecuteCommand`: each `DecodeNLRIHex` function builds a Go data structure, `json.Marshal`s it to a string, returns the string. The SDK wrapper then wraps that string in `{"json":"escaped_string"}` and marshals the whole struct, producing JSON-string-inside-JSON.

There are two paths that share the same `DecodeNLRIHex` function signature:

1. **In-process fast path** (`InProcessNLRIDecoder`): engine calls the function directly via the plugin registry. The result goes into `DecodeNLRIOutput{JSON: string}` which is marshaled for the RPC response, or appended raw to a JSON buffer in `format/text_json.go:241`.

2. **SDK RPC path** (`OnDecodeNLRI`): only used by external plugins (vpn, evpn, flowspec, ls). The SDK wraps in `{"json": jsonResult}`, double-encoding.

Goal: change `DecodeNLRIHex` functions to return `(any, error)` instead of `(string, error)`. The registry and SDK marshal the data once. Eliminate ~10 `json.Marshal` calls (plus 1 `strings.Builder` JSON path in labeled) across 9 NLRI plugins.

**Not in scope:**
- `OnEncodeNLRI` / `InProcessNLRIEncoder`: returns hex string, not JSON. No double-encoding.
- `OnDecodeCapability`: zero registrations. Update signature for consistency but no handler changes.
- Capability decode CLI tools (softver, gr, hostname, llnh, route_refresh, role): `RunDecodeMode`/`RunCLIDecode` functions write to stdout directly, not through SDK callbacks.
- `json.go` append-based builders in NLRI packages: these are the fast-path NLRI formatters (`nlri.JSONAppender`) used by `format/text_json.go:230`. They bypass DecodeNLRIHex entirely. Not affected.

**Relationship to spec-dispatch-response-passthrough:** that spec changed `OnExecuteCommand` (engine-to-plugin direction). This spec changes `OnDecodeNLRI` (plugin-to-engine codec path). Different callbacks, different RPC types. Both eliminate double-encoding by the same technique (handler returns `any`, SDK/registry marshals once).

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, plugin SDK contract
  → Constraint: SDK callback signatures are the plugin API surface.
- [ ] `ai/rules/json-format.md` - JSON key conventions
  → Constraint: kebab-case keys. NLRI decoders already use correct casing.

### RFC Summaries (MUST for protocol work)
- N/A: internal SDK change, not a wire protocol.

**Key insights:**
- `DecodeNLRIHex` is defined once per NLRI family (vpn, evpn, flowspec, rtc, mup, mvpn, vpls, labeled). ls uses an inline lambda instead of a named function. All share the in-process registry path except ls, which has no `InProcessNLRIDecoder` and is RPC-only for decode.
- The in-process path in `format/text_json.go:241` does `append(buf, decoded...)` treating the result as raw JSON bytes. Changing the return type to `json.RawMessage` (which is `[]byte`) preserves this behavior.
- 4 NLRI plugins register `OnDecodeNLRI` for the SDK RPC path (vpn, evpn, flowspec, ls). The other 5 (rtc, mup, mvpn, vpls, labeled) are in-process only.
- `OnDecodeCapability` has zero registrations but should be updated for API consistency.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/sdk/sdk_callbacks.go:135-172` - `OnDecodeNLRI` takes `func(family, hex string) (string, error)`, wraps result in `{"json": jsonResult}`.
  → Constraint: handler returns pre-marshaled JSON string. SDK wraps in struct, marshals again.
- [ ] `pkg/plugin/rpc/types.go:168-170` - `DecodeNLRIOutput{JSON string}`. The `JSON` field carries the already-marshaled string.
  → Constraint: must become `json.RawMessage` to hold raw JSON bytes without double-encoding.
- [ ] `internal/component/plugin/registry/registry.go:76-79` - `InProcessNLRIDecoder func(family, hex string) (string, error)`. Same signature as SDK callback.
  → Constraint: shared function pointer type. Changes to `(any, error)`. `DecodeNLRIByFamily` marshals the `any` to `json.RawMessage`.
- [ ] `internal/component/plugin/registry/registry.go:617-624` - `DecodeNLRIByFamily`: calls `InProcessNLRIDecoder`, returns `(string, error)`.
  → Constraint: callers at `codec.go:40` and `format/text_json.go:239` consume the string result.
- [ ] `internal/component/bgp/server/codec.go:35-45` - `handleDecodeNLRI`: wraps result in `DecodeNLRIOutput{JSON: result}`.
  → Constraint: if result becomes `json.RawMessage`, assign directly to the `JSON` field.
- [ ] `internal/component/bgp/server/codec.go:188-196` - `handleDecodeMPReach`: calls `DecodeNLRIByFamily`, wraps result in `json.RawMessage(result)`.
  → Constraint: if `DecodeNLRIByFamily` returns `json.RawMessage`, the conversion becomes identity; simplify to `return result, nil`.
- [ ] `internal/component/bgp/format/text_json.go:239-242` - `appendNLRIJSONValue` fallback: `append(buf, decoded...)`. Treats result as raw JSON bytes.
  → Constraint: works with both `string` and `[]byte` via `append`. No code change expected.
- [ ] `internal/component/bgp/plugins/nlri/vpn/vpn.go:57` - `p.OnDecodeNLRI(DecodeNLRIHex)`. One of 3 SDK registrations.
- [ ] `internal/component/bgp/plugins/nlri/vpn/vpn.go:80-98` - `DecodeNLRIHex`: builds `[]map[string]any`, `json.Marshal`, returns string.
  → Constraint: same pattern across 8 of 9 NLRI plugins. Each builds a map/struct, marshals, returns string. Exception: labeled builds JSON via `strings.Builder`, no `json.Marshal`.

**Behavior to preserve:**
- All NLRI JSON output produces identical content (same keys, same values, same structure).
- `format/text_json.go` fallback path works unchanged.
- CLI decode tools (`RunDecodeMode`, `RunCLIDecode`) unaffected.
- `nlri.JSONAppender` fast path unaffected.

**Behavior to change:**
- `OnDecodeNLRI` handler signature: `(string, error)` becomes `(any, error)`.
- `InProcessNLRIDecoder` type: `(string, error)` becomes `(any, error)`. Handlers return Go data structures.
- `DecodeNLRIByFamily` return type: `(string, error)` becomes `(json.RawMessage, error)`. This is the single marshal point: it calls `json.Marshal` on the `any` value from `InProcessNLRIDecoder`.
- `DecodeNLRIOutput.JSON`: `string` becomes `json.RawMessage`.
- SDK wrapper: marshals `any` data to `json.RawMessage` before embedding in output.
- `OnDecodeCapability` handler signature: `(string, error)` becomes `(any, error)` for consistency.
- ~10 `DecodeNLRIHex` handler functions: drop `json.Marshal`, return Go data structures directly. labeled drops `strings.Builder` JSON construction, returns a struct/map instead.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Engine needs to decode NLRI for display (show commands, event formatting, LG).

### Transformation Path (current)
1. `format/text_json.go` calls `registry.DecodeNLRIByFamily(family, hex)`.
2. Registry calls `DecodeNLRIHex(family, hex)` in the NLRI plugin.
3. Plugin builds `map[string]any`, `json.Marshal(result)`, returns `string(data)`. **Marshal #1.**
4. `codec.go` wraps in `DecodeNLRIOutput{JSON: string}`, returns as `any`.
5. Caller marshals the output struct. **Marshal #2.** JSON-string-inside-JSON.

### Transformation Path (proposed)
1. Same entry point.
2. Same registry call.
3. Plugin builds `map[string]any`, returns it directly as `any`. **Zero marshal.**
4. Registry marshals to `json.RawMessage`. **Marshal #1 (only one).**
5. `codec.go` wraps in `DecodeNLRIOutput{JSON: json.RawMessage}`. Embedded as raw JSON.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin DecodeNLRIHex -> registry | `any` value (Go map/struct) | [ ] |
| Registry -> codec.go | `json.RawMessage` (pre-marshaled) | [ ] |
| codec.go -> RPC wire | `DecodeNLRIOutput` with `json.RawMessage` JSON | [ ] |
| format/text_json.go -> JSON buffer | `append(buf, rawMessage...)` | [ ] |

### Integration Points
- `DecodeNLRIByFamily` (`registry.go:617`) - in-process entry point for NLRI decode
- `handleDecodeNLRI` (`codec.go:35`) - RPC handler wrapping registry result
- `handleDecodeMPReach` (`codec.go:175`) - MP_REACH decoder, calls `DecodeNLRIByFamily` at line 192
- `appendNLRIJSONValue` (`text_json.go:227`) - JSON formatter consuming decoded bytes
- `OnDecodeNLRI` (`sdk_callbacks.go:137`) - SDK callback for external plugin RPC path
- `DecodeNLRI` (`sdk_engine.go:160`) - SDK consumer of decode result (zero non-test callers currently)

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `registry.DecodeNLRIByFamily` call | -> | `InProcessNLRIDecoder` returns `json.RawMessage` | `TestDecodeNLRIByFamilyRawJSON` |
| SDK `OnDecodeNLRI` callback invoked via RPC | -> | SDK marshals `any` to `json.RawMessage` | `TestOnDecodeNLRIAny` |
| `handleDecodeNLRI` RPC handler | -> | `DecodeNLRIOutput.JSON` is `json.RawMessage` | `TestDecodeNLRIOutputJSON` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DecodeNLRIHex returns map for vpn NLRI | Registry marshals once, no double-encoding in DecodeNLRIOutput |
| AC-2 | DecodeNLRIHex returns struct for evpn NLRI | Same: marshaled once |
| AC-3 | format/text_json.go fallback path with VPN NLRI | `append(buf, decoded...)` produces valid JSON |
| AC-4 | SDK RPC path (OnDecodeNLRI) for external plugin | Single marshal, no JSON-string-inside-JSON |
| AC-5 | All 9 NLRI plugins updated | Every DecodeNLRIHex (and ls inline lambda) returns Go data, no json.Marshal/strings.Builder for decode results |
| AC-6 | OnDecodeCapability signature updated | Signature is `(any, error)`, wrapper marshals once |
| AC-7 | OnEncodeNLRI unchanged | Still returns `(string, error)` for hex |
| AC-8 | CLI decode tools unchanged | `RunDecodeMode`/`RunCLIDecode` still work, write to stdout |
| AC-9 | Full verification | `make ze-verify` passes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDecodeNLRIByFamilyRawJSON` | `internal/component/plugin/registry/registry_test.go` | InProcessNLRIDecoder returns json.RawMessage, no double-encoding | |
| `TestOnDecodeNLRIAny` | `pkg/plugin/sdk/sdk_callbacks_test.go` | SDK marshals once, result is raw JSON not quoted string | |
| `TestDecodeNLRIOutputJSON` | `internal/component/bgp/server/codec_test.go` | handleDecodeNLRI produces valid JSON in output | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs introduced) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing decode tests | `test/decode/` + `test/plugin/` | NLRI decode produces correct JSON via daemon | |

### Interop Tests
- N/A: internal SDK change, no wire protocol.

## Files to Modify

### SDK (1 file)
- `pkg/plugin/sdk/sdk_callbacks.go` - `OnDecodeNLRI` signature: `(string, error)` to `(any, error)`. Wrapper marshals `any` to `json.RawMessage`. `OnDecodeCapability` same change.

### RPC types (1 file)
- `pkg/plugin/rpc/types.go` - `DecodeNLRIOutput.JSON`: `string` to `json.RawMessage`.

### Registry (1 file)
- `internal/component/plugin/registry/registry.go` - `InProcessNLRIDecoder` type and `DecodeNLRIByFamily` return type.

### Engine codec (1 file)
- `internal/component/bgp/server/codec.go` - `handleDecodeNLRI` (line 35) adapts for `json.RawMessage`. `handleDecodeMPReach` (line 192) simplifies: `json.RawMessage(result)` becomes identity.

### Engine formatter (1 file, verify only)
- `internal/component/bgp/format/text_json.go` - verify `append(buf, decoded...)` works with `json.RawMessage`.

### NLRI plugins (9 plugins, 10 files)
- `internal/component/bgp/plugins/nlri/vpn/vpn.go` - DecodeNLRIHex: drop json.Marshal, return data
- `internal/component/bgp/plugins/nlri/evpn/plugin.go` - same
- `internal/component/bgp/plugins/nlri/flowspec/plugin.go` - same (plugin_protocol.go:150 is RunDecode path, out of scope)
- `internal/component/bgp/plugins/nlri/ls/plugin.go` - inline lambda in OnDecodeNLRI (no named DecodeNLRIHex, no InProcessNLRIDecoder): drop json.Marshal, return data
- `internal/component/bgp/plugins/nlri/rtc/rtc.go` - DecodeNLRIHex: drop json.Marshal, return data
- `internal/component/bgp/plugins/nlri/mup/mup.go` - same
- `internal/component/bgp/plugins/nlri/mvpn/mvpn.go` - same
- `internal/component/bgp/plugins/nlri/vpls/vpls.go` - same
- `internal/component/bgp/plugins/nlri/labeled/encode.go` - DecodeNLRIHex: drop strings.Builder JSON construction, return struct/map

### SDK consumer (1 file)
- `pkg/plugin/sdk/sdk_engine.go` - `DecodeNLRI` (line 160): unmarshals `DecodeNLRIOutput`, returns `out.JSON`. If `JSON` becomes `json.RawMessage`, return type changes from `(string, error)` to `(json.RawMessage, error)`. Zero non-test callers currently, so change is safe.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | Existing decode tests cover the path |
| Pipe completeness | No | - |
| Env var registration | No | - |
| Doctor check for runtime dependencies | No | - |
| Prometheus counters/metrics | No | - |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | - |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` - note OnDecodeNLRI data return is now `any` |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | No | - |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or runtime inventory changed? | No | - |
| 16 | Any changed source file is referenced by existing doc source anchors? | [ ] | Grep `docs/` for `source: sdk_callbacks.go` after implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Files to Create
- None (tests go in existing test files).

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

1. **Phase: Wiring** -- change registry type, SDK signature, RPC type
   - Files: `registry.go`, `sdk_callbacks.go`, `types.go`, `codec.go`
   - Tests: `TestDecodeNLRIByFamilyRawJSON`, `TestOnDecodeNLRIAny`
   - Verify: in-process path compiles, SDK accepts `any`, `text_json.go` works

2. **Phase: NLRI plugins** -- update all 9 decode handlers
   - Files: vpn, evpn, flowspec, ls (inline lambda), rtc, mup, mvpn, vpls, labeled
   - Verify: each plugin compiles, decode tests pass

3. **Phase: SDK engine consumer** -- update `DecodeNLRI` in sdk_engine.go
   - Files: `sdk_engine.go`
   - Verify: SDK test passes

4. **Phase: Full verification** -- `make ze-verify`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | NLRI JSON output identical before and after |
| No-layering | No handler still calls `json.Marshal` for decode results |
| Import hygiene | No new cross-package imports introduced |
| text_json.go | Fallback path `append(buf, decoded...)` works with `json.RawMessage` |
| codec.go:192 | `handleDecodeMPReach` uses updated `DecodeNLRIByFamily` return type |
| labeled | `DecodeNLRIHex` in labeled/encode.go returns data structure, not JSON string |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| `InProcessNLRIDecoder` uses `json.RawMessage` | `grep 'InProcessNLRIDecoder' registry.go` |
| No `json.Marshal` in NLRI DecodeNLRIHex | `grep -rn 'json.Marshal' nlri/*/` shows 0 in DecodeNLRIHex functions and ls lambda |
| `DecodeNLRIOutput.JSON` is `json.RawMessage` | `grep 'JSON.*json.RawMessage' types.go` |
| `OnDecodeCapability` updated | `grep 'OnDecodeCapability.*any' sdk_callbacks.go` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | NLRI hex data from handler is trusted internal code. Marshal errors caught by registry. |
| Type confusion | `any` can be any Go value. `json.Marshal` handles all serializable types. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior, RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural, DESIGN phase |
| Functional test fails | Check AC; if AC wrong, DESIGN; if AC correct, IMPLEMENT |
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

## Known Limitations

- Does not change `OnEncodeNLRI` (returns hex, not JSON).
- Does not change capability CLI decode tools (stdout writers, not SDK callbacks).
- `MarshalIndent` paths in NLRI plugins (used by `RunCLIDecode` for pretty output) not affected since they write to stdout.
- `json.go` append-based builders already exist for most NLRI types; this spec does not migrate DecodeNLRIHex to use them. That would be a further optimization (avoid building intermediate maps entirely).

## Design Insights

### Core Insight

Same pattern as `spec-dispatch-response-passthrough`: move marshal responsibility from N handler functions to a single marshal point (the registry/SDK wrapper), eliminating N-1 redundant marshals.

### Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `InProcessNLRIDecoder` returns `any`, `DecodeNLRIByFamily` returns `json.RawMessage` | a) Handler returns `json.RawMessage` (marshal stays in N handlers). b) Return `any` all the way and let each caller marshal. | Split gives single marshal point in `DecodeNLRIByFamily` while keeping handlers marshal-free. `format/text_json.go` already treats the result as raw bytes via `append`. |
| Update `OnDecodeCapability` for consistency | Leave it as `(string, error)` since unused | One API style across all decode callbacks. Zero cost since no registrations exist. |
| Keep `OnEncodeNLRI` as `(string, error)` | Change to `(any, error)` | Encode returns hex, not JSON. String is the correct type for hex. No double-encoding to fix. |

## RFC Documentation

N/A: no protocol work.

## Implementation Summary

### What Was Implemented
- `InProcessNLRIDecoder` returns `(any, error)`, `DecodeNLRIByFamily` returns `(json.RawMessage, error)`
- `DecodeNLRIOutput.JSON` changed from `string` to `json.RawMessage`
- `OnDecodeNLRI` and `OnDecodeCapability` SDK callbacks accept `(any, error)` handlers
- `DecodeNLRI` in sdk_engine.go returns `(json.RawMessage, error)`
- All 9 NLRI plugins updated: vpn, evpn, flowspec, ls, rtc, mup, mvpn, vpls, labeled
- `handleDecodeNLRI` and `decodeMPNLRIs` in codec.go simplified
- `RunCLIDecode`/`RunDecodeMode` callers marshal at output boundary

### Bugs Found/Fixed
- None

### Documentation Updates
- `docs/architecture/api/process-protocol.md` - wire format updated from `{"json":"..."}` to `{"json":<raw JSON>}`
- `docs/plugin-development/handlers.md` - SDK handler examples updated to `(any, error)` signatures

### Deviations from Plan
- `RunCLIDecode`/`RunDecodeMode` internal callers of `DecodeNLRIHex` needed `json.Marshal` added (4 plugins + labeled). Spec mentioned them as out of scope for changes, but the shared function signature made the change unavoidable.
- `update_text_test.go` test helpers and labeled decode tests needed updating (not listed in spec's Files to Modify).

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

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Handlers return struct, no marshal | grep | `grep -c 'json.Marshal' nlri/*/vpn.go` = 0 for decode path |
| Registry marshals once | Unit test | `TestDecodeNLRIByFamilyRawJSON` |
| Output identical | Functional test | Existing decode `.ci` tests |
| All 9 NLRI plugins updated | grep | `grep -rn 'string, error.*DecodeNLRI' nlri/` = 0, ls inline lambda returns `any` |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- [To be filled]

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

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean (Review Gate section filled)
- [ ] `make ze-test` passes
- [ ] Feature code integrated (`internal/*`, `pkg/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes

### Quality Gates (SHOULD pass -- defer with user approval)
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N/A with justification)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-codec-callback-passthrough.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-codec-callback-passthrough.md`
