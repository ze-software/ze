// Design: docs/architecture/wire/nlri-bgpls.md — BGP-LS attribute TLV registration
// Overview: attr.go — TLV interface and registry
package ls

// Register all BGP-LS attribute TLV decoders.
// Each TLV type maps its code to a decoder function defined in attr_node.go,
// attr_link.go, and attr_prefix.go.
func init() {
	// Node attribute TLVs (RFC 7752 Section 3.3.1)
	registerLsAttrTLV(TLVNodeFlagBits, decodeNodeFlagBits)
	registerLsAttrTLV(TLVOpaqueNodeAttr, decodeOpaqueNodeAttr)
	registerLsAttrTLV(TLVNodeName, decodeNodeName)
	registerLsAttrTLV(TLVISISAreaID, decodeISISAreaID)
	registerLsAttrTLV(TLVIPv4RouterIDLocal, decodeIPv4RouterIDLocal)
	registerLsAttrTLV(TLVIPv6RouterIDLocal, decodeIPv6RouterIDLocal)

	// Link attribute TLVs (RFC 7752 Section 3.3.2)
	registerLsAttrTLV(TLVIPv4RouterIDRemote, decodeIPv4RouterIDRemote)
	registerLsAttrTLV(TLVIPv6RouterIDRemote, decodeIPv6RouterIDRemote)
	registerLsAttrTLV(TLVAdminGroup, decodeAdminGroup)
	registerLsAttrTLV(TLVMaxLinkBandwidth, decodeMaxLinkBandwidth)
	registerLsAttrTLV(TLVMaxReservableBW, decodeMaxReservableBW)
	registerLsAttrTLV(TLVUnreservedBW, decodeUnreservedBW)
	registerLsAttrTLV(TLVTEDefaultMetric, decodeTEDefaultMetric)
	registerLsAttrTLV(TLVIGPMetric, decodeIGPMetric)
	registerLsAttrTLV(TLVSRLG, decodeSRLG)
	registerLsAttrTLV(TLVOpaqueLinkAttr, decodeOpaqueLinkAttr)
	registerLsAttrTLV(TLVLinkName, decodeLinkName)

	// Prefix attribute TLVs (RFC 7752 Section 3.3.3)
	registerLsAttrTLV(TLVIGPFlags, decodeIGPFlags)
	registerLsAttrTLV(TLVPrefixMetric, decodePrefixMetric)
	registerLsAttrTLV(TLVOpaquePrefixAttr, decodeOpaquePrefixAttr)

	// SR-MPLS node attribute TLVs (RFC 9085)
	registerLsAttrTLV(TLVSRCapabilities, decodeSRCapabilities)
	registerLsAttrTLV(TLVSRAlgorithm, decodeSRAlgorithm)
	registerLsAttrTLV(TLVSRLocalBlock, decodeSRLocalBlock)

	// SR-MPLS link attribute TLVs (RFC 9085)
	registerLsAttrTLV(TLVAdjacencySID, decodeAdjacencySID)

	// SR-MPLS prefix attribute TLVs (RFC 9085)
	registerLsAttrTLV(TLVPrefixSID, decodePrefixSID)
	registerLsAttrTLV(TLVSIDLabel, decodeSIDLabel)
	registerLsAttrTLV(TLVSRPrefixFlags, decodeSRPrefixFlags)
	registerLsAttrTLV(TLVSourceRouterID, decodeSourceRouterID)

	// BGP-EPE Peer SIDs (RFC 9086 Section 5)
	registerLsAttrTLV(TLVPeerNodeSID, decodePeerSID(TLVPeerNodeSID))
	registerLsAttrTLV(TLVPeerAdjSID, decodePeerSID(TLVPeerAdjSID))
	registerLsAttrTLV(TLVPeerSetSID, decodePeerSID(TLVPeerSetSID))

	// SRv6 End.X SID (RFC 9514 Section 4)
	registerLsAttrTLV(TLVSRv6EndXSID, decodeSRv6EndXSID(TLVSRv6EndXSID, 0))
	registerLsAttrTLV(TLVSRv6LANEndXISIS, decodeSRv6EndXSID(TLVSRv6LANEndXISIS, 6))
	registerLsAttrTLV(TLVSRv6LANEndXOSPF, decodeSRv6EndXSID(TLVSRv6LANEndXOSPF, 4))

	// Delay metrics (RFC 8571)
	registerLsAttrTLV(TLVUnidirectionalDelay, decodeUnidirectionalDelay)
	registerLsAttrTLV(TLVMinMaxDelay, decodeMinMaxDelay)
	registerLsAttrTLV(TLVDelayVariation, decodeDelayVariation)

	// SRv6 attribute TLVs (RFC 9514)
	registerLsAttrTLV(TLVSRv6EndpointBehavior, decodeSRv6EndpointBehavior)
	registerLsAttrTLV(TLVSRv6BGPPeerNodeSID, decodeSRv6BGPPeerNodeSID)
	registerLsAttrTLV(TLVSRv6SIDStructure, decodeSRv6SIDStructure)
}
