// Design: docs/research/l2tpv2-ze-integration.md -- DHCPv6-PD codec for BNG PPP sessions
// Related: ra.go -- Router Advertisement sent before DHCPv6
// Related: session_run.go -- afterLCPOpen starts DHCPv6 goroutine

package ppp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
)

// DHCPv6 message types (RFC 3315 Section 5.3).
const (
	DHCPv6Solicit   uint8 = 1
	DHCPv6Advertise uint8 = 2
	DHCPv6Request   uint8 = 3
	DHCPv6Renew     uint8 = 5
	DHCPv6Rebind    uint8 = 6
	DHCPv6Reply     uint8 = 7
	DHCPv6Release   uint8 = 8
	DHCPv6InfoReq   uint8 = 11
)

// DHCPv6 option codes (RFC 3315, RFC 3633).
const (
	D6OptClientID   uint16 = 1
	D6OptServerID   uint16 = 2
	D6OptIAPD       uint16 = 25 // RFC 3633
	D6OptIAPrefix   uint16 = 26 // RFC 3633
	D6OptStatusCode uint16 = 13
	D6OptDNSServers uint16 = 23 // RFC 3646
)

// DHCPv6 status codes (RFC 3315 Section 24.4, RFC 3633 Section 11.1).
const (
	D6StatusSuccess       uint16 = 0
	D6StatusNoPrefixAvail uint16 = 6
)

// DUID types (RFC 3315 Section 9).
const (
	DUIDTypeLLT uint16 = 1
	DUIDTypeEN  uint16 = 2
	DUIDTypeLL  uint16 = 3
)

// DHCPv6DUID represents a DHCP Unique Identifier.
type DHCPv6DUID struct {
	Type          uint16
	HWType        uint16 // for LLT and LL
	Time          uint32 // for LLT only
	EnterpriseNum uint32 // for EN only
	ID            []byte
}

// DHCPv6IAPD represents an IA_PD option (RFC 3633 Section 9).
type DHCPv6IAPD struct {
	IAID   uint32
	T1     uint32
	T2     uint32
	Prefix *DHCPv6IAPrefix
}

// DHCPv6IAPrefix represents an IA_Prefix option inside IA_PD.
type DHCPv6IAPrefix struct {
	PrefLifetime  uint32
	ValidLifetime uint32
	PrefixLen     uint8
	Prefix        netip.Prefix
}

// DHCPv6Message is a parsed DHCPv6 message.
type DHCPv6Message struct {
	Type          uint8
	TransactionID [3]byte
	ClientID      *DHCPv6DUID
	ServerID      *DHCPv6DUID
	IAPD          *DHCPv6IAPD
	StatusCode    *uint16
}

// maxDUIDLen caps the DUID ID field to prevent large allocations from
// attacker-crafted packets. RFC 3315 Section 9.1 limits DUIDs to 128
// octets (2 type + up to 126 payload). PPP MTU (~1500) bounds it
// further, but we enforce explicitly.
const maxDUIDLen = 128

var (
	errDHCPv6TooShort   = errors.New("dhcpv6: message too short")
	errDHCPv6OptOverrun = errors.New("dhcpv6: option length exceeds packet")
	errDHCPv6BadDUID    = errors.New("dhcpv6: DUID too short")
	errDHCPv6DUIDTooBig = errors.New("dhcpv6: DUID exceeds maximum length")
)

