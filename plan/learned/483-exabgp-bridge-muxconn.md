# 483 -- ExaBGP Bridge MuxConn

## Context

The ExaBGP bridge translates between ExaBGP plugins (text commands, ExaBGP JSON) and ze's plugin protocol. After ze adopted MuxConn (`#<id> verb json` framing) for all post-startup plugin communication, the bridge was never updated. Both directions silently failed: events dropped (bridge couldn't parse `#<id>` prefix as JSON), commands dropped (MuxConn rejected lines without `#` prefix). Tests didn't catch it because integration tests communicated directly via pipes, bypassing MuxConn.

## Decisions

- Inline MuxConn parsing over using `rpc.Conn`/`rpc.MuxConn` library, because the 5-stage startup uses a `bufio.Scanner` on stdin that buffers ahead -- creating a new `rpc.Conn` on the same stdin would lose buffered data.
- `pendingResponses` type (register/signal/wait) for cross-goroutine flush blocking, over fire-and-forget dispatch, because AC-3 requires the bridge to block until the engine confirms the flush.
- Separate `rpc.NewConn` signature change (`io.ReadCloser`/`io.WriteCloser`) as its own commit, because it unblocks the unified-cli spec (SSH as plugin transport) independently.
- `.ci` functional test not included, because the bridge transport (stdin/stdout vs TLS connect-back) is a separate problem requiring the bridge to be registered as an internal plugin.

## Consequences

- ExaBGP plugins can now send route announcements through the bridge after the 5-stage startup completes.
- Forward-barrier flush works transparently through the bridge (AC-3).
- `rpc.NewConn` now accepts any `io.ReadCloser`/`io.WriteCloser`, not just `net.Conn`. This enables SSH channels and stdio as plugin transport without adapters.
- The bridge still uses stdin/stdout (not TLS connect-back), so it cannot run as an external plugin through ze's process manager. A future spec must register it as an internal plugin to enable `.ci` tests and production use.

## Gotchas

- `bufio.Scanner` reads ahead in chunks -- reusing the scanner after the text-based startup is essential. Creating a new reader on the same file descriptor loses buffered data.
- External plugins in ze use TLS connect-back, not stdin/stdout pipes. The bridge's stdin/stdout assumption is a second, independent bug that the wire format fix does not address.
- `SetWriteDeadline` is not available on `*os.File` or SSH channels. The `rpc.NewConn` change degrades gracefully (skips deadlines via type assertion) but writes may block longer on non-TCP streams.

## Files

- `pkg/plugin/rpc/conn.go` -- `NewConn` signature: `net.Conn` to `io.ReadCloser`/`io.WriteCloser`
- `internal/exabgp/bridge/bridge.go` -- MuxConn event/command handling, flush injection
- `internal/exabgp/bridge/bridge_muxconn.go` -- MuxConn wire format helpers, pending responses
- `internal/exabgp/bridge/bridge_command.go` -- cross-reference update
- `internal/exabgp/bridge/bridge_event.go` -- cross-reference update
- `internal/exabgp/bridge/bridge_test.go` -- 15 new tests (39 total)
