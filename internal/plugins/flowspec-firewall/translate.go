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
	errUnknownProtocol      = errors.New("flowspec: IP protocol has no canonical firewall name")
	errUnreadableValue      = errors.New("flowspec: NLRI value cannot be read")
)

// flowAction holds the parsed traffic action from extended communities.
type flowAction struct {
	discard   bool
	rateLimit uint32 // 0 = not set; the unit is rateInPackets
	// rateInPackets distinguishes RFC 8955 Section 7.2 traffic-rate-packets
	// (sub-type 0x800c) from Section 7.1 traffic-rate-bytes (0x8006). Both
	// render as "rate-limit:<n>", and only the ":packets" suffix tells them
	// apart, so dropping the suffix installed a packets-per-second rate as a
	// bytes-per-second limit -- a peer asking for 1000 pkt/s got 1000 byte/s.
	rateInPackets bool
	markDSCP      uint8
	hasMark       bool
}

// translateFlowSpec converts a parsed FlowSpec NLRI and its actions into
// firewall Terms.
//
// A Term ANDs its matches, so every component whose values are ALTERNATIVES
// multiplies the terms rather than adding a match. Type 4 (Port = source OR
// destination) gives two terms, and type 3 (IP protocol) gives one term per
// protocol the peer listed. A rule with neither gives one term.
//
// It returns errUnsupportedComponent for a component ze cannot map,
// errUnknownProtocol for a protocol with no canonical firewall name,
// errUnreadableValue for a value ze cannot read, and errNoAction when the
// route carries no filtering or shaping action. Each refuses the WHOLE route:
// enforcing a rule without one of its narrowing conditions would drop more
// traffic than the peer asked ze to drop.
func translateFlowSpec(fs *flowspec.FlowSpec, act flowAction, nlriKey string) ([]firewall.Term, error) {
	var matches []firewall.Match
	var protoMatches []firewall.MatchProtocol
	var portAnyRanges []firewall.PortRange

	for _, comp := range fs.Components() {
		if comp.Type() == flowspec.FlowPort {
			ranges, err := valuesToPortRanges(comp)
			if err != nil {
				return nil, err
			}
			portAnyRanges = ranges
			continue
		}
		m, err := componentToMatch(comp, fs.Family())
		if err != nil {
			return nil, err
		}
		// A type 3 component lists alternatives, so its matches are held back
		// and expanded into one term each below. Every other component
		// contributes matches that AND together inside one term.
		for _, one := range m {
			if pm, ok := one.(firewall.MatchProtocol); ok {
				protoMatches = append(protoMatches, pm)
				continue
			}
			matches = append(matches, one)
		}
	}

	actions := actionToFirewall(act)
	if len(actions) == 0 {
		return nil, errNoAction
	}

	if len(protoMatches) == 0 {
		return portTerms(nlriKey, matches, portAnyRanges, actions), nil
	}

	// One term per protocol alternative. The key keeps its old spelling when
	// there is a single protocol, so the term names of every rule written
	// before this split are unchanged.
	terms := make([]firewall.Term, 0, len(protoMatches))
	for _, pm := range protoMatches {
		key := nlriKey
		if len(protoMatches) > 1 {
			var tb textbuf.Buffer
			key = tb.Str(nlriKey).Str("|p").Str(pm.Protocol).String()
		}
		withProto := make([]firewall.Match, len(matches), len(matches)+1)
		copy(withProto, matches)
		terms = append(terms, portTerms(key, append(withProto, pm), portAnyRanges, actions)...)
	}
	return terms, nil
}

