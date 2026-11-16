// Design: docs/architecture/iface/logical-name-resolution.md -- a configured-interface name binds through its entry's selector
// Related: interface_reference.go -- the reference source that admits the names bound here
// Related: config_mirror.go -- mirror destinations bound here
// Related: config_apply.go -- bridge members and tunnel local interfaces bound here

package iface

import "fmt"

// interfaceTargets binds the names a configured-interface reference leaf
// holds (a mirror target, a bridge member, a tunnel's local interface) to
// kernel devices. A name is the logical name of an entry, or of a VLAN unit
// written <entry>.<vlan-id>, the two forms configuredInterfaceNames accepts.
// The entry form is in the selector map. The unit form is not, and passing it
// through unchanged reached whatever device carried the logical name, or no
// device: a VLAN's kernel device is named after the device its PARENT resolved
// to (unitOSName), so a unit is bound through its parent's entry.
//
// Build one with newInterfaceTargets for the config whose selector map devices
// is. A value is read-only after construction.
type interfaceTargets struct {
	devices map[string]string
	// units maps every declared VLAN unit, by its <entry>.<vlan-id> name, to
	// the entry that declares it.
	units map[string]interfaceUnit
}

// interfaceUnit is one VLAN unit a reference may name: the logical name of the
// entry that declares it, and the unit, whose VLAN id names the device.
type interfaceUnit struct {
	parent string
	unit   *unitEntry
}

// newInterfaceTargets indexes the VLAN units cfg declares, for the selector map
// bindDevices answered for cfg. Every entry kind is indexed, disabled ones
// included: disabling says what ze does with an interface, never which device
// it is (bindDevices). The key is unitOSName of the entry's logical name, the
// same text the configured-interface reference source offers, so a name the
// commit accepted as a VLAN unit is found here.
func newInterfaceTargets(cfg *ifaceConfig, devices map[string]string) interfaceTargets {
	targets := interfaceTargets{devices: devices}
	if cfg == nil {
		return targets
	}
	for _, e := range allIfaceEntries(cfg) {
		for i := range e.Units {
			u := &e.Units[i]
			if u.VLANID <= 0 {
				continue
			}
			if targets.units == nil {
				targets.units = make(map[string]interfaceUnit)
			}
			targets.units[unitOSName(e.Name, u)] = interfaceUnit{parent: e.Name, unit: u}
		}
	}
	return targets
}

// unitNamed answers the VLAN unit a reference names, and false when it names
// an entry instead. An ethernet entry the selector map holds is a direct
// declaration, so a unit whose <entry>.<vlan-id> text equals it never shadows it.
func (t interfaceTargets) unitNamed(name string) (interfaceUnit, bool) {
	if _, isEntry := t.devices[name]; isEntry {
		return interfaceUnit{}, false
	}
	target, isUnit := t.units[name]
	return target, isUnit
}

// device returns the kernel device a reference names, and whether it is bound
// to one. A VLAN unit is <parent device>.<vlan-id>, and it is UNBOUND when its
// parent is. Every other name follows deviceFor: an ethernet entry through its
// selector, any other kind as its own device. A caller never falls back to the
// logical name when bound is false.
func (t interfaceTargets) device(name string) (device string, bound bool) {
	if target, isUnit := t.unitNamed(name); isUnit {
		parentDevice, parentBound := deviceFor(t.devices, target.parent)
		if !parentBound {
			return "", false
		}
		return unitOSName(parentDevice, target.unit), true
	}
	return deviceFor(t.devices, name)
}

// tunnelTargets binds the tunnel local interfaces of cfg, and of previous for
// the delete-then-create undo, against one interface listing. It lists only
// when a tunnel of either config names a local interface: otherwise both
// answers are empty, and that is safe only because tunnelSpecOnDevices never
// consults targets for a spec without a local interface. A listing the backend
// cannot answer is returned as the error, because no local interface can be
// bound without it; the caller's record defers a not-ready backend and fails
// the apply on any other answer.
func (cfg *ifaceConfig) tunnelTargets(previous *ifaceConfig, b Backend) (current, prior interfaceTargets, err error) {
	if !cfg.namesTunnelLocalInterface() && !previous.namesTunnelLocalInterface() {
		return interfaceTargets{}, interfaceTargets{}, nil
	}
	infos, err := b.ListInterfaces()
	if err != nil {
		return interfaceTargets{}, interfaceTargets{}, fmt.Errorf("list interfaces to bind tunnel local interfaces: %w", err)
	}
	return newInterfaceTargets(cfg, cfg.bindDevices(infos)), newInterfaceTargets(previous, previous.bindDevices(infos)), nil
}

// namesTunnelLocalInterface reports whether any tunnel of cfg takes its source
// from a local interface. A nil config names none.
func (cfg *ifaceConfig) namesTunnelLocalInterface() bool {
	if cfg == nil {
		return false
	}
	for i := range cfg.Tunnel {
		if cfg.Tunnel[i].Spec.LocalInterface != "" {
			return true
		}
	}
	return false
}

// tunnelSpecOnDevices returns spec with its local interface replaced by the
// kernel device it names, which is what the backend looks up: the logical
// name reached whatever device carried it, so an uplink bound by mac/match or
// os-name gave the tunnel the wrong parent, or none. A spec with a local ip is
// returned unchanged. An unbound local interface is an error rather than a
// deferral, because no later phase could configure a tunnel this apply did not
// create; the backend refused an absent local device the same way before.
func tunnelSpecOnDevices(spec TunnelSpec, targets interfaceTargets, tunnel string) (TunnelSpec, error) {
	if spec.LocalInterface == "" {
		return spec, nil
	}
	device, bound := targets.device(spec.LocalInterface)
	if !bound {
		return spec, fmt.Errorf("tunnel %q local interface %q: no present device answers the hardware selector "+
			"of the ethernet entry it names, so the tunnel has no underlay; connect that device, or point the "+
			"entry's mac match or os-name at a present device (show interface lists them)",
			tunnel, spec.LocalInterface)
	}
	spec.LocalInterface = device
	return spec, nil
}
