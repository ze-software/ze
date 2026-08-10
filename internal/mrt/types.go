// Design: docs/architecture/mrt.md — wire format types and constants

package mrt

// MRT Type codes (RFC 6396 Section 5.3).
const (
	TypeOSPFv2      uint16 = 11
	TypeTableDump   uint16 = 12
	TypeTableDumpV2 uint16 = 13
	TypeBGP4MP      uint16 = 16
	TypeBGP4MPET    uint16 = 17
	TypeISIS        uint16 = 32
	TypeISISET      uint16 = 33
	TypeOSPFv3      uint16 = 48
	TypeOSPFv3ET    uint16 = 49
)

// TABLE_DUMP subtypes (RFC 6396 Section 5.5).
const (
	TableDumpAFIIPv4 uint16 = 1
	TableDumpAFIIPv6 uint16 = 2
)

// TABLE_DUMP_V2 subtypes (RFC 6396 Section 5.6, RFC 6397, RFC 8050).
const (
	TDV2PeerIndexTable     uint16 = 1
	TDV2RIBIPv4Unicast     uint16 = 2
	TDV2RIBIPv4Multicast   uint16 = 3
	TDV2RIBIPv6Unicast     uint16 = 4
	TDV2RIBIPv6Multicast   uint16 = 5
	TDV2RIBGeneric         uint16 = 6
	TDV2GeoPeerTable       uint16 = 7  // RFC 6397
	TDV2RIBIPv4UnicastAP   uint16 = 8  // RFC 8050
	TDV2RIBIPv4MulticastAP uint16 = 9  // RFC 8050
	TDV2RIBIPv6UnicastAP   uint16 = 10 // RFC 8050
	TDV2RIBIPv6MulticastAP uint16 = 11 // RFC 8050
	TDV2RIBGenericAP       uint16 = 12 // RFC 8050
)

// BGP4MP / BGP4MP_ET subtypes (RFC 6396 Section 5.7, RFC 8050).
const (
	BGP4MPStateChange       uint16 = 0
	BGP4MPMessage           uint16 = 1
	BGP4MPEntry             uint16 = 2 // deprecated
	BGP4MPSnapshot          uint16 = 3 // deprecated
	BGP4MPMessageAS4        uint16 = 4
	BGP4MPStateChangeAS4    uint16 = 5
	BGP4MPMessageLocal      uint16 = 6
	BGP4MPMessageAS4Local   uint16 = 7
	BGP4MPMessageAP         uint16 = 8  // RFC 8050
	BGP4MPMessageAS4AP      uint16 = 9  // RFC 8050
	BGP4MPMessageLocalAP    uint16 = 10 // RFC 8050
	BGP4MPMessageAS4LocalAP uint16 = 11 // RFC 8050
)

// BGP FSM states (RFC 4271 Section 8.2.2).
const (
	FSMIdle        uint16 = 1
	FSMConnect     uint16 = 2
	FSMActive      uint16 = 3
	FSMOpenSent    uint16 = 4
	FSMOpenConfirm uint16 = 5
	FSMEstablished uint16 = 6
)

// Address families for MRT headers.
const (
	AFIIPv4 uint16 = 1
	AFIIPv6 uint16 = 2
)

// Peer Type bit flags (RFC 6396 Section 4.3.1 Figure 7).
const (
	PeerAS4  byte = 0x02 // bit 6: 1 = 32-bit AS
	PeerIPv6 byte = 0x01 // bit 7: 1 = IPv6
)

// Header sizes.
const (
	CommonHeaderLen = 12       // Timestamp(4) + Type(2) + Subtype(2) + Length(4)
	ExtTimestampLen = 4        // Microsecond Timestamp field
	MaxRecordLen    = 16 << 20 // 16 MiB safety cap
)

// PeerEntry represents a peer in a PEER_INDEX_TABLE.
type PeerEntry struct {
	Type  byte
	BGPID [4]byte
	IP    []byte // 4 or 16 bytes
	ASN   uint32
}

// IsIPv6 reports whether the peer uses IPv6.
func (p *PeerEntry) IsIPv6() bool { return p.Type&PeerIPv6 != 0 }

// IsAS4 reports whether the peer uses 4-byte AS numbers.
func (p *PeerEntry) IsAS4() bool { return p.Type&PeerAS4 != 0 }

// RIBEntry represents a single RIB entry within a TABLE_DUMP_V2 RIB record.
type RIBEntry struct {
	PeerIndex  uint16
	OrigTime   uint32
	PathID     uint32 // only for add-path subtypes (RFC 8050)
	Attributes []byte
}

// BGP4MPHeader represents the common fields of BGP4MP messages.
type BGP4MPHeader struct {
	PeerAS  uint32
	LocalAS uint32
	IfIndex uint16
	AFI     uint16
	PeerIP  []byte // 4 or 16 bytes
	LocalIP []byte // 4 or 16 bytes
}

// TableDumpRecord represents a TABLE_DUMP (v1) record.
type TableDumpRecord struct {
	ViewNumber uint16
	SeqNumber  uint16
	Prefix     []byte // 4 or 16 bytes
	PrefixLen  uint8
	Status     uint8
	OrigTime   uint32
	PeerIP     []byte // 4 or 16 bytes
	PeerAS     uint16
	Attributes []byte
}

// IsAddPathRIBSubtype reports whether a TABLE_DUMP_V2 subtype carries Path Identifiers per RFC 8050.
func IsAddPathRIBSubtype(subtype uint16) bool {
	return subtype >= TDV2RIBIPv4UnicastAP && subtype <= TDV2RIBGenericAP
}

// IsAS4Subtype reports whether a BGP4MP subtype uses 4-byte AS numbers.
func IsAS4Subtype(subtype uint16) bool {
	switch subtype {
	case BGP4MPMessageAS4, BGP4MPStateChangeAS4,
		BGP4MPMessageAS4Local, BGP4MPMessageAS4AP, BGP4MPMessageAS4LocalAP:
		return true
	}
	return false
}

// isStateChangeSubtype reports whether a BGP4MP subtype is a state change.
func isStateChangeSubtype(subtype uint16) bool {
	return subtype == BGP4MPStateChange || subtype == BGP4MPStateChangeAS4
}

// IsAddPathBGP4MPSubtype reports whether a BGP4MP subtype carries add-path NLRI.
func IsAddPathBGP4MPSubtype(subtype uint16) bool {
	return subtype >= BGP4MPMessageAP && subtype <= BGP4MPMessageAS4LocalAP
}

// RIBSubtypeAFI returns the AFI for an AFI/SAFI-specific RIB subtype.
// Returns 0 for non-AFI-specific subtypes (RIB_GENERIC, PEER_INDEX_TABLE, GEO_PEER_TABLE).
func RIBSubtypeAFI(subtype uint16) uint16 {
	switch subtype {
	case TDV2RIBIPv4Unicast, TDV2RIBIPv4Multicast,
		TDV2RIBIPv4UnicastAP, TDV2RIBIPv4MulticastAP:
		return AFIIPv4
	case TDV2RIBIPv6Unicast, TDV2RIBIPv6Multicast,
		TDV2RIBIPv6UnicastAP, TDV2RIBIPv6MulticastAP:
		return AFIIPv6
	}
	return 0
}
