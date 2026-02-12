# Spec: plugin-engine-decode

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture
4. `pkg/plugin/sdk/sdk.go` - SDK implementation
5. `internal/plugin/server.go` - engine RPC dispatch
6. `internal/plugin/registry/registry.go` - decode/encode by family

## Task

Add plugin→engine RPCs for NLRI decode and encode. Currently plugins can only receive decode/encode requests from the engine (via `ze-plugin-callback:decode-nlri`), but cannot ask the engine to decode/encode on their behalf. This blocks use cases like an HTTP API plugin that needs to decode arbitrary NLRI binary data using whatever plugin handles that family.

Two new RPCs on Socket A:
- `ze-plugin-engine:decode-nlri` — plugin sends (family, hex), engine returns JSON
- `ze-plugin-engine:encode-nlri` — plugin sends (family, args), engine returns hex

Two new SDK methods:
- `p.DecodeNLRI(ctx, family, hex) (string, error)`
- `p.EncodeNLRI(ctx, family, args) (string, error)`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall plugin architecture
- [ ] `.claude/rules/plugin-design.md` - RPC protocol, SDK patterns, registration

### RFC Summaries
N/A — no protocol-level changes.

**Key insights:**
- Engine-side `registry.DecodeNLRIByFamily()` and `registry.EncodeNLRIByFamily()` already exist
- RPC input types `DecodeNLRIInput` and `EncodeNLRIInput` already exist in `pkg/plugin/rpc/types.go`
- SDK pattern: `callEngineWithResult()` sends RPC on Socket A and parses response
- Engine dispatch: `dispatchPluginRPC()` in `server.go` switches on method name

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` - defines RPC input/output types; has `DecodeNLRIInput` and `EncodeNLRIInput` but no output types for the plugin→engine direction
- [ ] `pkg/plugin/sdk/sdk.go` - SDK with `callEngine()`, `callEngineWithResult()`, `UpdateRoute()`, `SubscribeEvents()`, `UnsubscribeEvents()`; no decode/encode engine calls
- [ ] `internal/plugin/server.go:937-954` - `dispatchPluginRPC()` handles update-route, subscribe-events, unsubscribe-events; no decode/encode
- [ ] `internal/plugin/registry/registry.go:220-240` - `DecodeNLRIByFamily()` and `EncodeNLRIByFamily()` exist, use in-process plugin decoders

**Behavior to preserve:**
- Existing engine→plugin callbacks (`ze-plugin-callback:decode-nlri`, `ze-plugin-callback:encode-nlri`) unchanged
- Existing SDK callback handlers (`OnDecodeNLRI`, `OnEncodeNLRI`) unchanged
- All existing plugin→engine RPCs unchanged
- `registry.DecodeNLRIByFamily()` and `registry.EncodeNLRIByFamily()` signatures unchanged

**Behavior to change:**
- `dispatchPluginRPC()` gains two new method cases
- SDK gains two new public methods
- `rpc/types.go` gains two new output types

## Data Flow (MANDATORY)

### Entry Point
- Plugin calls `p.DecodeNLRI(ctx, "ipv4/flow", "0701180A0000")` via SDK
- SDK sends `ze-plugin-engine:decode-nlri` RPC on Socket A

### Transformation Path
1. SDK marshals `DecodeNLRIInput{Family, Hex}` into JSON, sends NUL-framed on Socket A
2. Engine's `handleSingleProcessCommandsRPC()` goroutine reads request from Socket A
3. `dispatchPluginRPC()` matches `ze-plugin-engine:decode-nlri`
4. Handler unmarshals params, calls `registry.DecodeNLRIByFamily(family, hex)`
5. Registry finds plugin with matching family, calls its `InProcessNLRIDecoder`
6. Handler sends `DecodeNLRIOutput{JSON}` result back on Socket A
7. SDK receives response, unmarshals, returns JSON string to caller

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | NUL-framed JSON RPC on Socket A | [ ] |
| Engine → Registry | Direct function call (in-process) | [ ] |

### Integration Points
- `dispatchPluginRPC()` in `server.go:939` — add new switch cases
- `registry.DecodeNLRIByFamily()` in `registry.go:220` — called by handler
- `registry.EncodeNLRIByFamily()` in `registry.go:233` — called by handler
- `callEngineWithResult()` in `sdk.go:353` — used by new SDK methods

### Architectural Verification
- [ ] No bypassed layers (uses standard RPC dispatch)
- [ ] No unintended coupling (uses existing registry, no new imports)
- [ ] No duplicated functionality (reuses existing registry functions)
- [ ] Zero-copy preserved where applicable (N/A — string data, not wire bytes)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSDKDecodeNLRIEngineCall` | `pkg/plugin/sdk/sdk_test.go` | SDK sends decode-nlri RPC, receives JSON result | |
| `TestSDKEncodeNLRIEngineCall` | `pkg/plugin/sdk/sdk_test.go` | SDK sends encode-nlri RPC, receives hex result | |
| `TestDispatchDecodeNLRI` | `internal/plugin/server_test.go` | Engine handler routes to registry decoder | |
| `TestDispatchEncodeNLRI` | `internal/plugin/server_test.go` | Engine handler routes to registry encoder | |
| `TestDispatchDecodeNLRI_NoDecoder` | `internal/plugin/server_test.go` | Error when no decoder registered for family | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no numeric inputs; family and hex are strings validated by the registry.

