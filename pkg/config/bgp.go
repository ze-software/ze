package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"time"
)

const (
	configTrue   = "true"   // Config value for boolean true
	configEnable = "enable" // Config value for enabled state
)

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
				Field("vpls", Flex()),     // VPLS - complex inline format
				Field("evpn", Freeform()), // EVPN - complex format
			)),
		)),

		// Static routes - InlineList supports "route PREFIX attr val attr val;"
		Field("static", Container(
			Field("route", InlineList(TypePrefix, routeAttributes()...)),
		)),

		// Flow routes
		Field("flow", Freeform()),

		// L2VPN (VPLS, EVPN)
		Field("l2vpn", Freeform()),

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
	Address      netip.Addr
	Description  string
	RouterID     uint32
	LocalAddress netip.Addr
	LocalAS      uint32
	PeerAS       uint32
	HoldTime     uint16
	Passive      bool
	Families     []string
	Hostname     string
	DomainName   string
	Capabilities CapabilityConfig
	StaticRoutes []StaticRouteConfig
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

	// Handle template inheritance - extract routes from template FIRST
	if inheritName, ok := tree.Get("inherit"); ok {
		if tmpl, exists := templates[inheritName]; exists {
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
		if v, ok := cap.Get("software-version"); ok {
			nc.Capabilities.SoftwareVersion = v == configTrue || v == configEnable
		}
	}

	// Extract routes from this neighbor's static and announce blocks
	routes, err := extractRoutesFromTree(tree)
	if err != nil {
		return nc, err
	}
	nc.StaticRoutes = append(nc.StaticRoutes, routes...)

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
		sr.NextHop = v
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

	return sr, nil
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
