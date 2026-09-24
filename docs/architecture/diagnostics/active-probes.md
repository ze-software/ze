# Active Probes: Ping, Traceroute and Route Lookup

An operator, or an agent through MCP, validates a forwarding path from the
router itself. A gokrazy appliance carries no `ping` and no `traceroute`
binary, so these are in-daemon ICMP socket implementations.

<!-- source: internal/component/ping/cmd/ping.go -- ICMP echo probe -->
<!-- source: internal/component/traceroute/cmd/traceroute.go -- per-hop TTL probe -->
<!-- source: internal/component/traceroute/cmd/probe_round.go -- batch probe round -->
<!-- source: internal/core/probe/socket.go -- OpenICMP, the one socket construction -->
<!-- source: internal/core/probe/df.go -- DFMode -->
<!-- source: internal/core/probe/socket_linux.go -- listenDatagramICMP, the unprivileged socket -->
<!-- source: internal/core/probe/doctor.go -- checkICMPProbeSocket, the doctor check -->
<!-- source: internal/component/iface/cmd/show_route_lookup.go -- kernel longest-prefix lookup -->

## No shelling out

Every prober opens its ICMP socket through one constructor,
`probe.OpenICMP` in `internal/core/probe/socket.go`, and builds the Echo
Request itself. Running the system `ping` binary would carry a
command-injection surface, would return text instead of structured output, and
would not exist on gokrazy at all.

`OpenICMP` takes the family, the bind address and the Don't Fragment mode, and
every caller passes all three. No prober calls `net.ListenPacket` on its own:
the one that did would have to repeat the socket options below, and the one
that forgot would fragment silently while reporting a measured path.

### The router's ICMP producer

The Linux data plane also implements ICMP for traffic Ze forwards and for
packets addressed to the router. Ze enables forwarding through the interface
sysctls and installs routes through its netlink backend. Linux `ip_forward`
discards packets whose TTL is exhausted or whose DF bit prevents fragmentation
at the outgoing MTU; `ip_rcv_core` and `ip_options_compile` reject unprocessable
headers. Linux generates the resulting ICMP errors.

`internal/plugins/vrrp/gateway_icmp_integration_linux_test.go` exercises this
boundary with Ethernet packet injection in an isolated namespace. It captures
both the forwarding path and the return path, with valid controls for each
discard condition, and checks reserved fields in emitted errors. RFC 1191
assigns the next-hop MTU field, and RFC 4884 assigns the quoted-datagram length
octet; these fields and Parameter Problem's pointer are excluded from the
unused-bit check. These tests require Ze's runtime kernel under QEMU.

## The Don't Fragment mode

`probe.DFMode` is a typed enum whose zero value, `DFUnspecified`, is refused
by `OpenICMP`. A caller names one of three modes.

| Mode | CLI spelling | Socket option on Linux | What the kernel does |
|------|--------------|------------------------|----------------------|
| `DFOff` | the keyword absent | `IP_MTU_DISCOVER=IP_PMTUDISC_DONT` | clears the DF bit and fragments a probe larger than the path, and reports nothing back. The option is set rather than left alone because Linux's default, `IP_PMTUDISC_WANT`, sets the DF bit on every datagram that fits the path: a probe that named no mode carried DF anyway, which `TestProbeDFBitOnTheWire` observed on the router's link |
| `DFHonorCache` | `do-not-fragment honor-cache` | `IP_MTU_DISCOVER=IP_PMTUDISC_DO`, `IP_RECVERR=1` | sets the DF bit, refuses a probe larger than its cached path MTU for the destination with `EMSGSIZE`, and updates that cache from a router's Fragmentation Needed or Packet Too Big answer |
| `DFBypassCache` | `do-not-fragment bypass-cache` | `IP_MTU_DISCOVER=IP_PMTUDISC_PROBE`, `IP_RECVERR=1` | sets the DF bit and ignores the cached value, so the probe is put on the wire at its full size and the path answers for itself. This is the mode `tracepath` measures in |

The IPv6 socket takes the `IPV6_` twins of the same options. Linux, not Ze,
runs the RFC 1191 and RFC 8201 path-MTU state machines: Ze installs the option
and reads what comes back, and its tests assert the option it installed
(`TestOpenICMPInstallsDFMode`, which reads the option back off the socket).
`IP_RECVERR` is set for the two DF modes so the reported next-hop MTU reaches
the socket's error queue, which the next section reads.

