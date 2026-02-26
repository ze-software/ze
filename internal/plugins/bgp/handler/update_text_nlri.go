// Design: docs/architecture/api/commands.md — NLRI section parsing for update text command
// Related: update_text.go — shared text attribute parsing types and helpers
// Related: update_text_flowspec.go — FlowSpec NLRI parsing
// Related: update_text_vpls.go — VPLS NLRI parsing
// Related: update_text_evpn.go — EVPN NLRI parsing
package handler

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	labeled "codeberg.org/thomas-mangin/ze/internal/plugins/bgp-nlri-labeled"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
)

// parseNLRISection parses nlri <family> [rd <value>] [label <value>] <nlri-op>+
// <nlri-op> := add <prefix>+ [watchdog set <name>] | del <prefix>+
// accum contains NLRI accumulators: pathID, RD, labels.
// In-NLRI modifiers (rd/label without 'set') override accumulated values.
// Returns family, announce list, withdraw list, watchdog name, consumed token count, and any error.
func parseNLRISection(args []string, accum nlriAccum) (nlriParseResult, error) {
	// args[0] = "nlri"
	if len(args) < 2 {
		return nlriParseResult{}, route.ErrInvalidFamily
	}

	family, ok := nlri.ParseFamily(args[1])
	if !ok {
		return nlriParseResult{}, fmt.Errorf("%w: %s", route.ErrInvalidFamily, args[1])
	}

	// Check if family is supported (EOR is supported for all families)
	isEOR := len(args) > 2 && args[2] == kwEOR
	if !isEOR && !isSupportedFamily(family) {
		return nlriParseResult{}, fmt.Errorf("%w: %s", route.ErrFamilyNotSupported, args[1])
	}

	// RFC 4724: End-of-RIB marker
	// Syntax: nlri <family> eor
	if isEOR {
		return nlriParseResult{Family: family, Consumed: 3}, nil // Return empty lists with family set - signals EOR
	}

	// FlowSpec families use different parsing (components instead of prefixes)
	// RFC 8955 Section 4: FlowSpec NLRI = ordered list of match components
	if family.SAFI == nlri.SAFIFlowSpec || family.SAFI == nlri.SAFIFlowSpecVPN {
		return parseFlowSpecSection(args, family)
	}

	// VPLS families use different parsing (multi-field NLRI)
	// RFC 4761 Section 3.2.2: VPLS BGP NLRI format
	if family.SAFI == nlri.SAFIVPLS {
		return parseVPLSSection(args, family, accum)
	}

	// EVPN families use different parsing (route-type based)
	// RFC 7432: EVPN route types
	if family.SAFI == nlri.SAFIEVPN {
		return parseEVPNSection(args, family, accum)
	}

	consumed := 2 // "nlri" + family
	i := 2

	// Parse in-NLRI modifiers: rd <value>, label <value> (without 'set')
	// These override accumulated values for this nlri section only
	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		if token == kwRD {
			// rd <value> (in-NLRI modifier, no 'set')
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("rd requires value (ASN:NN or IP:NN)")
			}
			next := args[i+1]
			// If next token is 'set', this is accumulator syntax - don't handle here
			if next == kwSet {
				break
			}
			rd, err := nlri.ParseRDString(next)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid rd: %w", err)
			}
			accum.RD = rd
			i += 2
			consumed += 2
			continue
		}

		if token == kwLabel {
			// label <value> (in-NLRI modifier, no 'set')
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("label requires value (0-1048575)")
			}
			next := args[i+1]
			// If next token is 'set', this is accumulator syntax - don't handle here
			if next == kwSet {
				break
			}
			label, err := strconv.ParseUint(next, 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid label: %w", err)
			}
			if label > 0xFFFFF { // 20-bit max
				return nlriParseResult{}, fmt.Errorf("label out of range (max 1048575): %d", label)
			}
			accum.Labels = []uint32{uint32(label)} //nolint:gosec // G115: bounded by check above
			i += 2
			consumed += 2
			continue
		}

		// Not an in-NLRI modifier, proceed to add/del parsing
		break
	}

	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI
	var watchdog string

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section (except watchdog which is handled specially)
		if isBoundaryKeyword(token) && token != kwWatchdog {
			break
		}

		// Watchdog inside nlri section: watchdog set <name>
		if token == kwWatchdog {
			if mode != kwAdd {
				return nlriParseResult{}, errors.New("watchdog only valid after 'add' in nlri section")
			}
			if i+2 >= len(args) {
				return nlriParseResult{}, errors.New("watchdog requires 'set <name>'")
			}
			if args[i+1] != kwSet {
				return nlriParseResult{}, fmt.Errorf("watchdog requires 'set', got: %s", args[i+1])
			}
			watchdog = args[i+2]
			i += 3
			consumed += 3
			continue
		}

		// Mode switches
		if token == kwAdd {
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have a mode before prefixes
		if mode == "" {
			return nlriParseResult{}, fmt.Errorf("%w: got %q", route.ErrMissingAddDel, token)
		}

		// Skip NLRI type keyword ("prefix") — emitted by NLRI String() in event
		// format and included in withdrawal commands from bgp-rr.
		if token == "prefix" {
			i++
			consumed++
			continue
		}

		// Parse prefix based on family
		n, extra, err := parseNLRI(token, family, accum)
		if err != nil {
			return nlriParseResult{}, err
		}

		if mode == kwAdd {
			announce = append(announce, n)
		} else {
			withdraw = append(withdraw, n)
		}
		i += 1 + extra
		consumed += 1 + extra
	}

	// Must have at least one prefix
	if len(announce) == 0 && len(withdraw) == 0 {
		return nlriParseResult{}, route.ErrEmptyNLRISection
	}

	return nlriParseResult{Family: family, Announce: announce, Withdraw: withdraw, Watchdog: watchdog, Consumed: consumed}, nil
}

