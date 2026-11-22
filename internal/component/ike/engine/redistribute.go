// Design: docs/architecture/core-design.md -- redistribute source registration
package engine

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"sync"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

var redistOnce sync.Once

func registerIPsecRedistSources() {
	redistOnce.Do(func() {
		err := configredist.RegisterSource(configredist.RouteSource{
			Name:        ipsecName,
			Protocol:    ipsecName,
			Description: "tunnel routes from IPsec Child SAs",
		})
		if err != nil {
			slog.Error("BUG: failed to register ipsec redistribute source", "err", err)
		}
	})
}

// ipsecName is the one name this engine registers under: the redistribute
// source, its protocol, and the kernel capability subsystem.
const ipsecName = "ipsec"

var ipsecProtocolID = redistevents.RegisterProtocol(ipsecName)

var _ = registerIPsecProducer()

func registerIPsecProducer() bool {
	redistevents.RegisterProducer(ipsecProtocolID)
	return true
}

var ipsecRouteChange = events.Register[*redistevents.RouteChangeBatch](ipsecName, redistevents.EventType)

// emitRouteAdd announces every remote traffic selector child installs a policy for, one
// route-change batch per prefix. A Child SA can negotiate several selector pairs (IPv4
// first and IPv6 second on a route-based xfrm interface), and TSRemote holds only the
// first, so the prefixes come from childRemotePrefixes: announcing TSRemote alone left
// every later pair's prefix unknown to the redistribute consumers. A batch carries one
// AFI, so each prefix travels in its own. emitRouteRemove withdraws the same set.
func emitRouteAdd(bus ze.EventBus, child *ChildSA, log *slog.Logger) {
	if bus == nil {
		log.Warn("ipsec: emit route-change skipped: no event bus")
		return
	}
	if child == nil {
		log.Warn("ipsec: emit route-change skipped: nil Child SA")
		return
	}
	prefixes := childRemotePrefixes(child)
	if len(prefixes) == 0 {
		log.Warn("ipsec: emit route-change skipped: Child SA has no remote traffic selector")
		return
	}
	instance, err := childRouteSources.instance(child)
	if err != nil {
		logChildRouteSourceRefusal(log, err)
		return
	}
	for _, prefix := range prefixes {
		log.Debug("ipsec: emitting route-change add", "prefix", prefix.String())
		emitRouteChange(bus, redistevents.ActionAdd, prefix, instance, log)
	}
}

// emitRouteRemove withdraws every prefix emitRouteAdd announced for child. Both read
// childRemotePrefixes, so the withdrawal names exactly the announced set; a Child SA's
// selectors never change while it lives (a rekey builds a new ChildSA, and rekeyTunnelRoutes moves the routes).
func emitRouteRemove(bus ze.EventBus, child *ChildSA, log *slog.Logger) {
	if bus == nil {
		return
	}
	if child == nil {
		return
	}
	instance, err := childRouteSources.instance(child)
	if err != nil {
		logChildRouteSourceRefusal(log, err)
		return
	}
	for _, prefix := range childRemotePrefixes(child) {
		emitRouteChange(bus, redistevents.ActionRemove, prefix, instance, log)
	}
}

// rekeyTunnelRoutes moves peer's tunnel routes, on the route-change bus and in the
// tunnel-source guard, from retired to the successor a Child SA rekey installed. A rekey
// can narrow or change the selector set (newRekeyedChild adopts what the exchange
// negotiated), so it announces only the prefixes successor adds and withdraws only the
// ones it dropped. A prefix both carry is left alone: withdrawing and re-announcing it
// would flap the route at every rekey. The successor's prefixes are announced before the
// retired ones are withdrawn, make-before-break as the SA swap itself is.
func rekeyTunnelRoutes(bus ze.EventBus, peer string, retired, successor *ChildSA, log *slog.Logger) {
	if retired == nil {
		emitRouteAdd(bus, successor, log)
		guardTunnelRoute(true, peer, successor, log)
		return
	}
	before := childRemotePrefixes(retired)
	after := childRemotePrefixes(successor)
	added := prefixesAbsentFrom(after, before)
	dropped := prefixesAbsentFrom(before, after)
	if bus == nil {
		log.Warn("ipsec: rekey route-change skipped: no event bus", "peer", peer)
	} else {
		instance, err := childRouteSources.inherit(retired, successor)
		if err != nil {
			logChildRouteSourceRefusal(log, err)
		} else {
			for _, prefix := range added {
				emitRouteChange(bus, redistevents.ActionAdd, prefix, instance, log)
			}
			for _, prefix := range dropped {
				emitRouteChange(bus, redistevents.ActionRemove, prefix, instance, log)
			}
		}
	}
	// A guard entry is keyed by the interface too, so a successor on another interface
	// (never the case today: newRekeyedChild inherits IfID) moves the whole set.
	if retired.IfID != successor.IfID {
		guardTunnelPrefixes(true, peer, successor.IfID, after, log)
		guardTunnelPrefixes(false, peer, retired.IfID, before, log)
		return
	}
	guardTunnelPrefixes(true, peer, successor.IfID, added, log)
	guardTunnelPrefixes(false, peer, retired.IfID, dropped, log)
}

// prefixesAbsentFrom answers the prefixes of set that other does not hold, in set's order.
func prefixesAbsentFrom(set, other []netip.Prefix) []netip.Prefix {
	var out []netip.Prefix
	for _, prefix := range set {
		if slices.Contains(other, prefix) {
			continue
		}
		out = append(out, prefix)
	}
	return out
}

