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

// withXFRMNetlink swaps the XFRM netlink probe for the duration of a test.
func withXFRMNetlink(t *testing.T, err error) {
	t.Helper()
	original := xfrmNetlinkProbe
	xfrmNetlinkProbe = func() error { return err }
	t.Cleanup(func() { xfrmNetlinkProbe = original })
}

// ipsecConfigTree builds a config tree carrying vpn { ipsec { } }.
func ipsecConfigTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	return tree
}

// VALIDATES: AC-10. An appliance kernel builds XFRM in (CONFIG_XFRM_USER=y), so
// /proc/modules lists nothing for it. With the XFRM netlink socket open, the check
// reports no missing module.
// PREVENTS: the false error every appliance run produced. readLoadedModules parses
// /proc/modules, which holds LOADED MODULES ONLY, so a working built-in dataplane
// read as two missing modules and ze doctor exited 1 on a healthy host.
func TestKernelModulesBuiltInNotMissing(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{"nf_tables": true} }
	t.Cleanup(func() { loadedKernelModules = oldModules })
	withXFRMNetlink(t, nil)

	for i, d := range checkKernelModules(ipsecConfigTree()) {
		if strings.Contains(d.Message, "xfrm_user") || strings.Contains(d.Message, "xfrm_algo") {
			t.Errorf("diagnostic %d reports XFRM missing on a built-in kernel: %s", i, d.Message)
		}
	}
}

// VALIDATES: AC-10 does not weaken the check. A host whose XFRM netlink socket
// cannot be opened holds no IPsec dataplane at all, and that is still an error
// naming both modules and the netlink failure.
// PREVENTS: trading a false error for a false pass. A kernel with no XFRM installs
// no SA, which is exactly the readiness failure this check exists for.
func TestKernelModulesXFRMAbsentIsReported(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{"nf_tables": true} }
	t.Cleanup(func() { loadedKernelModules = oldModules })
	withXFRMNetlink(t, errors.New("protocol not supported"))

	diags := checkKernelModules(ipsecConfigTree())
	requireDiag(t, diags, "doctor-module-missing", diagnostic.SeverityError)
	found := false
	for _, d := range diags {
		if !strings.Contains(d.Message, "xfrm_user") {
			continue
		}
		found = true
		for _, want := range []string{"xfrm_algo", "protocol not supported"} {
			if !strings.Contains(d.Message, want) {
				t.Errorf("the message does not name %q: %s", want, d.Message)
			}
		}
	}
	if !found {
		t.Errorf("no diagnostic names xfrm_user: %+v", diags)
	}
}

// VALIDATES: AC-10 keeps the check scoped to a config that asks for IPsec. A host
// with no vpn ipsec container says nothing about XFRM, however broken its kernel.
// PREVENTS: a probe that runs on every doctor invocation and warns operators who
// never configured IPsec.
func TestKernelModulesXFRMSilentWithoutIPsecConfig(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })
	withXFRMNetlink(t, errors.New("protocol not supported"))

	for i, d := range checkKernelModules(config.NewTree()) {
		if strings.Contains(d.Message, "xfrm") {
			t.Errorf("diagnostic %d mentions XFRM without an ipsec config: %s", i, d.Message)
		}
	}
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

// fakeSysClassNet builds a sysfs-shaped tree where each named device carries the
// given MAC and an "up" operstate, and points the interface checks at it.
func fakeSysClassNet(t *testing.T, devices map[string]string) {
	t.Helper()
	root := t.TempDir()
	for device, mac := range devices {
		dir := filepath.Join(root, device)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "address"), []byte(mac+"\n"), 0o600); err != nil {
			t.Fatalf("write address: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "operstate"), []byte("up\n"), 0o600); err != nil {
			t.Fatalf("write operstate: %v", err)
		}
	}
	old := sysClassNetDir
	sysClassNetDir = root
	t.Cleanup(func() { sysClassNetDir = old })
}

// ethernetTree returns a config tree holding one netlink-backed ethernet entry.
func ethernetTree(name string, leaves, mac map[string]string) *config.Tree {
	tree := config.NewTree()
	ifaceBlock := tree.GetOrCreateContainer("interface")
	ifaceBlock.Set("backend", "netlink")
	entry := config.NewTree()
	for k, v := range leaves {
		entry.Set(k, v)
	}
	if len(mac) > 0 {
		macBlock := entry.GetOrCreateContainer("mac")
		for k, v := range mac {
			macBlock.Set(k, v)
		}
	}
	ifaceBlock.AddListEntry("ethernet", name, entry)
	return tree
}

// TestCheckInterfacesFollowsOSNameAlias verifies the doctor interface check
// judges an aliased entry by the device the alias names, not by the entry name.
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the doctor surface.
// PREVENTS: `ze doctor` calling a correct os-name config a missing interface,
// which is the same name-versus-device confusion the config apply path had.
func TestCheckInterfacesFollowsOSNameAlias(t *testing.T) {
	fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:01"})

	tree := ethernetTree("wan", map[string]string{"os-name": "enp1s0"}, nil)

	assertNoDiagCode(t, checkInterfaces(tree), "doctor-iface-missing")
}

// TestCheckInterfacesFollowsMACMatch verifies the check resolves a mac/match
// selector to the device carrying the address, and reports the two selector
// verdicts the config apply path distinguishes: an unmatched selector defers
// (warning), an ambiguous one is refused (error).
//
// VALIDATES: spec-fixit-iface-selector-ignored-by-apply, the doctor surface.
// PREVENTS: doctor staying silent about a selector that leaves an interface
// unconfigured, and calling a bound one missing.
func TestCheckInterfacesFollowsMACMatch(t *testing.T) {
	const wanted = "aa:bb:cc:00:00:01"

	t.Run("one device carries it", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "enp2s0": "aa:bb:cc:00:00:02"})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		diags := checkInterfaces(tree)
		assertNoDiagCode(t, diags, "doctor-iface-missing")
		assertNoDiagCode(t, diags, "doctor-iface-selector-unmatched")
		assertNoDiagCode(t, diags, "doctor-iface-selector-ambiguous")
	})

	t.Run("no device carries it", func(t *testing.T) {
		// A device named after the entry exists, so a name-based check would
		// report nothing at all while the interface stays unconfigured.
		fakeSysClassNet(t, map[string]string{"wan": "aa:bb:cc:00:00:02"})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		diags := checkInterfaces(tree)
		requireDiag(t, diags, "doctor-iface-selector-unmatched", diagnostic.SeverityWarning)
	})

	t.Run("two devices carry it", func(t *testing.T) {
		fakeSysClassNet(t, map[string]string{"enp1s0": wanted, "enp2s0": wanted})
		tree := ethernetTree("wan", nil, map[string]string{"match": wanted})
		diags := checkInterfaces(tree)
		requireDiag(t, diags, "doctor-iface-selector-ambiguous", diagnostic.SeverityError)
	})
}

// TestCheckInterfacesUnselectedEntryUnchanged pins the behavior of an entry with
// no selector: its name IS its device, and an absent one is still an error.
//
// VALIDATES: the selector work changes nothing for a plain entry.
// PREVENTS: the selector branches swallowing the missing-interface report.
func TestCheckInterfacesUnselectedEntryUnchanged(t *testing.T) {
	fakeSysClassNet(t, map[string]string{"enp1s0": "aa:bb:cc:00:00:01"})

	tree := ethernetTree("eth9", nil, nil)

	requireDiag(t, checkInterfaces(tree), "doctor-iface-missing", diagnostic.SeverityError)
}
