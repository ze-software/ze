# 1298 -- mcp2026-0-umbrella

## Context

Ze's MCP server spoke protocol revision `2025-06-18` and additionally accepted
`2025-03-26` and `2024-11-05`. The specification published `2026-07-28`, which is
not an additive release: it removes the `initialize` handshake, protocol-level
sessions and the `Mcp-Session-Id` header, the GET stream endpoint, and
server-initiated JSON-RPC requests on SSE streams. Those four are exactly what
Ze's transport was built on. Ze had also skipped `2025-11-25` entirely, so it was
two revisions behind.

The goal was full conformance as a **clean cutover** -- the older revisions
dropped rather than maintained alongside. Ze is unreleased and the MCP server is
not the plugin API, so `ai/rules/compatibility.md` says no compatibility surface
is owed (owner decision, 2026-07-28). Four sequenced child specs delivered it:
stateless core, MRTR, the Tasks extension, and caching + Apps.

## Decisions

- **Clean cutover over a dual-era server**, which the versioning page explicitly
  permits. Dual-era would have preserved the session registry, the SSE sink
  upgrade and the correlation map permanently *alongside* their replacements.
  The accepted cost is recorded in the spec's own compatibility matrix: a legacy
  client against a modern server simply fails, with no fall-forward.
- **Four sequenced phases over one cutover spec.** Phase 1 was atomic and not
  splittable -- `(*session).Elicit`, `TaskElicit` and `handleElicitResponse` all
  name the `session` type, so deleting `session.go` breaks the build unless all
  three go together. That type-system argument turned out to be stronger than
  the one the umbrella originally gave (about learning the client's version).
- **The `ze-test mcp` client was rewritten from the specification text, not from
  Ze's server.** MCP has no third-party interop lab in-tree, so the test client
  is the only independent reading available; deriving it from the server would
  make every functional test tautological. This paid off twice -- see Gotchas.
- **Live state (the event bus) stays out.** Surfacing it is a new feature, not a
  conformance obligation. When it is wanted the design is fixed: model the state
  as MCP resources under a `ze://` URI space and let `resourceSubscriptions`
  carry the change signal, rather than piping internal typed Go values through a
  Ze-defined notification type. Deferred to `plan/spec-mcp2026-5-state-resources.md`.

## Consequences

- The cutover is a **net deletion** of operational complexity. Sessions were the
  reason Ze needed TTL garbage collection, an absolute-lifetime cap, a
  max-session cap, a single-stream-per-session guard, a per-session outbound
  queue, and a documented requirement for sticky load-balancer routing. All of
  that is gone with no replacement, because the protocol no longer has the
  concept.
- Every request now authenticates. This is **strictly stronger** than what it
  replaced: a stolen `Mcp-Session-Id` used to be a bearer credential in its own
  right, and revoking a token now takes effect on the next request rather than
  at session expiry.
- `subscriptions/listen` is not implemented and that is conformant, not a gap:
  no server-side MUST obliges it, and after the GET stream is deleted Ze has
  nothing to advertise on a stream.
- Prompts, Roots, Sampling and Logging remain unimplemented. The last three are
  Deprecated in this revision, and new implementations are told not to adopt them.

## Gotchas

- **Independent implementation caught two real defects that a shared reading
  would have hidden.** The client agent, working only from the specification,
  found that `resources` is not a member of `ClientCapabilities` at all (it is a
  *ServerCapabilities* member) -- so Ze's gate on a client declaring it meant no
  conformant client could ever read a `ui://` resource, while `tools/list`
  actively advertised those resources. It also transcribed a Base64 test vector
  wrongly and found the implementation right, which is what proved the sentinel
  is standard Base64 rather than base64url.
- **A cross-phase seam produced a working feature nobody could reach.** Phase 1
  hardened `ze_execute`'s schema to `required: ["command"]` for its AC-15, which
  was correct while elicitation was deleted. Phase 2 restored elicitation and
  never reverted the contract. The handler worked; the published `inputSchema`
  told every client not to try. Each phase was locally correct. The fix is to
  derive the descriptor from the same capability the handler branches on, so the
  two cannot drift.
- **Two claims in this work were recorded as fact without reading the producing
  function, and both were false.** One asserted `ze:task-support forbidden` was
  inert; a live probe returned
  `-32602 tool ze_clear_bgp does not support task-augmented calls`. The other
  blamed a flaky test on a fixed timeout, when the runner already multiplies
  every per-test budget by `ParallelTimeoutHeadroom` (`internal/test/runner/parallel.go`).
  Both were plausible, both cited real line numbers, and neither survived
  `ai/rules/no-fabrication.md` applied properly. A coherent narrative is a
  hypothesis until the producer is read.
- **A green test run proved nothing once.** A `make ze-test` killed by a timeout
  finished as an orphan process *after* the verification that depended on it, so
  a full functional suite ran against a stale binary and passed. Only a
  discriminating mutation -- corrupting the part of an assertion that the old
  code discarded -- exposed it. When a build and a test race, the green is not
  evidence.

## Files

- `plan/spec-mcp2026-{1-stateless-core,2-mrtr,3-tasks-extension,4-caching-apps}.md`
- `plan/learned/{1299,1300,1301,1302}-mcp2026-*.md` (per-phase detail)
- `internal/component/mcp/` (the whole component)
- `internal/test/cli/cmd_mcp*.go` (the independently written client)
- `docs/architecture/mcp/overview.md`, `ai/digests/mcp.md`, `docs/guide/mcp/`
