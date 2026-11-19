// Design: docs/architecture/plugin/rib-storage-design.md -- original framed RS fixtures.
package rs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"net"
	"sync"
	"testing"
	"time"
)

// rsIntegrationWire has one joined reader. Only real source-lease frames are
// answered in that reader; forward/release frames remain visible to the old
// whole questions and are answered by their explicit native assertions.
type rsIntegrationWire struct {
	conn     *rpc.Conn
	requests chan *rpc.Request
	joined   chan struct{}
	cancel   context.CancelFunc
	mu       sync.Mutex
	pending  map[uint64]*rpc.Request
}

func newRSIntegrationWire(t *testing.T, host net.Conn) *rsIntegrationWire {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	wire := &rsIntegrationWire{conn: rpc.NewConn(host, host), requests: make(chan *rpc.Request, 128), joined: make(chan struct{}), cancel: cancel, pending: make(map[uint64]*rpc.Request)}
	go func() {
		defer close(wire.joined)
		defer close(wire.requests)
		acquired := false
		for {
			request, err := wire.conn.ReadRequest(ctx)
			if err != nil {
				return
			}
			if request.Method == rpc.MethodSourceLease {
				var input rpc.SourceLeaseInput
				if err := json.Unmarshal(request.Params, &input); err != nil {
					t.Error(err)
					return
				}
				if err := input.Validate(time.Now()); err != nil {
					t.Error(err)
					return
				}
				if input.SourceReceipt != 301 || input.PeerOwner != 1 {
					t.Errorf("lease changed original source: %+v", input)
					return
				}
				switch input.Action {
				case rpc.SourceLeaseAcquire:
					acquired = true
				case rpc.SourceLeaseRetain:
					if !acquired {
						t.Error("retain arrived before original acquire")
						return
					}
				case rpc.SourceLeaseRelease:
					acquired = false
				default:
					t.Error("unexpected source lease action")
					return
				}
				if err := wire.conn.SendResult(ctx, request.ID, &rpc.SourceLeaseOutput{Action: input.Action, SourceReceipt: input.SourceReceipt, PeerOwner: input.PeerOwner}); err != nil {
					return
				}
				continue
			}
			wire.mu.Lock()
			wire.pending[request.ID] = request
			wire.mu.Unlock()
			select {
			case wire.requests <- request:
			case <-ctx.Done():
				return
			}
		}
	}()
	return wire
}

func (wire *rsIntegrationWire) ReadRequest(ctx context.Context) (*rpc.Request, error) {
	select {
	case request, ok := <-wire.requests:
		if !ok {
			return nil, fmt.Errorf("original framed engine reader ended")
		}
		return request, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (wire *rsIntegrationWire) SendResult(ctx context.Context, id uint64, data any) error {
	wire.mu.Lock()
	request := wire.pending[id]
	delete(wire.pending, id)
	wire.mu.Unlock()
	if request == nil {
		return fmt.Errorf("no original framed request %d", id)
	}
	if request.Method == rpc.MethodForwardCached {
		var input rpc.ForwardCachedInput
		if err := json.Unmarshal(request.Params, &input); err != nil {
			return err
		}
		data = &rpc.CachedWriteOutput{Completed: true, DeadlineMS: input.DeadlineMS}
	}
	return wire.conn.SendResult(ctx, id, data)
}
