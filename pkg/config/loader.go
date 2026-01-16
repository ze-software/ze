package config

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/capability"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/plugin"
	"codeberg.org/thomas-mangin/zebgp/pkg/reactor"
	"codeberg.org/thomas-mangin/zebgp/pkg/trace"
)

// Origin attribute values.
const (
	originIGP = "igp"
	originEGP = "egp"
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
		// Check if this looks like old syntax and provide migration hint
		if hint := detectLegacySyntaxHint(input, err); hint != "" {
			return nil, nil, fmt.Errorf("parse config: %w\n\n%s", err, hint)
		}
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	// Log parse warnings
	trace.ConfigParsed("(input)", 0, p.Warnings())

	// Extract environment block (ZeBGP-specific, before conversion)
	envValues := ExtractEnvironment(tree)

	// Convert to typed config
	cfg, err := TreeToConfig(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("convert config: %w", err)
	}

	// Store environment values for later use
	cfg.EnvValues = envValues

	trace.ConfigLoaded(len(cfg.Peers))

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

	cfg, _, err := LoadReactorWithConfig(string(data))
	if err != nil {
		return nil, err
	}

	// Set config directory for process execution
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	cfg.ConfigDir = filepath.Dir(absPath)

	// Recreate reactor with config dir set
	return CreateReactorWithDir(cfg, cfg.ConfigDir)
}

// CreateReactor creates a Reactor from typed BGPConfig.
func CreateReactor(cfg *BGPConfig) (*reactor.Reactor, error) {
	return CreateReactorWithDir(cfg, "")
}

// CreateReactorWithDir creates a Reactor with a specific config directory.
// The configDir is used as the working directory for spawned processes.
func CreateReactorWithDir(cfg *BGPConfig, configDir string) (*reactor.Reactor, error) {
	// Load environment with config block values (if any)
	env, err := LoadEnvironmentWithConfig(cfg.EnvValues)
	if err != nil {
		return nil, fmt.Errorf("load environment: %w", err)
	}

	// Build reactor config
	reactorCfg := &reactor.Config{
		ListenAddr:  cfg.Listen,
		RouterID:    cfg.RouterID,
		LocalAS:     cfg.LocalAS,
		ConfigDir:   configDir,
		MaxSessions: env.TCP.Attempts, // tcp.attempts: exit after N sessions (0=unlimited)
	}

	// Set API socket path if plugins are configured
	if len(cfg.Plugins) > 0 {
		reactorCfg.APISocketPath = env.SocketPath()

		// Convert plugin configs
		for _, pc := range cfg.Plugins {
			reactorCfg.Plugins = append(reactorCfg.Plugins, reactor.PluginConfig{
				Name:          pc.Name,
				Run:           pc.Run,
				Encoder:       pc.Encoder,
				ReceiveUpdate: pc.ReceiveUpdate,
				StageTimeout:  pc.StageTimeout,
			})
		}
	}

	r := reactor.New(reactorCfg)

	// Add peers
	for i := range cfg.Peers {
		settings, err := configToPeer(&cfg.Peers[i], cfg)
		if err != nil {
			return nil, fmt.Errorf("convert peer %s: %w", cfg.Peers[i].Address, err)
		}
		if err := r.AddPeer(settings); err != nil {
			return nil, fmt.Errorf("add peer %s: %w", cfg.Peers[i].Address, err)
		}
	}

	return r, nil
}

