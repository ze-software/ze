// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- per-interface OSPFv2 runtime
// RFC: rfc/short/rfc2328.md (sec 9.5 Hello), rfc/short/rfc5340.md (sec 2.9 ff02::5)
//
// "Hello packets are sent periodically out all interfaces."

package iface

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const maxNeighbors = 1024

var (
	// allSPFRouters is the OSPFv2 AllSPFRouters multicast (224.0.0.5). allSPFRoutersV6 is
	// the OSPFv3 equivalent ff02::5 (RFC 5340 sec 2.9); a v6 interface must send Hellos to
	// it, not the IPv4 group -- the raw IPv6 socket rejects an IPv4 destination.
	allSPFRouters   = transport.AllSPFRouters
	allSPFRoutersV6 = netip.MustParseAddr("ff02::5")
)

type Sender interface {
	SendPacket(name string, dst netip.Addr, payload []byte) error
	JoinAllDRouters(name string) error
	LeaveAllDRouters(name string) error
}

// Encoder encodes the interface's outgoing Hello for its address family. The
// default encodes OSPFv2 (ospf/packet); the engine injects an OSPFv3 encoder
// (ospfv3/packet) for an IPv6 interface via SetEncoder. The interface builds the
// AF-neutral packet.Hello superset (NetworkMask for v2, InterfaceID for v6) and
// the encoder serializes the version it implements.
type Encoder interface {
	EncodeHello(routerID types.RouterID, areaID types.AreaID, h packet.Hello) []byte
}

// v4HelloEncoder is the OSPFv2 Hello encoder (the default). instanceID is the interface's
// RFC 6549 OSPFv2 Instance ID, stamped into the common header (offset 14) so every Hello
// carries the engine's Instance ID; instanceID 0 is the base instance and its bytes are
// identical to base OSPFv2.
type v4HelloEncoder struct {
	instanceID uint8
}

func (e v4HelloEncoder) EncodeHello(routerID types.RouterID, areaID types.AreaID, h packet.Hello) []byte {
	p := packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, Hello: &h}
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

type Metrics struct {
	InterfaceUp metrics.GaugeVec
	DRElections metrics.CounterVec
	// NBMANeighbors is the configured NBMA neighbor count by interface, af, and poll
	// state (attempt = silent/polled, heard = a Hello has been received).
	NBMANeighbors metrics.GaugeVec
	// NBMAPolls counts poll-rate (slow) Hellos sent to silent NBMA neighbors by
	// interface and af.
	NBMAPolls metrics.CounterVec
	// PTMPHostRoutes is the number of point-to-multipoint host routes this interface
	// contributes (1 when a PtMP interface is up), by interface and af.
	PTMPHostRoutes metrics.GaugeVec
}

func NopMetrics() Metrics {
	nop := metrics.NopRegistry{}
	return Metrics{
		InterfaceUp:    nop.GaugeVec("", "", nil),
		DRElections:    nop.CounterVec("", "", nil),
		NBMANeighbors:  nop.GaugeVec("", "", nil),
		NBMAPolls:      nop.CounterVec("", "", nil),
		PTMPHostRoutes: nop.GaugeVec("", "", nil),
	}
}

