// Design: docs/features/interfaces.md -- Interface config parsing and application
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface used for application
// Related: register.go -- OnConfigure calls applyConfig
// Related: config_apply.go -- reconciliation and application
// Related: config_sysctl.go -- sysctl and mirror application

package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errMissingEncapsulationBlock           = errors.New("missing encapsulation block")
	errEncapsulationBlockHasNoKindSelected = errors.New("encapsulation block has no kind selected")
	errLocalIpAndLocalInterfaceAre         = errors.New("local ip and local interface are mutually exclusive")
	errLocalIpOrLocalInterfaceRequired     = errors.New("local ip or local interface required")
	errEmptyWireguardEntry                 = errors.New("empty wireguard entry")
	errPrivateKeyIsRequired                = errors.New("private-key is required")
	errEmptyXFRMEntry                      = errors.New("empty xfrm entry")
	errXFRMIfIDRequired                    = errors.New("if-id is required for xfrm interfaces")
	errXFRMIfIDZero                        = errors.New("if-id must be non-zero (0 means unset)")

	errEmptyPeerEntry         = errors.New("empty peer entry")
	errPublicKeyIsRequired    = errors.New("public-key is required")
	errEndpointHasIpButNoPort = errors.New("endpoint has ip but no port")
	errEndpointHasPortButNoIp = errors.New("endpoint has port but no ip")
)

// yangTrue is the string representation of boolean true in YANG config JSON.
const yangTrue = "true"

// ifaceConfig is the parsed representation of the interface config section.
type ifaceConfig struct {
	Backend         string
	DHCPAuto        bool // auto-discover first ethernet for DHCP
	Ethernet        []ifaceEntry
	Dummy           []ifaceEntry
	Veth            []vethEntry
	Bridge          []bridgeEntry
	Tunnel          []tunnelEntry
	Wireguard       []wireguardEntry
	XFRM            []xfrmEntry
	PPPoEClient     []pppoeClientEntry
	Loopback        *loopbackEntry
	previousManaged map[string]bool // runtime-only: names Ze managed before this apply
}

// osNameMap returns the logical-name -> os-name overrides for every ethernet
// entry that sets the os-name selector. Ethernet is the matched physical kind:
// it binds to a pre-existing kernel device, so an operator may alias a logical
// name to a different OS device name. Created kinds (dummy/veth/bridge/tunnel/
// wireguard/xfrm) are created by Ze under the logical name, so they are never
// aliased here. Entries without os-name are omitted; the resolver defaults
// them to the logical name (so every name == os-name config resolves
// unchanged).
func (c *ifaceConfig) osNameMap() map[string]string {
	if c == nil {
		return nil
	}
	out := make(map[string]string)
	for i := range c.Ethernet {
		e := &c.Ethernet[i]
		if e.OSName != "" && e.OSName != e.Name {
			out[e.Name] = e.OSName
		}
	}
	return out
}

// permMACMap returns the logical-name -> match-MAC selector for every ethernet
// entry that sets mac/match. Same scope as osNameMap: ethernet is the matched
// physical kind, so only it can be bound to a pre-existing kernel device by its
// hardware MAC. The resolver matches the device's permanent (factory) MAC when
// it reports one and falls back to the current MAC otherwise, so the value is
// passed through verbatim (normalization happens in the resolver). Entries
// without mac/match are omitted.
func (c *ifaceConfig) permMACMap() map[string]string {
	if c == nil {
		return nil
	}
	out := make(map[string]string)
	for i := range c.Ethernet {
		e := &c.Ethernet[i]
		if e.MatchMAC != "" {
			out[e.Name] = e.MatchMAC
		}
	}
	return out
}

// tunnelEntry represents a configured tunnel interface. The Spec carries
// the encapsulation kind plus per-kind parameters; the embedded ifaceEntry
// carries the shared physical and unit fields (mtu, mac, addresses).
type tunnelEntry struct {
	ifaceEntry
	Spec TunnelSpec
}

// xfrmEntry represents a configured XFRM interface. The Spec carries the
// if_id and optional parent device; the embedded ifaceEntry carries the
// shared common and unit fields (mtu, addresses).
type xfrmEntry struct {
	ifaceEntry
	Spec XFRMSpec
}

// wireguardEntry represents a configured wireguard interface. The Spec
// carries the wireguard-specific parameters and peer list; the embedded
// ifaceEntry carries the shared common and unit fields inherited via the
// interface-common and interface-unit YANG groupings.
type wireguardEntry struct {
	ifaceEntry
	Spec WireguardSpec
}

