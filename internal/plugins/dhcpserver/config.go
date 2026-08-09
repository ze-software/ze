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
)

const defaultLeaseTimeSec = 86400

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
	Prefix         netip.Prefix
	Ranges         []addressRange
	LeaseTimeSec   uint32
	DefaultRouter  netip.Addr
	DNSServers     []netip.Addr
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
		cfg.Enabled = v == "true"
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
		addr, err := netip.ParseAddr(v)
		if err != nil {
			return sub, fmt.Errorf("default-router: %w", err)
		}
		sub.DefaultRouter = addr
	}

	const maxDNSServers = 63
	switch v := data["dns-server"].(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				addr, err := netip.ParseAddr(s)
				if err != nil {
					return sub, fmt.Errorf("dns-server: %w", err)
				}
				sub.DNSServers = append(sub.DNSServers, addr)
			}
		}
	case string:
		if v != "" {
			addr, err := netip.ParseAddr(v)
			if err != nil {
				return sub, fmt.Errorf("dns-server: %w", err)
			}
			sub.DNSServers = append(sub.DNSServers, addr)
		}
	}
	if len(sub.DNSServers) > maxDNSServers {
		return sub, fmt.Errorf("dns-server: count %d exceeds DHCP option maximum %d", len(sub.DNSServers), maxDNSServers)
	}

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
		pxe.Enabled = v == "true"
	}

	if v, ok := pxeMap["tftp-server"].(string); ok && v != "" {
		addr, err := netip.ParseAddr(v)
		if err != nil {
			return pxe, fmt.Errorf("pxe tftp-server: %w", err)
		}
		if !addr.Is4() {
			return pxe, fmt.Errorf("pxe tftp-server: must be IPv4 address")
		}
		pxe.TFTPServer = addr
	}

	if v, ok := pxeMap["bootfile-bios"].(string); ok {
		pxe.BootfileBIOS = v
	}

	if v, ok := pxeMap["bootfile-uefi"].(string); ok {
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
