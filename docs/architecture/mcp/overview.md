# MCP Architecture Overview

Ze exposes the [Model Context Protocol](https://modelcontextprotocol.io/) so that
AI assistants can drive the BGP daemon through the same command surface a human
operator uses at the CLI. The implementation lives in
`internal/component/mcp/` and is mounted by `cmd/ze/hub/` on an HTTP listener
configured under `environment.mcp.server`.

<!-- source: internal/component/mcp/handler.go — MCP component package layout -->
<!-- source: internal/component/mcp/streamable.go — Streamable HTTP transport -->
<!-- source: cmd/ze/hub/mcp.go — production mount point -->

## Protocol Profile

| Profile | Status | Used By |
|---------|--------|---------|
| 2024-11-05 JSON-RPC-over-POST | Legacy (test compatibility) | `internal/component/mcp/handler.go` (`Handler` factory) |
| 2025-06-18 Streamable HTTP | Current | `internal/component/mcp/streamable.go` (`NewStreamable`) |

`cmd/ze/hub/mcp.go:startMCPServer` mounts `NewStreamable` for all production
listeners. The legacy `Handler` factory remains for tests that exercise the
older single-shot POST semantics; it will be removed once every test has been
migrated to the session-oriented transport.

## Files

| File | Concern |
|------|---------|
| `handler.go` | JSON-RPC 2.0 types (`request`, `response`, `rpcError`, `callParams`), handcrafted tool catalogue, tool runner helper (`server` struct with optional `*session`), legacy `Handler` factory |
| `tools.go` | Command-registry -> MCP tool auto-generation: grouping, schema emission, dispatch |
| `streamable.go` | MCP 2025-06-18 Streamable HTTP dispatcher: POST/GET/DELETE, Origin gate, Bearer check, method dispatch, `handleElicitResponse` correlation router |
| `session.go` | Session registry (`sessionRegistry`), session state (`session`) with `clientElicit` bit and elicit correlation map, TTL garbage collection, SSE outbound queue |
| `elicit.go` | `session.Elicit`, schema validator (`validateElicitSchema`), elicit error sentinels |
| `reply_sink.go` | `replySink` interface + JSON and SSE implementations that let a POST upgrade its reply shape mid-dispatch |
| `schema/ze-mcp-conf.yang` | YANG configuration: server listener, token |

## Transport Shape (2025-06-18)

<!-- source: internal/component/mcp/streamable.go — ServeHTTP -->

| Method | Path | Body | Purpose |
|--------|------|------|---------|
| POST | `/mcp` | JSON-RPC 2.0 request | Client-to-server call. Response is `application/json` by default; a tool handler that invokes `session.Elicit` upgrades the POST reply in place to `text/event-stream` so the `elicitation/create` request and the terminal tool response ride the same HTTP response body |
| POST | `/mcp` | JSON-RPC 2.0 response (no `method`) | Client's reply to a server-initiated request (`elicitation/create`). Routed by correlation id to the suspended handler; returns 202 Accepted |
| GET | `/mcp` + `Accept: text/event-stream` | — | Client opens a server-to-client SSE stream bound to its session for notifications and task status. Elicitation flows do NOT use this stream -- they ride the originating POST's upgraded reply instead |
| DELETE | `/mcp` | — | Client terminates its session |
| GET | `/.well-known/oauth-protected-resource` | — | RFC 9728 protected resource metadata. Phase 2 populates; Phase 1 returns 404 |

## Headers

<!-- source: internal/component/mcp/streamable.go — handlePOST / handleGET -->

| Header | Direction | Semantics |
|--------|-----------|-----------|
| `Content-Type: application/json` | Client -> server (POST) | Required. CSRF guard: rejects `text/plain` form submissions from browsers |
| `Origin` | Client -> server | Validated against `StreamableConfig.AllowedOrigins`. Empty allowlist accepts only loopback-shaped origins (`null`, `localhost`, `127.0.0.1`, with or without port). Non-matching origin is rejected with 403 before any session lookup |
| `Authorization: Bearer <token>` | Client -> server | Required when `Token` is set. Constant-time compare. Phase 2 replaces the single shared token with per-identity and OAuth modes |
| `Mcp-Session-Id` | Server -> client (initialize response), then Client -> server (subsequent requests) | 22-char base64url of 128 random bits. Required on every non-initialize request. Returns 404 when absent or expired |
| `MCP-Protocol-Version` | Client -> server (post-initialize) | Missing header is tolerated and treated as `2025-03-26` per spec; unknown value returns 400 |

## Capability Negotiation

<!-- source: internal/component/mcp/streamable.go -- parseElicitationCapability -->
<!-- source: internal/component/mcp/session.go -- sessionRegistry.CreateWithCapabilities -->

The server records per-session capability bits at `initialize` from the
client's `params.capabilities` object. The only bit tracked today is
`clientElicit`, set from `capabilities.elicitation = {}` per the MCP
2025-06-18 elicitation spec. Handlers read it via
`session.ClientSupportsElicit()` before calling `session.Elicit` to keep
the server from emitting `elicitation/create` to a client that never
declared support.

| Bit | Source leaf | Consumer |
|-----|-------------|----------|
| `clientElicit` | `capabilities.elicitation: {}` | `session.Elicit`, `ze_execute` missing-command branch |

Missing, null, or non-object shapes (`capabilities.elicitation: null`,
`capabilities.elicitation: false`) are treated as "not declared." Unknown
capability keys are ignored.

## Session Lifecycle

<!-- source: internal/component/mcp/session.go — sessionRegistry -->

1. Client POST `initialize` on `/mcp`. Server creates a session in the
   `sessionRegistry`, returns `Mcp-Session-Id` on the response header, and
   includes the negotiated `protocolVersion` in the body.
2. Client sends every subsequent request with the `Mcp-Session-Id` header.
   Each `Get` refreshes the session's `lastSeenAt` timestamp.
3. The registry's GC goroutine sweeps every 30 seconds and evicts sessions
   whose `lastSeenAt` is older than `SessionTTL` (default 30 minutes, clamped
   to `[60 s, 24 h]`).
4. Client may DELETE `/mcp` to terminate its session explicitly.

Each session owns an outbound `chan []byte` drained by the SSE writer on the
GET stream. `session.Send` is best-effort non-blocking: a full queue returns
`errSessionQueueFull` rather than blocking the producer. Phase 3 adds per-
identity rate limiting so elicitation flows cannot starve one another.

## Mount Point

<!-- source: cmd/ze/hub/mcp.go — startMCPServer -->

`cmd/ze/hub/main.go` calls `startMCPServer(addrs, dispatch, commands, token)`
when `environment.mcp.server` has at least one entry. Each listener address
gets its own `net.Listener`; all are served by a single `http.Server` whose
handler is the `*zemcp.Streamable`. Shutdown calls `http.Server.Shutdown`;
the session registry's GC goroutine currently outlives shutdown until the
process exits (acceptable because session state lives only in RAM).

## Auto-Generated Tools

<!-- source: internal/component/mcp/tools.go — groupCommands, buildToolDef -->

`CommandLister` returns every registered CLI command (`CommandInfo`). The
tool generator groups by common prefix (`bgp rib status`, `bgp rib routes`
-> `bgp rib` group with `status` and `routes` actions), synthesises a JSON
Schema from each command's YANG RPC metadata, and emits an MCP tool named
`ze_<prefix_joined_with_underscores>`. The handcrafted `ze_execute` tool is
a raw dispatch escape hatch.

Tools are derived at every `tools/list` call, so newly registered commands
become available without any MCP code changes (rule: `derive-not-hardcode`).

## Security Model (Phase 1)

- Default binding is `127.0.0.1`. Phase 2 adds an opt-in `bind-remote` leaf.
- Bearer token is optional. Without it, any caller on the bind interface is
  trusted.
- Origin allowlist defaults to loopback-shaped origins.
- `application/json` content-type is required on POST to defeat browser
  `text/plain` form CSRF.
- 1 MiB request body cap via `http.MaxBytesReader`.

Phase 2 introduces OAuth 2.1 resource-server semantics (RFC 9728 metadata,
audience-bound tokens, `WWW-Authenticate` on 401) and a per-identity bearer
list as alternatives to the shared token.

## Roadmap

| Phase | Spec | Delivers |
|-------|------|----------|
| 1 | `spec-mcp-1-streamable-http.md` | This transport (landed) |
| 2 | `spec-mcp-2-remote-oauth.md` | Remote binding, OAuth 2.1, per-identity bearer list |
| 3 | `plan/learned/NNN-mcp-3-elicitation.md` | Server-initiated `elicitation/create`; POST reply upgrades to SSE on demand (landed) |
| 4 | `spec-mcp-4-tasks.md` | Task-augmented `tools/call`, `tasks/*` methods, durable task registry |
| 5 | `spec-mcp-5-apps.md` | Resources capability, `ui://` UI-resource scheme |
