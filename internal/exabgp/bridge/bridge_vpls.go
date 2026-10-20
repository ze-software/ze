// Design: docs/architecture/exabgp-bridge.md -- ExaBGP command translation
// RFC: rfc/short/rfc4761.md -- VPLS BGP NLRI (Section 3.2.2)
// Related: bridge_command.go -- convertFamilyRoute, which hands a `vpls` route here
// Related: bridge_attribute.go -- parseRouteAttributes, the attribute tail

package bridge

import (
	"errors"
	"fmt"
	"strings"
)

// bridgeVPLSFamily is ze's name for the family ExaBGP's `vpls` route announces:
// AFI 25 (L2VPN), SAFI 65 (VPLS), RFC 4761 Section 3.2.2.
const bridgeVPLSFamily = "l2vpn/vpls"

// bridgeVPLSKeyword is the word that names the route in ExaBGP's API:
// `announce vpls endpoint 5 base 10702 offset 1 size 8 rd 192.168.201.1:123
// next-hop 192.168.201.1 ...` (ExaBGP 4.2 and 5.0
// src/exabgp/configuration/announce/vpls.py, AnnounceVPLS).
const bridgeVPLSKeyword = "vpls"

// bridgeVPLSFields maps each ExaBGP NLRI keyword (AnnounceVPLS.assign) to the
// ze `mp l2vpn/vpls` keyword for the same RFC 4761 Section 3.2.2 field, in the
// order ze writes them. Every one is required: ExaBGP cannot pack a VPLS NLRI
// with a field unset, and ze refuses one without its route distinguisher.
var bridgeVPLSFields = [...]struct{ exabgp, ze string }{
	{"rd", "rd"},
	{"endpoint", "ve-id"},
	{"offset", "ve-block-offset"},
	{"size", "ve-block-size"},
	{"base", "label-base"},
}

var errVPLSFieldMissing = errors.New("vpls route is missing an NLRI field")

// convertVPLSRoute translates ExaBGP's `vpls` route into ze's
// `update text <attributes> mp l2vpn/vpls <verb> rd <rd> ve-id <n>
// ve-block-offset <n> ve-block-size <n> label-base <n>`.
//
// It reports false when rest does not begin with the `vpls` keyword. ExaBGP
// writes the NLRI fields and the attributes in one run in any order, so the
// five NLRI keywords are taken out by name and every other token goes to
// parseRouteAttributes, which refuses what it does not read.
func convertVPLSRoute(selector, rest, verb string) ([]Command, bool, error) {
	keyword, body, _ := strings.Cut(rest, " ")
	if !strings.EqualFold(keyword, bridgeVPLSKeyword) {
		return nil, false, nil
	}

	fields := make(map[string]string, len(bridgeVPLSFields))
	parts := strings.Fields(body)
	attrTokens := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		name := strings.ToLower(parts[i])
		if name == "route-distinguisher" {
			name = "rd"
		}
		if !isVPLSField(name) {
			attrTokens = append(attrTokens, parts[i])
			continue
		}
		if i+1 >= len(parts) {
			return nil, true, fmt.Errorf("vpls %s: %w", name, errMissingAttrValue)
		}
		i++
		fields[name] = parts[i]
	}

	nlriTokens := make([]string, 0, 2*len(bridgeVPLSFields))
	for _, field := range bridgeVPLSFields {
		value, present := fields[field.exabgp]
		if !present {
			return nil, true, fmt.Errorf("%w: %s", errVPLSFieldMissing, field.exabgp)
		}
		nlriTokens = append(nlriTokens, field.ze, value)
	}

	attrs, err := parseRouteAttributes(attrTokens)
	if err != nil {
		return nil, true, err
	}
	// ExaBGP's `withdraw` flag announces the route in its withdrawn state.
	if attrs.Withdraw {
		verb = nlriDel
	}
	return oneCommandPerNLRI(selector, bridgeVPLSFamily, verb, attrs, nlriTokens, false), true, nil
}

// isVPLSField reports whether an ExaBGP keyword names a VPLS NLRI field.
func isVPLSField(name string) bool {
	for _, field := range bridgeVPLSFields {
		if field.exabgp == name {
			return true
		}
	}
	return false
}
