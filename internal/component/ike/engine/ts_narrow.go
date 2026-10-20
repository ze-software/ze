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
	"slices"

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
// On the responder, largestPrefixIn therefore reduces a proposed range to the largest
// prefix INSIDE it, which RFC 7296 Section 2.9 permits because narrowing is always
// allowed, and the answer then carries that prefix. Rounding outward to the enclosing
// prefix would answer with traffic the peer never proposed. On the initiator an answered
// range is the negotiated value, so it becomes the exact set of prefixes tiling it
// (answerSelectors, rangePrefixes), one selector each.
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
// policy means "allow everything": every combination of the proposal is returned. That default is
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
// answer can never be wider than the proposal. The returned set is every acceptable
// pair, which a TS payload pair cannot always carry: closedSubset picks the part of it
// one TSi and one TSr payload can state exactly.
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

	// Bullets 1, 3 and 4. Acceptable intersections are collected in proposal order, and
	// the intersections covering the initiator's FIRST choices are kept apart so they can
	// lead the answer.
	//
	// RFC 7296 Section 3.13: "a packet matches a given TSi/TSr if it matches at least one
	// of the individual selectors in TSi, and at least one of the individual selectors in
	// TSr." The proposal is therefore every combination of a TSi entry with a TSr entry,
	// never entry i with entry i, and both branches walk that cross product.
	//
	// An unconfigured peer allows everything, so each combination is its own narrowing.
	// This is Section 2.9's second bullet: the policy allows the entire proposed set.
	var rest []tsPair
	var firstChoice []tsPair
	for i := range proposedI {
		for r := range proposedR {
			for _, pair := range acceptablePairs(tsPair{I: proposedI[i], R: proposedR[r]}, policy) {
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

// acceptablePairs returns the parts of one proposed combination the policy accepts and
// the dataplane can program exactly. An EMPTY policy accepts the combination whole. A
// combination whose two protocols are disjoint matches no packet, so it yields nothing.
func acceptablePairs(proposed tsPair, policy []tsPair) []tsPair {
	if _, ok := intersectProto(proposed.I.Proto, proposed.R.Proto); !ok {
		return nil
	}
	if len(policy) == 0 {
		pair, ok := programmablePair(proposed)
		if !ok {
			return nil
		}
		return []tsPair{pair}
	}
	var out []tsPair
	for _, p := range policy {
		pair, ok := intersectPair(proposed, p)
		if !ok {
			continue
		}
		pair, ok = programmablePair(pair)
		if !ok {
			continue
		}
		out = append(out, pair)
	}
	return out
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
		k := pairKey(&key, p)
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

// wireToSelectors converts a PROPOSAL the responder received into Ze's internal form.
// The initiator reads an answer through answerSelectors instead, because an answer is
// the negotiated value and must not be narrowed again.
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

	// RFC 7296 Section 2.23.1: "the server should first check that the initiator requested
	// transport mode, and then do address substitution on the Traffic Selectors", and
	// "After this address substitution, both the Traffic Selectors and the IKE UDP
	// source/destination addresses look the same, and the server does SPD lookup based on
	// those new Traffic Selectors."
	//
	// It is placed HERE, before the policy match below, because the policy match IS that
	// SPD lookup. A proposal carrying the peer's pre-NAT address intersects no policy that
	// names the address this node observes, so without this the responder answers
	// TS_UNACCEPTABLE to every conforming transport-mode client behind a NAT.
	//
	// It is a no-op for tunnel mode and for a NAT-free path, so every other exchange
	// narrows exactly as it did before (ts_nat_substitute.go).
	proposedI, proposedR = substituteResponderSelectors(sa, proposedI, proposedR)

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
		narrowed = oneTransportPair(narrowed)
	}

	// RFC 7296 Section 3.13: "a packet matches a given TSi/TSr if it matches at least one
	// of the individual selectors in TSi, and at least one of the individual selectors in
	// TSr." The initiator installs every combination of the answered payloads, so the
	// answer is cut to a set that IS every such combination. Answering the acceptable
	// pairs (A, C) and (B, D) as TSi {A, B} and TSr {C, D} would hand the peer A-D and
	// B-C as well, and RFC 7296 Section 2.9 lets the answer only narrow the policy.
	// Section 2.9 also licenses the cut: "When narrowing is done, there may be several
	// subsets that are acceptable but their union is not. In this case, the responder
	// arbitrarily chooses one of them". closedSubset keeps the first pair, so the
	// initiator's first choices still lead the answer.
	narrowed = closedSubset(narrowed, answerPairsMax)

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
// A selector Ze cannot program exactly is REFUSED (answerSelectors, answerPairs), never
// reduced to a programmable subset: the answer is the negotiated value, and a subset
// installs less than both ends agreed. An address range that is not a prefix is not
// such a selector, because it is installed as the exact set of prefixes tiling it.
//
// floor is the selector set of the SA being rekeyed, or nil for a fresh Child SA, exactly
// as it is for narrowChildSelectors above. It is in the orientation of the exchange in
// hand, which applyChildRekeyResponse (rekey.go) establishes before it calls. An answer
// that does not cover it is refused, so the set this function records is always one the
// caller can install unchanged.
func recordInitiatorSelectors(sa *SA, tsi, tsr *wire.PayloadTS, floor []tsPair) error {
	// RFC 4301 Section 4.4.2: "the entry for an inbound SA in the SAD MUST be initially
	// populated with the value or values negotiated at the time the SA was created".
	// The answer IS the negotiated value, so it is read exactly (answerSelectors), never
	// through wireToSelectors, whose narrowing is the responder's to do.
	iSels, err := answerSelectors(tsi.TrafficSelectors, "TSi")
	if err != nil {
		return err
	}
	rSels, err := answerSelectors(tsr.TrafficSelectors, "TSr")
	if err != nil {
		return err
	}

	// RFC 7296 Section 2.23.1: "Do address substitution before using those Traffic
	// Selectors for anything other than storing original content of them. This includes
	// verification that Traffic Selectors were narrowed correctly by the other end,
	// creation of the SAD entry, and so on."
	//
	// It is placed HERE, above every check below, because that sentence names this
	// function's whole job. checkAnswerWithin is the "verification that Traffic Selectors
	// were narrowed correctly by the other end", and sa.NegotiatedPairs is what becomes
	// the SAD entry. A conforming responder behind a NAT answers in ITS address space, so
	// without this the ceiling test refuses an answer that is correct.
	//
	// It is a no-op for tunnel mode and for a NAT-free path (ts_nat_substitute.go).
	iSels, rSels = substituteInitiatorSelectors(sa, iSels, rSels)

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
	pairs, err := answerPairs(iSels, rSels)
	if err != nil {
		return err
	}
	ceiling, what := sa.ProposedChildPairs, "the proposal ze sent"
	if len(ceiling) == 0 {
		ceiling, what = policyPairs(sa.PeerCfg, true), "the configured policy"
	}
	if err := checkAnswerWithin(pairs, ceiling, what); err != nil {
		return err
	}

	// A transport-mode inbound state holds ONE selector (inboundStateSelector, child.go),
	// so an answer of several pairs can be installed only as a cover wider than the
	// pairs, and RFC 4301 Section 5.2 forbids what that cover admits: "If an IPsec system
	// receives an inbound packet on an SA and the packet's header fields are not
	// consistent with the selectors for the SA, it MUST discard the packet." Narrowing the
	// answer again here would leave the peer sending traffic this node drops, so the
	// answer is refused and the caller deletes the Child SA (adoptAuthResponseNegotiation,
	// transport_mode.go), never installs it.
	if sa.UseTransportMode && len(pairs) > 1 {
		return fmt.Errorf("%w: the responder answered %s", errTSTransportPairs, pairsText(pairs))
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

// answerPairsMax bounds the policies one answer can expand into. An address range that
// is not a prefix becomes several prefixes, and each answered pair becomes every
// combination of its two halves' prefixes, so a hostile range could otherwise ask for
// thousands of kernel policies. An answer past the bound is refused, never cut.
const answerPairsMax = 64

// answerSelectors reads a responder's TS payload into Ze's form EXACTLY, and refuses
// what it cannot read exactly.
//
// An address range that is not one prefix becomes the exact set of prefixes that tiles
// it (rangePrefixes). A port range that is neither ANY, nor one port, nor OPAQUE is
// refused: both dataplanes program either every port or exactly one (xfrmSelectorPort,
// vppPortRange in internal/component/ike/dataplane), so the only exact form of
// 1024-65535 is 64512 policies. Cutting it to its start port, as wireToSelectors does
// for the responder, would install less than was negotiated.
func answerSelectors(list []wire.TrafficSelector, payload string) ([]tsSelector, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("%w: the responder answered %s with no selector", errTSUnusable, payload)
	}
	var sels []tsSelector
	for i := range list {
		ts := list[i]
		port, ok := ipsec.PortSelectorFromWire(ts.StartPort, ts.EndPort)
		if !ok {
			return nil, fmt.Errorf("%w: %s port range %d-%d has no exact dataplane form",
				errTSUnusable, payload, ts.StartPort, ts.EndPort)
		}
		nets, ok := rangePrefixes(ts.StartAddress, ts.EndAddress, answerPairsMax)
		if !ok {
			return nil, fmt.Errorf("%w: %s address range %v-%v is malformed or needs more than %d prefixes",
				errTSUnusable, payload, net.IP(ts.StartAddress), net.IP(ts.EndAddress), answerPairsMax)
		}
		for _, n := range nets {
			sels = append(sels, tsSelector{Net: n, Port: port, Proto: ts.IPProtocol})
		}
	}
	return sels, nil
}

// answerPairs builds the pairs an answer negotiated: every answered TSi selector with
// every answered TSr selector, after each range has become the prefixes tiling it.
//
// RFC 7296 Section 3.13: "There is no requirement that TSi and TSr contain the same
// number of individual Traffic Selectors. Thus, they are interpreted as follows: a
// packet matches a given TSi/TSr if it matches at least one of the individual selectors
// in TSi, and at least one of the individual selectors in TSr." Pairing entry i with
// entry i installed only A-C of an answer TSi {A}, TSr {C, D}, and dropped A-D.
//
// Every pair must be programmable exactly; one that is not refuses the answer, because
// dropping it would install less than was negotiated (RFC 4301 Section 4.4.2). An
// answer of more than answerPairsMax pairs is refused for the same reason.
//
// Both slices are non-empty: answerSelectors refuses an empty payload.
func answerPairs(iSels, rSels []tsSelector) ([]tsPair, error) {
	var pairs []tsPair
	for _, cross := range crossPairs(iSels, rSels) {
		pair, ok := programmablePair(cross)
		if !ok {
			return nil, fmt.Errorf("%w: the answered pair %v <-> %v has no exact dataplane form",
				errTSUnusable, cross.I.Net, cross.R.Net)
		}
		if len(pairs) == answerPairsMax {
			return nil, fmt.Errorf("%w: the answer expands into more than %d policies",
				errTSUnusable, answerPairsMax)
		}
		pairs = append(pairs, pair)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("%w: no answered TSi selector shares a protocol with any answered TSr selector",
			errTSUnusable)
	}
	return pairs, nil
}

// rangePrefixes returns the prefixes that tile the inclusive range start..end exactly,
// lowest first, or false when the range is empty, malformed, or needs more than limit.
//
// Each step takes the largest prefix that starts at the next uncovered address and ends
// inside the range (largestPrefixIn), so the union is the range and no prefix reaches
// past it. The loop is bounded by limit; an IPv6 range never needs more than 254.
func rangePrefixes(start, end []byte, limit int) ([]*net.IPNet, bool) {
	var out []*net.IPNet
	cur := start
	for len(out) < limit {
		n := largestPrefixIn(cur, end)
		if n == nil {
			return nil, false
		}
		out = append(out, n)
		last := prefixLastAddr(n.IP, n.Mask)
		if last == string(end) {
			return out, true
		}
		cur = nextAddr([]byte(last))
	}
	return nil, false
}

// nextAddr returns the address after a. Its one caller passes an address below the
// range end, which has a successor, so it does not wrap.
func nextAddr(a []byte) []byte {
	next := make([]byte, len(a))
	copy(next, a)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
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

// oneTransportPair narrows a transport-mode answer to its first pair.
//
// A transport-mode inbound XFRM state carries ONE selector and one SPI names one state
// (inboundStateSelector, child.go). Several pairs whose union is not one selector would
// leave the state either too narrow, dropping a negotiated pair's traffic, or a cover
// wider than the pairs, which breaks RFC 4301 Section 5.2: "If an IPsec system receives
// an inbound packet on an SA and the packet's header fields are not consistent with the
// selectors for the SA, it MUST discard the packet."
//
// RFC 7296 Section 2.9 lets the responder choose: "When narrowing is done, there may be
// several subsets that are acceptable but their union is not. In this case, the
// responder arbitrarily chooses one of them". The first pair is the one kept, because
// the same section binds the choice when the first selectors are acceptable: "If the
// responder's policy allows it to accept the first selector of TSi and TSr, then the
// responder MUST narrow the Traffic Selectors to a subset that includes the initiator's
// first choices." narrowSelectors leads its answer with that intersection, and with the
// rekey floor when the floor is answered.
//
// pairs MUST hold at least one pair; the caller refuses an empty set first.
func oneTransportPair(pairs []tsPair) []tsPair {
	return pairs[:1]
}

// errTSTransportPairs reports a transport-mode answer of several pairs. It wraps
// errTSUnusable: the answer can be legal, and only this node's one state selector cannot
// hold it exactly.
var errTSTransportPairs = fmt.Errorf("%w: a transport-mode Child SA holds one selector pair", errTSUnusable)

func singleAddress(n *net.IPNet) bool {
	if n == nil {
		return false
	}
	ones, bits := n.Mask.Size()
	return bits > 0 && ones == bits
}

// pairsToWire builds the TSi and TSr payloads that state pairs exactly: each distinct
// TSi selector once, and each distinct TSr selector once.
//
// RFC 7296 Section 3.13 reads the two payloads as every combination of their entries,
// so only a set that IS every such combination has a payload pair. For any other set it
// returns nil, nil, and every caller refuses: a payload pair for it would offer or answer
// combinations the set does not hold, and RFC 7296 Section 3.13 says of such a policy
// that it "cannot be negotiated as a single Child SA pair".
func pairsToWire(pairs []tsPair) (*wire.PayloadTS, *wire.PayloadTS) {
	if len(pairs) == 0 {
		return nil, nil
	}
	if _, missing := missingCombination(pairs); missing {
		return nil, nil
	}
	iSels, rSels := pairSelectors(pairs)
	return selectorsToWire(iSels, wire.PayloadTypeTSi), selectorsToWire(rSels, wire.PayloadTypeTSr)
}

// pairSelectors returns the distinct TSi and TSr selectors of pairs, in first-seen order.
func pairSelectors(pairs []tsPair) (iSels, rSels []tsSelector) {
	for _, p := range pairs {
		iSels = withSelector(iSels, p.I)
		rSels = withSelector(rSels, p.R)
	}
	return iSels, rSels
}

// crossPairs returns every combination of a TSi selector with a TSr selector that a
// packet can match, TSi-major.
//
// RFC 7296 Section 3.13: "a packet matches a given TSi/TSr if it matches at least one of
// the individual selectors in TSi, and at least one of the individual selectors in TSr."
// A packet carries one protocol, so a combination whose two protocols are disjoint
// matches no packet and is left out.
func crossPairs(iSels, rSels []tsSelector) []tsPair {
	out := make([]tsPair, 0, len(iSels)*len(rSels))
	for _, i := range iSels {
		for _, r := range rSels {
			if _, ok := intersectProto(i.Proto, r.Proto); !ok {
				continue
			}
			out = append(out, tsPair{I: i, R: r})
		}
	}
	return out
}

// missingCombination returns the first combination of the distinct selectors of pairs
// that pairs does not hold, and true. It returns false when pairs holds every one, which
// is the only shape a TSi and TSr payload pair can state exactly (RFC 7296 Section 3.13).
func missingCombination(pairs []tsPair) (tsPair, bool) {
	held := pairKeys(pairs)
	iSels, rSels := pairSelectors(pairs)
	var key textbuf.Buffer
	for _, cross := range crossPairs(iSels, rSels) {
		if !held[pairKey(&key, cross)] {
			return cross, true
		}
	}
	return tsPair{}, false
}

// closedSubset picks, from an acceptable answer, a subset that is every combination of
// its own selectors and holds at most limit pairs. The first pair is always kept; each
// later pair joins when every combination its selectors add is itself acceptable.
//
// RFC 7296 Section 2.9: "When narrowing is done, there may be several subsets that are
// acceptable but their union is not. In this case, the responder arbitrarily chooses one
// of them". A pair left out is one whose combinations with the kept pairs the policy
// does not allow, or one past the bound on installed policies.
func closedSubset(pairs []tsPair, limit int) []tsPair {
	held := pairKeys(pairs)
	var key textbuf.Buffer
	var iSels, rSels []tsSelector
	var out []tsPair
	for _, p := range pairs {
		nextI := withSelector(slices.Clip(iSels), p.I)
		nextR := withSelector(slices.Clip(rSels), p.R)
		cross := crossPairs(nextI, nextR)
		if len(cross) > limit {
			continue
		}
		if !allHeld(cross, held, &key) {
			continue
		}
		iSels, rSels, out = nextI, nextR, cross
	}
	return out
}

func allHeld(pairs []tsPair, held map[string]bool, key *textbuf.Buffer) bool {
	for _, p := range pairs {
		if !held[pairKey(key, p)] {
			return false
		}
	}
	return true
}

func pairKeys(pairs []tsPair) map[string]bool {
	held := make(map[string]bool, len(pairs))
	var key textbuf.Buffer
	for _, p := range pairs {
		held[pairKey(&key, p)] = true
	}
	return held
}

// withSelector returns list with s appended unless an equal selector is already in it.
func withSelector(list []tsSelector, s tsSelector) []tsSelector {
	for _, have := range list {
		if sameSelector(have, s) {
			return list
		}
	}
	return append(list, s)
}

func sameSelector(a, b tsSelector) bool {
	if a.Proto != b.Proto {
		return false
	}
	if a.Port != b.Port {
		return false
	}
	return a.Net.String() == b.Net.String()
}

// pairKey renders p as a map key: both halves' prefix, port and protocol.
func pairKey(key *textbuf.Buffer, p tsPair) string {
	key.Reset()
	key.Str(p.I.Net.String()).Byte('|').Str(p.I.Port.String()).Byte('|').Uint8(p.I.Proto).Byte('#')
	key.Str(p.R.Net.String()).Byte('|').Str(p.R.Port.String()).Byte('|').Uint8(p.R.Proto)
	return string(key.Bytes())
}
