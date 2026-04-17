// Design: docs/features/interfaces.md -- Interface config parsing and application
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface used for application
// Related: register.go -- OnConfigure calls applyConfig

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"sync/atomic"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"
	sysctlevents "codeberg.org/thomas-mangin/ze/internal/plugins/sysctl/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// yangTrue is the string representation of boolean true in YANG config JSON.
const yangTrue = "true"

// ifaceConfig is the parsed representation of the interface config section.
type ifaceConfig struct {
	Backend        string
	DHCPAuto       bool   // auto-discover first ethernet for DHCP
	ResolvConfPath string // path for DHCP DNS resolv.conf (empty = disabled)
	Ethernet       []ifaceEntry
	Dummy          []ifaceEntry
	Veth           []vethEntry
	Bridge         []bridgeEntry
	Tunnel         []tunnelEntry
	Wireguard      []wireguardEntry
	Loopback       *loopbackEntry
}

// tunnelEntry represents a configured tunnel interface. The Spec carries
// the encapsulation kind plus per-kind parameters; the embedded ifaceEntry
// carries the shared physical and unit fields (mtu, mac, addresses).
type tunnelEntry struct {
	ifaceEntry
	Spec TunnelSpec
}

// wireguardEntry represents a configured wireguard interface. The Spec
// carries the wireguard-specific parameters and peer list; the embedded
// ifaceEntry carries the shared common and unit fields inherited via the
// interface-common and interface-unit YANG groupings.
type wireguardEntry struct {
	ifaceEntry
	Spec WireguardSpec
}

// ifaceEntry represents a configured interface (ethernet or dummy).
type ifaceEntry struct {
	Name       string
	MTU        int
	MACAddress string
	Disable    bool
	Units      []unitEntry
}

// vethEntry extends ifaceEntry with a peer name.
type vethEntry struct {
	ifaceEntry
	Peer string
}

// bridgeEntry extends ifaceEntry with bridge-specific config.
type bridgeEntry struct {
	ifaceEntry
	STP     bool
	Members []string
}

// loopbackEntry has units only (no physical properties).
type loopbackEntry struct {
	Units []unitEntry
}

// unitEntry represents a logical unit on an interface.
type unitEntry struct {
	ID             int
	VLANID         int
	Addresses      []string
	Disable        bool
	RoutePriority  int // route metric for DHCP default routes (0 = kernel default)
	SysctlProfiles []string
	IPv4           *ipv4Sysctl
	IPv6           *ipv6Sysctl
	MirrorIngress  string // destination interface name, empty = not configured
	MirrorEgress   string
	DHCP           *dhcpUnitConfig
	DHCPv6         *dhcpv6UnitConfig
}

// dhcpUnitConfig holds DHCPv4 client settings parsed from the YANG
// "dhcp" container inside an interface unit.
type dhcpUnitConfig struct {
	Enabled  bool
	Hostname string
	ClientID string
}

// dhcpv6UnitConfig holds DHCPv6 client settings parsed from the YANG
// "dhcpv6" container inside an interface unit.
type dhcpv6UnitConfig struct {
	Enabled  bool
	PDLength int // 0 = not set (server decides)
	DUID     string
}

// ipv4Sysctl holds per-interface IPv4 sysctl settings.
// Pointer fields: nil = not configured (leave OS default), non-nil = set.
type ipv4Sysctl struct {
	Forwarding  *bool
	ArpFilter   *bool
	ArpAccept   *bool
	ProxyARP    *bool
	ArpAnnounce *int
	ArpIgnore   *int
	RPFilter    *int
}

// ipv6Sysctl holds per-interface IPv6 sysctl settings.
type ipv6Sysctl struct {
	Autoconf   *bool
	AcceptRA   *int
	Forwarding *bool
}

// parseIfaceSections finds the "interface" section and parses it. Returns a
// default config if no interface section is present. Parse errors propagate
// to the caller so OnConfigVerify can reject malformed input rather than
// silently apply a default.
func parseIfaceSections(sections []sdk.ConfigSection) (*ifaceConfig, error) {
	for _, s := range sections {
		if s.Root != "interface" {
			continue
		}
		cfg, err := parseIfaceConfig(s.Data)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}
	return &ifaceConfig{Backend: defaultBackendName}, nil
}

