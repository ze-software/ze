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

- **Nothing replaces `MaxSessions`.** It bounded *sessions*, an object that
  ceases to exist. After the cutover Ze holds no long-lived per-client state at
  all. Inventing a replacement cap would be inventing state.
- **The Provider-mode branch disappears rather than being exempted.** `ze-chaos`
  is unauthenticated *by configuration* (no `Token`, no `AuthMode`, so
  `NewStreamable` infers `AuthNone`), not by that branch. Running it through the
  uniform path is observably identical while removing the only code shape from
  which an unauthenticated path could later be reached by accident. A carve-out
  that is not needed is one that will outlive its reason.
- **Absence and mismatch of a required header are the same verdict.** No header
  gets a default. The pre-cutover code defaulted a missing version to
  `LegacyProtocolVersion`, which is the fail-open shape `fail-closed-guards.md`
  bans; with no handshake there is nothing to fall back to.
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
  before dispatch is the entire point of `-32020`; a load balancer routing on the
  header while the server executes on the body is the attack it closes.
- Four requirements the spec had missed were found by reading the specification
  text rather than the changelog: `-32602` for a malformed `_meta` is distinct
  from `-32020`; `-32021 MissingRequiredClientCapability` is a MUST; `serverInfo`
  belongs in *every* result's `_meta`; and `_meta` sits inside `params`, not at
  the message top level.

## Gotchas

- **A spec premise can go stale within a day.** The spec built AC-13 on a live
  nil-dereference in `resources.go`; it had been fixed the previous day. Re-read
  cited code before relying on it, especially in an active tree.
- **`resources` is not a client capability.** Ze gated `resources/list`/`read` on
  the client declaring it, but `ClientCapabilities` has exactly five members and
  `resources` is not among them -- it is a *server* capability. The gate meant no
  conformant client could read a `ui://` asset while `tools/list` advertised
  those assets, and the `-32021` `data.requiredCapabilities` named a value that
  field cannot legally hold. Removed: `resourcesList`/`resourcesRead` no longer
  take a capability argument at all, so it cannot come back by accident.
- **`chaos-web` is not a `ze-verify` stage.** Neither `ze-functional-test` nor
  `stagesForMode` runs it, so both MCP chaos tests are invisible to the main
  gate. A build-tag bug (`ze-chaos` compiled without `ze_bgp`, dying at startup
  with `no such module: ze-bgp-conf`) had been failing all six of them since the
  gate landed, unnoticed. `make ze-chaos-test` must be run explicitly.
- **`defaultMaxTerminalTasks` was assigned and never read.** Decision D-1 cited
  it as a surviving bound justifying the deletion of the session caps, while
  `sweep()` reaped on TTL alone. Implemented rather than weakening the argument:
  per-principal, oldest-terminal-first. Per-principal because a global cap hands
  every caller a cross-principal eviction primitive.
- **`enabled` was answering two questions.** `ExtractMCPConfig` returned
  `ok=false` unless the block was `enabled true` with a ported server, and the
  caller then discarded `auth-mode`, `token`, `BearerList` and `OAuth` with it --
  so `ze --mcp <port>` plus a configured bearer token produced an *accept-all*
  listener. Split: `enabled` answers "does config start a listener";
  `ExtractMCPSettings` answers "how does this listener authenticate". Found by
  strengthening a test whose title claim ("identity scope") was untested.

## Files

- Deleted: `internal/component/mcp/{session,elicit,reply_sink}.go` + tests
- Created: `internal/component/mcp/{meta,headers,discover}.go` + tests
- `internal/component/mcp/{streamable,streamable_tools,resources,tasks,tools}.go`
- `internal/component/config/loader_extract.go` (the `ExtractMCPSettings` split)
- `cmd/ze/hub/{main,service_mcp,listener_migrate}.go`
- `internal/test/runner/` (`header=` on the `http=` directive)
- `test/plugin/mcp-*.ci`, `test/chaos-web/`
