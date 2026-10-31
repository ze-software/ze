# BGP Reactions to Interface Address Changes

BGP assumed a configured IP always existed. Listeners were created at startup
and never adjusted. The reactor now watches the kernel interface address table
and reacts: it starts a listener when a passive peer's local address appears,
stops every listener on an address that goes away, and re-settles RFC 2545 Section 3's common-subnet
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

**An added address opens what the start opens, by the same rule.**
`listenerSpecs` names the (address, port) pairs the configuration accepts
connections on: the local address of each passive peer and dynamic group, at
the port `peerListenPort` answers for it. The start and an added address both
read it. A peer with its own `connection > local > port` gets its listener on
that port, and a peer that only dials out (`connection active`) gets no
listener at all. Before this rule was shared, the handler used the daemon's
port and ignored the connection mode, so an own-port peer's listener opened on
the wrong port.

**A removed address stops every listener on it, at any port.** Nothing can
accept on an address the host no longer holds, so the stop does not ask the
configuration which ports it wants. That also closes a listener a reload left
with no claimant: `installDynamicGroupsLocked` leaves a removed group's socket
open, because it may be shared with a peer and closing it is the peer-removal
path's decision. A session already running on that address is
not torn down by this path.

<!-- source: internal/component/bgp/reactor/reactor.go -- listenerSpecs, startMultiListeners -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- listenPortsAt, addressBound, handleAddressAdded, handleAddressRemoved -->

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

**Peers that share a `LocalAddress` and a listen port share one listener.** It
is created once, on the first read that finds the address. A peer on another
port of the same address has a listener of its own.

A new listener starts before the old one stops, so sessions survive an
interface migration: the interface component adds the new address, waits for
BGP's `bgp/listener-ready` for it, then removes the old one. BGP sends that
event once every listener the address owes is up. An address that only peers
dialling out are bound to owes none, so the event goes out as soon as it
appears. An address no peer or group names gets no event.
