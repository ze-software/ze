// Design: docs/guide/l2tp.md -- subscriber route lifecycle
// Related: redistribute.go -- source registration
// Related: subscriber/events/events.go -- route assignment and withdrawal notices

package l2tp

import (
	"log/slog"
	"net/netip"
	"sync"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/ze"
)

// routeRecord is the live state the observer tracks per session. framedRoutes
// holds both families' authorized framed routes, counted together against
// l2tpevents.FramedRoutesMax; each family's half is announced on that family's
// install and withdrawn on its release, and its emitted flag says the half is
// live in the feed now.
type routeRecord struct {
	instance          uint32
	network           l2tpevents.NetworkLifetime
	username          string
	v4                netip.Addr
	v6                netip.Addr
	framedRoutes      []FramedRoute
	framedEmittedIPv4 bool
	framedEmittedIPv6 bool
}

// subscriberRouteObserver retains live routes by subscriber ID for one access
// type. Its mutex protects the records and counters across event callbacks.
type subscriberRouteObserver struct {
	logger *slog.Logger
	bus    ze.EventBus
	access subscriber.AccessType
	source redistevents.ProtocolID
	emit   *events.Event[*redistevents.RouteChangeBatch]

	mu sync.Mutex
	// emitMu orders a replay against the incremental emits. Each incremental
	// emit holds it shared, after its record changed under mu; reemitAll
	// holds it exclusive from its snapshot to its last emit. A Remove whose
	// record changed after the snapshot is then published after the replay's
	// Add, never before it, so the replay cannot revive the route.
	emitMu  sync.RWMutex
	records map[string]*routeRecord

	// Totals count host route changes over the life of this reader.
	injectedTotal  uint64
	withdrawnTotal uint64
}

// SubscriberRouteSubscription owns one access type's subscriber route reader.
type SubscriberRouteSubscription struct {
	observer *subscriberRouteObserver
	mu       sync.Mutex
	inFlight sync.WaitGroup
	stopped  bool
	unsubs   []func()
}

// StartSubscriberRouteObserver subscribes to assignment, family release, down and replay for
// one access type. The owning transport MUST start it before emitting notices
// and retain it until its terminal notices have all returned.
func StartSubscriberRouteObserver(logger *slog.Logger, bus ze.EventBus, access subscriber.AccessType,
	source redistevents.ProtocolID, emit *events.Event[*redistevents.RouteChangeBatch],
) *SubscriberRouteSubscription {
	subscription := &SubscriberRouteSubscription{observer: newAccessRouteObserver(logger, bus, access, source, emit)}
	if bus != nil {
		subscription.unsubs = []func(){
			subevents.SessionIPAssigned.Subscribe(bus, func(p *subevents.SessionIPAssignedPayload) {
				subscription.deliver(func() { subscription.observer.assign(&p.Session, p.NetworkLifetime, p.Address, p.FramedRoutes) })
			}),
			subevents.SessionIPReleased.Subscribe(bus, func(p *subevents.SessionIPReleasedPayload) {
				subscription.deliver(func() { subscription.observer.release(&p.Session, p.NetworkLifetime, p.Family) })
			}),
			subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) {
				subscription.deliver(func() { subscription.observer.withdraw(&p.Session, p.NetworkLifetime) })
			}),
			redistevents.ReplayRequestEvent.Subscribe(bus, func(p *redistevents.ReplayRequest) {
				subscription.deliver(func() { subscription.observer.reemitAll(p.ReplayID) })
			}),
		}
	}
	return subscription
}

// Stop disables new callbacks, joins readers already admitted and withdraws
// any contributions they left live. The owning transport MUST drain first
// and MUST call Stop outside this reader's callbacks.
func (subscription *SubscriberRouteSubscription) Stop() {
	subscription.mu.Lock()
	if subscription.stopped {
		subscription.mu.Unlock()
		return
	}
	subscription.stopped = true
	unsubs := subscription.unsubs
	subscription.unsubs = nil
	subscription.mu.Unlock()
	for _, unsub := range unsubs {
		unsub()
	}
	subscription.inFlight.Wait()
	subscription.observer.withdrawAll()
}

