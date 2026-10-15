# TFTP server

A PXE ROM fetches the bootloader over TFTP before it switches to HTTP for the
larger payloads. RFC 1350 is five opcodes, 512-byte blocks, and stop-and-wait,
so an in-tree server costs less than an external dependency. RFC 2347 option
negotiation is supported.

<!-- source: internal/plugins/tftpserver/register.go -- plugin registration and engine lifecycle -->
<!-- source: internal/plugins/tftpserver/handler.go -- parseRRQ, resolvePath, serveFile, sendAndWaitACK -->
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
- **Only the ACK timeout causes a retransmission.** A duplicate or stale ACK
  gets no DATA in answer (RFC 1350 section 5, the Sorcerer's Apprentice fix):
  `sendAndWaitACK` keeps reading until the same deadline. After four sends
  without the expected ACK the transfer ends and its slot is released.
- **A channel semaphore bounds concurrency.** The `select` with a `default` arm
  rejects instantly instead of blocking, which a wait group or an atomic counter
  would not give.
- `SO_BINDTODEVICE` handles interface binding on Linux, so a changing address
  does not break the binding. Non-Linux binds to all interfaces.

<!-- source: internal/plugins/tftpserver/socket_linux.go -- SO_BINDTODEVICE binding -->
<!-- source: internal/plugins/tftpserver/socket_other.go -- the non-Linux fallback -->

## Config reload

A reload that changes nothing under `service tftp-server` leaves the server
alone: `sdk.ConfigTx` finds the parsed config equal to the running one and
installs nothing. A reload that changes it closes the listeners and binds them
again with a new transfer semaphore, and logs `tftpserver: config applied`. A
request that arrives during the swap is lost and the client retransmits it.
Every transfer runs under the generation of the config that started it. Once
the new listeners serve, the replaced generation is cancelled, which closes
the socket of every transfer still in flight: the transfer is cut, and the
client fetches the file again under the new config. No file is served from a
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
says which happened, and the fix is to free the port, then `ze signal restart`
for a listener a restarted plugin could not bind. A commit of the running
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

## Traps

- **Block numbers wrap at 65535**, which caps a single transfer near 32 MB at
  the default block size. PXE bootloaders are 1 to 5 MB. A larger negotiated
  block size lifts the cap.
- **A file truncated mid-transfer produces either an error packet or a short
  data block**, depending on timing: a read can return zero bytes or an error.
  Both outcomes are correct and a test must accept both.
- The retransmit tests wait for the real ACK timeout. The abort test takes
  about 27 seconds of wall clock, four 5-second timeouts and a quiet window. Making the timeout configurable to shorten it would add an
  option nothing else needs.

## Related

- `image-server.md` for the HTTP half of the same boot
- `pxe-staging.md` for what gets staged into the TFTP directory
