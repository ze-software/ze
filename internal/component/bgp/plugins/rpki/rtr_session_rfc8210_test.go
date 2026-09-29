// Design: docs/guide/rpki.md -- the RTR client's session rules, driven over a real transport.
// Related: rtr_session.go -- readLoop, handlePDU, cacheGroup; rtr_expire.go -- prepareQuery
package rpki

import (
	"encoding/binary"
	"io"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readRTRQuery reads one Reset Query or Serial Query from the router and returns it whole.
func readRTRQuery(conn net.Conn) ([]byte, error) {
	query := make([]byte, pduSerialQueryLen)
	if _, err := io.ReadFull(conn, query[:pduHeaderLen]); err != nil {
		return nil, err
	}
	if query[1] != pduSerialQuery {
		return query[:pduHeaderLen], nil
	}
	if _, err := io.ReadFull(conn, query[pduHeaderLen:]); err != nil {
		return nil, err
	}
	return query, nil
}

// ipv4AnnouncePDU is an IPv4 Prefix PDU announcing prefix/24, with Max Length 24.
func ipv4AnnouncePDU(prefix [4]byte, asn uint32) []byte {
	pdu := []byte{rtrVersionMax, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0}
	pdu = append(pdu, prefix[:]...)
	return binary.BigEndian.AppendUint32(pdu, asn)
}

// endOfDataTimed is an End of Data PDU carrying serial and the Refresh and Retry Intervals, in seconds.
func endOfDataTimed(serial, refresh, retry uint32) []byte {
	pdu := endOfDataPDU()
	binary.BigEndian.PutUint32(pdu[8:12], serial)
	binary.BigEndian.PutUint32(pdu[12:16], refresh)
	binary.BigEndian.PutUint32(pdu[16:20], retry)
	return pdu
}

// cacheResetPDU is a Cache Reset whose reserved octets 2-3 carry reserved.
func cacheResetPDU(reserved uint16) []byte {
	pdu := []byte{rtrVersionMax, pduCacheReset, 0, 0, 0, 0, 0, pduHeaderLen}
	binary.BigEndian.PutUint16(pdu[2:4], reserved)
	return pdu
}

// scriptRTRCache answers the connections the router opens in order, one reply each, and reports
// the type of the query that opened each connection. A connection past the script is closed
// unanswered. After a reply the cache waits for the router to close, so every connection carries
// exactly one query.
func scriptRTRCache(t *testing.T, replies ...[]byte) (uint16, <-chan byte) {
	t.Helper()
	queries := make(chan byte, len(replies)+8)
	attempt := 0
	port, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return
		}
		query, err := readRTRQuery(conn)
		if err != nil {
			return
		}
		select {
		case queries <- query[1]:
		default:
		}
		if attempt >= len(replies) {
			return
		}
		reply := replies[attempt]
		attempt++
		if _, err := conn.Write(reply); err != nil {
			return
		}
		_, _ = conn.Read(query[:1])
	})
	return port, queries
}

// takeQueries collects the types of the next count queries, or fails the test.
func takeQueries(t *testing.T, queries <-chan byte, count int) []byte {
	t.Helper()
	got := make([]byte, 0, count)
	for range count {
		select {
		case query := <-queries:
			got = append(got, query)
		case <-time.After(6 * time.Second):
			t.Fatalf("the router sent %d queries, want %d: % x", len(got), count, got)
		}
	}
	return got
}

// startCacheGroup runs a cache group holding one session to the cache on port, so no cache is
// more preferred than that one. The group stops when the test ends.
func startCacheGroup(t *testing.T, port uint16, roas *ROACache) {
	t.Helper()
	stop := make(chan struct{})
	s := newTestRTRSession(t, "127.0.0.1", port, 100, "", roas, newASPACache(), stop)
	done := make(chan struct{})
	go func() { defer close(done); newCacheGroup([]*RTRSession{s}, stop).Run() }()
	t.Cleanup(func() {
		close(stop)
		s.close()
		<-done
	})
}

// setDataDeadline gives the session's published data a lease that ends at deadline, current for
// the session's generation until then.
func setDataDeadline(s *RTRSession, deadline time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lease := s.dataLeaseLocked()
	lease.mu.Lock()
	defer lease.mu.Unlock()
	lease.renewLocked(deadline, 0)
	s.dataGeneration = lease.generation
}

