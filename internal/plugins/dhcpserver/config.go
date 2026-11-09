// Design: docs/architecture/provisioning/dhcp-server.md -- DHCP server config parsing

package dhcpserver

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/dnssource"
)

const defaultLeaseTimeSec = 86400

// leafTrue is the YANG boolean spelling a config leaf holds when it is set.
const leafTrue = "true"

var (
	errRangeStartOutsideSubnet = errors.New("range start outside subnet")
	errRangeStopOutsideSubnet  = errors.New("range stop outside subnet")
	errRangeStartAfterStop     = errors.New("range start after stop")
	errTooManyRanges           = errors.New("too many ranges (max 10)")
	errStaticIPOutsideSubnet   = errors.New("static-mapping IP outside subnet")
)

const maxRangesPerSubnet = 10

type pxeConfig struct {
	Enabled       bool
	TFTPServer    netip.Addr
	BootfileBIOS  string
	BootfileUEFI  string
	BootScriptURL string
}

type serverConfig struct {
	Enabled          bool
	ListenInterfaces []string
	SharedNetworks   []sharedNetwork
	PXE              pxeConfig
}

type sharedNetwork struct {
	Name    string
	Subnets []subnetConfig
}

type addressRange struct {
	Name  string
	Start netip.Addr
	Stop  netip.Addr
}

type subnetConfig struct {
	Prefix        netip.Prefix
	Ranges        []addressRange
	LeaseTimeSec  uint32
	DefaultRouter netip.Addr
	// ServerIdentifier is the operator's choice of option 54 for this subnet,
	// invalid when the leaf is unset. Whether the listen device holds it is a
	// runtime fact, checked for each reply (segment.serverID).
	ServerIdentifier netip.Addr
	DNSServers       []netip.Addr
	// Sources is the subnet's option 6 list: the written dns-source, or
	// subnetDNSDefault while the container is absent.
	Sources        dnssource.List
	DomainName     string
	StaticMappings []staticMapping
}

type staticMapping struct {
	Name string
	MAC  net.HardwareAddr
	IP   netip.Addr
}

func parseConfig(data string) (serverConfig, error) {
	var cfg serverConfig

	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return cfg, fmt.Errorf("dhcp-server config: unmarshal: %w", err)
	}

	svcMap, ok := root["service"].(map[string]any)
	if !ok {
		return cfg, nil
	}
	dhcpMap, ok := svcMap["dhcp-server"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	if v, ok := dhcpMap["enabled"].(string); ok {
		cfg.Enabled = v == leafTrue
	}

	switch v := dhcpMap["listen-interface"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				cfg.ListenInterfaces = append(cfg.ListenInterfaces, s)
			}
		}
	case string:
		if v != "" {
			cfg.ListenInterfaces = append(cfg.ListenInterfaces, v)
		}
	}

	pxe, err := parsePXEConfig(dhcpMap)
	if err != nil {
		return cfg, err
	}
	cfg.PXE = pxe

	snMap, ok := dhcpMap["shared-network"].(map[string]any)
	if !ok {
		return cfg, nil
	}

	for name, snv := range snMap {
		snData, ok := snv.(map[string]any)
		if !ok {
			continue
		}
		sn, err := parseSharedNetwork(name, snData)
		if err != nil {
			return cfg, fmt.Errorf("shared-network %q: %w", name, err)
		}
		cfg.SharedNetworks = append(cfg.SharedNetworks, sn)
	}
	sort.Slice(cfg.SharedNetworks, func(i, j int) bool {
		return cfg.SharedNetworks[i].Name < cfg.SharedNetworks[j].Name
	})

	return cfg, nil
}

func parseSharedNetwork(name string, data map[string]any) (sharedNetwork, error) {
	sn := sharedNetwork{Name: name}

	subnetMap, ok := data["subnet"].(map[string]any)
	if !ok {
		return sn, nil
	}

	for prefixStr, sv := range subnetMap {
		subData, ok := sv.(map[string]any)
		if !ok {
			continue
		}
		sub, err := parseSubnet(prefixStr, subData)
		if err != nil {
			return sn, fmt.Errorf("subnet %q: %w", prefixStr, err)
		}
		sn.Subnets = append(sn.Subnets, sub)
	}
	sort.Slice(sn.Subnets, func(i, j int) bool {
		return sn.Subnets[i].Prefix.String() < sn.Subnets[j].Prefix.String()
	})

	return sn, nil
}

