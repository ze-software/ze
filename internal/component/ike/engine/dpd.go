// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Dead Peer Detection
// RFC: rfc/short/rfc7296.md -- Liveness check via empty INFORMATIONAL (Sections 1.4, 2.4)

package engine

import (
	"log/slog"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// dpdState tracks DPD timing for a peer session.
type dpdState struct {
	interval   time.Duration
	timeout    time.Duration
	action     ipsec.DPDAction
	lastSent   time.Time
	awaitReply bool
	sentAt     time.Time
	probeMsgID uint32 // message ID of the outstanding DPD probe (RFC 7296 §2.3 correlation)

	// probeMsg is the datagram of the outstanding probe. The owner loop repeats it.
	// RFC 7296 Section 2.4 lets an endpoint call the other one failed only after
	// REPEATED attempts go unanswered for a timeout period. Section 2.1 makes a
	// retransmission carry the Message ID of the request it repeats. The stored
	// bytes satisfy both rules.
	//
	// lastAttempt is when the probe last went out, and retries counts how often it
	// was repeated. Together they drive the exponential backoff of Section 2.4.
	probeMsg    []byte
	lastAttempt time.Time
	retries     int
}

// matchesProbe reports whether an inbound INFORMATIONAL response with the given
// message ID is the reply to the outstanding DPD probe. Correlating by message ID
// (not just "any authenticated INFORMATIONAL response") rejects replayed or
// out-of-window responses that would otherwise mask a dead peer.
func (d *dpdState) matchesProbe(msgID uint32) bool {
	return d != nil && d.awaitReply && d.probeMsgID == msgID
}

func newDPDState(cfg ipsec.DPDConfig) *dpdState {
	if cfg.Interval == 0 {
		return nil
	}
	return &dpdState{
		interval: time.Duration(cfg.Interval) * time.Second,
		timeout:  time.Duration(cfg.Timeout) * time.Second,
		action:   cfg.Action,
		lastSent: time.Now(),
	}
}

// nextDeadline returns when the next DPD action should occur.
func (d *dpdState) nextDeadline() time.Time {
	if d == nil {
		return time.Time{}
	}
	if d.awaitReply {
		return d.sentAt.Add(d.timeout)
	}
	return d.lastSent.Add(d.interval)
}

// shouldSend reports whether it is time to send a DPD probe.
func (d *dpdState) shouldSend(now time.Time) bool {
	if d == nil {
		return false
	}
	if d.awaitReply {
		return false
	}
	return !now.Before(d.lastSent.Add(d.interval))
}

// awaitingReply reports whether a DPD probe is outstanding. The owner loop reads it
// to tell an unanswered probe from an unanswered Delete.
func (d *dpdState) awaitingReply() bool {
	return d != nil && d.awaitReply
}

// timedOut reports whether the peer failed to respond within timeout. The timeout
// bounds the whole liveness budget, so it spans the probe and every retransmission
// of it. RFC 7296 Section 2.4: the peer has failed once repeated attempts have gone
// unanswered for a timeout period.
func (d *dpdState) timedOut(now time.Time) bool {
	if d == nil || !d.awaitReply {
		return false
	}
	return !now.Before(d.sentAt.Add(d.timeout))
}

// shouldRetransmit reports whether the outstanding probe waited past its current
// backoff while the liveness budget still has room. RFC 7296 Section 2.1 repeats a
// request until it draws a reply, or until the SA is deemed failed. Section 2.4
// makes the wait between attempts grow.
//
// A probe with no stored datagram cannot be repeated. sendDPD is the only writer of
// the awaiting state, and it stores the datagram it sent, so this reads as a guard on
// the field rather than as a live path.
func (d *dpdState) shouldRetransmit(now time.Time) bool {
	if d == nil || !d.awaitReply || len(d.probeMsg) == 0 {
		return false
	}
	if d.timedOut(now) {
		return false
	}
	return !now.Before(d.lastAttempt.Add(retransmitBackoff(d.retries + 1)))
}

// noteRetransmit records that the outstanding probe went out again, which lengthens
// the wait before the next attempt.
func (d *dpdState) noteRetransmit(now time.Time) {
	d.retries++
	d.lastAttempt = now
}

// sendDPD sends an empty INFORMATIONAL request as a liveness check. RFC 7296
// Section 1.4: an INFORMATIONAL exchange is cryptographically protected with the
// negotiated keys. The liveness check carries no payload other than the empty
// Encrypted payload the syntax requires, so the inner chain of the probe is nil.
//
// Every exit releases what it took. The request window of Section 2.3 is claimed
// once the probe is certain to be built, and the build failure below hands it back.
// The state at the tail is entered only after a datagram reached the send path, so
// an awaited probe always has bytes behind it.
func sendDPD(sa *SA, tr *transport.UDPTransport, dpd *dpdState, log *slog.Logger) {
	if sa == nil {
		return
	}
	// An SA that cannot send writes no probe, and a probe that was never written must
	// not be awaited. RFC 7296 Section 2.4 lets an endpoint conclude the other one has
	// failed only once REPEATED attempts have gone unanswered, and an unbuilt probe is
	// zero attempts. The state at the tail of this function would still start the
	// liveness clock, while serviceRequestWindow (established.go) leaves an awaited
	// probe to its own budget and shouldRetransmit finds no datagram to repeat. The
	// only exit left was the dead-peer verdict, on a peer this side never asked
	// anything.
	//
	// THE PREDICATE IS THE SEND PATH, NEVER THE FALLBACK ARGUMENT. sendPath (sa.go)
	// answers with the SA's OWN socket first, so a floated SA sends from nattSocket
	// while tr is nil. Reading tr instead would silence Dead Peer Detection for the
	// whole life of a NAT-traversing tunnel that sends perfectly well, which is the
	// black hole Section 2.4 asks liveness checks to prevent.
	//
	// This returns BEFORE reserveRequestWindow, so the branch takes nothing and has
	// nothing to give back: no window, no Message ID, no awaited reply, and no change
	// to the probe clock. sendRaw warns once per dropped message for the same
	// condition, so the repeat rate here follows that convention.
	if out, _ := sa.sendPath(tr); out == nil {
		log.Warn("dpd: probe skipped, the SA has no send path",
			"peer", sa.PeerName, "local-port", sa.localPort)
		return
	}
	// RFC 7296 Section 2.3: one self-initiated request at a time. A probe that finds
	// the window held is deferred and not dropped. dpd.lastSent keeps its value, so
	// the next tick raises the probe again once the window frees.
	if !sa.reserveRequestWindow() {
		log.Debug("dpd: probe deferred, a request is outstanding", "peer", sa.PeerName)
		return
	}

	msgID := sa.NextMsgID
	probe, err := buildEncryptedMessageEx(sa, nil, msgID,
		wire.ExchangeInformational, initiatorFlag(sa))
	if err != nil {
		log.Warn("dpd: probe build failed, dropping", "peer", sa.PeerName, "error", err)
		sa.releaseRequestWindow()
		return
	}
	sendRaw(sa, tr, probe, log)
	sa.advanceMsgID()

	now := time.Now()
	dpd.lastSent = now
	dpd.sentAt = now
	dpd.awaitReply = true
	dpd.probeMsgID = msgID
	// Keep the exact datagram. A retransmission repeats this request. It does not
	// raise a new one, so it carries the same Message ID (RFC 7296 Section 2.1).
	// The one request window of Section 2.3 stays where it is.
	dpd.probeMsg = probe
	dpd.lastAttempt = now
	dpd.retries = 0
	log.Debug("dpd: sent probe", "peer", sa.PeerName, "msgid", msgID)
}

// retransmitDPD repeats the outstanding probe. RFC 7296 Section 2.1 repeats a
// request under its own Message ID. It stops when the request draws a reply, or
// when the SA is deemed failed.
func retransmitDPD(sa *SA, tr *transport.UDPTransport, dpd *dpdState, now time.Time, log *slog.Logger) {
	sendRaw(sa, tr, dpd.probeMsg, log)
	dpd.noteRetransmit(now)
	log.Debug("dpd: retransmitted probe", "peer", sa.PeerName,
		"msgid", dpd.probeMsgID, "attempt", dpd.retries)
}

// handleDPDResponse marks the DPD probe as answered and drops the datagram kept for
// its retransmission.
func handleDPDResponse(dpd *dpdState, log *slog.Logger, peerName string) {
	if dpd == nil {
		return
	}
	dpd.awaitReply = false
	dpd.probeMsg = nil
	dpd.retries = 0
	log.Debug("dpd: peer alive", "peer", peerName)
}
