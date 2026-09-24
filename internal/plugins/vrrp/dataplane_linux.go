// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- virtual-MAC dataplane (ARP/ND ownership)
//
// Making the virtual IP answer with the VIRTUAL MAC (RFC 9568 Section 7.3) is
// not automatic on Linux: when the macvlan's PARENT holds a real address in the
// same subnet, the kernel answers ARP for the VIP from the PARENT with its real
// MAC, and the macvlan never replies -- proven deterministically in QEMU (the
// macvlan receives the who-has but the kernel picks the parent). The fix,
// reverse-engineered from keepalived's use_vmac and confirmed byte-identical to
// its live kernel state, is a set of sysctls that (a) stop the parent answering
// for addresses not on it and (b) let the macvlan be the sole responder:
//
//	parent:  arp_ignore=1  arp_filter=1  rp_filter=1
//	macvlan: arp_ignore=1  rp_filter=0
//	global:  all.rp_filter=0   (effective rp_filter = max(all, iface), so the
//	                            macvlan cannot reach 0 unless all is 0 too)
//	         icmp_errors_use_inbound_ifaddr=1 (redirect source is the VIP on
//	                            the macvlan selected by the received VMAC)
//
// Combined with the macvlan being created in PRIVATE mode (register.go) and the
// VIP installed with the subnet prefix (register.go vipCIDRs), this makes an
// external host resolve the VIP to the virtual MAC. IPv6 needs none of this --
// ND answers from the macvlan natively (QEMU probe) -- so the v6 path is a
// no-op here.
//
// Scope and refcounting: the per-macvlan sysctls live and die with the device.
// The parent sysctls are shared by every group on that parent, and the global
// IPv4 knobs affect the whole namespace, so both are saved on the first group
// that needs them and restored on the last.

//go:build linux

package vrrp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// procNetRoot is the sysctl tree; a var so tests can point it at a temp dir.
var procNetRoot = "/proc/sys/net"

// sysctlKV is a resolved sysctl path and the value to write.
type sysctlKV struct {
	path  string
	value string
}

// ipv4Conf / ipv6Conf build a per-interface sysctl path. The procfs directory is
// the literal interface name (dots and all, e.g. eth0.100), unlike the dotted
// key form, so no escaping is needed.
func ipv4Conf(dev, key string) string { return filepath.Join(procNetRoot, "ipv4", "conf", dev, key) }
func ipv6Conf(dev, key string) string { return filepath.Join(procNetRoot, "ipv6", "conf", dev, key) }

// allRPFilterPath is the host-global reverse-path-filter knob.
func allRPFilterPath() string { return ipv4Conf("all", "rp_filter") }

// globalDataplaneSysctls includes ICMP source selection: the reverse route to a
// host may use the parent or another VRRP group. Linux must instead select the
// address on the inbound macvlan, whose destination MAC identifies the virtual
// router the host used (RFC 3768 Section 8.1, RFC 9568 Section 8.1.1).
func globalDataplaneSysctls() []sysctlKV {
	return []sysctlKV{
		{allRPFilterPath(), "0"},
		{filepath.Join(procNetRoot, "ipv4", "icmp_errors_use_inbound_ifaddr"), "1"},
	}
}

// vmacSysctls returns the sysctls set on the group's own macvlan (IPv4 groups
// only). These are tied to the device lifetime, so they need no save/restore.
//
// keepalived also sets disable_ipv6=1 on its vmac, but that is not part of what
// makes the virtual MAC answer: the IPv4 recipe reaches the virtual MAC with it
// removed (proven in QEMU, bridge topology, 5/5 after the cold-start resolution
// -- docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md), so ze does not
// touch IPv6 on the IPv4 vmac.
func vmacSysctls(vmac string) []sysctlKV {
	return []sysctlKV{
		{ipv4Conf(vmac, "arp_ignore"), "1"},
		{ipv4Conf(vmac, "rp_filter"), "0"},
	}
}