// parseIfaceConfig parses the interface config section JSON into ifaceConfig.
// The JSON is wrapped: {"interface": {...}}.
func parseIfaceConfig(data string) (*ifaceConfig, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("iface config: unmarshal: %w", err)
	}

	ifaceMap, ok := root["interface"].(map[string]any)
	if !ok {
		return &ifaceConfig{Backend: defaultBackendName}, nil
	}

	cfg := &ifaceConfig{
		Backend:        defaultBackendName,
		ResolvConfPath: "/tmp/resolv.conf",
	}

	if b, ok := ifaceMap["backend"].(string); ok && b != "" {
		cfg.Backend = b
	}

	if v, ok := ifaceMap["dhcp-auto"].(string); ok {
		cfg.DHCPAuto = v == yangTrue
	}

	if v, ok := ifaceMap["resolv-conf-path"].(string); ok {
		if v != "" && !filepath.IsAbs(v) {
			return nil, fmt.Errorf("interface resolv-conf-path: must be absolute path, got %q", v)
		}
		if v != "" && filepath.Clean(v) != v {
			return nil, fmt.Errorf("interface resolv-conf-path: path contains traversal or redundant separators: %q", v)
		}
		cfg.ResolvConfPath = v
	}

	if ethMap, ok := ifaceMap["ethernet"].(map[string]any); ok {
		for name, v := range ethMap {
			m, _ := v.(map[string]any)
			cfg.Ethernet = append(cfg.Ethernet, parseIfaceEntry(name, m))
		}
	}

	if dummyMap, ok := ifaceMap["dummy"].(map[string]any); ok {
		for name, v := range dummyMap {
			m, _ := v.(map[string]any)
			cfg.Dummy = append(cfg.Dummy, parseIfaceEntry(name, m))
		}
	}

	if vethMap, ok := ifaceMap["veth"].(map[string]any); ok {
		for name, v := range vethMap {
			m, _ := v.(map[string]any)
			entry := vethEntry{ifaceEntry: parseIfaceEntry(name, m)}
			if peer, ok := m["peer"].(string); ok {
				entry.Peer = peer
			}
			cfg.Veth = append(cfg.Veth, entry)
		}
	}

	if brMap, ok := ifaceMap["bridge"].(map[string]any); ok {
		for name, v := range brMap {
			m, _ := v.(map[string]any)
			entry := bridgeEntry{ifaceEntry: parseIfaceEntry(name, m)}
			if stp, ok := m["stp"].(string); ok {
				entry.STP = stp == yangTrue
			}
			if members, ok := m["member"].([]any); ok {
				for _, mem := range members {
					if s, ok := mem.(string); ok {
						entry.Members = append(entry.Members, s)
					}
				}
			}
			cfg.Bridge = append(cfg.Bridge, entry)
		}
	}

	if tunMap, ok := ifaceMap["tunnel"].(map[string]any); ok {
		for name, v := range tunMap {
			m, _ := v.(map[string]any)
			entry, err := parseTunnelEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("tunnel %q: %w", name, err)
			}
			cfg.Tunnel = append(cfg.Tunnel, entry)
		}
	}

	if wgMap, ok := ifaceMap["wireguard"].(map[string]any); ok {
		for name, v := range wgMap {
			m, _ := v.(map[string]any)
			entry, err := parseWireguardEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("wireguard %q: %w", name, err)
			}
			cfg.Wireguard = append(cfg.Wireguard, entry)
		}
	}

	if loMap, ok := ifaceMap["loopback"].(map[string]any); ok {
		lo := &loopbackEntry{}
		lo.Units = parseUnits(loMap)
		cfg.Loopback = lo
	}

	return cfg, nil
}

// parseTunnelEntry walks the JSON tree for one tunnel list entry and produces
// a tunnelEntry whose Spec is the resolved encapsulation case. The YANG
// schema enforces that exactly one case key is present under encapsulation;
// the parser still verifies and reports a clear error if zero or two are seen.
//
// VLAN units are rejected on L3 tunnel kinds because the Linux kernel does
// not allow VLAN tagging on netdevs that do not carry Ethernet frames; only
// gretap/ip6gretap (the L2/bridgeable kinds) accept VLAN sub-interfaces.
func parseTunnelEntry(name string, m map[string]any) (tunnelEntry, error) {
	entry := tunnelEntry{ifaceEntry: parseIfaceEntry(name, m)}
	// MAC address for tunnels comes from inside the encapsulation case
	// container (gretap/ip6gretap only), not from the list level. Clear
	// any list-level mac-address that parseIfaceEntry may have read.
	entry.MACAddress = ""
	entry.Spec.Name = name

	encMap, ok := m["encapsulation"].(map[string]any)
	if !ok {
		return entry, fmt.Errorf("missing encapsulation block")
	}

	var matchedKind TunnelKind
	var matchedCase map[string]any
	for caseName, raw := range encMap {
		k, ok := ParseTunnelKind(caseName)
		if !ok {
			return entry, fmt.Errorf("unknown encapsulation kind %q", caseName)
		}
		caseMap, _ := raw.(map[string]any)
		if matchedKind != TunnelKindUnknown {
			return entry, fmt.Errorf("multiple encapsulation cases set: %s and %s", matchedKind, k)
		}
		matchedKind = k
		matchedCase = caseMap
	}
	if matchedKind == TunnelKindUnknown {
		return entry, fmt.Errorf("encapsulation block has no kind selected")
	}
	entry.Spec.Kind = matchedKind
	if err := parseTunnelLeaves(&entry.Spec, matchedCase); err != nil {
		return entry, err
	}
	// MAC address lives inside the case container for bridgeable kinds
	// (gretap/ip6gretap). L3 kinds have no mac-address leaf in YANG.
	if matchedKind.IsBridgeable() {
		if mac, ok := matchedCase["mac-address"].(string); ok {
			entry.MACAddress = mac
		}
	}
	if entry.Spec.LocalAddress != "" && entry.Spec.LocalInterface != "" {
		return entry, fmt.Errorf("local ip and local interface are mutually exclusive")
	}
	if entry.Spec.LocalAddress == "" && entry.Spec.LocalInterface == "" {
		return entry, fmt.Errorf("local ip or local interface required")
	}
	if !matchedKind.IsBridgeable() {
		for i := range entry.Units {
			if entry.Units[i].VLANID > 0 {
				return entry, fmt.Errorf("vlan-id units are not supported on %s tunnels (only gretap and ip6gretap carry Ethernet frames)", matchedKind)
			}
		}
	}
	return entry, nil
}

// parseTunnelLeaves extracts the per-case leaves into the spec. Leaves that
// are not applicable to the kind are simply absent from the YANG and the
// JSON map (the schema rejects them at parse time), so we read them
// unconditionally and rely on the YANG-side filtering.
//
// The local and remote endpoints live in nested containers to match the
// existing ze convention used by `bgp peer connection { local { ip ... }
// remote { ip ... } }`. Numeric leaves report parse errors so an out-of-range
// value reaches the caller instead of being silently dropped (the YANG
// validator catches the same conditions earlier; this is defense in depth).
func parseTunnelLeaves(spec *TunnelSpec, caseMap map[string]any) error {
	if caseMap == nil {
		return nil
	}
	if local, ok := caseMap["local"].(map[string]any); ok {
		if v, ok := local["ip"].(string); ok && v != "" {
			spec.LocalAddress = v
		}
		if v, ok := local["interface"].(string); ok && v != "" {
			spec.LocalInterface = v
		}
	}
	if remote, ok := caseMap["remote"].(map[string]any); ok {
		if v, ok := remote["ip"].(string); ok && v != "" {
			spec.RemoteAddress = v
		}
	}
	if v, ok := caseMap["key"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("key %q: %w", v, err)
		}
		spec.Key = uint32(n)
		spec.KeySet = true
	}
	if v, ok := caseMap["ttl"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf("ttl %q: %w", v, err)
		}
		spec.TTL = uint8(n)
		spec.TTLSet = true
	}
	if v, ok := caseMap["tos"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf("tos %q: %w", v, err)
		}
		spec.Tos = uint8(n)
		spec.TosSet = true
	}
	if _, ok := caseMap["no-pmtu-discovery"]; ok {
		spec.NoPMTUDiscovery = true
	}
	if v, ok := caseMap["hoplimit"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf("hoplimit %q: %w", v, err)
		}
		spec.HopLimit = uint8(n)
		spec.HopLimitSet = true
	}
	if v, ok := caseMap["tclass"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf("tclass %q: %w", v, err)
		}
		spec.TClass = uint8(n)
		spec.TClassSet = true
	}
	if v, ok := caseMap["encaplimit"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 8)
		if err != nil {
			return fmt.Errorf("encaplimit %q: %w", v, err)
		}
		spec.EncapLimit = uint8(n)
		spec.EncapLimitSet = true
	}
	return nil
}

