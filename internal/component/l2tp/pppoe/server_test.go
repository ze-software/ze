// Design: docs/architecture/l2tp/followup-l2tp-call.md -- PPPoE subscriber relayed to L2TP.
//go:build linux

// Linux only: these tests read which descriptors handlePADR released, and
// closePPPoxFD closes a descriptor only on Linux (socket_other.go stubs it,
// since no other platform has a PPPoE session to release).

package pppoe

import (
	"errors"
	"log/slog"
	"sync"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

// precedenceSink binds the Service-Name "wholesale" AND reports relay-realm
// bindings, so a PADR for that service could be taken by either relay kind.
type precedenceSink struct {
	mu         sync.Mutex
	relays     []callsink.Request
	realmCalls int
}

func (p *precedenceSink) Relay(req callsink.Request) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.relays = append(p.relays, req)
	return req.Service == "wholesale", nil
}

func (p *precedenceSink) RelayRealm(callsink.RealmRequest) (callsink.RealmCall, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.realmCalls++
	return nil, false, nil
}

func (p *precedenceSink) RealmRelayConfigured() bool { return true }

// precedenceBackend satisfies NewProductionDriver's non-nil backend check.
// The driver is never started, so no method of it is called.
type precedenceBackend struct{ ppp.IfaceBackend }

// TestServiceNameRelayPrecedesRealm -- AC-7.
//
// VALIDATES: on a LAC that also has relay-realm bindings, a PADR whose
// Service-Name a relay binding names is relayed at PADS, as today: the sink's
// Service-Name relay is asked, the realm relay never is, and no local PPP
// session starts (no LCP, so no username and no realm to match).
// METHOD: the server is built the way boot builds it: the policy fields come
// from a parsed config through applyInterfaceConfig, and it holds a production
// PPP driver, so local termination is possible and the relay decision is what
// keeps it from starting. Only the kernel calls are faked. The driver is never
// started, so a StartSession handed to it stays queued and keeps the PPP unit
// descriptor open, while the relay path closes that descriptor itself. The
// unit is one end of a socket pair: EOF on the other end means the relay
// took the subscriber, and an open unit means local PPP was started.
// PREVENTS: realm relay taking (or delaying with local LCP) a subscriber a
// Service-Name binding already relays.
func TestServiceNameRelayPrecedesRealm(t *testing.T) {
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)
	sink := &precedenceSink{}
	callsink.Register(sink)

	p := extractFromConfigText(t, "pppoe { service-name wholesale; interface eth0 {} }")
	ic := &p.Interfaces[0]

	var nullFDs []int
	t.Cleanup(func() {
		for _, fd := range nullFDs {
			unix.Close(fd) //nolint:errcheck // test cleanup of /dev/null descriptors
		}
	})
	openNull := func() int {
		fd, err := unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
		if err != nil {
			t.Fatalf("open /dev/null: %v", err)
		}
		nullFDs = append(nullFDs, fd)
		return fd
	}
	unit, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	unitHeld := true
	t.Cleanup(func() {
		unix.Close(unit[1]) //nolint:errcheck // test cleanup
		if unitHeld {
			unix.Close(unit[0]) //nolint:errcheck // test cleanup: the queued StartSession is never run
		}
	})

	key := CookieKey{}
	hwAddr := [EthALen]byte{2, 0, 0, 0, 0, 1}
	srcMAC := [EthALen]byte{2, 0, 0, 0, 0, 2}
	srv := &InterfaceServer{
		ifName:      ic.Name,
		hwAddr:      hwAddr,
		sessions:    newSessionTable(ic.Name, ic.MaxSessions),
		cookieKey:   key,
		pppDriver:   ppp.NewProductionDriver(slog.Default(), precedenceBackend{}),
		logger:      slog.Default(),
		sendFrameFn: func([]byte) {},
		pppoeCreateFn: func(string, uint16, [EthALen]byte) (int, error) {
			return openNull(), nil
		},
		devPPPSetupFn: func(int) (int, int, int, error) {
			return openNull(), unit[0], 1, nil
		},
	}
	applyInterfaceConfig(srv, ic, &p)

	srv.handlePADR(&Packet{Code: CodePADR, SrcMAC: srcMAC, Tags: []Tag{
		{Type: TagACCookie, Value: GenerateCookie(key, hwAddr[:], srcMAC[:], nil)},
		{Type: TagServiceName, Value: []byte("wholesale")},
	}})

	unitHeld = !unitClosed(t, unit[1])

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.relays) != 1 {
		t.Fatalf("Service-Name relay asked %d times, want once", len(sink.relays))
	}
	if sink.relays[0].Service != "wholesale" {
		t.Fatalf("Service-Name relay asked for %q, want wholesale", sink.relays[0].Service)
	}
	if sink.realmCalls != 0 {
		t.Fatalf("realm relay asked %d times for a Service-Name relayed subscriber", sink.realmCalls)
	}
	if unitHeld {
		t.Fatal("a Service-Name relay subscriber started local PPP on a LAC with realm bindings")
	}
}

