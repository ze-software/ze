// Design: plan/learned/742-ipsec-8-ikev2-child-xfrm.md -- Dead Peer Detection
// RFC: rfc/short/rfc7296.md -- Liveness check via empty INFORMATIONAL (Section 2.4)

package engine

import (
	"log/slog"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ike/wire"
)

// dpdState tracks DPD timing for a peer session.
type dpdState struct {
	interval   time.Duration
	timeout    time.Duration
	action     ipsec.DPDAction
	lastSent   time.Time
	awaitReply bool
	sentAt     time.Time
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

// timedOut reports whether the peer failed to respond within timeout.
func (d *dpdState) timedOut(now time.Time) bool {
	if d == nil || !d.awaitReply {
		return false
	}
	return !now.Before(d.sentAt.Add(d.timeout))
}

// sendDPD sends an empty INFORMATIONAL request as a liveness check.
// RFC 7296 Section 1.4: INFORMATIONAL exchanges after IKE_SA_INIT must be
// encrypted under SK. Encryption integration requires ipsec-9 (SK wrapping).
func sendDPD(sa *SA, tr *transport.UDPTransport, dpd *dpdState, log *slog.Logger) {
	if sa == nil {
		return
	}

	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    sa.NextMsgID,
		},
	}
	sa.NextMsgID++

	if tr != nil {
		buf := make([]byte, 512)
		n := msg.WriteTo(buf, 0)
		remote := sa.remoteUDPAddr()
		if remote != nil {
			if err := tr.Send(buf[:n], remote); err != nil {
				log.Debug("dpd: send failed", "peer", sa.PeerName, "error", err)
			}
		}
	}

	now := time.Now()
	dpd.lastSent = now
	dpd.sentAt = now
	dpd.awaitReply = true
	log.Debug("dpd: sent probe", "peer", sa.PeerName, "msgid", msg.Header.MessageID)
}

// handleDPDResponse marks the DPD probe as answered.
func handleDPDResponse(dpd *dpdState, log *slog.Logger, peerName string) {
	if dpd == nil {
		return
	}
	dpd.awaitReply = false
	log.Debug("dpd: peer alive", "peer", peerName)
}
