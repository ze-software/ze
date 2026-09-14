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

// The network types and the area types this package reads are declared in
// internal/plugins/ospf/types (vocabulary.go), because the config resolver, iface,
// neighbor, lsdb and spf all act on the same words.
