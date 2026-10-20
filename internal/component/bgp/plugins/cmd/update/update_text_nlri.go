// Design: docs/architecture/api/commands.md — NLRI section parsing for update text command
// Overview: update_text.go — shared text attribute parsing types and helpers
// Related: update_text_flowspec.go — FlowSpec NLRI parsing
// Related: update_text_vpls.go — VPLS NLRI parsing
// Related: update_text_evpn.go — EVPN NLRI parsing
package update

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/parse"
)

var errFlowspecParsingRequiresParseflowspecsection = errors.New("flowspec parsing requires parseFlowSpecSection")

// parseNLRISection parses one NLRI section of the update text grammar. The
// keyword names the UPDATE section the prefixes travel in, the same two
// keywords the filter text protocol uses:
//
//	nlri [modifier]* <nlri-op>+            NLRI and Withdrawn Routes fields, IPv4 unicast
//	nlri eor                               IPv4 unicast End-of-RIB
//	mp <family> [modifier]* <nlri-op>+     MP_REACH_NLRI and MP_UNREACH_NLRI
//	mp <family> eor                        End-of-RIB for that family
//
// <modifier> := path-information <id> | rd <value> | label <value>
// <nlri-op>  := add <prefix>+ [watchdog set <name>] | del <prefix>+
//
// args[0] is kwNLRI or kwMP. accum carries the path-id, RD and label
// accumulators; nextHop is the section's next hop, which decides whether an
// IPv4 unicast announcement can use the section it names.
func parseNLRISection(args []string, accum nlriAccum, nextHop bgptypes.RouteNextHop) (nlriParseResult, error) {
	if args[0] == kwMP {
		return parseMPSection(args, accum, nextHop)
	}
	return parseLegacySection(args, accum, nextHop)
}

// parseLegacySection parses `nlri ...`: IPv4 unicast in the UPDATE's own NLRI
// and Withdrawn Routes fields.
//
// RFC 4271 Section 4.3: "This variable length field contains a list of IP
// address prefixes." The field has no AFI or SAFI, so the section takes no
// family token, and `nlri <family>` is refused with the form that replaced it.
func parseLegacySection(args []string, accum nlriAccum, nextHop bgptypes.RouteNextHop) (nlriParseResult, error) {
	if len(args) > 1 {
		if fam, isFamily := family.LookupFamily(args[1]); isFamily {
			return nlriParseResult{}, removedFamilyFormError(fam)
		}
		// RFC 4724 Section 2: "For the IPv4 unicast address family, the
		// End-of-RIB marker is an UPDATE message with the minimum length".
		if args[1] == kwEOR {
			return nlriParseResult{Family: family.IPv4Unicast, Consumed: 2}, nil
		}
	}
	result, err := parseINETSection(args, 1, family.IPv4Unicast, accum)
	if err != nil {
		return nlriParseResult{}, err
	}
	// RFC 4271 Section 5.1.3: the NEXT_HOP attribute of the NLRI field holds an
	// IPv4 address. An IPv6 next hop for IPv4 unicast travels in MP_REACH_NLRI
	// (RFC 8950 Section 3), which is the mp section.
	if len(result.Announce) > 0 && isExplicitIPv6(nextHop) {
		return nlriParseResult{}, fmt.Errorf("nlri add cannot carry next-hop %s: the NLRI field takes its next hop from the NEXT_HOP attribute, which holds an IPv4 address (RFC 4271 Section 5.1.3); an IPv6 next hop for IPv4 unicast travels in MP_REACH_NLRI (RFC 8950 Section 3), so write: mp ipv4/unicast add <prefix>", nextHop.Addr)
	}
	return result, nil
}

