// Design: docs/plan/spec-code-hygiene-fixes.md — Registry-based NLRI encoding helpers
//
// Package reactor uses these helpers to encode VPN, Labeled, and MUP NLRIs
// through the plugin registry instead of importing plugin packages directly.
// This preserves the plugin architecture's indirection layer.
package reactor

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// encodeVPNNLRI builds a VPN NLRI via the plugin registry.
// Replaces direct vpn.NewVPN() calls to avoid importing the VPN plugin.
func encodeVPNNLRI(family nlri.Family, rd nlri.RouteDistinguisher, labels []uint32, prefix netip.Prefix) (nlri.NLRI, error) {
	args := make([]string, 0, 4+2*len(labels))
	args = append(args, "rd", rd.String(), "prefix", prefix.String())
	for _, l := range labels {
		args = append(args, "label", strconv.FormatUint(uint64(l), 10))
	}

	hexStr, err := registry.EncodeNLRIByFamily(family.String(), args)
	if err != nil {
		return nil, fmt.Errorf("encode VPN NLRI: %w", err)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode VPN NLRI hex: %w", err)
	}

	return nlri.NewWireNLRI(family, data, false)
}

// encodeLabeledNLRI builds a labeled unicast NLRI via the plugin registry.
// Replaces direct labeled.NewLabeledUnicast() calls to avoid importing the labeled plugin.
func encodeLabeledNLRI(family nlri.Family, prefix netip.Prefix, labels []uint32, pathID uint32) (nlri.NLRI, error) {
	args := make([]string, 0, 2+2*len(labels))
	args = append(args, "prefix", prefix.String())
	for _, l := range labels {
		args = append(args, "label", strconv.FormatUint(uint64(l), 10))
	}

	hexStr, err := registry.EncodeNLRIByFamily(family.String(), args)
	if err != nil {
		return nil, fmt.Errorf("encode labeled NLRI: %w", err)
	}

	// Handle ADD-PATH: prepend path-id if non-zero.
	// The encoder produces NLRI bytes without path-id; WireNLRI expects
	// path-id as a 4-byte big-endian prefix when hasAddPath is true.
	if pathID > 0 {
		data, err := hex.DecodeString(fmt.Sprintf("%08X", pathID) + hexStr)
		if err != nil {
			return nil, fmt.Errorf("decode labeled NLRI hex: %w", err)
		}

		return nlri.NewWireNLRI(family, data, true)
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("decode labeled NLRI hex: %w", err)
	}

	return nlri.NewWireNLRI(family, data, false)
}

// encodeMUPNLRIBytes builds MUP NLRI bytes via the plugin registry.
// Replaces buildAPIMUPNLRI() to avoid importing the MUP plugin.
func encodeMUPNLRIBytes(spec bgptypes.MUPRouteSpec) ([]byte, error) {
	// Validate family/prefix consistency (preserved from buildAPIMUPNLRI).
	if err := validateMUPFamilyMatch(spec); err != nil {
		return nil, err
	}

	mupFamily := "ipv4/mup"
	if spec.IsIPv6 {
		mupFamily = "ipv6/mup"
	}

	args := mupSpecToArgs(spec)

	hexStr, err := registry.EncodeNLRIByFamily(mupFamily, args)
	if err != nil {
		return nil, err
	}

	return hex.DecodeString(hexStr)
}

// mupSpecToArgs converts a MUPRouteSpec to CLI-style string args for the registry encoder.
func mupSpecToArgs(spec bgptypes.MUPRouteSpec) []string {
	args := []string{"route-type", spec.RouteType}

	if spec.RD != "" {
		args = append(args, "rd", spec.RD)
	}
	if spec.Prefix != "" {
		args = append(args, "prefix", spec.Prefix)
	}
	if spec.Address != "" {
		args = append(args, "address", spec.Address)
	}
	if spec.TEID != "" {
		args = append(args, "teid", spec.TEID)
	}
	if spec.QFI > 0 {
		args = append(args, "qfi", strconv.FormatUint(uint64(spec.QFI), 10))
	}
	if spec.Endpoint != "" {
		args = append(args, "endpoint", spec.Endpoint)
	}
	if spec.Source != "" {
		args = append(args, "source", spec.Source)
	}

	return args
}

// validateMUPFamilyMatch checks that prefix/address family matches the IsIPv6 flag.
// This catches mismatches like IPv6 prefix with IPv4 AFI before they reach the encoder.
func validateMUPFamilyMatch(spec bgptypes.MUPRouteSpec) error {
	if spec.Prefix != "" {
		prefix, err := netip.ParsePrefix(spec.Prefix)
		if err == nil && spec.IsIPv6 != prefix.Addr().Is6() {
			expected := "IPv4"
			if spec.IsIPv6 {
				expected = "IPv6"
			}

			return fmt.Errorf("prefix %q is not %s", spec.Prefix, expected)
		}
	}

	if spec.Address != "" {
		addr, err := netip.ParseAddr(spec.Address)
		if err == nil && spec.IsIPv6 != addr.Is6() {
			expected := "IPv4"
			if spec.IsIPv6 {
				expected = "IPv6"
			}

			return fmt.Errorf("address %q is not %s", spec.Address, expected)
		}
	}

	return nil
}
