# Spec: dispatch-response-passthrough -- carry struct through OnExecuteCommand, marshal once

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 8/8 |
| Updated | 2026-05-31 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `pkg/plugin/sdk/sdk_callbacks.go:175-191` - OnExecuteCommand definition
4. `pkg/plugin/rpc/types.go:226-230` - ExecuteCommandOutput struct
5. `internal/component/bgp/plugins/rib/rib_commands.go:86` - CommandHandler type

## Task

Every plugin that handles commands via `OnExecuteCommand` follows the same pattern: build a struct or `map[string]any`, `json.Marshal` it to string, return the string. The SDK then wraps that string in `ExecuteCommandOutput{Data: string}` and marshals the whole struct for the RPC wire, double-encoding the data (JSON-string-inside-JSON).

There are 22 `OnExecuteCommand` registrations across the codebase (19 production + 3 test plugins), calling ~65 handler functions that each pre-marshal their result. Each marshal is wasted work because the SDK is going to marshal the entire output anyway.

Goal: change `OnExecuteCommand` so handlers return `(string, any, error)` instead of `(string, string, error)`. The SDK marshals the `any` data directly into the `ExecuteCommandOutput`, producing one marshal instead of two. Each handler drops its own `json.Marshal` call and returns its struct/map/slice directly.

**Relationship to spec-ipc-dispatch-data-raw:** that spec changes `DispatchCommandOutput.Data` (plugin-to-engine direction) from `string` to `json.RawMessage`. This spec changes `ExecuteCommandOutput.Data` (engine-to-plugin direction) and the `OnExecuteCommand` handler signature. The two specs touch different structs and different data flows, so they are independent and can land in any order. Both eliminate double-encoding on their respective RPC path.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugin-design.md` - cross-boundary value types, plugin SDK contract
  → Constraint: SDK callback signatures are the plugin API surface. Changing them is a breaking change for external plugins, but all current callers are internal (pre-release).
- [ ] `ai/rules/json-format.md` - JSON key conventions
  → Constraint: kebab-case keys. Handlers already use correct casing in their maps/struct tags.

### RFC Summaries (MUST for protocol work)
- N/A - internal SDK change, not a wire protocol.

**Key insights:**
- `OnExecuteCommand` is defined once in `pkg/plugin/sdk/sdk_callbacks.go:177`. Changing the callback signature from `(string, string, error)` to `(string, any, error)` is a single-site SDK change.
- The SDK wrapper at line 189 does `json.Marshal(&rpc.ExecuteCommandOutput{Status: status, Data: data})`. If `data` is `any`, the wrapper needs to marshal `data` first into the `Data` field (currently `string`, will need to become `json.RawMessage` or the marshal must stringify). The cleanest approach: marshal `data` to `json.RawMessage` inside the wrapper, assign to `Data`. This means `ExecuteCommandOutput.Data` becomes `json.RawMessage`. (Note: spec-ipc-dispatch-data-raw makes the same change to the separate `DispatchCommandOutput.Data` field on the other RPC path.)
- All 22 call sites (19 production + 3 test) are internal. No external plugin stability concern.
- RIB's `CommandHandler` type (`rib_commands.go:86`) is `func(r *RIBManager, selector string, args []string) (string, string, error)`. This needs the same `string` to `any` change for the data return.
- Many handlers define local structs (e.g., `type entry struct { Prefix string; NextHop string }`) and return slices of them. These structs are already perfectly shaped for direct JSON marshaling by the SDK.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/sdk/sdk_callbacks.go:175-191` - `OnExecuteCommand`: takes `func(serial, command, args, peer) (string, string, error)`, wraps in callback that unmarshals input, calls handler, marshals `ExecuteCommandOutput{Status, Data}`.
  → Constraint: handler returns `(status, data_as_json_string, error)`. SDK marshals the struct containing that string, double-encoding the data.
