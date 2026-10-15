// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/asn"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// ribSections returns one bgp delivery carrying bgpBody as the bgp subtree.
func ribSections(bgpBody string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootBGP, Data: `{"bgp":{` + bgpBody + `}}`}}
}

// TestRIBMaximumPathsRemovalRestoresDefault proves R-6 and AC-2: a reload
// that removes maximum-paths returns the RIB to one best path, the YANG
// default, instead of keeping the value it ran with.
// Method: configure 4, verify and apply a candidate without the multipath
// block, read the installed value; then roll back and read 4 again.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-6, AC-2, AC-4 for bgp-rib.
// PREVENTS: an absent leaf leaving the previous maximum-paths in force.
func TestRIBMaximumPathsRemovalRestoresDefault(t *testing.T) {
	r := newRIBManager(nil)
	tx := r.newConfigTx(false)
	require.NoError(t, tx.Configure(ribSections(`"multipath":{"maximum-paths":4,"relax-as-path":true}`)))
	require.Equal(t, uint32(4), r.maximumPaths.Load())
	require.True(t, r.relaxASPath.Load())

	require.NoError(t, tx.Verify(ribSections(`"router-id":"10.0.0.1"`)))
	require.NoError(t, tx.Apply())
	require.Equal(t, uint32(1), r.maximumPaths.Load(), "a removed maximum-paths is the default 1")
	require.False(t, r.relaxASPath.Load())

	require.NoError(t, tx.Rollback())
	require.Equal(t, uint32(4), r.maximumPaths.Load(), "rollback restores the previous maximum-paths")
	require.True(t, r.relaxASPath.Load())
}

// TestRIBMaximumPathsReloadBoundary proves the boundary row of the spec:
// 1 and 256 are accepted, 0 and 257 are refused at verify, and a refused
// verify leaves the running value and opens no apply.
// VALIDATES: spec-plugin-reload-applies-or-refuses boundary row and AC-3 for bgp-rib.
// PREVENTS: an out-of-range value read as "no multipath" in silence.
func TestRIBMaximumPathsReloadBoundary(t *testing.T) {
	r := newRIBManager(nil)
	tx := r.newConfigTx(false)
	require.NoError(t, tx.Configure(ribSections(`"multipath":{"maximum-paths":2}`)))

	for _, valid := range []string{`1`, `256`, `"256"`} {
		require.NoError(t, tx.Verify(ribSections(`"multipath":{"maximum-paths":`+valid+`}`)), "maximum-paths %s", valid)
	}
	for _, invalid := range []string{`0`, `257`, `"257"`, `"abc"`, `-1`} {
		err := tx.Verify(ribSections(`"multipath":{"maximum-paths":` + invalid + `}`))
		require.Error(t, err, "maximum-paths %s", invalid)
		require.Contains(t, err.Error(), "maximum-paths")
		require.ErrorIs(t, tx.Apply(), sdk.ErrConfigApplyUnverified, "a refused verify opens no apply")
		require.Equal(t, uint32(2), r.maximumPaths.Load(), "a refused candidate keeps the running value")
	}
}

// TestRIBReselectOnRuleChange proves AC-6 and R-5: raising maximum-paths
// on reload reselects the routes already stored, so the Loc-RIB carries the
// equal-cost sibling without any new UPDATE; the rollback reselects back.
// Method: two peers store the same prefix with equal attributes; the Loc-RIB
// path's ECMP list is read after the first selection, after the apply and
// after the rollback.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-6, R-5, AC-4 for bgp-rib.
// PREVENTS: stored prefixes keeping the old multipath set until their next UPDATE.
// MUTATION: drop the reselect call from the install and the apply check fails.
func TestRIBReselectOnRuleChange(t *testing.T) {
	r := newRIBManager(nil)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	t.Cleanup(func() { r.SetLocRIB(nil) })
	tx := r.newConfigTx(false)
	require.NoError(t, tx.Configure(ribSections(`"multipath":{"maximum-paths":1}`)))

	wirePrefix := ipv4Prefix(24, 10, 30, 0)
	for i, text := range []string{"192.0.2.1", "192.0.2.2"} {
		peer := netip.MustParseAddr(text)
		r.peerMeta[peer] = &peerMetadata{PeerASN: 65001, LocalASN: 65001}
		routes := storage.NewPeerRIB(text)
		r.bgpPeers[peer] = routes
		t.Cleanup(routes.Release)
		routes.Insert(family.IPv4Unicast, aigpSelectionAttrs([4]byte{198, 51, 100, byte(i + 1)}, nil, 100, 1), wirePrefix)
	}
	r.reselectAIGPRoutes()
	ecmp := func() int {
		path, _, found := loc.LPM(family.IPv4Unicast, netip.MustParseAddr("10.30.0.1"))
		require.True(t, found)
		return len(path.ECMP)
	}
	require.Equal(t, 0, ecmp(), "maximum-paths 1 selects one path")

	require.NoError(t, tx.Verify(ribSections(`"multipath":{"maximum-paths":2}`)))
	require.NoError(t, tx.Apply())
	require.Equal(t, 1, ecmp(), "the apply reselects the stored prefix into two paths")

	require.NoError(t, tx.Rollback())
	require.Equal(t, 0, ecmp(), "the rollback reselects the stored prefix back to one path")
}

// TestRIBASNotationSingleWriter proves R-16 and A-9: the rib records the
// AS notation only when it runs in a process of its own. In the daemon the
// reactor's SetConfigTree is the one writer, and it runs only after a
// reload committed, so a rib write would survive a refused commit.
// Method: an in-process transaction applies asdot and the process notation
// stays asplain; an external one records nothing at verify, asdot at apply,
// asplain again at rollback.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-16, A-9.
// PREVENTS: two writers of one process-wide setting disagreeing after a rollback.
// Not parallel: the notation is process state.
func TestRIBASNotationSingleWriter(t *testing.T) {
	original := asn.Configured()
	t.Cleanup(func() { asn.Record(original) })
	asn.Record(asn.NotationPlain)

	inProcess := newRIBManager(nil).newConfigTx(false)
	require.NoError(t, inProcess.Configure(ribSections(`"as-notation":"asdot"`)))
	require.Equal(t, asn.NotationPlain, asn.Configured(), "an in-process rib never writes the notation")
	require.NoError(t, inProcess.Verify(ribSections(`"as-notation":"asdot+"`)))
	require.NoError(t, inProcess.Apply())
	require.Equal(t, asn.NotationPlain, asn.Configured(), "an in-process rib apply never writes the notation")

	external := newRIBManager(nil).newConfigTx(true)
	require.NoError(t, external.Verify(ribSections(`"as-notation":"asdot"`)))
	require.Equal(t, asn.NotationPlain, asn.Configured(), "verify records nothing")
	require.NoError(t, external.Apply())
	require.Equal(t, asn.NotationDot, asn.Configured(), "an external rib records the applied notation")
	require.NoError(t, external.Rollback())
	require.Equal(t, asn.NotationPlain, asn.Configured(), "its rollback records the previous notation")

	require.Error(t, external.Verify(ribSections(`"as-notation":""`)), "an empty notation is refused at verify")
}
