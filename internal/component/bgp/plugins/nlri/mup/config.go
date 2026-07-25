// Design: docs/architecture/config/syntax.md -- MUP config route parsing
// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- MUP SAFI NLRI (SAFI 85)
// Related: register.go -- plugin registration
// Related: encode.go -- MUP NLRI encoder (EncodeNLRIHex)

package mup

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// MUP path attribute wire constants.
const (
	attrCodeNextHop   uint8 = 3  // NEXT_HOP (RFC 4271).
	attrCodeExtComm   uint8 = 16 // EXTENDED_COMMUNITIES (RFC 4360).
	attrCodePrefixSID uint8 = 40 // BGP Prefix-SID (RFC 8669/9252).

	flagWellKnownTrans = 0x40 // Well-known transitive.
	flagOptTrans       = 0xC0 // Optional transitive.
)

var errMUPMissingRouteType = errors.New("mup nlri requires a route type (mup-isd/mup-dsd/mup-t1st/mup-t2st)")

// parseConfigRoute implements registry.InProcessConfigRouteParser for MUP.
// It builds the family-specific NLRI from the content tokens and assembles the
// MUP-specific path attributes (NEXT_HOP code-3 for IPv4, extended-communities,
// Prefix-SID) from the pre-parsed attribute block. ORIGIN/AS_PATH/LOCAL_PREF/
// MP_REACH are owned by BuildPlugin.
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	family := "ipv4/mup"
	if req.IsIPv6 {
		family = "ipv6/mup"
	}

	args, err := mupArgsFromContent(req.Content)
	if err != nil {
		return registry.PluginRoute{}, err
	}

	nlriHex, err := EncodeNLRIHex(family, args)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("build MUP NLRI: %w", err)
	}
	nlri, err := hex.DecodeString(nlriHex)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("decode MUP NLRI hex: %w", err)
	}

	var attrs []registry.PluginRouteAttr

	// NEXT_HOP (code 3): IPv4 MUP with an IPv4 next-hop carries a legacy NEXT_HOP
	// attribute in addition to the MP_REACH next-hop (matches draft expectations).
	if !req.IsIPv6 && req.NextHop != "" {
		if ip, err := netip.ParseAddr(req.NextHop); err == nil && ip.Is4() {
			b := ip.As4()
			attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeNextHop, Flags: flagWellKnownTrans, Value: b[:]})
		}
	}

	// EXTENDED_COMMUNITIES (code 16): config order, unsorted (MUP convention).
	if len(req.ExtCommunity) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeExtComm, Flags: flagOptTrans, Value: req.ExtCommunity})
	}

	// BGP Prefix-SID (code 40): SRv6 SID information.
	if len(req.PrefixSID) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodePrefixSID, Flags: flagOptTrans, Value: req.PrefixSID})
	}

	return registry.PluginRoute{
		IsIPv6:       req.IsIPv6,
		NLRI:         nlri,
		NextHop:      req.NextHop,
		Attrs:        attrs,
		MapV4NextHop: true, // IPv6 MUP with an IPv4 next-hop uses IPv4-mapped IPv6.
	}, nil
}

// mupArgsFromContent converts NLRI content tokens into the key-value args
// EncodeNLRIHex expects. Content layout (after the operation keyword):
//
//	<route-type> <prefix|address> [rd RD] [teid TEID] [qfi QFI] [endpoint EP] [source SRC]
func mupArgsFromContent(content []string) ([]string, error) {
	if len(content) == 0 {
		return nil, errMUPMissingRouteType
	}

	routeType := content[0]
	args := []string{"route-type", routeType}

	// The token after the route type is the prefix (ISD/T1ST) or address (DSD/T2ST).
	if len(content) > 1 {
		switch routeType {
		case "mup-isd", "mup-t1st":
			args = append(args, "prefix", content[1])
		case "mup-dsd", "mup-t2st":
			args = append(args, "address", content[1])
		default:
			return nil, fmt.Errorf("unknown MUP route type %q", routeType)
		}
	}

	// Remaining key-value pairs.
	for i := 2; i < len(content); i += 2 {
		key := content[i]
		if i+1 >= len(content) {
			return nil, fmt.Errorf("missing value for %s", key)
		}
		val := content[i+1]
		switch key {
		case "rd", "teid", "qfi", "endpoint", "source":
			args = append(args, key, val)
		default:
			return nil, fmt.Errorf("unknown MUP keyword: %s", key)
		}
	}

	return args, nil
}
