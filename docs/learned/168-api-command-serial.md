# 168 — API Command Serial Numbers

## Objective

Document the already-implemented request/response serial correlation scheme between ZeBGP and external plugin processes.

## Decisions

- Alpha encoding for ZeBGP-initiated serials (0→a, 1→b, …, 9→j, 10→ba): prevents collision with process-initiated numeric serials without requiring negotiation or separate channels.
- Process uses `#N` numeric prefix; ZeBGP uses `#abc` alpha prefix; responses echo with `@serial` — three distinct namespaces that cannot collide.
- Fire-and-forget for commands without a serial prefix — no response generated, saves round-trip for high-volume route announcements.

## Patterns

- Alpha serial encoding: `digit % 10 → 'a' + digit`, reversed build, O(log n) length.
- `@serial done [json]` and `@serial error "msg"` — response always terminates; streaming uses `@serial+` continuation.

## Gotchas

None — this was a documentation-only spec for existing implemented behaviour.

## Files

- `internal/component/plugin/server.go` — `parseSerial()`, `encodeAlphaSerial()`, `isAlphaSerial()`
- `internal/component/plugin/process.go` — `SendRequest()`, `parseResponseSerial()`
- `internal/component/plugin/types.go` — `Response.Serial`
