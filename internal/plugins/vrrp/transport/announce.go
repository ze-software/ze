// RFC: rfc/short/rfc9568.md -- Section 8.1.2 / 8.2.2 (announce on Master transition)
// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- per-instance announcer worker
//
// announce.go runs one long-lived worker goroutine per instance
// (ai/rules/goroutine-lifecycle.md: no per-burst goroutines). AnnounceMaster
// enqueues a VIP list; the worker builds the family-appropriate frame per VIP and
// transmits it announceRepeatCount times with announceRepeatInterval spacing.
// keepalived's single-frame announce is lost exactly when a failover storms the
// LAN, so 3x100ms is the cheap-insurance default (Key Design Decisions). The queue
// is bounded so repeated Master flaps cannot pile unbounded announce jobs
// (security: resource exhaustion).

package transport

import (
	"net/netip"
	"time"
)

const (
	// announceRepeatCount is the number of times each VIP's announcement frame is
	// sent per Master transition. Internal constant (no config knob, umbrella
	// Finding 18).
	announceRepeatCount = 3
	// announceRepeatInterval is the spacing between the repeats.
	announceRepeatInterval = 100 * time.Millisecond
	// announceQueueDepth bounds the burst queue so Master flaps cannot accumulate
	// unbounded work; a dropped job is re-created by the next Master transition.
	announceQueueDepth = 16
	// announceBufLen sizes the reusable per-worker frame buffer; it must hold the
	// larger of the GARP frame (42) and the NA message (32).
	announceBufLen = 64
)

// frameBuilder builds an announcement frame for one VIP into buf, returning the
// byte count and whether a frame was produced (false for a VIP of the wrong
// family, so a mixed VIP list is filtered defensively).
type frameBuilder func(vip netip.Addr, buf []byte) (int, bool)

// announcer is the per-instance burst worker.
type announcer struct {
	jobs  chan []netip.Addr
	stop  chan struct{}
	done  chan struct{}
	build frameBuilder
	send  func(frame []byte) error
	// report is called after each send with its error (nil on success) so the
	// owning instance can count announcements_sent / *-send-error.
	report func(err error)
	// sleep is the inter-repeat delay, a seam so tests assert spacing without
	// real time.
	sleep func(time.Duration)
	buf   []byte
}

// newAnnouncer builds a worker. The worker is not running until start().
func newAnnouncer(build frameBuilder, send func([]byte) error, report func(error)) *announcer {
	return &announcer{
		jobs:   make(chan []netip.Addr, announceQueueDepth),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
		build:  build,
		send:   send,
		report: report,
		sleep:  time.Sleep,
		buf:    make([]byte, announceBufLen),
	}
}

// start launches the worker goroutine (one per instance).
func (a *announcer) start() { go a.run() }

// enqueue queues a burst for the given VIPs and reports whether it was accepted.
// It is non-blocking: a full queue drops the job (returns false), since the next
// Master transition re-announces the current VIP set. The caller's slice may be a
// lazy view, so it is copied.
func (a *announcer) enqueue(vips []netip.Addr) bool {
	cp := append([]netip.Addr(nil), vips...)
	select {
	case a.jobs <- cp:
		return true
	default:
		return false
	}
}

// stopped reports whether close() has been called, without blocking.
func (a *announcer) stopped() bool {
	select {
	case <-a.stop:
		return true
	default:
		return false
	}
}

// run drains queued bursts until stop is closed.
func (a *announcer) run() {
	defer close(a.done)
	for {
		select {
		case <-a.stop:
			return
		case vips := <-a.jobs:
			a.runBurst(vips)
		}
	}
}

// runBurst builds and sends each VIP's frame announceRepeatCount times with
// announceRepeatInterval spacing. It reuses a.buf (single worker, no concurrency).
func (a *announcer) runBurst(vips []netip.Addr) {
	for _, vip := range vips {
		n, ok := a.build(vip, a.buf)
		if !ok {
			continue
		}
		for i := range announceRepeatCount {
			a.report(a.send(a.buf[:n]))
			if i < announceRepeatCount-1 {
				a.sleep(announceRepeatInterval)
				if a.stopped() {
					return
				}
			}
		}
	}
}

// close stops the worker and waits for it to exit (join, no leak).
func (a *announcer) close() {
	close(a.stop)
	<-a.done
}