func (subscription *SubscriberRouteSubscription) deliver(f func()) {
	subscription.mu.Lock()
	if subscription.stopped {
		subscription.mu.Unlock()
		return
	}
	subscription.inFlight.Add(1)
	subscription.mu.Unlock()
	defer subscription.inFlight.Done()
	f()
}

func newAccessRouteObserver(logger *slog.Logger, bus ze.EventBus, access subscriber.AccessType,
	source redistevents.ProtocolID, emit *events.Event[*redistevents.RouteChangeBatch],
) *subscriberRouteObserver {
	if logger == nil {
		logger = slog.Default()
	}
	return &subscriberRouteObserver{
		logger:  logger.With("component", string(access)+"-redistribute"),
		bus:     bus,
		access:  access,
		source:  source,
		emit:    emit,
		records: make(map[string]*routeRecord),
	}
}

func (o *subscriberRouteObserver) ownsSession(session *subscriber.Session, operation string) bool {
	if session.AccessType != o.access {
		return false
	}
	if o.access == subscriber.AccessL2TP && (!session.L2TPBirth.IsValid() || session.L2TPBirth.String() != session.ID) {
		reportSessionBirthRefusal(o.logger, operation, session.TunnelID, session.SessionID)
		return false
	}
	return subscriber.RequireID(o.logger, session.ID) == nil
}

// assign installs one family under the subscriber's record. A notice with an
// address installs that address's family and its host route; a notice with no
// address and an interface identifier is the IPv6CP install, which has no
// host route. The record, and its instance, are created by the first family
// installed in the network lifetime, which copies the framed routes. Each
// family's framed routes are announced on that family's install, never on the
// other family's, so an IPv6-only session announces its IPv6 framed routes.
func (o *subscriberRouteObserver) assign(session *subscriber.Session, network l2tpevents.NetworkLifetime, addr netip.Addr, framedRoutes []FramedRoute) {
	installed := installedFamily(session, addr)
	if installed == l2tpevents.AddressFamilyUnspecified {
		return
	}
	if !o.ownsSession(session, "route assignment") {
		return
	}
	if len(framedRoutes) > l2tpevents.FramedRoutesMax {
		o.reportFramedRouteRefusal(session, len(framedRoutes))
		return
	}
	var prev netip.Addr
	o.mu.Lock()
	r := o.records[session.ID]
	if r != nil && r.network.IsValid() && network.IsValid() && r.network != network {
		o.mu.Unlock()
		return
	}
	if r == nil {
		instance, err := subscriberRouteSources.next()
		if err != nil {
			o.mu.Unlock()
			o.reportSourceRefusal(session, err)
			return
		}
		r = &routeRecord{instance: instance, network: network, username: session.Username}
		o.records[session.ID] = r
		if len(framedRoutes) > 0 {
			r.framedRoutes = make([]FramedRoute, len(framedRoutes))
			copy(r.framedRoutes, framedRoutes)
		}
	}
	if session.Username != "" && r.username == "" {
		r.username = session.Username
	}
	if addr.IsValid() {
		if addr.Is4() {
			prev = r.v4
			r.v4 = addr
		} else {
			prev = r.v6
			r.v6 = addr
		}
		o.injectedTotal++
	}
	emitFramed := r.takeFramedRoutes(installed)
	instance := r.instance
	username := r.username
	o.mu.Unlock()
	o.logger.Info(string(o.access)+": subscriber route inject",
		reportFieldSessionID, routeSessionLabel(session),
		"username", username,
		"family", installed.String(),
		"framed-routes", len(emitFramed))
	if addr.IsValid() {
		if prev.IsValid() && prev != addr {
			o.emitRemove(prev, instance)
		}
		o.emitAdd(addr, instance)
	}
	if len(emitFramed) > 0 {
		o.emitFramedRoutes(redistevents.ActionAdd, emitFramed, instance)
	}
}

// installedFamily names the family an assignment notice installs: the
// address's family when it carries one, IPv6 when it carries only an IPv6CP
// interface identifier, and Unspecified when it installs nothing.
func installedFamily(session *subscriber.Session, addr netip.Addr) l2tpevents.AddressFamily {
	if addr.Is4() {
		return l2tpevents.AddressFamilyIPv4
	}
	if addr.Is6() {
		return l2tpevents.AddressFamilyIPv6
	}
	if session.IPv6InterfaceID != [8]byte{} {
		return l2tpevents.AddressFamilyIPv6
	}
	return l2tpevents.AddressFamilyUnspecified
}

