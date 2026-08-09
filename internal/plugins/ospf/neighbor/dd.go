// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- Database Description exchange
// RFC 2328 Section 10.6: "If the Interface MTU field in the Database Description packet is larger than the router's interface MTU, the packet is rejected."

package neighbor

import (
	"slices"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const maxDDHeaders = (packet.MaxPacketLen - packet.CommonHeaderLen - 8) / types.LSAHeaderLen

func ddHeaderCapacity(mtu uint16) int {
	limit := ospfPayloadLimit(mtu)
	capacity := (limit - packet.CommonHeaderLen - 8) / types.LSAHeaderLen
	if capacity < 1 {
		return 1
	}
	if capacity > maxDDHeaders {
		return maxDDHeaders
	}
	return capacity
}

func (t *Table) HandleDBDesc(interfaceName string, router types.RouterID, dd packet.DBDesc) string {
	emit, reason := t.handleDBDesc(interfaceName, router, dd)
	t.emit(emit)
	return reason
}

// RFC 2328 Section 10.8: the router with the larger Router ID becomes the master.
func (t *Table) handleDBDesc(interfaceName string, router types.RouterID, dd packet.DBDesc) (eventEmission, string) {
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
	if cfg.InterfaceMTU != 0 && dd.InterfaceMTU > cfg.InterfaceMTU && !cfg.MTUIgnore {
		t.recordEventLocked("mtu-mismatch")
		return eventEmission{}, "mtu-mismatch"
	}
	if n.State < stateExStart {
		return eventEmission{}, "adjacency-not-ready"
	}
	if n.hasLastDD && sameDD(n.lastDD, dd) {
		if n.Master {
			return eventEmission{}, "duplicate-drop"
		}
		t.resendLastDDLocked(cfg, n)
		return eventEmission{}, "duplicate-resend"
	}
	if n.State == stateExStart {
		return t.handleExStartDDLocked(cfg, n, dd)
	}
	if n.State > stateExchange {
		t.recordEventLocked(reasonSeqNumberMismatch)
		t.restartExchangeLocked(cfg, n)
		t.sendInitialDDLocked(cfg, n)
		return t.setStateLocked(n, stateExStart), reasonSeqNumberMismatch
	}
	return t.handleExchangeDDLocked(cfg, n, dd)
}

func (t *Table) handleExStartDDLocked(cfg InterfaceConfig, n *Neighbor, dd packet.DBDesc) (eventEmission, string) {
	localMaster := compareRouterID(cfg.RouterID, n.RouterID) > 0
	if localMaster {
		if dd.Flags&(packet.DDFlagInit|packet.DDFlagMaster) != 0 || dd.DDSequence != n.DDSequence {
			return eventEmission{}, reasonNegotiation
		}
	} else {
		if dd.Flags&(packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster) != packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster || len(dd.Headers) != 0 || dd.DDSequence == 0 {
			return eventEmission{}, reasonNegotiation
		}
		n.DDSequence = dd.DDSequence
	}
	n.Master = localMaster
	n.Options = dd.Options
	n.lastDD = dd
	n.hasLastDD = true
	for _, h := range dd.Headers {
		if t.shouldRequestLocked(cfg, h) && !addRequest(n, h) {
			t.recordEventLocked(reasonRequestListLimit)
			t.restartExchangeLocked(cfg, n)
			t.sendInitialDDLocked(cfg, n)
			return t.setStateLocked(n, stateExStart), reasonRequestListLimit
		}
	}
	t.recordEventLocked("negotiation-done")
	emit := t.setStateLocked(n, stateExchange)
	if n.Master {
		n.DDSequence++
	}
	t.sendDBDescLocked(cfg, n, 0)
	return emit, ""
}

func (t *Table) handleExchangeDDLocked(cfg InterfaceConfig, n *Neighbor, dd packet.DBDesc) (eventEmission, string) {
	peerMaster := dd.Flags&packet.DDFlagMaster != 0
	if dd.Flags&packet.DDFlagInit != 0 || dd.Options != n.Options || peerMaster == n.Master {
		t.recordEventLocked(reasonSeqNumberMismatch)
		t.restartExchangeLocked(cfg, n)
		t.sendInitialDDLocked(cfg, n)
		return t.setStateLocked(n, stateExStart), reasonSeqNumberMismatch
	}
	switch {
	case n.Master:
		if dd.DDSequence != n.DDSequence {
			t.recordEventLocked(reasonSeqNumberMismatch)
			t.restartExchangeLocked(cfg, n)
			t.sendInitialDDLocked(cfg, n)
			return t.setStateLocked(n, stateExStart), reasonSeqNumberMismatch
		}
	case dd.DDSequence != n.DDSequence+1:
		t.recordEventLocked(reasonSeqNumberMismatch)
		t.restartExchangeLocked(cfg, n)
		t.sendInitialDDLocked(cfg, n)
		return t.setStateLocked(n, stateExStart), reasonSeqNumberMismatch
	default:
		n.DDSequence = dd.DDSequence
	}
	n.lastDD = dd
	n.hasLastDD = true
	for _, h := range dd.Headers {
		if t.shouldRequestLocked(cfg, h) && !addRequest(n, h) {
			t.recordEventLocked(reasonRequestListLimit)
			t.restartExchangeLocked(cfg, n)
			t.sendInitialDDLocked(cfg, n)
			return t.setStateLocked(n, stateExStart), reasonRequestListLimit
		}
	}
	if n.Master && (dd.Flags&packet.DDFlagMore != 0 || n.SummaryIndex < len(n.SummaryList)) {
		n.DDSequence++
		t.sendDBDescLocked(cfg, n, 0)
		return eventEmission{}, ""
	}
	if !n.Master {
		sentFlags := t.sendDBDescLocked(cfg, n, 0)
		if dd.Flags&packet.DDFlagMore != 0 || sentFlags&packet.DDFlagMore != 0 {
			return eventEmission{}, ""
		}
	}
	if dd.Flags&packet.DDFlagMore != 0 {
		return eventEmission{}, ""
	}
	t.recordEventLocked("exchange-done")
	if len(n.RequestList) == 0 {
		return t.setStateLocked(n, stateFull), ""
	}
	emit := t.setStateLocked(n, stateLoading)
	t.sendLSReqLocked(cfg, n)
	return emit, ""
}

func (t *Table) sendInitialDDLocked(cfg InterfaceConfig, n *Neighbor) {
	t.sendDBDescLocked(cfg, n, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)
}

func (t *Table) sendDBDescLocked(cfg InterfaceConfig, n *Neighbor, flags uint8) uint8 {
	headers := t.nextSummaryHeadersLocked(n, flags, cfg.InterfaceMTU)
	if flags&packet.DDFlagInit == 0 && n.SummaryIndex < len(n.SummaryList) {
		flags |= packet.DDFlagMore
	}
	dd := packet.DBDesc{InterfaceMTU: cfg.InterfaceMTU, Options: cfg.Options, Flags: flags, DDSequence: n.DDSequence, Headers: headers}
	n.lastSentDD = dd
	n.hasLastSentDD = true
	if t.sender == nil || !n.Address.IsValid() {
		t.armDDRetransmitLocked(cfg, n)
		return flags
	}
	buf := t.encoder.EncodeDBDesc(cfg.RouterID, cfg.AreaID, dd)
	_ = t.sender.SendPacket(cfg.Name, n.Address, buf)
	t.armDDRetransmitLocked(cfg, n)
	return flags
}

func (t *Table) nextSummaryHeadersLocked(n *Neighbor, flags uint8, mtu uint16) []packet.LSAHeader {
	if flags&packet.DDFlagInit != 0 || n.SummaryIndex >= len(n.SummaryList) {
		return nil
	}
	capacity := ddHeaderCapacity(mtu)
	end := min(n.SummaryIndex+capacity, len(n.SummaryList))
	headers := n.SummaryList[n.SummaryIndex:end]
	n.SummaryIndex = end
	return headers
}

func (t *Table) resendLastDDLocked(cfg InterfaceConfig, n *Neighbor) {
	if !n.hasLastSentDD || t.sender == nil || !n.Address.IsValid() {
		return
	}
	buf := t.encoder.EncodeDBDesc(cfg.RouterID, cfg.AreaID, n.lastSentDD)
	_ = t.sender.SendPacket(cfg.Name, n.Address, buf)
	t.armDDRetransmitLocked(cfg, n)
}

func sameDD(a, b packet.DBDesc) bool {
	return a.InterfaceMTU == b.InterfaceMTU && a.Options == b.Options && a.Flags == b.Flags && a.DDSequence == b.DDSequence && sameHeaders(a.Headers, b.Headers)
}

func sameHeaders(a, b []packet.LSAHeader) bool {
	if len(a) != len(b) {
		return false
	}
	return slices.EqualFunc(a, b, func(x, y packet.LSAHeader) bool { return x == y })
}