type Config struct {
	Name               string
	RouterID           types.RouterID
	AreaID             types.AreaID
	AreaType           string
	NetworkType        string
	NetworkMask        [4]byte
	InterfaceAddress   [4]byte
	Cost               uint16
	HelloInterval      uint16
	DeadInterval       uint16
	Priority           uint8
	Passive            bool
	InterfaceMTU       uint16
	MTUIgnore          bool
	RetransmitInterval uint16
	// IsV6 marks an OSPFv3 (IPv6) interface so the neighbor FSM applies AF-aware checks: the
	// Network Mask match is OSPFv2-only (OSPFv3 carries an Interface ID instead).
	IsV6 bool
	// InterfaceID is the OSPFv3 Interface ID (RFC 5340 sec 3.4.3) advertised in this
	// interface's Hellos; it must match the Interface ID the engine uses for this
	// interface's links in the Router-LSA. The engine sets it to the OS ifindex. It is
	// OSPFv3-only: the OSPFv2 wire carries a Network Mask in its place, so the v2 encoder
	// ignores it.
	InterfaceID uint32
	// InstanceID is the RFC 6549 OSPFv2 Interface Instance ID stamped into every packet
	// this interface transmits (header offset 14). The default v4 Hello encoder is built
	// with it in New; 0 is the base instance. It is distinct from the OSPFv3 Instance ID,
	// which the engine threads through the v6 encoder instead.
	InstanceID uint8
	// PollInterval is the RFC 2328 App C.5 NBMA poll rate (seconds): a configured but
	// currently silent NBMA neighbor is sent a Hello at this slower rate rather than at
	// HelloInterval. It applies only when NetworkType is NetworkNBMA.
	PollInterval uint16
	// NBMANeighbors is the statically configured neighbor list for a non-broadcast
	// interface (RFC 2328 App C.6): NBMA always, and the non-broadcast point-to-multipoint
	// variant. Hellos are unicast to these neighbors instead of a multicast group.
	NBMANeighbors []NBMANeighbor
	// BFD (RFC 5880 / RFC 5881) per-interface single-hop failure detection. AF-neutral:
	// the engine opens a session per Full neighbor when BFDEnabled and the BFD plugin is
	// loaded. Timers are microseconds (the api.SessionRequest unit); multiplier is Detect
	// Mult. Zero-value = BFD off (the interface runs on Hello/Dead timers alone).
	BFDEnabled    bool
	BFDMinTxUs    uint32
	BFDMinRxUs    uint32
	BFDMultiplier uint8
}

// Detail is the full per-interface state for `show ospf interface detail`
// (spec-ospf-ext-14): the ISM state, DR/BDR, all three timers, and (OSPFv3) the local
// Interface ID + Instance ID. Additive over Snapshot; the summary shape is unchanged.
type Detail struct {
	Name               string `json:"name"`
	Area               string `json:"area"`
	State              string `json:"state"`
	NetworkType        string `json:"network-type"`
	Cost               uint16 `json:"cost"`
	Priority           uint8  `json:"priority"`
	Passive            bool   `json:"passive"`
	DR                 string `json:"dr"`
	BDR                string `json:"bdr"`
	HelloInterval      uint16 `json:"hello-interval"`
	DeadInterval       uint16 `json:"dead-interval"`
	RetransmitInterval uint16 `json:"retransmit-interval"`
	InterfaceID        uint32 `json:"interface-id,omitempty"`
	InstanceID         uint8  `json:"instance-id"`
	IsV6               bool   `json:"ipv6"`
	NeighborCount      int    `json:"neighbor-count"`
}

// NBMANeighbor is one statically configured neighbor on a non-broadcast interface
// (RFC 2328 App C.6). Address is the IPv4 unicast Hello destination (OSPFv2).
// RouterID and LinkLocal identify the OSPFv3 neighbor; LinkLocal is the unicast
// destination (configured, or learned from the neighbor's first Hello). Priority 0
// marks the neighbor ineligible for the DR/BDR election (RFC 2328 sec 9.4 step 6
// still sends it a Start Hello once this router becomes DR/BDR).
type NBMANeighbor struct {
	Address   netip.Addr
	RouterID  types.RouterID
	LinkLocal netip.Addr
	Priority  uint8
}

type Neighbor struct {
	RouterID types.RouterID
	// Address is the neighbor's reachable source address (the IP the Hello arrived
	// from): an IPv4 address for OSPFv2, an IPv6 link-local for OSPFv3. It is the
	// unicast destination for DD/LSReq/LSUpdate and the SPF next-hop. DeclaredDR/BDR
	// stay [4]byte: in OSPFv2 they are interface addresses, in OSPFv3 Router IDs.
	Address     netip.Addr
	Priority    uint8
	TwoWay      bool
	DeclaredDR  [4]byte
	DeclaredBDR [4]byte
	LastSeen    time.Time
	// InterfaceID is the OSPFv3 Interface ID this neighbor advertises in its Hellos
	// (RFC 5340 sec 3.4.3); it is echoed as the Neighbor Interface ID in this router's
	// Router-LSA link. Zero for OSPFv2 (which has no Interface ID).
	InterfaceID uint32
}

