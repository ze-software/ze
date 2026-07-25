// Design: plan/learned/1044-ospf-ext-9-graceful-restart.md -- OSPFv3 GR preservation + v6 Grace-LSA.
// Related: gr_restarter.go (capture on prepare, restore on resume), origination_v6.go
//
//	(v6OriginateLinkLSA pattern), gr.go (grManager preserved Interface-ID map).
//
// RFC: rfc/short/rfc5187.md sec 3.1 (LSA-ID -> prefix correspondence preservation),
//
//	sec 3.2 (Interface-ID preservation), sec 2.1/2.2 (native LS Type 0x000B Grace-LSA).
package ospf

import (
	"encoding/binary"
	"maps"
	"net/netip"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	packet "github.com/ze-software/ze/internal/plugins/ospf/packet"
	ospftypes "github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// lsidToUint32 is the inverse of v6SummaryLSID: it reads the arbitrary 32-bit LSA ID from a
// LinkStateID's big-endian bytes.
func lsidToUint32(id ospftypes.LinkStateID) uint32 {
	return binary.BigEndian.Uint32(id[:])
}

// captureInterfaceIDs snapshots the OSPFv3 Interface ID of every configured interface for the
// RFC 5187 sec 3.2 preservation map persisted in the restart fact.
func (e *engine) captureInterfaceIDs() map[string]uint32 {
	out := map[string]uint32{}
	topo := e.lsdbTopology()
	for i := range topo {
		if topo[i].Name == "" {
			continue
		}
		out[topo[i].Name] = topo[i].InterfaceID
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// capturePrefixLSIDs snapshots the RFC 5187 sec 3.1 LSA-ID -> prefix correspondence for
// redistributed OSPFv3 External LSAs (the arbitrary 32-bit LSA ID assigned per prefix).
func (e *engine) capturePrefixLSIDs() map[string]uint32 {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.redistV6) == 0 {
		return nil
	}
	out := make(map[string]uint32, len(e.redistV6))
	for pfx, lsid := range e.redistV6 {
		out[pfx.String()] = lsidToUint32(lsid)
	}
	return out
}

// restorePrefixLSIDs rebuilds the redistV6 LSA-ID map on resume so each redistributed prefix
// re-originates under the SAME arbitrary 32-bit LSA ID (RFC 5187 sec 3.1, no network churn).
func (e *engine) restorePrefixLSIDs(m map[string]uint32) {
	if len(m) == 0 {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for s, v := range m {
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			continue
		}
		e.redistV6[pfx] = v6SummaryLSID(v)
		if v >= e.redistV6Next {
			e.redistV6Next = v + 1
		}
	}
}

// restoreInterfaceIDs stores the preserved OSPFv3 Interface IDs so grInterfaceID returns them
// for the grace window, keeping re-originated Link/Network/Router LSAs matching neighbor
// adjacency state (RFC 5187 sec 3.2).
func (e *engine) restoreInterfaceIDs(m map[string]uint32) {
	if e.gr == nil || len(m) == 0 {
		return
	}
	e.gr.mu.Lock()
	defer e.gr.mu.Unlock()
	if e.gr.preservedIfaceIDs == nil {
		e.gr.preservedIfaceIDs = map[string]uint32{}
	}
	maps.Copy(e.gr.preservedIfaceIDs, m)
}

// grInterfaceID returns the OSPFv3 Interface ID for an interface: the preserved value while a
// restart-fact pins it (RFC 5187 sec 3.2), else the live kernel ifindex. lsdbTopology uses it
// so re-originated OSPFv3 LSAs carry the pre-restart Interface IDs.
func (e *engine) grInterfaceID(name string) uint32 {
	if e.gr != nil {
		e.gr.mu.Lock()
		id, ok := e.gr.preservedIfaceIDs[name]
		e.gr.mu.Unlock()
		if ok {
			return id
		}
	}
	return interfaceIndex(name)
}

// v6OriginateGraceLSA originates (or MaxAge-flushes) the OSPFv3 native Grace-LSA (LS Type
// 0x000B, LS ID = Interface ID) on one interface, reusing the v6OriginateLinkLSA link-scope
// origination pattern (RFC 5187 sec 2.1/2.2). Returns false when the interface has no usable
// Interface ID.
func (e *engine) v6OriginateGraceLSA(router ospftypes.RouterID, info *ospflsdb.InterfaceInfo, period uint32, reason uint8, withdraw bool) bool {
	ifaceID := info.InterfaceID
	if ifaceID == 0 {
		return false
	}
	body := ospfv3packet.GraceLSA{GracePeriod: period, Reason: reason}
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	key := ospftypes.LSAKey{
		Type:              ospftypes.LSTypeGraceV6,
		LinkStateID:       v6SummaryLSID(ifaceID),
		AdvertisingRouter: router,
	}
	_, ok := e.lsdb.OriginateLinkSelf(info.Name, info.AreaID, key, bodyBytes, func(seq ospftypes.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header: v6OriginHeader(ospfv3types.LSTypeGrace, ospfv3types.LinkStateID(key.LinkStateID), router, seq, purge || withdraw),
			Grace:  &body,
		})
	})
	return ok
}