// removedFamilyFormError refuses `nlri <family>`, the form the two section
// keywords replaced, and names the form to write instead.
func removedFamilyFormError(fam family.Family) error {
	if fam == family.IPv4Unicast {
		return fmt.Errorf("nlri %s is no longer accepted: nlri takes no family and always means IPv4 unicast in the NLRI and Withdrawn Routes fields, so write: nlri add|del <prefix>, or nlri eor (mp %s add carries an IPv6 next hop in MP_REACH_NLRI)", fam, fam)
	}
	return fmt.Errorf("nlri %s is no longer accepted: nlri means IPv4 unicast in the NLRI and Withdrawn Routes fields, and %s travels in MP_REACH_NLRI and MP_UNREACH_NLRI, so write: mp %s add|del ..., or mp %s eor", fam, fam, fam, fam)
}

// isExplicitIPv6 reports whether the section names an IPv6 next-hop address.
// next-hop self resolves per destination peer, so the parser cannot judge it.
func isExplicitIPv6(nextHop bgptypes.RouteNextHop) bool {
	return nextHop.Policy == bgptypes.NextHopExplicit && nextHop.Addr.Is6()
}

// parseMPSection parses `mp <family> ...`: the family's NLRI in MP_REACH_NLRI
// and MP_UNREACH_NLRI.
//
// RFC 4760 Section 2: "The first one (MP_REACH_NLRI) is used to carry the set
// of reachable destinations together with the next hop information to be used
// for forwarding to these destinations. The second one (MP_UNREACH_NLRI) is
// used to carry the set of unreachable destinations." Every family takes this
// section, IPv4 unicast only where checkMPIPv4Unicast admits it.
func parseMPSection(args []string, accum nlriAccum, nextHop bgptypes.RouteNextHop) (nlriParseResult, error) {
	result, err := parseMPFamilySection(args, accum, nextHop)
	if err != nil {
		return nlriParseResult{}, err
	}
	if result.Family == family.IPv4Unicast {
		if err := checkMPIPv4Unicast(result, nextHop); err != nil {
			return nlriParseResult{}, err
		}
	}
	return result, nil
}

// checkMPIPv4Unicast refuses an `mp ipv4/unicast` section the encoder would
// not write in MP_REACH_NLRI or MP_UNREACH_NLRI. The reactor places IPv4
// unicast in MP_REACH_NLRI only for an IPv6 next hop (RFC 8950 Section 3,
// reactor inlineIPv4Unicast) and withdraws it in the Withdrawn Routes field,
// so any other `mp ipv4/unicast` section would name a section the wire does
// not carry.
func checkMPIPv4Unicast(result nlriParseResult, nextHop bgptypes.RouteNextHop) error {
	if len(result.Announce) == 0 && len(result.Withdraw) == 0 {
		return errors.New("mp ipv4/unicast eor is refused: the IPv4 unicast End-of-RIB is an UPDATE with no NLRI and no attributes (RFC 4724 Section 2), not an MP_UNREACH_NLRI, so write: nlri eor")
	}
	if len(result.Withdraw) > 0 {
		return errors.New("mp ipv4/unicast del is refused: ze withdraws IPv4 unicast in the Withdrawn Routes field (RFC 4271 Section 4.3), so write: nlri del <prefix>")
	}
	if !isExplicitIPv6(nextHop) {
		return errors.New("mp ipv4/unicast add needs an explicit IPv6 next hop: ze sends IPv4 unicast with any other next hop in the NLRI field (RFC 4271 Section 4.3), so write: nlri add <prefix>, or give next-hop <ipv6-address> (RFC 8950 Section 3)")
	}
	return nil
}

