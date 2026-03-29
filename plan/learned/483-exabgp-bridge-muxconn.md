# 483 -- ExaBGP Bridge MuxConn

## Context

The ExaBGP bridge translates between ExaBGP plugins (text commands, ExaBGP JSON) and ze's plugin protocol. After ze adopted MuxConn (`#<id> verb json` framing) for all post-startup plugin communication, the bridge was never updated. Both directions silently failed: events dropped (bridge couldn't parse `#<id>` prefix as JSON), commands dropped (MuxConn rejected lines without `#` prefix). Tests didn't catch it because integration tests communicated directly via pipes, bypassing MuxConn. Additionally, the bridge generated wrong command syntax (`nhop set` instead of `nhop`) that ze's parser would reject.

## Decisions

- Inline MuxConn parsing for standalone mode (stdin/stdout) over using `rpc.Conn`/`rpc.MuxConn` library, because the 5-stage startup uses a `bufio.Scanner` on stdin that buffers ahead -- creating a new `rpc.Conn` on the same fd would lose buffered data.
- SDK/TLS connect-back for engine-launched mode over stdin/stdout, because ze's process manager expects external plugins to connect back via TLS. Detected by checking `ZE_PLUGIN_HUB_TOKEN` env var at startup.
- Two bridge modes (standalone stdin/stdout, engine-launched TLS) over registering as internal plugin, because the bridge wraps an external subprocess and the standalone mode is useful for development.
- `rpc.NewConn` signature change (`io.ReadCloser`/`io.WriteCloser`) as its own commit, because it unblocks the unified-cli spec (SSH as plugin transport) independently.
- Removed `set`/`add` keywords from translated commands (`nhop set X` to `nhop X`) over keeping them, because ze's text command parser uses flat syntax without action keywords.

## Consequences

- ExaBGP plugins work end-to-end through ze's plugin infrastructure: config references the bridge, engine launches it, TLS connect-back, SDK handles 5-stage protocol, events and commands flow bidirectionally.
- Forward-barrier flush works transparently through the bridge after route commands.
- `rpc.NewConn` now accepts any `io.ReadCloser`/`io.WriteCloser`, not just `net.Conn`. Enables SSH channels and stdio as plugin transport without adapters. Unblocks `spec-unified-cli`.
- Standalone mode (stdin/stdout with inline MuxConn parsing) preserved for development and testing outside ze.
- `.ci` functional test (`exabgp-bridge-sdk.ci`) proves the full path: ExaBGP plugin command -> bridge translation -> TLS -> engine -> BGP UPDATE -> ze-peer.

## Gotchas

- `bufio.Scanner` reads ahead in chunks -- reusing the scanner after the text-based startup is essential. Creating a new reader on the same file descriptor loses buffered data. This is why the standalone mode uses inline parsing instead of `rpc.Conn`.
- The bridge had TWO independent bugs: wire format (MuxConn framing) and transport (TLS vs stdin/stdout). Conflating them caused wasted effort. Treat wire format and transport as separate concerns.
- Command syntax was wrong (`nhop set`, `origin set`) but existing tests passed because they tested translation functions in isolation, never against ze's actual parser. The `.ci` test caught this immediately.
- `SetWriteDeadline` is not available on `*os.File` or SSH channels. The `rpc.NewConn` change degrades gracefully (skips deadlines via type assertion).
- `cmd=api` in `.ci` test files is documentation metadata, not an action that sends commands. The route must come from the plugin.

## Files

- `pkg/plugin/rpc/conn.go` -- `NewConn` signature: `net.Conn` to `io.ReadCloser`/`io.WriteCloser`
- `cmd/ze/exabgp/main.go` -- TLS env var detection, mode branching
- `cmd/ze/exabgp/main_sdk.go` -- SDK/TLS connect-back mode implementation
- `internal/exabgp/bridge/bridge.go` -- MuxConn inline parsing, flush injection, `EncodeAddPathHex`
- `internal/exabgp/bridge/bridge_muxconn.go` -- MuxConn wire format helpers, pending responses
- `internal/exabgp/bridge/bridge_command.go` -- removed `set`/`add` from translated commands
- `internal/exabgp/bridge/bridge_test.go` -- 25 new tests (49 total)
- `test/plugin/exabgp-bridge-sdk.ci` -- end-to-end functional test
