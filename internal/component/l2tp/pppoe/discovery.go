// Design: docs/architecture/l2tp/bng-5-pppoe.md -- PPPoE discovery wire format
// Related: server.go -- InterfaceServer uses ParseDiscovery + Build* functions

package pppoe

import (
	"encoding/binary"
	"errors"
	"net"
	"slices"
)

// Ethernet constants.
const (
	EthHdrLen  = 14     // dst(6) + src(6) + ethertype(2)
	EthALen    = 6      // MAC address length
	EthMaxData = 1500   // standard Ethernet MTU
	EthMaxLen  = 1518   // max Ethernet frame (no VLAN tag)
	EthPPPDisc = 0x8863 // ethertype: PPPoE discovery
	EthPPPSes  = 0x8864 // ethertype: PPPoE session
)

// PPPoE header constants. RFC 2516 Section 4.
const (
	PPPoEHdrLen     = 6                        // ver/type(1) + code(1) + session_id(2) + length(2)
	PPPoEVerType    = 0x11                     // version=1, type=1 packed in one byte
	PPPoEMaxPayload = EthMaxData - PPPoEHdrLen // 1494
	PPPoEMaxMTU     = PPPoEMaxPayload - 2      // 1492 (subtract PPP protocol field)
)

// MinDiscFrame is the smallest valid discovery frame: Ethernet header
// + PPPoE header with zero-length payload.
const MinDiscFrame = EthHdrLen + PPPoEHdrLen

// PPPoE discovery codes. RFC 2516 Section 5.
const (
	CodePADI byte = 0x09
	CodePADO byte = 0x07
	CodePADR byte = 0x19
	CodePADS byte = 0x65
	CodePADT byte = 0xA7
)

// PPPoE tag types. RFC 2516 Section 9.
const (
	TagEndOfList      uint16 = 0x0000
	TagServiceName    uint16 = 0x0101
	TagACName         uint16 = 0x0102
	TagHostUniq       uint16 = 0x0103
	TagACCookie       uint16 = 0x0104
	TagVendorSpecific uint16 = 0x0105
	TagRelaySessionID uint16 = 0x0110
	TagPPPMaxPayload  uint16 = 0x0120
	TagSvcNameError   uint16 = 0x0201
	TagACSystemError  uint16 = 0x0202
	TagGenericError   uint16 = 0x0203
)

// TagHdrLen is the size of a tag header (type + length).
const TagHdrLen = 4

// BroadcastMAC is the Ethernet broadcast address used as PADI destination.
var BroadcastMAC = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

// Errors returned by parse functions.
var (
	ErrFrameTooShort    = errors.New("pppoe: frame too short")
	ErrBadVersion       = errors.New("pppoe: unsupported version/type (expected 0x11)")
	ErrInvalidTagLength = errors.New("pppoe: tag length extends past payload")
	ErrUnexpectedCode   = errors.New("pppoe: unexpected discovery code")
	ErrSIDNotZero       = errors.New("pppoe: session ID must be zero in discovery")
	ErrBadEthertype     = errors.New("pppoe: not a PPPoE discovery frame (ethertype != 0x8863)")
	ErrBroadcastSource  = errors.New("pppoe: source address is broadcast")
	ErrMulticastSource  = errors.New("pppoe: source address is multicast")
)

// Tag is a zero-copy view into a parsed PPPoE tag. Value is a
// sub-slice of the original frame buffer.
type Tag struct {
	Type  uint16
	Value []byte
}

// Packet holds a parsed PPPoE discovery frame. Tag values are
// sub-slices of the original frame buffer (zero-copy). The Packet
// does not own the underlying memory.
type Packet struct {
	SrcMAC [EthALen]byte
	DstMAC [EthALen]byte
	Code   byte
	SID    uint16
	Tags   []Tag
}

