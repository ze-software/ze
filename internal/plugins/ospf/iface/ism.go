// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- OSPFv2 Interface State Machine
// RFC: rfc/short/rfc2328.md
// RFC 2328 Section 9.1: "The state of an OSPF interface is one of Down, Loopback, Waiting, Point-to-point, DR Other, Backup, or DR."
// RFC 2328 Section 9.3: "The interface state machine changes state as interface events occur."

package iface

// State is one RFC 2328 OSPF interface state.
type State uint8

const (
	StateDown State = iota
	StateLoopback
	StateWaiting
	StatePointToPoint
	StateDROther
	StateBackup
	StateDR
)

func (s State) String() string {
	switch s {
	case StateDown:
		return "down"
	case StateLoopback:
		return "loopback"
	case StateWaiting:
		return "waiting"
	case StatePointToPoint:
		return "point-to-point"
	case StateDROther:
		return "dr-other"
	case StateBackup:
		return "backup"
	case StateDR:
		return "dr"
	default:
		return "unknown"
	}
}

const (
	NetworkBroadcast    = "broadcast"
	NetworkPointToPoint = "point-to-point"
	NetworkLoopback     = "loopback"
	// NetworkNBMA is a non-broadcast multi-access link (RFC 2328 App C.5): a DR/BDR is
	// elected over a manually configured neighbor list and Hellos are unicast/polled.
	NetworkNBMA = "nbma"
	// NetworkPointToMultipoint treats a multi-access link as a collection of
	// point-to-point links (RFC 2328 sec 9.5): no DR/BDR, an adjacency with every
	// reachable neighbor, and a host route for the interface address.
	NetworkPointToMultipoint = "point-to-multipoint"
)

const (
	AreaStub = "stub"
	AreaNSSA = "nssa"
)
