# 640 -- MCP 3 Elicitation

## Context

MCP 2025-06-18 adds `elicitation/create`: a server-initiated JSON-RPC
request that lets a tool handler ask the client for a missing input
mid-dispatch. Ze's Phase 1 and 2 shipped the Streamable HTTP transport
with session registry and OAuth 2.1 identity, but `tools/call` still
had to fail when an argument was missing. Operators interacting through
Claude or a similar client saw validation errors instead of a follow-up
question, and the handcrafted `ze_execute` tool could not be invoked
without already knowing the exact ze command to run. Goal: land
elicitation, wire one concrete elicitor (`ze_execute` empty-command
branch), and keep the door open for Phase 4's task-augmented
`tools/call`.

## Decisions

- **Per-POST reply sink that upgrades in place**, over GET-only
  server-to-client streams. The client's original `tools/call` already
  owns an HTTP body; upgrading it from `application/json` to
  `text/event-stream` when a handler elicits delivers both the elicit
  frame and the terminal tool result on the same response, with no new
  stream to coordinate. `session.outbound` stays reserved for the
  long-lived GET stream used by notifications and future task status.
- **Capability bit on the session**, not on the transport. The client's
  `capabilities.elicitation = {}` at initialize sets
  `session.clientElicit`; handlers check `session.ClientSupportsElicit()`
  before calling `Elicit`. Registry grew a `CreateWithCapabilities`
  constructor so `Create` stays the zero-capability legacy path.
- **Stdlib-only schema validator**, over importing a JSON Schema
  library. The elicitation schema subset (flat primitives + enum) is
  ~80 lines; a library would add surface area and dependency churn for
  no gain. The validator rejects nested objects, array roots, and
  composition keywords with `ErrElicitSchemaInvalid`, consistent with
  `rules/exact-or-reject`.
- **Correlation map primitive is elicitation-specific today.** Phase 4
  will want the same pattern keyed by task id; we considered naming it
  `RegisterPending`/`ResolvePending` up front but deferred per
  anti-premature-abstraction -- rename at second user.
- **Per-request context threaded through tool handlers via `server.ctx`.**
  The tool-handler callback signature is
  `func(s *server, args json.RawMessage) map[string]any` and predates
  elicitation; rather than widen every handler signature, the POST's
  `r.Context()` flows into `server.ctx` at construction in
  `Streamable.callTool`. `ze_execute` passes that to `session.Elicit` so
  client disconnect unblocks the suspended handler via `ctx.Done()`
  instead of waiting for the session TTL sweep. Legacy `Handler()` path
  leaves `server.ctx` nil; the handler falls back to
  `context.Background()`. Go guidance discourages storing ctx in
  structs, but the struct is per-request and the lifetime matches --
  pragmatic exception.
- **Client-error shape for POST response body with `error` object** is
  translated to an explicit `cancel` action, not `ErrElicitMalformed`.
  MCP does not define elicit-error semantics; treating RPC-level error
  as "never mind" is consistent with how operators see the UI. An
  accept with non-object `content` still returns 400 -- that is a
  protocol violation.
- **`.ci` tests live in `test/plugin/`**, over the spec's original
  `test/mcp/`. The latter does not exist and has no runner wiring; the
  `mcp-announce.ci` pattern in `test/plugin/` already demonstrates an
  MCP-only scenario (ze background + `ze-test mcp` foreground).
- **`ze_execute` schema drops the `command` required key.** With the
  elicit branch, `command` is legitimately optional on elicit-capable
  clients; keeping it required would be schema-lying. The description
  line documents the behavior instead.

## Consequences

- **Phase 4 can reuse the reply-sink abstraction.** Task progress
  frames will want the same POST-upgrade path when the client invoked
  the task synchronously. The `replySink` interface + sink-swapping
  machinery is the seam.
- **The `server` struct now carries a `*session`** (nil on legacy
  `Handler()` path). Any future tool handler that wants to emit
  server-initiated frames will read it there. Nil-aware handlers must
  degrade gracefully.
- **Capability-gating is the pattern** for future per-session client
  bits. Each new capability adds one bool to `session`, one
  `parseXCapability` helper, and an analogous `ClientSupportsX` check.
