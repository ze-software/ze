# Spec: plugin-engine-decode-mp

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - plugin architecture, SDK engine calls
4. `internal/plugin/mpwire.go` - MPReachWire, MPUnreachWire types
5. `internal/plugin/decode.go` - existing DecodeOpen/DecodeNotification pattern
6. `internal/plugin/server.go` - dispatchPluginRPC, handleCodecRPC
7. `pkg/plugin/sdk/sdk.go` - SDK engine call pattern (DecodeNLRI as reference)
8. `pkg/plugin/rpc/types.go` - RPC type definitions

## Task

Add three new plugin→engine RPCs that let plugins decode BGP wire data without knowing the family in advance:

1. **`ze-plugin-engine:decode-mp-reach`** — decode a full MP_REACH_NLRI attribute value (hex). Engine extracts family, next-hop, and decoded NLRI. Returns structured JSON.

2. **`ze-plugin-engine:decode-mp-unreach`** — decode a full MP_UNREACH_NLRI attribute value (hex). Engine extracts family and decoded withdrawn NLRI. Returns structured JSON.

3. **`ze-plugin-engine:decode-update`** — decode a full UPDATE message body (hex, after 19-byte BGP header). Engine parses attributes, NLRI, and withdrawn routes. Returns the same ze-bgp JSON format used in `deliver-event`.

**Why:** The existing `decode-nlri` RPC requires the caller to already know the family. A plugin that receives raw wire bytes (hex-format subscription, external tool, or cross-protocol bridge) cannot use `decode-nlri` without first parsing the MP_REACH_NLRI header to extract AFI/SAFI — and that parsing code is internal to the engine. These higher-level RPCs close that gap.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - overall plugin/engine architecture
- [ ] `.claude/rules/plugin-design.md` - RPC protocol, SDK patterns
- [ ] `.claude/rules/json-format.md` - ze-bgp JSON envelope, kebab-case keys, UPDATE format

### RFC Summaries
- [ ] `rfc/short/rfc4760.md` - MP_REACH_NLRI / MP_UNREACH_NLRI structure
- [ ] `rfc/short/rfc4271.md` - UPDATE message structure
- [ ] `rfc/short/rfc7911.md` - ADD-PATH (affects NLRI parsing)

