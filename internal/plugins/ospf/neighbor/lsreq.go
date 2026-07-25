// Design: plan/learned/960-ospf-6-neighbor-nsm.md -- OSPFv2 LS Request list
// RFC 2328 Section 10.9: "When the Link state request list becomes empty, the neighbor state machine is scheduled with the Loading Done event."

package neighbor

import (
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	lsReqEntryLen    = 12
	lsUpdateFixedLen = 4
	maxLSReqEntries  = (packet.MaxPacketLen - packet.CommonHeaderLen) / lsReqEntryLen
	maxLSUpdateBody  = packet.MaxPacketLen - packet.CommonHeaderLen - lsUpdateFixedLen
	maxRequestList   = 16384
)

func lsReqEntryCapacity(mtu uint16) int {
	limit := ospfPayloadLimit(mtu)
	capacity := (limit - packet.CommonHeaderLen) / lsReqEntryLen
	if capacity < 1 {
		return 1
	}
	if capacity > maxLSReqEntries {
		return maxLSReqEntries
	}
	return capacity
}

func lsUpdateBodyCapacity(mtu uint16) int {
	limit := ospfPayloadLimit(mtu)
	capacity := limit - packet.CommonHeaderLen - lsUpdateFixedLen
	if capacity < types.LSAHeaderLen {
		return types.LSAHeaderLen
	}
	if capacity > maxLSUpdateBody {
		return maxLSUpdateBody
	}
	return capacity
}

func (t *Table) HandleLSReq(interfaceName string, router types.RouterID, req packet.LSReq) string {
	emit, reason := t.handleLSReq(interfaceName, router, req)
	t.emit(emit)
	return reason
}

func (t *Table) handleLSReq(interfaceName string, router types.RouterID, req packet.LSReq) (eventEmission, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cfg, ok := t.cfg[interfaceName]
	if !ok {
		return eventEmission{}, reasonInterface
	}
	n, ok := t.lookupLocked(interfaceName, router)
	if !ok {
		return eventEmission{}, reasonNeighbor
	}
	if n.State < stateExchange {
		return eventEmission{}, reasonState
	}
	if t.lsdb == nil {
		return eventEmission{}, reasonLSDBUnavailable
	}
	lsas := make([]packet.LSA, 0, len(req.Requests))
	for _, r := range req.Requests {
		key := types.LSAKey(r)
		if _, ok := t.lookupLSAHeaderLocked(cfg.Name, cfg.AreaID, key); !ok {
			t.restartExchangeLocked(cfg, n)
			t.sendInitialDDLocked(cfg, n)
			t.recordEventLocked(reasonBadLSReq)
			return t.setStateLocked(n, stateExStart), reasonBadLSReq
		}
		lsa, ok := t.lookupLSALocked(cfg.Name, cfg.AreaID, key)
		if !ok {
			return eventEmission{}, reasonLSDBUnavailable
		}
		lsas = append(lsas, lsa)
	}
	t.sendLSUpdateLocked(cfg, n, lsas)
	return eventEmission{}, ""
}

func (t *Table) HandleLSUpdate(interfaceName string, router types.RouterID, update packet.LSUpdate) string {
	emit, reason := t.handleLSUpdate(interfaceName, router, update)
	t.emit(emit)
	return reason
}

func (t *Table) handleLSUpdate(interfaceName string, router types.RouterID, update packet.LSUpdate) (eventEmission, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.lookupLocked(interfaceName, router)
	if !ok {
		return eventEmission{}, reasonNeighbor
	}
	if n.State < stateExchange {
		return eventEmission{}, reasonState
	}
	if t.lsdb == nil {
		return eventEmission{}, reasonLSDBUnavailable
	}
	for _, lsa := range update.LSAs {
		local, ok := t.lookupLSAHeaderLocked(interfaceName, n.AreaID, lsa.Header.Key())
		if !ok || headerNewer(lsa.Header, local) {
			continue
		}
		removeSatisfiedRequest(n, local)
	}
	if len(n.RequestList) == 0 && n.State == stateLoading {
		t.recordEventLocked("loading-done")
		return t.setStateLocked(n, stateFull), ""
	}
	return eventEmission{}, ""
}

func (t *Table) shouldRequestLocked(cfg InterfaceConfig, h packet.LSAHeader) bool {
	if t.lsdb == nil {
		return false
	}
	local, ok := t.lookupLSAHeaderLocked(cfg.Name, cfg.AreaID, h.Key())
	if !ok {
		return true
	}
	return headerNewer(h, local)
}

func (t *Table) lookupLSAHeaderLocked(interfaceName string, area types.AreaID, key types.LSAKey) (packet.LSAHeader, bool) {
	if key.Type == types.LSTypeLink {
		if linkDB, ok := t.lsdb.(linkScopeLSDB); ok {
			return linkDB.LookupLink(interfaceName, key)
		}
		return packet.LSAHeader{}, false
	}
	return t.lsdb.Lookup(area, key)
}

