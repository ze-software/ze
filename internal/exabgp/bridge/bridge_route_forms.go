// Design: docs/architecture/exabgp-bridge.md -- the route forms beyond `announce route`
// Overview: bridge_command.go -- the line translator that calls these
//
// ExaBGP writes a route four ways. `announce route <prefix> <attrs>` and
// `announce <afi> <safi> <nlri> <attrs>` are the two bridge_command.go reads.
// The other two are here: `announce attributes <attrs> nlri <p> <p>`, which
// carries one attribute set over several prefixes, and `announce eor <afi>
// <safi>`, which carries no route at all.

package bridge

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// errEORNeedsFamily is what a bare `announce eor` answers when the bridge was
// given no family list. ExaBGP sends one End-of-RIB per NEGOTIATED family, and
// ze's update-text spells the marker per family rather than for all of them at
// once, so the expansion needs to know which families there are.
var errEORNeedsFamily = errors.New("the ExaBGP bridge needs a family on `announce eor`")

// errSplitLength is what a `split` answers when its length cannot cut the
// prefix it is written on.
var errSplitLength = errors.New("invalid split length")

// convertEOR translates `announce eor <afi> <safi>` into ze's per-family
// End-of-RIB. RFC 4724 Section 2: the marker is an UPDATE with no reachable
// NLRI and no withdrawn routes.
//
// It reports false when the line is not an End-of-RIB.
func convertEOR(selector, rest string, families []string) ([]Command, bool, error) {
	fields := strings.Fields(strings.TrimSpace(rest))
	if len(fields) < 2 || !strings.EqualFold(fields[0], announceVerb) || !strings.EqualFold(fields[1], "eor") {
		return nil, false, nil
	}

	// A bare `announce eor` is every family, which ExaBGP reads as every family
	// the session negotiated. The bridge answers with the families it DECLARED,
	// which is the set it asked to negotiate.
	if len(fields) < 4 {
		if len(families) == 0 {
			return nil, true, errEORNeedsFamily
		}
		commands := make([]Command, 0, len(families))
		for _, family := range families {
			commands = append(commands, eorCommand(selector, family))
		}
		return commands, true, nil
	}

	afi := strings.ToLower(fields[2])
	safi := strings.ToLower(fields[3])
	if !bridgeAFI[afi] {
		return nil, true, fmt.Errorf("%w: %q names no address family", errEORNeedsFamily, fields[2])
	}
	if _, known := bridgeSAFI[safi]; !known {
		return nil, true, fmt.Errorf("%w: %q names no subsequent address family", errEORNeedsFamily, fields[3])
	}

	var tb textbuf.Buffer
	family := tb.Str(afi).Byte('/').Str(canonicalExabgpSAFI(safi)).String()
	return []Command{eorCommand(selector, family)}, true, nil
}

// eorCommand writes one per-family End-of-RIB.
//
// It carries the zero RouteKey. RFC 4724 Section 2 makes the marker an UPDATE
// with no reachable NLRI and no withdrawn routes, so it names no route: it
// cancels nothing in a batch and nothing in a batch cancels it.
func eorCommand(selector, family string) Command {
	var tb textbuf.Buffer
	return Command{Text: tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(family).Str(" eor").String()}
}

// bridgeAFI is the address family set ExaBGP names in a family-qualified line.
// Source: ExaBGP src/exabgp/configuration/neighbor/family.py.
var bridgeAFI = map[string]bool{bridgeAFIv4: true, bridgeAFIv6: true, "l2vpn": true}

// convertAttributesForm translates ExaBGP's `announce attributes <attrs> nlri
// <prefix>...`, which states one attribute set and then every prefix that
// carries it.
//
// ExaBGP accepts the keyword in both spellings, `attributes` and `attribute`,
// and its own qa/api corpus writes each of them. The family is not stated: it
// is READ OFF the attributes, because `rd` makes the route a VPN route and a
// bare `label` makes it a labeled one.
//
// It reports false when the line is not this form.
func convertAttributesForm(selector, rest, verb string) ([]Command, bool, error) {
	fields := strings.Fields(strings.TrimSpace(rest))
	if len(fields) < 2 {
		return nil, false, nil
	}
	if !strings.EqualFold(fields[1], "attributes") && !strings.EqualFold(fields[1], "attribute") {
		return nil, false, nil
	}

	body := fields[2:]
	cut := -1
	for i, token := range body {
		if strings.EqualFold(token, "nlri") {
			cut = i
			break
		}
	}
	if cut < 0 {
		return nil, true, fmt.Errorf("%w: the attributes form needs an `nlri` section", ErrLineNotTranslated)
	}

	attrs, err := parseRouteAttributes(body[:cut])
	if err != nil {
		return nil, true, err
	}
	prefixes := body[cut+1:]
	if len(prefixes) == 0 {
		return nil, true, fmt.Errorf("%w: the `nlri` section names no prefix", ErrLineNotTranslated)
	}
	if attrs.Withdraw {
		verb = nlriDel
	}

	// Every prefix carries the same attribute set, so it is ONE ze command
	// naming them all. Whether they share an UPDATE on the wire is the
	// destination peer's `behavior { group-updates }` leaf, not the bridge's.
	family := familyForAttributes(attrs, prefixes[0])
	prefixes = defaultPrefixLengths(family, prefixes)
	return oneCommandPerNLRI(selector, family, verb, attrs, prefixes, false), true, nil
}

