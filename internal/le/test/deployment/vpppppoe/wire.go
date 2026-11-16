// Design: docs/labs/pppoe-interop.md -- bounded scratch-peer PPP control packets.
package testvpppppoe

import (
	"encoding/binary"
	"errors"
	"time"
)

const (
	controlLifetime = 90 * time.Second
	// A local VPP CLI round trip shares the mechanism readiness budget.
	// Cleanup keeps this bound even after detaching from a canceled run.
	controlCommandBudget = 30 * time.Second
)

type discoveryFrame struct {
	source  [6]byte
	code    byte
	session uint16
	tags    []byte
	vlan    []byte
}

// parseDiscovery validates the Ethernet envelope and every tag before a
// scratch response can echo them. The Ethernet/PPPoE offsets follow RFC2516
// Section 4; an optional 802.1Q header moves the PPPoE header from 14 to 18.
func parseDiscovery(frame []byte) (discoveryFrame, error) {
	var result discoveryFrame
	if len(frame) < 20 {
		return result, errors.New("short discovery Ethernet frame")
	}
	copy(result.source[:], frame[6:12])
	offset := 14
	typeCode := binary.BigEndian.Uint16(frame[12:14])
	if typeCode == 0x8100 {
		if len(frame) < 24 {
			return result, errors.New("short tagged discovery Ethernet frame")
		}
		result.vlan = append([]byte(nil), frame[12:16]...)
		typeCode = binary.BigEndian.Uint16(frame[16:18])
		offset = 18
	}
	if typeCode != 0x8863 {
		return result, errors.New("not PPPoE discovery")
	}
	if frame[offset] != 0x11 {
		return result, errors.New("unexpected PPPoE version/type")
	}
	length := int(binary.BigEndian.Uint16(frame[offset+4 : offset+6]))
	if length > len(frame)-offset-6 {
		return result, errors.New("truncated PPPoE discovery payload")
	}
	result.code = frame[offset+1]
	result.session = binary.BigEndian.Uint16(frame[offset+2 : offset+4])
	result.tags = append([]byte(nil), frame[offset+6:offset+6+length]...)
	for index := 0; index < len(result.tags); {
		if len(result.tags)-index < 4 {
			return result, errors.New("short discovery tag header")
		}
		length := int(binary.BigEndian.Uint16(result.tags[index+2 : index+4]))
		if length > len(result.tags)-index-4 {
			return result, errors.New("short discovery tag value")
		}
		index += 4 + length
	}
	return result, nil
}

// controlAck is a scratch-peer encoder, not the shipped PPP state machine.
// PPP protocol occupies bytes 0..1, followed by Code, Identifier, Length and
// options at bytes 2,3,4..5,6..n (RFC 1661 Section 5.2).
// RFC 1661 Section 5.2: "The acknowledged Configuration Options MUST NOT be reordered or modified in any way."
func controlAck(request []byte) ([]byte, error) {
	if len(request) < 6 {
		return nil, errors.New("PPP control packet lacks its header")
	}
	if request[2] != 1 {
		return nil, errors.New("PPP control packet is not a Configure-Request")
	}
	if int(binary.BigEndian.Uint16(request[4:6])) != len(request)-2 {
		return nil, errors.New("PPP control packet length differs from received bytes")
	}
	for offset := 6; offset < len(request); {
		if len(request)-offset < 2 {
			return nil, errors.New("PPP option lacks its header")
		}
		length := int(request[offset+1])
		if length < 2 {
			return nil, errors.New("PPP option length is below its header")
		}
		if length > len(request)-offset {
			return nil, errors.New("PPP option is truncated")
		}
		offset += length
	}
	// The reader reuses its receive buffer; the independent response must keep
	// the exact request bytes until the socket has accepted the write.
	ack := append([]byte(nil), request...)
	ack[2] = 2
	return ack, nil
}

type controlResult struct {
	Session            uint16 `json:"session"`
	Interface          string `json:"interface"`
	SocketNamespace    string `json:"socket-namespace"`
	SessionBeforePADS  bool   `json:"session-before-pads"`
	LCPRead            bool   `json:"lcp-read"`
	LCPWrite           bool   `json:"lcp-write"`
	IPCPOpen           bool   `json:"ipcp-open"`
	RouterSolicitation bool   `json:"router-solicitation"`
	DHCPv6             bool   `json:"dhcpv6"`
	BoundsBarrier      bool   `json:"bounds-barrier"`
	NamespaceRestored  bool   `json:"namespace-restored"`
}
