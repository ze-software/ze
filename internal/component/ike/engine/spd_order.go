// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions and their order
// Related: spd_policy.go -- spdPolicyParams, the projection whose selectors this compares
// Related: config.go -- validateIPsecSections, which runs verifySPDOrder at every commit door
// Related: icmp_overlap.go -- the other commit refusal that pairs an operator entry with a peer
// Related: ../dataplane/policy_owner.go -- PolicySelectorsEqual, and the install-time refusal of the same collision
// RFC: rfc/short/rfc4301.md -- SPD ordering (Section 4.4.1)

package engine

import (
	"context"
	"log/slog"
	"maps"
	"math"
	"net"
	"net/netip"
	"slices"
	"strconv"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// spdOrderRefusals answers every pair of SPD entries whose order the configuration
// leaves undecided, in a stable order: operator entries pairwise, then each operator
// entry against each peer, names sorted. The answer is nil when the order is total.
//
// RFC 4301 Section 4.4.1: the management interface "MUST support (total) ordering of
// these entries, as seen via this interface." Order alone need not be unique: two
// entries no packet can match both of may share one, because no order between them is
// observable. Two that a packet can match both of must not, because the dataplane then
// decides between them by the sequence it received them in (dataplane.go, the priority
// constants), which no configuration states. Overlap is judged on the policies each
// entry installs (spdPolicyParams, peerConfiguredPolicies) with the predicate the
// backend's own cross-peer check uses (dataplane.PolicySelectorsOverlap), so this and
// the dataplane cannot disagree on what an overlap is.
//
// Two operator entries that install the same selector in one direction are refused
// whatever their orders: the database holds one entry per selector (spdSelector), so
// the one installed second replaces the first and only one of the two would act.
//
// A peer joins the comparison with its configured traffic selectors at its
// policy-priority, the selectors its negotiated ones are narrowed from (RFC 7296
// Section 2.9), so a pair that may tie after a negotiation is refused before it. A
// peer with `vti bind` is skipped: its policies carry the XFRM interface's if_id, an
// SPD of their own that no operator entry is in (icmp_overlap.go states the kernel
// path). Two peers are not compared here: their negotiated selectors are refused at
// install when they would share one (dataplane.TransportSelectorConflictError and the
// cross-peer tunnel check).
//
// An operator entry and a peer that install the same selector in one direction are
// refused whatever their ranks too (spdPeerSelectorRefusal), every policy of the entry
// judged, its gates included: the dataplane would hold one of the two, so a gate would
// replace the peer's PROTECT policy with an allow. The selectors a peer negotiates
// narrower than it configures are refused at install by the backend's ownership ledger
// (dataplane.PolicyOwnedError).
//
// The forward projection of each side joins the comparison (spdJudgedPolicies): an
// operator entry's transit decisions sit at its order, and a tunnel-mode peer's forward
// entries at its policy-priority and one rank below it (peerConfiguredPolicies). The
// verifier runs with no dataplane in hand, so it judges the set Linux XFRM installs on
// Ze's kernel, the only backend with a forward lookup; on VPP a pair that meets only in
// FWD is refused although VPP would install neither forward entry. The gates of
// spdTransitPolicies are judged for a shared selector only: they are allows at one rank
// beneath every configured one, so an order between two of them is never observable.
func spdOrderRefusals(cfg *ipsec.IPsecConfig) []*opererr.Error {
	names := slices.Sorted(maps.Keys(cfg.Policies))
	var refusals []*opererr.Error
	for i, first := range names {
		a := cfg.Policies[first]
		for _, second := range names[i+1:] {
			b := cfg.Policies[second]
			if spdPoliciesShareASelector(spdJudgedPolicies(a, true), spdJudgedPolicies(b, true)) {
				refusals = append(refusals, spdSameSelectorRefusal(first, second, spdCollidingGate(first, a, second, b)))
				continue
			}
			if a.Order != b.Order {
				continue
			}
			if !spdPoliciesOverlap(spdJudgedPolicies(a, false), spdJudgedPolicies(b, false)) {
				continue
			}
			refusals = append(refusals, spdOrderTieRefusal(
				"vpn ipsec policy "+first+" and vpn ipsec policy "+second, second, a.Order))
		}
	}
	for _, peerName := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[peerName]
		if peer.VTIBind != "" {
			continue
		}
		peerPolicies := peerConfiguredPolicies(&peer)
		for _, name := range names {
			entry := cfg.Policies[name]
			// RFC 4301 Section 4.4.1.
			if shared, found := spdSharedKernelPolicy(spdJudgedPolicies(entry, true), peerPolicies); found {
				refusals = append(refusals, spdPeerSelectorRefusal(name, peerName, &shared))
				continue
			}
			rank, tied := spdRankTie(spdJudgedPolicies(entry, false), peerPolicies)
			if !tied {
				continue
			}
			which := "its policy-priority"
			if rank != int(peer.PolicyPriority) {
				which = "its forwarding entries at policy-priority + 1"
			}
			refusals = append(refusals, spdOrderTieRefusal(
				"vpn ipsec policy "+name+" and site-to-site peer "+peerName+" ("+which+")", name, entry.Order))
		}
	}
	return refusals
}

