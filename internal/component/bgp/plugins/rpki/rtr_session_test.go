package rpki

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRTRSession(t *testing.T, address string, port uint16, pref uint8, source string, cache *ROACache, aspas *aSPACache, stop <-chan struct{}) *RTRSession {
	t.Helper()
	session := newRTRSession(address, port, pref, source, cache, aspas, stop)
	t.Cleanup(func() {
		session.close()
		if session.dataLease != nil {
			session.dataLease.stop()
		}
	})
	return session
}

// TestRTRSessionStartsAtV2 exercises the actual TCP queries, including a cache
// that requires a supported lower version rather than the router's first choice.
func TestRTRSessionStartsAtV2(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-1 positive -- the first TCP query uses the highest implemented RTR version, 2.
	queries := make(chan [2]byte, 2)
	replies := make(chan error, 2)
	attempt := 0
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			replies <- err
			return
		}
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			replies <- err
			return
		}
		if query[1] == pduSerialQuery {
			var serial [4]byte
			if _, err := io.ReadFull(conn, serial[:]); err != nil {
				replies <- err
				return
			}
		}
		queries <- [2]byte{query[0], query[1]}
		attempt++
		if attempt == 1 {
			report := make([]byte, 16)
			n := writeErrorReport(report, 1, errUnsupportedVersion, nil)
			_, err := conn.Write(report[:n])
			replies <- err
			return
		}
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = 1, 1
		_, err := conn.Write(append(response, end...))
		replies <- err
	})
	roas, aspas := newROACache(), newASPACache()
	roas.Add(makeVRP("10.0.0.0/24", 24, 64500))
	aspas.Set(64500, []uint32{100})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", roas, aspas, make(chan struct{}))
	s.serial = 42
	require.NoError(t, s.syncOnce())
	require.NoError(t, <-replies)
	require.NoError(t, <-replies)
	assert.Equal(t, [2]byte{2, pduSerialQuery}, <-queries)
	assert.Equal(t, [2]byte{1, pduResetQuery}, <-queries)
	assert.Equal(t, HopNoAttestation, aspas.checkPair(100, 64500))
	assert.Equal(t, ValidationNotFound, roas.Validate("10.0.0.0/24", 64500))
}

// TestHandlePDUVersionDowngrade rejects an unsupported-version response with
// no common lower version, rather than tolerating it forever on the same stream.
func TestHandlePDUVersionDowngrade(t *testing.T) {
	// RFC requirement: RFC8210-12-1 positive -- a fatal Unsupported Protocol Version error with no common version closes the transport.
	closed := make(chan error, 1)
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			closed <- err
			return
		}
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			closed <- err
			return
		}
		report := make([]byte, 16)
		n := writeErrorReport(report, query[0], errUnsupportedVersion, nil)
		if _, err := conn.Write(report[:n]); err != nil {
			closed <- err
			return
		}
		_, err := conn.Read(query)
		closed <- err
	})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
	require.Error(t, s.syncOnce())
	require.ErrorIs(t, <-closed, io.EOF)
}

// TestRTRSessionRejectsV0Advertisement guards against guessing v1 after a cache
// advertises v0, as StayRTR v0.6.4 does with its default negotiation settings.
func TestRTRSessionRejectsV0Advertisement(t *testing.T) {
	queries := make(chan byte, 4)
	closed := make(chan error, 4)
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			closed <- err
			return
		}
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			closed <- err
			return
		}
		queries <- query[0]
		report := make([]byte, 16)
		n := writeErrorReport(report, 0, errUnsupportedVersion, nil)
		if _, err := conn.Write(report[:n]); err != nil {
			closed <- err
			return
		}
		_, err := conn.Read(query)
		closed <- err
	})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
	require.Error(t, s.syncOnce())
	require.ErrorIs(t, <-closed, io.EOF)
	require.Equal(t, byte(2), <-queries)
	select {
	case version := <-queries:
		t.Fatalf("cache advertised unsupported v0, but router retried at v%d", version)
	default:
	}
}

