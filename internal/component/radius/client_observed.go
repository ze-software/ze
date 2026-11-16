// Design: docs/research/l2tpv2-ze-integration.md -- observed RADIUS request writes
// Related: client.go -- the sole UDP request transport
package radius

import (
	"context"
	"errors"
)

// RequestWrite is the first actual UDP write outcome of one SendToServersObserved
// operation. Only the client's write path constructs it. Zero has no observed
// attempt and MUST NOT be read as a successful send. Safe to copy.
type RequestWrite struct {
	server  string
	bytes   int
	failure error
}

// Server is the destination of the observed write, empty only for a zero value.
func (write RequestWrite) Server() string { return write.server }

// Bytes is the number of bytes the real socket accepted, never a delivery ACK.
func (write RequestWrite) Bytes() int { return write.bytes }

// Err preserves an actual write failure. The zero receipt is explicitly unknown.
func (write RequestWrite) Err() error {
	if write.server == "" {
		return errors.New("radius: request has no observed write")
	}
	return write.failure
}

// SendToServersObserved preserves SendToServers' retry/failover behavior and
// synchronously observes its first actual UDP write outcome, once for the whole
// operation. A canceled/invalid request that never writes invokes no observer;
// the returned error names that refusal. The observer runs outside c.mu and
// MUST return promptly: it is part of the caller's bounded request operation.
// It reports a socket outcome, never server receipt or successful accounting.
func (c *Client) SendToServersObserved(ctx context.Context, pkt *Packet, observe func(RequestWrite)) (*Packet, error) {
	if ctx == nil {
		return nil, errors.New("radius: observed request has no context")
	}
	if observe == nil {
		return nil, errors.New("radius: observed request has no write observer")
	}
	var observed bool
	once := func(write RequestWrite) {
		if observed {
			return
		}
		observed = true
		observe(write)
	}
	return c.sendToServers(ctx, pkt, once)
}