func parseSubnet(prefixStr string, data map[string]any) (subnetConfig, error) {
	var sub subnetConfig

	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return sub, fmt.Errorf("invalid prefix: %w", err)
	}
	// This server speaks RFC 2131 only, so a subnet is IPv4. Every address
	// derived from the prefix reaches buildReply, which narrows it to four
	// bytes. Refuse the family at the boundary so no
	// client packet can reach a four-byte narrowing of an IPv6 address.
	if !prefix.Addr().Is4() {
		return sub, fmt.Errorf("subnet prefix %q: must be IPv4, this server does not speak DHCPv6", prefixStr)
	}
	sub.Prefix = prefix
	sub.LeaseTimeSec = defaultLeaseTimeSec

	if rangeData, ok := data["range"].(map[string]any); ok {
		ranges, err := parseRanges(rangeData, prefix)
		if err != nil {
			return sub, err
		}
		sub.Ranges = ranges
	}

	if v, ok := data["lease-time"].(string); ok {
		sec, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return sub, fmt.Errorf("lease-time: %w", err)
		}
		if sec < 60 || sec > 604800 {
			return sub, fmt.Errorf("lease-time: %d out of range 60..604800", sec)
		}
		sub.LeaseTimeSec = uint32(sec)
	}

	if v, ok := data["default-router"].(string); ok && v != "" {
		addr, err := parseIPv4Option("default-router", v)
		if err != nil {
			return sub, err
		}
		sub.DefaultRouter = addr
	}

	if v, ok := data["server-identifier"].(string); ok && v != "" {
		addr, err := parseServerIdentifier(prefix, v)
		if err != nil {
			return sub, err
		}
		sub.ServerIdentifier = addr
	}

	const maxDNSServers = 63
	switch v := data["dns-server"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				addr, err := parseIPv4Option("dns-server", s)
				if err != nil {
					return sub, err
				}
				sub.DNSServers = append(sub.DNSServers, addr)
			}
		}
	case string:
		if v != "" {
			addr, err := parseIPv4Option("dns-server", v)
			if err != nil {
				return sub, err
			}
			sub.DNSServers = append(sub.DNSServers, addr)
		}
	}
	if len(sub.DNSServers) > maxDNSServers {
		return sub, fmt.Errorf("dns-server: count %d exceeds DHCP option maximum %d", len(sub.DNSServers), maxDNSServers)
	}

	sources, err := parseSubnetSources(data)
	if err != nil {
		return sub, err
	}
	sub.Sources = sources

	if v, ok := data["domain-name"].(string); ok {
		if len(v) > 255 {
			return sub, fmt.Errorf("domain-name: length %d exceeds DHCP option maximum 255", len(v))
		}
		sub.DomainName = v
	}

	if smMap, ok := data["static-mapping"].(map[string]any); ok {
		for smName, smv := range smMap {
			smData, ok := smv.(map[string]any)
			if !ok {
				continue
			}
			sm, err := parseStaticMapping(smName, smData, prefix)
			if err != nil {
				return sub, fmt.Errorf("static-mapping %q: %w", smName, err)
			}
			sub.StaticMappings = append(sub.StaticMappings, sm)
		}
		sort.Slice(sub.StaticMappings, func(i, j int) bool {
			return sub.StaticMappings[i].Name < sub.StaticMappings[j].Name
		})
	}

	return sub, nil
}

func parseStaticMapping(name string, data map[string]any, prefix netip.Prefix) (staticMapping, error) {
	var sm staticMapping
	sm.Name = name

	macStr, ok := data["mac-address"].(string)
	if !ok || macStr == "" {
		return sm, errors.New("missing mac-address")
	}
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return sm, fmt.Errorf("mac-address: %w", err)
	}
	sm.MAC = mac

	ipStr, ok := data["ip-address"].(string)
	if !ok || ipStr == "" {
		return sm, errors.New("missing ip-address")
	}
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return sm, fmt.Errorf("ip-address: %w", err)
	}
	if !prefix.Contains(addr) {
		return sm, errStaticIPOutsideSubnet
	}
	sm.IP = addr

	return sm, nil
}

