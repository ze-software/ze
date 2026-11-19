// Design: docs/architecture/l2tp/subscriber-session-model.md -- selecting an original by subscriber attributes

package l2tp

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/subscriberid"
)

// matchCaptureRecorder records the selector a capture receives. The embedded
// nil Service panics on any other call, so the test also proves the capture
// is the only service method CaptureMatchedSession reaches.
type matchCaptureRecorder struct {
	Service
	selectors []SessionSelector
}

func (recorder *matchCaptureRecorder) CaptureSession(selector SessionSelector) (CapturedSession, error) {
	recorder.selectors = append(recorder.selectors, selector)
	return nil, ErrSessionNotFound
}

// TestMatchSessionsKeepsEveryAcceptedRowInSnapshotOrder checks the walk the
// RADIUS listener and the operator clear share: every accepted row across
// tunnels, in snapshot order, and nothing the predicate refused.
func TestMatchSessionsKeepsEveryAcceptedRowInSnapshotOrder(t *testing.T) {
	snapshot := Snapshot{Tunnels: []TunnelSnapshot{
		{LocalTID: 1, Sessions: []SessionSnapshot{
			{LocalSID: 3, TunnelLocalTID: 1, Username: "alice@example.net"},
			{LocalSID: 4, TunnelLocalTID: 1, Username: "bob@example.net"},
		}},
		{LocalTID: 2, Sessions: []SessionSnapshot{
			{LocalSID: 3, TunnelLocalTID: 2, Username: "alice@example.net"},
		}},
	}}
	matched := MatchSessions(&snapshot, func(session *SessionSnapshot) bool {
		return session.Username == "alice@example.net"
	})
	require.Len(t, matched, 2)
	require.Equal(t, uint16(1), matched[0].TunnelLocalTID)
	require.Equal(t, uint16(2), matched[1].TunnelLocalTID)
	require.Empty(t, MatchSessions(&snapshot, func(*SessionSnapshot) bool { return false }))
}

// TestCaptureMatchedSessionQualifiesEveryCoordinate checks that a matched row
// is captured by its session ID, tunnel, listener and Birth together, and that
// a row with no Birth is refused before the service is asked.
func TestCaptureMatchedSessionQualifiesEveryCoordinate(t *testing.T) {
	birth, err := subscriberid.ParseL2TPBirth("l2tp-00000000000000000000000000000001")
	require.NoError(t, err)
	listener := netip.MustParseAddrPort("192.0.2.1:1701")
	recorder := &matchCaptureRecorder{}

	_, err = CaptureMatchedSession(recorder, &SessionSnapshot{LocalSID: 7, TunnelLocalTID: 100, Listener: listener})
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Empty(t, recorder.selectors, "a row without a Birth must never become a lookup by session ID")

	_, err = CaptureMatchedSession(nil, &SessionSnapshot{LocalSID: 7, Birth: birth})
	require.ErrorIs(t, err, ErrSubsystemNotStarted)

	_, err = CaptureMatchedSession(recorder, &SessionSnapshot{LocalSID: 7, TunnelLocalTID: 100, Listener: listener, Birth: birth})
	require.ErrorIs(t, err, ErrSessionNotFound)
	require.Equal(t, []SessionSelector{{SID: 7, TID: 100, Listener: listener, Birth: birth}}, recorder.selectors)
}
