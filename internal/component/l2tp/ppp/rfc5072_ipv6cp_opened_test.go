// VALIDATES: RFC 5072 Section 2 on a session that enables both NCPs -- with
// IPCP Opened and IPV6CP driven out of Initial but held short of Opened, ze
// never starts its IPv6 service, the source of every IPv6 packet it sends.
// PREVENTS: a gate that keys the IPv6 service on "IPV6CP started" or on "the
// network phase is running" rather than on IPV6CP Opened.
//
// RFC: rfc/short/rfc5072.md
package ppp

import (
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

// playIPCPOpenIPv6CPHeld plays the peer of a session that enables both NCPs:
// IPCP is completed (ze's Configure-Request Acked, the peer's own request sent),
// while ze's IPV6CP Configure-Request is Acked but the peer never sends its own,
// so ze's IPV6CP stays in Ack-Rcvd. It returns when the connection closes or
// the deadline passes, and closes done.
func playIPCPOpenIPv6CPHeld(t *testing.T, conn net.Conn, deadline time.Time, done chan<- struct{}) {
	defer close(done)
	ipcpSentCR := false
	buf := make([]byte, MaxFrameLen)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return
		}
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		proto, payload, _, perr := ParseFrame(buf[:n])
		if perr != nil {
			t.Errorf("peer ParseFrame: %v", perr)
			return
		}
		pkt, perr := ParseLCPPacket(payload)
		if perr != nil {
			t.Errorf("peer ParseLCPPacket: %v", perr)
			return
		}
		if pkt.Code != LCPConfigureRequest {
			continue
		}
		switch proto {
		case ProtoIPCP:
			writePeerNCPFrame(t, conn, ProtoIPCP, LCPConfigureAck, pkt.Identifier, pkt.Data)
			if !ipcpSentCR {
				peerCR := []byte{3, 6}
				a4 := ipcpTestPeer.As4()
				peerCR = append(peerCR, a4[:]...)
				writePeerNCPFrame(t, conn, ProtoIPCP, LCPConfigureRequest, 0x40, peerCR)
				ipcpSentCR = true
			}
		case ProtoIPv6CP:
			writePeerNCPFrame(t, conn, ProtoIPv6CP, LCPConfigureAck, pkt.Identifier, pkt.Data)
		}
	}
}

// VALIDATES: with both NCPs enabled, IPCP Opened and IPV6CP in Ack-Rcvd (ze's
// request Acked, the peer's never sent), the session never starts its IPv6
// service and never reports the session up, until the NCP timeout ends it.
// METHOD: the IPv6 service attempt is observed through its log lines:
// afterLCPOpenIPv6Service logs "IPv6 service start failed" whenever it calls
// startIPv6Service (the service cannot start in the test), and logs "refusing
// to start the IPv6 service" when it is reached with no negotiated peer
// interface identifier, the form an early start takes in this scenario. The
// wait runs until EventSessionDown, the end of every path that could reach the
// service.
//
// RFC requirement: RFC5072-2-1 positive -- with IPCP Opened and IPV6CP out of Initial but not Opened (Ack-Rcvd), no EventSessionUp is emitted, the session ends on the NCP timeout with EventSessionDown, and the IPv6 service start is never attempted.
func TestRFC5072NoIPv6ServiceWhileIPv6CPShortOfOpened(t *testing.T) {
	// Both lines mark an attempt: the first when startIPv6Service ran and
	// failed, the second when the gate was passed without a negotiated peer
	// interface identifier, which is what an early start logs here because
	// the peer never sends its own IPV6CP Configure-Request.
	startAttemptLogs := []string{
		"IPv6 service start failed",
		"refusing to start the IPv6 service",
	}
	w := &captureWriter{}
	logger := slog.New(slog.NewTextHandler(w, nil))
	td := newNCPTestDriverIPLogged(t, &StartSession{}, autoAcceptIP, logger)
	defer td.cleanup()

	done := make(chan struct{})
	go playIPCPOpenIPv6CPHeld(t, td.peer, time.Now().Add(5*time.Second), done)
	t.Cleanup(func() { <-done })

	deadline := time.After(5 * time.Second)
	for {
		select {
		case ev := <-td.driver.EventsOut():
			switch got := ev.(type) {
			case EventSessionIPAssigned:
				got.Acknowledge()
			case EventSessionUp:
				t.Fatalf("EventSessionUp while IPV6CP was not Opened; log = %q", w.String())
			case EventSessionDown:
				log := w.String()
				for _, attempt := range startAttemptLogs {
					if strings.Contains(log, attempt) {
						t.Fatalf("IPv6 service start attempted while IPV6CP was not Opened (%q); log = %q", attempt, log)
					}
				}
				if !strings.Contains(got.Reason, "ncp: timeout") {
					t.Fatalf("session ended with %q, want the NCP timeout; log = %q", got.Reason, w.String())
				}
				return
			}
		case <-deadline:
			t.Fatalf("no EventSessionDown within 5 s; log = %q", w.String())
		}
	}
}
