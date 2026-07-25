// Design: docs/architecture/config/syntax.md -- FlowSpec config route parsing
// RFC: rfc/short/rfc8955.md -- FlowSpec NLRI + Traffic Filtering Action communities
// Related: config_builder.go -- buildFlowSpecComponents (match-criteria -> NLRI bytes)

package flowspec

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// FlowSpec path attribute wire constants.
const (
	attrCodeCommunity     uint8 = 8  // COMMUNITIES (RFC 1997).
	attrCodeExtComm       uint8 = 16 // EXTENDED_COMMUNITIES (RFC 4360) -- actions.
	attrCodeIPv6ExtComm   uint8 = 25 // IPv6 Address Specific Ext-Community (RFC 5701).
	flowFlagOptTransitive       = 0xC0
)

var errFlowSpecMissingCriteria = errors.New("flowspec nlri requires match criteria")

// parseConfigRoute implements registry.InProcessConfigRouteParser for FlowSpec.
// It builds the RFC 8955 NLRI from the match-criteria tokens (wrapping with a
// length prefix + RD for the VPN variant) and assembles the community /
// extended-community / IPv6 extended-community attributes from the pre-parsed
// attribute block. ORIGIN, AS_PATH and LOCAL_PREF are owned by BuildPlugin and
// FlowSpec never overrides them (matches the previous BuildFlowSpec behavior).
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	rd, criteria, err := flowSpecCriteriaFromContent(req.Content)
	if err != nil {
		return registry.PluginRoute{}, err
	}
	if len(criteria) == 0 {
		return registry.PluginRoute{}, errFlowSpecMissingCriteria
	}

	// Build the NLRI, rejecting any criterion that produced no component -- an
	// unrecognized key (typo) or a known key whose value(s) failed to parse. A
	// silently-dropped criterion would widen the filter (worst case: a zero-
	// component all-match rule). This makes the parser uniformly fail-loud.
	fs, dropped := buildFlowSpecComponents(criteria, req.IsIPv6)
	if len(dropped) > 0 {
		slices.Sort(dropped)
		return registry.PluginRoute{}, fmt.Errorf("flowspec: invalid or unrecognized match criteria: %s", strings.Join(dropped, ", "))
	}

	forVPN := rd != ""
	var body []byte
	if forVPN {
		body = fs.ComponentBytes()
	} else {
		body = fs.Bytes()
	}

	var nlri []byte
	if forVPN {
		rdBytes, err := flowRDStringToBytes(rd)
		if err != nil {
			return registry.PluginRoute{}, fmt.Errorf("flowspec rd: %w", err)
		}
		nlri = wrapFlowSpecVPN(rdBytes, body)
	} else {
		nlri = body
	}

	var attrs []registry.PluginRouteAttr

	// COMMUNITIES (code 8): config order, unsorted (matches BuildFlowSpec).
	if len(req.Community) > 0 {
		val := make([]byte, 0, 4*len(req.Community))
		for _, c := range req.Community {
			val = append(val, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeCommunity, Flags: flowFlagOptTransitive, Value: val})
	}
	// EXTENDED_COMMUNITIES (code 16): actions; sorted by type (RFC 4360).
	if len(req.ExtCommunity) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeExtComm, Flags: flowFlagOptTransitive, Value: sortExtCommunities(req.ExtCommunity)})
	}
	// IPv6 EXTENDED_COMMUNITIES (code 25): redirect-to-nexthop IPv6 (RFC 5701/7674).
	if len(req.IPv6ExtCommunity) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeIPv6ExtComm, Flags: flowFlagOptTransitive, Value: req.IPv6ExtCommunity})
	}

	return registry.PluginRoute{
		IsIPv6:  req.IsIPv6,
		NLRI:    nlri,
		NextHop: req.NextHop,
		Attrs:   attrs,
		// FlowSpec does not carry a configured AS_PATH or LOCAL_PREF (BuildPlugin
		// supplies the defaults), so both are intentionally left zero.
	}, nil
}

// flowSpecCriteriaFromContent splits the NLRI content tokens into the optional
// RD (VPN variant) and the match-criteria map. Layout (after the family name):
//
//	[rd RD] [add|del|eor] <criterion> <value|[ v1 v2 ... ]> ...
//
// source-ipv4/source-ipv6 and destination-ipv4/destination-ipv6 are normalized
// to the family-agnostic keys buildFlowSpecComponents expects.
func flowSpecCriteriaFromContent(content []string) (string, map[string][]string, error) {
	criteria := make(map[string][]string)
	rd := ""

	i := 0
	for i < len(content) {
		tok := content[i]
		switch {
		case tok == "rd" && i+1 < len(content):
			rd = content[i+1]
			i += 2
			continue
		case tok == "add" || tok == "del" || tok == "eor":
			i++
			continue
		}

		key := normalizeFlowSpecKey(tok)
		// Bracketed list: criterion [ v1 v2 ... ]
		if i+1 < len(content) && content[i+1] == "[" {
			j := i + 2
			closed := false
			for ; j < len(content); j++ {
				if content[j] == "]" {
					closed = true
					break
				}
				criteria[key] = append(criteria[key], content[j])
			}
			if !closed {
				return "", nil, fmt.Errorf("flowspec criterion %q: unterminated '[' list", key)
			}
			i = j + 1
			continue
		}
		// Single value.
		if i+1 < len(content) {
			criteria[key] = append(criteria[key], content[i+1])
			i += 2
			continue
		}
		i++
	}

	return rd, criteria, nil
}

