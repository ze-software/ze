// Package api provides the wire-encoded update parser.
// Handles hex and b64 encodings for peer update commands.
package plugin

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/bgp/context"
	"codeberg.org/thomas-mangin/ze/internal/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
)

// Wire mode errors.
var (
	ErrWireModeRequiresAttr = errors.New("wire mode requires attr set for announce")
	ErrAttrSetOnce          = errors.New("attr set can only appear once")
	ErrPathInfoWireMode     = errors.New("path-information only valid in text mode, use addpath flag for wire")
)

// kwSelf is the "self" keyword for next-hop.
const kwSelf = "self"

// ParseUpdateWire parses wire-encoded update command (hex or b64).
// Same structure as text mode but attrs/nlris are decoded wire bytes.
// Returns same UpdateTextResult as ParseUpdateText for uniform handling.
func ParseUpdateWire(args []string, encoding WireEncoding) (*UpdateTextResult, error) {
	var (
		attrsSet  bool
		attrsWire *attribute.AttributesWire
		nhop      RouteNextHop
		groups    []NLRIGroup
		watchdog  string
	)

	decode := decoderForEncoding(encoding)
	i := 0

	for i < len(args) {
		switch args[i] { //nolint:gosec // G602: loop condition guards access
		case kwAttr:
			if attrsSet {
				return nil, ErrAttrSetOnce
			}
			wire, consumed, err := parseWireAttrSection(args[i:], decode)
			if err != nil {
				return nil, err
			}
			attrsWire = wire
			attrsSet = true
			i += consumed

		case kwNhop:
			consumed, err := parseWireNhopSection(args[i:], decode, &nhop)
			if err != nil {
				return nil, err
			}
			i += consumed

		case kwPathInfo:
			return nil, ErrPathInfoWireMode

		case kwNLRI:
			family, announce, withdraw, consumed, err := parseWireNLRISection(args[i:], decode)
			if err != nil {
				return nil, err
			}

			// Wire mode requires attr set for announce
			if len(announce) > 0 && !attrsSet {
				return nil, ErrWireModeRequiresAttr
			}

			// Use wire directly (attrsWire may be nil for withdrawals)
			var wire *attribute.AttributesWire
			if attrsSet {
				wire = attrsWire
			}

			groups = append(groups, NLRIGroup{
				Family:   family,
				Announce: announce,
				Withdraw: withdraw,
				Wire:     wire,
				NextHop:  nhop, // Snapshot current nhop
			})
			i += consumed

		case kwWatchdog:
			if i+1 >= len(args) {
				return nil, errors.New("missing watchdog name")
			}
			watchdog = args[i+1]
			i += 2

		default:
			return nil, fmt.Errorf("unexpected token: %s", args[i]) //nolint:gosec // G602
		}
	}

	return &UpdateTextResult{Groups: groups, WatchdogName: watchdog}, nil
}

// decodeFunc decodes wire data from string.
type decodeFunc func(s string) ([]byte, error)

// decoderForEncoding returns the appropriate decoder for the encoding.
func decoderForEncoding(enc WireEncoding) decodeFunc {
	switch enc {
	case WireEncodingHex:
		return decodeHex
	case WireEncodingB64:
		return decodeB64
	case WireEncodingCBOR, WireEncodingText:
		return decodeHex // Default fallback for non-wire encodings
	}
	return decodeHex // Default fallback
}

// decodeHex decodes hex string, stripping whitespace.
func decodeHex(s string) ([]byte, error) {
	// Strip all whitespace
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	s = strings.ReplaceAll(s, "\n", "")
	return hex.DecodeString(s)
}

// decodeB64 decodes base64 string.
func decodeB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// isWireBoundaryKeyword returns true if token starts a new section in wire mode.
func isWireBoundaryKeyword(token string) bool {
	return token == kwAttr || token == kwNLRI || token == kwWatchdog ||
		token == kwNhop || token == kwPathInfo
}

// parseWireAttrSection parses attr set <data> section.
// Returns AttributesWire, consumed count, error.
func parseWireAttrSection(args []string, decode decodeFunc) (*attribute.AttributesWire, int, error) {
	// args[0] = "attr"
	if len(args) < 2 {
		return nil, 0, errors.New("attr requires set")
	}
	if args[1] != kwSet {
		return nil, 0, fmt.Errorf("wire mode only supports attr set, got: %s", args[1])
	}
	if len(args) < 3 {
		return nil, 0, errors.New("attr section requires wire data after set")
	}

	// Collect all data tokens until boundary
	var dataTokens []string
	consumed := 2 // "attr" + "set"
	for i := 2; i < len(args); i++ {
		if isWireBoundaryKeyword(args[i]) {
			break
		}
		dataTokens = append(dataTokens, args[i])
		consumed++
	}

	if len(dataTokens) == 0 {
		return nil, 0, errors.New("attr section requires wire data after set")
	}

	// Join and decode
	data := strings.Join(dataTokens, "")
	bytes, err := decode(data)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid attr data: %w", err)
	}

	wire := attribute.NewAttributesWire(bytes, context.APIContextID)
	return wire, consumed, nil
}

