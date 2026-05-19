// Design: docs/guide/command-catalogue.md -- NTP diagnostics state

package ntp

import (
	"sync/atomic"
	"time"
)

// serverState holds per-server NTP query results.
type serverState struct {
	Address        string        `json:"address"`
	Offset         time.Duration `json:"offset"`
	RTT            time.Duration `json:"rtt"`
	Stratum        uint8         `json:"stratum"`
	RootDelay      time.Duration `json:"root-delay"`
	RootDispersion time.Duration `json:"root-dispersion"`
	Reach          uint8         `json:"reach"`
	LastQuery      time.Time     `json:"last-query"`
	LastSuccess    time.Time     `json:"last-success"`
	LastError      string        `json:"last-error"`
}

// syncState is a snapshot of the NTP subsystem state, published atomically
// by the sync worker and read by show handlers without locking.
type syncState struct {
	Enabled      bool          `json:"enabled"`
	Synced       bool          `json:"synced"`
	Source       string        `json:"source"`
	Offset       time.Duration `json:"offset"`
	Stratum      uint8         `json:"stratum"`
	PollInterval int           `json:"poll-interval"`
	LastSync     time.Time     `json:"last-sync"`
	Servers      []serverState `json:"servers"`
}

// globalState is the package-level sync state, written by the sync worker
// and read by show RPC handlers. The pointer swap is atomic; readers get
// a consistent snapshot without blocking the writer.
var globalState atomic.Pointer[syncState]

// loadState returns the current sync state snapshot (nil if NTP has not started).
func loadState() *syncState {
	return globalState.Load()
}

// storeState publishes a new sync state snapshot.
func storeState(s *syncState) {
	globalState.Store(s)
}

// reachShift updates the reach register: shift left, set bit 0 if success.
// RFC 5905 Section 13.1: reachability register is an 8-bit shift register.
func reachShift(reach uint8, success bool) uint8 {
	reach <<= 1
	if success {
		reach |= 1
	}
	return reach
}

// selectBestServer picks the best server from the state: reachable, lowest
// stratum, smallest absolute offset as tiebreaker. Returns nil if no server
// is reachable.
func selectBestServer(servers []serverState) *serverState {
	var best *serverState
	for i := range servers {
		s := &servers[i]
		if s.Reach == 0 {
			continue
		}
		if best == nil {
			best = s
			continue
		}
		if s.Stratum < best.Stratum {
			best = s
			continue
		}
		if s.Stratum == best.Stratum && absDuration(s.Offset) < absDuration(best.Offset) {
			best = s
		}
	}
	return best
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
