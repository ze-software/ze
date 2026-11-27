// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT keepalive sender
// RFC: rfc/short/rfc3948.md -- NAT keepalive: single 0xFF byte (Section 2.3)

package transport

import (
	"log/slog"
	"net"
	"sync/atomic"
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
	failing := false
	defer func() {
		if failing {
			notifyNATKeepaliveSink(NATKeepaliveEvent{
				Kind:   NATKeepaliveEventRecovered,
				Local:  k.local,
				Remote: k.remote,
			})
		}
	}()

	for {
		select {
		case <-k.stopCh:
			return
		case <-ticker.C:
			if err := k.tr.SendFrom(pkt, k.local, k.remote); err != nil {
				reportAttr := notifyNATKeepaliveSink(NATKeepaliveEvent{
					Kind:   NATKeepaliveEventFailed,
					Local:  k.local,
					Remote: k.remote,
					Err:    err,
				})
				if !failing {
					failing = true
					if reportAttr.Equal(slog.Attr{}) {
						reportAttr = slog.String("report", "not-wired")
					}
					k.logger.Warn(
						"nat-keepalive: send failed; intermediate NAT mappings may expire and drop inbound IKE and ESP traffic until the peer renegotiates",
						"code", CodeDoctorIPsecNATKeepaliveFailed,
						"remote", k.remote,
						"error", err,
						"fix", "ze show vpn ipsec sa",
						reportAttr,
					)
				}
				continue
			}
			if failing {
				failing = false
				notifyNATKeepaliveSink(NATKeepaliveEvent{
					Kind:   NATKeepaliveEventRecovered,
					Local:  k.local,
					Remote: k.remote,
				})
			}
		}
	}
}

func notifyNATKeepaliveSink(ev NATKeepaliveEvent) slog.Attr {
	if sink := natKeepaliveSink.Load(); sink != nil {
		return (*sink)(ev)
	}
	return slog.Attr{}
}

// Stop signals the keepalive goroutine to stop and waits for it to finish.
func (k *Keepalive) Stop() {
	close(k.stopCh)
	<-k.done
}

// NATKeepaliveEventKind classifies one NAT-T keepalive health transition
// forwarded to the registered report sink.
type NATKeepaliveEventKind uint8

const (
	// NATKeepaliveEventUnspecified is the zero value.
	NATKeepaliveEventUnspecified NATKeepaliveEventKind = iota
	// NATKeepaliveEventFailed reports a NAT-T keepalive send failure.
	NATKeepaliveEventFailed
	// NATKeepaliveEventRecovered reports that keepalive sends succeeded again
	// or the keepalive stopped, ending the current failure run.
	NATKeepaliveEventRecovered
)

// NATKeepaliveEvent describes one NAT-T keepalive failure or recovery
// transition. Never changed once built.
type NATKeepaliveEvent struct {
	Kind   NATKeepaliveEventKind
	Local  *net.UDPAddr
	Remote *net.UDPAddr
	Err    error
}

// NATKeepaliveSink handles NAT-T keepalive failure and recovery transitions
// without blocking the keepalive ticker. On NATKeepaliveEventFailed it answers
// the log attribute describing whether the report was queued (report=queued or
// report-error=...). Safe for concurrent use.
type NATKeepaliveSink func(NATKeepaliveEvent) slog.Attr

// natKeepaliveSink is the registered NATKeepaliveSink, nil until one is
// registered.
var natKeepaliveSink atomic.Pointer[NATKeepaliveSink]

// RegisterNATKeepaliveSink records sink as the taker of NAT-T keepalive failure
// and recovery transitions, and a nil sink removes it. The IKE engine registers
// it in runEngine so keepalive failures reach the daemon's report bus
// non-blockingly in both internal and forked plugin modes without coupling
// transport to sdk or issuequeue. Safe for concurrent use.
func RegisterNATKeepaliveSink(sink NATKeepaliveSink) {
	if sink == nil {
		natKeepaliveSink.Store(nil)
		return
	}
	natKeepaliveSink.Store(&sink)
}
