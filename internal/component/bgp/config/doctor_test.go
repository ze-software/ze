// Design: docs/architecture/bgp/filter-path-asn.md -- the transit-leak filter obligation
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// TestDoctorReportsPeersWithNoRole drives the seam `ze doctor` reads.
//
// VALIDATES: AC-35. The set the doctor check enumerates is the set the config
// loader warns about, because both read rolelessPeers.
// PREVENTS: a doctor report that names different peers than the daemon's
// warning, which would leave an operator unable to tell which is right.
func TestDoctorReportsPeersWithNoRole(t *testing.T) {
	text := `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
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
        filter {
            export [ NO-TRANSIT ]
        }
    }
    peer ibgp {
        connection { remote { ip 10.0.0.4 } local { ip auto } }
        session { asn { remote 65000 } }
    }
}
`
	tree, err := config.ParseTreeWithYANG(text, nil)
	require.NoError(t, err)

	assert.Equal(t, []string{"unbound-a", "unbound-b"}, rolelessPeersFromTree(tree),
		"only the eBGP peers that declare no role are reported")
}

// TestRolelessPeersFromTreeIsSilentOnAnUnreadableConfig checks the answer for a
// tree the engine cannot resolve.
//
// VALIDATES: a config with no bgp block yields no names rather than a panic or
// an invented one.
// PREVENTS: `ze doctor` reporting a role gap for a config it never read, which
// would send the operator after a peer that does not exist.
func TestRolelessPeersFromTreeIsSilentOnAnUnreadableConfig(t *testing.T) {
	assert.Empty(t, rolelessPeersFromTree(config.NewTree()))
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
// no role through the seam `ze doctor` and the config-load warning both read.
//
// VALIDATES: a dynamic group with no RFC 9234 role is named. Its remote AS
// arrives in the OPEN (RFC 4271 Section 4.2), so PeerAS stays 0 on the template
// (reactor.ParseDynamicGroupTemplate) and the session can still be eBGP.
// PREVENTS: the IXP route-server shape this report exists for being the one
// shape it never reports. A predicate that reads PeerAS 0 as iBGP drops every
// listen range, and neither the warning nor `ze doctor` would say a word.
func TestRolelessDynamicGroupIsReported(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(dynamicGroupRoleConfig(), nil)
	require.NoError(t, err)

	assert.Contains(t, rolelessPeersFromTree(tree), "ix-unbound",
		"a listen range that declares no role is unbound, and Ze cannot know its AS until the OPEN")
}

// TestDynamicGroupWithRoleIsNotReported is the other half: a group that states
// the relationship is bound by the obligation and owes no report.
//
// VALIDATES: the role a dynamic group declares is read off its template
// (peerRoles, declaredRole), so declaring one silences the report.
// PREVENTS: a warning naming every listen range whatever its config says, which
// an operator learns to ignore.
func TestDynamicGroupWithRoleIsNotReported(t *testing.T) {
	tree, err := config.ParseTreeWithYANG(dynamicGroupRoleConfig(), nil)
	require.NoError(t, err)

	names := rolelessPeersFromTree(tree)
	require.NotEmpty(t, names,
		"the roleless group in the same config is reported, so an empty answer is the report being dead rather than the role being read")
	assert.NotContains(t, names, "ix-bound",
		"a group that declares `rs` states its relationship, so nothing is unbound")
}

// TestStaticPeerWithNoRemoteASIsNamed drives the set with a state the config
// path cannot produce: a STATIC peer carrying no remote AS. parsePeerFromTree
// refuses that config and PeersFromTree drops the peer (reactor/config.go), so
// reaching rolelessPeers with PeerAS 0 and IsDynamic false is a Ze defect.
//
// VALIDATES: the defect is NAMED rather than dropped. An unknown AS is not
// evidence of iBGP, so the one exclusion does not cover it.
// PREVENTS: one value carrying two facts. The predicate this replaced read
// PeerAS 0 as "iBGP, nothing owed", which hid a peer whose relationship Ze
// never learned behind the branch that exists for one Ze KNOWS.
func TestStaticPeerWithNoRemoteASIsNamed(t *testing.T) {
	pairs := []peerRole{{settings: &reactor.PeerSettings{Name: "no-remote-as", LocalAS: 65000}}}

	assert.Equal(t, []string{"no-remote-as"}, rolelessPeers(pairs),
		"a static peer with no remote AS is reported, because Ze cannot call it iBGP")
}
