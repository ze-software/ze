package testqemu

import (
	"context"
	"strings"
	"testing"
	"time"
)

// VALIDATES: G-2 for the QEMU runs: a host command whose context was canceled
// (a Ctrl-C reaching the run's signal root) is reported as canceled, and only
// a command that outlived its deadline is reported as having exceeded it.
// METHOD: run `sleep 10` under a context canceled after it starts, then under
// one whose deadline passes, and read each error's words.
// PREVENTS: the 2026-10-19 real Ctrl-C of `./le test qemu docker-lab` printing
// "command ssh exceeded its deadline: context canceled", which names a timeout
// the operator never hit instead of the interrupt they sent.
func TestHostCommandNamesCancelAndDeadlineApart(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	_, err := runHostCommand(canceled, commandSpec{Name: "sleep", Args: []string{"10"}})
	if err == nil {
		t.Fatal("a canceled command answered no error")
	}
	if strings.Contains(err.Error(), "deadline") {
		t.Errorf("a canceled command was reported as a deadline: %v", err)
	}
	if !strings.Contains(err.Error(), "sleep was canceled") {
		t.Errorf("a canceled command does not say it was canceled: %v", err)
	}

	expired, stop := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer stop()
	_, err = runHostCommand(expired, commandSpec{Name: "sleep", Args: []string{"10"}})
	if err == nil {
		t.Fatal("a command past its deadline answered no error")
	}
	if !strings.Contains(err.Error(), "sleep exceeded its deadline") {
		t.Errorf("a command past its deadline does not say so: %v", err)
	}
}
