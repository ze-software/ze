// Design: docs/features/interfaces.md -- offline clear verb for counters
// Related: clear.go -- the target parser under test.

package cli

import (
	"errors"
	"strings"
	"testing"
)

// TestClearCountersTarget proves the offline `ze interface clear counters`
// resets every interface only when all is typed. Method: feed the parser each
// target shape and compare the interface it answers, or the refusal.
//
// VALIDATES: bare refused with the fix named, all answers every interface
// (""), a name answers that interface, a second word is refused.
// PREVENTS: an early Enter on `ze interface clear counters` resetting every
// interface (explicit-all CLI rule, owner decision 2026-10-17).
func TestClearCountersTarget(t *testing.T) {
	name, err := clearCountersTarget(nil)
	if !errors.Is(err, errClearCountersNoTarget) {
		t.Fatalf("bare: got (%q, %v), want errClearCountersNoTarget", name, err)
	}
	if !strings.Contains(err.Error(), "ze interface clear counters all") {
		t.Errorf("bare refusal %q does not name the fix", err)
	}

	for _, tc := range []struct {
		rest []string
		want string
	}{
		{rest: []string{"all"}, want: ""},
		{rest: []string{"eth0"}, want: "eth0"},
	} {
		got, err := clearCountersTarget(tc.rest)
		if err != nil {
			t.Fatalf("%v: refused: %v", tc.rest, err)
		}
		if got != tc.want {
			t.Errorf("%v: got %q, want %q", tc.rest, got, tc.want)
		}
	}

	if _, err := clearCountersTarget([]string{"eth0", "eth1"}); !errors.Is(err, errClearCountersTooMany) {
		t.Errorf("two targets: got %v, want errClearCountersTooMany", err)
	}
}
