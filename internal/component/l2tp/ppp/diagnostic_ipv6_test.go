// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 diagnostic cause visibility

package ppp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
)

// Ordinary wrapped stop sentinels are quiet only after their owning context
// ends. A joined backend failure must remain visible through outer wrappers.
func TestNormalIPv6CarrierStopPreservesFailureVisibility(t *testing.T) {
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, cancelDeadline := context.WithTimeout(context.Background(), 0)
	defer cancelDeadline()
	const wrapperFormat = "carrier stopped: %w"
	backend := errors.New("carrier backend failed")
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{"live canceled", context.Background(), context.Canceled, false},
		{"live closed", context.Background(), net.ErrClosed, false},
		{"live pipe closed", context.Background(), io.ErrClosedPipe, false},
		{"nil cause", stopped, nil, false},
		{"canceled", stopped, context.Canceled, true},
		{"wrapped canceled", stopped, fmt.Errorf(wrapperFormat, context.Canceled), true},
		{"wrapped closed", stopped, fmt.Errorf(wrapperFormat, net.ErrClosed), true},
		{"wrapped pipe closed", stopped, fmt.Errorf(wrapperFormat, io.ErrClosedPipe), true},
		{"wrapped deadline", deadline, fmt.Errorf(wrapperFormat, context.DeadlineExceeded), true},
		{"different context cause", stopped, context.DeadlineExceeded, false},
		{"backend", stopped, backend, false},
		{"wrapped backend", stopped, fmt.Errorf(wrapperFormat, backend), false},
		{"joined canceled and backend", stopped, errors.Join(context.Canceled, backend), false},
		{"joined closed and backend", stopped, errors.Join(net.ErrClosed, backend), false},
		{"wrapped joined pipe and backend", stopped, fmt.Errorf(wrapperFormat, errors.Join(io.ErrClosedPipe, backend)), false},
		{"joined stop sentinels", stopped, errors.Join(context.Canceled, net.ErrClosed), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalIPv6CarrierStop(tc.ctx, tc.err); got != tc.want {
				t.Fatalf("normal carrier stop = %v, want %v for %v", got, tc.want, tc.err)
			}
		})
	}
}