// TestRTRFatalErrorReportDropsTheSession drives each fatal Error Code over TCP and watches the
// cache's side of the connection.
//
// VALIDATES: RFC 8210 Section 12, "Errors which are considered fatal MUST cause the session to
// be dropped": the router fails the sync and closes the transport after a fatal Error Report,
// and a load the cache sends after the report on the same connection is never published.
// PREVENTS: a router that logs a fatal error and keeps reading, as it does for a non-fatal one.
func TestRTRFatalErrorReportDropsTheSession(t *testing.T) {
	// Code 2 is the one code Section 12 does not mark fatal. Code 4 from a cache that advertises
	// a lower version is a downgrade, which TestRTRSessionStartsAtV2 drives.
	fatal := []uint16{0, 1, 3, 5, 6, 7, 8}

	t.Run("the transport is closed", func(t *testing.T) {
		// RFC requirement: RFC8210-12-1 positive -- after each fatal Error Code 0, 1, 3, 5, 6, 7 and 8 the router fails the sync and closes the TCP connection, which the cache reads as EOF instead of waiting for more.
		for _, code := range fatal {
			closed := make(chan error, 1)
			port, _ := serveRTR(t, func(conn net.Conn) {
				if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
					closed <- err
					return
				}
				if _, err := readRTRQuery(conn); err != nil {
					closed <- err
					return
				}
				report := make([]byte, 16)
				n := writeErrorReport(report, rtrVersionMax, code, nil)
				if _, err := conn.Write(report[:n]); err != nil {
					closed <- err
					return
				}
				var next [1]byte
				_, err := conn.Read(next[:])
				closed <- err
			})
			s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
			require.Error(t, s.syncOnce(), "fatal code %d did not fail the sync", code)
			require.ErrorIs(t, <-closed, io.EOF, "fatal code %d left the session open", code)
		}
	})

	t.Run("a load sent after the fatal report is not published", func(t *testing.T) {
		// RFC requirement: RFC8210-12-1 negative -- a cache that follows a fatal Corrupt Data report with a complete load on the same connection gets nothing published: the sync fails, the VRP stays unknown and the session is not marked synced.
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			report := make([]byte, 16)
			n := writeErrorReport(report, rtrVersionMax, 0, nil)
			load := slices.Concat(report[:n], cacheResponsePDU(),
				ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64500), endOfDataPDU())
			if _, err := conn.Write(load); err != nil {
				return
			}
			_, _ = conn.Read(report[:1])
		})
		roas := newROACache()
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", roas, newASPACache(), make(chan struct{}))
		require.Error(t, s.syncOnce())
		assert.Equal(t, ValidationNotFound, roas.Validate("192.0.2.0/24", 64500), "the load after a fatal error was published")
		assert.False(t, s.Snapshot().Synced)
	})
}

// TestRTRUnknownVersionErrorReportIsNotAnswered sends the router an Error Report whose Protocol
// Version it does not recognize, over TCP.
//
// VALIDATES: RFC 8210 Section 7, the exception "unless the received PDU is itself an Error Report
// PDU": the router terminates the connection and sends no Error Report back.
// PREVENTS: two parties answering each other's Error Reports.
func TestRTRUnknownVersionErrorReportIsNotAnswered(t *testing.T) {
	// RFC requirement: RFC8210-7-7 negative -- an Error Report carrying unrecognized version 99, with Error Code 0 or 4, draws no Error Report back: the router ends the sync and closes the connection, and the cache reads EOF with no byte before it.
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-7-2 negative -- the same unrecognized-version Error Report is answered with no Error Report of Error Code 4 or any other, and the connection is terminated.
	for _, code := range []uint16{0, errUnsupportedVersion} {
		answered := make(chan error, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				answered <- err
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				answered <- err
				return
			}
			report := make([]byte, 16)
			n := writeErrorReport(report, 99, code, nil)
			if _, err := conn.Write(report[:n]); err != nil {
				answered <- err
				return
			}
			var answer [1]byte
			_, err := io.ReadFull(conn, answer[:])
			answered <- err
		})
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.Error(t, s.syncOnce(), "code %d", code)
		require.ErrorIs(t, <-answered, io.EOF, "an unknown-version Error Report with code %d was answered", code)
	}
}

