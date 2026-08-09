// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- OSPFv2 neighbor record
// RFC 2328 Section 10.1: "The state of a neighbor can be: Down, Attempt, Init, 2-Way, ExStart, Exchange, Loading or Full."

package neighbor

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	maxNeighbors   = 1024
	ipv4HeaderLen  = 20
	minOSPFPayload = packet.CommonHeaderLen
)

func ospfPayloadLimit(mtu uint16) int {
	limit := packet.MaxPacketLen
	if mtu == 0 {
		return limit
	}
	payload := int(mtu) - ipv4HeaderLen
	if payload < minOSPFPayload {
		return minOSPFPayload
	}
	if payload < limit {
		return payload
	}
	return limit
}

const (
	reasonInterface         = "interface"
	reasonNeighbor          = "neighbor"
	reasonBadLSReq          = "bad-lsreq"
	reasonState             = "state"
	reasonLSDBUnavailable   = "lsdb-unavailable"
	reasonSeqNumberMismatch = "seq-number-mismatch"
	reasonNegotiation       = "negotiation"
	reasonRequestListLimit  = "request-list-limit"

	stateNameDown    = "down"
	stateNameInit    = "init"
	stateNameTwoWay  = "2-way"
	stateNameExStart = "exstart"
	stateNameLoading = "loading"
	stateNameFull    = "full"
)

type state uint8

const (
	stateDown state = iota
	stateAttempt
	stateInit
	stateTwoWay
	stateExStart
	stateExchange
	stateLoading
	stateFull
)

func (s state) String() string {
	switch s {
	case stateDown:
		return stateNameDown
	case stateAttempt:
		return "attempt"
	case stateInit:
		return stateNameInit
	case stateTwoWay:
		return stateNameTwoWay
	case stateExStart:
		return stateNameExStart
	case stateExchange:
		return "exchange"
	case stateLoading:
		return stateNameLoading
	case stateFull:
		return stateNameFull
	default:
		return "unknown"
	}
}

const (
	NetworkBroadcast         = "broadcast"
	NetworkPointToPoint      = "point-to-point"
	NetworkNBMA              = "nbma"
	NetworkPointToMultipoint = "point-to-multipoint"
)

type Sender interface {
	SendPacket(name string, dst netip.Addr, payload []byte) error
}

// Encoder serializes the neighbor FSM's outgoing packets for the address family.
// The default encodes OSPFv2 (ospf/packet); the engine injects an OSPFv3 encoder
// for a v6 interface. The table builds the AF-neutral bodies (packet.DBDesc /
// packet.LSReq / packet.LSUpdate) and the encoder serializes the version it
// implements (the v6 encoder also stamps the OSPFv3 Instance ID into the header).
type Encoder interface {
	EncodeDBDesc(routerID types.RouterID, areaID types.AreaID, dd packet.DBDesc) []byte
	EncodeLSReq(routerID types.RouterID, areaID types.AreaID, r packet.LSReq) []byte
	EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte
}

// v4Encoder is the OSPFv2 neighbor encoder (the default). instanceID is the engine's RFC
// 6549 OSPFv2 Instance ID, stamped into the DD/LSReq/LSUpdate common header (offset 14) so
// the neighbor FSM's outgoing packets carry the engine's Instance ID; 0 is the base
// instance and its bytes are identical to base OSPFv2.
type v4Encoder struct {
	instanceID uint8
}

// NewV4Encoder returns the OSPFv2 neighbor encoder for the given Instance ID (RFC 6549).
// The engine installs it via SetEncoder for a non-base instance; the base instance uses
// the zero-value default.
func NewV4Encoder(instanceID uint8) Encoder { return v4Encoder{instanceID: instanceID} }

func (e v4Encoder) EncodeDBDesc(routerID types.RouterID, areaID types.AreaID, dd packet.DBDesc) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, DBDesc: &dd})
}

func (e v4Encoder) EncodeLSReq(routerID types.RouterID, areaID types.AreaID, r packet.LSReq) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, LSReq: &r})
}

