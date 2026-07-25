// Design: docs/architecture/core-design.md -- FlowSpec to firewall translation
// RFC: rfc/short/rfc8955.md -- FlowSpec component types and traffic actions

package flowspecfirewall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errUnsupportedComponent = errors.New("flowspec: unsupported component type")
	errUnsupportedOperator  = errors.New("flowspec: non-equality operator not supported")
	errNoAction             = errors.New("flowspec: no traffic action")
)

// flowAction holds the parsed traffic action from extended communities.
type flowAction struct {
	discard   bool
	rateLimit uint32 // bytes/sec, 0 = not set
	markDSCP  uint8
	hasMark   bool
}

// translateFlowSpec converts a parsed FlowSpec NLRI and its actions into
// firewall Terms. Returns errUnsupportedComponent if any component cannot be
// mapped, or errNoAction if no filtering/shaping action is present.
// FlowSpec Type 4 (Port = src OR dst) produces two terms to avoid incorrect
// AND logic in nftables.
func translateFlowSpec(fs *flowspec.FlowSpec, act flowAction, nlriKey string) ([]firewall.Term, error) {
	var matches []firewall.Match
	var portAnyRanges []firewall.PortRange

	for _, comp := range fs.Components() {
		if comp.Type() == flowspec.FlowPort {
			ranges := valuesToPortRanges(extractNumericValues(comp))
			if len(ranges) > 0 {
				portAnyRanges = ranges
			}
			continue
		}
		m, err := componentToMatch(comp, fs.Family())
		if err != nil {
			return nil, err
		}
		matches = append(matches, m...)
	}

	actions := actionToFirewall(act)
	if len(actions) == 0 {
		return nil, errNoAction
	}

	if len(portAnyRanges) > 0 {
		srcMatches := append(append([]firewall.Match{}, matches...), firewall.MatchSourcePort{Ranges: portAnyRanges})
		dstMatches := append(append([]firewall.Match{}, matches...), firewall.MatchDestinationPort{Ranges: portAnyRanges})
		var tb textbuf.Buffer
		return []firewall.Term{
			{Name: termName(tb.Str(nlriKey).Str("|sp").String()), Matches: srcMatches, Actions: actions},
			{Name: termName(tb.Reset().Str(nlriKey).Str("|dp").String()), Matches: dstMatches, Actions: actions},
		}, nil
	}

	return []firewall.Term{{
		Name:    termName(nlriKey),
		Matches: matches,
		Actions: actions,
	}}, nil
}

// componentToMatch converts a single FlowSpec component to firewall matches.
// RFC 8955 Section 4.2.2: component types 1-13.
func componentToMatch(comp flowspec.FlowComponent, fam family.Family) ([]firewall.Match, error) {
	switch comp.Type() {
	case flowspec.FlowDestPrefix:
		pfx := extractPrefix(comp, fam)
		if !pfx.IsValid() {
			return nil, fmt.Errorf("flowspec: invalid destination prefix")
		}
		return []firewall.Match{firewall.MatchDestinationAddress{Prefix: pfx}}, nil

	case flowspec.FlowSourcePrefix:
		pfx := extractPrefix(comp, fam)
		if !pfx.IsValid() {
			return nil, fmt.Errorf("flowspec: invalid source prefix")
		}
		return []firewall.Match{firewall.MatchSourceAddress{Prefix: pfx}}, nil

	case flowspec.FlowIPProtocol:
		vals := extractNumericValues(comp)
		if len(vals) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchProtocol{Protocol: protoName(uint8(vals[0]))}}, nil

	case flowspec.FlowPort:
		// Type 4 (Port = src OR dst) is handled in translateFlowSpec by
		// splitting into two terms. If called directly, return dst-only
		// as a safe fallback (narrower than the intended OR).
		ranges := valuesToPortRanges(extractNumericValues(comp))
		if len(ranges) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchDestinationPort{Ranges: ranges}}, nil

	case flowspec.FlowDestPort:
		ranges := valuesToPortRanges(extractNumericValues(comp))
		if len(ranges) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchDestinationPort{Ranges: ranges}}, nil

	case flowspec.FlowSourcePort:
		ranges := valuesToPortRanges(extractNumericValues(comp))
		if len(ranges) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchSourcePort{Ranges: ranges}}, nil

	case flowspec.FlowICMPType:
		vals := extractNumericValues(comp)
		if len(vals) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchICMPType{Type: uint8(vals[0])}}, nil

	case flowspec.FlowTCPFlags:
		vals := extractNumericValues(comp)
		if len(vals) == 0 {
			return nil, nil
		}
		if vals[0] > 255 {
			return nil, fmt.Errorf("%w: tcp-flags value %d exceeds uint8", errUnsupportedComponent, vals[0])
		}
		flags := firewall.TCPFlags(vals[0])
		return []firewall.Match{firewall.MatchTCPFlags{Flags: flags, Mask: flags}}, nil

	case flowspec.FlowDSCP:
		vals := extractNumericValues(comp)
		if len(vals) == 0 {
			return nil, nil
		}
		return []firewall.Match{firewall.MatchDSCP{Value: uint8(vals[0])}}, nil

	case flowspec.FlowICMPCode, flowspec.FlowPacketLength,
		flowspec.FlowFragment, flowspec.FlowFlowLabel:
		return nil, fmt.Errorf("%w: %s", errUnsupportedComponent, comp.Type())

	default:
		return nil, fmt.Errorf("%w: unknown type %d", errUnsupportedComponent, comp.Type())
	}
}

