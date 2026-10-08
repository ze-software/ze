// RFC: rfc/short/rfc8210.md — Section 10, the router connects to its caches in preference order
// Related: rtr_session.go — cacheGroup, startFullSync; rpki.go — startSessions
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

// TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers drives the `preference` leaf of
// `bgp rpki cache-server` from the parsed config, the way OnConfigure does.
//
// VALIDATES: RFC 8210 Section 10 -- "The client router attempts to establish a session with
// each potential serving cache in preference order and then starts to load data from the most
// preferred cache to which it can connect and authenticate", with "the lower the value, the
// more preferred". The same section forbids the alternative reading of the leaf: "If data
// from multiple caches are held, implementations MUST NOT distinguish between data sources
// when performing validation of BGP announcements", so preference orders the CONNECTION and
// never the belief.
// PREVENTS: the leaf going back to what it was until 2026-09-20, a value parsed, stored on
// the session, printed by `show bgp rpki status` and read by no comparison, sort or failover,
// under which an operator who ordered three caches got a session to all three at once and the
// union of their records.
func TestCacheGroupLoadsFromTheMostPreferredCacheThatAnswers(t *testing.T) {
	// RFC requirement: RFC8210-4-1 positive -- the lowest configured preference that answers supplies the payload set.
	// RFC requirement: RFC8210-4-1 negative -- a less-preferred cache is not contacted while the preferred cache answers.
	t.Run("the less preferred cache is not contacted while the preferred one answers", func(t *testing.T) {
		standbyPort, standbyAccepts := serveRTR(t, replyEmptySync(t))
		preferredPort, preferredAccepts := serveRTR(t, replyEmptySync(t))

		// The standby is written FIRST, so a pass that reads config order rather than
		// preference contacts the wrong cache.
		rp := newStatePlugin(t)
		rp.startSessions(&rpkiConfig{CacheServers: []cacheServerConfig{
			{Address: "127.0.0.1", Port: standbyPort, Preference: 200},
			{Address: "127.0.0.1", Port: preferredPort, Preference: 10},
		}})

		waitAccept(t, preferredAccepts, "the most preferred cache server was never contacted")
		requireSynced(t, rp, 1)

		select {
		case <-standbyAccepts:
			t.Fatal("the standby cache was contacted while the preferred one was answering: " +
				"every configured server is being read, and their records merge into one set")
		case <-time.After(500 * time.Millisecond):
		}
	})

	t.Run("the next cache in preference order takes over when the preferred one is down", func(t *testing.T) {
		deadPort := refusedPort(t)
		standbyPort, standbyAccepts := serveRTR(t, replyEmptySync(t))

		rp := newStatePlugin(t)
		rp.startSessions(&rpkiConfig{CacheServers: []cacheServerConfig{
			{Address: "127.0.0.1", Port: deadPort, Preference: 10},
			{Address: "127.0.0.1", Port: standbyPort, Preference: 200},
		}})

		waitAccept(t, standbyAccepts,
			"the standby cache was never contacted after the preferred one refused the connection")
		requireSynced(t, rp, 1)

		snaps := rp.snapshots()
		require.Len(t, snaps, 2)
		assert.EqualValues(t, 10, snaps[0].Preference, "the sessions are held most preferred first")
		assert.False(t, snaps[0].Synced, "the preferred cache refused every connection")
		assert.True(t, snaps[1].Synced, "the next one in preference order carried the load")
	})

	t.Run("a full sync replaces the set the previous cache left", func(t *testing.T) {
		cache := newROACache()
		cache.Add(makeVRP("10.0.0.0/8", 24, 65001))

		s := newTestRTRSession(t, "127.0.0.1", 3323, 100, "", cache, newASPACache(), make(chan struct{}))
		s.startFullSync()
		applyPDU(t, s, cacheResponsePDU(), false)
		applyPDU(t, s, endOfDataPDU(), true)

		v4, v6 := cache.Count()
		assert.Equal(t, 0, v4+v6,
			"the set of the cache ze stopped reading survived a full sync from the cache that replaced it, "+
				"so a VRP no cache server would serve still covers a prefix and nothing can withdraw it")
	})
}

// applyPDU feeds one PDU through the session's own handler and states whether it ends the
// sync, so the test drives the producer rather than a copy of it.
func applyPDU(t *testing.T, s *RTRSession, pdu []byte, wantDone bool) {
	t.Helper()
	hdr, err := parseHeader(pdu[:pduHeaderLen])
	require.NoError(t, err)

	done, err := s.handlePDU(hdr, pdu)
	require.NoError(t, err)
	assert.Equal(t, wantDone, done)
}