type Snapshot struct {
	Name        string `json:"name"`
	Area        string `json:"area"`
	State       string `json:"state"`
	NetworkType string `json:"network_type"`
	Cost        uint16 `json:"cost"`
	Priority    uint8  `json:"priority"`
	Passive     bool   `json:"passive"`
	// BFD reports whether RFC 5880 single-hop BFD is enabled on this interface, so
	// `show ospf interface` can surface it (AC-1).
	BFD           bool   `json:"bfd,omitempty"`
	DR            string `json:"dr"`
	BDR           string `json:"bdr"`
	HelloInterval uint16 `json:"hello_interval"`
	DeadInterval  uint16 `json:"dead_interval"`
	NeighborCount int    `json:"neighbor_count"`
	// PollInterval and NBMANeighbors are populated only for an NBMA interface (RFC
	// 2328 App C.5/C.6); omitempty keeps every other network type's snapshot
	// byte-for-byte as before.
	PollInterval  uint16                 `json:"poll-interval,omitempty"`
	NBMANeighbors []NBMANeighborSnapshot `json:"nbma-neighbors,omitempty"`
}

// NBMANeighborSnapshot renders one configured NBMA neighbor for `show ospf interface`:
// its identity (IPv4 address or IPv6 Router ID), election priority, and whether a Hello
// has been heard (heard) or it is still being polled (attempt), RFC 2328 sec 10.1.
type NBMANeighborSnapshot struct {
	Neighbor string `json:"neighbor"`
	Priority uint8  `json:"priority"`
	State    string `json:"state"`
}

type EventSink interface {
	InterfaceStateChanged(Snapshot)
	DRChanged(Snapshot)
	NeighborChanged(Snapshot)
}

type NeighborEvent struct {
	InterfaceName string
	AreaID        types.AreaID
	LocalRouterID types.RouterID
	LocalDR       types.RouterID
	LocalBDR      types.RouterID
	NeighborID    types.RouterID
	Address       netip.Addr
	Priority      uint8
	TwoWay        bool
	DeclaredDR    [4]byte
	DeclaredBDR   [4]byte
	NetworkType   string
	DeadInterval  uint16
	InterfaceMTU  uint16
	MTUIgnore     bool
	// InterfaceID is the neighbor's advertised OSPFv3 Interface ID (zero for OSPFv2).
	InterfaceID uint32
}

type NeighborSink interface {
	NeighborHello(NeighborEvent)
	NeighborDown(interfaceName string, id types.RouterID)
	AdjOK(interfaceName string, dr, bdr types.RouterID)
	InterfaceDown(interfaceName string)
}

type nopNeighborSink struct{}

func (nopNeighborSink) NeighborHello(NeighborEvent)                  {}
func (nopNeighborSink) NeighborDown(string, types.RouterID)          {}
func (nopNeighborSink) AdjOK(string, types.RouterID, types.RouterID) {}
func (nopNeighborSink) InterfaceDown(string)                         {}

type nopEventSink struct{}

func (nopEventSink) InterfaceStateChanged(Snapshot) {}
func (nopEventSink) DRChanged(Snapshot)             {}
func (nopEventSink) NeighborChanged(Snapshot)       {}

type interfaceEvents struct {
	state    bool
	dr       bool
	neighbor bool
	snapshot Snapshot
}

func (ev interfaceEvents) emit(sink EventSink) {
	if sink == nil {
		return
	}
	if ev.state {
		sink.InterfaceStateChanged(ev.snapshot)
	}
	if ev.dr {
		sink.DRChanged(ev.snapshot)
	}
	if ev.neighbor {
		sink.NeighborChanged(ev.snapshot)
	}
}

type Interface struct {
	mu           sync.Mutex
	cfg          Config
	sender       Sender
	encoder      Encoder
	metrics      Metrics
	sink         EventSink
	neighborSink NeighborSink

	state     State
	neighbors map[types.RouterID]Neighbor
	dr        types.RouterID
	bdr       types.RouterID
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	// nbmaLastPoll records, per configured NBMA neighbor unicast destination, when a
	// poll-rate Hello was last sent to it while silent (RFC 2328 sec 10.1 Attempt).
	nbmaLastPoll map[netip.Addr]time.Time
}

