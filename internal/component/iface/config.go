// Design: docs/features/interfaces.md -- Interface config parsing and application
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface used for application
// Related: register.go -- OnConfigure calls applyConfig

package iface

import (
	"encoding/json"
	"fmt"
	"strconv"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// yangTrue is the string representation of boolean true in YANG config JSON.
const yangTrue = "true"

// ifaceConfig is the parsed representation of the interface config section.
type ifaceConfig struct {
	Backend  string
	Ethernet []ifaceEntry
	Dummy    []ifaceEntry
	Veth     []vethEntry
	Bridge   []bridgeEntry
	Loopback *loopbackEntry
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
	ID            int
	VLANID        int
	Addresses     []string
	Disable       bool
	IPv4          *ipv4Sysctl
	IPv6          *ipv6Sysctl
	MirrorIngress string // destination interface name, empty = not configured
	MirrorEgress  string
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

// parseIfaceSections finds the "interface" section and parses it.
// Returns a default config if no interface section is present.
func parseIfaceSections(sections []sdk.ConfigSection) *ifaceConfig {
	for _, s := range sections {
		if s.Root != "interface" {
			continue
		}
		cfg, err := parseIfaceConfig(s.Data)
		if err != nil {
			loggerPtr.Load().Warn("iface config: parse failed, using defaults", "err", err)
			return &ifaceConfig{Backend: defaultBackendName}
		}
		return cfg
	}
	return &ifaceConfig{Backend: defaultBackendName}
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
		Backend: defaultBackendName,
	}

	if b, ok := ifaceMap["backend"].(string); ok && b != "" {
		cfg.Backend = b
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

	if loMap, ok := ifaceMap["loopback"].(map[string]any); ok {
		lo := &loopbackEntry{}
		lo.Units = parseUnits(loMap)
		cfg.Loopback = lo
	}

	return cfg, nil
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
			u.Addresses = parseStringList(um, "address")
			u.IPv4 = parseIPv4Sysctl(um)
			u.IPv6 = parseIPv6Sysctl(um)
			if mirrorMap, ok := um["mirror"].(map[string]any); ok {
				u.MirrorIngress, _ = mirrorMap["ingress"].(string)
				u.MirrorEgress, _ = mirrorMap["egress"].(string)
			}
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
		for _, u := range units {
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
	for _, e := range cfg.Ethernet {
		if e.Disable {
			continue
		}
		addIfaceAddrs(e.Name, e.Units)
	}
	if cfg.Loopback != nil {
		for _, u := range cfg.Loopback.Units {
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
	case zeTypeDummy, zeTypeVeth, zeTypeBridge, "vlan":
		return true
	}
	return false
}

// applyConfig applies the parsed interface config declaratively via the backend.
// 1. Creates missing Ze-managed interfaces (dummy, veth, bridge, VLAN)
// 2. Sets properties (MTU, MAC, sysctl, mirror) on all configured interfaces
// 3. Adds missing addresses, removes extra addresses on configured interfaces
// 4. Deletes Ze-managed interfaces not in config.
//
// Returns collected errors. Application continues past individual failures
// so that one bad interface doesn't block the rest.
func applyConfig(cfg *ifaceConfig, b Backend) []error {
	log := loggerPtr.Load()
	var errs []error

	record := func(msg string, err error) {
		log.Warn(msg, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", msg, err))
	}

	// Phase 1: Create missing interfaces.
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
	allEntries := make([]ifaceEntry, 0, len(cfg.Ethernet)+len(cfg.Dummy)+len(cfg.Veth)+len(cfg.Bridge))
	allEntries = append(allEntries, cfg.Ethernet...)
	allEntries = append(allEntries, cfg.Dummy...)
	for _, e := range cfg.Veth {
		allEntries = append(allEntries, e.ifaceEntry)
	}
	for _, e := range cfg.Bridge {
		allEntries = append(allEntries, e.ifaceEntry)
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
		for _, u := range e.Units {
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
			errs = append(errs, applySysctl(b, osName, u)...)
			errs = append(errs, applyMirror(b, osName, u)...)
		}
	}

	// Phase 2b: Apply sysctl for loopback units.
	if cfg.Loopback != nil {
		for _, u := range cfg.Loopback.Units {
			if u.Disable {
				continue
			}
			errs = append(errs, applySysctl(b, "lo", u)...)
		}
	}

	// Phase 3: Reconcile addresses (add missing, remove extra).
	desiredAddrs, managedNames := cfg.desiredState()

	currentInfos, err := b.ListInterfaces()
	if err != nil {
		record("list interfaces for reconciliation", err)
		// Fall back to additive-only: add all desired addresses.
		for osName, addrs := range desiredAddrs {
			for addr := range addrs {
				if err := b.AddAddress(osName, addr); err != nil {
					log.Debug("iface config: add address", "iface", osName, "addr", addr, "err", err)
				}
			}
		}
		return errs
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

	return errs
}

// applySysctl applies per-interface IPv4/IPv6 sysctl settings from a unit config.
// Only settings explicitly configured (non-nil) are applied; OS defaults are left alone.
// Returns errors for settings that failed to apply.
func applySysctl(b Backend, osName string, u unitEntry) []error {
	var errs []error
	fail := func(what string, err error) {
		loggerPtr.Load().Warn("iface config: "+what, "iface", osName, "err", err)
		errs = append(errs, fmt.Errorf("%s %s: %w", osName, what, err))
	}
	if s := u.IPv4; s != nil {
		if s.Forwarding != nil {
			if err := b.SetIPv4Forwarding(osName, *s.Forwarding); err != nil {
				fail("ipv4 forwarding", err)
			}
		}
		if s.ArpFilter != nil {
			if err := b.SetIPv4ArpFilter(osName, *s.ArpFilter); err != nil {
				fail("ipv4 arp-filter", err)
			}
		}
		if s.ArpAccept != nil {
			if err := b.SetIPv4ArpAccept(osName, *s.ArpAccept); err != nil {
				fail("ipv4 arp-accept", err)
			}
		}
		if s.ProxyARP != nil {
			if err := b.SetIPv4ProxyARP(osName, *s.ProxyARP); err != nil {
				fail("ipv4 proxy-arp", err)
			}
		}
		if s.ArpAnnounce != nil {
			if err := b.SetIPv4ArpAnnounce(osName, *s.ArpAnnounce); err != nil {
				fail("ipv4 arp-announce", err)
			}
		}
		if s.ArpIgnore != nil {
			if err := b.SetIPv4ArpIgnore(osName, *s.ArpIgnore); err != nil {
				fail("ipv4 arp-ignore", err)
			}
		}
		if s.RPFilter != nil {
			if err := b.SetIPv4RPFilter(osName, *s.RPFilter); err != nil {
				fail("ipv4 rp-filter", err)
			}
		}
	}
	if s := u.IPv6; s != nil {
		if s.Autoconf != nil {
			if err := b.SetIPv6Autoconf(osName, *s.Autoconf); err != nil {
				fail("ipv6 autoconf", err)
			}
		}
		if s.AcceptRA != nil {
			if err := b.SetIPv6AcceptRA(osName, *s.AcceptRA); err != nil {
				fail("ipv6 accept-ra", err)
			}
		}
		if s.Forwarding != nil {
			if err := b.SetIPv6Forwarding(osName, *s.Forwarding); err != nil {
				fail("ipv6 forwarding", err)
			}
		}
	}
	return errs
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
