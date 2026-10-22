// Design: docs/architecture/exabgp-bridge.md -- the route forms beyond `announce route`
// Overview: bridge_command.go -- the line translator that calls these
//
// ExaBGP writes a route four ways. `announce route <prefix> <attrs>` and
// `announce <afi> <safi> <nlri> <attrs>` are the two bridge_command.go reads.
// The other two are here: `announce attributes <attrs> nlri <p> <p>`, which
// carries one attribute set over several prefixes, and `announce eor <afi>
// <safi>`, which carries no route at all. `announce route-refresh <afi> <safi>`
// is here too: it shares the verb and the family words, and asks the peer for
// its routes rather than sending one.

package bridge

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// errEORNeedsFamily is what `announce eor` answers when it carries a family
// token count other than none or two, as ExaBGP's api_eor refuses it.
var errEORNeedsFamily = errors.New("the ExaBGP bridge needs `announce eor` alone or `announce eor <afi> <safi>`")

// errRouteRefreshNeedsFamily is what `announce route-refresh` answers when it
// does not name one address family and one subsequent address family: a
// ROUTE-REFRESH carries exactly one AFI/SAFI (RFC 2918 Section 3).
var errRouteRefreshNeedsFamily = errors.New("the ExaBGP bridge needs `announce route-refresh <afi> <safi>`")

// errSplitLength is what a `split` answers when its length cannot cut the
// prefix it is written on.
var errSplitLength = errors.New("invalid split length")

// convertEOR translates `announce eor <afi> <safi>` into ze's per-family
// End-of-RIB. RFC 4724 Section 2: the marker is an UPDATE with no reachable
// NLRI and no withdrawn routes.
//
// It reports false when the line is not an End-of-RIB.
func convertEOR(selector, rest string) ([]Command, bool, error) {
	fields := strings.Fields(strings.TrimSpace(rest))
	if len(fields) < 2 || !strings.EqualFold(fields[0], announceVerb) || !strings.EqualFold(fields[1], "eor") {
		return nil, false, nil
	}

	// A bare `announce eor` is IPv4 unicast alone: ExaBGP's api_eor
	// (src/exabgp/reactor/api/__init__.py) answers
	// `Family(AFI.ipv4, SAFI.unicast)` when no family token follows.
	if len(fields) == 2 {
		return []Command{eorCommand(selector, "ipv4/unicast")}, true, nil
	}
	if len(fields) != 4 {
		return nil, true, errEORNeedsFamily
	}

	afi := strings.ToLower(fields[2])
	safi := strings.ToLower(fields[3])
	family, err := pairFamily(errEORNeedsFamily, afi, safi)
	if err != nil {
		return nil, true, err
	}
	return []Command{eorCommand(selector, family)}, true, nil
}

// convertRouteRefresh translates `announce route-refresh <afi> <safi>` into ze's
// `request peer <selector> refresh <family>`, which sends the peer a
// ROUTE-REFRESH. RFC 2918 Section 3: "A BGP speaker may send a ROUTE-REFRESH
// message to its peer only if it has received the Route Refresh Capability
// from its peer"; ze's refresh handler holds that check.
//
// It reports false when the line is not a route refresh. It is read before
// convertRoute, whose `announce route` prefix would otherwise take the line as
// a route with the prefix "-refresh".
func convertRouteRefresh(selector, rest string) (Translation, bool, error) {
	fields := strings.Fields(strings.ToLower(rest))
	if len(fields) < 2 || fields[0] != announceVerb || fields[1] != "route-refresh" {
		return Translation{}, false, nil
	}
	if len(fields) != 4 {
		return Translation{}, true, errRouteRefreshNeedsFamily
	}
	afi := fields[2]
	safi := fields[3]
	family, err := pairFamily(errRouteRefreshNeedsFamily, afi, safi)
	if err != nil {
		return Translation{}, true, err
	}

	var tb textbuf.Buffer
	tb.Str("request peer ").Str(selector).Str(" refresh ").Str(family)
	return Translation{Commands: textCommands(tb.String()), Selector: selector}, true, nil
}

// eorCommand writes one per-family End-of-RIB.
//
// It carries the zero RouteKey. RFC 4724 Section 2 makes the marker an UPDATE
// with no reachable NLRI and no withdrawn routes, so it names no route: it
// cancels nothing in a batch and nothing in a batch cancels it.
func eorCommand(selector, family string) Command {
	var tb textbuf.Buffer
	// RFC 4724 Section 2: IPv4 unicast's End-of-RIB is ze's `nlri eor`, every
	// other family's `mp <family> eor`.
	tb.Str("send bgp ").Str(selector).Str(" update text")
	if textparse.LegacyNLRISection(family, false) {
		tb.Str(" nlri")
	} else {
		tb.Str(" mp ").Str(family)
	}
	return Command{Text: tb.Str(" eor").String()}
}

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

	// Every prefix carries the same attribute set, so it is ONE ze command
	// naming them all. Whether they share an UPDATE on the wire is the
	// destination peer's `behavior { group-updates }` leaf, not the bridge's.
	family := familyForAttributes(attrs, prefixes[0])
	// The family is known only once the attributes are read, and it decides
	// how an IPv4 next hop is sent on an IPv6 route (familyNextHop), so the
	// attributes are read again under it.
	attrs, err = parseFamilyRouteAttributes(family, body[:cut])
	if err != nil {
		return nil, true, err
	}
	prefixes, err = defaultPrefixLengths(family, prefixes)
	if err != nil {
		return nil, true, err
	}
	// 5.0 cuts EACH prefix of the `nlri` section into its `split /<n>` pieces
	// (qa/old/commands-5.0.json: `split /25 nlri 10.0.0.0/24 20.0.0.0/24` packs
	// four /25 routes). 4.2 ignored the word here; the pin's ALLOWED table keeps
	// the more specifics for it too.
	if attrs.Split > 0 {
		prefixes, err = splitPrefixes(prefixes, &attrs)
		if err != nil {
			return nil, true, err
		}
	}
	warnRouteLine(attrs)
	return oneCommandPerNLRI(selector, family, verb, attrs, prefixes, false), true, nil
}

