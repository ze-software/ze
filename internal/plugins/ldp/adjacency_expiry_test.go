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
	sess := NewSession(c1, [4]byte{10, 0, 0, 1}, 0, lsrID, labelSpace,
		netip.MustParseAddr("10.0.0.2"), newLIB(), slogutil.DiscardLogger())
	var mu sync.Mutex
	sessions := map[string]*Session{key: sess}

	expireAdjacencies(slogutil.DiscardLogger(), adjTable, sessions, &mu)

	assert.True(t, sess.stopped(), "session must be stopped when its adjacency expires")
}