// TestASPAProviderListErrorOnWire checks the fatal report actually leaves the
// router and the rejected announcement never becomes a usable authorization.
func TestASPAProviderListErrorOnWire(t *testing.T) {
	// RFC requirement: RFC8210-5.11-1 negative -- a malformed non-Error-Report PDU still receives its required Error Report.
	// RFC requirement: RFC8210-5.11-4 positive -- the actual transmitted report has zero text length when diagnostic text is omitted.
	reported := make(chan []byte, 1)
	serverErr := make(chan error, 1)
	bad := aspaPDU(64500)
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			serverErr <- err
			return
		}
		query := make([]byte, pduResetQueryLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			serverErr <- err
			return
		}
		if _, err := conn.Write(bad); err != nil {
			serverErr <- err
			return
		}
		report := make([]byte, 16+len(bad))
		_, err := io.ReadFull(conn, report)
		reported <- report
		serverErr <- err
	})
	cache := newASPACache()
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), cache, make(chan struct{}))
	require.ErrorIs(t, s.syncOnce(), errASPAProviderList)
	require.NoError(t, <-serverErr)
	report := <-reported
	assert.Equal(t, byte(2), report[0])
	assert.Equal(t, pduErrorRpt, report[1])
	assert.Equal(t, uint16(9), binary.BigEndian.Uint16(report[2:4]))
	assert.Equal(t, bad, report[12:12+len(bad)])
	assert.Equal(t, uint32(0), binary.BigEndian.Uint32(report[12+len(bad):]))
	assert.Equal(t, HopNoAttestation, cache.checkPair(100, 64500))
}

func TestRTRErrorReportIsNotAnswered(t *testing.T) {
	// RFC requirement: RFC8210-5.11-1 positive -- an erroneous Error Report is not answered with another Error Report.
	client, cache := net.Pipe()
	finished := make(chan struct{})
	observed := make(chan error, 1)
	t.Cleanup(func() {
		_ = client.Close()
		_ = cache.Close()
		<-finished
	})
	go func() {
		defer close(finished)
		defer func() { _ = cache.Close() }()
		if err := cache.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			observed <- err
			return
		}
		// An unknown-version Error Report exercises the otherwise applicable
		// Unsupported Protocol Version response, not a harmless error code.
		report := []byte{99, pduErrorRpt, 0, 0, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0}
		if _, err := cache.Write(report); err != nil {
			observed <- err
			return
		}
		var answer [1]byte
		_, err := io.ReadFull(cache, answer[:])
		observed <- err
	}()
	s := newTestRTRSession(t, "cache.invalid", 323, 100, "", newROACache(), newASPACache(), make(chan struct{}))
	require.Error(t, s.readLoop(client))
	require.NoError(t, client.Close())
	require.ErrorIs(t, <-observed, io.EOF)
}