**Key insights:**
- `MPReachWire` and `MPUnreachWire` already parse the full attribute (family, next-hop, NLRI bytes)
- `filter.ApplyToUpdate()` already parses full UPDATE into `FilterResult`
- `formatFilterResultJSON()` already formats `FilterResult` as ze-bgp JSON
- `DecodeOpen()` / `DecodeNotification()` in `decode.go` establish the decode-to-struct pattern
- ADD-PATH changes NLRI parsing (4-byte path-id prefix per NLRI) — must be caller-specified since there is no peer session context in a stateless decode RPC

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/mpwire.go` - `MPReachWire` wraps MP_REACH bytes, methods: `AFI()`, `SAFI()`, `Family()`, `NextHop()`, `NLRIBytes()`, `NLRIs(hasAddPath)`; `MPUnreachWire` similar but no next-hop
- [ ] `internal/plugin/decode.go` - `DecodeOpen(body) DecodedOpen`, `DecodeNotification(body) DecodedNotification`, `DecodeRouteRefresh(body) DecodedRouteRefresh` — pattern: parse bytes, return struct, never panic
- [ ] `internal/plugin/filter.go:445` - `ApplyToUpdate(wire, body, nlriFilter) (FilterResult, error)` — parses UPDATE body into attributes + NLRI grouped by family
- [ ] `internal/plugin/text.go:262` - `formatFilterResultJSON()` — formats `FilterResult` as ze-bgp JSON with `"nlri": {"family": [...]}`
- [ ] `internal/plugin/text.go:383` - `formatNLRIJSONValue()` — delegates to `registry.DecodeNLRIByFamily()` for plugin families (VPN, EVPN, FlowSpec)
- [ ] `internal/plugin/server.go:1105-1153` - `handleCodecRPC()` shared helper + `handleDecodeNLRIRPC()`/`handleEncodeNLRIRPC()` (spec 240 pattern to follow)
- [ ] `pkg/plugin/sdk/sdk.go:390` - `DecodeNLRI()` method (reference for new methods)
- [ ] `pkg/plugin/rpc/types.go` - existing RPC input/output types

**Behavior to preserve:**
- All existing RPCs unchanged
- `formatFilterResultJSON()` output format is the canonical ze-bgp JSON — reuse it for decode-update
- MP_REACH/MP_UNREACH parsing via `MPReachWire`/`MPUnreachWire` is zero-copy — wrap, don't copy

**Behavior to change:**
- `dispatchPluginRPC()` gains three new method cases
- SDK gains three new public methods
- `rpc/types.go` gains new input/output types

## Data Flow (MANDATORY)

### Entry Point
- Plugin calls `p.DecodeMPReach(ctx, hex, addPath)` via SDK
- SDK sends `ze-plugin-engine:decode-mp-reach` RPC on Socket A

### Transformation Path — decode-mp-reach
1. SDK marshals `DecodeMPReachInput{Hex, AddPath}` into JSON, sends NUL-framed on Socket A
2. Engine's `dispatchPluginRPC()` matches method name
3. Handler hex-decodes the input, wraps as `MPReachWire(data)`
4. Extracts family via `mpw.Family()`, next-hop via `mpw.NextHop()`
5. Gets raw NLRI bytes via `mpw.NLRIBytes()`, hex-encodes them
6. Calls `registry.DecodeNLRIByFamily(family, nlriHex)` for plugin families
7. For core families (ipv4/ipv6 unicast/multicast): uses `mpw.NLRIs(addPath)` directly
8. Assembles result JSON with family, next-hop, and decoded NLRI
9. Sends result back on Socket A

### Transformation Path — decode-mp-unreach
Same as above but no next-hop, uses `MPUnreachWire`, action is always "del"

### Transformation Path — decode-update
1. SDK marshals `DecodeUpdateInput{Hex, AddPath}` into JSON
2. Engine hex-decodes, creates `AttributesWire` + calls `filter.ApplyToUpdate()`
3. Creates `FilterResult` with all attributes and NLRI grouped by family
4. Calls `formatFilterResultJSON()` (or equivalent) to produce ze-bgp JSON
5. Returns the JSON string

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | NUL-framed JSON RPC on Socket A | [ ] |
| Engine → Registry | Direct function call for plugin family NLRI decode | [ ] |
| Engine → Wire parsers | Direct call to `MPReachWire`, `filter.ApplyToUpdate()` | [ ] |

### Integration Points
- `dispatchPluginRPC()` in `server.go` — add three new switch cases
- `MPReachWire` / `MPUnreachWire` in `mpwire.go` — used by handlers (read-only)
- `filter.ApplyToUpdate()` in `filter.go` — used by decode-update handler
- `formatFilterResultJSON()` in `text.go` — used by decode-update handler (or extract reusable core)
- `registry.DecodeNLRIByFamily()` — used for plugin family NLRI decoding
- `handleCodecRPC()` in `server.go` — shared error-handling helper (reuse from spec 240)

### Architectural Verification
- [ ] No bypassed layers (uses standard RPC dispatch, existing parsers)
- [ ] No unintended coupling (handlers use existing public types)
- [ ] No duplicated functionality (reuses MPReachWire, FilterResult, formatFilterResultJSON)
- [ ] Zero-copy preserved where applicable (MPReachWire is a view, not a copy)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSDKDecodeMPReachEngineCall` | `pkg/plugin/sdk/sdk_test.go` | SDK sends decode-mp-reach RPC, receives structured JSON | |
| `TestSDKDecodeMPUnreachEngineCall` | `pkg/plugin/sdk/sdk_test.go` | SDK sends decode-mp-unreach RPC, receives structured JSON | |
| `TestSDKDecodeUpdateEngineCall` | `pkg/plugin/sdk/sdk_test.go` | SDK sends decode-update RPC, receives ze-bgp JSON | |
| `TestDispatchDecodeMPReach` | `internal/plugin/server_test.go` | Engine parses MP_REACH hex, returns family + next-hop + NLRI | |
| `TestDispatchDecodeMPUnreach` | `internal/plugin/server_test.go` | Engine parses MP_UNREACH hex, returns family + withdrawn NLRI | |
| `TestDispatchDecodeUpdate` | `internal/plugin/server_test.go` | Engine parses full UPDATE hex, returns ze-bgp JSON | |
| `TestDispatchDecodeMPReach_Malformed` | `internal/plugin/server_test.go` | Engine returns error for truncated/invalid hex | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — inputs are hex strings; validation is "valid hex" and "sufficient length for parsing."

