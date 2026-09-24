# MPLS in the Linux Kernel FIB

Ze programs MPLS forwarding into the Linux kernel through netlink. Labeled
system-RIB routes and native label-distribution entries have separate ownership.

| Path | Producer | Kernel form |
|------|----------|-------------|
| BGP labeled-unicast push | a labeled best-change entry, through the rich-route path | an IP route with an `RTA_ENCAP` MPLS label stack |
| Transit swap and pop (LDP, RSVP-TE) | `mplsfib` forwarding entries | an `AF_MPLS` route keyed by in-label |
| RSVP bypass control push | acknowledged `mplsfib` entry with a private table | a mark-selected IP route with an MPLS stack |

<!-- source: internal/plugins/fib/kernel/mpls.go -- shared MPLS constants, errors, validation -->
<!-- source: internal/plugins/fib/kernel/mplsentry.go -- the transit swap and pop entry path -->

## Decision: reuse the IP-route plumbing for the push

`processEvent` inspects the entry's labels and routes a labeled change through
the existing rich-route path, which imposes the stack as an MPLS encapsulation.
The alternative, a separate MPLS code path, was rejected: push, withdraw and
relabel then reuse the IP-route plumbing instead of duplicating it.

## Constraint: the push shares the FIB with other writers

An MPLS push bypasses sysrib best-path arbitration, so it can meet a foreign
route for the same prefix. A first install therefore uses `RouteAdd` and fails
with `EEXIST` rather than clobbering that route. Only a genuine relabel of Ze's
own push uses `RouteReplace`.

An `AF_MPLS` label is owned by its distribution source, not implicitly by every
Ze protocol. Its first installation uses `RouteAdd` and refuses an existing
foreign label. A replacement or withdrawal must match the recorded source.
Same-source replacement remains necessary for RSVP local repair, where the
incoming label stays fixed while its outgoing stack changes. The backend also
checks kernel ownership before replacing or deleting an installed label.

<!-- source: internal/plugins/fib/kernel/mplsentry.go -- source-owned label dispatch -->
<!-- source: internal/plugins/fib/kernel/mplsentry_linux.go -- first installation and owned replacement -->

`mplsfib.Apply` waits for synchronous native-owner acceptance. Publishing a
batch without an accepting owner is an error, as is a rejected kernel operation.
Callers must not advertise forwarding that the owner refused.

`mplsfib.RemoveLabelSource` retires the native owner's retained `AF_MPLS`
swap/pop labels for one source. A replacement producer can use it even when its
own label list was lost. The operation uses the same synchronous acknowledgment
as an entry batch: no accepting owner is an error, and every failed deletion
keeps its source claim for a later retry. Successfully removed labels stay
removed; other sources and prefix-keyed push contexts are not swept. A producer
must complete this cleanup before reassigning its retained labels to new peers.

`TestMPLSIntegration_SourceResetRetainsFailedDeletes` exercises partial cleanup
against a live kernel, preserves a foreign replacement and another source's
forwarding, and refuses label reuse until a successful cleanup retry.

RSVP bypass push entries use private tables in the `0x5a000000/0xffff0000`
packet-mark namespace. An exact-mark selector chooses the table; an unreachable
guard prevents a missing context from falling through to ordinary IP routing.
The owner acknowledges a context only after its route and selector are usable.
Each context is withdrawn independently, and shutdown removes private routes
and selectors regardless of the ordinary `flush-on-stop` setting.
The two fixed namespace guards remain for the network namespace's lifetime and
are reused on owner restart. Plugin shutdown is concurrent: removing the guards
would let a sender that already resolved its bypass race route deletion and
escape through the ordinary FIB. These guards reserve the mark namespace; they
do not retain any bypass route or exact-mark selector.

<!-- source: internal/core/mplsfib/events.go -- Apply -->
<!-- source: internal/plugins/fib/kernel/mplscontext_linux.go -- context installation, rollback and cleanup -->

## Constraint: a labeled path must be non-comparable

The unified Loc-RIB `Path` is a value type. Adding the label slice made it
non-comparable, so best-path change detection uses `Equal` rather than `!=`. That
`Equal` must compare labels: without it a relabel to the same next hop is
silently suppressed and the kernel keeps the old stack.

## Path MTU and label overhead

`mplsfib.Entry.PathMTU` carries the downstream frame payload budget, including
labels. A zero value leaves the output device MTU as the bound. The forwarding
owner writes a nonzero value as `RTA_METRICS/RTAX_MTU` on both IP push routes
and `AF_MPLS` transit routes; a replacement updates the metric with the labels
and next hop. Private push contexts use the same field and retain their
acknowledgement and mark-guard lifecycle.

For an outgoing stack of N bytes, the inner datagram limit is the smaller of
the downstream frame budget and output device MTU, minus N. There is no
additional configured initially-labeled datagram cap. Transit counts retained
labels as well as newly imposed labels, so the forwarding owner cannot
subtract the overhead when it installs the route.

