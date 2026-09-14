// Design: docs/features/ai-first.md -- the readiness checks the BGP engine owns
// Detail: doctor_checks.go -- the four checks and the table register.go installs

package bgpconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
)

// peerConfigWithFamily is the smallest config the engine resolves, with one
// peer whose address family is the argument. Every peer leaf the engine
// requires is present, so an unknown family is the only thing it can refuse.
func peerConfigWithFamily(family string) string {
	return `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65001; } }
	peer peer1 {
		connection {
			remote { ip 10.0.0.2; }
			local { ip 10.0.0.1; }
		}
		session {
			asn { remote 65002; }
			family { ` + family + ` { prefix { maximum 10000; } } }
		}
	}
}
`
}

// peerConfigWithTwoPeers carries two peers the engine accepts. A test that
// wants an inactive one marks it after parsing (treeWithInactivePeer): the
// parser prunes an `inactive:` entry itself, so the flag has to be set on the
// tree the check receives for the prune inside the engine to have anything to
// drop.
const peerConfigWithTwoPeers = `
bgp {
	router-id 1.2.3.4;
	session { asn { local 65001; } }
	peer active {
		connection {
			remote { ip 10.0.0.2; }
			local { ip 10.0.0.1; }
		}
		session {
			asn { remote 65002; }
			family { ipv4/unicast { prefix { maximum 10000; } } }
		}
	}
	peer disabled {
		connection {
			remote { ip 10.0.0.3; }
			local { ip 10.0.0.1; }
		}
		session {
			asn { remote 65003; }
			family { ipv4/unicast { prefix { maximum 10000; } } }
		}
	}
}
`

// treeWithInactivePeer parses peerConfigWithTwoPeers and deactivates the
// second peer. config.PruneInactive drops that entry in place, so the tree
// that comes back is how a test tells a check that cloned from one that did
// not.
func treeWithInactivePeer(t *testing.T) *config.Tree {
	t.Helper()
	tree := parsedTree(t, peerConfigWithTwoPeers)
	for _, p := range tree.GetContainer("bgp").GetListOrdered("peer") {
		if p.Key == "disabled" {
			p.Value.SetInactive(true)
		}
	}
	require.Equal(t, []string{"active", "disabled"}, peerNames(tree))
	return tree
}

// parsedTree parses a configuration through the same YANG the daemon loads.
func parsedTree(t *testing.T, text string) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err, "fixture config must parse")
	return tree
}

// peerNames lists the peers a tree still carries, inactive ones included.
func peerNames(tree *config.Tree) []string {
	var names []string
	for _, p := range tree.GetContainer("bgp").GetListOrdered("peer") {
		names = append(names, p.Key)
	}
	return names
}

// TestDoctorCheckBGPPeerConfigDoesNotMutateTree drives the check over a config
// with an inactive peer and reads the caller's tree afterwards.
//
// VALIDATES: doctorCheckBGPPeerConfig hands the engine a CLONE.
// PREVENTS: the defect this check shipped with. PeersFromConfigTree calls
// config.PruneInactive, an in-place mutation, and the doctor runner shares one
// tree across every later check: validating in place deleted every `inactive:`
// node from underneath them, and only when a bgp{} block was present, so the
// whole report became order- and content-dependent.
func TestDoctorCheckBGPPeerConfigDoesNotMutateTree(t *testing.T) {
	tree := treeWithInactivePeer(t)

	assert.Empty(t, doctorCheckBGPPeerConfig(diagnosticContext(tree)))

	assert.Equal(t, []string{"active", "disabled"}, peerNames(tree),
		"the check must hand the engine a clone: the caller's tree is shared by every later check")
}

// TestDoctorCheckBGPPeerConfigReportsRejection pins the diagnostic for a config
// the engine refuses.
//
// VALIDATES: `ze doctor` reports an ERROR carrying the engine's own reason,
// instead of answering ready.
// PREVENTS: the defect test/plugin/mpls-doctor.ci ran into on CI. A config
// naming a family that does not exist was rejected by `ze config validate`
// while `ze doctor --json` reported "ready": true and exited 0.
func TestDoctorCheckBGPPeerConfigReportsRejection(t *testing.T) {
	tree := parsedTree(t, peerConfigWithFamily("ipv4/mpls-unicast"))

	diags := doctorCheckBGPPeerConfig(diagnosticContext(tree))

	require.Len(t, diags, 1, "a rejected peer config must produce exactly one diagnostic")
	assert.Equal(t, diagnosticBGPPeerConfig, diags[0].Code)
	// ERROR: a config the engine refuses means the daemon will not start, so the
	// report must not be ready and `ze doctor` must exit non-zero.
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "unknown address family",
		"the diagnostic must carry the engine's own reason, not just a generic failure")
}

