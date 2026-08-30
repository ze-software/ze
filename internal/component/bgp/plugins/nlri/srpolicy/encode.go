// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI route encoding
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format (SAFI 73)
// Related: config.go -- SR-Policy config route parser shared by route encoding

package srpolicy

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errSRPolicyMissingRouteCommand = errors.New("missing SR-Policy route command")

// EncodeNLRIHex encodes the SR-Policy key NLRI fields and returns uppercase hex.
// Args format: ["distinguisher", "0", "color", "100", "endpoint", "10.0.0.1"].
// This implements the InProcessNLRIEncoder signature for the plugin registry.
func EncodeNLRIHex(famName string, args []string) (string, error) {
	afi, err := srPolicyAFI(famName)
	if err != nil {
		return "", err
	}

	distinguisher, color, endpoint, err := parseSRPolicyNLRIArgs(args)
	if err != nil {
		return "", err
	}
	if err := validateSRPolicyEndpoint(afi, endpoint); err != nil {
		return "", err
	}

	return textbuf.StringHexUpper(New(afi, distinguisher, color, endpoint).Bytes()), nil
}

// EncodeRoute encodes an SR-Policy route command into UPDATE bytes and NLRI bytes.
// The owner package reuses parseConfigRoute so config and canonical route encoding
// share SR-Policy field validation and Tunnel Encapsulation attribute construction.
func EncodeRoute(routeCmd, famName string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	_ = addPath // SR-Policy NLRIs do not support ADD-PATH.
	afi, err := srPolicyAFI(famName)
	if err != nil {
		return nil, nil, err
	}
	isIPv6 := afi == family.AFIIPv6

	content, nextHop, err := splitSRPolicyRouteCommand(routeCmd)
	if err != nil {
		return nil, nil, err
	}

	pr, err := parseConfigRoute(registry.ConfigRouteRequest{Content: content, NextHop: nextHop, IsIPv6: isIPv6})
	if err != nil {
		return nil, nil, err
	}

	nh, err := netip.ParseAddr(pr.NextHop)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid next-hop: %w", err)
	}

	rawAttrs := make([][]byte, 0, len(pr.Attrs))
	for i := range pr.Attrs {
		a := &pr.Attrs[i]
		rawAttrs = append(rawAttrs, srPolicyAttrWire(a.Flags, a.Code, a.Value))
	}

	ub := message.GetUpdateBuilder(localAS, isIBGP, asn4, addPath)
	defer message.PutUpdateBuilder(ub)

	params := message.PluginParams{
		AFI:          uint16(afi),
		SAFI:         byte(family.SAFISRPolicy),
		IsIPv6:       isIPv6,
		NLRI:         pr.NLRI,
		NextHop:      nh,
		RawAttrs:     rawAttrs,
		MapV4NextHop: true,
	}
	update := ub.BuildPlugin(params)
	return message.PackTo(update, nil), pr.NLRI, nil
}

func srPolicyAFI(famName string) (family.AFI, error) {
	fam, ok := family.LookupFamily(famName)
	if !ok {
		return 0, fmt.Errorf("unknown family: %s", famName)
	}
	if fam.SAFI != family.SAFISRPolicy {
		return 0, fmt.Errorf("unsupported family for SR-Policy: %s", famName)
	}
	return fam.AFI, nil
}

func parseSRPolicyNLRIArgs(args []string) (uint32, uint32, netip.Addr, error) {
	var distinguisher, color uint32
	var endpoint netip.Addr
	var hasDistinguisher, hasColor, hasEndpoint bool

	for i := 0; i < len(args); i++ {
		key := args[i]
		switch key {
		case fieldDistinguisher:
			i++
			if i >= len(args) {
				return 0, 0, netip.Addr{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return 0, 0, netip.Addr{}, fmt.Errorf("invalid distinguisher: %w", err)
			}
			distinguisher = uint32(v)
			hasDistinguisher = true
		case fieldColor:
			i++
			if i >= len(args) {
				return 0, 0, netip.Addr{}, fmt.Errorf("missing value for %s", key)
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return 0, 0, netip.Addr{}, fmt.Errorf("invalid color: %w", err)
			}
			color = uint32(v)
			hasColor = true
		case fieldEndpoint:
			i++
			if i >= len(args) {
				return 0, 0, netip.Addr{}, fmt.Errorf("missing value for %s", key)
			}
			ep, err := netip.ParseAddr(args[i])
			if err != nil {
				return 0, 0, netip.Addr{}, fmt.Errorf("invalid endpoint: %w", err)
			}
			endpoint = ep
			hasEndpoint = true
		default:
			return 0, 0, netip.Addr{}, fmt.Errorf("unknown sr-policy keyword: %s", key)
		}
	}

	if !hasDistinguisher || !hasColor || !hasEndpoint {
		return 0, 0, netip.Addr{}, errSRPolicyMissingFields
	}
	return distinguisher, color, endpoint, nil
}

func validateSRPolicyEndpoint(afi family.AFI, endpoint netip.Addr) error {
	if afi == family.AFIIPv6 {
		if !endpoint.Is6() || endpoint.Is4In6() {
			return fmt.Errorf("sr-policy endpoint %s is not IPv6", endpoint)
		}
		return nil
	}
	if !endpoint.Is4() {
		return fmt.Errorf("sr-policy endpoint %s is not IPv4", endpoint)
	}
	return nil
}

func splitSRPolicyRouteCommand(routeCmd string) ([]string, string, error) {
	fields := strings.Fields(routeCmd)
	if len(fields) == 0 {
		return nil, "", errSRPolicyMissingRouteCommand
	}
	if fields[0] == "route" {
		fields = fields[1:]
	}

	content := make([]string, 0, len(fields))
	var nextHop string
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "next-hop", "nhop":
			key := fields[i]
			i++
			if i >= len(fields) {
				return nil, "", fmt.Errorf("missing value for %s", key)
			}
			nextHop = fields[i]
		default:
			content = append(content, fields[i])
		}
	}
	if nextHop == "" {
		return nil, "", fmt.Errorf("missing value for next-hop")
	}
	return content, nextHop, nil
}

func srPolicyAttrWire(flags, code uint8, value []byte) []byte {
	vlen := len(value)
	if vlen > 255 || (flags&0x10) != 0 {
		buf := make([]byte, 4+vlen)
		buf[0] = flags | 0x10
		buf[1] = code
		buf[2] = byte(vlen >> 8)
		buf[3] = byte(vlen)
		copy(buf[4:], value)
		return buf
	}
	buf := make([]byte, 3+vlen)
	buf[0] = flags
	buf[1] = code
	buf[2] = byte(vlen)
	copy(buf[3:], value)
	return buf
}
