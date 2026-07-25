// Design: docs/architecture/config/syntax.md -- MVPN config route parsing
// RFC: rfc/short/rfc6514.md -- MCAST-VPN NLRI (SAFI 5) + route grouping
// Related: types.go -- MVPN NLRI types

package mvpn

import (
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// MVPN path attribute wire constants.
const (
	attrCodeOrigin       uint8 = 1
	attrCodeNextHop      uint8 = 3
	attrCodeMED          uint8 = 4
	attrCodeOriginatorID uint8 = 9
	attrCodeClusterList  uint8 = 10
	attrCodeExtComm      uint8 = 16

	flagWellKnownTrans = 0x40 // ORIGIN, NEXT_HOP.
	flagOptional       = 0x80 // MED, ORIGINATOR_ID, CLUSTER_LIST.
	flagOptTrans       = 0xC0 // EXT_COMMUNITIES.
)

var (
	errMVPNMissingRouteType = errors.New("mvpn nlri requires a route type (shared-join/source-join/source-ad)")
	errMVPNMissingRD        = errors.New("mvpn nlri requires an rd")
)

// parseConfigRoute implements registry.InProcessConfigRouteParser for MVPN.
// It builds the RFC 6514 MCAST-VPN NLRI from the content tokens and assembles
// the generic path attributes from the pre-parsed attribute block. MVPN sets
// Group=true so the reactor packs same-attribute routes into one UPDATE.
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	if len(req.Content) == 0 {
		return registry.PluginRoute{}, errMVPNMissingRouteType
	}

	var routeType byte
	switch req.Content[0] {
	case "source-ad":
		routeType = 5
	case "shared-join":
		routeType = 6
	case "source-join":
		routeType = 7
	default:
		return registry.PluginRoute{}, fmt.Errorf("unknown MVPN route type %q", req.Content[0])
	}

	var source, group, rdStr string
	var sourceAS uint32
	for i := 1; i < len(req.Content); i += 2 {
		key := req.Content[i]
		if i+1 >= len(req.Content) {
			return registry.PluginRoute{}, fmt.Errorf("missing value for %s", key)
		}
		val := req.Content[i+1]
		switch key {
		case "rp", "source":
			source = val
		case "group":
			group = val
		case "rd":
			rdStr = val
		case "source-as":
			n, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				return registry.PluginRoute{}, fmt.Errorf("mvpn source-as %q: %w", val, err)
			}
			sourceAS = uint32(n)
		default:
			return registry.PluginRoute{}, fmt.Errorf("unknown MVPN keyword: %s", key)
		}
	}

	if rdStr == "" {
		return registry.PluginRoute{}, errMVPNMissingRD
	}
	rdBytes, err := rdStringToBytes(rdStr)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("mvpn rd: %w", err)
	}
	srcAddr, err := netip.ParseAddr(source)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("mvpn source: %w", err)
	}
	grpAddr, err := netip.ParseAddr(group)
	if err != nil {
		return registry.PluginRoute{}, fmt.Errorf("mvpn group: %w", err)
	}

	nlri, err := mvpnNLRI(routeType, rdBytes, sourceAS, srcAddr, grpAddr)
	if err != nil {
		return registry.PluginRoute{}, err
	}

	// ORIGIN (code 1) is always present (matches BuildMVPN).
	attrs := []registry.PluginRouteAttr{{Code: attrCodeOrigin, Flags: flagWellKnownTrans, Value: []byte{req.Origin}}}

	// NEXT_HOP (code 3): MVPN carries a legacy NEXT_HOP whenever the next-hop is
	// IPv4, regardless of the NLRI AFI (matches BuildMVPN).
	if req.NextHop != "" {
		if ip, err := netip.ParseAddr(req.NextHop); err == nil && ip.Is4() {
			b := ip.As4()
			attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeNextHop, Flags: flagWellKnownTrans, Value: b[:]})
		}
	}
	if req.MED > 0 {
		m := req.MED
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeMED, Flags: flagOptional, Value: []byte{byte(m >> 24), byte(m >> 16), byte(m >> 8), byte(m)}})
	}
	if req.OriginatorID != 0 {
		o := req.OriginatorID
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeOriginatorID, Flags: flagOptional, Value: []byte{byte(o >> 24), byte(o >> 16), byte(o >> 8), byte(o)}})
	}
	if len(req.ClusterList) > 0 {
		val := make([]byte, 0, 4*len(req.ClusterList))
		for _, c := range req.ClusterList {
			val = append(val, byte(c>>24), byte(c>>16), byte(c>>8), byte(c))
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeClusterList, Flags: flagOptional, Value: val})
	}
	// EXTENDED_COMMUNITIES (code 16): config order, unsorted (MVPN convention).
	if len(req.ExtCommunity) > 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: attrCodeExtComm, Flags: flagOptTrans, Value: req.ExtCommunity})
	}

	return registry.PluginRoute{
		IsIPv6:          req.IsIPv6,
		NLRI:            nlri,
		NextHop:         req.NextHop,
		Attrs:           attrs,
		ASPath:          req.ASPath,
		LocalPreference: req.LocalPreference,
		Group:           true,
	}, nil
}

