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
			// A one-second Retry Interval lets the third round ask the failed cache again.
			binary.BigEndian.PutUint32(end[16:20], 1)
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
		// RFC 8210 Section 6: a round does not ask a failed cache again before its Retry
		// Interval runs out, so each round starts once the preferred cache may be asked.
		time.Sleep(time.Until(group.retryAt[0]))
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
		switch attempt {
		case 1:
			// The first load teaches the router this cache's two-second Retry Interval.
			first := slices.Concat(cacheResponsePDU(),
				ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64501),
				endOfDataTimed(3, 1, 2))
			if _, err := conn.Write(first); err != nil {
				replies <- err
				return
			}
		case 2:
			// No Data Available leaves the socket open: Ze, not the fake
			// peer's disconnect, must end this attempt and try the standby.
			noData := []byte{rtrVersionMax, pduErrorRpt, 0, 2, 0, 0, 0, 16, 0, 0, 0, 0, 0, 0, 0, 0}
			if _, err := conn.Write(noData); err != nil {
				replies <- err
				return
			}
		default:
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
	require.Equal(t, time.Second, group.poll())
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, queries, 1))

	require.Equal(t, 3600*time.Second, group.poll())
	require.Equal(t, []byte{pduSerialQuery}, takeQueries(t, queries, 1))
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1))
	require.Equal(t, held, states(), "the standby's wire load must supply the initial validation state")

	// RFC 8210 Section 6: the preferred cache failed less than its two-second Retry Interval
	// ago, so this round passes over it. RFC 8210 Section 8.3: with no more-preferred cache
	// available, the standby's Cache Reset is followed by a Reset Query to the standby itself,
	// which the script answers with nothing. The round then waits only until the preferred
	// cache may be asked again, and the next round runs after that wait.
	delay := group.poll()
	require.Equal(t, []byte{pduSerialQuery, pduResetQuery}, takeQueries(t, standbyQueries, 2))
	require.LessOrEqual(t, delay, 2*time.Second, "the next round must not wait past the preferred Retry Interval")
	select {
	case query := <-queries:
		t.Fatalf("the preferred cache was asked inside its Retry Interval: PDU %d", query)
	default:
	}
	require.Equal(t, held, states(), "Cache Reset must retain the last complete set")
	time.Sleep(delay)

	done := make(chan struct{})
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

// outageRTRCache answers the connections the router opens in order, one reply each, and reports
// the type of the query that opened each connection. A nil reply is an outage: the cache reads
// the query and drops the connection unanswered, so the router's query fails. A connection past
// the script is dropped the same way.
func outageRTRCache(t *testing.T, replies ...[]byte) (uint16, <-chan byte) {
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
		queries <- query[1]
		if attempt >= len(replies) {
			return
		}
		reply := replies[attempt]
		attempt++
		if reply == nil {
			return
		}
		if _, err := conn.Write(reply); err != nil {
			return
		}
		_, _ = conn.Read(query[:1])
	})
	return port, queries
}

// requireNoQuery fails the test when the cache receives a query within wait.
func requireNoQuery(t *testing.T, queries <-chan byte, wait time.Duration, why string) {
	t.Helper()
	select {
	case query := <-queries:
		t.Fatalf("%s: PDU type %d", why, query)
	case <-time.After(wait):
	}
}

// preferredLoad is a complete load announcing 192.0.<octet>.0/24 for AS 64501, with the given
// serial, Refresh Interval and Retry Interval in seconds.
func preferredLoad(octet byte, serial, refresh, retry uint32) []byte {
	return slices.Concat(cacheResponsePDU(),
		ipv4AnnouncePDU([4]byte{192, 0, octet, 0}, 64501),
		endOfDataTimed(serial, refresh, retry))
}

// standbyLoad is the standby cache's complete load: 203.0.113.0/24 for AS 64500, polled hourly.
func standbyLoad() []byte {
	return slices.Concat(cacheResponsePDU(),
		ipv4AnnouncePDU([4]byte{203, 0, 113, 0}, 64500),
		endOfDataTimed(5, 3600, 600))
}

