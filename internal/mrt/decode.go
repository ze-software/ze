// Design: docs/architecture/mrt.md — wire format decoding

package mrt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Exported so a caller can tell the failure kinds apart with errors.Is instead
// of matching on message text. internal/analyze needs exactly that: a truncated
// NLRI (ErrShortData) is a damaged record it should count and keep going on,
// while an unrecognized address family (ErrBadAFI) means the record is not one
// it understands at all.
var (
	// ErrShortData reports input that ends before a field the format requires.
	ErrShortData = errors.New("mrt: short data")
	// ErrBadAFI reports an Address Family Identifier this decoder does not handle.
	ErrBadAFI = errors.New("mrt: unsupported address family")
)

// Header is the MRT common header (RFC 6396 Section 2).
type Header struct {
	Timestamp uint32
	Type      uint16
	Subtype   uint16
	Length    uint32
}

// PeerIndexTable represents a TABLE_DUMP_V2 PEER_INDEX_TABLE (RFC 6396 Section 4.3.1).
type PeerIndexTable struct {
	CollectorBGPID [4]byte
	ViewName       string
	Peers          []PeerEntry
}

// RIBRecord represents a TABLE_DUMP_V2 AFI/SAFI-specific RIB record (RFC 6396 Section 4.3.2).
type RIBRecord struct {
	SequenceNumber uint32
	PrefixLength   uint8
	Prefix         []byte
	Entries        []RIBEntry
}

// RIBGenericRecord represents a TABLE_DUMP_V2 RIB_GENERIC record (RFC 6396 Section 4.3.3).
type RIBGenericRecord struct {
	SequenceNumber uint32
	AFI            uint16
	SAFI           uint8
	NLRI           []byte
	Entries        []RIBEntry
}

// MessageRecord represents a BGP4MP MESSAGE record (RFC 6396 Section 4.4.2).
type MessageRecord struct {
	BGP4MPHeader
	BGPMessage []byte
}

// StateChangeRecord represents a BGP4MP STATE_CHANGE record (RFC 6396 Section 4.4.1).
type StateChangeRecord struct {
	BGP4MPHeader
	OldState uint16
	NewState uint16
}

// GeoPeerEntry represents a peer in a GEO_PEER_TABLE (RFC 6397 Section 4.1).
type GeoPeerEntry struct {
	Type      byte
	BGPID     [4]byte
	Latitude  float32
	Longitude float32
}

// geoPeerTable represents a TABLE_DUMP_V2 GEO_PEER_TABLE (RFC 6397 Section 4.1).
type geoPeerTable struct {
	CollectorBGPID [4]byte
	CollectorLat   float32
	CollectorLon   float32
	Peers          []GeoPeerEntry
}

func ipSize(afi uint16) (int, error) {
	switch afi {
	case AFIIPv4:
		return 4, nil
	case AFIIPv6:
		return 16, nil
	}
	return 0, fmt.Errorf("%w: %d", ErrBadAFI, afi)
}

// DecodeHeader parses a 12-byte MRT common header.
func DecodeHeader(data []byte) (Header, error) {
	if len(data) < CommonHeaderLen {
		return Header{}, fmt.Errorf("header: %w (have %d, need %d)", ErrShortData, len(data), CommonHeaderLen)
	}
	return Header{
		Timestamp: binary.BigEndian.Uint32(data[0:4]),
		Type:      binary.BigEndian.Uint16(data[4:6]),
		Subtype:   binary.BigEndian.Uint16(data[6:8]),
		Length:    binary.BigEndian.Uint32(data[8:12]),
	}, nil
}

// DecodeMicrosecond reads the 4-byte microsecond timestamp that follows the
// common header in _ET type records (RFC 6396 Section 3).
func DecodeMicrosecond(data []byte) (uint32, error) {
	if len(data) < ExtTimestampLen {
		return 0, fmt.Errorf("microsecond: %w (have %d, need %d)", ErrShortData, len(data), ExtTimestampLen)
	}
	return binary.BigEndian.Uint32(data[0:4]), nil
}

