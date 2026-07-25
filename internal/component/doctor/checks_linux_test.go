//go:build linux

// Design: docs/features/ai-first.md -- Linux doctor coverage tests

package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
)

func TestCheckKernelModules_L2TP(t *testing.T) {
	// VALIDATES: AC-1 L2TP config without l2tp_ppp/pppol2tp returns doctor-l2tp-module.
	// PREVENTS: L2TP kernel module readiness being hidden behind generic module diagnostics.
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	tree := config.NewTree()
	tree.GetOrCreateContainer("l2tp")

	diags := checkKernelModules(tree)
	requireDiag(t, diags, "doctor-l2tp-module", diagnostic.SeverityError)
}

func TestCheckKernelModules_PPPoE(t *testing.T) {
	// VALIDATES: AC-2 PPPoE config without pppoe returns doctor-pppoe-module.
	// PREVENTS: PPPoE kernel module readiness being hidden behind generic module diagnostics.
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	tree := config.NewTree()
	tree.GetOrCreateContainer("pppoe")

	diags := checkKernelModules(tree)
	requireDiag(t, diags, "doctor-pppoe-module", diagnostic.SeverityError)
}

func TestCheckFirewallBackend(t *testing.T) {
	// VALIDATES: AC-3 firewall config without nf_tables returns doctor-firewall-nftables.
	// PREVENTS: nftables kernel support gaps being missed until firewall apply.
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	tree := config.NewTree()
	tree.GetOrCreateContainer("firewall")

	diags := checkFirewallBackend(tree)
	requireDiag(t, diags, "doctor-firewall-nftables", diagnostic.SeverityWarning)
}

func TestCheckTelemetryProcfs(t *testing.T) {
	// VALIDATES: AC-11 telemetry with unreadable /proc returns doctor-telemetry-procfs.
	// PREVENTS: OS collector readiness being assumed from config alone.
	oldRead := readFilePath
	readFilePath = func(string) ([]byte, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { readFilePath = oldRead })

	tree := config.NewTree()
	telemetry := tree.GetOrCreateContainer("telemetry")
	prom := telemetry.GetOrCreateContainer("prometheus")
	prom.Set("enabled", "true")

	diags := checkTelemetryProcfs(tree)
	requireDiag(t, diags, "doctor-telemetry-procfs", diagnostic.SeverityWarning)
}

func TestCheckSysctlProcfs(t *testing.T) {
	// VALIDATES: AC-15 sysctl config with unwritable /proc/sys returns doctor-sysctl-procfs.
	// PREVENTS: sysctl apply failures being deferred to daemon startup.
	oldAccess := accessPath
	accessPath = func(string, uint32) error { return os.ErrPermission }
	t.Cleanup(func() { accessPath = oldAccess })

	tree := config.NewTree()
	sysctl := tree.GetOrCreateContainer("sysctl")
	setting := config.NewTree()
	setting.Set("value", "1")
	sysctl.AddListEntry("setting", "net.ipv4.ip_forward", setting)

	diags := checkSysctlProcfs(tree)
	requireDiag(t, diags, "doctor-sysctl-procfs", diagnostic.SeverityWarning)
}

func TestCheckConntrackProcfs(t *testing.T) {
	// VALIDATES: AC-16 conntrack tuning with missing procfs sysctl path returns doctor-conntrack-procfs.
	// PREVENTS: conntrack procfs dependency gaps being missed during readiness checks.
	oldStat := statPath
	oldAccess := accessPath
	// os.DevNull always exists, so this simulates a present file (nil error).
	statPath = func(string) (os.FileInfo, error) { return os.Stat(os.DevNull) }
	accessPath = func(path string, _ uint32) error {
		if strings.HasSuffix(path, "nf_conntrack_max") {
			return os.ErrNotExist
		}
		return nil
	}
	t.Cleanup(func() {
		statPath = oldStat
		accessPath = oldAccess
	})

	tree := config.NewTree()
	system := tree.GetOrCreateContainer("system")
	system.GetOrCreateContainer("conntrack")

	diags := checkConntrackProcfs(tree)
	requireDiag(t, diags, "doctor-conntrack-procfs", diagnostic.SeverityWarning)
}