// portTerms renders one match set as terms under key. A type 4 (Port) component
// matches source OR destination while a Term ANDs its matches, so a port-any
// component becomes two terms rather than one that can never match.
func portTerms(key string, matches []firewall.Match, portAny []firewall.PortRange, actions []firewall.Action) []firewall.Term {
	if len(portAny) == 0 {
		return []firewall.Term{{Name: termName(key), Matches: matches, Actions: actions}}
	}
	// Each term owns its match array: the two differ only in the last element,
	// and a shared backing array would let the second overwrite the first.
	withPort := func(m firewall.Match) []firewall.Match {
		out := make([]firewall.Match, len(matches), len(matches)+1)
		copy(out, matches)
		return append(out, m)
	}
	var tb textbuf.Buffer
	return []firewall.Term{
		{
			Name:    termName(tb.Str(key).Str("|sp").String()),
			Matches: withPort(firewall.MatchSourcePort{Ranges: portAny}),
			Actions: actions,
		},
		{
			Name:    termName(tb.Reset().Str(key).Str("|dp").String()),
			Matches: withPort(firewall.MatchDestinationPort{Ranges: portAny}),
			Actions: actions,
		},
	}
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
		return protocolMatches(extractNumericValues(comp))

	case flowspec.FlowPort:
		// Type 4 (Port = src OR dst) is expanded into two terms by
		// translateFlowSpec, which reads it before this function is called.
		// A destination-only answer here would enforce half of what the peer
		// announced, so a caller that reached this line is refused.
		//
		// Only a Ze defect reaches this line, which would make it a panic
		// site. It stays an error return because this function runs on data a
		// peer sends: an error here is already counted and logged, while a
		// panic would put the daemon one mistaken call away from exiting.
		return nil, fmt.Errorf("%w: port (type 4) is expanded by translateFlowSpec, not matched here", errUnsupportedComponent)

	case flowspec.FlowDestPort:
		ranges, err := valuesToPortRanges(comp)
		if err != nil {
			return nil, err
		}
		return []firewall.Match{firewall.MatchDestinationPort{Ranges: ranges}}, nil

	case flowspec.FlowSourcePort:
		ranges, err := valuesToPortRanges(comp)
		if err != nil {
			return nil, err
		}
		return []firewall.Match{firewall.MatchSourcePort{Ranges: ranges}}, nil

	case flowspec.FlowICMPType:
		// RFC 8955 Section 4.2.2.7 gives the ICMP type field one octet.
		val, err := singleValue(comp, 255)
		if err != nil {
			return nil, err
		}
		return []firewall.Match{firewall.MatchICMPType{Type: uint8(val)}}, nil

	case flowspec.FlowTCPFlags:
		// RFC 8955 Section 4.2.2.9 allows a one or two octet bitmask, and the
		// firewall match holds the eight TCP flag bits.
		val, err := singleValue(comp, 255)
		if err != nil {
			return nil, err
		}
		flags := firewall.TCPFlags(val)
		return []firewall.Match{firewall.MatchTCPFlags{Flags: flags, Mask: flags}}, nil

	case flowspec.FlowDSCP:
		// RFC 8955 Section 4.2.2.11 gives the DSCP field one octet and RFC 2474
		// defines six bits of it, so 63 is the largest value a match can carry.
		val, err := singleValue(comp, 63)
		if err != nil {
			return nil, err
		}
		return []firewall.Match{firewall.MatchDSCP{Value: uint8(val)}}, nil

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
		dimension := firewall.RateDimensionBytes
		if act.rateInPackets {
			dimension = firewall.RateDimensionPackets
		}
		actions = append(actions, firewall.Limit{
			Rate:      uint64(act.rateLimit),
			Unit:      "second",
			Dimension: dimension,
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
			// AppendDecoded renders the packets form as "rate-limit:<n>:packets"
			// (extcomm_decoded.go). parseUint32 stops at the colon, so the number
			// is read the same way either way and only the suffix carries the unit.
			if _, unit, found := strings.Cut(val, ":"); found {
				act.rateInPackets = unit == "packets"
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
	// A refusal here needs no separate report: parseNLRIJSON reads the same
	// fields with the same helper and rejects the NLRI before it is used.
	pfx, err := firstPrefix(n.Destination, n.DestinationV6, isV6)
	if err != nil {
		return netip.Prefix{}
	}
	return pfx
}

func termName(nlriKey string) string {
	h := sha256.Sum256([]byte(nlriKey))
	var tb textbuf.Buffer
	return tb.Str("fs-").Str(hex.EncodeToString(h[:8])).String()
}

// protocolMatches maps the values of a FlowSpec type 3 component to protocol
// matches, keeping the peer's order and dropping repeats. RFC 8955 Section
// 4.2.2.3 gives the field one octet, so every value 0-255 is legal on the wire
// and the translator cannot assume a small set.
//
// A value with no canonical name is refused instead of rendered as digits.
// MatchProtocol carries a name, every backend resolves it through
// firewall.ProtocolNumber, and a spelling no backend knows fails inside
// Backend.Apply -- which returns before its single Flush, so one such rule from
// one peer would leave every other owner's ruleset unapplied.
//
// Dropping repeats bounds the match count by the size of the canonical table,
// so one NLRI cannot expand into an unbounded number of terms.
func protocolMatches(vals []uint64) ([]firewall.Match, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	matches := make([]firewall.Match, 0, len(vals))
	seen := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		if v > 255 {
			return nil, fmt.Errorf("%w: value %d exceeds the one-octet protocol field", errUnsupportedComponent, v)
		}
		name, ok := firewall.ProtocolName(uint8(v))
		if !ok {
			return nil, fmt.Errorf("%w: %d", errUnknownProtocol, v)
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		matches = append(matches, firewall.MatchProtocol{Protocol: name})
	}
	return matches, nil
}

// valuesToPortRanges maps the values of a port component to single-port
// ranges, keeping the peer's order. A port component lists ALTERNATIVES, and
// one MatchSourcePort or MatchDestinationPort carries every one of them, so no
// term expansion is needed here.
//
// Port 0 is translated rather than refused. It is a legal value of the
// two-octet port field (RFC 8955 Section 4.2.2.5), and both backends express
// it: the nft backend lowers a single range to one equality comparison
// (lowerPortMatch, internal/plugins/firewall/nft/lower_linux.go) and the VPP
// backend to the range 0-0 (internal/plugins/firewall/vpp/translate.go).
// Refusing a rule ze can enforce, or dropping the value and enforcing the rule
// without its port condition, are both wrong: the second is the wider one, and
// it turned "drop tcp to 10.1.0.0/24 destination-port =0" into a drop of ALL
// tcp to that prefix.
//
// A component with no value, or a value the field cannot hold, refuses the
// whole rule for the same reason: ze cannot read what the peer asked for, so
// ze must not enforce a looser version of it.
func valuesToPortRanges(comp flowspec.FlowComponent) ([]firewall.PortRange, error) {
	vals := extractNumericValues(comp)
	if len(vals) == 0 {
		return nil, fmt.Errorf("%w: %s carries no value", errUnreadableValue, comp.Type())
	}
	ranges := make([]firewall.PortRange, 0, len(vals))
	for _, v := range vals {
		if v > 65535 {
			return nil, fmt.Errorf("%w: %s value %d exceeds the two-octet port field", errUnreadableValue, comp.Type(), v)
		}
		ranges = append(ranges, firewall.PortRange{Lo: uint16(v), Hi: uint16(v)})
	}
	return ranges, nil
}

// singleValue reads the one value of a component whose firewall match holds
// exactly one, and refuses everything else. valueMax is the largest value that
// match can carry.
//
// A component listing several values means "any of these", and a Term ANDs its
// matches, so alternatives can only be expressed as one term each. Type 3 (IP
// protocol) is expanded that way because the canonical protocol table bounds
// the result. ICMP type, TCP flags and DSCP have no such bound, and they
// multiply with the protocol expansion: 256 ICMP types times 256 protocols is
// 65536 terms from one NLRI, which is the unbounded expansion protocolMatches
// exists to prevent. So the rule is refused, counted and logged instead.
// Enforcing the first value and discarding the rest would enforce a rule the
// peer never announced.
func singleValue(comp flowspec.FlowComponent, valueMax uint64) (uint64, error) {
	vals := extractNumericValues(comp)
	if len(vals) == 0 {
		return 0, fmt.Errorf("%w: %s carries no value", errUnreadableValue, comp.Type())
	}
	if len(vals) > 1 {
		return 0, fmt.Errorf("%w: %s lists %d alternatives and one firewall match holds one value", errUnsupportedComponent, comp.Type(), len(vals))
	}
	if vals[0] > valueMax {
		return 0, fmt.Errorf("%w: %s value %d exceeds %d", errUnsupportedComponent, comp.Type(), vals[0], valueMax)
	}
	return vals[0], nil
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

	if pfx, err := firstPrefix(n.Destination, n.DestinationV6, isV6); err != nil {
		return nil, err
	} else if pfx.IsValid() {
		fs.AddComponent(flowspec.NewFlowDestPrefixComponent(pfx))
	}
	if pfx, err := firstPrefix(n.Source, n.SourceV6, isV6); err != nil {
		return nil, err
	} else if pfx.IsValid() {
		fs.AddComponent(flowspec.NewFlowSourcePrefixComponent(pfx))
	}
	if vals, err := protocolValues(n.Protocol, n.NextHeader, isV6); err != nil {
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
	// firstNumericVals rather than firstUint16Vals: the component holds one
	// octet per value, and every value is carried so componentToMatch sees the
	// list the peer sent rather than its first entry.
	if vals, err := firstNumericVals(n.TCPFlags, nil, false); err != nil {
		return nil, err
	} else if len(vals) > 0 {
		fs.AddComponent(flowspec.NewFlowTCPFlagsComponent(vals...))
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

// Every helper below REFUSES a value it cannot read rather than skipping it.
// A skipped value is a dropped match, and a dropped match makes the rule WIDER
// than the peer announced: "drop tcp to 10.1.0.0/24 port 80" whose prefix and
// protocol were both skipped became an unconditional drop of port 80 to every
// address on every protocol. A rule ze cannot read is a rule ze must not
// enforce a looser version of.

// firstPrefix extracts the first prefix from v4 or v6 groups.
//
// The NLRI JSON writer emits "prefix/length/offset" (json.go,
// formatPrefixWithOffset), so the offset suffix is accepted here. RFC 8956
// Section 3.1 lets an IPv6 component match a bit range starting at a non-zero
// offset; the firewall data model has no match for that, so a non-zero offset
// is refused rather than widened into a whole-prefix match.
func firstPrefix(v4, v6 [][]string, isV6 bool) (netip.Prefix, error) {
	groups := v4
	if isV6 {
		groups = v6
	}
	if len(groups) == 0 || len(groups[0]) == 0 {
		return netip.Prefix{}, nil
	}
	raw := groups[0][0]
	text := raw
	if base, offset, ok := splitPrefixOffset(raw); ok {
		if offset != 0 {
			return netip.Prefix{}, fmt.Errorf("%w: prefix %q matches from bit offset %d, which no firewall backend can express", errUnsupportedComponent, raw, offset)
		}
		text = base
	}
	pfx, err := netip.ParsePrefix(text)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%w: prefix %q", errUnreadableValue, raw)
	}
	return pfx, nil
}

// splitPrefixOffset splits "addr/length/offset" into "addr/length" and the
// offset. It reports false for a plain "addr/length", which carries no offset.
func splitPrefixOffset(s string) (string, uint64, bool) {
	last := strings.LastIndexByte(s, '/')
	if last < 0 || strings.Count(s, "/") < 2 {
		return "", 0, false
	}
	offset, err := parseDigits(s[last+1:])
	if err != nil {
		return "", 0, false
	}
	return s[:last], offset, true
}

// protocolValues resolves the JSON protocol (IPv4) or next-header (IPv6) key.
//
// The writer spells a protocol it has a name for as that name ("=tcp",
// "=sctp") and every other value as digits ("=51"), so both forms are read
// here. A name outside the canonical firewall table is refused: no backend can
// lower it, and the value cannot simply be skipped without widening the rule.
func protocolValues(v4, v6 [][]string, isV6 bool) ([]uint8, error) {
	groups := v4
	if isV6 {
		groups = v6
	}
	var vals []uint8
	for _, group := range groups {
		for _, s := range group {
			token, err := stripEqualityOperator(s)
			if err != nil {
				return nil, err
			}
			if token == "" {
				continue
			}
			if token[0] >= '0' && token[0] <= '9' {
				n, err := parseDigits(token)
				if err != nil {
					return nil, err
				}
				if n > 255 {
					return nil, fmt.Errorf("%w: protocol %d exceeds the one-octet field", errUnreadableValue, n)
				}
				vals = append(vals, uint8(n))
				continue
			}
			num, ok := firewall.ProtocolNumber(token)
			if !ok {
				return nil, fmt.Errorf("%w: %q", errUnknownProtocol, token)
			}
			vals = append(vals, num)
		}
	}
	return vals, nil
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
			if v < 0 {
				continue
			}
			if v > 255 {
				return nil, fmt.Errorf("%w: value %d exceeds one octet", errUnreadableValue, v)
			}
			vals = append(vals, uint8(v))
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
			if v < 0 {
				continue
			}
			if v > 65535 {
				return nil, fmt.Errorf("%w: value %d exceeds two octets", errUnreadableValue, v)
			}
			vals = append(vals, uint16(v))
		}
	}
	return vals, nil
}

// parseEqualityValue parses a FlowSpec numeric string that uses the equality
// operator. It accepts bare numbers ("80") and equality prefixed ("=80"), and
// returns -1 for an empty value, which carries no match.
//
// It returns errUnsupportedOperator for a range operator (">80", ">=1024") and
// errUnreadableValue for anything else, including the flag NAMES the writer
// emits for tcp-flags and fragment ("syn", "is-fragment"). Those names are not
// numbers and ze cannot enforce them yet, so the rule carrying one is refused
// rather than enforced without its narrowing condition.
//
// The result is int64 rather than int so the caller's range check is exact on
// every build. On a 32-bit target (the arm appliance) an int would wrap, and a
// wrapped value can land back inside the range the caller accepts, which turns
// an out-of-range value from a peer into a plausible one.
func parseEqualityValue(s string) (int64, error) {
	token, err := stripEqualityOperator(s)
	if err != nil {
		return -1, err
	}
	if token == "" {
		return -1, nil
	}
	n, err := parseDigits(token)
	if err != nil {
		return -1, err
	}
	return int64(n), nil
}

// stripEqualityOperator removes a leading "=" and refuses every operator ze
// cannot translate into a firewall match.
func stripEqualityOperator(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	switch s[0] {
	case '=':
		return s[1:], nil
	case '>', '<', '!', '&', '|':
		return "", fmt.Errorf("%w: %q", errUnsupportedOperator, s)
	}
	return s, nil
}

// parseDigits reads a decimal value, refusing anything that is not one.
func parseDigits(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("%w: empty value", errUnreadableValue)
	}
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%w: %q is not a number", errUnreadableValue, s)
		}
		if n > 1<<50 {
			return 0, fmt.Errorf("%w: value %q is out of range", errUnreadableValue, s)
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}
