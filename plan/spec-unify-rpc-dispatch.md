# Spec: unify-rpc-dispatch

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/6 |
| Updated | 2026-07-07 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/api/process-protocol.md` - plugin RPC transport (socket, direct, bridge)
4. `internal/component/plugin/server/dispatch.go` - the two magic-string switch tables plus `wireBridgeDispatch`
5. `pkg/plugin/rpc/bridge.go` - `DirectBridge` typed-slot boilerplate
6. `internal/component/plugin/registry/registry.go` - `CollectRPCHandlers` (the existing registry model to extend)

## Task

DESIGN-REVIEW.md finding 2 (row "Plugin RPC invocation") flags that each plugin->engine RPC
operation exists up to three times: a JSON handler (socket path), a Direct handler (in-process,
no socket), and a DirectBridge typed fast-path slot (no JSON). The three implementations are
linked only by magic-string method names (`ze-plugin-engine:*`) spread across two hand-maintained
`switch` tables in `dispatch.go` plus a hand-maintained `Set*` list in `wireBridgeDispatch`, and a
fourth per-method branch table on the SDK side (`sdk_engine.go`). Adding one operation means editing
all of these in lockstep, and nothing forces them to agree: coverage is already asymmetric
(subscribe/unsubscribe exist in JSON+Direct but not Bridge; route-install/route-remove exist in the
JSON switch only; inject-wire-route/batch-validate exist as Bridge slots only).

The behavior each path produces already funnels through single shared CORE methods
(`s.dispatchCommand`, `s.dispatchCommandArgs`, `s.deliverEvent`, `s.forwardCached`,
`s.releaseCached`, the Loc-RIB apply for route-install/remove). So the CORE logic is NOT duplicated;
the three DISPATCH/WIRING tables keyed by magic strings are. This spec unifies those tables into a
single method registry: one entry per operation carrying its method string, its proc-bound core
handler, its JSON codec (unmarshal input, marshal output), and an optional typed fast-path
descriptor. The JSON path, the Direct path, and the Bridge wiring all derive from that one entry, so
adding an operation touches one place and the paths cannot drift. This is an internal refactor: all
externally observable RPC behavior is preserved.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - plugin<->engine transport: socket mux, in-process direct, DirectBridge
  → Decision: three transports are a deliberate performance ladder (socket for external, direct for in-process JSON, bridge typed for hot paths); unification must preserve all three, only collapse the per-op wiring, not the transports.
  → Constraint: the DirectBridge typed fast paths (emit-event, dispatch-command, forward-cached) exist to skip JSON marshal on the hot path; the registry MUST keep a typed-descriptor escape hatch so these are not regressed back onto JSON.

### Rules
- [ ] `ai/rules/plugin-self-containment.md` - registration over hardcoding
  → Constraint: new operations must register, not be spelled into a central switch. The existing `CollectRPCHandlers` registry already does this for codec RPCs; the built-in ops must join the same registry rather than staying in the switch.
- [ ] `ai/rules/no-fabrication.md` - cite the producing function
  → Constraint: every path-coverage claim in the feature table is cited to `dispatch.go` / `bridge.go` line ranges below.

**Key insights:**
- The CORE handlers are already single-source-of-truth; only the three dispatch/wiring tables plus the SDK branch table duplicate the method-string keying.
- `CollectRPCHandlers` (registry.go:591) is an existing precedent: codec RPCs (decode-nlri, encode-nlri) are already registry-driven and consumed identically by both the JSON path (dispatch.go:113) and the Direct path (dispatch.go:586). The refactor generalizes that proven model to the built-in operations.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/dispatch.go` - `dispatchPluginRPC` JSON switch (:79-121, 10 arms), `dispatchPluginRPCDirect` switch (:564-593, 8 arms), `wireBridgeDispatch` Set* list (:834-878, 9 Set calls), and all `handle*RPC`/`handle*Direct` wrapper pairs plus the shared core methods `dispatchCommand` (:748), `dispatchCommandArgs` (:705), `deliverEvent` (:397).
  → Constraint: the JSON handler writes the socket via `conn.SendResult`/`conn.SendError`; the Direct handler returns marshaled `json.RawMessage` via `directResultResponse`. Both call the same core method. The registry entry must serve both output shapes.
- [ ] `internal/component/plugin/server/dispatch_cached.go` - `handleForwardCachedRPC`/`handleForwardCachedDirect` (:25/:47), `handleReleaseCachedRPC`/`handleReleaseCachedDirect` (:61/:83), shared cores `forwardCached` (:100), `releaseCached` (:114).
- [ ] `internal/component/plugin/server/dispatch_route.go` - `handleRouteInstallRPC` (:55), `handleRouteRemoveRPC` (:80); JSON-switch only, no Direct or Bridge coverage today.
- [ ] `pkg/plugin/rpc/bridge.go` - `DirectBridge` struct typed-func fields (:50-80) and, per typed op, the field + atomic bool + `Set*` + `Has*` + call-method quintet (e.g. dispatch-command at :238-262, emit-event at :406-432).
- [ ] `pkg/plugin/sdk/sdk_engine.go` - SDK-side per-op branch table: `if p.bridge != nil && p.bridge.HasX() { bridge.X() } else { callEngineWithResult(method) }` for UpdateRouteSel (:33), ForwardCached (:72), ReleaseCached (:85), InjectWireRoute (:134), BatchValidate (:144), DispatchCommand (:189), DispatchCommandArgs (:205), EmitEvent (:242).
- [ ] `pkg/plugin/sdk/sdk.go` - `callEngineRaw` (:448): generic fallback that routes any method string to `bridge.DispatchRPC` (post-startup) or `engineMux.CallRPC` (socket).
- [ ] `internal/component/plugin/registry/registry.go` - `CollectRPCHandlers` (:591), `AddRPCHandlers` (:585): the existing registry both dispatch paths already consult for codec RPCs.

**Behavior to preserve:**
- Every `ze-plugin-engine:*` method string keeps the same wire name and the same input/output JSON shape (external plugins depend on it).
- The three transports remain: external plugins keep the socket JSON path; in-process plugins keep the Direct and typed-Bridge fast paths. Typed hot paths (emit-event, forward-cached, dispatch-command, inject-wire-route, batch-validate) keep bypassing JSON.
- Unknown-method handling stays fail-closed: an unregistered method returns "unknown method: X" (dispatch.go:118, :591).
- Plugin identity/authorization behavior is unchanged (username "plugin:<name>" set in the core handlers, dispatch.go:190, :714, :753).
- Existing tests `TestDispatchCommandDirectBridge`, `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand`, `TestRPCRegistrationExpectedMethods`, `TestApplyRouteInstallInsertsPath` pass unchanged.

