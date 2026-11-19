// Design: docs/research/vpp-deployment-reference.md -- owned connection test transport
package vpp

import (
	"context"
	"errors"
	"sync/atomic"

	"go.fd.io/govpp/api"
)

// recordingConnection is an initialized test transport: every operation is
// observable, and selecting a stream is an explicit refusal rather than a
// panic from an uninitialized GoVPP implementation.
type recordingConnection struct {
	disconnects atomic.Int32
	streams     atomic.Int32
	open        func(context.Context) (api.Stream, error)
}

func (*recordingConnection) NewAPIChannel() (api.Channel, error) {
	return nil, errors.New("recording connection has no native channel")
}
func (c *recordingConnection) NewStream(ctx context.Context, _ ...api.StreamOption) (api.Stream, error) {
	c.streams.Add(1)
	if c.open != nil {
		return c.open(ctx)
	}
	return nil, errors.New("recording connection stream selected")
}
func (c *recordingConnection) Disconnect() { c.disconnects.Add(1) }
