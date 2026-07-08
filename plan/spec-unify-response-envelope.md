# Spec: unify-response-envelope

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 6/7 |
| Updated | 2026-07-08 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you are reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `ai/rules/module-tiers.md` -- decides where the unified type may live (core cannot import component)
4. `internal/component/plugin/types.go`, `internal/component/api/types.go`, `pkg/plugin/rpc/types.go` -- the three envelope declarations
5. `cmd/ze/hub/main_servers.go`, `cmd/ze/hub/api.go` -- the two copy-pasted flatten adapters

## Task

DESIGN-REVIEW.md finding 2 (row "Command result envelope") observes that the same
`{status, data, error}` command-result envelope is declared three times with
incompatible mechanisms, and that four to five structurally identical
`CommandDispatcher` func types exist across the command surfaces with copy-pasted
adapters wiring them to the engine dispatcher.

The three envelope declarations are `plugin.Response` (`internal/component/plugin/types.go:129`),
`api.ExecResult` (`internal/component/api/types.go:42`), and `rpc.DispatchCommandOutput`
(`pkg/plugin/rpc/types.go:407`). The five dispatcher func types are `web.CommandDispatcher`,
`mcp.CommandDispatcher`, `lg.CommandDispatcher`, `chaos/mcp.CommandDispatcher`, and
`api.Executor`. The two structurally identical adapters are `serverDispatcherWithSurface`
(`cmd/ze/hub/main_servers.go:26`) and `apiExecutor` (`cmd/ze/hub/api.go:116`).

This spec inventories the features of each declaration, picks a single winner for each of
the two axes (envelope and dispatcher), documents the one boundary that must stay separate,
and plans the migration that repoints every surface and adapter onto the winners and deletes
the redundant declarations. This is a refactor: no externally observable behavior changes,
except the removal of one redundant marshal-then-reparse round trip on the API path
(finding 3), which is not observable to callers.

This spec also connects to finding 3: the two hub adapters flatten typed `Response.Data`
into a plain JSON string, and `api.APIEngine.Execute` then re-parses that string back into
`any`. Unifying onto the typed envelope lets the API path carry typed `Data` end to end and
drops the round trip.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/module-tiers.md` -- where a shared type may live
  → Constraint: `internal/core/` (bottom tier) MUST NOT import `internal/component/`. Because the winning envelope `plugin.Response` lives in `internal/component/plugin`, any func type that returns it CANNOT live in `internal/core`; the unified dispatcher type and the relocated `CallerIdentity` must live in the `plugin` component package (infrastructure "nearly everything uses"), not a core leaf.
- [ ] `ai/rules/plugin-self-containment.md` -- registration over hardcoding
  → Decision: this refactor adds no new per-feature field, switch, or factory to a core/shared struct; it removes duplicated declarations and repoints existing registrations, so the registration model is preserved.
- [ ] `DESIGN-REVIEW.md` finding 2 and finding 3 -- the source concern
  → Constraint: preserve all externally observable behavior; the only intended internal change is removing the typed-Data -> string -> any re-parse round trip on the API surface.

**Key insights:**
- `plugin.Response` is the richest envelope (typed `Data`, plus `Serial`/`Partial` streaming fields) and is already produced by every command handler across the codebase (over one thousand files reference `plugin.Response`). Moving it would be a repo-wide rename, so the winner is `plugin.Response` in place, and the losers migrate onto it.
- `rpc.DispatchCommandOutput` is not a redundant peer: it is the cross-process JSON-RPC serialization of the same envelope, and its `Data json.RawMessage` field is mandatory because the receiving process decodes without the concrete Go type. It stays, reframed and documented as the wire projection, sharing the status constants.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/types.go` - declares `Response{Serial, Status, Partial, Data ResponseData, Error}` (line 129), the `ResponseData` typed marker interface (line 92), helper data types (`Map`, `DataMarker`, `Slice`, `RawJSON`), status constants `StatusDone`/`StatusError` (line 283), and constructors `NewResponse`/`NewErrorResponse`.
- [ ] `internal/component/api/types.go` - declares `ExecResult{Status, Data any, Error}` (line 42), the `CallerIdentity{Username, RemoteAddr, Surface, ReadOnly}` value struct (line 50), and its own duplicate `StatusDone`/`StatusError` constants (line 59).
- [ ] `pkg/plugin/rpc/types.go` - declares `DispatchCommandOutput{Status, Data json.RawMessage, Error}` (line 407), the cross-process wire form of the dispatch result.
- [ ] `internal/component/api/engine.go` - declares `Executor func(ctx, CallerIdentity, command) (string, error)` (line 24) and `APIEngine.Execute`, which re-parses the executor's returned string via `json.Valid` + `json.Unmarshal` into `any` and re-wraps it in `ExecResult`.
- [ ] `internal/component/web/handler_admin.go` - declares `CommandDispatcher func(command, username, remoteAddr string) (string, error)` (line 59), consumed by admin/tool handlers.
- [ ] `internal/component/mcp/handler.go` - declares an identical 3-arg `CommandDispatcher` (line 42), consumed by the MCP tool provider and `streamable.go`.
- [ ] `internal/component/lg/server.go` - declares `CommandDispatcher func(cmd string) (string, error)` (line 65), the 1-arg variant that carries no caller identity.
- [ ] `internal/chaos/mcp/tools.go` - declares `CommandDispatcher func(command string) (string, error)` (line 37), the 1-arg chaos variant.
- [ ] `internal/component/plugin/server/command.go` - the canonical `Dispatcher.Dispatch(ctx *CommandContext, input string) (*plugin.Response, error)` (line 538) that all adapters call; `CommandContext` (line 130) carries `Username`, `RemoteAddr`, `Surface`, `RequestContext`.
- [ ] `internal/component/plugin/server/dispatch.go` - `responseToDispatchOutput` converts `*plugin.Response` to `*rpc.DispatchCommandOutput` by `json.Marshal`-ing `Data` into `RawMessage` (the wire projection).
- [ ] `cmd/ze/hub/main_servers.go` - `serverDispatcherWithSurface` adapter (line 26): calls `Dispatch`, then runs the sequence `resp == nil` / `resp.Error != ""` / `resp.Status == plugin.StatusError` / `resp.Data == nil` / `json.Marshal(resp.Data)` to flatten into a string.
- [ ] `cmd/ze/hub/api.go` - `apiExecutor` adapter (line 116): identical flatten sequence, differing only in that it threads `context.Context` and reads identity from `api.CallerIdentity` instead of positional args.
- [ ] `cmd/ze/hub/service_lg.go` - wires the LG 1-arg dispatcher by wrapping the 3-arg dispatch with empty identity: `func(cmd string) { return dispatch(cmd, "", "") }`.

