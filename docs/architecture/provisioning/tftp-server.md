# TFTP server

A PXE ROM fetches the bootloader over TFTP before it switches to HTTP for the
larger payloads. RFC 1350 is five opcodes, 512-byte blocks, and stop-and-wait,
so an in-tree server costs less than an external dependency. RFC 2347 option
negotiation is supported, with the blksize (RFC 2348), timeout and tsize
(RFC 2349) and windowsize (RFC 7440) options.

The wire facts the server shares with Ze's TFTP client (the tftp scheme of
`ze init --from`): opcodes, error codes, the block sizes and the ERROR encoder
are declared once in the core package `internal/core/tftp`, which both import.
The server keeps its own request parser and its DATA and OACK encoders, which
only it uses. The client is described in
`docs/architecture/appliance/on-device-installer.md`.

<!-- source: internal/core/tftp/tftp.go -- OpRRQ, CodeNotDefined, DefaultBlockSize, BlksizeEthernet, BuildError -->
<!-- source: internal/plugins/tftpserver/register.go -- plugin registration and engine lifecycle -->
<!-- source: internal/plugins/tftpserver/handler.go -- parseRRQ, handleRRQ, grantWindowsize, sendOACKAndWait, resolvePath, sendAndWaitACK -->
<!-- source: internal/plugins/tftpserver/window_send.go -- serveFile, windowSender.sendWindow, windowSender.awaitACK, windowPosition -->
<!-- source: internal/plugins/tftpserver/config.go -- config parsing and verification -->

## Decisions

- **A plugin, not a library.** It participates in the config and lifecycle
  system like every other service, so any Ze device can serve TFTP from config,
  not only the provisioning host.
- **Read-only.** A write request is always rejected with error code 4. The only
  use case is serving bootloader files, and refusing writes removes the whole
  write attack surface.
- **One ephemeral port per transfer.** RFC 1350 section 4 pairs transfer IDs
  this way. It also keeps the port-69 listener free for new requests.
