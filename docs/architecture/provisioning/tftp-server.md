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
- **A channel semaphore bounds concurrency.** The `select` with a `default` arm
  rejects instantly instead of blocking, which a wait group or an atomic counter
  would not give.
- `SO_BINDTODEVICE` handles interface binding on Linux, so a changing address
  does not break the binding. Non-Linux binds to all interfaces.

<!-- source: internal/plugins/tftpserver/socket_linux.go -- SO_BINDTODEVICE binding -->
<!-- source: internal/plugins/tftpserver/socket_other.go -- the non-Linux fallback -->

## Traps

- **Block numbers wrap at 65535**, which caps a single transfer near 32 MB at
  the default block size. PXE bootloaders are 1 to 5 MB. A larger negotiated
  block size lifts the cap.
- **A file truncated mid-transfer produces either an error packet or a short
  data block**, depending on timing: a read can return zero bytes or an error.
  Both outcomes are correct and a test must accept both.
- The retransmit test waits for the real ACK timeout and takes about 5 seconds
  of wall clock. Making the timeout configurable to shorten it would add an
  option nothing else needs.

## Related

- `image-server.md` for the HTTP half of the same boot
- `pxe-staging.md` for what gets staged into the TFTP directory
