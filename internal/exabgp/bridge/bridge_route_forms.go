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
func convertEOR(selector, rest string, families []string) ([]string, bool, error) {
	fields := strings.Fields(strings.TrimSpace(rest))
	if len(fields) < 2 || !strings.EqualFold(fields[0], "announce") || !strings.EqualFold(fields[1], "eor") {
		return nil, false, nil
	}

	// A bare `announce eor` is every family, which ExaBGP reads as every family
	// the session negotiated. The bridge answers with the families it DECLARED,
	// which is the set it asked to negotiate.
	if len(fields) < 4 {
		if len(families) == 0 {
			return nil, true, errEORNeedsFamily
		}
		commands := make([]string, 0, len(families))
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
	return []string{eorCommand(selector, family)}, true, nil
}

// eorCommand writes one per-family End-of-RIB.
func eorCommand(selector, family string) string {
	var tb textbuf.Buffer
	return tb.Str("send bgp ").Str(selector).Str(" update text nlri ").Str(family).Str(" eor").String()
}

// bridgeAFI is the address family set ExaBGP names in a family-qualified line.
// Source: ExaBGP src/exabgp/configuration/neighbor/family.py.
var bridgeAFI = map[string]bool{"ipv4": true, "ipv6": true, "l2vpn": true}

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
func convertAttributesForm(selector, rest, verb string) ([]string, bool, error) {
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
	return oneCommandPerNLRI(selector, family, verb, attrs, prefixes, false), true, nil
}

// familyForAttributes reads the family off an attribute set and the first
// prefix that carries it. A route distinguisher makes it a VPN route, a bare
// label makes it a labeled one, and the prefix decides the AFI.
func familyForAttributes(attrs routeAttributes, prefix string) string {
	afi := "ipv4"
	if strings.Contains(prefix, ":") {
		afi = "ipv6"
	}
	safi := "unicast"
	switch {
	case attrs.Has("rd"):
		safi = "mpls-vpn"
	case attrs.Has("label"):
		safi = "nlri-mpls"
	}
	var tb textbuf.Buffer
	return tb.Str(afi).Byte('/').Str(safi).String()
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
