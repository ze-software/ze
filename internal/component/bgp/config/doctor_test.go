// Design: docs/architecture/bgp/filter-path-asn.md -- the transit-leak filter obligation
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