// configToPeer converts PeerConfig to reactor.PeerSettings.
func configToPeer(nc *PeerConfig, global *BGPConfig) (*reactor.PeerSettings, error) {
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

	n := reactor.NewPeerSettings(nc.Address, localAS, nc.PeerAS, routerID)
	n.HoldTime = holdTime
	n.Passive = nc.Passive
	n.GroupUpdates = nc.GroupUpdates
	n.LocalAddress = nc.LocalAddress
	n.IgnoreFamilyMismatch = nc.IgnoreFamilyMismatch

	// Build capabilities.
	// Add Multiprotocol capabilities from configured families.
	// Skip disabled families, track required families.
	for _, fc := range nc.FamilyConfigs {
		// Skip disabled families
		if fc.Mode == FamilyModeDisable {
			continue
		}

		// Map AFI/SAFI strings to capability types
		familyKey := fc.AFI + "/" + fc.SAFI
		family, ok := nlri.ParseFamily(familyKey)
		if !ok {
			// Unknown family, skip
			continue
		}
		afi, safi := family.AFI, family.SAFI

		// Add capability
		n.Capabilities = append(n.Capabilities, &capability.Multiprotocol{
			AFI:  afi,
			SAFI: safi,
		})

		// Track required families
		if fc.Mode == FamilyModeRequire {
			n.RequiredFamilies = append(n.RequiredFamilies, capability.Family{
				AFI:  afi,
				SAFI: safi,
			})
		}

		// Track ignore families (lenient UPDATE validation)
		if fc.Mode == FamilyModeIgnore {
			n.IgnoreFamilies = append(n.IgnoreFamilies, capability.Family{
				AFI:  afi,
				SAFI: safi,
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

	// RFC 8950: Build ExtendedNextHop capability from nexthop families config.
	// Only include if capability is enabled AND there are families defined.
	if len(nc.NexthopFamilies) > 0 {
		extNH := &capability.ExtendedNextHop{
			Families: make([]capability.ExtendedNextHopFamily, 0, len(nc.NexthopFamilies)),
		}
		for _, nhf := range nc.NexthopFamilies {
			extNH.Families = append(extNH.Families, capability.ExtendedNextHopFamily{
				NLRIAFI:    capability.AFI(nhf.NLRIAFI),
				NLRISAFI:   capability.SAFI(nhf.NLRISAFI),
				NextHopAFI: capability.AFI(nhf.NextHopAFI),
			})
		}
		n.Capabilities = append(n.Capabilities, extNH)
	}

	// RFC 7911: Build ADD-PATH capability from config.
	// Global add-path applies to all configured families.
	// Per-family add-path overrides global settings.
	if nc.Capabilities.AddPathSend || nc.Capabilities.AddPathReceive || len(nc.AddPathFamilies) > 0 {
		addPath := &capability.AddPath{
			Families: make([]capability.AddPathFamily, 0),
		}

		// Global mode
		var globalMode capability.AddPathMode
		switch {
		case nc.Capabilities.AddPathSend && nc.Capabilities.AddPathReceive:
			globalMode = capability.AddPathBoth
		case nc.Capabilities.AddPathSend:
			globalMode = capability.AddPathSend
		case nc.Capabilities.AddPathReceive:
			globalMode = capability.AddPathReceive
		}

		// Apply global mode to all configured families
		if globalMode != capability.AddPathNone {
			for _, cap := range n.Capabilities {
				if mp, ok := cap.(*capability.Multiprotocol); ok {
					addPath.Families = append(addPath.Families, capability.AddPathFamily{
						AFI:  mp.AFI,
						SAFI: mp.SAFI,
						Mode: globalMode,
					})
				}
			}
		}

		// Override with per-family settings
		for _, apf := range nc.AddPathFamilies {
			var mode capability.AddPathMode
			switch {
			case apf.Send && apf.Receive:
				mode = capability.AddPathBoth
			case apf.Send:
				mode = capability.AddPathSend
			case apf.Receive:
				mode = capability.AddPathReceive
			}
			if mode != capability.AddPathNone {
				// Parse family string like "ipv4/unicast"
				family, ok := nlri.ParseFamily(apf.Family)
				if !ok {
					continue // Skip unknown families
				}
				afi, safi := family.AFI, family.SAFI
				addPath.Families = append(addPath.Families, capability.AddPathFamily{
					AFI:  afi,
					SAFI: safi,
					Mode: mode,
				})
			}
		}

		if len(addPath.Families) > 0 {
			n.Capabilities = append(n.Capabilities, addPath)
		}
	}

	// RFC 2918/7313: Add RouteRefresh and EnhancedRouteRefresh capabilities.
	// Both are needed for RFC 7313 BoRR/EoRR support.
	if nc.Capabilities.RouteRefresh {
		n.Capabilities = append(n.Capabilities, &capability.RouteRefresh{})
		n.Capabilities = append(n.Capabilities, &capability.EnhancedRouteRefresh{})
	}

	// ASN4 is enabled by default, disable if explicitly set to false in config.
	n.DisableASN4 = !nc.Capabilities.ASN4

	// Override port from environment (for testing).
	if p := os.Getenv("zebgp_tcp_port"); p != "" {
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

		// Create RouteNextHop from config
		var nextHop plugin.RouteNextHop
		if sr.NextHopSelf {
			nextHop = plugin.NewNextHopSelf()
		} else if attrs.NextHop.IsValid() {
			nextHop = plugin.NewNextHopExplicit(attrs.NextHop)
		}

		// Convert raw attributes
		var rawAttrs []reactor.RawAttribute
		for _, ra := range attrs.RawAttributes {
			rawAttrs = append(rawAttrs, reactor.RawAttribute{
				Code:  ra.Code,
				Flags: ra.Flags,
				Value: ra.Value,
			})
		}

		// Handle split: expand prefix into more-specific prefixes
		prefixes := []netip.Prefix{attrs.Prefix}
		if splitLen := parseSplitLen(sr.Split); splitLen > 0 {
			prefixes = splitPrefix(attrs.Prefix, splitLen)
		}

		// Create a route for each prefix (usually just one, unless split)
		for _, prefix := range prefixes {
			// Convert labels from MPLSLabel to uint32
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
				ExtCommunityBytes: attrs.ExtendedCommunity.Bytes,
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

			// Validate: VPN routes require at least one label
			// RFC 4364: MPLS label is required for VPN routes
			if route.IsVPN() && len(route.Labels) == 0 {
				return nil, fmt.Errorf("VPN route %s requires at least one label", prefix)
			}

			// Route to correct bucket based on watchdog field
			if sr.Watchdog != "" {
				wr := reactor.WatchdogRoute{
					StaticRoute:        route,
					InitiallyWithdrawn: sr.WatchdogWithdraw,
				}
				if n.WatchdogGroups == nil {
					n.WatchdogGroups = make(map[string][]reactor.WatchdogRoute)
				}
				n.WatchdogGroups[sr.Watchdog] = append(n.WatchdogGroups[sr.Watchdog], wr)
			} else {
				n.StaticRoutes = append(n.StaticRoutes, route)
			}
		}
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
		trace.PeerRoutes(nc.Address.String(), len(n.StaticRoutes))
	}

	// Convert process bindings
	for _, pb := range nc.ProcessBindings {
		n.ProcessBindings = append(n.ProcessBindings, reactor.ProcessBinding{
			PluginName:          pb.PluginName,
			Encoding:            pb.Content.Encoding,
			Format:              pb.Content.Format,
			ReceiveUpdate:       pb.Receive.Update,
			ReceiveOpen:         pb.Receive.Open,
			ReceiveNotification: pb.Receive.Notification,
			ReceiveKeepalive:    pb.Receive.Keepalive,
			ReceiveRefresh:      pb.Receive.Refresh,
			ReceiveState:        pb.Receive.State,
			ReceiveSent:         pb.Receive.Sent,
			ReceiveNegotiated:   pb.Receive.Negotiated,
			SendUpdate:          pb.Send.Update,
			SendRefresh:         pb.Send.Refresh,
		})
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

	// Parse originator-id (RFC 4456)
	if mr.OriginatorID != "" {
		ip, err := netip.ParseAddr(mr.OriginatorID)
		if err != nil {
			return route, fmt.Errorf("parse originator-id: %w", err)
		}
		if ip.Is4() {
			b := ip.As4()
			route.OriginatorID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
		}
	}

	// Parse cluster-list (RFC 4456, space-separated IPs)
	if mr.ClusterList != "" {
		parts := strings.Fields(mr.ClusterList)
		for _, p := range parts {
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
// RFC 8955 Section 4 defines the FlowSpec NLRI format.
// RFC 8955 Section 7 defines the Traffic Filtering Actions (extended communities).
// RFC 8955 Section 8 defines the FlowSpec VPN variant (SAFI 134) with Route Distinguisher.
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
	// For VPN routes, use component bytes (no length prefix - VPN adds its own)
	isVPN := fr.RD != ""
	route.NLRI = buildFlowSpecNLRI(fr.Match, fr.IsIPv6, isVPN)

	// Build communities from "then" actions
	route.CommunityBytes = buildFlowSpecCommunities(fr.Then)

	// Build extended communities:
	// 1. First, explicit extended-community (origin, target, etc.)
	// 2. Then, action-based extended communities (redirect, rate-limit, etc.)
	// This order matches ExaBGP output for compatibility.
	if ec := fr.ExtendedCommunity; ec != "" {
		extComm, err := ParseExtendedCommunity(ec)
		if err != nil {
			return route, fmt.Errorf("parse extended-community: %w", err)
		}
		route.ExtCommunityBytes = append(route.ExtCommunityBytes, extComm.Bytes...)
	}

	// Build action-based extended communities from "then" block (redirect, rate-limit, etc.)
	route.ExtCommunityBytes = append(route.ExtCommunityBytes,
		buildFlowSpecExtCommunities(fr.Then, route.NextHop)...)

	// Sort extended communities by type for RFC 4360 compliance.
	// ExaBGP sorts communities by type (origin before redirect, etc.)
	route.ExtCommunityBytes = sortExtCommunities(route.ExtCommunityBytes)

	// Build IPv6 Extended Communities (attribute 25) for IPv6-specific actions
	route.IPv6ExtCommunityBytes = buildFlowSpecIPv6ExtCommunity(fr.Then)

	return route, nil
}

// buildFlowSpecCommunities builds standard community bytes from FlowSpec "then" actions.
// RFC 1997 defines the standard BGP Community attribute format (ASN:NN, 4 bytes each).
func buildFlowSpecCommunities(then map[string]string) []byte {
	var result []byte

	for action, value := range then {
		if action == "community" {
			// Parse community list like [30740:0 30740:30740]
			value = strings.Trim(value, "[]")
			parts := strings.Fields(value)
			for _, part := range parts {
				asn, val, ok := strings.Cut(part, ":")
				if !ok {
					continue
				}
				a, _ := strconv.ParseUint(asn, 10, 16)
				v, _ := strconv.ParseUint(val, 10, 16)
				result = append(result, byte(a>>8), byte(a), byte(v>>8), byte(v))
			}
		}
	}

	return result
}

// buildFlowSpecExtCommunities builds extended community bytes from FlowSpec "then" actions.
// RFC 8955 Section 7 defines Traffic Filtering Action Extended Communities:
//   - Type 0x8006: Traffic-rate (rate-limit, discard) - RFC 8955 Section 7.3
//   - Type 0x8007: Traffic-action (sample, terminal) - RFC 8955 Section 7.4
//   - Type 0x8008: Redirect AS-2byte:value - RFC 8955 Section 7.5
//   - Type 0x8009: Traffic-marking (DSCP) - RFC 8955 Section 7.6
//   - Type 0x0800: Redirect to next-hop - RFC 7674 Section 3
//   - Type 0x010c: Redirect to IPv4 - RFC 7674 Section 3.1
func buildFlowSpecExtCommunities(then map[string]string, nextHop netip.Addr) []byte {
	var result []byte

	for action, value := range then {
		switch action {
		case "discard":
			// Traffic rate 0 = discard - type 0x8006
			result = append(result, 0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)

		case "rate-limit":
			// Traffic rate with value - type 0x8006
			rate := parseFlowRate(value)
			result = append(result, 0x80, 0x06)
			result = append(result, 0x00, 0x00) // 2 bytes padding
			// IEEE 754 float32 for rate
			rateBytes := encodeFloat32(rate)
			result = append(result, rateBytes...)

		case "redirect":
			// Check if redirect IP matches next-hop - use redirect-to-nexthop
			if ip, err := netip.ParseAddr(value); err == nil && ip == nextHop && nextHop.IsValid() {
				// Redirect to next-hop - type 0x0800, subtype 0x00
				result = append(result, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)
			} else {
				// Redirect to AS:NN or IP - type 0x8008
				ec := parseRedirectExtCommunity(value)
				result = append(result, ec...)
			}

		case "redirect-to-nexthop":
			// Redirect to next-hop - type 0x0800, subtype 0x00
			result = append(result, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00)

		case "redirect-to-nexthop-ietf":
			// RFC 7674: Redirect to IP (IETF encoding)
			// IPv4: type 0x01, subtype 0x0c - format: type(1) subtype(1) IPv4(4) reserved(2)
			// IPv6: uses attribute 25 (IPv6 Ext Community) - handled separately
			if ip, err := netip.ParseAddr(value); err == nil && ip.Is4() {
				ipBytes := ip.As4()
				result = append(result, 0x01, 0x0c,
					ipBytes[0], ipBytes[1], ipBytes[2], ipBytes[3],
					0x00, 0x00)
			}
			// IPv6 is handled in buildFlowSpecIPv6ExtCommunity

		case "copy":
			// Copy to IP - type 0x0800, subtype 0x01
			result = append(result, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01)

		case "mark":
			// DSCP marking - type 0x8009
			dscp := parseFlowOctet(value)
			result = append(result, 0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, dscp)

		case "action":
			// Traffic action - type 0x8007
			flags := parseActionFlags(value)
			result = append(result, 0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, flags)
		}
	}

	return result
}

// buildFlowSpecIPv6ExtCommunity builds IPv6 Extended Communities (attribute 25, RFC 5701).
// This is used for actions that require IPv6-specific encoding like redirect-to-nexthop-ietf with IPv6.
// RFC 5701 Section 2 defines the IPv6 Extended Community format (20 bytes per community).
// RFC 7674 Section 3.2 defines the Redirect to IPv6 action (subtype 0x000c).
func buildFlowSpecIPv6ExtCommunity(then map[string]string) []byte {
	var result []byte

	for action, value := range then {
		// Handle redirect-to-nexthop-ietf with IPv6 address
		if action == "redirect-to-nexthop-ietf" {
			// RFC 7674: Redirect to IPv6 address using IPv6 Extended Community (attr 25)
			// Format: subtype(2) + IPv6(16) + copy_flag(2) = 20 bytes
			if ip, err := netip.ParseAddr(value); err == nil && ip.Is6() {
				ipBytes := ip.As16()
				result = append(result, 0x00, 0x0c) // Subtype 0x000c = redirect to IP
				result = append(result, ipBytes[:]...)
				result = append(result, 0x00, 0x00) // Copy flag = 0
			}
		}
	}

	return result
}

// sortExtCommunities sorts extended communities by type for RFC 4360 compliance.
// Each extended community is 8 bytes. Sorting by the 64-bit value puts lower
// type codes first (e.g., origin 0x0003 before redirect 0x8008).
// Trailing bytes that don't form a complete community are discarded.
func sortExtCommunities(data []byte) []byte {
	if len(data) < 16 { // Need at least 2 communities to sort
		return data
	}

	// Validate and truncate to complete communities only
	count := len(data) / 8
	if count*8 != len(data) {
		// Discard trailing bytes that don't form a complete community
		data = data[:count*8]
	}
	communities := make([]uint64, count)
	for i := 0; i < count; i++ {
		offset := i * 8
		communities[i] = uint64(data[offset])<<56 |
			uint64(data[offset+1])<<48 |
			uint64(data[offset+2])<<40 |
			uint64(data[offset+3])<<32 |
			uint64(data[offset+4])<<24 |
			uint64(data[offset+5])<<16 |
			uint64(data[offset+6])<<8 |
			uint64(data[offset+7])
	}

	// Sort by value (lower type codes first)
	slices.Sort(communities)

	// Rebuild byte slice
	result := make([]byte, len(data))
	for i, c := range communities {
		offset := i * 8
		result[offset] = byte(c >> 56)
		result[offset+1] = byte(c >> 48)
		result[offset+2] = byte(c >> 40)
		result[offset+3] = byte(c >> 32)
		result[offset+4] = byte(c >> 24)
		result[offset+5] = byte(c >> 16)
		result[offset+6] = byte(c >> 8)
		result[offset+7] = byte(c)
	}
	return result
}

// buildFlowSpecNLRI builds FlowSpec NLRI bytes from match criteria.
// If forVPN is true, returns component bytes without length prefix (VPN adds its own).
// RFC 8955 Section 4 defines the FlowSpec NLRI encoding.
// RFC 8955 Section 4.2.2 defines component types 1-12.
// RFC 8956 Section 3.7 defines component type 13 (Flow Label, IPv6 only).
func buildFlowSpecNLRI(match map[string]string, isIPv6 bool, forVPN bool) []byte {
	family := nlri.IPv4FlowSpec
	if isIPv6 {
		family = nlri.IPv6FlowSpec
	}

	fs := nlri.NewFlowSpec(family)

	// Add destination prefix
	if dst, ok := match["destination"]; ok {
		prefix, offset := parseFlowPrefixWithOffset(dst)
		if prefix.IsValid() {
			if prefix.Addr().Is6() && offset > 0 {
				fs.AddComponent(nlri.NewFlowDestPrefixComponentWithOffset(prefix, offset))
			} else {
				fs.AddComponent(nlri.NewFlowDestPrefixComponent(prefix))
			}
		}
	}

	// Add source prefix
	if src, ok := match["source"]; ok {
		prefix, offset := parseFlowPrefixWithOffset(src)
		if prefix.IsValid() {
			if prefix.Addr().Is6() && offset > 0 {
				fs.AddComponent(nlri.NewFlowSourcePrefixComponentWithOffset(prefix, offset))
			} else {
				fs.AddComponent(nlri.NewFlowSourcePrefixComponent(prefix))
			}
		}
	}

	// Add protocol
	if proto, ok := match["protocol"]; ok {
		matches := parseFlowProtocolMatches(proto)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowIPProtocol, matches))
		}
	}

	// Add next-header (IPv6 equivalent of protocol)
	if nh, ok := match["next-header"]; ok {
		matches := parseFlowProtocolMatches(nh)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowIPProtocol, matches))
		}
	}

	// Add port (matches either source or destination)
	if port, ok := match["port"]; ok {
		matches := parseFlowMatches(port)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowPort, matches))
		}
	}

	// Add destination port
	if dp, ok := match["destination-port"]; ok {
		matches := parseFlowMatches(dp)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowDestPort, matches))
		}
	}

	// Add source port
	if sp, ok := match["source-port"]; ok {
		matches := parseFlowMatches(sp)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowSourcePort, matches))
		}
	}

	// Add packet length
	if pl, ok := match["packet-length"]; ok {
		matches := parseFlowMatches(pl)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowPacketLength, matches))
		}
	}

	// Add DSCP
	if dscp, ok := match["dscp"]; ok {
		vals := parseFlowOctets(dscp)
		if len(vals) > 0 {
			fs.AddComponent(nlri.NewFlowDSCPComponent(vals...))
		}
	}

	// Add traffic-class (IPv6)
	if tc, ok := match["traffic-class"]; ok {
		vals := parseFlowOctets(tc)
		if len(vals) > 0 {
			fs.AddComponent(nlri.NewFlowDSCPComponent(vals...))
		}
	}

	// Add flow-label (IPv6)
	if fl, ok := match["flow-label"]; ok {
		values := parseFlowLabels(fl)
		if len(values) > 0 {
			fs.AddComponent(nlri.NewFlowFlowLabelComponent(values...))
		}
	}

	// Add fragment
	if frag, ok := match["fragment"]; ok {
		flags := parseFlowFragment(frag)
		if len(flags) > 0 {
			fs.AddComponent(nlri.NewFlowFragmentComponent(flags...))
		}
	}

	// Add TCP flags
	if tcpf, ok := match["tcp-flags"]; ok {
		matches := parseFlowTCPFlagMatches(tcpf)
		if len(matches) > 0 {
			fs.AddComponent(nlri.NewFlowNumericComponent(nlri.FlowTCPFlags, matches))
		}
	}

	// Add ICMP type
	if it, ok := match["icmp-type"]; ok {
		types := parseFlowICMPTypes(it)
		if len(types) > 0 {
			fs.AddComponent(nlri.NewFlowICMPTypeComponent(types...))
		}
	}

	// Add ICMP code
	if ic, ok := match["icmp-code"]; ok {
		codes := parseFlowICMPCodes(ic)
		if len(codes) > 0 {
			fs.AddComponent(nlri.NewFlowICMPCodeComponent(codes...))
		}
	}

	// For VPN, return component bytes without length prefix
	if forVPN {
		return fs.ComponentBytes()
	}
	return fs.Bytes()
}