var errMVPNNLRITooLong = errors.New("mvpn nlri data exceeds the 255-byte RFC 6514 Length field")

// mvpnNLRI builds an RFC 6514 MCAST-VPN NLRI: RouteType(1) Length(1) Data, where
// Data = RD(8) [+ Source-AS(4) for join types 6/7] + Source(addr) + Group(addr).
// The Length field is a single octet, so the data is rejected if it exceeds 255
// bytes (route types 5/6/7 max out at 46 today; the guard protects future types).
func mvpnNLRI(routeType byte, rd [8]byte, sourceAS uint32, source, group netip.Addr) ([]byte, error) {
	data := make([]byte, 0, 8+4+17+17)
	data = append(data, rd[:]...)
	if routeType == 6 || routeType == 7 {
		data = append(data, byte(sourceAS>>24), byte(sourceAS>>16), byte(sourceAS>>8), byte(sourceAS))
	}
	data = appendMVPNAddr(data, source)
	data = appendMVPNAddr(data, group)

	if len(data) > 255 {
		return nil, errMVPNNLRITooLong
	}

	nlri := make([]byte, 0, 2+len(data))
	nlri = append(nlri, routeType, byte(len(data)))
	nlri = append(nlri, data...)
	return nlri, nil
}

// appendMVPNAddr appends a prefix-length + address pair (RFC 6514 Section 4):
// length is 32 for IPv4, 128 for IPv6.
func appendMVPNAddr(buf []byte, addr netip.Addr) []byte {
	if addr.Is4() {
		b := addr.As4()
		buf = append(buf, 32)
		return append(buf, b[:]...)
	}
	b := addr.As16()
	buf = append(buf, 128)
	return append(buf, b[:]...)
}

// rdStringToBytes parses an RFC 4364 Route Distinguisher string (ASN:NN or IP:NN)
// into its 8-byte wire form (2-byte type + 6-byte value).
func rdStringToBytes(s string) ([8]byte, error) {
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
		rd[1] = 0 // Type 0 (2-byte ASN, 4-byte number)
		rd[2], rd[3] = byte(asn>>8), byte(asn)
		rd[4], rd[5], rd[6], rd[7] = byte(num>>24), byte(num>>16), byte(num>>8), byte(num)
	} else {
		rd[1] = 2 // Type 2 (4-byte ASN, 2-byte number)
		rd[2], rd[3], rd[4], rd[5] = byte(asn>>24), byte(asn>>16), byte(asn>>8), byte(asn)
		rd[6], rd[7] = byte(num>>8), byte(num)
	}
	return rd, nil
}
