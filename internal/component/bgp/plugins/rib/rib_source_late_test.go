// Design: docs/architecture/plugin/rib-storage-design.md -- closed RIB admission.
// Related: rib_source_shutdown.go -- shutdown closes all new work first.
package rib

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// stoppedCaptureFixture models shutdown after admission is closed but before
// an already admitted reader has returned. Retained storage must not cause a
// newly refused operation to publish its unfinished selection.
func stoppedCaptureFixture(t *testing.T) (*RIBManager, netip.Addr, uint64) {
	t.Helper()
	r, peer := advertiseCaptureRIB(t)
	raw := ipv4Prefix(24, 198, 18, 241)
	advertiseCapturePath(t, r, advertiseCaptureOwner, family.IPv4Unicast, raw, unicastAttrs(peer.As4(), 30, 100), 241, 5)
	_, changed := r.checkBestPathChange(family.IPv4Unicast, raw, false, nil)
	require.True(t, changed)
	first, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, first, 1)
	advertiseCapturePath(t, r, advertiseCaptureOwner, family.IPv4Unicast, raw, unicastAttrs(peer.As4(), 40, 200), 242, 5)
	r.peerMu.Lock()
	r.stopping = true
	r.peerMu.Unlock()
	return r, peer, first[0].revision
}

// TestRIBShutdownRefusedMarkDoesNotReconcile ensures the closing guard also
// excludes deferred election and publication, while its reader's store lives.
func TestRIBShutdownRefusedMarkDoesNotReconcile(t *testing.T) {
	r, peer, revision := stoppedCaptureFixture(t)
	_, _, err := r.markStaleCommand([]string{peer.String(), "60"})
	require.ErrorIs(t, err, errRIBStopped)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, revision, current[0].revision)
	require.Equal(t, uint64(241), current[0].route.MessageID)
	require.Positive(t, r.bgpPeers[advertiseCaptureOwner].Len(), "an admitted reader still owns its useful stored route")
}

// TestRIBShutdownRefusedJSONDoesNotReconcile covers the JSON return defer,
// independently of structured delivery and without an SDK cancellation guard.
func TestRIBShutdownRefusedJSONDoesNotReconcile(t *testing.T) {
	r, peer, revision := stoppedCaptureFixture(t)
	event, err := parseEvent([]byte(transportReceived(peer.String(), 243, 701, 0, "198.18.241.0/24", "18c612f1")))
	require.NoError(t, err)
	r.handleReceived(event)
	current, err := r.advertiseSnapshot()
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, revision, current[0].revision)
	require.Equal(t, uint64(241), current[0].route.MessageID)
	require.Positive(t, r.bgpPeers[advertiseCaptureOwner].Len())
}

// TestRIBSourceLeaseStoppedOwnerDoesNotReport proves that a direct caller's
// live context cannot bypass source report admission closed by shutdown.
func TestRIBSourceLeaseStoppedOwnerDoesNotReport(t *testing.T) {
	f := newSourceFailureFixture(t)
	s := f.r.sourceLeases
	peer := netip.MustParseAddr("192.0.2.242")
	f.r.peerMeta[701] = &peerMetadata{Address: peer}
	f.r.peerMeta[702] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.243")}
	s.setAdmission(701, errors.New("prior source failure"))
	s.reported[701] = "previously acknowledged warning"
	s.stop()
	require.False(t, s.reportFailures(context.Background()))
	require.Empty(t, f.reports, "an ended SDK owner must not raise or clear warnings")
	s.setAdmission(702, errors.New("late failure"))
	require.Len(t, s.failures(), 1, "late intake must not repopulate stopped fault state")
}