// parseFlowPrefixWithOffset parses a FlowSpec prefix like "10.0.0.1/32" or "::1/128/120".
// Returns the prefix and offset (0 if no offset).
func parseFlowPrefixWithOffset(s string) (netip.Prefix, uint8) {
	// Handle IPv6 offset format: addr/len/offset
	parts := strings.Split(s, "/")
	if len(parts) >= 2 {
		addrStr := parts[0]
		lenStr := parts[1]
		var offset uint8
		if len(parts) >= 3 {
			if off, err := strconv.Atoi(parts[2]); err == nil && off >= 0 && off <= 255 {
				offset = uint8(off) // #nosec G115 -- bounds checked
			}
		}

		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return netip.Prefix{}, 0
		}
		prefixLen, err := strconv.Atoi(lenStr)
		if err != nil {
			return netip.Prefix{}, 0
		}
		return netip.PrefixFrom(addr, prefixLen), offset
	}

	// Try parsing as simple prefix
	prefix, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, 0
	}
	return prefix, 0
}

// parseFlowProtocols parses protocol values like "tcp", "=udp", "[ tcp udp ]".
//
//nolint:unused // Prepared for FlowSpec inline syntax parsing (not yet implemented)
func parseFlowProtocols(s string) []uint8 {
	matches := parseFlowProtocolMatches(s)
	result := make([]uint8, len(matches))
	for i, m := range matches {
		result[i] = uint8(m.Value) // #nosec G115 -- protocol is uint8
	}
	return result
}