// ParseDiscovery parses a raw PPPoE discovery frame (Ethernet header
// included, as received from SOCK_RAW). Returns a Packet with tags
// pointing into the original buffer. The caller must not modify buf
// while the Packet is in use.
//
// Validates: minimum length, ver/type=0x11, ethertype=0x8863,
// source is not broadcast/multicast, tag lengths within payload.
func ParseDiscovery(buf []byte) (Packet, error) {
	if len(buf) < MinDiscFrame {
		return Packet{}, ErrFrameTooShort
	}

	var pkt Packet
	copy(pkt.DstMAC[:], buf[0:EthALen])
	copy(pkt.SrcMAC[:], buf[EthALen:2*EthALen])

	if pkt.SrcMAC == [EthALen]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff} {
		return Packet{}, ErrBroadcastSource
	}
	if pkt.SrcMAC[0]&0x01 != 0 {
		return Packet{}, ErrMulticastSource
	}

	ethertype := binary.BigEndian.Uint16(buf[2*EthALen : 2*EthALen+2])
	if ethertype != EthPPPDisc {
		return Packet{}, ErrBadEthertype
	}

	verType := buf[EthHdrLen]
	if verType != PPPoEVerType {
		return Packet{}, ErrBadVersion
	}

	pkt.Code = buf[EthHdrLen+1]
	pkt.SID = binary.BigEndian.Uint16(buf[EthHdrLen+2 : EthHdrLen+4])
	payloadLen := int(binary.BigEndian.Uint16(buf[EthHdrLen+4 : EthHdrLen+6]))

	tagsStart := EthHdrLen + PPPoEHdrLen
	if tagsStart+payloadLen > len(buf) {
		return Packet{}, ErrFrameTooShort
	}

	pkt.Tags = parseTags(buf[tagsStart : tagsStart+payloadLen])
	if pkt.Tags == nil && payloadLen >= TagHdrLen {
		return Packet{}, ErrInvalidTagLength
	}

	return pkt, nil
}

// maxTags caps the number of tags parsed from a single frame to limit
// allocation under crafted-packet floods. A legitimate PPPoE discovery
// frame contains at most ~10 tags; 32 is generous headroom.
const maxTags = 32

// parseTags iterates over the TLV-encoded tags in payload. Returns nil
// (not empty slice) if a tag's length field extends past the end of the
// payload. An End-Of-List tag terminates parsing. At most maxTags are
// returned; excess tags are silently ignored.
func parseTags(payload []byte) []Tag {
	tags := make([]Tag, 0, min(len(payload)/TagHdrLen, maxTags))
	off := 0
	for off+TagHdrLen <= len(payload) {
		tagType := binary.BigEndian.Uint16(payload[off : off+2])
		tagLen := int(binary.BigEndian.Uint16(payload[off+2 : off+4]))

		if tagType == TagEndOfList {
			break
		}

		if off+TagHdrLen+tagLen > len(payload) {
			return nil
		}

		var val []byte
		if tagLen > 0 {
			val = payload[off+TagHdrLen : off+TagHdrLen+tagLen]
		}
		tags = append(tags, Tag{Type: tagType, Value: val})
		off += TagHdrLen + tagLen
		if len(tags) >= maxTags {
			break
		}
	}

	return tags
}

// FindTag returns the first tag with the given type, or nil if not
// found. The returned value is a sub-slice of the original buffer.
func (p *Packet) FindTag(tagType uint16) *Tag {
	for i := range p.Tags {
		if p.Tags[i].Type == tagType {
			return &p.Tags[i]
		}
	}
	return nil
}

// FindAllTags returns all tags with the given type.
func (p *Packet) FindAllTags(tagType uint16) []Tag {
	var result []Tag
	for _, t := range p.Tags {
		if t.Type == tagType {
			result = append(result, t)
		}
	}
	return result
}

// ServiceNameString returns the Service-Name tag value as a string,
// or empty string if no Service-Name tag is present.
func (p *Packet) ServiceNameString() string {
	t := p.FindTag(TagServiceName)
	if t == nil {
		return ""
	}
	return string(t.Value)
}