// TestCacheGroupCacheResetFromPreferredReloadsIt drives two caches from the parsed
// `bgp rpki cache-server` list through startSessions. The preference 10 cache holds the data and
// answers the router's second poll, a Serial Query, with Cache Reset.
//
// VALIDATES: RFC 8210 Section 8.3, "If there are no more-preferred caches, it MUST issue a Reset
// Query and get an entire new load from the cache.": in the same round the router sends that
// cache a Reset Query and validation then uses its new set, and RFC 8210 Section 4, "The router
// MUST choose the most preferred, by configuration, cache": the preference 20 cache receives no
// connection.
// PREVENTS: a Cache Reset from the most preferred cache handing the load to a less-preferred
// cache, which was what `poll` did until 2026-10-27.
func TestCacheGroupCacheResetFromPreferredReloadsIt(t *testing.T) {
	// RFC requirement: RFC8210-8.3-1 positive -- with two caches configured, the preference 10 cache answers the Serial Query with Cache Reset and, in the same round, receives a Reset Query whose load replaces the set: its new VRP validates and the reset one stops validating.
	// RFC requirement: RFC8210-8.3-1 negative -- the Cache Reset does not hand the load to the preference 20 cache: that cache receives no query and its VRP never validates.
	// RFC requirement: RFC8210-4-1 positive -- after a Cache Reset the preference 10 cache, written second in the config, still supplies the set validation uses.
	// RFC requirement: RFC8210-4-1 negative -- the preference 20 cache, written first in the config, is not queried while the preference 10 cache answers.
	preferredPort, preferredQueries := outageRTRCache(t,
		preferredLoad(2, 5, 1, 1), cacheResetPDU(0), preferredLoad(3, 9, 3600, 600))
	standbyPort, standbyQueries := outageRTRCache(t, standbyLoad())

	// The standby is written FIRST, so a pass that reads config order contacts the wrong cache.
	rp := newStatePlugin(t)
	rp.startSessions(&rpkiConfig{CacheServers: []cacheServerConfig{
		{Address: "127.0.0.1", Port: standbyPort, Preference: 20},
		{Address: "127.0.0.1", Port: preferredPort, Preference: 10},
	}})

	assert.Equal(t, []byte{pduResetQuery, pduSerialQuery, pduResetQuery}, takeQueries(t, preferredQueries, 3),
		"after Cache Reset the router must ask the same cache for its whole set")
	require.Eventually(t, func() bool {
		return rp.cache.Validate("192.0.3.0/24", 64501) == ValidationValid
	}, 6*time.Second, time.Millisecond, "the reload after the Cache Reset was not published")
	assert.Equal(t, ValidationNotFound, rp.cache.Validate("192.0.2.0/24", 64501),
		"the set the Cache Reset invalidated survived its reload")
	assert.Equal(t, ValidationNotFound, rp.cache.Validate("203.0.113.0/24", 64500),
		"the standby's set reached validation")
	requireNoQuery(t, standbyQueries, 500*time.Millisecond,
		"the standby was queried although the most preferred cache answered the Reset Query")
}

// TestCacheGroupRepeatedCacheResetMovesOn runs two polling rounds over two caches. In the second,
// the preference 10 cache answers the Serial Query with Cache Reset, and the Reset Query that
// follows with Cache Reset again.
//
// VALIDATES: the bound on RFC 8210 Section 8.3's Reset Query: one per cache per round. A second
// Cache Reset fails the attempt, and the round moves on to the preference 20 cache in the
// preference order, the way RFC 8210 Section 8.4 moves on from a cache with no data.
// PREVENTS: a cache that answers every query with Cache Reset pinning the router in a loop of
// Reset Queries, and that cache's failure leaving the router with no set it can load.
func TestCacheGroupRepeatedCacheResetMovesOn(t *testing.T) {
	preferredPort, preferredQueries := outageRTRCache(t,
		preferredLoad(2, 5, 3600, 600), cacheResetPDU(0), cacheResetPDU(0))
	standbyPort, standbyQueries := outageRTRCache(t, standbyLoad())
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	group := newCacheGroup([]*RTRSession{
		newTestRTRSession(t, "127.0.0.1", preferredPort, 10, "", roas, aspas, stop),
		newTestRTRSession(t, "127.0.0.1", standbyPort, 20, "", roas, aspas, stop),
	}, stop)

	require.Equal(t, 3600*time.Second, group.poll())
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, preferredQueries, 1))
	require.Equal(t, ValidationValid, roas.Validate("192.0.2.0/24", 64501))

	require.Equal(t, 3600*time.Second, group.poll(), "the standby's load ends the round")
	assert.Equal(t, []byte{pduSerialQuery, pduResetQuery}, takeQueries(t, preferredQueries, 2),
		"one Reset Query follows the first Cache Reset")
	assert.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1),
		"the second Cache Reset hands the round to the next cache")
	assert.Equal(t, ValidationValid, roas.Validate("203.0.113.0/24", 64500), "the standby's set is in use")
	assert.Equal(t, ValidationNotFound, roas.Validate("192.0.2.0/24", 64501),
		"the preferred cache's set survived the switch")
	requireNoQuery(t, preferredQueries, 200*time.Millisecond,
		"the router asked the resetting cache again in the same round")
}

