// Design: docs/architecture/ldp/mpls-ldp.md -- adjacency expiry tears the session (F6) test
package ldp

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: F6 -- when an adjacency times out (e.g. after its interface is
// removed and Hellos stop), its session is stopped so the peer's labels are
// withdrawn, rather than lingering until the session's own keepalive timeout.
func TestExpireAdjacenciesStopsSession(t *testing.T) {
	lsrID := [4]byte{10, 0, 0, 2}
	var labelSpace uint16
	key := AdjacencyKey(lsrID, labelSpace)

	adjTable := newAdjacencyTable()
	adj, _ := adjTable.Update(PDUHeader{LSRID: lsrID, LabelSpace: labelSpace}, HelloMessage{}, "")
	adj.LastSeen = time.Now().Add(-time.Hour) // force the adjacency past its hold time

	c1, c2 := net.Pipe()
	t.Cleanup(func() { _ = c2.Close() })
	sess := NewSession(c1, SessionConfig{
		LocalLSRID:     [4]byte{10, 0, 0, 1},
		PeerLSRID:      lsrID,
		PeerLabelSpace: labelSpace,
		PeerAddr:       netip.MustParseAddr("10.0.0.2"),
		KeepaliveTime:  DefaultKeepaliveTime,
	}, newLIB(), slogutil.DiscardLogger())
	var mu sync.Mutex
	sessions := map[string]*Session{key: sess}

	expireAdjacencies(slogutil.DiscardLogger(), adjTable, sessions, map[string]*setupRetry{}, &mu)

	assert.True(t, sess.stopped(), "session must be stopped when its adjacency expires")
}

// AC-5: an adjacency that expires while its session is still in setup leaves no
// backoff entry once the session has ended, so a neighbor rediscovered later is
// dialed on its first Hello. The session is opened by startSessionForAdj, so the
// end-of-session closure that runs after the expiry is the engine's own.
func TestAdjacencyExpiryLeavesNoBackoffEntry(t *testing.T) {
	h := newSessionEndHarness(t)
	adj := testAdjacency()
	key := AdjacencyKey(adj.PeerLSRID, adj.PeerLabelSpace)

	adjTable := newAdjacencyTable()
	entry, _ := adjTable.Update(PDUHeader{LSRID: adj.PeerLSRID, LabelSpace: adj.PeerLabelSpace},
		HelloMessage{TransportAddr: adj.TransportAddr}, adj.Interface)

	remote := h.mustStart(adj)
	h.readInitialization(remote) // the session is in OPENSENT, never operational
	entry.LastSeen = time.Now().Add(-time.Hour)
	expireAdjacencies(slogutil.DiscardLogger(), adjTable, h.sessions, h.retries, &h.mu)
	h.awaitSessionEnded(key)

	if r := h.backoff(key); r != nil {
		t.Errorf("backoff entry after the adjacency expired: delay %s; want none, so a neighbor rediscovered later is dialed at once", r.delay)
	}
}
