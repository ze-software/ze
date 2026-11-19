// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact ordered accounting owners.
package l2tpauthradius

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// TestAccountingOverlappingBirthsKeepDistinctRecords sends two real Accounting-
// Starts with identical wire labels. Ending one original must leave the other
// issued ID live; a repeated old Down cannot terminate its sibling.
func TestAccountingOverlappingBirthsKeepDistinctRecords(t *testing.T) {
	acct, server := newOrderedAccounting(t, 0, time.Hour)
	capture := server.capture
	first := orderedAccountingAssignment(t, "first")
	second := orderedAccountingAssignment(t, "second")
	acct.onSessionIPAssigned(first)
	firstStart := capture.waitForCounterRecord(t, "first Start", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStart && packet.username == "first"
	})
	acct.onSessionIPAssigned(second)
	secondStart := capture.waitForCounterRecord(t, "second Start with reused labels", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStart && packet.username == "second"
	})
	if firstStart.sessionID == secondStart.sessionID {
		t.Fatal("distinct births shared one issued accounting ID")
	}
	ids := acct.currentSessionIDs()
	if ids[first.Birth] != firstStart.sessionID {
		t.Fatalf("first original issued ID = %q, want %q", ids[first.Birth], firstStart.sessionID)
	}
	if ids[second.Birth] != secondStart.sessionID {
		t.Fatalf("second original issued ID = %q, want %q", ids[second.Birth], secondStart.sessionID)
	}

	down := orderedAccountingDown(first)
	acct.onSessionDown(down)
	capture.waitForCounterRecord(t, "first Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop && packet.sessionID == firstStart.sessionID
	})
	acct.onSessionDown(down)
	ids = acct.currentSessionIDs()
	if _, found := ids[first.Birth]; found {
		t.Fatal("the ended first original still has a current accounting ID")
	}
	if ids[second.Birth] != secondStart.sessionID {
		t.Fatal("first or repeated stale Down removed the second original")
	}
	acct.onSessionDown(orderedAccountingDown(second))
	capture.waitForCounterRecord(t, "second Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop && packet.sessionID == secondStart.sessionID
	})
	acct.Stop()
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.packets) != 4 {
		t.Fatalf("wire records = %+v, want exactly two Starts and their two Stops", capture.packets)
	}
}

// TestAccountingDownReturnsWhileStartAwaitsRadius holds the actual signed UDP
// Start exchange. The publishing callback must return while the remote answer
// remains held; the active server then receives its one matching Stop. A
// separate owner-model test proves operation completion, rather than inferring
// Client.Exchange completion from this server sending its acknowledgement.
func TestAccountingDownReturnsWhileStartAwaitsRadius(t *testing.T) {
	acct, server := newOrderedAccounting(t, radius.AcctStatusStart, time.Hour)
	assigned := orderedAccountingAssignment(t, "held-start")
	acct.onSessionIPAssigned(assigned)
	server.waitHeld(t)
	done := orderedAccountingEnd(t, acct, orderedAccountingDown(assigned), server)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SessionDown waits for remote RADIUS while Start is held")
	}
	server.release()
	stop := server.capture.waitForCounterRecord(t, "held Start's ordered Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop
	})
	acct.Stop()
	assertOrderedAccountingWire(t, server, []uint8{radius.AcctStatusStart, radius.AcctStatusStop}, stop.sessionID)
	if stop.terminateCause != uint32(l2tpevents.TerminateCauseAdminReset) {
		t.Fatalf("Stop cause = %d, want the original administrative cause", stop.terminateCause)
	}
}

// TestAccountingDownReturnsWhileInterimAwaitsRadius holds an actual Interim.
// Ending the session must return before its remote acknowledgement. The active
// server also refuses a competing Stop submitted before that acknowledgement.
// This wire observation does not prove when Client.Exchange itself returns.
func TestAccountingDownReturnsWhileInterimAwaitsRadius(t *testing.T) {
	acct, server := newOrderedAccounting(t, radius.AcctStatusInterimUpdate, 30*time.Millisecond)
	assigned := orderedAccountingAssignment(t, "held-interim")
	acct.onSessionIPAssigned(assigned)
	server.waitHeld(t)
	done := orderedAccountingEnd(t, acct, orderedAccountingDown(assigned), server)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SessionDown waits for remote RADIUS while Interim is held")
	}
	server.release()
	stop := server.capture.waitForCounterRecord(t, "held Interim's ordered Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop
	})
	acct.Stop()
	assertOrderedAccountingWire(t, server, []uint8{
		radius.AcctStatusStart, radius.AcctStatusInterimUpdate, radius.AcctStatusStop,
	}, stop.sessionID)
}