// pppoeClientEntry represents a configured PPPoE client interface.
// The PPP session is created dynamically via PPPoE discovery and LCP/NCP
// negotiation; addresses are assigned by the server via IPCP/IPv6CP.
type pppoeClientEntry struct {
	Name            string
	SourceInterface string
	Username        string
	AuthSecret      string //nolint:gosec // not a hardcoded credential; parsed from ze:sensitive YANG leaf
	ServiceName     string
	ACName          string
	NoDefaultRoute  bool
	MTU             int
	Disable         bool
	// RoutePriority is the kernel route metric of the default route the
	// session installs, defaultLearnedRouteMetric unless the operator wrote
	// the leaf. A PPPoE default is learned from the access concentrator, so
	// it ranks with the other learned defaults rather than at metric 0.
	RoutePriority int
}

// ifaceEntry represents a configured interface (ethernet or dummy).
type ifaceEntry struct {
	Name       string
	OSName     string // os-name selector: kernel device this logical name maps to (empty = name itself)
	MatchMAC   string // mac/match selector: bind to the kernel device carrying this MAC (empty = no MAC match)
	MTU        int
	MACAddress string
	Disable    bool
	Offload    *offloadConfig
	Units      []unitEntry
}

// offloadConfig holds per-interface ethtool offload and sysfs steering
// settings. Pointer fields: nil = not configured (preserve OS default),
// non-nil = set explicitly. Matches ipv4Settings/ipv6Settings three-state pattern.
type offloadConfig struct {
	GRO         *bool
	GSO         *bool
	SG          *bool
	TSO         *bool
	LRO         *bool
	HWTCOffload *bool
	RPS         *bool
	RFS         *bool
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

// defaultLearnedRouteMetric is the kernel route metric of a default route the
// interface layer learns from the network: a DHCPv4 lease, a router
// advertisement, or a PPPoE session. It repeats the default the route-priority
// leaves declare in yang/ze-iface-conf.yang, because the config tree delivers
// only what the operator wrote: an absent leaf arrives as an absent key, never
// as its schema default.
//
// 254 ranks a learned default below every route an operator or a routing
// protocol produces, the way rib/admin-distance
// (internal/component/sysrib/yang/ze-rib-conf.yang) ranks connected 0, static
// 10, ebgp 20, ospf 110, isis 115 and ibgp 200. It is also the administrative
// distance a Cisco IOS DHCP client gives the default route it learns, which is
// the same ranking decision on another vendor. The interface layer programs the kernel
// directly and never consults that table, so this constant is the iface side
// of the same ordering, not a reader of it.
//
// The number matters beyond ranking: ze installs a learned default with
// RouteReplace, which matches on destination, metric and table and takes no
// protocol, so a learned default at metric 0 OVERWRITES an operator's static
// default at metric 0, gateway included. Two defaults at different metrics
// coexist instead, and the lower one wins.
const defaultLearnedRouteMetric = 254

// maxRoutePriority is the largest metric a route-priority leaf accepts. It
// repeats the range those leaves declare in yang/ze-iface-conf.yang: 1024 below
// the uint32 ceiling, so the link-down metric (base + deprioritizedMetric) still
// fits the netlink attribute. It is uint64 so the comparison holds its value on
// a build whose int is 32 bits, where the constant does not fit an int at all.
const maxRoutePriority uint64 = 4294966271

// unitEntry represents a logical unit on an interface.
type unitEntry struct {
	Label     string
	VLANID    int
	Addresses []string
	Disable   bool
	// RoutePriority is the kernel route metric for the default routes this
	// unit learns (DHCP, PPPoE). It holds defaultLearnedRouteMetric unless
	// the operator wrote the leaf, so a learned default never lands on the
	// metric an operator's static default occupies.
	RoutePriority int
	// RoutePrioritySet says the operator wrote route-priority on this unit.
	// It separates "the operator asked ze to own the default routes of this
	// interface" from "this is the metric a learned route takes", which the
	// value alone can no longer express now that its default is non-zero.
	RoutePrioritySet bool
	SysctlProfiles   []string
	IPv4             *ipv4Settings
	IPv6             *ipv6Settings
	MirrorIngress    string // destination interface name, empty = not configured
	MirrorEgress     string
	MPLSEnable       *bool // enable MPLS label input (net.mpls.conf.<iface>.input)
	// 802.1p QoS maps (IEEE 802.1Q PCP, 3 bits). nil = not configured,
	// no netlink attribute sent. Keys and values are 0-7.
	IngressQoSMap map[uint32]uint32 // received PCP -> internal priority
	EgressQoSMap  map[uint32]uint32 // internal priority -> transmitted PCP
}

// dhcpUnitConfig holds DHCPv4 client settings parsed from the YANG
// "dhcp" container inside the ipv4 family container.
type dhcpUnitConfig struct {
	Enabled  bool
	Hostname string
	ClientID string
}

// dhcpv6UnitConfig holds DHCPv6 client settings parsed from the YANG
// "dhcpv6" container inside the ipv6 family container.
type dhcpv6UnitConfig struct {
	Enabled  bool
	PDLength int // 0 = not set (server decides)
	DUID     string
}

type rpfMode int

const (
	rpfModeDisable rpfMode = 0
	rpfModeStrict  rpfMode = 1
	rpfModeLoose   rpfMode = 2
)

func parseRPFMode(s string) (rpfMode, bool) {
	switch s {
	case "disable":
		return rpfModeDisable, true
	case "strict":
		return rpfModeStrict, true
	case "loose":
		return rpfModeLoose, true
	default:
		return 0, false
	}
}

func (m rpfMode) rpfSysctlValue() int {
	return int(m)
}

// ipv4Settings holds per-interface IPv4 configuration: addresses and sysctl knobs.
// Pointer fields: nil = not configured (leave OS default), non-nil = set.
type ipv4Settings struct {
	Addresses   []string
	Forwarding  *bool
	ArpFilter   *bool
	ArpAccept   *bool
	ProxyARP    *bool
	ArpAnnounce *int
	ArpIgnore   *int
	RPFCheck    *rpfMode
	DHCP        *dhcpUnitConfig
}

// ipv6Settings holds per-interface IPv6 configuration: addresses and sysctl knobs.
type ipv6Settings struct {
	Addresses  []string
	DHCPv6     *dhcpv6UnitConfig
	Autoconf   *bool
	AcceptRA   *int
	Forwarding *bool
	RPFCheck   *rpfMode
	// RouterAdvertisement is the send side: what this unit advertises to hosts
	// on the link. AcceptRA above is the receive side. See config_ra.go.
	RouterAdvertisement *raUnitConfig
}

// parseIfaceSections finds the "interface" section and parses it. Returns a
// default config if no interface section is present. Parse errors propagate
// to the caller so OnConfigVerify can reject malformed input rather than
// silently apply a default.
func parseIfaceSections(sections []sdk.ConfigSection) (*ifaceConfig, error) {
	for _, s := range sections {
		if s.Root != configRootInterface {
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

	ifaceMap, ok := root[configRootInterface].(map[string]any)
	if !ok {
		return &ifaceConfig{Backend: defaultBackendName}, nil
	}

	cfg := &ifaceConfig{
		Backend: defaultBackendName,
	}

	if b, ok := ifaceMap["backend"].(string); ok && b != "" {
		cfg.Backend = b
	}

	if v, ok := ifaceMap["dhcp-auto"].(string); ok {
		cfg.DHCPAuto = v == yangTrue
	}

	if ethMap, ok := ifaceMap["ethernet"].(map[string]any); ok {
		for name, v := range ethMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("ethernet: %w", err)
			}
			m, _ := v.(map[string]any)
			entry, err := parseIfaceEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("ethernet %q: %w", name, err)
			}
			cfg.Ethernet = append(cfg.Ethernet, entry)
		}
	}

	if dummyMap, ok := ifaceMap["dummy"].(map[string]any); ok {
		for name, v := range dummyMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("dummy: %w", err)
			}
			m, _ := v.(map[string]any)
			entry, err := parseIfaceEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("dummy %q: %w", name, err)
			}
			cfg.Dummy = append(cfg.Dummy, entry)
		}
	}

	if vethMap, ok := ifaceMap["veth"].(map[string]any); ok {
		for name, v := range vethMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("veth: %w", err)
			}
			m, _ := v.(map[string]any)
			iface, err := parseIfaceEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("veth %q: %w", name, err)
			}
			entry := vethEntry{ifaceEntry: iface}
			if peer, ok := m["peer"].(string); ok {
				if err := ValidateIfaceName(peer); err != nil {
					return nil, fmt.Errorf("veth %q peer: %w", name, err)
				}
				entry.Peer = peer
			}
			cfg.Veth = append(cfg.Veth, entry)
		}
	}

	if brMap, ok := ifaceMap["bridge"].(map[string]any); ok {
		for name, v := range brMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("bridge: %w", err)
			}
			m, _ := v.(map[string]any)
			iface, err := parseIfaceEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("bridge %q: %w", name, err)
			}
			entry := bridgeEntry{ifaceEntry: iface}
			if stp, ok := m["stp"].(string); ok {
				entry.STP = stp == yangTrue
			}
			// configvalue.LeafList, not a []any assertion: a leaf-list carrying ONE
			// value arrives as a bare string, and the assertion dropped it. A
			// bridge with a single member enslaved nothing, with no error.
			entry.Members = configvalue.LeafList(m["member"])
			cfg.Bridge = append(cfg.Bridge, entry)
		}
	}

	if tunMap, ok := ifaceMap["tunnel"].(map[string]any); ok {
		for name, v := range tunMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("tunnel: %w", err)
			}
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
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("wireguard: %w", err)
			}
			m, _ := v.(map[string]any)
			entry, err := parseWireguardEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("wireguard %q: %w", name, err)
			}
			cfg.Wireguard = append(cfg.Wireguard, entry)
		}
	}

	if xfrmMap, ok := ifaceMap["xfrm"].(map[string]any); ok {
		for name, v := range xfrmMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("xfrm: %w", err)
			}
			m, _ := v.(map[string]any)
			entry, err := parseXFRMEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("xfrm %q: %w", name, err)
			}
			cfg.XFRM = append(cfg.XFRM, entry)
		}
	}

	if pppoeMap, ok := ifaceMap["pppoe-client"].(map[string]any); ok {
		for name, v := range pppoeMap {
			if err := ValidateIfaceName(name); err != nil {
				return nil, fmt.Errorf("pppoe-client: %w", err)
			}
			m, _ := v.(map[string]any)
			entry, err := parsePPPoEClientEntry(name, m)
			if err != nil {
				return nil, fmt.Errorf("pppoe-client %q: %w", name, err)
			}
			cfg.PPPoEClient = append(cfg.PPPoEClient, entry)
		}
	}

	if loMap, ok := ifaceMap["loopback"].(map[string]any); ok {
		lo := &loopbackEntry{}
		var err error
		lo.Units, err = parseUnits(loMap, "")
		if err != nil {
			return nil, fmt.Errorf("loopback: %w", err)
		}
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
	iface, err := parseIfaceEntry(name, m)
	if err != nil {
		return tunnelEntry{}, err
	}
	entry := tunnelEntry{ifaceEntry: iface}
	// MAC address for tunnels comes from inside the encapsulation case
	// container (gretap/ip6gretap only), not from the list level. Clear
	// any list-level mac/address that parseIfaceEntry may have read.
	entry.MACAddress = ""
	entry.Spec.Name = name

	encMap, ok := m["encapsulation"].(map[string]any)
	if !ok {
		return entry, errMissingEncapsulationBlock
	}

	var matchedKind TunnelKind
	var matchedCase map[string]any
	for caseName, raw := range encMap {
		k, ok := parseTunnelKind(caseName)
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
		return entry, errEncapsulationBlockHasNoKindSelected
	}
	entry.Spec.Kind = matchedKind
	if err := parseTunnelLeaves(&entry.Spec, matchedCase); err != nil {
		return entry, err
	}
	// MAC address lives inside the case container for bridgeable kinds
	// (gretap/ip6gretap). L3 kinds have no mac/address leaf in YANG.
	if matchedKind.isBridgeable() {
		if macC, ok := matchedCase["mac"].(map[string]any); ok {
			if mac, ok := macC["address"].(string); ok {
				entry.MACAddress = mac
			}
		}
	}
	if entry.Spec.LocalAddress != "" && entry.Spec.LocalInterface != "" {
		return entry, errLocalIpAndLocalInterfaceAre
	}
	if entry.Spec.LocalAddress == "" && entry.Spec.LocalInterface == "" {
		return entry, errLocalIpOrLocalInterfaceRequired
	}
	if !matchedKind.isBridgeable() {
		for i := range entry.Units {
			if entry.Units[i].VLANID > 0 {
				return entry, fmt.Errorf("vlan-id units are not supported on %s tunnels (only gretap and ip6gretap carry Ethernet frames)", matchedKind)
			}
		}
	}
	return entry, nil
}