// MatchServiceName checks whether the packet's Service-Name matches
// the configured service names. An empty allowedNames slice accepts
// any service. An empty Service-Name tag in the packet matches any
// configured service (RFC 2516 Section 5.1: "A Service-Name tag with
// a zero-length data section ... indicates any service is acceptable").
func MatchServiceName(pkt *Packet, allowedNames []string) bool {
	if len(allowedNames) == 0 {
		return true
	}
	tag := pkt.FindTag(TagServiceName)
	if tag == nil {
		return false
	}
	if len(tag.Value) == 0 {
		return true
	}
	return slices.Contains(allowedNames, string(tag.Value))
}

// Builder constructs PPPoE discovery frames into a caller-provided
// buffer. The caller provides the buffer (typically [EthMaxLen]byte
// on the stack) and the builder writes into it.
type Builder struct {
	buf       []byte
	tagOff    int
	truncated bool
}

// NewBuilder initializes a Builder that writes into buf. The buffer
// must be at least MinDiscFrame bytes.
func NewBuilder(buf []byte, srcMAC, dstMAC [EthALen]byte, code byte, sid uint16) Builder {
	copy(buf[0:EthALen], dstMAC[:])
	copy(buf[EthALen:2*EthALen], srcMAC[:])
	binary.BigEndian.PutUint16(buf[2*EthALen:], EthPPPDisc)

	buf[EthHdrLen] = PPPoEVerType
	buf[EthHdrLen+1] = code
	binary.BigEndian.PutUint16(buf[EthHdrLen+2:], sid)
	binary.BigEndian.PutUint16(buf[EthHdrLen+4:], 0) // length, updated by Finish

	return Builder{buf: buf, tagOff: MinDiscFrame}
}

// addTag appends a tag to the frame. Returns false if the tag would
// not fit in the buffer.
func (b *Builder) addTag(tagType uint16, value []byte) bool {
	needed := TagHdrLen + len(value)
	if b.tagOff+needed > len(b.buf) {
		b.truncated = true
		return false
	}
	binary.BigEndian.PutUint16(b.buf[b.tagOff:], tagType)
	binary.BigEndian.PutUint16(b.buf[b.tagOff+2:], uint16(len(value)))
	if len(value) > 0 {
		copy(b.buf[b.tagOff+TagHdrLen:], value)
	}
	b.tagOff += needed
	return true
}

// AddTagString is a convenience for string-valued tags.
func (b *Builder) AddTagString(tagType uint16, s string) bool {
	return b.addTag(tagType, []byte(s))
}

// AddTagCopy copies a Tag from a parsed packet into the frame being
// built. Used to echo Host-Uniq, Relay-Session-Id, etc.
func (b *Builder) AddTagCopy(t *Tag) bool {
	if t == nil {
		return true
	}
	return b.addTag(t.Type, t.Value)
}

// Finish writes the PPPoE payload length field and returns the
// complete frame slice. Returns nil if the frame was truncated.
func (b *Builder) Finish() []byte {
	if b.truncated {
		return nil
	}
	payloadLen := uint16(b.tagOff - MinDiscFrame)
	binary.BigEndian.PutUint16(b.buf[EthHdrLen+4:], payloadLen)
	return b.buf[:b.tagOff]
}

// BuildPADI constructs a PADI frame for PPPoE client discovery
// (RFC 2516 Section 5.1). Sent to the broadcast address to solicit
// PADO responses from access concentrators. The hostUniq tag is
// included so the client can correlate responses.
func BuildPADI(buf []byte, srcMAC [EthALen]byte, serviceName string, hostUniq []byte) []byte {
	b := NewBuilder(buf, srcMAC, [EthALen]byte(BroadcastMAC), CodePADI, 0)

	b.AddTagString(TagServiceName, serviceName)
	if len(hostUniq) > 0 {
		b.addTag(TagHostUniq, hostUniq)
	}

	return b.Finish()
}

