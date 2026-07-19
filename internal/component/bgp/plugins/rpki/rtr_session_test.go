package rpki

import (
	"encoding/binary"
	"testing"
	"time"

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

// TestPollingCadenceAtLeastHourly verifies a fresh session re-queries its cache at least once an
// hour.
//
// VALIDATES: RFC 6810 Section 6.1 -- the router polls (Serial/Reset Query) no less frequently than
// once an hour. The Run loop re-connects and re-queries after waiting s.retryInterval; the default
// retryInterval seeds that cadence below the one-hour ceiling.
// PREVENTS: A default cadence that lets VRP data go stale for more than an hour.
func TestPollingCadenceAtLeastHourly(t *testing.T) {
	// RFC requirement: RFC6810-6.1-1 positive -- a fresh session's default retryInterval (the wait
	// between one sync completing and the next query in Run's loop) is <= one hour, so the router
	// re-queries at least hourly without any cache-supplied interval.
	stopCh := make(chan struct{})
	s := NewRTRSession("192.0.2.1", 3323, 100, "", NewROACache(), NewASPACache(), stopCh)

	assert.LessOrEqual(t, s.retryInterval, time.Hour, "default re-query cadence is at least hourly")
}

// TestCacheResetTriggersResetQuery verifies a Cache Reset PDU puts the session back into full-reset
// mode.
//
// VALIDATES: RFC 6810 Section 6.3 -- on Cache Reset (the cache cannot serve the requested
// incremental) the router issues a Reset Query. handlePDU clears the serial to 0, which is exactly
// the condition connectAndSync uses to send a Reset Query on the next connection.
// PREVENTS: A Cache Reset being ignored, leaving the router stuck sending Serial Queries the cache
// cannot answer.
func TestCacheResetTriggersResetQuery(t *testing.T) {
	newSession := func() *RTRSession {
		stopCh := make(chan struct{})
		return NewRTRSession("192.0.2.1", 3323, 100, "", NewROACache(), NewASPACache(), stopCh)
	}

	t.Run("cache reset clears serial for a reset query", func(t *testing.T) {
		// RFC requirement: RFC6810-6.3-1 positive -- a Cache Reset PDU ends the current sync and
		// clears the serial to 0, so the next connectAndSync sends a Reset Query.
		s := newSession()
		s.serial = 42 // pretend a prior incremental sync

		done, err := s.handlePDU(RTRHeader{Type: pduCacheReset}, make([]byte, pduHeaderLen))

		require.ErrorIs(t, err, errRtrCacheResetReceivedWillDo, "cache reset signals a full re-sync")
		assert.True(t, done, "the current sync attempt ends")
		assert.Equal(t, uint32(0), s.serial, "serial cleared to 0 drives a Reset Query on reconnect")
	})

	t.Run("serial notify does not force a reset query", func(t *testing.T) {
		// RFC requirement: RFC6810-6.3-1 negative -- an ordinary Serial Notify does NOT clear the
		// serial, so the reset-query fallback is specific to Cache Reset, not to any received PDU.
		s := newSession()
		s.serial = 42

		done, err := s.handlePDU(RTRHeader{Type: pduSerialNotify}, make([]byte, pduHeaderLen))

		require.NoError(t, err)
		assert.False(t, done)
		assert.Equal(t, uint32(42), s.serial, "serial preserved: no Reset Query forced")
	})
}

// TestNoDataAvailableKeepsResetQueryMode verifies that a No Data Available error leaves the session
// in the state that re-issues Reset Queries.
//
// VALIDATES: RFC 6810 Section 6.4 -- when the cache reports No Data Available and there is no other
// cache to fall back to, the router keeps issuing periodic Reset Queries. handlePDU treats code 2 as
// non-fatal (the session is not torn down as an error) and leaves the serial at 0, so each Run-loop
// reconnect re-sends a Reset Query.
// PREVENTS: A No Data Available report being treated as fatal (giving up) or silently advancing the
// serial (switching to Serial Queries the empty cache cannot satisfy).
func TestNoDataAvailableKeepsResetQueryMode(t *testing.T) {
	newSession := func() *RTRSession {
		stopCh := make(chan struct{})
		return NewRTRSession("192.0.2.1", 3323, 100, "", NewROACache(), NewASPACache(), stopCh)
	}

	t.Run("no data available is non-fatal and keeps serial at zero", func(t *testing.T) {
		// RFC requirement: RFC6810-6.4-1 positive -- an Error Report with code 2 (No Data Available)
		// is non-fatal: handlePDU returns no error and leaves serial at 0, so the periodic Run-loop
		// reconnect keeps issuing Reset Queries.
		s := newSession()
		require.Equal(t, uint32(0), s.serial)

		hdr := RTRHeader{Type: pduErrorRpt, SessionID: errNoDataAvail, Length: pduHeaderLen}
		done, err := s.handlePDU(hdr, make([]byte, pduHeaderLen))

		require.NoError(t, err, "No Data Available must not be fatal")
		assert.False(t, done, "the session stays up to retry")
		assert.Equal(t, uint32(0), s.serial, "serial stays 0 so the next query is a Reset Query")
		assert.False(t, isFatalError(errNoDataAvail), "code 2 is classified non-fatal")
	})

	t.Run("a completed sync advances the serial out of reset mode", func(t *testing.T) {
		// RFC requirement: RFC6810-6.4-1 negative -- once a sync completes (End of Data), the serial
		// advances to a non-zero value, so the next query is an incremental Serial Query, not a
		// periodic Reset Query. Reset-query polling is specific to the no-data (serial==0) condition.
		s := newSession()

		buf := make([]byte, pduEndOfDataLen)
		buf[1] = pduEndOfData
		binary.BigEndian.PutUint32(buf[4:8], pduEndOfDataLen)
		binary.BigEndian.PutUint32(buf[8:12], 77) // serial number

		done, err := s.handlePDU(RTRHeader{Type: pduEndOfData, Length: pduEndOfDataLen}, buf)

		require.NoError(t, err)
		assert.True(t, done, "End of Data completes the sync")
		assert.Equal(t, uint32(77), s.serial, "serial advanced: session leaves reset-query mode")
	})
}