func New(cfg Config, sender Sender, m Metrics) *Interface {
	if m.InterfaceUp == nil || m.DRElections == nil || m.NBMANeighbors == nil || m.NBMAPolls == nil || m.PTMPHostRoutes == nil {
		m = NopMetrics()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Interface{
		cfg:          cfg,
		sender:       sender,
		encoder:      v4HelloEncoder{instanceID: cfg.InstanceID},
		metrics:      m,
		sink:         nopEventSink{},
		neighborSink: nopNeighborSink{},
		state:        StateDown,
		neighbors:    make(map[types.RouterID]Neighbor),
		ctx:          ctx,
		cancel:       cancel,
		nbmaLastPoll: make(map[netip.Addr]time.Time),
	}
}

func (i *Interface) SetEventSink(s EventSink) {
	if s == nil {
		return
	}
	i.mu.Lock()
	i.sink = s
	i.mu.Unlock()
}

func (i *Interface) SetNeighborSink(s NeighborSink) {
	if s == nil {
		return
	}
	i.mu.Lock()
	i.neighborSink = s
	i.mu.Unlock()
}

// SetEncoder installs the address-family Hello encoder. The engine calls this for
// an OSPFv3 interface; an interface left untouched encodes OSPFv2 (the default).
func (i *Interface) SetEncoder(e Encoder) {
	if e == nil {
		return
	}
	i.mu.Lock()
	i.encoder = e
	i.mu.Unlock()
}

func (i *Interface) Start() {
	i.mu.Lock()
	startTimers := true
	if i.cfg.Passive {
		i.state = StateDown
		i.setUpLocked(false)
		startTimers = false
	} else {
		switch i.cfg.NetworkType {
		case NetworkLoopback:
			i.state = StateLoopback
			i.setUpLocked(false)
			startTimers = false
		case NetworkPointToPoint, NetworkPointToMultipoint:
			// RFC 2328 sec 9.5: point-to-multipoint is treated as a collection of
			// point-to-point links -- the point-to-point ISM state, no Waiting, no
			// DR/BDR election.
			i.state = StatePointToPoint
			i.setUpLocked(true)
			i.setPTMPHostRouteLocked(i.cfg.NetworkType == NetworkPointToMultipoint)
		default:
			// Broadcast and NBMA (RFC 2328 sec 9.3): an eligible interface waits for the
			// election; a priority-0 interface goes straight to DROther.
			if i.cfg.Priority == 0 {
				i.state = StateDROther
			} else {
				i.state = StateWaiting
			}
			i.setUpLocked(true)
		}
	}
	helloInterval := i.cfg.HelloInterval
	deadInterval := i.cfg.DeadInterval
	networkType := i.cfg.NetworkType
	priority := i.cfg.Priority
	ev := i.eventsLocked(false, false)
	sink := i.sink
	i.mu.Unlock()
	ev.emit(sink)

	if !startTimers {
		return
	}
	if helloInterval > 0 {
		i.wg.Go(func() { i.helloLoop(time.Duration(helloInterval) * time.Second) })
	}
	if deadInterval > 0 {
		i.wg.Go(func() { i.inactivityLoop(time.Duration(deadInterval) * time.Second) })
	}
	if (networkType == NetworkBroadcast || networkType == NetworkNBMA) && priority > 0 && deadInterval > 0 {
		i.wg.Go(func() { i.waitTimer(time.Duration(deadInterval) * time.Second) })
	}
}

func (i *Interface) Stop() {
	i.cancel()
	i.wg.Wait()
	i.mu.Lock()
	wasDRMember := i.dr == i.cfg.RouterID || i.bdr == i.cfg.RouterID
	sender := i.sender
	name := i.cfg.Name
	i.state = StateDown
	i.neighbors = make(map[types.RouterID]Neighbor)
	i.dr = types.RouterID{}
	i.bdr = types.RouterID{}
	i.setUpLocked(false)
	ev := i.eventsLocked(false, true)
	sink := i.sink
	neighborSink := i.neighborSink
	i.mu.Unlock()
	if wasDRMember && sender != nil {
		_ = sender.LeaveAllDRouters(name)
	}
	neighborSink.InterfaceDown(name)
	ev.emit(sink)
}

func (i *Interface) State() State {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.state
}

func (i *Interface) DR() types.RouterID {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.dr
}

func (i *Interface) BDR() types.RouterID {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.bdr
}

func (i *Interface) forceWaitTimer() { i.runElection() }

// ReceiveDecodedHello processes a Hello already decoded by the engine's codec (the codec is
// version-specific; the interface FSM is AF-neutral). src is the datagram source for neighbor
// addressing (OSPFv2) and is ignored for the Router-ID-keyed OSPFv3 path.
func (i *Interface) ReceiveDecodedHello(router types.RouterID, src netip.Addr, h packet.Hello, now time.Time) string {
	return i.receiveHello(router, src, h, now)
}

func (i *Interface) ReceiveHello(router types.RouterID, h packet.Hello, now time.Time) string {
	return i.receiveHello(router, netip.AddrFrom4([4]byte(router)), h, now)
}

func (i *Interface) receiveHello(router types.RouterID, src netip.Addr, h packet.Hello, now time.Time) string {
	i.mu.Lock()
	if reason := i.validateHelloLocked(h); reason != "" {
		i.mu.Unlock()
		return reason
	}
	if len(i.neighbors) >= maxNeighbors {
		if _, ok := i.neighbors[router]; !ok {
			i.mu.Unlock()
			return "neighbor-limit"
		}
	}
	old, had := i.neighbors[router]
	n := Neighbor{
		RouterID:    router,
		Address:     src,
		Priority:    h.Priority,
		TwoWay:      helloHasNeighbor(h, i.cfg.RouterID),
		DeclaredDR:  h.DR,
		DeclaredBDR: h.BDR,
		LastSeen:    now,
		InterfaceID: h.InterfaceID,
	}
	i.neighbors[router] = n
	// RFC 2328 sec 9.2 BackupSeen: while Waiting, end the wait only when a neighbor declares
	// ITSELF the Backup DR, or declares ITSELF the DR with no Backup DR. A Hello that merely
	// names some other router as DR/BDR does not end the wait (the old condition was broader).
	backupSeen := i.state == StateWaiting && n.TwoWay &&
		(n.DeclaredBDR == [4]byte(router) || (n.DeclaredDR == [4]byte(router) && n.DeclaredBDR == ([4]byte{})))
	neighborChange := !had || old.TwoWay != n.TwoWay || old.Priority != n.Priority || old.DeclaredDR != n.DeclaredDR || old.DeclaredBDR != n.DeclaredBDR
	ev := interfaceEvents{}
	if backupSeen || (neighborChange && (i.state == StateDROther || i.state == StateBackup || i.state == StateDR)) {
		ev = i.runElectionLocked()
	}
	if neighborChange {
		ev.neighbor = true
		ev.snapshot = i.snapshotLocked()
	}
	neighborUpdate := i.neighborEventLocked(n)
	neighborSink := i.neighborSink
	adjOK := ev.dr
	sink := i.sink
	i.mu.Unlock()
	neighborSink.NeighborHello(neighborUpdate)
	if adjOK {
		neighborSink.AdjOK(neighborUpdate.InterfaceName, neighborUpdate.LocalDR, neighborUpdate.LocalBDR)
	}
	ev.emit(sink)
	return ""
}

func (i *Interface) neighborEventLocked(n Neighbor) NeighborEvent {
	return NeighborEvent{
		InterfaceName: i.cfg.Name,
		AreaID:        i.cfg.AreaID,
		LocalRouterID: i.cfg.RouterID,
		LocalDR:       i.dr,
		LocalBDR:      i.bdr,
		NeighborID:    n.RouterID,
		Address:       n.Address,
		Priority:      n.Priority,
		TwoWay:        n.TwoWay,
		DeclaredDR:    n.DeclaredDR,
		DeclaredBDR:   n.DeclaredBDR,
		NetworkType:   i.cfg.NetworkType,
		DeadInterval:  i.cfg.DeadInterval,
		InterfaceMTU:  i.cfg.InterfaceMTU,
		MTUIgnore:     i.cfg.MTUIgnore,
		InterfaceID:   n.InterfaceID,
	}
}

func (i *Interface) expireNeighbors(now time.Time) int {
	i.mu.Lock()
	dead := time.Duration(i.cfg.DeadInterval) * time.Second
	removedIDs := make([]types.RouterID, 0)
	for id, n := range i.neighbors {
		if dead > 0 && now.Sub(n.LastSeen) >= dead {
			delete(i.neighbors, id)
			removedIDs = append(removedIDs, id)
		}
	}
	removed := len(removedIDs)
	ev := interfaceEvents{}
	if removed > 0 {
		ev = i.runElectionLocked()
		ev.neighbor = true
		ev.snapshot = i.snapshotLocked()
	}
	neighborSink := i.neighborSink
	adjOK := ev.dr
	interfaceName := i.cfg.Name
	dr := i.dr
	bdr := i.bdr
	sink := i.sink
	i.mu.Unlock()
	for _, id := range removedIDs {
		neighborSink.NeighborDown(interfaceName, id)
	}
	if adjOK {
		neighborSink.AdjOK(interfaceName, dr, bdr)
	}
	ev.emit(sink)
	return removed
}

func (i *Interface) buildHelloPacket() []byte {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.buildHelloPacketLocked()
}

// buildHelloPacketLocked encodes one Hello for the interface. The same packet is
// sent to the multicast group (broadcast / point-to-point / PtMP broadcast variant)
// or reused verbatim for each unicast neighbor (NBMA / non-broadcast PtMP), so the
// unicast fan-out allocates one buffer, not one per neighbor.
func (i *Interface) buildHelloPacketLocked() []byte {
	ids := make([]types.RouterID, 0, len(i.neighbors))
	for id := range i.neighbors {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, compareRouterID)
	hello := packet.Hello{
		NetworkMask:   i.cfg.NetworkMask,
		InterfaceID:   i.cfg.InterfaceID,
		HelloInterval: i.cfg.HelloInterval,
		Options:       i.expectedOptionsLocked(),
		Priority:      i.cfg.Priority,
		DeadInterval:  uint32(i.cfg.DeadInterval),
		DR:            i.routerAddressLocked(i.dr),
		BDR:           i.routerAddressLocked(i.bdr),
		Neighbors:     ids,
	}
	return i.encoder.EncodeHello(i.cfg.RouterID, i.cfg.AreaID, hello)
}

func (i *Interface) routerAddressLocked(id types.RouterID) [4]byte {
	if id == (types.RouterID{}) {
		return [4]byte{}
	}
	if id == i.cfg.RouterID {
		return i.cfg.InterfaceAddress
	}
	if n, ok := i.neighbors[id]; ok {
		return addrIdentity(n.Address, id)
	}
	return [4]byte(id)
}

// addrIdentity is the [4]byte identity used for OSPFv2 DR/BDR election and Hello
// encoding: the IPv4 interface address when the reachable address is IPv4
// (OSPFv2), otherwise the Router ID (OSPFv3 elects and declares DR/BDR by Router
// ID, RFC 5340 sec 4.2, and its reachable address is an IPv6 link-local).
func addrIdentity(a netip.Addr, rid types.RouterID) [4]byte {
	if a.Is4() {
		return a.As4()
	}
	return [4]byte(rid)
}

// addr4OrInvalid lifts an OSPFv2 [4]byte interface address to a netip.Addr,
// mapping the zero value to an invalid Addr (so a missing address is not treated
// as 0.0.0.0). OSPFv3 interfaces leave the [4]byte zero and carry their reachable
// address as an IPv6 link-local elsewhere.
func addr4OrInvalid(a [4]byte) netip.Addr {
	if a == ([4]byte{}) {
		return netip.Addr{}
	}
	return netip.AddrFrom4(a)
}

func (i *Interface) Snapshot() Snapshot {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.snapshotLocked()
}

// DetailSnapshot returns the full interface state (ISM, DR/BDR by Router ID, timers,
// OSPFv3 Interface ID / Instance ID).
func (i *Interface) DetailSnapshot() Detail {
	i.mu.Lock()
	defer i.mu.Unlock()
	return Detail{
		Name:               i.cfg.Name,
		Area:               i.cfg.AreaID.String(),
		State:              i.state.String(),
		NetworkType:        i.cfg.NetworkType,
		Cost:               i.cfg.Cost,
		Priority:           i.cfg.Priority,
		Passive:            i.cfg.Passive,
		DR:                 i.dr.String(),
		BDR:                i.bdr.String(),
		HelloInterval:      i.cfg.HelloInterval,
		DeadInterval:       i.cfg.DeadInterval,
		RetransmitInterval: i.cfg.RetransmitInterval,
		InterfaceID:        i.cfg.InterfaceID,
		InstanceID:         i.cfg.InstanceID,
		IsV6:               i.cfg.IsV6,
		NeighborCount:      len(i.neighbors),
	}
}

func (i *Interface) snapshotLocked() Snapshot {
	snap := Snapshot{
		Name:          i.cfg.Name,
		Area:          i.cfg.AreaID.String(),
		State:         i.state.String(),
		NetworkType:   i.cfg.NetworkType,
		Cost:          i.cfg.Cost,
		Priority:      i.cfg.Priority,
		DR:            i.dr.String(),
		BDR:           i.bdr.String(),
		HelloInterval: i.cfg.HelloInterval,
		DeadInterval:  i.cfg.DeadInterval,
		NeighborCount: len(i.neighbors),
		Passive:       i.cfg.Passive,
		BFD:           i.cfg.BFDEnabled,
	}
	if i.cfg.NetworkType == NetworkNBMA {
		snap.PollInterval = i.cfg.PollInterval
		snap.NBMANeighbors = i.nbmaNeighborSnapshotsLocked()
	}
	return snap
}

func (i *Interface) eventsLocked(dr, neighbor bool) interfaceEvents {
	return interfaceEvents{state: true, dr: dr, neighbor: neighbor, snapshot: i.snapshotLocked()}
}

func (i *Interface) helloLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			_ = i.SendHello()
		}
	}
}