// actionToFirewall converts parsed traffic actions to firewall actions.
// RFC 8955 Section 7: traffic filtering actions.
func actionToFirewall(act flowAction) []firewall.Action {
	if act.discard {
		return []firewall.Action{firewall.Drop{}}
	}

	var actions []firewall.Action

	if act.rateLimit > 0 {
		actions = append(actions, firewall.Limit{
			Rate:      uint64(act.rateLimit),
			Unit:      "second",
			Dimension: firewall.RateDimensionBytes,
		})
	}

	if act.hasMark {
		actions = append(actions, firewall.SetDSCP{Value: act.markDSCP})
	}

	if len(actions) > 0 {
		actions = append(actions, firewall.Accept{})
	}

	return actions
}

// parseExtendedCommunities extracts traffic actions from string-encoded
// extended communities in the BGP event.
func parseExtendedCommunities(extComms []string) flowAction {
	var act flowAction
	for _, ec := range extComms {
		switch {
		case ec == "rate-limit:0" || strings.HasPrefix(ec, "rate-limit:0 "):
			act.discard = true
		case strings.HasPrefix(ec, "rate-limit:"):
			val := ec[len("rate-limit:"):]
			if val == "" {
				continue
			}
			act.rateLimit = parseUint32(val)
			if act.rateLimit == 0 {
				act.discard = true
			}
		case strings.HasPrefix(ec, "mark:"):
			val := ec[len("mark:"):]
			if val == "" {
				continue
			}
			v := parseUint32(val)
			if v <= 63 {
				act.markDSCP = uint8(v)
				act.hasMark = true
			}
		}
	}
	return act
}

const maxUint32Safe = 429496729 // largest value where n*10+9 <= math.MaxUint32

func parseUint32(s string) uint32 {
	var n uint32
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return n
		}
		digit := uint32(c - '0')
		if n > maxUint32Safe || (n == maxUint32Safe && digit > 5) {
			return 0xFFFFFFFF
		}
		n = n*10 + digit
	}
	return n
}

// destPrefixFromJSON extracts the destination prefix directly from the NLRI JSON,
// avoiding a wire-byte round-trip through FlowSpec component constructors.
func destPrefixFromJSON(fam family.Family, data json.RawMessage) netip.Prefix {
	var n nlriJSON
	if json.Unmarshal(data, &n) != nil {
		return netip.Prefix{}
	}
	isV6 := fam.AFI == family.AFIIPv6
	return firstPrefix(n.Destination, n.DestinationV6, isV6)
}

