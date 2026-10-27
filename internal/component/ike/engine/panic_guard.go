// Design: docs/guide/monitoring.md -- the ike rows of the contained-panic table
// Detail: docs/architecture/ike/ipsec-7-ikev2-engine.md -- inbound dispatch
// RFC: rfc/short/rfc7296.md -- the Delete a contained panic in an established SA sends
// Related: register.go -- dispatchInbound and dispatchNATTInbound, which hand each datagram to deliverDatagram
// Related: established.go -- maintainSA, whose owner loop handles each message through handleOwnedInboundGuarded

package engine

import (
	"log/slog"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// ikePanicTasks is the IKE engine's panic boundary: one taskguard.Guard and
// each task it guards, declared once. A contained panic is logged, counted in
// ze_task_panics_total{component="ike",task} and crash-recorded by the guard.
type ikePanicTasks struct {
	guard *taskguard.Guard
	// inbound is one datagram on the shared dispatch goroutine of a socket:
	// its panic drops that datagram, and the goroutine reads the next one.
	inbound taskguard.Task
	// established is one message an established SA's owner loop handles: its
	// panic deletes that IKE SA and sets it up again.
	established taskguard.Task
}

// newIKEPanicTasks declares the IKE tasks on a guard that logs to log.
func newIKEPanicTasks(log *slog.Logger) *ikePanicTasks {
	guard := taskguard.New("ike", log)
	return &ikePanicTasks{
		guard:       guard,
		inbound:     guard.Task("inbound", "this one datagram is dropped; the peer retransmits it, and every other datagram and SA is unaffected"),
		established: guard.Task("established", "this IKE SA is deleted, with a Delete sent unless the peer stopped answering, and set up again through IKE_SA_INIT and IKE_AUTH; other SAs are unaffected"),
	}
}

// ikePanics answers the IKE panic boundary. It is built on first use, so the
// guard's logger is resolved after the log levels are configured. A test MAY
// replace it to read the guard's log line, and MUST restore it.
var ikePanics = sync.OnceValue(func() *ikePanicTasks {
	return newIKEPanicTasks(slogutil.Logger("ike"))
})

// deliverDatagram hands one datagram that passed the header checks to the SA
// its SPI pair names, or answers it as a message outside any IKE SA. natt says
// it arrived on the NAT-T socket with its non-ESP marker stripped.
//
// It is the inbound task's panic boundary, so a panic here drops this datagram
// and the dispatch goroutine keeps serving every SA on the socket. A responder
// handshake the panic interrupted keeps its peer's half-open slot only until
// responderHandshakeTimeout frees it, as a handshake the initiator abandoned does.
func deliverDatagram(pkt transport.Packet, iSPI, rSPI [8]byte, natt bool, table *SATable,
	tr *transport.UDPTransport, notifyLimiter *outboundNotifyLimiter, log *slog.Logger,
) {
	defer ikePanics().inbound.Contain(describeDatagram(pkt, iSPI, rSPI), nil)

	sa := table.lookupInbound(iSPI, rSPI)
	if sa != nil {
		routeInbound(sa, pkt, table, tr, log)
		return
	}
	if tryResponderSAInit(pkt, iSPI, rSPI, table, tr, log) {
		return
	}
	if natt {
		log.Debug("ike: no SA for NAT-T packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
	} else {
		log.Debug("ike: no SA for packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
	}
	// RFC 7296 Section 2.21.4 gives a MAY here.
	// A message received outside the context of a known IKE SA, and that is
	// not a request to start one, can draw an unprotected INVALID_IKE_SPI.
	// answerOutOfSA holds each guard.
	answerOutOfSA(tr, pkt, natt, notifyLimiter, log)
	// RFC 7296 Section 2.21.4 SHOULD, beside that MAY: when the source holds an
	// IKE SA, tell the peer over that SA (notify_suspicious.go).
	noteSuspicious(pkt)
}

// describeDatagram names the unit a contained inbound panic ended: the datagram,
// by its source and SPI pair, and its exchange type.
func describeDatagram(pkt transport.Packet, iSPI, rSPI [8]byte) func() taskguard.Context {
	return func() taskguard.Context {
		unit := "datagram from " + pkt.RemoteAddr.String() + ", ispi " + SPIHex(iSPI) + " rspi " + SPIHex(rSPI)
		return taskguard.Context{Unit: unit, MessageType: exchangeTypeOf(pkt.Data)}
	}
}

// exchangeTypeOf names the Exchange Type octet of an IKE header, or answers ""
// (untracked) for data too short to hold one.
//
// RFC 7296 Section 3.1: the Exchange Type is the octet at offset 18 of the
// IKE header.
func exchangeTypeOf(data []byte) string {
	if len(data) < 28 {
		return ""
	}
	return "exchange type " + strconv.Itoa(int(data[18]))
}

// handleOwnedInboundGuarded is handleOwnedInbound under the established task's
// panic boundary. A panic while one message of an established SA is handled
// leaves that SA's state (message IDs, the request window, a half-applied rekey)
// unknown, so the SA cannot carry on: the outcome asks the owner loop to
// re-establish, which deletes the IKE SA through sayGoodbye and builds a fresh
// one through IKE_SA_INIT and IKE_AUTH.
//
// RFC 7296 Section 2.4: "Closing the IKE SA implicitly closes all associated
// Child SAs. In this case, an IKE endpoint SHOULD send a Delete payload
// indicating that it has closed the IKE SA unless the other endpoint is no
// longer responding."
func (ps *PeerSession) handleOwnedInboundGuarded(sa *SA, pkt transport.Packet, tr *transport.UDPTransport,
	dp dataplane.Dataplane, log *slog.Logger,
) (out ownedOutcome) {
	defer ikePanics().established.Contain(ps.describeOwned(sa, pkt), func(*taskguard.Panic) {
		out = ownedOutcome{reestablish: true}
	})
	return ps.handleOwnedInbound(sa, pkt, tr, dp, log)
}

// describeOwned names the unit a contained established panic ended: this
// peer's IKE SA, and the exchange type of the message it was handling.
func (ps *PeerSession) describeOwned(sa *SA, pkt transport.Packet) func() taskguard.Context {
	return func() taskguard.Context {
		unit := "IKE SA of peer " + ps.peerName + ", ispi " + SPIHex(sa.InitiatorSPI) + " rspi " + SPIHex(sa.ResponderSPI)
		return taskguard.Context{Unit: unit, MessageType: exchangeTypeOf(pkt.Data)}
	}
}
