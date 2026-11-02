package rs

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSessionEndedClassifiesEngineClose proves the shutdown classification a
// forked bgp-rs needs. Goal: an engine call that fails because the engine
// closed the connection (daemon stop, which sends no `bye`) is shutdown and
// not an ERROR, while any other failure still is one. Method: feed
// sessionEnded the exact error shapes the mux returns (its reader's EOF
// wrapped with %w, and ErrMuxConnClosed) and a plain failure, with stopping
// both false and true.
func TestSessionEndedClassifiesEngineClose(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		stopping bool
		want     bool
	}{
		{"mux reader EOF", fmt.Errorf("mux conn read error: %w", io.EOF), false, true},
		{"mux closed", rpc.ErrMuxConnClosed, false, true},
		{"plain failure", errors.New("unknown update id"), false, false},
		{"plain failure after Run returned", errors.New("unknown update id"), true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := &routeServer{}
			rs.stopping.Store(tc.stopping)
			if got := rs.sessionEnded(tc.err); got != tc.want {
				t.Fatalf("sessionEnded(%v) with stopping=%v = %v, want %v", tc.err, tc.stopping, got, tc.want)
			}
		})
	}
}