// TestRTRStartupSerialNotifyOfAnyVersionIsIgnored feeds readLoop a Serial Notify of a version the
// router does not speak, before the Cache Response, over an in-memory connection.
//
// VALIDATES: RFC 8210 Section 5.2, the router "MUST simply ignore the Serial Notify PDU, even if
// the Serial Notify PDU is for an unexpected protocol version".
// PREVENTS: the version check of Section 7 reaching the Serial Notify it exempts.
func TestRTRStartupSerialNotifyOfAnyVersionIsIgnored(t *testing.T) {
	exchange := func(t *testing.T, script func(cache net.Conn) error) (*RTRSession, net.Conn, <-chan error) {
		t.Helper()
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
			observed <- script(cache)
		}()
		return newTestRTRSession(t, "cache.invalid", 323, 100, "", newROACache(), newASPACache(), make(chan struct{})), client, observed
	}

	t.Run("a Serial Notify of unexpected version 99 is ignored", func(t *testing.T) {
		// RFC requirement: RFC8210-5.2-1 positive -- a Serial Notify of unexpected version 99, received before the Cache Response, is ignored: no Error Report goes back, neither its Session ID nor its serial is adopted, and the sync the Cache Response starts completes with the End of Data serial.
		s, client, observed := exchange(t, func(cache net.Conn) error {
			notify := make([]byte, 12)
			notify[0], notify[1] = 99, pduSerialNotify
			binary.BigEndian.PutUint16(notify[2:4], 0xBEEF)
			binary.BigEndian.PutUint32(notify[4:8], 12)
			binary.BigEndian.PutUint32(notify[8:12], 999)
			response := cacheResponsePDU()
			binary.BigEndian.PutUint16(response[2:4], 7)
			for _, pdu := range [][]byte{notify, response, endOfDataTimed(5, 3600, 600)} {
				if _, err := cache.Write(pdu); err != nil {
					return err
				}
			}
			var answer [1]byte
			_, err := io.ReadFull(cache, answer[:])
			return err
		})
		require.NoError(t, s.readLoop(client))
		require.NoError(t, client.Close())
		require.ErrorIs(t, <-observed, io.EOF, "the router answered the Serial Notify")
		snap := s.Snapshot()
		assert.Equal(t, uint32(5), snap.Serial, "the serial is the End of Data's, not the notify's")
		assert.Equal(t, uint16(7), snap.SessionID, "the Session ID is the Cache Response's, not the notify's")
	})

	t.Run("a Cache Response of unexpected version 99 is refused", func(t *testing.T) {
		// RFC requirement: RFC8210-5.2-1 negative -- the version exemption belongs to the Serial Notify alone: a Cache Response of version 99 in the same startup window fails with Unsupported Protocol Version and draws an Error Report with Error Code 4.
		reported := make(chan []byte, 1)
		s, client, observed := exchange(t, func(cache net.Conn) error {
			response := cacheResponsePDU()
			response[0] = 99
			if _, err := cache.Write(response); err != nil {
				return err
			}
			report := make([]byte, 16+len(response))
			_, err := io.ReadFull(cache, report)
			reported <- report
			return err
		})
		require.ErrorIs(t, s.readLoop(client), errRtrUnsupportedProtocol)
		require.NoError(t, <-observed)
		report := <-reported
		assert.Equal(t, pduErrorRpt, report[1])
		assert.Equal(t, errUnsupportedVersion, binary.BigEndian.Uint16(report[2:4]))
	})
}