// parseXFRMEntry walks the JSON tree for one xfrm list entry and produces
// an xfrmEntry. if-id is mandatory and must be non-zero.
func parseXFRMEntry(name string, m map[string]any) (xfrmEntry, error) {
	iface, err := parseIfaceEntry(name, m)
	if err != nil {
		return xfrmEntry{}, err
	}
	entry := xfrmEntry{ifaceEntry: iface}
	entry.MACAddress = ""
	entry.Spec.Name = name

	if m == nil {
		return entry, errEmptyXFRMEntry
	}

	ifIDStr, ok := m["if-id"].(string)
	if !ok || ifIDStr == "" {
		return entry, errXFRMIfIDRequired
	}
	ifID, err := strconv.ParseUint(ifIDStr, 10, 32)
	if err != nil {
		return entry, fmt.Errorf("if-id %q: %w", ifIDStr, err)
	}
	if ifID == 0 {
		return entry, errXFRMIfIDZero
	}
	entry.Spec.IfID = uint32(ifID) //nolint:gosec // ParseUint bitSize=32 bounds value

	if dev, ok := m["dev"].(string); ok && dev != "" {
		if err := ValidateIfaceName(dev); err != nil {
			return entry, fmt.Errorf("dev: %w", err)
		}
		entry.Spec.PhysicalDev = dev
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
	if v, ok := caseMap["vni"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return fmt.Errorf("vni %q: %w", v, err)
		}
		spec.VNI = uint32(n)
		spec.VNISet = true
	}
	if v, ok := caseMap["port"].(string); ok && v != "" {
		n, err := strconv.ParseUint(v, 10, 16)
		if err != nil {
			return fmt.Errorf("port %q: %w", v, err)
		}
		spec.Port = uint16(n)
		spec.PortSet = true
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
	iface, err := parseIfaceEntry(name, m)
	if err != nil {
		return wireguardEntry{}, err
	}
	entry := wireguardEntry{ifaceEntry: iface}
	// Wireguard uses interface-common (no mac/address leaf). Clear any
	// list-level mac/address that parseIfaceEntry may have read from a
	// hand-edited config. Same defense-in-depth as parseTunnelEntry.
	entry.MACAddress = ""
	entry.Spec.Name = name

	if m == nil {
		return entry, errEmptyWireguardEntry
	}

	privStr, ok := m["private-key"].(string)
	if !ok || privStr == "" {
		return entry, errPrivateKeyIsRequired
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
		return peer, errEmptyPeerEntry
	}

	if _, ok := m["disable"]; ok {
		peer.Disable = true
	}

	pubStr, ok := m["public-key"].(string)
	if !ok || pubStr == "" {
		return peer, errPublicKeyIsRequired
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
			return peer, errEndpointHasIpButNoPort
		}
		if peer.EndpointIP == "" && peer.EndpointPort != 0 {
			return peer, errEndpointHasPortButNoIp
		}
	}

	peer.AllowedIPs = configvalue.LeafList(m["allowed-ips"])

	if kaStr, ok := m["persistent-keepalive"].(string); ok && kaStr != "" {
		ka, err := strconv.ParseUint(kaStr, 10, 16)
		if err != nil {
			return peer, fmt.Errorf("persistent-keepalive %q: %w", kaStr, err)
		}
		peer.PersistentKeepalive = uint16(ka) //nolint:gosec // ParseUint bitSize=16 bounds value
	}

	return peer, nil
}

