// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ICMP processing icmp-error-check turns on
// Related: sa_options.go -- peerSAOptions, which sets the two kernel flags these refusals judge
// Related: unmatched.go -- the catch-all whose bypass disposition refusal A judges
// RFC: rfc/short/rfc4301.md -- Section 6.2, ICMP error processing

package engine

import (
	"context"
	"log/slog"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
)

// A peer whose icmp-error-check is true (the default) has every Child SA installed with
// XFRM_POLICY_ICMP on its out policy (child.go, ICMPErrorReturn) and XFRM_STATE_ICMP on
// its inbound state (ICMPErrorCheck). Each flag turns on a kernel path that overrides an
// operator SPD entry, and neither path can be told to respect one, so a configuration
// that puts such an entry beside such a peer is refused at verify (`ze config validate`
// and every commit, through validateIPsecSections in config.go) rather than installed
// with a disposition the kernel does not honor (docs/architecture/linux-limitations.md,
// LL-XFRM-ICMP-TRANSIT).
//
// Both refusals are conservative. Neither narrows on the operator entry's LOCAL prefix
// or on its order, because the ICMP message's Ze-side address is chosen by the kernel
// when the message is sent or arrives, so no commit can know it. An entry ranked below
// the peer's PROTECT policy is overridden for every triggering packet whose Ze address
// falls outside the peer's local selectors. A protocol-0 bypass entry ranked above it
// may not be overridden for every triggering packet, since the entry already matches
// some of the packets the error is about, but it is overridden for some, so it is
// refused too.
//
// A route-based peer (`vti bind`) is exempt from both. Its Child SAs' policies carry the
// xfrm interface's if_id, and an operator entry carries if_id 0. Outbound, the reverse
// lookup of icmp_route_lookup is xfrm_lookup (net/ipv4/icmp.c; net/ipv6/icmp.c in
// icmpv6_route_lookup), which is xfrm_lookup_with_ifid with if_id 0
// (net/xfrm/xfrm_policy.c), and xfrm_policy_match answers no match for a policy whose
// if_id differs, so such a peer's out policy is never the error's mapping target.
// Inbound, ICMP decrypted on an xfrm interface is policy-checked at that interface's
// if_id (xfrmi_decode_session, net/xfrm/xfrm_interface_core.c), where no operator entry
// is consulted. Neither kernel path can then override an operator entry for that peer.

// icmpErrorTypesV4 and icmpErrorTypesV6 are the ICMP error types the kernel sends
// through icmp_route_lookup (net/ipv4/icmp.c, from __icmp_send) and icmpv6_route_lookup
// (net/ipv6/icmp.c, from icmp6_send), the lookups that map an error into a Child SA.
// IPv4: destination unreachable 3, source quench 4, redirect 5, time exceeded 11 and
// parameter problem 12, every type icmp_pointers marks .error that __icmp_send is
// called with; 4 has no caller in Linux 7.3 and is kept because __icmp_send takes the
// type as an argument and icmp_pointers marks it an error. IPv6: destination
// unreachable 1, packet too big 2, time exceeded 3 and parameter problem 4, the error
// types icmp6_send is called with; a redirect (137) leaves through ndisc, which does not
// call icmpv6_route_lookup.
var (
	icmpErrorTypesV4 = []uint8{3, 4, 5, 11, 12}
	icmpErrorTypesV6 = []uint8{1, 2, 3, 4}
)

// The wildcard of each family, the address set a selector side that names no prefix
// covers.
var (
	anyIPv4 = netip.PrefixFrom(netip.IPv4Unspecified(), 0)
	anyIPv6 = netip.PrefixFrom(netip.IPv6Unspecified(), 0)
)

