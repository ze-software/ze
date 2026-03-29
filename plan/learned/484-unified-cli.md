# 484 -- Unified CLI (Plugin Debug Shell)

## Context

Developers debugging plugin issues had no way to manually interact with the engine using the plugin protocol. The existing `ze bgp plugin cli` connected via SSH and executed one-shot commands, but could not perform the 5-stage plugin handshake or send SDK-level RPCs. To reproduce plugin bugs, developers had to write and compile a test plugin, which was slow and cumbersome.

## Decisions

- SSH channel as plugin transport, over TLS connect-back, because SSH already handles auth and provides a bidirectional stream. This eliminated the need for a prepare-session RPC, acceptor changes, and YANG schema additions.
- Single Q&A flow with defaults, over separate auto/manual modes, because one flow with Enter-for-defaults covers both cases with simpler UX.
- Q&A happens locally on the terminal before the SSH session opens, over mixing prompts and protocol on the same stream, because MuxConn framing and human-readable prompts cannot share a single stream.
- `sdk.NewWithIO(name, reader, writer)` constructor added, over wrapping SSH channels in net.Conn adapters, because `rpc.NewConn` already accepts `io.ReadCloser`/`io.WriteCloser` (landed in spec-exabgp-bridge-muxconn).
- `HandleAdHocPluginSession` on the plugin Server, over modifying process startup, because ad-hoc sessions run with `coordinator == nil` (barriers skipped) and need no tier synchronization.
- `Process.SetConn()` to bypass rawConn/InitConns, over changing rawConn type from net.Conn, because SetConn already existed for tests and avoids touching the process lifecycle.

## Consequences

- Developers can now manually speak the plugin protocol against a running daemon for debugging.
- `sdk.NewWithIO` enables any io.ReadCloser/io.WriteCloser as plugin transport (SSH, pipes, etc.).
- The SSH server gains a new command type (`plugin protocol`) alongside one-shot and streaming commands.
- The `HandleAdHocPluginSession` pattern can be reused for other ad-hoc plugin connections (e.g., web-based debug shell).

## Gotchas

- The initial design used TLS with a prepare-session RPC, which was massively overcomplicated. The user's insight that SSH already provides auth cut through all that complexity.
- `handleProcessStartupRPC` works with `coordinator == nil` by design (stageTransition returns true) but this was not obvious from reading the code -- required tracing the nil check.
- `ssh.Session` from the Wish library embeds `gossh.Channel` (Read/Write/Close) which satisfies both `io.ReadCloser` and `io.WriteCloser`, so `rpc.NewConn(sess, sess)` works directly.

## Files

- `cmd/ze/bgp/cmd_plugin.go` -- rewritten: Q&A, persistent SSH session, SDK handshake, interactive mode
- `cmd/ze/internal/ssh/client/client.go` -- added `OpenProtocolSession` for bidirectional SSH
- `internal/component/ssh/ssh.go` -- added `PluginProtocolFunc` and detection in `execMiddleware`
- `internal/component/plugin/server/adhoc.go` -- new: `HandleAdHocPluginSession`
- `pkg/plugin/sdk/sdk.go` -- added `NewWithIO` constructor
- `docs/features.md` -- plugin debug shell entry
- `docs/guide/command-reference.md` -- updated `ze bgp plugin` commands
- `docs/guide/plugins.md` -- added Debugging Plugins section