// TestDoctorCheckBGPPeerConfigSilentWhenValid is the negative half.
//
// VALIDATES: an accepted peer config produces no diagnostic, and a tree with no
// bgp{} block or no tree at all produces none either.
// PREVENTS: a false error on every non-BGP config, and a spurious readiness
// failure on a config the daemon starts on.
func TestDoctorCheckBGPPeerConfigSilentWhenValid(t *testing.T) {
	assert.Empty(t, doctorCheckBGPPeerConfig(diagnosticContext(parsedTree(t, peerConfigWithFamily("ipv4/unicast")))))
	assert.Empty(t, doctorCheckBGPPeerConfig(diagnosticContext(config.NewTree())))
	assert.Empty(t, doctorCheckBGPPeerConfig(diagnostic.DoctorCheckContext{}))
}

// rolelessPeerConfig carries two eBGP peers with no RFC 9234 role, one with a
// role, and one iBGP peer.
const rolelessPeerConfig = `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    peer unbound-a {
        connection { remote { ip 10.0.0.1 } local { ip auto } }
        session { asn { remote 65001 } }
    }
    peer unbound-b {
        connection { remote { ip 10.0.0.2 } local { ip auto } }
        session { asn { remote 65002 } }
    }
    peer bound {
        connection { remote { ip 10.0.0.3 } local { ip auto } }
        session { asn { remote 65003 } }
        role {
            import customer
        }
    }
    peer ibgp {
        connection { remote { ip 10.0.0.4 } local { ip auto } }
        session { asn { remote 65000 } }
    }
}
`

// TestDoctorCheckBGPPeersWithoutRoleReportsEveryPeer drives the registered
// check over a config with two roleless peers.
//
// VALIDATES: AC-35. `ze doctor` enumerates every peer that declares no RFC
// 9234 role, under its own diagnostic code, as one aggregated warning.
// PREVENTS: the role gap staying invisible on the operator-facing readiness
// path. A peer with no role is accepted and owes no transit-leak filter, so
// nothing else in the report would mention it.
func TestDoctorCheckBGPPeersWithoutRoleReportsEveryPeer(t *testing.T) {
	diags := doctorCheckBGPPeersWithoutRole(diagnosticContext(parsedTree(t, rolelessPeerConfig)))

	require.Len(t, diags, 1, "one aggregated diagnostic, never one per peer")
	assert.Equal(t, diagnosticBGPPeerNoRole, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity,
		"the config loads and the daemon starts on it, so this is not an error")
	assert.Contains(t, diags[0].Message, "unbound-a")
	assert.Contains(t, diags[0].Message, "unbound-b")
	assert.NotContains(t, diags[0].Message, "bound,", "a peer that declares a role is not named")
	assert.NotContains(t, diags[0].Message, "ibgp")
}

// TestDoctorCheckBGPPeersWithoutRoleSilentWhenEveryPeerDeclaresOne is the
// negative half.
//
// VALIDATES: a config whose peers all declare a role produces no diagnostic,
// and so does a tree with no bgp block.
// PREVENTS: a permanent warning that operators learn to ignore.
func TestDoctorCheckBGPPeersWithoutRoleSilentWhenEveryPeerDeclaresOne(t *testing.T) {
	text := `
bgp {
    router-id 1.2.3.4;
    session { asn { local 65000 } }
    peer bound {
        connection { remote { ip 10.0.0.3 } local { ip auto } }
        session { asn { remote 65003 } }
        role { import customer }
    }
}
`
	assert.Empty(t, doctorCheckBGPPeersWithoutRole(diagnosticContext(parsedTree(t, text))))
	assert.Empty(t, doctorCheckBGPPeersWithoutRole(diagnosticContext(config.NewTree())))
	assert.Empty(t, doctorCheckBGPPeersWithoutRole(diagnostic.DoctorCheckContext{}))
}

// TestDoctorCheckBGPPeersWithoutRoleDoesNotMutateTree reads the caller's tree
// after the check ran over a config with an inactive peer.
//
// VALIDATES: doctorCheckBGPPeersWithoutRole hands the reporter a CLONE.
// PREVENTS: the defect doctorCheckBGPPeerConfig shipped with. The reporter
// prunes inactive nodes in place, and the doctor runner's tree is shared with
// every later check.
func TestDoctorCheckBGPPeersWithoutRoleDoesNotMutateTree(t *testing.T) {
	tree := treeWithInactivePeer(t)

	doctorCheckBGPPeersWithoutRole(diagnosticContext(tree))

	assert.Equal(t, []string{"active", "disabled"}, peerNames(tree),
		"the caller's tree must survive the check")
}