// refuseICMPErrorBypass is refusal A: a bypass entry that would carry an ICMP error Ze
// sends, beside a peer whose Child SAs map such an error into the SA.
//
// RFC 4301 Section 6.2: "If no SA exists that would carry the outbound ICMP message in
// question, and if no SPD entry would allow carriage of this outbound ICMP error
// message, then an IPsec implementation MUST map the message to the SA that would carry
// the return traffic associated with the packet that triggered the ICMP error message."
//
// Linux performs the mapping in icmp_route_lookup (net/ipv4/icmp.c) and
// icmpv6_route_lookup (net/ipv6/icmp.c): when the normal lookup of the error answers the
// plain route (no policy, or a template-free allow policy, which is a BYPASS entry) or
// -EPERM, the kernel looks the reversed triggering packet up with XFRM_LOOKUP_ICMP and
// sends the error into the Child SA whose out policy carries XFRM_POLICY_ICMP. It cannot
// tell a BYPASS entry from no entry, so an error a bypass entry would carry in clear is
// mapped too, which the sentence's second condition excludes. The error's destination is
// the source of the triggering packet, so the reversed lookup meets the SA's out policy
// only when that source is inside the peer's REMOTE selectors.
//
// The catch-all under `unmatched bypass` carries every such error, so it is refused
// beside any such peer. An operator entry is refused when it is bypass, out or both,
// protocol 0 or the ICMP protocol of its family, its ICMP type any or an error type the
// kernel sends, and its remote prefix overlaps one of the peer's remote selectors. A
// peer with `vti bind` is skipped (the exemption above).
//
// The first conflict is answered, peers and entries in sorted name order, so the same
// configuration always draws the same refusal. The answer is nil when there is none.
func refuseICMPErrorBypass(cfg *ipsec.IPsecConfig) *opererr.Error {
	policyNames := slices.Sorted(maps.Keys(cfg.Policies))
	for _, peerName := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[peerName]
		if peer.SkipICMPErrorCheck {
			continue
		}
		if peer.VTIBind != "" {
			continue
		}
		if cfg.Unmatched == dataplane.SPActionBypass {
			return opererr.New(
				"ipsec peer "+strconv.Quote(peerName)+" has icmp-error-check true and `vpn ipsec unmatched bypass` carries ICMP errors in clear",
				"Linux sends an ICMP error this router generates for a packet from the peer's remote selectors into the peer's Child SA, "+
					"although the bypass catch-all would carry it in clear; RFC 4301 Section 6.2 maps an error into the return SA only when no SPD entry would carry it",
				"set vpn ipsec site-to-site peer "+peerName+" icmp-error-check false, or set vpn ipsec unmatched discard")
		}
		remote := peerRemoteSpans(peer.TrafficSelectors)
		for _, policyName := range policyNames {
			policy := cfg.Policies[policyName]
			if !bypassCarriesICMPError(&policy) {
				continue
			}
			if !spansMeet(icmpSpans(&policy), remote) {
				continue
			}
			return opererr.New(
				"ipsec peer "+strconv.Quote(peerName)+" has icmp-error-check true and `vpn ipsec policy "+policyName+"` carries ICMP errors in clear to the peer's remote selectors",
				"Linux sends an ICMP error this router generates for a packet from the peer's remote selectors ("+spanList(remote)+") into the peer's Child SA, "+
					"although the bypass entry would carry it in clear; RFC 4301 Section 6.2 maps an error into the return SA only when no SPD entry would carry it",
				bypassEntryFix(peerName, policyName, &policy, remote))
		}
	}
	return nil
}

