//go:build ze_bgp

// Design: docs/architecture/exabgp-bridge.md -- the commands a 4.2 or 5.0 script writes
// Overview: old_commands_test.go -- the judge these rows exempt commands from
//
// The rows of oldAllowed up to the marker are copied, reason and pattern, from
// the ALLOWED table of the pin's qa/bin/test_old_commands (ExaBGP 685f772e):
// each is a change ExaBGP 6.0 made on purpose, so ExaBGP 6.0 compatibility is
// the new behavior (M3). The Python pattern
// `(?!.*\blabel\b)` has no RE2 form, so that row carries it as `unless`.
// Rows after the marker are ze's own, each with the RFC text or the pin source
// that justifies it.

package bridge

import (
	"regexp"
	"slices"
	"strings"
)

// oldAllowedRow exempts the commands of the named releases that match pattern
// and do not match unless.
type oldAllowedRow struct {
	releases string
	pattern  *regexp.Regexp
	unless   *regexp.Regexp
	reason   string
}

func oldAllow(releases, pattern, unless, reason string) oldAllowedRow {
	row := oldAllowedRow{releases: releases, pattern: regexp.MustCompile(pattern), reason: reason}
	if unless != "" {
		row.unless = regexp.MustCompile(unless)
	}
	return row
}

var oldAllowed = []oldAllowedRow{
	oldAllow("4.2", `^(announce|withdraw) flow route .*\b(discard|accept|rate-limit|redirect|copy|mark|action|redirect-to-nexthop)\b.*\b(discard|accept|rate-limit|redirect|copy|mark|action|extended-community|community|redirect-to-nexthop(-ietf)?)\b`, "",
		"4.2 kept only the first extended community of the `then` of a flow route: `discard mark 10` lost its mark; this tree sends every action, as 5.0 did"),
	oldAllow("4.2", `^announce route 2(2[4-9]|3\d)\.`, "",
		"a prefix in 224.0.0.0/4 is announced as SAFI 2 multicast since 5.0; 4.2 sent it as unicast"),
	oldAllow("4.2", `\bbgp-prefix-sid-srv6 \( (ipv6|vpn|l3vpn) `, "",
		"4.2's SRv6 SID (`( ipv6|vpn|l3vpn <sid> )`) followed an early draft; 5.0 replaced it with RFC 9252's `( l3-service|l2-service ... )`, and refuses it as this tree does"),
	oldAllow("4.2", `^(announce|withdraw) attributes? .*\bsplit\b`, "",
		"4.2 took `split` in `announce attribute(s)` and ignored it, sending each prefix whole; this tree announces the more specifics, as `announce route ... split` always did"),
	oldAllow("4.2", `\bextended-community .*\bextended-community\b`, "",
		"`extended-community` given twice: 4.2 sent the first list only, this tree sends both"),
	oldAllow("4.2", `\b(origin|target):\d+\.\d+\.\d+\.\d+:`, "",
		"4.2 read the IPv4 address of `target:<ip>:<n>` and `origin:<ip>:<n>` as a number and sent a two octet AS community (type 0x00); this tree sends type 0x01, RFC 4360 3.2"),
	oldAllow("5.0", `^(announce|withdraw) ipv[46] mpls-vpn `, "",
		"5.0 packed the VPN route of `announce ipv4|ipv6 mpls-vpn` in the unicast NLRI field, where a peer reads the label and the route distinguisher as the prefix; this tree sends it as SAFI 128, as `announce route ... rd ... label` did in 5.0"),
	oldAllow("4.2 5.0", `\b(redirect|copy) \d+\.\d+\.\d+\.\d+`, "",
		"a flow redirect to an address sends the draft-ietf-idr-flowspec-redirect-ip community, which carries the address, not the draft-simpson one with the address as next-hop: CHANGELOG \"Incompatible\", `redirect-simpson <ip>` sends the old bytes"),
	oldAllow("4.2 5.0", `^(neighbor \S+ )?withdraw (route|attributes?) .*\blabel\b`, "",
		"a labelled or VPN route is withdrawn with the one Compatibility field of RFC 8277 2.4, 0x800000, rather than the label it was given, whatever the Multiple Labels Capability: CHANGELOG \"Change\", a receiver ignores the field"), //nolint:misspell // verbatim quote of the pin's ALLOWED reason
	oldAllow("4.2 5.0", `^(announce|withdraw) (route|attributes?) .*\b(rd|route-distinguisher)\b`, `\blabel\b`,
		"5.0 sent a VPN route given without a label with no label field, so a peer read the route distinguisher as the label; this tree refuses to announce it, and withdraws it with the RFC 8277 2.4 Compatibility field (Label._with_label_field)"),
	oldAllow("4.2 5.0", `^(announce|withdraw) flow route .*\b(flow-label|traffic-class)\b`, "",
		"5.0 took the IPv6 only components on an IPv4 flow route, and sent traffic-class as an IPv4 dscp or flow-label as nothing; this tree builds an IPv6 flow route, or refuses the mixture"),
	oldAllow("4.2 5.0", `^announce flow route destination [0-9a-f]*:.*\bdscp\b`, "",
		"dscp is an IPv4 component (RFC 8955), traffic-class its IPv6 one (RFC 8956): 5.0 put it on an IPv6 flow route, this tree refuses it"),
	oldAllow("5.0", `^(announce|withdraw) ipv[46] \S+ .*\bsplit\b`, "",
		"5.0 took `split` in an announce family and ignored it, sending the prefix whole; this tree announces the more specifics, as `announce route ... split` always did"),
	oldAllow("4.2 5.0", `^(announce|withdraw) (ipv[46] (multicast|nlri-mpls|mpls-vpn|flow)|attributes? .*\b(rd|route-distinguisher|label)\b) .*\bnext-hop .*\bnext-hop\b`, "",
		"`next-hop` given twice: 5.0 sent the first for IPv4 unicast and the last for the MP families, this tree the first for both (static.first_next_hop)"),
	oldAllow("4.2 5.0", `\bpath-information$`, "",
		"`path-information` with no value: 5.0 ignored it, this tree refuses it"),
	oldAllow("4.2 5.0", `\bnext-hop fe80:`, "",
		"a link-local next-hop without the link-local next-hop capability: RFC 2545 wants a global one, 5.0 sent it, this tree refuses it (Neighbor.next_hop_refused)"),
	oldAllow("4.2 5.0", `\b(origin|target):(\d+L|\d{6,}|[7-9]\d{4}|6[6-9]\d{3}|65[6-9]\d\d|655[4-9]\d|6553[6-9]):`, "",
		"a four octet AS in a route target or origin: 5.0 sent type 0x01, an IPv4 address, this tree type 0x02 (RFC 5668); CHANGELOG"),
	// End of the rows copied from the pin. Ze's own rows follow.
}

// oldCommandAllowed reports whether a row exempts the command for the release.
func oldCommandAllowed(release, command string) bool {
	for i := range oldAllowed {
		row := &oldAllowed[i]
		if !slices.Contains(strings.Fields(row.releases), release) {
			continue
		}
		if !row.pattern.MatchString(command) {
			continue
		}
		if row.unless != nil && row.unless.MatchString(command) {
			continue
		}
		return true
	}
	return false
}