// RFC 2516 Section 4: PPPoE adds 6 bytes of header on top of Ethernet,
// leaving 1494 bytes for PPP. The 2-byte PPP protocol field further
// reduces the IP MTU to 1492 for a standard 1500-byte Ethernet link.
const pppoeDefaultMTU = 1492

func parsePPPoEClientEntry(name string, m map[string]any) (pppoeClientEntry, error) {
	entry := pppoeClientEntry{Name: name, MTU: pppoeDefaultMTU, RoutePriority: defaultLearnedRouteMetric}
	if m == nil {
		return entry, errors.New("empty pppoe-client entry")
	}

	src, ok := m["source-interface"].(string)
	if !ok || src == "" {
		return entry, errors.New("source-interface is required")
	}
	if err := ValidateIfaceName(src); err != nil {
		return entry, fmt.Errorf("source-interface: %w", err)
	}
	entry.SourceInterface = src

	authMap, _ := m["authentication"].(map[string]any)
	if authMap == nil {
		return entry, errors.New("authentication block is required")
	}
	user, ok := authMap["username"].(string)
	if !ok || user == "" {
		return entry, errors.New("authentication username is required")
	}
	entry.Username = user
	pass, ok := authMap["password"].(string)
	if !ok || pass == "" {
		return entry, errors.New("authentication password is required")
	}
	entry.AuthSecret = pass

	if sn, ok := m["service-name"].(string); ok {
		entry.ServiceName = sn
	}
	if ac, ok := m["ac-name"].(string); ok {
		entry.ACName = ac
	}
	if _, ok := m["no-default-route"]; ok {
		entry.NoDefaultRoute = true
	}
	if _, ok := m["disable"]; ok {
		entry.Disable = true
	}
	if mtuStr, ok := m["mtu"].(string); ok && mtuStr != "" {
		mtu, err := strconv.Atoi(mtuStr)
		if err != nil {
			return entry, fmt.Errorf("mtu %q: %w", mtuStr, err)
		}
		if mtu < 68 || mtu > pppoeDefaultMTU {
			return entry, fmt.Errorf("mtu %d out of range 68..%d", mtu, pppoeDefaultMTU)
		}
		entry.MTU = mtu
	}
	if rpStr, ok := m["route-priority"].(string); ok && rpStr != "" {
		rp, err := strconv.ParseUint(rpStr, 10, 32)
		if err != nil {
			return entry, fmt.Errorf("route-priority %q: %w", rpStr, err)
		}
		if rp > maxRoutePriority {
			return entry, fmt.Errorf("route-priority %d out of range 0..%d", rp, maxRoutePriority)
		}
		entry.RoutePriority = int(rp)
	}

	return entry, nil
}

