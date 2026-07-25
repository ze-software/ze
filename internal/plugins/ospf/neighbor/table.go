// Design: plan/learned/960-ospf-6-neighbor-nsm.md -- per-interface OSPFv2 neighbor table
// The IS-IS adjacency table uses the same snapshot and bounded-table pattern.

package neighbor

import (
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type tableKey struct {
	iface  string
	router types.RouterID
}

type eventEmission struct {
	up   bool
	down bool
	snap Snapshot
}

type LSDB interface {
	Lookup(types.AreaID, types.LSAKey) (packet.LSAHeader, bool)
	LookupLSA(types.AreaID, types.LSAKey) (packet.LSA, bool)
	Install(types.AreaID, packet.LSA) bool
	Summary(types.AreaID) []packet.LSAHeader
}

type linkScopeLSDB interface {
	LookupLink(string, types.LSAKey) (packet.LSAHeader, bool)
	LookupLinkLSA(string, types.LSAKey) (packet.LSA, bool)
	LinkLSAs(string) []packet.LSA
}

type Table struct {
	mu        sync.RWMutex
	cfg       map[string]InterfaceConfig
	neighbors map[tableKey]*Neighbor
	metrics   Metrics
	sink      EventSink
	sender    Sender
	encoder   Encoder
	lsdb      LSDB
	now       func() time.Time
}

func NewTable(m Metrics) *Table {
	if m.Neighbors == nil || m.AdjacenciesFull == nil || m.NSMEvents == nil {
		m = NopMetrics()
	}
	return &Table{
		cfg:       make(map[string]InterfaceConfig),
		neighbors: make(map[tableKey]*Neighbor),
		metrics:   m,
		encoder:   v4Encoder{},
		now:       time.Now,
	}
}

// SetEncoder installs the address-family packet encoder. The engine calls this for
// an OSPFv3 family; a table left untouched encodes OSPFv2 (the default).
func (t *Table) SetEncoder(e Encoder) {
	if e == nil {
		return
	}
	t.mu.Lock()
	t.encoder = e
	t.mu.Unlock()
}

func (t *Table) SetEventSink(s EventSink) {
	t.mu.Lock()
	t.sink = s
	t.mu.Unlock()
}

func (t *Table) SetSender(s Sender) {
	t.mu.Lock()
	t.sender = s
	t.mu.Unlock()
}

func (t *Table) SetLSDB(db LSDB) {
	t.mu.Lock()
	t.lsdb = db
	t.mu.Unlock()
}

func (t *Table) ConfigureInterface(cfg InterfaceConfig) {
	t.mu.Lock()
	t.cfg[cfg.Name] = cfg
	t.mu.Unlock()
}

func (t *Table) DeleteInterface(name string) {
	t.mu.Lock()
	for k, n := range t.neighbors {
		if k.iface != name {
			continue
		}
		t.setStateLocked(n, stateDown)
		delete(t.neighbors, k)
	}
	delete(t.cfg, name)
	t.mu.Unlock()
}

func (t *Table) Hello(in HelloInput) string {
	emit, reason := t.hello(in)
	t.emit(emit)
	return reason
}

func (t *Table) hello(in HelloInput) (eventEmission, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cfg := t.interfaceConfigLocked(in)
	if cfg.Name == "" {
		return eventEmission{}, reasonInterface
	}
	if in.NeighborID == cfg.RouterID {
		return eventEmission{}, "own-router-id"
	}
	t.recordEventLocked("hello-received")
	n, ok := t.lookupLocked(cfg.Name, in.NeighborID)
	if !ok {
		if len(t.neighbors) >= maxNeighbors {
			t.reapDownLocked()
		}
		if len(t.neighbors) >= maxNeighbors {
			return eventEmission{}, "neighbor-limit"
		}
		n = newNeighbor(cfg, in.NeighborID)
		t.neighbors[tableKey{iface: cfg.Name, router: in.NeighborID}] = n
	}
	n.AreaID = cfg.AreaID
	n.Address = in.Address
	n.Priority = in.Priority
	n.InterfaceID = in.InterfaceID
	n.DeclaredDR = in.DeclaredDR
	n.DeclaredBDR = in.DeclaredBDR
	n.LastSeen = in.Now
	if in.Now.IsZero() {
		n.LastSeen = t.now()
	}
	if cfg.DeadInterval > 0 {
		n.InactivityDeadline = n.LastSeen.Add(time.Duration(cfg.DeadInterval) * time.Second)
	}
	if n.State == stateDown {
		t.setStateLocked(n, stateInit)
	}
	if !in.TwoWay {
		t.recordNeighborEventLocked(n, "1-way-received")
		n.RequestList = nil
		n.hasLastDD = false
		return t.setStateLocked(n, stateInit), ""
	}
	t.recordNeighborEventLocked(n, "2-way-received")
	if n.State == stateInit {
		t.setStateLocked(n, stateTwoWay)
	}
	if shouldAdj(cfg, n) && n.State == stateTwoWay {
		t.startExchangeLocked(cfg, n)
		t.sendInitialDDLocked(cfg, n)
		return t.setStateLocked(n, stateExStart), ""
	}
	return eventEmission{}, ""
}

func (t *Table) AdjOK(interfaceName string, dr, bdr types.RouterID) {
	emits := t.adjOK(interfaceName, dr, bdr)
	for i := range emits {
		t.emit(emits[i])
	}
}

func (t *Table) adjOK(interfaceName string, dr, bdr types.RouterID) []eventEmission {
	t.mu.Lock()
	defer t.mu.Unlock()
	cfg, ok := t.cfg[interfaceName]
	if !ok {
		return nil
	}
	cfg.LocalDR = dr
	cfg.LocalBDR = bdr
	t.cfg[interfaceName] = cfg
	emits := make([]eventEmission, 0, 1)
	for _, n := range t.neighbors {
		if n.InterfaceName != interfaceName || n.State < stateTwoWay {
			continue
		}
		t.recordNeighborEventLocked(n, "adj-ok")
		if shouldAdj(cfg, n) {
			if n.State == stateTwoWay {
				t.startExchangeLocked(cfg, n)
				t.sendInitialDDLocked(cfg, n)
				emits = append(emits, t.setStateLocked(n, stateExStart))
			}
			continue
		}
		if n.State > stateTwoWay {
			n.RequestList = nil
			n.hasLastDD = false
			emits = append(emits, t.setStateLocked(n, stateTwoWay))
		}
	}
	return emits
}

func (t *Table) NeighborDown(interfaceName string, id types.RouterID) {
	emit := t.neighborDown(interfaceName, id)
	t.emit(emit)
}

func (t *Table) neighborDown(interfaceName string, id types.RouterID) eventEmission {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.lookupLocked(interfaceName, id)
	if !ok {
		return eventEmission{}
	}
	t.recordNeighborEventLocked(n, "kill-nbr")
	n.RequestList = nil
	n.hasLastDD = false
	return t.setStateLocked(n, stateDown)
}

func (t *Table) InterfaceDown(interfaceName string) {
	emits := t.interfaceDown(interfaceName)
	for i := range emits {
		t.emit(emits[i])
	}
}

func (t *Table) interfaceDown(interfaceName string) []eventEmission {
	t.mu.Lock()
	defer t.mu.Unlock()
	emits := make([]eventEmission, 0)
	for _, n := range t.neighbors {
		if n.InterfaceName != interfaceName {
			continue
		}
		t.recordNeighborEventLocked(n, "ll-down")
		n.RequestList = nil
		n.hasLastDD = false
		emits = append(emits, t.setStateLocked(n, stateDown))
	}
	return emits
}

// ResetAll tears down every neighbor (clear ospf neighbor / process): each drops to
// Down and re-forms from the next Hello. Mirrors Expire's lock discipline (collect
// emissions under the lock, emit outside it). Returns the number of neighbors reset.
func (t *Table) ResetAll() int {
	emits, n := t.resetAll()
	for i := range emits {
		t.emit(emits[i])
	}
	return n
}

func (t *Table) resetAll() ([]eventEmission, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	emits := make([]eventEmission, 0, len(t.neighbors))
	count := 0
	for _, n := range t.neighbors {
		if n.State == stateDown {
			continue
		}
		t.recordNeighborEventLocked(n, "kill-nbr")
		n.RequestList = nil
		n.hasLastDD = false
		emits = append(emits, t.setStateLocked(n, stateDown))
		count++
	}
	return emits, count
}

func (t *Table) Expire(now time.Time) int {
	emits, n := t.expire(now)
	for i := range emits {
		t.emit(emits[i])
	}
	return n
}

func (t *Table) expire(now time.Time) ([]eventEmission, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	emits := make([]eventEmission, 0)
	count := 0
	for _, n := range t.neighbors {
		if n.State == stateDown || n.InactivityDeadline.IsZero() || now.Before(n.InactivityDeadline) {
			continue
		}
		t.recordNeighborEventLocked(n, "inactivity-timer")
		n.RequestList = nil
		n.hasLastDD = false
		emits = append(emits, t.setStateLocked(n, stateDown))
		count++
	}
	return emits, count
}

func (t *Table) Snapshot() []Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()
	now := t.now()
	out := make([]Snapshot, 0, len(t.neighbors))
	for _, n := range t.neighbors {
		out = append(out, snapshotOf(n, now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Interface != out[j].Interface {
			return out[i].Interface < out[j].Interface
		}
		return out[i].RouterID < out[j].RouterID
	})
	return out
}

// DetailSnapshot returns the full per-neighbor state (spec-ospf-ext-14 `... neighbor
// detail`). It is additive over Snapshot; the summary shape is unchanged.
func (t *Table) DetailSnapshot() []Detail {
	t.mu.RLock()
	defer t.mu.RUnlock()
	now := t.now()
	out := make([]Detail, 0, len(t.neighbors))
	for _, n := range t.neighbors {
		out = append(out, detailOf(n, now))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Interface != out[j].Interface {
			return out[i].Interface < out[j].Interface
		}
		return out[i].RouterID < out[j].RouterID
	})
	return out
}