func termName(nlriKey string) string {
	h := sha256.Sum256([]byte(nlriKey))
	var tb textbuf.Buffer
	return tb.Str("fs-").Str(hex.EncodeToString(h[:8])).String()
}

func protoName(proto uint8) string {
	switch proto {
	case 1:
		return "icmp"
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 47:
		return "gre"
	case 58:
		return "icmpv6"
	default:
		var tb textbuf.Buffer
		return tb.Reset().Uint8(proto).String()
	}
}

func valuesToPortRanges(vals []uint64) []firewall.PortRange {
	ranges := make([]firewall.PortRange, 0, len(vals))
	for _, v := range vals {
		if v > 0 && v <= 65535 {
			ranges = append(ranges, firewall.PortRange{Lo: uint16(v), Hi: uint16(v)})
		}
	}
	return ranges
}

// extractPrefix parses a prefix component's wire bytes into netip.Prefix.
func extractPrefix(comp flowspec.FlowComponent, fam family.Family) netip.Prefix {
	data := comp.Bytes()
	if len(data) < 2 {
		return netip.Prefix{}
	}
	pfxLen := int(data[1])
	pfxBytes := (pfxLen + 7) / 8
	if len(data) < 2+pfxBytes {
		return netip.Prefix{}
	}

	if fam.AFI == family.AFIIPv4 {
		var b [4]byte
		copy(b[:], data[2:2+pfxBytes])
		return netip.PrefixFrom(netip.AddrFrom4(b), pfxLen)
	}
	var b [16]byte
	off := 2
	if fam.AFI == family.AFIIPv6 {
		// RFC 8956: IPv6 has offset byte after prefix length
		if len(data) < 3 {
			return netip.Prefix{}
		}
		off = 3
		if len(data) < off+pfxBytes {
			return netip.Prefix{}
		}
	}
	copy(b[:], data[off:off+pfxBytes])
	return netip.PrefixFrom(netip.AddrFrom16(b), pfxLen)
}

// extractNumericValues returns the match values from a numeric component.
func extractNumericValues(comp flowspec.FlowComponent) []uint64 {
	data := comp.Bytes()
	if len(data) < 2 {
		return nil
	}

	var vals []uint64
	pos := 1 // skip type byte
	for pos < len(data) {
		op := data[pos]
		pos++
		valueLen := 1 << ((op >> 4) & 0x03)
		if pos+valueLen > len(data) {
			break
		}
		var val uint64
		for i := range valueLen {
			val = val<<8 | uint64(data[pos+i])
		}
		vals = append(vals, val)
		pos += valueLen
		if op&0x80 != 0 {
			break // end-of-list
		}
	}
	return vals
}

// nlriJSON is the JSON representation of a FlowSpec NLRI from the BGP event.
// Keys match the flowspec plugin's JSON output (json.go flowSpecKey).
type nlriJSON struct {
	Destination   [][]string `json:"destination"`
	DestinationV6 [][]string `json:"destination-ipv6"`
	Source        [][]string `json:"source"`
	SourceV6      [][]string `json:"source-ipv6"`
	Protocol      [][]string `json:"protocol"`
	NextHeader    [][]string `json:"next-header"`
	Port          [][]string `json:"port"`
	DestPort      [][]string `json:"destination-port"`
	SourcePort    [][]string `json:"source-port"`
	ICMPType      [][]string `json:"icmp-type"`
	ICMPCode      [][]string `json:"icmp-code"`
	TCPFlags      [][]string `json:"tcp-flags"`
	PacketLength  [][]string `json:"packet-length"`
	DSCP          [][]string `json:"dscp"`
	Fragment      [][]string `json:"fragment"`
	FlowLabel     [][]string `json:"flow-label"`
}

