// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- OSPFv2 Neighbor State Machine
// RFC: rfc/short/rfc2328.md
// RFC 2328 Section 10.3: "The neighbor state machine is event driven."

package neighbor

import (
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC 2328 Section 10.4: point-to-point and point-to-multipoint neighbors always become
// adjacent; broadcast and NBMA neighbors become adjacent only when either router is DR or
// Backup DR. The predicate is address-family-neutral: it keys on the network type, so both
// the OSPFv2 and OSPFv3 families share it.
func shouldAdj(cfg InterfaceConfig, n *Neighbor) bool {
	switch cfg.NetworkType {
	case NetworkPointToPoint, NetworkPointToMultipoint:
		return true
	case NetworkBroadcast, NetworkNBMA, "":
		return cfg.RouterID == cfg.LocalDR || cfg.RouterID == cfg.LocalBDR || n.RouterID == cfg.LocalDR || n.RouterID == cfg.LocalBDR
	default:
		return false
	}
}

func startExchange(cfg InterfaceConfig, n *Neighbor) {
	n.Master = compareRouterID(cfg.RouterID, n.RouterID) > 0
	n.DDSequence = initialDDSequence(cfg.RouterID)
	n.RequestList = nil
	n.hasLastDD = false
	n.hasLastSentDD = false
	n.SummaryList = nil
	n.SummaryIndex = 0
	n.hasLastLSReq = false
	n.lastLSReqs = nil
	n.ddRetransmitDeadline = time.Time{}
	n.lsReqRetransmitDeadline = time.Time{}
}

func compareRouterID(a, b types.RouterID) int {
	for i := range a {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}

// initialDDSequence seeds the Database Description sequence number from the local Router ID
// (RFC 2328 sec 10.6 wants a value not seen recently; the local Router ID is a stable,
// non-zero seed). The earlier form mixed the local ID's high bytes with the neighbor's low
// bytes -- a copy-paste bug that produced a meaningless half-and-half value.
func initialDDSequence(rid types.RouterID) uint32 {
	v := uint32(rid[0])<<24 | uint32(rid[1])<<16 | uint32(rid[2])<<8 | uint32(rid[3])
	if v == 0 {
		return 1
	}
	return v
}
