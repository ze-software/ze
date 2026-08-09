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
//
// Combined with the macvlan being created in PRIVATE mode (register.go) and the
// VIP installed with the subnet prefix (register.go vipCIDRs), this makes an
// external host resolve the VIP to the virtual MAC. IPv6 needs none of this --
// ND answers from the macvlan natively (QEMU probe) -- so the v6 path is a
// no-op here.
//
// Scope and refcounting: the per-macvlan sysctls live and die with the device.
// The parent sysctls are shared by every group on that parent, and all.rp_filter
// is host-global, so both are saved on the first group that needs them and
// restored on the last (keepalived leaves them set; ze restores, so removing
// VRRP leaves the host as it was).

//go:build linux

package vrrp

import (
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
	// parentRefs counts IPv4 groups whose macvlan sits on each parent device;
	// parentSaved holds the prior values to restore when the count hits zero.
	parentRefs  = map[string]int{}
	parentSaved = map[string][]sysctlKV{}
	// globalRefs counts IPv4 groups host-wide; globalSaved is the prior
	// all.rp_filter value, restored when the count hits zero.
	globalRefs  int
	globalSaved sysctlKV
	globalHave  bool
)

// applyDataplaneSysctls installs the virtual-MAC ARP recipe for one IPv4 group's
// macvlan on parent. IPv6 groups are a no-op. Best-effort: a write that fails
// (should not happen with CAP_NET_ADMIN, which VRRP already holds) is logged and
// the instance still runs -- only the L2 ownership degrades, never the election.
// Refcounts are always incremented so revert stays balanced.
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
			logger().Warn("vrrp: set macvlan accept_dad failed", "vmac", vmac, "error", err)
		}
		return nil
	}
	for _, kv := range vmacSysctls(vmac) {
		if err := sysctlWrite(kv.path, kv.value); err != nil {
			logger().Warn("vrrp: set macvlan sysctl failed", "path", kv.path, "value", kv.value, "error", err)
		}
	}
	dataplaneMu.Lock()
	defer dataplaneMu.Unlock()
	if globalRefs == 0 {
		if cur, err := sysctlRead(allRPFilterPath()); err == nil {
			globalSaved = sysctlKV{allRPFilterPath(), cur}
			globalHave = true
		}
		if err := sysctlWrite(allRPFilterPath(), "0"); err != nil {
			logger().Warn("vrrp: set all.rp_filter failed", "error", err)
		}
	}
	globalRefs++
	if parentRefs[parent] == 0 {
		var saved []sysctlKV
		for _, kv := range parentSysctls(parent) {
			if cur, err := sysctlRead(kv.path); err == nil {
				saved = append(saved, sysctlKV{kv.path, cur})
			}
			if err := sysctlWrite(kv.path, kv.value); err != nil {
				logger().Warn("vrrp: set parent sysctl failed", "path", kv.path, "value", kv.value, "error", err)
			}
		}
		parentSaved[parent] = saved
	}
	parentRefs[parent]++
	return nil
}

// reassertDataplaneSysctls re-writes the recipe values without touching the
// refcount or the saved originals. The engine calls it for every running IPv4
// instance on each config apply, so that if another subsystem overwrote a shared
// knob the recipe self-heals. The prime case: the iface component emits the
// parent's arp_ignore/arp_filter/rp_filter from unit config on every apply
// (config_sysctl.go), which would otherwise silently clobber VRRP's values --
// VRRP manages those knobs while a group is active and re-asserts here so a
// user's conflicting unit config cannot leave the dataplane half-configured.
// Quiet (no per-apply logging): the initial applyDataplaneSysctls already warned
// on any write failure.
func reassertDataplaneSysctls(parent, vmac, family string) {
	if family == familyIPv6 {
		return
	}
	for _, kv := range vmacSysctls(vmac) {
		_ = sysctlWrite(kv.path, kv.value)
	}
	_ = sysctlWrite(allRPFilterPath(), "0")
	for _, kv := range parentSysctls(parent) {
		_ = sysctlWrite(kv.path, kv.value)
	}
}

// revertDataplaneSysctls undoes applyDataplaneSysctls for one IPv4 group. The
// per-macvlan sysctls vanish with the device (no action). Parent and global
// sysctls are restored to their saved values on the last group that used them.
func revertDataplaneSysctls(parent, vmac, family string) {
	_ = vmac // the macvlan's own sysctls are freed when the device is deleted
	if family == familyIPv6 {
		return
	}
	dataplaneMu.Lock()
	defer dataplaneMu.Unlock()
	if n := parentRefs[parent]; n > 0 {
		parentRefs[parent] = n - 1
		if parentRefs[parent] == 0 {
			for _, kv := range parentSaved[parent] {
				if err := sysctlWrite(kv.path, kv.value); err != nil {
					logger().Warn("vrrp: restore parent sysctl failed", "path", kv.path, "value", kv.value, "error", err)
				}
			}
			delete(parentSaved, parent)
			delete(parentRefs, parent)
		}
	}
	if globalRefs > 0 {
		globalRefs--
		if globalRefs == 0 && globalHave {
			if err := sysctlWrite(globalSaved.path, globalSaved.value); err != nil {
				logger().Warn("vrrp: restore all.rp_filter failed", "error", err)
			}
			globalHave = false
		}
	}
}