// TestCacheGroupFailsBackAtPreferredRetryInterval runs a cache group in its own goroutine, as
// startSessions does. The preference 10 cache loads with a one-second Refresh and Retry Interval,
// drops the next poll, and answers the one after it. The preference 20 cache loads with a
// one-hour Refresh Interval.
//
// VALIDATES: RFC 8210 Section 10, "When a more-preferred cache becomes available, if resources
// allow, it would be prudent for the client to start fetching from that cache.": the router asks
// the preference 10 cache again at that cache's own Retry Interval and switches to its set at End
// of Data. RFC 8210 Section 6, "The router SHOULD NOT poll the cache sooner than indicated by
// this parameter.": the preference 20 cache receives no query besides its load.
// PREVENTS: fail-back waiting for the standby's Refresh Interval, an hour by default, as it did
// until 2026-10-27, and a fail-back probe that polls the standby early.
func TestCacheGroupFailsBackAtPreferredRetryInterval(t *testing.T) {
	preferredPort, preferredQueries := outageRTRCache(t,
		preferredLoad(2, 5, 1, 1), nil, preferredLoad(3, 9, 3600, 600))
	standbyPort, standbyQueries := outageRTRCache(t, standbyLoad())
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	group := newCacheGroup([]*RTRSession{
		newTestRTRSession(t, "127.0.0.1", preferredPort, 10, "", roas, aspas, stop),
		newTestRTRSession(t, "127.0.0.1", standbyPort, 20, "", roas, aspas, stop),
	}, stop)
	done := make(chan struct{})
	go func() { defer close(done); group.Run() }()
	t.Cleanup(func() {
		close(stop)
		<-done
	})

	require.Equal(t, []byte{pduResetQuery, pduSerialQuery}, takeQueries(t, preferredQueries, 2))
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1),
		"the dropped poll hands the load to the standby")
	failedOver := time.Now()

	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, preferredQueries, 1),
		"the preferred cache was not asked again while the standby held the load")
	assert.Less(t, time.Since(failedOver), 3*time.Second,
		"the preferred cache was asked again later than its one-second Retry Interval allows")
	require.Eventually(t, func() bool {
		return roas.Validate("192.0.3.0/24", 64501) == ValidationValid
	}, 6*time.Second, time.Millisecond, "the preferred cache's set did not replace the standby's")
	assert.Equal(t, ValidationNotFound, roas.Validate("203.0.113.0/24", 64500),
		"the standby's set survived the fail-back")
	requireNoQuery(t, standbyQueries, 1500*time.Millisecond,
		"the standby was polled before its one-hour Refresh Interval")
}