func TestCheckPolicyRouteNetlink(t *testing.T) {
	// VALIDATES: Policy routing config with unavailable route netlink returns doctor-policyroute-netlink.
	// PREVENTS: policy route netlink dependency gaps being missed during readiness checks.
	oldHandle := newRouteNetlinkHandle
	newRouteNetlinkHandle = func() (routeNetlinkHandle, error) { return nil, errors.New("netlink unavailable") }
	t.Cleanup(func() { newRouteNetlinkHandle = oldHandle })

	tree := config.NewTree()
	policy := tree.GetOrCreateContainer("policy")
	route := config.NewTree()
	route.AddListEntry("rule", "r1", config.NewTree())
	policy.AddListEntry("route", "pbr", route)

	diags := checkPolicyRouteNetlink(tree)
	requireDiag(t, diags, "doctor-policyroute-netlink", diagnostic.SeverityWarning)
}

func TestCheckNTPClockPrivilege_Missing(t *testing.T) {
	oldRead := readFilePath
	oldUID := currentUID
	readFilePath = func(string) ([]byte, error) {
		return []byte("Name:\tze\nCapEff:\t0000000000000000\n"), nil
	}
	currentUID = func() int { return 1000 }
	t.Cleanup(func() {
		readFilePath = oldRead
		currentUID = oldUID
	})

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ntp := env.GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")

	diags := checkNTPClockPrivilege(tree)
	requireDiag(t, diags, "doctor-ntp-clock-privilege", diagnostic.SeverityWarning)
}

func TestCheckNTPClockPrivilege_Present(t *testing.T) {
	oldRead := readFilePath
	oldUID := currentUID
	readFilePath = func(string) ([]byte, error) {
		return []byte("Name:\tze\nCapEff:\t0000000002000000\n"), nil
	}
	currentUID = func() int { return 1000 }
	t.Cleanup(func() {
		readFilePath = oldRead
		currentUID = oldUID
	})

	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	ntp := env.GetOrCreateContainer("ntp")
	ntp.Set("enabled", "true")

	diags := checkNTPClockPrivilege(tree)
	assert.Empty(t, diags)
}

func TestCheckNTPClockPrivilege_Disabled(t *testing.T) {
	tree := config.NewTree()
	diags := checkNTPClockPrivilege(tree)
	assert.Empty(t, diags)
}

func TestCheckVPPDPDK_ModuleMissing(t *testing.T) {
	oldModules := loadedKernelModules
	oldStat := statPath
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	statPath = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() {
		loadedKernelModules = oldModules
		statPath = oldStat
	})

	tree := config.NewTree()
	iface := tree.GetOrCreateContainer("interface")
	iface.Set("backend", "vpp")
	vpp := tree.GetOrCreateContainer("vpp")
	dpdk := vpp.GetOrCreateContainer("dpdk")
	pci := config.NewTree()
	pci.Set("name", "xe0")
	dpdk.AddListEntry("interface", "0000:03:00.0", pci)

	diags := checkVPPDPDK(tree)
	foundModule := false
	foundPCI := false
	for i := range diags {
		if diags[i].Code == "doctor-vpp-dpdk" && strings.Contains(diags[i].Message, "VFIO") {
			foundModule = true
		}
		if diags[i].Code == "doctor-vpp-dpdk" && strings.Contains(diags[i].Message, "PCI") {
			foundPCI = true
		}
	}
	assert.True(t, foundModule, "expected VFIO module diagnostic")
	assert.True(t, foundPCI, "expected PCI device diagnostic")
}

func TestCheckVPPDPDK_NoInterfaces(t *testing.T) {
	tree := config.NewTree()
	iface := tree.GetOrCreateContainer("interface")
	iface.Set("backend", "vpp")
	tree.GetOrCreateContainer("vpp")

	diags := checkVPPDPDK(tree)
	assert.Empty(t, diags)
}

func TestCheckVPPDPDK_NotVPP(t *testing.T) {
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface")
	diags := checkVPPDPDK(tree)
	assert.Empty(t, diags)
}

func TestCheckKernelModules_L2TPOneAccepted(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{"pppol2tp": true} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	tree := config.NewTree()
	tree.GetOrCreateContainer("l2tp")

	diags := checkKernelModules(tree)
	for i := range diags {
		assert.NotEqual(t, "doctor-l2tp-module", diags[i].Code)
	}
}

func TestCheckRandomSeedGokrazyMissing(t *testing.T) {
	oldStat := statPath
	statPath = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { statPath = oldStat })

	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformGokrazy})
	requireDiag(t, diags, "doctor-random-seed", diagnostic.SeverityWarning)
	assert.Contains(t, diags[0].Message, "/perm/random.seed")
}

