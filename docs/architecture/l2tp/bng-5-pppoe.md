# PPPoE access concentrator

A PPPoE access concentrator, as an alternative to L2TP for a direct-attach
broadband network gateway. Discovery wire format, AC-Cookie, session tables,
kernel sockets, the discovery reader, and the CLI surface.

<!-- source: internal/component/l2tp/pppoe/discovery.go -- ParseDiscovery, Builder, MatchServiceName -->
<!-- source: internal/component/l2tp/pppoe/cookie.go -- GenerateCookie, VerifyCookie, CookieKey -->
<!-- source: internal/component/l2tp/pppoe/session.go -- SessionTable, AllocSID, Add, Remove, Lookup -->
<!-- source: internal/component/l2tp/pppoe/kernel_linux.go -- AF_PACKET and AF_PPPOX sockets -->
<!-- source: internal/component/l2tp/pppoe/subsystem.go -- Subsystem, discoveryReader, eventConsumer -->
<!-- source: internal/component/l2tp/pppoe/server.go -- InterfaceServer, handlePADI, handlePADR, handlePADT -->

## RFC obligations carried by this code

RFC 2516 defines the PPPoE discovery stage. The five packet types are PADI,
PADO, PADR, PADS and PADT, and the tag set is the standard one. The reference
summary is `rfc/short/rfc2516.md`.

## Decisions

**The interface index is the tunnel id and the PPPoE session id is the session
id.** The PPP driver treats both as opaque keys, so this is the natural scope
mapping and needs no translation layer.

**One AF_PACKET raw socket per namespace.** A single socket handles every access
interface, and dispatch is by the interface index from the receive call. This
matches the accel-ppp design.

**Session state is per interface.** Each interface holds its own session table
with an independent session id space over the full range 1 to 65535. There is no
global lock to contend on.

**The AC-Cookie is HMAC-SHA256 with a timestamp.** It is hardware accelerated
and simpler than the MD5 and DES construction accel-ppp uses. The timestamp
bounds replay.

**PADS is sent AFTER the kernel setup succeeds.** Sending it first and then
failing the kernel setup leaves the subscriber waiting for an LCP exchange that
never starts.

**`Remove()` returns the socket descriptor atomically.** The discovery reader
and the event consumer would otherwise both close it.

**PADI is rate limited per client MAC address.**

<!-- source: internal/component/l2tp/pppoe/ratelimit.go -- PADILimiter, Check -->

## Traps this code exists to avoid

**`Lookup` returns a live pointer and the caller mutates it.** `handlePADR`
mutates the session state, the unit number and the socket descriptor after
`Add`, without re-acquiring the lock. The snapshot method was safe because it
copies under the lock; the single-session lookup was not. `LookupSnapshot` is
the safe pair.

**Snapshot for the CLI, raw pointer for the hot path.** That is the rule
whenever a table hands out a pointer that is mutated after insertion.

<!-- source: internal/component/l2tp/pppoe/snapshot.go -- Sessions, LookupSnapshot, Snapshot -->

**A struct that crosses the huge-parameter threshold changes every call site.**
Adding four fields to the PPP session start struct pushed it to 304 bytes, and
every pass-by-value site had to become a pointer. The channel type stays a value
because it owns the transfer.

**`SIOCGIFHWADDR` has no Go accessor.** The `unix.Ifreq` type carries no
hardware-address method. Reading it needs pointer arithmetic into the raw
request union at known offsets.

## Patterns worth reusing

- **The YANG triple.** Config schema, API RPCs and the CLI tree are three
  separate YANG modules. The config and API modules are embedded in the
  component's schema package; the command module lives under the CLI handler's
  yang directory. Blank imports in the CLI handler wire all three.
- **Transport-agnostic PPP integration.** A new transport feeds the PPP session
  start call with its own transport-specific fields, and the PPP driver stays
  unaware of the transport. The shared kernel setup lives in one place.

<!-- source: internal/component/l2tp/ppp/devppp_linux.go -- DevPPPSetup -->
<!-- source: internal/component/l2tp/ppp/devppp_other.go -- non-Linux stub -->

The shared setup came out of an exact duplicate: L2TP and PPPoE carried
character-for-character identical ioctl sequences that differed only in the
error message prefix. Extracting it removed 148 lines with no behavior change.