// parseWireguardEntry walks the JSON tree for one wireguard list entry and
// produces a wireguardEntry. private-key is mandatory and must decode to a
// valid 32-byte Curve25519 key via wgtypes.ParseKey; public-key on each peer
// is likewise mandatory. Sensitive leaves (private-key, preshared-key) are
// already plaintext at this layer -- the config parser's parseLeaf has
// decoded any $9$ prefix before the tree reaches us.
func parseWireguardEntry(name string, m map[string]any) (wireguardEntry, error) {
	entry := wireguardEntry{ifaceEntry: parseIfaceEntry(name, m)}
	// Wireguard uses interface-common (no mac-address leaf). Clear any
	// list-level mac-address that parseIfaceEntry may have read from a
	// hand-edited config. Same defense-in-depth as parseTunnelEntry.
	entry.MACAddress = ""
	entry.Spec.Name = name

	if m == nil {
		return entry, fmt.Errorf("empty wireguard entry")
	}

	privStr, ok := m["private-key"].(string)
	if !ok || privStr == "" {
		return entry, fmt.Errorf("private-key is required")
	}
	priv, err := wgtypes.ParseKey(privStr)
	if err != nil {
		return entry, fmt.Errorf("private-key: %w", err)
	}
	entry.Spec.PrivateKey = priv

	if portStr, ok := m["listen-port"].(string); ok && portStr != "" {
		p, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return entry, fmt.Errorf("listen-port %q: %w", portStr, err)
		}
		entry.Spec.ListenPort = uint16(p) //nolint:gosec // ParseUint bitSize=16 bounds value
		entry.Spec.ListenPortSet = true
	}

	if markStr, ok := m["fwmark"].(string); ok && markStr != "" {
		fw, err := strconv.ParseUint(markStr, 10, 32)
		if err != nil {
			return entry, fmt.Errorf("fwmark %q: %w", markStr, err)
		}
		entry.Spec.FirewallMark = uint32(fw) //nolint:gosec // ParseUint bitSize=32 bounds value
	}

	if peerMap, ok := m["peer"].(map[string]any); ok {
		seenPubKeys := make(map[string]string, len(peerMap))
		for pname, pv := range peerMap {
			pm, _ := pv.(map[string]any)
			peer, err := parseWireguardPeer(pname, pm)
			if err != nil {
				return entry, fmt.Errorf("peer %q: %w", pname, err)
			}
			pubKeyStr := peer.PublicKey.String()
			if prev, dup := seenPubKeys[pubKeyStr]; dup {
				return entry, fmt.Errorf("peer %q: duplicate public-key (same as peer %q)", pname, prev)
			}
			seenPubKeys[pubKeyStr] = pname
			entry.Spec.Peers = append(entry.Spec.Peers, peer)
		}
	}

	return entry, nil
}

// parseWireguardPeer walks the JSON tree for one peer list entry.
// public-key is mandatory; preshared-key, endpoint, allowed-ips, and
// persistent-keepalive are optional. The disable leaf makes the peer
// absent from the kernel peer set on reload while remaining in config.
func parseWireguardPeer(name string, m map[string]any) (WireguardPeerSpec, error) {
	peer := WireguardPeerSpec{Name: name}

	if m == nil {
		return peer, fmt.Errorf("empty peer entry")
	}

	if _, ok := m["disable"]; ok {
		peer.Disable = true
	}

	pubStr, ok := m["public-key"].(string)
	if !ok || pubStr == "" {
		return peer, fmt.Errorf("public-key is required")
	}
	pub, err := wgtypes.ParseKey(pubStr)
	if err != nil {
		return peer, fmt.Errorf("public-key: %w", err)
	}
	peer.PublicKey = pub

	if psStr, ok := m["preshared-key"].(string); ok && psStr != "" {
		ps, err := wgtypes.ParseKey(psStr)
		if err != nil {
			return peer, fmt.Errorf("preshared-key: %w", err)
		}
		peer.PresharedKey = ps
		peer.HasPresharedKey = true
	}

	if ep, ok := m["endpoint"].(map[string]any); ok {
		if ipStr, ok := ep["ip"].(string); ok {
			peer.EndpointIP = ipStr
		}
		if portStr, ok := ep["port"].(string); ok && portStr != "" {
			p, err := strconv.ParseUint(portStr, 10, 16)
			if err != nil {
				return peer, fmt.Errorf("endpoint port %q: %w", portStr, err)
			}
			peer.EndpointPort = uint16(p) //nolint:gosec // ParseUint bitSize=16 bounds value
		}
		// Endpoint requires both ip and port together.
		if peer.EndpointIP != "" && peer.EndpointPort == 0 {
			return peer, fmt.Errorf("endpoint has ip but no port")
		}
		if peer.EndpointIP == "" && peer.EndpointPort != 0 {
			return peer, fmt.Errorf("endpoint has port but no ip")
		}
	}

	peer.AllowedIPs = parseStringList(m, "allowed-ips")

	if kaStr, ok := m["persistent-keepalive"].(string); ok && kaStr != "" {
		ka, err := strconv.ParseUint(kaStr, 10, 16)
		if err != nil {
			return peer, fmt.Errorf("persistent-keepalive %q: %w", kaStr, err)
		}
		peer.PersistentKeepalive = uint16(ka) //nolint:gosec // ParseUint bitSize=16 bounds value
	}

	return peer, nil
}