func parseIfaceEntry(name string, m map[string]any) (ifaceEntry, error) {
	entry := ifaceEntry{Name: name}
	if m == nil {
		return entry, nil
	}
	if osn, ok := m["os-name"].(string); ok {
		entry.OSName = osn
	}
	if mtu, ok := m["mtu"].(string); ok {
		entry.MTU, _ = strconv.Atoi(mtu)
	}
	if macC, ok := m["mac"].(map[string]any); ok {
		if mac, ok := macC["address"].(string); ok {
			entry.MACAddress = mac
		}
		if match, ok := macC["match"].(string); ok {
			entry.MatchMAC = match
		}
	}
	if _, ok := m["disable"]; ok {
		entry.Disable = true
	}
	entry.Offload = parseOffloadConfig(m)
	parentCoS, _ := m["class-of-service"].(string)
	var err error
	entry.Units, err = parseUnits(m, parentCoS)
	if err != nil {
		return entry, fmt.Errorf("%s: %w", name, err)
	}
	return entry, nil
}

func parseUnits(m map[string]any, parentCoS string) ([]unitEntry, error) {
	unitMap, ok := m["unit"].(map[string]any)
	if !ok {
		return nil, nil //nolint:nilnil // no unit container means no units, not an error
	}
	var units []unitEntry
	for name, v := range unitMap {
		if err := validateUnitName(name); err != nil {
			return nil, fmt.Errorf("unit %q: %w", name, err)
		}
		um, _ := v.(map[string]any)
		u := unitEntry{Label: name, RoutePriority: defaultLearnedRouteMetric}
		if um != nil {
			if vid, ok := um["vlan-id"].(string); ok {
				u.VLANID, _ = strconv.Atoi(vid)
			}
			if _, ok := um["disable"]; ok {
				u.Disable = true
			}
			// Same bounds and same refusals as parsePPPoEClientEntry: the two
			// leaves carry one number to one netlink attribute. Dropping the
			// error here put every unparsable value at metric 0, which is the
			// metric a plain static route takes.
			if rp, ok := um["route-priority"].(string); ok && rp != "" {
				priority, err := strconv.ParseUint(rp, 10, 32)
				if err != nil {
					return nil, fmt.Errorf("unit %q: route-priority %q: %w", name, rp, err)
				}
				if priority > maxRoutePriority {
					return nil, fmt.Errorf("unit %q: route-priority %d out of range 0..%d", name, priority, maxRoutePriority)
				}
				u.RoutePriority = int(priority)
				u.RoutePrioritySet = true
			}
			u.SysctlProfiles = configvalue.LeafList(um["sysctl-profile"])
			var err error
			u.IPv4, err = parseIPv4Settings(um)
			if err != nil {
				return nil, fmt.Errorf("unit %q: %w", name, err)
			}
			u.IPv6, err = parseIPv6Settings(um)
			if err != nil {
				return nil, fmt.Errorf("unit %q: %w", name, err)
			}

			// Merge per-family addresses into flat list for the apply path.
			if u.IPv4 != nil {
				u.Addresses = append(u.Addresses, u.IPv4.Addresses...)
			}
			if u.IPv6 != nil {
				u.Addresses = append(u.Addresses, u.IPv6.Addresses...)
			}

			if mirrorMap, ok := um["mirror"].(map[string]any); ok {
				u.MirrorIngress, _ = mirrorMap["ingress"].(string)
				u.MirrorEgress, _ = mirrorMap["egress"].(string)
			}

			if mplsMap, ok := um["mpls"].(map[string]any); ok {
				if v, ok := mplsMap["enable"].(string); ok {
					b := v == yangTrue
					u.MPLSEnable = &b
				}
			}

			u.IngressQoSMap, err = parseQoSMap(um, "ingress-qos-map", "priority")
			if err != nil {
				return nil, fmt.Errorf("unit %q: %w", name, err)
			}
			u.EgressQoSMap, err = parseQoSMap(um, "egress-qos-map", "pcp")
			if err != nil {
				return nil, fmt.Errorf("unit %q: %w", name, err)
			}

			unitCoS, _ := um["class-of-service"].(string)
			ingress, egress, cosErr := cos.Resolve(parentCoS, unitCoS, u.IngressQoSMap != nil || u.EgressQoSMap != nil)
			if cosErr != nil {
				return nil, fmt.Errorf("unit %q: %w", name, cosErr)
			}
			if ingress != nil || egress != nil {
				u.IngressQoSMap = ingress
				u.EgressQoSMap = egress
			}

			if (u.IngressQoSMap != nil || u.EgressQoSMap != nil) && u.VLANID <= 0 {
				return nil, fmt.Errorf("unit %q: qos maps require vlan-id", name)
			}
		}
		units = append(units, u)
	}
	return units, nil
}

