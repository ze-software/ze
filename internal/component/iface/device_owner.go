// Design: docs/features/interfaces.md -- plugin-owned devices (macvlan)
// Related: macvlan.go -- MacvlanSpec and ComposeOwnedDeviceName
// Related: address_owner.go -- the sibling plugin-owned ADDRESS registry
// Related: config_apply.go -- reconcileOwnedDevices consumes ownedMacvlans()

package iface

import (
	"fmt"
	"sync"
)

// ownedDeviceAliasPrefix marks a kernel link as owned by ze's owned-device
// registry. It is written as the link's IFLA_IFALIAS at create time, as
// "ze:owned:<owner>". The reconcile orphan scan deletes any macvlan carrying
// this prefix that has no matching registration -- covering owner release AND
// crash leftovers without in-memory history (the kernel remembers ownership,
// the process need not). This is the one structural difference from the
// address registry (address_owner.go), which must remember what it did
// (staleIfaces) because an address carries no owner.
const ownedDeviceAliasPrefix = "ze:owned:"

// deviceOwnerMu protects deviceOwners, deviceOwnerTrigger, and gaugeOwners.
var deviceOwnerMu sync.Mutex

// deviceOwners maps owner name -> device name -> desired MacvlanSpec. Device
// names are unique across owners (RegisterOwnedMacvlan's conflict check), so
// ownedMacvlans() can key by device name. Empty by default: a process with no
// registered owner has zero owned devices and the reconcile device pass is a
// no-op.
var deviceOwners = map[string]map[string]MacvlanSpec{}

// deviceOwnerTrigger is invoked after every registry mutation, wired once at
// startup to the SAME reconcile channel the address registry uses
// (setDeviceOwnerReconcileTrigger). nil (a no-op) until then, keeping the
// registry usable standalone in unit tests.
var deviceOwnerTrigger func()

// setDeviceOwnerReconcileTrigger wires the callback invoked whenever the
// owned-device registry changes. Wired beside setAddressOwnerReconcileTrigger
// to the same registryReconcileCh so one worker serves both registries.
// Passing nil detaches the trigger (e.g. on shutdown).
func setDeviceOwnerReconcileTrigger(trigger func()) {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	deviceOwnerTrigger = trigger
}

// RegisterOwnedMacvlan declares that owner wants a bridge-mode macvlan named
// spec.Name on spec.Parent carrying spec.MAC. The reconcile device pass
// creates it (marked "ze:owned:<owner>") BEFORE the address passes, so a VIP
// registered on spec.Name via RegisterOwnedAddresses lands on an existing
// device in the same pass. Re-registering the same owner+name replaces that
// owner's spec (idempotent for the desired state when unchanged; the pass
// re-asserts only on drift).
//
// Returns an error naming the conflicting owner if a DIFFERENT owner already
// registered spec.Name -- the original registration is left unchanged.
func RegisterOwnedMacvlan(owner string, spec MacvlanSpec) error {
	if owner == "" {
		return fmt.Errorf("iface: owned macvlan owner is empty")
	}
	if err := spec.validate(); err != nil {
		return err
	}

	deviceOwnerMu.Lock()
	for otherOwner, devices := range deviceOwners {
		if otherOwner == owner {
			continue
		}
		if _, exists := devices[spec.Name]; exists {
			deviceOwnerMu.Unlock()
			return fmt.Errorf("iface: macvlan device %q already owned by %q", spec.Name, otherOwner)
		}
	}
	if deviceOwners[owner] == nil {
		deviceOwners[owner] = make(map[string]MacvlanSpec)
	}
	deviceOwners[owner][spec.Name] = spec
	trigger := deviceOwnerTrigger
	deviceOwnerMu.Unlock()

	updateOwnedDeviceGauge()
	if trigger != nil {
		trigger()
	}
	return nil
}

// UnregisterOwnedMacvlan removes one device registration for owner. A no-op
// (no reconcile trigger) if that owner never registered the name. The next
// reconcile pass sees an aliased kernel macvlan with no registration and
// deletes it (which removes its addresses with it).
func UnregisterOwnedMacvlan(owner, name string) {
	deviceOwnerMu.Lock()
	devices, ok := deviceOwners[owner]
	_, existed := devices[name]
	if ok {
		delete(devices, name)
		if len(devices) == 0 {
			delete(deviceOwners, owner)
		}
	}
	trigger := deviceOwnerTrigger
	deviceOwnerMu.Unlock()

	if !existed {
		return
	}
	updateOwnedDeviceGauge()
	if trigger != nil {
		trigger()
	}
}

// UnregisterOwnedMacvlans removes every device owner previously registered.
// A no-op (no reconcile trigger) if owner has no registrations. The next
// reconcile pass deletes each released device by its alias marker.
func UnregisterOwnedMacvlans(owner string) {
	deviceOwnerMu.Lock()
	_, existed := deviceOwners[owner]
	delete(deviceOwners, owner)
	trigger := deviceOwnerTrigger
	deviceOwnerMu.Unlock()

	if !existed {
		return
	}
	updateOwnedDeviceGauge()
	if trigger != nil {
		trigger()
	}
}

// ownedMacvlans returns a fresh snapshot merging every registered owner:
// specs maps device name -> desired MacvlanSpec, owners maps device name ->
// owner (for the "ze:owned:<owner>" alias). Consumed by the reconcile device
// pass. Device names are unique across owners (conflict check), so no key
// collides. The returned maps are copies the caller may mutate.
func ownedMacvlans() (specs map[string]MacvlanSpec, owners map[string]string) {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	specs = make(map[string]MacvlanSpec)
	owners = make(map[string]string)
	for owner, devices := range deviceOwners {
		for name, spec := range devices {
			specs[name] = spec
			owners[name] = owner
		}
	}
	return specs, owners
}

// isRegisteredMacvlanParent reports whether name is the parent of any
// currently-registered owned macvlan. The monitor wiring in register.go calls
// this so a parent appearing (interface/created) or coming up (interface/up)
// after a device was registered re-triggers the reconcile pass, creating the
// device with no further plugin calls (holo bug 12: fire-and-forget create is
// replaced by an event-driven retry).
func isRegisteredMacvlanParent(name string) bool {
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	for _, devices := range deviceOwners {
		for _, spec := range devices {
			if spec.Parent == name {
				return true
			}
		}
	}
	return false
}

// gaugeOwners records the owner label values currently present in the
// ze_iface_owned_devices GaugeVec, so an owner that drops to zero devices has
// its series deleted (not left stale at its last value). Guarded by
// deviceOwnerMu.
var gaugeOwners = map[string]bool{}

// updateOwnedDeviceGauge refreshes ze_iface_owned_devices{owner} from the live
// registry: one gauge per owner set to that owner's device count, and any
// previously-present owner no longer registered has its series deleted. Called
// after every registry mutation and by the reconcile device pass. No-op until
// bindMetricsRegistry has installed the gauge.
func updateOwnedDeviceGauge() {
	m := ifaceMetricsPtr.Load()
	if m == nil || m.ownedDevices == nil {
		return
	}
	deviceOwnerMu.Lock()
	defer deviceOwnerMu.Unlock()
	seen := make(map[string]bool, len(deviceOwners))
	for owner, devices := range deviceOwners {
		m.ownedDevices.With(owner).Set(float64(len(devices)))
		seen[owner] = true
	}
	for owner := range gaugeOwners {
		if !seen[owner] {
			m.ownedDevices.Delete(owner)
		}
	}
	gaugeOwners = seen
}