// DecodePeerIndexTable parses a TABLE_DUMP_V2 PEER_INDEX_TABLE message body.
func DecodePeerIndexTable(data []byte) (*PeerIndexTable, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("peer index table: %w (have %d, need >=8)", ErrShortData, len(data))
	}

	pit := &PeerIndexTable{}
	copy(pit.CollectorBGPID[:], data[0:4])

	viewNameLen := int(binary.BigEndian.Uint16(data[4:6]))
	off := 6
	if off+viewNameLen > len(data) {
		return nil, fmt.Errorf("peer index table view name: %w", ErrShortData)
	}
	if viewNameLen > 0 {
		pit.ViewName = string(data[off : off+viewNameLen])
	}
	off += viewNameLen

	if off+2 > len(data) {
		return nil, fmt.Errorf("peer index table peer count: %w", ErrShortData)
	}
	peerCount := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	pit.Peers = make([]PeerEntry, 0, peerCount)
	for i := range peerCount {
		if off >= len(data) {
			return nil, fmt.Errorf("peer entry %d: %w", i, ErrShortData)
		}
		pe := PeerEntry{Type: data[off]}
		off++

		if off+4 > len(data) {
			return nil, fmt.Errorf("peer entry %d bgp id: %w", i, ErrShortData)
		}
		copy(pe.BGPID[:], data[off:off+4])
		off += 4

		ipLen := 4
		if pe.IsIPv6() {
			ipLen = 16
		}
		if off+ipLen > len(data) {
			return nil, fmt.Errorf("peer entry %d ip: %w", i, ErrShortData)
		}
		pe.IP = make([]byte, ipLen)
		copy(pe.IP, data[off:off+ipLen])
		off += ipLen

		asLen := 2
		if pe.IsAS4() {
			asLen = 4
		}
		if off+asLen > len(data) {
			return nil, fmt.Errorf("peer entry %d as: %w", i, ErrShortData)
		}
		if asLen == 4 {
			pe.ASN = binary.BigEndian.Uint32(data[off : off+4])
		} else {
			pe.ASN = uint32(binary.BigEndian.Uint16(data[off : off+2]))
		}
		off += asLen

		pit.Peers = append(pit.Peers, pe)
	}

	return pit, nil
}

// DecodeRIBRecord parses a TABLE_DUMP_V2 AFI/SAFI-specific RIB record
// (subtypes 2-5 and add-path 8-11).
func DecodeRIBRecord(subtype uint16, data []byte) (*RIBRecord, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("rib record: %w (have %d, need >=5)", ErrShortData, len(data))
	}

	rec := &RIBRecord{
		SequenceNumber: binary.BigEndian.Uint32(data[0:4]),
		PrefixLength:   data[4],
	}
	off := 5

	prefixBytes := int((rec.PrefixLength + 7) / 8)
	if off+prefixBytes > len(data) {
		return nil, fmt.Errorf("rib record prefix: %w", ErrShortData)
	}
	rec.Prefix = make([]byte, prefixBytes)
	copy(rec.Prefix, data[off:off+prefixBytes])
	off += prefixBytes

	if off+2 > len(data) {
		return nil, fmt.Errorf("rib record entry count: %w", ErrShortData)
	}
	entryCount := binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	addPath := IsAddPathRIBSubtype(subtype) && subtype != TDV2RIBGenericAP
	entries, err := decodeRIBEntries(data[off:], entryCount, addPath)
	if err != nil {
		return nil, fmt.Errorf("rib record entries: %w", err)
	}
	rec.Entries = entries

	return rec, nil
}

// DecodeRIBGenericRecord parses a TABLE_DUMP_V2 RIB_GENERIC record (subtypes 6 and 12).
func DecodeRIBGenericRecord(subtype uint16, data []byte) (*RIBGenericRecord, error) {
	if len(data) < 7 {
		return nil, fmt.Errorf("rib generic: %w (have %d, need >=7)", ErrShortData, len(data))
	}

	rec := &RIBGenericRecord{
		SequenceNumber: binary.BigEndian.Uint32(data[0:4]),
		AFI:            binary.BigEndian.Uint16(data[4:6]),
		SAFI:           data[6],
	}
	off := 7

	// NLRI is AFI/SAFI-specific. For RIB_GENERIC the encoding is a prefix
	// length byte followed by prefix bytes (standard BGP NLRI encoding).
	// For RIB_GENERIC_ADDPATH (subtype 12), a 4-byte Path ID precedes the
	// prefix length byte inside the NLRI blob (RFC 8050 Section 4.2).
	nlriStart := off
	if subtype == TDV2RIBGenericAP {
		if off+4 >= len(data) {
			return nil, fmt.Errorf("rib generic addpath nlri: %w", ErrShortData)
		}
		off += 4 // skip Path ID to reach prefix length
	}
	if off >= len(data) {
		return nil, fmt.Errorf("rib generic nlri: %w", ErrShortData)
	}
	nlriPrefixLen := data[off]
	nlriBytes := (off - nlriStart) + 1 + int((nlriPrefixLen+7)/8)

	if nlriStart+nlriBytes > len(data) {
		return nil, fmt.Errorf("rib generic nlri data: %w", ErrShortData)
	}
	rec.NLRI = make([]byte, nlriBytes)
	copy(rec.NLRI, data[nlriStart:nlriStart+nlriBytes])
	off = nlriStart + nlriBytes

	if off+2 > len(data) {
		return nil, fmt.Errorf("rib generic entry count: %w", ErrShortData)
	}
	entryCount := binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	// RIB_GENERIC_ADDPATH (12): RIB entries are NOT redefined per RFC 8050 Section 4.2.
	entries, err := decodeRIBEntries(data[off:], entryCount, false)
	if err != nil {
		return nil, fmt.Errorf("rib generic entries: %w", err)
	}
	rec.Entries = entries

	return rec, nil
}