func (e v4Encoder) EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID, InstanceID: e.instanceID}, LSUpdate: &u})
}

func encodeV4(p packet.Packet) []byte {
	buf := make([]byte, p.EncodedLen())
	p.WriteTo(buf, 0)
	return buf
}

type InterfaceConfig struct {
	Name               string
	AreaID             types.AreaID
	RouterID           types.RouterID
	NetworkType        string
	InterfaceAddress   [4]byte
	Options            types.Options
	InterfaceMTU       uint16
	MTUIgnore          bool
	DeadInterval       uint16
	RetransmitInterval uint16
	LocalDR            types.RouterID
	LocalBDR           types.RouterID
	// BFD (RFC 5880 / RFC 5881) per-interface single-hop failure detection. AF-neutral:
	// the engine opens a session for a neighbor on this interface when BFDEnabled. Timers
	// are microseconds; multiplier is Detect Mult. Carried here so the neighbor config is
	// self-describing; the engine also reads it from its running interfaceConfig.
	BFDEnabled    bool
	BFDMinTxUs    uint32
	BFDMinRxUs    uint32
	BFDMultiplier uint8
}

type HelloInput struct {
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
	Now         time.Time
}

type Metrics struct {
	Neighbors       metrics.GaugeVec
	AdjacenciesFull metrics.GaugeVec
	NSMEvents       metrics.CounterVec
}

func NopMetrics() Metrics {
	nop := metrics.NopRegistry{}
	return Metrics{
		Neighbors:       nop.GaugeVec("", "", nil),
		AdjacenciesFull: nop.GaugeVec("", "", nil),
		NSMEvents:       nop.CounterVec("", "", nil),
	}
}

type EventSink interface {
	NeighborUp(Snapshot)
	NeighborDown(Snapshot)
}

type Neighbor struct {
	InterfaceName      string
	AreaID             types.AreaID
	RouterID           types.RouterID
	Address            netip.Addr
	Priority           uint8
	InterfaceID        uint32
	DeclaredDR         [4]byte
	DeclaredBDR        [4]byte
	State              state
	LastSeen           time.Time
	InactivityDeadline time.Time
	Master             bool
	DDSequence         uint32
	Options            types.Options
	// LastEvent is the most recent NSM event name applied to this neighbor (for the ext-14
	// `show ospf neighbor detail` last-event field).
	LastEvent               string
	RequestList             []packet.LSAHeader
	SummaryList             []packet.LSAHeader
	SummaryIndex            int
	lastDD                  packet.DBDesc
	hasLastDD               bool
	lastSentDD              packet.DBDesc
	hasLastSentDD           bool
	lastLSReqs              []packet.LSReq
	hasLastLSReq            bool
	ddRetransmitDeadline    time.Time
	lsReqRetransmitDeadline time.Time
}

func newNeighbor(cfg InterfaceConfig, id types.RouterID) *Neighbor {
	return &Neighbor{InterfaceName: cfg.Name, AreaID: cfg.AreaID, RouterID: id, State: stateDown}
}

type Snapshot struct {
	Interface string `json:"interface"`
	// OpaqueCapable reports whether the neighbor advertised the RFC 5250 O-bit in its DD.
	OpaqueCapable bool   `json:"opaque-capable"`
	Area          string `json:"area"`
	RouterID      string `json:"router_id"`
	State         string `json:"state"`
	Address       string `json:"address,omitempty"`
	Priority      uint8  `json:"priority"`
	// BFD carries the RFC 5880 session state for a BFD-protected neighbor (up / down /
	// init / admin-down), or "" when BFD is not active. The neighbor table does not own the
	// session; the engine annotates this from its BFD client map for `show ospf neighbor`.
	BFD          string `json:"bfd,omitempty"`
	DR           string `json:"dr"`
	BDR          string `json:"bdr"`
	DeadTime     int64  `json:"dead_time"`
	Master       bool   `json:"master"`
	DDSequence   uint32 `json:"dd_sequence"`
	RequestCount int    `json:"request_count"`
}

