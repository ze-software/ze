// RFC: rfc/short/rfc8210.md — Section 10, the router connects to its caches in preference order
// Related: rtr_session.go — cacheGroup, startFullSync; rpki.go — startSessions
package rpki

import (
	"encoding/binary"
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

		s := newRTRSession("127.0.0.1", 3323, 100, "", cache, newASPACache(), make(chan struct{}))
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
	pdu[1] = pduCacheResp
	binary.BigEndian.PutUint32(pdu[4:8], pduHeaderLen)
	return pdu
}

// endOfDataPDU is an End of Data carrying the three default intervals and no prefix before it.
func endOfDataPDU() []byte {
	pdu := make([]byte, pduEndOfDataLen)
	pdu[1] = pduEndOfData
	binary.BigEndian.PutUint32(pdu[4:8], pduEndOfDataLen)
	binary.BigEndian.PutUint32(pdu[12:16], 3600)
	binary.BigEndian.PutUint32(pdu[16:20], 600)
	binary.BigEndian.PutUint32(pdu[20:24], 7200)
	return pdu
}
