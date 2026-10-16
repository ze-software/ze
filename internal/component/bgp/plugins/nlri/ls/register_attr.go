// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV registration
// Overview: attr.go — TLV interface and registry
package ls

import "github.com/ze-software/ze/internal/core/bgp/nlri/bgpls"

// Register all BGP-LS attribute TLV decoders.
// Each TLV type maps its code to a decoder function defined in attr_node.go,
// attr_link.go, and attr_prefix.go.
func init() {
	// Node attribute TLVs (RFC 7752 Section 3.3.1)
	registerLsAttrTLV(bgpls.TLVNodeFlagBits, decodeNodeFlagBits)
	registerLsAttrTLV(bgpls.TLVOpaqueNodeAttr, decodeOpaqueNodeAttr)
	registerLsAttrTLV(bgpls.TLVNodeName, decodeNodeName)
	registerLsAttrTLV(bgpls.TLVISISAreaID, decodeISISAreaID)
	registerLsAttrTLV(bgpls.TLVIPv4RouterIDLocal, decodeIPv4RouterIDLocal)
	registerLsAttrTLV(bgpls.TLVIPv6RouterIDLocal, decodeIPv6RouterIDLocal)

	// Link attribute TLVs (RFC 7752 Section 3.3.2)
	registerLsAttrTLV(bgpls.TLVIPv4RouterIDRemote, decodeIPv4RouterIDRemote)
	registerLsAttrTLV(bgpls.TLVIPv6RouterIDRemote, decodeIPv6RouterIDRemote)
	registerLsAttrTLV(bgpls.TLVAdminGroup, decodeAdminGroup)
	registerLsAttrTLV(bgpls.TLVMaxLinkBandwidth, decodeMaxLinkBandwidth)
	registerLsAttrTLV(bgpls.TLVMaxReservableBW, decodeMaxReservableBW)
	registerLsAttrTLV(bgpls.TLVUnreservedBW, decodeUnreservedBW)
	registerLsAttrTLV(bgpls.TLVTEDefaultMetric, decodeTEDefaultMetric)
	registerLsAttrTLV(tlvMPLSProtocolMask, decodeMPLSProtocolMask)
	registerLsAttrTLV(bgpls.TLVIGPMetric, decodeIGPMetric)
	registerLsAttrTLV(bgpls.TLVSRLG, decodeSRLG)
	registerLsAttrTLV(bgpls.TLVOpaqueLinkAttr, decodeOpaqueLinkAttr)
	registerLsAttrTLV(bgpls.TLVLinkName, decodeLinkName)

	// Prefix attribute TLVs (RFC 7752 Section 3.3.3)
	registerLsAttrTLV(bgpls.TLVIGPFlags, decodeIGPFlags)
	registerLsAttrTLV(bgpls.TLVPrefixMetric, decodePrefixMetric)
	registerLsAttrTLV(bgpls.TLVOpaquePrefixAttr, decodeOpaquePrefixAttr)

	// SR-MPLS node attribute TLVs (RFC 9085)
	// The core decoder returns *bgpls.LsSRCapabilities. On error the adapter
	// returns a nil lsAttrTLV, never that typed nil pointer inside a non-nil
	// interface, so a caller's nil check stays true.
	registerLsAttrTLV(bgpls.TLVSRCapabilities, func(data []byte) (lsAttrTLV, error) {
		// RFC 9085 Section 2.1.2
		capabilities, err := bgpls.DecodeSRCapabilities(data)
		if err != nil {
			return nil, err
		}
		return capabilities, nil
	})
	registerLsAttrTLV(bgpls.TLVSRAlgorithm, decodeSRAlgorithm)
	registerLsAttrTLV(bgpls.TLVSRLocalBlock, decodeSRLocalBlock)

	// SR-MPLS link attribute TLVs (RFC 9085)
	registerLsAttrTLV(bgpls.TLVAdjacencySID, decodeAdjacencySID)

	// SR-MPLS prefix attribute TLVs (RFC 9085)
	registerLsAttrTLV(bgpls.TLVPrefixSID, decodePrefixSID)
	registerLsAttrTLV(bgpls.TLVSIDLabel, decodeSIDLabel)
	registerLsAttrTLV(bgpls.TLVSRPrefixFlags, decodeSRPrefixFlags)
	registerLsAttrTLV(bgpls.TLVSourceRouterID, decodeSourceRouterID)

	// BGP-EPE Peer SIDs (RFC 9086 Section 5)
	registerLsAttrTLV(bgpls.TLVPeerNodeSID, decodePeerSID(bgpls.TLVPeerNodeSID))
	registerLsAttrTLV(bgpls.TLVPeerAdjSID, decodePeerSID(bgpls.TLVPeerAdjSID))
	registerLsAttrTLV(bgpls.TLVPeerSetSID, decodePeerSID(bgpls.TLVPeerSetSID))

	// SRv6 End.X SID (RFC 9514 Section 4)
	registerLsAttrTLV(bgpls.TLVSRv6EndXSID, decodeSRv6EndXSID(bgpls.TLVSRv6EndXSID, 0))
	registerLsAttrTLV(bgpls.TLVSRv6LANEndXISIS, decodeSRv6EndXSID(bgpls.TLVSRv6LANEndXISIS, 6))
	registerLsAttrTLV(bgpls.TLVSRv6LANEndXOSPF, decodeSRv6EndXSID(bgpls.TLVSRv6LANEndXOSPF, 4))

	// Delay metrics (RFC 8571)
	registerLsAttrTLV(bgpls.TLVUnidirectionalDelay, decodeUnidirectionalDelay)
	registerLsAttrTLV(bgpls.TLVMinMaxDelay, decodeMinMaxDelay)
	registerLsAttrTLV(bgpls.TLVDelayVariation, decodeDelayVariation)

	// SRv6 attribute TLVs (RFC 9514)
	registerLsAttrTLV(tlvSRv6Capabilities, decodeSRv6Capabilities)
	registerLsAttrTLV(tlvSRv6Locator, decodeSRv6Locator)
	registerLsAttrTLV(bgpls.TLVSRv6EndpointBehavior, decodeSRv6EndpointBehavior)
	registerLsAttrTLV(bgpls.TLVSRv6BGPPeerNodeSID, decodeSRv6BGPPeerNodeSID)
	registerLsAttrTLV(bgpls.TLVSRv6SIDStructure, decodeSRv6SIDStructure)
}
