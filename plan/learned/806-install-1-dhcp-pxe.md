# 806 -- install-1-dhcp-pxe

## Context

Ze needed PXE boot support in its DHCP server plugin to enable bare-metal provisioning. The existing dhcpserver plugin (RFC 2131/2132) had clean buffer-first packet building with `safeAppendOption()` and a per-subnet handler model. The goal was to add PXE option detection and injection as an additive extension without forking or duplicating the handler.

## Decisions

- **PXE as additive extension** over separate PXE-specific handler: PXE logic lives entirely within existing `buildReply()` flow via `appendPXEOptions()`, called only when both `pxe.Enabled` and `isPXEClient(req)` are true. Zero impact on non-PXE path.
- **Server-wide pxeConfig** over per-subnet PXE config: PXE boot servers typically serve a single network segment. The pxeConfig struct is a field on `serverConfig`, threaded through `startServer()` to each `newDHCPHandler()`.
- **Reply buffer 1500 bytes (Ethernet MTU)** over original 576: PXE options add ~60 bytes; 576 left <100 bytes headroom. 1500 is the natural ceiling for unfragmented DHCP. Negligible allocation cost (one per reply).
- **Option 93 default to BIOS (0)** over rejecting PXE clients without architecture option: some older PXE ROMs omit option 93. Defaulting to BIOS is the standard practice.
- **Option 43 with boot item suboption (type 71)** included: some PXE ROMs require vendor-specific option 43 with a PXE boot item to proceed. Added as a fixed 7-byte payload.

## Consequences

- DHCP PXE is config-driven: any ze device can become a PXE provisioning server by adding a `pxe {}` block under `dhcp-server`.
- The `newDHCPHandler()` signature is now 3 arguments (`sub, serverIP, pxe`). All existing test helpers pass `pxeConfig{}` (zero value) for the third argument.
- TFTP server and image server (specs 2-3) are independent plugins that PXE directs clients to; no direct import between them and dhcpserver.

## Gotchas

- `siaddr` (bytes 20-23 of DHCP header) must be set to TFTP server IP for PXE. Some PXE ROMs check only `siaddr` and ignore option 66, so both must be set.
- `isPXEClient()` checks for 10-byte prefix "PXEClient:" in option 60, not exact match. The full option 60 includes architecture and UNDI version strings that vary by client.
- `parsePXEArch()` validates that option 93 length is exactly 2 bytes and even, returning 0 (BIOS) for any malformed input. RFC 4578 allows multiple architecture types but only the first is used.

## Files

- `internal/plugins/dhcpserver/handler.go` -- PXE option constants, isPXEClient, parsePXEArch, appendPXEOptions, parseOptionBytes
- `internal/plugins/dhcpserver/config.go` -- pxeConfig struct, parsePXEConfig
- `internal/plugins/dhcpserver/register.go` -- PXE config threading through startServer
- `internal/plugins/dhcpserver/schema/ze-dhcp-server-conf.yang` -- pxe container
- `internal/plugins/dhcpserver/handler_test.go` -- 12 PXE tests
- `internal/plugins/dhcpserver/config_test.go` -- 3 PXE config tests
- `test/install/dhcp-pxe-config.ci` -- functional test
- `docs/guide/configuration.md` -- PXE config documentation
- `docs/guide/plugins.md` -- dhcpserver plugin entry