- [ ] `pkg/plugin/rpc/types.go:226-230` - `ExecuteCommandOutput{Status string, Data string}`. The `Data` field is a JSON string.
  → Constraint: `Data` is `string` today. Must become `json.RawMessage` (or equivalent) to hold pre-marshaled bytes without double-encoding.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:86` - `type CommandHandler func(r *RIBManager, selector string, args []string) (string, string, error)`. RIB's internal dispatch type.
  → Constraint: ~30 handlers registered via this type, all returning pre-marshaled JSON strings.
- [ ] `internal/component/bgp/plugins/rib/rib_commands.go:651-693` - `statusJSON()` example: builds `map[string]any`, `json.Marshal(result)`, returns `string(data)`.
  → Constraint: every `*JSON()` function follows this pattern. The function name even says "JSON" because it marshals.
- [ ] `internal/plugins/fib/kernel/backend.go:22-41` - `showInstalled()`: defines local `entry` struct, builds slice, marshals, returns string.
  → Constraint: same pattern in simpler form.
- [ ] `internal/component/plugin/ipc/rpc.go:268-278` - `SendExecuteCommand`: sends RPC, unmarshals result into `rpc.ExecuteCommandOutput`. Returns `*ExecuteCommandOutput` to engine callers.
  → Constraint: unmarshals `Data` field. If `Data` becomes `json.RawMessage`, `json.Unmarshal` handles it natively (no code change needed, but verify).
- [ ] `internal/component/plugin/server/command.go:739-743` - engine consumer of `ExecuteCommandOutput`: uses `rpcOut.Data` as error text on error path (line 741: `Error: rpcOut.Data`), and as `plugin.RawJSON(rpcOut.Data)` on success path (line 743).
  → Constraint: if `Data` becomes `json.RawMessage` (`[]byte`), the error path `Error: rpcOut.Data` won't compile (`Error` is `string`). Needs `string()` conversion. In practice, all current handlers return `("error", "", goError)` so `Data` is empty on errors, but the code must still compile.
- [ ] `internal/component/plugin/server/system.go:485-488` - same pattern as command.go: `rpcOut.Data` used as error text and as `plugin.RawJSON`.
  → Constraint: same compilation issue on error path.
- [ ] `internal/component/plugin/server/subsystem.go:266-269` - same pattern: `out.Data` used as error text and as `plugin.RawJSON`.
  → Constraint: same compilation issue on error path.
- [ ] `internal/test/plugins/fakeredist/register.go:61`, `fakel2tp/register.go:59`, `fakefib/register.go:46` - test plugins registering `OnExecuteCommand` with the old signature.
  → Constraint: must update to match new `(string, any, error)` return type.

**Behavior to preserve:**
- All command outputs produce identical JSON content.
- Error handling: `(status="error", "", err)` pattern unchanged.
- `statusDone` / `statusError` constants unchanged.
- No change to how data appears to consumers (CLI, SSH, web, LG).

**Behavior to change:**
- `OnExecuteCommand` callback signature: `(string, string, error)` becomes `(string, any, error)`.
- RIB `CommandHandler` type: same change.
- ~65 handler functions: drop `json.Marshal` + `string()`, return struct/map/slice directly.
- SDK wrapper: marshals `any` data to `json.RawMessage` before embedding in `ExecuteCommandOutput`.
- Handler functions renamed: drop `JSON` suffix (e.g., `statusJSON` becomes `status`, `retainRoutesJSON` becomes `retainRoutes`).

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Engine dispatches a command to a plugin via RPC or DirectBridge.
- Plugin's `OnExecuteCommand` callback is invoked.

### Transformation Path (current)
1. Handler builds struct/map (e.g., `map[string]any{"running": true, "peers": 5}`).
2. Handler calls `json.Marshal(result)` and returns `string(data)`. **Marshal #1.**
3. SDK wraps in `ExecuteCommandOutput{Status: "done", Data: that_string}`.
4. SDK calls `json.Marshal(&output)`. **Marshal #2.** Data is now JSON-string-inside-JSON.
5. Engine receives, eventually unwraps into `plugin.RawJSON(data_string)`.

### Transformation Path (proposed)
1. Handler builds struct/map (unchanged).
2. Handler returns struct/map directly as `any`. **Zero marshal.**
3. SDK marshals `any` data to `json.RawMessage`. **Marshal #1 (only one).**
4. SDK wraps in `ExecuteCommandOutput{Status: "done", Data: raw_bytes}`.
5. SDK calls `json.Marshal(&output)`. Data embedded as raw JSON, not double-encoded.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Handler -> SDK wrapper | `any` value (Go struct/map/slice) | [ ] |
| SDK -> RPC wire | `ExecuteCommandOutput` with `json.RawMessage` data | [ ] |
| RPC wire -> engine unmarshal | `json.Unmarshal` into `ExecuteCommandOutput` in `ipc/rpc.go:274` | [ ] |
| Engine unmarshal -> consumer | `rpcOut.Data` (`json.RawMessage`) converted to `string` for error or `plugin.RawJSON` for data | [ ] |

### Integration Points
- `ExecuteCommandOutput` (`pkg/plugin/rpc/types.go:227`) - wire type for RPC response.
- `plugin.RawJSON` (`internal/component/plugin/types.go:112`) - engine-side type for pre-serialized JSON from plugins. `type RawJSON string`, so `json.RawMessage` (`[]byte`) requires `string()` conversion.
- `command.go:739-743`, `system.go:485-488`, `subsystem.go:266-269` - three engine-side consumers of `ExecuteCommandOutput`.

### Architectural Verification
- [ ] No bypassed layers (SDK still wraps, RPC still carries, engine still unwraps)
- [ ] No unintended coupling (no new imports between packages)
- [ ] No duplicated functionality (reusing existing marshal path)
- [ ] Zero-copy preserved where applicable (struct passed by value, not re-serialized)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| Plugin `OnExecuteCommand` handler returns `any` | -> | SDK wrapper marshals once into `ExecuteCommandOutput` | `TestOnExecuteCommandAnyData` |
| RIB `CommandHandler` returns `any` | -> | RIB dispatch wraps into `OnExecuteCommand` | `TestRIBCommandHandlerAny` |
| Engine receives `ExecuteCommandOutput` with `json.RawMessage` Data | -> | Consumer extracts `plugin.RawJSON` without double-decode | `test/plugin/command-single-encode.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Handler returns `map[string]any` via `OnExecuteCommand` | SDK marshals data once; `ExecuteCommandOutput.Data` contains raw JSON, not a quoted JSON string. |
| AC-2 | Handler returns a typed struct (e.g., `[]entry`) | Same as AC-1: marshaled once, valid JSON in `Data`. |
| AC-3 | Handler returns `nil` data with status "done" | `Data` is empty/omitted. No marshal error. |
| AC-4 | Handler returns error | Error propagation unchanged. `Data` empty. |
| AC-5 | All 22 `OnExecuteCommand` call sites updated (19 production + 3 test) | Every handler returns `any` instead of pre-marshaled string. No `json.Marshal` in handler code for command results. |
| AC-6 | RIB `CommandHandler` type updated | Type signature is `(string, any, error)`. All ~30 registered handlers updated. |
| AC-7 | Engine-side consumers updated | `command.go`, `system.go`, `subsystem.go` handle `json.RawMessage` Data correctly: `string(rpcOut.Data)` on error path, `plugin.RawJSON(rpcOut.Data)` on success path. |
| AC-8 | Command output identical | `show bgp rib status`, `show fib-kernel`, `show sysctl`, etc. produce identical JSON output as before. |
| AC-9 | Full verification | `make ze-verify` passes. |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestOnExecuteCommandAnyMap` | `pkg/plugin/sdk/sdk_callbacks_test.go` | map[string]any data marshaled once, no double-encoding | |
| `TestOnExecuteCommandAnyStruct` | `pkg/plugin/sdk/sdk_callbacks_test.go` | typed struct data marshaled correctly | |
| `TestOnExecuteCommandAnyNil` | `pkg/plugin/sdk/sdk_callbacks_test.go` | nil data produces empty/omitted Data field | |
| `TestOnExecuteCommandAnySlice` | `pkg/plugin/sdk/sdk_callbacks_test.go` | slice data marshaled as JSON array | |
| `TestRIBStatusNoMarshal` | `internal/component/bgp/plugins/rib/rib_commands_test.go` | status() returns map, not string | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (no numeric inputs introduced) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `command-single-encode` | `test/plugin/command-single-encode.ci` | Plugin command returns structured data, CLI receives valid JSON | |

### Interop Tests (MANDATORY for protocol features)
- N/A - internal SDK change, no wire protocol.

### Future (if deferring any tests)
- Allocation benchmark comparing before/after could be added as follow-up.

## Files to Modify

### SDK (1 file)
- `pkg/plugin/sdk/sdk_callbacks.go` - `OnExecuteCommand` signature: `(string, string, error)` to `(string, any, error)`. Wrapper marshals `any` to `json.RawMessage`.

### RPC types (1 file)
- `pkg/plugin/rpc/types.go` - `ExecuteCommandOutput.Data`: `string` to `json.RawMessage`.

### BGP plugins (handler changes)
- `internal/component/bgp/plugins/rib/rib_commands.go` - `CommandHandler` type + ~30 handlers: drop `json.Marshal`, return struct/map.
- `internal/component/bgp/plugins/rib/rib_commands_community.go` - community handlers: same.
- `internal/component/bgp/plugins/rib/rib_pipeline.go` - pipeline terminal handlers: same.
- `internal/component/bgp/plugins/rib/rib_pipeline_best.go` - best pipeline: same.
- `internal/component/bgp/plugins/rib/rib_inject.go` - inject pipeline: same.
- `internal/component/bgp/plugins/rib/rib.go` - OnExecuteCommand call site.
- `internal/component/bgp/plugins/rr/rr.go` - `peersJSON` handler.
- `internal/component/bgp/plugins/rs/server.go` + `server_handlers.go` - RS handlers.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` + `rib_commands.go` - adj-RIB-in handlers.
- `internal/component/bgp/plugins/bmp/bmp.go` + `state.go` - BMP handlers.
- `internal/component/bgp/plugins/healthcheck/healthcheck.go` - healthcheck handlers.
- `internal/component/bgp/plugins/rpki/rpki.go` - RPKI handlers.
- `internal/component/bgp/plugins/gr/gr.go` + `gr_llgr.go` - GR/LLGR handlers.
- `internal/component/bgp/plugins/route_refresh/route_refresh.go` - RR handlers.
- `internal/component/bgp/plugins/softver/softver.go` - softver handlers.
- `internal/component/bgp/plugins/hostname/hostname.go` - hostname handlers.
- `internal/component/bgp/plugins/llnh/llnh.go` - LLNH handlers.
- `internal/component/bgp/plugins/watchdog/watchdog.go` + `server.go` - watchdog handlers.
- `internal/component/bgp/plugins/role/decode.go` - role decode.
- NLRI decode plugins: `nlri/vpn/`, `nlri/evpn/`, `nlri/flowspec/`, `nlri/ls/`, `nlri/rtc/`, `nlri/mup/`, `nlri/mvpn/`, `nlri/vpls/`.

