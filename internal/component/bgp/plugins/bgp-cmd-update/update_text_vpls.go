// Design: docs/architecture/api/commands.md — VPLS text parsing for update text command
// Overview: update_text.go — shared text attribute parsing types and helpers
// Related: update_text_nlri.go — NLRI section parsing and dispatch
// Related: update_text_evpn.go — EVPN route type text parsing
package bgpcmdupdate

import (
	"errors"
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/route"
)

// VPLS NLRI keywords for text parsing.
const (
	kwVEID          = "ve-id"
	kwVEBlockOffset = "ve-block-offset"
	kwVEBlockSize   = "ve-block-size"
	kwLabelBase     = "label-base"
)

// isVPLSBoundary returns true if token ends VPLS section (next section starts).
// VPLS-specific keywords (rd, label, ve-*) are NOT boundaries.
func isVPLSBoundary(token string) bool {
	switch token {
	case kwRD, kwLabel, kwVEID, kwVEBlockOffset, kwVEBlockSize, kwLabelBase:
		return false // These are valid within VPLS
	default: // check general boundary keywords
		return isBoundaryKeyword(token)
	}
}

// parseVPLSSection parses VPLS NLRI section.
// RFC 4761 Section 3.2.2: VPLS BGP NLRI format.
// Syntax: nlri l2vpn/vpls add rd <rd> ve-id <n> ve-block-offset <n> ve-block-size <n> label-base <n>.
func parseVPLSSection(args []string, family nlri.Family, _ nlriAccum) (nlriParseResult, error) {
	// args[0] = "nlri", args[1] = "l2vpn/vpls"
	consumed := 2
	i := 2

	mode := "" // "", "add", or "del"

	// VPLS fields
	var rd nlri.RouteDistinguisher
	var veID, veBlockOffset, veBlockSize uint16
	var labelBase uint32
	hasRD := false

	for i < len(args) {
		token := args[i]

		// Boundary keywords end this section (except VPLS-specific keywords)
		if isVPLSBoundary(token) {
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

		// Must have mode before VPLS fields
		if mode == "" {
			return nlriParseResult{}, fmt.Errorf("%w: got %q", route.ErrMissingAddDel, token)
		}

		// Parse VPLS-specific fields
		switch token {
		case kwRD:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("rd requires value")
			}
			var err error
			rd, err = nlri.ParseRDString(args[i+1])
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid rd: %w", err)
			}
			hasRD = true
			i += 2
			consumed += 2

		case kwVEID:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("ve-id requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 16)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid ve-id: %w", err)
			}
			veID = uint16(val)
			i += 2
			consumed += 2

		case kwVEBlockOffset:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("ve-block-offset requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 16)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid ve-block-offset: %w", err)
			}
			veBlockOffset = uint16(val)
			i += 2
			consumed += 2

		case kwVEBlockSize:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("ve-block-size requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 16)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid ve-block-size: %w", err)
			}
			veBlockSize = uint16(val)
			i += 2
			consumed += 2

		case kwLabelBase, kwLabel:
			if i+1 >= len(args) {
				return nlriParseResult{}, errors.New("label-base requires value")
			}
			val, err := strconv.ParseUint(args[i+1], 10, 32)
			if err != nil {
				return nlriParseResult{}, fmt.Errorf("invalid label-base: %w", err)
			}
			labelBase = uint32(val)
			i += 2
			consumed += 2

		default: // unknown VPLS keyword - reject with error
			return nlriParseResult{}, fmt.Errorf("unknown vpls keyword: %s", token)
		}
	}

	// Validate required fields
	if !hasRD {
		return nlriParseResult{}, errors.New("vpls requires rd")
	}

	// Create VPLS NLRI via registry encoder (avoids direct plugin import)
	encodeArgs := []string{
		"rd", rd.String(),
		"ve-id", strconv.FormatUint(uint64(veID), 10),
		"ve-block-offset", strconv.FormatUint(uint64(veBlockOffset), 10),
		"ve-block-size", strconv.FormatUint(uint64(veBlockSize), 10),
		"label-base", strconv.FormatUint(uint64(labelBase), 10),
	}

	vplsNLRI, err := encodeViaRegistry(family, encodeArgs, false)
	if err != nil {
		return nlriParseResult{}, err
	}

	return buildSingleNLRIResult(family, mode, vplsNLRI, consumed)
}