// TestAccountingTerminalKeepsItsOriginalEndpoint replaces the current client
// while the original Start is accepted but unacknowledged. The terminal record
// belongs to the same retained server episode and repeats its issued ID.
func TestAccountingTerminalKeepsItsOriginalEndpoint(t *testing.T) {
	acct, original := newOrderedAccounting(t, radius.AcctStatusStart, time.Hour)
	replacement, replacementClient := newOrderedAccountingServer(t, 0)
	assigned := orderedAccountingAssignment(t, "original-endpoint")
	acct.onSessionIPAssigned(assigned)
	original.waitHeld(t)
	acct.setClient(replacementClient, "replacement-nas", time.Hour, replacement.address, nil, "")
	done := orderedAccountingEnd(t, acct, orderedAccountingDown(assigned), original)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("endpoint replacement made SessionDown wait for RADIUS")
	}
	original.release()
	stop := original.capture.waitForCounterRecord(t, "original endpoint Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop
	})
	acct.Stop()
	assertOrderedAccountingWire(t, original, []uint8{radius.AcctStatusStart, radius.AcctStatusStop}, stop.sessionID)
	replacement.capture.mu.Lock()
	defer replacement.capture.mu.Unlock()
	if len(replacement.capture.packets) != 0 {
		t.Fatalf("replacement endpoint received old accounting records: %+v", replacement.capture.packets)
	}
}

func orderedAccountingAssignment(t *testing.T, username string) *l2tpevents.SessionIPAssignedPayload {
	t.Helper()
	birth, err := subscriberid.NewL2TPBirth()
	if err != nil {
		t.Fatal(err)
	}
	return &l2tpevents.SessionIPAssignedPayload{
		Birth: birth, SubscriberID: birth.String(), TunnelID: 91, SessionID: 17,
		Username: username, PeerAddr: "198.51.100.17",
	}
}

func orderedAccountingDown(assigned *l2tpevents.SessionIPAssignedPayload) *l2tpevents.SessionDownPayload {
	return &l2tpevents.SessionDownPayload{
		Birth: assigned.Birth, SubscriberID: assigned.SubscriberID,
		TunnelID: assigned.TunnelID, SessionID: assigned.SessionID,
		Username: assigned.Username, Cause: l2tpevents.TerminateCauseAdminReset,
	}
}

// The one test helper is joined even on an assertion failure. Releasing the
// real server before joining prevents a failed old synchronous callback from
// extending its network timeout into another test.
func orderedAccountingEnd(t *testing.T, acct *radiusAcct, down *l2tpevents.SessionDownPayload, server *orderedAccountingServer) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		acct.onSessionDown(down)
	}()
	t.Cleanup(func() {
		server.release()
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			t.Error("the accounting callback did not finish after server release")
			if err := server.client.Close(); err != nil {
				t.Error(err)
			}
			<-done
		}
	})
	return done
}

type orderedAccountingServer struct {
	address     string
	client      *radius.Client
	capture     *acctCapture
	held        chan struct{}
	gate        chan struct{}
	releaseOnce sync.Once
	earlyStop   bool     // guarded by capture.mu
	nasIDs      []string // guarded by capture.mu
	eventTimes  [][]byte // Event-Timestamp values in wire order, guarded by capture.mu
}

func (server *orderedAccountingServer) release() {
	server.releaseOnce.Do(func() { close(server.gate) })
}

func (server *orderedAccountingServer) waitHeld(t *testing.T) {
	t.Helper()
	select {
	case <-server.held:
	case <-time.After(5 * time.Second):
		t.Fatal("the real accounting exchange never reached its held server")
	}
}

func newOrderedAccounting(t *testing.T, heldStatus uint8, interval time.Duration) (*radiusAcct, *orderedAccountingServer) {
	t.Helper()
	server, client := newOrderedAccountingServer(t, heldStatus)
	acct := newRADIUSAcct()
	acct.setClient(client, "original-nas", interval, server.address, nil, "")
	t.Cleanup(acct.Stop)
	t.Cleanup(server.release)
	return acct, server
}

