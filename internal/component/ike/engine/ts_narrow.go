// Design: docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md -- RFC 7296 Section 2.9 traffic-selector narrowing
// Overview: child.go -- the Child SA lifecycle that installs the narrowed selectors
// Related: initiator.go -- proposeChildTSPayloads and the wildcard fallback
// Related: responder_eap.go -- the second responder producer, which narrows identically
// Related: responder.go -- buildAuthResponse, the responder producer that narrows
// Related: rekey.go -- respondChildRekey, which narrows against the scope in use
// RFC: rfc/short/rfc7296.md -- Traffic Selector negotiation (Section 2.9, Section 2.9.2)
// RFC: rfc/short/rfc7296.md -- Traffic Selector payload encoding (Section 3.13.1)

package engine

import (
	"errors"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errTSUnacceptable reports a Child SA request whose traffic selectors narrow to the
// empty set. notifyForRefusal maps it to TS_UNACCEPTABLE.
//
// RFC 7296 Section 2.9 states the rule this reports: "If the responder's policy does not
// allow it to accept any part of the proposed Traffic Selectors, it responds with a
// TS_UNACCEPTABLE Notify message" (rfc/full/rfc7296.txt:2426-2428).
var errTSUnacceptable = errors.New("ike: no acceptable traffic selector")

// tsSelector is one traffic selector in Ze's internal form.
//
// The address is a PREFIX rather than the RFC's arbitrary inclusive range, and that is
// deliberate. The XFRM policy selector carries an address plus a prefix length, so a
// range that is not prefix-expressible has no exact representation in the dataplane.
// largestPrefixIn therefore reduces such a range to the largest prefix INSIDE it, which
// RFC 7296 Section 2.9 permits because narrowing is always allowed. Rounding outward to
// the enclosing prefix would answer with traffic the peer never proposed.
type tsSelector struct {
	Net   *net.IPNet
	Port  ipsec.PortSelector
	Proto uint8
}

// tsPair is one narrowed TSi/TSr pair. RFC 7296 negotiates the two lists together, and
// the dataplane installs one policy pair per entry.
type tsPair struct {
	I tsSelector
	R tsSelector
}

// narrowSelectors implements the four-bullet narrowing procedure of RFC 7296 Section 2.9
// and the two rekey constraints of Section 2.9.2.
//
// proposedI and proposedR are the peer's TSi and TSr, and ORDER IS SIGNIFICANT: entry
// zero of each is the initiator's first choice.
//
// policy is the operator's configured selector list in TSi/TSr orientation. An EMPTY
// policy means "allow everything": the proposal is returned unchanged. That default is
// load-bearing. Every configuration written before the traffic-selector list existed has
// no entries, and narrowing those to the empty set would answer each of them with
// TS_UNACCEPTABLE.
//
// floor is the selector set of the SA being rekeyed, or nil for a fresh Child SA. When it
// is set, the floor is answered whenever the peer's proposal still covers it:
//
//	Section 2.9.2 MUST NOT: "Thus, the new SA MUST NOT have narrower selectors than the
//	original."
//	Section 2.9.2 MUST NOT: "The responder MUST NOT narrow down the Traffic Selectors
//	narrower than the scope currently in use."
//
// A proposal that covers no floor pair falls through to the two branches below, and both
// answer a set derived from the proposal alone. That set is NARROWER than the floor, and
// this function does not refuse it. narrowChildSelectors does, and it is the only caller.
// The refusal lives there because the transport-mode filter runs there and can drop a
// pair as well.
//
// The second result is false when nothing is acceptable, and the caller then answers
// TS_UNACCEPTABLE. Every returned selector is a subset of some proposed selector, so the
// answer can never be wider than the proposal.
func narrowSelectors(proposedI, proposedR []tsSelector, policy, floor []tsPair) ([]tsPair, bool) {
	if len(proposedI) == 0 || len(proposedR) == 0 {
		return nil, false
	}

	// RFC 7296 Section 2.9.2: a rekey may not shrink the scope in use. The floor is
	// therefore answered directly whenever the peer's proposal still covers it, which
	// keeps the new SA carrying the old scope even when policy has since narrowed.
	if len(floor) > 0 {
		if kept := floorWithinProposal(floor, proposedI, proposedR); len(kept) > 0 {
			return kept, true
		}
	}

	// An unconfigured peer allows everything, so the proposal is its own narrowing.
	// This is Section 2.9's second bullet: the policy allows the entire proposed set.
	if len(policy) == 0 {
		out := make([]tsPair, 0, len(proposedI))
		for i := range proposedI {
			r := proposedR[min(i, len(proposedR)-1)]
			pair, ok := programmablePair(tsPair{I: proposedI[i], R: r})
			if !ok {
				continue
			}
			out = append(out, pair)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}

	// Bullets 1, 3 and 4. Acceptable intersections are collected in proposal order, and
	// the intersections covering the initiator's FIRST choices are kept apart so they can
	// lead the answer.
	var rest []tsPair
	var firstChoice []tsPair
	for i := range proposedI {
		for r := range proposedR {
			for _, p := range policy {
				pair, ok := intersectPair(tsPair{I: proposedI[i], R: proposedR[r]}, p)
				if !ok {
					continue
				}
				pair, ok = programmablePair(pair)
				if !ok {
					continue
				}
				if i == 0 && r == 0 {
					firstChoice = append(firstChoice, pair)
					continue
				}
				rest = append(rest, pair)
			}
		}
	}

	// RFC 7296 Section 2.9 MUST: "If the responder's policy allows it to accept the
	// first selector of TSi and TSr, then the responder MUST narrow the Traffic
	// Selectors to a subset that includes the initiator's first choices." Leading the
	// answer with the first-choice intersections is what satisfies it, and it is why the
	// two slices are collected apart rather than appended in one pass.
	out := make([]tsPair, 0, len(firstChoice)+len(rest))
	out = append(out, firstChoice...)
	out = append(out, rest...)
	if len(out) == 0 {
		return nil, false
	}
	return dedupePairs(out), true
}

// floorWithinProposal keeps the rekey floor when the peer's proposal still covers it.
//
// RFC 7296 Section 2.9.2 forbids narrowing below the scope in use, and it does not
// license widening beyond the proposal. When the proposal no longer covers the floor,
// the floor cannot be answered and the caller falls through to ordinary narrowing.
func floorWithinProposal(floor []tsPair, proposedI, proposedR []tsSelector) []tsPair {
	kept := make([]tsPair, 0, len(floor))
	for _, f := range floor {
		if coveredBy(f.I, proposedI) && coveredBy(f.R, proposedR) {
			kept = append(kept, f)
		}
	}
	return kept
}

// coversFloor reports whether every pair of the scope in use sits inside some answered
// pair.
//
// RFC 7296 Section 2.9.2 MUST NOT: "The responder MUST NOT narrow down the Traffic
// Selectors narrower than the scope currently in use" (rfc/full/rfc7296.txt:2551-2552).
// A floor pair that no answered pair covers is that narrowing, so it is a refusal rather
// than an answer.
//
// Both arguments are in the SAME orientation. respondChildRekey swaps the stored floor
// into the exchange's orientation before it narrows, and the answer is already in that
// orientation. Comparing the two across orientations reports no cover for every pair
// whose halves differ, which refuses every rekey.
func coversFloor(answer, floor []tsPair) bool {
	for _, f := range floor {
		if !pairWithinAny(f, answer) {
			return false
		}
	}
	return true
}

// pairsText renders a selector set for the operator who reads the refusal.
//
// The refused rekey logs both sets (inbound.go), and the two are only actionable when an
// operator can see which prefix pair the peer dropped.
func pairsText(pairs []tsPair) string {
	var text textbuf.Buffer
	for i, p := range pairs {
		if i > 0 {
			text.Str(", ")
		}
		text.Str(p.I.Net.String()).Str(" <-> ").Str(p.R.Net.String())
	}
	return string(text.Bytes())
}

// coveredBy reports whether some selector in list is a superset of s.
func coveredBy(s tsSelector, list []tsSelector) bool {
	for _, c := range list {
		if containsNet(c.Net, s.Net) {
			return true
		}
	}
	return false
}

// intersectPair intersects a proposed pair with one policy pair.
func intersectPair(proposed, policy tsPair) (tsPair, bool) {
	i, ok := intersectSelector(proposed.I, policy.I)
	if !ok {
		return tsPair{}, false
	}
	r, ok := intersectSelector(proposed.R, policy.R)
	if !ok {
		return tsPair{}, false
	}
	return tsPair{I: i, R: r}, true
}

// intersectSelector intersects one proposed selector with one policy selector. The
// result is a subset of BOTH, so it can never widen the proposal.
func intersectSelector(proposed, policy tsSelector) (tsSelector, bool) {
	n, ok := intersectNet(proposed.Net, policy.Net)
	if !ok {
		return tsSelector{}, false
	}
	port, ok := intersectPort(proposed.Port, policy.Port)
	if !ok {
		return tsSelector{}, false
	}
	proto, ok := intersectProto(proposed.Proto, policy.Proto)
	if !ok {
		return tsSelector{}, false
	}
	return tsSelector{Net: n, Port: port, Proto: proto}, true
}

// intersectNet returns the narrower of two prefixes when one contains the other, and
// reports false when they are disjoint. Two prefixes never partly overlap, so the
// containment test is the whole intersection.
func intersectNet(a, b *net.IPNet) (*net.IPNet, bool) {
	switch {
	case containsNet(a, b):
		return b, true
	case containsNet(b, a):
		return a, true
	default:
		return nil, false
	}
}

// containsNet reports whether outer covers every address of inner.
func containsNet(outer, inner *net.IPNet) bool {
	if outer == nil || inner == nil {
		return false
	}
	outerOnes, outerBits := outer.Mask.Size()
	innerOnes, innerBits := inner.Mask.Size()
	if outerBits != innerBits || outerBits == 0 {
		return false
	}
	return outerOnes <= innerOnes && outer.Contains(inner.IP)
}

// intersectPort intersects two port forms.
//
// RFC 7296 Section 3.13.1: "according to [IPSECARCH], 'ANY' includes 'OPAQUE'", so ANY
// intersected with OPAQUE is OPAQUE rather than the empty set. A single port and OPAQUE
// are disjoint, because an opaque port is by definition not a specific one.
func intersectPort(a, b ipsec.PortSelector) (ipsec.PortSelector, bool) {
	switch {
	case a.Form == ipsec.PortAny:
		return b, true
	case b.Form == ipsec.PortAny:
		return a, true
	case a.Form == ipsec.PortOpaque && b.Form == ipsec.PortOpaque:
		return a, true
	case a.Form == ipsec.PortSingle && b.Form == ipsec.PortSingle && a.Port == b.Port:
		return a, true
	default:
		return ipsec.PortSelector{}, false
	}
}

// intersectProto intersects two IP protocol selectors. RFC 7296 Section 3.13.1 gives
// protocol 0 the meaning "any protocol", so it is the identity here.
func intersectProto(a, b uint8) (uint8, bool) {
	switch {
	case a == 0:
		return b, true
	case b == 0:
		return a, true
	case a == b:
		return a, true
	default:
		return 0, false
	}
}

// programmablePair keeps a narrowed pair only when BOTH halves can be programmed
// exactly.
//
// ai/rules/protocol.md, applied at negotiation time rather than at commit time.
// The operator's policy passes ipsec.ValidateTrafficSelectors, but the PEER'S PROPOSAL
// never does: it is attacker-controlled and arrives long after commit. A proposal Ze
// answers but cannot program would put one set of selectors on the wire and a different
// set in the kernel, which is the defect this package exists to remove.
func programmablePair(p tsPair) (tsPair, bool) {
	i, ok := programmableSelector(p.I)
	if !ok {
		return tsPair{}, false
	}
	r, ok := programmableSelector(p.R)
	if !ok {
		return tsPair{}, false
	}
	return tsPair{I: i, R: r}, true
}

func programmableSelector(s tsSelector) (tsSelector, bool) {
	if s.Net == nil {
		return tsSelector{}, false
	}
	ones, bits := s.Net.Mask.Size()
	if bits == 0 && ones == 0 {
		return tsSelector{}, false
	}
	// RFC 7296 Section 3.13.1 requires the port fields to be 0 and 65535 when the
	// protocol defines no port. A specific or opaque port under protocol 0 could not be
	// encoded conformantly, so it is narrowed away rather than emitted.
	if s.Proto == 0 && s.Port.Form != ipsec.PortAny {
		return tsSelector{}, false
	}
	// OPAQUE ports have no exact dataplane encoding. The kernel selector derives its
	// port mask from the port value, so "exactly port 0" cannot be built and would
	// install as any-port -- which is WIDER than the OPAQUE set RFC 7296 Section 3.13.1
	// describes, and Section 2.9 forbids widening. A peer that proposes OPAQUE-only
	// therefore finds no acceptable subset and is answered with TS_UNACCEPTABLE.
	if s.Port.Form == ipsec.PortOpaque {
		return tsSelector{}, false
	}
	return s, true
}

// dedupePairs removes duplicate answers. One proposed pair can intersect several policy
// entries and yield the same result more than once.
func dedupePairs(pairs []tsPair) []tsPair {
	out := make([]tsPair, 0, len(pairs))
	seen := make(map[string]bool, len(pairs))
	var key textbuf.Buffer
	for _, p := range pairs {
		key.Reset()
		key.Str(p.I.Net.String()).Byte('|').Str(p.I.Port.String()).Byte('|')
		key.Str(p.R.Net.String()).Byte('|').Str(p.R.Port.String()).Byte('|').Uint8(p.I.Proto)
		k := string(key.Bytes())
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, p)
	}
	return out
}

// policyPairs converts a peer's configured selectors into TSi/TSr orientation.
//
// RFC 7296 Section 2.9: TSi is the initiator's selector and TSr the responder's. The
// operator writes local and remote, so the two swap when Ze is the responder. This is
// the same role mapping createFirstChildSA applies to the negotiated result.
func policyPairs(peer ipsec.SiteToSitePeer, isInitiator bool) []tsPair {
	if len(peer.TrafficSelectors) == 0 {
		return nil
	}
	out := make([]tsPair, 0, len(peer.TrafficSelectors))
	for _, ts := range peer.TrafficSelectors {
		local := tsSelector{Net: ts.LocalPrefix, Port: ts.LocalPort, Proto: ts.Protocol}
		remote := tsSelector{Net: ts.RemotePrefix, Port: ts.RemotePort, Proto: ts.Protocol}
		if isInitiator {
			out = append(out, tsPair{I: local, R: remote})
			continue
		}
		out = append(out, tsPair{I: remote, R: local})
	}
	return out
}

// swapPairs exchanges the I and R half of every pair.
//
// RFC 7296 Section 2.9 orients TSi and TSr by who INITIATED the exchange, so one stored
// selector set is TSi/TSr in one exchange and TSr/TSi in the next. This is the whole
// conversion between the two, and it is its own inverse.
func swapPairs(pairs []tsPair) []tsPair {
	out := make([]tsPair, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, tsPair{I: p.R, R: p.I})
	}
	return out
}

