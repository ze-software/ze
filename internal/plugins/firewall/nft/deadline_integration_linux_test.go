//go:build integration && linux

package firewallnft

import (
	"testing"
	"time"

	"github.com/mdlayher/netlink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestNftApplyDeadlineSurfacesError proves the deadline reaches a real netlink
// socket: a connection carrying the SockOption returns a deadline error from a
// blocking receive instead of hanging.
//
// VALIDATES: fixit-firewall-concurrency-deadlock AC-10 -- a kernel that never
// acks yields a timeout error within the deadline rather than blocking.
//
// PREVENTS: the shipped behaviour where firewall.ApplyAll held the
// process-wide reconcileMu across an unbounded Flush, so one wedged kernel
// stalled every firewall owner in the process, indefinitely.
func TestNftApplyDeadlineSurfacesError(t *testing.T) {
	conn, err := netlink.Dial(unix.NETLINK_NETFILTER, nil)
	if err != nil {
		t.Skipf("netlink unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// The production SockOption, applied to a real socket.
	require.NoError(t, netlinkDeadlineOption(50*time.Millisecond)(conn))

	start := time.Now()
	_, err = conn.Receive() // nothing was sent, so nothing will ever be acked
	elapsed := time.Since(start)

	require.Error(t, err, "a receive with no pending reply must not block forever")
	assert.ErrorIs(t, asKernelTimeout(err), firewall.ErrKernelTimeout,
		"the blocked kernel call must surface as ErrKernelTimeout")
	assert.Less(t, elapsed, 5*time.Second,
		"the deadline must bound the call, not merely decorate the error")
}
