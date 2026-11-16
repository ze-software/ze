// Design: docs/architecture/memory/lifetime-contracts.md -- aborted retained ownership
// Related: recent_cache.go -- late activation, expiry and pressured gap scans
package reactor

import (
	"bytes"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestAbortedPublicationRetainSurvivesLateLifecycle holds a real receipt and
// adopted variant after publication abort. Late activation, explicit expiry
// and a pressured gap scan all run while that borrow is live; none may reopen
// admission or return its bytes. Only its final Release restores pool balance.
func TestAbortedPublicationRetainSurvivesLateLifecycle(t *testing.T) {
	bufMuxGlobalMu.Lock()
	defer bufMuxGlobalMu.Unlock()
	_, baseline := combinedBufMuxStats()
	cache := newRecentUpdateCache(8)
	clock := sim.NewFakeClock(time.Now())
	cache.SetClock(clock)
	cache.setSafetyValveDuration(time.Minute)
	cache.setPressureHighWater(0.5)
	cache.setPressureValve(time.Nanosecond)
	cache.setPressureSource(func() float64 { return 1 })

	receipt := getReadBuf(false)
	if receipt.Buf == nil {
		t.Fatal("cannot borrow the real receipt")
	}
	body := testUpdatePayload()
	copy(receipt.Buf, body)
	update := &ReceivedUpdate{poolBuf: receipt, WireUpdate: wireu.NewWireUpdate(receipt.Buf[:len(body)], 0)}
	update.WireUpdate.SetMessageID(1)
	cache.Add(update)
	if !cache.Retain(1) {
		t.Fatal("new receipt refused its initial retain")
	}
	held := true
	t.Cleanup(func() {
		if held {
			cache.Release(1)
		}
	})
	variant := getReadBuf(false)
	if variant.Buf == nil {
		t.Fatal("cannot borrow the adopted variant")
	}
	for i := range variant.Buf {
		variant.Buf[i] = 0x6b
	}
	update.adoptFwdHandle(variant)
	wantReceipt := bytes.Clone(receipt.Buf)
	cache.abortPublication(1)

	// This later fully-acked entry puts id 1 behind the processing frontier,
	// so an ordinary retained entry would be eligible for the pressured valve.
	later := &ReceivedUpdate{WireUpdate: wireu.NewWireUpdate(bytes.Clone(body), 0)}
	later.WireUpdate.SetMessageID(2)
	cache.Add(later)
	cache.Activate(2, 0)
	clock.Add(2 * time.Minute)
	cache.Activate(1, 3)
	if cache.Delete(1) {
		t.Error("explicit expiry revoked an aborted retained borrow")
	}
	cache.runGapScan()
	if cache.Contains(1) {
		t.Error("late activation reopened aborted lookup admission")
	}
	if cache.Len() != 1 {
		t.Errorf("lifecycle controls left %d physical entries, want the retained tombstone", cache.Len())
	}
	_, used := combinedBufMuxStats()
	if used != baseline+2*4096 {
		t.Errorf("lifecycle controls hold %d bytes, want %d", used, baseline+2*4096)
	}
	if !bytes.Equal(receipt.Buf, wantReceipt) {
		t.Error("lifecycle controls returned or changed the retained receipt")
	}
	if !bytes.Equal(variant.Buf, bytes.Repeat([]byte{0x6b}, 4096)) {
		t.Error("lifecycle controls returned or changed the adopted variant")
	}
	cache.Release(1)
	held = false
	if cache.Len() != 0 {
		t.Errorf("final Release left %d physical entries", cache.Len())
	}
	_, used = combinedBufMuxStats()
	if used != baseline {
		t.Errorf("final Release holds %d bytes, started with %d", used, baseline)
	}
}
