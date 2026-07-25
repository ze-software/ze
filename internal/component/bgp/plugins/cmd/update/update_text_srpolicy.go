// Design: docs/architecture/api/commands.md -- SR-Policy text parsing for update text command
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format (SAFI 73)
// Overview: update_text.go -- main update text parser and shared constants
// Related: update_text_nlri.go -- generic NLRI section parsing
package update

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/srpolicy"
	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// SR-Policy keywords.
const (
	kwDistinguisher = "distinguisher"
	kwColor         = "color"
	kwEndpoint      = "endpoint"
)

// parseSRPolicySection parses SR-Policy NLRI section.
// RFC 9830 Section 2.1: SR-Policy NLRI = distinguisher + color + endpoint.
// Syntax: nlri ipv4/sr-policy add distinguisher <d> color <c> endpoint <ep>
//
//	nlri ipv4/sr-policy del distinguisher <d> color <c> endpoint <ep>
func parseSRPolicySection(args []string, fam family.Family) (nlriParseResult, error) {
	// args[0] = "nlri", args[1] = "ipv4/sr-policy" or "ipv6/sr-policy"
	consumed := 2
	i := 2

	mode := ""
	var distinguisher uint32
	var color uint32
	var endpoint netip.Addr
	hasDistinguisher := false
	hasColor := false
	hasEndpoint := false

	for i < len(args) {
		token := args[i]

		if isBoundaryKeyword(token) {
			break
		}

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

		if mode == "" {
			return nlriParseResult{}, fmt.Errorf("%w: got %q", route.ErrMissingAddDel, token)
		}

		switch token {
		case kwDistinguisher:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("sr-policy: distinguisher requires value")
			}
			v, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("sr-policy: invalid distinguisher: %w", err)
			}
			distinguisher = uint32(v)
			hasDistinguisher = true
			i += 2
			consumed += 2

		case kwColor:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("sr-policy: color requires value")
			}
			v, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("sr-policy: invalid color: %w", err)
			}
			color = uint32(v)
			hasColor = true
			i += 2
			consumed += 2

		case kwEndpoint:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("sr-policy: endpoint requires value")
			}
			addr, err := netip.ParseAddr(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("sr-policy: invalid endpoint: %w", err)
			}
			endpoint = addr
			hasEndpoint = true
			i += 2
			consumed += 2

		default:
			return nlriParseResult{}, fmt.Errorf("sr-policy: unknown keyword %q; valid: distinguisher, color, endpoint", token)
		}
	}

	if !hasDistinguisher {
		return nlriParseResult{}, errors.New("sr-policy: missing distinguisher")
	}
	if !hasColor {
		return nlriParseResult{}, errors.New("sr-policy: missing color")
	}
	if !hasEndpoint {
		return nlriParseResult{}, errors.New("sr-policy: missing endpoint")
	}

	sp := srpolicy.New(fam.AFI, distinguisher, color, endpoint)
	wireNLRI, err := nlri.NewWireNLRI(fam, sp.Bytes(), false)
	if err != nil {
		return nlriParseResult{}, fmt.Errorf("sr-policy: %w", err)
	}

	return buildSingleNLRIResult(fam, mode, wireNLRI, consumed)
}