Linux already fragments IPv4 before MPLS LWT encapsulation and subtracts LWT
headroom from the IP MTU. Ze passes the frame budget unchanged to preserve
that accounting. The runtime patch also makes forwarded IPv6 honour the
route metric, bounds the metric by the current device MTU, and preserves the
explicit LSP bound when a local socket probes PMTU. IPv6 errors report the
resulting inner limit even when label overhead takes it below 1280.
If the resulting IPv4 limit cannot hold the packet's actual header and an
eight-byte fragment payload, fragmentation returns `EMSGSIZE`. This check
precedes subtraction and rounding, so small physical MTUs and IP options
cannot produce an underflow or an endless stream of empty fragments.

Upstream Linux 7.2's `mpls_forward` drops an oversized labeled frame without
examining the inner IP header. The runtime kernel applies
`gokrazy/kernel/patches/0002-mpls-ip-mtu.patch` through the existing patch
series. Its native transit path uses the complete constructed label stack,
fragments DF-clear IPv4 with the kernel's IP fragment helpers, and sends
each fragment through the original MPLS next hop. DF-set IPv4 produces ICMP
Destination Unreachable code 4, and IPv6 produces Packet Too Big, with the
inner datagram limit in the MTU field. The normal ICMP senders retain their
error suppression, source selection and return-route policy. IPv4 transit
supplies an input-route context; the ICMP sender performs the return lookup
with its reflected mark and ICMP protocol. No preliminary ordinary-IP lookup
can veto a policy-selected return route.

Transit fragmentation preserves the original IPv4 options in the first
fragment. Later fragments omit options whose copy bit is clear. Validation
must not consume Record Route or Timestamp slots without filling them.
For DF-set errors, option processing uses the attached input-route context
before the normal ICMP sender builds its reply.

`CONFIG_MPLS_IP_MTU` identifies the patched capability in the runtime kernel
requirements. An upstream host kernel that can install MPLS routes is
insufficient evidence for this behaviour.

<!-- source: internal/core/mplsfib/events.go -- Entry.PathMTU -->
<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- buildRichRoute -->
<!-- source: gokrazy/kernel/patches/0002-mpls-ip-mtu.patch -- native IP MTU enforcement -->

`TestMPLSIntegration_PathMTU` installs entries through `mplsfib.Apply` and
injects packets over a veth pair. It checks push and transit, exact-fit
IPv4/IPv6 datagrams, complete fragment reassembly, and both ICMP errors with
their MTU, quotation and checksum. Its downstream-path and local-link cases
make different bounds decisive; the transit case retains an inner label.
IPv4 errors must also traverse an ICMP-only routing rule and honour inbound
source-address policy over a conflicting preferred source.
`TestMPLSIntegration_FragmentProgress` follows packets with unusable fragment
budgets with a forwarded marker on the same veth queue, covering rounding to
zero, physical-link underflow and the maximum IPv4 option header.
The carrier must run inside a guest booted with the rebuilt runtime kernel.
A capability skip leaves the behaviour unverified.
`TestMPLSIntegration_TransitIPv4Options` checks Record Route and Timestamp
slots across exact-fit forwarding, first-fragment preservation, later-fragment
omission and DF-set ICMP quotation.

<!-- source: internal/plugins/fib/kernel/mplsmtu_integration_linux_test.go -- TestMPLSIntegration_PathMTU -->

## Operator surface

`show mpls forwarding` reads the kernel table. The non-Linux build carries a stub
so the command exists everywhere.

<!-- source: internal/component/mpls/show_forwarding.go -- the show mpls forwarding command -->
<!-- source: internal/component/mpls/forwarding_linux.go -- the kernel AF_MPLS table reader -->

Per-interface MPLS input is enabled through the iface YANG (`mpls { enable }`,
which sets `net.mpls.conf.<iface>.input`), not through a standalone MPLS module.
The global label-space size is `net.mpls.platform_labels`.

## Trap: unit tests that drive the FIB directly prove nothing about the daemon

The kernel MPLS code was complete and unit-tested while the production path was
broken: labels were dropped in the Loc-RIB, so a labeled-unicast route installed
a plain IP route. The tests drove the FIB backend with a hand-built struct and
never crossed the BGP-to-Loc-RIB boundary where the loss happened. A protocol
feature is not proven until a test enters at the user or wire entry point. The
QEMU integration test is that evidence for the push path; it drives a labeled
change through `processEvent` to a live kernel and asserts push, relabel,
withdraw, and non-clobber of a foreign route.

## Trap: an AF_MPLS entry that reads back proves nothing about forwarding

The kernel accepts a labeled frame only on an interface whose
`net.mpls.conf.<iface>.input` sysctl is set. A table of correct swap and pop
entries therefore forwards nothing on a router that never set it, and a
read-back of the table looks the same either way. So the swap and pop
integration tests inject a labeled frame on the peer end of a veth pair and
read what the kernel sends back: the swapped label stack and the next hop's
hardware address for a swap, the bare inner packet for a pop with a next hop,
and the datagram a local socket receives for an egress pop through loopback.
Each carries a control that injects the same frame with `input` unset and
asserts the kernel forwards nothing. The tests write the sysctl themselves,
because the product writes it through the iface component and the sysctl
plugin, which the FIB package cannot reach.

<!-- source: internal/plugins/fib/kernel/mplsframe_integration_linux_test.go -- the veth harness and the labeled frame -->
<!-- source: internal/plugins/fib/kernel/mplsentry_integration_linux_test.go -- the swap and pop forwarding proofs -->
