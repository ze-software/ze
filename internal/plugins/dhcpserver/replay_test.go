package dhcpserver

import (
	"encoding/json"
	"net"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// wanSnapshot answers the JSON body of one system-dns snapshot holding a
// WAN dhcpv4 lease of addresses, as the engine's replay carries it.
func wanSnapshot(t *testing.T, sequence uint64, addresses ...netip.Addr) string {
	t.Helper()
	body, err := json.Marshal(dnssource.Snapshot{Sequence: sequence, Records: []dnssource.Record{{
		Origin: "dhcpv4", Rank: 100, Interface: "wan0", Unit: "0",
		Addresses: addresses, Received: time.Now(), Expires: time.Now().Add(time.Hour),
	}}})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return string(body)
}

// TestReplyWaitsForReplay proves spec-home-cpe-23b AC-13 for the DHCP
// server: a server that starts while a WAN lease exists holds a DISCOVER
// until the system-dns replay is applied, so its first OFFER carries the
// learned option 6 and the configured lease, never the empty selection and
// the 60-second lease of D-9 it would build from the empty record set it
// starts with. Method: a pass-through subnet whose Follower expects the
// replay, as followLearnedResolvers sets it; one goroutine answers a
// DISCOVER; it must still be held after a pause; the replay then goes
// through the event handler followLearnedResolvers installs, and the OFFER
// it released is read.
//
// PREVENTS: review NOTE 15, the DHCP server answering before its replay.
func TestReplyWaitsForReplay(t *testing.T) {
	h := dnsTestSubnet(t, subnetDNSDefault)
	h.learned.ExpectReplay()

	offers := make(chan []byte, 1)
	go func() {
		offers <- h.handle(buildDiscover(net.HardwareAddr{0x02, 0, 0, 0, 0x13, 1}, 0x1301))
	}()

	select {
	case <-offers:
		t.Fatal("a DISCOVER was answered before the replay was applied")
	case <-time.After(100 * time.Millisecond):
	}

	var publisher dhcpPublisher
	handler := learnedSnapshotHandler(h.learned, &publisher, func() metrics.Counter { return &countingCounter{} }, slogutil.DiscardLogger())
	if err := handler(wanSnapshot(t, 1, wanDNS)); err != nil {
		t.Fatalf("replay answered %v", err)
	}

	var offer []byte
	select {
	case offer = <-offers:
	case <-time.After(time.Second):
		t.Fatal("the applied replay did not release the held DISCOVER")
	}
	if offer == nil {
		t.Fatal("no DHCPOFFER")
	}
	if got := option6Of(t, offer); !slices.Equal(got, []netip.Addr{wanDNS}) {
		t.Fatalf("option 6 of the first OFFER = %v, want the replayed %v", got, wanDNS)
	}
	if got := leaseOf(t, offer); got != 3600 {
		t.Fatalf("lease of the first OFFER = %d, want the configured 3600", got)
	}
}

// TestRejectedReplayDoesNotRelease: a snapshot the server cannot decode is
// not the replay, so it releases no held reply; the next valid one does.
// Method: as TestReplyWaitsForReplay, feeding an undecodable body first.
func TestRejectedReplayDoesNotRelease(t *testing.T) {
	h := dnsTestSubnet(t, subnetDNSDefault)
	h.learned.ExpectReplay()

	offers := make(chan []byte, 1)
	go func() {
		offers <- h.handle(buildDiscover(net.HardwareAddr{0x02, 0, 0, 0, 0x13, 2}, 0x1302))
	}()

	var publisher dhcpPublisher
	handler := learnedSnapshotHandler(h.learned, &publisher, func() metrics.Counter { return &countingCounter{} }, slogutil.DiscardLogger())
	if err := handler("{not a snapshot"); err != nil {
		t.Fatalf("rejected snapshot answered %v", err)
	}
	select {
	case <-offers:
		t.Fatal("an undecodable snapshot released the held DISCOVER")
	case <-time.After(100 * time.Millisecond):
	}

	if err := handler(wanSnapshot(t, 1, wanDNS)); err != nil {
		t.Fatalf("replay answered %v", err)
	}
	select {
	case offer := <-offers:
		if got := option6Of(t, offer); !slices.Equal(got, []netip.Addr{wanDNS}) {
			t.Fatalf("option 6 = %v, want %v", got, wanDNS)
		}
	case <-time.After(time.Second):
		t.Fatal("the valid replay did not release the held DISCOVER")
	}
}

// TestReplyNotHeldWithoutReplay: a handler built alone expects no replay, so
// a DISCOVER is answered at once. Method: time one exchange.
func TestReplyNotHeldWithoutReplay(t *testing.T) {
	h := dnsTestSubnet(t, subnetDNSDefault)
	start := time.Now()
	offerOption6(t, h, 0x13)
	if waited := time.Since(start); waited > 500*time.Millisecond {
		t.Fatalf("a handler expecting no replay held a DISCOVER %v", waited)
	}
}