func TestCheckRandomSeedGokrazyPresent(t *testing.T) {
	oldStat := statPath
	// os.DevNull always exists, so this simulates a present file (nil error).
	statPath = func(string) (os.FileInfo, error) { return os.Stat(os.DevNull) }
	t.Cleanup(func() { statPath = oldStat })

	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformGokrazy})
	assert.Empty(t, diags)
}

func TestCheckRandomSeedSystemdMissing(t *testing.T) {
	oldStat := statPath
	statPath = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { statPath = oldStat })

	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformSystemd})
	requireDiag(t, diags, "doctor-random-seed", diagnostic.SeverityWarning)
	assert.Contains(t, diags[0].Message, "/var/lib/systemd/random-seed")
}

func TestCheckRandomSeedSystemdPresent(t *testing.T) {
	oldStat := statPath
	// os.DevNull always exists, so this simulates a present file (nil error).
	statPath = func(string) (os.FileInfo, error) { return os.Stat(os.DevNull) }
	t.Cleanup(func() { statPath = oldStat })

	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformSystemd})
	assert.Empty(t, diags)
}

func TestCheckRandomSeedPlainLinux(t *testing.T) {
	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformPlainLinux})
	requireDiag(t, diags, "doctor-random-seed", diagnostic.SeverityWarning)
	assert.Contains(t, diags[0].Message, "non-systemd Linux")
}

func TestCheckRandomSeedNilPlatform(t *testing.T) {
	diags := checkRandomSeed(nil)
	assert.Empty(t, diags)
}

func TestCheckRandomSeedPathOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-seed")
	if err := env.Set(doctorRandomSeedEnv, path); err != nil {
		t.Fatalf("set %s: %v", doctorRandomSeedEnv, err)
	}
	t.Cleanup(func() { _ = env.Set(doctorRandomSeedEnv, "") })

	diags := checkRandomSeed(&host.PlatformInfo{Type: host.PlatformGokrazy})
	requireDiag(t, diags, "doctor-random-seed", diagnostic.SeverityWarning)
	assert.Equal(t, path, diags[0].Path)
}

func TestCheckMachineIDMissingGokrazy(t *testing.T) {
	// VALIDATES: AC-13 Gokrazy without /etc/machine-id emits doctor-machine-id-missing.
	// PREVENTS: appliance identity gaps being missed until services need a machine ID.
	oldRead := readFilePath
	readFilePath = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	t.Cleanup(func() { readFilePath = oldRead })

	diags := checkMachineID(&host.PlatformInfo{Type: host.PlatformGokrazy}, nil)

	requireDiag(t, diags, "doctor-machine-id-missing", diagnostic.SeverityWarning)
}

func TestCheckMachineIDPresentSystemd(t *testing.T) {
	// VALIDATES: AC-14 Systemd with /etc/machine-id emits no machine-id diagnostic.
	// PREVENTS: false positives on standard Linux hosts with a valid machine ID.
	oldRead := readFilePath
	readFilePath = func(string) ([]byte, error) { return []byte("00112233445566778899aabbccddeeff\n"), nil }
	t.Cleanup(func() { readFilePath = oldRead })

	diags := checkMachineID(&host.PlatformInfo{Type: host.PlatformSystemd}, nil)

	assert.Empty(t, diags)
}

func TestCheckMachineIDPathOverride(t *testing.T) {
	// VALIDATES: the private machine-id path override drives deterministic functional-test fixtures.
	// PREVENTS: doctor-machine-id-missing coverage depending on the host's real /etc/machine-id.
	path := filepath.Join(t.TempDir(), "missing-machine-id")
	if err := env.Set(doctorMachineIDEnv, path); err != nil {
		t.Fatalf("set %s: %v", doctorMachineIDEnv, err)
	}
	t.Cleanup(func() { _ = env.Set(doctorMachineIDEnv, "") })

	diags := checkMachineID(&host.PlatformInfo{Type: host.PlatformGokrazy}, nil)

	requireDiag(t, diags, "doctor-machine-id-missing", diagnostic.SeverityWarning)
	for i := range diags {
		if diags[i].Code == "doctor-machine-id-missing" {
			assert.Equal(t, path, diags[i].Path)
		}
	}
}
