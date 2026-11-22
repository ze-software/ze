// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP session ownership, PADS and PADT ordering
// Related: server.go -- handlePADR, sendPADS, finishSessionDown
// Related: current_core_fixture_test.go -- the real PPP driver harness the teardown tests drive

package pppoe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// The events a vppOrderLog records, in the order the product must produce them.
const (
	orderSessionCreated = "session created"
	orderSessionDeleted = "session deleted"
	orderPADSSent       = "PADS sent"
	orderPADSRefusal    = "PADS refusal sent"
	orderPADTSent       = "PADT sent"
	orderOtherFrame     = "other discovery frame sent"
)

// vppOrderLog is the one timeline of a session's data-plane owner and the
// discovery writer. Safe for concurrent use: the PPP driver's worker and the
// test goroutine both append to it.
type vppOrderLog struct {
	mu     sync.Mutex
	events []string
	frames []Packet
}

func (l *vppOrderLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *vppOrderLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

// sent returns every discovery frame the server handed its writer, parsed.
func (l *vppOrderLog) sent() []Packet {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.frames)
}

// record classifies one frame handed to the discovery writer. A PADS with a
// SESSION_ID confirms a session; one with 0x0000 refuses the PADR.
func (l *vppOrderLog) record(t *testing.T, frame []byte) Packet {
	t.Helper()
	packet, err := ParseDiscovery(frame)
	if err != nil {
		t.Errorf("server sent an unparseable discovery frame: %v", err)
		return Packet{}
	}
	event := orderOtherFrame
	switch packet.Code {
	case CodePADS:
		event = orderPADSRefusal
		if packet.SID != 0 {
			event = orderPADSSent
		}
	case CodePADT:
		event = orderPADTSent
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.frames = append(l.frames, packet)
	l.events = append(l.events, event)
	return packet
}

// vppOrderOwner has the shape the VPP provider gives the server: one owner is
// the frame channel, the data-plane handle and the transport at once
// (dataplane/vpp/factory.go: NewOpenedSession(owner, owner, owner)), so the
// first Close from either role deletes the VPP session. The embedded handle
// is nil: a data-plane operation this test does not expect panics rather than
// pretending VPP did the work.
type vppOrderOwner struct {
	ppp.SessionHandle
	log      *vppOrderLog
	identity ppp.SessionIdentity
	sid      uint16
	peer     [EthALen]byte
	control  bytes.Buffer
	mu       sync.Mutex
	closes   int
}

func (o *vppOrderOwner) Control() io.ReadWriter { return &o.control }
func (*vppOrderOwner) Opened() io.Reader        { return nil }
func (*vppOrderOwner) InterruptRead() error {
	return errors.New("the ordering owner has no running reader")
}
func (o *vppOrderOwner) Identity() ppp.SessionIdentity { return o.identity }

// Close deletes the VPP session once; a later Close from the other role is
// the idempotent repeat the provider contract allows.
func (o *vppOrderOwner) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closes++
	if o.closes == 1 {
		o.log.add(orderSessionDeleted)
	}
	return nil
}

func (o *vppOrderOwner) deleted() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.closes != 0
}

// newVPPOrderOwner records the session as created: the provider has returned
// a confirmed VPP session for this SESSION_ID and peer MAC.
func newVPPOrderOwner(log *vppOrderLog, sid uint16, peer [EthALen]byte) *vppOrderOwner {
	owner := &vppOrderOwner{
		log: log, sid: sid, peer: peer,
		identity: ppp.SessionIdentity{InterfaceName: "pppoe_session0", NativeIndex: 7},
	}
	log.add(orderSessionCreated)
	return owner
}

// vppOrderAdmission is a server whose session provider is the VPP-shaped
// owner. Its PPP driver is constructed and never started, so the StartSession
// handlePADR transfers stays queued and no PPP worker closes the owner.
type vppOrderAdmission struct {
	server    *InterfaceServer
	log       *vppOrderLog
	owner     *vppOrderOwner
	opens     int
	padsOwner *vppOrderOwner // the owner as it stood when the confirming PADS left
	padsAlive bool
}