// parseMPFamilySection parses `mp <family> [modifier]* <nlri-op>+` and
// `mp <family> eor`, dispatching each family to the parser that owns its NLRI
// text.
func parseMPFamilySection(args []string, accum nlriAccum, nextHop bgptypes.RouteNextHop) (nlriParseResult, error) {
	// args[0] = "mp"
	if len(args) < 2 {
		return nlriParseResult{}, route.ErrInvalidFamily
	}

	fam, ok := family.LookupFamily(args[1])
	if !ok {
		return nlriParseResult{}, fmt.Errorf("%w: %s", route.ErrInvalidFamily, args[1])
	}

	// Check if family is supported (EOR is supported for all families)
	isEOR := len(args) > 2 && args[2] == kwEOR
	if !isEOR && !isSupportedFamily(fam) {
		return nlriParseResult{}, fmt.Errorf("%w: %s", route.ErrFamilyNotSupported, args[1])
	}

	// RFC 4724: End-of-RIB marker
	// Syntax: mp <family> eor
	if isEOR {
		return nlriParseResult{Family: fam, Consumed: 3}, nil // Return empty lists with family set - signals EOR
	}

	// FlowSpec families use different parsing (components instead of prefixes)
	// RFC 8955 Section 4: FlowSpec NLRI = ordered list of match components
	if fam.SAFI == family.SAFIFlowSpec || fam.SAFI == family.SAFIFlowSpecVPN {
		return parseFlowSpecSection(args, fam)
	}

	// VPLS families use different parsing (multi-field NLRI)
	// RFC 4761 Section 3.2.2: VPLS BGP NLRI format
	if fam.SAFI == family.SAFIVPLS {
		return parseVPLSSection(args, fam, accum)
	}

	// EVPN families use different parsing (route-type based)
	// RFC 7432: EVPN route types
	if fam.SAFI == family.SAFIEVPN {
		return parseEVPNSection(args, fam, accum)
	}

	// A family this package does not parse itself reaches its registered plugin
	// encoder, which takes the WHOLE token run rather than one prefix per
	// token: `mp ipv4/mup add route-type mup-isd rd 100:100 prefix 10.0.0.0/24`
	// is one NLRI written as eight tokens.
	if !nativeTextFamily(fam) {
		return parseRegistryNLRISection(args, fam, accum, nextHop)
	}

	return parseINETSection(args, 2, fam, accum)
}

// parseINETSection parses the modifiers and add/del runs of a section whose
// family this package parses one prefix per token. start is the index of the
// first token after the section header (`nlri`, or `mp <family>`).
func parseINETSection(args []string, start int, fam family.Family, accum nlriAccum) (nlriParseResult, error) {
	consumed := start
	i := start

	// Parse in-NLRI modifiers: path-information <id>, rd <value>, label <value>
	// These set per-section values (info/path-information is per-NLRI-section, not top-level).
	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		if token == kwPathInfo {
			// path-information <id> (alias-resolved from "info")
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("path-information requires a value")
			}
			// RFC 7911 Section 3
			id, err := parse.PathID(args[i+1])
			if err != nil {
				return nlriParseResult{}, err
			}
			accum.PathID = id
			accum.HasPathID = true
			i += 2
			consumed += 2
			continue
		}

		if token == kwRD {
			// rd <value> (in-NLRI modifier)
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("rd requires value (ASN:NN or IP:NN)")
			}
			rd, err := nlri.ParseRDString(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid rd: %w", err)
			}
			accum.RD = rd
			i += 2
			consumed += 2
			continue
		}

		if token == kwLabel {
			// label <value> | label [ <value>... ] (in-NLRI modifier). The stack
			// is read by the same parser the top-level keyword uses
			// (update_text.go parseLabelStack), so the two positions accept the
			// same values.
			if i+1 >= len(args) {
				return nlriParseResult{}, errLabelRequiresValue
			}
			labels, used, err := parseLabelStack(args[i+1:])
			if err != nil {
				return nlriParseResult{}, err
			}
			accum.Labels = labels
			i += 1 + used
			consumed += 1 + used
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

		// Watchdog inside an NLRI section: watchdog set <name>
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
		// format and included in withdrawal commands from bgp-rs.
		if token == "prefix" {
			i++
			consumed++
			continue
		}

		// Parse prefix based on family. A labeled withdrawal keeps one label of
		// the stack, which the session writer sets to 0x800000: RFC 8277 Section
		// 2.4 gives a withdrawn NLRI one Compatibility field, "used whether or not
		// the Multiple Labels Capability has been sent or received on the
		// session", and a second label would reach the peer as prefix bits.
		fields := accum
		if mode == kwDel && len(accum.Labels) > 1 {
			fields.Labels = accum.Labels[:1]
		}
		n, extra, err := parseNLRI(token, fam, fields)
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

	return nlriParseResult{Family: fam, Announce: announce, Withdraw: withdraw, Watchdog: watchdog, Consumed: consumed}, nil
}