The keyword is the same word on `show ping`, `resolve ping`, `show traceroute`
and `resolve traceroute`, and it is additive: a command that never names it
asks for `DFOff` explicitly. `monitor ping`, `monitor traceroute` and
`show probe-round` open their sockets through the same constructor with
`DFOff` and carry no keyword yet.

Off Linux the socket layer has no `IP_MTU_DISCOVER`. The package still
compiles there (`socket_other.go`), a probe with DF off opens as before, and a
mode that sets DF is refused with `probe.ErrDFUnsupported` rather than opened
with the bit silently clear (`TestProbeCapabilityAbsentOffLinux`).

## The error queue

A DF probe is refused in one of two places, and Linux hands both to the
socket's error queue (`ai/rules/rfc-compliance.md` counts the kernel's RFC
1191 and RFC 8201 state machines as Ze's, so Ze reads the queue rather than
parsing the ICMP error itself).

| Refusal | What the kernel does | What Ze reads |
|---------|----------------------|---------------|
| A router on the path answers Fragmentation Needed or Packet Too Big | queues a `sock_extended_err` with `ee_errno=EMSGSIZE`, `ee_origin=ICMP`, `ee_info` = the reported next-hop MTU, the router's address as the offender, and the quoted echo header as the data; then sets `sk_err`, so the next ordinary read on the socket returns `EMSGSIZE` once | the receive loop treats that read error as the wake and drains the queue |
| This host's kernel refuses a send larger than its cached path MTU (honor-cache mode) | `WriteTo` fails with `EMSGSIZE`, and a `LOCAL` entry carrying the cached estimate is queued, quoting nothing | the sender drains the queue right after the failed write, before anything else reads it. On ping that drain is the same one that resolves a router's answer, so an answer queued for an earlier probe is not dropped by it |

`Socket.DrainErrors` (`internal/core/probe/socket.go`, over `DrainErrorQueue`
in `errqueue_linux.go`) reads the queue with `MSG_ERRQUEUE|MSG_DONTWAIT`, never blocks, and reads at most
`probe.ErrQueueDrainMax` entries in one call: a host flooding ICMP errors
cannot hold a probe goroutine in the drain. Each entry is a `probe.QueuedError`
whose `Outcome` is one of three names, and a zero is never one of them.

| Outcome | Meaning | `MTU` |
|---------|---------|-------|
| `ErrQueueEmpty` | nothing was queued | unset |
| `ErrQueueMTUReported` | a refusal was queued and its reported value is usable | the value, in octets |
| `ErrQueueMTUUnreported` | an error was queued and it carries no usable value: `EMSGSIZE` with a zero next-hop MTU, which RFC 1191 Section 3 makes an unmodified router's signal that a search must begin; `EMSGSIZE` with an IPv6 value below 1280, which RFC 8201 Section 4 discards (1280 is RFC 8200 Section 5); or another errno, an ICMP error that was never about size | unset |

The two families have opposite floor rules and neither is applied to the
other: on IPv4 a value below 68 is reported, raised to 68, because RFC 1191
Section 3 clamps the estimate and discards no message (68 is RFC 791). The
parser tests (`TestReportedMTUZeroIsNotAValue`,
`TestReportedMTUBelowIPv6MinimumIsDiscarded`,
`TestReportedMTUBelowSixtyEightIsNotDiscardedOnIPv4`) drive it over the
control-message bytes the kernel writes, so they run with no privilege.

The kernel matches a queued ICMP error to a raw socket by protocol and bound
address only, so an unconnected probe socket receives every error quoting an
ICMP datagram from this host (`TestProbeErrorQueueAndAnotherFlow` observed
one foreign entry). Each entry therefore carries the quoted echo header, and
every reader matches its identifier and sequence against the probe it sent
before it believes the value, which is the validation RFC 8201 Section 4
and RFC 8899 Section 4.6.1 ask for. `QueuedError.SizeRefusalOf` is that
match, shared by the ping session and the path MTU search
(`docs/architecture/diagnostics/path-mtu.md`), and `probe.ParseEchoReply`
is its twin for an ordinary read: the parser half of `BuildICMPEcho`, so a
reply is matched on the two fields the request was built with. An entry
quoting another probe leaves ours to time out as before.