// FloodNeighbor is a value snapshot of a neighbor that participates in flooding.
type FloodNeighbor struct {
	RouterID types.RouterID
	Address  netip.Addr
	State    string
	// InterfaceID is the neighbor's advertised OSPFv3 Interface ID (zero for OSPFv2),
	// echoed as the Neighbor Interface ID in this router's Router-LSA link.
	InterfaceID uint32
	// OpaqueCapable is true when the neighbor set the O-bit in its Database Description
	// packets (RFC 5250 §3.1); flooding queues opaque LSAs only to such neighbors.
	OpaqueCapable bool
}

func snapshotOf(n *Neighbor, now time.Time) Snapshot {
	dead := int64(0)
	if n.State != stateDown && !n.InactivityDeadline.IsZero() && now.Before(n.InactivityDeadline) {
		dead = int64(n.InactivityDeadline.Sub(now).Seconds())
	}
	return Snapshot{
		Interface:     n.InterfaceName,
		Area:          n.AreaID.String(),
		RouterID:      n.RouterID.String(),
		State:         n.State.String(),
		Address:       naddrString(n.Address),
		Priority:      n.Priority,
		DR:            addrString(n.DeclaredDR),
		BDR:           addrString(n.DeclaredBDR),
		DeadTime:      dead,
		Master:        n.Master,
		DDSequence:    n.DDSequence,
		RequestCount:  len(n.RequestList),
		OpaqueCapable: n.Options.Has(types.OptionO),
	}
}

// Detail is the full per-neighbor state for `show ospf neighbor detail` (spec-ospf-ext-14).
// It exposes the internal fields the summary Snapshot omits. The address-family-specific
// Options-bit decoding (O-bit for OSPFv2; R/V6/E/N/AF for OSPFv3) is done by the engine,
// which knows the AF; this raw value carries the bits.
type Detail struct {
	Interface      string `json:"interface"`
	Area           string `json:"area"`
	RouterID       string `json:"router-id"`
	Address        string `json:"address,omitempty"`
	State          string `json:"state"`
	Priority       uint8  `json:"priority"`
	Master         bool   `json:"master"`
	DDSequence     uint32 `json:"dd-sequence"`
	Options        uint32 `json:"options"`
	InterfaceID    uint32 `json:"interface-id,omitempty"`
	RequestListLen int    `json:"request-list"`
	SummaryListLen int    `json:"summary-list"`
	DeadTime       int64  `json:"dead-time"`
	LastEvent      string `json:"last-event,omitempty"`
	DR             string `json:"dr"`
	BDR            string `json:"bdr"`
}

func detailOf(n *Neighbor, now time.Time) Detail {
	dead := int64(0)
	if n.State != stateDown && !n.InactivityDeadline.IsZero() && now.Before(n.InactivityDeadline) {
		dead = int64(n.InactivityDeadline.Sub(now).Seconds())
	}
	return Detail{
		Interface:      n.InterfaceName,
		Area:           n.AreaID.String(),
		RouterID:       n.RouterID.String(),
		Address:        naddrString(n.Address),
		State:          n.State.String(),
		Priority:       n.Priority,
		Master:         n.Master,
		DDSequence:     n.DDSequence,
		Options:        uint32(n.Options),
		InterfaceID:    n.InterfaceID,
		RequestListLen: len(n.RequestList),
		SummaryListLen: len(n.SummaryList),
		DeadTime:       dead,
		LastEvent:      n.LastEvent,
		DR:             addrString(n.DeclaredDR),
		BDR:            addrString(n.DeclaredBDR),
	}
}

func addrString(a [4]byte) string {
	if a == ([4]byte{}) {
		return ""
	}
	return netip.AddrFrom4(a).String()
}

// naddrString renders a reachable neighbor address (IPv4 or IPv6 link-local),
// returning "" for the invalid zero value.
func naddrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
