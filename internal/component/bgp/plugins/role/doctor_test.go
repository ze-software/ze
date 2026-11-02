// Design: docs/guide/bgp-role.md -- peers that declare no role
// Detail: doctor.go -- checkRolelessPeers and its registration
package role

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The tests below parse the config text an operator types through the YANG
// the daemon loads: the bgp module (blank import) and this plugin's role
// module (register.go), so the role container reaches the tree.

// parsedRoleTree parses a configuration through the same YANG the daemon loads.
func parsedRoleTree(t *testing.T, text string) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err, "fixture config must parse: these tests are about the report, not the schema")
	return tree
}

// doctorContext hands a tree to a check the way the doctor runner does.
func doctorContext(tree *config.Tree) registry.DoctorCheckContext {
	return registry.DoctorCheckContext{Tree: tree}
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

// TestDoctorCheckRolelessPeersReportsEveryPeer drives the registered check over
// a config with two roleless eBGP peers.
//
// VALIDATES: AC-35. `ze doctor` enumerates every peer that declares no RFC
// 9234 role, under its own diagnostic code, as one aggregated warning, and the
// set is exactly the one rolelessPeers names (the set the config warning logs).
// PREVENTS: the role gap staying invisible on the operator-facing readiness
// path, and a doctor report naming other peers than the warning does.
func TestDoctorCheckRolelessPeersReportsEveryPeer(t *testing.T) {
	tree := parsedRoleTree(t, rolelessPeerConfig)

	names, err := rolelessPeersFromTree(tree)
	require.NoError(t, err)
	assert.Equal(t, []string{"unbound-a", "unbound-b"}, names,
		"only the eBGP peers that declare no role are reported")

	diags := checkRolelessPeers(doctorContext(tree))
	require.Len(t, diags, 1, "one aggregated diagnostic, never one per peer")
	assert.Equal(t, "doctor-bgp-peer-no-role", diags[0].Code)
	assert.Equal(t, string(diagnostic.SeverityWarning), diags[0].Severity,
		"the config loads and the daemon starts on it, so this is not an error")
	assert.Contains(t, diags[0].Message, "unbound-a, unbound-b")
	assert.NotContains(t, diags[0].Message, "bound,", "a peer that declares a role is not named")
	assert.NotContains(t, diags[0].Message, "ibgp")
}

// TestDoctorCheckRolelessPeersIsRegistered reads the check back from the
// plugin registry the doctor runner reads.
//
// VALIDATES: the check reaches `ze doctor` under the name, phase and code the
// engine table carried before it moved here.
// PREVENTS: a check that exists and is never run.
func TestDoctorCheckRolelessPeersIsRegistered(t *testing.T) {
	for _, check := range registry.PluginDoctorChecks() {
		if check.Name != "bgp-peer-no-role" {
			continue
		}
		assert.Equal(t, rolePluginName, check.PluginName)
		assert.Equal(t, []string{"doctor-bgp-peer-no-role"}, check.Codes)
		require.NotNil(t, check.Check)
		assert.Len(t, check.Check(doctorContext(parsedRoleTree(t, rolelessPeerConfig))), 1)
		return
	}
	t.Fatal("bgp-role registers no bgp-peer-no-role doctor check")
}

// TestDoctorCheckRolelessPeersSilentWhenEveryPeerDeclaresOne is the negative
// half.
//
// VALIDATES: a config whose peers all declare a role produces no diagnostic,
// and so do a tree with no bgp block and no tree at all.
// PREVENTS: a permanent warning that operators learn to ignore.
func TestDoctorCheckRolelessPeersSilentWhenEveryPeerDeclaresOne(t *testing.T) {
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
	assert.Empty(t, checkRolelessPeers(doctorContext(parsedRoleTree(t, text))))
	assert.Empty(t, checkRolelessPeers(doctorContext(config.NewTree())))
	assert.Empty(t, checkRolelessPeers(registry.DoctorCheckContext{}))
}

// TestRolelessPeersFromTreeIsSilentWithoutBGP checks the answer for a tree
// that carries no bgp block.
//
// VALIDATES: no names and no error, rather than a panic or an invented peer.
// PREVENTS: `ze doctor` reporting a role gap for a config that has no BGP.
func TestRolelessPeersFromTreeIsSilentWithoutBGP(t *testing.T) {
	names, err := rolelessPeersFromTree(config.NewTree())
	require.NoError(t, err)
	assert.Empty(t, names)
}

// peerConfigWithTwoPeers carries two roleless eBGP peers. A test that wants an
// inactive one marks it after parsing (treeWithInactivePeer): the parser prunes
// an `inactive:` entry itself, so the flag has to be set on the tree the check
// receives for the prune inside the check to have anything to drop.
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

// peerNames lists the peers a tree still carries, inactive ones included.
func peerNames(tree *config.Tree) []string {
	var names []string
	for _, p := range tree.GetContainer("bgp").GetListOrdered("peer") {
		names = append(names, p.Key)
	}
	return names
}

// TestDoctorCheckRolelessPeersDoesNotMutateTree runs the check over a config
// with an inactive roleless peer, then reads the caller's tree.
//
// VALIDATES: the check prunes a CLONE, and the prune is real: the inactive
// peer, which declares no role, is not named.
// PREVENTS: the defect doctorCheckBGPPeerConfig shipped with. The prune works
// in place, and the doctor runner's tree is shared with every later check.
func TestDoctorCheckRolelessPeersDoesNotMutateTree(t *testing.T) {
	tree := parsedRoleTree(t, peerConfigWithTwoPeers)
	for _, p := range tree.GetContainer("bgp").GetListOrdered("peer") {
		if p.Key == "disabled" {
			p.Value.SetInactive(true)
		}
	}
	require.Equal(t, []string{"active", "disabled"}, peerNames(tree))

	diags := checkRolelessPeers(doctorContext(tree))

	assert.Equal(t, []string{"active", "disabled"}, peerNames(tree),
		"the caller's tree must survive the check")
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "active")
	assert.NotContains(t, diags[0].Message, "disabled",
		"an inactive peer runs no session, so it is not named")
}