### Functional Tests
N/A for this change — the RPC plumbing is tested via unit tests. A functional test would require Python ze_api changes (separate scope).

### Future
- Functional test with Python plugin exercising decode RPC (requires ze_api.py update)
- Cross-external-plugin decode (when requesting plugin's family is handled by another external plugin)

## Files to Modify
- `pkg/plugin/rpc/types.go` — add `DecodeNLRIOutput` and `EncodeNLRIOutput`
- `pkg/plugin/sdk/sdk.go` — add `DecodeNLRI()` and `EncodeNLRI()` methods + type aliases
- `internal/plugin/server.go` — add dispatch cases and handlers in `dispatchPluginRPC()`

## Files to Create
- No new files (changes go into existing files)

## Implementation Steps

1. **Write SDK unit tests** — `TestSDKDecodeNLRIEngineCall` and `TestSDKEncodeNLRIEngineCall`
   → **Review:** Do tests follow the `TestSDKUpdateRoute` pattern?

2. **Write engine dispatch tests** — `TestDispatchDecodeNLRI`, `TestDispatchEncodeNLRI`, `TestDispatchDecodeNLRI_NoDecoder`
   → **Review:** Do tests register a test plugin in the registry?

3. **Run tests** — Verify FAIL (paste output)
   → **Review:** Do tests fail for the right reason (method not found)?

4. **Add output types** — `DecodeNLRIOutput` and `EncodeNLRIOutput` in `rpc/types.go`
   → **Review:** JSON tags use kebab-case?

5. **Add SDK methods** — `DecodeNLRI()` and `EncodeNLRI()` in `sdk.go`
   → **Review:** Follow `UpdateRoute()` pattern exactly?

6. **Add engine handlers** — dispatch cases and handler functions in `server.go`
   → **Review:** Call `registry.DecodeNLRIByFamily()` / `registry.EncodeNLRIByFamily()`?

7. **Run tests** — Verify PASS (paste output)
   → **Review:** All tests pass? No flaky behavior?

8. **Update documentation** — `.claude/rules/plugin-design.md` SDK Engine Calls table
   → **Review:** Table consistent with actual implementation?

9. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

## Implementation Summary

### What Was Implemented
- Two new RPC methods: `ze-plugin-engine:decode-nlri` and `ze-plugin-engine:encode-nlri`
- SDK methods `DecodeNLRI()` and `EncodeNLRI()` following the `UpdateRoute()` pattern
- Engine dispatch cases + handler functions in `dispatchPluginRPC()`
- Shared `handleCodecRPC()` helper to avoid code duplication (linter `dupl` check)
- Output types `DecodeNLRIOutput` and `EncodeNLRIOutput` in `pkg/plugin/rpc/types.go`
- Registry `Snapshot()`/`Restore()` for safe test isolation

### Bugs Found/Fixed
- **Data race in tests**: Tests using `registry.Reset()` + `t.Parallel()` caused concurrent map writes. Fixed by removing `t.Parallel()` (tests mutate global state).
- **Registry pollution**: `t.Cleanup(registry.Reset)` left empty registry for subsequent tests. Fixed by adding `Snapshot()`/`Restore()` to registry and using save/restore pattern in cleanup.

### Design Insights
- `handleCodecRPC()` extracts the shared unmarshal→call→respond pattern, preventing dupl lint violations while keeping each handler's specific logic in a focused closure.
- Registry `Snapshot()`/`Restore()` is the correct test pattern for any test that needs to temporarily replace global compile-time registrations.

### Documentation Updates
- `.claude/rules/plugin-design.md` — added `DecodeNLRI` and `EncodeNLRI` to SDK Engine Calls table

### Deviations from Plan
- Added `handleCodecRPC()` shared helper (not in original plan) to satisfy `dupl` linter
- Added `registry.Snapshot()`/`registry.Restore()` (not in original plan) to fix test isolation

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `ze-plugin-engine:decode-nlri` RPC | ✅ Done | `server.go:951-953` dispatch, `server.go:1126-1137` handler | |
| `ze-plugin-engine:encode-nlri` RPC | ✅ Done | `server.go:954-956` dispatch, `server.go:1142-1153` handler | |
| SDK `p.DecodeNLRI()` method | ✅ Done | `sdk.go:390` | |
| SDK `p.EncodeNLRI()` method | ✅ Done | `sdk.go:406` | |
| Engine dispatch handlers | ✅ Done | `server.go:1105-1153` | Shared `handleCodecRPC` + two thin wrappers |
| Documentation update | ✅ Done | `.claude/rules/plugin-design.md:229-230` | SDK Engine Calls table |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSDKDecodeNLRIEngineCall` | ✅ Done | `sdk_test.go:1398` | |
| `TestSDKEncodeNLRIEngineCall` | ✅ Done | `sdk_test.go:1471` | |
| `TestDispatchDecodeNLRI` | ✅ Done | `server_test.go:1068` | |
| `TestDispatchEncodeNLRI` | ✅ Done | `server_test.go:1139` | |
| `TestDispatchDecodeNLRI_NoDecoder` | ✅ Done | `server_test.go:1207` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/rpc/types.go` | ✅ Modified | Added `DecodeNLRIOutput`, `EncodeNLRIOutput` |
| `pkg/plugin/sdk/sdk.go` | ✅ Modified | Added `DecodeNLRI()`, `EncodeNLRI()`, type aliases |
| `pkg/plugin/sdk/sdk_test.go` | ✅ Modified | Added 2 SDK engine-call tests |
| `internal/plugin/server.go` | ✅ Modified | Added dispatch cases + `handleCodecRPC` + 2 handlers |
| `internal/plugin/server_test.go` | ✅ Modified | Added 3 dispatch tests |
| `.claude/rules/plugin-design.md` | ✅ Modified | Added to SDK Engine Calls table |
| `internal/plugin/registry/registry.go` | 🔄 Changed | Added `Snapshot()`/`Restore()` (not in plan, needed for test isolation) |

### Audit Summary
- **Total items:** 18
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (registry.go — unplanned but necessary for test isolation)

## Checklist

### 🏗️ Design
- [x] No premature abstraction (reuses existing registry functions)
- [x] No speculative features (decode/encode are the requested RPCs)
- [x] Single responsibility (each handler does one thing)
- [x] Explicit behavior (standard RPC dispatch, no magic)
- [x] Minimal coupling (only imports registry package)
- [x] Next-developer test (follows existing UpdateRoute pattern)

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS (all 5 pass with -race)
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior (N/A — unit tests cover RPC plumbing)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes (240/240)

### Documentation (during implementation)
- [x] Required docs read
- [x] RFC references added to code (N/A — no protocol changes)

### Completion
- [x] Architecture docs updated with learnings (plugin-design.md SDK Engine Calls)
- [x] Implementation Audit completed
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