// decodeRIBEntries parses a sequence of RIB entries.
// When addPath is true, each entry includes a 4-byte Path Identifier (RFC 8050 Section 4.1).
func decodeRIBEntries(data []byte, count uint16, addPath bool) ([]RIBEntry, error) {
	entries := make([]RIBEntry, 0, count)
	off := 0

	for i := range int(count) {
		// Peer Index (2) + Originated Time (4) = 6 minimum
		minLen := 6
		if addPath {
			minLen += 4 // Path Identifier
		}
		minLen += 2 // Attribute Length

		if off+minLen > len(data) {
			return nil, fmt.Errorf("rib entry %d: %w (have %d, need >=%d)", i, ErrShortData, len(data)-off, minLen)
		}

		e := RIBEntry{
			PeerIndex: binary.BigEndian.Uint16(data[off : off+2]),
			OrigTime:  binary.BigEndian.Uint32(data[off+2 : off+6]),
		}
		off += 6

		if addPath {
			e.PathID = binary.BigEndian.Uint32(data[off : off+4])
			off += 4
		}

		attrLen := int(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2

		if off+attrLen > len(data) {
			return nil, fmt.Errorf("rib entry %d attrs: %w (have %d, need %d)", i, ErrShortData, len(data)-off, attrLen)
		}
		e.Attributes = make([]byte, attrLen)
		copy(e.Attributes, data[off:off+attrLen])
		off += attrLen

		entries = append(entries, e)
	}

	return entries, nil
}

// DecodeBGP4MPMessage parses a BGP4MP MESSAGE variant.
// Supports subtypes: 1 (MESSAGE), 4 (MESSAGE_AS4), 6 (MESSAGE_LOCAL),
// 7 (MESSAGE_AS4_LOCAL), and add-path variants 8-11.
func DecodeBGP4MPMessage(subtype uint16, data []byte) (*MessageRecord, error) {
	hdr, off, err := decodeBGP4MPHeader(subtype, data)
	if err != nil {
		return nil, fmt.Errorf("bgp4mp message: %w", err)
	}

	msg := &MessageRecord{BGP4MPHeader: hdr}
	if off < len(data) {
		msg.BGPMessage = make([]byte, len(data)-off)
		copy(msg.BGPMessage, data[off:])
	}
	return msg, nil
}

// DecodeBGP4MPStateChange parses a BGP4MP STATE_CHANGE variant (subtypes 0 and 5).
func DecodeBGP4MPStateChange(subtype uint16, data []byte) (*StateChangeRecord, error) {
	hdr, off, err := decodeBGP4MPHeader(subtype, data)
	if err != nil {
		return nil, fmt.Errorf("bgp4mp state change: %w", err)
	}

	if off+4 > len(data) {
		return nil, fmt.Errorf("bgp4mp state change states: %w", ErrShortData)
	}

	return &StateChangeRecord{
		BGP4MPHeader: hdr,
		OldState:     binary.BigEndian.Uint16(data[off : off+2]),
		NewState:     binary.BigEndian.Uint16(data[off+2 : off+4]),
	}, nil
}

// decodeBGP4MPHeader parses the common BGP4MP header fields and returns the
// parsed header plus the offset where the payload (BGP message or states) begins.
func decodeBGP4MPHeader(subtype uint16, data []byte) (BGP4MPHeader, int, error) {
	as4 := IsAS4Subtype(subtype)
	asLen := 2
	if as4 {
		asLen = 4
	}

	// AS fields (2 or 4 each) + Interface Index (2) + Address Family (2)
	minHdr := asLen*2 + 4
	if len(data) < minHdr {
		return BGP4MPHeader{}, 0, fmt.Errorf("bgp4mp header: %w (have %d, need >=%d)", ErrShortData, len(data), minHdr)
	}

	off := 0
	var hdr BGP4MPHeader

	if as4 {
		hdr.PeerAS = binary.BigEndian.Uint32(data[off : off+4])
		off += 4
		hdr.LocalAS = binary.BigEndian.Uint32(data[off : off+4])
		off += 4
	} else {
		hdr.PeerAS = uint32(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
		hdr.LocalAS = uint32(binary.BigEndian.Uint16(data[off : off+2]))
		off += 2
	}

	hdr.IfIndex = binary.BigEndian.Uint16(data[off : off+2])
	off += 2
	hdr.AFI = binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	ipLen, err := ipSize(hdr.AFI)
	if err != nil {
		return BGP4MPHeader{}, 0, err
	}

	if off+ipLen*2 > len(data) {
		return BGP4MPHeader{}, 0, fmt.Errorf("bgp4mp header ips: %w", ErrShortData)
	}

	hdr.PeerIP = make([]byte, ipLen)
	copy(hdr.PeerIP, data[off:off+ipLen])
	off += ipLen

	hdr.LocalIP = make([]byte, ipLen)
	copy(hdr.LocalIP, data[off:off+ipLen])
	off += ipLen

	return hdr, off, nil
}

// DecodeTableDump parses a TABLE_DUMP (type 12) message body.
// Subtype determines IP size: 1 = IPv4 (4 bytes), 2 = IPv6 (16 bytes).
func DecodeTableDump(subtype uint16, data []byte) (*TableDumpRecord, error) {
	ipLen, err := ipSize(subtype) // subtype is AFI for TABLE_DUMP
	if err != nil {
		return nil, fmt.Errorf("table dump: %w", err)
	}

	// ViewNumber(2) + SeqNumber(2) + Prefix(ipLen) + PrefixLen(1) + Status(1) +
	// OrigTime(4) + PeerIP(ipLen) + PeerAS(2) + AttrLength(2)
	minLen := 2 + 2 + ipLen + 1 + 1 + 4 + ipLen + 2 + 2
	if len(data) < minLen {
		return nil, fmt.Errorf("table dump: %w (have %d, need >=%d)", ErrShortData, len(data), minLen)
	}

	rec := &TableDumpRecord{}
	off := 0

	rec.ViewNumber = binary.BigEndian.Uint16(data[off : off+2])
	off += 2
	rec.SeqNumber = binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	rec.Prefix = make([]byte, ipLen)
	copy(rec.Prefix, data[off:off+ipLen])
	off += ipLen

	rec.PrefixLen = data[off]
	off++
	rec.Status = data[off]
	off++

	rec.OrigTime = binary.BigEndian.Uint32(data[off : off+4])
	off += 4

	rec.PeerIP = make([]byte, ipLen)
	copy(rec.PeerIP, data[off:off+ipLen])
	off += ipLen

	rec.PeerAS = binary.BigEndian.Uint16(data[off : off+2])
	off += 2

	attrLen := int(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2

	if off+attrLen > len(data) {
		return nil, fmt.Errorf("table dump attrs: %w (have %d, need %d)", ErrShortData, len(data)-off, attrLen)
	}
	rec.Attributes = make([]byte, attrLen)
	copy(rec.Attributes, data[off:off+attrLen])

	return rec, nil
}

// decodeGeoPeerTable parses a TABLE_DUMP_V2 GEO_PEER_TABLE message body (RFC 6397).
func decodeGeoPeerTable(data []byte) (*geoPeerTable, error) {
	// CollectorBGPID(4) + Lat(4) + Lon(4) + PeerCount(2) = 14
	if len(data) < 14 {
		return nil, fmt.Errorf("geo peer table: %w (have %d, need >=14)", ErrShortData, len(data))
	}

	gpt := &geoPeerTable{}
	copy(gpt.CollectorBGPID[:], data[0:4])
	gpt.CollectorLat = math.Float32frombits(binary.BigEndian.Uint32(data[4:8]))
	gpt.CollectorLon = math.Float32frombits(binary.BigEndian.Uint32(data[8:12]))

	peerCount := int(binary.BigEndian.Uint16(data[12:14]))
	off := 14

	gpt.Peers = make([]GeoPeerEntry, 0, peerCount)
	for i := range peerCount {
		// Type(1) + BGPID(4) + Lat(4) + Lon(4) = 13
		if off+13 > len(data) {
			return nil, fmt.Errorf("geo peer entry %d: %w", i, ErrShortData)
		}

		ge := GeoPeerEntry{
			Type: data[off],
		}
		off++
		copy(ge.BGPID[:], data[off:off+4])
		off += 4
		ge.Latitude = math.Float32frombits(binary.BigEndian.Uint32(data[off : off+4]))
		off += 4
		ge.Longitude = math.Float32frombits(binary.BigEndian.Uint32(data[off : off+4]))
		off += 4

		gpt.Peers = append(gpt.Peers, ge)
	}

	return gpt, nil
}
