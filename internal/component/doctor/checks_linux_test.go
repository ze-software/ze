//go:build linux

// Design: docs/features/ai-first.md -- Linux doctor coverage tests

package doctor

import (
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

// ipsecConfigTree builds a config tree carrying an EMPTY vpn { ipsec { } }.
// It installs no Security Association, so nothing about it is in use.
func ipsecConfigTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	return tree
}

// ipsecPeerConfigTree builds a config tree carrying one site-to-site peer, which
// is what makes IPsec in use (AC-11, AC-12).
func ipsecPeerConfigTree() *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
	peer := config.NewTree()
	peer.Set("remote-address", "203.0.113.7")
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", "branch", peer)
	return tree
}

// VALIDATES: AC-11. An empty vpn ipsec block describes no tunnel, so the module
// check says nothing about it, whatever the host's module list holds.
// PREVENTS: over-reporting. Under a refusal an over-reporting predicate stops a
// working router (R-3), and the same predicate decides the module rows, the
// listener binds and the startup gate (owner decision 6).
func TestKernelModulesSilentForAnEmptyIPsecBlock(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	for i, d := range checkKernelModules(ipsecConfigTree()) {
		if strings.Contains(d.Message, "IPsec") {
			t.Errorf("diagnostic %d reports on an empty ipsec block: %s", i, d.Message)
		}
	}
}

// VALIDATES: a configured IPsec peer on a host with neither firewall table still
// gets its marking warning, so the shared predicate did not silence the rows that
// remain here.
// PREVENTS: under-reporting (R-2). The predicate that ends over-reporting must not
// also end the reporting that was correct.
func TestKernelModulesWarnsForAConfiguredIPsecPeer(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	requireDiag(t, checkKernelModules(ipsecPeerConfigTree()), "doctor-module-missing", diagnostic.SeverityWarning)
}

// VALIDATES: the module check speaks about no XFRM row at all. The kernel XFRM
// dataplane is an enrolled capability (internal/component/kernelcap), which
// answers once for ze doctor, for the startup refusal and for config validate.
// PREVENTS: two reporters for one fact. The module list cannot answer this
// question anyway: an appliance kernel builds XFRM in, so xfrm_user appears in no
// module row and a working dataplane read as two missing modules.
func TestKernelModulesLeavesXFRMToTheCapability(t *testing.T) {
	oldModules := loadedKernelModules
	loadedKernelModules = func() map[string]bool { return map[string]bool{} }
	t.Cleanup(func() { loadedKernelModules = oldModules })

	for _, tree := range []*config.Tree{config.NewTree(), ipsecConfigTree(), ipsecPeerConfigTree()} {
		for i, d := range checkKernelModules(tree) {
			if strings.Contains(d.Message, "xfrm") || strings.Contains(d.Message, "XFRM") {
				t.Errorf("diagnostic %d still reports XFRM from the module check: %s", i, d.Message)
			}
		}
	}
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