// spdJudgedPolicies is what the order check compares for one operator entry: its out
// and in halves and its transit decisions, all at its order, and with gates its transit
// gates as well (spdTransitPolicies). It is spdEntryPolicies on a dataplane with a
// forward lookup that gathers fragments (Ze's kernel, capability bit 3), the widest set
// XFRM installs: the verifier runs with no dataplane in hand, so it judges a port-scoped
// bypass's forward entries too, and on a kernel without the bit it refuses a pair that
// meets only there although neither forward entry would be installed.
func spdJudgedPolicies(p ipsec.SPDPolicy, gates bool) []dataplane.SPParams {
	transit := spdTransitPolicies(p, true)
	out := spdPolicyParams(p)
	out = append(out, transit.decisions...)
	if !gates {
		return out
	}
	return append(out, transit.gates...)
}

// spdSharedKernelPolicy answers the first policy of an operator entry that is, to the
// dataplane, one policy with a policy of the peer, and whether there is one.
//
// RFC 4301 Section 4.4.1: "Thus, a user or administrator MUST be able to order the
// entries to express a desired access control policy." Two policies with one selector,
// direction and if_id cannot be ordered: the kernel holds one policy per such identity
// whatever the priorities (xfrm_policy_insert_list replaces the one it holds), so the
// one installed second would act alone. Identity is judged by the backend's own key
// (dataplane.PolicySelectorsEqual), the one its ownership ledger refuses an install on.
func spdSharedKernelPolicy(entry, peer []dataplane.SPParams) (dataplane.SPParams, bool) {
	for i := range entry {
		for j := range peer {
			if dataplane.PolicySelectorsEqual(entry[i], peer[j]) {
				return entry[i], true
			}
		}
	}
	return dataplane.SPParams{}, false
}

// spdPeerSelectorRefusal is the refusal of an operator entry that installs the selector
// a peer's Child SA also installs, in one direction: both names, the shared policy, and
// the command that removes the operator entry.
func spdPeerSelectorRefusal(entry, peer string, shared *dataplane.SPParams) *opererr.Error {
	return opererr.New(
		"vpn ipsec policy "+entry+" and site-to-site peer "+peer+" install the same SPD selector "+
			ipNetText(shared.Src)+" -> "+ipNetText(shared.Dst)+" ("+dirLabel(shared.Dir)+")",
		"the dataplane holds one SPD entry per selector and direction, whatever the order and the "+
			"policy-priority, so the operator entry would replace the policy that protects the peer's traffic, "+
			"which would then leave unprotected, or the peer's teardown would remove the operator entry",
		"delete vpn ipsec policy "+entry)
}

// spdRankTie answers the rank of the first pair, one policy of each set, that sits at
// one priority and that one packet can match, and whether there is such a pair.
func spdRankTie(entry, peer []dataplane.SPParams) (int, bool) {
	for i := range entry {
		for j := range peer {
			if entry[i].Priority != peer[j].Priority {
				continue
			}
			if dataplane.PolicySelectorsOverlap(entry[i], peer[j]) {
				return peer[j].Priority, true
			}
		}
	}
	return 0, false
}

// spdOrderTieRefusal is the refusal of two overlapping entries that share an order,
// with the command that gives the named operator entry an order of its own. The order
// is at most math.MaxInt32 (the YANG range), so it converts to int unchanged.
func spdOrderTieRefusal(pair, entry string, order uint32) *opererr.Error {
	separate := int(order) + 1
	if order == math.MaxInt32 {
		separate = int(order) - 1
	}
	return opererr.New(
		pair+" overlap in selector and direction and share order "+strconv.Itoa(int(order)),
		"RFC 4301 Section 4.4.1 requires a total ordering of the SPD entries, and a packet both match "+
			"would be decided by the sequence the dataplane received them in, which no configuration states",
		"set vpn ipsec policy "+entry+" order "+strconv.Itoa(separate))
}

