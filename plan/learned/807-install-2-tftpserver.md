# 807 -- install-2-tftpserver

## Context

Ze needed a TFTP server plugin for PXE provisioning. When `ze install remote` boots a target machine, the PXE ROM fetches the bootloader via TFTP (port 69) before switching to HTTP for larger payloads. RFC 1350 is simple enough (5 opcodes, 512-byte blocks, stop-and-wait) that an in-tree implementation avoids external dependencies.

## Decisions

- **Plugin, not library.** TFTP server is a standard ze plugin (`registry.Register()` in `init()`, YANG config, `RunEngine`) over a standalone library, because it participates in the config/lifecycle system like all other ze services. Reusable by any ze device via config.
- **Read-only.** WRQ (write) is always rejected with ERROR code 4 over allowing writes with restrictions, because the only use case is serving bootloader files. Smaller attack surface.
- **Ephemeral port per transfer.** Per RFC 1350 Section 4, each transfer gets its own `net.DialUDP` connection over reusing the port-69 listener, so the main listener stays available for new requests and client TID pairing is correct.
- **Semaphore for concurrency limit.** Channel-based semaphore (`make(chan struct{}, cfg.MaxTransfers)`) over sync.WaitGroup or atomic counter, because the `select/default` pattern gives instant rejection without blocking.
- **SO_BINDTODEVICE on Linux.** Interface-specific binding uses `SO_BINDTODEVICE` via `ListenConfig.Control` over parsing interface addresses, because it handles dynamic address changes and is the standard kernel mechanism.

## Consequences

- The tftpserver plugin is available to any ze deployment via `service { tftp-server { ... } }` config, not only `ze install`.
- Block number wrap at 65535 limits single transfers to ~32MB. PXE bootloaders are 1-5MB, so this is fine for v1. RFC 2348 (blocksize option) would lift this if needed.
- Non-Linux platforms bind to all interfaces regardless of `listen-interface` config (socket_other.go fallback).

## Gotchas

- `fmt.Sprintf` is blocked by hook even on cold paths (config error messages). Use `fmt.Errorf` for errors, `b.WriteString()` chains for string building.
- `TestTFTPRetransmitOnTimeout` takes 5s wall-clock (waiting for the real ACK timeout). Cannot be shortened without making the timeout configurable, which is unnecessary complexity.
- File truncation mid-transfer produces either ERROR or short DATA depending on timing. Both are correct: `os.File.Read` may return 0 bytes (short block, ends transfer) or an error (ERROR packet sent). Tests accept both outcomes.

## Files

- `internal/plugins/tftpserver/register.go` -- plugin registration, RunEngine, config verify
- `internal/plugins/tftpserver/handler.go` -- TFTP protocol: parseRRQ, buildData, buildError, resolvePath, serve, handleRRQ, serveFile, sendAndWaitACK
- `internal/plugins/tftpserver/config.go` -- config parsing and verification
- `internal/plugins/tftpserver/handler_test.go` -- 20 unit tests
- `internal/plugins/tftpserver/config_test.go` -- 5 config tests
- `internal/plugins/tftpserver/socket_linux.go` -- SO_BINDTODEVICE binding
- `internal/plugins/tftpserver/socket_other.go` -- non-Linux fallback
- `internal/plugins/tftpserver/yang/` -- YANG schema, embed, register
- `test/install/tftp-server-config.ci` -- functional test
- `docs/guide/plugins.md` -- tftpserver entry in Infrastructure table