func parseIfaceEntry(name string, m map[string]any) ifaceEntry {
	entry := ifaceEntry{Name: name}
	if m == nil {
		return entry
	}
	if mtu, ok := m["mtu"].(string); ok {
		entry.MTU, _ = strconv.Atoi(mtu)
	}
	if mac, ok := m["mac-address"].(string); ok {
		entry.MACAddress = mac
	}
	if _, ok := m["disable"]; ok {
		entry.Disable = true
	}
	entry.Units = parseUnits(m)
	return entry
}

func parseUnits(m map[string]any) []unitEntry {
	unitMap, ok := m["unit"].(map[string]any)
	if !ok {
		return nil
	}
	var units []unitEntry
	for idStr, v := range unitMap {
		id, _ := strconv.Atoi(idStr)
		um, _ := v.(map[string]any)
		u := unitEntry{ID: id}
		if um != nil {
			if vid, ok := um["vlan-id"].(string); ok {
				u.VLANID, _ = strconv.Atoi(vid)
			}
			if _, ok := um["disable"]; ok {
				u.Disable = true
			}
			if rp, ok := um["route-priority"].(string); ok {
				u.RoutePriority, _ = strconv.Atoi(rp)
			}
			u.Addresses = parseStringList(um, "address")
			u.SysctlProfiles = parseStringList(um, "sysctl-profile")
			u.IPv4 = parseIPv4Sysctl(um)
			u.IPv6 = parseIPv6Sysctl(um)
			if mirrorMap, ok := um["mirror"].(map[string]any); ok {
				u.MirrorIngress, _ = mirrorMap["ingress"].(string)
				u.MirrorEgress, _ = mirrorMap["egress"].(string)
			}
			u.DHCP = parseDHCPv4Config(um)
			u.DHCPv6 = parseDHCPv6Config(um)
		}
		units = append(units, u)
	}
	return units
}

func parseIPv4Sysctl(um map[string]any) *ipv4Sysctl {
	v4, ok := um["ipv4"].(map[string]any)
	if !ok {
		return nil
	}
	s := &ipv4Sysctl{}
	set := false
	if v, ok := v4["forwarding"].(string); ok {
		b := v == yangTrue
		s.Forwarding = &b
		set = true
	}
	if v, ok := v4["arp-filter"].(string); ok {
		b := v == yangTrue
		s.ArpFilter = &b
		set = true
	}
	if v, ok := v4["arp-accept"].(string); ok {
		b := v == yangTrue
		s.ArpAccept = &b
		set = true
	}
	if v, ok := v4["proxy-arp"].(string); ok {
		b := v == yangTrue
		s.ProxyARP = &b
		set = true
	}
	if v, ok := v4["arp-announce"].(string); ok {
		n, err := strconv.Atoi(v)
		if err == nil {
			s.ArpAnnounce = &n
			set = true
		}
	}
	if v, ok := v4["arp-ignore"].(string); ok {
		n, err := strconv.Atoi(v)
		if err == nil {
			s.ArpIgnore = &n
			set = true
		}
	}
	if v, ok := v4["rp-filter"].(string); ok {
		n, err := strconv.Atoi(v)
		if err == nil {
			s.RPFilter = &n
			set = true
		}
	}
	if !set {
		return nil
	}
	return s
}

func parseIPv6Sysctl(um map[string]any) *ipv6Sysctl {
	v6, ok := um["ipv6"].(map[string]any)
	if !ok {
		return nil
	}
	s := &ipv6Sysctl{}
	set := false
	if v, ok := v6["autoconf"].(string); ok {
		b := v == yangTrue
		s.Autoconf = &b
		set = true
	}
	if v, ok := v6["accept-ra"].(string); ok {
		n, err := strconv.Atoi(v)
		if err == nil {
			s.AcceptRA = &n
			set = true
		}
	}
	if v, ok := v6["forwarding"].(string); ok {
		b := v == yangTrue
		s.Forwarding = &b
		set = true
	}
	if !set {
		return nil
	}
	return s
}

// parseDHCPv4Config reads the "dhcp" container from a unit map and returns
// a dhcpUnitConfig if the container is present. Returns nil when absent.
func parseDHCPv4Config(um map[string]any) *dhcpUnitConfig {
	dm, ok := um["dhcp"].(map[string]any)
	if !ok {
		return nil
	}
	cfg := &dhcpUnitConfig{}
	if v, ok := dm["enabled"].(string); ok {
		cfg.Enabled = v == yangTrue
	}
	if v, ok := dm["hostname"].(string); ok {
		cfg.Hostname = v
	}
	if v, ok := dm["client-id"].(string); ok {
		cfg.ClientID = v
	}
	return cfg
}

// parseDHCPv6Config reads the "dhcpv6" container from a unit map and returns
// a dhcpv6UnitConfig if the container is present. Returns nil when absent.
func parseDHCPv6Config(um map[string]any) *dhcpv6UnitConfig {
	dm, ok := um["dhcpv6"].(map[string]any)
	if !ok {
		return nil
	}
	cfg := &dhcpv6UnitConfig{}
	if v, ok := dm["enabled"].(string); ok {
		cfg.Enabled = v == yangTrue
	}
	if pd, ok := dm["pd"].(map[string]any); ok {
		if v, ok := pd["length"].(string); ok {
			cfg.PDLength, _ = strconv.Atoi(v)
		}
	}
	if v, ok := dm["duid"].(string); ok {
		cfg.DUID = v
	}
	return cfg
}

func parseStringList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		return []string{val}
	}
	return nil
}