func (t *Table) Lookup(interfaceName string, id types.RouterID) (Snapshot, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.neighbors[tableKey{iface: interfaceName, router: id}]
	if !ok {
		return Snapshot{}, false
	}
	return snapshotOf(n, t.now()), true
}

// AddressOf returns the reachable source address of the neighbor with the given
// Router ID (any interface), used as the OSPFv3 SPF next-hop (the neighbor's IPv6
// link-local). Only adjacencies at Exchange or beyond have a usable address. On a
// point-to-point link a Router ID appears on a single interface, so the first match
// is unambiguous.
func (t *Table) AddressOf(id types.RouterID) (netip.Addr, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for key, n := range t.neighbors {
		if key.router == id && n.State >= stateExchange && n.Address.IsValid() {
			return n.Address, true
		}
	}
	return netip.Addr{}, false
}

// NeighborAddress returns the raw reachable source address of the neighbor identified by
// (interfaceName, id): the IPv4 address for OSPFv2, the IPv6 link-local for OSPFv3. It is
// the BFD session Peer. Unlike Snapshot.Address (a string), this returns the raw netip.Addr
// so an IPv6 zone is never lost to a parse round-trip (spec R-2). The second return is false
// when the neighbor is absent or has no valid address yet.
func (t *Table) NeighborAddress(interfaceName string, id types.RouterID) (netip.Addr, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.neighbors[tableKey{iface: interfaceName, router: id}]
	if !ok || !n.Address.IsValid() {
		return netip.Addr{}, false
	}
	return n.Address, true
}

