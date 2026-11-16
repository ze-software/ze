// Design: docs/architecture/api/ipc_protocol.md -- cancellation of a partial request frame.
// Related: conn.go -- the write mutex spans this hook and its cancellation join.
package rpc

import (
	"context"
	"sync/atomic"
)

type cancellableWriteKey struct{}

type cancellableWrite struct{ begun atomic.Bool }

// armWriteCancellation runs only after Conn.mu is held. A cancellation while
// this frame is being written closes the stream: its partial line cannot be
// recovered. The writer MUST invoke the returned join before releasing mu, so
// an old callback cannot close a subsequent caller's connection.
func (c *Conn) armWriteCancellation(ctx context.Context) (func(), error) {
	write, marked := ctx.Value(cancellableWriteKey{}).(*cancellableWrite)
	if !marked {
		return func() {}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	write.begun.Store(true)
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = c.readCloser.Close()  //nolint:errcheck // Cancellation ends both halves; the write reports its cause.
		_ = c.writeCloser.Close() //nolint:errcheck // A partial request stream is deliberately unusable.
		close(done)
	})
	return func() {
		if !stop() {
			<-done
		}
	}, nil
}