// parseINETNLRI parses a single prefix for unicast/multicast families.
// pathID is the ADD-PATH path identifier (0 = not set).
// Returns the NLRI, extra args consumed (always 0 for INET), and any error.
//
//nolint:unparam // extra return used by other NLRI parsers with same signature
func parseINETNLRI(token string, fam family.Family, pathID uint32) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && fam.AFI != family.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, fam)
	}
	if !isIPv4 && fam.AFI != family.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, fam)
	}

	return nlri.NewINET(fam, prefix, pathID), 0, nil // 0 extra args consumed
}

// parseNLRI dispatches to the appropriate NLRI parser based on family.
// Returns NLRI, extra args consumed, and any error.
// For FlowSpec families, this function is not used - see parseFlowSpecSection.
func parseNLRI(token string, fam family.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	//exhaustive:ignore // parseNLRISection calls this only for a nativeTextFamily family it has not already handed to its own section parser (FlowSpec, VPLS, EVPN), so only unicast, multicast, VPN and labeled unicast arrive; every other SAFI goes to parseRegistryNLRISection.
	switch fam.SAFI {
	case family.SAFIVPN: // SAFI 128 - MPLS VPN
		return parseVPNNLRI(token, fam, accum)
	case family.SAFIMPLSLabel: // SAFI 4 - Labeled Unicast
		return parseLabeledNLRI(token, fam, accum)
	case family.SAFIFlowSpec, family.SAFIFlowSpecVPN:
		// FlowSpec uses special parsing - should not reach here
		return nil, 0, errFlowspecParsingRequiresParseflowspecsection
	case family.SAFIUnicast, family.SAFIMulticast:
		return parseINETNLRI(token, fam, accum.PathID)
	default:
		// No other SAFI reaches here (see the ignore reason above); refuse
		// rather than guess a prefix encoding for it.
		return nil, 0, fmt.Errorf("%w: %s", route.ErrFamilyNotSupported, fam)
	}
}

// parseVPNNLRI parses a prefix for VPN families (SAFI 128).
// Requires RD and labels from accum.
// RFC 4364 Section 4.3.4: VPN NLRI = labels + RD + prefix.
func parseVPNNLRI(token string, fam family.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && fam.AFI != family.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, fam)
	}
	if !isIPv4 && fam.AFI != family.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, fam)
	}

	// Require RD for VPN families
	if accum.RD.Type == 0 && accum.RD.Value == [6]byte{} {
		return nil, 0, fmt.Errorf("%w: rd required for %s", route.ErrMissingRD, fam)
	}

	// Require at least one label for VPN families
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", route.ErrMissingLabel, fam)
	}

	// Build args for registry encoder: "rd <val> label <val> ... prefix <val> [path-id <val>]"
	encodeArgs := []string{"rd", accum.RD.String()}
	for _, l := range accum.Labels {
		encodeArgs = append(encodeArgs, "label", strconv.FormatUint(uint64(l), 10))
	}
	encodeArgs = append(encodeArgs, "prefix", prefix.String())
	if accum.HasPathID {
		encodeArgs = append(encodeArgs, "path-id", strconv.FormatUint(uint64(accum.PathID), 10))
	}

	wire, err := encodeViaRegistry(fam, encodeArgs, accum.PathID, accum.HasPathID)
	if err != nil {
		return nil, 0, err
	}
	return wire, 0, nil
}