// normalizeFlowSpecKey maps the IPv4/IPv6 source/destination variants to the
// family-agnostic keys buildFlowSpecComponents uses.
func normalizeFlowSpecKey(k string) string {
	switch k {
	case "source-ipv4", kwSourceIPv6:
		return kwSource
	case "destination-ipv4", kwDestinationIPv6:
		return kwDestination
	}
	return k
}

// wrapFlowSpecVPN wraps FlowSpec component bytes with the RFC 8955 Section 8 VPN
// NLRI envelope: Length + RD(8) + components. Length uses the 2-byte 0xfnnn form
// when the payload reaches 240 bytes.
func wrapFlowSpecVPN(rd [8]byte, components []byte) []byte {
	payloadLen := 8 + len(components)
	var nlri []byte
	if payloadLen < 240 {
		nlri = make([]byte, 0, 1+payloadLen)
		nlri = append(nlri, byte(payloadLen))
	} else {
		nlri = make([]byte, 0, 2+payloadLen)
		nlri = append(nlri, 0xF0|byte(payloadLen>>8), byte(payloadLen))
	}
	nlri = append(nlri, rd[:]...)
	nlri = append(nlri, components...)
	return nlri
}

// flowRDStringToBytes parses an RFC 4364 Route Distinguisher string (ASN:NN or
// IP:NN) into its 8-byte wire form.
func flowRDStringToBytes(s string) ([8]byte, error) {
	var rd [8]byte
	left, right, found := strings.Cut(s, ":")
	if !found {
		return rd, fmt.Errorf("invalid rd %q: expected ASN:NN or IP:NN", s)
	}
	if ip, err := netip.ParseAddr(left); err == nil && ip.Is4() {
		num, err := strconv.ParseUint(right, 10, 16)
		if err != nil {
			return rd, fmt.Errorf("invalid rd number %q", right)
		}
		b := ip.As4()
		rd[1] = 1 // Type 1 (IPv4)
		copy(rd[2:6], b[:])
		rd[6], rd[7] = byte(num>>8), byte(num)
		return rd, nil
	}
	asn, err := strconv.ParseUint(left, 10, 32)
	if err != nil {
		return rd, fmt.Errorf("invalid rd ASN %q", left)
	}
	num, err := strconv.ParseUint(right, 10, 32)
	if err != nil {
		return rd, fmt.Errorf("invalid rd number %q", right)
	}
	if asn <= 0xFFFF {
		rd[1] = 0 // Type 0
		rd[2], rd[3] = byte(asn>>8), byte(asn)
		rd[4], rd[5], rd[6], rd[7] = byte(num>>24), byte(num>>16), byte(num>>8), byte(num)
	} else {
		rd[1] = 2 // Type 2
		rd[2], rd[3], rd[4], rd[5] = byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn)
		rd[6], rd[7] = byte(num>>8), byte(num)
	}
	return rd, nil
}

// sortExtCommunities sorts 8-byte extended communities by their 64-bit value for
// RFC 4360 compliance. Trailing partial communities are discarded.
func sortExtCommunities(data []byte) []byte {
	count := len(data) / 8
	if count < 2 {
		return data
	}
	if count*8 != len(data) {
		data = data[:count*8]
	}
	values := make([]uint64, count)
	for i := range count {
		o := i * 8
		values[i] = uint64(data[o])<<56 | uint64(data[o+1])<<48 | uint64(data[o+2])<<40 |
			uint64(data[o+3])<<32 | uint64(data[o+4])<<24 | uint64(data[o+5])<<16 |
			uint64(data[o+6])<<8 | uint64(data[o+7])
	}
	slices.Sort(values)
	out := make([]byte, len(data))
	for i, v := range values {
		o := i * 8
		out[o] = byte(v >> 56)
		out[o+1] = byte(v >> 48)
		out[o+2] = byte(v >> 40)
		out[o+3] = byte(v >> 32)
		out[o+4] = byte(v >> 24)
		out[o+5] = byte(v >> 16)
		out[o+6] = byte(v >> 8)
		out[o+7] = byte(v)
	}
	return out
}