// TestCacheGroupFullRoundWaitsForRetryInterval runs full polling rounds over two caches, the way
// Run does at the holder's Refresh Interval. The preference 10 cache loads with a one-second
// Refresh Interval and a two-second Retry Interval, then drops the next poll. The preference 20
// cache then holds the load with a ten-second Refresh Interval, StayRTR's default, so a round at
// the holder's Refresh Interval can fall inside the preference 10 cache's Retry Interval.
//
// VALIDATES: RFC 8210 Section 6, "The router SHOULD NOT retry sooner than indicated by this
// parameter.": a full round inside the preference 10 cache's Retry Interval passes over it and
// polls the holder, and the first round after that Retry Interval asks it again and takes its set.
// PREVENTS: a full round retrying a failed more-preferred cache before its own Retry Interval runs
// out because the holder's Refresh Interval is shorter, which poll did until 2026-10-27.
func TestCacheGroupFullRoundWaitsForRetryInterval(t *testing.T) {
	// RFC requirement: RFC8210-6-4 positive -- after the preference 10 cache's query fails, the first full round once its two-second Retry Interval has run out sends it a Reset Query, and its new VRP then validates in place of the standby's.
	// RFC requirement: RFC8210-6-4 negative -- a full round that runs inside that Retry Interval sends the preference 10 cache no query: the standby answers the round's Serial Query, and the standby's VRP stays the one that validates.
	preferredPort, preferredQueries := outageRTRCache(t,
		preferredLoad(2, 5, 1, 2), nil, preferredLoad(3, 9, 3600, 600))
	standbyPort, standbyQueries := outageRTRCache(t,
		slices.Concat(cacheResponsePDU(),
			ipv4AnnouncePDU([4]byte{203, 0, 113, 0}, 64500),
			endOfDataTimed(5, 10, 600)),
		slices.Concat(cacheResponsePDU(), endOfDataTimed(6, 10, 600)))
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	group := newCacheGroup([]*RTRSession{
		newTestRTRSession(t, "127.0.0.1", preferredPort, 10, "", roas, aspas, stop),
		newTestRTRSession(t, "127.0.0.1", standbyPort, 20, "", roas, aspas, stop),
	}, stop)

	require.Equal(t, time.Second, group.poll(), "the preference 10 cache's load ends the first round")
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, preferredQueries, 1))

	require.Equal(t, 10*time.Second, group.poll(), "the dropped poll hands the load to the standby")
	require.Equal(t, []byte{pduSerialQuery}, takeQueries(t, preferredQueries, 1))
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1))
	retryEnd := group.retryAt[0]

	require.Equal(t, 10*time.Second, group.poll(), "the standby's incremental answer ends the round")
	require.True(t, time.Now().Before(retryEnd), "the round outlasted the Retry Interval it tests")
	require.Equal(t, []byte{pduSerialQuery}, takeQueries(t, standbyQueries, 1))
	requireNoQuery(t, preferredQueries, 100*time.Millisecond,
		"the preference 10 cache was asked inside its Retry Interval")
	assert.Equal(t, ValidationValid, roas.Validate("203.0.113.0/24", 64500), "the standby's set is in use")
	assert.Equal(t, ValidationNotFound, roas.Validate("192.0.2.0/24", 64501),
		"the preference 10 cache's old set survived the switch")

	time.Sleep(time.Until(retryEnd))
	require.Equal(t, 3600*time.Second, group.poll(), "the preference 10 cache's reload ends the round")
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, preferredQueries, 1))
	assert.Equal(t, ValidationValid, roas.Validate("192.0.3.0/24", 64501), "the reload is not in use")
	assert.Equal(t, ValidationNotFound, roas.Validate("203.0.113.0/24", 64500),
		"the standby's set survived the fail-back")
	requireNoQuery(t, standbyQueries, 100*time.Millisecond,
		"the standby was asked although the preference 10 cache answered")
}

// ipv4WithdrawPDU is an IPv4 Prefix PDU withdrawing prefix/24, Max Length 24, for asn.
func ipv4WithdrawPDU(prefix [4]byte, asn uint32) []byte {
	pdu := ipv4AnnouncePDU(prefix, asn)
	pdu[8] = 0
	return pdu
}

// TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData feeds two sessions of one cache group,
// PDU by PDU, through the session's own handler: the old cache loads a set, then the router
// switches to the new cache, whose load withdraws a VRP and an ASPA the old cache announced.
// Validation is read after every step of the switch.
//
// VALIDATES: RFC 8210 Section 10, "A client may hold data from multiple caches but MUST keep the
// data marked as to source, as later updates MUST affect the correct data.": the new cache's
// withdraws never reach the old cache's records, its End of Data replaces the set exactly, and a
// later delta from the old cache no longer touches the set in use. The same section, "The client
// SHOULD attempt to maintain at least one set of data, regardless of whether it has chosen a
// different cache or established a new connection to the previous cache." and "If a client loses
// connectivity to a cache it is using or otherwise decides to switch to a new cache, it SHOULD
// retain the data from the previous cache until it has a full set of data from one or more other
// caches.": until End of Data, validation answers from the old set, unchanged, ROAs and ASPAs alike.
// PREVENTS: a staged withdraw or announcement reaching the store before End of Data, an End of
// Data that merges the two caches' sets, and the old cache's withdraw deleting the new cache's VRP.
func TestCacheSwitchWindowKeepsPreviousSetUntilEndOfData(t *testing.T) {
	// RFC requirement: RFC8210-10-2 positive -- the new cache's End of Data leaves exactly its own VRP and ASPA in the stores, ROAs and ASPAs alike.
	// RFC requirement: RFC8210-10-2 negative -- the new cache's withdraw of the old cache's VRP and ASPA changes nothing before End of Data, and a later delta withdraw from the old cache is refused and leaves the new cache's VRP valid.
	// RFC requirement: RFC8210-10-3 positive -- while the switch is in progress, validation answers Valid for every VRP and ASPA provider of the old cache's set.
	// RFC requirement: RFC8210-10-3 negative -- no step of the switch before End of Data leaves the stores empty or holding the new cache's VRP or ASPA.
	// RFC requirement: RFC8210-10-4 positive -- the old cache's set stays the one validation uses until the new cache's End of Data.
	// RFC requirement: RFC8210-10-4 negative -- the new cache's staged announcements do not validate before its End of Data.
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	oldCache := newTestRTRSession(t, "127.0.0.1", 3323, 10, "", roas, aspas, stop)
	newCache := newTestRTRSession(t, "127.0.0.1", 3324, 20, "", roas, aspas, stop)
	newCacheGroup([]*RTRSession{oldCache, newCache}, stop)

	// The old cache's set: two VRPs, and two customers with one provider each.
	oldCache.startFullSync()
	for _, pdu := range [][]byte{
		cacheResponsePDU(),
		ipv4AnnouncePDU([4]byte{192, 0, 2, 0}, 64500),
		ipv4AnnouncePDU([4]byte{198, 51, 100, 0}, 64501),
		aspaPDU(64500, 64501),
		aspaPDU(64510, 64511),
	} {
		applyPDU(t, oldCache, pdu, false)
	}
	applyPDU(t, oldCache, endOfDataPDU(), true)

	requireOldSet := func(step string) {
		t.Helper()
		require.Equal(t, ValidationValid, roas.Validate("192.0.2.0/24", 64500), "%s: old VRP 192.0.2.0/24", step)
		require.Equal(t, ValidationValid, roas.Validate("198.51.100.0/24", 64501), "%s: old VRP 198.51.100.0/24", step)
		require.Equal(t, ValidationNotFound, roas.Validate("203.0.113.0/24", 64502), "%s: new VRP 203.0.113.0/24", step)
		require.True(t, aspas.isProvider(64500, 64501), "%s: old ASPA of customer 64500", step)
		require.True(t, aspas.isProvider(64510, 64511), "%s: old ASPA of customer 64510", step)
		require.False(t, aspas.isProvider(64520, 64521), "%s: new ASPA of customer 64520", step)
		v4, v6 := roas.Count()
		require.Equal(t, 2, v4+v6, "%s: the store holds the old cache's two VRPs and nothing else", step)
	}
	requireOldSet("old cache loaded")

	// The switch: the new cache's load withdraws the old cache's 192.0.2.0/24 and the ASPA of
	// customer 64500, and announces its own VRP and ASPA. End of Data has not arrived.
	newCache.startFullSync()
	for _, step := range []struct {
		name string
		pdu  []byte
	}{
		{"Cache Response", cacheResponsePDU()},
		{"withdraw of the old VRP", ipv4WithdrawPDU([4]byte{192, 0, 2, 0}, 64500)},
		{"announcement of the new VRP", ipv4AnnouncePDU([4]byte{203, 0, 113, 0}, 64502)},
		{"withdraw of the old ASPA", aspaWithdrawPDU(64500)},
		{"announcement of the new ASPA", aspaPDU(64520, 64521)},
	} {
		applyPDU(t, newCache, step.pdu, false)
		requireOldSet(step.name)
	}

	applyPDU(t, newCache, endOfDataPDU(), true)
	assert.Equal(t, ValidationNotFound, roas.Validate("192.0.2.0/24", 64500), "the old VRP survived the switch")
	assert.Equal(t, ValidationNotFound, roas.Validate("198.51.100.0/24", 64501),
		"the old VRP the new cache never named survived the switch: the two sets merged")
	assert.Equal(t, ValidationValid, roas.Validate("203.0.113.0/24", 64502), "the new cache's VRP is not in use")
	v4, v6 := roas.Count()
	assert.Equal(t, 1, v4+v6, "the store does not hold exactly the new cache's set")
	assert.False(t, aspas.isProvider(64500, 64501), "the old ASPA of customer 64500 survived the switch")
	assert.False(t, aspas.isProvider(64510, 64511), "the old ASPA of customer 64510 survived the switch")
	assert.True(t, aspas.isProvider(64520, 64521), "the new cache's ASPA is not in use")

	// A later delta from the old cache withdraws a VRP the new cache also announced. It is an
	// update to the old cache's data, which is no longer held, so it must not reach the store.
	for _, pdu := range [][]byte{cacheResponsePDU(), ipv4WithdrawPDU([4]byte{203, 0, 113, 0}, 64502)} {
		applyPDU(t, oldCache, pdu, false)
	}
	hdr, err := parseHeader(endOfDataPDU()[:pduHeaderLen])
	require.NoError(t, err)
	_, err = oldCache.handlePDU(hdr, endOfDataPDU())
	require.ErrorIs(t, err, errRtrDataExpired, "the old cache's delta was applied to the new cache's set")
	assert.Equal(t, ValidationValid, roas.Validate("203.0.113.0/24", 64502),
		"the old cache's withdraw deleted the new cache's VRP")
}