// takeFramedRoutes returns the family's framed routes when they are not live
// yet and marks them live; nil when they already are or the family has none.
// Caller MUST hold the observer's mu.
func (r *routeRecord) takeFramedRoutes(family l2tpevents.AddressFamily) []FramedRoute {
	routes := framedRoutesOf(r.framedRoutes, family)
	if len(routes) == 0 {
		return nil
	}
	switch family {
	case l2tpevents.AddressFamilyIPv4:
		if r.framedEmittedIPv4 {
			return nil
		}
		r.framedEmittedIPv4 = true
	case l2tpevents.AddressFamilyIPv6:
		if r.framedEmittedIPv6 {
			return nil
		}
		r.framedEmittedIPv6 = true
	case l2tpevents.AddressFamilyUnspecified:
		return nil
	}
	return routes
}

// liveFramedRoutes returns the framed routes now announced, both families.
// Caller MUST hold the observer's mu.
func (r *routeRecord) liveFramedRoutes() []FramedRoute {
	var live []FramedRoute
	if r.framedEmittedIPv4 {
		live = append(live, framedRoutesOf(r.framedRoutes, l2tpevents.AddressFamilyIPv4)...)
	}
	if r.framedEmittedIPv6 {
		live = append(live, framedRoutesOf(r.framedRoutes, l2tpevents.AddressFamilyIPv6)...)
	}
	return live
}

// framedRoutesOf selects the framed routes of one family: Framed-Route
// (RFC 2865 Section 5.22) is IPv4, Framed-IPv6-Route (RFC 3162 Section 2.5)
// is IPv6.
func framedRoutesOf(routes []FramedRoute, family l2tpevents.AddressFamily) []FramedRoute {
	var selected []FramedRoute
	for _, fr := range routes {
		if framedRouteFamily(fr) == family {
			selected = append(selected, fr)
		}
	}
	return selected
}

func framedRouteFamily(fr FramedRoute) l2tpevents.AddressFamily {
	if fr.Prefix.Addr().Is4() {
		return l2tpevents.AddressFamilyIPv4
	}
	return l2tpevents.AddressFamilyIPv6
}

// release removes one family's routes while the session and the other family
// stay up: IPv4 its /32 and its framed routes, IPv6 its framed routes (and a
// /128 host route, which only a notice with an IPv6 address installs). The
// record and its instance stay, and the family's emitted flag is cleared, so
// its next install re-adds exactly its own routes under the same instance.
func (o *subscriberRouteObserver) release(session *subscriber.Session, network l2tpevents.NetworkLifetime, family l2tpevents.AddressFamily) {
	if !o.ownsSession(session, "route release") {
		return
	}
	o.mu.Lock()
	r := o.records[session.ID]
	if r == nil {
		o.mu.Unlock()
		return
	}
	if r.network.IsValid() && network.IsValid() && r.network != network {
		o.mu.Unlock()
		return
	}
	var host netip.Addr
	var framed []FramedRoute
	switch family {
	case l2tpevents.AddressFamilyIPv4:
		host = r.v4
		r.v4 = netip.Addr{}
		if r.framedEmittedIPv4 {
			framed = framedRoutesOf(r.framedRoutes, family)
			r.framedEmittedIPv4 = false
		}
	case l2tpevents.AddressFamilyIPv6:
		host = r.v6
		r.v6 = netip.Addr{}
		if r.framedEmittedIPv6 {
			framed = framedRoutesOf(r.framedRoutes, family)
			r.framedEmittedIPv6 = false
		}
	case l2tpevents.AddressFamilyUnspecified:
		o.mu.Unlock()
		return
	}
	if host.IsValid() {
		o.withdrawnTotal++
	}
	instance := r.instance
	username := r.username
	o.mu.Unlock()
	if host.IsValid() {
		o.emitRemove(host, instance)
	}
	if len(framed) > 0 {
		o.emitFramedRoutes(redistevents.ActionRemove, framed, instance)
	}
	o.logger.Info(string(o.access)+": subscriber family routes withdrawn",
		reportFieldSessionID, routeSessionLabel(session),
		"username", username,
		"family", family.String(),
		"framed-routes", len(framed))
}