// parseFlowProtocolMatches parses protocol values with operators.
func parseFlowProtocolMatches(s string) []nlri.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []nlri.FlowMatch

	protoMap := map[string]uint8{
		"icmp": 1, "igmp": 2, "tcp": 6, "udp": 17, "gre": 47, "esp": 50, "ah": 51,
	}

	for _, p := range parts {
		var op nlri.FlowOperator

		// Parse operator prefix
		switch {
		case strings.HasPrefix(p, "!="):
			op = nlri.FlowOpNotEq
			p = strings.TrimPrefix(p, "!=")
		case strings.HasPrefix(p, "="):
			op = nlri.FlowOpEqual
			p = strings.TrimPrefix(p, "=")
		default:
			op = nlri.FlowOpEqual
		}

		p = strings.ToLower(p)
		if v, ok := protoMap[p]; ok {
			result = append(result, nlri.FlowMatch{Op: op, Value: uint64(v)})
		} else if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, nlri.FlowMatch{Op: op, Value: n})
		}
	}
	return result
}

// parseFlowPorts parses port values like "=80", ">1024", "[ =80 =8080 ]", ">8080&<8088".
//
//nolint:unused // Prepared for FlowSpec inline syntax parsing (not yet implemented)
func parseFlowPorts(s string) []uint16 {
	matches := parseFlowMatches(s)
	result := make([]uint16, len(matches))
	for i, m := range matches {
		result[i] = uint16(m.Value) //nolint:gosec // Value range validated by caller
	}
	return result
}