// spdCollidingGate answers the name of the entry whose transit gate is the only policy
// two operator entries share a selector through, or "" when they share one without a
// gate (a half or a transit decision of each). Only a bypass has gates, and its gates
// carry its own selectors in the other forwarding lookup, so a gate alone meets the
// other entry only when that entry names the same traffic with local and remote
// swapped.
func spdCollidingGate(first string, a ipsec.SPDPolicy, second string, b ipsec.SPDPolicy) string {
	if spdPoliciesShareASelector(spdJudgedPolicies(a, false), spdJudgedPolicies(b, false)) {
		return ""
	}
	if spdPoliciesShareASelector(spdTransitPolicies(a, true).gates, spdJudgedPolicies(b, true)) {
		return first
	}
	return second
}

// spdSameSelectorRefusal is the refusal of two operator entries that install the same
// selector in one direction. gate names the entry whose transit gate is the colliding
// policy (spdCollidingGate), or is "" when the entries collide without one.
//
// An order does not separate the two: the dataplane identifies an SPD entry by its
// selector, direction and if_id, never its priority (spdSelector), so whatever the
// orders the second install replaces the first. The fix is therefore one entry fewer.
func spdSameSelectorRefusal(first, second, gate string) *opererr.Error {
	if gate != "" {
		return opererr.New(
			"vpn ipsec policy "+first+" and vpn ipsec policy "+second+" name the same selector in the same direction "+
				"through the forwarding gate of vpn ipsec policy "+gate,
			"vpn ipsec policy "+gate+" is a bypass, so Ze installs a gate for it: an allow beneath every entry, in the "+
				"other forwarding lookup, that keeps that lookup's catch-all from dropping the forwarded traffic the "+
				"bypass admits. The other entry names the same traffic with local and remote swapped and installs a "+
				"policy at the gate's selector. The dataplane holds one SPD entry per selector whatever its order, so "+
				"a different order does not separate them: the entry installed second replaces the first. Keep the "+
				"traffic in one entry",
			"delete vpn ipsec policy "+second)
	}
	return opererr.New(
		"vpn ipsec policy "+first+" and vpn ipsec policy "+second+" name the same selector in the same direction",
		"the dataplane holds one SPD entry per selector, so the entry installed second would replace the first "+
			"whatever their orders, and only one of the two would act",
		"delete vpn ipsec policy "+second)
}

// spdPoliciesOverlap reports whether one packet can match a policy of each set.
func spdPoliciesOverlap(a, b []dataplane.SPParams) bool {
	for i := range a {
		for j := range b {
			if dataplane.PolicySelectorsOverlap(a[i], b[j]) {
				return true
			}
		}
	}
	return false
}

// spdPoliciesShareASelector reports whether the two sets install one selector twice.
func spdPoliciesShareASelector(a, b []dataplane.SPParams) bool {
	for i := range a {
		for j := range b {
			if selectorOf(&a[i]) == selectorOf(&b[j]) {
				return true
			}
		}
	}
	return false
}

// peerConfiguredPolicies projects a peer's configured traffic selectors onto the out
// and in policies its Child SAs would install, at its policy-priority, and for a peer
// that is not transport-mode onto their forward entries (appendPeerForwardPolicies), in
// the orientation childPolicies uses: local is the source of an outbound packet and the
// destination of an inbound one. An EMPTY list is a peer that accepts any selector
// (SiteToSitePeer.TrafficSelectors), and a side that names no prefix covers every
// address of the family its other side names, or of both (spdSelectorFamilies).
func peerConfiguredPolicies(peer *ipsec.SiteToSitePeer) []dataplane.SPParams {
	selectors := peer.TrafficSelectors
	if len(selectors) == 0 {
		selectors = []ipsec.TrafficSelectorPolicy{{LocalPort: ipsec.AnyPort(), RemotePort: ipsec.AnyPort()}}
	}
	var out []dataplane.SPParams
	for i := range selectors {
		ts := selectors[i]
		for _, family := range spdSelectorFamilies(&ts) {
			local := peerSelectorSide(ts.LocalPrefix, family)
			remote := peerSelectorSide(ts.RemotePrefix, family)
			localPort, remotePort := spdPortMatch(ts.LocalPort), spdPortMatch(ts.RemotePort)
			base := dataplane.SPParams{Priority: int(peer.PolicyPriority), UpperProto: ts.Protocol}
			outbound, inbound := base, base
			outbound.Dir, outbound.Src, outbound.Dst = dataplane.SADirOut, local, remote
			outbound.SrcPort, outbound.DstPort = localPort, remotePort
			inbound.Dir, inbound.Src, inbound.Dst = dataplane.SADirIn, remote, local
			inbound.SrcPort, inbound.DstPort = remotePort, localPort
			out = append(out, outbound, inbound)
			if peer.Mode != dataplane.ModeTransport {
				out = appendPeerForwardPolicies(out, outbound, inbound)
			}
		}
	}
	return out
}