// cacheResponsePDU is a Cache Response carrying session id zero.
func cacheResponsePDU() []byte {
	pdu := make([]byte, pduHeaderLen)
	pdu[0], pdu[1] = rtrVersionMax, pduCacheResp
	binary.BigEndian.PutUint32(pdu[4:8], pduHeaderLen)
	return pdu
}

// endOfDataPDU is an End of Data carrying the three default intervals and no prefix before it.
func endOfDataPDU() []byte {
	pdu := make([]byte, pduEndOfDataLen)
	pdu[0], pdu[1] = rtrVersionMax, pduEndOfData
	binary.BigEndian.PutUint32(pdu[4:8], pduEndOfDataLen)
	binary.BigEndian.PutUint32(pdu[12:16], 3600)
	binary.BigEndian.PutUint32(pdu[16:20], 600)
	binary.BigEndian.PutUint32(pdu[20:24], 7200)
	return pdu
}

func TestRTRCacheSwitchKeepsSerialBasesSeparate(t *testing.T) {
	// RFC requirement: RFC8210-10-2 positive -- each cache switch replaces the published VRPs and ASPAs with the selected cache's complete set.
	// RFC requirement: RFC8210-10-2 negative -- an old cache's matching serial cannot reuse the other cache's base, merge its prefixes, or preserve its provider authorization.
	queries := make(chan [2]uint8, 8)
	failures := make(chan error, 8)
	reply := func(octet byte, provider uint32, failSecond bool) func(net.Conn) {
		calls := 0
		return func(conn net.Conn) {
			calls++
			if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				failures <- err
				return
			}
			var query [8]byte
			if _, err := io.ReadFull(conn, query[:]); err != nil {
				failures <- err
				return
			}
			if query[1] == pduSerialQuery {
				var serial [4]byte
				if _, err := io.ReadFull(conn, serial[:]); err != nil {
					failures <- err
					return
				}
			}
			queries <- [2]uint8{octet, query[1]}
			if failSecond && calls == 2 {
				return
			}
			response, end := cacheResponsePDU(), endOfDataPDU()
			response[0], end[0] = 2, 2
			// Both caches deliberately use the same session ID and serial.
			binary.BigEndian.PutUint32(end[8:12], 1)
			prefix := []byte{2, pduIPv4Prefix, 0, 0, 0, 0, 0, 20, 1, 24, 24, 0, 192, 0, octet, 0, 0, 0, 0xfb, 0xf4}
			for _, pdu := range [][]byte{response, prefix, aspaPDU(64500, provider), end} {
				if _, err := conn.Write(pdu); err != nil {
					failures <- err
					return
				}
			}
		}
	}
	first, _ := serveRTR(t, reply(2, 64501, true))
	second, _ := serveRTR(t, reply(3, 64502, false))
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	group := newCacheGroup([]*RTRSession{
		newTestRTRSession(t, "127.0.0.1", first, 10, "", roas, aspas, stop),
		newTestRTRSession(t, "127.0.0.1", second, 20, "", roas, aspas, stop),
	}, stop)
	for _, selected := range []int{2, 3, 2} {
		group.poll()
		for _, candidate := range []int{2, 3} {
			prefix := "192.0.2.0/24"
			provider := uint32(64501)
			if candidate == 3 {
				prefix, provider = "192.0.3.0/24", 64502
			}
			want := ValidationNotFound
			if candidate == selected {
				want = ValidationValid
			}
			require.Equal(t, want, roas.Validate(prefix, 64500), "cache %d selected, checking %s", selected, prefix)
			require.Equal(t, candidate == selected, aspas.isProvider(64500, provider))
		}
	}
	for _, want := range [][2]uint8{{2, pduResetQuery}, {2, pduSerialQuery}, {3, pduResetQuery}, {2, pduResetQuery}} {
		require.Equal(t, want, <-queries)
	}
	select {
	case err := <-failures:
		t.Fatal(err)
	default:
	}
}

