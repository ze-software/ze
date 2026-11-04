// Design: docs/architecture/chaos-web-dashboard.md -- deterministic active-set decisions.
package web

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// TestActiveSetEvictionBreaksClockTiesByPeerIndex exercises real promotion at
// capacity with tied activity times and different insertion orders. Pinned peers
// remain protected, and older activity still outranks the index tie-break.
func TestActiveSetEvictionBreaksClockTiesByPeerIndex(t *testing.T) {
	at := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	for rotation := range 10 {
		for range 20 {
			active := newActiveSet(10)
			for offset := range 10 {
				active.Promote((offset+rotation)%10, PriorityMedium, at)
			}
			active.Pin(0, at)
			if !active.Promote(10, PriorityHigh, at.Add(time.Second)) {
				t.Fatal("promotion at capacity refused")
			}
			if active.Contains(1) {
				t.Fatalf("rotation %d: tied eviction kept lowest unpinned peer 1; members %v", rotation, active.Indices())
			}
			if !active.Contains(0) {
				t.Fatal("tied eviction removed pinned peer 0")
			}
			active.Entry(9).LastActive = at.Add(-time.Second)
			if !active.Promote(11, PriorityHigh, at.Add(2*time.Second)) {
				t.Fatal("second promotion at capacity refused")
			}
			if active.Contains(9) {
				t.Fatal("index tie-break displaced an older activity timestamp")
			}
		}
	}
}

// TestDashboardBroadcastOrdersPeerBatches checks the producer's complete frame
// sequence, not only final DOM membership, for promoted, updated and decayed peers.
func TestDashboardBroadcastOrdersPeerBatches(t *testing.T) {
	for rotation := range 10 {
		for _, phase := range []string{"promote", "update", "decay"} {
			d := newTestDashboard(24)
			d.state.Active = newActiveSet(10)
			d.state.dirtyGlobal = true
			client := d.broker.Subscribe()
			now := time.Now()
			var want []string
			for offset := range 10 {
				idx := (offset + rotation) % 10
				d.state.Active.Promote(idx, PriorityMedium, now)
				switch phase {
				case "promote":
					d.state.newlyPromoted[idx] = true
				case "update":
					d.state.dirtyPeers[idx] = true
				case "decay":
					d.state.Active.Entry(idx).LastActive = now.Add(-time.Hour)
				}
			}
			for idx := range 10 {
				switch phase {
				case "promote":
					want = append(want, renderPeerRemoval(idx), d.renderPeerRowInsert(idx))
				case "update":
					want = append(want, d.renderPeerRow(idx))
				case "decay":
					want = append(want, renderPeerRemoval(idx))
				}
			}
			d.broadcastDirty(false)
			var got []string
			for len(client.ch) > 0 {
				message := <-client.ch
				if strings.Contains(message.Data, `id="stats"`) {
					continue
				}
				if strings.Contains(message.Data, `id="events"`) {
					continue
				}
				got = append(got, message.Data)
			}
			d.broker.Unsubscribe(client)
			d.broker.Close()
			if !slices.Equal(got, want) {
				t.Fatalf("rotation %d %s: SSE peer frames are not in ascending peer-index order", rotation, phase)
			}
		}
	}
}