// validateUniqueMatchMAC rejects two ethernet interfaces that select the same
// kernel device by mac/match: both would bind to one NIC, an ambiguous config
// the YANG `unique "mac/match"` documents but the schema validator does not
// enforce on its own. MACs are compared in canonical form so "AA:.." and
// "aa:.." collide.
func validateUniqueMatchMAC(cfg *ifaceConfig) error {
	seen := make(map[string]string)
	for i := range cfg.Ethernet {
		e := &cfg.Ethernet[i]
		if e.MatchMAC == "" {
			continue
		}
		key := normalizeMAC(e.MatchMAC)
		if prev, dup := seen[key]; dup {
			return fmt.Errorf("ethernet %q and %q both match MAC %s; a hardware MAC selects at most one device", prev, e.Name, key)
		}
		seen[key] = e.Name
	}
	return nil
}

// validateVPPQoSMaps checks that all QoS maps in the config are compatible
// with VPP's QoS pipeline. VPP's "qos record" copies the VLAN PCP value
// verbatim into internal QoS bits, so ingress maps must be identity
// (pcp == priority for every entry). Egress maps are arbitrary (handled
// by "qos egress-map"). Called only when the backend is VPP.
func validateVPPQoSMaps(cfg *ifaceConfig) error {
	check := func(iface string, units []unitEntry) error {
		for i := range units {
			for pcp, prio := range units[i].IngressQoSMap {
				if pcp != prio {
					return fmt.Errorf(
						"%s unit %q: VPP ingress QoS maps must be identity (pcp == priority); got pcp %d -> priority %d. "+
							"VPP qos-record copies PCP verbatim; use the netlink backend for remapped ingress",
						iface, units[i].Label, pcp, prio,
					)
				}
			}
		}
		return nil
	}
	for _, e := range cfg.Ethernet {
		if err := check(e.Name, e.Units); err != nil {
			return err
		}
	}
	for _, e := range cfg.Dummy {
		if err := check(e.Name, e.Units); err != nil {
			return err
		}
	}
	for _, e := range cfg.Veth {
		if err := check(e.Name, e.Units); err != nil {
			return err
		}
	}
	for _, e := range cfg.Bridge {
		if err := check(e.Name, e.Units); err != nil {
			return err
		}
	}
	return nil
}

