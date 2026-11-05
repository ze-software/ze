package ppp

import (
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDriverRejectsUnsupportedNetworkMRU reaches the driver's actual admission
// path; an invalid cap must not create a session or retain transferred files.
func TestDriverRejectsUnsupportedNetworkMRU(t *testing.T) {
	for _, tc := range []struct {
		name string
		mru  uint16
		ipv6 bool
	}{
		{"below IPv4", 67, false},
		{"below IPv6", 1279, true},
		{"above capacity", 1501, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reg := newPipeRegistry()
			installPipeRegistry(t, reg)
			pair := newPipePair(reg, 29001)
			defer closeConn(pair.peerEnd)
			ops, _, _ := newFakeOps()
			d := newTestDriverNoResponder(&fakeBackend{}, ops)
			start := &StartSession{TunnelID: 29, SessionID: 1, ChanFD: 29001, UnitFD: 29002,
				MaxMRU: tc.mru, DisableIPv6CP: !tc.ipv6}
			d.spawnSession(start)
			ev := waitForEvent(t, d.EventsOut(), time.Second)
			rejected, ok := ev.(EventSessionRejected)
			if !ok || !strings.Contains(rejected.Reason, "MRU") {
				t.Fatalf("event=%+v, want MRU rejection", ev)
			}
			if start.ChanFD != -1 || start.UnitFD != -1 || len(d.sessions) != 0 {
				t.Fatalf("refused admission retained descriptors or session: %+v", start)
			}
		})
	}
}

func TestPeerMRUBelowIPv4FloorIsNaked(t *testing.T) {
	for _, peerMRU := range []uint16{64, 67} {
		s, rec, _ := newRFC1661Session(LCPStateReqSent)
		s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 9, Data: optStream(mruOpt(peerMRU))})
		packets := decodeFrames(t, rec)
		last := packets[len(packets)-1].Pkt
		opts, err := ParseLCPOptions(last.Data)
		if err != nil {
			t.Fatal(err)
		}
		mru, present := lookupMRUOption(opts)
		if last.Code != LCPConfigureNak || last.Identifier != 9 || !present || mru != 68 {
			t.Fatalf("MRU%d got %+v MRU%d, want matching Nak68", peerMRU, last, mru)
		}
	}
	// The minimum supported IPv4 peer remains useful, rather than making a
	// blanket refusal satisfy the small-peer negative cases.
	s, rec, _ := newRFC1661Session(LCPStateReqSent)
	s.handleLCPPacket(LCPPacket{Code: LCPConfigureRequest, Identifier: 10, Data: optStream(mruOpt(68))})
	packets := decodeFrames(t, rec)
	last := packets[len(packets)-1].Pkt
	if last.Code != LCPConfigureAck || last.Identifier != 10 {
		t.Fatalf("minimum IPv4 peer got %+v, want matching Ack", last)
	}
}

// A correlated peer Nak can request a smaller local receive MRU, but cannot
// make Ze advertise an Information field too small for supported IP service.
func TestPeerNakCannotLowerLocalMRUBelowIPv4Floor(t *testing.T) {
	for _, suggestion := range []uint16{64, 65, 67, 68} {
		t.Run(strconv.Itoa(int(suggestion)), func(t *testing.T) {
			s, rec, _ := newRFC1661Session(LCPStateReqSent)
			t.Cleanup(s.stopRestartTimer)
			s.maxMRU = 1492
			s.sendConfigureRequest()
			request := lastLCPConfigureRequest(t, rec)
			s.handleLCPPacket(LCPPacket{
				Code: LCPConfigureNak, Identifier: request.Identifier,
				Data: optStream(mruOpt(suggestion)),
			})
			retry := lastLCPConfigureRequest(t, rec)
			if retry.Identifier == request.Identifier {
				t.Fatal("correlated Nak did not produce a replacement Configure-Request")
			}
			opts, err := ParseLCPOptions(retry.Data)
			if err != nil {
				t.Fatal(err)
			}
			mru, present := lookupMRUOption(opts)
			want := uint16(1492)
			if suggestion == 68 {
				want = 68
			}
			if !present || mru != want || mru < 68 {
				t.Fatalf("correlated Nak%d advertised MRU%d present=%v, want%d", suggestion, mru, present, want)
			}
		})
	}
}

func TestIPv6MRUServiceGate(t *testing.T) {
	for _, tc := range []struct {
		name  string
		peer  uint16
		local uint16
		cap   uint16
		want  bool
	}{
		{"defaults", 0, 0, 1500, true},
		{"exact IPv6 floor", 1280, 1280, 1280, true},
		{"smaller peer", 1279, 1500, 1500, false},
		{"smaller local request", 1500, 1279, 1500, false},
		{"smaller session cap", 1500, 1500, 1279, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, events := newRFC1661Session(LCPStateOpened)
			s.negotiatedMRU, s.localMRU, s.maxMRU = tc.peer, tc.local, tc.cap
			s.disableIPv6CP = false
			if got := s.ipv6MRUSupported(); got != tc.want {
				t.Fatalf("IPv6 MRU support=%v, want%v", got, tc.want)
			}
			for _, proto := range []uint16{ProtoIPv6CP, ProtoIPv6} {
				if got := s.supportsProtocol(proto); got != tc.want {
					t.Fatalf("protocol0x%04x support=%v, want%v", proto, got, tc.want)
				}
			}
			if tc.want {
				return
			}
			if s.runNCPPhase() {
				t.Fatal("IPv6-only session with unsupported MRU reported usable service")
			}
			if len(s.ipEventsOut) != 0 {
				t.Fatal("unsupported IPv6 size reached address allocation")
			}
			if _, ok := (<-events).(EventSessionDown); !ok {
				t.Fatal("unusable IPv6-only session did not report its failure")
			}
			s.peerInterfaceIDNegotiated = true
			s.peerInterfaceID = ipv6cpTestPeerID
			if !s.onNCPOpened(AddressFamilyIPv6) || len(events) != 0 {
				t.Fatal("unsupported IPv6 size published an assigned address")
			}
		})
	}
}

