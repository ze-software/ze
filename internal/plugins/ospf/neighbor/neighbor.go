// Design: plan/spec-ospf-6-neighbor-nsm.md -- OSPFv2 neighbor record
// RFC 2328 Section 10.1: "The state of a neighbor can be: Down, Attempt, Init, 2-Way, ExStart, Exchange, Loading or Full."

package neighbor

import (
	"net/netip"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
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
	NetworkBroadcast    = "broadcast"
	NetworkPointToPoint = "point-to-point"
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

// v4Encoder is the OSPFv2 neighbor encoder (the default), behavior-identical to the
// prior direct ospf/packet encode.
type v4Encoder struct{}

func (v4Encoder) EncodeDBDesc(routerID types.RouterID, areaID types.AreaID, dd packet.DBDesc) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID}, DBDesc: &dd})
}

func (v4Encoder) EncodeLSReq(routerID types.RouterID, areaID types.AreaID, r packet.LSReq) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID}, LSReq: &r})
}

func (v4Encoder) EncodeLSUpdate(routerID types.RouterID, areaID types.AreaID, u packet.LSUpdate) []byte {
	return encodeV4(packet.Packet{Header: packet.Header{RouterID: routerID, AreaID: areaID}, LSUpdate: &u})
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
	InterfaceName           string
	AreaID                  types.AreaID
	RouterID                types.RouterID
	Address                 netip.Addr
	Priority                uint8
	InterfaceID             uint32
	DeclaredDR              [4]byte
	DeclaredBDR             [4]byte
	State                   state
	LastSeen                time.Time
	InactivityDeadline      time.Time
	Master                  bool
	DDSequence              uint32
	Options                 types.Options
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
	Interface    string `json:"interface"`
	Area         string `json:"area"`
	RouterID     string `json:"router_id"`
	State        string `json:"state"`
	Address      string `json:"address,omitempty"`
	Priority     uint8  `json:"priority"`
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
}

func snapshotOf(n *Neighbor, now time.Time) Snapshot {
	dead := int64(0)
	if n.State != stateDown && !n.InactivityDeadline.IsZero() && now.Before(n.InactivityDeadline) {
		dead = int64(n.InactivityDeadline.Sub(now).Seconds())
	}
	return Snapshot{
		Interface:    n.InterfaceName,
		Area:         n.AreaID.String(),
		RouterID:     n.RouterID.String(),
		State:        n.State.String(),
		Address:      naddrString(n.Address),
		Priority:     n.Priority,
		DR:           addrString(n.DeclaredDR),
		BDR:          addrString(n.DeclaredBDR),
		DeadTime:     dead,
		Master:       n.Master,
		DDSequence:   n.DDSequence,
		RequestCount: len(n.RequestList),
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
