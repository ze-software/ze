// Design: docs/architecture/isis/isis-5-adjacency.md -- point-to-point ES-IS discovery.
// Related: runtime.go -- receive dispatch and periodic Hello send.
// RFC 1195 Sections 4.4, 5.3.10; RFC 995 Sections 7.2.2, 8.7 (ISO 9542).
//
// ISO 9542 ISH layout, byte offsets from the beginning of the PDU:
//   0 NLPID=0x82 | 1 length | 2 version | 3 reserved | 4 type=4
//   5..6 holding time | 7..8 checksum | 9 NET length | 10.. NET | TLVs
// The packet codec validates the header, checksum, NET and option boundaries.

package circuit

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// sendISH announces the real configured NET before sending a P2P IIH. The fixed
// maximum ISO 9542 PDU length bounds the stack buffer; no MTU padding is added.
func (c *Circuit) sendISH() error {
	if c.net.Len() < types.MinNETLen {
		return fmt.Errorf("isis: circuit %s has no NET for ISO 9542 discovery", c.name)
	}
	h := packet.ISH{
		NET:         c.net,
		HoldingTime: c.holdTime(c.p2pPreferredLevel()),
		TLVs:        []packet.TLV{c.protocolsSupportedTLV()},
	}
	var buf [255]byte
	n := h.WriteTo(buf[:], 0)
	if n == 0 {
		return fmt.Errorf("isis: circuit %s cannot encode ISO 9542 discovery", c.name)
	}
	pdu := buf[:n]
	c.mu.Lock()
	sign := c.signISH
	c.mu.Unlock()
	if sign != nil {
		var err error
		pdu, err = sign(pdu)
		if err != nil {
			return err
		}
	}
	return c.sender.SendISH(c.name, pdu)
}

// receiveISH records ISO 9542 discovery, never an Up adjacency. An unauthenticated
// ISH cannot refresh or alter an established adjacency: only its verified IIH can.
func (c *Circuit) receiveISH(src adjacency.SNPA, pdu []byte) adjacency.Transition {
	if c.kind != adjacency.KindP2P {
		return adjacency.Transition{Rejected: true, RejectReason: "ish-not-point-to-point"}
	}
	h, err := packet.DecodeISH(pdu)
	if err != nil {
		return adjacency.Transition{Rejected: true, RejectReason: "ish-decode"}
	}
	defer packet.ReleaseTLVs(h.TLVs)
	if h.NET.SystemID() == c.systemID {
		return adjacency.Transition{Rejected: true, RejectReason: "own-system-id"}
	}
	level := adjacency.Level2
	area := h.NET.AreaID()
	if c.formsLevel(adjacency.Level1) {
		for _, local := range c.areas {
			if local.Equal(area) {
				level = adjacency.Level1
				break
			}
		}
	}
	if !c.formsLevel(level) {
		return adjacency.Transition{Rejected: true, RejectReason: "l1-area-mismatch"}
	}
	now := c.now()
	var tr adjacency.Transition
	ok := c.table.Update(h.NET.SystemID(), level, func(adj *adjacency.Adjacency, _ bool) {
		tr.State = adj.State
		if adj.State == adjacency.StateUp {
			return
		}
		if adj.State == adjacency.StateInitializing {
			if !adj.ISHOnly {
				return
			}
		}
		if h.HoldingTime == 0 {
			tr = adjacency.Down(adj, now, c.grace)
			return
		}
		// RFC 5303 Section 2.1: "For example, according to ISO 10589, receipt of an
		// Intermediate System Hello (ISH) will cause an adjacency to go to Initializing
		// state; however, receipt of an ISH will have no effect on the three-way state
		// of an adjacency, which remains firmly Down until it receives an IIH from a
		// neighbor that contains the three-way handshaking option."
		adj.SystemID = h.NET.SystemID()
		adj.SNPA = src
		adj.Level = level
		adj.Areas = []types.AreaID{area}
		adj.Protocols = receivedProtocols(h.TLVs)
		adj.State = adjacency.StateInitializing
		adj.ISHOnly = true
		adj.HoldTime = h.HoldingTime
		adj.HoldExpiry = now.Add(time.Duration(h.HoldingTime) * time.Second)
		adj.LastSeen = now
		tr.State = adj.State
	})
	if !ok {
		return adjacency.Transition{Rejected: true, RejectReason: "table-full"}
	}
	return tr
}

// receivedProtocols converts TLV 129 to a bounded value stored with the adjacency.
func receivedProtocols(tlvs []packet.TLV) adjacency.Protocols {
	var protocols adjacency.Protocols
	present := false
	for _, tlv := range tlvs {
		if tlv.Type != packet.TLVProtocolsSupported {
			continue
		}
		present = true
		for _, nlpid := range tlv.Value {
			switch nlpid {
			case packet.NLPIDIPv4:
				protocols |= adjacency.ProtocolIPv4
			case packet.NLPIDIPv6:
				protocols |= adjacency.ProtocolIPv6
			case packet.NLPIDCLNP:
				protocols |= adjacency.ProtocolCLNP
			}
		}
	}
	// RFC 1195 Section 4.4: "If this field is missing, then it is assumed that
	// the packet was transmitted by an OSI-only router."
	if !present {
		return adjacency.ProtocolCLNP
	}
	return protocols
}
