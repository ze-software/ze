package bgp_nlri_flowspec

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/route"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// EncodeRoute encodes a FlowSpec route command into UPDATE body bytes and NLRI bytes.
// This implements the InProcessRouteEncoder signature for the plugin registry.
func EncodeRoute(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error) {
	isIPv6 := strings.HasPrefix(family, "ipv6/")
	ub := message.NewUpdateBuilder(localAS, isIBGP, asn4, addPath)

	// Parse route command - expects "match <spec> then <action>"
	args := strings.Fields(routeCmd)
	if len(args) < 1 {
		return nil, nil, fmt.Errorf("missing FlowSpec command")
	}

	// Parse using API parser
	parsed, err := route.ParseFlowSpecArgs(args)
	if err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}

	// Build FlowSpec NLRI
	var fam Family
	if isIPv6 {
		fam = Family{AFI: AFIIPv6, SAFI: SAFIFlowSpec}
	} else {
		fam = Family{AFI: AFIIPv4, SAFI: SAFIFlowSpec}
	}

	fs := NewFlowSpec(fam)

	// Add components based on parsed route
	if parsed.DestPrefix != nil {
		fs.AddComponent(NewFlowDestPrefixComponent(*parsed.DestPrefix))
	}
	if parsed.SourcePrefix != nil {
		fs.AddComponent(NewFlowSourcePrefixComponent(*parsed.SourcePrefix))
	}
	if len(parsed.Protocols) > 0 {
		fs.AddComponent(NewFlowIPProtocolComponent(parsed.Protocols...))
	}
	if len(parsed.Ports) > 0 {
		fs.AddComponent(NewFlowPortComponent(parsed.Ports...))
	}
	if len(parsed.DestPorts) > 0 {
		fs.AddComponent(NewFlowDestPortComponent(parsed.DestPorts...))
	}
	if len(parsed.SourcePorts) > 0 {
		fs.AddComponent(NewFlowSourcePortComponent(parsed.SourcePorts...))
	}

	// Get NLRI bytes
	nlriBytes := fs.Bytes()

	// Convert to FlowSpecParams
	params, err := flowSpecRouteToParams(parsed, nlriBytes)
	if err != nil {
		return nil, nil, err
	}

	// Build UPDATE
	update := ub.BuildFlowSpec(params)

	// Pack UPDATE body using PackTo
	updateBody := message.PackTo(update, nil)

	return updateBody, nlriBytes, nil
}

// flowSpecRouteToParams converts FlowSpecRoute to FlowSpecParams.
func flowSpecRouteToParams(r bgptypes.FlowSpecRoute, nlriBytes []byte) (message.FlowSpecParams, error) {
	p := message.FlowSpecParams{
		IsIPv6: r.Family == bgptypes.AFINameIPv6,
		NLRI:   nlriBytes,
	}

	// Convert actions to extended communities
	var extComms []byte

	// Discard action = rate-limit to 0 (RFC 5575)
	if r.Actions.Discard {
		// Traffic-rate with rate=0 means discard
		// Type 0x80, Subtype 0x06, 2 reserved bytes, 4-byte IEEE 754 float (0.0)
		extComms = append(extComms, 0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
	}

	// Rate-limit action (RFC 5575)
	if r.Actions.RateLimit > 0 {
		// Traffic-rate extended community
		// Type 0x80, Subtype 0x06, 2 reserved bytes, 4-byte IEEE 754 float
		rate := float32(r.Actions.RateLimit)
		bits := floatToIEEE754(rate)
		extComms = append(extComms, 0x80, 0x06, 0x00, 0x00, byte(bits>>24), byte(bits>>16), byte(bits>>8), byte(bits))
	}

	// DSCP marking (RFC 5575)
	if r.Actions.MarkDSCP > 0 {
		// Traffic-marking extended community
		// Type 0x80, Subtype 0x09, 6 bytes with DSCP in last byte
		extComms = append(extComms, 0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, r.Actions.MarkDSCP)
	}

	// Redirect action (RFC 5575/7674)
	if r.Actions.Redirect != "" {
		ec, err := parseRedirectTarget(r.Actions.Redirect)
		if err != nil {
			return p, fmt.Errorf("invalid redirect: %w", err)
		}
		extComms = append(extComms, ec[:]...)
	}

	p.ExtCommunityBytes = extComms

	return p, nil
}

// floatToIEEE754 converts a float32 to IEEE 754 bits.
func floatToIEEE754(f float32) uint32 {
	// Use math.Float32bits for proper IEEE 754 conversion
	return math.Float32bits(f)
}

// parseRedirectTarget parses a redirect target in ASN:value format.
// Supports both 2-byte ASN (RFC 5575) and 4-byte ASN (RFC 7674).
func parseRedirectTarget(s string) ([8]byte, error) {
	var ec [8]byte
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return ec, fmt.Errorf("invalid redirect format: %s", s)
	}

	asn, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return ec, fmt.Errorf("invalid ASN in redirect: %s", parts[0])
	}

	if asn <= 65535 {
		// 2-byte ASN format (RFC 5575)
		// Type 0x80, Subtype 0x08, 2-byte ASN, 4-byte local value
		target, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return ec, fmt.Errorf("invalid target in redirect: %s", parts[1])
		}
		ec[0] = 0x80
		ec[1] = 0x08
		ec[2] = byte(asn >> 8)
		ec[3] = byte(asn)
		ec[4] = byte(target >> 24)
		ec[5] = byte(target >> 16)
		ec[6] = byte(target >> 8)
		ec[7] = byte(target)
	} else {
		// 4-byte ASN format (RFC 7674)
		// Type 0x82, Subtype 0x08, 4-byte ASN, 2-byte local value
		target, err := strconv.ParseUint(parts[1], 10, 16)
		if err != nil {
			return ec, fmt.Errorf("invalid target in redirect (4-byte ASN max 16-bit local): %s", parts[1])
		}
		ec[0] = 0x82
		ec[1] = 0x08
		ec[2] = byte(asn >> 24)
		ec[3] = byte(asn >> 16)
		ec[4] = byte(asn >> 8)
		ec[5] = byte(asn)
		ec[6] = byte(target >> 8)
		ec[7] = byte(target)
	}

	return ec, nil
}
