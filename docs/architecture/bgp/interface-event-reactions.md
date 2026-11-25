# BGP Reactions to Interface Address Changes

BGP assumed a configured IP always existed. Listeners were created at startup
and never adjusted. The reactor now watches the kernel interface address table
and reacts: it starts a listener when a passive peer's local address appears,
stops every listener on an address that goes away, and re-settles RFC 2545 Section 3's common-subnet
condition for every live peer. It also watches the kernel neighbor table, which
names the link-local address of another router on a shared subnet.

<!-- source: internal/component/bgp/reactor/reactor_address_watch.go -- startAddressWatch, addressWatchWorker, addressSync.apply -->
<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddressAdded, handleAddressRemoved, refreshPeerLinkScopes -->

## The decisions

**Watch the kernel address table, not the interface bus topics.** The
`interface/addr-added` and `interface/addr-removed` topics come from the
interface monitor, which runs only when an `interface` block configures a
backend. A BGP-only config has none, so BGP once heard no address change at
all. The kernel interface backend package registers an address watcher in
`internal/core/network` (`network.RegisterAddressWatcher`) whichever backend
is active, because the kernel table is what BGP binds to and what
`network.InterfacePrefixes` reads. The reactor opens the watch when it starts,
before its listeners, and reads the table once to record it. On a platform
with no kernel backend the watch cannot start: the reactor logs a warning
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
and binds it on the next read. Any other refusal is not tried again: the
reactor logs it at Error under `doctor-bgp-listen`, naming the address, port
and owner with the kernel's cause and its fix (`CAP_NET_BIND_SERVICE` and its
remedy for a port below the unprivileged port start), as every listen route
reports it (`docs/architecture/behavior/peer-lifecycle.md`, "What a refused
listener tells the operator").

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- handleAddressAdded -->
<!-- source: internal/component/bgp/reactor/listen_refusal.go -- logListenRefusal -->

**Listener map keys come from `net.JoinHostPort(addr, port)`, never from string
concatenation.** Concatenation produces `::1:179` for IPv6 instead of
`[::1]:179`, and every lookup then misses. An earlier hand-written helper had
exactly that defect.

The stored key keeps the spelling used to bind the socket. A later address event
or configuration may use the same interface's name or numeric index instead.
`sameTransportAddress` compares those aliases using Go's name-first physical
interface namespace. `listenerForAddressPort` returns the existing listener and
its original key, so adding an alias reuses the socket and removing its last
claimant closes that entry. A different interface or listen port stays distinct.
An unresolved alias never removes a listener on an assumed interface.
The declaration list retains distinct zone spellings, including future numeric
device names. Only the runtime lookup against an actual bound listener merges
physical aliases; building the declarations never drops a prospective socket.
<!-- source: internal/component/bgp/reactor/transport_scope.go -- sameTransportAddress, listenerForAddressPort -->
<!-- source: internal/component/bgp/reactor/reactor.go -- listenerSpecs, startListenerForAddressPort -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- addPeerLocked, doRemovePeer -->

The original declaration and the observed TCP endpoint identify one stored
socket. Generic TCP can bind an IPv4 wildcard as an IPv6 wildcard; a later
declaration matching that observed endpoint reuses the same listener. Each
configured peer or group counts once across those verified identities. Shared
listeners attribute by remote address, and static context accepts another
declaration only when it identifies that same stored socket on the receiving
port. Wildcards are not treated as interchangeable without that binding.
The stored declaration is compared with canonical IPv4 identity, so a group
reload can change its mapped spelling without losing ownership of a retained
socket. Its original map key and configured group spelling remain unchanged.
The original declaration participates when choosing a socket for a live peer
addition and when selecting an entry for final removal, before claims are
counted. A canonical replacement therefore reuses and releases the original
mapped-key entry even when its observed wildcard has another address family.

If neither the stored declaration nor the observed endpoint establishes a
match without an interface lookup, an unreadable interface table leaves
ownership unknown. Removal retains that socket and incoming connections use remote-address
attribution. Removal checks the exact configured key and other bindings whose
stored or observed zone-free address matches the removed peer on its listen
port. Only a known empty claimant
set closes the original entry, even when the last peer's spelling cannot resolve
to its key. Other ports and the global listener remain unchanged. A simulation
without a TCP endpoint uses its exact configured key alone.
<!-- source: internal/component/bgp/reactor/transport_scope.go -- newListenerBinding, listenerBindingForAddressPort, withStoredDeclaration -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- listenerClaimants, listenerPeerKey -->
<!-- source: internal/component/bgp/reactor/reactor_connection.go -- handleConnectionWithContext, listenerClaimantsForBinding -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- doRemovePeer -->

**Addresses are normalized with `addr.Unmap()` before comparison.** An IPv4
address can be read as `::ffff:10.0.0.1`. Without `Unmap`, the match against a
peer `LocalAddress` of `10.0.0.1` fails in silence.
<!-- source: internal/component/bgp/reactor/transport_scope.go -- sameTransportAddressUsing -->
<!-- source: internal/component/bgp/reactor/reactor_peers.go -- doRemovePeer -->

**Subnet membership compares addresses without transport zones.**
`network.SharesSubnet` and the peer address captured by `newLinkScopeWith`
use `addr.Unmap().WithZone("")` for prefix comparisons. A scoped peer such as
`fe80::2%if0` therefore belongs to a connected `fe80::/64`; its zone does not
make `netip.Prefix.Contains` reject the address. The peer's settings, session
settings and TCP endpoint retain their zone. Address removal and restoration
replace only the comparison snapshot, so a missing prefix still denies
membership and separate connected prefixes do not prove a joint subnet.

<!-- source: internal/core/network/connected.go -- SharesSubnet -->
<!-- source: internal/component/bgp/reactor/link_scope.go -- newLinkScopeWith, sharesNextHopSubnet -->
<!-- test: internal/core/network/connected_scoped_test.go TestSharesSubnetScopedAddress -->
<!-- test: internal/component/bgp/reactor/link_scope_scoped_test.go TestLinkScopeScopedPeerMembership, TestLinkScopeScopedPeerRefresh -->

`scoped-peer-prefix-refresh.ci` removes and restores the connected prefix while
the receiver keeps its original TCP connection. Each phase sends a new subject
and a useful control after the address watcher applies the change. The receiver
checks exact permitted bytes and rejects advertisements or unowned withdrawals
of the forbidden subject throughout the connection. These assertions cover
newly received routes.
<!-- source: internal/test/fixture/plugin_fixture_scoped_peer_prefix_process_linux.go -- scopedPrefixRun -->
<!-- test: test/plugin/scoped-peer-prefix-refresh.ci -->

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

**The reactor watches the kernel neighbor table too, for the same reason and
by the same route.** RFC 2545 Section 3 asks for the link-local address of the
next hop after its global, and for a next hop that is another router on the
subnet shared with the peer only the neighbor table names that address. A
neighbor change is no bus topic, because the interface monitor that would
publish one runs only with an `interface` block. The interface component
registers the table in `internal/core/network` as a reader and a watcher
(`network.RegisterNeighborTable`), so the reactor imports no interface code.
`startNeighborWatch` starts one worker when the reactor starts, after the
address watch, and `superviseTableWatch` keeps the watch open with the retry
delays the address watch has.

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- startNeighborWatch, neighborWatchWorker -->
<!-- source: internal/core/network/neighbors.go -- RegisterNeighborTable, WatchNeighbors -->
<!-- source: internal/component/bgp/reactor/reactor_table_watch.go -- superviseTableWatch -->

**A neighbor signal re-reads the whole table, once for every live peer.** A
router's link-local is derived from two entries, its global and the
link-local with the same link-layer address on the same device, so a signal
names no entry the reactor could apply alone. `refreshPeerLinkScopes` reads
the table once and gives each live session an immutable replacement snapshot.
A session whose connected prefixes and neighbor view are unchanged keeps the
snapshot it has. The address watch calls the same function, and one mutex
serializes the two callers. The table is never read per UPDATE.

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- refreshPeerLinkScopes, refreshPeerLinkScopesMu -->
<!-- source: internal/component/bgp/reactor/link_scope_neighbors.go -- readNeighborView, neighborView.equal -->

**Every neighbor snapshot carries the state of the watch it was read under.**
The worker publishes each state it reaches: running, or stopped with the
error. A change of state is logged, as a warning with the fix when the watch
stopped, and refreshes every live peer so that no snapshot goes on claiming a
watch that ended. A next hop sent without its router's link-local then says
which case it is: the watched table names no usable entry for that router, or
the table is not watched and the answer may be out of date. An unreadable
table warns once per peer.

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- noteNeighborWatch, neighborWatchState -->
<!-- source: internal/component/bgp/reactor/link_scope_neighbors.go -- warnThirdPartyLinkLocalUnknown, warnThirdPartyLinkLocalUnwatched, warnNeighborTableUnread -->

**A neighbor change sets the form of the advertisements made after it, and
re-advertises nothing.** A route already sent with the global address alone is
not sent again when the table later names the link-local. That
re-advertisement is not built.

<!-- source: internal/component/bgp/reactor/reactor_iface.go -- neighborWatchWorker -->

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