// FloodNeighbors returns Exchange, Loading, and Full neighbors on interfaceName.
func (t *Table) FloodNeighbors(interfaceName string) []FloodNeighbor {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]FloodNeighbor, 0, len(t.neighbors))
	for key, n := range t.neighbors {
		if key.iface != interfaceName || n.State < stateExchange {
			continue
		}
		out = append(out, FloodNeighbor{RouterID: n.RouterID, Address: n.Address, State: n.State.String(), InterfaceID: n.InterfaceID, OpaqueCapable: n.Options.Has(types.OptionO)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RouterID.String() < out[j].RouterID.String() })
	return out
}

// AcceptsFlooding returns an empty reason when a neighbor may send LS Update or
// LS Ack packets on interfaceName. OSPF flooding starts in Exchange.
func (t *Table) AcceptsFlooding(interfaceName string, id types.RouterID) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if _, ok := t.cfg[interfaceName]; !ok {
		return reasonInterface
	}
	n, ok := t.lookupLocked(interfaceName, id)
	if !ok {
		return reasonNeighbor
	}
	if n.State < stateExchange {
		return reasonState
	}
	return ""
}

func (t *Table) interfaceConfigLocked(in HelloInput) InterfaceConfig {
	cfg, ok := t.cfg[in.InterfaceName]
	if !ok {
		return InterfaceConfig{}
	}
	if in.AreaID != (types.AreaID{}) {
		cfg.AreaID = in.AreaID
	}
	if in.LocalRouterID != (types.RouterID{}) {
		cfg.RouterID = in.LocalRouterID
	}
	if in.NetworkType != "" {
		cfg.NetworkType = in.NetworkType
	}
	if in.DeadInterval != 0 {
		cfg.DeadInterval = in.DeadInterval
	}
	if in.InterfaceMTU != 0 {
		cfg.InterfaceMTU = in.InterfaceMTU
	}
	if in.MTUIgnore {
		cfg.MTUIgnore = true
	}
	if in.LocalDR != (types.RouterID{}) || in.LocalBDR != (types.RouterID{}) {
		cfg.LocalDR = in.LocalDR
		cfg.LocalBDR = in.LocalBDR
	}
	return cfg
}

func (t *Table) startExchangeLocked(cfg InterfaceConfig, n *Neighbor) {
	startExchange(cfg, n)
	if t.lsdb != nil {
		n.SummaryList = t.databaseSummaryLocked(cfg)
	}
}