// childRouteSourceAllocator assigns one nonzero scalar to a logical Child SA
// route lifetime and retains no history or per-event allocation. Rekey transfers
// that lifetime explicitly; a new Child SA receives another instance even when
// its selectors, owner, SPI or interface happen to equal a previous SA's.
// Safe for concurrent use. Its lock also protects ChildSA.redistributeInstance.
type childRouteSourceAllocator struct {
	mu   sync.Mutex
	next uint32
	// failure is the first lost publication in this process. There is no
	// producer replay to reconstruct it, so only restart clears the finding.
	failure *opererr.Error
}

var childRouteSources childRouteSourceAllocator

var (
	errChildRouteSourceExhausted = errors.New("IPsec redistribution source identities are exhausted")
	errChildRouteSourceAssigned  = errors.New("IPsec rekey successor already owns another redistribution source")
)

func childRouteSourceExhaustedError() *opererr.Error {
	return opererr.Wrap(errChildRouteSourceExhausted,
		"IPsec redistribution source identities are exhausted",
		"Ze refused a Child SA route publication instead of reusing another source's identity. Source publication cannot recover in this process",
		privilege.RestartRemedy())
}

func childRouteSourceAssignedError() *opererr.Error {
	return opererr.Wrap(errChildRouteSourceAssigned,
		"An IPsec rekey successor already owns another redistribution source",
		"Ze refused the route publication instead of replacing an independent Child SA's identity. Source publication cannot recover in this process",
		privilege.RestartRemedy())
}

func (a *childRouteSourceAllocator) instance(child *ChildSA) (uint32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.instanceLocked(child)
}

// instanceLocked assigns at most once. Caller MUST hold mu.
func (a *childRouteSourceAllocator) instanceLocked(child *ChildSA) (uint32, error) {
	if child.redistributeInstance != 0 {
		return child.redistributeInstance, nil
	}
	if a.next == ^uint32(0) {
		return a.refuseLocked(childRouteSourceExhaustedError())
	}
	a.next++
	child.redistributeInstance = a.next
	return child.redistributeInstance, nil
}

func (a *childRouteSourceAllocator) inherit(retired, successor *ChildSA) (uint32, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	instance, err := a.instanceLocked(retired)
	if err != nil {
		return 0, err
	}
	if successor.redistributeInstance != 0 {
		if successor.redistributeInstance != instance {
			return a.refuseLocked(childRouteSourceAssignedError())
		}
	}
	successor.redistributeInstance = instance
	return instance, nil
}

// refuseLocked records one permanent source-output failure, without a route
// history or an unbounded failure list. Caller MUST hold mu.
func (a *childRouteSourceAllocator) refuseLocked(failure *opererr.Error) (uint32, error) {
	if a.failure == nil {
		a.failure = failure
	}
	return 0, failure
}

func logChildRouteSourceRefusal(log *slog.Logger, err error) {
	log.Warn("IPsec redistribution lost a source publication and cannot reconstruct it in this process",
		"error", err, "fix", privilege.RestartRemedy(),
		"code", diagnostic.CodeDoctorIPsecRedistributionSource)
}

var redistributionSourceCheckDecl = rpc.DoctorCheckDecl{
	Name:  "ipsec-redistribution-source",
	Phase: rpc.DoctorPhaseRuntime,
	Order: 744,
	Codes: []string{diagnostic.CodeDoctorIPsecRedistributionSource},
}

// answerRedistributionSource reads the actual producer's first lost publication.
// No source replay exists today, so a later successful emit cannot clear it.
func answerRedistributionSource(_ *ikeEngineState) []rpc.DoctorCheckDiagnostic {
	return childRouteSources.findings()
}

func (a *childRouteSourceAllocator) findings() []rpc.DoctorCheckDiagnostic {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure == nil {
		return nil
	}
	return []rpc.DoctorCheckDiagnostic{{
		Code:     diagnostic.CodeDoctorIPsecRedistributionSource,
		Severity: severityError,
		Message:  a.failure.Problem() + ". " + a.failure.Why(),
		Fix:      a.failure.Fix(),
	}}
}

// emitRouteChange publishes one route-change batch carrying prefix under action.
func emitRouteChange(bus ze.EventBus, action redistevents.RouteAction, prefix netip.Prefix, instance uint32, log *slog.Logger) {
	batch := redistevents.AcquireBatch()
	batch.Protocol = ipsecProtocolID
	if prefix.Addr().Is4() {
		batch.AFI = 1
		batch.SAFI = 1
	} else {
		batch.AFI = 2
		batch.SAFI = 1
	}
	batch.Entries = append(batch.Entries, redistevents.RouteChangeEntry{
		SourceInstance: instance,
		Action:         action,
		Prefix:         prefix,
	})
	n, err := ipsecRouteChange.Emit(bus, batch)
	if err != nil {
		log.Warn("ipsec: emit route-change failed", "action", action, "prefix", prefix.String(), "error", err)
	} else {
		log.Debug("ipsec: route-change delivered", "action", action, "prefix", prefix.String(), "subscribers", n)
	}
	redistevents.ReleaseBatch(batch)
}

func netIPNetToPrefix(ipNet *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(ipNet.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, _ := ipNet.Mask.Size()
	return netip.PrefixFrom(addr.Unmap(), ones), true
}
