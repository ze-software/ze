package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// diagnosticContext is the shape the doctor registry hands a check.
func diagnosticContext(tree *config.Tree) diagnostic.DoctorCheckContext {
	return diagnostic.DoctorCheckContext{Tree: tree}
}

// strictDoctorTree parses a configuration for the doctor check to read.
func strictDoctorTree(t *testing.T, text string) *config.Tree {
	t.Helper()
	tree, err := config.ParseTreeForValidation(text)
	require.NoError(t, err)
	require.NotNil(t, tree)
	return tree
}

const strictPeerConfig = `
bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	peer peer1 {
		connection {
			remote { ip 192.0.2.1; }
			local { ip auto; }
			bfd { enabled true; strict true; }
		}
		session { asn { remote 65001; } }
	}
}
`

// withBFDEngine adds the top-level bfd container to a parsed tree.
//
// It is added rather than parsed because the `bfd` keyword is registered by the
// BFD component, which this package's test binary does not link: parsing the
// block here fails with "unknown top-level keyword". What the check reads is
// the container's PRESENCE, and that is what this supplies.
func withBFDEngine(tree *config.Tree) *config.Tree {
	engine := config.NewTree()
	engine.Set("enabled", "true")
	tree.SetContainer("bfd", engine)
	return tree
}

// TestStrictPeersWithoutEngineNamesThePeer is AC-15: a peer that asks for BFD
// strict mode with no BFD engine configured must be told about, because
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5 holds such a peer in
// OpenSent for the life of the process and nothing on the peer's own surface
// says why.
//
// VALIDATES: The peer is named when no top-level bfd block exists, and is not
// named when one does.
//
// PREVENTS: The silent case. Against a peer that never advertises capability 74
// the session establishes with no forwarding-path check at all, so the operator
// gets neither the gate they asked for nor a word about it.
func TestStrictPeersWithoutEngineNamesThePeer(t *testing.T) {
	names := strictPeersWithoutEngine(strictDoctorTree(t, strictPeerConfig))
	require.Equal(t, []string{"peer1"}, names)

	withEngine := strictPeersWithoutEngine(withBFDEngine(strictDoctorTree(t, strictPeerConfig)))
	require.Empty(t, withEngine, "a configured BFD engine answers the strict peer")
}

// TestStrictPeersWithoutEngineIgnoresNonStrictPeers is the negative half.
//
// VALIDATES: A peer with plain BFD, and a peer with strict under enabled false,
// are not named.
//
// PREVENTS: A check that fires on every BFD peer in a configuration whose
// engine block is declared elsewhere, which would train an operator to ignore
// it.
func TestStrictPeersWithoutEngineIgnoresNonStrictPeers(t *testing.T) {
	plain := strictPeersWithoutEngine(strictDoctorTree(t, `
bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	peer peer1 {
		connection {
			remote { ip 192.0.2.1; }
			local { ip auto; }
			bfd { enabled true; }
		}
		session { asn { remote 65001; } }
	}
}
`))
	require.Empty(t, plain, "plain BFD needs no engine to establish")

	suspended := strictPeersWithoutEngine(strictDoctorTree(t, `
bgp {
	router-id 10.0.0.1;
	session { asn { local 65000; } }
	peer peer1 {
		connection {
			remote { ip 192.0.2.1; }
			local { ip auto; }
			bfd { enabled false; strict true; }
		}
		session { asn { remote 65001; } }
	}
}
`))
	require.Empty(t, suspended, "a suspended bfd block runs no strict-mode procedures")
}

// TestCheckBFDStrictHasEngineRaisesTheCode drives the registered check itself.
//
// VALIDATES: The check answers one diagnostic carrying the registered code and
// naming the peer, and answers nothing where the engine exists.
//
// PREVENTS: A check whose predicate is right and whose plumbing never reports,
// which is how the same feature's FSM Event 34 went unreachable for a whole
// review round.
func TestCheckBFDStrictHasEngineRaisesTheCode(t *testing.T) {
	found := checkBFDStrictHasEngine(diagnosticContext(strictDoctorTree(t, strictPeerConfig)))
	require.Len(t, found, 1)
	require.Equal(t, codeBFDStrictWithoutEngine, found[0].Code)
	require.Contains(t, found[0].Message, "peer1")

	clean := checkBFDStrictHasEngine(diagnosticContext(withBFDEngine(strictDoctorTree(t, strictPeerConfig))))
	require.Empty(t, clean)
}
