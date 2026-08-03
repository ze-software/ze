# 1299 -- mcp2026-1-stateless-core

## Context

Phase 1 of the `2026-07-28` cutover: replace the whole session-based transport
with a stateless per-request one. Delete the `initialize` handshake, the 570-line
session layer, `Mcp-Session-Id`, the GET SSE stream, DELETE, and the elicitation
machinery. Add per-request `_meta` parsing, standard header validation,
`server/discover`, `resultType`, and per-request authentication. Atomic: the
pieces interlock, and the type system makes it so -- `(*session).Elicit`,
`TaskElicit` and `handleElicitResponse` all name `*session` in their signatures.

## Decisions

- **Nothing replaces `MaxSessions`.** It bounded *sessions*, and that object no
  longer exists. After the cutover Ze holds no long-lived per-client state at
  all. A replacement cap would invent new state.
- **The Provider-mode branch is deleted rather than exempted.** `ze-chaos`
  is unauthenticated *by configuration* (no `Token`, no `AuthMode`, so
  `NewStreamable` infers `AuthNone`), not by that branch. The uniform path gives
  an observably identical result. It also deletes the only code shape that can
  later reach an unauthenticated path by accident. A carve-out that is not needed
  is one that will outlive its reason.
- **Absence and mismatch of a required header are the same verdict.** No header
  gets a default. The pre-cutover code defaulted a missing version to
  `LegacyProtocolVersion`. That is the fail-open shape `evidence.md`
  bans. With no handshake, there is no value to use instead.
- **Per-request identity and capabilities are passed by VALUE.** No pointer, no
  nil-able context, no capability struct whose zero value reads as "supported".
  The compiler forces every handler to have them.

## Consequences

- `ok()` became the single site that stamps `resultType` and
  `_meta["io.modelcontextprotocol/serverInfo"]`, and it takes `map[string]any`
  rather than `any` -- so the compiler, not discipline, keeps every result
  conformant.
- Validation order is now load-bearing and fixed: header (`-32020`) → `_meta`
  (`-32602`) → version (`-32022`) → authenticate → dispatch. Header validation
  before dispatch is the entire point of `-32020`. It closes one attack: a load
  balancer routes on the header while the server executes on the body.
- Four requirements the spec had missed were found in the specification text
  rather than in the changelog:
  - `-32602` for a malformed `_meta` is distinct from `-32020`.
  - `-32021 MissingRequiredClientCapability` is a MUST.
  - `serverInfo` belongs in *every* result's `_meta`.
  - `_meta` sits inside `params`, not at the message top level.

## Gotchas

- **A spec premise can become stale within a day.** The spec built AC-13 on a
  live nil-dereference in `resources.go`. That defect had been fixed the previous
  day. Re-read cited code before you rely on it, especially in an active tree.
- **`resources` is not a client capability.** Ze gated `resources/list` and
  `resources/read` on a client that declares it. But `ClientCapabilities` has
  exactly five members, and `resources` is not among them, because `resources` is
  a *server* capability. The gate meant that no conformant client was able to
  read a `ui://` asset, and `tools/list` advertised those assets at the same
  time. The `-32021` `data.requiredCapabilities` also named a value that field
  cannot legally hold. `resourcesList` and `resourcesRead` no longer take a
  capability argument at all, so the gate cannot return by accident.
- **`chaos-web` is not a `ze-verify` stage.** Neither `ze-functional-test` nor
  `stagesForMode` runs it, so both MCP chaos tests are invisible to the main
  gate. A build-tag bug had been failing all six of them since the gate landed,
  and nobody saw it. `ze-chaos` compiled without `ze_bgp` and died at startup
  with `no such module: ze-bgp-conf`. `make ze-chaos-test` must be run
  explicitly.
- **`defaultMaxTerminalTasks` was assigned and never read.** Decision D-1 cited
  it as a surviving bound that justified the deletion of the session caps, while
  `sweep()` deleted on TTL alone. The cap is now implemented, and the argument
  was not weakened. It deletes per principal, oldest terminal task first. A
  global cap would hand every caller a cross-principal eviction primitive.
- **`enabled` answered two questions.** `ExtractMCPConfig` returned `ok=false`
  unless the block was `enabled true` with a ported server. The caller then
  discarded `auth-mode`, `token`, `BearerList` and `OAuth` with it. So
  `ze --mcp <port>` plus a configured bearer token produced an *accept-all*
  listener. The two questions are now split: `enabled` answers "does config start
  a listener", and `ExtractMCPSettings` answers "how does this listener
  authenticate". A stronger test found the defect, because the old test left its
  own title claim ("identity scope") untested.

## Files

- Deleted: `internal/component/mcp/{session,elicit,reply_sink}.go` + tests
- Created: `internal/component/mcp/{meta,headers,discover}.go` + tests
- `internal/component/mcp/{streamable,streamable_tools,resources,tasks,tools}.go`
- `internal/component/config/loader_extract.go` (the `ExtractMCPSettings` split)
- `cmd/ze/hub/{main,service_mcp,listener_migrate}.go`
- `internal/test/runner/` (`header=` on the `http=` directive)
- `test/plugin/mcp-*.ci`, `test/chaos-web/`
