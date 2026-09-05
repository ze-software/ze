//go:build linux

package doctor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/env"

	// The BGP YANG module is what makes `bgp {` a known top-level keyword. The
	// doctor package does not import it in production (it navigates the tree
	// generically), so without this blank import the fixtures below fail to
	// parse with "unknown top-level keyword: bgp" and the tests would be
	// asserting on nothing.
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
)

// mplsTree builds the tree test/plugin/mpls-doctor.ci configures by PARSING the
// config text, not by hand-assembling containers.
//
// This is load-bearing. A hand-built tree that puts the family in a
// GetOrCreateContainer("family") passes against a check that reads
// GetContainer("family") -- and that is exactly the bug that shipped: `family
// ipv4/mpls-label { ... }` parses to a LIST entry keyed by the family name, so
// the real tree never had a "family" container and containerPeersLabeled was
// dead code. Parsing means the test cannot disagree with the parser about the
// shape it is asserting on.
func mplsTree(t *testing.T, family string) *config.Tree {
	t.Helper()
	text := `
bgp {
	peer peer1 {
		connection {
			remote { ip 10.0.0.2; }
			local { ip 10.0.0.1; }
		}
		session {
			asn { local 65001; remote 65002; }
			router-id 1.2.3.4
			family { ` + family + ` { prefix { maximum 10000; } } }
		}
	}
}
fib {
	kernel { }
}
`
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(text)
	require.NoError(t, err, "fixture config must parse")
	return tree
}

// withModules swaps the loaded-module reader for the duration of a test.
func withModules(t *testing.T, fn func() map[string]bool) {
	t.Helper()
	old := loadedKernelModules
	loadedKernelModules = fn
	t.Cleanup(func() { loadedKernelModules = old })
}

// procRootEnv names the /proc root every doctor-tier probe reads. It is
// registered by internal/component/kernelcap, which owns ProcPath.
const procRootEnv = "ze.test.doctor.procfs-root"

// withProcRoot points every kernelcap.ProcPath lookup at a directory this test
// owns, so the MPLS probe reads a filesystem the test built instead of the
// host's /proc. Every MPLS test below sets one: without it the probe reads the
// real kernel, and on an MPLS-capable host every "must be silent" assertion
// passes for the host's reason rather than for the test's.
func withProcRoot(t *testing.T, root string) {
	t.Helper()
	old := env.Get(procRootEnv)
	require.NoError(t, env.Set(procRootEnv, root))
	t.Cleanup(func() { _ = env.Set(procRootEnv, old) })
}

// procRootWithMPLS builds a fake /proc carrying net.mpls.platform_labels, the
// sysctl af_mpls creates when the kernel holds an AF_MPLS forwarding table. It
// creates no module list, which is the shape of a kernel built with
// CONFIG_MPLS_ROUTING=y.
func procRootWithMPLS(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "sys", "net", "mpls")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "platform_labels"), []byte("1048575\n"), 0o644))
	return root
}

// procRootWithMPLSLabels writes an arbitrary value into the capability probe, so
// a test can drive the VALUE rather than only the file's presence.
func procRootWithMPLSLabels(t *testing.T, value string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "sys", "net", "mpls")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "platform_labels"), []byte(value), 0o644))
	return root
}

// procRootWithUnreadableMPLS makes sys/net/mpls a regular FILE, so a stat of the
// sysctl below it fails with ENOTDIR. That is an error which is not
// fs.ErrNotExist, and the filesystem produces it whatever user the test runs as.
// A chmod 000 would not do it, because root ignores the mode and this suite runs
// as root under `./le qemu all-tests`.
func procRootWithUnreadableMPLS(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "net"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sys", "net", "mpls"), nil, 0o644))
	return root
}

// VALIDATES: AC-8. The MPLS probe decides on the CAPABILITY and not on the
// loaded-module list. A kernel that carries net.mpls.platform_labels and lists
// no module reads as PRESENT; a /proc with neither reads as ABSENT; a probe that
// cannot be read reads as UNKNOWN.
// PREVENTS: a false refusal on every appliance. ze's runtime kernel builds MPLS
// in (CONFIG_MPLS_ROUTING=y, gokrazy/kernel/runtime.config), so no mpls_router.ko
// exists and readLoadedModules can never list one. Under a refusal a module-list
// probe would stop the one kernel ze ships that forwards labels.
func TestMPLSCapabilityReadsBuiltInKernel(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })
	withProcRoot(t, procRootWithMPLS(t))

	assert.Equal(t, kernelcap.StatePresent, kernelcap.MPLS().State,
		"a builtin AF_MPLS table lists no module and MUST read as present")
}

// VALIDATES: absence is ENOENT and nothing else. That is the one state that
// refuses a start.
// PREVENTS: a read failure of any other kind being reported as a missing kernel
// feature, which would send an operator to rebuild a kernel that is already right.
func TestMPLSCapabilityAbsentIsENOENT(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })
	withProcRoot(t, t.TempDir())

	result := kernelcap.MPLS()
	assert.Equal(t, kernelcap.StateAbsent, result.State)
	require.Error(t, result.Reason, "an absent capability must carry its evidence")
	assert.True(t, errors.Is(result.Reason, fs.ErrNotExist), "absence is ENOENT: %v", result.Reason)
}

