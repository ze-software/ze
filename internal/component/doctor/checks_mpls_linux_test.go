//go:build linux

package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
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

// withProcRoot points every procPath() lookup at a directory this test owns, so
// checkMPLSSupport reads a filesystem the test built instead of the host's
// /proc. Every MPLS test below sets one: without it the check reads the real
// kernel, and on an MPLS-capable host every "must be silent" assertion passes
// for the host's reason rather than for the test's.
func withProcRoot(t *testing.T, root string) {
	t.Helper()
	old := env.Get(doctorProcRootEnv)
	require.NoError(t, env.Set(doctorProcRootEnv, root))
	t.Cleanup(func() { _ = env.Set(doctorProcRootEnv, old) })
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
// as root under make ze-qemu-needs-linux-test.
func procRootWithUnreadableMPLS(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sys", "net"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sys", "net", "mpls"), nil, 0o644))
	return root
}

// VALIDATES: checkMPLSSupport decides on the CAPABILITY probe and not on the
// loaded-module list. With the module list empty and net.mpls.platform_labels
// present the check is silent; with neither present it warns; with the probe
// unreadable it reports doctor-mpls-unknown.
// PREVENTS: a false doctor-mpls-unavailable on every appliance. ze's runtime
// kernel builds MPLS in (CONFIG_MPLS_ROUTING=y, gokrazy/kernel/runtime.config),
// so no mpls_router.ko exists and readLoadedModules can never list one. The
// module-list check would have warned that MPLS is missing on the one kernel ze
// ships that forwards it.
func TestCheckMPLSSupportProbesCapabilityNotModuleList(t *testing.T) {
	t.Run("builtin-mpls-with-empty-module-list-is-silent", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, procRootWithMPLS(t))

		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-label")),
			"a builtin AF_MPLS table lists no module and MUST NOT be reported as missing")
	})

	t.Run("no-mpls-anywhere-warns", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, t.TempDir())

		diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-mpls-unavailable", diags[0].Code)
		assert.Contains(t, diags[0].Message, "platform_labels")
	})

	t.Run("present-but-zero-is-disabled", func(t *testing.T) {
		// The state every appliance boots in. ze's runtime kernel builds MPLS in,
		// so af_mpls creates the sysctl, and the sysctl's own default is 0, which
		// disables MPLS entirely (docs/guide/mpls.md). A check that stopped at the
		// stat went SILENT here, on exactly the machines building MPLS in was
		// meant to serve.
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, procRootWithMPLSLabels(t, "0\n"))

		diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-mpls-disabled", diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Contains(t, diags[0].Message, "sysctl {}")
	})

	t.Run("one-label-is-enough-to-be-silent", func(t *testing.T) {
		// The boundary. 0 is the only disabling value; 1 is a working, if tiny,
		// label space and must not warn.
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, procRootWithMPLSLabels(t, "1\n"))

		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-label")),
			"a label space of 1 forwards, so it MUST NOT be reported as disabled")
	})

	t.Run("unreadable-probe-is-unknown", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, procRootWithUnreadableMPLS(t))

		diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-mpls-unknown", diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	})
}

func codes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	// Indexed, not ranged by value: Diagnostic is 184 bytes (gocritic
	// rangeValCopy), and only Code is read here.
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

// VALIDATES: with a kernel FIB, a labeled BGP family and no AF_MPLS table,
// checkMPLSSupport emits doctor-mpls-unavailable even though both MPLS modules
// are listed as loaded. The module list cannot silence the check.
// PREVENTS: the module list creeping back in as the deciding evidence. It is
// wrong in both directions: it reports MPLS absent on a kernel that builds it in,
// and it would report MPLS present on a host where the modules are loaded but
// af_mpls registered no forwarding table.
// It also still covers the silent regression test/plugin/mpls-doctor.ci was
// supposed to catch and could not. That .ci is skip-os everywhere but Linux, so
// it had never run until CI existed, and this check had NO behavioral unit test
// at all -- TestDoctorDependencyInventory only asserts the code is registered,
// which stays true however broken the check is.
func TestCheckMPLSSupportWarnsEvenWhenModulesAreLoaded(t *testing.T) {
	withModules(t, func() map[string]bool {
		return map[string]bool{"mpls_router": true, "mpls_iptunnel": true}
	})
	withProcRoot(t, t.TempDir())

	diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-mpls-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "platform_labels")
	assert.Contains(t, diags[0].Message, "AF_MPLS")
}

// VALIDATES: the check is silent once the kernel holds an AF_MPLS table, and
// when the config gives it no reason to care.
// PREVENTS: a warning on every kernel-FIB config that imposes no labels, and a
// warning on a host that is in fact ready.
// Every subtest sets a proc root, including the two the config gate decides. A
// silence that the probe could also explain proves nothing about the gate, and
// on an MPLS-capable developer host that is exactly what it would be.
func TestCheckMPLSSupportSilentWhenNotApplicable(t *testing.T) {
	t.Run("kernel-holds-an-af-mpls-table", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, procRootWithMPLS(t))
		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-label")))
	})

	t.Run("no-labeled-family", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, t.TempDir())
		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/unicast")),
			"a plain unicast peer over the kernel FIB imposes no labels")
	})

	t.Run("no-kernel-fib", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		withProcRoot(t, t.TempDir())
		tree := mplsTree(t, "ipv4/mpls-label")
		tree.RemoveContainer("fib")
		assert.Empty(t, checkMPLSSupport(tree), "MPLS support only matters for the kernel FIB")
	})
}

// VALIDATES: every family name the check treats as labeled is one a peer can
// actually negotiate, driven from the registered family list.
// PREVENTS: exactly the defect mpls-doctor.ci carried -- it declared
// `ipv4/mpls-unicast`, a family that exists nowhere in ze, so mplsInUse was
// false and the assertion could never fire. A name-level mismatch between the
// check and the config surface is invisible until someone reads both.
func TestMPLSInUseNamesRealFamilies(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })
	withProcRoot(t, t.TempDir())

	for _, family := range []string{"ipv4/mpls-label", "ipv6/mpls-label", "ipv4/mpls-vpn", "ipv6/mpls-vpn"} {
		t.Run(family, func(t *testing.T) {
			assert.NotEmpty(t, checkMPLSSupport(mplsTree(t, family)),
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
		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-unicast")),
			"a family that does not exist must not satisfy the MPLS check")
	})
}

// VALIDATES: an unreadable module list no longer reaches checkMPLSSupport at
// all. The capability probe answers, and a nil module map changes neither
// answer.
// PREVENTS: the old nil-map branch returning. It read an unreadable
// /proc/modules as doctor-mpls-unknown, which is the right shape for the wrong
// evidence: after the probe replaced the module list, a nil map says nothing
// about whether the kernel forwards labels, so reporting "unknown" for it would
// hide a working kernel behind a warning. The unknown answer now belongs to an
// unreadable PROBE, which TestCheckMPLSSupportProbesCapabilityNotModuleList
// covers.
func TestCheckMPLSSupportIgnoresUnreadableModuleList(t *testing.T) {
	t.Run("capability-present", func(t *testing.T) {
		withModules(t, func() map[string]bool { return nil })
		withProcRoot(t, procRootWithMPLS(t))

		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-label")))
	})

	t.Run("capability-absent", func(t *testing.T) {
		withModules(t, func() map[string]bool { return nil })
		withProcRoot(t, t.TempDir())

		diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-mpls-unavailable", codes(diags)[0])
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
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