- **The spec's AC-15 phrasing was corrected** from "registration time
  (server-side sanity)" to "call-site sanity at `session.Elicit()`";
  elicitation schemas are per-call, never pre-registered. AC-15a/b/c
  were added to the spec to cover capability gate, unknown-id, and
  context-cancel -- umbrella gaps.
- **`cmd/ze-test/mcp.go` now detects `text/event-stream`** and routes
  elicit frames through a queue of replies supplied by stdin
  directives (`elicit-accept`/`-decline`/`-cancel`). Missing queue
  entries auto-cancel so misconfigured tests do not hang the daemon.

## Gotchas

- **`auto_linter.sh` runs in parallel with editor Write/Edit.** When a
  parallel session has broken code elsewhere in the tree (e.g.
  `adj_rib_in` mid-refactor), our own edits trip the linter on files
  we have not touched. Verify cleanly with `go test ./path` or `go vet
  ./path` on the actual package to filter noise.
- **`context` imports need paired consumers in the same Edit.** Adding
  `"context"` and the `context.Background()` call site in two separate
  edits leaves an unused-import linter window; the linter ran between
  the two edits and flagged it. Bundle.
- **SSE frames are single-line JSON by invariant** (see
  `reply_sink.go` godoc) but the scanner default buffer of 64 KB is
  not enough for `ze_execute` dispatches that return large command
  output. Bumped to 2 MB on the client side.
- **JSON unmarshals numeric ids to `float64`.** Comparing
  `frame["id"] == reqID` where `reqID` is an `int` silently fails.
  Type-assert to `float64` and cast.
- **Schema `required: []any{"command"}`**, not `[]string{"command"}`.
  `map[string]any` serialisation treats `[]string` as a distinct type
  and the MCP client libraries accept only `[]any` uniformly across
  schema fields. Copy-paste from Go idioms bites here.
- **Parallel session uncommitted work** on `adj_rib_in`, `format`, and
  `filter` blocked `make ze-verify-fast` at the lint stage. Targeted
  package tests (`go test -race ./internal/component/mcp/`) confirmed
  this spec's code compiles and passes cleanly; the full verify will
  pass once the parallel session commits.
- **UK spellings trip the misspell linter.** "cancelled" and
  "behaviour" hit `misspell` (US dictionary). Spec prose survives by
  virtue of being markdown-only (`plan/**/*.md` not linted), but
  godoc comments and guide docs do get checked. Stick to US spellings
  (canceled, behavior, serialize, color) in new text.
- **`//nolint:containedctx` is the right hatch for per-request structs.**
  Go lint default-rejects `ctx` fields on structs; the `//nolint`
  directive plus a clear godoc paragraph is the accepted exception
  when the struct is truly per-request (like `http.Request` in stdlib).

## Files

- `internal/component/mcp/elicit.go` -- `session.Elicit`,
  `validateElicitSchema`, `buildElicitFrame`, error sentinels
- `internal/component/mcp/session.go` -- `clientElicit` bit,
  correlation map, `CreateWithCapabilities`, sink hooks
- `internal/component/mcp/streamable.go` -- `parseElicitationCapability`,
  `doInitialize` capability thread, `callTool(sess)`,
  `handleElicitResponse`, POST->SSE upgrade in `handlePOST`
- `internal/component/mcp/handler.go` -- `server.session` field,
  `ze_execute` missing-command elicit branch, relaxed handcrafted
  schema
- `internal/component/mcp/reply_sink.go` -- `jsonReplySink`,
  `sseReplySink`, `replySink` interface
- `internal/component/mcp/{elicit,reply_sink,streamable}_test.go` --
  unit coverage for every AC
- `cmd/ze-test/mcp.go` -- `--elicit` flag, SSE reader,
  `elicit-{accept,decline,cancel}` stdin directives
- `test/plugin/elicitation-{accept,decline,no-capability}.ci` --
  functional scenarios
- `docs/guide/mcp/elicitation.md` -- new user guide
- `docs/architecture/mcp/overview.md` -- Transport Shape + Capability
  Negotiation subsection + Files table
- `docs/features.md`, `docs/features/mcp-integration.md`,
  `docs/architecture/api/commands.md`, `docs/functional-tests.md`,
  `docs/comparison.md` -- surface updates
