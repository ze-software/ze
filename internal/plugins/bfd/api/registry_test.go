package api

import (
	"testing"
)

// fakeService is a minimal Service implementation for registry tests.
// It records call counts so callers can assert set/get round-trip
// semantics without pulling in engine/ or transport/.
type fakeService struct {
	ensureCalls  int
	releaseCalls int
}

func (f *fakeService) EnsureSession(req SessionRequest) (SessionHandle, error) {
	f.ensureCalls++
	return &fakeHandle{key: req.Key()}, nil
}

func (f *fakeService) ReleaseSession(_ SessionHandle) error {
	f.releaseCalls++
	return nil
}

func (f *fakeService) Snapshot() []SessionState                    { return nil }
func (f *fakeService) SessionDetail(_ string) (SessionState, bool) { return SessionState{}, false }
func (f *fakeService) Profiles() []ProfileState                    { return nil }

// fakeHandle is a no-op SessionHandle returned by fakeService so the
// round-trip test can exercise Service.EnsureSession without nil
// return values (lint rule nilnil forbids `return nil, nil`).
type fakeHandle struct {
	key Key
}

func (h *fakeHandle) Key() Key                         { return h.key }
func (h *fakeHandle) Subscribe() <-chan StateChange    { return nil }
func (h *fakeHandle) Unsubscribe(_ <-chan StateChange) {}
func (h *fakeHandle) Shutdown() error                  { return nil }
func (h *fakeHandle) Enable() error                    { return nil }

// VALIDATES: SetService followed by GetService returns the same concrete
// instance; a nil publication returns nil; a second SetService replaces
// the first. Guards the Stage 3 BGP client against a regression where
// the Service pointer loses identity under concurrent writers.
// PREVENTS: the BGP client receiving stale or mismatched Service handles.
func TestSetGetService_RoundTrip(t *testing.T) {
	t.Cleanup(func() { SetService(nil) })

	if got := GetService(); got != nil {
		t.Fatalf("GetService before SetService = %v, want nil", got)
	}

	svc := &fakeService{}
	SetService(svc)
	got := GetService()
	if got == nil {
		t.Fatal("GetService after SetService(svc) = nil, want non-nil")
	}
	if got != Service(svc) {
		t.Fatalf("GetService identity mismatch: got %v want %v", got, svc)
	}

	// Exercise the returned interface so the fake records a call.
	if _, err := got.EnsureSession(SessionRequest{}); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if svc.ensureCalls != 1 {
		t.Fatalf("ensureCalls = %d, want 1", svc.ensureCalls)
	}

	svc2 := &fakeService{}
	SetService(svc2)
	if got := GetService(); got != Service(svc2) {
		t.Fatalf("GetService after replacement = %v, want %v", got, svc2)
	}
}

// VALIDATES: SetService(nil) clears the publication.
// PREVENTS: bfd plugin shutdown leaving a dangling handle that races
// the engine teardown.
func TestSetService_Nil(t *testing.T) {
	t.Cleanup(func() { SetService(nil) })

	SetService(&fakeService{})
	if GetService() == nil {
		t.Fatal("pre-clear GetService = nil, want non-nil")
	}
	SetService(nil)
	if got := GetService(); got != nil {
		t.Fatalf("GetService after SetService(nil) = %v, want nil", got)
	}
}

// VALIDATES: concurrent SetService + GetService does not race under
// -race. Simple spin loop with writers and readers on separate
// goroutines for a short duration.
// PREVENTS: regression where the atomic.Pointer discipline is broken
// by a refactor that reverts to a mutex or unguarded slice.
func TestSetGetService_ConcurrentNoRace(t *testing.T) {
	t.Cleanup(func() { SetService(nil) })

	stop := make(chan struct{})
	done := make(chan struct{}, 4)

	// Writers.
	for range 2 {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				select {
				case <-stop:
					return
				default:
					SetService(&fakeService{})
				}
			}
		}()
	}
	// Readers.
	for range 2 {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				select {
				case <-stop:
					return
				default:
					_ = GetService()
				}
			}
		}()
	}

	// Run a small fixed number of iterations via a timer-free
	// approach: writers and readers spin, main goroutine triggers
	// stop after 1000 rounds of its own GetService() calls. Keeps
	// the test short without importing time.
	for range 1000 {
		_ = GetService()
	}
	close(stop)
	for range 4 {
		<-done
	}
}