// ParseDHCPv6 parses a DHCPv6 message from wire bytes.
func ParseDHCPv6(buf []byte) (*DHCPv6Message, error) {
	if len(buf) < 4 {
		return nil, errDHCPv6TooShort
	}

	msg := &DHCPv6Message{
		Type: buf[0],
	}
	copy(msg.TransactionID[:], buf[1:4])

	off := 4
	for off < len(buf) {
		if len(buf)-off < 4 {
			return nil, errDHCPv6OptOverrun
		}
		optCode := binary.BigEndian.Uint16(buf[off:])
		optLen := int(binary.BigEndian.Uint16(buf[off+2:]))
		off += 4
		if off+optLen > len(buf) {
			return nil, errDHCPv6OptOverrun
		}
		data := buf[off : off+optLen]

		switch optCode {
		case D6OptClientID:
			duid, err := parseDUID(data)
			if err != nil {
				return nil, err
			}
			msg.ClientID = duid
		case D6OptServerID:
			duid, err := parseDUID(data)
			if err != nil {
				return nil, err
			}
			msg.ServerID = duid
		case D6OptIAPD:
			iapd, err := parseIAPD(data)
			if err != nil {
				return nil, err
			}
			msg.IAPD = iapd
		case D6OptStatusCode:
			if len(data) >= 2 {
				code := binary.BigEndian.Uint16(data)
				msg.StatusCode = &code
			}
		}
		off += optLen
	}
	return msg, nil
}

func parseDUID(data []byte) (*DHCPv6DUID, error) {
	if len(data) < 2 {
		return nil, errDHCPv6BadDUID
	}
	if len(data) > maxDUIDLen {
		return nil, errDHCPv6DUIDTooBig
	}
	d := &DHCPv6DUID{
		Type: binary.BigEndian.Uint16(data),
	}
	rest := data[2:]

	switch d.Type {
	case DUIDTypeLLT:
		if len(rest) < 6 {
			return nil, errDHCPv6BadDUID
		}
		d.HWType = binary.BigEndian.Uint16(rest)
		d.Time = binary.BigEndian.Uint32(rest[2:])
		d.ID = make([]byte, len(rest)-6)
		copy(d.ID, rest[6:])
	case DUIDTypeEN:
		if len(rest) < 4 {
			return nil, errDHCPv6BadDUID
		}
		d.EnterpriseNum = binary.BigEndian.Uint32(rest)
		d.ID = make([]byte, len(rest)-4)
		copy(d.ID, rest[4:])
	case DUIDTypeLL:
		if len(rest) < 2 {
			return nil, errDHCPv6BadDUID
		}
		d.HWType = binary.BigEndian.Uint16(rest)
		d.ID = make([]byte, len(rest)-2)
		copy(d.ID, rest[2:])
	default:
		d.ID = make([]byte, len(rest))
		copy(d.ID, rest)
	}
	return d, nil
}

func parseIAPD(data []byte) (*DHCPv6IAPD, error) {
	if len(data) < 12 {
		return nil, errors.New("dhcpv6: IA_PD too short")
	}
	iapd := &DHCPv6IAPD{
		IAID: binary.BigEndian.Uint32(data),
		T1:   binary.BigEndian.Uint32(data[4:]),
		T2:   binary.BigEndian.Uint32(data[8:]),
	}

	// Parse sub-options (IA_Prefix).
	off := 12
	for off < len(data) {
		if len(data)-off < 4 {
			break
		}
		subCode := binary.BigEndian.Uint16(data[off:])
		subLen := int(binary.BigEndian.Uint16(data[off+2:]))
		off += 4
		if off+subLen > len(data) {
			break
		}
		if subCode == D6OptIAPrefix && subLen >= 25 {
			iapd.Prefix = parseIAPrefix(data[off : off+subLen])
		}
		off += subLen
	}
	return iapd, nil
}

func parseIAPrefix(data []byte) *DHCPv6IAPrefix {
	pref := &DHCPv6IAPrefix{
		PrefLifetime:  binary.BigEndian.Uint32(data),
		ValidLifetime: binary.BigEndian.Uint32(data[4:]),
		PrefixLen:     data[8],
	}
	var addr [16]byte
	copy(addr[:], data[9:25])
	pref.Prefix = netip.PrefixFrom(netip.AddrFrom16(addr), int(pref.PrefixLen))
	return pref
}

// DHCPv6ReplyConfig holds parameters for building a DHCPv6 reply
// containing an IA_PD with a prefix.
type DHCPv6ReplyConfig struct {
	Type          uint8
	TransactionID [3]byte
	ServerID      DHCPv6DUID
	ClientID      *DHCPv6DUID
	IAID          uint32
	Prefix        netip.Prefix
	PrefLifetime  uint32
	ValidLifetime uint32
	T1            uint32
	T2            uint32
}

