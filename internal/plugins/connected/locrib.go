// Design: docs/architecture/core-design.md -- connected prefixes in the shared Loc-RIB
// Related: connected.go -- routeObserver, which refcounts the prefixes inserted here
// Related: events/events.go -- the OS-installed declaration sysrib reads

package connected

import (
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	connectedevents "github.com/ze-software/ze/internal/plugins/connected/events"
)

// routeSink receives Loc-RIB install and remove operations when the plugin holds
// no local Loc-RIB, which is a forked subprocess: locrib.Default() answers nil
// there. The forked wiring installs one, and it ships each operation to the
// engine over the route-install RPC.
type routeSink interface {
	InsertForward(fam family.Family, prefix netip.Prefix, p locrib.Path)
	Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32)
	Flush()
}

// setLocRIB records where connected prefixes are published. loc is the shared
// Loc-RIB when connected runs in-process and nil when it runs forked, in which
// case remote carries the operations to the engine.
//
// Called once at plugin start, before any address event is handled.
func (o *routeObserver) setLocRIB(loc *locrib.RIB, remote routeSink) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.loc = loc
	o.remote = remote
}

// pathInstance is the within-protocol identifier every connected Path carries.
// The observer refcounts by prefix and inserts once per prefix, so there is
// never a second connected path for one prefix to tell apart.
const pathInstance uint32 = 0

// insertPath publishes one connected prefix into the shared Loc-RIB.
//
// The Path names NO next-hop, and that is the whole representation. An invalid
// NextHop is what the recursive resolver already treats as the end of a chain
// ("Connected route: current is the directly-reachable NH",
// internal/component/sysrib/nhresolver.go), so a protocol next-hop covered only
// by an interface prefix becomes resolvable rather than unreachable.
//
// Ze programs nothing for this prefix. The kernel created the route when the
// address was assigned, and connectedevents declares that, so a connected winner
// makes sysrib WITHDRAW Ze's own entry instead of installing a second one.
func (o *routeObserver) insertPath(prefix netip.Prefix) {
	path := locrib.Path{
		Source:   connectedevents.ProtocolID,
		Instance: pathInstance,
		// No distance: the Loc-RIB ranks the path at the distance
		// `rib { distance { connected } }` declares, and re-ranks it on a reload.
	}
	fam := familyOf(prefix)
	if o.loc != nil {
		o.loc.InsertForward(fam, prefix, path, nil)
		return
	}
	if o.remote != nil {
		o.remote.InsertForward(fam, prefix, path)
		o.remote.Flush()
	}
}

// removePath withdraws a connected prefix whose last address is gone. The
// prefix's next-best path, from any protocol, becomes the winner and Ze programs
// it, because the kernel has just removed the connected route with the address.
func (o *routeObserver) removePath(prefix netip.Prefix) {
	fam := familyOf(prefix)
	if o.loc != nil {
		o.loc.Remove(fam, prefix, connectedevents.ProtocolID, pathInstance)
		return
	}
	if o.remote != nil {
		o.remote.Remove(fam, prefix, connectedevents.ProtocolID, pathInstance)
		o.remote.Flush()
	}
}

// familyOf returns the Loc-RIB family a prefix belongs to.
func familyOf(p netip.Prefix) family.Family {
	if p.Addr().Is4() {
		return family.IPv4Unicast
	}
	return family.IPv6Unicast
}