// wireToSelectors converts a received TS payload into Ze's internal form.
//
// A selector Ze cannot express exactly is NARROWED, never rounded outward:
//   - an address range that is not prefix-expressible becomes the largest prefix inside
//     it (largestPrefixIn);
//   - a port range that is neither ANY, nor a single port, nor OPAQUE becomes its start
//     port, which is a subset of the proposed range.
//
// Both reductions are subsets, so RFC 7296 Section 2.9's ban on widening holds through
// the conversion. A selector with no usable subset is omitted, and a payload whose every
// selector is omitted narrows to the empty set and draws TS_UNACCEPTABLE.
func wireToSelectors(list []wire.TrafficSelector) []tsSelector {
	out := make([]tsSelector, 0, len(list))
	for i := range list {
		ts := list[i]
		n := largestPrefixIn(ts.StartAddress, ts.EndAddress)
		if n == nil {
			continue
		}
		port, ok := ipsec.PortSelectorFromWire(ts.StartPort, ts.EndPort)
		if !ok {
			// Narrow an arbitrary inclusive range to its first port. RFC 7296 Section
			// 2.9 permits narrowing; the XFRM selector carries a port and a mask, so an
			// arbitrary range has no exact form.
			if ts.StartPort == 0 || ts.StartPort > ts.EndPort {
				continue
			}
			port = ipsec.PortSelector{Form: ipsec.PortSingle, Port: ts.StartPort}
		}
		out = append(out, tsSelector{Net: n, Port: port, Proto: ts.IPProtocol})
	}
	return out
}

