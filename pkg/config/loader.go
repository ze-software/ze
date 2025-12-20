package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/capability"
	"github.com/exa-networks/zebgp/pkg/reactor"
	"github.com/exa-networks/zebgp/pkg/trace"
)

// LoadReactor parses config and creates a configured Reactor.
func LoadReactor(input string) (*reactor.Reactor, error) {
	_, r, err := LoadReactorWithConfig(input)
	return r, err
}

// LoadReactorWithConfig parses config and returns both Config and Reactor.
func LoadReactorWithConfig(input string) (*BGPConfig, *reactor.Reactor, error) {
	// Parse input
	p := NewParser(BGPSchema())
	tree, err := p.Parse(input)
	if err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	// Log parse warnings
	trace.ConfigParsed("(input)", 0, p.Warnings())

	// Convert to typed config
	cfg, err := TreeToConfig(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("convert config: %w", err)
	}

	trace.ConfigLoaded(len(cfg.Neighbors))

	// Create reactor
	r, err := CreateReactor(cfg)
	if err != nil {
		return nil, nil, err
	}

	return cfg, r, nil
}

// LoadReactorFile loads config from file and creates Reactor.
func LoadReactorFile(path string) (*reactor.Reactor, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Config file path from user
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return LoadReactor(string(data))
}

// CreateReactor creates a Reactor from typed BGPConfig.
func CreateReactor(cfg *BGPConfig) (*reactor.Reactor, error) {
	// Build reactor config
	reactorCfg := &reactor.Config{
		ListenAddr: cfg.Listen,
		RouterID:   cfg.RouterID,
		LocalAS:    cfg.LocalAS,
	}

	r := reactor.New(reactorCfg)

	// Add neighbors
	for i := range cfg.Neighbors {
		neighbor, err := configToNeighbor(&cfg.Neighbors[i], cfg)
		if err != nil {
			return nil, fmt.Errorf("convert neighbor %s: %w", cfg.Neighbors[i].Address, err)
		}
		if err := r.AddNeighbor(neighbor); err != nil {
			return nil, fmt.Errorf("add neighbor %s: %w", cfg.Neighbors[i].Address, err)
		}
	}

	return r, nil
}

// configToNeighbor converts NeighborConfig to reactor.Neighbor.
func configToNeighbor(nc *NeighborConfig, global *BGPConfig) (*reactor.Neighbor, error) {
	// Determine local AS (inherit from global if not set)
	localAS := nc.LocalAS
	if localAS == 0 {
		localAS = global.LocalAS
	}

	// Determine router ID (inherit from global if not set)
	routerID := nc.RouterID
	if routerID == 0 {
		routerID = global.RouterID
	}

	// Determine hold time (default 90s)
	holdTime := time.Duration(nc.HoldTime) * time.Second
	if holdTime == 0 {
		holdTime = 90 * time.Second
	}

	n := reactor.NewNeighbor(nc.Address, localAS, nc.PeerAS, routerID)
	n.HoldTime = holdTime
	n.Passive = nc.Passive

	// Build capabilities.
	// Add Multiprotocol capabilities from configured families.
	for _, family := range nc.Families {
		switch family {
		case "ipv4 unicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv4,
				SAFI: capability.SAFIUnicast,
			})
		case "ipv6 unicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv6,
				SAFI: capability.SAFIUnicast,
			})
		case "ipv4 multicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv4,
				SAFI: capability.SAFIMulticast,
			})
		case "ipv6 multicast":
			n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
				AFI:  capability.AFIIPv6,
				SAFI: capability.SAFIMulticast,
			})
		}
	}

	// Add FQDN capability if hostname or domain is set.
	if nc.Hostname != "" || nc.DomainName != "" {
		n.Capabilities = append(n.Capabilities, &capability.FQDN{
			Hostname:   nc.Hostname,
			DomainName: nc.DomainName,
		})
	}
	if nc.Capabilities.SoftwareVersion {
		n.Capabilities = append(n.Capabilities, &capability.SoftwareVersion{
			Version: "ExaBGP/5.0.0-0+test",
		})
	}

	// Override port from environment (for testing).
	if p := os.Getenv("exabgp_tcp_port"); p != "" {
		if v, err := strconv.ParseUint(p, 10, 16); err == nil {
			n.Port = uint16(v) //nolint:gosec // Validated above
		}
	}

	// Convert static routes with typed attribute parsing.
	for _, sr := range nc.StaticRoutes {
		attrs, err := ParseRouteAttributes(sr)
		if err != nil {
			return nil, fmt.Errorf("static route %s: %w", sr.Prefix, err)
		}

		route := reactor.StaticRoute{
			Prefix:            attrs.Prefix,
			NextHop:           attrs.NextHop,
			Origin:            uint8(attrs.Origin),
			LocalPreference:   attrs.LocalPreference,
			MED:               attrs.MED,
			Communities:       attrs.Community.Values,
			LargeCommunities:  attrs.LargeCommunity.Values,
			ExtCommunity:      attrs.ExtendedCommunity.Raw,
			ExtCommunityBytes: attrs.ExtendedCommunity.Bytes,
			PathID:            uint32(attrs.PathID),
			Label:             uint32(attrs.Label),
			RD:                attrs.RD.Raw,
			RDBytes:           attrs.RD.Bytes,
		}

		n.StaticRoutes = append(n.StaticRoutes, route)
	}

	// Convert MVPN routes
	for _, mr := range nc.MVPNRoutes {
		route, err := convertMVPNRoute(mr)
		if err != nil {
			return nil, fmt.Errorf("mvpn route: %w", err)
		}
		n.MVPNRoutes = append(n.MVPNRoutes, route)
	}

	// Convert VPLS routes
	for _, vr := range nc.VPLSRoutes {
		route, err := convertVPLSRoute(vr)
		if err != nil {
			return nil, fmt.Errorf("vpls route %s: %w", vr.Name, err)
		}
		n.VPLSRoutes = append(n.VPLSRoutes, route)
	}

	// Convert FlowSpec routes
	for _, fr := range nc.FlowSpecRoutes {
		route, err := convertFlowSpecRoute(fr)
		if err != nil {
			return nil, fmt.Errorf("flowspec route %s: %w", fr.Name, err)
		}
		n.FlowSpecRoutes = append(n.FlowSpecRoutes, route)
	}

	// Convert MUP routes
	for _, mr := range nc.MUPRoutes {
		route, err := convertMUPRoute(mr)
		if err != nil {
			return nil, fmt.Errorf("mup route: %w", err)
		}
		n.MUPRoutes = append(n.MUPRoutes, route)
	}

	// Log static routes
	if len(n.StaticRoutes) > 0 {
		trace.NeighborRoutes(nc.Address.String(), len(n.StaticRoutes))
	}

	return n, nil
}

