// Design: docs/architecture/core-design.md -- the write deadline of a refusal on a raw connection
// Related: peer_admin.go -- rejectAdminDownConnection
// Related: reactor_connection.go -- rejectConnectionCollision, the bounded sibling

package reactor

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/sim"
)

// TestAdminDownRefusalIsBounded proves the refusal of a connection to an
// administratively held peer cannot block on a peer that reads nothing. The
// connection is a pipe whose other end is never read, so the NOTIFICATION
// write blocks. The clock handed in sits just under the 30-second bound in the
// past, which leaves the write 300 ms of real time, since a pipe judges its
// deadline against real time.
//
// VALIDATES: AC-12 -- the refusal returns once its write deadline has passed
// and the connection is closed afterwards.
// PREVENTS: the goroutine that serves the administrative hold parked for good
// by one peer that connects and reads nothing.
func TestAdminDownRefusalIsBounded(t *testing.T) {
	const left = 300 * time.Millisecond

	server, client := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	clk := sim.NewFakeClock(time.Now().Add(left - fwdWriteDeadlineDefault))

	started := time.Now()
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		rejectAdminDownConnection(server, clk)
	}()

	select {
	case <-returned:
		require.GreaterOrEqual(t, time.Since(started), left/2,
			"precondition: the refusal never blocked on the unread connection, so no bound was waited out")
	case <-time.After(left + wedgeSlack):
		t.Fatal("the refusal was still blocked " + (left + wedgeSlack).String() +
			" after it started: its write carries no deadline")
	}

	_, err := client.Read(make([]byte, 1))
	require.True(t, errors.Is(err, io.EOF), "the refused connection must be closed afterwards, got %v", err)
}
