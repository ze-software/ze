// Detail: checks_as112_coordination.go -- checkAS112RedistributeOriginCoordination

// VALIDATES: AC-11 (redistribute path) -- doctor-as112-redistribute-origin-
// uncoordinated fires when the as112 service originates AS112 (asn 112, the
// default) via `import as112` while an eBGP session to a public remote ASN
// exists, and stays silent for an explicit non-112 asn, a private remote, iBGP,
// or when as112 is not imported into bgp.
// PREVENTS: an operator silently becoming an uncoordinated global AS112 origin
// through the easy redistribute path, undetected by `ze doctor`.

package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// newAS112RedistTree builds a config tree with an as112 service block and,
// optionally, a `redistribute { destination bgp { import as112 } }` rule (list
// or scalar form). Callers add bgp peers separately.
func newAS112RedistTree(enabled bool, asn string, importAS112, scalarImport bool) *config.Tree {
	tree := config.NewTree()
	svc := tree.GetOrCreateContainer("service")
	as112 := svc.GetOrCreateContainer("as112")
	if enabled {
		as112.Set("enabled", "true")
	}
	if asn != "" {
		as112.Set("asn", asn)
	}
	if importAS112 {
		rd := tree.GetOrCreateContainer("redistribute")
		dest := config.NewTree()
		if scalarImport {
			dest.Set("import", "as112")
		} else {
			dest.AddListEntry("import", "as112", config.NewTree())
		}
		rd.AddListEntry("destination", "bgp", dest)
	}
	return tree
}

func TestCheckAS112RedistributeOrigin_NoService_NoDiagnostic(t *testing.T) {
	tree := config.NewTree()
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_Disabled_NoDiagnostic(t *testing.T) {
	tree := newAS112RedistTree(false, "112", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "15169", false))
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_NotImported_NoDiagnostic(t *testing.T) {
	tree := newAS112RedistTree(true, "112", false, false) // enabled, but no import as112
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "15169", false))
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_ExplicitNon112ASN_NoDiagnostic(t *testing.T) {
	// An operator originating under its own/private AS is not the global-AS112
	// concern.
	tree := newAS112RedistTree(true, "65001", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("65001", "15169", false))
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_PrivateRemote_NoDiagnostic(t *testing.T) {
	// 64600 is inside the RFC 6996 Section 4 private-use range.
	tree := newAS112RedistTree(true, "112", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("112", "64600", false))
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_IBGP_NoDiagnostic(t *testing.T) {
	// iBGP (local == remote) keeps the origin internal; not flagged.
	tree := newAS112RedistTree(true, "112", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("15169", "15169", false))
	assert.Empty(t, checkAS112RedistributeOriginCoordination(tree))
}

func TestCheckAS112RedistributeOrigin_PublicEBGP_Flagged(t *testing.T) {
	tree := newAS112RedistTree(true, "112", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("64512", "15169", false)) // private local, public remote -> eBGP

	diags := checkAS112RedistributeOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-redistribute-origin-uncoordinated", diags[0].Code)
	assert.Contains(t, diags[0].Message, "bgp/peer/p1")
	assert.Contains(t, diags[0].Message, "15169")
}

func TestCheckAS112RedistributeOrigin_DefaultASN_Flagged(t *testing.T) {
	// asn unset -> defaults to 112; still the concern.
	tree := newAS112RedistTree(true, "", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("64512", "15169", false))

	diags := checkAS112RedistributeOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-redistribute-origin-uncoordinated", diags[0].Code)
}

func TestCheckAS112RedistributeOrigin_ScalarImportForm_Flagged(t *testing.T) {
	tree := newAS112RedistTree(true, "112", true, true) // scalar `import as112;`
	bgp := tree.GetOrCreateContainer("bgp")
	bgp.AddListEntry("peer", "p1", newAS112PeerTree("64512", "15169", false))

	diags := checkAS112RedistributeOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-redistribute-origin-uncoordinated", diags[0].Code)
}

func TestCheckAS112RedistributeOrigin_GroupPeer_Flagged(t *testing.T) {
	tree := newAS112RedistTree(true, "112", true, false)
	bgp := tree.GetOrCreateContainer("bgp")
	group := config.NewTree()
	group.AddListEntry("peer", "p1", newAS112PeerTree("64512", "15169", false))
	bgp.AddListEntry("group", "g1", group)

	diags := checkAS112RedistributeOriginCoordination(tree)
	require.Len(t, diags, 1)
	assert.Contains(t, diags[0].Message, "bgp/group/g1/peer/p1")
}

// --- checkAS112RedistributeNotImported (silent-inert redistribute knob) ---

// VALIDATES: doctor-as112-redistribute-not-imported fires when service as112 is
// enabled with a redistribute-only knob (explicit asn or community) but no
// `import as112`, and stays silent for a DNS-only node, a correctly-imported
// node, or a disabled service.
// PREVENTS: the silent misconfiguration where an operator sets asn/community
// expecting the covering prefixes in BGP but never wires the import, so the
// producer's events reach no RIB and `ze doctor` says nothing.

func TestCheckAS112RedistributeNotImported_ASNSetNoImport_Flagged(t *testing.T) {
	tree := newAS112RedistTree(true, "65001", false, false) // asn set, no import
	diags := checkAS112RedistributeNotImported(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-redistribute-not-imported", diags[0].Code)
	assert.Contains(t, diags[0].Message, "import as112")
}

func TestCheckAS112RedistributeNotImported_CommunitySetNoImport_Flagged(t *testing.T) {
	tree := newAS112RedistTree(true, "", false, false) // no explicit asn
	as112 := tree.GetContainer("service").GetContainer("as112")
	as112.SetSlice("community", []string{"no-export"})
	diags := checkAS112RedistributeNotImported(tree)
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-as112-redistribute-not-imported", diags[0].Code)
}

func TestCheckAS112RedistributeNotImported_Imported_NoDiagnostic(t *testing.T) {
	tree := newAS112RedistTree(true, "65001", true, false) // asn set AND imported
	assert.Empty(t, checkAS112RedistributeNotImported(tree))
}

func TestCheckAS112RedistributeNotImported_DNSOnly_NoDiagnostic(t *testing.T) {
	// Enabled, no asn/community, no import: a valid DNS-only node, no nag.
	tree := newAS112RedistTree(true, "", false, false)
	assert.Empty(t, checkAS112RedistributeNotImported(tree))
}

func TestCheckAS112RedistributeNotImported_Disabled_NoDiagnostic(t *testing.T) {
	// asn set but service disabled: nothing is originated regardless.
	tree := newAS112RedistTree(false, "65001", false, false)
	assert.Empty(t, checkAS112RedistributeNotImported(tree))
}
