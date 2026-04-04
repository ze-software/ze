// Design: docs/features/interfaces.md -- Interface config parsing and application
// Overview: iface.go -- shared types and topic constants
// Related: backend.go -- Backend interface used for application
// Related: register.go -- OnConfigure calls applyConfig

package iface

import (
	"encoding/json"
	"fmt"
	"strconv"
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

// applyConfig applies the parsed interface config to the OS via the backend.
// Creates interfaces that don't exist, sets properties, adds addresses.
// Existing interfaces that are not in the config are left alone (no deletion).
func applyConfig(cfg *ifaceConfig, b Backend) {
	log := loggerPtr.Load()

	for _, e := range cfg.Dummy {
		if e.Disable {
			continue
		}
		if err := b.CreateDummy(e.Name); err != nil {
			log.Debug("iface config: create dummy (may already exist)", "name", e.Name, "err", err)
		}
		applyIfaceProps(b, e, log)
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
		applyIfaceProps(b, e.ifaceEntry, log)
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
		applyIfaceProps(b, e.ifaceEntry, log)
	}

	// Ethernet interfaces are not created (they're physical), but properties
	// and addresses are applied.
	for _, e := range cfg.Ethernet {
		if e.Disable {
			continue
		}
		applyIfaceProps(b, e, log)
	}

	// Loopback: addresses only (no physical properties to set).
	if cfg.Loopback != nil {
		for _, u := range cfg.Loopback.Units {
			if u.Disable {
				continue
			}
			for _, addr := range u.Addresses {
				if err := b.AddAddress("lo", addr); err != nil {
					log.Debug("iface config: loopback address (may already exist)", "addr", addr, "err", err)
				}
			}
		}
	}
}

func applyIfaceProps(b Backend, e ifaceEntry, log interface{ Warn(string, ...any) }) {
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
		osName := e.Name
		if u.VLANID > 0 {
			if err := b.CreateVLAN(e.Name, u.VLANID); err != nil {
				log.Warn("iface config: create vlan (may already exist)",
					"parent", e.Name, "vlan", u.VLANID, "err", err)
			}
			osName = fmt.Sprintf("%s.%d", e.Name, u.VLANID)
		} else if u.ID > 0 {
			// Non-VLAN units > 0: addresses go on the parent interface.
			osName = e.Name
		}

		for _, addr := range u.Addresses {
			if err := b.AddAddress(osName, addr); err != nil {
				log.Warn("iface config: add address (may already exist)",
					"iface", osName, "addr", addr, "err", err)
			}
		}
	}
}