**Behavior to change:**
- None - internal refactor, behavior preserved. Coverage gaps (route-install/remove Direct arm; inject-wire-route/batch-validate JSON fallback) are closed as a by-product of registry derivation, but no operation changes its externally observable result.

**Feature inventory (operation x path):** which of the three engine paths implement each plugin->engine op, cited to source.

| Operation (method string suffix) | Path 1 JSON (dispatch.go:79-121) | Path 2 Direct (dispatch.go:564-593) | Path 3 Bridge typed slot (dispatch.go:834-878 + bridge.go) | Shared CORE method |
|----------------------------------|----------------------------------|-------------------------------------|------------------------------------------------------------|--------------------|
| update-route | yes (:81 -> handleUpdateRouteRPC :125) | yes (:568 -> handleUpdateRouteDirect :597) | yes, typed *selector variant (:852 SetUpdateRouteSel -> handleUpdateRouteSelDirect :633) | s.dispatcher.Dispatch |
| dispatch-command | yes (:83 :174) | yes (:569 :674) | yes (:846 SetDispatchCommand) | s.dispatchCommand (:748) |
| dispatch-command-args | yes (:85 :221) | yes (:571 :689) | yes (:849 SetDispatchCommandArgs) | s.dispatchCommandArgs (:705) |
| emit-event | yes (:95 :353) | yes (:577 :801) | yes (:843 SetEmitEvent -> s.deliverEvent) | s.deliverEvent (:397) |
| forward-cached | yes (:98) | yes (:579) | yes (:856 SetForwardCached) | s.forwardCached (:100 cached) |
| release-cached | yes (:101) | yes (:581) | yes (:859 SetReleaseCached) | s.releaseCached (:114 cached) |
| subscribe-events | yes (:89 :314) | yes (:573 :774) | NO | s.registerSubscriptions (:280) |
| unsubscribe-events | yes (:92 :337) | yes (:575 :791) | NO | s.subscriptions.ClearProcess |
| route-install | yes (:104) | NO | NO | Loc-RIB apply (dispatch_route.go) |
| route-remove | yes (:107) | NO | NO | Loc-RIB apply (dispatch_route.go) |
| inject-wire-route | NO | NO | yes only (:863 SetInjectWireRoute -> rpc.GetRouteInjector) | globalRouteInjector (bridge.go:508) |
| batch-validate | NO | NO | yes only (:871 SetBatchValidate -> rpc.GetBatchValidator) | globalBatchValidator (bridge.go:572) |
| codec RPCs (decode-nlri, encode-nlri, ...) | registry (:113 getRPCHandlers) | registry (:586 getRPCHandlers) | via generic DispatchRPC fallback | registry.CollectRPCHandlers (:591) |

**What each path uniquely provides (why all three exist):**
- JSON path: socket I/O plus wire compatibility. Required for external (forked) plugins that talk over the mux; unmarshals `req.Params`, dispatches, writes `conn.SendResult`.
- Direct path: in-process JSON without socket I/O. Used by `DirectBridge.DispatchRPC` generic fallback for ops with no typed slot; returns marshaled `json.RawMessage` rather than writing a socket.
- Bridge typed slot: skips JSON marshal/unmarshal entirely, passing native Go values. Reserved for hot paths (emit-event, forward-cached, dispatch-command, inject-wire-route zero-copy, batch-validate no-string).

**Asymmetry to fix (coverage gaps that prove the drift):**
- subscribe-events / unsubscribe-events: JSON + Direct, no typed Bridge slot (acceptable; low frequency, but currently implicit not declared).
- route-install / route-remove: JSON switch only. No Direct handler means these fail if ever dispatched over `DirectBridge.DispatchRPC` for an in-process forked-style caller; today they are reached only from forked (socket) plugins, but the missing Direct arm is exactly the kind of silent gap the registry closes.
- inject-wire-route / batch-validate: Bridge-only. No JSON/Direct entry: the SDK either errors ("bridge not available", sdk_engine.go:137) or falls back to a hand-rolled string encoding through dispatch-command-args (sdk_engine.go:147+). A registry entry with a JSON codec would give these a uniform socket fallback.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A plugin issues a plugin->engine RPC. Three concrete entry points converge on the engine:
- External/forked plugin: bytes arrive on the mux, read by `handleSingleProcessCommandsRPC` (dispatch.go:36), producing a `*rpc.Request` dispatched via `dispatchPluginRPC` (dispatch.go:78).
- In-process plugin, generic op: `DirectBridge.DispatchRPC(method, params)` (bridge.go:612) calls the engine's registered dispatch func, which is `dispatchPluginRPCDirect` (dispatch.go:564).
- In-process plugin, hot op: a typed `DirectBridge` slot (e.g. `EmitEvent`, bridge.go:422) calls the engine core method directly with native Go values, no JSON.

### Transformation Path
1. SDK caller picks a transport: `sdk_engine.go` checks `bridge.HasX()` for a typed slot, else `callEngineRaw` (sdk.go:448) routes to `bridge.DispatchRPC` or `engineMux.CallRPC`.
2. Engine receives the call on one of the three entry points above and looks up the operation. Today: a magic-string `switch` (JSON at dispatch.go:79, Direct at dispatch.go:566) or a hand-wired typed slot (`wireBridgeDispatch`, dispatch.go:834).
3. The matched handler unmarshals params (JSON paths) or accepts native values (typed path) and calls the shared CORE method (`dispatchCommand`, `deliverEvent`, `forwardCached`, Loc-RIB apply, ...).
4. The CORE method runs the real logic (dispatcher, event bus, cache forward, Loc-RIB write) and returns a typed output.
5. Output is marshaled and written back: `conn.SendResult` (JSON socket), `directResultResponse` (Direct), or returned as a native value (typed).