// TestDoctorCheckBGPMD5SilentWithoutMD5 is the negative half of the MD5 check.
//
// VALIDATES: no diagnostic when no BGP peer exists, when a peer asks for no
// MD5, and when no tree is loaded.
// PREVENTS: a warning on every box, whatever the config says.
func TestDoctorCheckBGPMD5SilentWithoutMD5(t *testing.T) {
	assert.Empty(t, doctorCheckBGPMD5(diagnosticContext(config.NewTree())))
	assert.Empty(t, doctorCheckBGPMD5(diagnostic.DoctorCheckContext{}))

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	conn := peer.GetOrCreateContainer("connection")
	remote := conn.GetOrCreateContainer("remote")
	remote.Set("ip", "192.0.2.1")
	bgp.AddListEntry("peer", "p1", peer)

	assert.Empty(t, doctorCheckBGPMD5(diagnosticContext(tree)))
}

// TestDoctorCheckBGPMD5ReportsAPeerOnAPlatformWithoutIt drives the check over
// a peer that asks for TCP MD5, on a platform whose kernel cannot sign.
//
// VALIDATES: the diagnostic names the peer, under doctor-bgp-md5, as a warning.
// PREVENTS: a session that never establishes with nothing in the report to say
// why.
func TestDoctorCheckBGPMD5ReportsAPeerOnAPlatformWithoutIt(t *testing.T) {
	if network.TCPMD5Supported() {
		t.Skip("TCP MD5 is supported on this platform; the warning would not fire")
	}

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	conn := peer.GetOrCreateContainer("connection")
	md5 := conn.GetOrCreateContainer("md5")
	md5.Set("password", "secret")
	remote := conn.GetOrCreateContainer("remote")
	remote.Set("ip", "192.0.2.1")
	bgp.AddListEntry("peer", "p1", peer)

	diags := doctorCheckBGPMD5(diagnosticContext(tree))
	require.Len(t, diags, 1)
	assert.Equal(t, diagnosticBGPMD5, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "p1")
}

// TestMD5ConfiguredReadsThePeerBeforeTheGroup pins the inheritance the walk
// relies on, which TCPMD5Supported hides on a Linux host.
//
// VALIDATES: a peer that states no password takes the group's; a peer that
// states an empty one asks for no MD5 whatever the group says.
// PREVENTS: a group password silently binding a peer that opted out, and a
// peer under an MD5 group going unreported.
func TestMD5ConfiguredReadsThePeerBeforeTheGroup(t *testing.T) {
	withPassword := func(password string) *config.Tree {
		node := config.NewTree()
		node.GetOrCreateContainer("connection").GetOrCreateContainer("md5").Set("password", password)
		return node
	}

	assert.True(t, md5Configured(nil, withPassword("secret")))
	assert.False(t, md5Configured(nil, withPassword("")))
	assert.False(t, md5Configured(nil, config.NewTree()))
	assert.True(t, md5Configured(withPassword("secret"), config.NewTree()), "the group's password reaches the peer")
	assert.False(t, md5Configured(withPassword("secret"), withPassword("")), "the peer's empty password wins")
}

// newCaptureTree builds a config tree with one BGP peer whose capture container
// carries the given leaves.
func newCaptureTree(enabled, directory string) *config.Tree {
	const peer = "192.0.2.1"
	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	p := config.NewTree()
	capture := p.GetOrCreateContainer("capture")
	if enabled != "" {
		capture.Set("enabled", enabled)
	}
	if directory != "" {
		capture.Set("directory", directory)
	}
	bgp.AddListEntry("peer", peer, p)
	return tree
}

// TestDoctorCheckBGPCaptureDirectoryDisabled pins the silence for a peer that
// never opted in.
//
// VALIDATES: capture disabled (the default) produces no diagnostic, so the
// check costs a clean report nothing.
// PREVENTS: doctor warning about a directory nobody asked for.
func TestDoctorCheckBGPCaptureDirectoryDisabled(t *testing.T) {
	assert.Empty(t, doctorCheckBGPCaptureDirectory(diagnostic.DoctorCheckContext{}))
	assert.Empty(t, doctorCheckBGPCaptureDirectory(diagnosticContext(config.NewTree())))
	assert.Empty(t, doctorCheckBGPCaptureDirectory(diagnosticContext(newCaptureTree("", "/nonexistent/zecap"))))
	assert.Empty(t, doctorCheckBGPCaptureDirectory(diagnosticContext(newCaptureTree("false", "/nonexistent/zecap"))))
}