- **A window, then a wait.** A transfer sends `windowsize` DATA blocks, then
  waits for an ACK (RFC 7440 section 4). With no windowsize acknowledged the
  window is one block, which is RFC 1350 lockstep: a request that names
  neither option is sent exactly as before. A client's windowsize is granted
  as the smaller of its request and `max-windowsize` (default 64), and a grant
  of one block is not acknowledged, so `windowsize 1` alone draws no OACK
  (tftp-hpa's `negotiate_windowsize` does the same).
- **Where the next window starts.** An ACK of the window's last block starts
  the next window after it. An ACK of an earlier block of the window starts a
  full new window straight after that block, as the RFC 7440 section 4 figure
  shows. A timeout sends the window again from the block after the last ACK
  received. Each block is read from the file at its offset, so a window sent
  again costs no buffer: the transfer holds one block whatever its window.
  Block numbers wrap at 65535 on the wire, so an ACK is placed by its 16-bit
  distance from the block before the window.
- **Only the ACK timeout causes a retransmission.** A duplicate or stale ACK
  gets no DATA in answer (RFC 1350 section 5, the Sorcerer's Apprentice fix):
  the ACK of the block before the window and an ACK of a block not yet sent
  are ignored, and the wait goes on until the same deadline. After four sends
  of a window without progress the transfer ends and its slot is released.
- **The timeout is the client's, or the config's.** A client's RFC 2349
  timeout of 1 to 255 seconds is acknowledged with its own value and used for
  the ACK 0 wait and every window. Any other value is not acknowledged (the
  RFC lets a server decline, never counter-propose), and the transfer uses
  the `timeout` leaf (default 5 seconds). tftp-hpa's `negotiate_timeout`
  declines the same way; dnsmasq clamps to 255 instead, which Ze does not
  copy.
- **A channel semaphore bounds concurrency.** The `select` with a `default` arm
  rejects instantly instead of blocking, which a wait group or an atomic counter
  would not give.
- `SO_BINDTODEVICE` handles interface binding on Linux, so a changing address
  does not break the binding. Non-Linux binds to all interfaces.

<!-- source: internal/plugins/tftpserver/socket_linux.go -- SO_BINDTODEVICE binding -->
<!-- source: internal/plugins/tftpserver/socket_other.go -- the non-Linux fallback -->

## Config reload

A reload that changes only `timeout` or `max-windowsize`, while every
`listen-interface` is bound, keeps the listeners, the semaphore and every
transfer in flight. With an interface left unbound at boot it takes the full
path below, which tries that interface again; while that interface is still
missing the strict bind refuses the reload, the limits change included, and
the running listeners and transfers go on. Each request takes the values
running when it arrives, so a transfer keeps the timeout and window it
started with, and the next request gets the new ones.
<!-- source: internal/plugins/tftpserver/config_tx.go -- tftpServer.install, tftpServer.onlyLimitsChange, limitsOf -->

A reload that changes nothing under `service tftp-server` leaves the server
alone: `sdk.ConfigTx` finds the parsed config equal to the running one and
installs nothing. A reload that changes it closes the listeners and binds them
again with a new transfer semaphore, and logs `tftpserver: config applied`. A
request that arrives during the swap is lost and the client retransmits it.
Every transfer runs under the generation of the config that started it. Once
the new listeners serve, the replaced generation ends, which closes the socket
of every transfer still in flight before the reload returns: the transfer is
cut, and the client fetches the file again under the new config. A transfer
that a replaced listener starts after that is refused. No file is served from a
root directory the operator replaced, and `max-transfers` holds at once. A
reload that sets `enabled false` stops serving the same way.

A reload binds every `listen-interface` or none. When one cannot be bound, the
apply binds the replaced config's listeners again under the replaced
generation, so its transfers go on, and returns an error naming the interface
and the fix, so the commit fails and rolls back. The replaced listeners are
bound one by one, so when another process takes the port of one of them in that
window, the others still serve. The error then wraps
`sdk.ErrRunningConfigLost`, says how many of the interfaces that served are
lost and names each one with its cause, and ends with
`sdk.RunningConfigLostNext`: the rollback binds the running config again and
restarts the plugin if that fails too, the reload answer's `Rollback:` line
says which happened, and the fix is to free the port, then, for a listener a
restarted plugin could not bind, to stop the whole daemon and start it again,
which restarts every protocol and drops every session. The text names the
restart routes of `sdk.RestartSteps()`, one per line: no command restarts one
plugin, and `ze signal restart` leaves ze stopped. A commit of the running
config is no fix: it has no diff, so no apply reaches the plugin. The
transaction is broken: its rollback installs the running config again, and when
that fails too it answers an error, so the bridge acks `CodeBroken` and the
engine restarts the plugin; until an install succeeds, a later apply of the
running config installs it rather than skipping it as unchanged. The boot
configure is not strict: a configure error at boot fails the plugin and aborts
the whole daemon start, so it logs `listen failed` for an interface and serves
on the others, or logs `no interfaces bound` when none binds. A VLAN, bridge or
dummy that ze's own `interface {}` config creates exists by then, because the
server starts after the interface plugin (`StartAfter: ["interface"]`). An
interface that appears later is not bound until the next reload.
<!-- source: internal/plugins/tftpserver/config_tx.go -- tftpServer.install, tftpServer.restore, bindTFTP; pkg/plugin/sdk/config_tx.go -- ConfigTx.Apply, ConfigTx.Rollback -->
<!-- source: internal/plugins/tftpserver/handler.go -- handleRRQ -->

## Panic containment

A Ze defect met while serving one client costs that client only. The `tftp`
guard wraps two tasks, and each contained panic is logged, counted in
`ze_task_panics_total{component="tftp",task=...}` and crash-recorded.

| Task | Wraps | After a panic |
|------|-------|---------------|
| `request` | `serveRequests`, the listener's read loop and `handleRRQ` | The client is sent an ERROR with code 0 from the listening socket, and `serve` starts the next pass on the same socket |
| `transfer` | `runTransfer`, one transfer on its own socket | The client is sent an ERROR with code 0 on the transfer's socket, which is still open, then the socket leaves its generation and closes and the semaphore slot is freed by `defer` |

RFC 1350 section 2 says most errors end the connection and are signaled by an
error packet that is never acknowledged or retransmitted, so a lost ERROR costs
the client its timeout. `serveFile` closes its file by `defer`, and the
generation's mutex is released by `defer`, so the unwind leaks no file and
holds no lock.
<!-- source: internal/plugins/tftpserver/panic_guard.go -- tftpGuard, requestTask, transferTask, signalPanic -->
<!-- source: internal/plugins/tftpserver/handler.go -- serveRequests, runTransfer -->

## Traps

- **Block numbers wrap at 65535**, which caps a single transfer near 32 MB at
  the default block size. PXE bootloaders are 1 to 5 MB. A larger negotiated
  block size lifts the cap.
- **A file truncated mid-transfer produces either an error packet or a short
  data block**, depending on timing: a read can return zero bytes or an error.
  Both outcomes are correct and a test must accept both.
- The RFC 1350 retransmit tests wait for the real five-second ACK timeout. The
  abort test takes about 27 seconds of wall clock, four 5-second timeouts and
  a quiet window. The window and timeout tests ask for the RFC 2349 `timeout 1`
  instead, which keeps them to seconds.

## Related

- `image-server.md` for the HTTP half of the same boot
- `pxe-staging.md` for what gets staged into the TFTP directory
