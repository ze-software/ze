//go:build integration && linux

// Design: rate.go -- per-interface rate tracker (collect loop, ListRates/GetRate)
//
// Integration coverage for the exact pipeline ddos-detect consumes: the rate
// tracker's collect() goroutine reads raw kernel counters via the netlink backend
// and derives per-second rates. The ddos functional tests could never observe this
// in QEMU (the .ci runner truncates daemon logs), so this test drives it directly
// on a real kernel with full Go-test visibility: flood lo and assert the tracker
// reports a non-zero RxPps/RxBps. A zero rate here -- while the raw counters climb
// -- pinpoints a break in the tracker's delta pipeline (the source ddos-detect and
// the ze_interface_*_per_second metrics both consume).
//
// This test runs in the VM's MAIN namespace, and it must. withNetNS pins only the
// test goroutine's thread into an ephemeral namespace, while tracker.Start()
// spawns collect() on another thread that stays in the ORIGINAL namespace, so
// collect() reads the wrong (quiet) lo and always sees a 0 delta. In the main
// namespace the tracker's collect() and the flood share a namespace, which is what
// the daemon does.

package iface

import (
	"net"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

func TestRateTracker_LoopbackFloodProducesNonZeroRate(t *testing.T) {
	if err := LoadBackend("netlink"); err != nil {
		t.Fatalf("load netlink backend: %v", err)
	}
	t.Cleanup(func() { _ = CloseBackend() })

	// lo is up in the VM's main namespace; make sure.
	if lo, lerr := netlink.LinkByName("lo"); lerr == nil {
		_ = netlink.LinkSetUp(lo)
	}

	tracker := newRateTracker()
	globalTracker.Store(tracker)
	defer globalTracker.Store(nil)
	tracker.Start()
	defer tracker.Stop()

	before, err := GetInterface("lo")
	if err != nil || before.Stats == nil {
		t.Fatalf("GetInterface(lo) before: info=%+v err=%v", before, err)
	}

	// An UNCONNECTED socket (ListenPacket + WriteTo), like the .ci observer's
	// sendto: a connected UDP socket to a closed loopback port surfaces the ICMP
	// port-unreachable as ECONNREFUSED and stalls the flood; unconnected sends keep
	// flowing, so lo's counters actually climb.
	pc, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	defer func() {
		if cerr := pc.Close(); cerr != nil {
			t.Logf("close conn: %v", cerr)
		}
	}()
	dst := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: 9999}
	payload := make([]byte, 1024)

	flood := func(n int) {
		for range n {
			_, werr := pc.WriteTo(payload, dst)
			if werr != nil {
				continue // drop (e.g. ENOBUFS); keep flooding
			}
		}
	}

	// Drive the flood across several 1s collect cycles, tracking the MAX rate the
	// tracker reports for lo (a single 1s tick with no packets reads 0). A brief
	// sleep between bursts lets the collect() goroutine actually run and tick.
	var maxRxPps, maxRxBps float64
	var haveRate bool
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		flood(20000)
		if r, ok := tracker.get("lo"); ok {
			haveRate = true
			if r.RxPps > maxRxPps {
				maxRxPps = r.RxPps
			}
			if r.RxBps > maxRxBps {
				maxRxBps = r.RxBps
			}
			if maxRxPps > 0 || maxRxBps > 0 {
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	after, err := GetInterface("lo")
	if err != nil || after.Stats == nil {
		t.Fatalf("GetInterface(lo) after: info=%+v err=%v", after, err)
	}
	rawDeltaPkts := after.Stats.RxPackets - before.Stats.RxPackets
	rawDeltaBytes := after.Stats.RxBytes - before.Stats.RxBytes
	t.Logf("raw lo counters moved: rx-packets delta=%d, rx-bytes delta=%d", rawDeltaPkts, rawDeltaBytes)

	if rawDeltaPkts == 0 {
		t.Fatalf("loopback flood did not move lo rx-packets -- test cannot drive the pipeline")
	}
	if !haveRate {
		t.Fatalf("lo never appeared in the rate tracker (ListRates)")
	}
	t.Logf("tracker lo max rate: RxPps=%.1f RxBps=%.1f", maxRxPps, maxRxBps)

	if maxRxPps <= 0 && maxRxBps <= 0 {
		t.Fatalf("rate tracker reports zero RxPps AND RxBps for lo across the whole flood while raw counters moved by %d packets / %d bytes -- the rate tracker delta pipeline (the source ddos-detect and ze_interface_*_per_second consume) is broken",
			rawDeltaPkts, rawDeltaBytes)
	}
}
