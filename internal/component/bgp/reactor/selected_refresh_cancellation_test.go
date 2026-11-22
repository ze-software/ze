// Design: docs/architecture/update-building.md -- joined exact marker writes.
package reactor

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// Request cancellation and process-stop cancellation both join a real blocked
// socket write. Neither may return success or retain the destination write path.
func TestSelectedRefreshCancellationJoinsFinalWrite(t *testing.T) {
	for _, stopProcess := range []bool{false, true} {
		t.Run(map[bool]string{false: "request", true: "process"}[stopProcess], func(t *testing.T) {
			fixture, input := selectedRefreshFixture(t)
			original := fixture.owner
			ownerContext, stopOwner := context.WithCancel(t.Context())
			defer stopOwner()
			ended := make(chan struct{})
			defer close(ended)
			owner, err := plugin.NewSelectedOwner(original.Sender(), ended, nil)
			require.NoError(t, err)
			fixture.owner, err = owner.WithConnection(original.CurrentSelection, ownerContext)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(t.Context())
			left, right := net.Pipe()
			var done chan error
			joined := false
			t.Cleanup(func() {
				cancel()
				stopOwner()
				if err := left.Close(); err != nil {
					t.Error(err)
				}
				if err := right.Close(); err != nil {
					t.Error(err)
				}
				if done == nil {
					return
				}
				if !joined {
					select {
					case <-done: //nolint:errcheck // test cleanup joins the canceled marker goroutine
					case <-time.After(5 * time.Second):
						t.Error("marker caller did not join cleanup")
					}
				}
			})
			blocked := &observedWriteConn{Conn: left, writing: make(chan struct{})}
			session := fixture.peer.session
			session.conn = blocked
			writer := &observedBGPWriter{writer: blocked, session: session}
			writer.context.Store(uint32(session.sendCtxID))
			session.wireWriter = writer
			session.bufWriter = bufio.NewWriter(writer)
			request, err := plugin.NewSelectedRefresh(input)
			require.NoError(t, err)
			done = make(chan error, 1)
			go func() {
				_, err := fixture.api.SelectRefresh(ctx, request, fixture.owner)
				done <- err
			}()
			select {
			case <-blocked.writing:
			case err := <-done:
				joined = true
				t.Fatalf("marker returned before its blocked final write: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("marker did not enter the final socket write")
			}
			if stopProcess {
				stopOwner()
			} else {
				cancel()
			}
			select {
			case err := <-done:
				joined = true
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("canceled marker did not join its final write")
			}
			if stopProcess {
				require.NoError(t, ctx.Err(), "process cancellation must be independent of the caller")
			} else {
				require.NoError(t, ownerContext.Err(), "request cancellation must leave the selecting process live")
			}
			if session.writeMu.TryLock() {
				session.writeMu.Unlock()
			} else {
				t.Fatal("canceled marker retained the destination write path")
			}
		})
	}
}