// bypassEntryFix is the fix of refusal A for one operator entry: turn the peer's check
// off, or keep the entry off the peer's remote selectors, or keep it off ICMP errors.
// The remote-prefix form is offered only when the selectors leave an address outside
// them, which they do not when a remote selector covers a whole family: a peer with no
// selector, or a selector that names no remote prefix. The last form follows
// the entry's protocol: an ICMP type selector exists only under protocol 1 or 58
// (SPDPolicy.validateICMP), so protocol 0 is narrowed by its protocol instead.
func bypassEntryFix(peerName, policyName string, policy *ipsec.SPDPolicy, remote []netip.Prefix) string {
	fix := "set vpn ipsec site-to-site peer " + peerName + " icmp-error-check false"
	if !slices.ContainsFunc(remote, func(span netip.Prefix) bool { return span.Bits() == 0 }) {
		fix += ", or set vpn ipsec policy " + policyName + " remote prefix <prefix> with a prefix that does not overlap " + spanList(remote)
	}
	if policy.Protocol == 0 {
		return fix + ", or set vpn ipsec policy " + policyName + " protocol <protocol> with the protocol the entry is for, other than 1 (ICMP) and 58 (ICMPv6)"
	}
	// The example is the echo request of the family, which validateICMPFragments
	// accepts on any direction, where type 0 with code 0 it refuses on out and both.
	errorTypes, example := icmpErrorTypesV4, "8"
	if policy.Protocol == dataplane.UpperProtoICMPv6 {
		errorTypes, example = icmpErrorTypesV6, "128"
	}
	return fix + ", or set vpn ipsec policy " + policyName + " icmp-type <type> with a type that is not an ICMP error (" + typeList(errorTypes) + "), such as " + example
}

// spanList names a set of address prefixes for an operator message, comma separated.
func spanList(spans []netip.Prefix) string {
	names := make([]string, 0, len(spans))
	for _, span := range spans {
		names = append(names, span.String())
	}
	return strings.Join(names, ", ")
}

// typeList names a set of ICMP types for an operator message, comma separated.
func typeList(types []uint8) string {
	var text []byte
	for i, icmpType := range types {
		if i > 0 {
			text = append(text, ", "...)
		}
		text = strconv.AppendUint(text, uint64(icmpType), 10)
	}
	return string(text)
}

// bypassCarriesICMPError reports whether an operator entry would carry an outbound ICMP
// error Ze sends: bypass, on the out side, with a protocol and an ICMP type an error
// can match. The address families its protocol admits are icmpSpans' answer.
func bypassCarriesICMPError(policy *ipsec.SPDPolicy) bool {
	if policy.Action != dataplane.SPActionBypass {
		return false
	}
	if policy.Direction != ipsec.SPDDirOut && policy.Direction != ipsec.SPDDirBoth {
		return false
	}
	if !policy.ICMP.HasType {
		return true
	}
	// An ICMP type selector exists only under protocol 1 or 58 (SPDPolicy.validateICMP),
	// so the type is judged against that protocol's error types. A code range narrower
	// than the type's codes still carries the errors inside it.
	if policy.Protocol == dataplane.UpperProtoICMPv6 {
		return slices.Contains(icmpErrorTypesV6, policy.ICMP.Type)
	}
	return slices.Contains(icmpErrorTypesV4, policy.ICMP.Type)
}