func (i *Interface) inactivityLoop(deadInterval time.Duration) {
	for {
		delay := i.nextInactivityDelay(time.Now(), deadInterval)
		timer := time.NewTimer(delay)
		select {
		case <-i.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			i.expireNeighbors(time.Now())
		}
	}
}

func (i *Interface) nextInactivityDelay(now time.Time, maxDelay time.Duration) time.Duration {
	i.mu.Lock()
	defer i.mu.Unlock()
	dead := time.Duration(i.cfg.DeadInterval) * time.Second
	if dead <= 0 || len(i.neighbors) == 0 {
		return maxDelay
	}
	delay := maxDelay
	for _, n := range i.neighbors {
		until := n.LastSeen.Add(dead).Sub(now)
		if until < delay {
			delay = until
		}
	}
	if delay < 0 {
		return 0
	}
	return delay
}

func (i *Interface) waitTimer(delay time.Duration) {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-i.ctx.Done():
	case <-t.C:
		i.runElection()
	}
}

func (i *Interface) SendHello() error {
	return i.sendHelloAt(time.Now())
}

// sendHelloAt sends this interface's Hello(s) as of now. Broadcast, point-to-point,
// and the point-to-multipoint broadcast variant send one packet to the all-routers
// multicast group. NBMA and the non-broadcast point-to-multipoint variant unicast a
// Hello to each configured neighbor (RFC 2328 sec 9.5), at HelloInterval to neighbors
// heard from and at PollInterval to silent ones. now is a parameter so the poll
// cadence is testable.
func (i *Interface) sendHelloAt(now time.Time) error {
	i.mu.Lock()
	if i.sender == nil || i.cfg.Passive || i.cfg.NetworkType == NetworkLoopback {
		i.mu.Unlock()
		return nil
	}
	if i.nonBroadcastLocked() {
		targets := i.helloTargetsLocked(now)
		payload := i.buildHelloPacketLocked()
		sender := i.sender
		name := i.cfg.Name
		i.mu.Unlock()
		return sendUnicast(sender, name, targets, payload)
	}
	dst := allSPFRouters
	if i.cfg.IsV6 {
		dst = allSPFRoutersV6
	}
	payload := i.buildHelloPacketLocked()
	sender := i.sender
	name := i.cfg.Name
	i.mu.Unlock()
	return sender.SendPacket(name, dst, payload)
}