// parseINETNLRI parses a single prefix for unicast/multicast families.
// pathID is the ADD-PATH path identifier (0 = not set).
// Returns the NLRI, extra args consumed (always 0 for INET), and any error.
//
//nolint:unparam // extra return used by other NLRI parsers with same signature
func parseINETNLRI(token string, family nlri.Family, pathID uint32) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, family)
	}

	return nlri.NewINET(family, prefix, pathID), 0, nil // 0 extra args consumed
}

// parseNLRI dispatches to the appropriate NLRI parser based on family.
// Returns NLRI, extra args consumed, and any error.
// For FlowSpec families, this function is not used - see parseFlowSpecSection.
func parseNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	switch family.SAFI { //nolint:exhaustive // Other SAFIs use INET parser via default
	case nlri.SAFIVPN: // SAFI 128 - MPLS VPN
		return parseVPNNLRI(token, family, accum)
	case nlri.SAFIMPLSLabel: // SAFI 4 - Labeled Unicast
		return parseLabeledNLRI(token, family, accum)
	case nlri.SAFIFlowSpec, nlri.SAFIFlowSpecVPN:
		// FlowSpec uses special parsing - should not reach here
		return nil, 0, fmt.Errorf("flowspec parsing requires parseFlowSpecSection")
	default: // INET unicast/multicast families
		return parseINETNLRI(token, family, accum.PathID)
	}
}

// parseVPNNLRI parses a prefix for VPN families (SAFI 128).
// Requires RD and labels from accum.
// RFC 4364 Section 4.3.4: VPN NLRI = labels + RD + prefix.
func parseVPNNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, family)
	}

	// Require RD for VPN families
	if accum.RD.Type == 0 && accum.RD.Value == [6]byte{} {
		return nil, 0, fmt.Errorf("%w: rd required for %s", route.ErrMissingRD, family)
	}

	// Require at least one label for VPN families
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", route.ErrMissingLabel, family)
	}

	// Build args for registry encoder: "rd <val> label <val> ... prefix <val> [path-id <val>]"
	encodeArgs := []string{"rd", accum.RD.String()}
	for _, l := range accum.Labels {
		encodeArgs = append(encodeArgs, "label", strconv.FormatUint(uint64(l), 10))
	}
	encodeArgs = append(encodeArgs, "prefix", prefix.String())
	if accum.PathID != 0 {
		encodeArgs = append(encodeArgs, "path-id", strconv.FormatUint(uint64(accum.PathID), 10))
	}

	hexStr, err := registry.EncodeNLRIByFamily(family.String(), encodeArgs)
	if err != nil {
		return nil, 0, fmt.Errorf("vpn encode: %w", err)
	}
	wireBytes, err := hex.DecodeString(strings.ToLower(hexStr))
	if err != nil {
		return nil, 0, fmt.Errorf("vpn hex decode: %w", err)
	}
	wire, err := nlri.NewWireNLRI(family, wireBytes, accum.PathID != 0)
	if err != nil {
		return nil, 0, err
	}
	return wire, 0, nil
}

// parseLabeledNLRI parses a prefix for labeled unicast families (SAFI 4).
// Requires labels from accum.
// RFC 8277: Labeled Unicast NLRI = labels + prefix.
func parseLabeledNLRI(token string, family nlri.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && family.AFI != nlri.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, family)
	}
	if !isIPv4 && family.AFI != nlri.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, family)
	}

	// Require at least one label for labeled unicast
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", route.ErrMissingLabel, family)
	}

	return labeled.NewLabeledUnicast(family, prefix, accum.Labels, accum.PathID), 0, nil
}

// isSupportedFamily returns true if the family is supported in text mode.
func isSupportedFamily(f nlri.Family) bool {
	switch f {
	case nlri.IPv4Unicast, nlri.IPv6Unicast, nlri.IPv4Multicast, nlri.IPv6Multicast:
		return true
	case nlri.IPv4VPN, nlri.IPv6VPN: // VPN families (SAFI 128)
		return true
	case nlri.IPv4LabeledUnicast, nlri.IPv6LabeledUnicast: // Labeled unicast (SAFI 4)
		return true
	case nlri.IPv4FlowSpec, nlri.IPv6FlowSpec: // FlowSpec (SAFI 133) - RFC 8955
		return true
	case nlri.IPv4FlowSpecVPN, nlri.IPv6FlowSpecVPN: // FlowSpec VPN (SAFI 134) - RFC 8955
		return true
	case nlri.L2VPNVPLS: // VPLS (SAFI 65) - RFC 4761
		return true
	case nlri.L2VPNEVPN: // EVPN (SAFI 70) - RFC 7432
		return true
	default: // unsupported family
		return false
	}
}