func newVPPOrderAdmission(t *testing.T, openErr error) *vppOrderAdmission {
	t.Helper()
	var unused [][]byte
	admission := &vppOrderAdmission{log: &vppOrderLog{}}
	server := newRecordingServer(&unused, nil, 8)
	server.acName = "ze-ac"
	server.pppDriver = ppp.NewProductionDriver(server.logger)
	server.openSession = func(_ context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
		admission.opens++
		if openErr != nil {
			return nil, openErr
		}
		owner := newVPPOrderOwner(admission.log, request.SessionID, request.PeerMAC)
		opened, err := dataplane.NewOpenedSession(owner, owner, owner)
		if err != nil {
			return nil, err
		}
		admission.owner = owner
		return opened, nil
	}
	server.sendFrameFn = func(frame []byte) {
		packet := admission.log.record(t, frame)
		if packet.Code == CodePADS && packet.SID != 0 {
			admission.padsOwner = admission.owner
			admission.padsAlive = admission.owner != nil && !admission.owner.deleted()
		}
	}
	admission.server = server
	return admission
}

func wantOrder(t *testing.T, log *vppOrderLog, want ...string) {
	t.Helper()
	if got := log.snapshot(); !slices.Equal(got, want) {
		t.Fatalf("event order = %q, want %q", got, want)
	}
}

// TestPPPoESessionCreatedBeforePADS -- spec-pppoe-vpp-dataplane AC-4.
//
// VALIDATES: a PADR reaching the production handlePADR opens the session
// through the selected provider before any PADS leaves, and the PADS that
// leaves names the SESSION_ID and client MAC of the VPP session that exists
// at that moment, undeleted.
// METHOD: the provider is a VPP-shaped owner built with NewOpenedSession, the
// only constructor the access refuses to bypass; it logs "session created" as
// Open returns, and the captured discovery writer logs "PADS sent". The
// order of the two entries is the assertion.
// PREVENTS: a PADS sent before the data plane exists, so the client starts
// LCP on a session VPP does not yet forward (spec: "PADS only after the
// session data plane exists"). MUTATION: in handlePADR, send the PADS frame
// before s.openSession is called.
func TestPPPoESessionCreatedBeforePADS(t *testing.T) {
	admission := newVPPOrderAdmission(t, nil)
	server := admission.server
	client := [EthALen]byte{2, 0, 0, 0, 0, 0x42}

	server.handlePADR(padrFrom(server, client, "", nil, nil))

	wantOrder(t, admission.log, orderSessionCreated, orderPADSSent)
	sent := admission.log.sent()
	pads := sent[0]
	owner := admission.padsOwner
	if owner == nil {
		t.Fatal("PADS left before the provider returned a session")
	}
	if !admission.padsAlive {
		t.Fatal("PADS left after its VPP session was deleted")
	}
	if owner.sid != pads.SID {
		t.Fatalf("VPP session SESSION_ID = 0x%04x, PADS SESSION_ID = 0x%04x", owner.sid, pads.SID)
	}
	if owner.peer != client {
		t.Fatalf("VPP session peer MAC = %s, want the client %s", net.HardwareAddr(owner.peer[:]), net.HardwareAddr(client[:]))
	}
	if pads.DstMAC != client {
		t.Fatalf("PADS destination = %s, want the client %s", net.HardwareAddr(pads.DstMAC[:]), net.HardwareAddr(client[:]))
	}
	if session := server.sessions.Lookup(pads.SID); session == nil || session.Opened == nil || session.Opened.Dataplane() != owner {
		t.Fatal("the admitted session does not hold the VPP session the PADS confirmed")
	}
	if queued := len(server.pppDriver.SessionsIn()); queued != 1 {
		t.Fatalf("StartSession queued %d times, want once after PADS", queued)
	}
}

// TestPPPoESessionOpenFailureSendsNoPADS -- spec-pppoe-vpp-dataplane AC-4, negative.
//
// VALIDATES: when the provider refuses to create the VPP session, the PADR
// draws no PADS confirming a session. RFC 2516 Section 5.4 gives a confirming
// PADS "the unique value generated for this PPPoE session", and none exists;
// the only reply the RFC permits is a refusal with SESSION_ID 0x0000 whose
// AC-System-Error tag "MAY be included in PADS packets", so any frame sent
// must be exactly that.
// METHOD: the same production handlePADR as the positive case, with Open
// returning an error. Open must have been called once, so a PADR refused
// before admission cannot pass this test by sending nothing.
// PREVENTS: a PADS sent for a session whose data plane was never created.
func TestPPPoESessionOpenFailureSendsNoPADS(t *testing.T) {
	admission := newVPPOrderAdmission(t, errors.New("VPP refused the PPPoE session add"))
	server := admission.server
	client := [EthALen]byte{2, 0, 0, 0, 0, 0x43}

	server.handlePADR(padrFrom(server, client, "", nil, nil))

	if admission.opens != 1 {
		t.Fatalf("provider Open called %d times, want once", admission.opens)
	}
	for _, event := range admission.log.snapshot() {
		if event == orderSessionCreated {
			t.Fatal("a refused Open recorded a created session")
		}
		if event == orderPADSSent {
			t.Fatal("a refused Open drew a PADS confirming a session")
		}
	}
	for _, frame := range admission.log.sent() {
		if frame.Code != CodePADS {
			t.Fatalf("a refused Open sent discovery code 0x%02x", frame.Code)
		}
		if frame.SID != 0 {
			t.Fatalf("refusal PADS SESSION_ID = 0x%04x, want 0x0000", frame.SID)
		}
		if frame.FindTag(TagACSystemError) == nil {
			t.Fatal("refusal PADS carries no AC-System-Error tag")
		}
	}
	if sessions := server.sessions.Sessions(); len(sessions) != 0 {
		t.Fatalf("a refused Open left %d sessions in the table", len(sessions))
	}
}

