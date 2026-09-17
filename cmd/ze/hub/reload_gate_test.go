package hub

import (
	"sync"
	"testing"
	"time"
)

// VALIDATES: shutdown waits for a commit-driven reload before the store is
// closed, and refuses a reload that arrives after shutdown began. This is the
// order api-peer-save and concurrent-config-commit depend on: a reload that
// outlives Run promotes its candidate into a closed store.
func TestReloadGateWaitsForAdmittedReloadAndRefusesLater(t *testing.T) {
	gate := &reloadGate{}
	if !gate.enter() {
		t.Fatal("an open gate must admit a reload")
	}

	closed := make(chan struct{})
	var once sync.Once
	go func() {
		gate.close()
		once.Do(func() { close(closed) })
	}()

	select {
	case <-closed:
		t.Fatal("close returned while a reload was still admitted")
	case <-time.After(50 * time.Millisecond):
	}
	if gate.enter() {
		t.Fatal("a closing gate must refuse a new reload")
	}

	gate.leave()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close did not return after the admitted reload left")
	}
	if gate.enter() {
		t.Fatal("a closed gate must refuse a new reload")
	}
}