// TestRTRCacheResetReturnsToPreferredCache drives populated caches through the
// existing polling rounds, with the preferred cache recovering after Cache Reset.
//
// RFC 8210 Section 8.3: "When a router receives this, the router SHOULD attempt
// to connect to any more-preferred caches in its cache list."
// VALIDATES: The next round requests the preferred cache's complete set, while
// validation keeps using the standby's set until the preferred End of Data.
// PREVENTS: Sticking to the standby after Cache Reset, publishing a partial load,
// or merging two caches' authorizations instead of replacing the old set.
// MUTATION: Start subsequent cacheGroup.poll rounds at the holder rather than
// the head of the configured preference order; the preferred query and final
// validation states must fail.
func TestRTRCacheResetReturnsToPreferredCache(t *testing.T) {
	// RFC requirement: RFC8210-8.3-2 positive -- after the standby answers its Serial Query with Cache Reset, the next polling round sends the more-preferred cache a Reset Query and uses its completed, distinct VRP set.
	// RFC requirement: RFC8210-8.3-2 negative -- the router does not remain on the less-preferred cache when the preferred cache recovers; the old-only authorization disappears and the shared prefix rejects the old origin after the preferred End of Data.
	stop := make(chan struct{})
	finish := make(chan struct{})
	queries := make(chan byte, 4)
	replies := make(chan error, 3)
	attempt := 0
	preferredPort, _ := serveRTR(t, func(conn net.Conn) {
		if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
			replies <- err
			return
		}
		query, err := readRTRQuery(conn)
		if err != nil {
			replies <- err
			return
		}
		queries <- query[1]
		attempt++
		if attempt < 3 {
			// No Data Available leaves the socket open: Ze, not the fake
			// peer's disconnect, must end this attempt and try the standby.
			noData := []byte{rtrVersionMax, pduErrorRpt, 0, 2, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0}
			if _, err := conn.Write(noData); err != nil {
				replies <- err
				return
			}
		} else {
			partial := slices.Concat(cacheResponsePDU(),
				ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64501),
				ipv4AnnouncePDU([4]byte{198, 51, 100, 0}, 64501))
			if _, err := conn.Write(partial); err != nil {
				replies <- err
				return
			}
			select {
			case <-finish:
			case <-stop:
				return
			}
			if _, err := conn.Write(endOfDataTimed(9, 3600, 600)); err != nil {
				replies <- err
				return
			}
		}
		var answer [1]byte
		_, err = conn.Read(answer[:])
		replies <- err
	})
	standbyLoad := slices.Concat(cacheResponsePDU(),
		ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64500),
		ipv4AnnouncePDU([4]byte{203, 0, 113, 0}, 64500),
		endOfDataTimed(5, 3600, 600))
	standbyPort, standbyQueries := scriptRTRCache(t, standbyLoad, cacheResetPDU(0))
	roas, aspas := newROACache(), newASPACache()
	preferred := newTestRTRSession(t, "127.0.0.1", preferredPort, 10, "", roas, aspas, stop)
	standby := newTestRTRSession(t, "127.0.0.1", standbyPort, 200, "", roas, aspas, stop)
	group := newCacheGroup([]*RTRSession{preferred, standby}, stop)
	t.Cleanup(func() { close(stop) })
	states := func() [4]uint8 {
		// RFC 8210 Section 10: observe the real validation consumer, not
		// session flags or the number of records the fake peers wrote.
		return [4]uint8{
			roas.Validate("203.0.113.0/24", 64500),
			roas.Validate("192.0.2.0/24", 64500),
			roas.Validate("192.0.2.0/24", 64501),
			roas.Validate("198.51.100.0/24", 64501),
		}
	}
	held := [4]uint8{ValidationValid, ValidationValid, ValidationInvalid, ValidationNotFound}

	// RFC 8210 Sections 8.3 and 10: use normal rounds without modifying
	// timers, forcing holder/serial state, or changing reconnect policy.
	require.Equal(t, 3600*time.Second, group.poll())
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, queries, 1))
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1))
	require.Equal(t, held, states(), "the standby's wire load must supply the initial validation state")

	require.Equal(t, 600*time.Second, group.poll())
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, queries, 1))
	require.Equal(t, []byte{pduSerialQuery}, takeQueries(t, standbyQueries, 1))
	require.Equal(t, held, states(), "Cache Reset must retain the last complete set")

	done := make(chan struct{})
	var delay time.Duration
	go func() {
		defer close(done)
		delay = group.poll()
	}()
	t.Cleanup(func() {
		preferred.close()
		standby.close()
		<-done
	})
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, queries, 1))
	require.Eventually(t, func() bool {
		preferred.mu.Lock()
		defer preferred.mu.Unlock()
		if len(preferred.pendingVRPs) != 2 {
			return false
		}
		return preferred.pendingVRPs[1].Prefix.String() == "198.51.100.0/24"
	}, 3*time.Second, time.Millisecond, "the preferred partial load never reached the real session")
	require.Equal(t, held, states(), "a parsed partial preferred load must not replace the working set")
	close(finish)
	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("the preferred End of Data did not complete the polling round")
	}
	require.Equal(t, 3600*time.Second, delay)
	require.Equal(t, [4]uint8{ValidationNotFound, ValidationInvalid, ValidationValid, ValidationValid},
		states(), "the preferred full set must replace, not merge with, the standby's set")
	select {
	case query := <-standbyQueries:
		t.Fatalf("the standby was queried again after preferred recovery: PDU %d", query)
	default:
	}
	for range 3 {
		select {
		case err := <-replies:
			require.ErrorIs(t, err, io.EOF, "the router must finish each preferred-cache exchange")
		case <-time.After(6 * time.Second):
			t.Fatal("the preferred cache did not observe the router close its connection")
		}
	}
}
