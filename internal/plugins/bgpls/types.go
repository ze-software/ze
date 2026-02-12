// Package bgpls implements BGP-LS family types and plugin for ze.
// RFC 7752: North-Bound Distribution of Link-State and TE Information Using BGP
// RFC 9085: BGP-LS Extensions for Segment Routing
// RFC 9514: BGP-LS Extensions for SRv6
package bgpls

import (
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
)

// Type aliases for nlri types used by BGP-LS.
// This allows the bgpls package to be self-contained while reusing nlri definitions.
type (
	Family            = nlri.Family
	AFI               = nlri.AFI
	SAFI              = nlri.SAFI
	BGPLSNLRIType     = nlri.BGPLSNLRIType
	BGPLSProtocolID   = nlri.BGPLSProtocolID
	BGPLSNLRI         = nlri.BGPLSNLRI
	NodeDescriptor    = nlri.NodeDescriptor
	LinkDescriptor    = nlri.LinkDescriptor
	PrefixDescriptor  = nlri.PrefixDescriptor
	SRv6SIDDescriptor = nlri.SRv6SIDDescriptor
	BGPLSNode         = nlri.BGPLSNode
	BGPLSLink         = nlri.BGPLSLink
	BGPLSPrefix       = nlri.BGPLSPrefix
	BGPLSSRv6SID      = nlri.BGPLSSRv6SID
)

// Re-export constants from nlri for local use.
const (
	AFIBGPLS            = nlri.AFIBGPLS
	SAFIBGPLinkState    = nlri.SAFIBGPLinkState
	SAFIBGPLinkStateVPN = nlri.SAFIBGPLinkStateVPN
)

// Re-export NLRI type constants.
const (
	BGPLSNodeNLRI     = nlri.BGPLSNodeNLRI
	BGPLSLinkNLRI     = nlri.BGPLSLinkNLRI
	BGPLSPrefixV4NLRI = nlri.BGPLSPrefixV4NLRI
	BGPLSPrefixV6NLRI = nlri.BGPLSPrefixV6NLRI
	BGPLSSRv6SIDNLRI  = nlri.BGPLSSRv6SIDNLRI
)

// Re-export Protocol ID constants.
const (
	ProtoISISL1  = nlri.ProtoISISL1
	ProtoISISL2  = nlri.ProtoISISL2
	ProtoOSPFv2  = nlri.ProtoOSPFv2
	ProtoDirect  = nlri.ProtoDirect
	ProtoStatic  = nlri.ProtoStatic
	ProtoOSPFv3  = nlri.ProtoOSPFv3
	ProtoBGP     = nlri.ProtoBGP
	ProtoRSVPTE  = nlri.ProtoRSVPTE
	ProtoSegment = nlri.ProtoSegment
)

// Re-export TLV constants.
const (
	TLVLocalNodeDesc      = nlri.TLVLocalNodeDesc
	TLVRemoteNodeDesc     = nlri.TLVRemoteNodeDesc
	TLVAutonomousSystem   = nlri.TLVAutonomousSystem
	TLVBGPLSIdentifier    = nlri.TLVBGPLSIdentifier
	TLVOSPFAreaID         = nlri.TLVOSPFAreaID
	TLVIGPRouterID        = nlri.TLVIGPRouterID
	TLVLinkLocalRemoteID  = nlri.TLVLinkLocalRemoteID
	TLVIPv4InterfaceAddr  = nlri.TLVIPv4InterfaceAddr
	TLVIPv4NeighborAddr   = nlri.TLVIPv4NeighborAddr
	TLVIPv6InterfaceAddr  = nlri.TLVIPv6InterfaceAddr
	TLVIPv6NeighborAddr   = nlri.TLVIPv6NeighborAddr
	TLVMultiTopologyID    = nlri.TLVMultiTopologyID
	TLVOSPFRouteType      = nlri.TLVOSPFRouteType
	TLVIPReachabilityInfo = nlri.TLVIPReachabilityInfo
	TLVSRv6SID            = nlri.TLVSRv6SID
)

// Re-export errors.
var (
	ErrBGPLSTruncated   = nlri.ErrBGPLSTruncated
	ErrBGPLSInvalidType = nlri.ErrBGPLSInvalidType
)

// BGPLSFamily is the address family for BGP-LS.
var BGPLSFamily = nlri.Family{AFI: AFIBGPLS, SAFI: SAFIBGPLinkState}

// Re-export constructor functions.
var (
	NewBGPLSNode     = nlri.NewBGPLSNode
	NewBGPLSLink     = nlri.NewBGPLSLink
	NewBGPLSPrefixV4 = nlri.NewBGPLSPrefixV4
	NewBGPLSPrefixV6 = nlri.NewBGPLSPrefixV6
	NewBGPLSSRv6SID  = nlri.NewBGPLSSRv6SID
	ParseBGPLS       = nlri.ParseBGPLS
)

// ParseBGPLSWithRest parses a single BGP-LS NLRI and returns the remaining data.
// This enables parsing multiple packed NLRIs from MP_REACH/MP_UNREACH.
// RFC 7752 Section 3.2: NLRI format is Type(2) + Length(2) + Value(Length).
func ParseBGPLSWithRest(data []byte) (BGPLSNLRI, []byte, error) {
	// Need at least 4 bytes for Type + Length header
	if len(data) < 4 {
		return nil, nil, ErrBGPLSTruncated
	}

	// RFC 7752 Section 3.2 - Total NLRI Length (bytes 2-3)
	nlriLen := int(data[2])<<8 | int(data[3])
	totalLen := 4 + nlriLen

	if len(data) < totalLen {
		return nil, nil, ErrBGPLSTruncated
	}

	// Parse just this NLRI
	parsed, err := ParseBGPLS(data[:totalLen])
	if err != nil {
		return nil, nil, err
	}

	return parsed, data[totalLen:], nil
}
