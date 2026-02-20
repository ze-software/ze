// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure

package sim

import (
	"context"
	"net"
)

// Dialer abstracts outbound TCP connection creation.
//
// Production code uses RealDialer. Simulation code provides mock dialers
// that return mock connections (e.g., net.Pipe-based) without real TCP.
type Dialer interface {
	// DialContext connects to the address on the named network using the
	// provided context. Same semantics as net.Dialer.DialContext.
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// ListenerFactory abstracts inbound TCP listener creation.
//
// Production code uses RealListenerFactory. Simulation code provides mock
// factories that return mock listeners accepting mock connections.
type ListenerFactory interface {
	// Listen announces on the local network address.
	Listen(ctx context.Context, network, address string) (net.Listener, error)
}

// RealDialer implements Dialer using net.Dialer.
// Supports optional local address binding for source IP selection.
type RealDialer struct {
	// LocalAddr is the local address to bind to for outgoing connections.
	// If nil, the OS chooses the local address.
	LocalAddr *net.TCPAddr
}

// DialContext creates a real TCP connection using net.Dialer.
func (d *RealDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	nd := net.Dialer{}
	if d.LocalAddr != nil {
		nd.LocalAddr = d.LocalAddr
	}
	return nd.DialContext(ctx, network, address)
}

// RealListenerFactory implements ListenerFactory using net.ListenConfig.
type RealListenerFactory struct{}

// Listen creates a real TCP listener using net.ListenConfig.
func (RealListenerFactory) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	var lc net.ListenConfig
	return lc.Listen(ctx, network, address)
}
