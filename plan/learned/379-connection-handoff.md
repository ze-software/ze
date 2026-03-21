# 379 — Connection Handoff

## Objective

Enable plugins to receive listen sockets from the engine via SCM_RIGHTS fd passing over Unix domain sockets, similar to systemd socket activation.

## Decisions

- Raw SCM_RIGHTS (1 framing byte + ancillary data) instead of YANG RPC wrapper — simpler, avoids FrameReader ordering issues
- Fd receive between Stage 1 and Stage 2 — must happen before FrameReader starts on Socket B, or the framing byte is consumed and ancillary fd data is silently lost
- Text-mode plugins skip connection handoff with a warning — TextConn's buffered reader conflicts with raw socket reads
- Port validation extracted to standalone `validHandoffPort()` for boundary-testable logic without I/O
- Python SDK detects socket family via `getsockname()` probe — supports both IPv4 and IPv6 listeners

## Patterns

- `SendFD`/`ReceiveFD` as thin wrappers over `WriteMsgUnix`/`ReadMsgUnix` + `unix.UnixRights`
- Always call `unix.CloseOnExec(fd)` after `ParseUnixRights()` — macOS lacks `MSG_CMSG_CLOEXEC`
- `socket.fromfd()` in Python dups the fd — must close the dup'd socket after `recvmsg()` to avoid fd leak
- SDK `Run()` auto-receives listeners in a loop between Stage 1 OK and Stage 2, using `range reg.ConnectionHandlers` to know how many fds to expect
- `Plugin.Close()` must close received listeners to prevent socket leaks

## Gotchas

- FrameReader timing is fragile: any future refactoring that adds a `callbackConn.ReadRequest()` before the fd receive loop breaks handoff silently (no error — just lost ancillary data)
- `rpc.Conn.WriteConn()` returns the raw `net.Conn` needed for SCM_RIGHTS — this accessor was added specifically for this feature
- Internal plugins (`net.Pipe`) cannot support fd passing — `SendFD` type-asserts `*net.UnixConn` and returns a clear error
- Free port allocation in tests has a TOCTOU race (bind to 0, get port, close, rebind) — acceptable for tests, not production
- Testing port boundary values that require `net.Listen` makes tests environment-dependent — extract validation into a pure function instead

## Files

- `internal/component/plugin/ipc/fdpass.go` — `SendFD()` / `ReceiveFD()`
- `internal/component/plugin/server/startup.go` — `handoffListenSockets()`, `validHandoffPort()`
- `internal/component/plugin/registration.go` — `ConnectionHandler` struct
- `pkg/plugin/rpc/types.go` — `ConnectionHandlerDecl`
- `pkg/plugin/sdk/sdk.go` — `ReceiveListener()`, `Listeners()`, auto-receive in `Run()`
- `test/scripts/ze_api.py` — Python `declare_connection_handler()`, `receive_listener()`
- `docs/architecture/api/process-protocol.md` — user documentation
- `test/plugin/handoff-listen.ci` — functional test (end-to-end fd handoff)
- `test/plugin/handoff-no-declare.ci` — regression test (no connection-handlers)
