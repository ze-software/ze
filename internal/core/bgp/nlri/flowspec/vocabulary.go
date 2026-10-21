// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec component keywords and name tables
// RFC: rfc/short/rfc5575.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: format.go — value-to-name rendering, the reverse of these tables

package flowspec

// FlowSpec component keywords.
const (
	KwDestinationIPv4 = "destination-ipv4" // Type 1 (IPv4)
	KwDestinationIPv6 = "destination-ipv6" // Type 1 (IPv6, RFC 8956 Section 3.1)
	KwSourceIPv4      = "source-ipv4"      // Type 2 (IPv4)
	KwSourceIPv6      = "source-ipv6"      // Type 2 (IPv6, RFC 8956 Section 3.2)
	KwProtocol        = "protocol"         // Type 3 (IPv4)
	KwNextHeader      = "next-header"      // Type 3 (IPv6)
	KwPort            = "port"             // Type 4
	KwDestPort        = "destination-port" // Type 5
	KwSourcePort      = "source-port"      // Type 6
	KwICMPType        = "icmp-type"        // Type 7
	KwICMPCode        = "icmp-code"        // Type 8
	KwTCPFlags        = "tcp-flags"        // Type 9
	KwPacketLength    = "packet-length"    // Type 10
	KwDSCP            = "dscp"             // Type 11
	// KwTrafficClass is the ExaBGP spelling of Type 11. RFC 8956 Section 8
	// registers the type under one name for both families -- "IPv4 Name: DSCP"
	// and "IPv6 Name: DSCP" -- so dscp is the word Ze writes and this one is an
	// accepted alias. It is declared here, beside the word it aliases, because
	// the config path took it while the text path refused it for as long as the
	// two paths each held their own copy of the vocabulary.
	KwTrafficClass = "traffic-class" // Type 11, ExaBGP spelling
	KwFragment     = "fragment"      // Type 12
	KwFlowLabel    = "flow-label"    // Type 13 (IPv6 only)
	KwRD           = "rd"            // Route Distinguisher (VPN)
)

// keywordComponentType maps each component keyword to the type it encodes.
// RFC 8955 Section 4.2.2 and RFC 8956 Section 3 number the types. It is the one
// statement of which keyword names which component: the FlowSpec plugin's text
// parser (parseComponentText), the ExaBGP bridge and the ExaBGP migration read
// it through KeywordComponentType rather than listing the keywords again.
var keywordComponentType = map[string]FlowComponentType{
	KwDestinationIPv4: FlowDestPrefix,
	KwDestinationIPv6: FlowDestPrefix,
	KwSourceIPv4:      FlowSourcePrefix,
	KwSourceIPv6:      FlowSourcePrefix,
	KwProtocol:        FlowIPProtocol,
	KwNextHeader:      FlowIPProtocol,
	KwPort:            FlowPort,
	KwDestPort:        FlowDestPort,
	KwSourcePort:      FlowSourcePort,
	KwICMPType:        FlowICMPType,
	KwICMPCode:        FlowICMPCode,
	KwTCPFlags:        FlowTCPFlags,
	KwPacketLength:    FlowPacketLength,
	KwDSCP:            FlowDSCP,
	KwTrafficClass:    FlowDSCP,
	KwFragment:        FlowFragment,
	KwFlowLabel:       FlowFlowLabel,
}

// KeywordComponentType answers the component type a keyword encodes, and false
// for a word that names no component (rd, or an unknown word).
func KeywordComponentType(keyword string) (FlowComponentType, bool) {
	t, ok := keywordComponentType[keyword]
	return t, ok
}

// ProtocolNameToNumber maps protocol names to numbers. It is the ONLY FlowSpec
// protocol vocabulary: protocolNumberToName is derived from it here, and the
// FlowSpec plugin's config parser and text encoder read it through this
// package. Three copies of this table used to exist and they had already
// diverged, so a name one path accepted another path did not.
// IANA Protocol Numbers: https://www.iana.org/assignments/protocol-numbers
var ProtocolNameToNumber = map[string]uint8{
	"icmp":   1,
	"igmp":   2,
	"tcp":    6,
	"udp":    17,
	"gre":    47,
	"esp":    50,
	"ah":     51,
	"icmpv6": 58,
	"ospf":   89,
	"sctp":   132,
}

// TCPFlagNameToValue maps TCP flag names to values. It is the ONLY FlowSpec TCP
// flag vocabulary: the FlowSpec plugin's config parser and text encoder both
// read it, so a name one path accepts is a name every path accepts.
// RFC 8955 Section 4.2.2.9.
var TCPFlagNameToValue = map[string]uint8{
	"fin":    0x01,
	"syn":    0x02,
	"rst":    0x04,
	"reset":  0x04, // alias for rst
	"psh":    0x08,
	"push":   0x08, // alias for psh
	"ack":    0x10,
	"urg":    0x20,
	"urgent": 0x20, // alias for urg
	"ece":    0x40,
	"cwr":    0x80,
}

// FragmentFlagNameToValue maps fragment flag names to values.
// RFC 8955 Section 4.2.2.12.
var FragmentFlagNameToValue = map[string]uint8{
	"dont-fragment":  0x01,
	"is-fragment":    0x02,
	"first-fragment": 0x04,
	"last-fragment":  0x08,
	"df":             0x01, // alias
	"isf":            0x02, // alias
	"ff":             0x04, // alias
	"lf":             0x08, // alias
}
