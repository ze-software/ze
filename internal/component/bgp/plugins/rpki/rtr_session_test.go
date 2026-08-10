package rpki

import (
	"context"
	"encoding/binary"
	"io"
	"net"
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
	session := newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)

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
		return newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
	}

	t.Run("unsupported version downgrades and reconnects", func(t *testing.T) {
		// RFC requirement: RFC9582-7-2 positive -- an Error Report with code 4 (Unsupported Version)
		// decrements the session version (2 -> 1) and signals a reconnect (errRtrVersionDowngrade).
		s := newSession()
		require.Equal(t, rtrVersionMax, s.version)

		hdr := rTRHeader{Version: rtrVersionMax, Type: pduErrorRpt, SessionID: errUnsupportedVersion, Length: pduHeaderLen}
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

		hdr := rTRHeader{Version: rtrVersionMax, Type: pduErrorRpt, SessionID: errNoDataAvail, Length: pduHeaderLen}
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
	// RFC requirement: RFC8210-8.1-1 positive -- the same Run-loop wait is what makes the router send
	// a Serial Query or Reset Query periodically under v1; the seeded interval is finite and short, so
	// polling recurs rather than stopping after the first sync.
	stopCh := make(chan struct{})
	s := newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)

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
		return newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
	}

	t.Run("cache reset clears serial for a reset query", func(t *testing.T) {
		// RFC requirement: RFC6810-6.3-1 positive -- a Cache Reset PDU ends the current sync and
		// clears the serial to 0, so the next connectAndSync sends a Reset Query.
		// RFC requirement: RFC8210-8.3-1 positive -- with no more-preferred cache to fall back to
		// (ze runs every configured cache in parallel), the Cache Reset drives this same session back
		// to serial 0, which is exactly the Reset Query that fetches an entire new load.
		s := newSession()
		s.serial = 42 // pretend a prior incremental sync

		done, err := s.handlePDU(rTRHeader{Type: pduCacheReset}, make([]byte, pduHeaderLen))

		require.ErrorIs(t, err, errRtrCacheResetReceivedWillDo, "cache reset signals a full re-sync")
		assert.True(t, done, "the current sync attempt ends")
		assert.Equal(t, uint32(0), s.serial, "serial cleared to 0 drives a Reset Query on reconnect")
	})

	t.Run("serial notify does not force a reset query", func(t *testing.T) {
		// RFC requirement: RFC6810-6.3-1 negative -- an ordinary Serial Notify does NOT clear the
		// serial, so the reset-query fallback is specific to Cache Reset, not to any received PDU.
		// RFC requirement: RFC8210-8.3-1 negative -- a PDU that is not a Cache Reset leaves the serial
		// intact, so the full reload is triggered only by the Cache Reset the RFC names and never by an
		// arbitrary cache-to-router PDU.
		s := newSession()
		s.serial = 42

		done, err := s.handlePDU(rTRHeader{Type: pduSerialNotify}, make([]byte, pduHeaderLen))

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
		return newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
	}

	t.Run("no data available is non-fatal and keeps serial at zero", func(t *testing.T) {
		// RFC requirement: RFC6810-6.4-1 positive -- an Error Report with code 2 (No Data Available)
		// is non-fatal: handlePDU returns no error and leaves serial at 0, so the periodic Run-loop
		// reconnect keeps issuing Reset Queries.
		// RFC requirement: RFC8210-8.4-1 positive -- when the cache cannot supply an update and there
		// is no other cache to switch to, this is the state that makes the router keep issuing periodic
		// Reset Queries: non-fatal handling plus serial 0.
		s := newSession()
		require.Equal(t, uint32(0), s.serial)

		hdr := rTRHeader{Type: pduErrorRpt, SessionID: errNoDataAvail, Length: pduHeaderLen}
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
		// RFC requirement: RFC8210-8.4-1 negative -- a session that HAS been supplied an update leaves
		// periodic-Reset-Query mode, so the periodic reload is confined to the cannot-supply case and
		// does not discard a healthy incremental sync.
		s := newSession()

		buf := make([]byte, pduEndOfDataLen)
		buf[1] = pduEndOfData
		binary.BigEndian.PutUint32(buf[4:8], pduEndOfDataLen)
		binary.BigEndian.PutUint32(buf[8:12], 77) // serial number

		done, err := s.handlePDU(rTRHeader{Type: pduEndOfData, Length: pduEndOfDataLen}, buf)

		require.NoError(t, err)
		assert.True(t, done, "End of Data completes the sync")
		assert.Equal(t, uint32(77), s.serial, "serial advanced: session leaves reset-query mode")
	})
}