// parseFlowMatches parses FlowSpec match expressions with operators.
// Formats: "=80", ">1024", "[ =80 =8080 ]", ">8080&<8088", "!=443".
func parseFlowMatches(s string) []nlri.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []nlri.FlowMatch

	for _, p := range parts {
		// Handle range operators like ">8080&<8088" by splitting on &
		rangeParts := strings.Split(p, "&")
		for i, rp := range rangeParts {
			var op nlri.FlowOperator
			isAnd := i > 0 // Parts after & are AND-ed with previous

			// Parse operator prefix
			switch {
			case strings.HasPrefix(rp, "!="):
				op = nlri.FlowOpNotEq
				rp = strings.TrimPrefix(rp, "!=")
			case strings.HasPrefix(rp, ">="):
				op = nlri.FlowOpGreater | nlri.FlowOpEqual
				rp = strings.TrimPrefix(rp, ">=")
			case strings.HasPrefix(rp, "<="):
				op = nlri.FlowOpLess | nlri.FlowOpEqual
				rp = strings.TrimPrefix(rp, "<=")
			case strings.HasPrefix(rp, ">"):
				op = nlri.FlowOpGreater
				rp = strings.TrimPrefix(rp, ">")
			case strings.HasPrefix(rp, "<"):
				op = nlri.FlowOpLess
				rp = strings.TrimPrefix(rp, "<")
			case strings.HasPrefix(rp, "="):
				op = nlri.FlowOpEqual
				rp = strings.TrimPrefix(rp, "=")
			default:
				op = nlri.FlowOpEqual // Default to equality
			}

			if n, err := strconv.ParseUint(rp, 10, 32); err == nil {
				result = append(result, nlri.FlowMatch{
					Op:    op,
					And:   isAnd,
					Value: n,
				})
			}
		}
	}
	return result
}

// parseFlowOctets parses octet values (DSCP, traffic-class).
func parseFlowOctets(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
		}
	}
	return result
}

// icmpTypeNames maps ICMP type symbolic names to values.
// Per IANA ICMP Type Numbers: https://www.iana.org/assignments/icmp-parameters
// ExaBGP compatible naming (lowercase, hyphens).
var icmpTypeNames = map[string]uint8{
	"echo-reply":            0,
	"unreachable":           3,
	"redirect":              5,
	"echo-request":          8,
	"router-advertisement":  9,
	"router-solicit":        10,
	"time-exceeded":         11,
	"parameter-problem":     12,
	"timestamp":             13,
	"timestamp-reply":       14,
	"photuris":              40,
	"experimental-mobility": 41,
	"extended-echo-request": 42,
	"extended-echo-reply":   43,
	"experimental-one":      253,
	"experimental-two":      254,
}

// parseFlowICMPTypes parses ICMP type values or names.
// Handles: [ unreachable echo-request echo-reply ] or [ 3 8 0 ] or [ =3 =8 =0 ].
// Unknown names are logged as warnings and skipped.
func parseFlowICMPTypes(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		// Try numeric first
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
			continue
		}
		// Try symbolic name
		if n, ok := icmpTypeNames[strings.ToLower(p)]; ok {
			result = append(result, n)
			continue
		}
		// Unknown name - log warning
		trace.Log(trace.Config, "unknown ICMP type name: %s", p)
	}
	return result
}