// largestPrefixIn returns the longest prefix that starts at start and stays inside the
// inclusive range start..end, or nil when the range is empty or malformed.
//
// A CIDR-aligned range yields exactly that prefix, so the common case is lossless. A
// range such as 10.0.0.5-10.0.0.9 yields 10.0.0.5/32, which is a subset. It never yields
// the enclosing prefix, because that would widen the proposal.
func largestPrefixIn(start, end []byte) *net.IPNet {
	if len(start) == 0 || len(start) != len(end) {
		return nil
	}
	if string(start) > string(end) {
		return nil // an empty range
	}
	bits := len(start) * 8
	ip := make(net.IP, len(start))
	copy(ip, start)
	for ones := 0; ones <= bits; ones++ {
		mask := net.CIDRMask(ones, bits)
		if !ip.Mask(mask).Equal(ip) {
			continue
		}
		if prefixLastAddr(ip, mask) <= string(end) {
			return &net.IPNet{IP: ip, Mask: mask}
		}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
}

// prefixLastAddr returns the highest address of a prefix as a byte string, so lexical
// order equals numeric order for equal-length addresses.
func prefixLastAddr(ip net.IP, mask net.IPMask) string {
	last := make([]byte, len(ip))
	for i := range ip {
		last[i] = ip[i] | ^mask[i]
	}
	return string(last)
}

// selectorsToWire encodes narrowed selectors into a TS payload.
//
// It is the SINGLE producer of the Start Port and End Port octets on every send path, so
// the three port MUSTs of RFC 7296 Section 3.13.1 are decided in one place:
//
//	Start Port: "For protocols for which port is undefined (including protocol 0), or if
//	all ports are allowed, this field MUST be zero."
//	End Port: the same condition, "this field MUST be 65535."
//	"Systems working with [IPSECARCH] that wish to indicate 'OPAQUE' ports, but not 'ANY'
//	ports, MUST set the start port to 65535 and the end port to 0."
//
// ipsec.PortSelector.Wire owns the three encodings. Nothing here writes a port literal.
func selectorsToWire(sels []tsSelector, payloadType uint8) *wire.PayloadTS {
	out := make([]wire.TrafficSelector, 0, len(sels))
	for _, s := range sels {
		start, end := netRange(s.Net)
		if start == nil {
			continue
		}
		tsType := wire.TSTypeIPv4AddrRange
		if len(start) == 16 {
			tsType = wire.TSTypeIPv6AddrRange
		}
		startPort, endPort := s.Port.Wire()
		out = append(out, wire.TrafficSelector{
			TSType:       tsType,
			IPProtocol:   s.Proto,
			StartPort:    startPort,
			EndPort:      endPort,
			StartAddress: start,
			EndAddress:   end,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return &wire.PayloadTS{TSPayloadType: payloadType, TrafficSelectors: out}
}

// netRange expands a prefix into the inclusive address range RFC 7296 Section 3.13.1
// puts on the wire.
func netRange(n *net.IPNet) (start, end []byte) {
	if n == nil {
		return nil, nil
	}
	ip := n.IP
	mask := n.Mask
	if v4 := ip.To4(); v4 != nil {
		ip = v4
		if len(mask) == 16 {
			mask = mask[12:]
		}
	}
	if len(ip) != len(mask) {
		return nil, nil
	}
	start = make([]byte, len(ip))
	end = make([]byte, len(ip))
	for i := range ip {
		start[i] = ip[i] & mask[i]
		end[i] = start[i] | ^mask[i]
	}
	return start, end
}

// narrowChildSelectors narrows a received TSi/TSr pair against the peer's configured
// policy and records the result on the SA.
//
// It is the single entry point every responder producer uses, so the two producers
// (buildAuthResponse and startResponderEAP) cannot drift apart. A second producer that
// skipped narrowing would leave the EAP path answering with the old wildcard.
//
// floor is the selector set of the SA being rekeyed, or nil for a fresh Child SA. It is
// in the orientation of the exchange in hand, which respondChildRekey establishes before
// it calls.
//
// It returns errTSUnacceptable when nothing is acceptable, and notifyForRefusal maps
// that to the TS_UNACCEPTABLE notify RFC 7296 Section 2.9 asks for. A result that does
// not cover the floor is one of those cases, so the answer this function records is
// always a set the caller can install unchanged.
func narrowChildSelectors(sa *SA, tsi, tsr *wire.PayloadTS, floor []tsPair) error {
	proposedI := wireToSelectors(tsi.TrafficSelectors)
	proposedR := wireToSelectors(tsr.TrafficSelectors)

	// Narrowing is the RESPONDER's job (RFC 7296 Section 2.9), and every caller of this
	// function is answering a proposal somebody else sent. This node is therefore the
	// RESPONDER of the exchange in hand, so its configured local side is TSr.
	//
	// The IKE SA role is NOT that role. Either end may start a CREATE_CHILD_SA
	// (Section 1.3.2), so a peer-initiated rekey on a tunnel this node initiated has
	// sa.IsInitiator true while this node responds. Orienting the policy by the IKE SA
	// role there compared the local prefix against the peer's, found no intersection, and
	// answered TS_UNACCEPTABLE to every rekey the peer started. The initiator's own
	// adoption path is recordInitiatorSelectors below, which orients by true.
	narrowed, ok := narrowSelectors(proposedI, proposedR, policyPairs(sa.PeerCfg, false), floor)
	if !ok {
		return errTSUnacceptable
	}

	// RFC 7296 Section 2.23.1 MUST: "For transport mode, it MUST use exactly one IP
	// address in the TSi and TSr payloads." A selector wider than a host cannot satisfy
	// it, so a transport-mode exchange refuses rather than narrows silently.
	if sa.UseTransportMode || sa.PeerRequestedTransport {
		narrowed = keepSingleAddress(narrowed)
		if len(narrowed) == 0 {
			return errTSUnacceptable
		}
	}

	// RFC 7296 Section 2.9.2 MUST NOT: "Thus, the new SA MUST NOT have narrower selectors
	// than the original" (rfc/full/rfc7296.txt:2539-2540), and "The responder MUST NOT
	// narrow down the Traffic Selectors narrower than the scope currently in use"
	// (rfc/full/rfc7296.txt:2551-2552).
	//
	// narrowSelectors answers the floor while the proposal covers it. When the proposal
	// covers no floor pair it answers the intersection instead, and that answer is narrower
	// than the scope in use. No legal answer is left: Section 2.9 refuses an answer wider
	// than the proposal, and Section 2.9.2 refuses one narrower than the floor. So the
	// exchange is refused with the TS_UNACCEPTABLE that Section 2.9 already names.
	//
	// Section 2.9.2 says what such a request means: "If the rekeyed SA would ever need to
	// have a narrower scope than the currently used SA, that would mean that the policy was
	// changed in a way such that the currently used SA is against the policy. In that case,
	// the SA should have been already deleted after the policy change took effect."
	// Refusing therefore declines a state that should not exist, rather than a workable
	// rekey.
	if len(floor) > 0 && !coversFloor(narrowed, floor) {
		return fmt.Errorf("%w: the rekey proposal narrows the scope in use %s down to %s",
			errTSUnacceptable, pairsText(floor), pairsText(narrowed))
	}

	sa.NegotiatedPairs = narrowed
	sa.NegotiatedTSi = narrowed[0].I.Net
	sa.NegotiatedTSr = narrowed[0].R.Net
	return nil
}

// errTSWidened reports a responder answer that is not a subset of what Ze proposed, or not
// a subset of Ze's own configured policy.
//
// RFC 7296 Section 2.9 states the one direction the answer CAN move in: "the responder is
// allowed to narrow the choices by selecting a subset of the traffic selectors" and "If the
// responder's policy does not allow it to accept any part of the proposed Traffic Selectors,
// it responds with a TS_UNACCEPTABLE Notify message". There is no widening arm. An answer
// outside the proposal is a protocol violation, and installing it would let the peer choose
// the traffic Ze forwards.
var errTSWidened = errors.New("ike: the responder answered with traffic selectors wider than the proposal")

// errTSUnusable marks an answer this node could neither read nor program. It is separate
// from errTSWidened because it accuses the peer of nothing: the answer may be perfectly
// legal and simply outside what this backend can express.
var errTSUnusable = errors.New("ike: the responder's traffic selectors cannot be adopted")

// recordInitiatorSelectors stores the selectors a responder answered with. It first makes
// sure the answer widened nothing.
//
// RFC 7296 Section 2.9 lets the responder narrow, so the answer can be a strict subset of
// what Ze proposed. The INITIATOR installs the answer rather than its own proposal, or
// the two ends would program different traffic. It is not narrowed again here: the
// responder already did the narrowing, and narrowing an answer a second time would shrink
// the SA below what both ends agreed.
//
// THE ANSWER IS NOT TRUSTED, ONLY ADOPTED. Narrowing is the responder's to do, and the
// direction is one-way. Every answered selector must therefore sit inside a selector Ze
// proposed.
//
// The initiator installed whatever came back before this test existed. A peer that answered
// 0.0.0.0/0 to a proposal of 10.1.0.0/16 had Ze program a policy for the whole internet.
// That is a policy bypass, and the far end drives all of it.
//
// THE CEILING IS THE PROPOSAL. It falls back to the configured policy only when there was
// no proposal to speak of. sa.ProposedChildPairs is what proposeChildTSPayloads put on the
// wire, and that is already the operator's policy in every ordinary case.
//
// Transport mode is why the policy is not read beside it. RFC 7296 Section 2.23.1 requires
// "exactly one IP address in the TSi and TSr payloads". transportSelectorPairs therefore
// replaces the operator's PREFIXES with the SA's own endpoint addresses. Those addresses
// need not sit inside the configured prefixes. A test of the answer against the policy as
// well refuses the responder's correct narrowing of ze's own correct proposal.
//
// An EMPTY proposal means the wildcard went on the wire. The operator's policy is then the
// only ceiling left, and ze applies it. That is the fallback path for a configured policy
// no TS payload can carry, where the wildcard is wider than the operator asked for. An
// empty policy in turn means everything. That is the load-bearing default for every
// configuration written before the traffic-selector list existed, and the test is skipped.
//
// A selector Ze cannot program exactly is still reduced to a programmable subset by
// wireToSelectors, because a peer can answer with a range no backend can express. That
// reduction only ever shrinks a selector, so it cannot turn a widening answer into a
// permitted one.
//
// floor is the selector set of the SA being rekeyed, or nil for a fresh Child SA, exactly
// as it is for narrowChildSelectors above. It is in the orientation of the exchange in
// hand, which applyChildRekeyResponse (rekey.go) establishes before it calls. An answer
// that does not cover it is refused, so the set this function records is always one the
// caller can install unchanged.
func recordInitiatorSelectors(sa *SA, tsi, tsr *wire.PayloadTS, floor []tsPair) error {
	iSels := wireToSelectors(tsi.TrafficSelectors)
	rSels := wireToSelectors(tsr.TrafficSelectors)
	// A GUARD MUST DENY OR SAY SOMETHING (ai/rules/evidence.md). Both exits
	// below used to `return nil`, which skipped checkAnswerWithin AND left
	// NegotiatedPairs unset: the answer was neither checked nor adopted, and the caller
	// read success. The SA then went up with no negotiated selector at all, and the
	// endpoint-only fallback programmed a narrower tunnel than either end agreed while
	// nothing said so.
	//
	// Refusing is the conformant answer, not merely the safe one. RFC 7296 Section 2.9
	// gives the responder one legal move, to narrow, and an answer this node cannot read
	// or cannot program is not a narrowing it can adopt. Installing the proposal instead
	// would program traffic the responder never agreed to (ai/rules/protocol.md).
	if len(iSels) == 0 || len(rSels) == 0 {
		return fmt.Errorf("%w: the responder answered with %d TSi and %d TSr selectors this node could not decode",
			errTSUnusable, len(iSels), len(rSels))
	}
	pairs := make([]tsPair, 0, len(iSels))
	for i := range iSels {
		r := rSels[min(i, len(rSels)-1)]
		if pair, ok := programmablePair(tsPair{I: iSels[i], R: r}); ok {
			pairs = append(pairs, pair)
		}
	}
	if len(pairs) == 0 {
		return fmt.Errorf("%w: none of the %d answered selector pairs is programmable",
			errTSUnusable, len(iSels))
	}
	ceiling, what := sa.ProposedChildPairs, "the proposal ze sent"
	if len(ceiling) == 0 {
		ceiling, what = policyPairs(sa.PeerCfg, true), "the configured policy"
	}
	if err := checkAnswerWithin(pairs, ceiling, what); err != nil {
		return err
	}

	// RFC 7296 Section 2.9.2 MUST NOT: "Thus, the new SA MUST NOT have narrower selectors
	// than the original" (rfc/full/rfc7296.txt:2539-2540), and "The responder MUST NOT
	// narrow down the Traffic Selectors narrower than the scope currently in use"
	// (rfc/full/rfc7296.txt:2551-2552).
	//
	// The first sentence binds the NEW SA, so it binds the end that installs one as much as
	// the end that answers. A peer that answers below the scope in use has broken the
	// second, and adopting that answer would build the SA the first forbids. Neither is
	// available, so the exchange is refused and the SA in use stays.
	//
	// The initiator sends no error notification for it. RFC 7296 Section 2.21.3: "there
	// should not be any reason for a peer to send error messages to the other end except as
	// a response. Because sending such error messages as an INFORMATIONAL exchange might
	// lead to further errors that could cause loops, such errors SHOULD NOT be sent"
	// (rfc/full/rfc7296.txt:3331-3337). The caller drops the pending rekey and logs both
	// selector sets instead (inbound.go).
	if len(floor) > 0 && !coversFloor(pairs, floor) {
		return fmt.Errorf("%w: the rekey answer narrows the scope in use %s down to %s",
			errTSUnacceptable, pairsText(floor), pairsText(pairs))
	}

	sa.NegotiatedPairs = pairs
	sa.NegotiatedTSi = pairs[0].I.Net
	sa.NegotiatedTSr = pairs[0].R.Net
	return nil
}

// checkAnswerWithin reports the first answered pair that no pair of ceiling covers.
//
// An EMPTY ceiling permits everything and is not a refusal: it is the wildcard proposal and
// the unconfigured policy, both of which mean "no constraint". Every non-empty ceiling is
// absolute (ai/rules/evidence.md: a pair matching nothing is refused, never
// admitted by default).
func checkAnswerWithin(answer, ceiling []tsPair, what string) error {
	if len(ceiling) == 0 {
		return nil
	}
	for _, a := range answer {
		if !pairWithinAny(a, ceiling) {
			return fmt.Errorf("%w: %v <-> %v is outside %s",
				errTSWidened, a.I.Net, a.R.Net, what)
		}
	}
	return nil
}

// pairWithinAny reports whether some ceiling pair covers BOTH halves of a.
//
// The SAME entry must cover the two halves. A pair whose TSi comes from one policy row, and
// whose TSr comes from another, describes a flow that neither row permits.
func pairWithinAny(a tsPair, ceiling []tsPair) bool {
	for _, c := range ceiling {
		if selectorWithin(a.I, c.I) && selectorWithin(a.R, c.R) {
			return true
		}
	}
	return false
}

// selectorWithin reports whether inner is covered by outer in all three dimensions.
//
// The port and protocol tests reuse the intersection helpers: an intersection that yields
// inner unchanged is exactly the statement that outer already permitted it.
func selectorWithin(inner, outer tsSelector) bool {
	if !containsNet(outer.Net, inner.Net) {
		return false
	}
	port, ok := intersectPort(inner.Port, outer.Port)
	if !ok || port != inner.Port {
		return false
	}
	proto, ok := intersectProto(inner.Proto, outer.Proto)
	return ok && proto == inner.Proto
}

// keepSingleAddress drops every selector whose prefix covers more than one address.
//
// RFC 7296 Section 2.23.1 permits SEVERAL selectors in transport mode ("It can have
// multiple Traffic Selectors if it has, for example, multiple port ranges that it wants
// to negotiate"), so this filters by ADDRESS WIDTH and never by selector count.
func keepSingleAddress(pairs []tsPair) []tsPair {
	out := make([]tsPair, 0, len(pairs))
	for _, p := range pairs {
		if singleAddress(p.I.Net) && singleAddress(p.R.Net) {
			out = append(out, p)
		}
	}
	return out
}

func singleAddress(n *net.IPNet) bool {
	if n == nil {
		return false
	}
	ones, bits := n.Mask.Size()
	return bits > 0 && ones == bits
}

// pairsToWire builds the TSi and TSr payloads of a narrowed answer.
func pairsToWire(pairs []tsPair) (*wire.PayloadTS, *wire.PayloadTS) {
	iSels := make([]tsSelector, 0, len(pairs))
	rSels := make([]tsSelector, 0, len(pairs))
	for _, p := range pairs {
		iSels = append(iSels, p.I)
		rSels = append(rSels, p.R)
	}
	return selectorsToWire(iSels, wire.PayloadTypeTSi), selectorsToWire(rSels, wire.PayloadTypeTSr)
}
