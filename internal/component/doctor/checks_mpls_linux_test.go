//go:build linux

package doctor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"

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

func codes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	// Indexed, not ranged by value: Diagnostic is 184 bytes (gocritic
	// rangeValCopy), and only Code is read here.
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

// VALIDATES: with a kernel FIB, a labeled BGP family, and neither MPLS module
// loaded, checkMPLSSupport emits doctor-mpls-unavailable naming both modules.
// PREVENTS: the silent regression test/plugin/mpls-doctor.ci was supposed to
// catch and could not. That .ci is skip-os everywhere but Linux, so it had never
// run until CI existed, and this check had NO behavioral unit test at all --
// TestDoctorDependencyInventory only asserts the code is registered, which stays
// true however broken the check is.
func TestCheckMPLSSupportWarnsWhenModulesAbsent(t *testing.T) {
	withModules(t, func() map[string]bool { return map[string]bool{} })

	diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-mpls-unavailable", diags[0].Code)
	assert.Contains(t, diags[0].Message, "mpls_router")
	assert.Contains(t, diags[0].Message, "mpls_iptunnel")
}

// VALIDATES: the check is silent once both modules are loaded, and when the
// config gives it no reason to care.
// PREVENTS: a warning on every kernel-FIB config that imposes no labels, and a
// warning on a host that is in fact ready.
func TestCheckMPLSSupportSilentWhenNotApplicable(t *testing.T) {
	t.Run("modules-loaded", func(t *testing.T) {
		withModules(t, func() map[string]bool {
			return map[string]bool{"mpls_router": true, "mpls_iptunnel": true}
		})
		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/mpls-label")))
	})

	t.Run("no-labeled-family", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		assert.Empty(t, checkMPLSSupport(mplsTree(t, "ipv4/unicast")),
			"a plain unicast peer over the kernel FIB imposes no labels")
	})

	t.Run("no-kernel-fib", func(t *testing.T) {
		withModules(t, func() map[string]bool { return map[string]bool{} })
		tree := mplsTree(t, "ipv4/mpls-label")
		tree.RemoveContainer("fib")
		assert.Empty(t, checkMPLSSupport(tree), "MPLS modules only matter for the kernel FIB")
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

// VALIDATES: an unreadable module list is REPORTED, not passed over.
// PREVENTS: the failure mode that made diagnosing mpls-doctor.ci expensive --
// with /proc/modules unreadable the check returned nil, so a broken check and a
// healthy host produced byte-identical output.
func TestCheckMPLSSupportReportsUnreadableModuleList(t *testing.T) {
	withModules(t, func() map[string]bool { return nil })

	diags := checkMPLSSupport(mplsTree(t, "ipv4/mpls-label"))

	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-mpls-unknown", codes(diags)[0])
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
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