// errDHCPv6BufferTooSmall reports a reply that will not fit the caller's fixed
// buffer. The Build* writers index buf directly (buffer-first contract), so an
// oversized reply would panic mid-encode; the Checked* wrappers return this
// (wrapped, with the required length) instead so the caller drops the reply
// rather than transmit a truncated (malformed) DHCPv6 packet.
var errDHCPv6BufferTooSmall = errors.New("dhcpv6: reply exceeds buffer")

// iapdReplyLen is the fixed byte count of the IA_PD option (with its single
// IA_Prefix sub-option) that BuildDHCPv6Reply emits: option header + IAID/T1/T2
// (16) plus the IA_Prefix sub-option (29).
const iapdReplyLen = 16 + 29

// duidLen reports the bytes writeDUID writes for d, mirroring its type switch.
func duidLen(d *DHCPv6DUID) int {
	n := 2 // Type
	switch d.Type {
	case DUIDTypeLLT:
		n += 2 + 4 // HWType + Time
	case DUIDTypeEN:
		n += 4 // EnterpriseNum
	case DUIDTypeLL:
		n += 2 // HWType
	}
	return n + len(d.ID)
}

// duidOptionLen reports the bytes writeDUIDOption writes: the 4-byte option
// header plus the DUID body.
func duidOptionLen(d *DHCPv6DUID) int { return 4 + duidLen(d) }

// dhcpv6ReplyLen reports the exact byte count BuildDHCPv6Reply writes for cfg.
func dhcpv6ReplyLen(cfg DHCPv6ReplyConfig) int {
	n := 4 // message header
	n += duidOptionLen(&cfg.ServerID)
	if cfg.ClientID != nil {
		n += duidOptionLen(cfg.ClientID)
	}
	n += iapdReplyLen
	return n
}

// dhcpv6StatusReplyLen reports the exact byte count BuildDHCPv6StatusReply writes.
func dhcpv6StatusReplyLen(cfg DHCPv6StatusReplyConfig) int {
	n := 4 // message header
	n += duidOptionLen(&cfg.ServerID)
	if cfg.ClientID != nil {
		n += duidOptionLen(cfg.ClientID)
	}
	n += 4 + 2 + len(cfg.StatusMessage) // status option header + code + message
	return n
}

// CheckedBuildDHCPv6Reply is the capacity-checked form of BuildDHCPv6Reply.
// It writes nothing and returns an error naming the required length when buf is
// too small, so the caller can drop the reply instead of panicking mid-encode.
func CheckedBuildDHCPv6Reply(buf []byte, cfg DHCPv6ReplyConfig) (int, error) {
	need := dhcpv6ReplyLen(cfg)
	if need > len(buf) {
		return 0, fmt.Errorf("%w: needs %d bytes but buffer has %d", errDHCPv6BufferTooSmall, need, len(buf))
	}
	return BuildDHCPv6Reply(buf, cfg), nil
}

// CheckedBuildDHCPv6StatusReply is the capacity-checked form of
// BuildDHCPv6StatusReply (see CheckedBuildDHCPv6Reply).
func CheckedBuildDHCPv6StatusReply(buf []byte, cfg DHCPv6StatusReplyConfig) (int, error) {
	need := dhcpv6StatusReplyLen(cfg)
	if need > len(buf) {
		return 0, fmt.Errorf("%w: needs %d bytes but buffer has %d", errDHCPv6BufferTooSmall, need, len(buf))
	}
	return BuildDHCPv6StatusReply(buf, cfg), nil
}

