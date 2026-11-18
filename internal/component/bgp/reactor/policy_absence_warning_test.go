// Design: docs/guide/bgp-policy.md -- internal learned-export policy diagnostics.
package reactor

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
)

// The actual ordinary destination owner loses no local/API default, but its
// missing learned-export policy must be visible at the Established warning.
// MUTATION: omit the learned-only absence from the shared learnedExportAbsences producer.
func TestLearnedExportWarningReportsAndClears(t *testing.T) {
	f := newSelectedWriterFixture(t, false)
	peer := f.peer
	peer.settings.PeerAS = peer.settings.LocalAS
	peer.settings.Name = "C"
	peer.settings.ExportPolicy = filterapi.Policy{}
	peer.refreshForwardFacts()
	require.NotNil(t, peer.selectedOwnerSnapshot())
	require.NoError(t, peer.selectedOwnerSnapshot().err)
	require.True(t, peer.selectedOwnerSnapshot().owner.IsSet())
	require.False(t, peer.selectedOwnerSnapshot().owner.Explicit())
	sink := rfc5882WarnSink(t)
	peer.warnRFC8212AbsentPolicy()
	lines := rfc5882WarnLines(sink)
	require.Len(t, lines, 1)
	rfc8212RequireLine(t, lines, "peer=C", "no export policy", "no routes learned from other peers", "code=doctor-bgp-learned-export-no-policy", `fix="set bgp peer C filter export final accept"`)
	require.NotContains(t, lines[0], "RFC 8212")
	sink.Reset()
	peer.settings.ExportPolicy = mustPolicy(t, nil, routepolicy.FinalReject)
	peer.refreshForwardFacts()
	peer.warnRFC8212AbsentPolicy()
	require.Empty(t, rfc5882WarnLines(sink), "a configured rejection is the operator's explicit choice")
}