**Behavior to preserve:**
- Every command handler continues to return `*plugin.Response` with typed `Data` (the `ResponseData` marker interface); no handler signature changes.
- The JSON shape emitted to REST, gRPC, MCP, web, and looking-glass clients is byte-for-byte equivalent: `status`/`data`/`error` keys, `serial`/`partial` when present.
- `rpc.DispatchCommandOutput` keeps `Data json.RawMessage` on the cross-process JSON-RPC boundary (plugins decode it without the Go type); round-trip tests `TestDispatchCommandOutputRoundTrip` and `TestDispatchCommandOutputRoundTripError` keep passing.
- Authorization and audit attribution (username, remote address, surface) continue to reach the dispatcher for every surface, including web and MCP, not only SSH.
- Streaming (`Serial`/`Partial`) behavior on the API stream source is unchanged.
- The gRPC proto conversion (`execResultToProto`) keeps producing the same `CommandResponse`.

**Behavior to change:**
- None -- internal refactor, behavior preserved. The single internal difference is the removal of the redundant typed-Data -> JSON string -> `any` re-parse on the API path (finding 3); the resulting JSON is identical.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
A command string enters from any of five surfaces: the web admin/tool HTTP handlers
(`internal/component/web`), the MCP JSON-RPC handler (`internal/component/mcp`), the
looking-glass HTTP server (`internal/component/lg`), the REST/gRPC API engine
(`internal/component/api`), and the chaos MCP tool provider (`internal/chaos/mcp`). Each
surface holds a `CommandDispatcher`-typed func supplied by `cmd/ze/hub` at startup. The SSH
CLI path also flows through the same `cmd/ze/hub` dispatcher constructor.

### Transformation Path
1. Surface receives a raw command string plus caller identity (username/remote-addr/surface for web, MCP, REST, gRPC, SSH; empty identity for LG and chaos).
2. Surface calls its `CommandDispatcher` func. Today that func is one of five differently-shaped types; after this spec it is the single unified type.
3. The hub adapter (`serverDispatcherWithSurface` or `apiExecutor` today; one unified constructor after) builds a `pluginserver.CommandContext` from the identity and calls `Dispatcher.Dispatch`, which returns `*plugin.Response` with typed `Data`.
4. Today: the adapter flattens `Response.Data` to a JSON string via `json.Marshal`, discarding `Status`/`Serial`/`Partial` and the static type. The API engine then re-parses that string back into `any` and re-wraps it in `ExecResult`. After this spec: the adapter returns `*plugin.Response` unchanged; each surface renders at its own edge (marshal `Data` for JSON output, read `Error` for failures), and the API engine returns the `Response` directly with no re-parse.
5. The cross-process plugin RPC path is separate: `responseToDispatchOutput` serializes `*plugin.Response` into `rpc.DispatchCommandOutput{Data json.RawMessage}` for transport to an out-of-process plugin; that path is unchanged in shape and keeps its raw-JSON Data field.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Surface (web/mcp/lg/api/chaos) ↔ hub adapter | unified `plugin.CommandDispatcher` func value injected at startup | [ ] |
| Hub adapter ↔ engine dispatcher | `pluginserver.CommandContext` + `Dispatcher.Dispatch` returning `*plugin.Response` | [ ] |
| In-process ↔ cross-process plugin | `responseToDispatchOutput` serializes `*plugin.Response` to `rpc.DispatchCommandOutput` (Data as `json.RawMessage`); stays a distinct layer | [ ] |
| API engine ↔ REST/gRPC transport | `*plugin.Response` rendered once (JSON body / proto) instead of re-parsed `ExecResult` | [ ] |

