package interoplab

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// VALIDATES: RemovalNamesCleanup answers true for exactly the two removal
// errors whose text already ends with the cleanup command, also when wrapped,
// and false for any other removal error and for nil.
// PREVENTS: a caller that adds its own fix line to an error that carries one,
// or that drops the fix from an error that does not.
func TestRemovalNamesCleanup(t *testing.T) {
	busy := errors.New("container x: device or resource busy")
	for _, row := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "settling", err: &settlingRemovalError{name: "x", err: busy, appeared: true}, want: true},
		{name: "pending", err: &pendingRemovalError{name: "x", removal: busy, stop: context.Canceled}, want: true},
		{name: "wrapped settling", err: fmt.Errorf("remove: %w", &settlingRemovalError{name: "x", err: busy}), want: true},
		{name: "plain", err: busy, want: false},
		{name: "nil", err: nil, want: false},
	} {
		if got := RemovalNamesCleanup(row.err); got != row.want {
			t.Errorf("%s: RemovalNamesCleanup = %v, want %v", row.name, got, row.want)
		}
	}
}