// parseQoSMap reads an 802.1p QoS map list from the unit container. The YANG
// list key (PCP for ingress, priority for egress) arrives as the JSON object
// key; valueLeaf names the single value leaf inside each entry. Both sides
// are 3-bit 802.1p values (0-7). Returns nil when the list is absent OR
// present but empty, so the backend never emits an empty netlink mapping
// attribute (the vendor lib serializes any non-nil map).
func parseQoSMap(um map[string]any, listName, valueLeaf string) (map[uint32]uint32, error) {
	listMap, ok := um[listName].(map[string]any)
	if !ok || len(listMap) == 0 {
		return nil, nil //nolint:nilnil // absent or empty list means unconfigured, not an error
	}
	// At most 8 entries can be valid (keys are 0-7); do not size the map
	// from the raw JSON key count.
	m := make(map[uint32]uint32, min(len(listMap), 8))
	for keyStr, v := range listMap {
		key, err := parsePCPValue(keyStr)
		if err != nil {
			return nil, fmt.Errorf("%s key %q: %w", listName, keyStr, err)
		}
		if _, dup := m[key]; dup {
			// "06" and "6" are distinct JSON keys but the same 802.1p value;
			// silent last-write-wins would be nondeterministic.
			return nil, fmt.Errorf("%s key %q: duplicate entry for value %d", listName, keyStr, key)
		}
		entry, _ := v.(map[string]any)
		valStr, ok := entry[valueLeaf].(string)
		if !ok {
			return nil, fmt.Errorf("%s entry %q: missing %s", listName, keyStr, valueLeaf)
		}
		val, err := parsePCPValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("%s entry %q: %s %q: %w", listName, keyStr, valueLeaf, valStr, err)
		}
		m[key] = val
	}
	return m, nil
}