// TestRTRUnknownNegotiationVersion checks the receive-side negotiation rule
// over TCP, including the required Error Report and transport termination.
func TestRTRUnknownNegotiationVersion(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-2 positive -- an unrecognized version 99 Cache Response produces Error Report 4 and transport termination.
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-2 negative -- a recognized version 2 Cache Response completes synchronization without an Unsupported Version report.
	// RFC requirement: RFC8210-7-7 positive -- an unknown received version produces Error Report 4 and closes the stream.
	// RFC requirement: RFC8210-7-7 negative -- a supported received version completes its data transfer without an unsupported-version error.
	// RFC requirement: RFC8210-7-3 positive -- a v0 Cache Response closes the stream instead of publishing an unsupported-version load.
	for _, version := range []byte{0, 2, 99} {
		name := map[byte]string{0: "unsupported v0", 2: "recognized", 99: "unrecognized"}[version]
		t.Run(name, func(t *testing.T) {
			type reply struct {
				report []byte
				err    error
			}
			observed := make(chan reply, 1)
			port, _ := serveRTR(t, func(conn net.Conn) {
				if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
					observed <- reply{err: err}
					return
				}
				query := make([]byte, pduResetQueryLen)
				if _, err := io.ReadFull(conn, query); err != nil {
					observed <- reply{err: err}
					return
				}
				response := cacheResponsePDU()
				response[0] = version
				payload := response
				if version == 2 {
					end := endOfDataPDU()
					end[0] = version
					payload = append(payload, end...)
				}
				if _, err := conn.Write(payload); err != nil {
					observed <- reply{err: err}
					return
				}
				var report []byte
				if version != 2 {
					report = make([]byte, 16+len(response))
					if _, err := io.ReadFull(conn, report); err != nil {
						observed <- reply{err: err}
						return
					}
				}
				_, err := conn.Read(query)
				observed <- reply{report: report, err: err}
			})
			s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
			err := s.syncOnce()
			if version != 2 {
				require.ErrorIs(t, err, errRtrUnsupportedProtocol)
			} else {
				require.NoError(t, err)
			}
			got := <-observed
			require.ErrorIs(t, got.err, io.EOF)
			if version != 2 {
				require.Len(t, got.report, 24)
				assert.Equal(t, pduErrorRpt, got.report[1])
				assert.Equal(t, errUnsupportedVersion, binary.BigEndian.Uint16(got.report[2:4]))
				assert.Equal(t, version, got.report[12])
			}
		})
	}
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
		return newTestRTRSession(t, "192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
	}

	t.Run("cache reset clears serial for a reset query", func(t *testing.T) {
		// RFC requirement: RFC6810-6.3-1 positive -- a Cache Reset PDU ends the current sync and
		// clears the serial to 0, so the next connectAndSync sends a Reset Query.
		// RFC requirement: RFC8210-8.3-1 positive -- with no more-preferred cache to fall back to
		// (ze runs every configured cache in parallel), the Cache Reset drives this same session back
		// to serial 0, which is exactly the Reset Query that fetches an entire new load.
		s := newSession()
		s.serial = 42 // pretend a prior incremental sync

		done, err := s.handlePDU(rTRHeader{Version: rtrVersionMax, Type: pduCacheReset}, make([]byte, pduHeaderLen))

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

// No Data Available must end the current response, even if the cache keeps the
// TCP connection open. Otherwise neither failover nor the retry timer can run.
func TestNoDataAvailableKeepsResetQueryMode(t *testing.T) {
	// RFC requirement: RFC8210-8.4-1 positive -- consecutive No Data Available responses on open connections produce periodic Reset Queries.
	// RFC requirement: RFC8210-8.4-1 negative -- after a complete load arrives, the next query uses its serial rather than discarding it for another reset.
	queries := make(chan uint8, 4)
	attempt := 0
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		query := make([]byte, pduHeaderLen)
		if _, err := io.ReadFull(conn, query); err != nil {
			return
		}
		if query[1] == pduSerialQuery {
			if _, err := io.CopyN(io.Discard, conn, 4); err != nil {
				return
			}
		}
		queries <- query[1]
		attempt++
		if attempt <= 2 {
			report := []byte{1, pduErrorRpt, 0, 2, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0}
			if _, err := conn.Write(report); err != nil {
				return
			}
			// The router must close this unanswered transaction rather than
			// wait for the cache's EOF before scheduling the retry.
			_, _ = conn.Read(query)
			return
		}
		response, end := cacheResponsePDU(), endOfDataPDU()
		response[0], end[0] = 1, 1
		binary.BigEndian.PutUint32(end[8:12], 77)
		prefix := []byte{1, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 192, 0, 2, 0, 0, 0, 0xfb, 0xf4}
		payload := slices.Concat(response, prefix, end)
		if _, err := conn.Write(payload); err != nil {
			return
		}
	})
	stop := make(chan struct{})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), stop)
	s.version = rtrVersionMin
	s.retryInterval = time.Second
	done := make(chan struct{})
	go func() { defer close(done); newCacheGroup([]*RTRSession{s}, stop).Run() }()
	t.Cleanup(func() {
		close(stop)
		s.close()
		<-done
	})
	for range 3 {
		select {
		case query := <-queries:
			assert.Equal(t, pduResetQuery, query)
		case <-time.After(4 * time.Second):
			t.Fatal("No Data Available did not trigger a periodic Reset Query")
		}
	}
	require.Eventually(t, func() bool {
		return s.cache.Validate("192.0.2.0/24", 64500) == ValidationValid && s.Snapshot().State == sessionIdle
	}, 4*time.Second, time.Millisecond, "the later complete load was not published")
	// The group is now waiting an hour for refresh. A direct next poll must
	// retain the just-published serial rather than the no-data reset state.
	require.NoError(t, s.syncOnce())
	select {
	case query := <-queries:
		assert.Equal(t, pduSerialQuery, query)
	case <-time.After(4 * time.Second):
		t.Fatal("completed load did not permit the next incremental query")
	}
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
		return newTestRTRSession(t, "192.0.2.1", 3323, 100, "", newROACache(), newASPACache(), stopCh)
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

		s := newTestRTRSession(t, "127.0.0.1", uint16(port), 100, "", newROACache(), newASPACache(), stopCh) //nolint:gosec // listener port fits uint16
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
