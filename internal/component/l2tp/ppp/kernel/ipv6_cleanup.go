// Design: docs/research/l2tpv2-ze-integration.md -- retained IPv6 socket cleanup
package kernel

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// This capability belongs only to the exact captured IPv6 socket provider.
// Generic Close errors and net.ErrClosed from opaque objects prove no release.
type ipv6SocketCleanupRetry interface{ RetryIPv6SocketCleanup(context.Context) error }

// The two fixed slots are this carrier's original owners, not a registry.
// Close preserves its first result forever; retry updates only failed slots.
type ipv6SocketCleanup struct {
	mu                       sync.Mutex
	ra, dhcp                 io.Closer
	started                  bool
	firstErr, raErr, dhcpErr error
}

func (c *ipv6SocketCleanup) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return c.firstErr
	}
	c.started = true
	if c.dhcp != nil {
		c.dhcpErr = c.dhcp.Close()
	}
	if c.ra != nil {
		c.raErr = c.ra.Close()
	}
	c.firstErr = errors.Join(c.dhcpErr, c.raErr)
	return c.firstErr
}
func (c *ipv6SocketCleanup) retry(ctx context.Context) error {
	if ctx == nil {
		return errors.New("IPv6 carrier cleanup requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.mu.TryLock() {
		return errors.New("IPv6 carrier cleanup is already running")
	}
	defer c.mu.Unlock()
	if !c.started {
		return errors.New("IPv6 carrier cleanup requires its first Close")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.dhcpErr != nil {
		if retry, ok := c.dhcp.(ipv6SocketCleanupRetry); ok {
			c.dhcpErr = retry.RetryIPv6SocketCleanup(ctx)
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(err, c.dhcpErr, c.raErr)
	}
	if c.raErr != nil {
		if retry, ok := c.ra.(ipv6SocketCleanupRetry); ok {
			c.raErr = retry.RetryIPv6SocketCleanup(ctx)
		}
	}
	return errors.Join(c.dhcpErr, c.raErr)
}

// A partial carrier owns cleanup only; it cannot borrow a nonexistent lane or
// publish service traffic. Zero/nil are invalid; only acquisition constructs it.
type partialIPv6Carrier struct {
	cleanup *ipv6SocketCleanup
	cause   error
}

func (c *partialIPv6Carrier) ReadRS(context.Context) error { return c.cause }
func (c *partialIPv6Carrier) WriteRA([]byte, int) error    { return c.cause }
func (c *partialIPv6Carrier) ReadDHCPv6(context.Context, []byte) (int, netip.AddrPort, error) {
	return 0, netip.AddrPort{}, c.cause
}
func (c *partialIPv6Carrier) WriteDHCPv6([]byte, netip.AddrPort) (int, error) { return 0, c.cause }
func (c *partialIPv6Carrier) Close() error                                    { return c.cleanup.Close() }
func (c *partialIPv6Carrier) RetryIPv6CarrierCleanup(ctx context.Context) error {
	return c.cleanup.retry(ctx)
}

var _ ppp.IPv6Carrier = (*partialIPv6Carrier)(nil)

// Both opener error pairs transfer every nonnil partial owner here. A successful
// first cleanup needs no returned owner; failed cleanup MUST remain reachable
// through the non-usable value returned WITH the original acquisition error.
func failedIPv6Acquisition(ra raSocket, dhcp dhcpSocket, cause error) (ppp.IPv6Carrier, error) {
	cleanup := &ipv6SocketCleanup{ra: ra, dhcp: ipv6DHCPCleanupOwner(dhcp)}
	if err := cleanup.Close(); err != nil {
		failure := errors.Join(cause, err)
		return &partialIPv6Carrier{cleanup: cleanup, cause: failure}, failure
	}
	return nil, cause
}