### Functional Tests
N/A for this change — RPC plumbing is tested via unit tests. Functional tests would require Python ze_api changes.

### Future
- Functional test with Python plugin exercising decode-mp-reach RPC
- ADD-PATH decode test (requires crafting NLRI with 4-byte path-id prefix)

## Files to Modify
- `pkg/plugin/rpc/types.go` — add input/output types for three new RPCs
- `pkg/plugin/sdk/sdk.go` — add `DecodeMPReach()`, `DecodeMPUnreach()`, `DecodeUpdate()` methods
- `internal/plugin/server.go` — add dispatch cases and handler functions
- `.claude/rules/plugin-design.md` — update SDK Engine Calls table

## Files to Create
- No new files (all changes go into existing files)

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Define RPC types** — input/output types in `rpc/types.go`
   → **Review:** JSON tags kebab-case? Output contains family, next-hop, decoded NLRI?

2. **Write SDK unit tests** — three `TestSDK*EngineCall` tests following DecodeNLRI pattern
   → **Review:** Tests simulate engine response, verify SDK parses result correctly?

3. **Write engine dispatch tests** — four dispatch tests following TestDispatchDecodeNLRI pattern
   → **Review:** Tests use registry Snapshot/Restore? Test malformed input case?

4. **Run tests** — Verify FAIL (paste output)
   → **Review:** Tests fail for the right reason (method not found)?

5. **Add SDK methods** — `DecodeMPReach()`, `DecodeMPUnreach()`, `DecodeUpdate()` in `sdk.go`
   → **Review:** Follow `DecodeNLRI()` pattern exactly?

6. **Add engine handlers** — dispatch cases + handler functions in `server.go`
   → **Review:** Use `handleCodecRPC()` shared helper? Use MPReachWire/MPUnreachWire correctly?

7. **Run tests** — Verify PASS (paste output)
   → **Review:** All tests pass? No flaky behavior?

8. **Update documentation** — `.claude/rules/plugin-design.md` SDK Engine Calls table
   → **Review:** Table consistent with actual implementation?

9. **Verify all** — `make lint && make test && make functional` (paste output)
   → **Review:** Zero lint issues? All tests pass?

10. **Final self-review** — Re-read all code changes for bugs, edge cases, improvements
    → **Review:** Error messages clear? No debug statements? ADD-PATH handled?

## RPC Wire Format

### decode-mp-reach

Request:
```
{"method":"ze-plugin-engine:decode-mp-reach","params":{"hex":"000185040c0100010200020100010200","add-path":false},"id":3}
```

Response:
```
{"result":{"family":"ipv4/unicast","next-hop":"12.1.0.1","nlri":["1.0.1.0/24","1.0.2.0/24"]},"id":3}
```

Error:
```
{"error":"MP_REACH_NLRI too short: 2 bytes","id":3}
```

### decode-mp-unreach

Request:
```
{"method":"ze-plugin-engine:decode-mp-unreach","params":{"hex":"00010118c0a800","add-path":false},"id":4}
```

Response:
```
{"result":{"family":"ipv4/unicast","nlri":["192.168.0.0/24"]},"id":4}
```

### decode-update

