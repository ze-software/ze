// Design: docs/architecture/config/syntax.md — peer configuration extraction and route expansion
// Related: loader_prefix.go — prefix expansion for route splitting
// Related: loader_routes.go — BGP route type conversion

package bgpconfig

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/env"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/reactor"
)

// Env var registrations for BGP config overrides.
var (
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.tcp.port", Type: "int", Default: "179", Description: "Override BGP TCP port for all peers"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.bgp.connect", Type: "bool", Default: "true", Description: "Override peer connect mode (initiate outbound)"})
	_ = env.MustRegister(env.EnvEntry{Key: "ze.bgp.bgp.accept", Type: "bool", Default: "true", Description: "Override peer accept mode (accept inbound)"})
)

// PeersFromConfigTree builds PeerSettings from a config tree.
// This replaces the TreeToConfig → configToPeer pipeline by:
//  1. Resolving templates at the map level (ResolveBGPTree)
//  2. Parsing basic peer settings via reactor.PeersFromTree
//  3. Extracting routes from all template layers (globs → templates → peer)
//  4. Applying environment overrides (port)
//
// Routes stay in the config package because they depend on config-internal
// types (StaticRouteConfig, ParseRouteAttributes, etc.) that reactor cannot import.
func PeersFromConfigTree(tree *config.Tree) ([]*reactor.PeerSettings, error) {
	// Step 1: Resolve templates at the map level.
	bgpTree, err := ResolveBGPTree(tree)
	if err != nil {
		return nil, err
	}

	// Step 2: Parse basic peer settings from the resolved map.
	peers, err := reactor.PeersFromTree(bgpTree)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return peers, nil
	}

	// Step 3: Extract routes from group and peer layers.
	// Routes accumulate from 2 layers:
	//   Layer 1: Group-level routes (shared by all peers in the group)
	//   Layer 2: Peer's own routes
	bgpContainer := tree.GetContainer("bgp")
	if bgpContainer == nil {
		return peers, nil
	}

	// Build name -> PeerSettings index for matching.
	// The peer list key is now the peer name (not the IP address).
	peerIndex := make(map[string]*reactor.PeerSettings, len(peers))
	for _, ps := range peers {
		peerIndex[ps.Name] = ps
	}

	// Grouped peers: routes from group + peer layers.
	for _, groupEntry := range bgpContainer.GetListOrdered("group") {
		groupTree := groupEntry.Value

		for _, peerEntry := range groupTree.GetListOrdered("peer") {
			addr := peerEntry.Key
			peerTree := peerEntry.Value

			ps, ok := peerIndex[addr]
			if !ok {
				continue
			}

			// Layer 1: Routes from group defaults.
			if err := patchRoutes(ps, addr, groupTree); err != nil {
				return nil, err
			}

			// Layer 2: Routes from peer's own tree.
			if err := patchRoutes(ps, addr, peerTree); err != nil {
				return nil, err
			}
		}
	}

	// Standalone peers: routes from peer's own tree only.
	for _, peerEntry := range bgpContainer.GetListOrdered("peer") {
		addr := peerEntry.Key
		peerTree := peerEntry.Value

		ps, ok := peerIndex[addr]
		if !ok {
			continue
		}

		if err := patchRoutes(ps, addr, peerTree); err != nil {
			return nil, err
		}
	}

	// Step 4: Apply environment overrides.
	applyPortOverride(peers)
	applyConnectionOverride(peers)

	// Step 4b: Re-validate connection mode after env overrides.
	for _, ps := range peers {
		if !ps.Connection.Connect && !ps.Connection.Accept {
			return nil, fmt.Errorf("peer %s: connect and accept cannot both be false (after env override)", ps.Name)
		}
	}

	// Step 5: Validate capability-process constraints.
	if err := ValidatePeerProcessCaps(peers); err != nil {
		return nil, err
	}

	return peers, nil
}