func (i *Interface) runElection() {
	i.mu.Lock()
	ev := i.runElectionLocked()
	neighborSink := i.neighborSink
	interfaceName := i.cfg.Name
	dr := i.dr
	bdr := i.bdr
	sink := i.sink
	// RFC 2328 sec 9.4 step 6: when this NBMA router becomes DR or BDR it must start
	// sending Hellos to its priority-0 (ineligible) neighbors so they begin the
	// adjacency. Capture the send inputs under the lock; send outside it.
	startTargets := i.startHelloTargetsLocked(ev.dr)
	var startPayload []byte
	if len(startTargets) > 0 {
		startPayload = i.buildHelloPacketLocked()
	}
	sender := i.sender
	i.mu.Unlock()
	if len(startTargets) > 0 {
		_ = sendUnicast(sender, interfaceName, startTargets, startPayload)
	}
	if ev.dr {
		neighborSink.AdjOK(interfaceName, dr, bdr)
	}
	ev.emit(sink)
}

func (i *Interface) runElectionLocked() interfaceEvents {
	// RFC 2328 sec 9.4: the DR/BDR election runs on broadcast AND NBMA (the same
	// election over a manually configured neighbor set); point-to-point,
	// point-to-multipoint, loopback, and passive interfaces never elect.
	if i.cfg.Passive || (i.cfg.NetworkType != NetworkBroadcast && i.cfg.NetworkType != NetworkNBMA) {
		return interfaceEvents{}
	}
	candidates := make([]Candidate, 0, len(i.neighbors)+1)
	candidates = append(candidates, Candidate{RouterID: i.cfg.RouterID, Address: addr4OrInvalid(i.cfg.InterfaceAddress), Priority: i.cfg.Priority, TwoWay: true, DeclaredDR: i.routerAddressLocked(i.dr), DeclaredBDR: i.routerAddressLocked(i.bdr), Self: true})
	for _, n := range i.neighbors {
		candidates = append(candidates, Candidate{RouterID: n.RouterID, Address: n.Address, Priority: n.Priority, TwoWay: n.TwoWay, DeclaredDR: n.DeclaredDR, DeclaredBDR: n.DeclaredBDR})
	}
	res := electDRBDR(candidates)
	if res.DR == i.dr && res.BDR == i.bdr {
		if i.state == StateWaiting {
			i.state = StateDROther
			return i.eventsLocked(false, false)
		}
		return interfaceEvents{}
	}
	oldMember := i.dr == i.cfg.RouterID || i.bdr == i.cfg.RouterID
	i.dr = res.DR
	i.bdr = res.BDR
	switch i.cfg.RouterID {
	case i.dr:
		i.state = StateDR
	case i.bdr:
		i.state = StateBackup
	default:
		i.state = StateDROther
	}
	i.metrics.DRElections.With(i.cfg.Name).Inc()
	if i.sender != nil {
		newMember := i.dr == i.cfg.RouterID || i.bdr == i.cfg.RouterID
		if !oldMember && newMember {
			_ = i.sender.JoinAllDRouters(i.cfg.Name)
		}
		if oldMember && !newMember {
			_ = i.sender.LeaveAllDRouters(i.cfg.Name)
		}
	}
	return i.eventsLocked(true, true)
}

