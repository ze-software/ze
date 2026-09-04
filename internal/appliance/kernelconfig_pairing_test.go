package appliance

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// unverifiedRuntimeSymbols are the `CONFIG_*=y` lines in
// gokrazy/kernel/runtime.config that gokrazy/kernel/runtime.require does NOT
// assert. Kconfig drops a symbol it does not know without a word, so a name in
// runtime.config alone buys nothing and says so nowhere: the built .config
// simply has no line for it, which is indistinguishable from a symbol never
// asked for.
//
// That is not hypothetical. On 2026-09-03 `CONFIG_NF_CONNTRACK_NETLINK` was
// added to give flow-export the conntrack dump interface. It is not a Kconfig
// symbol at all -- the real one is `CONFIG_NF_CT_NETLINK` -- and every
// dependency it would have needed was already satisfied, so nothing looked
// wrong. The runtime.require entry is the only reason the build refused instead
// of shipping a kernel that silently lacked the feature.
//
// This list exists so that state is ACKNOWLEDGED rather than silent. A symbol
// here is one somebody decided not to assert. Adding a new `=y` line without
// touching runtime.require now fails this test, and the author chooses: assert
// it, or record it here with a reason.
//
// Shrinking this list is worthwhile work and needs a build per architecture to
// do safely, because a symbol that does not resolve on one arch turns a missing
// feature into a refused build. The list itself shows why: CONFIG_X86_POWERNOW_K8
// and its neighbours are x86-only, and asserting them would refuse every arm64
// runtime build. That is the trade this list makes explicit rather than the
// silence it replaces.
var unverifiedRuntimeSymbols = map[string]bool{
	"CONFIG_9P_FS":                          true,
	"CONFIG_9P_FS_POSIX_ACL":                true,
	"CONFIG_ATA":                            true,
	"CONFIG_BLK_DEV_NVME":                   true,
	"CONFIG_BLK_DEV_SR":                     true,
	"CONFIG_BNXT":                           true,
	"CONFIG_BRIDGE":                         true,
	"CONFIG_CDROM":                          true,
	"CONFIG_CGROUP_BPF":                     true,
	"CONFIG_CGROUP_FREEZER":                 true,
	"CONFIG_CGROUP_PIDS":                    true,
	"CONFIG_CPU_FREQ_DEFAULT_GOV_POWERSAVE": true,
	"CONFIG_CPU_FREQ_GOV_POWERSAVE":         true,
	"CONFIG_DEFAULT_BBR":                    true,
	"CONFIG_EFIVAR_FS":                      true,
	"CONFIG_I40E":                           true,
	"CONFIG_I6300ESB_WDT":                   true,
	"CONFIG_ICE":                            true,
	"CONFIG_IGB":                            true,
	"CONFIG_IGC":                            true,
	"CONFIG_INET_DIAG":                      true,
	"CONFIG_IP_ADVANCED_ROUTER":             true,
	"CONFIG_IP_NF_NAT":                      true,
	"CONFIG_IP_NF_TARGET_MASQUERADE":        true,
	"CONFIG_IRQ_TIME_ACCOUNTING":            true,
	"CONFIG_ISO9660_FS":                     true,
	"CONFIG_KVM":                            true,
	"CONFIG_KVM_AMD":                        true,
	"CONFIG_KVM_AMD_SEV":                    true,
	"CONFIG_KVM_INTEL":                      true,
	"CONFIG_LOOP":                           true,
	"CONFIG_MLX5_CORE":                      true,
	"CONFIG_MLX5_CORE_EN":                   true,
	"CONFIG_MLX5_EN":                        true,
	"CONFIG_MODULE_UNLOAD":                  true,
	"CONFIG_NAMESPACES":                     true,
	"CONFIG_NETFILTER_ADVANCED":             true,
	"CONFIG_NETFILTER_XT_MARK":              true,
	"CONFIG_NETFILTER_XT_MATCH_COMMENT":     true,
	"CONFIG_NETFILTER_XT_MATCH_MULTIPORT":   true,
	"CONFIG_NETFILTER_XT_NAT":               true,
	"CONFIG_NETFILTER_XT_TARGET_MASQUERADE": true,
	"CONFIG_NET_9P":                         true,
	"CONFIG_NET_9P_VIRTIO":                  true,
	"CONFIG_NET_NS":                         true,
	"CONFIG_NET_SOCK_MSG":                   true,
	"CONFIG_NET_UDP_TUNNEL":                 true,
	"CONFIG_NVME_CORE":                      true,
	"CONFIG_NVME_HWMON":                     true,
	"CONFIG_NVME_MULTIPATH":                 true,
	"CONFIG_NVME_TARGET_PASSTHRU":           true,
	"CONFIG_OVERLAY_FS":                     true,
	"CONFIG_SCSI":                           true,
	"CONFIG_SCSI_MOD":                       true,
	"CONFIG_SENSORS_CORSAIR_CPRO":           true,
	"CONFIG_SENSORS_K10TEMP":                true,
	"CONFIG_SENSORS_NCT6683":                true,
	"CONFIG_SOCK_CGROUP_DATA":               true,
	"CONFIG_SP5100_TCO":                     true,
	"CONFIG_SQUASHFS_XZ":                    true,
	"CONFIG_TCP_CONG_BBR":                   true,
	"CONFIG_TMPFS":                          true,
	"CONFIG_TUN":                            true,
	"CONFIG_USB_EHCI_HCD":                   true,
	"CONFIG_USB_STORAGE":                    true,
	"CONFIG_USB_XHCI_HCD":                   true,
	"CONFIG_VIRTIO":                         true,
	"CONFIG_VIRTIO_BALLOON":                 true,
	"CONFIG_VIRTIO_BLK":                     true,
	"CONFIG_VIRTIO_PCI":                     true,
	"CONFIG_VIRTIO_RING":                    true,
	"CONFIG_WIREGUARD":                      true,
	"CONFIG_X86_AMD_FREQ_SENSITIVITY":       true,
	"CONFIG_X86_AMD_PLATFORM_DEVICE":        true,
	"CONFIG_X86_POWERNOW_K8":                true,
}