// TestSerialNotifyIgnoredDuringStartup verifies that a Serial Notify arriving in the startup window
// (before version negotiation has settled and before the Cache Response) changes nothing.
//
// VALIDATES: RFC 8210 Sections 5.2 and 7 -- the router MUST ignore Serial Notify PDUs received from
// the cache during the initial startup period. handlePDU's pduSerialNotify arm returns (false, nil)
// without touching serial, sessionID or state (rtr_session.go:359-361).
// PREVENTS: A pre-negotiation Serial Notify advancing the serial, adopting a Session ID, or aborting
// the startup exchange -- and equally, a blanket "drop everything during startup" that would swallow
// the Cache Response the exchange depends on.
func TestSerialNotifyIgnoredDuringStartup(t *testing.T) {
	newSession := func() *RTRSession {
		stopCh := make(chan struct{})
		return newRTRSession("192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
	}

	t.Run("serial notify in the startup window is ignored", func(t *testing.T) {
		// RFC requirement: RFC8210-5.2-1 positive -- a Serial Notify received before the Cache Response
		// (state still "idle", the initial startup period) is ignored: no error, sync not complete, and
		// neither the Session ID it carries nor the serial in its body is adopted.
		// RFC requirement: RFC8210-7-8 positive -- the same PDU received before version negotiation has
		// completed is handled by ignoring it, so it neither aborts nor perturbs negotiation.
		s := newSession()
		require.Equal(t, sessionIdle, s.state, "startup window: no Cache Response seen")

		buf := make([]byte, 12)
		buf[0] = rtrVersionMax
		buf[1] = pduSerialNotify
		binary.BigEndian.PutUint16(buf[2:4], 0xBEEF) // Session ID offered by the notify
		binary.BigEndian.PutUint32(buf[4:8], 12)
		binary.BigEndian.PutUint32(buf[8:12], 999) // serial offered by the notify

		done, err := s.handlePDU(rTRHeader{Version: rtrVersionMax, Type: pduSerialNotify, SessionID: 0xBEEF, Length: 12}, buf)

		require.NoError(t, err, "a startup Serial Notify must not error the session")
		assert.False(t, done, "it does not complete or abort the startup exchange")
		assert.Equal(t, uint32(0), s.serial, "the notified serial is not adopted")
		assert.Equal(t, uint16(0), s.sessionID, "the notified Session ID is not adopted")
		assert.Equal(t, sessionIdle, s.state, "the startup state is untouched")
		assert.Equal(t, rtrVersionMax, s.version, "negotiation is not perturbed")
	})

	t.Run("cache response in the same window is acted on", func(t *testing.T) {
		// RFC requirement: RFC8210-5.2-1 negative -- the ignore is specific to Serial Notify: a Cache
		// Response arriving in the same startup window IS processed (Session ID adopted, state moves to
		// establish). An implementation that dropped every startup PDU would pass the positive case and
		// fail here, so the pair pins "ignore Serial Notify" rather than "ignore everything".
		// RFC requirement: RFC8210-7-8 negative -- likewise, handling Serial Notify by ignoring it does
		// not extend to the PDU that carries the negotiated Session ID.
		s := newSession()

		done, err := s.handlePDU(rTRHeader{Version: rtrVersionMax, Type: pduCacheResp, SessionID: 0xBEEF, Length: pduHeaderLen}, make([]byte, pduHeaderLen))

		require.NoError(t, err)
		assert.False(t, done)
		assert.Equal(t, uint16(0xBEEF), s.sessionID, "Cache Response is not ignored")
		assert.Equal(t, sessionEstablish, s.state, "the exchange advances")
	})
}

// TestFirstPDUOnConnectionIsAQuery verifies the router opens every transport connection with a query.
//
// VALIDATES: RFC 8210 Sections 7, 8.1 -- a router MUST start each transport connection by issuing
// either a Reset Query or a Serial Query. connectAndSync writes one of the two before entering
// readLoop (rtr_session.go:165-176), which is also what starts version negotiation.
// PREVENTS: A connection opening silently (waiting for the cache to speak first), which would stall
// the session and never negotiate a version.
func TestFirstPDUOnConnectionIsAQuery(t *testing.T) {
	// firstPDU dials a throwaway listener with the given session and returns the first bytes written.
	firstPDU := func(t *testing.T, prepare func(*RTRSession), want int) []byte {
		t.Helper()
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = ln.Close() }()

		tcpAddr, ok := ln.Addr().(*net.TCPAddr)
		require.True(t, ok, "listener address is a TCP address")
		port := tcpAddr.Port
		stopCh := make(chan struct{})
		defer close(stopCh)

		s := newRTRSession("127.0.0.1", uint16(port), 100, "", newROACache(), newASPACache(), stopCh) //nolint:gosec // listener port fits uint16
		prepare(s)

		done := make(chan error, 1)
		go func() { done <- s.connectAndSync() }()

		conn, err := ln.Accept()
		require.NoError(t, err)
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))

		buf := make([]byte, want)
		_, err = io.ReadFull(conn, buf)
		require.NoError(t, err, "the router must speak first on a new connection")

		_ = conn.Close() // ends readLoop so connectAndSync returns
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("connectAndSync did not return after the cache closed the connection")
		}
		return buf
	}

	t.Run("fresh session opens with a reset query", func(t *testing.T) {
		// RFC requirement: RFC8210-7-1 positive -- the very first bytes a fresh session writes on a
		// newly established transport are a Reset Query PDU, carrying the version that starts
		// negotiation.
		// RFC requirement: RFC8210-8.1-2 positive -- when the transport is first established with no
		// prior serial, that opening PDU is the Reset Query the RFC requires.
		buf := firstPDU(t, func(*RTRSession) {}, pduResetQueryLen)

		assert.Equal(t, rtrVersionMax, buf[0], "opening query carries the router's protocol version")
		assert.Equal(t, pduResetQuery, buf[1], "a serial-less session opens with a Reset Query")
		assert.Equal(t, uint32(pduResetQueryLen), binary.BigEndian.Uint32(buf[4:8]))
	})

	t.Run("resuming session opens with a serial query", func(t *testing.T) {
		// RFC requirement: RFC8210-7-1 positive -- a session resuming from a known serial still opens
		// the connection with a query, this time the Serial Query carrying its Session ID and serial;
		// the "connection starts with a query" rule holds on both branches.
		// RFC requirement: RFC8210-8.1-2 positive -- the same holds for a re-established transport.
		buf := firstPDU(t, func(s *RTRSession) {
			s.serial = 42
			s.sessionID = 0x1234
		}, pduSerialQueryLen)

		assert.Equal(t, rtrVersionMax, buf[0])
		assert.Equal(t, pduSerialQuery, buf[1], "a resuming session opens with a Serial Query")
		assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(buf[2:4]))
		assert.Equal(t, uint32(42), binary.BigEndian.Uint32(buf[8:12]))
	})
}