// BuildPADR constructs a PADR frame for PPPoE client discovery
// (RFC 2516 Section 5.3). Sent unicast to the selected AC after
// receiving a PADO. Echoes the AC-Cookie and Service-Name from the
// PADO. The hostUniq tag is included for correlation.
func BuildPADR(buf []byte, srcMAC [EthALen]byte, pado *Packet, serviceName string, hostUniq []byte) []byte {
	b := NewBuilder(buf, srcMAC, pado.SrcMAC, CodePADR, 0)

	b.AddTagString(TagServiceName, serviceName)
	// RFC 2516 Section 5.3: MUST echo AC-Cookie and Relay-Session-Id if present.
	b.AddTagCopy(pado.FindTag(TagACCookie))
	b.AddTagCopy(pado.FindTag(TagRelaySessionID))
	if len(hostUniq) > 0 {
		b.addTag(TagHostUniq, hostUniq)
	}

	return b.Finish()
}

// BuildPADO constructs a PADO frame in response to a PADI. It echoes
// the Host-Uniq and Relay-Session-Id tags from the PADI if present.
func BuildPADO(buf []byte, acMAC [EthALen]byte, padi *Packet, acName string, serviceNames []string, cookie []byte) []byte {
	b := NewBuilder(buf, acMAC, padi.SrcMAC, CodePADO, 0)

	b.AddTagString(TagACName, acName)
	for _, sn := range serviceNames {
		b.AddTagString(TagServiceName, sn)
	}

	svcTag := padi.FindTag(TagServiceName)
	if svcTag != nil && len(svcTag.Value) > 0 && !slices.Contains(serviceNames, string(svcTag.Value)) {
		b.AddTagCopy(svcTag)
	}

	b.addTag(TagACCookie, cookie)
	b.AddTagCopy(padi.FindTag(TagHostUniq))
	b.AddTagCopy(padi.FindTag(TagRelaySessionID))

	if maxPL := padi.FindTag(TagPPPMaxPayload); maxPL != nil && len(maxPL.Value) == 2 {
		b.AddTagCopy(maxPL)
	}

	return b.Finish()
}

// BuildPADS constructs a PADS frame in response to a PADR. Echoes
// the Service-Name, Host-Uniq, and Relay-Session-Id from the PADR.
func BuildPADS(buf []byte, acMAC [EthALen]byte, padr *Packet, acName string, sid uint16) []byte {
	b := NewBuilder(buf, acMAC, padr.SrcMAC, CodePADS, sid)

	b.AddTagString(TagACName, acName)
	b.AddTagCopy(padr.FindTag(TagServiceName))
	b.AddTagCopy(padr.FindTag(TagHostUniq))
	b.AddTagCopy(padr.FindTag(TagRelaySessionID))

	return b.Finish()
}

// BuildPADSError constructs a PADS frame with session_id=0 and an
// error tag (Service-Name-Error or AC-System-Error).
func BuildPADSError(buf []byte, acMAC [EthALen]byte, padr *Packet, acName string, errTag uint16) []byte {
	b := NewBuilder(buf, acMAC, padr.SrcMAC, CodePADS, 0)

	b.AddTagString(TagACName, acName)
	b.addTag(errTag, nil)
	b.AddTagCopy(padr.FindTag(TagHostUniq))
	b.AddTagCopy(padr.FindTag(TagRelaySessionID))

	return b.Finish()
}

// BuildPADT constructs a PADT frame to terminate a session.
func BuildPADT(buf []byte, srcMAC, dstMAC [EthALen]byte, sid uint16, acName string) []byte {
	b := NewBuilder(buf, srcMAC, dstMAC, CodePADT, sid)

	b.AddTagString(TagACName, acName)

	return b.Finish()
}