// refuseICMPCheckDiscard is refusal B: an inbound discard entry that matches ICMP,
// beside a peer whose Child SAs deliver the ICMP it refuses.
//
// Linux icmp_rcv (net/ipv4/icmp.c) and icmpv6_rcv (net/ipv6/icmp.c): when the inbound
// policy check refuses an ICMP message that arrived on a Child SA whose last secpath
// state carries XFRM_STATE_ICMP, the kernel checks again on the header the message
// carries after its ICMP header, with addresses and ports reversed, and delivers the
// message when that header matches the SA. That is whatever operator discard entry
// refused it and whatever the ICMP type, error or not. The message's own source and its
// local destination do not enter the second check, so no prefix of the discard entry
// keeps it out of the override, and neither does its order.
//
// No requirement id names this. Sections 6.1.1 and 6.1.2 ask for an administrator's
// control over the ICMP a node accepts, and on a Child SA of such a peer the discard
// entry would be that control in name only. The `unmatched discard` catch-all is not
// refused: overriding it for an error whose payload matches the SA is exactly the
// Section 6.2 processing.
//
// An entry is refused when it is discard, in or both, protocol 0 or the ICMP protocol of
// its family, any ICMP type, in a family the peer's selectors can carry. A peer with
// `vti bind` is skipped (the exemption above). The first conflict is answered in sorted
// name order, nil when there is none.
//
// The fix offers no change of the entry's direction: an out entry would be another
// entry, discarding what this router sends, and validateICMPFragments refuses some of
// them.
func refuseICMPCheckDiscard(cfg *ipsec.IPsecConfig) *opererr.Error {
	policyNames := slices.Sorted(maps.Keys(cfg.Policies))
	for _, peerName := range slices.Sorted(maps.Keys(cfg.Peers)) {
		peer := cfg.Peers[peerName]
		if peer.SkipICMPErrorCheck {
			continue
		}
		if peer.VTIBind != "" {
			continue
		}
		families := peerSelectorFamilies(peer.TrafficSelectors)
		for _, policyName := range policyNames {
			policy := cfg.Policies[policyName]
			if policy.Action != dataplane.SPActionDiscard {
				continue
			}
			if policy.Direction != ipsec.SPDDirIn && policy.Direction != ipsec.SPDDirBoth {
				continue
			}
			if !spansMeet(icmpSpans(&policy), families) {
				continue
			}
			return opererr.New(
				"ipsec peer "+strconv.Quote(peerName)+" has icmp-error-check true and `vpn ipsec policy "+policyName+"` discards inbound ICMP",
				"Linux delivers ICMP that arrives on the peer's Child SA and that the discard entry refuses, whenever the packet the ICMP message carries matches the SA, "+
					"so the entry does not stop that ICMP",
				"set vpn ipsec site-to-site peer "+peerName+" icmp-error-check false, or delete vpn ipsec policy "+policyName)
		}
	}
	return nil
}

// icmpSpans answers the address sets an operator entry covers on its REMOTE side, kept
// to the families in which its protocol matches ICMP: protocol 0 matches every
// protocol, 1 is ICMP in IPv4 and 58 is ICMPv6. Any other protocol matches no ICMP and
// answers no span.
func icmpSpans(policy *ipsec.SPDPolicy) []netip.Prefix {
	spans := sideSpans(policy.RemotePrefix, policy.LocalPrefix)
	if policy.Protocol == 0 {
		return spans
	}
	kept := make([]netip.Prefix, 0, len(spans))
	for _, span := range spans {
		if policy.Protocol == dataplane.UpperProtoICMP && span.Addr().Is4() {
			kept = append(kept, span)
		}
		if policy.Protocol == dataplane.UpperProtoICMPv6 && span.Addr().Is6() {
			kept = append(kept, span)
		}
	}
	return kept
}

// peerRemoteSpans answers the address sets a peer's remote traffic selectors cover. An
// EMPTY list is a peer that accepts any selector (SiteToSitePeer.TrafficSelectors), so
// it covers every address of both families.
func peerRemoteSpans(selectors []ipsec.TrafficSelectorPolicy) []netip.Prefix {
	if len(selectors) == 0 {
		return []netip.Prefix{anyIPv4, anyIPv6}
	}
	spans := make([]netip.Prefix, 0, len(selectors))
	for i := range selectors {
		spans = append(spans, sideSpans(selectors[i].RemotePrefix, selectors[i].LocalPrefix)...)
	}
	return spans
}