func parseRanges(data map[string]any, prefix netip.Prefix) ([]addressRange, error) {
	if v, hasStart := data["start"]; hasStart {
		if _, isStr := v.(string); isStr {
			r, err := parseSingleRange("default", data, prefix)
			if err != nil {
				return nil, err
			}
			return []addressRange{r}, nil
		}
	}

	var ranges []addressRange
	for name, v := range data {
		rm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		r, err := parseSingleRange(name, rm, prefix)
		if err != nil {
			return nil, fmt.Errorf("range %q: %w", name, err)
		}
		ranges = append(ranges, r)
	}

	if len(ranges) > maxRangesPerSubnet {
		return nil, errTooManyRanges
	}

	sort.Slice(ranges, func(i, j int) bool {
		return addrToUint32(ranges[i].Start) < addrToUint32(ranges[j].Start)
	})

	for i := 1; i < len(ranges); i++ {
		if addrToUint32(ranges[i].Start) <= addrToUint32(ranges[i-1].Stop) {
			return nil, fmt.Errorf("ranges %q and %q overlap", ranges[i-1].Name, ranges[i].Name)
		}
	}

	return ranges, nil
}

func parseSingleRange(name string, data map[string]any, prefix netip.Prefix) (addressRange, error) {
	var r addressRange
	r.Name = name

	startStr, ok := data["start"].(string)
	if !ok || startStr == "" {
		return r, fmt.Errorf("range %q: missing start", name)
	}
	start, err := netip.ParseAddr(startStr)
	if err != nil {
		return r, fmt.Errorf("range start: %w", err)
	}
	if !prefix.Contains(start) {
		return r, errRangeStartOutsideSubnet
	}
	r.Start = start

	stopStr, ok := data["stop"].(string)
	if !ok || stopStr == "" {
		return r, fmt.Errorf("range %q: missing stop", name)
	}
	stop, err := netip.ParseAddr(stopStr)
	if err != nil {
		return r, fmt.Errorf("range stop: %w", err)
	}
	if !prefix.Contains(stop) {
		return r, errRangeStopOutsideSubnet
	}
	r.Stop = stop

	if addrToUint32(r.Start) > addrToUint32(r.Stop) {
		return r, errRangeStartAfterStop
	}

	return r, nil
}

// parseIPv4Option parses one address leaf of a DHCPv4 subnet, naming the leaf
// in every error so an operator is told which one is wrong. The family is
// checked here, at the config boundary, rather than where the option is
// written: buildReply and addrToUint32 both narrow an address to four bytes
// with As4, which panics on an IPv6 address, and on the reply path a DHCP
// client would be the one to trigger it. An IPv4-mapped IPv6 spelling is
// refused with the rest, which is what IPv4AddressValidator accepts for every
// other module.
func parseIPv4Option(field, value string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("%s: %w", field, err)
	}
	if !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("%s %q: must be an IPv4 address", field, value)
	}
	return addr, nil
}

// parseServerIdentifier checks the server-identifier leaf of the subnet
// prefix: an IPv4 unicast address that is neither the network nor the
// broadcast address of prefix. It need not fall inside prefix: a subnet served
// only through a relay holds no address of the server's, and the server is
// reached at an address of its own segment.
//
// RFC 2131 Section 4.1: "To accommodate potentially incomplete network
// connectivity, a server MUST choose an address as a 'server identifier'
// that, to the best of the server's knowledge, is reachable from the client."
// No client reaches the server at a network, broadcast, multicast, loopback,
// link-local or unspecified address, so a commit refuses each.
func parseServerIdentifier(prefix netip.Prefix, value string) (netip.Addr, error) {
	addr, err := parseIPv4Option("server-identifier", value)
	if err != nil {
		return netip.Addr{}, err
	}
	const fix = "Fix: set server-identifier to an address the listen interface holds, or delete the leaf " +
		"and Ze uses the interface's address in the subnet"
	if !addr.IsGlobalUnicast() {
		return netip.Addr{}, fmt.Errorf("subnet %s: server-identifier %s is not a unicast host address, "+
			"so no client can reach the server at it.\n%s", prefix, addr, fix)
	}
	if prefix.Contains(addr) && !isHostAddress(prefix, addr) {
		return netip.Addr{}, fmt.Errorf("subnet %s: server-identifier %s is the subnet's network or broadcast address, "+
			"which names no host, so no client can reach the server at it.\n%s", prefix, addr, fix)
	}
	return addr, nil
}