// TestPPPoEPADTAfterSessionDeletedOnServerTeardown -- spec-pppoe-vpp-dataplane AC-11.
//
// VALIDATES: handleSessionDown, the server entry every Ze-originated teardown
// reaches (onSessionDown after LCP Terminate, idle or an auth refusal, and
// closeInterface on reload), deletes the VPP session before the PADT leaves.
// METHOD: the session is admitted through handlePADR with the VPP-shaped
// owner; the PPP driver is never started, so only the server can delete the
// session and the order the log shows is the server's own.
// PREVENTS: a PADT sent while VPP still forwards the session. MUTATION: in
// finishSessionDown, move s.closeTransport(owner.opened) below s.sendFrame.
func TestPPPoEPADTAfterSessionDeletedOnServerTeardown(t *testing.T) {
	admission := newVPPOrderAdmission(t, nil)
	server := admission.server
	client := [EthALen]byte{2, 0, 0, 0, 0, 0x44}
	server.handlePADR(padrFrom(server, client, "", nil, nil))
	wantOrder(t, admission.log, orderSessionCreated, orderPADSSent)
	sid := admission.owner.sid

	server.handleSessionDown(server.sessions.Lookup(sid))

	wantOrder(t, admission.log, orderSessionCreated, orderPADSSent, orderSessionDeleted, orderPADTSent)
	padt := admission.log.sent()[1]
	if padt.SID != sid || padt.DstMAC != client {
		t.Fatalf("PADT SESSION_ID 0x%04x to %s, want 0x%04x to %s",
			padt.SID, net.HardwareAddr(padt.DstMAC[:]), sid, net.HardwareAddr(client[:]))
	}
}

// vppOrderRole is one ownership role of the shared VPP owner. Both roles
// close the same owner; the data-plane role, which the PPP driver closes,
// waits until the PADT has left, so the driver can never be the one that
// deleted the session first and the order the log shows is the server's.
// The wait has no timeout: inside the synctest bubble a server that never
// sends PADT leaves every goroutine blocked, which synctest reports as a
// deadlock instead of hanging.
type vppOrderRole struct {
	ppp.SessionHandle
	owner   *vppOrderOwner
	release <-chan struct{}
}

func (r *vppOrderRole) Identity() ppp.SessionIdentity { return r.owner.Identity() }
func (r *vppOrderRole) Close() error {
	if r.release != nil {
		<-r.release
	}
	return r.owner.Close()
}

// vppOrderTermination is one live session run by the real PPP driver over a
// VPP-shaped owner, with the subsystem's event consumer in reach.
type vppOrderTermination struct {
	sub      *Subsystem
	server   *InterfaceServer
	log      *vppOrderLog
	protocol *fixtureCurrentPPP
	sid      uint16
	client   net.HardwareAddr
}

