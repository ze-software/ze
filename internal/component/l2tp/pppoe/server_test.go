package pppoe

import (
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/metrics"
)

// RFC requirement: RFC2516-7-1 positive -- handlePADT tears a session down only when the PADT's SESSION_ID and source MAC both match the stored session; the session-data (0x8864) pairing is enforced by the kernel pppox socket bound to (sid, MAC) in pppoeCreate (kernel_linux.go:91).
// RFC requirement: RFC2516-7-1 negative -- a PADT whose source MAC does not match the session's MAC is rejected: the session is left intact.
func TestHandlePADTVerifiesMACAndSID(t *testing.T) {
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}

	newServer := func() (*InterfaceServer, uint16) {
		st := newSessionTable("eth0", 100)
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID: %v", err)
		}
		if err := st.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
			t.Fatalf("Add: %v", err)
		}
		return &InterfaceServer{ifName: "eth0", sessions: st, logger: slog.Default()}, sid
	}

	// Positive: matching SESSION_ID and source MAC tears the session down.
	s, sid := newServer()
	var src [EthALen]byte
	copy(src[:], mac)
	s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: src})
	if s.sessions.Lookup(sid) != nil {
		t.Error("PADT with matching SID and MAC should have removed the session")
	}

	// Negative: a mismatched source MAC leaves the session intact.
	s, sid = newServer()
	var wrong [EthALen]byte
	copy(wrong[:], net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x99})
	s.handlePADT(&Packet{Code: CodePADT, SID: sid, SrcMAC: wrong})
	if s.sessions.Lookup(sid) == nil {
		t.Error("PADT with a mismatched MAC must not remove the session")
	}
}

