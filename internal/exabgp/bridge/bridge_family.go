// Design: docs/architecture/core-design.md — ExaBGP text command to ZeBGP translation
// Overview: bridge_command.go — the command grammar that reads a family off a line
// Related: bridge_event.go — the JSON encoder that renders a family key back

package bridge

import (
	"regexp"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The SAFI names the bridge translates a route into. Most SAFIs carry the same
// word in both vocabularies, and a name below is that shared word. Where the
// two differ the ze word and the ExaBGP word are separate constants, because a
// family name built from the wrong one names no family at all.
const (
	bridgeUnicastSAFI   = "unicast"
	bridgeMulticastSAFI = "multicast"
	bridgeVPNSAFI       = "mpls-vpn"
	bridgeMUPSAFI       = "mup"
	bridgeFlowSAFI      = "flow"
	bridgeFlowVPNSAFI   = "flow-vpn"
)

// The two SAFIs whose word differs between the vocabularies, each named once
// per side. ExaBGP's are SAFI._names (src/exabgp/protocol/family.py); ze's are
// the second half of what family.MustRegister composed
// (internal/component/bgp/plugins/nlri/labeled/types.go, .../mvpn/types.go).
//
// SAFI 4 was `nlri-mpls` on BOTH sides until 2026-09-19, and the single
// constant was read as ze's word at three sites. Each built `ipv4/nlri-mpls`,
// which family.LookupFamily answers no family for, so a labeled route from a
// script reached ze under a name ze does not know.
const (
	bridgeLabeledSAFI = "nlri-mpls"
	zeLabeledSAFI     = "mpls-label"
	bridgeMVPNSAFI    = "mcast-vpn"
	zeMVPNSAFI        = "mvpn"
)

// The AFI ExaBGP writes for the two IP families the bridge translates.
const (
	bridgeAFIv4 = "ipv4"
	bridgeAFIv6 = "ipv6"
)

// bridgeSAFI maps the SAFI an ExaBGP script writes to the one ze names. Most
// are the same word; the rows that differ are the whole reason a mapping exists
// rather than a passthrough.
var bridgeSAFI = map[string]string{
	bridgeUnicastSAFI:   bridgeUnicastSAFI,
	bridgeMulticastSAFI: bridgeMulticastSAFI,
	bridgeLabeledSAFI:   zeLabeledSAFI,
	bridgeFlowSAFI:      bridgeFlowSAFI,
	"flowspec":          bridgeFlowSAFI,
	bridgeFlowVPNSAFI:   bridgeFlowVPNSAFI,
	"flowspec-vpn":      bridgeFlowVPNSAFI,
	bridgeMVPNSAFI:      zeMVPNSAFI,
	bridgeMUPSAFI:       bridgeMUPSAFI,
}

// exabgpSAFI is bridgeSAFI read the other way: the word ExaBGP writes for the
// SAFI ze names. The JSON encoder renders a family key with it, because a
// script reads `ipv4 nlri-mpls` where ze's own event says `ipv4/mpls-label`.
//
// Only the renamed SAFIs are rows. A word absent here is the same on both
// sides and passes through, which is every SAFI but these two.
// TestSAFIVocabularyRoundTrips holds each row to bridgeSAFI, so the two
// directions cannot drift apart.
var exabgpSAFI = map[string]string{
	zeLabeledSAFI: bridgeLabeledSAFI,
	zeMVPNSAFI:    bridgeMVPNSAFI,
}

var (
	// bridgeSRPolicyRE matches an SR-Policy route, which states the AFI and
	// then the policy fields in place of a prefix.
	bridgeSRPolicyRE = regexp.MustCompile(`(?i)^(ipv[46])\s+sr-policy\s+(.+)$`)

	// bridgeFamilyRE matches a route that states its family as an AFI and a
	// SAFI. It captures both and the route that follows them.
	//
	// The alternation is BUILT from bridgeSAFI rather than written beside it. It
	// was a second copy of that vocabulary until 2026-09-05, and the two had
	// drifted: mcast-vpn was in neither, so every one of api-mvpn's fourteen
	// frames was refused by a translator that could name the family perfectly
	// well once it reached the mapping.
	bridgeFamilyRE = regexp.MustCompile(`(?i)^(ipv[46])\s+(` + bridgeSAFIAlternation() + `)\s+(.+)$`)
)

// bridgeSAFIAlternation renders bridgeSAFI's keys as a regexp alternation,
// longest first so a reader never has to reason about which branch wins.
func bridgeSAFIAlternation() string {
	names := make([]string, 0, len(bridgeSAFI))
	for name := range bridgeSAFI {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	return strings.Join(names, "|")
}

// canonicalExabgpSAFI answers the SAFI ze names for the one an ExaBGP script
// wrote. An unmapped word is returned unchanged, because the regexp that
// selected it was built from the same map and cannot offer one.
func canonicalExabgpSAFI(safi string) string {
	if canonical, ok := bridgeSAFI[safi]; ok {
		return canonical
	}
	return safi
}

// safiFamily joins an AFI and a SAFI into the family ze names.
func safiFamily(afi, safi string) string {
	var tb textbuf.Buffer
	return tb.Str(afi).Byte('/').Str(safi).String()
}

// exabgpSAFIName answers the SAFI ExaBGP writes for the one ze names, which is
// the same word for every SAFI but the two exabgpSAFI holds.
func exabgpSAFIName(safi string) string {
	if renamed, ok := exabgpSAFI[safi]; ok {
		return renamed
	}
	return safi
}

// exabgpFamilyName renders a ze family name as the phrase ExaBGP writes for it:
// the two halves separated by a space rather than a slash, and the SAFI in
// ExaBGP's vocabulary. "ipv4/mpls-label" becomes "ipv4 nlri-mpls".
//
// A name with no slash is not a family and is returned unchanged, so a key the
// encoder was handed by mistake reaches the reader as itself rather than as an
// invented family.
func exabgpFamilyName(zeFamily string) string {
	afi, safi, ok := strings.Cut(zeFamily, "/")
	if !ok {
		return zeFamily
	}
	var tb textbuf.Buffer
	return tb.Str(afi).Byte(' ').Str(exabgpSAFIName(safi)).String()
}