// withdraw removes each recorded host route and every framed route still
// announced, of either family, for this subscriber and network lifetime. A
// family released earlier contributes nothing a second time.
func (o *subscriberRouteObserver) withdraw(session *subscriber.Session, network l2tpevents.NetworkLifetime) {
	if !o.ownsSession(session, "route withdrawal") {
		return
	}
	o.mu.Lock()
	r := o.records[session.ID]
	if r == nil || r.network.IsValid() && network.IsValid() && r.network != network {
		o.mu.Unlock()
		return
	}
	delete(o.records, session.ID)
	withdrawn := r.hostRouteCount()
	o.withdrawnTotal += withdrawn
	o.mu.Unlock()
	o.emitRecordRemovals(r)
	o.logger.Info(string(o.access)+": subscriber routes withdrawn",
		reportFieldSessionID, routeSessionLabel(session),
		"username", r.username,
		"withdrawn", withdrawn)
}

func (r *routeRecord) hostRouteCount() uint64 {
	var count uint64
	if r.v4.IsValid() {
		count++
	}
	if r.v6.IsValid() {
		count++
	}
	return count
}

// withdrawAll runs only after Stop has joined all event callbacks. Detach the
// records before sending any removals to synchronous EventBus recipients.
func (o *subscriberRouteObserver) withdrawAll() {
	o.mu.Lock()
	records := o.records
	o.records = nil
	for _, r := range records {
		o.withdrawnTotal += r.hostRouteCount()
	}
	o.mu.Unlock()
	for _, r := range records {
		o.emitRecordRemovals(r)
	}
}

func (o *subscriberRouteObserver) emitRecordRemovals(r *routeRecord) {
	if r.v4.IsValid() {
		o.emitRemove(r.v4, r.instance)
	}
	if r.v6.IsValid() {
		o.emitRemove(r.v6, r.instance)
	}
	o.emitFramedRoutes(redistevents.ActionRemove, r.liveFramedRoutes(), r.instance)
}

func routeSessionLabel(session *subscriber.Session) uint16 {
	if session.AccessType == subscriber.AccessPPPoE {
		return session.PPPoESID
	}
	return session.SessionID
}

// Stats returns the cumulative host route counters and current record count.
func (o *subscriberRouteObserver) Stats() (injected, withdrawn uint64, active int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.injectedTotal, o.withdrawnTotal, len(o.records)
}

// emitAdd builds and emits a single-entry add batch for the given address.
// Nil bus is tolerated (no emission, state still tracked).
func (o *subscriberRouteObserver) emitAdd(addr netip.Addr, instance uint32) {
	o.emitEntries(0, familyForAddr(addr), redistevents.RouteChangeEntry{
		Action:         redistevents.ActionAdd,
		Prefix:         prefixForAddr(addr),
		SourceInstance: instance,
	})
}

// emitRemove builds and emits a single-entry remove batch for the given address.
func (o *subscriberRouteObserver) emitRemove(addr netip.Addr, instance uint32) {
	o.emitEntries(0, familyForAddr(addr), redistevents.RouteChangeEntry{
		Action:         redistevents.ActionRemove,
		Prefix:         prefixForAddr(addr),
		SourceInstance: instance,
	})
}

// emitEntries builds and emits one batch for fam with the given entries, tagged
// with replayID (0 for the normal incremental path; nonzero echoes a
// redistribute ReplayRequest so the orchestrator can replay to a peer that
// established after injection). Central emit path for the observer. Nil bus or
// no entries is a no-op.
func (o *subscriberRouteObserver) emitEntries(replayID uint64, fam family.Family, entries ...redistevents.RouteChangeEntry) {
	if o.bus == nil || len(entries) == 0 {
		return
	}
	// An incremental emit takes emitMu shared; a replay emit runs inside
	// reemitAll, which holds it exclusive already.
	if replayID == 0 {
		o.emitMu.RLock()
		defer o.emitMu.RUnlock()
	}
	b := redistevents.AcquireBatch()
	defer redistevents.ReleaseBatch(b)
	b.Protocol = o.source
	b.AFI = uint16(fam.AFI)
	b.SAFI = uint8(fam.SAFI)
	b.ReplayID = replayID
	b.Entries = append(b.Entries, entries...)
	if _, err := o.emit.Emit(o.bus, b); err != nil {
		const problem = "Subscriber route notification failed"
		const fix = "ze show doctor"
		o.logger.Warn(problem, "code", report.CodeInternalDefect, "access-type", o.access, "error", err, "fix", fix)
		report.RaiseWarningFix(string(o.access), report.CodeInternalDefect, "subscriber route publication",
			problem, fix, map[string]any{reportFieldCause: err.Error()})
	}
}