// BuildDHCPv6Reply writes a DHCPv6 Advertise or Reply message with an
// IA_PD containing the delegated prefix.
// The caller must guarantee buf has capacity (see CheckedBuildDHCPv6Reply).
func BuildDHCPv6Reply(buf []byte, cfg DHCPv6ReplyConfig) int {
	off := 0

	// Message header.
	buf[off] = cfg.Type
	copy(buf[off+1:off+4], cfg.TransactionID[:])
	off += 4

	// Server ID option.
	off += writeDUIDOption(buf[off:], D6OptServerID, &cfg.ServerID)

	// Client ID option.
	if cfg.ClientID != nil {
		off += writeDUIDOption(buf[off:], D6OptClientID, cfg.ClientID)
	}

	// IA_PD option with IA_Prefix sub-option.
	iapdStart := off
	binary.BigEndian.PutUint16(buf[off:], D6OptIAPD)
	off += 4 // skip len (fill later)
	binary.BigEndian.PutUint32(buf[off:], cfg.IAID)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], cfg.T1)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], cfg.T2)
	off += 4

	// IA_Prefix sub-option (RFC 3633 Section 10).
	binary.BigEndian.PutUint16(buf[off:], D6OptIAPrefix)
	binary.BigEndian.PutUint16(buf[off+2:], 25) // pref(4)+valid(4)+len(1)+prefix(16)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], cfg.PrefLifetime)
	off += 4
	binary.BigEndian.PutUint32(buf[off:], cfg.ValidLifetime)
	off += 4
	buf[off] = uint8(cfg.Prefix.Bits())
	off++
	addr := cfg.Prefix.Addr().As16()
	copy(buf[off:off+16], addr[:])
	off += 16

	// Fill IA_PD length.
	iapdLen := off - iapdStart - 4
	binary.BigEndian.PutUint16(buf[iapdStart+2:], uint16(iapdLen))

	return off
}

// DHCPv6StatusReplyConfig holds parameters for a status-only reply
// (e.g., NoPrefixAvail).
type DHCPv6StatusReplyConfig struct {
	TransactionID [3]byte
	ServerID      DHCPv6DUID
	ClientID      *DHCPv6DUID
	StatusCode    uint16
	StatusMessage string
}

// BuildDHCPv6StatusReply writes a DHCPv6 Reply with a Status Code option.
func BuildDHCPv6StatusReply(buf []byte, cfg DHCPv6StatusReplyConfig) int {
	off := 0

	buf[off] = DHCPv6Reply
	copy(buf[off+1:off+4], cfg.TransactionID[:])
	off += 4

	off += writeDUIDOption(buf[off:], D6OptServerID, &cfg.ServerID)
	if cfg.ClientID != nil {
		off += writeDUIDOption(buf[off:], D6OptClientID, cfg.ClientID)
	}

	// Status Code option.
	msgBytes := []byte(cfg.StatusMessage)
	binary.BigEndian.PutUint16(buf[off:], D6OptStatusCode)
	binary.BigEndian.PutUint16(buf[off+2:], uint16(2+len(msgBytes)))
	off += 4
	binary.BigEndian.PutUint16(buf[off:], cfg.StatusCode)
	off += 2
	copy(buf[off:], msgBytes)
	off += len(msgBytes)

	return off
}

func writeDUIDOption(buf []byte, optCode uint16, d *DHCPv6DUID) int {
	binary.BigEndian.PutUint16(buf, optCode)
	duidStart := 4
	n := writeDUID(buf[duidStart:], d)
	binary.BigEndian.PutUint16(buf[2:], uint16(n))
	return duidStart + n
}

func writeDUID(buf []byte, d *DHCPv6DUID) int {
	binary.BigEndian.PutUint16(buf, d.Type)
	off := 2
	switch d.Type {
	case DUIDTypeLLT:
		binary.BigEndian.PutUint16(buf[off:], d.HWType)
		off += 2
		binary.BigEndian.PutUint32(buf[off:], d.Time)
		off += 4
		copy(buf[off:], d.ID)
		off += len(d.ID)
	case DUIDTypeEN:
		binary.BigEndian.PutUint32(buf[off:], d.EnterpriseNum)
		off += 4
		copy(buf[off:], d.ID)
		off += len(d.ID)
	case DUIDTypeLL:
		binary.BigEndian.PutUint16(buf[off:], d.HWType)
		off += 2
		copy(buf[off:], d.ID)
		off += len(d.ID)
	default:
		copy(buf[off:], d.ID)
		off += len(d.ID)
	}
	return off
}