### Non-BGP plugins (handler changes)
- `internal/plugins/sysctl/sysctl.go` + `register.go` - sysctl handlers.
- `internal/plugins/l2tpshaper/shaper.go` + `register.go` - L2TP shaper.
- `internal/plugins/l2tppool/register.go` - L2TP pool.
- `internal/plugins/fib/kernel/backend.go` + `register.go` - FIB kernel.
- `internal/plugins/fib/vpp/fibvpp.go` + `register.go` - FIB VPP.
- `internal/plugins/fib/p4/fibp4.go` + `register.go` - FIB P4.
- `internal/plugins/sysrib/sysrib.go` + `register.go` - sysrib.
- `internal/plugins/static/register.go` - static.
- `internal/plugins/policyroute/register.go` - policy route.

### Component plugins (handler changes)
- `internal/component/rsvpte/register.go` - RSVP-TE handlers.
- `internal/component/ldp/register.go` - LDP handlers.

### Test plugins (handler changes)
- `internal/test/plugins/fakeredist/register.go` - test plugin, `OnExecuteCommand` signature.
- `internal/test/plugins/fakel2tp/register.go` - test plugin, `OnExecuteCommand` signature.
- `internal/test/plugins/fakefib/register.go` - test plugin, `OnExecuteCommand` signature.