// TestRTRCacheResetReloadsTheWholeSet runs a cache group with one configured cache, so no cache is
// more preferred, against a cache that answers the router's Serial Query with Cache Reset.
//
// VALIDATES: RFC 8210 Section 8.3, "If there are no more-preferred caches, it MUST issue a Reset
// Query and get an entire new load from the cache": the next query on the wire is a Reset Query,
// and its load replaces the set the router held.
// PREVENTS: a Cache Reset that leaves the router on Serial Queries, or a reload merged into the
// set whose base the cache no longer holds.
func TestRTRCacheResetReloadsTheWholeSet(t *testing.T) {
	first := slices.Concat(cacheResponsePDU(), ipv4AnnouncePDU([4]byte{10, 0, 0, 0}, 64500), endOfDataTimed(5, 1, 1))
	later := slices.Concat(cacheResponsePDU(), ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64501), endOfDataTimed(9, 1, 1))

	t.Run("cache reset with no more-preferred cache", func(t *testing.T) {
		// RFC requirement: RFC8210-8.3-1 positive -- with one configured cache, so none is more preferred, a Cache Reset in answer to the Serial Query makes the next query on the wire a Reset Query, and its load replaces the old set: the earlier VRP stops validating and the new one validates.
		// RFC requirement: RFC6810-6.3-1 positive -- the same exchange: after the Cache Reset the router issues a Reset Query and takes an entire new load in place of the old set.
		port, queries := scriptRTRCache(t, first, cacheResetPDU(0), later)
		roas := newROACache()
		startCacheGroup(t, port, roas)
		assert.Equal(t, []byte{pduResetQuery, pduSerialQuery, pduResetQuery}, takeQueries(t, queries, 3))
		require.Eventually(t, func() bool {
			return roas.Validate("192.0.2.0/24", 64501) == ValidationValid
		}, 6*time.Second, time.Millisecond, "the load after the Cache Reset was not published")
		assert.Equal(t, ValidationNotFound, roas.Validate("10.0.0.0/24", 64500), "the old set survived the Cache Reset reload")
	})

	t.Run("a serial query answered with data", func(t *testing.T) {
		// RFC requirement: RFC8210-8.3-1 negative -- when the cache answers the Serial Query with data instead of Cache Reset, the router keeps querying by serial and merges the delta into the set it holds, so the Reset Query and the replacement follow only a Cache Reset.
		// RFC requirement: RFC6810-6.3-1 negative -- likewise, without a Cache Reset no Reset Query is issued and the held set is kept.
		port, queries := scriptRTRCache(t, first, later)
		roas := newROACache()
		startCacheGroup(t, port, roas)
		assert.Equal(t, []byte{pduResetQuery, pduSerialQuery, pduSerialQuery}, takeQueries(t, queries, 3))
		assert.Equal(t, ValidationValid, roas.Validate("192.0.2.0/24", 64501))
		assert.Equal(t, ValidationValid, roas.Validate("10.0.0.0/24", 64500), "a delta replaced the held set")
	})
}