// parseWireNhopSection parses nhop <set <data>|del> section.
// Returns consumed count, error.
func parseWireNhopSection(args []string, decode decodeFunc, nhop *RouteNextHop) (int, error) {
	// args[0] = "nhop"
	if len(args) < 2 {
		return 0, errors.New("nhop requires set or del")
	}

	switch args[1] {
	case kwSet:
		if len(args) < 3 {
			return 0, errors.New("nhop set requires data")
		}
		value := args[2]

		// Check for "self" keyword (same in all modes)
		if value == kwSelf {
			*nhop = NewNextHopSelf()
			return 3, nil
		}

		// Decode wire data
		bytes, err := decode(value)
		if err != nil {
			return 0, fmt.Errorf("invalid next-hop: %w", err)
		}

		// Parse as IP address from bytes
		addr, err := parseIPFromBytes(bytes)
		if err != nil {
			return 0, fmt.Errorf("invalid next-hop: %w", err)
		}

		*nhop = NewNextHopExplicit(addr)
		return 3, nil

	case kwDel:
		// nhop del must not have additional arguments (before next keyword)
		if len(args) > 2 && !isWireBoundaryKeyword(args[2]) {
			return 0, errors.New("nhop del takes no arguments")
		}
		*nhop = RouteNextHop{} // Clear
		return 2, nil

	default:
		return 0, fmt.Errorf("nhop requires set or del, got: %s", args[1])
	}
}

// parseIPFromBytes parses an IP address from raw bytes.
func parseIPFromBytes(b []byte) (netip.Addr, error) {
	switch len(b) {
	case 4:
		return netip.AddrFrom4([4]byte(b)), nil
	case 16:
		return netip.AddrFrom16([16]byte(b)), nil
	default:
		return netip.Addr{}, fmt.Errorf("invalid IP length: %d (expected 4 or 16)", len(b))
	}
}

// parseWireNLRISection parses nlri <family> [addpath] add <data>... [del <data>...]...
// Returns family, announce list, withdraw list, consumed count, error.
func parseWireNLRISection(args []string, decode decodeFunc) (nlri.Family, []nlri.NLRI, []nlri.NLRI, int, error) {
	// args[0] = "nlri"
	if len(args) < 2 {
		return nlri.Family{}, nil, nil, 0, ErrInvalidFamily
	}

	family, ok := nlri.ParseFamily(args[1])
	if !ok {
		return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: %s", ErrInvalidFamily, args[1])
	}

	consumed := 2 // "nlri" + family
	i := 2

	// Check for addpath flag
	addPath := false
	if i < len(args) && args[i] == "addpath" {
		addPath = true
		i++
		consumed++
	}

	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI

	for i < len(args) {
		token := args[i] //nolint:gosec // G602

		// Boundary keywords end this section
		if isWireBoundaryKeyword(token) {
			break
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

		// Must have a mode before data
		if mode == "" {
			return nlri.Family{}, nil, nil, 0, fmt.Errorf("%w: got %q", ErrMissingAddDel, token)
		}

		// Decode wire data
		bytes, err := decode(token)
		if err != nil {
			return nlri.Family{}, nil, nil, 0, fmt.Errorf("invalid nlri data: %w", err)
		}

		// Split into individual NLRIs
		nlris, err := splitWireNLRIs(bytes, family, addPath)
		if err != nil {
			return nlri.Family{}, nil, nil, 0, fmt.Errorf("failed to split NLRIs for %s: %w", family, err)
		}

		if mode == kwAdd {
			announce = append(announce, nlris...)
		} else {
			withdraw = append(withdraw, nlris...)
		}

		i++
		consumed++
	}

	// Must have at least one NLRI
	if len(announce) == 0 && len(withdraw) == 0 {
		return nlri.Family{}, nil, nil, 0, ErrEmptyNLRISection
	}

	return family, announce, withdraw, consumed, nil
}

// splitWireNLRIs splits concatenated wire-encoded NLRIs into individual NLRI objects.
// Uses GetNLRISizeFunc for family-specific boundary detection.
func splitWireNLRIs(data []byte, family nlri.Family, addPath bool) ([]nlri.NLRI, error) {
	if len(data) == 0 {
		return nil, nil
	}

	sizeFunc := message.GetNLRISizeFunc(family.AFI, family.SAFI, addPath)
	var result []nlri.NLRI
	offset := 0

	for offset < len(data) {
		size, err := sizeFunc(data[offset:])
		if err != nil {
			return nil, err
		}
		if size <= 0 || offset+size > len(data) {
			return nil, fmt.Errorf("invalid NLRI size %d at offset %d", size, offset)
		}

		// Extract this NLRI's bytes
		nlriBytes := data[offset : offset+size]

		// Wrap in WireNLRI
		wn, err := nlri.NewWireNLRI(family, nlriBytes, addPath)
		if err != nil {
			return nil, err
		}
		result = append(result, wn)

		offset += size
	}

	return result, nil
}

// handleUpdateHex handles: peer <addr> update hex ...
// Parses wire hex format and dispatches to reactor batch methods.
func handleUpdateHex(ctx *CommandContext, args []string) (*Response, error) {
	return handleUpdateWire(ctx, args, WireEncodingHex)
}

// handleUpdateB64 handles: peer <addr> update b64 ...
// Parses wire base64 format and dispatches to reactor batch methods.
func handleUpdateB64(ctx *CommandContext, args []string) (*Response, error) {
	return handleUpdateWire(ctx, args, WireEncodingB64)
}

// handleUpdateWire handles wire-encoded update commands (hex/b64).
// Parses the wire format and dispatches to reactor batch methods.
// RFC 4271 Section 4.3: UPDATE Message Format.
func handleUpdateWire(ctx *CommandContext, args []string, encoding WireEncoding) (*Response, error) {
	result, err := ParseUpdateWire(args, encoding)
	if err != nil {
		return &Response{Status: statusError, Data: err.Error()}, err
	}

	if result.WatchdogName != "" {
		errMsg := "watchdog not yet implemented for wire mode"
		return &Response{Status: statusError, Data: errMsg}, errors.New(errMsg)
	}

	return dispatchNLRIGroups(ctx, result.Groups)
}
