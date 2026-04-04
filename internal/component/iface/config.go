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
	ID        int
	VLANID    int
	Addresses []string
	Disable   bool
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
				entry.STP = stp == "true"
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
		}
		units = append(units, u)
	}
	return units
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
// 2. Sets properties (MTU, MAC) on all configured interfaces
// 3. Adds missing addresses, removes extra addresses on configured interfaces
// 4. Deletes Ze-managed interfaces not in config.
func applyConfig(cfg *ifaceConfig, b Backend) {
	log := loggerPtr.Load()

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
			log.Warn("iface config: bridge stp", "name", e.Name, "err", err)
		}
		for _, member := range e.Members {
			if err := b.BridgeAddPort(e.Name, member); err != nil {
				log.Warn("iface config: bridge add port", "bridge", e.Name, "port", member, "err", err)
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
				log.Warn("iface config: set mtu", "name", e.Name, "mtu", e.MTU, "err", err)
			}
		}
		if e.MACAddress != "" {
			if err := b.SetMACAddress(e.Name, e.MACAddress); err != nil {
				log.Warn("iface config: set mac", "name", e.Name, "err", err)
			}
		}
		for _, u := range e.Units {
			if u.Disable {
				continue
			}
			if u.VLANID > 0 {
				if err := b.CreateVLAN(e.Name, u.VLANID); err != nil {
					log.Debug("iface config: create vlan (may already exist)",
						"parent", e.Name, "vlan", u.VLANID, "err", err)
				}
			}
		}
	}

	// Phase 3: Reconcile addresses (add missing, remove extra).
	desiredAddrs, managedNames := cfg.desiredState()

	currentInfos, err := b.ListInterfaces()
	if err != nil {
		log.Warn("iface config: list interfaces for reconciliation failed", "err", err)
		// Fall back to additive-only: add all desired addresses.
		for osName, addrs := range desiredAddrs {
			for addr := range addrs {
				if err := b.AddAddress(osName, addr); err != nil {
					log.Debug("iface config: add address", "iface", osName, "addr", addr, "err", err)
				}
			}
		}
		return
	}

	currentAddrs := currentAddrSet(currentInfos)

	// Add missing addresses on configured interfaces.
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range desired {
			if current != nil && current[addr] {
				continue // already present
			}
			if err := b.AddAddress(osName, addr); err != nil {
				log.Warn("iface config: add address", "iface", osName, "addr", addr, "err", err)
			}
		}
	}

	// Remove extra addresses on configured interfaces (only interfaces in config).
	for osName, desired := range desiredAddrs {
		current := currentAddrs[osName]
		for addr := range current {
			if desired[addr] {
				continue // should be there
			}
			if err := b.RemoveAddress(osName, addr); err != nil {
				log.Warn("iface config: remove stale address", "iface", osName, "addr", addr, "err", err)
			} else {
				log.Info("iface config: removed stale address", "iface", osName, "addr", addr)
			}
		}
	}

	// Phase 4: Delete Ze-managed interfaces not in config.
	currentIfaces := currentIfaceSet(currentInfos)
	for name, linkType := range currentIfaces {
		if !zeManageable(linkType) {
			continue // never delete physical or loopback
		}
		if managedNames[name] {
			continue // in config, keep
		}
		if err := b.DeleteInterface(name); err != nil {
			log.Warn("iface config: delete unmanaged interface", "name", name, "type", linkType, "err", err)
		} else {
			log.Info("iface config: deleted interface not in config", "name", name, "type", linkType)
		}
	}
}