func (t *Table) lookupLSALocked(interfaceName string, area types.AreaID, key types.LSAKey) (packet.LSA, bool) {
	if key.Type == types.LSTypeLink {
		if linkDB, ok := t.lsdb.(linkScopeLSDB); ok {
			return linkDB.LookupLinkLSA(interfaceName, key)
		}
		return packet.LSA{}, false
	}
	return t.lsdb.LookupLSA(area, key)
}

func addRequest(n *Neighbor, h packet.LSAHeader) bool {
	key := h.Key()
	for _, existing := range n.RequestList {
		if existing.Key() == key {
			return true
		}
	}
	if len(n.RequestList) >= maxRequestList {
		return false
	}
	n.RequestList = append(n.RequestList, h)
	return true
}

func removeSatisfiedRequest(n *Neighbor, h packet.LSAHeader) {
	key := h.Key()
	for idx, existing := range n.RequestList {
		if existing.Key() != key || headerNewer(existing, h) {
			continue
		}
		copy(n.RequestList[idx:], n.RequestList[idx+1:])
		n.RequestList = n.RequestList[:len(n.RequestList)-1]
		return
	}
}

func (t *Table) sendLSReqLocked(cfg InterfaceConfig, n *Neighbor) {
	if len(n.RequestList) == 0 {
		return
	}
	n.lastLSReqs = n.lastLSReqs[:0]
	capacity := lsReqEntryCapacity(cfg.InterfaceMTU)
	for start := 0; start < len(n.RequestList); start += capacity {
		end := min(start+capacity, len(n.RequestList))
		req := packet.LSReq{Requests: make([]packet.LSRequestEntry, 0, end-start)}
		for _, h := range n.RequestList[start:end] {
			req.Requests = append(req.Requests, packet.LSRequestEntry{Type: h.Type, LinkStateID: h.LinkStateID, AdvertisingRouter: h.AdvertisingRouter})
		}
		n.lastLSReqs = append(n.lastLSReqs, req)
		t.sendLSReqPacketLocked(cfg, n, req)
	}
	n.hasLastLSReq = len(n.lastLSReqs) > 0
	t.armLSReqRetransmitLocked(cfg, n)
}

func (t *Table) sendLSReqPacketLocked(cfg InterfaceConfig, n *Neighbor, req packet.LSReq) {
	if t.sender == nil || !n.Address.IsValid() {
		return
	}
	buf := t.encoder.EncodeLSReq(cfg.RouterID, cfg.AreaID, req)
	_ = t.sender.SendPacket(cfg.Name, n.Address, buf)
}

func (t *Table) sendLSUpdateLocked(cfg InterfaceConfig, n *Neighbor, lsas []packet.LSA) {
	if t.sender == nil || !n.Address.IsValid() || len(lsas) == 0 {
		return
	}
	start := 0
	size := 0
	maxBody := lsUpdateBodyCapacity(cfg.InterfaceMTU)
	for i := range lsas {
		lsaLen := lsas[i].EncodedLen()
		if start < i && size+lsaLen > maxBody {
			t.sendLSUpdatePacketLocked(cfg, n, lsas[start:i])
			start = i
			size = 0
		}
		size += lsaLen
	}
	if start < len(lsas) {
		t.sendLSUpdatePacketLocked(cfg, n, lsas[start:])
	}
}

func (t *Table) sendLSUpdatePacketLocked(cfg InterfaceConfig, n *Neighbor, lsas []packet.LSA) {
	buf := t.encoder.EncodeLSUpdate(cfg.RouterID, cfg.AreaID, packet.LSUpdate{LSAs: lsas})
	_ = t.sender.SendPacket(cfg.Name, n.Address, buf)
}

func (t *Table) resendLastLSReqLocked(cfg InterfaceConfig, n *Neighbor) {
	if !n.hasLastLSReq {
		return
	}
	for _, req := range n.lastLSReqs {
		t.sendLSReqPacketLocked(cfg, n, req)
	}
}

// RFC 2328 Section 13.1: sequence number wins first, then checksum, then MaxAge, then age difference beyond MaxAgeDiff.
func headerNewer(a, b packet.LSAHeader) bool {
	if a.Sequence != b.Sequence {
		return a.Sequence.NewerThan(b.Sequence)
	}
	if a.Checksum != b.Checksum {
		return a.Checksum > b.Checksum
	}
	if a.Age.IsMaxAge() != b.Age.IsMaxAge() {
		return a.Age.IsMaxAge()
	}
	aa := a.Age.Age()
	ba := b.Age.Age()
	// Lower age is newer only when the age difference is significant.
	return ba > aa && ba-aa > types.MaxAgeDiff
}