// unitClosed reports whether the far end of the socket pair holding peer was
// closed: a read answers EOF once it is, and EAGAIN while it is still open.
func unitClosed(t *testing.T, peer int) bool {
	t.Helper()
	if err := unix.SetNonblock(peer, true); err != nil {
		t.Fatalf("set nonblocking: %v", err)
	}
	var b [1]byte
	n, err := unix.Read(peer, b[:])
	if errors.Is(err, unix.EAGAIN) {
		return false
	}
	if err != nil {
		t.Fatalf("read unit peer: %v", err)
	}
	if n != 0 {
		t.Fatalf("unit peer read %d bytes, want EOF or nothing", n)
	}
	return true
}

// heldFD is one end of a socket pair handed to handlePADR as a session
// descriptor; the test keeps the other end and asks unitClosed whether
// handlePADR closed its end.
type heldFD struct {
	given int
	kept  int
	// released is set once closedByServer saw the given end closed, so the
	// cleanup never closes a descriptor number the server already closed.
	released bool
}

// newHeldFD opens a socket pair and registers the cleanup of both ends.
func newHeldFD(t *testing.T) *heldFD {
	t.Helper()
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	h := &heldFD{given: pair[0], kept: pair[1]}
	t.Cleanup(func() {
		unix.Close(h.kept) //nolint:errcheck // test cleanup
		if !h.released {
			unix.Close(h.given) //nolint:errcheck // test cleanup: the server kept this end
		}
	})
	return h
}

// closedByServer reports whether handlePADR closed the given end.
func (h *heldFD) closedByServer(t *testing.T) bool {
	t.Helper()
	if unitClosed(t, h.kept) {
		h.released = true
	}
	return h.released
}

// driverlessFixture is a LAC built the way boot builds it when no interface
// backend loaded: the policy fields come from a parsed config through
// applyInterfaceConfig, and the server holds no PPP driver. A precedenceSink
// is registered, so the box has a "wholesale" Service-Name relay and
// relay-realm bindings. The session socket, the PPP channel and the PPP unit
// are each one end of a socket pair the test watches.
type driverlessFixture struct {
	srv                *InterfaceServer
	sink               *precedenceSink
	pppox, chann, unit *heldFD
	sent               [][]byte
	hwAddr, srcMAC     [EthALen]byte
	key                CookieKey
}

func newDriverlessFixture(t *testing.T, config string) *driverlessFixture {
	t.Helper()
	callsink.Unregister()
	t.Cleanup(callsink.Unregister)
	f := &driverlessFixture{
		sink:   &precedenceSink{},
		pppox:  newHeldFD(t),
		chann:  newHeldFD(t),
		unit:   newHeldFD(t),
		hwAddr: [EthALen]byte{2, 0, 0, 0, 0, 1},
		srcMAC: [EthALen]byte{2, 0, 0, 0, 0, 2},
	}
	callsink.Register(f.sink)

	p := extractFromConfigText(t, config)
	ic := &p.Interfaces[0]
	f.srv = &InterfaceServer{
		ifName:      ic.Name,
		hwAddr:      f.hwAddr,
		sessions:    newSessionTable(ic.Name, ic.MaxSessions),
		cookieKey:   f.key,
		logger:      slog.Default(),
		sendFrameFn: func(frame []byte) { f.sent = append(f.sent, append([]byte(nil), frame...)) },
		pppoeCreateFn: func(string, uint16, [EthALen]byte) (int, error) {
			return f.pppox.given, nil
		},
		devPPPSetupFn: func(int) (int, int, int, error) {
			return f.chann.given, f.unit.given, 1, nil
		},
	}
	applyInterfaceConfig(f.srv, ic, &p)
	return f
}

// padr hands the server a valid PADR for service.
func (f *driverlessFixture) padr(service string) {
	f.srv.handlePADR(&Packet{Code: CodePADR, SrcMAC: f.srcMAC, Tags: []Tag{
		{Type: TagACCookie, Value: GenerateCookie(f.key, f.hwAddr[:], f.srcMAC[:], nil)},
		{Type: TagServiceName, Value: []byte(service)},
	}})
}

// onlyPADS parses the one frame the server sent, failing unless exactly one
// PADS was sent.
func (f *driverlessFixture) onlyPADS(t *testing.T) *Packet {
	t.Helper()
	if len(f.sent) != 1 {
		t.Fatalf("handlePADR sent %d frames, want exactly one PADS", len(f.sent))
	}
	pkt, err := ParseDiscovery(f.sent[0])
	if err != nil {
		t.Fatalf("ParseDiscovery of the reply: %v", err)
	}
	if pkt.Code != CodePADS {
		t.Fatalf("reply code 0x%02x, want PADS 0x%02x", pkt.Code, CodePADS)
	}
	return &pkt
}