// parentSysctls returns the sysctls set on the parent device (IPv4 groups only).
// Shared across groups on one parent, so the caller saves the prior values.
func parentSysctls(parent string) []sysctlKV {
	return []sysctlKV{
		{ipv4Conf(parent, "arp_ignore"), "1"},
		{ipv4Conf(parent, "arp_filter"), "1"},
		{ipv4Conf(parent, "rp_filter"), "1"},
	}
}

// Seams so the refcount + save/restore logic is unit-testable without /proc.
var (
	sysctlRead  = osSysctlRead
	sysctlWrite = osSysctlWrite
)

func osSysctlRead(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path built from a validated device name
	if err != nil {
		return "", err
	}
	return trimSysctl(string(b)), nil
}

func osSysctlWrite(path, value string) error {
	return os.WriteFile(path, []byte(value), 0o644) //nolint:gosec // procfs knob, not a secret
}

// trimSysctl strips the trailing newline procfs adds to a scalar knob.
func trimSysctl(s string) string {
	for s != "" && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

var (
	dataplaneMu sync.Mutex
	// parentRefs counts successful IPv4 groups on each parent. parentSaved
	// retains originals until restoration succeeds, including at zero refs.
	parentRefs  = map[string]int{}
	parentSaved = map[string][]sysctlKV{}
	// globalRefs counts successful IPv4 groups host-wide. globalSaved also
	// retains failed restorations after setup failure or the last teardown.
	globalRefs  int
	globalSaved []sysctlKV
)

// applyDataplaneSysctls installs the recipe before the instance can advertise.
// Failure attempts shared restoration and acquires no references; failed
// restorations retain their originals. A successful call MUST be paired with
// revertDataplaneSysctls when the group is removed.
func applyDataplaneSysctls(parent, vmac, family string) error {
	if family == familyIPv6 {
		// IPv6 needs no ARP-flux recipe (ND answers from the macvlan natively --
		// spec-vrrp-6), but DAD must be disabled on the macvlan. A VRRP VIP lives
		// on exactly one router at a time, so Duplicate Address Detection is
		// pointless, and leaving it on costs a ~1s tentative window on every
		// promotion: the VIP is unreachable during it and the first advert sources
		// from the macvlan's auto link-local instead of the configured link-local
		// (both observed in the keepalived IPv6 interop lab). Set once at create,
		// before any VIP is installed; the knob dies with the device (no revert).
		if err := sysctlWrite(ipv6Conf(vmac, "accept_dad"), "0"); err != nil {
			return fmt.Errorf("disable DAD on %s: %w", vmac, err)
		}
		return nil
	}
	dataplaneMu.Lock()
	defer dataplaneMu.Unlock()
	var restoreErr error
	if globalRefs == 0 {
		globalSaved, restoreErr = restoreDataplaneSysctls(globalSaved)
	}
	if parentRefs[parent] == 0 {
		restoreErr = errors.Join(restoreErr, restoreParentSysctls(parent))
	}
	if restoreErr != nil {
		return restoreErr
	}
	for _, kv := range vmacSysctls(vmac) {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			return fmt.Errorf("set macvlan sysctl %s: %w", kv.path, err)
		}
	}
	if globalRefs == 0 {
		var err error
		globalSaved, err = applySharedSysctls(globalDataplaneSysctls())
		if err != nil {
			return err
		}
	}
	if parentRefs[parent] == 0 {
		saved, err := applySharedSysctls(parentSysctls(parent))
		if len(saved) != 0 {
			parentSaved[parent] = saved
		}
		if err != nil {
			// Existing groups still own the namespace recipe. Only a failed
			// first group may roll back the globals it just applied.
			if globalRefs == 0 {
				globalSaved, restoreErr = restoreDataplaneSysctls(globalSaved)
			}
			return errors.Join(err, restoreErr)
		}
	}
	globalRefs++
	parentRefs[parent]++
	return nil
}