// convertMVPNRoute converts config MVPN route to reactor MVPN route.
func convertMVPNRoute(mr MVPNRouteConfig) (reactor.MVPNRoute, error) {
	route := reactor.MVPNRoute{
		IsIPv6:          mr.IsIPv6,
		SourceAS:        mr.SourceAS,
		LocalPreference: mr.LocalPreference,
		MED:             mr.MED,
	}

	// Route type
	switch mr.RouteType {
	case "source-ad":
		route.RouteType = 5
	case "shared-join":
		route.RouteType = 6
	case "source-join":
		route.RouteType = 7
	default:
		return route, fmt.Errorf("unknown MVPN route type: %s", mr.RouteType)
	}

	// Origin
	route.Origin = parseOrigin(mr.Origin)

	// Parse RD
	if mr.RD != "" {
		rd, err := ParseRouteDistinguisher(mr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse Source/RP IP
	if mr.Source != "" {
		ip, err := netip.ParseAddr(mr.Source)
		if err != nil {
			return route, fmt.Errorf("parse source: %w", err)
		}
		route.Source = ip
	}

	// Parse Group IP
	if mr.Group != "" {
		ip, err := netip.ParseAddr(mr.Group)
		if err != nil {
			return route, fmt.Errorf("parse group: %w", err)
		}
		route.Group = ip
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	return route, nil
}

// convertVPLSRoute converts config VPLS route to reactor VPLS route.
func convertVPLSRoute(vr VPLSRouteConfig) (reactor.VPLSRoute, error) {
	route := reactor.VPLSRoute{
		Name:            vr.Name,
		Endpoint:        vr.Endpoint,
		Base:            vr.Base,
		Offset:          vr.Offset,
		Size:            vr.Size,
		LocalPreference: vr.LocalPreference,
		MED:             vr.MED,
	}

	// Origin
	route.Origin = parseOrigin(vr.Origin)

	// Parse RD
	if vr.RD != "" {
		rd, err := ParseRouteDistinguisher(vr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if vr.NextHop != "" {
		ip, err := netip.ParseAddr(vr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse AS Path
	if vr.ASPath != "" {
		asPath, err := parseASPathSimple(vr.ASPath)
		if err != nil {
			return route, fmt.Errorf("parse as-path: %w", err)
		}
		route.ASPath = asPath
	}

	// Parse communities
	if vr.Community != "" {
		comm, err := ParseCommunity(vr.Community)
		if err != nil {
			return route, fmt.Errorf("parse community: %w", err)
		}
		route.Communities = comm.Values
	}

	// Parse extended communities
	if vr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(vr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// Parse originator-id
	if vr.OriginatorID != "" {
		ip, err := netip.ParseAddr(vr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (space-separated IPs)
	if vr.ClusterList != "" {
		parts := strings.Fields(vr.ClusterList)
		for _, p := range parts {
			// Remove brackets
			p = strings.Trim(p, "[]")
			if p == "" {
				continue
			}
			ip, err := netip.ParseAddr(p)
			if err != nil {
				return route, fmt.Errorf("parse cluster-list: %w", err)
			}
			if ip.Is4() {
				b := ip.As4()
				route.ClusterList = append(route.ClusterList, uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))
			}
		}
	}

	return route, nil
}

// convertFlowSpecRoute converts config FlowSpec route to reactor FlowSpec route.
func convertFlowSpecRoute(fr FlowSpecRouteConfig) (reactor.FlowSpecRoute, error) {
	route := reactor.FlowSpecRoute{
		Name:   fr.Name,
		IsIPv6: fr.IsIPv6,
	}

	// Parse RD for flow-vpn
	if fr.RD != "" {
		rd, err := ParseRouteDistinguisher(fr.RD)
		if err != nil {
			return route, fmt.Errorf("parse RD: %w", err)
		}
		route.RD = rd.Bytes
	}

	// Parse NextHop
	if fr.NextHop != "" {
		ip, err := netip.ParseAddr(fr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Build FlowSpec NLRI from match criteria
	// TODO: Build actual NLRI bytes from match map
	// For now, store as placeholder - needs flowspec encoding logic
	route.NLRI = buildFlowSpecNLRI(fr.Match, fr.IsIPv6)

	// Parse extended communities from "then" actions
	if ec := fr.ExtendedCommunity; ec != "" {
		extComm, err := ParseExtendedCommunity(ec)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = extComm.Bytes
	}

	return route, nil
}

// buildFlowSpecNLRI builds FlowSpec NLRI bytes from match criteria.
// This is a placeholder - full implementation requires flowspec component encoding.
func buildFlowSpecNLRI(match map[string]string, isIPv6 bool) []byte {
	// TODO: Implement proper FlowSpec NLRI encoding
	// Each match component needs to be encoded per RFC 5575
	return nil
}

// convertMUPRoute converts config MUP route to reactor MUP route.
func convertMUPRoute(mr MUPRouteConfig) (reactor.MUPRoute, error) {
	route := reactor.MUPRoute{
		IsIPv6: mr.IsIPv6,
	}

	// Route type
	switch mr.RouteType {
	case "mup-isd":
		route.RouteType = 1
	case "mup-dsd":
		route.RouteType = 2
	case "mup-t1st":
		route.RouteType = 3
	case "mup-t2st":
		route.RouteType = 4
	default:
		return route, fmt.Errorf("unknown MUP route type: %s", mr.RouteType)
	}

	// Parse NextHop
	if mr.NextHop != "" {
		ip, err := netip.ParseAddr(mr.NextHop)
		if err != nil {
			return route, fmt.Errorf("parse next-hop: %w", err)
		}
		route.NextHop = ip
	}

	// Parse extended communities
	if mr.ExtendedCommunity != "" {
		ec, err := ParseExtendedCommunity(mr.ExtendedCommunity)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = ec.Bytes
	}

	// TODO: Build MUP NLRI from route config
	// This requires MUP-specific encoding
	route.NLRI = buildMUPNLRI(mr)

	return route, nil
}

// buildMUPNLRI builds MUP NLRI bytes from route config.
// This is a placeholder - full implementation requires MUP NLRI encoding.
func buildMUPNLRI(mr MUPRouteConfig) []byte {
	// TODO: Implement proper MUP NLRI encoding
	return nil
}

// parseOrigin converts origin string to code.
// Empty or unset defaults to IGP (0).
func parseOrigin(s string) uint8 {
	switch strings.ToLower(s) {
	case "", "igp":
		return 0 // IGP is default
	case "egp":
		return 1
	default:
		return 2 // incomplete
	}
}

// parseASPathSimple parses an AS path string like "[ 30740 30740 ]" to []uint32.
func parseASPathSimple(s string) ([]uint32, error) {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint32
	for _, p := range parts {
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ASN: %s", p)
		}
		result = append(result, uint32(n))
	}
	return result, nil
}