func (t *Table) databaseSummaryLocked(cfg InterfaceConfig) []packet.LSAHeader {
	out := t.lsdb.Summary(cfg.AreaID)
	linkDB, ok := t.lsdb.(linkScopeLSDB)
	if !ok {
		return out
	}
	for _, lsa := range linkDB.LinkLSAs(cfg.Name) {
		out = append(out, lsa.Header)
	}
	return out
}

func (t *Table) restartExchangeLocked(cfg InterfaceConfig, n *Neighbor) {
	t.startExchangeLocked(cfg, n)
}

func (t *Table) armDDRetransmitLocked(cfg InterfaceConfig, n *Neighbor) {
	if cfg.RetransmitInterval == 0 {
		return
	}
	n.ddRetransmitDeadline = t.now().Add(time.Duration(cfg.RetransmitInterval) * time.Second)
}

func (t *Table) armLSReqRetransmitLocked(cfg InterfaceConfig, n *Neighbor) {
	if cfg.RetransmitInterval == 0 {
		return
	}
	n.lsReqRetransmitDeadline = t.now().Add(time.Duration(cfg.RetransmitInterval) * time.Second)
}

func (t *Table) Retransmit(now time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	sent := 0
	for _, n := range t.neighbors {
		cfg, ok := t.cfg[n.InterfaceName]
		if !ok {
			continue
		}
		if (n.State == stateExStart || n.State == stateExchange) && !n.ddRetransmitDeadline.IsZero() && !now.Before(n.ddRetransmitDeadline) {
			t.resendLastDDLocked(cfg, n)
			t.armDDRetransmitLocked(cfg, n)
			sent++
		}
		if n.State == stateLoading && !n.lsReqRetransmitDeadline.IsZero() && !now.Before(n.lsReqRetransmitDeadline) {
			t.resendLastLSReqLocked(cfg, n)
			t.armLSReqRetransmitLocked(cfg, n)
			sent++
		}
	}
	return sent
}

func (t *Table) reapDownLocked() {
	for k, n := range t.neighbors {
		if n.State == stateDown {
			delete(t.neighbors, k)
		}
	}
}

func (t *Table) lookupLocked(interfaceName string, id types.RouterID) (*Neighbor, bool) {
	n, ok := t.neighbors[tableKey{iface: interfaceName, router: id}]
	return n, ok
}

func (t *Table) setStateLocked(n *Neighbor, next state) eventEmission {
	prev := n.State
	if prev == next {
		return eventEmission{}
	}
	n.State = next
	switch next {
	case stateDown, stateTwoWay, stateFull:
		n.ddRetransmitDeadline = time.Time{}
		n.lsReqRetransmitDeadline = time.Time{}
		n.hasLastLSReq = false
		n.lastLSReqs = nil
	case stateLoading:
		n.ddRetransmitDeadline = time.Time{}
	default:
	}
	if prev != stateDown {
		t.metrics.Neighbors.With(n.AreaID.String(), n.InterfaceName, prev.String()).Set(float64(t.countStateLocked(n.AreaID, n.InterfaceName, prev)))
	}
	if next != stateDown {
		t.metrics.Neighbors.With(n.AreaID.String(), n.InterfaceName, next.String()).Set(float64(t.countStateLocked(n.AreaID, n.InterfaceName, next)))
	}
	snap := snapshotOf(n, t.now())
	switch {
	case prev != stateFull && next == stateFull:
		t.metrics.AdjacenciesFull.With(n.AreaID.String()).Inc()
		return eventEmission{up: true, snap: snap}
	case prev == stateFull && next != stateFull:
		t.metrics.AdjacenciesFull.With(n.AreaID.String()).Dec()
		return eventEmission{down: true, snap: snap}
	default:
		return eventEmission{}
	}
}

func (t *Table) countStateLocked(area types.AreaID, interfaceName string, st state) int {
	count := 0
	for _, n := range t.neighbors {
		if n.AreaID == area && n.InterfaceName == interfaceName && n.State == st {
			count++
		}
	}
	return count
}

func (t *Table) recordEventLocked(event string) {
	t.metrics.NSMEvents.With(event).Inc()
}

// recordNeighborEventLocked records the last NSM event on the neighbor (for the ext-14
// `show ospf neighbor detail` last-event field) and the process-wide event counter.
func (t *Table) recordNeighborEventLocked(n *Neighbor, event string) {
	n.LastEvent = event
	t.recordEventLocked(event)
}

func (t *Table) emit(ev eventEmission) {
	if t == nil || t.sink == nil {
		return
	}
	if ev.up {
		t.sink.NeighborUp(ev.snap)
	}
	if ev.down {
		t.sink.NeighborDown(ev.snap)
	}
}