Each entry also carries `Dest`, the destination of the refused datagram as
the kernel names the `MSG_ERRQUEUE` read. On a raw ICMP socket it is the
address alone; on a UDP socket it carries the port the refused datagram was
sent to, and a `LOCAL` entry carries port 0 there because the kernel fills
that entry from the socket's connected port, which an unconnected socket
has not got.

### A foreign socket

The option table and the parser are declared once and exported for a
socket this package did not open. The IKE transport
(`internal/component/ike/transport`, `docs/architecture/ike/ipsec-9-ikev2-eap-nat.md`)
is the second consumer: `probe.EnableErrorQueue` installs `IP_RECVERR` on
its two UDP sockets at creation, `probe.WithDFMode` toggles
`IP_MTU_DISCOVER` around one write and restores the prior value on every
exit path, and `probe.DrainErrorQueue` reads the queue. `WithDFMode` sets
a socket-wide option, so its caller MUST hold whatever serializes every
write on that socket for the whole call. Neither export has a non-Linux
stub of its own: the transport's Linux file is the only caller, and the
transport's own stub answers `probe.ErrDFUnsupported` there.

### What the payload says

A refused probe is a row of its own on ping and a hop record on traceroute,
with the same two keys on both, owned by the probe layer
(`probe.FieldNextHopMTU`, `probe.FieldNextHopMTUReported`):

| Key | Present | Value |
|-----|---------|-------|
| `next-hop-mtu-reported` | on every refused row | `true` when a usable value was reported |
| `next-hop-mtu` | only when the row says `true` | the reported next-hop MTU in octets. A zero is never written |

On `show ping` and `resolve ping` the row's `status` is `too-big` for a
refusal a router answered on this run, and `too-big-cached` for a send this
host's kernel refused against its cached path MTU: that probe never reached
the wire, the value is the cache's estimate rather than a router's answer, and
the batch summary leaves it out of `sent`. A batch in which any probe was
refused also carries the two keys on the summary, with the smallest reported
value, because RFC 1191 Section 3 lets an estimate only decrease on a report.
Every DF batch also carries `path-mtu` on the summary: the estimate the
kernel holds for the destination once the probes have run, read with
`probe.KernelPathMTU` (`IP_MTU` on a UDP socket connected to the
destination). It is the value the run left in the cache: a router's answer
lowered it, and a batch that met no refusal reports the route's MTU. The key
is absent when the kernel holds no estimate, and a zero is never written.
<!-- source: internal/component/ping/cmd/ping.go -- doPingCtx, readPathMTU -->
On `show traceroute` the refused hop names the router that answered (or `*`
for a refusal at send), carries the two keys, and ends the trace: a larger TTL
is answered by the same router the same way.

### The kernel's estimate

`probe.KernelPathMTU(ctx, dest)` answers the path MTU the kernel currently
holds for a destination, the value `IP_MTU` or `IPV6_MTU` reads. Linux answers
that option only on a socket that holds a route to the destination, so the
function connects a throwaway UDP socket (connect on UDP sends nothing and
needs no privilege) and reads the option off it: the estimate is a property of
the route, not of the socket that asks, so it is the same cache entry a probe
socket updated. It answers `probe.ErrPathMTUUnknown` for no route or no
estimate and never a zero.

### The proof against the kernel

`internal/core/probe/errqueue_integration_linux_test.go` (`integration &&
linux`, run by `./le qemu all-tests`) builds three namespaces joined by two
veth pairs, sender, router and far, with the router forwarding and its far
link clamped to 1400. It is the spec's interop scenario `probe-df-clamped-path`:
the Linux router is the other implementation.

| Test | Proves |
|------|--------|
| `TestProbeErrorQueueReportsNextHopMTU`, `...IPv6` | a 1500-octet DF probe yields one entry reporting 1400 from the router, quoting the probe, and the ordinary read returned `EMSGSIZE` first |
| `TestProbeBypassCacheDisagreesWithPoisonedCache` | after honor-cache learned 1400 and the clamp was lifted to 1500, `KernelPathMTU` reads 1400, a 1478-octet honor-cache send is refused with a local entry of 1400, and the same probe in bypass mode is answered |
| `TestProbeErrorQueueAndAnotherFlow` | what the kernel queues on one socket about another socket's flow, and that it quotes the other probe |
| `TestProbeDFBitOnTheWire` | from an `AF_PACKET` capture on the router's link: DF set for both DF modes and clear for `DFOff` |