// TestRTRResetQueryWithoutFastResyncData records the first query of a connection in each case
// where the router lacks the data for fast resynchronization, and in the one case where it has it.
//
// VALIDATES: RFC 8210 Section 8.1, "In all other cases, the router lacks the necessary data for
// fast resynchronization and therefore MUST fall back to a Reset Query": no remembered session,
// expired data, a switch to a different cache, and a version downgrade each open with a Reset Query.
// PREVENTS: a Serial Query carrying a serial the cache on the other end cannot relate to.
func TestRTRResetQueryWithoutFastResyncData(t *testing.T) {
	// recordQueries answers every query with an empty load and reports each query whole.
	recordQueries := func(t *testing.T) (uint16, <-chan []byte) {
		t.Helper()
		queries := make(chan []byte, 8)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			query, err := readRTRQuery(conn)
			if err != nil {
				return
			}
			queries <- query
			response, end := cacheResponsePDU(), endOfDataPDU()
			response[0], end[0] = query[0], query[0]
			if _, err := conn.Write(slices.Concat(response, end)); err != nil {
				return
			}
			_, _ = conn.Read(query[:1])
		})
		return port, queries
	}
	heldSession := func(t *testing.T, port uint16) *RTRSession {
		t.Helper()
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		s.serial, s.sessionID = 42, 7
		return s
	}

	t.Run("no remembered session", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 positive -- a router that remembers no Session ID or serial for the cache opens with a Reset Query.
		port, queries := recordQueries(t)
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		require.NoError(t, s.syncOnce())
		assert.Equal(t, pduResetQuery, (<-queries)[1])
	})

	t.Run("expired data", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 positive -- a router that remembers serial 42 but whose data expired opens with a Reset Query, not a Serial Query.
		port, queries := recordQueries(t)
		s := heldSession(t, port)
		setDataDeadline(s, time.Now().Add(-time.Minute))
		require.NoError(t, s.syncOnce())
		assert.Equal(t, pduResetQuery, (<-queries)[1])
	})

	t.Run("a different cache", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 positive -- when the preferred cache is down and the router moves its load to another cache, that cache is asked with a Reset Query even though its session still holds serial 42 over unexpired data.
		port, queries := recordQueries(t)
		stop := make(chan struct{})
		dead := newTestRTRSession(t, "127.0.0.1", refusedPort(t), 10, "", newROACache(), newASPACache(), stop)
		standby := heldSession(t, port)
		group := newCacheGroup([]*RTRSession{dead, standby}, stop)
		setDataDeadline(standby, time.Now().Add(time.Hour))
		group.holder = dead
		group.poll()
		assert.Equal(t, pduResetQuery, (<-queries)[1])
	})

	t.Run("a version downgrade", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 positive -- a cache that answers the version 2 Serial Query with Unsupported Protocol Version at version 1 is asked again with a version 1 Reset Query, because the serial belongs to the abandoned version.
		queries := make(chan []byte, 4)
		attempt := 0
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			query, err := readRTRQuery(conn)
			if err != nil {
				return
			}
			queries <- query
			attempt++
			reply := slices.Concat(cacheResponsePDU(), endOfDataPDU())
			reply[0], reply[pduHeaderLen] = 1, 1
			if attempt == 1 {
				reply = make([]byte, 16)
				reply = reply[:writeErrorReport(reply, 1, errUnsupportedVersion, nil)]
			}
			if _, err := conn.Write(reply); err != nil {
				return
			}
			_, _ = conn.Read(query[:1])
		})
		s := heldSession(t, port)
		setDataDeadline(s, time.Now().Add(time.Hour))
		require.NoError(t, s.syncOnce())
		assert.Equal(t, []byte{rtrVersionMax, pduSerialQuery}, (<-queries)[:2])
		assert.Equal(t, []byte{1, pduResetQuery}, (<-queries)[:2])
	})

	t.Run("unexpired data from the same cache", func(t *testing.T) {
		// RFC requirement: RFC8210-8.1-3 negative -- a router that holds unexpired data and the Session ID from the same cache opens with a Serial Query carrying that Session ID and serial, so the fallback applies only where the data is lacking.
		port, queries := recordQueries(t)
		stop := make(chan struct{})
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), stop)
		s.serial, s.sessionID = 42, 7
		group := newCacheGroup([]*RTRSession{s}, stop)
		setDataDeadline(s, time.Now().Add(time.Hour))
		group.holder = s
		group.poll()
		query := <-queries
		require.Equal(t, pduSerialQuery, query[1])
		assert.Equal(t, uint16(7), binary.BigEndian.Uint16(query[2:4]), "Session ID")
		assert.Equal(t, uint32(42), binary.BigEndian.Uint32(query[8:12]), "serial")
	})
}