// newVPPOrderTermination admits one session the way the LCP-termination RFC
// tests do (rfc2516AdmitTerminationSession), with the VPP-shaped owner as the
// original transport and data plane. It MUST run inside a synctest bubble.
func newVPPOrderTermination(t *testing.T) *vppOrderTermination {
	t.Helper()
	log := &vppOrderLog{}
	padtSent := make(chan struct{})
	var once sync.Once
	var unused [][]byte
	server := newRecordingServer(&unused, nil, 8)
	server.ifIndex = 1
	server.sendFrameFn = func(frame []byte) {
		if log.record(t, frame).Code == CodePADT {
			once.Do(func() { close(padtSent) })
		}
	}
	sub := newTestSubsystem(t, newRecordBus(), server.ifIndex)
	sub.servers[server.ifIndex] = server
	client := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	sid, err := server.sessions.AllocSID()
	if err != nil {
		t.Fatal(err)
	}
	session := &Session{SID: sid, MAC: client, IfName: "eth0", State: StateSession}
	if err := server.sessions.Add(session); err != nil {
		t.Fatal(err)
	}
	owner := newVPPOrderOwner(log, sid, [EthALen]byte(client))
	dataplaneRole := &vppOrderRole{owner: owner, release: padtSent}
	transportRole := &vppOrderRole{owner: owner}
	opened, err := dataplane.NewOpenedSession(owner, dataplaneRole, transportRole)
	if err != nil {
		t.Fatal(err)
	}
	session.Opened = opened
	protocol := fixtureCurrentStart(t, server, session, &fixtureCurrentStartConfig{method: ppp.AuthMethodNone})
	server.pppDriver = protocol.driver
	protocol.authenticate(t, &fixtureCurrentAuthConfig{})
	up := protocol.up(t)
	sub.handlePPPEvent(up)
	if !up.Control.Active() {
		t.Fatal("the admitted PPP worker is not live")
	}
	return &vppOrderTermination{sub: sub, server: server, log: log, protocol: protocol, sid: sid, client: client}
}

func (v *vppOrderTermination) assertPADT(t *testing.T) {
	t.Helper()
	wantOrder(t, v.log, orderSessionCreated, orderSessionDeleted, orderPADTSent)
	padt := v.log.sent()[0]
	if padt.SID != v.sid || net.HardwareAddr(padt.DstMAC[:]).String() != v.client.String() {
		t.Fatalf("PADT SESSION_ID 0x%04x to %s, want 0x%04x to %s",
			padt.SID, net.HardwareAddr(padt.DstMAC[:]), v.sid, v.client)
	}
	if v.server.sessions.Lookup(v.sid) != nil {
		t.Fatal("the terminated session is still in the table")
	}
}

// TestPPPoEPADTAfterSessionDeletedOnLCPTerminate -- spec-pppoe-vpp-dataplane AC-11.
//
// VALIDATES: an LCP Terminate-Request the real PPP driver receives ends the
// session through the subsystem's event consumer (EventSessionDown,
// onSessionDown, handleSessionDown), and the VPP session is deleted before
// the PADT leaves.
// METHOD: the driver runs the session over the fixtureCurrentStart wire with
// the VPP-shaped owner behind both roles. The driver's own close of the data
// plane is held until PADT has left, so only the server's transport close can
// delete the session first.
// PREVENTS: a PADT sent while VPP still forwards the session after LCP
// termination. MUTATION: in finishSessionDown, move
// s.closeTransport(owner.opened) below s.sendFrame.
func TestPPPoEPADTAfterSessionDeletedOnLCPTerminate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		v := newVPPOrderTermination(t)
		v.protocol.send(t, ppp.ProtoLCP, ppp.LCPTerminateRequest, 0x77, nil)
		lcpDown, ok := v.protocol.next(t).(ppp.EventLCPDown)
		if !ok {
			t.Fatal("Terminate-Request did not bring LCP down")
		}
		v.sub.handlePPPEvent(lcpDown)
		v.protocol.read(t, ppp.ProtoLCP, ppp.LCPTerminateAck)
		down, ok := v.protocol.next(t).(ppp.EventSessionDown)
		if !ok {
			t.Fatal("the driver reported no session down after LCP termination")
		}
		v.sub.handlePPPEvent(down)
		v.assertPADT(t)
	})
}

// TestPPPoEPADTAfterSessionDeletedOnInterfaceRemoval -- spec-pppoe-vpp-dataplane AC-11.
//
// VALIDATES: an operator who removes the access interface from the config
// ends its live session through the reload path (closeInterface), and the
// VPP session is deleted before the PADT leaves. The PPPoE subsystem has no
// operator "clear session" command and RADIUS Disconnect-Request is answered
// with a NAK for PPPoE (authradius coa.go handleDisconnect), so this is the
// operator-originated teardown that exists.
// METHOD: closeInterface runs on its own goroutine while the test feeds the
// driver's events to the subsystem consumer until it returns. The driver's
// data-plane close is held until PADT has left, as in the LCP case.
// PREVENTS: a PADT sent while VPP still forwards a session the reload ended.
// MUTATION: in finishSessionDown, move s.closeTransport(owner.opened) below
// s.sendFrame.
func TestPPPoEPADTAfterSessionDeletedOnInterfaceRemoval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		v := newVPPOrderTermination(t)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- v.sub.closeInterface(ctx, v.server) }()
		for {
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("closeInterface: %v", err)
				}
				v.assertPADT(t)
				return
			case event := <-v.protocol.driver.EventsOut():
				v.sub.handlePPPEvent(event)
			}
		}
	})
}