// addrToUint32 narrows an IPv4 address to its numeric form. It panics on an
// IPv6 address, so every caller MUST have taken its address through
// parseIPv4Option or through a prefix this package has already checked.
func addrToUint32(a netip.Addr) uint32 {
	b := a.As4()
	return binary.BigEndian.Uint32(b[:])
}

func parsePXEConfig(dhcpMap map[string]any) (pxeConfig, error) {
	var pxe pxeConfig

	pxeMap, ok := dhcpMap["pxe"].(map[string]any)
	if !ok {
		return pxe, nil
	}

	if v, ok := pxeMap["enabled"].(string); ok {
		pxe.Enabled = v == leafTrue
	}

	if v, ok := pxeMap["tftp-server"].(string); ok && v != "" {
		addr, err := parseIPv4Option("pxe tftp-server", v)
		if err != nil {
			return pxe, err
		}
		pxe.TFTPServer = addr
	}

	// Each bootfile is sent in option 67, whose one length octet counts at most
	// 255 octets of data (RFC 2132 Section 2).
	if v, ok := pxeMap["bootfile-bios"].(string); ok {
		if len(v) > 255 {
			return pxe, fmt.Errorf("pxe bootfile-bios: length %d exceeds DHCP option maximum 255", len(v))
		}
		pxe.BootfileBIOS = v
	}

	if v, ok := pxeMap["bootfile-uefi"].(string); ok {
		if len(v) > 255 {
			return pxe, fmt.Errorf("pxe bootfile-uefi: length %d exceeds DHCP option maximum 255", len(v))
		}
		pxe.BootfileUEFI = v
	}

	if v, ok := pxeMap["boot-script-url"].(string); ok && v != "" {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			return pxe, fmt.Errorf("pxe boot-script-url: must be an http:// or https:// URL")
		}
		if len(v) > 255 {
			return pxe, fmt.Errorf("pxe boot-script-url: length %d exceeds DHCP option maximum 255", len(v))
		}
		pxe.BootScriptURL = v
	}

	if pxe.Enabled {
		if !pxe.TFTPServer.IsValid() {
			return pxe, fmt.Errorf("pxe: tftp-server is required when enabled")
		}
		if pxe.BootfileBIOS == "" {
			return pxe, fmt.Errorf("pxe: bootfile-bios is required when enabled")
		}
		if pxe.BootfileUEFI == "" {
			return pxe, fmt.Errorf("pxe: bootfile-uefi is required when enabled")
		}
	}

	return pxe, nil
}

// subnetDNSSourcePath is the YANG path of a subnet's option 6 list, named in
// its refusals and in every address the selection drops.
const subnetDNSSourcePath = "service dhcp-server shared-network subnet dns-source"

// subnetDNSDefault is a subnet's built-in option 6 list (D-5): the dns-server
// entries when any are set, else the IPv4 resolvers Ze learned. The servers
// are handed to LAN clients, so a loopback address is never one of them.
var subnetDNSDefault = dnssource.StatedDefault(subnetDNSSourcePath, dnssource.FamilyIPv4, dnssource.AudienceHandedOn,
	dnssource.ModeFirstUsable, dnssource.ClassConfigured, dnssource.ClassReceived)

// subnetDNSRefusals are the classes a subnet's list cannot name.
var subnetDNSRefusals = []dnssource.Refusal{
	{
		Class: dnssource.ClassRadius,
		Why:   "radius names the resolvers a RADIUS server gives one PPP subscriber, and a DHCP client is not a subscriber",
		Fix: "delete " + subnetDNSSourcePath + " source radius; the subnet takes configured (dns-server), " +
			"received and self",
	},
}

// parseSubnetSources reads a subnet's dns-source container, or answers the
// built-in default when it is absent.
func parseSubnetSources(data map[string]any) (dnssource.List, error) {
	body, ok := data["dns-source"].(map[string]any)
	if !ok {
		return subnetDNSDefault, nil
	}
	written, err := dnssource.ReadWritten(subnetDNSSourcePath, body)
	if err != nil {
		return dnssource.List{}, err
	}
	return dnssource.NewWrittenList(subnetDNSSourcePath, dnssource.FamilyIPv4, dnssource.AudienceHandedOn, written,
		subnetDNSRefusals...)
}
