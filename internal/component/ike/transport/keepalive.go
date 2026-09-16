// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive sender
// RFC: rfc/short/rfc3948.md -- NAT keepalive: single 0xFF byte (Section 2.3)

package transport

import (
	"log/slog"
	"net"
	"time"
)

const (
	DefaultKeepaliveInterval = 20 * time.Second
	keepaliveByte            = 0xFF
)

// Keepalive sends periodic NAT keepalive packets to maintain NAT bindings.
// It writes through the transport's Send, under the transport's lock, so a
// keepalive never leaves while SendDF holds the socket's DF option toggled.
type Keepalive struct {
	tr       *UDPTransport
	remote   *net.UDPAddr
	interval time.Duration
	stopCh   chan struct{}
	done     chan struct{}
	logger   *slog.Logger
}

// NewKeepalive creates a keepalive sender on the given transport for the remote address.
func NewKeepalive(tr *UDPTransport, remote *net.UDPAddr, interval time.Duration, logger *slog.Logger) *Keepalive {
	if interval <= 0 {
		interval = DefaultKeepaliveInterval
	}
	return &Keepalive{
		tr:       tr,
		remote:   remote,
		interval: interval,
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
		logger:   logger,
	}
}

// Run sends keepalive packets at the configured interval until Stop is called.
func (k *Keepalive) Run() {
	defer close(k.done)
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()

	pkt := []byte{keepaliveByte}

	for {
		select {
		case <-k.stopCh:
			return
		case <-ticker.C:
			if err := k.tr.Send(pkt, k.remote); err != nil {
				k.logger.Debug("nat-keepalive: send failed", "remote", k.remote, "error", err)
			}
		}
	}
}

// Stop signals the keepalive goroutine to stop and waits for it to finish.
func (k *Keepalive) Stop() {
	close(k.stopCh)
	<-k.done
}
