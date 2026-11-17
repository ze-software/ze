// Design: docs/features/interfaces.md -- exact active backend selection
// Related: backend.go -- RequireActiveBackend observes name and owner atomically

package iface

import "testing"

// TestRequireActiveBackend proves exact identity returns the existing backend;
// an absent or mismatched backend refuses without loading a substitute.
func TestRequireActiveBackend(t *testing.T) {
	backendsMu.Lock()
	previous, previousName := activeBackend, activeBackendName
	backendsMu.Unlock()
	t.Cleanup(func() {
		backendsMu.Lock()
		activeBackend, activeBackendName = previous, previousName
		backendsMu.Unlock()
	})
	want := &fakeBackend{}
	for _, state := range []struct {
		name     string
		backend  Backend
		accepted bool
	}{
		{name: "test-required", backend: want, accepted: true},
		{name: "test-other", backend: want},
		{name: "test-required"},
		{},
	} {
		backendsMu.Lock()
		activeBackend, activeBackendName = state.backend, state.name
		backendsMu.Unlock()
		got, err := RequireActiveBackend("test-required")
		if state.accepted {
			if err != nil || got != want {
				t.Fatalf("exact selection: backend=%v error=%v", got, err)
			}
			continue
		}
		if err == nil || got != nil {
			t.Fatalf("mismatched selection: backend=%v error=%v", got, err)
		}
	}
}