// desiredState builds a map of OS interface name -> desired addresses from config.
// Also returns the set of Ze-managed interface names (dummy, veth, bridge, VLAN)
// that should exist. Physical interfaces (ethernet) are never in the managed set.
func (cfg *ifaceConfig) desiredState() (addrs map[string]map[string]bool, managed map[string]bool) {
	addrs = make(map[string]map[string]bool)
	managed = make(map[string]bool)

	addIfaceAddrs := func(name string, units []unitEntry) {
		for i := range units {
			u := &units[i]
			if u.Disable {
				continue
			}
			osName := name
			if u.VLANID > 0 {
				osName = fmt.Sprintf("%s.%d", name, u.VLANID)
				managed[osName] = true
			}
			if addrs[osName] == nil {
				addrs[osName] = make(map[string]bool)
			}
			for _, a := range u.Addresses {
				addrs[osName][a] = true
			}
		}
	}

	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Veth {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if e.Disable {
			continue
		}
		managed[e.Name] = true
		addIfaceAddrs(e.Name, e.Units)
	}
	for _, e := range cfg.Ethernet {
		if e.Disable {
			continue
		}
		addIfaceAddrs(e.Name, e.Units)
	}
	if cfg.Loopback != nil {
		for i := range cfg.Loopback.Units {
			u := &cfg.Loopback.Units[i]
			if u.Disable {
				continue
			}
			if addrs["lo"] == nil {
				addrs["lo"] = make(map[string]bool)
			}
			for _, a := range u.Addresses {
				addrs["lo"][a] = true
			}
		}
	}

	return addrs, managed
}

// currentAddrSet builds a map of OS interface name -> set of current CIDR addresses.
func currentAddrSet(infos []InterfaceInfo) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	for i := range infos {
		if len(infos[i].Addresses) == 0 {
			continue
		}
		m := make(map[string]bool, len(infos[i].Addresses))
		for _, a := range infos[i].Addresses {
			cidr := fmt.Sprintf("%s/%d", a.Address, a.PrefixLength)
			m[cidr] = true
		}
		result[infos[i].Name] = m
	}
	return result
}

// currentIfaceSet builds a set of OS interface names by type.
func currentIfaceSet(infos []InterfaceInfo) map[string]string {
	result := make(map[string]string, len(infos))
	for i := range infos {
		result[infos[i].Name] = infos[i].Type
	}
	return result
}

// zeManageable returns true if the interface type is one Ze creates/deletes
// (not physical ethernet or loopback).
func zeManageable(linkType string) bool {
	switch linkType {
	case zeTypeDummy, zeTypeVeth, zeTypeBridge, zeTypeWireguard, "vlan":
		return true
	}
	return kernelTunnelKinds[linkType]
}

// indexTunnelSpecs returns a name -> Spec map for the previous config's
// tunnel entries. Used by applyConfig to detect Spec changes across reloads
// so that only changed tunnels are recreated. Returns an empty map if
// previous is nil (first apply).
func indexTunnelSpecs(previous *ifaceConfig) map[string]TunnelSpec {
	if previous == nil {
		return nil
	}
	specs := make(map[string]TunnelSpec, len(previous.Tunnel))
	for i := range previous.Tunnel {
		e := &previous.Tunnel[i]
		specs[e.Name] = e.Spec
	}
	return specs
}

// indexWireguardSpecs returns a name -> Spec map for the previous config's
// wireguard entries. Used by applyConfig to decide whether a reload needs
// to touch the kernel at all: if the Spec is unchanged, the netdev and
// peer set are already correct and ConfigureWireguardDevice can be skipped.
func indexWireguardSpecs(previous *ifaceConfig) map[string]WireguardSpec {
	if previous == nil {
		return nil
	}
	specs := make(map[string]WireguardSpec, len(previous.Wireguard))
	for i := range previous.Wireguard {
		e := &previous.Wireguard[i]
		specs[e.Name] = e.Spec
	}
	return specs
}

// wireguardSpecEqual reports whether two WireguardSpec values describe
// the same desired kernel state. Slice fields (Peers, AllowedIPs) prevent
// the direct == comparison that tunnelEntry uses, so the helper does a
// field-by-field walk that treats the Peer list as an unordered set
// keyed by public-key.
func wireguardSpecEqual(a, b WireguardSpec) bool {
	if a.Name != b.Name {
		return false
	}
	if a.PrivateKey != b.PrivateKey {
		return false
	}
	if a.ListenPortSet != b.ListenPortSet || a.ListenPort != b.ListenPort {
		return false
	}
	if a.FirewallMark != b.FirewallMark {
		return false
	}
	if len(a.Peers) != len(b.Peers) {
		return false
	}
	byKeyA := make(map[WireguardKey]*WireguardPeerSpec, len(a.Peers))
	for i := range a.Peers {
		byKeyA[a.Peers[i].PublicKey] = &a.Peers[i]
	}
	for i := range b.Peers {
		pa, ok := byKeyA[b.Peers[i].PublicKey]
		if !ok {
			return false
		}
		if !wireguardPeerEqual(*pa, b.Peers[i]) {
			return false
		}
	}
	return true
}

// wireguardPeerEqual reports whether two peer specs describe the same
// kernel-visible peer configuration. Name is excluded because it is a
// config-file label, not part of the kernel state.
func wireguardPeerEqual(a, b WireguardPeerSpec) bool {
	if a.PublicKey != b.PublicKey {
		return false
	}
	if a.HasPresharedKey != b.HasPresharedKey {
		return false
	}
	if a.HasPresharedKey && a.PresharedKey != b.PresharedKey {
		return false
	}
	if a.EndpointIP != b.EndpointIP || a.EndpointPort != b.EndpointPort {
		return false
	}
	if a.PersistentKeepalive != b.PersistentKeepalive {
		return false
	}
	if a.Disable != b.Disable {
		return false
	}
	if len(a.AllowedIPs) != len(b.AllowedIPs) {
		return false
	}
	setA := make(map[string]struct{}, len(a.AllowedIPs))
	for _, cidr := range a.AllowedIPs {
		setA[cidr] = struct{}{}
	}
	for _, cidr := range b.AllowedIPs {
		if _, ok := setA[cidr]; !ok {
			return false
		}
	}
	return true
}