// icmpCodeNames maps ICMP code symbolic names to values.
// Per IANA ICMP Type Numbers: https://www.iana.org/assignments/icmp-parameters
// ExaBGP compatible naming (lowercase, hyphens).
var icmpCodeNames = map[string]uint8{
	// Destination Unreachable (type 3)
	"network-unreachable":                   0,
	"host-unreachable":                      1,
	"protocol-unreachable":                  2,
	"port-unreachable":                      3,
	"fragmentation-needed":                  4,
	"source-route-failed":                   5,
	"destination-network-unknown":           6,
	"destination-host-unknown":              7,
	"source-host-isolated":                  8,
	"destination-network-prohibited":        9,
	"destination-host-prohibited":           10,
	"network-unreachable-for-tos":           11,
	"host-unreachable-for-tos":              12,
	"communication-prohibited-by-filtering": 13,
	"host-precedence-violation":             14,
	"precedence-cutoff-in-effect":           15,
	// Redirect (type 5)
	"redirect-for-network":      0,
	"redirect-for-host":         1,
	"redirect-for-tos-and-net":  2,
	"redirect-for-tos-and-host": 3,
	// Time Exceeded (type 11)
	"ttl-eq-zero-during-transit":    0,
	"ttl-eq-zero-during-reassembly": 1,
	// Parameter Problem (type 12)
	"required-option-missing": 1,
	"ip-header-bad":           2,
}

// parseFlowICMPCodes parses ICMP code values or names.
// Handles: [ host-unreachable network-unreachable ] or [ 1 0 ] or [ =1 =0 ].
// Unknown names are logged as warnings and skipped.
func parseFlowICMPCodes(s string) []uint8 {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []uint8

	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		// Try numeric first
		if n, err := strconv.ParseUint(p, 10, 8); err == nil {
			result = append(result, uint8(n))
			continue
		}
		// Try symbolic name
		if n, ok := icmpCodeNames[strings.ToLower(p)]; ok {
			result = append(result, n)
			continue
		}
		// Unknown name - log warning
		trace.Log(trace.Config, "unknown ICMP code name: %s", p)
	}
	return result
}

// parseFlowFragment parses fragment flags like "[ first-fragment last-fragment ]".
func parseFlowFragment(s string) []nlri.FlowFragmentFlag {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []nlri.FlowFragmentFlag

	flagMap := map[string]nlri.FlowFragmentFlag{
		"dont-fragment":  nlri.FlowFragDontFragment,
		"is-fragment":    nlri.FlowFragIsFragment,
		"first-fragment": nlri.FlowFragFirstFragment,
		"last-fragment":  nlri.FlowFragLastFragment,
	}

	for _, p := range parts {
		if f, ok := flagMap[p]; ok {
			result = append(result, f)
		}
	}
	return result
}

// parseFlowTCPFlags parses TCP flags like "[SYN RST&FIN&!=push]".
// Returns simple flag values (for backwards compatibility).
//
//nolint:unused // Prepared for FlowSpec inline syntax parsing (not yet implemented)
func parseFlowTCPFlags(s string) []uint8 {
	matches := parseFlowTCPFlagMatches(s)
	result := make([]uint8, len(matches))
	for i, m := range matches {
		result[i] = uint8(m.Value) // #nosec G115 -- TCP flags fit in uint8
	}
	return result
}

// parseFlowTCPFlagMatches parses TCP flags with AND and NOT operators.
// TCP flags use bitmask matching:
//   - 0x01 = MATCH (exact match)
//   - 0x02 = NOT (negate)
//   - 0x40 = AND (AND with previous)
func parseFlowTCPFlagMatches(s string) []nlri.FlowMatch {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	var result []nlri.FlowMatch

	flagMap := map[string]uint8{
		"fin": 0x01, "syn": 0x02, "rst": 0x04, "reset": 0x04,
		"psh": 0x08, "push": 0x08,
		"ack": 0x10, "urg": 0x20, "urgent": 0x20,
		"ece": 0x40, "cwr": 0x80,
	}

	for _, p := range parts {
		// Handle combined flags like "RST&FIN&!=push"
		flagParts := strings.Split(p, "&")
		for i, fp := range flagParts {
			var op nlri.FlowOperator
			isAnd := i > 0 // Parts after & are AND-ed

			// Check for != (NOT+MATCH)
			if strings.HasPrefix(fp, "!=") {
				op = 0x02 | 0x01 // NOT | MATCH
				fp = strings.TrimPrefix(fp, "!=")
			}
			// For simple flags, use no operator (INCLUDE)

			if isAnd {
				op |= 0x40 // AND
			}

			fp = strings.ToLower(fp)
			if f, ok := flagMap[fp]; ok {
				result = append(result, nlri.FlowMatch{Op: op, And: isAnd, Value: uint64(f)})
			}
		}
	}
	return result
}

// parseFlowLabels parses flow-label values like "2013" or "=2013".
func parseFlowLabels(s string) []uint32 {
	var result []uint32
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	for _, p := range parts {
		p = strings.TrimPrefix(p, "=")
		val, err := strconv.ParseUint(p, 10, 32)
		if err == nil {
			result = append(result, uint32(val))
		}
	}
	return result
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

	// Build MUP NLRI
	nlriBytes, err := buildMUPNLRI(mr)
	if err != nil {
		return route, fmt.Errorf("build MUP NLRI: %w", err)
	}
	route.NLRI = nlriBytes

	// Parse SRv6 Prefix-SID if present
	if mr.PrefixSID != "" {
		sid, err := ParsePrefixSIDSRv6(mr.PrefixSID)
		if err != nil {
			return route, fmt.Errorf("parse prefix-sid-srv6: %w", err)
		}
		route.PrefixSID = sid.Bytes
	}

	return route, nil
}