// TestRTRReservedOctetsIgnoredOnReceipt sends the router PDUs whose reserved fields hold ones.
//
// VALIDATES: RFC 8210 Section 5, reserved fields "MUST be ignored on receipt", for the Cache Reset
// PDU, whose octets 2-3 are reserved, and for the IPv6 Prefix PDU.
// PREVENTS: a receiver that refuses or misreads a PDU because a reserved field is not zero.
func TestRTRReservedOctetsIgnoredOnReceipt(t *testing.T) {
	t.Run("a Cache Reset with reserved octets set", func(t *testing.T) {
		// RFC requirement: RFC8210-5-1 positive -- a Cache Reset whose reserved octets 2-3 are 0xFFFF is still a Cache Reset: the sync ends with the reset and the serial is cleared for a Reset Query.
		// RFC requirement: RFC6810-5.1-3 positive -- the same Cache Reset, whose field of unspecified content is all ones, is acted on as a Cache Reset.
		port, queries := scriptRTRCache(t, cacheResetPDU(0xFFFF))
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		s.serial = 42
		require.ErrorIs(t, s.syncOnce(), errRtrCacheResetReceivedWillDo)
		assert.Equal(t, pduSerialQuery, <-queries)
		assert.Equal(t, uint32(0), s.Snapshot().Serial)
	})

	t.Run("an IPv6 Prefix with reserved fields set", func(t *testing.T) {
		// RFC requirement: RFC6810-5.1-3 positive -- an IPv6 Prefix PDU whose header zero field, reserved flag bits and zero octet are all ones still authorizes its route.
		roas := newROACache()
		s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", roas, newASPACache(), make(chan struct{}))
		prefix := []byte{rtrVersionMax, pduIPv6Prefix, 0xFF, 0xFF, 0, 0, 0, 32, 0xFF, 32, 48, 0xFF,
			0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xfb, 0xf4}
		applyPDU(t, s, cacheResponsePDU(), false)
		applyPDU(t, s, prefix, false)
		applyPDU(t, s, endOfDataPDU(), true)
		assert.Equal(t, ValidationValid, roas.Validate("2001:db8::/32", 64500))
	})

	t.Run("octets 2-3 of a Cache Response are its Session ID", func(t *testing.T) {
		// RFC requirement: RFC8210-5-1 negative -- only reserved fields are ignored: the same 0xFFFF in octets 2-3 of a Cache Response is its Session ID, and the router adopts it.
		// RFC requirement: RFC6810-5.1-3 negative -- a field of specified content in the same position is read, not ignored.
		s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		response := cacheResponsePDU()
		binary.BigEndian.PutUint16(response[2:4], 0xFFFF)
		applyPDU(t, s, response, false)
		assert.Equal(t, uint16(0xFFFF), s.Snapshot().SessionID)
	})
}

// TestRTRPollsAtLeastHourly checks the poll cadence a router keeps when its cache sends no timing.
//
// VALIDATES: RFC 6810 Section 6.1, "a router MUST send either a Serial Query or a Reset Query no
// less frequently than once an hour": an End of Data with no intervals leaves both waits at or
// under one hour, and the group does query again once the wait ends.
// PREVENTS: a poll loop that stops after the first sync, or a default wait above one hour.
func TestRTRPollsAtLeastHourly(t *testing.T) {
	t.Run("an End of Data without intervals keeps the hourly wait", func(t *testing.T) {
		// RFC requirement: RFC6810-6.1-1 positive -- after an End of Data whose interval fields are zero, as a version 0 cache sends none, the wait after a sync and the wait after a failed query are both at most one hour.
		s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", newROACache(), newASPACache(), make(chan struct{}))
		applyPDU(t, s, cacheResponsePDU(), false)
		applyPDU(t, s, endOfDataTimed(5, 0, 0), true)
		assert.LessOrEqual(t, s.pollDelay(true), time.Hour)
		assert.LessOrEqual(t, s.pollDelay(false), time.Hour)
	})

	t.Run("the group queries again when the wait ends", func(t *testing.T) {
		// RFC requirement: RFC6810-6.1-1 positive -- the running cache group sends its next query when the wait after a sync ends: with a one-second wait the cache sees a Reset Query and then a Serial Query.
		sync := slices.Concat(cacheResponsePDU(), endOfDataTimed(5, 1, 1))
		port, queries := scriptRTRCache(t, sync, sync)
		startCacheGroup(t, port, newROACache())
		assert.Equal(t, []byte{pduResetQuery, pduSerialQuery}, takeQueries(t, queries, 2))
	})
}

