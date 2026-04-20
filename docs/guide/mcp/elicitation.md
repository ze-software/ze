# MCP Elicitation

<!-- source: internal/component/mcp/elicit.go -- session.Elicit -->
<!-- source: internal/component/mcp/session.go -- correlation + capability bit -->
<!-- source: internal/component/mcp/streamable.go -- POST -> SSE upgrade -->
<!-- source: internal/component/mcp/handler.go -- ze_execute missing-command branch -->

Ze implements the MCP 2025-06-18
[elicitation](https://modelcontextprotocol.io/specification/2025-06-18/client/elicitation)
capability. A tool handler that needs information it does not have yet --
typically because the client omitted an argument -- can ask the client for
it mid-dispatch instead of failing with a validation error. The canonical
example is the handcrafted `ze_execute` tool: when called without a
`command` argument, the server responds with an `elicitation/create`
request asking the client which ze command to run, waits for the reply,
then dispatches the accepted command.

## Capability Negotiation

The spec requires clients that support elicitation to declare it at
initialize:

```json
{
    "method": "initialize",
    "params": {
        "protocolVersion": "2025-06-18",
        "capabilities": { "elicitation": {} }
    }
}
```

The server records a single bit (`clientElicit`) per session from that
declaration and refuses to emit `elicitation/create` for any session where
the bit is clear. Tool handlers that rely on elicitation are expected to
fall back to a hard error when `session.ClientSupportsElicit()` returns
false, so a client that never advertised the capability still gets a
deterministic outcome (e.g. `missing required argument: command`) rather
than a hang on a response that will never arrive.

## Transport Shape

Ze answers every client `POST /mcp` with either `application/json` (the
common case) or `text/event-stream`. The response upgrades to SSE
automatically when, and only when, the tool handler invokes
`session.Elicit`. Concretely:

1. Client sends `tools/call` over `POST /mcp`.
2. Handler calls `session.Elicit(ctx, message, schema)` with a flat
   object JSON Schema describing the requested fields.
3. The POST's reply sink upgrades in place from `jsonReplySink` to
   `sseReplySink`. Headers go out as `Content-Type: text/event-stream`,
   the `elicitation/create` request frame is written, the handler
   suspends on the per-elicit correlation channel.
4. Client reads the SSE frame, prompts the operator (or routes through
   the agent's confirmation flow), and POSTs a JSON-RPC response whose
   `id` matches. No new stream is opened -- the reply is a normal
   `POST /mcp` with the `Mcp-Session-Id` header and no `method` field.
5. Server routes the response to the suspended handler via the
   correlation map. The handler resumes and its terminal result rides
   the same SSE stream as a final frame.

The terminal response is guaranteed to reach the same HTTP response
body that carried the elicit. Clients do not need a separate GET SSE
stream for elicitation flows.

## Schema Subset

Per the spec, elicitation schemas MUST be flat (no nested objects, no
`oneOf`/`anyOf`/`allOf`, no arrays as root). Ze enforces this before
sending the frame:

| Allowed at property level | Comment |
|---------------------------|---------|
| `type: string` (optionally with `enum`) | `enum` is validated for non-empty string members |
| `type: number` / `integer` | |
| `type: boolean` | |
| `description`, `title`, `default` | |

The server rejects a schema with `ErrElicitSchemaInvalid` instead of
sending a malformed frame. Reject-at-verify is symmetric with the rest of
ze's configuration and protocol surface.

## Accept / Decline / Cancel

The client's response carries one of three actions. Ze translates each
into a typed outcome the handler can branch on:

| Client action | Handler sees | Typical handler response |
|--------------|--------------|--------------------------|
| `accept` with `content` | `content map[string]any, nil` | Extract fields, dispatch the intended work |
| `decline` | `nil, ErrElicitDeclined` | Report an error to the user; do not dispatch |
| `cancel` | `nil, ErrElicitCanceled` | Same as decline; the client backed out |
| malformed / unknown action | `nil, ErrElicitMalformed` | Return a protocol error |
| context canceled | `nil, ctx.Err()` | Clean up, return early |
| capability absent | `nil, ErrElicitUnsupported` | Handler should have pre-checked with `ClientSupportsElicit` |
| too many pending | `nil, ErrElicitTooMany` | Back off; per-session cap is 32 |

## Writing an Elicit-Aware Tool Handler

1. Pre-check `s.session != nil && s.session.ClientSupportsElicit()`.
   If false, return an `isError` tool result with a message that tells
   the caller what is missing.
2. Build a flat schema as `map[string]any` describing the fields.
3. Call `s.session.Elicit(ctx, humanMessage, schema)`.
4. On `err == nil`, extract fields with the `_, _ := content[k].(T)`
   idiom and treat an empty/zero result as a failure.
5. On `errors.Is(err, ErrElicitDeclined)` or `ErrElicitCanceled`, return
   an `isError` result; do not treat it as a bug.
6. On any other error, surface the error text verbatim.

The `ze_execute` handler in `internal/component/mcp/handler.go` is the
reference implementation. Functional coverage lives in
`test/plugin/elicitation-{accept,decline,no-capability}.ci`.

## Testing With `ze-test mcp`

The `ze-test mcp` client understands elicitation scenarios. Add
`--elicit` to declare the capability, and use stdin directives to queue
replies:

```
# Stdin lines (in order)
elicit-accept {"command":"peer list"}
@ze_execute {"command":""}
```

Other directives: `elicit-decline`, `elicit-cancel`. When the client
encounters an elicit frame and the queue is empty, it auto-replies with
`cancel` so a misconfigured test does not hang the daemon.

See the `elicitation-*.ci` scenarios in `test/plugin/` for complete
orchestration examples.
