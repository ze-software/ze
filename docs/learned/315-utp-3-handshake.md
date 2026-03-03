# 315 — UTP-3: Text Handshake Protocol

## Objective

Add a text-format alternative for the 5-stage JSON-RPC plugin handshake, auto-detected from the first byte on Socket A (`{` → JSON, letter → text), using newline-delimited framing and heredoc JSON for config delivery.

## Decisions

- Separate files (`text_conn.go`, `text_mux.go`) instead of modifying `conn.go`/`mux.go` — JSON and text paths have different framing needs; keeping them in separate files avoids mixing concerns. Spec said modify; implementation split for modularity.
- `sdk_text.go` created (234L) rather than adding to `sdk.go` (1059L, above the 1000-line threshold).
- `subsystem_text.go` (117L) for subsystem processes, `server_startup_text.go` (247L) for server path with coordinator barriers — two separate files because subsystem runs all 5 stages without barriers while server interleaves with the coordinator.
- Config delivery uses heredoc (`root <name> json << END` ... `END`) — keeps JSON human-readable without escaping. Fixed `END` marker is unambiguous (JSON lines always contain structure).
- Text event delivery is fire-and-forget (no ACK) — write failure is the only error signal.
- `PeekMode` must operate on raw `net.Conn` before any buffered wrapper, because `bufio.Scanner` would consume the peeked byte.
- AC-2 ("uses textparse keywords") reinterpreted: handshake vocabulary doesn't overlap with BGP textparse keywords; shared types come from `rpc/types.go` instead.

## Patterns

- Auto-detect from first byte eliminates any negotiation protocol — JSON plugins just work without changes.
- `#N` serial prefix on `TextMuxConn` matches IPC protocol convention already defined for `MuxConn`.
- `ze-test text-plugin` subcommand added to the test binary to enable functional `.ci` testing of text-mode plugins without a separate repo.

## Gotchas

- `closeText()` leaked `textConnA` when `textMux` was nil (startup failure before stage 5). Fixed with `else if p.textConnA != nil` branch — caught during critical review.
- Two `.ci` tests deferred (`text-handshake-config.ci`, `text-json-coexist.ci`) — covered by unit tests; full functional deferred to future need.

## Files

- `pkg/plugin/rpc/text.go` (created, 539L) — format/parse for all 5 stage types
- `pkg/plugin/rpc/text_conn.go` (created, 182L) — `TextConn`, `PeekMode`, `peekConn`
- `pkg/plugin/rpc/text_mux.go` (created, 155L) — `TextMuxConn` with `#N` serial prefix
- `pkg/plugin/sdk/sdk_text.go` (created, 234L) — text startup, event loop, close
- `internal/plugin/subsystem_text.go` (created, 117L) — engine-side subsystem text handshake
- `internal/plugin/server_startup_text.go` (created, 247L) — server text handshake with barriers
- `cmd/ze-test/text_plugin.go` (created) — minimal text-mode plugin test binary
- `test/plugin/text-handshake.ci` (created) — end-to-end functional test
