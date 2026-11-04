// Design: docs/plugin-development/protocol.md -- The filter-update batch.
package ipc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestCallFilterTransportAdmissionHonorsParent exercises the encoded branch's
// queue admission with a full queue and no worker. Cancellation and a deadline
// already expired must refuse the route without waiting for the transport's
// five-second limit or changing any queued request.
func TestCallFilterTransportAdmissionHonorsParent(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancellation"
		if deadline {
			name = "shorter deadline"
		}
		t.Run(name, func(t *testing.T) {
			conn := &PluginConn{}
			batcher := &conn.filters
			batcher.start.Do(func() {
				batcher.queue = make(chan *filterRequest, filterQueueDepth)
				for range filterQueueDepth {
					batcher.queue <- &filterRequest{}
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			want := context.Canceled
			if deadline {
				ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				want = context.DeadlineExceeded
			}
			route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "f"})
			out, err := conn.CallFilter(ctx, &route)
			if !errors.Is(err, want) || out != nil {
				t.Fatalf("admission verdict=%+v error=%v, want parent %v", out, err, want)
			}
			if len(batcher.queue) != filterQueueDepth {
				t.Fatalf("queue holds %d routes, want the original full queue", len(batcher.queue))
			}
		})
	}
}
