# MPLS in the Linux Kernel FIB

Ze programs MPLS forwarding into the Linux kernel through netlink. Two distinct
paths exist and they must not be conflated.

| Path | Producer | Kernel form |
|------|----------|-------------|
| BGP labeled-unicast push | a labeled best-change entry, through the rich-route path | an IP route with an `RTA_ENCAP` MPLS label stack |
| Transit swap and pop (LDP, RSVP-TE) | `mplsfib` forwarding entries | an `AF_MPLS` route keyed by in-label |

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

The transit path has the opposite rule: the `AF_MPLS` in-label space is Ze's
own, so `addMPLSSwap` uses `RouteReplace`. Re-programming a live in-label is
normal there (RSVP-TE local repair does it), and `RouteAdd` would fail `EEXIST`
and silently drop the repair.

<!-- source: internal/plugins/fib/kernel/mplsentry_linux.go -- RouteReplace on the AF_MPLS swap -->

## Constraint: a labeled path must be non-comparable

The unified Loc-RIB `Path` is a value type. Adding the label slice made it
non-comparable, so best-path change detection uses `Equal` rather than `!=`. That
`Equal` must compare labels: without it a relabel to the same next hop is
silently suppressed and the kernel keeps the old stack.

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