Off Linux the drain answers `probe.ErrErrQueueUnsupported` and the estimate
`probe.ErrPathMTUUnsupported` (`errqueue_other.go`), never an empty queue or a
zero.

The operator's path is proven by three `.ci` tests in `test/plugin/`, each
building its own namespaces (`docs/functional-tests.md`, "A clamped path
built by the test itself"): `ping-do-not-fragment-reports-mtu` reads
`too-big` and `next-hop-mtu` 1400 off `show ping ... do-not-fragment
honor-cache` and `bypass-cache` through a daemon holding `CAP_NET_RAW`,
`ping-do-not-fragment-unprivileged` reads the same payload from a daemon
whose bounding set lost the capability, and `doctor-icmp-probe-missing`
reads `doctor-icmp-probe` from `ze doctor` in a namespace where neither
socket opens.

<!-- source: internal/core/probe/errqueue.go -- ErrQueueOutcome, QueuedError, SizeRefusalOf, FieldNextHopMTU -->
<!-- source: internal/core/probe/icmp.go -- BuildICMPEcho, ParseEchoReply -->
<!-- source: internal/core/probe/errqueue_linux.go -- DrainErrorQueue, classifyReportedMTU, KernelPathMTU -->
<!-- source: internal/core/probe/socket_linux.go -- EnableErrorQueue, WithDFMode -->
<!-- source: internal/component/ping/cmd/stream.go -- runPingSession, tooBigResult -->

Traceroute reuses ping's echo construction (`probe.BuildICMPEcho`) instead of
taking a dependency. The whole per-hop logic is about 200 lines, and a
third-party library for that would have to be evaluated and maintained.

Per-hop TTL control uses the `golang.org/x/net/ipv4` and `ipv6` PacketConn
wrappers, not raw syscalls. `x/net` was already a dependency, and it sets
TTL and hop limit across platforms. A `ttlSetter` interface hides
`SetTTL` against `SetHopLimit`, because the probe loop body is identical for
both families. IPv6 works through the same path, selected by `dest.Is6()`.

## The source address decides the family

`resolve traceroute <target> source <address>` binds the probe socket to that
address, and one socket carries one family. So the source is read before the
target is resolved, and the family of the source is the family the target
resolves in: an IPv6 source resolves a name to its AAAA record, an IPv4 source
to its A record. With no source the resolution stays family-agnostic and the
first answer wins.

A target with no address in the source family is refused by name, before any
socket is opened: `traceroute: source ::1 is IPv6 but target "192.0.2.1" has no
IPv6 address`. The older order resolved the target first and bound second, so
the operator got a bind failure that named the socket and neither argument.

`ResolveTarget` unmaps the address it answers with. `LookupNetIP` returns an
IPv4 answer in the IPv4-mapped IPv6 form, that form reports `Is6`, and the
socket family is read off that address.

<!-- source: internal/core/probe/icmp.go -- ResolveTarget, Family -->
<!-- source: internal/component/traceroute/cmd/resolve.go -- handleResolveTraceroute -->

## Reply matching

A reply is accepted only when the identifier and the sequence number match,
and the read loop skips everything else. On a shared host, other processes
are pinging at the same time on the same socket family. The identifier is the
socket's own, `Socket.Identifier`: two random octets chosen at open on the
raw kind, and the port the kernel bound on the datagram kind, so two probers
open at once in one daemon answer to different identifiers.

A Time Exceeded or Destination Unreachable answer is matched the same way.
`embeddedICMPOffset` (`internal/component/traceroute/cmd/traceroute.go`) reads
the IHL of the IP header the error quotes, finds the original ICMP header
behind it, and both `doTracerouteCtx` and `StreamProbeRound` compare the
quoted identifier and sequence with the probe they sent. An error that quotes
another process's probe is skipped. Only an error too short to carry the
quoted header (under 36 bytes for IPv4, 56 for IPv6) is accepted unmatched, so
a truncated answer still records the hop rather than a timeout.

## Bounds and privileges

Ping count is capped at 100 and the timeout at 30 seconds, so one CLI call
cannot hold resources indefinitely.

`OpenICMP` answers a `*probe.Socket` of one of two kinds, and the kind is
what privilege decides.

| Kind | Opened when | Who picks the identifier | What the ordinary read delivers |
|------|-------------|--------------------------|--------------------------------|
| `SocketRaw` | the daemon holds `CAP_NET_RAW` | Ze, at open | every ICMP datagram of the family, error messages included |
| `SocketDatagram` | the raw socket is refused with `EPERM` or `EACCES` and the daemon's group is inside `net.ipv4.ping_group_range` | the kernel, from the bound port; it rewrites the id field of every echo sent | echo replies carrying the socket's identifier only |

The datagram kind is Linux's unprivileged ICMP socket (`SOCK_DGRAM`,
`IPPROTO_ICMP`), opened by `listenDatagramICMP` with `unix.Socket` because
`net.ListenConfig` knows no such network and `golang.org/x/net/icmp` is not
vendored. The same `IP_MTU_DISCOVER` and `IP_RECVERR` options go on it, and
its error queue quotes the echo from its ICMP header as the raw kind does
(`ping_err` and `raw_err` hand `ip_icmp_error` the same pointer), so ping's
Don't Fragment modes and the reported next-hop MTU work on both kinds.
`TestProbeUnprivilegedSocketReportsNextHopMTU` proves it against the clamped
router with `CAP_NET_RAW` dropped. The kernel also looks the datagram socket
up by the quoted identifier before queueing an error, so unlike the raw kind
its queue never holds another flow's refusal
(`TestProbeUnprivilegedSocketIgnoresAnotherFlow`).

