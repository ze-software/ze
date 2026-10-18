// Design: docs/features/interfaces.md -- IPv6 reverse-path check
// Related: backend.go -- Backend.ReplaceIPv6RPF, which enforces the set built here
// Related: config_apply.go -- applyConfigPass hands the set to the backend

package iface

// IPv6RPFMode names how an IPv6 reverse-path check judges a source address.
// The zero value is unspecified and is never a check a backend installs.
type IPv6RPFMode uint8

const (
	// IPv6RPFUnspecified is the zero value: no mode was chosen.
	IPv6RPFUnspecified IPv6RPFMode = iota
	// IPv6RPFStrict passes a packet only when the route back to its source
	// leaves through the interface the packet arrived on.
	IPv6RPFStrict
	// IPv6RPFLoose passes a packet when any route to its source exists.
	IPv6RPFLoose
)

// IPv6RPFCheck is one interface's IPv6 reverse-path check. Interface is the
// OS device name, never the logical ze name, because the dataplane matches
// the device a packet arrived on.
type IPv6RPFCheck struct {
	Interface string
	Mode      IPv6RPFMode
}

// ipv6RPFMode maps the configured rpf-check value to the check a backend
// installs. Disable maps to Unspecified, which installs nothing: Linux has no
// IPv6 check of its own to turn off, so no check is the disabled state.
func ipv6RPFMode(m rpfMode) IPv6RPFMode {
	switch m {
	case rpfModeStrict:
		return IPv6RPFStrict
	case rpfModeLoose:
		return IPv6RPFLoose
	case rpfModeDisable:
		return IPv6RPFUnspecified
	}
	// rpfMode is closed: parseRPFMode and the IPv4 rp-filter range check in
	// config.go are its only producers, and both refuse every other value.
	panic("BUG: iface: rpfMode out of range")
}

// desiredIPv6RPF returns every IPv6 reverse-path check cfg asks for, keyed to
// the device each unit configures. It walks the same units, with the same
// device binding, as the sysctl phase of applyConfigPass: an entry whose
// selector bound no device is skipped there and here, and a VLAN unit is
// checked on its VLAN device. The result is the complete set, so a check
// whose leaf, unit or interface left the config is absent from it.
func desiredIPv6RPF(cfg *ifaceConfig, devices map[string]string) []IPv6RPFCheck {
	var checks []IPv6RPFCheck
	add := func(osName string, u *unitEntry) {
		if u.IPv6 == nil {
			return
		}
		if u.IPv6.RPFCheck == nil {
			return
		}
		mode := ipv6RPFMode(*u.IPv6.RPFCheck)
		if mode == IPv6RPFUnspecified {
			return
		}
		checks = append(checks, IPv6RPFCheck{Interface: unitOSName(osName, u), Mode: mode})
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
