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
type Keepalive struct {
	conn     *net.UDPConn
	remote   *net.UDPAddr
	interval time.Duration
	stopCh   chan struct{}
	done     chan struct{}
	logger   *slog.Logger
}

// NewKeepalive creates a keepalive sender for the given connection and remote address.
func NewKeepalive(conn *net.UDPConn, remote *net.UDPAddr, interval time.Duration, logger *slog.Logger) *Keepalive {
	if interval <= 0 {
		interval = DefaultKeepaliveInterval
	}
	return &Keepalive{
		conn:     conn,
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
			if _, err := k.conn.WriteToUDP(pkt, k.remote); err != nil {
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
