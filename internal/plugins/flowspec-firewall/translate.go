// Design: docs/architecture/core-design.md -- FlowSpec to firewall translation
// RFC: rfc/short/rfc8955.md -- FlowSpec component types and traffic actions

package flowspecfirewall

import (
	"crypto/sha256"
	"encoding/hex"
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
	errUnsupportedAction    = errors.New("flowspec: traffic filtering action ze cannot perform")
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
	continueRules bool
	sample        bool
	// unperformable holds the first RFC 8955 Section 7 traffic filtering
	// action ze recognized and has no firewall action for, and is empty when
	// every community was either performed or is not a traffic filtering
	// action at all.
	unperformable string
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
// errUnreadableValue for a value ze cannot read, errUnsupportedAction for a
// traffic filtering action ze cannot perform. Each refuses the whole route:
// enforcing a rule without one of its narrowing conditions would drop more
// traffic than the peer asked ze to drop, and enforcing it without one of its
// ACTIONS would permit traffic the peer asked ze to move elsewhere.
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

	var err error
	protoMatches, err = transportProtocols(fs, protoMatches)
	if err != nil {
		return nil, err
	}

	// RFC 8955 Section 7: "Multiple Traffic Filtering Actions defined in this
	// document may be present for a single Flow Specification and SHOULD be
	// applied to the traffic flow [...] If not all of the Traffic Filtering
	// Actions can be applied to a traffic flow, they should be treated as
	// interfering Traffic Filtering Actions". Section 7.7 leaves the choice
	// among interfering actions to the implementation and asks that the
	// behavior be documented, so this is ze's documented choice: refuse the
	// whole route and name the action (docs/guide/flowspec-protected-router.md).
	//
	// The alternative is to enforce the subset ze can perform, which for a
	// route carrying rt-redirect beside a rate limit installs a rule ending in
	// Accept: the traffic the peer asked ze to send to a scrubbing instance
	// then flows to its original destination, and the peer is told the route
	// was accepted. A refusal is counted and logged, so an operator can see it.
	if act.unperformable != "" {
		return nil, fmt.Errorf("%w: %s", errUnsupportedAction, act.unperformable)
	}

	actions := actionToFirewall(act)

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
		if fam.AFI == family.AFIIPv6 {
			// RFC 8956 Section 3.4 -- see rfc/short/rfc8956.md.
			return []firewall.Match{firewall.MatchICMPv6Type{Type: uint8(val)}}, nil
		}
		return []firewall.Match{firewall.MatchICMPType{Type: uint8(val)}}, nil

	case flowspec.FlowTCPFlags:
		match, err := tcpFlagsMatch(comp)
		if err != nil {
			return nil, err
		}
		return []firewall.Match{match}, nil

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
	var actions []firewall.Action
	if act.sample {
		actions = append(actions, firewall.Log{Prefix: "flowspec "})
	}
	if act.discard {
		return append(actions, firewall.Drop{})
	}
	if act.hasMark {
		actions = append(actions, firewall.SetDSCP{Value: act.markDSCP})
	}
	// RFC 8955 Section 7.3: "When this bit is set, the traffic filtering
	// engine will evaluate any subsequent Flow Specifications".
	if !act.continueRules {
		actions = append(actions, firewall.Accept{})
	}
	return actions
}

// parseExtendedCommunities extracts traffic actions from string-encoded
// extended communities in the BGP event.
//
// A community that is a traffic filtering action ze cannot perform is recorded
// in flowAction.unperformable rather than skipped, so translateFlowSpec can
// refuse the route and name it. Skipping it made a redirect indistinguishable
// from a route target, and a route carrying only a redirect was refused as
// "no traffic action", which told the operator the peer had asked for nothing.
func parseExtendedCommunities(extComms []string) flowAction {
	var act flowAction
	rateSeen := false
	for _, ec := range extComms {
		switch {
		case strings.HasPrefix(ec, "rate-limit:"):
			value, unit, _ := strings.Cut(strings.TrimPrefix(ec, "rate-limit:"), ":")
			rate := parseUint32(value)
			packets := unit == "packets"
			if rateSeen {
				if packets != act.rateInPackets {
					act.unperformable = "simultaneous byte and packet rates"
				}
				rate = min(rate, act.rateLimit)
			}
			rateSeen = true
			act.rateInPackets = packets
			act.rateLimit = rate
			if rate == 0 {
				act.discard = true
			}
		case strings.HasPrefix(ec, "traffic-action:"):
			flags := strings.TrimPrefix(ec, "traffic-action:")
			act.continueRules = flags == "terminal" || flags == "sample-terminal"
			act.sample = flags == "sample" || flags == "sample-terminal"
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
		default:
			if act.unperformable == "" && unperformableAction(ec) {
				act.unperformable = ec
			}
		}
	}
	return act
}

// unperformableAction reports whether ec names a traffic filtering action that
// the arms above did not perform.
//
// Two spellings reach it, and both come from AppendDecoded
// (internal/core/bgp/attribute/extcomm_decoded.go), which writes every extended
// community the daemon puts in the event.
//
// The first is a name that renderer gives an action ze has no firewall action
// for: rt-redirect in its three administrator forms (RFC 8955 Section 7.4),
// and the redirect-to-nexthop pair of
// draft-ietf-idr-flowspec-redirect-ip.
//
// Unknown extended communities are retained by BGP and have no local action.
// RFC 9184 assigns 0x80-0x82 to generic transitive communities; membership
// of this range alone does not identify a traffic-filtering action.
//
// A route target, a route origin and every other community a FlowSpec route
// carries render under their own name or under a type outside that range, so
// they are ignored rather than refused.
func unperformableAction(ec string) bool {
	for _, name := range []string{"redirect:", "redirect-to-nexthop ", "copy-to-nexthop "} {
		if strings.HasPrefix(ec, name) {
			return true
		}
	}
	return false
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
		return nil, fmt.Errorf("%w: protocol expression is not an equality list", errUnsupportedOperator)
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

// valuesToPortRanges preserves the numeric operator list, including AND groups,
// inequalities and operands wider than the packet's port field.
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
	return numericPortRanges(comp)
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

// extractPrefix accepts only a whole CIDR prefix. A nonzero IPv6 offset needs
// a partial-address matcher; treating it as a CIDR would select other packets.
func extractPrefix(comp flowspec.FlowComponent, fam family.Family) netip.Prefix {
	prefix, ok := comp.(interface {
		Prefix() netip.Prefix
		Offset() uint8
	})
	if !ok || prefix.Offset() != 0 {
		return netip.Prefix{}
	}
	value := prefix.Prefix()
	if !value.IsValid() || (value.Addr().Is4() != (fam.AFI == family.AFIIPv4)) {
		return netip.Prefix{}
	}
	return value
}

// extractNumericValues returns the match values from a numeric component.
func extractNumericValues(comp flowspec.FlowComponent) []uint64 {
	list, ok := comp.(interface{ Matches() []flowspec.FlowMatch })
	if !ok {
		return nil
	}
	matches := list.Matches()
	values := make([]uint64, 0, len(matches))
	for i, match := range matches {
		if match.Op != flowspec.FlowOpEqual || (i > 0 && match.And) {
			return nil
		}
		values = append(values, match.Value)
	}
	return values
}