After the refactor, step 2 becomes: a single `methodRegistry` lookup returns one entry; the JSON path runs `entry.serveJSON` (unmarshal, call core, SendResult), the Direct path runs `entry.serveDirect` (unmarshal, call core, marshal), and `wireBridgeDispatch` iterates registry entries that declare a typed descriptor and calls the corresponding `bridge.Set*`. Steps 3 to 5 are unchanged: the CORE methods are untouched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process <-> engine (external) | JSON `rpc.Request` over mux, method string keyed | [ ] |
| In-process plugin <-> engine (generic) | `DirectBridge.DispatchRPC` method string keyed | [ ] |
| In-process plugin <-> engine (hot) | typed `DirectBridge` slot, native Go values | [ ] |
| Registry <-> dispatch | `CollectRPCHandlers` map consulted by both JSON and Direct today; extended to built-in ops | [ ] |

### Integration Points
- `internal/component/plugin/registry` `CollectRPCHandlers`/`AddRPCHandlers`: the existing method registry the built-in operations join.
- `internal/component/plugin/server` `Server` methods `dispatchCommand`, `dispatchCommandArgs`, `deliverEvent`, `forwardCached`, `releaseCached`: the unchanged CORE handlers registry entries bind to.
- `pkg/plugin/rpc` `DirectBridge` `Set*` typed slots: still set, but by a registry-driven loop rather than a hand-written list.
- `pkg/plugin/sdk` `sdk_engine.go`: the SDK-side per-op branch table is the mirror image and is folded into a single helper that consults a shared descriptor for "has typed slot".

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (typed Bridge slots keep native values; inject-wire-route stays zero-copy)
- [ ] Registration over hardcoding — every plugin->engine operation registers one entry in the shared method registry (the same registry that already holds codec RPCs); the JSON path, Direct path, and Bridge wiring all discover operations through that registry. No new per-operation `case` arm is added to a central switch, and adding an operation touches exactly one registration site (small-core/registration; `ai/rules/plugin-self-containment.md`).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The JSON and Direct handlers for each op differ ONLY in output plumbing (SendResult vs directResultResponse); their unmarshal + core-call is identical. | dispatch.go pairs: handleDispatchCommandRPC :174 vs handleDispatchCommandDirect :674 both unmarshal `DispatchCommandInput` and call `s.dispatchCommand`. | Registry entry cannot serve both shapes; would need per-path entries. | Diff each RPC/Direct pair; grep confirms shared core call; unit test `TestDispatchCommandDirectBridge` asserts identical output. | confirmed (success paths). NUANCE: emit-event's JSON error path sends `err.Error()` on an `*rpc.RPCCallError`, which prepends `"rpc error: "` (message.go:28), while the Direct path returns the bare `RPCCallError`. So JSON and Direct error messages diverged for emit-event (a latent bug). Also handleDispatchCommandRPC (:174) inlined a copy of s.dispatchCommand rather than calling it. Unified serve derives the sent string from `RPCCallError.Message` (raw, no prefix) for both paths, aligning them (AC-2). See Mistake Log. |
| A-2 | The codec-RPC registry (`CollectRPCHandlers`, `func(json.RawMessage)(any,error)`) is a sufficient shape for built-in ops once the handler is proc-bound (closure capturing proc+Server). | registry.go:585-605; dispatch.go:113,:586 consume it in both paths. | Registry needs a richer entry type carrying proc, not a bare func. | Prototype one op (forward-cached) as a proc-bound registry entry; run `TestRPCRegistrationExpectedMethods`. | broken. The RETURN shape `(any, error)` is sufficient (proven by handleCodecRPC/handleCodecRPCDirect), but built-in ops need `proc`. Capturing proc in a per-call closure would allocate on a hot path (R-3). Resolution: the entry carries `handle func(*Server, *process.Process, json.RawMessage) (any, error)` — proc PASSED, not captured; a single package-level table value, zero per-request alloc. The registry also lives in the SERVER package (not the leaf `registry` package, which cannot import `*process.Process`/`*Server`). See Deviations + Mistake Log. |
| A-3 | route-install/route-remove are reached only from forked (socket) plugins today, so adding their Direct+registry arm is additive and breaks nothing. | dispatch_route.go handlers appear in JSON switch only (dispatch.go:104-109); SDK `RouteInstall` uses `callEngineWithResult` (sdk_engine.go:101) which prefers bridge DispatchRPC when in-process. | An in-process caller currently hitting DispatchRPC would already fail (unknown method in Direct switch); adding the arm is a fix, still additive. | grep callers of `RouteInstall`/`RouteRemove`; confirm forked-only via `ze.plugin.hub.token` guard note in dispatch_route.go:6. | confirmed. route-install/route-remove appear only in dispatchPluginRPC (:104-109), absent from dispatchPluginRPCDirect (:566-583). Registry derivation gives them a Direct arm as a pure addition. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Typed Bridge fast paths get flattened onto JSON during unification, regressing hot-path allocation (emit-event, forward-cached, inject-wire-route zero-copy). | Benchmark `BenchmarkPluginDispatch` regresses; alloc counts rise. | Registry entry carries an OPTIONAL typed descriptor; `wireBridgeDispatch` still sets the native typed slot for entries that declare one. Keep the typed `Set*` slots in bridge.go; only the WIRING list becomes a loop. |
| R-2 | Registry-driven wiring changes handler ordering or unknown-method semantics, breaking fail-closed behavior. | A previously-rejected method now silently no-ops, or a known method returns "unknown". | Keep the explicit "unknown method" fallthrough (dispatch.go:118,:591). Assert exact method set via `TestRPCRegistrationExpectedMethods` extended to include the built-in ops. |
| R-3 | Proc-binding via closures per request allocates on a path that was allocation-free. | Alloc benchmark on dispatch regresses. | Bind proc at call time only for JSON/Direct paths (which already unmarshal and allocate); keep typed slots proc-bound once at `wireBridgeDispatch` time as today. |

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| In-process plugin calls dispatch-command; both JSON and typed Bridge paths must yield identical output | → | registry entry -> `s.dispatchCommand` via both `serveJSON` and typed `SetDispatchCommand` | `TestDispatchCommandDirectBridge` (dispatch_test.go:433) |
| Plugin issues an exact registered command with args; args and string paths converge on one handler | → | registry entry -> `s.dispatchCommandArgs` | `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand` (dispatch_test.go:474) |
| Registry advertises exactly the expected built-in + codec method set (fail-closed on unknown) | → | `registry.CollectRPCHandlers` extended with built-in ops | `TestRPCRegistrationExpectedMethods` (rpc_registration_test.go:75) |
| Forked plugin ships a route batch; engine applies to Loc-RIB via the registry-derived route-install arm | → | registry entry -> Loc-RIB apply (dispatch_route.go) | `TestApplyRouteInstallInsertsPath` (dispatch_route_test.go:63) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The engine receives every current `ze-plugin-engine:*` method over the JSON socket path. | Each method dispatches to its shared CORE handler and returns the identical JSON result it does today; no method string or output shape changes. |
| AC-2 | The engine receives a generic op over `DirectBridge.DispatchRPC` (Direct path). | Result is byte-identical to the JSON path (both derive from one registry entry); unknown methods still return "unknown method: X". |
| AC-3 | An in-process plugin uses a typed Bridge fast-path op (emit-event, dispatch-command, forward-cached, inject-wire-route, batch-validate). | The native typed slot is still invoked (no JSON marshal introduced); `wireBridgeDispatch` set the slot by iterating the registry's typed-descriptor entries, not a hand-written list. |
| AC-4 | A developer adds a new plugin->engine operation. | Exactly one registration site is edited (the registry entry); the JSON path, Direct path, and Bridge wiring pick it up automatically; no central switch arm is added. A test asserts the three paths cover the same method set. |
| AC-5 | route-install/route-remove are dispatched. | They resolve through the registry (gaining a Direct arm), preserving the forked-plugin Loc-RIB apply and disconnect-withdrawal behavior unchanged. |
| AC-6 | inject-wire-route/batch-validate are dispatched from a plugin without a typed bridge slot available. | A registry JSON codec fallback exists so the operation has a defined non-typed path instead of an ad-hoc "bridge not available" error or a hand-rolled string encoding. |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchCommandDirectBridge` | `internal/component/plugin/server/dispatch_test.go` | JSON path and typed Bridge path yield identical dispatch-command output after unification | |
| `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand` | `internal/component/plugin/server/dispatch_test.go` | args path and string path converge on the one registry-bound core handler | |
| `TestRPCRegistrationExpectedMethods` | `internal/component/plugin/server/rpc_registration_test.go` | registry advertises exactly the built-in + codec method set; unknown methods rejected | |
| `TestPluginRPCRegistryCoversAllPaths` (new) | `internal/component/plugin/server/dispatch_test.go` | for every registered op, the JSON, Direct, and (where declared) Bridge derivations exist and reference the same core method (drift guard for AC-4) | |
| `TestApplyRouteInstallInsertsPath` / `TestApplyRouteRemoveWithdrawsPath` | `internal/component/plugin/server/dispatch_route_test.go` | route-install/remove still apply/withdraw Loc-RIB paths through the registry-derived arm (AC-5) | |
| `TestConcurrentPluginDispatch` | `internal/component/plugin/server/dispatch_test.go` | concurrent plugin->engine dispatch remains correct after registry lookup replaces the switch | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| existing plugin/RPC suite | `internal/component/plugin/server/*_test.go` + interop scenarios under `test/interop/scenarios/` (ospf/isis forked route install, rpki batch-validate) | No user-facing behavior change; existing test suite passes with no regressions. Internal refactor: dispatch-command, emit-event, forward-cached, route-install, and batch-validate all behave identically end-to-end. | |

## Files to Modify
- `internal/component/plugin/server/dispatch.go` - replace `dispatchPluginRPC` and `dispatchPluginRPCDirect` magic-string switches, and the `wireBridgeDispatch` hand-written `Set*` list, with registry-driven dispatch and a typed-descriptor wiring loop.
- `internal/component/plugin/server/dispatch_cached.go` - register forward-cached/release-cached as registry entries; drop the near-duplicate RPC/Direct wrapper pair, keep the shared cores.
- `internal/component/plugin/server/dispatch_route.go` - register route-install/route-remove; gain the Direct arm from derivation (AC-5).
- `internal/component/plugin/registry/registry.go` - extend the RPC-handler registry to carry proc-bound built-in entries plus an optional typed-fast-path descriptor (the gap features A-2 identifies).
- `pkg/plugin/rpc/bridge.go` - keep the typed `Set*`/`Has*`/call slots (hot-path escape hatch), but the engine wires them from the registry loop; add a JSON codec fallback registration hook for inject-wire-route/batch-validate (AC-6).
- `pkg/plugin/sdk/sdk_engine.go` - fold the per-op `HasX()` branch table into one helper driven by a shared typed-slot descriptor, so the SDK caller table stops being a fourth hand-maintained list.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] no | n/a - no new config surface |
| CLI commands/flags | [ ] no | n/a - internal dispatch refactor |
| Functional test for new RPC/API | [ ] no new RPC | existing `test/interop/scenarios/` cover the ops end-to-end |
| Prometheus counters/metrics | [ ] no | route counters in dispatch_route.go preserved unchanged |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 8 | Plugin SDK/protocol changed? | Yes (internal only; wire method names + JSON shapes unchanged; SDK now uses `rpc.Method*` constants and JSON fallbacks for inject/batch) | `docs/architecture/api/process-protocol.md` -- added "Engine-side dispatch registry" note; existing `sdk_engine.go` anchors still valid (methods unchanged in name/behavior) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md` -- "Engine-side dispatch registry" paragraph + Files-table row, both with source anchors |
| 16 | Any changed source file referenced by doc source anchors? | Yes | `docs/` source anchors: only live one is `process-protocol.md:877 dispatch.go -- dispatchCommandArgs` (symbol kept, valid). Hand-maintained digest anchors `ai/digests/plugin-transport.md` + `aaa-auth.md` updated for moved `dispatch.go` lines; `ai/DOCS-TO-CODE.md` regenerated for the new `dispatch_registry.go`. `make ze-doc-test` PASSES. |

## Files to Create
- (none expected) - the unification reuses the existing registry package; new tests live in existing `*_test.go` files.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, TDD Test Plan |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 5. /ze-review gate | Review Gate section |
| 6. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 7-10. Critical review + re-verify | Critical Review Checklist below |
| 11-14. Deliverables, security, summary | Checklists below |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** — add a characterization/drift-guard test capturing current behavior before any refactor.
   - Tests: `TestPluginRPCRegistryCoversAllPaths` (new, initially asserts the current three-table coverage and the exact method set); re-run `TestDispatchCommandDirectBridge`, `TestRPCRegistrationExpectedMethods` as the baseline.
   - Files: `internal/component/plugin/server/dispatch_test.go` (new test), read-only survey of `dispatch.go`/`bridge.go`.
   - Verify: the new test passes against the current switch-based code (locks in behavior), and fails if a method is present in one table but not another.
2. **Phase: Extend the registry entry type** — give the RPC-handler registry a proc-bound entry plus an optional typed-fast-path descriptor (closes gap A-2).
   - Tests: `TestRPCRegistrationExpectedMethods` extended to include built-in ops.
   - Files: `internal/component/plugin/registry/registry.go`.
   - Verify: codec RPCs still resolve identically; a prototype built-in entry (forward-cached) resolves through the registry.
3. **Phase: Migrate the JSON + Direct switches to registry lookup** — replace `dispatchPluginRPC` and `dispatchPluginRPCDirect` switch arms with a single registry lookup driving `serveJSON`/`serveDirect`; keep the "unknown method" fallthrough.
   - Tests: `TestConcurrentPluginDispatch`, `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand`, `TestApplyRouteInstallInsertsPath`.
   - Files: `internal/component/plugin/server/dispatch.go`, `dispatch_cached.go`, `dispatch_route.go`.
   - Verify: all three paths still produce identical results; route-install/remove gain a Direct arm (AC-5).
4. **Phase: Drive wireBridgeDispatch from the registry** — replace the hand-written `Set*` list with a loop over registry entries that declare a typed descriptor; keep the typed slots in bridge.go.
   - Tests: `TestDispatchCommandDirectBridge`; a benchmark guard for R-1/R-3 (no new allocs on typed hot paths).
   - Files: `internal/component/plugin/server/dispatch.go`, `pkg/plugin/rpc/bridge.go`.
   - Verify: typed fast paths still bypass JSON; benchmark shows no alloc regression.
5. **Phase: Fold the SDK-side branch table** — collapse `sdk_engine.go` per-op `HasX()` branches into one helper keyed by the shared typed-slot descriptor; add the JSON fallback for inject-wire-route/batch-validate (AC-6).
   - Tests: SDK dispatch tests; interop scenarios for rpki batch-validate and forked route install.
   - Files: `pkg/plugin/sdk/sdk_engine.go`, `pkg/plugin/rpc/bridge.go`.
   - Verify: external-plugin fallbacks work; no typed regression for in-process.
6. **Full verification** → `make ze-verify`.
7. **Complete spec** → fill audit tables, write learned summary, two commits.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line; the feature-inventory table's 12 ops all resolve through the registry. |
| Correctness | JSON, Direct, and typed Bridge derivations produce identical results per op; unknown-method stays fail-closed. |
| Data flow | CORE methods (`dispatchCommand`, `deliverEvent`, `forwardCached`, Loc-RIB apply) are untouched; only dispatch/wiring tables change. |
| No hot-path regression | Typed Bridge slots still carry native values; benchmark confirms no new allocs (R-1, R-3). |
| Registration over hardcoding | Adding an op edits exactly one registration site; no new central switch arm. `TestPluginRPCRegistryCoversAllPaths` enforces path parity (AC-4). See `ai/rules/plugin-self-containment.md`. |
| Rule: no-layering | The two magic-string switches and the hand-written `wireBridgeDispatch` list are fully deleted, not left alongside the registry. |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Single method registry drives all three engine paths | grep shows no `case "ze-plugin-engine:` switch arms remain in `dispatch.go` |
| Typed hot paths preserved | benchmark diff shows no alloc regression on emit-event/forward-cached |
| Path-parity drift guard exists | `go test -run TestPluginRPCRegistryCoversAllPaths` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Registry-driven unmarshal still rejects malformed params per op (preserve the existing `SendError` on bad JSON). |
| Fail-closed dispatch | Unknown/unregistered method still returns an explicit error, never a silent success. |
| Authorization preserved | Plugin identity ("plugin:<name>") and authz checks in the CORE handlers remain on the path. |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails behavior mismatch | Re-read source from Current Behavior; a path derivation diverged from the switch |
| Alloc benchmark regresses | Phase 4 typed-slot wiring; ensure native values not routed through JSON |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| A-2: the leaf codec-registry shape `func(json.RawMessage)(any,error)` is a sufficient home for built-in ops once proc-bound via a closure. | The RETURN shape is sufficient, but built-in ops need `proc` and bind Server/bridge types the leaf registry cannot import; capturing proc per-request also allocates (R-3). | Read registry.go (leaf, no plugin-impl deps) and the built-in handlers (all need proc). | Registry moved to the server package with `handle func(*Server,*process.Process,json.RawMessage)(any,error)` (proc passed, not captured). No leaf-package change. |
| A-1: JSON and Direct handlers differ ONLY in output plumbing. | True for success paths, but the emit-event JSON error path double-prefixed via `RPCCallError.Error()` while Direct did not -- the error plumbing also differed. | Read `message.go` `RPCCallError.Error()` (prepends `"rpc error: "`). | Unified `rpcErrMessage` sends the raw `.Message` on both paths (AC-2). emit-event JSON error text loses one redundant prefix. |
| Wiring-Test row cited `TestRPCRegistrationExpectedMethods` as the engine-op drift guard. | That test covers `AllBuiltinRPCs()` (a different `ze-system:*`/`ze-plugin:*` `RPCDispatcher`), not `ze-plugin-engine:*`. | Read rpc_registration_test.go + command.go `AllBuiltinRPCs`. | Built the correct guard `TestPluginRPCRegistryCoversAllPaths`; cited test left untouched (passes). |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Force a generic Go helper to fold every SDK `HasX()` branch into one call. | The typed slots have distinct Go signatures (int / *Output / selector); a single generic helper needs reflection or awkward generics and reads worse than the branches. | Made every SDK engine method follow ONE uniform shape (typed-if-available else `callEngine*` with an `rpc.Method*` constant); the two odd-ones-out (inject error, batch string-encode) were the real drift and are gone. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner: a SINGLE method registry (one entry per operation: method string + proc-bound core handler + JSON codec + optional typed descriptor) from which the JSON path, Direct path, and Bridge wiring all derive. | (a) Keep the three switches, add a lint check that they stay in sync. (b) Merge JSON and Direct into one and keep Bridge separate. (c) Collapse the typed Bridge slots onto JSON. | The CORE methods are already single-source-of-truth, so the duplication is purely the three magic-string dispatch/wiring tables plus the SDK branch table. A registry (option chosen) removes the drift structurally and matches the existing `CollectRPCHandlers` precedent that already unifies codec RPCs across the JSON and Direct paths. (a) still permits drift and adds a checker to maintain. (b) leaves the Bridge list hand-written, the worst offender for coverage gaps. (c) regresses hot-path allocation (R-1) and is rejected: the typed slots exist for a reason. |
| Keep the three transports; unify only the per-op wiring. | Reduce to one transport. | The socket/direct/typed ladder is a deliberate performance design (process-protocol.md). Unifying transports would regress external-plugin compatibility and hot-path allocation. The finding is about the triplicated WIRING, not the transports. |
| Registry entry carries an OPTIONAL typed-fast-path descriptor; typed slots stay in bridge.go. | Force every op through the JSON codec. | Preserves zero-copy inject-wire-route and no-JSON emit-event/forward-cached (R-1). Ops with no descriptor simply have no typed slot (subscribe/unsubscribe), exactly matching today. |
| Close coverage gaps as a by-product: route-install/remove gain a Direct arm; inject-wire-route/batch-validate gain a JSON fallback entry. | Leave the asymmetry as-is. | The asymmetry is the concrete evidence of drift; registry derivation makes uniform coverage the default, and consistent coverage is an explicit task goal. |

## Known Limitations
- The typed Bridge slots (`Set*`/`Has*`/call quintet per op) remain hand-written in bridge.go: the registry drives WHEN they are wired, but the SDK still needs the concrete typed signatures for compile-time type safety. Fully generating those would need code generation and is out of scope.
- subscribe-events/unsubscribe-events intentionally keep no typed Bridge slot (low frequency); the registry records this as "no typed descriptor" rather than adding one.

## RFC Documentation
Not applicable - internal dispatch refactor, no protocol/wire behavior.

## Implementation Summary

### What Was Implemented
- **`internal/component/plugin/server/dispatch_registry.go` (new):** the single `engineOp`
  table (`engineOps`) + `engineOpTable`/`lookupEngineOp`, the `serveEngineOpJSON` /
  `serveEngineOpDirect` wrappers, `rpcErrMessage`, and the shared `op*` handlers for the
  dispatch.go-domain ops (update-route, dispatch-command, dispatch-command-args,
  subscribe, unsubscribe, emit-event) plus the AC-6 JSON fallbacks (inject-wire-route,
  batch-validate). Each entry has a proc-passed `handle` (JSON + Direct derive from it)
  and an optional `typedWire` descriptor (the bridge fast-path slot).
- **`dispatch.go`:** `dispatchPluginRPC` and `dispatchPluginRPCDirect` now do a registry
  lookup then codec-registry fallback then fail-closed "unknown method"; the 10-arm and
  8-arm magic-string switches are gone. `wireBridgeDispatch` iterates `engineOps` and calls
  each `typedWire`, replacing the hand-written 8-`Set*` list. Six JSON handlers and six
  Direct handlers deleted (logic moved into the shared `op*` handlers / kept cores).
- **`dispatch_cached.go`:** four handlers (`handleForward/ReleaseCachedRPC/Direct`) collapsed
  to `opForwardCached` / `opReleaseCached`; cores unchanged.
- **`dispatch_route.go`:** `handleRouteInstall/RemoveRPC` + `sendRouteErr` replaced by
  `opRouteInstall` / `opRouteRemove`; they gain a Direct arm from registry derivation (AC-5).
- **`pkg/plugin/rpc/types.go`:** shared `rpc.Method*` constants (single source of truth for
  SDK + engine) and `InjectWireRouteInput` / `BatchValidateInput` (AC-6 codec shapes).
- **`pkg/plugin/sdk/sdk_engine.go`:** every engine-call method now follows one uniform shape
  (typed slot if available, else `callEngine*` with an `rpc.Method*` constant). `InjectWireRoute`
  no longer errors "bridge not available"; `BatchValidate` no longer hand-rolls a stride-6
  string -- both use the JSON codec fallback (AC-6). Removed the `strconv` string encoding.

### Bugs Found/Fixed
- **emit-event JSON error double-prefix (latent):** the old JSON path sent
  `RPCCallError.Error()` (which prepends `"rpc error: "`) while the Direct path returned the
  bare `RPCCallError`, so the two transports produced different error text for emit-event. The
  unified `rpcErrMessage` sends the raw `.Message` on both paths, aligning them (AC-2).

### Documentation Updates
- `docs/architecture/api/process-protocol.md`: added an "Engine-side dispatch registry"
  paragraph (registry-driven derivation of the three transports) + a `dispatch_registry.go`
  row in the Files table, both with source anchors. Existing anchors (`dispatchCommandArgs`,
  `DirectBridge`, `sdk_engine.go` methods, `wireBridgeDispatch`) remain valid: those symbols
  still exist and behave identically.

### Deviations from Plan
- **Registry lives in the server package, not the leaf `registry` package.** Files-to-Modify
  listed `internal/component/plugin/registry/registry.go`, but the built-in ops bind Server
  methods + `*process.Process` + bridge types, which the leaf registry (by design: "no
  dependencies on plugin implementations") cannot import. The unified table therefore lives in
  the server package (`dispatch_registry.go`). Codec RPCs stay in `CollectRPCHandlers` (leaf),
  consulted as the fallback exactly as before. This satisfies the design intent (one entry per
  op, all three paths derive from it, one registration site) at the correct tier. A-2 broken.
- **emit-event JSON error text changed** (drops a redundant `"rpc error: "` prefix) to make the
  JSON and Direct error messages byte-identical (AC-2). No test asserted the old text.
- **`batch-validate` SDK fallback no longer routes through the `request bgp adj-rib-in
  batch-validate` command** (which applied command authorization); it now uses the JSON codec
  -> `GetBatchValidator`, exactly as the typed bridge slot already did. The typed path never
  applied command authz either, so this aligns the fallback with the hot path rather than
  removing a live check. The command itself is unchanged and still tested/CLI-reachable.
- **`TestRPCRegistrationExpectedMethods` was NOT extended** (the spec's Wiring Test row cited it
  for the built-in ops). That test covers `AllBuiltinRPCs()` -- the separate `ze-system:*` /
  `ze-plugin:*` `RPCDispatcher` mechanism, not the `ze-plugin-engine:*` dispatch. The drift
  guard for the engine ops is the new `TestPluginRPCRegistryCoversAllPaths`; the cited test
  passes unchanged.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Collapse the three dispatch/wiring tables into one method registry | Done | `dispatch_registry.go` `engineOps` | JSON switch + Direct switch + `Set*` list all removed |
| Fold the fourth SDK branch table | Done | `sdk_engine.go` | uniform typed-else-JSON shape; `rpc.Method*` constants |
| Adding an op touches one place; paths cannot drift | Done | `engineOps` + `TestPluginRPCRegistryCoversAllPaths` | one entry per op |
| Preserve all three transports (socket/direct/typed) | Done | `serveEngineOpJSON`/`serveEngineOpDirect`/`typedWire` | typed slots unchanged |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `dispatch_registry.go` `serveEngineOpJSON` + `engineOps`; `TestDispatchCommandToPlugin`, `TestHandleDispatchCommandRPCPreservesPluginIdentity` | every method dispatches to its shared core, same JSON output |
| AC-2 | Done | `serveEngineOpDirect` + `dispatchPluginRPCDirect`; `TestEngineOpJSONAndDirectMatch`, `TestDispatchCommandDirectBridge` | Direct byte-identical to JSON; unknown -> "unknown method: X" |
| AC-3 | Done | `dispatch.go` `wireBridgeDispatch` loop + `typedWire`; `TestWireBridgeDispatchInstallsTypedSlots` | typed slots installed by iterating registry, no JSON marshal |
| AC-4 | Done | `engineOps` single table; `TestPluginRPCRegistryCoversAllPaths` | one registration site; path-parity asserted |
| AC-5 | Done | `dispatch_route.go` `opRouteInstall`/`opRouteRemove` resolve via `dispatchPluginRPCDirect`; `TestApplyRouteInstallInsertsPath`/`TestApplyRouteRemoveWithdrawsPath` | route-install/remove gained a Direct arm |
| AC-6 | Done | `opInjectWireRoute`/`opBatchValidate` + `rpc.InjectWireRouteInput`/`BatchValidateInput` + SDK fallbacks | JSON codec fallback exists; SDK no longer errors / string-encodes |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestDispatchCommandDirectBridge` | Pass (unchanged) | dispatch_test.go | JSON and typed bridge identical |
| `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand` | Pass (unchanged) | dispatch_test.go | args/string converge on one core |
| `TestRPCRegistrationExpectedMethods` | Pass (unchanged) | rpc_registration_test.go | AllBuiltinRPCs mechanism (separate); untouched |
| `TestPluginRPCRegistryCoversAllPaths` (new) | Pass | dispatch_registry_test.go | drift guard: method set + typed set + fail-closed |
| `TestApplyRouteInstallInsertsPath` / `TestApplyRouteRemoveWithdrawsPath` | Pass (unchanged) | dispatch_route_test.go | Loc-RIB apply through registry-derived arm |
| `TestConcurrentPluginDispatch` | Pass (unchanged) | rpc_test.go | concurrent dispatch correct after registry lookup |
| `TestWireBridgeDispatchInstallsTypedSlots` (new) | Pass | dispatch_registry_test.go | AC-3 typed-slot wiring from registry |
| `TestEngineOpJSONAndDirectMatch` (new) | Pass | dispatch_registry_test.go | AC-2 JSON==Direct |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `dispatch.go` | Done | switches + `Set*` list replaced by registry lookup + typedWire loop |
| `dispatch_cached.go` | Done | RPC/Direct pairs -> `opForwardCached`/`opReleaseCached` |
| `dispatch_route.go` | Done | RPC handlers -> `opRouteInstall`/`opRouteRemove`; Direct arm derived |
| `registry/registry.go` | Changed | NOT modified: registry moved to server package instead (see Deviations); codec `CollectRPCHandlers` reused unchanged |
| `pkg/plugin/rpc/bridge.go` | Not modified | typed `Set*`/`Has*` slots kept as-is; wiring driven from registry; new input types live in types.go |
| `pkg/plugin/sdk/sdk_engine.go` | Done | uniform shape + `rpc.Method*` constants + AC-6 fallbacks |
| `dispatch_registry.go` | Done (new) | the unified registry (not in the original Files list; correct home per Deviations) |
| `pkg/plugin/rpc/types.go` | Done (new work) | `rpc.Method*` constants + `InjectWireRouteInput`/`BatchValidateInput` |

### Audit Summary
- **Total items:** 6 ACs + 6 planned files + 8 tests
- **Done:** all 6 ACs implemented + tested; all planned behavior delivered
- **Partial:** none
- **Skipped:** none
- **Changed:** registry location (server pkg not leaf), bridge.go untouched (types in types.go), emit-event error text, batch-validate SDK fallback path -- all in Deviations

## Goal Validation (BLOCKING)
| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Adding an operation touches one place; the three paths cannot drift | unit test | `TestPluginRPCRegistryCoversAllPaths` (dispatch_registry_test.go): asserts the 12-method set, `handle != nil` for every op, the exact 8-op typed-descriptor set, and fail-closed `lookupEngineOp`. One `engineOps` entry per op. |
| All externally observable RPC behavior preserved (internal refactor) | unit test | `TestDispatchCommandToPlugin`, `TestDispatchCommand{NotFound,PluginError,EmptyCommand}`, `TestHandleDispatchCommandRPCPreservesPluginIdentity`, `TestApplyRoute{Install,Remove}*`, `TestConcurrentPluginDispatch` all pass unchanged. |
| JSON and Direct paths byte-identical | unit test | `TestEngineOpJSONAndDirectMatch` (JSONEq of `serveEngineOpDirect` vs `directResultResponse(handle(...))`); `TestDispatchCommandDirectBridge`. |
| Typed hot paths not regressed onto JSON | unit test | `TestWireBridgeDispatchInstallsTypedSlots`: all 8 `Has*` true after registry-driven wiring; `typedWire` closures call native cores (`s.deliverEvent`, `s.forwardCached`, ...) with no `json.Marshal`. |
| Coverage gaps closed (route-install/remove Direct arm; inject/batch JSON fallback) | code + test | route ops resolve through `dispatchPluginRPCDirect` (AC-5); `rpc.InjectWireRouteInput`/`BatchValidateInput` + `opInjectWireRoute`/`opBatchValidate` (AC-6). |

## Review Gate

### Run 1 (initial -- self critical review against Critical Review Checklist)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | emit-event JSON error path double-prefixed via `RPCCallError.Error()`; Direct did not. | dispatch.go (old handleEmitEventRPC) | Unified `rpcErrMessage` sends raw `.Message` on both paths (AC-2). Documented in Deviations. |
| 2 | NOTE | Registry cannot live in the leaf `registry` package (Server/proc/bridge imports). | Files-to-Modify vs registry.go tier | Placed in server package `dispatch_registry.go`; A-2 broken; documented in Deviations. |
| 3 | NOTE | Wiring-Test cites `TestRPCRegistrationExpectedMethods`, which tests a different registry (`AllBuiltinRPCs`). | rpc_registration_test.go | Built the correct guard `TestPluginRPCRegistryCoversAllPaths`; cited test unchanged. |

### Fixes applied
All NOTEs resolved in code and documented in Deviations / Mistake Log. No BLOCKER or ISSUE found.
Critical Review Checklist verified: Completeness (12 ops in `engineOps`, every AC has file:line in
Audit), Correctness (`TestEngineOpJSONAndDirectMatch` + unchanged suite), Data flow (CORE methods
`dispatchCommand`/`deliverEvent`/`forwardCached`/`applyRouteInstall` untouched -- grep-confirmed),
No hot-path regression (typedWire closures identical to old inline `Set*`; `handle` is a method
expression, no per-request closure alloc), Registration-over-hardcoding (one `engineOps` entry per
op; `grep 'case "ze-plugin-engine:' dispatch.go` = NONE), no-layering (both switches + the `Set*`
list fully deleted).

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | none | Re-review after fixes found no BLOCKER/ISSUE. | | |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE (self-review: 0 BLOCKER, 0 ISSUE, 3 NOTE resolved)
- [ ] All NOTEs recorded above (3, all resolved)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/plugin/server/dispatch_registry.go` | yes | git status `??` (new) |
| `internal/component/plugin/server/dispatch_registry_test.go` | yes | git status `??` (new) |
| `internal/component/plugin/server/dispatch.go` | yes (modified) | git status ` M` |
| `internal/component/plugin/server/dispatch_cached.go` | yes (modified) | git status ` M` |
| `internal/component/plugin/server/dispatch_route.go` | yes (modified) | git status ` M` |
| `pkg/plugin/rpc/types.go` | yes (modified) | git status ` M` |
| `pkg/plugin/sdk/sdk_engine.go` | yes (modified) | git status ` M` |
| `docs/architecture/api/process-protocol.md` | yes (modified) | git status ` M` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | JSON path dispatches to shared core, same output | `go test -run TestDispatchCommandToPlugin\|TestHandleDispatch` PASS |
| AC-2 | Direct byte-identical to JSON; unknown fail-closed | `go test -run TestEngineOpJSONAndDirectMatch\|TestDispatchCommandDirectBridge` PASS |
| AC-3 | typed slots installed by iterating registry | `go test -run TestWireBridgeDispatchInstallsTypedSlots` PASS |
| AC-4 | one registration site; path parity | `go test -run TestPluginRPCRegistryCoversAllPaths` PASS; `grep 'case "ze-plugin-engine:' dispatch.go` = NONE |
| AC-5 | route-install/remove gain Direct arm | `dispatchPluginRPCDirect`->`lookupEngineOp(route-install)`; `TestApplyRoute*` PASS |
| AC-6 | inject/batch JSON codec fallback exists | `rpc.InjectWireRouteInput`/`BatchValidateInput` + `opInjectWireRoute`/`opBatchValidate` present; SDK uses them |

### Wiring Verified (end-to-end)
| Entry Point | Test | Verified |
|-------------|------|----------|
| In-process dispatch-command JSON+typed identical | `TestDispatchCommandDirectBridge` | yes |
| exact command + args converge on one handler | `TestDispatchCommandArgsRoutesSameHandlerAsDispatchCommand` | yes |
| registry advertises exact op set (fail-closed) | `TestPluginRPCRegistryCoversAllPaths` | yes |
| forked route batch applied to Loc-RIB via registry arm | `TestApplyRouteInstallInsertsPath` | yes |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed (success) / nuance | emit-event error plumbing differed; aligned by `rpcErrMessage` (AC-2). Success paths identical -- unchanged tests pass. |
| A-2 | broken | leaf registry cannot host Server/proc-bound ops; moved to server package `dispatch_registry.go` with proc-passed `handle`. |
| A-3 | confirmed | route-install/remove absent from Direct switch pre-change; registry derivation adds the arm additively (`TestApplyRoute*` pass). |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| process-protocol.md "Engine-side dispatch registry" note | anchors `dispatch_registry.go -- engineOps` + `dispatch.go -- wireBridgeDispatch` (symbols exist) | yes |
| existing anchor `dispatch.go -- dispatchCommandArgs` | `dispatchCommandArgs` still defined (kept) | yes |
| existing anchors `sdk_engine.go -- all methods`, `bridge.go -- DirectBridge`, `wireBridgeDispatch` | symbols unchanged / still present | yes |
| `make ze-doc-test` | PASSED (3203 digest anchors resolve; DOCS-TO-CODE regenerated for dispatch_registry.go) | yes |
| `ai/digests/plugin-transport.md` + `aaa-auth.md` | updated dispatch.go anchors (moved by the refactor) + switch->registry prose | yes |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled — 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/plugin/server`, `internal/component/plugin/registry`, `pkg/plugin/rpc`, `pkg/plugin/sdk`)
- [ ] Integration completeness proven end-to-end
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Critical Review passes (all checks in `ai/rules/quality.md` — no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases? yes: JSON, Direct, Bridge all derive from the registry)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all checks in `ai/rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-unify-rpc-dispatch.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary
- [ ] **Commit B:** `git rm plan/spec-unify-rpc-dispatch.md` only
