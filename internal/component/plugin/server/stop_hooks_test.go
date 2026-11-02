package server

import (
	"errors"
	"testing"
)

// TestServerStopRunsOnStopHooks proves OnStop's contract: Stop runs every hook
// still registered before it returns, a withdrawn hook does not run, a hook
// may withdraw itself from inside Stop without deadlocking, a second Stop runs
// nothing again, and OnStop refuses once Stop has started. Method: register
// counting hooks on a real Server, withdraw one, call Stop twice, read counts.
func TestServerStopRunsOnStopHooks(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	var ran, withdrawnRan, selfRan int
	if _, err := s.OnStop(func() { ran++ }); err != nil {
		t.Fatalf("OnStop: %v", err)
	}
	withdraw, err := s.OnStop(func() { withdrawnRan++ })
	if err != nil {
		t.Fatalf("OnStop: %v", err)
	}
	withdraw()
	var self func()
	self, err = s.OnStop(func() {
		selfRan++
		self()
	})
	if err != nil {
		t.Fatalf("OnStop: %v", err)
	}

	s.Stop()
	s.Stop()

	if ran != 1 {
		t.Errorf("a registered hook ran %d times across two Stops, want 1", ran)
	}
	if withdrawnRan != 0 {
		t.Errorf("a withdrawn hook ran %d times, want 0", withdrawnRan)
	}
	if selfRan != 1 {
		t.Errorf("a self-withdrawing hook ran %d times, want 1", selfRan)
	}

	late, err := s.OnStop(func() { t.Error("a hook offered after Stop ran") })
	if !errors.Is(err, errServerStopped) {
		t.Fatalf("OnStop after Stop: got %v, want errServerStopped", err)
	}
	if late != nil {
		t.Error("a refused OnStop answered a withdrawal")
	}
}