// dynamicGroupRoleConfig builds a config carrying two listen-range groups: one
// that declares no RFC 9234 role and one that declares `rs`. A dynamic group
// states no remote AS, so it is the shape a report keyed on the AS drops.
func dynamicGroupRoleConfig() string {
	return `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    group ix-unbound {
        connection {
            remote { ip dynamic; connect false; range 10.0.0.0/8; }
            local  { ip 10.0.0.1; accept true; }
        }
    }
    group ix-bound {
        connection {
            remote { ip dynamic; connect false; range 10.1.0.0/16; }
            local  { ip 10.0.0.1; accept true; }
        }
        role {
            import rs
        }
    }
}
`
}

// TestRolelessDynamicGroupIsReported drives a listen-range group that declares
// no role through the set `ze doctor` and the config warning both read.
//
// VALIDATES: a dynamic group with no RFC 9234 role is named. Its remote AS
// arrives in the OPEN (RFC 4271 Section 4.2), so the config holds none and the
// session can still be eBGP.
// PREVENTS: the IXP route-server shape this report exists for being the one
// shape it never reports. A predicate that reads a missing AS as iBGP drops
// every listen range, and neither the warning nor `ze doctor` would say a word.
func TestRolelessDynamicGroupIsReported(t *testing.T) {
	names, err := rolelessPeersFromTree(parsedRoleTree(t, dynamicGroupRoleConfig()))
	require.NoError(t, err)

	assert.Contains(t, names, "ix-unbound",
		"a listen range that declares no role is unbound, and Ze cannot know its AS until the OPEN")
}

// TestDynamicGroupWithRoleIsNotReported is the other half: a group that states
// the relationship owes no report.
//
// VALIDATES: the role a dynamic group declares is read off the group
// (parseRoleFromMap), so declaring one silences the report.
// PREVENTS: a warning naming every listen range whatever its config says, which
// an operator learns to ignore.
func TestDynamicGroupWithRoleIsNotReported(t *testing.T) {
	names, err := rolelessPeersFromTree(parsedRoleTree(t, dynamicGroupRoleConfig()))
	require.NoError(t, err)

	require.NotEmpty(t, names,
		"the roleless group in the same config is reported, so an empty answer is the report being dead rather than the role being read")
	assert.NotContains(t, names, "ix-bound",
		"a group that declares `rs` states its relationship, so nothing is unbound")
}