var runtimeConfigSetPattern = regexp.MustCompile(`(?m)^(CONFIG_[A-Z0-9_]+)=y$`)
var runtimeRequirePattern = regexp.MustCompile(`(?m)^(CONFIG_[A-Z0-9_]+)\s*$`)

func readKernelFile(t *testing.T, name string) string {
	t.Helper()
	// The test binary runs in internal/appliance; the kernel manifests are at
	// the repository root.
	data, err := os.ReadFile(filepath.Join("..", "..", "gokrazy", "kernel", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

// VALIDATES: every symbol runtime.config turns on is either asserted by
// runtime.require or listed as knowingly unasserted.
//
// PREVENTS: a misspelled or renamed Kconfig symbol shipping as a silent no-op.
// The build only refuses a symbol runtime.require names; one that appears in
// runtime.config alone is dropped by Kconfig without a diagnostic.
func TestEveryRuntimeConfigSymbolIsAssertedOrAcknowledged(t *testing.T) {
	required := map[string]bool{}
	for _, match := range runtimeRequirePattern.FindAllStringSubmatch(readKernelFile(t, "runtime.require"), -1) {
		required[match[1]] = true
	}
	if len(required) == 0 {
		t.Fatal("parsed no symbols out of runtime.require; the parser or the file shape changed")
	}

	unpaired := make([]string, 0)
	for _, match := range runtimeConfigSetPattern.FindAllStringSubmatch(readKernelFile(t, "runtime.config"), -1) {
		symbol := match[1]
		if required[symbol] || unverifiedRuntimeSymbols[symbol] {
			continue
		}
		unpaired = append(unpaired, symbol)
	}
	sort.Strings(unpaired)
	if len(unpaired) != 0 {
		t.Errorf("runtime.config turns on %d symbol(s) that runtime.require does not assert and unverifiedRuntimeSymbols does not acknowledge.\n"+
			"Kconfig drops an unknown symbol silently, so one of these misspelled ships as a no-op.\n"+
			"Add it to runtime.require to have the build check it, or to unverifiedRuntimeSymbols with a reason:\n  %s",
			len(unpaired), strings.Join(unpaired, "\n  "))
	}
}

// VALIDATES: the acknowledgement list does not rot. An entry for a symbol that
// runtime.config no longer sets, or that runtime.require now asserts, is a
// stale exemption and hides the next real one.
func TestUnverifiedRuntimeSymbolsHasNoStaleEntries(t *testing.T) {
	set := map[string]bool{}
	for _, match := range runtimeConfigSetPattern.FindAllStringSubmatch(readKernelFile(t, "runtime.config"), -1) {
		set[match[1]] = true
	}
	required := map[string]bool{}
	for _, match := range runtimeRequirePattern.FindAllStringSubmatch(readKernelFile(t, "runtime.require"), -1) {
		required[match[1]] = true
	}
	for symbol := range unverifiedRuntimeSymbols {
		if !set[symbol] {
			t.Errorf("unverifiedRuntimeSymbols names %s, which runtime.config no longer sets to y: drop the entry", symbol)
		}
		if required[symbol] {
			t.Errorf("unverifiedRuntimeSymbols names %s, which runtime.require now asserts: drop the entry, the build checks it", symbol)
		}
	}
}