// TestNoDataAvailableMovesToNextCacheInPreferenceOrder runs two caches in one group. The
// preference 10 cache holds the data, then answers the next poll with No Data Available.
//
// VALIDATES: RFC 8210 Section 8.4, "When a router receives this kind of error, the router SHOULD
// attempt to connect to any other caches in its cache list, in preference order.": in the same
// round the preference 20 cache receives a Reset Query and its set replaces the old one.
// PREVENTS: a round that ends on No Data Available and leaves the router on a cache that has no
// data to give until the next Retry Interval.
func TestNoDataAvailableMovesToNextCacheInPreferenceOrder(t *testing.T) {
	// RFC requirement: RFC8210-8.4-2 positive -- in the round where the preference 10 cache answers No Data Available, the preference 20 cache receives a Reset Query and its VRP then validates.
	// RFC requirement: RFC8210-8.4-2 negative -- the preference 20 cache is not asked while the preference 10 cache answers, and after the switch the preference 10 cache's VRP no longer validates.
	preferredPort, preferredQueries := outageRTRCache(t,
		preferredLoad(2, 1, 3600, 600), errorReportPDU(2, errNoDataAvail))
	standbyPort, standbyQueries := outageRTRCache(t, standbyLoad())
	roas, aspas := newROACache(), newASPACache()
	stop := make(chan struct{})
	group := newCacheGroup([]*RTRSession{
		newTestRTRSession(t, "127.0.0.1", preferredPort, 10, "", roas, aspas, stop),
		newTestRTRSession(t, "127.0.0.1", standbyPort, 20, "", roas, aspas, stop),
	}, stop)

	require.Equal(t, 3600*time.Second, group.poll(), "the preference 10 cache's load ends the first round")
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, preferredQueries, 1))
	requireNoQuery(t, standbyQueries, 100*time.Millisecond,
		"the preference 20 cache was asked although the preference 10 cache answered")
	require.Equal(t, ValidationValid, roas.Validate("192.0.2.0/24", 64501), "the preference 10 cache's set is not in use")

	require.Equal(t, 3600*time.Second, group.poll(),
		"the round ended on No Data Available instead of loading the preference 20 cache")
	require.Equal(t, []byte{pduSerialQuery}, takeQueries(t, preferredQueries, 1))
	require.Equal(t, []byte{pduResetQuery}, takeQueries(t, standbyQueries, 1))
	assert.Equal(t, ValidationValid, roas.Validate("203.0.113.0/24", 64500), "the preference 20 cache's set is not in use")
	assert.Equal(t, ValidationNotFound, roas.Validate("192.0.2.0/24", 64501),
		"the preference 10 cache's set survived the switch")
}