// The peer's small receive limit does not remove independently useful IPv4.
// A later peer IPv6CP packet must not reopen the refused IPv6 lifetime.
func TestSmallPeerMRURetainsIPv4(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	driverUnit, peerUnit := net.Pipe()
	previousUnit := SetNewUnitFileForTest(func(int) io.ReadCloser { return driverUnit })
	t.Cleanup(func() { RestoreNewUnitFile(previousUnit) })
	pair := newPipePair(reg, 29101)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	go autoAcceptIP(d)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(peerUnit)
		closeConn(pair.peerEnd)
		d.Stop()
	})
	local := optStream(mruOpt(1500), magicOpt(0x11223344))
	peer := optStream(mruOpt(128), magicOpt(0x55667788))
	d.SessionsIn() <- StartSession{
		TunnelID: 29, SessionID: 2, ChanFD: 29101, UnitFD: 29102, UnitNum: 42,
		MaxMRU: 1500, IPTimeout: 5 * time.Second,
		ProxyLCPInitialRecv: peer, ProxyLCPLastSent: local, ProxyLCPLastRecv: peer,
	}
	if _, ok := waitForEvent(t, d.EventsOut(), time.Second).(EventLCPUp); !ok {
		t.Fatal("valid IPv4-capable proxy was not admitted")
	}
	td := &ncpTestDriver{t: t, driver: d, backend: backend, peer: pair.peerEnd}
	td.completeIPCP(t)
	if event, ok := td.waitForEvent(t, time.Second).(EventSessionIPAssigned); !ok || event.Family != AddressFamilyIPv4 {
		t.Fatal("small-MRU IPv4 service did not publish its IPv4 address")
	}
	if _, ok := td.waitForEvent(t, time.Second).(EventSessionUp); !ok {
		t.Fatal("small-MRU IPv4 session did not become usable")
	}
	calls := backend.MTUCalls()
	if len(calls) != 1 || calls[0].mtu != 128 {
		t.Fatalf("IPv4 interface MTU=%+v, want128", calls)
	}
	// Linux delivers network-control protocols on the unit after connect;
	// the refusal must return through the channel without starting IPv6CP.
	writeUnitNCPPacket(t, peerUnit, ProtoIPv6CP, LCPConfigureRequest, 11, []byte{1, 10, 2, 0, 0, 0, 0, 0, 0, 1})
	if reply := td.readPeerNCPPacket(t, ProtoLCP); reply.Code != LCPProtocolReject ||
		len(reply.Data) < 2 || binary.BigEndian.Uint16(reply.Data[:2]) != ProtoIPv6CP {
		t.Fatalf("refused IPv6CP reopened or was not rejected: %+v", reply)
	}
}

func TestIPv6MinimumMRUCompletesNCP(t *testing.T) {
	reg := newPipeRegistry()
	installPipeRegistry(t, reg)
	pair := newPipePair(reg, 29201)
	backend := &fakeBackend{}
	ops, _, _ := newFakeOps()
	d := makeTestDriver(backend, ops)
	go autoAcceptIP(d)
	if err := d.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeConn(pair.peerEnd)
		d.Stop()
	})
	local := optStream(mruOpt(1280), magicOpt(0x11223344))
	peer := optStream(mruOpt(1280), magicOpt(0x55667788))
	d.SessionsIn() <- StartSession{
		TunnelID: 29, SessionID: 3, ChanFD: 29201, UnitFD: 29202, UnitNum: 42,
		MaxMRU: 1280, DisableIPCP: true, IPTimeout: 5 * time.Second,
		ProxyLCPInitialRecv: peer, ProxyLCPLastSent: local, ProxyLCPLastRecv: peer,
	}
	if _, ok := waitForEvent(t, d.EventsOut(), time.Second).(EventLCPUp); !ok {
		t.Fatal("minimum-MRU IPv6 session was not admitted")
	}
	td := &ncpTestDriver{t: t, driver: d, backend: backend, peer: pair.peerEnd}
	td.completeIPv6CP(t)
	if event, ok := td.waitForEvent(t, time.Second).(EventSessionIPAssigned); !ok || event.Family != AddressFamilyIPv6 {
		t.Fatal("minimum-MRU IPv6 session did not publish its address")
	}
	if _, ok := td.waitForEvent(t, time.Second).(EventSessionUp); !ok {
		t.Fatal("minimum-MRU IPv6 session did not become usable")
	}
	calls := backend.MTUCalls()
	if len(calls) != 1 || calls[0].mtu != 1280 {
		t.Fatalf("IPv6 interface MTU=%+v, want1280", calls)
	}
}