func (i *Interface) validateHelloLocked(h packet.Hello) string {
	// The Network Mask match is OSPFv2-only (RFC 2328 sec 10.5) and applies to every
	// multi-access-style link (broadcast, NBMA, point-to-multipoint); it is skipped on
	// point-to-point and loopback. OSPFv3 carries an Interface ID in the Hello instead
	// of a Network Mask, so the v6 path skips it entirely.
	if !i.cfg.IsV6 {
		switch i.cfg.NetworkType {
		case NetworkBroadcast, NetworkNBMA, NetworkPointToMultipoint:
			if h.NetworkMask != i.cfg.NetworkMask {
				return DropReasonNetworkMask
			}
		}
	}
	if h.HelloInterval != i.cfg.HelloInterval {
		return "hello-interval"
	}
	if h.DeadInterval != uint32(i.cfg.DeadInterval) {
		return "dead-interval"
	}
	expected := i.expectedOptionsLocked()
	if h.Options.Has(types.OptionE) != expected.Has(types.OptionE) {
		return DropReasonOptionsE
	}
	// RFC 3101: the N-bit (NSSA capability) must also match or the adjacency does not
	// form. expectedOptionsLocked sets N only for an NSSA interface.
	if h.Options.Has(types.OptionNP) != expected.Has(types.OptionNP) {
		return DropReasonOptionsN
	}
	return ""
}

// DropReasonOptionsE / DropReasonOptionsN are the Hello option-mismatch drop reasons
// (E-bit external-capability and N-bit NSSA-capability).
const (
	DropReasonOptionsE    = "options-e"
	DropReasonOptionsN    = "options-n"
	DropReasonNetworkMask = "network-mask"
)

func (i *Interface) expectedOptionsLocked() types.Options {
	var o types.Options
	switch i.cfg.AreaType {
	case AreaStub:
		return o.Clear(types.OptionE)
	case AreaNSSA:
		return o.Clear(types.OptionE).Set(types.OptionNP)
	default:
		return o.Set(types.OptionE)
	}
}

func (i *Interface) setUpLocked(up bool) {
	v := 0.0
	if up {
		v = 1
	}
	i.metrics.InterfaceUp.With(i.cfg.AreaID.String(), i.cfg.Name).Set(v)
}

func helloHasNeighbor(h packet.Hello, id types.RouterID) bool {
	return slices.Contains(h.Neighbors, id)
}