// patchRoutes extracts routes from a peer's *Tree and patches them into PeerSettings.
func patchRoutes(ps *reactor.PeerSettings, addr string, peerTree *config.Tree) error {
	// Extract routes from peer's own tree.
	routes, err := extractRoutesFromTree(peerTree)
	if err != nil {
		return fmt.Errorf("peer %s routes: %w", addr, err)
	}

	// Convert and patch static routes.
	if err := patchStaticRoutes(ps, routes.StaticRoutes, addr); err != nil {
		return err
	}

	// Convert and patch exotic routes.
	for i := range routes.MVPNRoutes {
		route, err := convertMVPNRoute(routes.MVPNRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s mvpn route: %w", addr, err)
		}
		ps.MVPNRoutes = append(ps.MVPNRoutes, route)
	}

	for i := range routes.VPLSRoutes {
		route, err := convertVPLSRoute(routes.VPLSRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s vpls route: %w", addr, err)
		}
		ps.VPLSRoutes = append(ps.VPLSRoutes, route)
	}

	for i := range routes.FlowSpecRoutes {
		route, err := convertFlowSpecRoute(routes.FlowSpecRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s flowspec route: %w", addr, err)
		}
		ps.FlowSpecRoutes = append(ps.FlowSpecRoutes, route)
	}

	for i := range routes.MUPRoutes {
		route, err := convertMUPRoute(routes.MUPRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s mup route: %w", addr, err)
		}
		ps.MUPRoutes = append(ps.MUPRoutes, route)
	}

	// Extract exotic routes from legacy ExaBGP syntax blocks.
	mvpnRoutes := extractMVPNRoutes(peerTree)
	for i := range mvpnRoutes {
		route, err := convertMVPNRoute(mvpnRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s mvpn route: %w", addr, err)
		}
		ps.MVPNRoutes = append(ps.MVPNRoutes, route)
	}

	vplsRoutes := extractVPLSRoutes(peerTree)
	for i := range vplsRoutes {
		route, err := convertVPLSRoute(vplsRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s vpls route: %w", addr, err)
		}
		ps.VPLSRoutes = append(ps.VPLSRoutes, route)
	}

	flowSpecRoutes := extractFlowSpecRoutes(peerTree)
	for i := range flowSpecRoutes {
		route, err := convertFlowSpecRoute(flowSpecRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s flowspec route: %w", addr, err)
		}
		ps.FlowSpecRoutes = append(ps.FlowSpecRoutes, route)
	}

	mupRoutes := extractMUPRoutes(peerTree)
	for i := range mupRoutes {
		route, err := convertMUPRoute(mupRoutes[i])
		if err != nil {
			return fmt.Errorf("peer %s mup route: %w", addr, err)
		}
		ps.MUPRoutes = append(ps.MUPRoutes, route)
	}

	return nil
}

// patchStaticRoutes converts StaticRouteConfig to reactor.StaticRoute and adds them to PeerSettings.
func patchStaticRoutes(ps *reactor.PeerSettings, routes []StaticRouteConfig, addr string) error {
	for i := range routes {
		sr := &routes[i]
		attrs, err := ParseRouteAttributes(sr)
		if err != nil {
			return fmt.Errorf("peer %s static route %s: %w", addr, sr.Prefix, err)
		}

		// Create RouteNextHop from config.
		var nextHop bgptypes.RouteNextHop
		if sr.NextHopSelf {
			nextHop = bgptypes.NewNextHopSelf()
		} else if attrs.NextHop.IsValid() {
			nextHop = bgptypes.NewNextHopExplicit(attrs.NextHop)
		}

		// Convert raw attributes.
		var rawAttrs []reactor.RawAttribute
		for _, ra := range attrs.RawAttributes {
			rawAttrs = append(rawAttrs, reactor.RawAttribute{
				Code:  ra.Code,
				Flags: ra.Flags,
				Value: ra.Value,
			})
		}

		// Handle split: expand prefix into more-specific prefixes.
		prefixes := []netip.Prefix{attrs.Prefix}
		if splitLen := parseSplitLen(sr.Split); splitLen > 0 {
			prefixes = expandPrefix(attrs.Prefix, splitLen)
		}

		// Create a route for each prefix (usually just one, unless split).
		for _, prefix := range prefixes {
			labels := make([]uint32, len(attrs.Labels))
			for i, l := range attrs.Labels {
				labels[i] = uint32(l)
			}

			route := reactor.StaticRoute{
				Prefix:            prefix,
				NextHop:           nextHop,
				Origin:            uint8(attrs.Origin),
				LocalPreference:   attrs.LocalPreference,
				MED:               attrs.MED,
				Communities:       attrs.Community.Values,
				LargeCommunities:  attrs.LargeCommunity.Values,
				ExtCommunity:      attrs.ExtendedCommunity.Raw,
				ExtCommunityBytes: sortExtCommunities(attrs.ExtendedCommunity.Bytes),
				PathID:            uint32(attrs.PathID),
				Labels:            labels,
				RD:                attrs.RD.Raw,
				RDBytes:           attrs.RD.Bytes,
				ASPath:            attrs.ASPath.Values,
				AggregatorASN:     attrs.Aggregator.ASN,
				AggregatorIP:      attrs.Aggregator.IP,
				HasAggregator:     attrs.Aggregator.Valid,
				AtomicAggregate:   attrs.AtomicAggregate,
				OriginatorID:      attrs.OriginatorID,
				ClusterList:       attrs.ClusterList,
				PrefixSIDBytes:    attrs.PrefixSID.Bytes,
				RawAttributes:     rawAttrs,
			}

			// RFC 4364: VPN routes require at least one label.
			if route.IsVPN() && len(route.Labels) == 0 {
				return fmt.Errorf("peer %s VPN route %s requires at least one label", addr, prefix)
			}

			ps.StaticRoutes = append(ps.StaticRoutes, route)
		}
	}

	return nil
}

