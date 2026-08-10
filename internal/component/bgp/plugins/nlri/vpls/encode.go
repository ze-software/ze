// Design: docs/architecture/wire/nlri.md — VPLS NLRI plugin
// RFC: rfc/short/rfc4761.md
// Related: config.go — VPLS config route parser (uses EncodeNLRIHex)

package vpls

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errRdRequiresValue            = errors.New("rd requires value")
	errVeIdRequiresValue          = errors.New("ve-id requires value")
	errVeBlockOffsetRequiresValue = errors.New("ve-block-offset requires value")
	errVeBlockSizeRequiresValue   = errors.New("ve-block-size requires value")
	errLabelRequiresValue         = errors.New("label requires value")
	errRdRequiredForVpls          = errors.New("rd required for VPLS")
	errMissingVplsCommand         = errors.New("missing VPLS command")
	errMissingRouteDistinguisher  = errors.New("missing route-distinguisher")
)

// EncodeNLRIHex encodes VPLS NLRI from CLI-style args and returns uppercase hex.
// Args format: ["rd", "1:1", "ve-id", "1", "ve-block-offset", "0", "ve-block-size", "10", "label-base", "100"]
// This implements the InProcessNLRIEncoder signature for the plugin registry.
func EncodeNLRIHex(family string, args []string) (string, error) {
	if family != familyVPLS {
		return "", fmt.Errorf("unsupported family for VPLS: %s", family)
	}

	var rd RouteDistinguisher
	var veID, veBlockOffset, veBlockSize uint16
	var labelBase uint32
	var hasRD bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "rd":
			i++
			if i >= len(args) {
				return "", errRdRequiresValue
			}
			parsed, err := ParseRDString(args[i])
			if err != nil {
				return "", fmt.Errorf("invalid rd: %w", err)
			}
			rd = parsed
			hasRD = true
		case "ve-id":
			i++
			if i >= len(args) {
				return "", errVeIdRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 16)
			if err != nil {
				return "", fmt.Errorf("invalid ve-id: %w", err)
			}
			veID = uint16(v) //nolint:gosec // validated by ParseUint with bitSize 16
		case "ve-block-offset":
			i++
			if i >= len(args) {
				return "", errVeBlockOffsetRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 16)
			if err != nil {
				return "", fmt.Errorf("invalid ve-block-offset: %w", err)
			}
			veBlockOffset = uint16(v) //nolint:gosec // validated by ParseUint with bitSize 16
		case "ve-block-size":
			i++
			if i >= len(args) {
				return "", errVeBlockSizeRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 16)
			if err != nil {
				return "", fmt.Errorf("invalid ve-block-size: %w", err)
			}
			veBlockSize = uint16(v) //nolint:gosec // validated by ParseUint with bitSize 16
		case "label-base", "label":
			i++
			if i >= len(args) {
				return "", errLabelRequiresValue
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return "", fmt.Errorf("invalid label: %w", err)
			}
			labelBase = uint32(v) //nolint:gosec // validated by ParseUint with bitSize 32
		default:
			return "", fmt.Errorf("unknown VPLS keyword: %s", args[i])
		}
	}

	if !hasRD {
		return "", errRdRequiredForVpls
	}

	v := newVPLSFull(rd, veID, veBlockOffset, veBlockSize, labelBase)
	return textbuf.StringHexUpper(v.Bytes()), nil
}

// EncodeRoute encodes a VPLS route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, _ string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	ub := message.GetUpdateBuilder(localAS, isIBGP, asn4, addPath)
	defer message.PutUpdateBuilder(ub)

	// Parse route command
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, errMissingVplsCommand
	}

	// Parse using VPLS argument parser
	parsed, err := parseVPLSArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Parse RD
	rd, err := ParseRDString(parsed.RD)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid RD: %w", err)
	}

	// Build the VPLS NLRI (RFC 4761). VE-ID matches the block offset.
	vplsNLRI := newVPLSFull(rd, parsed.VEBlockOffset, parsed.VEBlockOffset, parsed.VEBlockSize, parsed.LabelBase)
	nlriBytes := vplsNLRI.Bytes()

	// Build UPDATE via the generic plugin builder (L2VPN AFI 25, SAFI 65).
	params := message.PluginParams{
		AFI:     25,
		SAFI:    65,
		NLRI:    nlriBytes,
		NextHop: parsed.NextHop,
	}
	update := ub.BuildPlugin(params)
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// parseVPLSArgs parses VPLS command arguments for encode command.
// Format: rd <rd> ve-block-offset <n> ve-block-size <n> label <n> next-hop <addr>.
func parseVPLSArgs(args []string) (bgptypes.VPLSRoute, error) {
	var route bgptypes.VPLSRoute
	if len(args)%2 != 0 {
		return route, fmt.Errorf("missing value for %s", args[len(args)-1])
	}

	for i := 0; i < len(args)-1; i += 2 {
		key := strings.ToLower(args[i])
		value := args[i+1]

		switch key {
		case "rd":
			route.RD = value
		case "ve-block-offset":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-offset: %s", value)
			}
			route.VEBlockOffset = uint16(n)
		case "ve-block-size":
			n, err := strconv.ParseUint(value, 10, 16)
			if err != nil {
				return route, fmt.Errorf("invalid ve-block-size: %s", value)
			}
			route.VEBlockSize = uint16(n)
		case "label-base", "label":
			n, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return route, fmt.Errorf("invalid label: %s", value)
			}
			route.LabelBase = uint32(n)
		case "next-hop":
			nh, err := netip.ParseAddr(value)
			if err != nil {
				return route, fmt.Errorf("invalid next-hop: %s", value)
			}
			route.NextHop = nh

		default:
			return route, fmt.Errorf("unknown vpls keyword: %s", key)
		}
	}

	if route.RD == "" {
		return route, errMissingRouteDistinguisher
	}

	return route, nil
}