// parseLabeledNLRI parses a prefix for labeled unicast families (SAFI 4).
// Requires labels from accum.
// RFC 8277: Labeled Unicast NLRI = labels + prefix.
func parseLabeledNLRI(token string, fam family.Family, accum nlriAccum) (nlri.NLRI, int, error) {
	prefix, err := netip.ParsePrefix(token)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %s", route.ErrInvalidPrefix, token)
	}

	// Validate prefix matches family AFI
	isIPv4 := prefix.Addr().Is4()
	if isIPv4 && fam.AFI != family.AFIIPv4 {
		return nil, 0, fmt.Errorf("%w: IPv4 prefix for %s", route.ErrFamilyMismatch, fam)
	}
	if !isIPv4 && fam.AFI != family.AFIIPv6 {
		return nil, 0, fmt.Errorf("%w: IPv6 prefix for %s", route.ErrFamilyMismatch, fam)
	}

	// Require at least one label for labeled unicast
	if len(accum.Labels) == 0 {
		return nil, 0, fmt.Errorf("%w: label required for %s", route.ErrMissingLabel, fam)
	}

	// Build args for registry encoder: "prefix" <val> "label" <val>... ["path-id" <val>]
	encodeArgs := []string{"prefix", prefix.String()}
	for _, l := range accum.Labels {
		encodeArgs = append(encodeArgs, "label", strconv.FormatUint(uint64(l), 10))
	}
	if accum.HasPathID {
		encodeArgs = append(encodeArgs, "path-id", strconv.FormatUint(uint64(accum.PathID), 10))
	}

	wire, err := encodeViaRegistry(fam, encodeArgs, accum.PathID, accum.HasPathID)
	if err != nil {
		return nil, 0, err
	}
	return wire, 0, nil
}

// encodeViaRegistry encodes NLRI args via the plugin registry and wraps as WireNLRI.
// Common encode tail for all family-specific section parsers.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// A registered encoder answers with the NLRI payload alone: vpn.EncodeNLRIHex
// and labeled.EncodeNLRIHex each return the type's Bytes(), which excludes the
// identifier by contract. So the four octets are prepended here, and the flag
// handed to NewWireNLRI describes the bytes this function built rather than the
// bytes the encoder returned.
//
// hasPathID is the operator's keyword, not a test on pathID. The field reserves
// no value, so `path-information 0` names identifier zero and a route written
// with it MUST reach the peer carrying four zero octets.
func encodeViaRegistry(fam family.Family, args []string, pathID uint32, hasPathID bool) (nlri.NLRI, error) {
	hexStr, err := registry.EncodeNLRIByFamily(fam.String(), args)
	if err != nil {
		return nil, fmt.Errorf("%s encode: %w", fam, err)
	}
	payload, err := hex.DecodeString(strings.ToLower(hexStr))
	if err != nil {
		return nil, fmt.Errorf("%s hex decode: %w", fam, err)
	}
	return wireNLRI(fam, payload, pathID, hasPathID)
}

// wireNLRI wraps one NLRI's bytes, with the ADD-PATH path identifier in front
// when the command gave one (RFC 7911 Section 3).
func wireNLRI(fam family.Family, payload []byte, pathID uint32, hasPathID bool) (nlri.NLRI, error) {
	if !hasPathID {
		return nlri.NewWireNLRI(fam, payload, false)
	}
	wireBytes := make([]byte, 4+len(payload)) // command path: result owned by the NLRI
	binary.BigEndian.PutUint32(wireBytes, pathID)
	copy(wireBytes[4:], payload)
	return nlri.NewWireNLRI(fam, wireBytes, true)
}

// buildNLRIResult constructs a section parse result with empty-check validation.
func buildNLRIResult(fam family.Family, announce, withdraw []nlri.NLRI, consumed int) (nlriParseResult, error) {
	if len(announce) == 0 && len(withdraw) == 0 {
		return nlriParseResult{}, route.ErrEmptyNLRISection
	}
	return nlriParseResult{Family: fam, Announce: announce, Withdraw: withdraw, Consumed: consumed}, nil
}

// buildSingleNLRIResult wraps one NLRI into announce or withdraw based on mode.
// Used by section parsers (EVPN, VPLS) that produce exactly one NLRI.
func buildSingleNLRIResult(fam family.Family, mode string, n nlri.NLRI, consumed int) (nlriParseResult, error) {
	switch mode {
	case kwAdd:
		return buildNLRIResult(fam, []nlri.NLRI{n}, nil, consumed)
	case kwDel:
		return buildNLRIResult(fam, nil, []nlri.NLRI{n}, consumed)
	default: // no mode set — neither add nor del was specified
		return nlriParseResult{}, route.ErrEmptyNLRISection
	}
}