// buildMUPNLRI builds MUP NLRI bytes from route config.
// Returns an error if any address/prefix parsing fails.
func buildMUPNLRI(mr MUPRouteConfig) ([]byte, error) {
	// Determine route type code
	var routeType nlri.MUPRouteType
	switch mr.RouteType {
	case "mup-isd":
		routeType = nlri.MUPISD
	case "mup-dsd":
		routeType = nlri.MUPDSD
	case "mup-t1st":
		routeType = nlri.MUPT1ST
	case "mup-t2st":
		routeType = nlri.MUPT2ST
	default:
		return nil, fmt.Errorf("unknown MUP route type: %s", mr.RouteType)
	}

	// Parse RD
	var rd nlri.RouteDistinguisher
	if mr.RD != "" {
		parsed, err := ParseRouteDistinguisher(mr.RD)
		if err != nil {
			return nil, fmt.Errorf("invalid RD %q: %w", mr.RD, err)
		}
		// Convert config.RouteDistinguisher to nlri.RouteDistinguisher
		// Bytes[0:2] is the type, Bytes[2:8] is the value
		rdType := uint16(parsed.Bytes[0])<<8 | uint16(parsed.Bytes[1])
		rd.Type = nlri.RDType(rdType)
		copy(rd.Value[:], parsed.Bytes[2:8])
	}

	// Build route-type-specific data
	var data []byte
	switch routeType {
	case nlri.MUPISD:
		// ISD: prefix-len (1 byte) + prefix (variable)
		if mr.Prefix == "" {
			return nil, fmt.Errorf("MUP ISD requires prefix")
		}
		prefix, err := netip.ParsePrefix(mr.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid ISD prefix %q: %w", mr.Prefix, err)
		}
		data = buildMUPPrefix(prefix)

	case nlri.MUPDSD:
		// DSD: address (4 or 16 bytes)
		if mr.Address == "" {
			return nil, fmt.Errorf("MUP DSD requires address")
		}
		addr, err := netip.ParseAddr(mr.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid DSD address %q: %w", mr.Address, err)
		}
		data = addr.AsSlice()

	case nlri.MUPT1ST:
		// T1ST: prefix + TEID (4) + QFI (1) + endpoint-len + endpoint + [source-len + source]
		if mr.Prefix == "" {
			return nil, fmt.Errorf("MUP T1ST requires prefix")
		}
		prefix, err := netip.ParsePrefix(mr.Prefix)
		if err != nil {
			return nil, fmt.Errorf("invalid T1ST prefix %q: %w", mr.Prefix, err)
		}
		data = buildMUPPrefix(prefix)
		// Add TEID (4 bytes)
		teid := parseTEID(mr.TEID)
		data = append(data, byte(teid>>24), byte(teid>>16), byte(teid>>8), byte(teid))
		// Add QFI (1 byte)
		data = append(data, mr.QFI)
		// Add endpoint
		if mr.Endpoint != "" {
			ep, err := netip.ParseAddr(mr.Endpoint)
			if err != nil {
				return nil, fmt.Errorf("invalid T1ST endpoint %q: %w", mr.Endpoint, err)
			}
			epBytes := ep.AsSlice()
			data = append(data, byte(len(epBytes)*8)) // endpoint length in bits
			data = append(data, epBytes...)
		}
		// Add source (optional, for T1ST)
		if mr.Source != "" {
			src, err := netip.ParseAddr(mr.Source)
			if err != nil {
				return nil, fmt.Errorf("invalid T1ST source %q: %w", mr.Source, err)
			}
			srcBytes := src.AsSlice()
			data = append(data, byte(len(srcBytes)*8)) // source length in bits
			data = append(data, srcBytes...)
		}

	case nlri.MUPT2ST:
		// T2ST: endpoint-len + endpoint + TEID (variable based on teid/bits)
		if mr.Address == "" {
			return nil, fmt.Errorf("MUP T2ST requires address")
		}
		ep, err := netip.ParseAddr(mr.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid T2ST endpoint %q: %w", mr.Address, err)
		}
		epBytes := ep.AsSlice()
		data = append(data, byte(len(epBytes)*8)) // endpoint length in bits
		data = append(data, epBytes...)
		// Add TEID with bit-encoded length
		teid, bits := parseTEIDWithBits(mr.TEID)
		teidBytes := encodeTEIDWithBits(teid, bits)
		data = append(data, teidBytes...)
	}

	// Determine AFI
	afi := nlri.AFIIPv4
	if mr.IsIPv6 {
		afi = nlri.AFIIPv6
	}

	mup := nlri.NewMUPFull(afi, nlri.MUPArch3GPP5G, routeType, rd, data)
	return mup.Bytes(), nil
}

// buildMUPPrefix encodes a prefix for MUP NLRI.
func buildMUPPrefix(prefix netip.Prefix) []byte {
	bits := prefix.Bits()
	addr := prefix.Addr()
	addrBytes := addr.AsSlice()
	prefixBytes := (bits + 7) / 8
	result := make([]byte, 1+prefixBytes)
	result[0] = byte(bits)
	copy(result[1:], addrBytes[:prefixBytes])
	return result
}

// parseTEID parses TEID from string, handling "12345" format.
func parseTEID(s string) uint32 {
	// Handle "12345/32" format - just get the value part
	if idx := strings.Index(s, "/"); idx > 0 {
		s = s[:idx]
	}
	if n, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint32(n)
	}
	return 0
}

// parseTEIDWithBits parses TEID with bit length from "12345/32" format.
func parseTEIDWithBits(s string) (uint32, int) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return parseTEID(s), 32
	}
	teid := parseTEID(parts[0])
	bits, err := strconv.Atoi(parts[1])
	if err != nil {
		bits = 32
	}
	return teid, bits
}

// encodeTEIDWithBits encodes TEID with the specified bit length.
func encodeTEIDWithBits(teid uint32, bits int) []byte {
	if bits <= 0 {
		return nil
	}
	byteLen := (bits + 7) / 8
	result := make([]byte, byteLen)
	for i := 0; i < byteLen; i++ {
		shift := (byteLen - 1 - i) * 8
		result[i] = byte(teid >> shift)
	}
	return result
}

// parseOrigin converts origin string to code.
// Empty or unset defaults to IGP (0).
func parseOrigin(s string) uint8 {
	switch strings.ToLower(s) {
	case "", originIGP:
		return 0 // IGP is default
	case originEGP:
		return 1
	default:
		return 2 // incomplete
	}
}

// parseASPathSimple parses an AS path string like "[ 30740 30740 ]" to []uint32.
func parseASPathSimple(s string) ([]uint32, error) {
	s = strings.Trim(s, "[]")
	parts := strings.Fields(s)
	result := make([]uint32, 0, len(parts))
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

// parseFlowRate parses a rate value for FlowSpec rate-limit action.
func parseFlowRate(s string) float32 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0
	}
	return float32(n)
}

// encodeFloat32 encodes a float32 as 4 bytes (IEEE 754).
func encodeFloat32(f float32) []byte {
	bits := math.Float32bits(f)
	return []byte{
		byte(bits >> 24),
		byte(bits >> 16),
		byte(bits >> 8),
		byte(bits),
	}
}

