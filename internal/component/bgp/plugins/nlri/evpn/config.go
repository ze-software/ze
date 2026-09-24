// Design: docs/architecture/wire/nlri-evpn.md -- explicit EVPN route origination
// RFC: rfc/short/rfc7432.md
package evpn

import (
	"encoding/binary"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// parseConfigRoute shares the route-command NLRI producer and sender checks.
// BuildPlugin supplies the session-dependent AS_PATH, LOCAL_PREF and MP_REACH.
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	route, err := parseL2VPNArgs(req.Content)
	if err != nil {
		return registry.PluginRoute{}, err
	}
	nextHop := req.NextHop
	if nextHop != "" {
		if addr, parseErr := netip.ParseAddr(nextHop); parseErr == nil {
			route.NextHop = addr
		}
	} else if route.NextHop.IsValid() {
		nextHop = route.NextHop.String()
	}
	if len(route.ExtCommunityBytes) == 0 {
		route.ExtCommunityBytes = req.ExtCommunity
	} else {
		route.ExtCommunityBytes = append(route.ExtCommunityBytes, req.ExtCommunity...)
	}
	params, err := l2vpnRouteToEVPNParams(route)
	if err != nil {
		return registry.PluginRoute{}, err
	}
	if err := message.ValidateEVPNOrigination(params.NLRI, params.ExtCommunityBytes, false); err != nil {
		return registry.PluginRoute{}, err
	}

	attrs := []registry.PluginRouteAttr{{Code: uint8(attribute.AttrOrigin), Flags: uint8(attribute.FlagTransitive), Value: []byte{req.Origin}}}
	if req.MED != 0 {
		attrs = append(attrs, registry.PluginRouteAttr{
			Code: uint8(attribute.AttrMED), Flags: uint8(attribute.FlagOptional),
			Value: []byte{byte(req.MED >> 24), byte(req.MED >> 16), byte(req.MED >> 8), byte(req.MED)},
		})
	}
	if len(req.Community) != 0 {
		communities := slices.Clone(req.Community)
		slices.Sort(communities)
		value := make([]byte, 4*len(communities))
		for i, community := range communities {
			binary.BigEndian.PutUint32(value[4*i:], community)
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrCommunity), Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive), Value: value})
	}
	if req.OriginatorID != 0 {
		id := req.OriginatorID
		attrs = append(attrs, registry.PluginRouteAttr{
			Code: uint8(attribute.AttrOriginatorID), Flags: uint8(attribute.FlagOptional),
			Value: []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)},
		})
	}
	if len(req.ClusterList) != 0 {
		value := make([]byte, 4*len(req.ClusterList))
		for i, id := range req.ClusterList {
			binary.BigEndian.PutUint32(value[4*i:], id)
		}
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrClusterList), Flags: uint8(attribute.FlagOptional), Value: value})
	}
	if len(params.ExtCommunityBytes) != 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrExtCommunity), Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive), Value: params.ExtCommunityBytes})
	}
	if len(req.IPv6ExtCommunity) != 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrIPv6ExtCommunity), Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive), Value: req.IPv6ExtCommunity})
	}
	if len(req.PrefixSID) != 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrPrefixSID), Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive), Value: req.PrefixSID})
	}
	return registry.PluginRoute{
		NLRI: params.NLRI, NextHop: nextHop, Attrs: attrs,
		ASPath: req.ASPath, LocalPreference: req.LocalPreference,
	}, nil
}
