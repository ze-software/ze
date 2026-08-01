# Learned: spec-bng-5 -- PPPoE Access Concentrator

## What was built

RFC 2516 PPPoE access concentrator as an alternative to L2TP for direct-attach BNG, in 8 phases across 2 commits:

1. **Discovery wire format** (`discovery.go`): parse/build PADI, PADO, PADR, PADS, PADT with all standard tags. 18 unit tests.
2. **AC-Cookie** (`cookie.go`): HMAC-SHA256 cookie with timestamp for DoS protection. 9 tests.
3. **Session management** (`session.go`): bitmap-backed SID allocation (1-65535), per-interface session tables with MAC index. 9 tests.
4. **Kernel integration** (`kernel_linux.go`): AF_PACKET discovery socket, AF_PPPOX + PX_PROTO_OE session sockets, interface resolution via ioctl.
5. **Subsystem + server** (`subsystem.go`, `server.go`, `config.go`, `service.go`, `ratelimit.go`): discovery reader goroutine, per-interface dispatch, PPP Driver integration, PADI rate limiting.
6. **YANG + CLI + hub** (`schema/`, `cmd/pppoe/`, hub wiring): 3 YANG schemas, 5 CLI RPC handlers, hub registration.
7. **StartSession extensions** (`start_session.go`): AccessInterface, SubscriberMAC, ServiceName, VendorTags for RADIUS passthrough.
8. **devPPPSetup extraction** (`ppp/devppp_linux.go`): shared /dev/ppp ioctl sequence for both L2TP and PPPoE.

Total: 29 new files, 41 unit tests, 3 functional test scripts (Linux-only).

## Key decisions

- **ifindex as TunnelID, PPPoE SID as SessionID.** PPP Driver treats these as opaque keys; natural scope mapping.
- **Single AF_PACKET/SOCK_RAW per namespace.** One socket handles all access interfaces; dispatch by ifindex from recvfrom. Matches accel-ppp pattern.
- **Per-interface session state.** Each interface gets its own SessionTable with independent SID space (full 1-65535 range). No global lock contention.
- **HMAC-SHA256 cookie (not MD5+DES).** Modern, hardware-accelerated, simpler than accel-ppp's approach.
- **PADS sent after kernel setup.** If sent before and kernel fails, subscriber waits for LCP that never comes.
- **Remove() returns PppoxFD atomically.** Prevents double-close between discovery reader and event consumer.

## What surprised us

- **StartSession grew past hugeParam.** Adding 4 fields (28 bytes of string/slice headers) pushed it to 304 bytes. All internal pass-by-value sites (spawnSession, emitRejection, run, test helpers) had to switch to pointer. The channel type stays value since it owns the data transfer.
- **devPPPSetup duplication was exact.** L2TP and PPPoE had character-for-character identical ioctl sequences (differing only in error message prefix). Extraction to `ppp.DevPPPSetup()` removed 148 lines with zero behavioral change.
- **LookupSession race was subtle.** `Lookup()` returns a `*Session` pointer after releasing the lock, but handlePADR mutates session fields (State, UnitNum, PppoxFD) after `Add()` without re-acquiring. The `Sessions()` snapshot method was safe (copies under lock), but the single-session lookup was not. Fixed with `LookupSnapshot()`.
- **SIOCGIFHWADDR has no Go accessor.** `x/sys/unix.Ifreq` lacks a hardware address method (upstream TODO). Required unsafe pointer arithmetic into the raw ifreq union at known offsets.

## Patterns worth reusing

- **Snapshot + LookupSnapshot pair.** When a table's `Lookup()` returns a live pointer but fields are mutated after insertion, provide a `LookupSnapshot()` that copies under the lock. CLI handlers use snapshots; hot-path code uses raw pointers.
- **YANG triple (conf + api + cmd).** Config schema, API RPCs, and CLI tree are three separate YANG modules. The conf and api modules are embedded in the component's `schema/` package; the cmd module is in `cmd/<name>/yang/`. Blank imports in the CLI handler wire everything.
- **Transport-agnostic PPP integration.** New transports (PPPoE, future IPoE) feed `ppp.StartSession` with transport-specific fields. The PPP Driver is oblivious to the transport. Shared kernel setup lives in `ppp/devppp_linux.go`.

## Remaining integration work

- AC-3: full PPP LCP/Auth/IPCP over PPPoE (requires Linux kernel, covered by functional tests)
- AC-14: NAS-Port-Type=Ethernet in RADIUS accounting (requires RADIUS plugin integration)
- Subsystem.Reload: currently a no-op; config changes require daemon restart
- TR-101 vendor tag parsing (only first Vendor-Specific tag captured; sub-option parse is follow-up)
- PADO delay, VLAN auto-discovery, MAC filter (deferred to future specs)

## Files

None recorded.
