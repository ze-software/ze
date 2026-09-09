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

// TestPADTRemovesOnlyTheNamedSession -- AC-5. A PADT names one session of a
// MAC that holds several: that session is torn down and the other stays
// reachable both by SID and by MAC.
func TestPADTRemovesOnlyTheNamedSession(t *testing.T) {
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x04}

	st := newSessionTable("eth0", 100)
	sid1, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid1: %v", err)
	}
	if err := st.Add(&Session{SID: sid1, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("Add sid1: %v", err)
	}
	sid2, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid2: %v", err)
	}
	if err := st.Add(&Session{SID: sid2, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("Add sid2: %v", err)
	}

	s := &InterfaceServer{ifName: "eth0", sessions: st, logger: slog.Default()}

	var src [EthALen]byte
	copy(src[:], mac)
	s.handlePADT(&Packet{Code: CodePADT, SID: sid1, SrcMAC: src})

	if s.sessions.Lookup(sid1) != nil {
		t.Error("PADT naming sid1 should have removed sid1")
	}
	if s.sessions.Lookup(sid2) == nil {
		t.Error("PADT naming sid1 must leave sid2 in place")
	}
	if got := s.sessions.sessionsByMAC(mac); len(got) != 1 || got[0].SID != sid2 {
		t.Errorf("sessionsByMAC after PADT = %v, want exactly [SID %d]", got, sid2)
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

// TestPADRAtPerMACCapAnswersError -- AC-1. A MAC already holding sessions
// equal to the per-interface per-MAC cap sends a further valid PADR (right
// cookie, right Service-Name): the AC must answer with a PADS carrying an
// error tag and session ID 0x0000, allocate no new session for it, and count
// the refusal so an operator can see it without a packet capture.
//
// This test was written RED against a stub admitPerMACCap that admitted
// everything (spec-pppoe-padr-replay-allocates-unbounded-sessions phase 1),
// and the red was exactly the shape below: no PPP driver is configured, so a
// PADR that reaches the allocation path returns from server.go's "no PPP
// driver" branch before building any frame, and `sent` stays empty. Deleting
// the admitPerMACCap call in handlePADR reproduces that red today, which is
// what makes the len(sent) == 1 assertion discriminate rather than merely
// pass.
func TestPADRAtPerMACCapAnswersError(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	previous := pppoeMetricsPtr.Swap(initPPPoEMetrics(reg))
	t.Cleanup(func() { pppoeMetricsPtr.Store(previous) })

	key := CookieKey{}
	hwAddr := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	srcMAC := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	mac := net.HardwareAddr(srcMAC[:])

	st := newSessionTable("eth0", 100)
	existingSID, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	// State is StateSession, not StateDiscovery, so the dedup branch above
	// this check does not match: this PADR is a genuine attempt at a second
	// session, which is what the per-MAC cap must judge.
	if err := st.Add(&Session{SID: existingSID, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	var sent [][]byte
	s := &InterfaceServer{
		ifName:            "eth0",
		hwAddr:            hwAddr,
		sessions:          st,
		cookieKey:         key,
		cookieTimeout:     30 * time.Second,
		maxSessionsPerMAC: 1,
		logger:            slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	cookie := GenerateCookie(key, hwAddr[:], srcMAC[:], nil)
	pkt := &Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: cookie},
			{Type: TagServiceName},
		},
	}

	s.handlePADR(pkt)

	if len(sent) != 1 {
		t.Fatalf("handlePADR sent %d frame(s) for a PADR at the per-MAC cap, want exactly 1 (a PADS carrying an error tag)", len(sent))
	}

	pads, err := ParseDiscovery(sent[0])
	if err != nil {
		t.Fatalf("ParseDiscovery(reply): %v", err)
	}
	if pads.Code != CodePADS {
		t.Errorf("Code = 0x%02x, want 0x%02x (PADS)", pads.Code, CodePADS)
	}
	if pads.SID != 0 {
		t.Errorf("SID = %d, want 0x0000", pads.SID)
	}
	if pads.FindTag(TagACSystemError) == nil {
		t.Error("reply is missing an error tag")
	}
	if got := st.Count(); got != 1 {
		t.Errorf("sessions.Count() = %d, want 1: no session must be allocated for a PADR refused at the per-MAC cap", got)
	}

	want := `ze_pppoe_discovery_refusals_total{reason="per-mac-cap-reached"} 1`
	if got := scrapeRefusals(t, reg, reasonPerMACCapReached); got != want {
		t.Errorf("scraped %q, want %q", got, want)
	}
}

// TestPADRReplayAfterPPPReturnsExistingSID -- AC-2. A MAC replays the exact
// PADR that already admitted its session, after that session has left
// StateDiscovery and is live in PPP (StateSession): the AC must answer with
// the existing session ID and allocate nothing new. "Replay" here is the
// cookie: the incoming PADR carries the same AC-Cookie bytes the admitting
// PADR did, which is what matchLiveCookie correlates against the session's
// stored Cookie (session.go).
func TestPADRReplayAfterPPPReturnsExistingSID(t *testing.T) {
	key := CookieKey{}
	hwAddr := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	srcMAC := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	mac := net.HardwareAddr(srcMAC[:])
	cookie := GenerateCookie(key, hwAddr[:], srcMAC[:], nil)

	st := newSessionTable("eth0", 100)
	existingSID, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	// State is StateSession (past discovery, live in PPP), and Cookie is the
	// same value the replay below presents: this is what "the session this
	// PADR already admitted" means to matchLiveCookie.
	if err := st.Add(&Session{
		SID: existingSID, MAC: mac, IfName: "eth0", PppoxFD: -1,
		State: StateSession, Cookie: append([]byte(nil), cookie...),
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	var sent [][]byte
	s := &InterfaceServer{
		ifName:            "eth0",
		hwAddr:            hwAddr,
		sessions:          st,
		cookieKey:         key,
		cookieTimeout:     30 * time.Second,
		maxSessionsPerMAC: 8,
		logger:            slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	pkt := &Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: cookie},
			{Type: TagServiceName},
		},
	}

	s.handlePADR(pkt)

	if len(sent) != 1 {
		t.Fatalf("handlePADR sent %d frame(s) for a replayed PADR after PPP, want exactly 1 (a PADS carrying the existing session ID)", len(sent))
	}

	pads, err := ParseDiscovery(sent[0])
	if err != nil {
		t.Fatalf("ParseDiscovery(reply): %v", err)
	}
	if pads.Code != CodePADS {
		t.Errorf("Code = 0x%02x, want 0x%02x (PADS)", pads.Code, CodePADS)
	}
	if pads.SID != existingSID {
		t.Errorf("SID = %d, want %d (the existing session, not a new one)", pads.SID, existingSID)
	}
	if got := st.Count(); got != 1 {
		t.Errorf("sessions.Count() = %d, want 1: a replayed PADR must not allocate a new session", got)
	}
}

// TestPADRReplayDuringTeardownIsNotDeduped -- R-2. A session the PPP
// event-consumer goroutine has already marked StateTeardown (handleSessionDown,
// server.go, via SessionTable.markTeardown) must not be handed back to a
// PADR replaying that session's cookie: the mitigation is "the dedup branch
// answers only sessions the table still holds ... a session in teardown is
// treated as absent." maxSessionsPerMAC is set well above 1 and no pppDriver
// is configured, so if the fix correctly treats the teardown session as
// absent, the PADR falls through toward a fresh admission attempt and sends
// nothing (the same "no PPP driver" branch TestPADRAtPerMACCapAnswersError's
// comment documents) -- any frame at all would have to be the wrongly-deduped
// PADS naming the dying session's real SID, so len(sent) == 0 is the proof.
func TestPADRReplayDuringTeardownIsNotDeduped(t *testing.T) {
	key := CookieKey{}
	hwAddr := [EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	srcMAC := [EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	mac := net.HardwareAddr(srcMAC[:])
	cookie := GenerateCookie(key, hwAddr[:], srcMAC[:], nil)

	st := newSessionTable("eth0", 100)
	teardownSID, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if err := st.Add(&Session{
		SID: teardownSID, MAC: mac, IfName: "eth0", PppoxFD: -1,
		State: StateSession, Cookie: append([]byte(nil), cookie...),
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Simulate what handleSessionDown does on the event-consumer goroutine
	// before this PADR (on the discovery-reader goroutine) is processed.
	if got := st.markTeardown(teardownSID); got == nil {
		t.Fatalf("markTeardown(%d): session not found", teardownSID)
	}

	var sent [][]byte
	s := &InterfaceServer{
		ifName:            "eth0",
		hwAddr:            hwAddr,
		sessions:          st,
		cookieKey:         key,
		cookieTimeout:     30 * time.Second,
		maxSessionsPerMAC: 8,
		logger:            slog.Default(),
		sendFrameFn: func(frame []byte) {
			sent = append(sent, frame)
		},
	}

	pkt := &Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: cookie},
			{Type: TagServiceName},
		},
	}

	s.handlePADR(pkt)

	if len(sent) != 0 {
		t.Fatalf("handlePADR sent %d frame(s) for a PADR replaying a session in teardown, want 0 (R-2: a dying session must not be handed back)", len(sent))
	}
	if got := st.sessionsByMAC(mac); len(got) != 1 || got[0].State != StateTeardown {
		t.Errorf("the teardown session must be left untouched by the refused replay, got %v", got)
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