// appendPeerForwardPolicies appends the forward entries a tunnel-mode Child SA of the
// peer installs for one selector pair, in the shape child.go builds them: the inbound
// selectors in FWD at the policy-priority (childReceivePolicies), the outbound
// selectors in FWD and the inbound ones in OUT one rank below it
// (appendChildOutboundForwardPolicy, appendChildDeliveryPolicy), each left out where
// the Child SA leaves it out because it would carry the kernel identity of another.
func appendPeerForwardPolicies(out []dataplane.SPParams, outbound, inbound dataplane.SPParams) []dataplane.SPParams {
	forwardIn := inbound
	forwardIn.Dir = dataplane.SADirFwd
	out = append(out, forwardIn)
	forwardOut := outbound
	forwardOut.Dir, forwardOut.Priority = dataplane.SADirFwd, outbound.Priority+1
	if selectorOf(&forwardOut) != selectorOf(&forwardIn) {
		out = append(out, forwardOut)
	}
	delivery := inbound
	delivery.Dir, delivery.Priority = dataplane.SADirOut, inbound.Priority+1
	if selectorOf(&delivery) != selectorOf(&outbound) {
		out = append(out, delivery)
	}
	return out
}

// spdSelectorFamilies answers the wildcard of each family one configured selector
// carries: the family its written prefixes name, or both when neither side names one.
// The address sets come from sideSpans (icmp_overlap.go), the reading the ICMP overlap
// refusal applies to the same selectors, so the two refusals agree on a family.
func spdSelectorFamilies(ts *ipsec.TrafficSelectorPolicy) []netip.Prefix {
	var v4, v6 bool
	for _, span := range sideSpans(ts.RemotePrefix, ts.LocalPrefix) {
		v4 = v4 || span.Addr().Is4()
		v6 = v6 || span.Addr().Is6()
	}
	families := make([]netip.Prefix, 0, 2)
	if v4 {
		families = append(families, anyIPv4)
	}
	if v6 {
		families = append(families, anyIPv6)
	}
	return families
}

// peerSelectorSide is one side of a configured selector in one family: the prefix
// written, or that family's wildcard when the side names none.
func peerSelectorSide(prefix *net.IPNet, family netip.Prefix) *net.IPNet {
	if prefix != nil {
		return prefix
	}
	if family.Addr().Is4() {
		return &net.IPNet{IP: net.IPv4zero.To4(), Mask: net.CIDRMask(0, 32)}
	}
	return &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
}

// verifySPDOrder is the verify half: it answers the first refusal. It is a step of
// validateIPsecSections (config.go), the offline verifier, so `ze config validate`,
// every commit door and the ike plugin's OnConfigVerify refuse the pair with both
// names, the shared order or selector and the command that separates them.
func verifySPDOrder(cfg *ipsec.IPsecConfig) error {
	refusals := spdOrderRefusals(cfg)
	if len(refusals) == 0 {
		return nil
	}
	return refusals[0]
}

// warnSPDOrderAtStart is the start-time half of verifySPDOrder. The configuration the
// daemon starts with is never verified, and a start never refuses the IPsec block
// (owner decision 2026-11-19, "Install, log, doctor"), so the start installs both
// entries, in their order and then by name (spdInstallOrder), and says so at Error
// with the refusal's facts and fix. Its code is config-plugin-verify because `ze
// doctor` reports the same pair under that code: the config-semantics check runs the
// offline verifier over the running configuration, as warnICMPOverlapAtStart states.
func warnSPDOrderAtStart(cfg *ipsec.IPsecConfig, log *slog.Logger) {
	for _, refusal := range spdOrderRefusals(cfg) {
		log.LogAttrs(context.Background(), slog.LevelError, "ike: "+refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeConfigPluginVerify))...)
	}
}
