// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS SRv6 attribute TLVs
// RFC: rfc/short/rfc9514.md — BGP-LS Extensions for SRv6
// Overview: attr.go — TLV interface, registration, and iterator
// Related: attr_link.go — link attribute TLV types (SRv6 End.X SID)
package ls

import (
	"encoding/binary"
	"net/netip"
)

// SRv6 attribute TLV type codes.
// RFC 9514 Section 8 defines SRv6 attribute TLVs.
const (
	TLVSRv6EndpointBehavior uint16 = 1250 // RFC 9514 Section 8
	TLVSRv6BGPPeerNodeSID   uint16 = 1251 // RFC 9514 Section 5.1
	TLVSRv6SIDStructure     uint16 = 1252 // RFC 9514 Section 8
)

// --- TLV 1250: SRv6 Endpoint Behavior ---

// LsSRv6EndpointBehavior represents SRv6 Endpoint Behavior (TLV 1250).
// RFC 9514 Section 8: Behavior(2) + Flags(1) + Algorithm(1).
type LsSRv6EndpointBehavior struct {
	EndpointBehavior uint16
	Flags            uint8
	Algorithm        uint8
}

func (t *LsSRv6EndpointBehavior) Code() uint16 { return TLVSRv6EndpointBehavior }
func (t *LsSRv6EndpointBehavior) Len() int     { return 4 + 4 }

func (t *LsSRv6EndpointBehavior) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRv6EndpointBehavior, 4)
	vOff := off + 4
	binary.BigEndian.PutUint16(buf[vOff:], t.EndpointBehavior)
	buf[vOff+2] = t.Flags
	buf[vOff+3] = t.Algorithm
	return n
}

func (t *LsSRv6EndpointBehavior) ToJSON() map[string]any {
	return map[string]any{
		"srv6-endpoint-behavior": map[string]any{
			"behavior":  int(t.EndpointBehavior),
			"flags":     int(t.Flags),
			"algorithm": int(t.Algorithm),
		},
	}
}

func decodeSRv6EndpointBehavior(data []byte) (LsAttrTLV, error) {
	if len(data) < 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsSRv6EndpointBehavior{
		EndpointBehavior: binary.BigEndian.Uint16(data[0:2]),
		Flags:            data[2],
		Algorithm:        data[3],
	}, nil
}

// --- TLV 1251: SRv6 BGP Peer Node SID ---

// LsSRv6BGPPeerNodeSID represents SRv6 BGP Peer Node SID (TLV 1251).
// RFC 9514 Section 5.1: Flags(1) + Weight(1) + Reserved(2) + PeerAS(4) + PeerBGPID(4).
// Total value length: 12 bytes. The SRv6 SID itself comes from the SRv6 SID NLRI descriptor (TLV 518).
type LsSRv6BGPPeerNodeSID struct {
	Flags     uint8
	Weight    uint8
	PeerAS    uint32
	PeerBGPID netip.Addr
}

func (t *LsSRv6BGPPeerNodeSID) Code() uint16 { return TLVSRv6BGPPeerNodeSID }
func (t *LsSRv6BGPPeerNodeSID) Len() int     { return 4 + 12 } // header(4) + flags(1)+weight(1)+reserved(2)+AS(4)+BGPID(4)

func (t *LsSRv6BGPPeerNodeSID) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRv6BGPPeerNodeSID, 12)
	vOff := off + 4
	buf[vOff] = t.Flags
	buf[vOff+1] = t.Weight
	buf[vOff+2] = 0 // reserved
	buf[vOff+3] = 0 // reserved
	binary.BigEndian.PutUint32(buf[vOff+4:], t.PeerAS)
	b := t.PeerBGPID.As4()
	copy(buf[vOff+8:], b[:])
	return n
}

func (t *LsSRv6BGPPeerNodeSID) ToJSON() map[string]any {
	return map[string]any{
		"srv6-bgp-peer-node-sid": map[string]any{
			"flags":       int(t.Flags),
			"weight":      int(t.Weight),
			"peer-as":     t.PeerAS,
			"peer-bgp-id": t.PeerBGPID.String(),
		},
	}
}

func decodeSRv6BGPPeerNodeSID(data []byte) (LsAttrTLV, error) {
	// RFC 9514 Section 5.1: exactly 12 bytes
	if len(data) < 12 {
		return nil, ErrBGPLSTruncated
	}
	return &LsSRv6BGPPeerNodeSID{
		Flags:     data[0],
		Weight:    data[1],
		PeerAS:    binary.BigEndian.Uint32(data[4:8]),
		PeerBGPID: netip.AddrFrom4([4]byte(data[8:12])),
	}, nil
}

// --- TLV 1252: SRv6 SID Structure ---

// LsSRv6SIDStructureAttr represents SRv6 SID Structure as a standalone attribute TLV (1252).
// RFC 9514 Section 8: LocBlockLen(1) + LocNodeLen(1) + FuncLen(1) + ArgLen(1).
// Note: This also appears as a sub-TLV inside SRv6 End.X SID (TLV 1106).
type LsSRv6SIDStructureAttr struct {
	LocBlockLen uint8
	LocNodeLen  uint8
	FuncLen     uint8
	ArgLen      uint8
}

func (t *LsSRv6SIDStructureAttr) Code() uint16 { return TLVSRv6SIDStructure }
func (t *LsSRv6SIDStructureAttr) Len() int     { return 4 + 4 }

func (t *LsSRv6SIDStructureAttr) WriteTo(buf []byte, off int) int {
	n := writeTLV(buf, off, TLVSRv6SIDStructure, 4)
	vOff := off + 4
	buf[vOff] = t.LocBlockLen
	buf[vOff+1] = t.LocNodeLen
	buf[vOff+2] = t.FuncLen
	buf[vOff+3] = t.ArgLen
	return n
}

func (t *LsSRv6SIDStructureAttr) ToJSON() map[string]any {
	return map[string]any{
		"srv6-sid-structure": map[string]any{
			"loc-block-len": int(t.LocBlockLen),
			"loc-node-len":  int(t.LocNodeLen),
			"func-len":      int(t.FuncLen),
			"arg-len":       int(t.ArgLen),
		},
	}
}

func decodeSRv6SIDStructure(data []byte) (LsAttrTLV, error) {
	if len(data) != 4 {
		return nil, ErrBGPLSTruncated
	}
	return &LsSRv6SIDStructureAttr{
		LocBlockLen: data[0],
		LocNodeLen:  data[1],
		FuncLen:     data[2],
		ArgLen:      data[3],
	}, nil
}