// TestRTRValidationIgnoresWhichCacheSupplied loads one VRP from the preferred cache in one run and
// from the standby cache in another, through the plugin's own session start.
//
// VALIDATES: RFC 8210 Section 10, "implementations MUST NOT distinguish between data sources when
// performing validation of BGP announcements": the same VRP gives the same validation states
// whichever configured cache supplied it.
// PREVENTS: validation that weights or marks data by the preference of the cache it came from.
func TestRTRValidationIgnoresWhichCacheSupplied(t *testing.T) {
	serveVRP := func(t *testing.T) uint16 {
		t.Helper()
		port, _ := scriptRTRCache(t, slices.Concat(cacheResponsePDU(),
			ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64500), endOfDataPDU()))
		return port
	}
	states := func(t *testing.T, preferred, standby uint16) [3]uint8 {
		t.Helper()
		rp := newStatePlugin(t)
		rp.startSessions(&rpkiConfig{CacheServers: []cacheServerConfig{
			{Address: "127.0.0.1", Port: preferred, Preference: 10},
			{Address: "127.0.0.1", Port: standby, Preference: 200},
		}})
		requireSynced(t, rp, 1)
		return [3]uint8{
			rp.cache.Validate("192.0.2.0/24", 64500),
			rp.cache.Validate("192.0.2.0/24", 64501),
			rp.cache.Validate("198.51.100.0/24", 64500),
		}
	}
	fromPreferred := states(t, serveVRP(t), refusedPort(t))
	fromStandby := states(t, refusedPort(t), serveVRP(t))

	// RFC requirement: RFC8210-10-1 positive -- the VRP learned from the standby cache validates the matching route Valid, exactly as the same VRP learned from the preferred cache does.
	assert.Equal(t, ValidationValid, fromPreferred[0])
	assert.Equal(t, fromPreferred[0], fromStandby[0])
	// RFC requirement: RFC8210-10-1 negative -- a route the VRP does not authorize is Invalid, and an uncovered route NotFound, with either cache as the source, so no source makes a route more or less valid.
	assert.Equal(t, ValidationInvalid, fromPreferred[1])
	assert.Equal(t, ValidationNotFound, fromPreferred[2])
	assert.Equal(t, fromPreferred, fromStandby)
}

// TestASPAEmptyAnnouncementDrawsErrorReport9 sends the router ASPA announcements over TCP.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 5.12, "For an announcement, the PDU MUST contain at
// least one Provider Autonomous System Number. If it does not, an Error Report PDU with Error Code
// 9 ("ASPA Provider List Error") MUST be returned by the router."
// PREVENTS: an empty announcement accepted as an authorization, or refused without the report.
func TestASPAEmptyAnnouncementDrawsErrorReport9(t *testing.T) {
	t.Run("an announcement with a provider is published", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-1 positive -- an announcement carrying one Provider AS completes the sync with no Error Report sent, and the provider becomes attested for its customer.
		answered := make(chan error, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				answered <- err
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				answered <- err
				return
			}
			if _, err := conn.Write(slices.Concat(cacheResponsePDU(), aspaPDU(64500, 100), endOfDataPDU())); err != nil {
				answered <- err
				return
			}
			var answer [1]byte
			_, err := io.ReadFull(conn, answer[:])
			answered <- err
		})
		aspas := newASPACache()
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), aspas, make(chan struct{}))
		require.NoError(t, s.syncOnce())
		require.ErrorIs(t, <-answered, io.EOF, "the router sent a PDU after a valid announcement")
		assert.Equal(t, HopProviderPlus, aspas.checkPair(100, 64500))
	})

	t.Run("an announcement without providers draws Error Code 9", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-1 negative -- an announcement with no Provider AS draws an Error Report PDU with Error Code 9 that encapsulates it, and no authorization for its customer is published.
		bad := aspaPDU(64500)
		reported := make(chan []byte, 1)
		port, _ := serveRTR(t, func(conn net.Conn) {
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				return
			}
			if _, err := readRTRQuery(conn); err != nil {
				return
			}
			if _, err := conn.Write(slices.Concat(cacheResponsePDU(), bad)); err != nil {
				return
			}
			report := make([]byte, 16+len(bad))
			if _, err := io.ReadFull(conn, report); err != nil {
				return
			}
			reported <- report
		})
		aspas := newASPACache()
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), aspas, make(chan struct{}))
		require.ErrorIs(t, s.syncOnce(), errASPAProviderList)
		var report []byte
		select {
		case report = <-reported:
		case <-time.After(3 * time.Second):
			t.Fatal("the router returned no Error Report for an announcement without providers")
		}
		assert.Equal(t, pduErrorRpt, report[1])
		assert.Equal(t, uint16(9), binary.BigEndian.Uint16(report[2:4]))
		assert.Equal(t, bad, report[12:12+len(bad)])
		assert.Equal(t, HopNoAttestation, aspas.checkPair(100, 64500))
	})
}

