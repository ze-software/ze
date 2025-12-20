package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	configTrue   = "true"   // Config value for boolean true
	configEnable = "enable" // Config value for enabled state
)

// flowRouteAttributes returns field definitions for FlowSpec routes.
// Format: route NAME { rd VALUE; match { ... } then { ... } }
func flowRouteAttributes() []FieldDef {
	return []FieldDef{
		Field("rd", Leaf(TypeString)),
		Field("next-hop", Leaf(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("match", Freeform()), // Match criteria
		Field("then", Freeform()),  // Actions
	}
}

// mcastVpnAttributes returns field definitions for MCAST-VPN routes.
// Format: mcast-vpn <type> rp <ip> group <ip> rd <rd> source-as <asn> next-hop <ip> ...
func mcastVpnAttributes() []FieldDef {
	return []FieldDef{
		Field("rp", Leaf(TypeString)),
		Field("group", Leaf(TypeString)),
		Field("source", Leaf(TypeString)),
		Field("rd", Leaf(TypeString)),
		Field("source-as", Leaf(TypeUint32)),
		Field("next-hop", Leaf(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("community", ValueOrArray(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
	}
}

// vplsAttributes returns field definitions for VPLS routes.
func vplsAttributes() []FieldDef {
	return []FieldDef{
		Field("rd", Leaf(TypeString)),
		Field("endpoint", Leaf(TypeUint16)),
		Field("base", Leaf(TypeUint32)),
		Field("offset", Leaf(TypeUint16)),
		Field("size", Leaf(TypeUint16)),
		Field("next-hop", Leaf(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
		Field("as-path", ValueOrArray(TypeString)),
		Field("community", ValueOrArray(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("originator-id", Leaf(TypeIPv4)),
		Field("cluster-list", ValueOrArray(TypeString)),
	}
}

// routeAttributes returns the common route attribute field definitions.
// Used by both static and announce route schemas.
func routeAttributes() []FieldDef {
	return []FieldDef{
		Field("next-hop", Leaf(TypeString)),
		Field("origin", Leaf(TypeString)),
		Field("local-preference", Leaf(TypeUint32)),
		Field("med", Leaf(TypeUint32)),
		Field("community", ValueOrArray(TypeString)),
		Field("extended-community", ValueOrArray(TypeString)),
		Field("large-community", ValueOrArray(TypeString)),
		Field("as-path", ValueOrArray(TypeString)),
		Field("path-information", Leaf(TypeString)),
		Field("label", Leaf(TypeString)),
		Field("rd", Leaf(TypeString)),
		Field("aggregator", Leaf(TypeString)),
		Field("atomic-aggregate", Flex()), // Can be standalone or followed by aggregator
		Field("originator-id", Leaf(TypeIPv4)),
		Field("cluster-list", Leaf(TypeString)),
		Field("name", Leaf(TypeString)),
		Field("split", Leaf(TypeString)),
		Field("watchdog", Leaf(TypeString)),
		Field("withdraw", Flex()),                    // Withdraw route (flag)
		Field("attribute", ValueOrArray(TypeString)), // Generic attributes
		Field("bgp-prefix-sid", Flex()),              // Prefix SID - can use ( ... ) syntax
		Field("bgp-prefix-sid-srv6", Flex()),         // SRv6 Prefix SID - can use ( ... ) syntax
	}
}

// neighborFields returns the common field definitions for neighbor and template schemas.
func neighborFields() []FieldDef {
	return []FieldDef{
		Field("description", Leaf(TypeString)),
		Field("router-id", Leaf(TypeIPv4)),
		Field("local-address", Leaf(TypeIP)),
		Field("local-as", Leaf(TypeUint32)),
		Field("peer-as", Leaf(TypeUint32)),
		Field("hold-time", LeafWithDefault(TypeUint16, "90")),
		Field("passive", LeafWithDefault(TypeBool, "false")),
		Field("group-updates", LeafWithDefault(TypeBool, configTrue)),
		Field("host-name", Leaf(TypeString)),
		Field("domain-name", Leaf(TypeString)),
		Field("md5-password", Leaf(TypeString)),
		Field("md5-ip", Leaf(TypeIP)),
		Field("ttl-security", Leaf(TypeUint16)),
		Field("outgoing-ttl", Leaf(TypeUint16)),
		Field("incoming-ttl", Leaf(TypeUint16)),
		Field("multi-session", LeafWithDefault(TypeBool, "false")),
		Field("inherit", Leaf(TypeString)),  // template name
		Field("nexthop", Freeform()),        // nexthop configuration
		Field("manual-eor", Leaf(TypeBool)), // manual end-of-RIB
		Field("auto-flush", Leaf(TypeBool)), // auto-flush routes
		Field("adj-rib-out", Leaf(TypeBool)),
		Field("adj-rib-in", Leaf(TypeBool)),

		// Address families: "ipv4 unicast", "ipv6 unicast", etc.
		Field("family", Freeform()), // { ipv4 unicast; ipv6 unicast; }

		// Capabilities
		Field("capability", Container(
			Field("asn4", LeafWithDefault(TypeBool, configTrue)),
			Field("route-refresh", Flex()), // flag, value, or block
			Field("graceful-restart", Flex( // flag, value, or block
				Field("restart-time", LeafWithDefault(TypeUint16, "120")),
			)),
			Field("add-path", Flex( // flag, value (send/receive), or block
				Field("send", LeafWithDefault(TypeBool, "false")),
				Field("receive", LeafWithDefault(TypeBool, "false")),
			)),
			Field("nexthop", Flex()),
			Field("multi-session", Flex()),
			Field("operational", Flex()),
			Field("aigp", Flex()),
			Field("software-version", Flex()),
		)),

		// Announce routes - structured by AFI/SAFI
		// announce { ipv4 { unicast <prefix> <attrs>; } ipv6 { unicast <prefix> <attrs>; } }
		Field("announce", Container(
			Field("ipv4", Container(
				Field("unicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("multicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("mpls-vpn", InlineList(TypePrefix, routeAttributes()...)),
				Field("mcast-vpn", InlineList(TypeString, mcastVpnAttributes()...)),
				Field("mup", Flex()),      // MUP - complex inline format with nested parens
				Field("flow", Freeform()), // FlowSpec - complex format
			)),
			Field("ipv6", Container(
				Field("unicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("multicast", InlineList(TypePrefix, routeAttributes()...)),
				Field("mpls-vpn", InlineList(TypePrefix, routeAttributes()...)),
				Field("mcast-vpn", InlineList(TypeString, mcastVpnAttributes()...)),
				Field("mup", Flex()),      // MUP - complex inline format with nested parens
				Field("flow", Freeform()), // FlowSpec - complex format
			)),
			Field("l2vpn", Container(
				Field("vpls", Flex(vplsAttributes()...)),
				Field("evpn", Freeform()), // EVPN - complex format
			)),
		)),

		// Static routes - InlineList supports "route PREFIX attr val attr val;"
		Field("static", Container(
			Field("route", InlineList(TypePrefix, routeAttributes()...)),
		)),

		// Flow routes - named list with match/then sub-blocks
		Field("flow", Container(
			Field("route", List(TypeString, flowRouteAttributes()...)),
		)),

		// L2VPN (VPLS, EVPN)
		Field("l2vpn", Container(
			Field("vpls", Flex(vplsAttributes()...)),
			Field("evpn", Freeform()), // EVPN - complex format
		)),

		// Add-path per-family configuration
		Field("add-path", Freeform()),

		// API configuration - can be named or anonymous
		Field("api", Freeform()),
	}
}

// BGPSchema returns the schema for ZeBGP configuration.
func BGPSchema() *Schema {
	schema := NewSchema()

	// Global settings
	schema.Define("router-id", Leaf(TypeIPv4))
	schema.Define("local-as", Leaf(TypeUint32))
	schema.Define("listen", MultiLeaf(TypeString)) // "address port"

	// Process definitions (API)
	schema.Define("process", List(TypeString,
		Field("run", MultiLeaf(TypeString)), // command with args
		Field("encoder", Leaf(TypeString)),  // json, text
		Field("respawn", Leaf(TypeBool)),    // respawn on exit
	))

	// Neighbor definitions - uses shared neighborFields
	schema.Define("neighbor", List(TypeIP, neighborFields()...))

	// Template definitions - contains named neighbor templates
	// template { neighbor <name> { ... } }
	schema.Define("template", Container(
		Field("neighbor", List(TypeString, neighborFields()...)),
	))

	return schema
}

// BGPConfig is the typed configuration structure.
type BGPConfig struct {
	RouterID  uint32
	LocalAS   uint32
	Listen    string
	Neighbors []NeighborConfig
	Processes []ProcessConfig
}

// NeighborConfig holds neighbor configuration.
type NeighborConfig struct {
	Address        netip.Addr
	Description    string
	RouterID       uint32
	LocalAddress   netip.Addr
	LocalAS        uint32
	PeerAS         uint32
	HoldTime       uint16
	Passive        bool
	GroupUpdates   bool // Group compatible routes in single UPDATE
	Families       []string
	Hostname       string
	DomainName     string
	Capabilities   CapabilityConfig
	StaticRoutes   []StaticRouteConfig
	MVPNRoutes     []MVPNRouteConfig
	VPLSRoutes     []VPLSRouteConfig
	FlowSpecRoutes []FlowSpecRouteConfig
	MUPRoutes      []MUPRouteConfig
}

// CapabilityConfig holds capability settings.
type CapabilityConfig struct {
	ASN4            bool
	RouteRefresh    bool
	GracefulRestart bool
	RestartTime     uint16
	AddPathSend     bool
	AddPathReceive  bool
	SoftwareVersion bool
}

// StaticRouteConfig holds a static route.
type StaticRouteConfig struct {
	Prefix            netip.Prefix
	NextHop           string
	NextHopSelf       bool   // Use local address as next-hop
	Origin            string // igp, egp, incomplete
	LocalPreference   uint32
	MED               uint32
	Community         string
	ExtendedCommunity string
	LargeCommunity    string
	ASPath            string
	PathInformation   string // path-id for add-path
	Label             string // MPLS label
	RD                string // Route Distinguisher
	Aggregator        string // ASN:IP format
	AtomicAggregate   bool   // ATOMIC_AGGREGATE attribute
	Attribute         string // Raw attribute hex: [ code flags value ]
}

// MVPNRouteConfig holds an MVPN route configuration.
type MVPNRouteConfig struct {
	RouteType         string // shared-join, source-join, source-ad
	IsIPv6            bool
	RD                string
	SourceAS          uint32
	Source            string // source IP or RP IP
	Group             string // multicast group
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ExtendedCommunity string
}

// VPLSRouteConfig holds a VPLS route configuration.
type VPLSRouteConfig struct {
	Name              string
	RD                string
	Endpoint          uint16
	Base              uint32
	Offset            uint16
	Size              uint16
	NextHop           string
	Origin            string
	LocalPreference   uint32
	MED               uint32
	ASPath            string
	Community         string
	ExtendedCommunity string
	OriginatorID      string
	ClusterList       string
}

// FlowSpecRouteConfig holds a FlowSpec route configuration.
type FlowSpecRouteConfig struct {
	Name              string
	IsIPv6            bool
	RD                string // for flow-vpn
	Match             map[string]string
	Then              map[string]string
	NextHop           string
	ExtendedCommunity string
}

// MUPRouteConfig holds a MUP route configuration.
type MUPRouteConfig struct {
	RouteType         string // mup-isd, mup-dsd, mup-t1st, mup-t2st
	IsIPv6            bool
	Prefix            string
	Address           string
	RD                string
	TEID              string
	QFI               uint8
	Endpoint          string
	NextHop           string
	ExtendedCommunity string
	PrefixSID         string
}

// ProcessConfig holds process configuration.
type ProcessConfig struct {
	Name    string
	Run     string
	Encoder string
}

// TreeToConfig converts a parsed tree to a typed BGPConfig.
func TreeToConfig(tree *Tree) (*BGPConfig, error) {
	cfg := &BGPConfig{}

	// Global settings
	if v, ok := tree.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nil, fmt.Errorf("invalid router-id: %w", err)
		}
		cfg.RouterID = ipToUint32(ip)
	}

	if v, ok := tree.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid local-as: %w", err)
		}
		cfg.LocalAS = uint32(n)
	}

	if v, ok := tree.Get("listen"); ok {
		cfg.Listen = v
	}

	// Processes
	for name, proc := range tree.GetList("process") {
		pc := ProcessConfig{Name: name}
		if v, ok := proc.Get("run"); ok {
			pc.Run = v
		}
		if v, ok := proc.Get("encoder"); ok {
			pc.Encoder = v
		}
		cfg.Processes = append(cfg.Processes, pc)
	}

	// Parse templates first (for inheritance)
	templates := make(map[string]*Tree)
	if tmpl := tree.GetContainer("template"); tmpl != nil {
		for name, neighborTree := range tmpl.GetList("neighbor") {
			templates[name] = neighborTree
		}
	}

	// Neighbors
	for addr, n := range tree.GetList("neighbor") {
		nc, err := parseNeighborConfig(addr, n, templates)
		if err != nil {
			return nil, fmt.Errorf("neighbor %s: %w", addr, err)
		}
		cfg.Neighbors = append(cfg.Neighbors, nc)
	}

	return cfg, nil
}

func parseNeighborConfig(addr string, tree *Tree, templates map[string]*Tree) (NeighborConfig, error) {
	nc := NeighborConfig{}

	// Address
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return nc, fmt.Errorf("invalid address: %w", err)
	}
	nc.Address = ip

	// Handle template inheritance - extract routes and settings from template FIRST
	var tmpl *Tree
	if inheritName, ok := tree.Get("inherit"); ok {
		if t, exists := templates[inheritName]; exists {
			tmpl = t
			routes, err := extractRoutesFromTree(tmpl)
			if err != nil {
				return nc, fmt.Errorf("template %s: %w", inheritName, err)
			}
			nc.StaticRoutes = append(nc.StaticRoutes, routes...)
		}
	}

	// Simple fields
	if v, ok := tree.Get("description"); ok {
		nc.Description = v
	}

	if v, ok := tree.Get("router-id"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nc, fmt.Errorf("invalid router-id: %w", err)
		}
		nc.RouterID = ipToUint32(ip)
	}

	if v, ok := tree.Get("local-address"); ok {
		ip, err := netip.ParseAddr(v)
		if err != nil {
			return nc, fmt.Errorf("invalid local-address: %w", err)
		}
		nc.LocalAddress = ip
	}

	if v, ok := tree.Get("local-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid local-as: %w", err)
		}
		nc.LocalAS = uint32(n)
	}

	if v, ok := tree.Get("peer-as"); ok {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return nc, fmt.Errorf("invalid peer-as: %w", err)
		}
		nc.PeerAS = uint32(n)
	}

	if v, ok := tree.Get("hold-time"); ok {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return nc, fmt.Errorf("invalid hold-time: %w", err)
		}
		nc.HoldTime = uint16(n)
	}

	if v, ok := tree.Get("passive"); ok {
		nc.Passive = v == configTrue
	}

	// group-updates defaults to true, check template first then neighbor
	nc.GroupUpdates = true
	if tmpl != nil {
		if v, ok := tmpl.Get("group-updates"); ok {
			nc.GroupUpdates = v == configTrue
		}
	}
	if v, ok := tree.Get("group-updates"); ok {
		nc.GroupUpdates = v == configTrue
	}

	if v, ok := tree.Get("host-name"); ok {
		nc.Hostname = v
	}

	if v, ok := tree.Get("domain-name"); ok {
		nc.DomainName = v
	}

	// Families - Freeform stores "ipv4 unicast" as key with value "true"
	if familyTree := tree.GetContainer("family"); familyTree != nil {
		nc.Families = append(nc.Families, familyTree.Values()...)
	}

	// Capabilities
	if cap := tree.GetContainer("capability"); cap != nil {
		if v, ok := cap.Get("asn4"); ok {
			nc.Capabilities.ASN4 = v == configTrue
		}
		if v, ok := cap.Get("route-refresh"); ok {
			nc.Capabilities.RouteRefresh = v == configTrue
		}
		if gr := cap.GetContainer("graceful-restart"); gr != nil {
			nc.Capabilities.GracefulRestart = true
			if v, ok := gr.Get("restart-time"); ok {
				n, _ := strconv.ParseUint(v, 10, 16)
				nc.Capabilities.RestartTime = uint16(n)
			}
		}
		if ap := cap.GetContainer("add-path"); ap != nil {
			if v, ok := ap.Get("send"); ok {
				nc.Capabilities.AddPathSend = v == configTrue
			}
			if v, ok := ap.Get("receive"); ok {
				nc.Capabilities.AddPathReceive = v == configTrue
			}
		}
		if v, ok := cap.GetFlex("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
	}

	// Extract routes from this neighbor's static and announce blocks
	routes, err := extractRoutesFromTree(tree)
	if err != nil {
		return nc, err
	}
	nc.StaticRoutes = append(nc.StaticRoutes, routes...)

	// Extract exotic routes
	nc.MVPNRoutes = extractMVPNRoutes(tree)
	nc.VPLSRoutes = extractVPLSRoutes(tree)
	nc.FlowSpecRoutes = extractFlowSpecRoutes(tree)
	nc.MUPRoutes = extractMUPRoutes(tree)

	return nc, nil
}

// extractRoutesFromTree extracts all routes from a neighbor or template tree.
// Handles both static { route ... } and announce { ipv4/ipv6 { unicast/multicast ... } } blocks.
func extractRoutesFromTree(tree *Tree) ([]StaticRouteConfig, error) {
	var routes []StaticRouteConfig

	// Static routes
	if static := tree.GetContainer("static"); static != nil {
		for prefix, route := range static.GetList("route") {
			sr, err := parseRouteConfig(prefix, route)
			if err != nil {
				return nil, err
			}
			routes = append(routes, sr)
		}
	}

	// Announce routes - parse from announce { ipv4 { unicast ... } } structure
	if announce := tree.GetContainer("announce"); announce != nil {
		// Parse IPv4 routes
		if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
			for prefix, route := range ipv4.GetList("unicast") {
				sr, err := parseRouteConfig(prefix, route)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for prefix, route := range ipv4.GetList("multicast") {
				sr, err := parseRouteConfig(prefix, route)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
		}
		// Parse IPv6 routes
		if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
			for prefix, route := range ipv6.GetList("unicast") {
				sr, err := parseRouteConfig(prefix, route)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
			for prefix, route := range ipv6.GetList("multicast") {
				sr, err := parseRouteConfig(prefix, route)
				if err != nil {
					return nil, err
				}
				routes = append(routes, sr)
			}
		}
	}

	return routes, nil
}

// parseRouteConfig extracts a StaticRouteConfig from a parsed route tree.
func parseRouteConfig(prefix string, route *Tree) (StaticRouteConfig, error) {
	sr := StaticRouteConfig{}

	// Try as prefix first, then as bare IP (host route)
	p, err := netip.ParsePrefix(prefix)
	if err != nil {
		// Try as bare IP, convert to /32 or /128
		ip, err2 := netip.ParseAddr(prefix)
		if err2 != nil {
			return sr, fmt.Errorf("invalid prefix %s: %w", prefix, err)
		}
		bits := 32
		if ip.Is6() {
			bits = 128
		}
		p = netip.PrefixFrom(ip, bits)
	}
	sr.Prefix = p

	if v, ok := route.Get("next-hop"); ok {
		if v == "self" {
			sr.NextHopSelf = true
		} else {
			sr.NextHop = v
		}
	}
	if v, ok := route.Get("local-preference"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		sr.LocalPreference = uint32(n)
	}
	if v, ok := route.Get("med"); ok {
		n, _ := strconv.ParseUint(v, 10, 32)
		sr.MED = uint32(n)
	}
	if v, ok := route.Get("community"); ok {
		sr.Community = v
	}
	if v, ok := route.Get("extended-community"); ok {
		sr.ExtendedCommunity = v
	}
	if v, ok := route.Get("large-community"); ok {
		sr.LargeCommunity = v
	}
	if v, ok := route.Get("as-path"); ok {
		sr.ASPath = v
	}
	if v, ok := route.Get("origin"); ok {
		sr.Origin = v
	}
	if v, ok := route.Get("path-information"); ok {
		sr.PathInformation = v
	}
	if v, ok := route.Get("label"); ok {
		sr.Label = v
	}
	if v, ok := route.Get("rd"); ok {
		sr.RD = v
	}
	if v, ok := route.Get("aggregator"); ok {
		sr.Aggregator = v
	}
	// atomic-aggregate can be a standalone flag or have a value
	if _, ok := route.Get("atomic-aggregate"); ok {
		sr.AtomicAggregate = true
	}
	if v, ok := route.Get("attribute"); ok {
		sr.Attribute = v
	}

	return sr, nil
}

// extractMVPNRoutes extracts MVPN routes from announce { ipv4/ipv6 { mcast-vpn ... } }.
func extractMVPNRoutes(tree *Tree) []MVPNRouteConfig {
	var routes []MVPNRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MVPN
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		for routeType, route := range ipv4.GetList("mcast-vpn") {
			r := parseMVPNRoute(routeType, route, false)
			routes = append(routes, r)
		}
	}

	// IPv6 MVPN
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		for routeType, route := range ipv6.GetList("mcast-vpn") {
			r := parseMVPNRoute(routeType, route, true)
			routes = append(routes, r)
		}
	}

	return routes
}

func parseMVPNRoute(routeType string, route *Tree, isIPv6 bool) MVPNRouteConfig {
	r := MVPNRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("source-as"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.SourceAS = uint32(n)
		}
	}
	// Source can be "source" or "rp" depending on route type
	if v, ok := route.Get("source"); ok {
		r.Source = v
	} else if v, ok := route.Get("rp"); ok {
		r.Source = v
	}
	if v, ok := route.Get("group"); ok {
		r.Group = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}

	return r
}

// parseInlineKeyValues parses an inline "key value key value ..." string into a map.
// Handles arrays like "[ a b c ]" and parenthesized content like "( ... )".
func parseInlineKeyValues(inline string) map[string]string {
	result := make(map[string]string)
	tokens := tokenizeInline(inline)

	i := 0
	for i < len(tokens) {
		key := tokens[i]
		i++
		if i >= len(tokens) {
			break
		}

		// Collect value (might be array or parenthesized)
		if tokens[i] == "[" {
			// Array: collect until ]
			var arr []string
			i++ // skip [
			for i < len(tokens) && tokens[i] != "]" {
				arr = append(arr, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip ]
			}
			result[key] = "[" + strings.Join(arr, " ") + "]"
		} else if tokens[i] == "(" {
			// Parenthesized: collect until )
			depth := 1
			var paren []string
			i++ // skip (
			for i < len(tokens) && depth > 0 {
				if tokens[i] == "(" {
					depth++
				} else if tokens[i] == ")" {
					depth--
					if depth == 0 {
						break
					}
				}
				paren = append(paren, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++ // skip )
			}
			result[key] = "(" + strings.Join(paren, " ") + ")"
		} else {
			// Simple value
			result[key] = tokens[i]
			i++
		}
	}

	return result
}

// tokenizeInline splits an inline string into tokens, preserving brackets and parens.
func tokenizeInline(s string) []string {
	var tokens []string
	var current strings.Builder

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '[', ']', '(', ')':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(c))
		case '\\':
			// Skip backslash continuations - they're artifacts from multiline parsing
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// parseVPLSFromInline creates a VPLSRouteConfig from an inline string.
func parseVPLSFromInline(inline string) VPLSRouteConfig {
	kv := parseInlineKeyValues(inline)
	r := VPLSRouteConfig{}

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["endpoint"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := kv["base"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := kv["offset"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := kv["size"]; ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["origin"]; ok {
		r.Origin = v
	}
	if v, ok := kv["local-preference"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := kv["med"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := kv["as-path"]; ok {
		r.ASPath = v
	}
	if v, ok := kv["community"]; ok {
		r.Community = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["originator-id"]; ok {
		r.OriginatorID = v
	}
	if v, ok := kv["cluster-list"]; ok {
		r.ClusterList = v
	}

	return r
}

// extractVPLSRoutes extracts VPLS routes from l2vpn { vpls ... } and announce { l2vpn { vpls ... } }.
// Order: announce inline first, then l2vpn named, then l2vpn inline (to match ExaBGP behavior).
func extractVPLSRoutes(tree *Tree) []VPLSRouteConfig {
	var routes []VPLSRouteConfig

	// From announce { l2vpn { vpls ... } } - inline routes first
	if announce := tree.GetContainer("announce"); announce != nil {
		if l2vpn := announce.GetContainer("l2vpn"); l2vpn != nil {
			// Inline first
			for _, inline := range l2vpn.GetMultiValues("vpls") {
				if inline != "" && inline != configTrue {
					r := parseVPLSFromInline(inline)
					routes = append(routes, r)
				}
			}
			// Named blocks from announce
			for name, route := range l2vpn.GetList("vpls") {
				r := parseVPLSRoute(name, route)
				routes = append(routes, r)
			}
		}
	}

	// From l2vpn block - named blocks then inline
	if l2vpn := tree.GetContainer("l2vpn"); l2vpn != nil {
		// Named blocks: vpls site5 { ... }
		for name, route := range l2vpn.GetList("vpls") {
			r := parseVPLSRoute(name, route)
			routes = append(routes, r)
		}
		// Inline: vpls rd X endpoint Y ...;
		for _, inline := range l2vpn.GetMultiValues("vpls") {
			if inline != "" && inline != configTrue {
				r := parseVPLSFromInline(inline)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseVPLSRoute(name string, route *Tree) VPLSRouteConfig {
	r := VPLSRouteConfig{Name: name}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("endpoint"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Endpoint = uint16(n)
		}
	}
	if v, ok := route.Get("base"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.Base = uint32(n)
		}
	}
	if v, ok := route.Get("offset"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Offset = uint16(n)
		}
	}
	if v, ok := route.Get("size"); ok {
		if n, err := strconv.ParseUint(v, 10, 16); err == nil {
			r.Size = uint16(n)
		}
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("origin"); ok {
		r.Origin = v
	}
	if v, ok := route.Get("local-preference"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.LocalPreference = uint32(n)
		}
	}
	if v, ok := route.Get("med"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			r.MED = uint32(n)
		}
	}
	if v, ok := route.Get("as-path"); ok {
		r.ASPath = v
	}
	if v, ok := route.Get("community"); ok {
		r.Community = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("originator-id"); ok {
		r.OriginatorID = v
	}
	if v, ok := route.Get("cluster-list"); ok {
		r.ClusterList = v
	}

	return r
}

// extractFlowSpecRoutes extracts FlowSpec routes from flow { route ... }.
func extractFlowSpecRoutes(tree *Tree) []FlowSpecRouteConfig {
	var routes []FlowSpecRouteConfig

	flow := tree.GetContainer("flow")
	if flow == nil {
		return routes
	}

	// Use ordered iteration to preserve config order.
	for _, entry := range flow.GetListOrdered("route") {
		r := parseFlowSpecRoute(entry.Key, entry.Value)
		routes = append(routes, r)
	}

	return routes
}

func parseFlowSpecRoute(name string, route *Tree) FlowSpecRouteConfig {
	r := FlowSpecRouteConfig{
		Name:  name,
		Match: make(map[string]string),
		Then:  make(map[string]string),
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}

	// Parse match block - Freeform stores:
	// - "keyword value" -> "true" for simple values like "source 10.0.0.1/32"
	// - "keyword" -> "value" for arrays like "fragment [ last-fragment ]"
	if match := route.GetContainer("match"); match != nil {
		for _, key := range match.Values() {
			val, _ := match.Get(key)
			if val == "true" || val == "" {
				// Legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.Match[parts[0]] = parts[1]
				} else {
					r.Match[key] = ""
				}
			} else {
				// Array format: key is keyword, val has the values
				r.Match[key] = val
			}
		}
	}

	// Parse then block - Freeform stores:
	// - "keyword" -> "true" for flags like "discard"
	// - "keyword" -> "value" for arrays like "community [30740:0 30740:30740]"
	// - "keyword value" -> "true" for simple key-value (older format)
	if then := route.GetContainer("then"); then != nil {
		for _, key := range then.Values() {
			val, _ := then.Get(key)
			if val == "true" || val == "" {
				// Flag or legacy format: key might be "keyword value"
				parts := strings.SplitN(key, " ", 2)
				if len(parts) == 2 {
					r.Then[parts[0]] = parts[1]
				} else {
					r.Then[key] = ""
				}
			} else {
				// Array format: key is keyword, val has the values
				r.Then[key] = val
			}
		}
	}

	// Determine if IPv6 based on match criteria
	for key, val := range r.Match {
		if key == "source" || key == "destination" {
			if strings.Contains(val, ":") {
				r.IsIPv6 = true
				break
			}
		}
	}

	return r
}

// parseMUPFromInline creates a MUPRouteConfig from an inline string.
// Format: "mup-isd PREFIX rd RD next-hop NH ..." or "mup-dsd ADDR rd RD ..."
func parseMUPFromInline(inline string, isIPv6 bool) MUPRouteConfig {
	tokens := tokenizeInline(inline)
	if len(tokens) == 0 {
		return MUPRouteConfig{}
	}

	r := MUPRouteConfig{
		IsIPv6: isIPv6,
	}

	// First token is route type
	r.RouteType = tokens[0]

	// Second token is prefix or address
	if len(tokens) > 1 {
		if r.RouteType == "mup-isd" || r.RouteType == "mup-t1st" {
			r.Prefix = tokens[1]
		} else {
			r.Address = tokens[1]
		}
	}

	// Parse remaining as key-value pairs starting from index 2
	kv := make(map[string]string)
	i := 2
	for i < len(tokens) {
		key := tokens[i]
		i++
		if i >= len(tokens) {
			break
		}

		if tokens[i] == "[" {
			// Array
			var arr []string
			i++
			for i < len(tokens) && tokens[i] != "]" {
				arr = append(arr, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++
			}
			kv[key] = "[" + strings.Join(arr, " ") + "]"
		} else if tokens[i] == "(" {
			// Parenthesized
			depth := 1
			var paren []string
			i++
			for i < len(tokens) && depth > 0 {
				if tokens[i] == "(" {
					depth++
				} else if tokens[i] == ")" {
					depth--
					if depth == 0 {
						break
					}
				}
				paren = append(paren, tokens[i])
				i++
			}
			if i < len(tokens) {
				i++
			}
			kv[key] = "(" + strings.Join(paren, " ") + ")"
		} else {
			kv[key] = tokens[i]
			i++
		}
	}

	if v, ok := kv["rd"]; ok {
		r.RD = v
	}
	if v, ok := kv["teid"]; ok {
		r.TEID = v
	}
	if v, ok := kv["qfi"]; ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := kv["endpoint"]; ok {
		r.Endpoint = v
	}
	if v, ok := kv["next-hop"]; ok {
		r.NextHop = v
	}
	if v, ok := kv["extended-community"]; ok {
		r.ExtendedCommunity = v
	}
	if v, ok := kv["bgp-prefix-sid-srv6"]; ok {
		r.PrefixSID = v
	}

	return r
}

// extractMUPRoutes extracts MUP routes from announce { ipv4/ipv6 { mup ... } }.
func extractMUPRoutes(tree *Tree) []MUPRouteConfig {
	var routes []MUPRouteConfig

	announce := tree.GetContainer("announce")
	if announce == nil {
		return routes
	}

	// IPv4 MUP
	if ipv4 := announce.GetContainer("ipv4"); ipv4 != nil {
		// Named blocks (if any)
		for routeType, route := range ipv4.GetList("mup") {
			r := parseMUPRoute(routeType, route, false)
			routes = append(routes, r)
		}
		// Inline: mup mup-isd PREFIX rd RD ...;
		for _, inline := range ipv4.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, false)
				routes = append(routes, r)
			}
		}
	}

	// IPv6 MUP
	if ipv6 := announce.GetContainer("ipv6"); ipv6 != nil {
		// Named blocks (if any)
		for routeType, route := range ipv6.GetList("mup") {
			r := parseMUPRoute(routeType, route, true)
			routes = append(routes, r)
		}
		// Inline
		for _, inline := range ipv6.GetMultiValues("mup") {
			if inline != "" && inline != configTrue {
				r := parseMUPFromInline(inline, true)
				routes = append(routes, r)
			}
		}
	}

	return routes
}

func parseMUPRoute(routeType string, route *Tree, isIPv6 bool) MUPRouteConfig {
	r := MUPRouteConfig{
		RouteType: routeType,
		IsIPv6:    isIPv6,
	}

	// Route type determines which field to use for prefix/address
	if strings.HasSuffix(routeType, "-isd") || strings.HasSuffix(routeType, "-t1st") {
		// These have prefix
		for _, key := range route.Values() {
			if strings.Contains(key, "/") || strings.Contains(key, ":") {
				r.Prefix = key
				break
			}
		}
	} else {
		// mup-dsd, mup-t2st have address
		for _, key := range route.Values() {
			if !strings.Contains(key, "/") && (strings.Contains(key, ".") || strings.Contains(key, ":")) {
				r.Address = key
				break
			}
		}
	}

	if v, ok := route.Get("rd"); ok {
		r.RD = v
	}
	if v, ok := route.Get("teid"); ok {
		r.TEID = v
	}
	if v, ok := route.Get("qfi"); ok {
		if n, err := strconv.ParseUint(v, 10, 8); err == nil {
			r.QFI = uint8(n)
		}
	}
	if v, ok := route.Get("endpoint"); ok {
		r.Endpoint = v
	}
	if v, ok := route.Get("next-hop"); ok {
		r.NextHop = v
	}
	if v, ok := route.Get("extended-community"); ok {
		r.ExtendedCommunity = v
	}
	if v, ok := route.Get("bgp-prefix-sid-srv6"); ok {
		r.PrefixSID = v
	}

	return r
}

// ipToUint32 converts an IPv4 address to uint32.
func ipToUint32(ip netip.Addr) uint32 {
	if !ip.Is4() {
		return 0
	}
	b := ip.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// ToReactorConfig converts BGPConfig to reactor configuration.
func (c *BGPConfig) ToReactorNeighbors() []*NeighborReactor {
	neighbors := make([]*NeighborReactor, 0, len(c.Neighbors))

	for _, nc := range c.Neighbors {
		n := &NeighborReactor{
			Address:  nc.Address,
			Port:     179,
			LocalAS:  nc.LocalAS,
			PeerAS:   nc.PeerAS,
			RouterID: nc.RouterID,
			HoldTime: time.Duration(nc.HoldTime) * time.Second,
			Passive:  nc.Passive,
		}

		// Use global LocalAS if not set per-neighbor
		if n.LocalAS == 0 {
			n.LocalAS = c.LocalAS
		}

		// Use global RouterID if not set per-neighbor
		if n.RouterID == 0 {
			n.RouterID = c.RouterID
		}

		neighbors = append(neighbors, n)
	}

	return neighbors
}

// NeighborReactor is the reactor-compatible neighbor config.
type NeighborReactor struct {
	Address  netip.Addr
	Port     uint16
	LocalAS  uint32
	PeerAS   uint32
	RouterID uint32
	HoldTime time.Duration
	Passive  bool
}