// isSupportedFamily returns true if the family is supported in text mode.
//
// The switch names the families this package parses ITSELF. Everything else is
// asked of the plugin registry rather than listed a second time here: the two
// declarations had drifted, and mup and mvpn were refused with "family not
// supported in text mode" by a parser standing next to a registered encoder
// that could have answered (ai/rules/principles.md).
func isSupportedFamily(f family.Family) bool {
	if nativeTextFamily(f) {
		return true
	}
	return registry.HasNLRIEncoderForFamily(f.String())
}

// nativeTextFamily reports whether this package parses the family's NLRI with
// a parser of its own, rather than handing the tokens to a plugin encoder.
func nativeTextFamily(f family.Family) bool {
	switch f {
	case family.IPv4Unicast,
		family.IPv6Unicast,
		family.IPv4Multicast,
		family.IPv6Multicast:
		return true
	case family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIVPN}: // VPN families (SAFI 128)
		return true
	case family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel},
		family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIMPLSLabel}: // Labeled unicast (SAFI 4)
		return true
	case family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
		family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}: // FlowSpec (SAFI 133) - RFC 8955
		return true
	case family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
		family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN}: // FlowSpec VPN (SAFI 134) - RFC 8955
		return true
	case family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}: // VPLS (SAFI 65) - RFC 4761
		return true
	case family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}: // EVPN (SAFI 70) - RFC 7432
		return true
	default: // unsupported family
		return false
	}
}

// errLaterAddDel refuses an add or del written after the first field of a
// registry family section. The section is one NLRI, so a second mode would
// either be read as a value by the plugin encoder or silently win.
var errLaterAddDel = errors.New("add or del after the first field: write one add or del right after the family")

// errFamilyRefusesAddPath refuses a path-information that applies to a section
// whose plugin declared its NLRI carries no ADD-PATH identifier
// (registry.Registration.NLRIAddPathRefusal).
var errFamilyRefusesAddPath = errors.New("path-information not supported")

// parseRegistryNLRISection parses an NLRI section whose family this package
// does not parse itself, by handing the section's whole token run to the
// plugin encoder registered for it.
//
// The section is `add <token>...` or `del <token>...`, and it ends at the next
// boundary keyword. The tokens go to the encoder unread, because their grammar
// belongs to the plugin that declared the family: the mup encoder reads
// `route-type mup-isd rd 100:100 prefix 10.0.0.0/24`
// (internal/component/bgp/plugins/nlri/mup/encode.go, EncodeNLRIHex) and the
// mvpn encoder reads its own.
//
// A family whose plugin declared that its NLRI carries no ADD-PATH identifier
// refuses a path-information in exactly three positions: one at top level,
// which the accumulator it was handed carries; one right after the family; and
// one inside the run. A path-information inside another family's section lives
// in that section's own accumulator copy, so it never reaches this parser.
//
// A family whose plugin declared that its section writes path attributes
// (registry.SectionRouteParserFor) hands an `add` run to its config-route
// parser instead, which answers the NLRI and those attributes. A `del` run
// names the NLRI alone, so it stays with the NLRI encoder.
func parseRegistryNLRISection(args []string, fam family.Family, accum nlriAccum, nextHop bgptypes.RouteNextHop) (nlriParseResult, error) {
	// args[0] = "mp", args[1] = family; the caller has consumed neither.
	refusal, refusesAddPath := registry.NLRIAddPathRefusalFor(fam.String())
	if refusesAddPath && accum.HasPathID {
		return nlriParseResult{}, refuseAddPath(fam, refusal)
	}
	i := 2
	if refusesAddPath && i < len(args) && args[i] == kwPathInfo {
		return nlriParseResult{}, refuseAddPath(fam, refusal)
	}
	mode := ""
	if i < len(args) && (args[i] == kwAdd || args[i] == kwDel) {
		mode = args[i]
		i++
	}
	if mode == "" {
		return nlriParseResult{}, fmt.Errorf("%w: %s needs add or del", route.ErrMissingAddDel, fam)
	}

	start := i
	for i < len(args) && !endsRegistrySection(args[i]) {
		token := args[i]
		if token == kwAdd {
			return nlriParseResult{}, fmt.Errorf("%w: %s section has %q after its first field", errLaterAddDel, fam, token)
		}
		if token == kwDel {
			return nlriParseResult{}, fmt.Errorf("%w: %s section has %q after its first field", errLaterAddDel, fam, token)
		}
		if refusesAddPath && token == kwPathInfo {
			return nlriParseResult{}, refuseAddPath(fam, refusal)
		}
		i++
	}
	if i == start {
		return nlriParseResult{}, route.ErrEmptyNLRISection
	}

	if mode == kwAdd {
		if parse, ok := registry.SectionRouteParserFor(fam.String()); ok {
			return parseRegistryRouteSection(parse, fam, args[start:i], accum, nextHop, i)
		}
	}
	encoded, err := encodeViaRegistry(fam, args[start:i], accum.PathID, accum.HasPathID)
	if err != nil {
		return nlriParseResult{}, err
	}
	return buildSingleNLRIResult(fam, mode, encoded, i)
}

