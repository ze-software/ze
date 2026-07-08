# 1082 -- unify-response-envelope

## Context
The `{status, data, error}` command-result envelope was declared three times with
incompatible mechanisms (`plugin.Response` with typed `Data`, `api.ExecResult` with
`Data any`, `rpc.DispatchCommandOutput` with `Data json.RawMessage`), and the same
dispatcher shape was declared five times (`web`/`mcp`/`lg`/`chaos.CommandDispatcher`,
`api.Executor`) with two copy-pasted flatten adapters in `cmd/ze/hub`
(`serverDispatcherWithSurface`, `apiExecutor`). DESIGN-REVIEW findings 2 and 3.
The goal: one in-process envelope, one dispatcher type consumed by every surface,
one adapter, and typed `Data` carried to the REST/gRPC edge without a
marshal-to-string-then-reparse round trip (finding 3).

## Decisions
- Winner envelope = `plugin.Response` in place, over moving a new envelope to
  `internal/core` (core cannot import component) or making `api.ExecResult` (Data
  `any`, strictly weaker) the winner. `ExecResult`/`CallerIdentity`/`Executor`
  reduced to aliases; five surface `CommandDispatcher` types aliased to
  `plugin.CommandDispatcher`. Chose ALIASES over renaming so only the ~15 dispatcher
  INVOCATION sites changed, not the ~31 web signatures that merely thread it.
- Flatten centralized once in `plugin.ResponseJSON` + `CommandDispatcher.JSON`, over
  leaving it inlined at each edge. Text surfaces call `.JSON`; the API engine returns
  the typed `*plugin.Response` directly (finding 3 closed).
- Added `plugin.Text` (pre-rendered plain text): renders verbatim on text surfaces,
  encodes as a JSON string in the API. Needed because the web BGP-decode tool returned
  raw text through a web-only dispatcher that historically bypassed `json.Marshal`;
  routing it through the shared flatten would have re-quoted/escaped it.
- `rpc.DispatchCommandOutput` kept separate (not merged), reframed in its doc comment
  as the cross-process wire projection whose `Data json.RawMessage` is mandatory.

## Consequences
- Every surface now depends on `internal/component/plugin` for the envelope+dispatcher
  type; this is intended (plugin is shared infrastructure). `ServiceDeps.Dispatch` and
  `mcpServiceDeps.Dispatch` are typed `plugin.CommandDispatcher` -- still infra, so the
  feature-gate registry names no *service* package type.
- `serverDispatcher(s, surface)` uses `caller.Surface` when set (REST/gRPC set it per
  request) else the fixed `surface` (web/ssh/mcp/cli). One constructor covers both old
  adapters' audit-attribution behaviors.
- The API path (REST/gRPC) now carries a `Status=error` response's diagnostic `Data`
  to the client (e.g. as112 health `{healthy:false}`) instead of collapsing it to
  `error:"unknown error"`. Text/SSH surfaces still flatten error responses to the Go
  error via `ResponseJSON`, so only the API surface gains this. This is the
  finding-3-intended consequence of returning typed `Data` end to end.

## Gotchas
- `plugin.Response.Data` is the `ResponseData` marker interface -> the envelope is
  **marshal-only**: `json.Unmarshal` into `plugin.Response`/`api.ExecResult` FAILS when
  a `data` field is present. No production code unmarshals the envelope (grep-verified);
  tests that did were switched to a scalar-status struct. Assumption A-1 ("strict
  superset") holds for marshaling only.
- Test fakes returning a JSON string wrap it as `plugin.RawJSON(S)` -> the surface
  renders byte-identically (`json.Marshal(RawJSON(validJSON)) == validJSON`, then the
  outer `json.Marshal` COMPACTS it, matching the old adapter). But a fake returning
  NON-JSON plain text now marshals to a QUOTED JSON string `"text"`; ~4 assertions
  across api/mcp/web tests were updated to the quoted form (documented `// test-relax:`).
- The SSE snapshot writer (`sse_snapshot.go`) still continues embedded newlines as
  fresh `data:` lines (defense in depth), but `dispatch.JSON`'s `json.Marshal` now
  compacts insignificant whitespace before the payload reaches it, so a JSON payload
  can no longer carry a raw newline to the SSE writer. `plugin.Text` is the only path
  that reaches a text surface with raw newlines, and it is not used by SSE views.
- CONTEXT TRAP: the old `serverDispatcherWithSurface` never set `CommandContext.RequestContext`, so `CommandContext.Context()` fell back to the SERVER context (cancels on daemon shutdown) -- NOT "no cancellation". A first attempt passed `context.Background()` as the request ctx, silently dropping shutdown cancellation for in-flight web/mcp/lg/ssh/cli commands. Fix: `serverDispatcher` threads the ctx only when it is not `context.Background()`; text surfaces pass Background and keep the nil-RequestContext server-ctx fallback, the API path threads its real request ctx. Verify context-fallback claims against `CommandContext.Context()`, not intuition.
- BYTE-DRIFT: removing the API re-parse (finding 3) changes REST/gRPC JSON key ORDER (plugin order, not sorted) and number fidelity (int64 no longer coerced to float64) and drops `"data":""` on nil-Data. Semantically equal and more faithful, but NOT byte-identical -- AC-4 "byte-identical" holds only for text surfaces; the API surface is semantically-identical. Do not "fix" this by re-adding the round trip (it re-introduces the float64 precision loss).
- `cli-show-version.ci` fails when `ze` is built via `make bin/ze` (stamps CalVer
  `-X main.version=$(date +%y.%m.%d)`); it expects the `dev` default. Build with
  `-ldflags "-X main.version=dev"` for that test. Orthogonal to this refactor.

## Files
- `internal/component/plugin/dispatch.go` (new), `internal/component/plugin/types.go` (Text)
- `internal/component/api/{types,engine}.go`, `internal/component/api/grpc/{convert,server}.go`, `internal/component/api/rest/server.go`
- `internal/component/web/*` (alias + ~10 flatten sites), `internal/component/mcp/handler.go`, `internal/component/lg/server.go`, `internal/chaos/mcp/tools.go`
- `cmd/ze/hub/{main,main_servers,api,service_registry,service_web,service_lg,service_mcp,service_ssh,ssh_infra}.go`
- `pkg/plugin/rpc/types.go` (doc comment)
- Tests: `internal/component/plugin/dispatch_test.go` (new) + updated fakes across api/mcp/web/lg/hub
