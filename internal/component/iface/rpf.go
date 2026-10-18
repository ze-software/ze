// Design: docs/features/interfaces.md -- reverse-path check
// Related: rpf4.go -- desiredIPv4RPF, the set Backend.ReplaceIPv4RPF enforces
// Related: rpf6.go -- desiredIPv6RPF, the set Backend.ReplaceIPv6RPF enforces
// Related: config_apply.go -- applyConfigPass hands both sets to the backend

package iface

// RPFMode names how a reverse-path check judges a source address.
// The zero value is unspecified and is never a check a backend installs.
type RPFMode uint8

const (
	// RPFUnspecified is the zero value: no mode was chosen.
	RPFUnspecified RPFMode = iota
	// RPFStrict passes a packet only when the route back to its source
	// leaves through the interface the packet arrived on.
	RPFStrict
	// RPFLoose passes a packet when any route to its source exists.
	RPFLoose
)

// RPFCheck is one interface's reverse-path check for one address family; the
// family is the Backend method the check is handed to. Interface is the OS
// device name, never the logical ze name, because the dataplane matches the
// device a packet arrived on.
type RPFCheck struct {
	Interface string
	Mode      RPFMode
}

// rpfCheckMode maps the configured rpf-check value to the check a backend
// installs. Disable maps to Unspecified, which installs nothing: the whole
// set is handed over on every apply, so an interface absent from it is an
// interface with no check, which is the disabled state.
func rpfCheckMode(m rpfMode) RPFMode {
	switch m {
	case rpfModeStrict:
		return RPFStrict
	case rpfModeLoose:
		return RPFLoose
	case rpfModeDisable:
		return RPFUnspecified
	}
	// rpfMode is closed: parseRPFMode and the IPv4 rp-filter range check in
	// config.go are its only producers, and both refuse every other value.
	panic("BUG: iface: rpfMode out of range")
}

// desiredRPF returns every reverse-path check cfg asks for, keyed to the
// device each unit configures; leaf selects the family's rpf-check leaf of a
// unit, nil when the unit sets none. It walks the same units, with the same
// device binding, as the sysctl phase of applyConfigPass: an entry whose
// selector bound no device is skipped there and here, and a VLAN unit is
// checked on its VLAN device. The result is the complete set, so a check
// whose leaf, unit or interface left the config is absent from it.
func desiredRPF(cfg *ifaceConfig, devices map[string]string, leaf func(*unitEntry) *rpfMode) []RPFCheck {
	var checks []RPFCheck
	add := func(osName string, u *unitEntry) {
		configured := leaf(u)
		if configured == nil {
			return
		}
		mode := rpfCheckMode(*configured)
		if mode == RPFUnspecified {
			return
		}
		checks = append(checks, RPFCheck{Interface: unitOSName(osName, u), Mode: mode})
	}
	entries := allIfaceEntries(cfg)
	for i := range entries {
		if entries[i].Disable {
			continue
		}
		device, bound := deviceFor(devices, entries[i].Name)
		if !bound {
			continue
		}
		for j := range entries[i].Units {
			if entries[i].Units[j].Disable {
				continue
			}
			add(device, &entries[i].Units[j])
		}
	}
	if cfg != nil && cfg.Loopback != nil {
		for j := range cfg.Loopback.Units {
			if cfg.Loopback.Units[j].Disable {
				continue
			}
			add("lo", &cfg.Loopback.Units[j])
		}
	}
	return checks
}