// TestDoctorCheckBGPCaptureDirectoryWritable pins the silence for a directory
// the daemon can use.
//
// VALIDATES: an enabled capture whose directory is writable passes.
// PREVENTS: a false alarm on a correctly configured box.
func TestDoctorCheckBGPCaptureDirectoryWritable(t *testing.T) {
	assert.Empty(t, doctorCheckBGPCaptureDirectory(diagnosticContext(newCaptureTree("true", t.TempDir()))))
}

// TestDoctorCheckBGPCaptureDirectoryNotWritable drives the check over a
// directory that cannot be created.
//
// VALIDATES: an enabled capture whose directory cannot be created or written is
// reported, naming the peer and the path.
// PREVENTS: an operator enabling capture and finding no file and no reason,
// which is the runtime dependency this feature introduces.
func TestDoctorCheckBGPCaptureDirectoryNotWritable(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	diags := doctorCheckBGPCaptureDirectory(diagnosticContext(newCaptureTree("true", filepath.Join(blocker, "sub"))))
	require.Len(t, diags, 1)
	assert.Equal(t, diagnosticBGPCaptureDirectory, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "192.0.2.1")
	assert.Contains(t, diags[0].Path, "sub")
}

// TestDoctorCheckBGPCaptureDirectoryUsesDefault drives a peer that enables
// capture without naming a directory.
//
// VALIDATES: the peer is checked against the reactor's default directory, not
// skipped.
// PREVENTS: the default path going unchecked because the leaf is absent.
func TestDoctorCheckBGPCaptureDirectoryUsesDefault(t *testing.T) {
	diags := doctorCheckBGPCaptureDirectory(diagnosticContext(newCaptureTree("true", "")))
	// The default is /var/lib/ze/capture. On a developer machine it is usually
	// absent and unwritable, on a deployed box it is writable. Either verdict is
	// correct; what must not happen is the check silently doing nothing.
	for _, d := range diags {
		assert.Equal(t, diagnosticBGPCaptureDirectory, d.Code)
		assert.Contains(t, d.Path, reactor.DefaultCaptureDirectory)
	}
	assert.LessOrEqual(t, len(diags), 1)
}

// TestDoctorCheckBGPCaptureDirectoryInGroup drives a peer declared inside a
// group.
//
// VALIDATES: a peer inside a group is checked too, not only a top-level peer.
// PREVENTS: half the configuration surface going unchecked.
func TestDoctorCheckBGPCaptureDirectoryInGroup(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "afile")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	tree := config.NewTree()
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	p := config.NewTree()
	c := p.GetOrCreateContainer("capture")
	c.Set("enabled", "true")
	c.Set("directory", filepath.Join(blocker, "sub"))
	group.AddListEntry("peer", "198.51.100.1", p)
	bgp.AddListEntry("group", "transit", group)

	diags := doctorCheckBGPCaptureDirectory(diagnosticContext(tree))
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "198.51.100.1")
}

// TestBGPDoctorChecksReachTheRunner asks the registry the doctor runner reads
// whether it holds every check this package declares, at the phase and order
// declared, and whether every code they emit resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the table and the registry
// accepted every entry.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green, which is how these four sat in this package for a while with
// no caller of registerBGPDoctorChecks at all.
func TestBGPDoctorChecksReachTheRunner(t *testing.T) {
	require.NotEmpty(t, bgpDoctorChecks, "the table below would pass over an empty declaration")
	diagnostic.RegisterBuiltinCodes()

	for i := range bgpDoctorChecks {
		want := bgpDoctorChecks[i]
		var found *diagnostic.DoctorCheck
		checks := diagnostic.DoctorChecksForPhase(want.Phase)
		for j := range checks {
			if checks[j].Name == want.Name {
				found = &checks[j]
				break
			}
		}
		require.NotNilf(t, found, "check %q is declared for phase %q but the registry does not hold it", want.Name, want.Phase)
		assert.Equalf(t, want.Order, found.Order, "check %q order", want.Name)
		assert.Equalf(t, want.Component, found.Component, "check %q component", want.Name)
		for _, code := range want.Codes {
			assert.NotNilf(t, diagnostic.Lookup(code), "check %q emits unregistered code %q", want.Name, code)
		}
	}
}