The fallback is taken only for a privilege refusal. `privilegeRefused` is
that guard: a raw socket refused for any other reason (an unsupported
family, an address that cannot be bound, a descriptor limit) stays a
failure, because a datagram socket opened over it would answer as if the raw
one had, on a host whose real defect nobody was told about
(`TestOpenICMPFallsBackOnlyOnPrivilegeRefusal`, and against the live opener
`TestProbeUnprivilegedFallbackIsNotTriedForOtherRefusals`). When both kinds
are refused, the error carries both refusals and names both fixes,
`CAP_NET_RAW` and `net.ipv4.ping_group_range`.

Traceroute cannot run on the datagram kind: the kernel delivers Time Exceeded
to a raw socket only, so `show traceroute`, `monitor traceroute` and
`show probe-round` refuse it through `openRawProbeConn`
(`internal/component/traceroute/cmd/traceroute.go`) and name `CAP_NET_RAW`,
rather than tracing a path whose every hop would time out.

`Socket` speaks `*net.IPAddr` on both kinds and translates to the
`*net.UDPAddr` the datagram conn wants, so a prober builds its destination
once. The x/net TTL wrappers take the concrete conn from `Socket.PacketConn`.

The doctor check `icmp-probe-socket` (`checkICMPProbeSocket`,
`internal/core/probe/doctor.go`) tries the two kinds in the same order before
an operator's first probe. A raw socket that opens is silent. The datagram
kind standing in is `doctor-icmp-probe-unprivileged`, so the operator learns
traceroute is unavailable before it refuses. Neither kind opening is
`doctor-icmp-probe`, whose message carries each kind's refusal and the group
the daemon runs as, so it names which of the two fixes applies. `ze explain
<code>` answers for both. ze runs as root on gokrazy, so the raw kind is what
production opens; the fallback matters the day Ze drops privileges or runs
outside the appliance.

## Route lookup

`show ip route lookup <dest>` returns the kernel's longest-prefix answer:
matching prefix, next hop, interface, protocol and metric. It calls
`RouteGet` from `vishvananda/netlink` on Linux and returns "not available"
through a build-tag stub elsewhere. The existing `show ip route`, which filters
on an exact prefix, is unchanged.

## The offline wrapper was deleted, not kept

`ze traceroute` used to wrap the OS binary. It was removed when `show
traceroute` landed. On gokrazy there is no OS traceroute to wrap, and on a
normal Linux host the operator can run `traceroute` directly. Keeping both
paths would mean two behaviors under one name (`ai/rules/no-layering.md`).