// TestASPAProviderListOfZeroOrMoreUniqueAscending drives ASPA PDUs through the session's handler.
//
// VALIDATES: draft-ietf-sidrops-8210bis Section 5.12, "There are zero or more 32-bit Provider
// Autonomous System Number fields in increasing numeric order. Each Provider Autonomous System
// Number in a given ASPA PDU MUST be unique."
// PREVENTS: a withdrawal refused for carrying no provider, or a repeated provider accepted.
func TestASPAProviderListOfZeroOrMoreUniqueAscending(t *testing.T) {
	t.Run("zero providers on a withdrawal, unique ascending on an announcement", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-3 positive -- a withdrawal with zero Provider AS fields is accepted and removes its customer's record, and an announcement of unique providers in increasing order is accepted whole.
		aspas := newASPACache()
		aspas.Set(64500, []uint32{100})
		s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", newROACache(), aspas, make(chan struct{}))
		withdraw := aspaPDU(64500)
		withdraw[2] = 0
		applyPDU(t, s, cacheResponsePDU(), false)
		applyPDU(t, s, withdraw, false)
		applyPDU(t, s, aspaPDU(64501, 100, 200, 300), false)
		applyPDU(t, s, endOfDataPDU(), true)
		assert.Equal(t, HopNoAttestation, aspas.checkPair(100, 64500), "the withdrawal was not applied")
		assert.Equal(t, HopProviderPlus, aspas.checkPair(100, 64501))
		assert.Equal(t, HopProviderPlus, aspas.checkPair(300, 64501))
	})

	t.Run("a repeated or descending provider list is refused", func(t *testing.T) {
		// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-3 negative -- an announcement that repeats a provider, or lists providers in decreasing order, fails with the provider-list error and publishes nothing for its customer.
		for _, bad := range [][]byte{aspaPDU(64501, 100, 100), aspaPDU(64501, 200, 100)} {
			aspas := newASPACache()
			s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", newROACache(), aspas, make(chan struct{}))
			applyPDU(t, s, cacheResponsePDU(), false)
			hdr, err := parseHeader(bad[:pduHeaderLen])
			require.NoError(t, err)
			_, err = s.handlePDU(hdr, bad)
			require.ErrorIs(t, err, errASPAProviderList)
			applyPDU(t, s, endOfDataPDU(), true)
			assert.Equal(t, HopNoAttestation, aspas.checkPair(100, 64501))
		}
	})
}

// TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess reads the wait the cache group schedules
// after a failed query, before and after the cache has answered once.
//
// VALIDATES: RFC 8210 Section 6, "If the router has never issued a successful query against a
// particular cache, it SHOULD retry periodically using the default Retry Interval, above", whose
// recommended default is 600 seconds.
// PREVENTS: a cache that never answered being retried on some other schedule, or the default
// overriding the Retry Interval a cache has sent.
func TestRTRRetriesAtTheDefaultRetryIntervalBeforeAnySuccess(t *testing.T) {
	t.Run("a cache never queried successfully", func(t *testing.T) {
		// RFC requirement: RFC8210-6-6 positive -- a cache that has refused every connection is tried again after the default Retry Interval, 600 seconds.
		stop := make(chan struct{})
		s := newTestRTRSession(t, "127.0.0.1", refusedPort(t), 100, "", newROACache(), newASPACache(), stop)
		assert.Equal(t, 600*time.Second, newCacheGroup([]*RTRSession{s}, stop).poll())
	})

	t.Run("a cache that sent its own Retry Interval", func(t *testing.T) {
		// RFC requirement: RFC8210-6-6 negative -- once a query succeeded and its End of Data set a Retry Interval of 30 seconds, a later failed query waits those 30 seconds, not the default, so the default holds only before the first success.
		port, _ := scriptRTRCache(t, slices.Concat(cacheResponsePDU(), endOfDataTimed(5, 3600, 30)))
		stop := make(chan struct{})
		s := newTestRTRSession(t, "127.0.0.1", port, 100, "", newROACache(), newASPACache(), stop)
		group := newCacheGroup([]*RTRSession{s}, stop)
		require.Equal(t, time.Hour, group.poll(), "the first query succeeds and waits the Refresh Interval")
		assert.Equal(t, 30*time.Second, group.poll(), "the second query fails and waits the cache's Retry Interval")
	})
}