Request:
```
{"method":"ze-plugin-engine:decode-update","params":{"hex":"0000001c4001010040020602010000fde80e0700010104c0a80101180a00","add-path":false},"id":5}
```

Response (ze-bgp JSON format, same as deliver-event):
```
{"result":{"json":"{\"update\":{\"attr\":{\"origin\":\"igp\",\"as-path\":[65000]},\"nlri\":{\"ipv4/unicast\":[{\"next-hop\":\"192.168.1.1\",\"action\":\"add\",\"nlri\":[\"10.0.0.0/24\"]}]}}}"},"id":5}
```

## RFC Documentation

### Reference Comments
- RFC 4760 Section 3 — MP_REACH_NLRI attribute structure (AFI + SAFI + NH + NLRI)
- RFC 4760 Section 4 — MP_UNREACH_NLRI attribute structure (AFI + SAFI + Withdrawn)
- RFC 4271 Section 4.3 — UPDATE message structure (Withdrawn + Attrs + NLRI)
- RFC 7911 Section 3 — ADD-PATH: 4-byte path-id prefix per NLRI when negotiated

## Design Decisions

- **ADD-PATH as explicit parameter**: Since these RPCs are stateless (no peer session), the caller must specify whether NLRI includes ADD-PATH path-id. Default is `false`.
- **Hex input is attribute VALUE only**: For decode-mp-reach/unreach, the hex is the attribute value bytes (after the attribute flags/type/length TLV header). This matches what `MPReachWire` expects.
- **UPDATE hex is message body only**: For decode-update, the hex is the UPDATE message body (after the 19-byte BGP marker+length+type header). This matches what `UnpackUpdate` expects.
- **Reuse handleCodecRPC**: Same shared error-handling pattern from spec 240.
- **decode-update returns wrapped JSON string**: The `result.json` field contains the ze-bgp JSON as an escaped string, matching the `decode-nlri` pattern where the engine doesn't interpret the result.

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented

### Bugs Found/Fixed

### Design Insights

### Documentation Updates

### Deviations from Plan

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `ze-plugin-engine:decode-mp-reach` RPC | | | |
| `ze-plugin-engine:decode-mp-unreach` RPC | | | |
| `ze-plugin-engine:decode-update` RPC | | | |
| SDK `p.DecodeMPReach()` method | | | |
| SDK `p.DecodeMPUnreach()` method | | | |
| SDK `p.DecodeUpdate()` method | | | |
| Engine dispatch handlers | | | |
| Documentation update | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestSDKDecodeMPReachEngineCall` | | | |
| `TestSDKDecodeMPUnreachEngineCall` | | | |
| `TestSDKDecodeUpdateEngineCall` | | | |
| `TestDispatchDecodeMPReach` | | | |
| `TestDispatchDecodeMPUnreach` | | | |
| `TestDispatchDecodeUpdate` | | | |
| `TestDispatchDecodeMPReach_Malformed` | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `pkg/plugin/rpc/types.go` | | |
| `pkg/plugin/sdk/sdk.go` | | |
| `pkg/plugin/sdk/sdk_test.go` | | |
| `internal/plugin/server.go` | | |
| `internal/plugin/server_test.go` | | |
| `.claude/rules/plugin-design.md` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### 🏗️ Design (see `rules/design-principles.md`)
- [x] No premature abstraction (reuses existing MPReachWire, FilterResult, formatFilterResultJSON)
- [x] No speculative features (three specific RPCs requested)
- [x] Single responsibility (each handler decodes one thing)
- [x] Explicit behavior (ADD-PATH is explicit parameter, not inferred)
- [x] Minimal coupling (handlers use existing internal types, no new dependencies)
- [x] Next-developer test (follows spec 240 pattern exactly)

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (N/A — string inputs)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior (N/A — unit tests cover RPC plumbing)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] RFC summaries read (all referenced RFCs)
- [ ] RFC references added to code
- [ ] RFC constraint comments added

### Completion (after tests pass - see Completion Checklist)
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit completed
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
