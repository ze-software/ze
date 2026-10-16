// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV type codes
// RFC: rfc/short/rfc7752.md
// RFC: rfc/short/rfc9085.md
// RFC: rfc/short/rfc9514.md
//
// The BGP-LS plugin registers a decoder for each code it decodes; this file
// only names the codes.

package bgpls

// Link attribute TLV type codes.
// RFC 7752 Section 3.3.2 defines link attribute TLVs.
const (
	TLVIPv4RouterIDRemote uint16 = 1030 // RFC 7752 Section 3.3.2.1
	TLVIPv6RouterIDRemote uint16 = 1031 // RFC 7752 Section 3.3.2.1
	TLVAdminGroup         uint16 = 1088 // RFC 7752 Section 3.3.2.5
	TLVMaxLinkBandwidth   uint16 = 1089 // RFC 7752 Section 3.3.2.3
	TLVMaxReservableBW    uint16 = 1090 // RFC 7752 Section 3.3.2.4
	TLVUnreservedBW       uint16 = 1091 // RFC 7752 Section 3.3.2.4
	TLVTEDefaultMetric    uint16 = 1092 // RFC 7752 Section 3.3.2.7
	TLVIGPMetric          uint16 = 1095 // RFC 7752 Section 3.3.2.4
	TLVSRLG               uint16 = 1096 // RFC 7752 Section 3.3.2.6
	TLVOpaqueLinkAttr     uint16 = 1097 // RFC 7752 Section 3.3.2.10
	TLVLinkName           uint16 = 1098 // RFC 7752 Section 3.3.2.7
)

// SR-MPLS link attribute TLV type code.
const (
	TLVAdjacencySID uint16 = 1099 // RFC 9085 Section 2.2.1
)

const (
	TLVPeerNodeSID uint16 = 1101 // RFC 9086 Section 5
	TLVPeerAdjSID  uint16 = 1102 // RFC 9086 Section 5
	TLVPeerSetSID  uint16 = 1103 // RFC 9086 Section 5
)

const (
	TLVSRv6EndXSID     uint16 = 1106 // RFC 9514 Section 4
	TLVSRv6LANEndXISIS uint16 = 1107 // RFC 9514 Section 4 (IS-IS)
	TLVSRv6LANEndXOSPF uint16 = 1108 // RFC 9514 Section 4 (OSPFv3)
)

const (
	TLVUnidirectionalDelay uint16 = 1114 // RFC 8571 Section 3
	TLVMinMaxDelay         uint16 = 1115 // RFC 8571 Section 4
	TLVDelayVariation      uint16 = 1116 // RFC 8571 Section 5
)

// Node attribute TLV type codes.
// RFC 7752 Section 3.3.1 defines node attribute TLVs.
const (
	TLVNodeFlagBits      uint16 = 1024 // RFC 7752 Section 3.3.1.1
	TLVOpaqueNodeAttr    uint16 = 1025 // RFC 7752 Section 3.3.1.5
	TLVNodeName          uint16 = 1026 // RFC 7752 Section 3.3.1.3
	TLVISISAreaID        uint16 = 1027 // RFC 7752 Section 3.3.1.2
	TLVIPv4RouterIDLocal uint16 = 1028 // RFC 7752 Section 3.3.1.4
	TLVIPv6RouterIDLocal uint16 = 1029 // RFC 7752 Section 3.3.1.4
)

// SR-MPLS node attribute TLV type codes.
const (
	TLVSRCapabilities uint16 = 1034 // RFC 9085 Section 3
	TLVSRAlgorithm    uint16 = 1035 // RFC 9085 Section 4
	TLVSRLocalBlock   uint16 = 1036 // RFC 9085 Section 5
)

// Prefix attribute TLV type codes.
// RFC 7752 Section 3.3.3 defines prefix attribute TLVs.
const (
	TLVIGPFlags         uint16 = 1152 // RFC 7752 Section 3.3.3.1
	TLVPrefixMetric     uint16 = 1155 // RFC 7752 Section 3.3.3.4
	TLVOpaquePrefixAttr uint16 = 1157 // RFC 7752 Section 3.3.3.6
)

// SR-MPLS prefix attribute TLV type codes.
const (
	TLVPrefixSID      uint16 = 1158 // RFC 9085 Section 2.3.1
	TLVSIDLabel       uint16 = 1161 // RFC 9085 Section 2.1.1
	TLVSRPrefixFlags  uint16 = 1170 // RFC 9085 Section 2.3.2
	TLVSourceRouterID uint16 = 1171 // RFC 9085 Section 2.3.3
)

// SRv6 attribute TLV type codes.
// RFC 9514 Section 8 defines SRv6 attribute TLVs.
const (
	TLVSRv6EndpointBehavior uint16 = 1250 // RFC 9514 Section 8
	TLVSRv6BGPPeerNodeSID   uint16 = 1251 // RFC 9514 Section 5.1
	TLVSRv6SIDStructure     uint16 = 1252 // RFC 9514 Section 8
)