// familyForAttributes reads the family off an attribute set and the first
// prefix that carries it. A route distinguisher makes it a VPN route, a bare
// label makes it a labeled one, and the prefix decides the AFI and, for a
// group address, the multicast SAFI (prefixSAFI).
func familyForAttributes(attrs routeAttributes, prefix string) string {
	afi := bridgeAFIv4
	if strings.Contains(prefix, ":") {
		afi = bridgeAFIv6
	}
	safi := prefixSAFI(prefix)
	switch {
	case attrs.Has("rd"):
		safi = bridgeVPNSAFI
	case attrs.Has(bridgeAttrLabel):
		// ze's word for SAFI 4: the family goes into update text, which
		// knows no `nlri-mpls`.
		safi = zeLabeledSAFI
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
//
// It is keyed by ze's SAFI word, because defaultPrefixLengths reads the family
// the bridge has already built for update text.
var bridgePrefixSAFI = map[string]bool{
	bridgeUnicastSAFI:   true,
	bridgeMulticastSAFI: true,
	zeLabeledSAFI:       true,
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
func defaultPrefixLengths(family string, tokens []string) ([]string, error) {
	_, safi, ok := strings.Cut(family, "/")
	if !ok || !bridgePrefixSAFI[safi] {
		return tokens, nil
	}

	var completed []string
	for i, token := range tokens {
		if err := refuseMaskNotNumber(token); err != nil {
			return nil, err
		}
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
		return tokens, nil
	}
	return completed, nil
}

// errMaskNotNumber is what a prefix whose mask is no number answers.
var errMaskNotNumber = errors.New("the mask of a prefix is no number")

// refuseMaskNotNumber refuses a prefix written with a mask that is no number,
// `10.0.0.0/x`. ExaBGP 5.0 took it as the host route 10.0.0.0/32; 6.0 refuses
// it in the configuration and on the API (X-29), so the bridge refuses it in
// ExaBGP's terms rather than handing ze a token whose refusal names no mask.
//
// A token whose part before the slash is no address answers nil on purpose: it
// is not a prefix with a bad mask, and the NLRI run carries such tokens (an RD,
// a label), which ze judges in its own terms.
func refuseMaskNotNumber(token string) error {
	address, mask, ok := strings.Cut(token, "/")
	if !ok {
		return nil
	}
	if !isAddress(address) {
		return nil
	}
	if _, err := strconv.ParseUint(mask, 10, 8); err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w: %q has the mask %q; write the prefix length as a number, %s/<length>",
		ErrLineNotTranslated, errMaskNotNumber, token, mask, address)
}

// isAddress answers whether text is one IPv4 or IPv6 address. The parse error
// is the answer "no", not a failure to report.
func isAddress(text string) bool {
	_, err := netip.ParseAddr(text)
	return err == nil
}

// splitPrefix cuts one prefix into every subnet of the given length, which is
// what ExaBGP's `split /<length>` writes.
//
// ExaBGP announces the pieces rather than the whole: `announce route
// 1.0.0.0/21 ... split /23` puts four /23 routes on the wire and no /21. A
// length shorter than the prefix's own cuts nothing: ExaBGP's API sends the
// prefix itself and logs it, as 4.2 and 5.0 did (`split`,
// configuration/grammar/tree/static.py, X-27), so whole is set and the one
// piece is the prefix. A length longer than an address, or one that would
// produce more than splitLimit pieces, is refused rather than expanded,
// because `split /32` on a /8 is 16 million NLRIs in one command.
func splitPrefix(prefix string, length int) (pieces []string, whole bool, err error) {
	parsed, err := netip.ParsePrefix(prefix)
	if err != nil {
		return nil, false, fmt.Errorf("%w: %q", errSplitLength, prefix)
	}
	if length < parsed.Bits() {
		return []string{prefix}, true, nil
	}
	bits := parsed.Addr().BitLen()
	if length > bits {
		return nil, false, fmt.Errorf("%w: /%d is longer than an address of %s; write a length up to /%d",
			errSplitLength, length, prefix, bits)
	}
	if length-parsed.Bits() > splitLimitBits {
		return nil, false, fmt.Errorf("%w: /%d cuts %s into more than %d pieces",
			errSplitLength, length, prefix, 1<<splitLimitBits)
	}

	count := 1 << (length - parsed.Bits())
	out := make([]string, 0, count)
	addr := parsed.Masked().Addr()
	for range count {
		out = append(out, netip.PrefixFrom(addr, length).String())
		addr = nextPrefixAddr(addr, length)
	}
	return out, false, nil
}

// splitPrefixes cuts every prefix into its attrs.Split pieces, in the order
// the prefixes were written, and records a warning for each prefix a length
// shorter than its own leaves whole (splitPrefix).
func splitPrefixes(prefixes []string, attrs *routeAttributes) ([]string, error) {
	var pieces []string
	for _, prefix := range prefixes {
		cut, whole, err := splitPrefix(prefix, attrs.Split)
		if err != nil {
			return nil, err
		}
		if whole {
			var tb textbuf.Buffer
			tb.Str("split /").Int(int64(attrs.Split)).Str(" is shorter than the prefix ").
				Str(prefix).Str(", so the prefix is sent whole")
			attrs.Warnings = append(attrs.Warnings, tb.String())
		}
		pieces = append(pieces, cut...)
	}
	return pieces, nil
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