// applyConfig applies the parsed interface config declaratively via the backend.
// 1. Creates missing Ze-managed interfaces (dummy, veth, tunnel, bridge, VLAN)
// 2. Sets properties (MTU, MAC, sysctl, mirror) on all configured interfaces
// 3. Adds missing addresses, removes extra addresses on configured interfaces
// 4. Deletes Ze-managed interfaces not in config.
//
// previous is the last successfully applied config, or nil on first apply.
// It is used to detect tunnels whose Spec changed across the reload, so that
// only those tunnels are deleted-and-recreated; tunnels with unchanged Spec
// stay up across SIGHUP, preserving any traffic flowing through them.
//
// Returns collected errors. Application continues past individual failures
// so that one bad interface doesn't block the rest.
func applyConfig(cfg, previous *ifaceConfig, b Backend) []error {
	log := loggerPtr.Load()
	var errs []error

	record := func(msg string, err error) {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}

	// Phase 1: Create missing interfaces.
	//
	// Order matters: tunnels are created BEFORE bridges so a bridge can
	// list a gretap tunnel in its `member` block on a fresh start. Bridges
	// are still created AFTER veths and dummies for the same reason.
	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		if err := b.CreateDummy(e.Name); err != nil {
			log.Debug("iface config: create dummy (may already exist)", "name", e.Name, "err", err)
		}
	}
	for _, e := range cfg.Veth {
		if e.Disable {
			continue
		}
		peer := e.Peer
		if peer == "" {
			peer = e.Name + "-peer"
		}
		if err := b.CreateVeth(e.Name, peer); err != nil {
			log.Debug("iface config: create veth (may already exist)", "name", e.Name, "err", err)
		}
	}
	previousTunnelSpecs := indexTunnelSpecs(previous)
	for i := range cfg.Tunnel {
		e := &cfg.Tunnel[i]
		if e.Disable {
			continue
		}
		prev, hadPrev := previousTunnelSpecs[e.Name]
		if hadPrev && prev == e.Spec {
			// Spec unchanged: keep the existing netdev. Phase 2 still
			// applies MTU/MAC and Phase 3 reconciles addresses, so any
			// non-Spec change still takes effect.
			continue
		}
		if hadPrev {
			// Spec changed: delete-then-create. Linux does not support
			// modifying most tunnel kinds in place; this matches VyOS's
			// behavior for gretap/ip6gretap.
			if err := b.DeleteInterface(e.Name); err != nil {
				log.Debug("iface config: delete tunnel before recreate",
					"name", e.Name, "err", err)
			}
		}
		if err := b.CreateTunnel(e.Spec); err != nil {
			log.Debug("iface config: create tunnel",
				"name", e.Name, "kind", e.Spec.Kind, "err", err)
		}
	}
	previousWireguardSpecs := indexWireguardSpecs(previous)
	for i := range cfg.Wireguard {
		e := &cfg.Wireguard[i]
		if e.Disable {
			continue
		}
		prev, hadPrev := previousWireguardSpecs[e.Name]
		if hadPrev && wireguardSpecEqual(prev, e.Spec) {
			// Spec unchanged: keep the existing netdev and peer set.
			// Phase 2 still applies MTU and Phase 3 reconciles addresses,
			// so any non-Spec change still takes effect.
			continue
		}
		if !hadPrev {
			// New wireguard interface: create the netdev first.
			if err := b.CreateWireguardDevice(e.Name); err != nil {
				// CreateWireguardDevice fails on "already exists" when
				// the previous-state tracker is stale. That is harmless.
				// A genuine failure (e.g. missing kernel module) means
				// we must skip ConfigureWireguardDevice -- there is
				// nothing to configure.
				if _, getErr := b.GetInterface(e.Name); getErr != nil {
					record(fmt.Sprintf("wireguard %s create", e.Name), err)
					continue
				}
				log.Debug("iface config: create wireguard (already exists)",
					"name", e.Name, "err", err)
			}
		}
		// Whether newly created or spec-changed, push the full desired
		// state. wgctrl handles key rotation, endpoint changes, peer
		// add/remove, keepalive updates in a single genetlink message
		// with ReplacePeers: true -- peers that are still in the spec
		// preserve their handshake state because the kernel matches
		// them by public key.
		if err := b.ConfigureWireguardDevice(e.Spec); err != nil {
			record(fmt.Sprintf("wireguard %s configure", e.Name), err)
		}
	}
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		if err := b.CreateBridge(e.Name); err != nil {
			log.Debug("iface config: create bridge (may already exist)", "name", e.Name, "err", err)
		}
		if err := b.BridgeSetSTP(e.Name, e.STP); err != nil {
			record(fmt.Sprintf("bridge %s stp", e.Name), err)
		}
		for _, member := range e.Members {
			if err := b.BridgeAddPort(e.Name, member); err != nil {
				record(fmt.Sprintf("bridge %s add port %s", e.Name, member), err)
			}
		}
	}

	// Phase 2: Set properties and create VLANs.
	allEntries := make([]ifaceEntry, 0, len(cfg.Ethernet)+len(cfg.Dummy)+len(cfg.Veth)+len(cfg.Bridge)+len(cfg.Tunnel)+len(cfg.Wireguard))
	allEntries = append(allEntries, cfg.Ethernet...)
	allEntries = append(allEntries, cfg.Dummy...)
	for _, e := range cfg.Veth {
		allEntries = append(allEntries, e.ifaceEntry)
	}
	for _, e := range cfg.Bridge {
		allEntries = append(allEntries, e.ifaceEntry)
	}
	for i := range cfg.Tunnel {
		allEntries = append(allEntries, cfg.Tunnel[i].ifaceEntry)
	}
	for i := range cfg.Wireguard {
		allEntries = append(allEntries, cfg.Wireguard[i].ifaceEntry)
	}

	for _, e := range allEntries {
		if e.Disable {
			continue
		}
		if e.MTU > 0 {
			if err := b.SetMTU(e.Name, e.MTU); err != nil {
				record(fmt.Sprintf("%s set mtu %d", e.Name, e.MTU), err)
			}
		}
		if e.MACAddress != "" {
			if err := b.SetMACAddress(e.Name, e.MACAddress); err != nil {
				record(fmt.Sprintf("%s set mac", e.Name), err)
			}
		}
		for i := range e.Units {
			u := &e.Units[i]
			if u.Disable {
				continue
			}
			osName := e.Name
			if u.VLANID > 0 {
				if err := b.CreateVLAN(e.Name, u.VLANID); err != nil {
					log.Debug("iface config: create vlan (may already exist)",
						"parent", e.Name, "vlan", u.VLANID, "err", err)
				}
				osName = fmt.Sprintf("%s.%d", e.Name, u.VLANID)
			}
			applySysctl(osName, *u)
			applySysctlProfiles(osName, u.SysctlProfiles)
			errs = append(errs, applyMirror(b, osName, *u)...)
		}
	}

	// Phase 2b: Apply sysctl for loopback units.
	if cfg.Loopback != nil {
		for i := range cfg.Loopback.Units {
			u := &cfg.Loopback.Units[i]
			if u.Disable {
				continue
			}
			applySysctl("lo", *u)
			applySysctlProfiles("lo", u.SysctlProfiles)
		}
	}

	// Phase 2c: Bring all non-disabled interfaces administratively UP.
	// Without this, DHCP and other protocols cannot send packets on the
	// interface even if the physical link is connected.
	for _, e := range allEntries {
		if e.Disable {
			continue
		}
		if err := b.SetAdminUp(e.Name); err != nil {
			log.Debug("iface config: admin up (may already be up)", "name", e.Name, "err", err)
		}
	}

	// Phase 3+4: Reconcile addresses (add missing, remove extra) and prune
	// non-config interfaces. If the backend is not yet ready (vpp handshake
	// still in flight), log + defer: reconcile will re-run once
	// vppevents.EventConnected fires. The additive-only fallback still
	// applies desired addresses so the daemon is usable before vpp comes up.
	reconcileErrs, deferred := reconcileOnReady(cfg, b)
	if deferred {
		log.Debug("iface reconcile deferred, backend not ready")
		addDesiredAddresses(cfg, b)
		return errs
	}
	errs = append(errs, reconcileErrs...)

	return errs
}