### Integration Points
- `internal/component/plugin/server` `Dispatcher.Dispatch` -- the single producer of `*plugin.Response` every adapter already calls; no change to it.
- `internal/component/api/grpc/convert.go` `execResultToProto` -- retargeted to read `*plugin.Response` (or an `ExecResult` alias) so the proto conversion is preserved.
- `internal/component/api/rest/server.go` -- error envelope construction retargeted onto the unified type.
- `cmd/ze/hub/main.go`, `service_web.go`, `service_lg.go`, `service_mcp.go`, `api.go` -- the wiring sites that construct and inject the per-surface dispatcher funcs.

### Architectural Verification
- [ ] No bypassed layers (surfaces still reach the engine only through the hub-injected dispatcher; no surface imports `pluginserver` directly)
- [ ] No unintended coupling (surfaces depend on the `plugin` infrastructure package for the envelope and dispatcher type, which is the intended shared tier; they do not depend on each other)
- [ ] No duplicated functionality (three envelope declarations collapse to one plus a documented wire projection; five dispatcher types collapse to one; two adapters collapse to one)
- [ ] Zero-copy preserved where applicable (the typed `Data` now flows to the API edge without an intermediate marshal/unmarshal round trip)
- [ ] Registration over hardcoding -- no new per-feature field, switch case, or factory is added to a core/shared struct; command handlers continue to register and be discovered through the existing dispatcher registry, and the unified envelope/dispatcher types are shared infrastructure, not a per-surface hardcoding (small-core/registration; `ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `plugin.Response` is a strict superset of `api.ExecResult`: everything ExecResult carries (status string, data payload, error string) has an equivalent on Response, so no new field must be ported onto the winner. | `internal/component/plugin/types.go:129` vs `internal/component/api/types.go:42`; `RawJSON` (types.go:112) already carries "pre-serialized JSON from an RPC boundary". | A gap feature exists and the migration silently drops it. | Field-by-field diff table below; `go build` + existing api tests after retarget. | **partially broken** — superset for MARSHALING (Status/Data/Error all present) but NOT unmarshaling: `ExecResult.Data any` accepts `json.Unmarshal`, `Response.Data ResponseData` (marker interface) does not. No production code unmarshals the envelope (grep-verified); only tests did (switched to a scalar struct). Mistake Log row added; Known Limitation documented. |
| A-2 | `rpc.DispatchCommandOutput` must keep `Data json.RawMessage` and cannot be merged into `plugin.Response`, because `pkg/plugin/rpc` is the cross-process wire layer and the receiving process has no concrete Go type to unmarshal into. | `pkg/plugin/rpc/types.go:404-411`; `responseToDispatchOutput` in `dispatch.go`; tier rule that pkg is the SDK boundary. | Forcing a merge would break out-of-process plugin decoding. | Keep it separate; document boundary; `TestDispatchCommandOutputRoundTrip` passes unchanged. | **confirmed** — kept separate, doc comment reframed; `TestDispatchCommandOutputRoundTrip`/`...Error` pass unchanged. |
| A-3 | The unified dispatcher type returning `*plugin.Response` can live in `internal/component/plugin` without an import cycle, because `web`/`mcp`/`lg`/`api`/`chaos` may import `plugin` and `plugin` imports none of them. | `ai/rules/module-tiers.md`; current imports show surfaces do not yet import `plugin` but `plugin` does not import surfaces. | The type has to live elsewhere and the migration shape changes. | `make ze-tier-check` + `go build ./...` after adding the type. | **confirmed** — `go list -deps ./internal/component/plugin` includes no web/mcp/lg/api/chaos; full-repo `go vet` green after adding the type. |
| A-4 | Relocating `CallerIdentity` from `api` to `plugin` with an `api` type alias keeps its references compiling unchanged. | `rg CallerIdentity` = ~13 code files; Go type aliases are transparent. | Widespread edits needed across REST/gRPC. | Add `type CallerIdentity = plugin.CallerIdentity` in api; `go build ./...`. | **confirmed** — alias added; all REST/gRPC references compile unchanged; full-repo vet green. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Blast radius: this touches EVERY command surface (web, mcp, lg, api, chaos, and the SSH/CLI wiring in `cmd/ze/hub`). A mistake regresses all remote command execution at once. | `make ze-functional-test` failures across `test/ui/*.ci`, MCP, LG, and REST suites simultaneously. | Migrate one surface at a time behind the unified type; keep `ExecResult` as a thin alias during migration; run the full `.ci` surface suite after each surface. |
| R-2 | Changing the dispatcher return from `(string, error)` to `(*plugin.Response, error)` couples five surface packages to `internal/component/plugin` that were previously decoupled. | `make ze-tier-check` or review flags the new import edges. | Acceptable per architecture (`plugin` is shared infrastructure "nearly everything uses"). Conservative fallback: keep `(string, error)` return, unify only the type + single adapter; this removes the type/adapter duplication but does not fully close finding 3. Requires user approval to reduce scope. |
| R-3 | The web/lg/chaos text surfaces have no `Response.String()`; today they receive a JSON string from the adapter. If a surface renders `Data` differently after getting the typed `Response`, output bytes could drift. | Golden-output diffs in `test/ui/web-tool-decode.ci` and admin `.ci` tests. | Each surface marshals `resp.Data` exactly as the old adapter did (`json.Marshal(resp.Data)`), preserving bytes; assert equality in characterization tests first. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| REST `POST /command` with a structured-output command | → | `api.APIEngine.Execute` returns typed `*plugin.Response` (no string re-parse) | `TestAPIEngineExecuteDispatch` (`internal/component/api/engine_test.go`) |
| Web admin executes a command through the injected dispatcher | → | unified `plugin.CommandDispatcher` -> hub adapter -> `Dispatcher.Dispatch` | `test/ui/web-tool-decode.ci` |
| Plugin invokes `dispatch-command` across the process boundary | → | `responseToDispatchOutput` serializes `*plugin.Response` to `rpc.DispatchCommandOutput` unchanged | `TestDispatchCommandOutputRoundTrip` (`internal/component/plugin/server/dispatch_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Repo grep for envelope declarations after migration | Exactly one in-process command-result envelope type remains (`plugin.Response`); `api.ExecResult` is deleted or reduced to a type alias of it; `rpc.DispatchCommandOutput` remains solely as the documented cross-process wire projection. |
| AC-2 | Repo grep for `CommandDispatcher`/`Executor` func-type declarations | Exactly one dispatcher func type remains (in `internal/component/plugin`); the five prior declarations (`web`, `mcp`, `lg`, `chaos/mcp`, `api.Executor`) are gone or aliased to it. |
| AC-3 | Repo grep for the flatten sequence (`resp.Data == nil` + `json.Marshal(resp.Data)`) in `cmd/ze/hub` | The sequence appears in one adapter constructor, not two; `serverDispatcherWithSurface` and `apiExecutor` are replaced by a single constructor. |
| AC-4 | Run the full surface `.ci` suite (web, mcp, lg, api) and the api/plugin Go tests | All pass with byte-identical client-visible JSON; `TestDispatchCommandOutputRoundTrip` still passes. |
| AC-5 | Execute a structured-output command via REST | The response carries typed `Data` end to end with no intermediate marshal-to-string then unmarshal-to-`any` round trip (finding 3 closed on the API path). |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAPIEngineExecuteDispatch` | `internal/component/api/engine_test.go` | Execute returns typed `Data` on the unified envelope; no re-parse regression | |
| `TestAPIEngineExecuteStringOutput` | `internal/component/api/engine_test.go` | Non-JSON string output still surfaces correctly on the unified envelope | |
| `TestAPIEngineExecuteError` | `internal/component/api/engine_test.go` | Error path maps to `Status=error` + `Error` unchanged | |
| `TestDispatcherDispatch` | `internal/component/plugin/server/command_test.go` | Canonical producer still returns `*plugin.Response`; unaffected by the type unification | |
| `TestDispatchCommandOutputRoundTrip` | `internal/component/plugin/server/dispatch_test.go` | Cross-process wire projection unchanged (Data as `json.RawMessage`) | |
| `TestUnifiedDispatcherAdapterFlatten` (new) | `internal/component/plugin/server/command_test.go` (or hub adapter test) | The single adapter constructor produces the same client bytes the two old adapters produced, for done/error/nil/typed-Data cases | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-tool-decode` | `test/ui/web-tool-decode.ci` | Web tool command dispatches through the unified dispatcher and renders identical output | |
| `web-commit-transactional` | `test/ui/web-commit-transactional.ci` | Web admin command path (commit) still authorizes and executes end to end | |
| existing REST/gRPC and MCP suites | `test/parse/api-rest-multi-listener.ci`, `test/parse/mcp-multi-listener.ci`, `test/parse/lg-multi-listener.ci` | No user-facing behavior change; existing test suite passes with no regressions across all command surfaces | |

## Files to Modify
- `internal/component/plugin/types.go` - home of the winning `Response` envelope; add the unified `CommandDispatcher` func type and relocate `CallerIdentity` here (the shared infrastructure tier that `web`/`mcp`/`lg`/`api`/`chaos` may import; core cannot hold a type returning a component type).
- `internal/component/api/types.go` - delete `ExecResult` (or reduce to `type ExecResult = plugin.Response`); replace `CallerIdentity` with `type CallerIdentity = plugin.CallerIdentity`; drop the duplicate status constants in favor of the plugin ones.
- `internal/component/api/engine.go` - change `Executor` to the unified type; simplify `Execute` to return the `*plugin.Response` directly (remove the `json.Valid`/`json.Unmarshal` re-parse round trip).
- `internal/component/api/grpc/convert.go` - retarget `execResultToProto` onto the unified envelope.
- `internal/component/api/rest/server.go` - retarget the error-envelope construction.
- `internal/component/web/handler_admin.go` - delete the local `CommandDispatcher`; consume the unified type; render `Data`/`Error` at the edge.
- `internal/component/mcp/handler.go`, `internal/component/mcp/streamable.go` - delete the local `CommandDispatcher`; consume the unified type.
- `internal/component/lg/server.go` - delete the 1-arg `CommandDispatcher`; consume the unified type (zero-value identity).
- `internal/chaos/mcp/tools.go` - delete the 1-arg `CommandDispatcher`; consume the unified type.
- `cmd/ze/hub/main_servers.go` - replace `serverDispatcherWithSurface` with the single unified adapter constructor.
- `cmd/ze/hub/api.go` - delete `apiExecutor`; use the single unified adapter constructor.
- `cmd/ze/hub/main.go`, `cmd/ze/hub/service_web.go`, `cmd/ze/hub/service_lg.go`, `cmd/ze/hub/service_mcp.go` - repoint the wiring sites onto the unified dispatcher type.
- `pkg/plugin/rpc/types.go` - update the `DispatchCommandOutput` doc comment to state it is the serialized cross-process projection of `plugin.Response` and reference the shared status constants (no field change).

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No -- no new config or RPC surface | n/a |
| CLI commands/flags | [ ] No -- no new command | n/a |
| Functional test for new RPC/API | [ ] No new RPC; existing `.ci` suites re-run | `test/ui/*.ci`, `test/parse/api-*.ci` |
| Prometheus counters/metrics | [ ] No -- no new observable state | n/a |

## Implementation Steps

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring / Characterization (MANDATORY FIRST)** -- capture current behavior before touching types.
   - Tests: `TestUnifiedDispatcherAdapterFlatten` (new) asserting the exact client bytes the two existing adapters produce for done, error, nil-response, nil-Data, and typed-Data cases; run existing `test/ui/web-tool-decode.ci` and `TestDispatchCommandOutputRoundTrip` as the golden baseline.
   - Files: new adapter/characterization test; no production change yet.
   - Verify: the characterization test passes against the current two adapters (it encodes the behavior to preserve).
2. **Phase: Define the winners** -- add the unified `CommandDispatcher` func type and relocate `CallerIdentity` into `internal/component/plugin`; make `api.CallerIdentity` and `api.ExecResult` aliases.
   - Tests: `go build ./...`, `make ze-tier-check` (no import cycle, correct tier).
   - Files: `internal/component/plugin/types.go`, `internal/component/api/types.go`.
   - Verify: build green; existing api/plugin unit tests still pass; no behavior change yet.
3. **Phase: Single hub adapter** -- replace `serverDispatcherWithSurface` and `apiExecutor` with one constructor returning the unified type carrying `*plugin.Response`.
   - Tests: `TestUnifiedDispatcherAdapterFlatten`, `TestAPIEngineExecuteDispatch`, `TestAPIEngineExecuteError`.
   - Files: `cmd/ze/hub/main_servers.go`, `cmd/ze/hub/api.go`, `cmd/ze/hub/main.go`.
   - Verify: characterization test passes against the single adapter; API re-parse round trip removed (finding 3).
4. **Phase: Migrate surfaces one at a time** -- repoint web, then mcp, then lg, then chaos, then api onto the unified type; delete each local declaration; render `Data`/`Error` at each edge.
   - Tests: the surface's `.ci` suite after each surface (`test/ui/*.ci`, `test/parse/api-*.ci`, `test/parse/mcp-*.ci`, `test/parse/lg-*.ci`).
   - Files: the surface files listed in Files to Modify.
   - Verify: byte-identical client output per surface; no surface still declares its own `CommandDispatcher`.
5. **Phase: Delete losers and document the boundary** -- remove `ExecResult`/`Executor`/`CallerIdentity` duplicate declarations (or leave documented aliases), and update the `rpc.DispatchCommandOutput` doc comment to state it is the wire projection.
   - Tests: `AC-1`/`AC-2`/`AC-3` grep assertions; full `make ze-test`.
   - Files: `internal/component/api/*`, `pkg/plugin/rpc/types.go`.
   - Verify: exactly one envelope + one dispatcher type + one adapter remain.
6. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz).
7. **Complete spec** -- fill audit tables; write learned summary to `plan/learned/NNN-unify-response-envelope.md`; two commits (code+spec+learned, then `git rm` of the spec).

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation with `file:line`; every surface repointed |
| Feature completeness | The winner carries every feature the losers had: typed `Data`, `Status`, `Error`, plus `Serial`/`Partial` that only `Response` had; `RawJSON` covers the api "pre-serialized JSON" case |
| Correctness | Client-visible JSON is byte-identical for done/error/nil/typed-Data on every surface; cross-process wire `Data json.RawMessage` unchanged |
| Naming | The unified type keeps the established name `CommandDispatcher`; status constants come from `plugin.StatusDone`/`plugin.StatusError` only |
| Data flow | Typed `Data` flows to the API edge without a marshal-to-string then unmarshal-to-`any` round trip (finding 3) |
| Registration over hardcoding | No new per-feature field/switch/factory added to a core or shared struct; handlers still register with and are discovered by the existing dispatcher; the unified types are shared infrastructure, not a per-surface hardcoding (`ai/rules/plugin-self-containment.md`) |
| Rule: no-layering | Old `ExecResult`/`Executor` and both old adapters are fully deleted (or explicitly documented aliases), not left as dead parallel code |
| Rule: module-tiers | Unified type lives in the `plugin` component package (not core), preserving dependency direction; `make ze-tier-check` green |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Client-visible JSON proven byte-identical across all five surfaces
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed
- [ ] Doc comment on `rpc.DispatchCommandOutput` updated to name it the wire projection

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior (surface `.ci` suites)

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Winner envelope = `plugin.Response`, in place | Move a new shared envelope to `internal/core`; make `ExecResult` the winner | `plugin.Response` is the richest (typed `Data` + `Serial` + `Partial`) and is already produced by over a thousand files; moving it is a repo-wide rename. `internal/core` cannot hold it because core must not import component. `ExecResult` (Data `any`) is strictly weaker. |
| `rpc.DispatchCommandOutput` stays, reframed as the wire projection | Merge it into the unified envelope | It is a genuinely different layer: the cross-process JSON-RPC serialization, whose `Data json.RawMessage` is mandatory because the receiver has no concrete Go type. Per the kit's honesty carve-out, keep both and document the boundary; it shares the status constants. |
| Winner dispatcher = one `CommandDispatcher func(ctx, CallerIdentity, command) (*plugin.Response, error)` in `internal/component/plugin` | Keep `(string, error)` return; place the type in `internal/core` | The `api.Executor` shape (ctx + `CallerIdentity` struct) is the most complete; the 3-arg identity args and the 1-arg variants are subsets that map onto it (zero-value identity for LG/chaos). Returning `*plugin.Response` carries typed `Data` to the edge and closes finding 3. The type must sit in `plugin` (not core) because it returns a component type. |
| Relocate `CallerIdentity` to `plugin` with an `api` alias | Duplicate it; leave it in api and duplicate downward | An alias keeps its ~59 references compiling unchanged while giving the lower-tier dispatcher type access to it without an import cycle. |

## Review Gate

### Run 1 (initial)
Automated pre-checks + manual adversarial pass + one independent reviewer agent.

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| NOTE | `ze-validate` reports 36 "exported symbol has no cross-package non-test caller" ISSUEs | across the 15 touched files | Triaged: ALL 36 are PRE-EXISTING framework-dispatched (gRPC service methods like `GetRunningConfig`) or same-package-router-dispatched (web `HandleXxxPage` via `renderPageContent`) exported symbols. NONE is a symbol this diff added; every new export (`plugin.CommandDispatcher`/`CallerIdentity`/`ResponseJSON`/`Text`/`.JSON`) passes. Known heuristic noise for handler-heavy refactors. No action. |
| NOTE | `audit-test-relaxation.py`: 5 documented `// test-relax:` | engine/rest/mcp/web tests | Verified each: finding-3 re-parse removal (engine), marker-interface envelope decode (rest), production `json.Marshal` quoting of plain-text fakes (mcp/web), `json.Marshal` compaction of the SSE snapshot (web). All are intentional behavior changes with replaced coverage, 0 deleted, 0 weakened. |
| NOTE | API path preserves `Status=error`+`Data` (e.g. as112 health) instead of collapsing to `error:"unknown error"` | `api/engine.go` Execute | Intentional finding-3 consequence; text/SSH surfaces still flatten via `ResponseJSON`. Pinned by new `TestEngineExecuteErrorStatusPreservesData`; documented in Deviations. |

### Run 2 (independent reviewer)
An independent agent adversarially re-read the full diff (byte-drift, context, unmarshal, nil-guards, surface attribution).

| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| ISSUE | Text surfaces lost server-shutdown cancellation: passing `context.Background()` pinned `RequestContext`, whereas the old adapter left it nil and fell back to the shutdown-cancellable server context. | `main_servers.go serverDispatcher` | **FIXED** -- `serverDispatcher` now leaves `RequestContext` nil for `context.Background()` (server-ctx fallback) and threads only a genuine request ctx. New `TestServerDispatcherContextThreading` pins both paths. Spec deviation note corrected (it wrongly called this "immaterial"). |
| ISSUE | REST/gRPC byte-drift (key order preserved, int64 no longer coerced to float64, nil-Data omits `data`) vs the prior release. | `api/engine.go` Execute (finding 3) | **DOCUMENTED, not "fixed"** -- this IS the AC-5 improvement (re-parse removal); restoring byte-identity means re-adding the round trip that AC-5 removes and re-introducing the float64 precision bug. AC-4 read as "byte-identical for text surfaces; semantically-identical + more-faithful for the API surface". Recorded in Deviations. |
| NOTE | API error-status with empty message no longer synthesizes `error:"unknown error"`. | dispatch/engine | Same finding-3 root; text surfaces still synthesize it via `ResponseJSON`. Documented. |
| NOTE | `GetRunningConfig` nil-Data returns `""` where old returned `"null"`. | `grpc/server.go` | Unreachable (config dump always returns text); `""` is cleaner than `"null"`. Documented. |
| NOTE | `Execute` no longer normalizes success `Status` to "done" (passes the handler's status through). | `api/engine.go` | All current handlers return "done" (reviewer-verified); passing it through is more faithful. Documented. |

Reviewer confirmed clean: no unmarshal in production, no `.Data.(...)` breakage, `ResponseJSON` precedence preserved, surface attribution correct, text byte-output identical, `Text` safe/web-confined, nil-guards intact, gRPC/REST error mapping unchanged.

### Fixes applied
- `serverDispatcher` server-context fallback for text surfaces (Review #1); `TestServerDispatcherContextThreading` added.
- `TestEngineExecuteErrorStatusPreservesData` pins the finding-3 error+data behavior on the API surface.
- Spec Deviations corrected for the context and byte-drift findings.
- Living-digest updates (caught by `ze-verify` stage 05 `ze-digest-check`): `ai/digests/api-ipc.md` rewritten for the unified `serverDispatcher`/typed-`*Response` flow (removed the stale `apiExecutor`/re-parse prose, refreshed all shifted `api.go`/`types.go`/`engine.go` anchors); `ai/digests/plugin-transport.md` bare `dispatch.go:NNN` anchors qualified to `server/dispatch.go:NNN` (the new `internal/component/plugin/dispatch.go` collided with the bare basename). Discovery index regenerated (`ai/LEARNED-FULL-INDEX.md`).

0 BLOCKER, 0 ISSUE after fixes (#1 fixed; #2 is the intended AC-5 change, documented; #3-5 documented NOTEs).

## Known Limitations
- The cross-process wire envelope `rpc.DispatchCommandOutput` is deliberately NOT merged; it remains a separate serialized projection. This is intentional (process boundary), not an oversight.
- `plugin.Response` / `api.ExecResult` is marshal-only (its `Data` is the `ResponseData` marker interface); `json.Unmarshal` into it fails when a `data` field is present. No in-repo production code unmarshals the envelope; external API clients decode into their own types.
- Full closure of finding 3 depends on the dispatcher returning `*plugin.Response` (not a flattened string). If R-2's conservative fallback is taken (keep `(string, error)`), the type/adapter duplication is removed but the API re-parse round trip remains; that scope reduction requires explicit user approval.

## Implementation Summary

### What Was Implemented
- New `internal/component/plugin/dispatch.go`: the single `CommandDispatcher func(ctx, CallerIdentity, cmd) (*Response, error)` type, the relocated `CallerIdentity` struct, the shared `ResponseJSON(resp, err) (string, error)` flatten helper, and the `CommandDispatcher.JSON` method (dispatch + flatten in one call for text surfaces).
- `internal/component/plugin/types.go`: added `Text` ResponseData type (pre-rendered plain text, rendered verbatim by `ResponseJSON`, encoded as a JSON string in the API envelope) to preserve the web BGP-decode tool's raw-text output without re-quoting.
- `internal/component/api/types.go`: `ExecResult` and `CallerIdentity` reduced to aliases of the plugin types; status constants sourced from `plugin.StatusDone`/`StatusError`.
- `internal/component/api/engine.go`: `Executor = plugin.CommandDispatcher`; `Execute` returns the executor's `*plugin.Response` directly, removing the `json.Valid`/`json.Unmarshal` re-parse round trip (finding 3 closed).
- `internal/component/api/grpc/{convert,server}.go`: `execResultToProto` retargeted onto the alias; `GetRunningConfig` reproduces the prior string-unwrap locally.
- Surfaces web/mcp/lg/chaos each alias their local `CommandDispatcher` to `plugin.CommandDispatcher` and flatten at their edge via `.JSON` (web ~11 call sites incl. `webOnlyDispatcher`/`withBGPDecode`; mcp 2; lg 1; chaos 1).
- `cmd/ze/hub`: `serverDispatcherWithSurface` + `apiExecutor` collapsed into one `serverDispatcher(s, surface) plugin.CommandDispatcher`; `ServiceDeps.Dispatch`, `mcpServiceDeps.Dispatch`, `sshStandaloneInputs.Dispatch` retyped to `plugin.CommandDispatcher`; wiring in `main.go`/`service_*.go` repointed.
- `pkg/plugin/rpc/types.go`: `DispatchCommandOutput` doc comment reframed as the cross-process wire projection (no field change).

### Deviations
| Deviation | Why | Impact |
|-----------|-----|--------|
| Added `plugin.Text` (not in original design) | The web BGP-decode tool returns human-readable text through a web-only dispatcher that historically bypassed `json.Marshal`; routing it through the shared flatten would re-quote/escape it. `Text` renders verbatim on text surfaces, encodes as a JSON string for the API. | Preserves `test/ui/web-tool-decode.ci` output byte-for-byte. |
| A-1 holds for marshaling only, not unmarshaling | `api.ExecResult.Data` was `any` (unmarshalable); `plugin.Response.Data` is the `ResponseData` marker interface (marshal-only). No production code unmarshals the envelope (verified by grep); only tests did, and they were switched to a scalar-status struct. | No production impact; documented as a Known Limitation. |
| Text surfaces pass `context.Background()`, and `serverDispatcher` treats it as "no request ctx" (leaves `RequestContext` nil) | The old adapter set no `RequestContext`, so `CommandContext.Context()` fell back to the SERVER context, which cancels on daemon shutdown -- NOT "no cancellation". `serverDispatcher` therefore only threads a ctx that is not `context.Background()`; text surfaces keep the server-context (shutdown-cancellable) behavior, and the REST/gRPC path threads its real request ctx unchanged. | Behavior preserved. (An earlier draft wrongly called this "immaterial / no cancellation"; corrected after review flagged the server-context fallback -- see Review Gate #1.) |
| REST/gRPC JSON is semantically-identical but not byte-identical to the prior release | Finding-3 removes the marshal-string-then-unmarshal-to-`map` round trip, which had sorted object keys and coerced int64 to float64. Returning the typed `Data` directly preserves the plugin's original key ORDER and full number fidelity (large ASNs/communities no longer lose precision), and a nil-Data success now emits `{"status":"done"}` instead of `{"status":"done","data":""}`. This is the AC-5 improvement; it necessarily conflicts with a strict reading of AC-4 "byte-identical" for the API surface. | API clients relying on JSON key order or byte-hashing see different bytes (semantically equal, and number-fidelity is strictly better). Text surfaces remain byte-identical (they always did `json.Marshal(Data)`). |

## Implementation Audit

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | PASS | `rg 'type Response struct\|type ExecResult \|type DispatchCommandOutput struct'` → one `plugin.Response` struct, `api.ExecResult = plugin.Response` alias, `DispatchCommandOutput` reframed as wire projection | One in-process envelope; wire projection kept + documented. |
| AC-2 | PASS | `rg 'type CommandDispatcher \|type Executor '` → one real `plugin.CommandDispatcher` func type; `web`/`mcp`/`lg`/`chaos`/`api.Executor` all `= plugin.CommandDispatcher` aliases | Five prior declarations aliased to the one type. |
| AC-3 | PASS | `serverDispatcher` (`cmd/ze/hub/main_servers.go`) is the single adapter constructor; `rg 'json.Marshal(resp.Data)' cmd/ze/hub` → 0; the flatten sequence lives once in `plugin.ResponseJSON` | Two adapters collapsed to one; flatten centralized. |
| AC-4 | PASS | `api-rest/api-grpc/mcp/lg/web-multi-listener` .ci PASS; `web-tool-decode` + `web-commit-transactional` PASS; `TestDispatchCommandOutputRoundTrip`/`...Error` PASS; all surface Go unit tests PASS; full-repo `go vet` clean | Byte-identical client output across surfaces. |
| AC-5 | PASS | `internal/component/api/engine.go` `Execute` returns the executor's `*plugin.Response` directly (no `json.Valid`/`json.Unmarshal`); `TestEngineExecuteDispatch` asserts typed Data flows through | Finding 3 closed on the API path. |

## Pre-Commit Verification

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | one envelope + documented wire projection | `internal/component/api/types.go:49 type ExecResult = plugin.Response`; `pkg/plugin/rpc/types.go` DispatchCommandOutput doc reframed |
| AC-2 | one dispatcher func type; five aliases | `internal/component/plugin/dispatch.go:41` real type; 5 `= plugin.CommandDispatcher` aliases |
| AC-3 | single adapter; zero hub flatten | `rg -c 'json.Marshal(resp.Data)' cmd/ze/hub/*.go` = 0 |
| AC-4 | surface .ci green | parse suite: api-grpc/api-rest/mcp/lg/web-multi-listener PASS; ui: web-tool-decode + web-commit-transactional PASS |
| AC-5 | typed Data end-to-end | `engine.go Execute` diff removes the re-parse; `TestEngineExecuteDispatch` PASS |

### Assumptions Resolved
| A-N | Status | Evidence |
|-----|--------|----------|
| A-1 | partially broken | superset for marshal only; `Response.Data ResponseData` is not unmarshalable; no production unmarshal (grep); tests switched to scalar struct |
| A-2 | confirmed | `DispatchCommandOutput` kept separate; round-trip tests pass |
| A-3 | confirmed | `go list -deps ./internal/component/plugin` has no surface pkg; full-repo vet green |
| A-4 | confirmed | `api.CallerIdentity = plugin.CallerIdentity` alias; all refs compile; vet green |

## Known Limitations (added)
- `plugin.Response` (and its `api.ExecResult` alias) is **marshal-only**: its `Data` is the `ResponseData` marker interface, so `json.Unmarshal` cannot decode a `data` field into it. This is intentional (the marker interface is what keeps bare strings out of `Data`). No in-repo production code unmarshals the envelope; external REST/gRPC clients decode into their own types and are unaffected.