// parseRedirectExtCommunity parses redirect action value to extended community bytes.
// Formats: AS:NN (type 0x8008) or IP (redirect-to-nexthop).
func parseRedirectExtCommunity(s string) []byte {
	if s == "" {
		return nil
	}

	// Check if it's an IP address (redirect to IP)
	if ip, err := netip.ParseAddr(s); err == nil {
		if ip.Is4() {
			// Redirect to IPv4: type 0x8008, subtype 0x08
			b := ip.As4()
			return []byte{0x80, 0x08, b[0], b[1], b[2], b[3], 0x00, 0x00}
		}
		// IPv6 not commonly used for redirect
		return nil
	}

	// Parse AS:NN format
	parts := strings.Split(s, ":")
	if len(parts) == 2 {
		asn, err1 := strconv.ParseUint(parts[0], 10, 32)
		nn, err2 := strconv.ParseUint(parts[1], 10, 32)
		if err1 == nil && err2 == nil {
			if asn <= 0xFFFF {
				// 2-byte AS format: type 0x80, subtype 0x08
				return []byte{
					0x80, 0x08,
					byte(asn >> 8), byte(asn),
					byte(nn >> 24), byte(nn >> 16), byte(nn >> 8), byte(nn),
				}
			}
			// 4-byte AS format: type 0x82, subtype 0x08
			return []byte{
				0x82, 0x08,
				byte(asn >> 24), byte(asn >> 16), byte(asn >> 8), byte(asn),
				byte(nn >> 8), byte(nn),
			}
		}
	}

	return nil
}

// parseFlowOctet parses a single octet value.
func parseFlowOctet(s string) byte {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 8)
	if err != nil {
		return 0
	}
	return byte(n)
}

// parseActionFlags parses traffic action flags.
func parseActionFlags(s string) byte {
	var flags byte
	s = strings.ToLower(s)
	if strings.Contains(s, "terminal") {
		flags |= 0x01
	}
	if strings.Contains(s, "sample") {
		flags |= 0x02
	}
	return flags
}

// detectLegacySyntaxHint checks if a parse error is likely due to old syntax
// and returns a helpful hint for migration.
func detectLegacySyntaxHint(input string, parseErr error) string {
	errMsg := parseErr.Error()

	// Check for common old syntax patterns
	hasNeighborKeyword := strings.Contains(errMsg, "unknown top-level keyword: neighbor")
	hasTemplateNeighbor := strings.Contains(errMsg, "unknown field in template: neighbor")
	hasPeerGlobError := strings.Contains(errMsg, "invalid key for peer") && strings.Contains(errMsg, "invalid IP")

	// Also check input for old syntax patterns
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "neighbor ") {
			hasNeighborKeyword = true
			break
		}
	}

	if hasNeighborKeyword || hasTemplateNeighbor || hasPeerGlobError {
		return "Hint: This config appears to use deprecated ExaBGP syntax.\n" +
			"Run 'zebgp config check <file>' to verify, then\n" +
			"Run 'zebgp config migrate <file>' to upgrade."
	}

	return ""
}

// parseSplitLen parses a split specification like "/25" and returns the prefix length.
// Returns 0 if no split or invalid format.
func parseSplitLen(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.TrimPrefix(s, "/")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 128 {
		return 0
	}
	return n
}

// splitPrefix splits a prefix into more-specific prefixes with the given length.
// For example, 10.0.0.0/24 split to /25 produces two /25 prefixes.
func splitPrefix(prefix netip.Prefix, targetLen int) []netip.Prefix {
	sourceBits := prefix.Bits()

	// Validate target length
	maxBits := 32
	if prefix.Addr().Is6() {
		maxBits = 128
	}

	if targetLen <= sourceBits || targetLen > maxBits {
		return []netip.Prefix{prefix}
	}

	// Calculate number of resulting prefixes: 2^(targetLen - sourceBits)
	numPrefixes := 1 << (targetLen - sourceBits)
	result := make([]netip.Prefix, 0, numPrefixes)

	baseAddr := prefix.Addr()
	for i := 0; i < numPrefixes; i++ {
		newAddr := addToAddr(baseAddr, i, targetLen)
		result = append(result, netip.PrefixFrom(newAddr, targetLen))
	}

	return result
}

// addToAddr adds an offset to an address at the given prefix boundary.
func addToAddr(addr netip.Addr, offset int, prefixLen int) netip.Addr {
	if offset == 0 {
		return addr
	}

	maxBits := 32
	if addr.Is6() {
		maxBits = 128
	}
	shift := maxBits - prefixLen

	if addr.Is4() {
		v4 := addr.As4()
		val := uint32(v4[0])<<24 | uint32(v4[1])<<16 | uint32(v4[2])<<8 | uint32(v4[3])
		val += uint32(offset) << shift //nolint:gosec // offset is bounded
		return netip.AddrFrom4([4]byte{byte(val >> 24), byte(val >> 16), byte(val >> 8), byte(val)})
	}

	// IPv6
	v6 := addr.As16()
	hi := uint64(v6[0])<<56 | uint64(v6[1])<<48 | uint64(v6[2])<<40 | uint64(v6[3])<<32 |
		uint64(v6[4])<<24 | uint64(v6[5])<<16 | uint64(v6[6])<<8 | uint64(v6[7])
	lo := uint64(v6[8])<<56 | uint64(v6[9])<<48 | uint64(v6[10])<<40 | uint64(v6[11])<<32 |
		uint64(v6[12])<<24 | uint64(v6[13])<<16 | uint64(v6[14])<<8 | uint64(v6[15])

	if shift >= 64 {
		hi += uint64(offset) << (shift - 64) //nolint:gosec // offset is bounded
	} else {
		addLo := uint64(offset) << shift //nolint:gosec // offset is bounded
		newLo := lo + addLo
		if newLo < lo {
			hi++
		}
		lo = newLo
	}

	return netip.AddrFrom16([16]byte{
		byte(hi >> 56), byte(hi >> 48), byte(hi >> 40), byte(hi >> 32),
		byte(hi >> 24), byte(hi >> 16), byte(hi >> 8), byte(hi),
		byte(lo >> 56), byte(lo >> 48), byte(lo >> 40), byte(lo >> 32),
		byte(lo >> 24), byte(lo >> 16), byte(lo >> 8), byte(lo),
	})
}
