// Design: docs/architecture/core-design.md -- connected prefixes in the shared Loc-RIB
// Related: connected.go -- routeObserver, which tracks the addresses covering each prefix inserted here
// Related: events/events.go -- the OS-installed declaration sysrib reads

package connected

import (
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/textbuf"
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
// The observer keeps one path per prefix and updates its covering devices, so
// there is never a second connected path for one prefix to tell apart.
const pathInstance uint32 = 0

// insertPath publishes one connected prefix into the shared Loc-RIB.
//
// The Path names no gateway and carries the devices its addresses cover. An
// invalid NextHop is what the recursive resolver treats as the end of a chain
// ("Connected route: current is the directly-reachable NH",
// internal/component/sysrib/nhresolver.go), so a protocol next-hop covered only
// by an interface prefix becomes resolvable rather than unreachable.
//
// Ze programs nothing for this prefix. The kernel created the route when the
// address was assigned, and connectedevents declares that, so a connected winner
// makes sysrib WITHDRAW Ze's own entry instead of installing a second one.
//
// The caller MUST hold emitMu until publication finishes, so covering cannot
// change while the path is built or overtake an earlier address publication.
func (o *routeObserver) insertPath(prefix netip.Prefix, covering map[connectedAddr]struct{}) {
	path := locrib.Path{
		Source:   connectedevents.ProtocolID,
		Instance: pathInstance,
		// No distance: the Loc-RIB ranks the path at the distance
		// `rib { distance { connected } }` declares, and re-ranks it on a reload.
	}
	devices := make([]string, 0, len(covering))
	for address := range covering {
		devices = append(devices, address.device())
	}
	slices.Sort(devices)
	devices = slices.Compact(devices)
	for i, device := range devices {
		if i == 0 {
			path.Interface = device
			continue
		}
		path.ECMP = append(path.ECMP, locrib.NextHop{Interface: device})
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

// device restores the OS device from the interface monitor's parent/unit pair.
// Unit zero denotes the parent itself; a positive unit denotes parent.unit.
func (address connectedAddr) device() string {
	if address.unit == 0 {
		return address.name
	}
	var name textbuf.Buffer
	return name.Reset().Str(address.name).Byte('.').Int(int64(address.unit)).String()
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

// withdrawLocalPaths releases only this observer's connected paths when the
// internal plugin stops. The caller MUST wait for SDK Run to return first:
// it fences and joins direct event delivery before this map is read.
// Forked paths remain owned by the engine's route-install disconnect cleanup.
func (o *routeObserver) withdrawLocalPaths() {
	if o.loc == nil {
		return
	}
	for prefix := range o.prefixes {
		o.removePath(prefix)
	}
}

// familyOf returns the Loc-RIB family a prefix belongs to.
func familyOf(p netip.Prefix) family.Family {
	if p.Addr().Is4() {
		return family.IPv4Unicast
	}
	return family.IPv6Unicast
}