// peerSelectorFamilies answers the wildcard of each family a peer's selectors can
// carry: both for an EMPTY list, which accepts any selector.
func peerSelectorFamilies(selectors []ipsec.TrafficSelectorPolicy) []netip.Prefix {
	var v4, v6 bool
	if len(selectors) == 0 {
		v4, v6 = true, true
	}
	for i := range selectors {
		for _, span := range sideSpans(selectors[i].RemotePrefix, selectors[i].LocalPrefix) {
			v4 = v4 || span.Addr().Is4()
			v6 = v6 || span.Addr().Is6()
		}
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

// sideSpans answers the address set one selector side covers. A written prefix covers
// itself. A side that names none covers every address of the family its other side
// names, because one selector carries one family, and of both families when neither
// side names a prefix.
//
// A prefix that is not an address of either family cannot come from the parser, which
// builds each one with net.ParseCIDR. It is answered as both wildcards, so a malformed
// value draws a refusal rather than slipping past one.
func sideSpans(prefix, other *net.IPNet) []netip.Prefix {
	if prefix != nil {
		if span, ok := netipPrefix(prefix); ok {
			return []netip.Prefix{span}
		}
		return []netip.Prefix{anyIPv4, anyIPv6}
	}
	if other != nil {
		if span, ok := netipPrefix(other); ok {
			if span.Addr().Is4() {
				return []netip.Prefix{anyIPv4}
			}
			return []netip.Prefix{anyIPv6}
		}
	}
	return []netip.Prefix{anyIPv4, anyIPv6}
}

// netipPrefix converts a parsed prefix, an IPv4 address in its 16-octet form included.
func netipPrefix(n *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, bits := n.Mask.Size()
	if bits == 0 {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap()
	if addr.Is4() && bits == 128 {
		ones -= 96
	}
	if ones < 0 {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, ones).Masked(), true
}

// spansMeet reports whether some address is in both sets. Prefixes of different
// families never meet (netip.Prefix.Overlaps).
func spansMeet(a, b []netip.Prefix) bool {
	for _, x := range a {
		if slices.ContainsFunc(b, x.Overlaps) {
			return true
		}
	}
	return false
}

// icmpOverlapRefusals answers both refusals' first conflicts, A first, nil for each
// refusal that finds none. The config verify and the start log read the same two
// predicates through it.
func icmpOverlapRefusals(cfg *ipsec.IPsecConfig) []*opererr.Error {
	var refusals []*opererr.Error
	if refusal := refuseICMPErrorBypass(cfg); refusal != nil {
		refusals = append(refusals, refusal)
	}
	if refusal := refuseICMPCheckDiscard(cfg); refusal != nil {
		refusals = append(refusals, refusal)
	}
	return refusals
}

// verifyICMPOverlap is the verify half: it answers the first refusal, A before B. It is a
// step of validateIPsecSections (config.go), the offline verifier, so `ze config
// validate`, every commit door and the ike plugin's OnConfigVerify refuse a conflict
// with the entries it names and their fix. The answer is an *opererr.Error, which
// errors.As still finds under the wrap each caller adds.
func verifyICMPOverlap(cfg *ipsec.IPsecConfig) error {
	refusals := icmpOverlapRefusals(cfg)
	if len(refusals) == 0 {
		return nil
	}
	return refusals[0]
}

// warnICMPOverlapAtStart is the start-time half of verifyICMPOverlap. The configuration
// the daemon starts with is never verified, so a box booting a file that holds such a
// conflict installs it with no commit to refuse. The start says it, at Error level, with
// the same facts and fix as the refusal, as warnImplicitDiscardAtStart does for the
// default discard catch-all.
//
// Its code is config-plugin-verify because `ze doctor` reports the same conflict under
// that code: the config-semantics doctor check runs every plugin's offline verifier over
// the running configuration (config.ValidateSemantics), and validateIPsecSections calls
// verifyICMPOverlap. A doctor check of the ike plugin's own would report one conflict
// twice.
func warnICMPOverlapAtStart(cfg *ipsec.IPsecConfig, log *slog.Logger) {
	for _, refusal := range icmpOverlapRefusals(cfg) {
		log.LogAttrs(context.Background(), slog.LevelError, "ike: "+refusal.Problem(),
			refusal.LogAttrs(slog.String("code", diagnostic.CodeConfigPluginVerify))...)
	}
}
