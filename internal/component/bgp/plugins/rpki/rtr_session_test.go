package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRTRSessionStartsAtV2 verifies a fresh RTR session opens at protocol version 2.
//
// VALIDATES: RFC 9582 Section 7 — a router that supports RTR v2 starts the session by sending its
// first query with version=2 (rtrVersionMax).
// PREVENTS: Silently negotiating an older version and never exercising the ASPA (v2) PDU path.
func TestRTRSessionStartsAtV2(t *testing.T) {
	// RFC requirement: RFC9582-7-1 positive -- a fresh session's negotiated version is rtrVersionMax
	// (2) and the first query it writes (a Reset Query, since serial==0) carries version byte 2.
	stopCh := make(chan struct{})
	session := NewRTRSession("192.0.2.1", 3323, 100, "", NewROACache(), NewASPACache(), stopCh)

	assert.Equal(t, rtrVersionMax, session.version, "fresh session negotiates the max version")
	assert.Equal(t, uint8(2), session.version, "RFC 9582 v2 session")

	// The first query a serial==0 session sends is a Reset Query written with the negotiated version.
	buf := make([]byte, pduResetQueryLen)
	n := writeResetQuery(buf, 0, session.version)
	assert.Equal(t, pduResetQueryLen, n)
	assert.Equal(t, uint8(2), buf[0], "first query carries protocol version 2")
	assert.Equal(t, pduResetQuery, buf[1])
}

// TestHandlePDUVersionDowngrade verifies the response to an RTR Error Report PDU.
//
// VALIDATES: RFC 9582 Section 7 — an "Unsupported Protocol Version" error (code 4) makes the router
// decrement its version and re-establish (signaled via errRtrVersionDowngrade, which Run treats as
// a reconnect); an unrelated error code does not change the negotiated version.
// PREVENTS: Version downgrade firing on the wrong error, or never firing on version mismatch.
func TestHandlePDUVersionDowngrade(t *testing.T) {
	newSession := func() *RTRSession {
		stopCh := make(chan struct{})
		return NewRTRSession("192.0.2.1", 3323, 100, "", NewROACache(), NewASPACache(), stopCh)
	}

	t.Run("unsupported version downgrades and reconnects", func(t *testing.T) {
		// RFC requirement: RFC9582-7-2 positive -- an Error Report with code 4 (Unsupported Version)
		// decrements the session version (2 -> 1) and signals a reconnect (errRtrVersionDowngrade).
		s := newSession()
		require.Equal(t, rtrVersionMax, s.version)

		hdr := RTRHeader{Version: rtrVersionMax, Type: pduErrorRpt, SessionID: errUnsupportedVersion, Length: pduHeaderLen}
		done, err := s.handlePDU(hdr, make([]byte, pduHeaderLen))

		require.ErrorIs(t, err, errRtrVersionDowngrade, "reconnect must be signaled")
		assert.False(t, done, "sync is not complete on downgrade")
		assert.Equal(t, rtrVersionMin, s.version, "version decremented to v1")
	})

	t.Run("unrelated error code does not downgrade", func(t *testing.T) {
		// RFC requirement: RFC9582-7-2 negative -- an Error Report carrying a different code (No Data
		// Available, 2) does NOT decrement the version: the downgrade is specific to code 4.
		s := newSession()
		require.Equal(t, rtrVersionMax, s.version)

		hdr := RTRHeader{Version: rtrVersionMax, Type: pduErrorRpt, SessionID: errNoDataAvail, Length: pduHeaderLen}
		done, err := s.handlePDU(hdr, make([]byte, pduHeaderLen))

		require.NoError(t, err, "a non-fatal, non-version error is tolerated")
		assert.False(t, done)
		assert.Equal(t, rtrVersionMax, s.version, "version unchanged by an unrelated error")
	})
}