// TestPADIWithoutServiceNameIsServedAsAnyService -- AC-1. RFC 2516 Section
// 5.2: "If the Access Concentrator can not serve the PADI it MUST NOT respond
// with a PADO", and that MUST NOT is conditioned on being unable to serve the
// request, not on the packet's composition. With no service-name configured,
// a PADI carrying no Service-Name tag is a request for any service, and
// MatchServiceName already accepts it: this test pins the PADO going out,
// matching accel-ppp's and FreeBSD's behavior on receive.
func TestPADIWithoutServiceNameIsServedAsAnyService(t *testing.T) {
	var sent [][]byte
	s := &InterfaceServer{
		ifName:   "eth0",
		hwAddr:   [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		sessions: newSessionTable("eth0", 100),
		logger:   slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	pkt := &Packet{
		Code:   CodePADI,
		SrcMAC: [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
	}

	s.handlePADI(pkt)

	if len(sent) != 1 {
		t.Errorf("handlePADI sent %d frame(s) for a PADI with no Service-Name tag and no configured names, want exactly 1 (a PADO)", len(sent))
	}
}

// TestPADIWithoutServiceNameRefusedWhenNamesConfigured -- AC-2. Pins
// MatchServiceName's existing behavior against regression: RFC 2516 Section
// 5.2's "MUST NOT respond with a PADO" applies once the AC cannot serve the
// request, which is the case here because a tagless PADI cannot be matched
// against a non-empty allow-list.
func TestPADIWithoutServiceNameRefusedWhenNamesConfigured(t *testing.T) {
	var sent [][]byte
	s := &InterfaceServer{
		ifName:       "eth0",
		hwAddr:       [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF},
		sessions:     newSessionTable("eth0", 100),
		serviceNames: []string{"internet"},
		logger:       slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	pkt := &Packet{
		Code:   CodePADI,
		SrcMAC: [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
	}

	s.handlePADI(pkt)

	if len(sent) != 0 {
		t.Errorf("handlePADI sent %d frame(s) for a tagless PADI with names configured, want 0", len(sent))
	}
}

// TestPADRWithoutServiceNameGetsServiceNameError -- AC-3. RFC 2516 Section
// 5.3: "The PADR packet MUST contain exactly one TAG of TAG_TYPE
// Service-Name". Section 5.4: "If the Access Concentrator does not like the
// Service-Name in the PADR, then it MUST reply with a PADS containing a TAG
// of TAG_TYPE Service-Name-Error ... In this case the SESSION_ID MUST be set
// to 0x0000". A PADR missing the tag must be refused before any session is
// allocated, not admitted because the allow-list is empty.
//
// RFC requirement: RFC2516-5.4-2 positive -- a PADR whose Service-Name the AC will not serve is answered with exactly one frame, a PADS whose SESSION_ID is 0 and which carries a Service-Name-Error tag, and no session is allocated for it.
func TestPADRWithoutServiceNameGetsServiceNameError(t *testing.T) {
	key := CookieKey{}
	hwAddr := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	srcMAC := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	cookie := GenerateCookie(key, hwAddr[:], srcMAC[:], nil)

	st := newSessionTable("eth0", 100)
	var sent [][]byte
	s := &InterfaceServer{
		ifName:        "eth0",
		hwAddr:        hwAddr,
		sessions:      st,
		cookieKey:     key,
		cookieTimeout: 30 * time.Second,
		logger:        slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	pkt := &Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: cookie},
		},
	}

	s.handlePADR(pkt)

	if len(sent) != 1 {
		t.Fatalf("handlePADR sent %d frame(s) for a PADR with no Service-Name tag, want exactly 1 (a PADS carrying Service-Name-Error)", len(sent))
	}

	pads, err := ParseDiscovery(sent[0])
	if err != nil {
		t.Fatalf("ParseDiscovery(reply): %v", err)
	}
	if pads.Code != CodePADS {
		t.Errorf("Code = 0x%02x, want 0x%02x (PADS)", pads.Code, CodePADS)
	}
	if pads.SID != 0 {
		t.Errorf("SID = %d, want 0", pads.SID)
	}
	if pads.FindTag(TagSvcNameError) == nil {
		t.Error("reply is missing the Service-Name-Error tag")
	}
	if got := st.Count(); got != 0 {
		t.Errorf("sessions.Count() = %d, want 0: no session must be allocated for a refused PADR", got)
	}
}

// TestPADRRefusalIsCounted -- AC-10. A PADR refused for a missing
// Service-Name tag must be visible to an operator without a packet capture:
// this test proves the ze_pppoe_discovery_refusals_total counter carries
// exactly one increment under the service-name-missing label, using a real
// (non-global) Prometheus registry scraped through its own HTTP handler, the
// same path an operator's exporter uses.
//
// VALIDATES: AC-10, the refusal counter increments on the PADR path phase 2
// added (requireServiceNameTag).
// PREVENTS: a refusal branch that logs at Debug but never reaches an
// operator who has metrics scraping and no log access.
func TestPADRRefusalIsCounted(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	// The counter lives on a package-level pointer because the registry
	// arrives after Subsystem.Start on a PPPoE-only daemon. Swap in a
	// registry of this test's own and put the previous one back, so no other
	// test in the package sees this one's counts.
	previous := pppoeMetricsPtr.Swap(initPPPoEMetrics(reg))
	t.Cleanup(func() { pppoeMetricsPtr.Store(previous) })

	key := CookieKey{}
	hwAddr := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	srcMAC := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	cookie := GenerateCookie(key, hwAddr[:], srcMAC[:], nil)

	s := &InterfaceServer{
		ifName:        "eth0",
		hwAddr:        hwAddr,
		sessions:      newSessionTable("eth0", 100),
		cookieKey:     key,
		cookieTimeout: 30 * time.Second,
		logger:        slog.Default(),
		sendFrameFn:   func([]byte) {},
	}

	pkt := &Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: cookie},
		},
	}

	s.handlePADR(pkt)

	want := `ze_pppoe_discovery_refusals_total{reason="service-name-missing"} 1`
	if got := scrapeRefusals(t, reg, reasonServiceNameMissing); got != want {
		t.Errorf("scraped %q, want %q", got, want)
	}
}