// VALIDATES: AC-6. A probe that cannot reach its evidence reports cannot-determine
// and carries the reason, so the gate warns and ze still starts.
// PREVENTS: a working deployment turned dead by an unreadable probe (R-1), and a
// guard reporting an answer it did not get (ai/rules/evidence.md).
func TestMPLSCapabilityUnreadableIsUnknown(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })
	withProcRoot(t, procRootWithUnreadableMPLS(t))

	result := kernelcap.MPLS()
	assert.Equal(t, kernelcap.StateUnknown, result.State)
	require.Error(t, result.Reason)
	assert.False(t, errors.Is(result.Reason, fs.ErrNotExist),
		"an unreadable probe is not an absent one: %v", result.Reason)
}

// VALIDATES: AC-10. A label space of 0 is PRESENT, not a fault. The table exists,
// and ze writes a non-zero size before it programs its first label
// (internal/plugins/fib/kernel/labelspace_linux.go).
// PREVENTS: a doctor row an operator cannot act on. Every appliance boots in this
// state, because ze's runtime kernel builds MPLS in and the sysctl still defaults
// to 0; reporting it would fire on every appliance for a fault ze repairs itself.
func TestMPLSCapabilityPresentWhenLabelSpaceIsZero(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })
	withProcRoot(t, procRootWithMPLSLabels(t, "0\n"))

	assert.Equal(t, kernelcap.StatePresent, kernelcap.MPLS().State,
		"an existing table with an empty label space is present and repairable")
}

// VALIDATES: the module list cannot silence the capability. Both MPLS modules
// listed as loaded on a host with no AF_MPLS table still reads as absent.
// PREVENTS: the module list creeping back in as the deciding evidence. It is
// wrong in both directions: it reports MPLS absent on a kernel that builds it in,
// and it would report MPLS present on a host where the modules are loaded but
// af_mpls registered no forwarding table.
func TestMPLSCapabilityIgnoresTheModuleList(t *testing.T) {
	for name, modules := range map[string]map[string]bool{
		"both modules loaded":    {"mpls_router": true, "mpls_iptunnel": true},
		"module list unreadable": nil,
	} {
		t.Run(name, func(t *testing.T) {
			withModules(t, func() map[string]bool { return modules })
			withProcRoot(t, t.TempDir())
			assert.Equal(t, kernelcap.StateAbsent, kernelcap.MPLS().State)

			withProcRoot(t, procRootWithMPLS(t))
			assert.Equal(t, kernelcap.StatePresent, kernelcap.MPLS().State)
		})
	}
}

// VALIDATES: every family name the predicate treats as labeled is one a peer can
// actually negotiate, driven from a PARSED config.
// PREVENTS: exactly the defect mpls-doctor.ci carried -- it declared
// `ipv4/mpls-unicast`, a family that exists nowhere in ze, so the predicate was
// false and the assertion could never fire. A name-level mismatch between the
// predicate and the config surface is invisible until someone reads both.
func TestMPLSInUseNamesRealFamilies(t *testing.T) {
	for _, family := range []string{"ipv4/mpls-label", "ipv6/mpls-label", "ipv4/mpls-vpn", "ipv6/mpls-vpn"} {
		t.Run(family, func(t *testing.T) {
			assert.True(t, kernelcap.MPLSInUse(mplsTree(t, family)),
				"%s must count as MPLS forwarding", family)
		})
	}

	// The family mpls-doctor.ci used to declare does not exist. The PARSER does
	// not reject it -- `family` is a list with a free-form key, and the
	// registered-family check lives in ValidateBGPPeers, not in Parse (which is
	// why `ze config validate` rejects it while `ze doctor` did not; see
	// checkBGPPeerConfig in checks_config.go). What matters here is narrower and
	// is what the .ci depended on: an unregistered name must not count as MPLS
	// forwarding.
	t.Run("unregistered-family-does-not-count", func(t *testing.T) {
		assert.False(t, kernelcap.MPLSInUse(mplsTree(t, "ipv4/mpls-unicast")),
			"a family that does not exist must not satisfy the MPLS predicate")
	})

	// AC-7. `fib { kernel { } }` gates the predicate BEFORE any labeled family,
	// so an MPLS config on a VPP or P4 backend is never judged on the kernel's
	// AF_MPLS table. Losing this gate turns every VPP deployment with MPLS into a
	// refusal.
	t.Run("another-fib-backend-is-never-gated", func(t *testing.T) {
		tree := mplsTree(t, "ipv4/mpls-label")
		tree.RemoveContainer("fib")
		assert.False(t, kernelcap.MPLSInUse(tree), "MPLS support only matters for the kernel FIB")
	})
}

// VALIDATES: readLoadedModules parses a /proc/modules-shaped file, and reports
// an unreadable path as nil rather than as an empty set.
// PREVENTS: "file missing" and "no modules loaded" collapsing into one answer,
// which is what let the stub path in mpls-doctor.ci fail invisibly.
func TestReadLoadedModulesDistinguishesEmptyFromUnreadable(t *testing.T) {
	oldRead := readFilePath
	t.Cleanup(func() { readFilePath = oldRead })

	readFilePath = func(string) ([]byte, error) {
		return []byte("mpls_router 32768 1 mpls_iptunnel, Live 0x0\nmpls_iptunnel 16384 0 - Live 0x0\n"), nil
	}
	loaded := readLoadedModules()
	require.NotNil(t, loaded)
	assert.True(t, loaded["mpls_router"])
	assert.True(t, loaded["mpls_iptunnel"])

	readFilePath = func(string) ([]byte, error) { return []byte(""), nil }
	empty := readLoadedModules()
	require.NotNil(t, empty, "an empty file means no modules, which is a real answer")
	assert.Empty(t, empty)

	readFilePath = func(string) ([]byte, error) { return nil, errors.New("permission denied") }
	assert.Nil(t, readLoadedModules(), "an unreadable file must be nil, not an empty set")
}
