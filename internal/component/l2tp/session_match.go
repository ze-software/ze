// Design: docs/architecture/l2tp/subscriber-session-model.md -- selecting an original by subscriber attributes
// Related: session_capture.go -- CaptureSession, the guarded capture a match feeds

package l2tp

// MatchSessions answers a copy of every session row of snapshot that match
// accepts, in the snapshot's order (tunnels by local ID, then sessions by
// local ID). It is the one walk every selection by a subscriber attribute
// uses: the RADIUS Disconnect-Request and CoA-Request listener, and the
// operator's clear by user name or address. The walk is bounded by the
// snapshot's rows. A match is a read of a cold copy, never authority: the
// caller binds the original with CaptureMatchedSession before acting on it.
func MatchSessions(snapshot *Snapshot, match func(*SessionSnapshot) bool) []SessionSnapshot {
	var matched []SessionSnapshot
	for tunnelIndex := range snapshot.Tunnels {
		tunnel := &snapshot.Tunnels[tunnelIndex]
		for sessionIndex := range tunnel.Sessions {
			session := &tunnel.Sessions[sessionIndex]
			if !match(session) {
				continue
			}
			matched = append(matched, *session)
		}
	}
	return matched
}

// CaptureMatchedSession binds the original a MatchSessions row names,
// qualified by every readable coordinate and its Birth, so a session ID that a
// replacement reused since the snapshot cannot be selected. A row with no
// valid Birth is refused as not found: it never becomes an unguarded lookup
// by session ID. A nil service answers ErrSubsystemNotStarted.
func CaptureMatchedSession(service Service, matched *SessionSnapshot) (CapturedSession, error) {
	if !matched.Birth.IsValid() {
		return nil, ErrSessionNotFound
	}
	if service == nil {
		return nil, ErrSubsystemNotStarted
	}
	return service.CaptureSession(SessionSelector{
		SID: matched.LocalSID, TID: matched.TunnelLocalTID,
		Listener: matched.Listener, Birth: matched.Birth,
	})
}
