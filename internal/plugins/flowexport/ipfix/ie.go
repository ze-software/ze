// Design: rfc/short/rfc7012.md -- IPFIX Information Element definitions
// Related: template.go -- consumes these IE IDs to build template field specifiers

package ipfix

// IANA IPFIX Information Element IDs (RFC 7012).
const (
	IEOctetDeltaCount          = 1
	IEPacketDeltaCount         = 2
	IEProtocolIdentifier       = 4
	IESourceTransportPort      = 7
	IESourceIPv4Address        = 8
	IEIngressInterface         = 10
	IEDestinationTransportPort = 11
	IEDestinationIPv4Address   = 12
	IEEgressInterface          = 14
	IEBgpSourceAsNumber        = 16
	IEBgpDestinationAsNumber   = 17
	IEBgpNextHopIPv4Address    = 18
	IESourceIPv6Address        = 27
	IEDestinationIPv6Address   = 28
	// IEOctetTotalCount / IEPacketTotalCount are cumulative (monotonic)
	// counters. Interface counter export reports raw kernel counters, so it
	// uses the Total IEs rather than the Delta IEs (1/2) to avoid collectors
	// misinterpreting cumulative values as per-interval deltas (RFC 7012).
	IEOctetTotalCount       = 85
	IEPacketTotalCount      = 86
	IEFlowStartSeconds      = 150
	IEFlowEndSeconds        = 151
	IEFlowStartMilliseconds = 152
	IEFlowEndMilliseconds   = 153
	IEInterfaceName         = 82
)