// familyForAttributes reads the family off an attribute set and the first
// prefix that carries it. A route distinguisher makes it a VPN route, a bare
// label makes it a labeled one, and the prefix decides the AFI.
func familyForAttributes(attrs routeAttributes, prefix string) string {
	afi := bridgeAFIv4
	if strings.Contains(prefix, ":") {
		afi = bridgeAFIv6
	}
	safi := bridgeUnicastSAFI
	switch {
	case attrs.Has("rd"):
		safi = bridgeVPNSAFI
	case attrs.Has(bridgeAttrLabel):
		safi = bridgeLabeledSAFI
	}
	var tb textbuf.Buffer
	return tb.Str(afi).Byte('/').Str(safi).String()
}

// bridgePrefixSAFI are the SAFIs whose NLRI is a plain prefix, which is the set
// ExaBGP reads with its `prefix` parser rather than with a per-family one.
//
// The families NOT here carry a field list as their NLRI: mup writes `mup-isd
// 10.0.1.0/24`, mvpn writes `shared-join rp 10.99.199.1 group 239.251.255.228`,
// and flowspec writes a match block. Their bare addresses are FIELD VALUES, so
// giving one a prefix length would corrupt the route rather than complete it.
var bridgePrefixSAFI = map[string]bool{
	bridgeUnicastSAFI:   true,
	bridgeMulticastSAFI: true,
	bridgeLabeledSAFI:   true,
	bridgeVPNSAFI:       true,
}

// defaultPrefixLengths gives a bare address the host length ExaBGP gives it.
//
// ExaBGP writes a host route as an address alone. Its `prefix` parser splits on
// `/`, and on failure takes 32, or 128 when the address holds a colon
// (src/exabgp/configuration/static/parser.py). The API reuses that parser, so a
// script may write `announce route 1.2.3.4 next-hop 5.6.7.8` and mean
// 1.2.3.4/32. Ze reads a prefix and answers `invalid prefix: 1.2.3.4`, so the
// bridge completes the token rather than handing ze one it cannot read.
//
// api-check writes exactly that line, and it is the last thing between that
// case and the wire: its `.ci` expects the /32 form in the frame.
//
// It is called at the two points a token run becomes NLRI: buildRouteCommand,
// before `split /<n>` cuts a prefix that must already be one, and
// convertAttributesForm, whose `nlri` section names the prefixes itself.
func defaultPrefixLengths(family string, tokens []string) []string {
	_, safi, ok := strings.Cut(family, "/")
	if !ok || !bridgePrefixSAFI[safi] {
		return tokens
	}

	var completed []string
	for i, token := range tokens {
		address, err := netip.ParseAddr(token)
		if err != nil {
			continue
		}
		if completed == nil {
			completed = make([]string, len(tokens))
			copy(completed, tokens)
		}
		completed[i] = netip.PrefixFrom(address, address.BitLen()).String()
	}
	if completed == nil {
		return tokens
	}
	return completed
}

// splitPrefix cuts one prefix into every subnet of the given length, which is
// what ExaBGP's `split /<length>` writes.
//
// ExaBGP announces the pieces rather than the whole: `announce route
// 1.0.0.0/21 ... split /23` puts four /23 routes on the wire and no /21. A
// length shorter than the prefix's own cannot cut it, and a length that would
// produce more than splitLimit pieces is refused rather than expanded, because
// `split /32` on a /8 is 16 million NLRIs in one command.
func splitPrefix(prefix string, length int) ([]string, error) {
	parsed, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("%w: %q", errSplitLength, prefix)
	}
	bits := parsed.Addr().BitLen()
	if length < parsed.Bits() || length > bits {
		return nil, fmt.Errorf("%w: /%d does not cut %s", errSplitLength, length, prefix)
	}
	if length-parsed.Bits() > splitLimitBits {
		return nil, fmt.Errorf("%w: /%d cuts %s into more than %d pieces",
			errSplitLength, length, prefix, 1<<splitLimitBits)
	}

	count := 1 << (length - parsed.Bits())
	out := make([]string, 0, count)
	addr := parsed.Masked().Addr()
	for range count {
		out = append(out, netip.PrefixFrom(addr, length).String())
		addr = nextPrefixAddr(addr, length)
	}
	return out, nil
}

// splitLimitBits bounds a split to 2^10 pieces. ExaBGP has no such bound, and
// nothing in its corpus comes near one: the widest cut in qa/api is a /21 into
// /23s, which is four.
const splitLimitBits = 10

// nextPrefixAddr answers the first address of the subnet that follows the one
// of the given length starting at addr.
func nextPrefixAddr(addr netip.Addr, length int) netip.Addr {
	bytes := addr.AsSlice()
	carry := 1 << ((8 - length%8) % 8)
	for i := (length - 1) / 8; i >= 0; i-- {
		sum := int(bytes[i]) + carry
		bytes[i] = byte(sum)
		if sum < 256 {
			break
		}
		carry = 1
	}
	next, _ := netip.AddrFromSlice(bytes)
	return next
}