### Engine-side consumers (code changes required)
- `internal/component/plugin/server/command.go:739-743` - error path uses `rpcOut.Data` as string (line 741), success path wraps as `plugin.RawJSON(rpcOut.Data)` (line 743). Both need adapting for `json.RawMessage` Data.
- `internal/component/plugin/server/system.go:485-488` - same pattern as command.go.
- `internal/component/plugin/server/subsystem.go:266-269` - same pattern as command.go.
- `internal/component/plugin/ipc/rpc.go:274` - `json.Unmarshal` into `ExecuteCommandOutput`. Verify handles `json.RawMessage` Data natively (should work, no code change expected).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | No | - |
| YANG validation constraints | No | - |
| YANG custom validators | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | Yes | `test/plugin/command-single-encode.ci` |
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
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` - note OnExecuteCommand data return is now `any` |
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
- `test/plugin/command-single-encode.ci` - functional test proving single-encode

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

1. **Phase: Wiring (MANDATORY FIRST)** -- change SDK signature and RPC type
   - Tests: `TestOnExecuteCommandAnyMap`, `TestOnExecuteCommandAnyNil`
   - Files: `pkg/plugin/sdk/sdk_callbacks.go`, `pkg/plugin/rpc/types.go`
   - Verify: SDK accepts `any` data, marshals once. Existing tests compile (may fail until handlers updated).

2. **Phase: RIB handlers** -- update CommandHandler type and all RIB handler functions
   - Tests: `TestRIBStatusNoMarshal`
   - Files: `rib_commands.go`, `rib_commands_community.go`, `rib_pipeline.go`, `rib_pipeline_best.go`, `rib_inject.go`, `rib.go`
   - Verify: RIB handlers return struct/map. `show bgp rib status` output unchanged.

3. **Phase: BGP protocol plugins** -- update remaining BGP plugin handlers
   - Tests: existing plugin tests
   - Files: rr, rs, adj_rib_in, bmp, healthcheck, rpki, gr, route_refresh, softver, hostname, llnh, watchdog, role, nlri/*
   - Verify: each plugin compiles, handler tests pass.

4. **Phase: Non-BGP plugins** -- update sysctl, fib, l2tp, sysrib, static, policyroute, rsvpte, ldp
   - Tests: existing plugin tests
   - Files: see Files to Modify (non-BGP section)
   - Verify: each plugin compiles, handler tests pass.

5. **Phase: Test plugins** -- update fakeredist, fakel2tp, fakefib
   - Files: `internal/test/plugins/fakeredist/register.go`, `fakel2tp/register.go`, `fakefib/register.go`
   - Verify: test plugins compile with new signature.

6. **Phase: Engine-side consumers** -- adapt command.go, system.go, subsystem.go for `json.RawMessage` Data
   - Files: `internal/component/plugin/server/command.go`, `system.go`, `subsystem.go`
   - Changes: error path needs `string(rpcOut.Data)` conversion; success path `plugin.RawJSON(rpcOut.Data)` needs `string()` conversion (since `RawJSON` is `type RawJSON string` and Data is now `[]byte`).
   - Verify: `ipc/rpc.go` SendExecuteCommand unmarshals correctly. End-to-end command dispatch works.

7. **Functional tests** -- create .ci file
   - Files: `test/plugin/command-single-encode.ci`

8. **Full verification** -- `make ze-verify`

### Critical Review Checklist (/implement stage 6)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Command JSON output identical before and after |
| Naming | Renamed handlers (dropped JSON suffix) follow existing naming conventions |
| Data flow | Handler returns struct, SDK marshals once, no double-encoding |
| No-layering | No handler still calls `json.Marshal` for command results |
| Engine consumers | `command.go`, `system.go`, `subsystem.go` compile and handle `json.RawMessage` Data |
| Import hygiene | No new cross-package imports introduced |

### Deliverables Checklist (/implement stage 10)

| Deliverable | Verification method |
|-------------|---------------------|
| `OnExecuteCommand` accepts `any` | `grep 'func.*OnExecuteCommand' pkg/plugin/sdk/sdk_callbacks.go` |
| No `json.Marshal` in RIB handlers | `grep -c 'json.Marshal' internal/component/bgp/plugins/rib/rib_commands.go` returns 0 |
| `ExecuteCommandOutput.Data` is `json.RawMessage` | `grep 'Data.*json.RawMessage' pkg/plugin/rpc/types.go` |
| Functional test exists | `ls test/plugin/command-single-encode.ci` |
| No handler named `*JSON()` returns string | `grep -rn 'func.*JSON().*string' internal/component/bgp/plugins/rib/` returns 0 |
| Engine consumers handle `json.RawMessage` Data | `grep -n 'rpcOut.Data\|out.Data' internal/component/plugin/server/{command,system,subsystem}.go` shows `string()` conversion |
| Test plugins updated | `grep 'OnExecuteCommand' internal/test/plugins/*/register.go` shows new signature |

### Security Review Checklist (/implement stage 11)

| Check | What to look for |
|-------|-----------------|
| Input validation | `any` data from handler is trusted internal code, not external input. SDK marshals it; invalid data produces a marshal error, not a security issue. |
| Type confusion | `any` can be any Go value. `json.Marshal` handles all serializable types. Non-serializable types (channels, funcs) produce marshal errors, caught by SDK wrapper. |

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

## Design Insights

## Core Insight

The `OnExecuteCommand` callback signature forces every handler to pre-marshal its data to string, then the SDK re-marshals the whole output struct. Changing the data return from `string` to `any` moves the marshal responsibility to a single point (the SDK wrapper), eliminating ~65 redundant marshal calls.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Change `OnExecuteCommand` data return from `string` to `any` | Add new `OnExecuteCommandTyped` method keeping old one | All 22 call sites are internal, pre-release. No backward compatibility concern. One method is cleaner than two. |
| Change `ExecuteCommandOutput.Data` from `string` to `json.RawMessage` | Keep as `string`, have SDK stringify the marshaled bytes | `json.RawMessage` is the standard Go type for "already-marshaled JSON bytes". It embeds correctly without double-encoding. (Note: separate from `DispatchCommandOutput.Data` in spec-ipc-dispatch-data-raw, which is a different struct on a different RPC path.) |
| Rename handler functions (drop `JSON` suffix) | Keep existing names | Functions no longer produce JSON; the name would be misleading. `statusJSON` -> `status`, `retainRoutesJSON` -> `retainRoutes`. |

## Known Limitations

- Does not change the dispatcher-to-consumer boundary (serverDispatcherWithSurface still marshals `*plugin.Response`). That is a separate, lower-priority concern.
- Does not change the pipe chain (still parses/serializes per operator). Show commands are human-speed.
- Handler-local struct types (e.g., `type entry struct`) stay as-is. They are already well-shaped for JSON.

## RFC Documentation

N/A - no protocol work.

## Implementation Summary

### What Was Implemented
- [To be filled after implementation]

### Bugs Found/Fixed
- [To be filled after implementation]

### Documentation Updates
- [To be filled after implementation]

### Deviations from Plan
- [To be filled after implementation]

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
| Handlers return struct, no marshal | grep | `grep -c 'json.Marshal' rib_commands.go` = 0 |
| SDK marshals once | Unit test | `TestOnExecuteCommandAnyMap` |
| Output identical | Functional test | `command-single-encode.ci` |
| All 22 call sites updated | grep | `grep -rn 'string, string, error' */register.go` = 0 |
| Engine consumers handle json.RawMessage | grep | `string(rpcOut.Data)` in command.go, system.go, subsystem.go |

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
- [ ] Feature code integrated (`internal/*`, `cmd/*`, `pkg/*`)
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
- [ ] Write learned summary to `plan/learned/825-dispatch-response-passthrough.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/spec-dispatch-response-passthrough.md`
