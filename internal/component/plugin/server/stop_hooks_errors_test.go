// Design: docs/architecture/api/commands.md -- daemon-owned work reports cleanup failures
// Related: stop_hooks_test.go -- withdrawal and once-only shutdown ownership

package server

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// errorStopOwner keeps the missing owner contract observable before its
// implementation exists, without breaking compilation of the old server.
type errorStopOwner interface {
	OnStop(func() error) (func(), error)
}

func requireErrorStopOwner(t *testing.T, s *Server) errorStopOwner {
	t.Helper()
	owner, ok := any(s).(errorStopOwner)
	if !ok {
		t.Fatal("the daemon stop owner cannot accept cleanup failures: OnStop must accept func() error")
	}
	return owner
}

// TestServerStopPreservesAllCleanupFailures proves daemon-owned resources can
// report every cleanup failure alongside an existing fatal plugin cause.
// Method: register two failing hooks, a successful hook and a withdrawn hook
// on the actual Server, then inspect StopFailure after two stops.
func TestServerStopPreservesAllCleanupFailures(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := requireErrorStopOwner(t, s)
	pluginCause := errors.New("plugin failed before shutdown")
	pluginFailure := opererr.Wrap(pluginCause, "a plugin stopped the daemon", "its work stopped", "ze doctor")
	s.stopFailure.Store(pluginFailure)
	first := errors.New("first resource cleanup failed")
	second := errors.New("second resource cleanup failed")
	withdrawn := errors.New("withdrawn resource cleanup failed")
	unrelated := errors.New("unrelated resource cleanup failed")
	var firstRuns, secondRuns, successRuns, withdrawnRuns int
	if _, err := owner.OnStop(func() error {
		firstRuns++
		return first
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.OnStop(func() error {
		secondRuns++
		return second
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.OnStop(func() error {
		successRuns++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	withdraw, err := owner.OnStop(func() error {
		withdrawnRuns++
		return withdrawn
	})
	if err != nil {
		t.Fatal(err)
	}
	withdraw()
	s.Stop()
	s.Stop()
	failure := s.StopFailure()
	if failure == nil {
		t.Fatal("cleanup failures were reported as a clean daemon exit")
	}
	for _, cause := range []error{pluginFailure, pluginCause, first, second} {
		if !errors.Is(failure, cause) {
			t.Errorf("StopFailure lost %v: %v", cause, failure)
		}
	}
	for _, cause := range []error{withdrawn, unrelated} {
		if errors.Is(failure, cause) {
			t.Errorf("StopFailure retained a resource failure that did not occur: %v", cause)
		}
	}
	if firstRuns != 1 {
		t.Errorf("first failing hook ran %d times, want 1", firstRuns)
	}
	if secondRuns != 1 {
		t.Errorf("second failing hook ran %d times, want 1", secondRuns)
	}
	if successRuns != 1 {
		t.Errorf("successful hook ran %d times, want 1", successRuns)
	}
	if withdrawnRuns != 0 {
		t.Errorf("withdrawn hook ran %d times, want 0", withdrawnRuns)
	}
	if failure.Fix() == "" {
		t.Error("cleanup failure has no operator recovery command")
	}
	late, err := owner.OnStop(func() error {
		t.Error("late cleanup hook ran")
		return nil
	})
	if !errors.Is(err, errServerStopped) {
		t.Errorf("late registration = %v, want errServerStopped", err)
	}
	if late != nil {
		t.Error("refused late cleanup hook received a withdrawal")
	}
}

// TestServerStopSuccessfulCleanupIsClean proves successful hooks do not make
// an ordinary daemon stop fail. Method: run a nil-returning owner on Server.
func TestServerStopSuccessfulCleanupIsClean(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := requireErrorStopOwner(t, s)
	ran := false
	if _, err := owner.OnStop(func() error {
		ran = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if !ran {
		t.Error("successful cleanup did not run")
	}
	if failure := s.StopFailure(); failure != nil {
		t.Errorf("successful cleanup made the daemon stop fail: %v", failure)
	}
}

// TestServerStopCleanupFailureWithoutPluginCause proves a resource cleanup
// failure alone changes the daemon exit result, with its identity retained.
func TestServerStopCleanupFailureWithoutPluginCause(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := requireErrorStopOwner(t, s)
	cause := errors.New("resource cleanup failed")
	if _, err := owner.OnStop(func() error { return cause }); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	failure := s.StopFailure()
	if failure == nil {
		t.Fatal("resource cleanup failure was reported as a clean daemon exit")
	}
	if !errors.Is(failure, cause) {
		t.Errorf("StopFailure = %v, want cleanup cause %v", failure, cause)
	}
}

// TestServerStopSuccessfulCleanupPreservesPluginFailure proves a successful
// cleanup keeps the original fatal-plugin diagnosis intact, without wrapping it.
func TestServerStopSuccessfulCleanupPreservesPluginFailure(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := requireErrorStopOwner(t, s)
	failure := opererr.New("a plugin stopped the daemon", "its work stopped", "ze doctor")
	s.stopFailure.Store(failure)
	if _, err := owner.OnStop(func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if got := s.StopFailure(); got != failure {
		t.Errorf("successful cleanup replaced plugin diagnosis: got %v, want %v", got, failure)
	}
}

// TestServerStopRetainsPluginFailureRecordedAfterCleanup proves the shared
// recorder keeps a fatal plugin diagnosis that completes after cleanup.
// Method: stop with a failing hook, then record the already elected plugin
// cause through the same producer stopDaemonForPlugin uses.
func TestServerStopRetainsPluginFailureRecordedAfterCleanup(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cleanupCause := errors.New("cleanup failed before plugin diagnosis was recorded")
	if _, err := s.OnStop(func() error { return cleanupCause }); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	pluginCause := errors.New("elected fatal plugin cause")
	pluginFailure := opererr.Wrap(pluginCause, "a plugin stopped the daemon", "its work stopped", "ze doctor")
	s.recordStopFailure(pluginFailure)
	failure := s.StopFailure()
	if failure == nil {
		t.Fatal("recorded stop failures were lost")
	}
	for _, cause := range []error{cleanupCause, pluginFailure, pluginCause} {
		if !errors.Is(failure, cause) {
			t.Errorf("StopFailure lost %v: %v", cause, failure)
		}
	}
}
