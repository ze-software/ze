// Design: docs/architecture/api/commands.md — FlowSpec text parsing for update text command
// Overview: update_text.go — shared text attribute parsing types and helpers
// Related: update_text_nlri.go — NLRI section parsing and dispatch
package handler

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
)

// isFlowSpecBoundary returns true if token ends FlowSpec section (next section starts).
// rd is NOT a boundary since it's valid within flow-vpn rules.
func isFlowSpecBoundary(token string) bool {
	if token == kwRD {
		return false // rd is valid within flowspec-vpn (after add/del)
	}
	return isBoundaryKeyword(token)
}

// parseFlowSpecSection parses nlri <flowspec-family> add [rd <value>] <components>+ | del <components>+
// RFC 8955 Section 4: FlowSpec NLRI = ordered list of match components.
// For flow-vpn families, rd is required after add/del. For flow families, rd is invalid.
func parseFlowSpecSection(args []string, family nlri.Family) (nlriParseResult, error) {
	// args[0] = "nlri", args[1] = family string
	consumed := 2
	i := 2

	// Parse add/del + components
	mode := "" // "", "add", or "del"
	var announce, withdraw []nlri.NLRI

	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access

		// Boundary keywords end this section (rd is valid within flowspec-vpn)
		if isFlowSpecBoundary(token) {
			break
		}

		// Mode switches
		if token == kwAdd {
			// Create a new FlowSpec for this add block
			// (consecutive add tokens are implicit continuation)
			mode = kwAdd
			i++
			consumed++
			continue
		}
		if token == kwDel {
			// (consecutive del tokens are implicit continuation)
			mode = kwDel
			i++
			consumed++
			continue
		}

		// Must have a mode before components
		if mode == "" {
			return nlriParseResult{}, fmt.Errorf("%w: got %q", route.ErrMissingAddDel, token)
		}

		// Parse FlowSpec components for this rule
		fs, extra, err := parseFlowSpecComponents(args[i:], family)
		if err != nil {
			return nlriParseResult{}, err
		}

		if mode == kwAdd {
			announce = append(announce, fs)
		} else {
			withdraw = append(withdraw, fs)
		}
		i += extra
		consumed += extra
	}

	if len(announce) == 0 && len(withdraw) == 0 {
		return nlriParseResult{}, route.ErrEmptyNLRISection
	}

	return nlriParseResult{Family: family, Announce: announce, Withdraw: withdraw, Consumed: consumed}, nil
}

// parseFlowSpecComponents parses FlowSpec components until boundary or mode switch.
// For flow-vpn: rd <value> is required after add/del.
// For flow: rd is invalid.
// Calls flowspec.EncodeFlowSpecComponents directly (in-process plugin).
// RFC 8955: Components are ANDed together.
func parseFlowSpecComponents(args []string, family nlri.Family) (nlri.NLRI, int, error) {
	consumed := 0
	i := 0

	// Parse rd <value> if present (must be first after add/del)
	var rd nlri.RouteDistinguisher
	hasRD := false
	if i < len(args) && args[i] == kwRD {
		if i+1 >= len(args) {
			return nil, 0, errors.New("rd requires value (ASN:NN or IP:NN)")
		}
		var err error
		rd, err = nlri.ParseRDString(args[i+1])
		if err != nil {
			return nil, 0, fmt.Errorf("invalid rd: %w", err)
		}
		hasRD = true
		i += 2
		consumed += 2
	}

	// Validate rd presence based on family
	isVPN := family.SAFI == nlri.SAFIFlowSpecVPN
	if isVPN && !hasRD {
		return nil, 0, fmt.Errorf("%w: rd required for %s", route.ErrMissingRD, family)
	}
	if !isVPN && hasRD {
		return nil, 0, fmt.Errorf("rd not allowed for %s (use %s/flow-vpn)", family, family.AFI)
	}

	// Collect component tokens until boundary or mode switch
	start := i
	for i < len(args) {
		token := args[i] //nolint:gosec // G602 false positive: loop condition guards access
		if isBoundaryKeyword(token) || token == kwAdd || token == kwDel {
			break
		}
		i++
		consumed++
	}

	if i == start {
		return nil, 0, errors.New("flowspec requires at least one component")
	}

	// Build args for plugin: for VPN, prepend "rd <value>"
	var pluginArgs []string
	if isVPN {
		pluginArgs = append(pluginArgs, "rd", rd.String())
	}
	pluginArgs = append(pluginArgs, args[start:i]...)

	// Call flowspec encoder via registry (in-process fast path)
	hexStr, err := registry.EncodeNLRIByFamily(family.String(), pluginArgs)
	if err != nil {
		return nil, 0, fmt.Errorf("flowspec encode: %w", err)
	}
	wireBytes, err := hex.DecodeString(strings.ToLower(hexStr))
	if err != nil {
		return nil, 0, fmt.Errorf("flowspec hex decode: %w", err)
	}

	// Return WireNLRI wrapping the encoded bytes
	// FlowSpec doesn't support ADD-PATH per RFC 8955
	wire, err := nlri.NewWireNLRI(family, wireBytes, false)
	if err != nil {
		return nil, 0, err
	}

	return wire, consumed, nil
}