// reemitAll re-emits every currently-tracked subscriber route (each session's
// live v4/v6 address and any already-emitted RADIUS framed routes) as adds
// tagged with replayID, so the redistribute orchestrator can replay them to a
// peer that establishes after the original emit. Reflects the CURRENT live set;
// a session torn down before the peer joined is absent. A zero replayID is a
// no-op (the orchestrator only allocates nonzero tokens).
func (o *subscriberRouteObserver) reemitAll(replayID uint64) {
	if o.bus == nil || replayID == 0 {
		return
	}
	type sessionAddress struct {
		address  netip.Addr
		instance uint32
	}
	var v4addrs, v6addrs []sessionAddress
	var v4framed, v6framed []redistevents.RouteChangeEntry
	o.emitMu.Lock()
	defer o.emitMu.Unlock()
	o.mu.Lock()
	for _, r := range o.records {
		if r.v4.IsValid() {
			v4addrs = append(v4addrs, sessionAddress{address: r.v4, instance: r.instance})
		}
		if r.v6.IsValid() {
			v6addrs = append(v6addrs, sessionAddress{address: r.v6, instance: r.instance})
		}
		for _, fr := range r.liveFramedRoutes() {
			e := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: fr.Prefix, Metric: fr.Metric, SourceInstance: r.instance}
			if fr.Prefix.Addr().Is4() {
				v4framed = append(v4framed, e)
			} else {
				v6framed = append(v6framed, e)
			}
		}
	}
	o.mu.Unlock()

	for _, a := range v4addrs {
		o.emitEntries(replayID, family.IPv4Unicast, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: prefixForAddr(a.address), SourceInstance: a.instance})
	}
	for _, a := range v6addrs {
		o.emitEntries(replayID, family.IPv6Unicast, redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: prefixForAddr(a.address), SourceInstance: a.instance})
	}
	if len(v4framed) > 0 {
		o.emitEntries(replayID, family.IPv4Unicast, v4framed...)
	}
	if len(v6framed) > 0 {
		o.emitEntries(replayID, family.IPv6Unicast, v6framed...)
	}
}

// emitFramedRoutes emits a batch per address family for the given
// framed routes. Groups routes by AFI so each batch has a consistent
// family header.
func (o *subscriberRouteObserver) emitFramedRoutes(action redistevents.RouteAction, routes []FramedRoute, instance uint32) {
	if o.bus == nil || len(routes) == 0 {
		return
	}
	var v4, v6 []redistevents.RouteChangeEntry
	for _, fr := range routes {
		entry := redistevents.RouteChangeEntry{
			Action:         action,
			Prefix:         fr.Prefix,
			Metric:         fr.Metric,
			SourceInstance: instance,
		}
		if fr.Prefix.Addr().Is4() {
			v4 = append(v4, entry)
		} else {
			v6 = append(v6, entry)
		}
	}
	if len(v4) > 0 {
		o.emitBatch(family.IPv4Unicast, v4)
	}
	if len(v6) > 0 {
		o.emitBatch(family.IPv6Unicast, v6)
	}
}

func (o *subscriberRouteObserver) emitBatch(fam family.Family, entries []redistevents.RouteChangeEntry) {
	o.emitEntries(0, fam, entries...)
}

// prefixForAddr returns /32 for IPv4, /128 for IPv6.
func prefixForAddr(addr netip.Addr) netip.Prefix {
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32)
	}
	return netip.PrefixFrom(addr, 128)
}

// familyForAddr returns ipv4/unicast for IPv4, ipv6/unicast for IPv6.
func familyForAddr(addr netip.Addr) family.Family {
	if addr.Is4() {
		return family.IPv4Unicast
	}
	return family.IPv6Unicast
}
