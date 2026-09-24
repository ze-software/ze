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
// It writes through the transport's SendFrom, under the transport's lock, so a
// keepalive uses the SA's local endpoint and never inherits a probe's DF option.
type Keepalive struct {
	tr       *UDPTransport
	local    *net.UDPAddr
	remote   *net.UDPAddr
	interval time.Duration
	stopCh   chan struct{}
	done     chan struct{}
	logger   *slog.Logger
}

// NewKeepalive creates a keepalive sender for the given endpoints. A nil local
// uses normal routing; otherwise its port MUST match the transport's bound port.
// The caller MUST keep both addresses unchanged until Stop returns.
func NewKeepalive(tr *UDPTransport, local, remote *net.UDPAddr, interval time.Duration, logger *slog.Logger) *Keepalive {
	if interval <= 0 {
		interval = DefaultKeepaliveInterval
	}
	return &Keepalive{
		tr:       tr,
		local:    local,
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
			if err := k.tr.SendFrom(pkt, k.local, k.remote); err != nil {
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
