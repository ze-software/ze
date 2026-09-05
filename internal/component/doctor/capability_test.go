// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
//
// The predicates that decide whether a subsystem is in use, and the AC-9 rule
// that no enrolled probe executes a binary. The predicates live in
// internal/component/kernelcap and are read here, from a package that consumes
// all three of them: the capability gate, checkKernelModules and
// extractIPsecListeners (owner decision 6, 2026-08-14).

package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// ipsecTreeWith builds `vpn { ipsec { ... } }` and hands back the ipsec
// container so a case can fill it in.
func ipsecTreeWith(t *testing.T) (*config.Tree, *config.Tree) {
	t.Helper()
	tree := config.NewTree()
	return tree, tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")
}

// VALIDATES: AC-11 and A-6. An empty `vpn { ipsec { } }` block installs no
// Security Association, so IPsec is not in use: nothing is gated, no listener is
// opened, and checkKernelModules does not warn.
// PREVENTS: a refusal, two UDP binds and a module warning for a configuration
// that would have carried no packet. ParseIPsecConfig fills Peers only from
// site-to-site/peer and RemoteAccess only from remote-access
// (internal/component/ike/ipsec/config.go), so a block with neither describes no
// tunnel.
func TestIPsecNotInUseForEmptyBlock(t *testing.T) {
	tree, _ := ipsecTreeWith(t)
	assert.False(t, kernelcap.IPsecInUse(tree), "an empty vpn ipsec block installs no SA")
	assert.Empty(t, extractIPsecListeners(tree), "an empty block binds no listener")

	assert.False(t, kernelcap.IPsecInUse(config.NewTree()), "a config with no vpn block uses no IPsec")
	assert.False(t, kernelcap.IPsecInUse(nil), "a nil tree uses no IPsec")

	// A vpn block with no ipsec container inside it.
	vpnOnly := config.NewTree()
	vpnOnly.GetOrCreateContainer("vpn")
	assert.False(t, kernelcap.IPsecInUse(vpnOnly), "a vpn block with no ipsec container uses no IPsec")

	// The empty containers the parser creates for a block whose children were
	// all removed. Neither installs anything.
	tree, ipsec := ipsecTreeWith(t)
	ipsec.GetOrCreateContainer("site-to-site")
	assert.False(t, kernelcap.IPsecInUse(tree), "a site-to-site block with no peer installs no SA")
}

// VALIDATES: AC-12. One site-to-site peer makes IPsec in use, so the capability
// gate, the module check and the listener check all act on it.
// PREVENTS: under-reporting (R-2). A predicate tightened to end AC-11's false
// positives must not go silent on the configuration the gate exists for.
func TestIPsecInUseForOneSiteToSitePeer(t *testing.T) {
	tree, ipsec := ipsecTreeWith(t)
	peer := config.NewTree()
	peer.Set("remote-address", "203.0.113.7")
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", "branch", peer)

	assert.True(t, kernelcap.IPsecInUse(tree), "one site-to-site peer installs a Child SA")
	assert.Len(t, extractIPsecListeners(tree), 2, "IKE binds UDP 500 and UDP 4500")
}

// VALIDATES: the second shape that installs an SA. A remote-access block serves
// road-warrior clients and needs the same kernel dataplane.
// PREVENTS: a predicate that covers only the shape its author was looking at,
// which would let a remote-access VPN start on a kernel that cannot carry it.
func TestIPsecInUseForRemoteAccess(t *testing.T) {
	tree, ipsec := ipsecTreeWith(t)
	remote := ipsec.GetOrCreateContainer("remote-access")
	remote.Set("ike-group", "IKE-1")
	remote.Set("esp-group", "ESP-1")

	assert.True(t, kernelcap.IPsecInUse(tree), "a remote-access block installs Child SAs")
	assert.Len(t, extractIPsecListeners(tree), 2, "IKE binds UDP 500 and UDP 4500")
}

// VALIDATES: AC-7. An MPLS config on a VPP FIB backend is never judged on the
// kernel's AF_MPLS table. The `fib { kernel { } }` condition gates the predicate
// BEFORE any labeled family is looked at.
// PREVENTS: turning every VPP deployment that carries MPLS into a refusal. VPP
// does its own MPLS, and `fib/kernel` is what activates the plugin that programs
// labels into the kernel.
func TestMPLSCapabilityNotGatedOnVPPBackend(t *testing.T) {
	build := func(backend string) *config.Tree {
		tree := config.NewTree()
		tree.GetOrCreateContainer("fib").GetOrCreateContainer(backend)
		tree.GetOrCreateContainer("ldp")
		return tree
	}

	assert.True(t, kernelcap.MPLSInUse(build("kernel")), "LDP on the kernel FIB needs the kernel table")
	assert.False(t, kernelcap.MPLSInUse(build("vpp")), "VPP programs its own MPLS")
	assert.False(t, kernelcap.MPLSInUse(build("p4")), "another backend programs its own MPLS")

	noFIB := config.NewTree()
	noFIB.GetOrCreateContainer("ldp")
	assert.False(t, kernelcap.MPLSInUse(noFIB), "no fib block names no backend to gate")
}

// VALIDATES: AC-9. No enrolled probe executes an external binary. The probes all
// live in internal/component/kernelcap, so the rule is checked over that
// package's source: a probe changed to shell out to `ip` or `mpls` turns this
// test red.
// PREVENTS: the fault the enrolment exists to remove being used to detect
// itself. An external program is a second dependency that can be absent for its
// own reasons, so a probe that runs one answers a different question than the one
// it was asked.
//
// The rule is scoped to the ENROLLED probes, not to the process: checkVPPVersion
// in this package execs vppctl today, and VPP is not enrolled.
func TestCapabilityProbeExecsNoBinary(t *testing.T) {
	dir := filepath.Join("..", "kernelcap")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "the enrolled probes must be readable to be checked")

	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, readErr := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // a fixed path inside the repository
		require.NoError(t, readErr)
		scanned++
		for _, banned := range []string{`"os/exec"`, "exec.Command", "exec.CommandContext", "exec.LookPath", "syscall.Exec"} {
			assert.NotContains(t, string(body), banned,
				"%s runs an external binary; a capability probe reads the kernel through netlink, procfs or a syscall", name)
		}
	}
	require.Positive(t, scanned, "no probe source was scanned, so this test asserted nothing")
}