// reconcileOnReady runs Phase 3 (address reconcile) and Phase 4 (prune
// non-config interfaces) for cfg against backend b. It is called from
// applyConfig on every config apply; it is also called from the
// vppevents.EventConnected / EventReconnected handler in register.go so that
// reconciliation deferred at startup (because the vpp backend was not yet
// ready) fires once GoVPP is connected.
//
// Returns (nil, true) when the backend reports iface.ErrBackendNotReady --
// callers can retry later. Returns (errs, false) with any operational
// failures encountered during a normal reconcile.
func reconcileOnReady(cfg *ifaceConfig, b Backend) (errs []error, deferred bool) {
	log := loggerPtr.Load()
	desiredAddrs, managedNames := cfg.desiredState()

	currentInfos, err := b.ListInterfaces()
	if err != nil {
		if errors.Is(err, ErrBackendNotReady) {
			return nil, true
		}
		errs = append(errs, fmt.Errorf("list interfaces for reconciliation: %w", err))
		return errs, false
	}

	record := func(msg string, err error) {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}

	currentAddrs := currentAddrSet(currentInfos)

	// Add missing addresses on configured interfaces.
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range desired {
			if current != nil && current[addr] {
				continue
			}
			if err := b.AddAddress(osName, addr); err != nil {
				record(fmt.Sprintf("%s add address %s", osName, addr), err)
			}
		}
	}

	// Remove extra addresses on configured interfaces.
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range current {
			if desired[addr] {
				continue
			}
			if err := b.RemoveAddress(osName, addr); err != nil {
				record(fmt.Sprintf("%s remove stale address %s", osName, addr), err)
			} else {
				log.Info("iface config: removed stale address", "iface", osName, "addr", addr)
			}
		}
	}

	// Phase 4: Delete Ze-managed interfaces not in config.
	currentIfaces := currentIfaceSet(currentInfos)
	for name, linkType := range currentIfaces {
		if !zeManageable(linkType) {
			continue
		}
		if managedNames[name] {
			continue
		}
		if err := b.DeleteInterface(name); err != nil {
			record(fmt.Sprintf("delete %s (%s)", name, linkType), err)
		} else {
			log.Info("iface config: deleted interface not in config", "name", name, "type", linkType)
		}
	}

	return errs, false
}

// addDesiredAddresses adds every configured address without consulting the
// backend for current state. Used as the additive-only fallback when
// reconcileOnReady defers; the full reconcile fires later via
// vppevents.EventConnected.
func addDesiredAddresses(cfg *ifaceConfig, b Backend) {
	log := loggerPtr.Load()
	desiredAddrs, _ := cfg.desiredState()
	for osName, addrs := range desiredAddrs {
		for addr := range addrs {
			if err := b.AddAddress(osName, addr); err != nil {
				log.Debug("iface config: add address", "iface", osName, "addr", addr, "err", err)
			}
		}
	}
}

