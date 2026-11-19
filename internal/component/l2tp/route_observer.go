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

// routeRecord is the live state the observer tracks per session.
type routeRecord struct {
	instance      uint32
	network       l2tpevents.NetworkLifetime
	username      string
	v4            netip.Addr
	v6            netip.Addr
	framedRoutes  []FramedRoute
	framedEmitted bool
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

// StartSubscriberRouteObserver subscribes to assignment, down and replay for
// one access type. The owning transport MUST start it before emitting notices
// and retain it until its terminal notices have all returned.
func StartSubscriberRouteObserver(logger *slog.Logger, bus ze.EventBus, access subscriber.AccessType,
	source redistevents.ProtocolID, emit *events.Event[*redistevents.RouteChangeBatch],
) *SubscriberRouteSubscription {
	subscription := &SubscriberRouteSubscription{observer: newAccessRouteObserver(logger, bus, access, source, emit)}
	if bus != nil {
		subscription.unsubs = []func(){
			subevents.SessionIPAssigned.Subscribe(bus, func(p *subevents.SessionIPAssignedPayload) {
				subscription.deliver(func() { subscription.observer.assign(p.Session, p.NetworkLifetime, p.Address, p.FramedRoutes) })
			}),
			subevents.SessionDown.Subscribe(bus, func(p *subevents.SessionDownPayload) {
				subscription.deliver(func() { subscription.observer.withdraw(p.Session, p.NetworkLifetime) })
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

func (o *subscriberRouteObserver) ownsSession(session subscriber.Session, operation string) bool {
	if session.AccessType != o.access {
		return false
	}
	if o.access == subscriber.AccessL2TP && (!session.L2TPBirth.IsValid() || session.L2TPBirth.String() != session.ID) {
		reportSessionBirthRefusal(o.logger, operation, session.TunnelID, session.SessionID)
		return false
	}
	return subscriber.RequireID(o.logger, session.ID) == nil
}

// assign tracks each family under the same subscriber ID and copies framed
// routes from the first address assignment of that network lifetime.
func (o *subscriberRouteObserver) assign(session subscriber.Session, network l2tpevents.NetworkLifetime, addr netip.Addr, framedRoutes []FramedRoute) {
	if !addr.IsValid() || !o.ownsSession(session, "route assignment") {
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
	if addr.Is4() {
		prev = r.v4
		r.v4 = addr
	} else {
		prev = r.v6
		r.v6 = addr
	}
	var emitFramed []FramedRoute
	if !r.framedEmitted && len(r.framedRoutes) > 0 {
		emitFramed = r.framedRoutes
		r.framedEmitted = true
	}
	o.injectedTotal++
	instance := r.instance
	username := r.username
	o.mu.Unlock()
	o.logger.Info(string(o.access)+": subscriber route inject",
		"session-id", routeSessionLabel(session),
		"username", username,
		"address", addr.String(),
		"family", familyOf(addr))
	if prev.IsValid() && prev != addr {
		o.emitRemove(prev, instance)
	}
	o.emitAdd(addr, instance)
	if len(emitFramed) > 0 {
		o.emitFramedRoutes(redistevents.ActionAdd, emitFramed, instance)
	}
}

// withdraw removes each recorded host route and all captured framed routes
// for this subscriber and network lifetime.
func (o *subscriberRouteObserver) withdraw(session subscriber.Session, network l2tpevents.NetworkLifetime) {
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
		"session-id", routeSessionLabel(session),
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
	if r.framedEmitted {
		o.emitFramedRoutes(redistevents.ActionRemove, r.framedRoutes, r.instance)
	}
}

func routeSessionLabel(session subscriber.Session) uint16 {
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
			problem, fix, map[string]any{"cause": err.Error()})
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
		if r.framedEmitted {
			for _, fr := range r.framedRoutes {
				e := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: fr.Prefix, Metric: fr.Metric, SourceInstance: r.instance}
				if fr.Prefix.Addr().Is4() {
					v4framed = append(v4framed, e)
				} else {
					v6framed = append(v6framed, e)
				}
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

// familyOf returns "ipv4" or "ipv6" for the given address.
func familyOf(a netip.Addr) string {
	if a.Is4() {
		return "ipv4"
	}
	return "ipv6"
}