// One bounded test server worker owns reads and signed acknowledgements.
// A selected exchange stays held until release; no request spawns a goroutine.
func newOrderedAccountingServer(t *testing.T, heldStatus uint8) (*orderedAccountingServer, *radius.Client) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	server := &orderedAccountingServer{
		address: conn.LocalAddr().String(), capture: newAcctCapture(),
		held: make(chan struct{}), gate: make(chan struct{}),
	}
	sharedKey := []byte("ordered-accounting-original")
	done := make(chan struct{})
	go func() {
		defer close(done)
		var heldOnce sync.Once
		type heldReply struct {
			packet        *radius.Packet
			from          *net.UDPAddr
			authenticator [radius.AuthenticatorLen]byte
		}
		pending := make([]heldReply, 0, 8)
		buffer := make([]byte, radius.MaxPacketLen)
		acknowledge := func(reply heldReply) bool {
			response := make([]byte, radius.HeaderLen)
			response[0], response[1] = radius.CodeAccountingResp, reply.packet.Identifier
			binary.BigEndian.PutUint16(response[2:4], radius.HeaderLen)
			authenticator := radius.ResponseAuthenticator(radius.CodeAccountingResp,
				reply.packet.Identifier, radius.HeaderLen, reply.authenticator, nil, sharedKey)
			copy(response[4:4+radius.AuthenticatorLen], authenticator[:])
			if _, writeErr := conn.WriteToUDP(response, reply.from); writeErr != nil {
				if errors.Is(writeErr, net.ErrClosed) {
					return false
				}
				t.Error(writeErr)
				return false
			}
			return true
		}
		for {
			// A read timeout only lets this one worker observe release; it
			// never decides success. Reads and capture continue while ACKs wait.
			if err := conn.SetReadDeadline(time.Now().Add(25 * time.Millisecond)); err != nil {
				if errors.Is(err, net.ErrClosed) {
					return
				}
				t.Error(err)
				return
			}
			length, from, readErr := conn.ReadFromUDP(buffer)
			if readErr != nil {
				if errors.Is(readErr, net.ErrClosed) {
					return
				}
				if timeout, ok := errors.AsType[net.Error](readErr); ok {
					if !timeout.Timeout() {
						t.Error(readErr)
						return
					}
					select {
					case <-server.gate:
						for _, reply := range pending {
							if !acknowledge(reply) {
								return
							}
						}
						pending = pending[:0]
					default:
					}
					continue
				}
				t.Error(readErr)
				return
			}
			packet, decodeErr := radius.Decode(buffer[:length])
			if decodeErr != nil {
				t.Error(decodeErr)
				return
			}
			if packet.Code != radius.CodeAccountingReq {
				t.Errorf("unexpected packet code %d", packet.Code)
				return
			}
			status := packet.FindAttr(radius.AttrAcctStatusType)
			if len(status) != 4 {
				t.Errorf("missing Acct-Status-Type: %x", status)
				return
			}
			kind := uint8(binary.BigEndian.Uint32(status))
			server.capture.mu.Lock()
			server.nasIDs = append(server.nasIDs, string(packet.FindAttr(radius.AttrNASIdentifier)))
			server.eventTimes = append(server.eventTimes, append([]byte(nil), packet.FindAttr(radius.AttrEventTimestamp)...))
			if kind == radius.AcctStatusStop {
				if len(pending) != 0 {
					server.earlyStop = true
				}
			}
			server.capture.mu.Unlock()
			server.capture.add(packet)
			reply := heldReply{packet: packet, from: from}
			copy(reply.authenticator[:], buffer[4:4+radius.AuthenticatorLen])
			if kind == heldStatus {
				select {
				case <-server.gate:
				default:
					if len(pending) == cap(pending) {
						t.Error("held accounting exchange exceeded eight bounded requests")
						return
					}
					pending = append(pending, reply)
					heldOnce.Do(func() { close(server.held) })
					continue
				}
			}
			if !acknowledge(reply) {
				return
			}
		}
	}()
	t.Cleanup(func() {
		server.release()
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
		<-done
	})
	client, err := radius.NewClient(radius.ClientConfig{
		Servers: []radius.Server{{Address: server.address, SharedKey: sharedKey}},
		Timeout: 5 * time.Second, Retries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	server.client = client
	return server, client
}

func assertOrderedAccountingWire(t *testing.T, server *orderedAccountingServer, statuses []uint8, sessionID string) {
	t.Helper()
	capture := server.capture
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if server.earlyStop {
		t.Fatal("a competing Stop reached the wire before the held acknowledgement")
	}
	if len(server.nasIDs) != len(capture.packets) {
		t.Fatal("wire observation omitted a record's NAS-Identifier")
	}
	for _, nasID := range server.nasIDs {
		if nasID != "original-nas" {
			t.Fatalf("record NAS-Identifier = %q, want original-nas", nasID)
		}
	}
	if len(capture.packets) != len(statuses) {
		t.Fatalf("accounting records = %+v, want statuses %v", capture.packets, statuses)
	}
	for index, packet := range capture.packets {
		if packet.statusType != statuses[index] {
			t.Fatalf("record %d status=%d, want %d", index, packet.statusType, statuses[index])
		}
		if packet.sessionID != sessionID {
			t.Fatalf("record %d belongs to %q, want original %q", index, packet.sessionID, sessionID)
		}
	}
}