// reconcileOnVPPReady is invoked from the register.go EventBus handler when
// vppevents.EventConnected or EventReconnected fires. It looks up the
// currently-active config and, if one exists and the currently active
// backend is vpp, runs reconcileOnReady. It also retries b.StartMonitor so
// a monitor deferred at startup (backend not ready at OnConfigure time)
// installs as soon as the backend becomes live. No-op when no config has
// been applied yet, when the backend is still unregistered, or when the
// active backend is not vpp (the subscription is installed unconditionally
// for simplicity but should not mutate netlink state on a vpp lifecycle
// event, since vpp-ready has no meaning for the netlink backend).
//
// Exposed at package level so the register-time handler is easy to test
// without standing up the SDK event loop.
func reconcileOnVPPReady(activeCfg *atomic.Pointer[ifaceConfig]) {
	log := loggerPtr.Load()
	cfg := activeCfg.Load()
	if cfg == nil {
		return
	}
	// Guard against firing against a non-vpp backend. A reload that flipped
	// the backend from vpp to netlink would otherwise call netlink's
	// StartMonitor on every EventConnected / EventReconnected, leaking a
	// fresh monitor goroutine per event (netlink's StartMonitor is not
	// idempotent -- see ifacenetlink/monitor_linux.go:364).
	if cfg.Backend != vppBackendName {
		return
	}
	b := GetBackend()
	if b == nil {
		return
	}

	// Retry the monitor install if it was deferred at OnConfigure time.
	// StartMonitor is idempotent (a second call after a first success is
	// a no-op), so a call on every vpp event is safe.
	if eb := GetEventBus(); eb != nil {
		if err := b.StartMonitor(eb); err != nil {
			if errors.Is(err, ErrBackendNotReady) {
				log.Debug("iface monitor still deferred after vpp event")
				return
			}
			log.Warn("iface monitor start on vpp ready", "err", err)
		}
	}

	errs, deferred := reconcileOnReady(cfg, b)
	if deferred {
		log.Debug("iface reconcile still deferred after vpp event")
		return
	}
	for _, e := range errs {
		log.Warn("iface reconcile on vpp ready", "err", e)
	}
}

// applySysctl emits per-interface sysctl defaults on the EventBus.
// The sysctl plugin receives these and writes them to the kernel.
// Only settings explicitly configured (non-nil) are emitted.
func applySysctl(osName string, u unitEntry) {
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()

	emit := func(key, value string) {
		payload, _ := json.Marshal(struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		}{Key: key, Value: value, Source: "interface"})
		if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
			log.Debug("iface: sysctl emit failed", "key", key, "err", err)
		}
	}

	boolVal := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}

	if s := u.IPv4; s != nil {
		if s.Forwarding != nil {
			emit("net.ipv4.conf."+osName+".forwarding", boolVal(*s.Forwarding))
		}
		if s.ArpFilter != nil {
			emit("net.ipv4.conf."+osName+".arp_filter", boolVal(*s.ArpFilter))
		}
		if s.ArpAccept != nil {
			emit("net.ipv4.conf."+osName+".arp_accept", boolVal(*s.ArpAccept))
		}
		if s.ProxyARP != nil {
			emit("net.ipv4.conf."+osName+".proxy_arp", boolVal(*s.ProxyARP))
		}
		if s.ArpAnnounce != nil {
			emit("net.ipv4.conf."+osName+".arp_announce", strconv.Itoa(*s.ArpAnnounce))
		}
		if s.ArpIgnore != nil {
			emit("net.ipv4.conf."+osName+".arp_ignore", strconv.Itoa(*s.ArpIgnore))
		}
		if s.RPFilter != nil {
			emit("net.ipv4.conf."+osName+".rp_filter", strconv.Itoa(*s.RPFilter))
		}
	}
	if s := u.IPv6; s != nil {
		if s.Autoconf != nil {
			emit("net.ipv6.conf."+osName+".autoconf", boolVal(*s.Autoconf))
		}
		if s.AcceptRA != nil {
			emit("net.ipv6.conf."+osName+".accept_ra", strconv.Itoa(*s.AcceptRA))
		}
		if s.Forwarding != nil {
			emit("net.ipv6.conf."+osName+".forwarding", boolVal(*s.Forwarding))
		}
	}
}

// applySysctlProfiles resolves named profiles and emits their settings as
// sysctl defaults via EventBus. Emits clear-profile-defaults first to remove
// stale keys from a previous config cycle, then emits each profile's settings
// in order (last wins on key overlap).
func applySysctlProfiles(osName string, profiles []string) {
	if len(profiles) == 0 {
		return
	}
	eb := GetEventBus()
	if eb == nil {
		return
	}
	log := loggerPtr.Load()

	// Clear stale profile defaults for this interface before re-emitting.
	clearPayload, _ := json.Marshal(struct {
		Interface string `json:"interface"`
	}{Interface: osName})
	if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventClearProfileDefaults, string(clearPayload)); err != nil {
		log.Debug("iface: clear-profile-defaults emit failed", "iface", osName, "err", err)
	}

	for _, name := range profiles {
		p, ok := sysctlreg.LookupProfile(name)
		if !ok {
			log.Warn("iface: unknown sysctl profile", "profile", name, "iface", osName)
			continue
		}
		resolved := sysctlreg.ResolveProfileSettings(p.Settings, osName)
		for _, s := range resolved {
			payload, _ := json.Marshal(struct {
				Key    string `json:"key"`
				Value  string `json:"value"`
				Source string `json:"source"`
			}{Key: s.Key, Value: s.Value, Source: "profile:" + name})
			if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDefault, string(payload)); err != nil {
				log.Debug("iface: profile sysctl emit failed", "key", s.Key, "profile", name, "err", err)
			}
		}
	}
}

// applyMirror configures traffic mirroring on an interface from unit config.
// Only applied when at least one of ingress/egress destination is configured.
// Returns errors for mirror operations that failed.
func applyMirror(b Backend, osName string, u unitEntry) []error {
	if u.MirrorIngress == "" && u.MirrorEgress == "" {
		return nil
	}

	var errs []error
	fail := func(what string, err error) {
		loggerPtr.Load().Warn("iface config: "+what, "iface", osName, "err", err)
		errs = append(errs, fmt.Errorf("%s %s: %w", osName, what, err))
	}

	ingress := u.MirrorIngress != ""
	egress := u.MirrorEgress != ""

	if ingress && egress && u.MirrorIngress == u.MirrorEgress {
		if err := b.SetupMirror(osName, u.MirrorIngress, true, true); err != nil {
			fail("mirror", err)
		}
		return errs
	}
	if ingress {
		if err := b.SetupMirror(osName, u.MirrorIngress, true, false); err != nil {
			fail("mirror ingress", err)
		}
	}
	if egress {
		if err := b.SetupMirror(osName, u.MirrorEgress, false, true); err != nil {
			fail("mirror egress", err)
		}
	}
	return errs
}