// parseNLRIJSON builds a FlowSpec from JSON NLRI component fields.
func parseNLRIJSON(fam family.Family, data json.RawMessage) (*flowspec.FlowSpec, error) {
	var n nlriJSON
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("flowspec NLRI JSON: %w", err)
	}

	fs := flowspec.NewFlowSpec(fam)
	isV6 := fam.AFI == family.AFIIPv6

	if pfx := firstPrefix(n.Destination, n.DestinationV6, isV6); pfx.IsValid() {
		fs.AddComponent(flowspec.NewFlowDestPrefixComponent(pfx))
	}
	if pfx := firstPrefix(n.Source, n.SourceV6, isV6); pfx.IsValid() {
		fs.AddComponent(flowspec.NewFlowSourcePrefixComponent(pfx))
	}
	if vals, err := firstNumericVals(n.Protocol, n.NextHeader, isV6); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowIPProtocolComponent(vals...))
	}
	if vals, err := firstUint16Vals(n.Port); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowPortComponent(vals...))
	}
	if vals, err := firstUint16Vals(n.DestPort); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowDestPortComponent(vals...))
	}
	if vals, err := firstUint16Vals(n.SourcePort); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowSourcePortComponent(vals...))
	}
	if vals, err := firstNumericVals(n.ICMPType, nil, false); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowICMPTypeComponent(vals...))
	}
	if len(n.ICMPCode) > 0 {
		return nil, fmt.Errorf("%w: icmp-code", errUnsupportedComponent)
	}
	if vals, err := firstUint16Vals(n.TCPFlags); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowTCPFlagsComponent(uint8(vals[0])))
	}
	if len(n.PacketLength) > 0 {
		return nil, fmt.Errorf("%w: packet-length", errUnsupportedComponent)
	}
	if vals, err := firstNumericVals(n.DSCP, nil, false); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowDSCPComponent(vals...))
	}
	if len(n.Fragment) > 0 {
		return nil, fmt.Errorf("%w: fragment", errUnsupportedComponent)
	}
	if len(n.FlowLabel) > 0 {
		return nil, fmt.Errorf("%w: flow-label", errUnsupportedComponent)
	}

	return fs, nil
}

// firstPrefix extracts the first prefix from v4 or v6 groups.
func firstPrefix(v4, v6 [][]string, isV6 bool) netip.Prefix {
	groups := v4
	if isV6 {
		groups = v6
	}
	for _, group := range groups {
		for _, s := range group {
			pfx, err := netip.ParsePrefix(s)
			if err == nil {
				return pfx
			}
		}
	}
	return netip.Prefix{}
}

// firstNumericVals extracts uint8 values from OR-groups of equality-only numeric strings.
// Returns errUnsupportedOperator if any non-equality operator (>, <, >=, <=, !=) is found.
func firstNumericVals(v4, v6 [][]string, isV6 bool) ([]uint8, error) {
	groups := v4
	if isV6 {
		groups = v6
	}
	var vals []uint8
	for _, group := range groups {
		for _, s := range group {
			v, err := parseEqualityValue(s)
			if err != nil {
				return nil, err
			}
			if v >= 0 && v <= 255 {
				vals = append(vals, uint8(v))
			}
		}
	}
	return vals, nil
}

// firstUint16Vals extracts uint16 values from OR-groups of equality-only numeric strings.
// Returns errUnsupportedOperator if any non-equality operator is found.
func firstUint16Vals(groups [][]string) ([]uint16, error) {
	var vals []uint16
	for _, group := range groups {
		for _, s := range group {
			v, err := parseEqualityValue(s)
			if err != nil {
				return nil, err
			}
			if v >= 0 && v <= 65535 {
				vals = append(vals, uint16(v))
			}
		}
	}
	return vals, nil
}

// parseEqualityValue parses a FlowSpec numeric string that uses the equality operator.
// Accepts bare numbers ("80") and equality prefixed ("=80").
// Returns errUnsupportedOperator for range operators (">80", ">=1024", etc.).
func parseEqualityValue(s string) (int, error) {
	if s == "" {
		return -1, nil
	}
	start := 0
	switch s[0] {
	case '=':
		start = 1
	case '>', '<', '!':
		return -1, fmt.Errorf("%w: %q", errUnsupportedOperator, s)
	}
	if start >= len(s) {
		return -1, nil
	}
	var n int
	for _, c := range s[start:] {
		if c < '0' || c > '9' {
			return -1, nil
		}
		if n > 1<<50 {
			return -1, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
