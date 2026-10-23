// Design: docs/architecture/wire/nlri-evpn.md -- explicit EVPN route origination
// RFC: rfc/short/rfc7432.md
package evpn

import (
	"net/netip"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// parseConfigRoute shares the route-command NLRI producer and sender checks.
// It emits the extended communities, its own merged with the configured ones,
// because the origination checks judge that merged set. The central config
// dispatcher encodes every other attribute, and BuildPlugin owns MP_REACH.
func parseConfigRoute(req registry.ConfigRouteRequest) (registry.PluginRoute, error) {
	route, intent, err := parseL2VPNArgs(req.Content)
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
	params, err := l2vpnRouteToEVPNParams(route, intent)
	if err != nil {
		return registry.PluginRoute{}, err
	}
	if err := message.ValidateEVPNOriginationMode(params.NLRI, params.ExtCommunityBytes, false, params.RedundancyMode); err != nil {
		return registry.PluginRoute{}, err
	}

	var attrs []registry.PluginRouteAttr
	if len(params.ExtCommunityBytes) != 0 {
		attrs = append(attrs, registry.PluginRouteAttr{Code: uint8(attribute.AttrExtCommunity), Flags: uint8(attribute.FlagOptional | attribute.FlagTransitive), Value: params.ExtCommunityBytes})
	}
	return registry.PluginRoute{
		NLRI: params.NLRI, NextHop: nextHop, Attrs: attrs,
	}, nil
}
