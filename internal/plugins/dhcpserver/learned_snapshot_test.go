package dhcpserver

import (
	"encoding/json"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// countingCounter is a metrics.Counter that remembers how often it stepped.
type countingCounter struct{ steps float64 }

func (c *countingCounter) Inc()          { c.steps++ }
func (c *countingCounter) Add(v float64) { c.steps += v }

// TestLearnedSnapshotBatchKeepsLaterSnapshot proves a system-dns snapshot the
// DHCP server cannot apply does not cost it the valid snapshot delivered after
// it in the same batch. Method: the handler followLearnedResolvers installs is
// fed a batch of [undecodable body, valid snapshot] the way the SDK's batch
// loop feeds it (pkg/plugin/sdk OnEvent and activateBridge), which moves to
// the next event only on a nil answer; the test then reads the follower's
// records and the rejection counter.
//
// VALIDATES: spec-home-cpe-23b, a rejected snapshot is logged at WARN and
// counted, never returned to the SDK. PREVENTS: a bad snapshot ending the
// batch, so option 6 keeps stale learned resolvers until the next lease
// change.
func TestLearnedSnapshotBatchKeepsLaterSnapshot(t *testing.T) {
	record := dnssource.Record{
		Origin: "dhcpv4", Rank: 100, Interface: "eth0", Unit: "0",
		Received:  time.Unix(1, 0).UTC(),
		Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.53")},
	}
	body, err := json.Marshal(&dnssource.Snapshot{Sequence: 1, Records: []dnssource.Record{record}})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var learned dnssource.Follower
	var publisher dhcpPublisher
	rejected := &countingCounter{}
	handler := learnedSnapshotHandler(&learned, &publisher, rejected, slogutil.DiscardLogger())

	for i, event := range []string{"{not a snapshot", string(body)} {
		if err := handler(event); err != nil {
			t.Fatalf("event %d answered %v: the SDK's batch loop stops there and drops every later snapshot", i, err)
		}
	}

	if rejected.steps != 1 {
		t.Fatalf("rejection counter stepped %v times, want 1", rejected.steps)
	}
	if !learned.Held() {
		t.Fatal("the valid snapshot after the rejected one was not applied")
	}
	got := learned.Records()
	if len(got) != 1 {
		t.Fatalf("follower holds %d records, want 1: %+v", len(got), got)
	}
	if !slices.Equal(got[0].Addresses, record.Addresses) {
		t.Fatalf("follower holds %v, want %v", got[0].Addresses, record.Addresses)
	}
}
