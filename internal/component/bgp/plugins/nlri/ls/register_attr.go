// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV registration
// Overview: attr.go — TLV interface and registry
package ls

// Register all BGP-LS attribute TLV decoders.
// Each TLV type maps its code to a decoder function defined in attr_node.go,
// attr_link.go, and attr_prefix.go.
func init() {
	// Node attribute TLVs (RFC 7752 Section 3.3.1)
	RegisterLsAttrTLV(TLVNodeFlagBits, decodeNodeFlagBits)
	RegisterLsAttrTLV(TLVOpaqueNodeAttr, decodeOpaqueNodeAttr)
	RegisterLsAttrTLV(TLVNodeName, decodeNodeName)
	RegisterLsAttrTLV(TLVISISAreaID, decodeISISAreaID)
	RegisterLsAttrTLV(TLVIPv4RouterIDLocal, decodeIPv4RouterIDLocal)
	RegisterLsAttrTLV(TLVIPv6RouterIDLocal, decodeIPv6RouterIDLocal)

	// Link attribute TLVs (RFC 7752 Section 3.3.2)
	RegisterLsAttrTLV(TLVIPv4RouterIDRemote, decodeIPv4RouterIDRemote)
	RegisterLsAttrTLV(TLVIPv6RouterIDRemote, decodeIPv6RouterIDRemote)
	RegisterLsAttrTLV(TLVAdminGroup, decodeAdminGroup)
	RegisterLsAttrTLV(TLVMaxLinkBandwidth, decodeMaxLinkBandwidth)
	RegisterLsAttrTLV(TLVMaxReservableBW, decodeMaxReservableBW)
	RegisterLsAttrTLV(TLVUnreservedBW, decodeUnreservedBW)
	RegisterLsAttrTLV(TLVTEDefaultMetric, decodeTEDefaultMetric)
	RegisterLsAttrTLV(TLVIGPMetric, decodeIGPMetric)
	RegisterLsAttrTLV(TLVSRLG, decodeSRLG)
	RegisterLsAttrTLV(TLVOpaqueLinkAttr, decodeOpaqueLinkAttr)
	RegisterLsAttrTLV(TLVLinkName, decodeLinkName)

	// Prefix attribute TLVs (RFC 7752 Section 3.3.3)
	RegisterLsAttrTLV(TLVIGPFlags, decodeIGPFlags)
	RegisterLsAttrTLV(TLVPrefixMetric, decodePrefixMetric)
	RegisterLsAttrTLV(TLVOpaquePrefixAttr, decodeOpaquePrefixAttr)

	// SR-MPLS node attribute TLVs (RFC 9085)
	RegisterLsAttrTLV(TLVSRCapabilities, decodeSRCapabilities)
	RegisterLsAttrTLV(TLVSRAlgorithm, decodeSRAlgorithm)
	RegisterLsAttrTLV(TLVSRLocalBlock, decodeSRLocalBlock)

	// SR-MPLS link attribute TLVs (RFC 9085)
	RegisterLsAttrTLV(TLVAdjacencySID, decodeAdjacencySID)

	// SR-MPLS prefix attribute TLVs (RFC 9085)
	RegisterLsAttrTLV(TLVPrefixSID, decodePrefixSID)
	RegisterLsAttrTLV(TLVSIDLabel, decodeSIDLabel)
	RegisterLsAttrTLV(TLVSRPrefixFlags, decodeSRPrefixFlags)
	RegisterLsAttrTLV(TLVSourceRouterID, decodeSourceRouterID)

	// BGP-EPE Peer SIDs (RFC 9086 Section 5)
	RegisterLsAttrTLV(TLVPeerNodeSID, decodePeerSID(TLVPeerNodeSID))
	RegisterLsAttrTLV(TLVPeerAdjSID, decodePeerSID(TLVPeerAdjSID))
	RegisterLsAttrTLV(TLVPeerSetSID, decodePeerSID(TLVPeerSetSID))

	// SRv6 End.X SID (RFC 9514 Section 4)
	RegisterLsAttrTLV(TLVSRv6EndXSID, decodeSRv6EndXSID(TLVSRv6EndXSID, 0))
	RegisterLsAttrTLV(TLVSRv6LANEndXISIS, decodeSRv6EndXSID(TLVSRv6LANEndXISIS, 6))
	RegisterLsAttrTLV(TLVSRv6LANEndXOSPF, decodeSRv6EndXSID(TLVSRv6LANEndXOSPF, 4))

	// Delay metrics (RFC 8571)
	RegisterLsAttrTLV(TLVUnidirectionalDelay, decodeUnidirectionalDelay)
	RegisterLsAttrTLV(TLVMinMaxDelay, decodeMinMaxDelay)
	RegisterLsAttrTLV(TLVDelayVariation, decodeDelayVariation)

	// SRv6 attribute TLVs (RFC 9514)
	RegisterLsAttrTLV(TLVSRv6EndpointBehavior, decodeSRv6EndpointBehavior)
	RegisterLsAttrTLV(TLVSRv6BGPPeerNodeSID, decodeSRv6BGPPeerNodeSID)
	RegisterLsAttrTLV(TLVSRv6SIDStructure, decodeSRv6SIDStructure)
}
