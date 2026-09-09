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

**A PPPoE session reads its RADIUS rate profile from the shared session
metadata store, not from a PPPoE-specific path.** The RADIUS auth handler stores
the Access-Accept attributes under the `(tunnel id, session id)` pair the PPP
driver carried, and for PPPoE that pair is `(ifindex, PPPoE session id)`, which
`onSessionUp` already holds. So the subscriber's Filter-Id rates reach
`subscriber.Session` with no new plumbing and no second attribute path. The two
rate fields had no producer at all until 2026-09-06: every PPPoE session handed
the shaper a zero pair and was shaped at the configured default whatever the
RADIUS server answered.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -- onSessionUp, the LoadSessionMetadata read -->

**Ze refuses a PADR with no Service-Name tag and serves a PADI with none.**
accel-ppp does exactly this: `pppoe_recv_PADR` discards a tagless PADR, and
`pppoe_recv_PADI` starts its match true when nothing is configured. FreeBSD's
`ng_pppoe` tolerates both packets instead, substituting an empty tag on the
PADI and keying the PADR admission on the cookie alone. RFC 2516 Sections 5.1
and 5.3 bind the sending host, not the AC, so this choice is about robustness
against a malformed peer, not about conformance.

<!-- source: internal/component/l2tp/pppoe/server.go -- requireServiceNameTag, handlePADR -->

**Ze always emits exactly one Service-Name tag in the PADO and the PADS,
including the zero-length case.** RFC 2516 Section 5.2 requires a Service-Name
tag identical to the PADI's in every PADO, and Section 5.4 requires exactly
one in every PADS. `AddTagCopy` skips a nil tag, which is correct for the
genuinely optional Host-Uniq and Relay-Session-Id tags, so `BuildPADO` and
`BuildPADS` write the Service-Name tag through `AddTagString` instead: an
absent or zero-length source tag becomes a zero-length echo, never no tag at
all. Both reference implementations are less conformant here. FreeBSD's
`ng_pppoe` PADO can carry zero tags or two, and its PADS carries none when the
PADR carried none. accel-ppp's PADO carries none when nothing is configured
and the PADI carried none; its PADS already matches Ze.

<!-- source: internal/component/l2tp/pppoe/discovery.go -- BuildPADO, BuildPADS -->

**Every discovery refusal increments `ze_pppoe_discovery_refusals_total`,
labelled by `reason`.** A PADI refused for an unoffered service name gives no
reply on the wire, because RFC 2516 Section 5.2 requires that, so the counter
is the only visibility that refusal has. The reason set is closed:
`rate-limited`, `service-name-mismatch`, `service-name-missing`,
`cookie-invalid`, `session-id-exhausted`. Where a refusal already logs at
Debug, the log line carries the same reason string, so the log and the
counter are one vocabulary, not two. The rate-limiter refusal counts with no
paired log line: a PADI flood would otherwise turn Debug logging itself into
the resource problem the limiter exists to prevent. One counter set serves
every interface server, so a refusal on any access interface accumulates on
one series per reason.

**`Subsystem.Start` registers the counters with the plugin registry rather
than reading one out of it.** On a PPPoE-only daemon the metrics registry does
not exist yet at that moment: `runYANGConfig` runs `engine.Start`, which runs
`Subsystem.Start`, and only afterwards runs `startStandaloneTelemetry`, which
creates the registry. A daemon that also carries a `bgp` block gets its
registry earlier, from the reactor's config loader during plugin start, so
which event happens first is a property of the operator's configuration.
Reading `registry.GetMetricsRegistry` at `Start` therefore binds nothing on
the PPPoE-only deployment and leaves `ze_pppoe_discovery_refusals_total`
absent for the process lifetime, with no log line saying so.
`registry.InjectPluginMetrics` holds the hook until a registry arrives, so
whichever event happens second does the binding.

<!-- source: internal/component/l2tp/pppoe/metrics.go -- registerDiscoveryMetrics, bindPPPoEMetrics, countRefusal -->
<!-- source: internal/component/l2tp/pppoe/server.go -- the six call sites that count a refusal -->

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
