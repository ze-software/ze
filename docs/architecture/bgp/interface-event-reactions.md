# BGP Reactions to Interface Address Changes

BGP assumed a configured IP always existed. Listeners were created at startup
and never adjusted. The reactor now watches the kernel interface address table
and reacts: it starts a listener when a peer's local address appears, stops it
when the address goes away, and re-settles RFC 2545 Section 3's common-subnet
condition for every live peer.

<!-- source: internal/component/bgp/reactor/reactor_address_watch.go -- startAddressWatch, addressWatchWorker, addressSync.apply -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddressAdded, handleAddressRemoved, refreshPeerLinkScopes -->

## The decisions

**Watch the kernel address table, not the interface bus topics.** The
`interface/addr-added` and `interface/addr-removed` topics come from the
interface monitor, which runs only when an `interface` block configures a
backend. A BGP-only config has none, so BGP once heard no address change at
all. The netlink interface backend package registers an address watcher in
`internal/core/network` (`network.RegisterAddressWatcher`) whichever backend
is active, because the kernel table is what BGP binds to and what
`network.InterfacePrefixes` reads. The reactor opens the watch when it starts,
before its listeners, and reads the table once to record it. On a platform
with no netlink backend the watch cannot start: the reactor logs a warning
naming the cause and tries again after a delay that doubles from one second to
one minute, as it does when a running watch ends.

<!-- source: internal/core/network/addresses.go -- RegisterAddressWatcher, WatchAddresses -->
<!-- source: internal/plugins/iface/netlink/address_watch_linux.go -- watchKernelAddresses -->
<!-- source: internal/component/bgp/reactor/reactor_table_watch.go -- superviseTableWatch -->

**A signal names no address, so each one re-reads the table and compares it
with the one last applied.** An address in the new read and not the old is an
addition, one in the old and not the new a removal. A change of prefix length
alone re-settles the common-subnet condition without touching a listener.

**An address the kernel cannot bind yet is tried again.** An IPv6 address in
duplicate address detection is already in the table, and binding it fails with
`EADDRNOTAVAIL` until DAD clears its tentative flag. The kernel reports that
clearing as an address update, so the reactor leaves the address unrecorded
and binds it on the next read.

**Listener map keys come from `net.JoinHostPort(addr, port)`, never from string
concatenation.** Concatenation produces `::1:179` for IPv6 instead of
`[::1]:179`, and every lookup then misses. An earlier hand-written helper had
exactly that defect.

**Addresses are normalized with `addr.Unmap()` before comparison.** An IPv4
address can be read as `::ffff:10.0.0.1`. Without `Unmap`, the match against a
peer `LocalAddress` of `10.0.0.1` fails in silence.

**A removed address stops its listener only.** A session already running on
that address is not torn down by this path.

**Address changes refresh the received NEXT_HOP view as well as export link
scope.** One interface read supplies unmasked local addresses for RFC 4271
Section 6.3 own-address checks and masked prefixes for the existing RFC 2545
common-subnet checks. Each live session receives an immutable replacement.
If the interface read fails, legacy IPv4 announcements are withheld until a
later successful refresh, rather than validated against stale addresses.

<!-- source: internal/core/network/connected.go -- InterfacePrefixes -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- refreshPeerLinkScopes -->
<!-- source: internal/component/bgp/reactor/session_next_hop.go -- receiveNextHopScope -->

## Constraints

**The address handlers run on the watch worker and take `r.mu` themselves.**
They are never called with it held. A stopping reactor refuses the listener
(`errReactorStopping`), so a change applied during a stop opens nothing.

**Peers that share a `LocalAddress` share one listener.** It is created once,
on the first read that finds the address.

A new listener starts before the old one stops, so sessions survive an
interface migration: the interface component adds the new address, waits for
BGP's `bgp/listener-ready` for it, then removes the old one.
