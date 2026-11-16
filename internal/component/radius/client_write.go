// Design: docs/research/l2tpv2-ze-integration.md -- cancellable bounded RADIUS requests
// Related: client.go -- request encoding/retry; client_observed.go -- actual write outcome
package radius

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// resolveRequestAddress preserves UDP4 destination selection while making DNS
// part of the request's existing context, rather than an unbounded predecessor
// to its retry budget. Numeric destinations take no resolver operation.
func resolveRequestAddress(ctx context.Context, address string) (*net.UDPAddr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	host, service, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	port, err := net.DefaultResolver.LookupPort(ctx, "udp", service)
	if err != nil {
		return nil, err
	}
	if host == "" {
		return &net.UDPAddr{Port: port}, nil
	}
	if parsed := net.ParseIP(host); parsed != nil {
		if ipv4 := parsed.To4(); ipv4 != nil {
			return &net.UDPAddr{IP: ipv4, Port: port}, nil
		}
		return nil, fmt.Errorf("radius: UDP4 server %s has no IPv4 destination", address)
	}
	answers, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, answer := range answers {
		if ipv4 := answer.IP.To4(); ipv4 != nil {
			return &net.UDPAddr{IP: ipv4, Port: port}, nil
		}
	}
	return nil, fmt.Errorf("radius: server %s resolved without an IPv4 destination", address)
}

// writeRequest checks cancellation again at the actual serialized socket write.
// Every writer installs its own finite deadline and clears it while still under
// c.mu, so a queued request cannot inherit another request's socket deadline.
// The observer runs only after a real write attempt and outside that mutex.
func (c *Client) writeRequest(ctx context.Context, data []byte, address *net.UDPAddr, observe func(RequestWrite)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return errors.New("radius: client closed")
	}
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return err
	}
	deadline := time.Now().Add(c.config.Timeout)
	if bound, ok := ctx.Deadline(); ok {
		if bound.Before(deadline) {
			deadline = bound
		}
	}
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		c.mu.Unlock()
		return err
	}
	written, writeErr := c.conn.WriteToUDP(data, address)
	if writeErr == nil {
		if written != len(data) {
			writeErr = io.ErrShortWrite
		}
	}
	clearErr := c.conn.SetWriteDeadline(time.Time{})
	c.mu.Unlock()
	if observe != nil {
		observe(RequestWrite{server: address.String(), bytes: written, failure: writeErr})
	}
	return errors.Join(writeErr, clearErr)
}