// ValidatePeerProcessCaps checks that peers with route-refresh or graceful-restart
// capabilities have at least one process binding with SendUpdate=true.
// These capabilities require a process to resend routes on demand.
func ValidatePeerProcessCaps(peers []*reactor.PeerSettings) error {
	for _, ps := range peers {
		needsProcess := false
		capName := ""
		for _, cap := range ps.Capabilities {
			switch cap.Code() { //nolint:exhaustive // only route-refresh and GR require process bindings
			case capability.CodeRouteRefresh:
				needsProcess = true
				if capName == "" {
					capName = "route-refresh"
				}
			case capability.CodeGracefulRestart:
				needsProcess = true
				capName = "graceful-restart"
			}
		}
		// Graceful-restart is stored in RawCapabilityConfig (built by GR plugin at runtime),
		// not as a capability.Capability in the slice.
		if _, ok := ps.RawCapabilityConfig["graceful-restart"]; ok {
			needsProcess = true
			capName = "graceful-restart"
		}
		if !needsProcess {
			continue
		}

		hasValidProcess := false
		for _, b := range ps.ProcessBindings {
			if b.SendUpdate {
				hasValidProcess = true
				break
			}
		}
		if hasValidProcess {
			continue
		}

		if len(ps.ProcessBindings) == 0 {
			return fmt.Errorf("peer %s: %s requires process with send [ update ]\n  no process bindings configured",
				ps.Address, capName)
		}
		var names []string
		for _, b := range ps.ProcessBindings {
			names = append(names, "process "+b.PluginName)
		}
		return fmt.Errorf("peer %s: %s requires process with send [ update ]\n  configured: %s - none have send [ update ]",
			ps.Address, capName, strings.Join(names, ", "))
	}
	return nil
}

// applyPortOverride overrides peer port from ze.bgp.tcp.port (dot or underscore notation).
func applyPortOverride(peers []*reactor.PeerSettings) {
	p := env.Get("ze.bgp.tcp.port")
	if p == "" {
		return
	}
	v, err := strconv.ParseUint(p, 10, 16)
	if err != nil {
		return
	}
	port := uint16(v) //nolint:gosec // Validated above
	for _, ps := range peers {
		ps.Port = port
	}
}

// applyConnectionOverride overrides peer connection mode from
// ze.bgp.bgp.connect and ze.bgp.bgp.accept (dot or underscore notation).
func applyConnectionOverride(peers []*reactor.PeerSettings) {
	if v := env.Get("ze.bgp.bgp.connect"); v != "" {
		if connect, err := config.ParseBoolStrict(v); err == nil {
			for _, ps := range peers {
				ps.Connection.Connect = connect
			}
		}
	}
	if v := env.Get("ze.bgp.bgp.accept"); v != "" {
		if accept, err := config.ParseBoolStrict(v); err == nil {
			for _, ps := range peers {
				ps.Connection.Accept = accept
			}
		}
	}
}