// parseRegistryRouteSection parses an `add` run through the family's
// config-route parser, the one config and `ze bgp encode` use, so the three
// paths accept the same words and build the same bytes. It answers the NLRI and
// the attributes the run wrote; ParseUpdateText merges those attributes.
//
// The request carries no attribute block: the attributes written before the
// section are already in the UPDATE's own block, so the typed fields the parser
// could fill from one (AS_PATH, LOCAL_PREF) come back unset and are not read.
func parseRegistryRouteSection(parse func(registry.ConfigRouteRequest) (registry.PluginRoute, error),
	fam family.Family, run []string, accum nlriAccum, nextHop bgptypes.RouteNextHop, consumed int,
) (nlriParseResult, error) {
	route, err := parse(registry.ConfigRouteRequest{
		Content: run,
		NextHop: nextHop.String(),
		IsIPv6:  fam.AFI == family.AFIIPv6,
	})
	if err != nil {
		return nlriParseResult{}, fmt.Errorf("%s: %w", fam, err)
	}
	encoded, err := wireNLRI(fam, route.NLRI, accum.PathID, accum.HasPathID)
	if err != nil {
		return nlriParseResult{}, err
	}
	result, err := buildSingleNLRIResult(fam, kwAdd, encoded, consumed)
	if err != nil {
		return nlriParseResult{}, err
	}
	result.Attrs = route.Attrs
	return result, nil
}

// refuseAddPath builds the one error a path-information applying to fam's
// section gets: the reason fam's plugin registered, and the two ways out.
func refuseAddPath(fam family.Family, reason string) error {
	return fmt.Errorf("%w for %s: %s; drop path-information, or send the %s section in its own command",
		errFamilyRefusesAddPath, fam, reason, fam)
}

// endsRegistrySection reports whether a token ends a plugin family's token run.
//
// It is isBoundaryKeyword less `rd`, `label` and `path-information`. Those three
// are NLRI modifiers, and a plugin grammar spells them INSIDE its own run: the
// mup encoder reads `rd 100:100` (nlri/mup/encode.go EncodeNLRIHex) and the mvpn
// encoder reads `rd 65000:99999` (nlri/mvpn/encode.go parseMVPNFields). Stopping
// at one would hand the encoder a section with no RD, and leave `rd` as the next
// top-level token, where ParseUpdateText refuses it as an attribute written
// after an NLRI section. So the plugin keeps all three.
func endsRegistrySection(token string) bool {
	switch token {
	case kwRD, kwLabel, kwPathInfo:
		return false
	}
	return isBoundaryKeyword(token)
}
