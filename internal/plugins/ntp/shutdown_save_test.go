// Design: docs/features/interfaces.md -- the saved clock survives a clean NTP stop.
// Related: register.go -- final save failure is a failed shutdown after cleanup.

package ntp

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// ntpStopStore uses the real statestore write boundary and supplies its storage
// outcome. No other storage operation is reached by a final save.
type ntpStopStore struct {
	statestore.Storage
	cause error
	keys  []string
}

func (s *ntpStopStore) WriteKey(key string, _ []byte) error {
	s.keys = append(s.keys, key)
	return s.cause
}

// TestNTPFinalSaveFailureStillStopsWorker preserves the write cause and recovery
// while proving the worker is stopped on both the failed and successful rail.
func TestNTPFinalSaveFailureStillStopsWorker(t *testing.T) {
	previous := statestore.Store()
	t.Cleanup(func() { statestore.SetStore(previous) })
	cause := errors.New("managed store is read-only")
	for _, current := range []struct {
		name        string
		persistPath string
		cause       error
		wantWrites  int
	}{
		{name: "write refused", persistPath: "/perm/ze/timefile", cause: cause, wantWrites: 1},
		{name: "write accepted", persistPath: "/perm/ze/timefile", wantWrites: 1},
		{name: "persistence disabled"},
	} {
		t.Run(current.name, func(t *testing.T) {
			store := &ntpStopStore{cause: current.cause}
			statestore.SetStore(store)
			worker := newSyncWorker(ntpConfig{PersistPath: current.persistPath}, nil)
			// A completed worker still requires the stop signal and join protocol.
			// This fixture starts no synchronization or clock adjustment loop.
			close(worker.done)
			refusal := finishNTPWorker(worker)
			if current.cause == nil {
				if refusal != nil {
					t.Fatalf("successful shutdown refused: %v", refusal)
				}
			} else {
				if refusal == nil || !errors.Is(refusal, cause) {
					t.Fatalf("shutdown refusal=%v, want original managed-store cause", refusal)
				}
				if refusal.Fix() != "ze show doctor" {
					t.Errorf("shutdown recovery=%q, want ze show doctor", refusal.Fix())
				}
			}
			select {
			case <-worker.stop:
			default:
				t.Error("shutdown did not stop the worker")
			}
			if len(store.keys) != current.wantWrites {
				t.Fatalf("managed-store writes=%v, want %d", store.keys, current.wantWrites)
			}
			if current.wantWrites == 1 && store.keys[0] != zefs.KeyNTPLastTime.Pattern {
				t.Errorf("saved key=%q, want NTP last-time key", store.keys[0])
			}
		})
	}
	if refusal := finishNTPWorker(nil); refusal != nil {
		t.Errorf("no worker needs no final save: %v", refusal)
	}
}