// applySharedSysctls reads every original before writing shared state.
// A failed write returns any originals whose restoration remains outstanding.
func applySharedSysctls(recipe []sysctlKV) ([]sysctlKV, error) {
	saved := make([]sysctlKV, len(recipe))
	for i, kv := range recipe {
		value, err := sysctlRead(kv.path)
		if err != nil {
			return nil, fmt.Errorf("read sysctl %s: %w", kv.path, err)
		}
		saved[i] = sysctlKV{kv.path, value}
	}
	for i, kv := range recipe {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			pending, restoreErr := restoreDataplaneSysctls(saved[:i+1])
			return pending, errors.Join(fmt.Errorf("set sysctl %s: %w", kv.path, err), restoreErr)
		}
	}
	return saved, nil
}

// restoreDataplaneSysctls releases only successfully restored originals.
// Compacting the snapshot retains exact failed keys without a second journal.
func restoreDataplaneSysctls(saved []sysctlKV) ([]sysctlKV, error) {
	var result error
	pending := saved[:0]
	for _, kv := range saved {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			pending = append(pending, kv)
			result = errors.Join(result, fmt.Errorf("restore sysctl %s: %w", kv.path, err))
		}
	}
	if len(pending) == 0 {
		return nil, result
	}
	return pending, result
}

func restoreParentSysctls(parent string) error {
	pending, err := restoreDataplaneSysctls(parentSaved[parent])
	if len(pending) == 0 {
		delete(parentSaved, parent)
	} else {
		parentSaved[parent] = pending
	}
	return err
}

// reassertDataplaneSysctls re-writes the recipe values without touching the
// refcount or the saved originals. The engine calls it for every running IPv4
// instance on each config apply, so that if another subsystem overwrote a shared
// knob the recipe self-heals. The prime case: the iface component emits the
// parent's arp_ignore/arp_filter/rp_filter from unit config on every apply
// (config_sysctl.go), which would otherwise silently clobber VRRP's values --
// VRRP manages those knobs while a group is active and re-asserts here so a
// user's conflicting unit config cannot leave the dataplane half-configured.
// Reassertion failures are logged because permissions or devices can change
// after initial setup; an earlier successful apply cannot explain a new failure.
func reassertDataplaneSysctls(parent, vmac, family string) {
	if family == familyIPv6 {
		return
	}
	dataplaneMu.Lock()
	defer dataplaneMu.Unlock()
	for _, kv := range vmacSysctls(vmac) {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			logger().Warn("vrrp: reassert macvlan sysctl failed", "path", kv.path, "error", err)
		}
	}
	for _, kv := range globalDataplaneSysctls() {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			logger().Warn("vrrp: reassert global sysctl failed", "path", kv.path, "error", err)
		}
	}
	for _, kv := range parentSysctls(parent) {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			logger().Warn("vrrp: reassert parent sysctl failed", "path", kv.path, "error", err)
		}
	}
}

// revertDataplaneSysctls undoes applyDataplaneSysctls for one IPv4 group.
// Each successful group releases one reference. At zero references, repeated
// teardown calls also retry outstanding restores without releasing another group.
// Per-macvlan settings vanish with the device.
func revertDataplaneSysctls(parent, vmac, family string) {
	_ = vmac // the macvlan's own sysctls are freed when the device is deleted
	if family == familyIPv6 {
		return
	}
	dataplaneMu.Lock()
	defer dataplaneMu.Unlock()
	if n := parentRefs[parent]; n > 0 {
		if n == 1 {
			delete(parentRefs, parent)
		} else {
			parentRefs[parent] = n - 1
		}
		globalRefs--
	}
	// A failed setup on another parent may have no successful group to tear
	// down. Retry those unreferenced snapshots on a later group's teardown too.
	for device := range parentSaved {
		if parentRefs[device] == 0 {
			if err := restoreParentSysctls(device); err != nil {
				logger().Warn("vrrp: restore parent sysctl failed", "parent", device, "error", err)
			}
		}
	}
	if globalRefs == 0 {
		var err error
		globalSaved, err = restoreDataplaneSysctls(globalSaved)
		if err != nil {
			logger().Warn("vrrp: restore global sysctl failed", "error", err)
		}
	}
}