// parsePCPValue parses a 3-bit 802.1p value (PCP or internal priority).
// IEEE 802.1Q: the PCP field of the TCI is 3 bits, so 0-7.
func parsePCPValue(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, errors.New("not a number")
	}
	if n > 7 {
		return 0, errors.New("out of range (0-7)")
	}
	return uint32(n), nil
}

func parseIPv4Settings(um map[string]any) (*ipv4Settings, error) {
	v4, ok := um["ipv4"].(map[string]any)
	if !ok {
		return nil, nil //nolint:nilnil // absent container means unconfigured, not an error
	}
	s := &ipv4Settings{}
	set := false
	s.Addresses = configvalue.LeafList(v4["address"])
	for _, a := range s.Addresses {
		p, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("ipv4 address %q: %w", a, err)
		}
		if !p.Addr().Is4() {
			return nil, fmt.Errorf("ipv4 address %q: not an IPv4 address", a)
		}
	}
	if len(s.Addresses) > 0 {
		set = true
	}
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
	if v, ok := v4["rpf-check"].(string); ok {
		m, valid := parseRPFMode(v)
		if !valid {
			return nil, fmt.Errorf("ipv4 rpf-check: invalid value %q (expected strict, loose, or disable)", v)
		}
		s.RPFCheck = &m
		set = true
	} else if v, ok := v4["rp-filter"].(string); ok {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 2 {
			return nil, fmt.Errorf("ipv4 rp-filter: invalid value %q (expected 0, 1, or 2)", v)
		}
		m := rpfMode(n)
		s.RPFCheck = &m
		set = true
		if log := loggerPtr.Load(); log != nil {
			log.Warn("iface: rp-filter is deprecated, use rpf-check (strict|loose|disable)")
		}
	}
	s.DHCP = parseDHCPv4Config(v4)
	if s.DHCP != nil {
		set = true
	}
	if !set {
		return nil, nil //nolint:nilnil // no ipv4 knobs configured, not an error
	}
	return s, nil
}

func parseIPv6Settings(um map[string]any) (*ipv6Settings, error) {
	v6, ok := um["ipv6"].(map[string]any)
	if !ok {
		return nil, nil //nolint:nilnil // absent container means unconfigured, not an error
	}
	s := &ipv6Settings{}
	set := false
	s.Addresses = configvalue.LeafList(v6["address"])
	for _, a := range s.Addresses {
		p, err := netip.ParsePrefix(a)
		if err != nil {
			return nil, fmt.Errorf("ipv6 address %q: %w", a, err)
		}
		if !p.Addr().Is6() || p.Addr().Is4In6() {
			return nil, fmt.Errorf("ipv6 address %q: not an IPv6 address", a)
		}
	}
	if len(s.Addresses) > 0 {
		set = true
	}
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
	if v, ok := v6["rpf-check"].(string); ok {
		m, valid := parseRPFMode(v)
		if !valid {
			return nil, fmt.Errorf("ipv6 rpf-check: invalid value %q (expected strict, loose, or disable)", v)
		}
		s.RPFCheck = &m
		set = true
	}
	s.DHCPv6 = parseDHCPv6Config(v6)
	if s.DHCPv6 != nil {
		set = true
	}
	ra, err := parseRAConfig(v6)
	if err != nil {
		return nil, err
	}
	s.RouterAdvertisement = ra
	if ra != nil {
		set = true
	}
	if !set {
		return nil, nil //nolint:nilnil // no ipv6 knobs configured, not an error
	}
	return s, nil
}

func parseOffloadConfig(m map[string]any) *offloadConfig {
	om, ok := m["offload"].(map[string]any)
	if !ok {
		return nil
	}
	o := &offloadConfig{}
	set := false
	parseBool := func(key string) *bool {
		v, ok := om[key].(string)
		if !ok {
			return nil
		}
		set = true
		b := v == yangTrue
		return &b
	}
	o.GRO = parseBool("gro")
	o.GSO = parseBool("gso")
	o.SG = parseBool("sg")
	o.TSO = parseBool("tso")
	o.LRO = parseBool("lro")
	o.HWTCOffload = parseBool("hw-tc-offload")
	o.RPS = parseBool("rps")
	o.RFS = parseBool("rfs")
	if !set {
		return nil
	}
	return o
}

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
