package rib

import (
	"encoding/json"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// bestFactRows answers the received rows of `show bgp rib` as decoded maps,
// through the streaming path or, with terminal set, the buffered document
// (routesDocument).
func bestFactRows(t *testing.T, r *RIBManager, terminal bool) []map[string]any {
	t.Helper()
	rows, err := bestFactRowsDecoded(r, terminal)
	require.NoError(t, err)
	return rows
}

// bestFactRowsDecoded is bestFactRows for a goroutine that is not the test's
// own: it answers the decode error, because FailNow MUST run on the test
// goroutine.
func bestFactRowsDecoded(r *RIBManager, terminal bool) ([]map[string]any, error) {
	var answer []byte
	if terminal {
		// The buffered form: the one document a terminal-less read builds
		// when it answers the whole selection at once (routesDocument, which
		// `show bgp rib protocol` answers). The `json` word that reached it
		// on `show bgp rib` is gone (D-7).
		source, release := r.newShowSource("*", scopeReceived, wholeTableRead{})
		answer = []byte(newRoutesDocument(source).Meta().JSON)
		release()
	} else {
		streamed, err := json.Marshal(r.showPipeline("*", ribPipe("received")))
		if err != nil {
			return nil, err
		}
		answer = streamed
	}
	var result struct {
		Routes []map[string]any `json:"routes"`
	}
	if err := json.Unmarshal(answer, &result); err != nil {
		return nil, err
	}
	return result.Routes, nil
}

// bestFactRIB holds two peers announcing 10.0.0.0/24, each with its own
// address as the next hop, before any election has run.
func bestFactRIB(t testing.TB) (*RIBManager, family.Family, []byte) {
	t.Helper()
	r := newTestRIBManagerWithBus(newTestEventBus())
	electionFixtureLocRIB(t, r, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2"))
	fam := family.Family{AFI: 1, SAFI: 1}
	prefix := ipv4Prefix(24, 10, 0, 0)
	for _, fixture := range []struct {
		owner uint64
		peer  string
	}{{1, "192.0.2.1"}, {2, "192.0.2.2"}} {
		peer := fixture.peer
		addr := netip.MustParseAddr(peer)
		r.peerMeta[fixture.owner] = &peerMetadata{Address: addr, PeerASN: 65001, LocalASN: 65000}
		r.bgpPeers[fixture.owner] = storage.NewPeerRIB(peer)
		r.bgpPeers[fixture.owner].Insert(fam, makeAttrBytes(addr.As4()), prefix)
	}
	return r, fam, prefix
}

// TestShowRIBRowsSayWhichPathWon pins the `best` member of a received row of
// `show bgp rib`: true on the path the election recorded as the winner, false
// on every other path of the prefix. The looking glass reads it as
// birdwatcher's `primary`, which reported false on every row before rows
// carried it. Method: two peers announce one prefix, the election runs, and
// both the streaming answer and the buffered document are read.
//
// VALIDATES: a received row carries `best`, true on the recorded winner only.
// PREVENTS: the looking glass reporting `primary: false` on every path.
func TestShowRIBRowsSayWhichPathWon(t *testing.T) {
	r, fam, prefix := bestFactRIB(t)
	change, elected := r.checkBestPathChange(fam, prefix, false, nil)
	require.True(t, elected, "the election must record a winner")
	winner := change.NextHop.String()

	for _, terminal := range []bool{false, true} {
		rows := bestFactRows(t, r, terminal)
		require.Len(t, rows, 2, "terminal=%v", terminal)
		for _, row := range rows {
			best, ok := row["best"].(bool)
			require.True(t, ok, "terminal=%v: row of %v carries no best member: %v", terminal, row["peer"], row)
			assert.Equal(t, row["peer"] == winner, best, "terminal=%v: row of %v", terminal, row["peer"])
		}
	}
}

// TestShowRIBBestFactUnderElectionAndPeerDown runs the best lookup a `show`
// row makes against elections and peerMu writers. Elections take a bestPrev
// shard lock and then peerMu; a row that took the shard lock while holding
// peerMu would wait behind a queued writer while that writer waits on it.
// Method: the three run concurrently; a stall past the deadline is the
// deadlock, and -race checks the reads.
//
// VALIDATES: the best lookup runs with no lock held, after the walk.
// PREVENTS: a show answer deadlocking elections behind a peer-down writer.
func TestShowRIBBestFactUnderElectionAndPeerDown(t *testing.T) {
	r, fam, prefix := bestFactRIB(t)

	const rounds = 200
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for range rounds {
			r.checkBestPathChange(fam, prefix, false, nil)
		}
	}()
	go func() {
		defer wg.Done()
		for range rounds {
			r.peerMu.Lock()
			r.peerMu.Unlock() //nolint:staticcheck,gocritic // the empty critical section is the queued writer under test
		}
	}()
	go func() {
		defer wg.Done()
		for i := range rounds {
			if _, err := bestFactRowsDecoded(r, i%2 == 0); err != nil {
				t.Errorf("show answer round %d: %v", i, err)
				return
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("show, election and a peerMu writer deadlocked")
	}
}