// relayAsks returns the services the Service-Name relay was asked for and how
// often the realm relay was asked.
func (f *driverlessFixture) relayAsks() (services []string, realmCalls int) {
	f.sink.mu.Lock()
	defer f.sink.mu.Unlock()
	for _, req := range f.sink.relays {
		services = append(services, req.Service)
	}
	return services, f.sink.realmCalls
}

// TestServiceNameRelayNeedsNoPPPDriver -- defect: a LAC with no interface
// backend dropped the subscribers its Service-Name bindings relay.
//
// VALIDATES: with no PPP driver, a PADR whose Service-Name a relay binding
// names is still relayed: the relay is asked once, the subscriber gets one
// admitting PADS whose SESSION_ID the table holds, the unused PPP unit is
// closed, and the session socket and PPP channel stay open for the bridge.
// METHOD: driverlessFixture. Red when handlePADR checks for the driver before
// the relay decision: the relay is then never asked and no frame is sent.
// PREVENTS: a relay-only LAC, which needs no local PPP, refusing every
// subscriber because it has no interface backend.
func TestServiceNameRelayNeedsNoPPPDriver(t *testing.T) {
	f := newDriverlessFixture(t, "pppoe { service-name wholesale; interface eth0 {} }")

	f.padr("wholesale")

	services, realmCalls := f.relayAsks()
	if len(services) != 1 {
		t.Fatalf("Service-Name relay asked %d times with no PPP driver, want once", len(services))
	}
	if services[0] != "wholesale" {
		t.Fatalf("Service-Name relay asked for %q, want wholesale", services[0])
	}
	if realmCalls != 0 {
		t.Fatalf("realm relay asked %d times, want 0", realmCalls)
	}
	pads := f.onlyPADS(t)
	if pads.SID == 0 {
		t.Fatal("the relayed subscriber's PADS carries SESSION_ID 0, want the admitted session's")
	}
	if pads.FindTag(TagACSystemError) != nil {
		t.Fatal("the relayed subscriber's PADS carries AC-System-Error")
	}
	if f.srv.sessions.Lookup(pads.SID) == nil {
		t.Fatalf("the table holds no session %d for the relayed subscriber", pads.SID)
	}
	if !f.unit.closedByServer(t) {
		t.Fatal("the unused PPP unit of a relayed subscriber was left open")
	}
	if f.chann.closedByServer(t) {
		t.Fatal("the PPP channel the L2TP bridge needs was closed")
	}
	if f.pppox.closedByServer(t) {
		t.Fatal("the session socket the L2TP bridge needs was closed")
	}
}

// TestNoPPPDriverRefusesUnrelayedSubscriber -- defect: a path that needs the
// PPP driver refuses the subscriber on the wire.
//
// VALIDATES: with no PPP driver, a PADR no Service-Name binding relays is
// refused with one PADS carrying AC-System-Error and SESSION_ID 0: local
// termination needs the driver, and so does the realm relay, which decides
// only after local LCP and authentication. No session stays in the table,
// every descriptor handlePADR opened for it is closed, and the realm relay is
// never asked.
// METHOD: driverlessFixture with a service the sink does not bind. Red when
// handlePADR drops the session without a reply: no frame is sent.
// PREVENTS: a subscriber left retransmitting PADR with no answer, and the
// session socket and PPP channel leaked on the refusal.
func TestNoPPPDriverRefusesUnrelayedSubscriber(t *testing.T) {
	f := newDriverlessFixture(t, "pppoe { interface eth0 {} }")

	f.padr("retail")

	services, realmCalls := f.relayAsks()
	if len(services) != 1 {
		t.Fatalf("Service-Name relay asked %d times, want once", len(services))
	}
	if realmCalls != 0 {
		t.Fatalf("realm relay asked %d times with no PPP driver, want 0", realmCalls)
	}
	pads := f.onlyPADS(t)
	if pads.SID != 0 {
		t.Fatalf("refusing PADS carries SESSION_ID %d, want 0", pads.SID)
	}
	if pads.FindTag(TagACSystemError) == nil {
		t.Fatal("refusing PADS carries no AC-System-Error tag")
	}
	if got := f.srv.sessions.Count(); got != 0 {
		t.Fatalf("the table holds %d sessions after the refusal, want 0", got)
	}
	if !f.unit.closedByServer(t) {
		t.Fatal("the PPP unit of a refused subscriber was left open")
	}
	if !f.chann.closedByServer(t) {
		t.Fatal("the PPP channel of a refused subscriber was left open")
	}
	if !f.pppox.closedByServer(t) {
		t.Fatal("the session socket of a refused subscriber was left open")
	}
}
