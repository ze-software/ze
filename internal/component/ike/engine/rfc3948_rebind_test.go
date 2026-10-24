package engine

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// The four rebind log lines (rebind.go). The interop checker matches the same texts.
const (
	rbLogMigrated = "ike: peer endpoint moved, Child SAs migrated to the new address and port"
	rbLogRekeyed  = "ike: peer endpoint moved, Child SA cannot be migrated here, so it is rekeyed onto the new address and port"
	rbLogRefused  = "ike: peer endpoint moved, migrating the Child SA failed and the old endpoints stay in place, so it is rekeyed onto the new address and port"
	rbLogLost     = "ike: peer endpoint moved, the Child SA was lost while migrating it, so the IKE SA is closed and set up again."
)

// rbDataplane is a TunnelMigrator that records every migration and answers err.
type rbDataplane struct {
	mockDP
	err        error
	migrations []dataplane.TunnelMigration
}

func (dp *rbDataplane) MigrateTunnel(m dataplane.TunnelMigration) error {
	dp.migrations = append(dp.migrations, m)
	return dp.err
}

type rbOptions struct {
	migrator   bool
	migrateErr error
	transport  bool
}

// rbFixture is an established, NAT-detected IKE SA with one installed Child SA, its
// peer on a NAT-T socket (peerTr), and a second peer socket (movedTr) standing for the
// public port a NAT rebinding moves the peer to. MOBIKE is not negotiated and this node
// is not behind a NAT, so an authenticated message from movedTr is an adopted move.
type rbFixture struct {
	local, peer      *SA
	ps               *PeerSession
	child, peerChild *ChildSA
	dp               dataplane.Dataplane
	mock             *mockDP
	migrator         *rbDataplane
	myTr             *transport.UDPTransport
	peerTr, movedTr  *transport.UDPTransport
	myPort, oldPort  uint16
	movedPort        uint16
	log              *slog.Logger
	logs             *bytes.Buffer
}

func rbOwner(t *testing.T, o rbOptions) *rbFixture {
	t.Helper()
	discard := slogutil.DiscardLogger()
	ini, resp, _ := establishPSK(t)
	f := &rbFixture{local: ini, peer: resp, logs: &bytes.Buffer{}}
	f.log = slog.New(slog.NewTextHandler(f.logs, nil))
	f.peerTr = mbTransport(t, "127.0.0.1:0", true)
	f.movedTr = mbTransport(t, "127.0.0.1:0", true)
	f.myTr = mbTransport(t, "127.0.0.1:0", true)
	f.myPort = uint16(nttPeerAddr(t, f.myTr).Port)
	f.oldPort = uint16(nttPeerAddr(t, f.peerTr).Port)
	f.movedPort = uint16(nttPeerAddr(t, f.movedTr).Port)
	f.local.bindSockets(nil, f.myTr)
	f.local.floatToNATTPort()
	f.local.NATDetected = true
	f.local.UseTransportMode = o.transport
	f.local.peerEndpoint = nttPeerAddr(t, f.peerTr)
	if o.migrator {
		f.migrator = &rbDataplane{err: o.migrateErr}
		f.dp, f.mock = f.migrator, &f.migrator.mockDP
		f.local.mobike.canMigrate = true
	} else {
		f.mock = &mockDP{}
		f.dp = f.mock
	}
	child, err := createFirstChildSA(f.local, testESPGroup(),
		f.local.PeerCfg.LocalAddress, f.local.PeerCfg.RemoteAddress, 7, f.dp, discard)
	if err != nil {
		t.Fatalf("install initial Child SA: %v", err)
	}
	peerChild, err := createFirstChildSA(f.peer, testESPGroup(),
		f.peer.PeerCfg.LocalAddress, f.peer.PeerCfg.RemoteAddress, 0, nil, discard)
	if err != nil {
		t.Fatalf("peer initial Child SA: %v", err)
	}
	// Both ends of one Child SA run one mode. A transport-mode rekey asks for transport
	// with USE_TRANSPORT_MODE, and ze refuses an answer from a tunnel-mode peer (RFC 7296
	// Section 1.3.1), so the peer's Child SA runs transport mode too. Only the Child SA's
	// mode is set: the peer's selectors stay the fixture's wildcard scope, which the
	// rekey narrows exactly as before.
	if o.transport {
		peerChild.Mode = modeTransport
	}
	f.child, f.peerChild = child, peerChild
	// The configured 10.0.0.1 is no address of this host, so sends would leave from
	// nowhere. The Child SAs above keep the configured tunnel addresses.
	f.local.PeerCfg.LocalAddress = "127.0.0.1"
	f.ps = &PeerSession{peerName: f.local.PeerName, peerCfg: f.local.PeerCfg, espGroup: testESPGroup()}
	f.ps.setChildSA(child)
	if child.udpRemotePort != f.oldPort {
		t.Fatalf("setup: Child SA encapsulates to %d, want the IKE peer port %d", child.udpRemotePort, f.oldPort)
	}
	return f
}

// dpd delivers the peer's DPD, an empty authenticated INFORMATIONAL request, from the
// socket from, and drains ze's answer when one is sent.
func (f *rbFixture) dpd(t *testing.T, from *transport.UDPTransport, corrupt bool) ownedOutcome {
	t.Helper()
	raw := mbInformational(t, f.peer, f.local.ExpectedMsgID, false, nil)
	if corrupt {
		raw[len(raw)-1] ^= 0xFF
	}
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: raw, RemoteAddr: nttPeerAddr(t, from), LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, f.log)
	if !corrupt && !out.reestablish {
		rbDrain(from)
	}
	return out
}

// rbDrain takes ze's answer off the peer socket, so the next read sees the next
// request. A missing answer is not this helper's assertion.
func rbDrain(tr *transport.UDPTransport) {
	select {
	case <-tr.Recv():
	case <-time.After(rtxArrive):
	}
}

// tick runs the owner tick's Child SA rekey trigger with no lifetime due, so only an
// owed rekey can raise it.
func (f *rbFixture) tick() {
	f.ps.serviceChildRekeyTrigger(f.local, nil, time.Now(), f.myTr, f.log)
}

// answerRekey has the peer answer ze's Child SA rekey request, read off the moved
// socket, and delivers the answer to ze from there.
func (f *rbFixture) answerRekey(t *testing.T) {
	t.Helper()
	req := mbReceive(t, f.movedTr)
	msg := parseMsg(t, req.Data)
	if msg.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("the owed rekey sent exchange %d, want CREATE_CHILD_SA", msg.Header.ExchangeType)
	}
	answer, replacement, err := respondChildRekey(f.peer, mbDecrypt(t, f.peer, req.Data),
		f.peerChild, msg.Header.MessageID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("peer answers the Child SA rekey: %v", err)
	}
	f.peerChild = replacement
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: nttPeerAddr(t, f.movedTr), LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, f.log)
	if out.reestablish {
		t.Fatal("the rekey answer closed the IKE SA")
	}
	if f.ps.getChildSA() == f.child {
		t.Fatal("the rekey answer installed no replacement Child SA")
	}
}

// wantReplacementOnMovedPort asserts the pair installed after the first one encapsulates
// from this node's NAT-T port to the moved peer port, outbound and inbound.
func (f *rbFixture) wantReplacementOnMovedPort(t *testing.T) {
	t.Helper()
	if len(f.mock.sas) != 4 {
		t.Fatalf("installed SA count = %d, want the first pair and one replacement pair", len(f.mock.sas))
	}
	in, out := encapPortsByDir(t, f.mock.sas[2:])
	if want := [2]uint16{f.myPort, f.movedPort}; out != want {
		t.Fatalf("replacement outbound encap sport/dport = %v, want %v", out, want)
	}
	if want := [2]uint16{f.movedPort, f.myPort}; in != want {
		t.Fatalf("replacement inbound encap sport/dport = %v, want %v", in, want)
	}
}

func (f *rbFixture) wantLog(t *testing.T, line string) {
	t.Helper()
	if !strings.Contains(f.logs.String(), line) {
		t.Fatalf("log lacks %q; got:\n%s", line, f.logs.String())
	}
}

func (f *rbFixture) wantNoRebindLog(t *testing.T) {
	t.Helper()
	for _, line := range []string{rbLogMigrated, rbLogRekeyed, rbLogRefused, rbLogLost} {
		if strings.Contains(f.logs.String(), line) {
			t.Fatalf("log carries %q though no rebind was due; got:\n%s", line, f.logs.String())
		}
	}
}

// VALIDATES: AC-11. After an adopted NAT rebinding on a tunnel-mode Child SA over a
// backend with a TunnelMigrator, the current pair and a distinct superseded pair are each
// migrated once to the moved peer port, from this node's NAT-T port, keeping their SPIs,
// and no Child SA rekey is started.
// PREVENTS: installed states left encapsulating to the port the NAT no longer maps.
//
// RFC requirement: RFC3948-2.1-2 positive -- after the peer's IKE port moves, the
// migrated Child SA states carry the new IKE port as their encapsulation port.
func TestRFC3948RebindMigratesTunnelChildSAs(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true})
	superseded := *f.child
	superseded.InboundSPI++
	superseded.OutboundSPI++
	f.ps.supersededChild = &superseded

	f.dpd(t, f.movedTr, false)

	if len(f.migrator.migrations) != 2 {
		t.Fatalf("migrations = %d, want the current and the superseded pair", len(f.migrator.migrations))
	}
	for i, spi := range []uint32{f.child.InboundSPI, superseded.InboundSPI} {
		m := f.migrator.migrations[i]
		if m.InboundSPI != spi {
			t.Fatalf("migration %d moved inbound SPI %#x, want %#x", i, m.InboundSPI, spi)
		}
		if m.RemotePort != f.movedPort {
			t.Fatalf("migration %d remote port = %d, want the moved IKE port %d", i, m.RemotePort, f.movedPort)
		}
		if m.LocalPort != f.myPort {
			t.Fatalf("migration %d local port = %d, want the NAT-T socket port %d", i, m.LocalPort, f.myPort)
		}
	}
	if f.child.udpRemotePort != f.movedPort {
		t.Fatalf("the migrated Child SA records remote port %d, want %d", f.child.udpRemotePort, f.movedPort)
	}
	if len(f.mock.sas) != 2 {
		t.Fatalf("migration reinstalled states: %d installed, want the first pair only", len(f.mock.sas))
	}
	f.wantLog(t, rbLogMigrated)
	f.tick()
	if f.ps.pendingRekey != nil {
		t.Fatal("a migrated Child SA was also rekeyed")
	}
}

// VALIDATES: AC-13. With no TunnelMigrator, the move marks a rekey owed, the next tick
// sends the Child SA rekey, and the rekeyed pair installs on the moved peer port.
// PREVENTS: a Child SA left on the old port where the dataplane cannot migrate it.
//
// RFC requirement: RFC3948-2.1-2 positive -- where no migration is possible, the rekeyed
// Child SA's encapsulation ports are the moved IKE ports.
func TestRFC3948RebindRekeysWithoutAMigrator(t *testing.T) {
	f := rbOwner(t, rbOptions{})
	f.dpd(t, f.movedTr, false)
	if !f.ps.rebindRekeyOwed {
		t.Fatal("the move left no rekey owed")
	}
	f.wantLog(t, rbLogRekeyed)
	f.wantLog(t, `reason="`+rebindReasonDataplane+`"`)

	f.tick()
	f.answerRekey(t)
	f.wantReplacementOnMovedPort(t)
	if f.ps.rebindRekeyOwed {
		t.Fatal("the installed rekey did not pay the owed rekey")
	}
}

// VALIDATES: AC-14. A transport-mode Child SA is rekeyed, never migrated, even with a
// TunnelMigrator present, and the rekeyed pair lands on the moved peer port.
// PREVENTS: a transport-mode state handed to the tunnel migrator.
//
// RFC requirement: RFC3948-2.1-2 positive -- a transport-mode Child SA follows the moved
// IKE port through its rekey.
func TestRFC3948RebindRekeysTransportChildSA(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true, transport: true})
	f.dpd(t, f.movedTr, false)
	if len(f.migrator.migrations) != 0 {
		t.Fatalf("a transport-mode Child SA was migrated %d times", len(f.migrator.migrations))
	}
	f.wantLog(t, `reason="`+rebindReasonTransport+`"`)
	f.tick()
	f.answerRekey(t)
	f.wantReplacementOnMovedPort(t)
}

// VALIDATES: AC-15. A migration that fails with anything but ErrTunnelMigrationLost
// leaves the old endpoints standing, logs the row 3 line, and the owed rekey follows.
// Both shapes are driven: an ErrNotSupported-wrapped error and one that wraps nothing,
// the shape of the local-port refusal.
// PREVENTS: a branch testing for ErrNotSupported, which misses most refusals.
func TestRFC3948RebindRekeysWhenMigrationRefused(t *testing.T) {
	for name, cause := range map[string]error{
		"not-supported": fmt.Errorf("xfrm migrate: IPv6 endpoint: %w", dataplane.ErrNotSupported),
		"no-sentinel":   errors.New("xfrm migrate: local port 4501 is not 4500"),
	} {
		t.Run(name, func(t *testing.T) {
			f := rbOwner(t, rbOptions{migrator: true, migrateErr: cause})
			f.dpd(t, f.movedTr, false)
			if len(f.migrator.migrations) == 0 {
				t.Fatal("no migration was attempted")
			}
			if f.child.udpRemotePort != f.oldPort {
				t.Fatalf("a refused migration moved the Child SA record to port %d", f.child.udpRemotePort)
			}
			f.wantLog(t, rbLogRefused)
			if f.local.State == StateDead {
				t.Fatal("a refused migration closed the IKE SA")
			}
			f.tick()
			f.answerRekey(t)
			f.wantReplacementOnMovedPort(t)
		})
	}
}

// VALIDATES: AC-16. ErrTunnelMigrationLost closes the IKE SA for re-establishment, as
// MOBIKE does: no key is reinstalled and no rekey is owed.
// PREVENTS: reinstalling keys the TunnelMigrator contract says are lost.
func TestRFC3948RebindRetiresALostChildSA(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true,
		migrateErr: fmt.Errorf("xfrm migrate: %w", dataplane.ErrTunnelMigrationLost)})
	out := f.dpd(t, f.movedTr, false)
	if !out.reestablish {
		t.Fatal("a lost tunnel did not ask for re-establishment")
	}
	if f.local.State != StateDead {
		t.Fatalf("IKE SA state = %s, want dead", f.local.State)
	}
	if f.ps.rebindRekeyOwed {
		t.Fatal("a lost tunnel owes a rekey")
	}
	if len(f.mock.sas) != 2 {
		t.Fatalf("a lost tunnel reinstalled states: %d installed", len(f.mock.sas))
	}
	f.wantLog(t, rbLogLost)
}

// VALIDATES: AC-17, the negative polarity. No migration, no owed rekey and no rebind
// log line when the endpoint did not move, when MOBIKE is negotiated, when this node
// is behind a NAT, or when the datagram from the new endpoint fails authentication.
// PREVENTS: a rebind driven by anything but an adopted move.
//
// RFC requirement: RFC3948-2.1-2 negative -- with no adopted move of the IKE port, the
// Child SA encapsulation ports do not change.
func TestRFC3948RebindNotRunWithoutAnAdoptedMove(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(*rbFixture)
		from    func(*rbFixture) *transport.UDPTransport
		corrupt bool
	}{
		{"same-endpoint", func(*rbFixture) {}, func(f *rbFixture) *transport.UDPTransport { return f.peerTr }, false},
		{"mobike", func(f *rbFixture) {
			f.local.mobike.enabled = true
			f.local.mobike.local = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(f.myPort)}
		}, func(f *rbFixture) *transport.UDPTransport { return f.movedTr }, false},
		{"behind-nat", func(f *rbFixture) { f.local.BehindNAT = true },
			func(f *rbFixture) *transport.UDPTransport { return f.movedTr }, false},
		{"unauthenticated", func(*rbFixture) {}, func(f *rbFixture) *transport.UDPTransport { return f.movedTr }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := rbOwner(t, rbOptions{migrator: true})
			tc.setup(f)
			f.dpd(t, tc.from(f), tc.corrupt)
			if len(f.migrator.migrations) != 0 {
				t.Fatalf("migrated %d times with no adopted move", len(f.migrator.migrations))
			}
			if f.ps.rebindRekeyOwed {
				t.Fatal("a rekey is owed with no adopted move")
			}
			if f.child.udpRemotePort != f.oldPort {
				t.Fatalf("Child SA remote port = %d, want %d", f.child.udpRemotePort, f.oldPort)
			}
			f.wantNoRebindLog(t)
			f.tick()
			if f.ps.pendingRekey != nil {
				t.Fatal("a rekey started with no adopted move")
			}
		})
	}
}

// VALIDATES: AC-18. A held request window, a TEMPORARY_FAILURE hold and a refusal hold
// each defer the owed rekey without dropping it; once the hold ends the next tick sends
// it. A peer-initiated rekey pays the owed rekey too.
// PREVENTS: an edge-triggered owed rekey lost to the first hold it meets.
func TestRFC3948RebindRekeyOwedSurvivesAHold(t *testing.T) {
	holds := []struct {
		name         string
		hold, unhold func(*rbFixture)
	}{
		{"window", func(f *rbFixture) {
			if !f.local.reserveRequestWindow() {
				t.Fatal("setup: the request window was already held")
			}
		}, func(f *rbFixture) { f.local.releaseRequestWindow() }},
		{"temporary-failure", func(f *rbFixture) { f.ps.childRekeyHoldUntil = time.Now().Add(time.Hour) },
			func(f *rbFixture) { f.ps.childRekeyHoldUntil = time.Time{} }},
		{"refused", func(f *rbFixture) { f.ps.childRekeyRefusedUntil = time.Now().Add(time.Hour) },
			func(f *rbFixture) { f.ps.childRekeyRefusedUntil = time.Time{} }},
	}
	for _, h := range holds {
		t.Run(h.name, func(t *testing.T) {
			f := rbOwner(t, rbOptions{})
			f.dpd(t, f.movedTr, false)
			h.hold(f)
			f.tick()
			if f.ps.pendingRekey != nil {
				t.Fatal("the owed rekey ran through the hold")
			}
			if !f.ps.rebindRekeyOwed {
				t.Fatal("the hold dropped the owed rekey")
			}
			h.unhold(f)
			f.tick()
			f.answerRekey(t)
			f.wantReplacementOnMovedPort(t)
			if f.ps.rebindRekeyOwed {
				t.Fatal("the installed rekey did not pay the owed rekey")
			}
		})
	}
	t.Run("peer-initiated", func(t *testing.T) {
		f := rbOwner(t, rbOptions{})
		f.dpd(t, f.movedTr, false)
		f.peer.NextMsgID = f.local.ExpectedMsgID
		req, _, err := initiateChildRekey(f.peer, f.peerChild)
		if err != nil {
			t.Fatalf("peer starts a Child SA rekey: %v", err)
		}
		f.ps.handleOwnedInbound(f.local, transport.Packet{
			Data: req, RemoteAddr: nttPeerAddr(t, f.movedTr), LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
		}, f.myTr, f.dp, f.log)
		if f.ps.getChildSA() == f.child {
			t.Fatal("ze installed no replacement for the peer's rekey")
		}
		f.wantReplacementOnMovedPort(t)
		if f.ps.rebindRekeyOwed {
			t.Fatal("the peer's rekey did not pay the owed rekey")
		}
	})
}

// VALIDATES: AC-18, the two ends of a parallel IKE SA. A move while a pending child is
// set migrates nothing and owes a rekey; promoting the pending child clears it, so the
// next tick sends no rekey. Reaping a pending SA that never authenticated keeps the
// current pair on the old ports, so the owed rekey still runs.
// PREVENTS: a rekey of a freshly promoted child, and a reap that forgets the debt.
func TestRFC3948RebindOwedRekeyClearsOnPromotion(t *testing.T) {
	t.Run("promoted", func(t *testing.T) {
		f := rbOwner(t, rbOptions{migrator: true})
		pending := &SA{PeerName: f.local.PeerName, State: StateEstablished}
		f.ps.setPendingSA(pending)
		f.ps.setPendingChild(&ChildSA{InboundSPI: 0xA1, OutboundSPI: 0xB1})
		f.dpd(t, f.movedTr, false)
		if len(f.migrator.migrations) != 0 {
			t.Fatal("the retiring SA migrated while a pending child exists")
		}
		if !f.ps.rebindRekeyOwed {
			t.Fatal("the move with a pending child owes no rekey")
		}
		f.wantLog(t, `reason="`+rebindReasonPending+`"`)
		if got := f.ps.resolvePendingAfterOwnerLoop(nil, f.dp, nil, f.log); got != pendingContinue {
			t.Fatalf("resolution = %d, want the pending SA promoted", got)
		}
		if f.ps.rebindRekeyOwed {
			t.Fatal("promotion left the owed rekey set")
		}
		f.ps.serviceChildRekeyTrigger(pending, nil, time.Now(), f.myTr, f.log)
		if f.ps.pendingRekey != nil {
			t.Fatal("the promoted child was rekeyed for the old pair's move")
		}
	})
	t.Run("reaped", func(t *testing.T) {
		f := rbOwner(t, rbOptions{})
		f.ps.setPendingSA(&SA{PeerName: f.local.PeerName, CreatedAt: time.Now().Add(-time.Hour)})
		f.dpd(t, f.movedTr, false)
		f.ps.reapStalePending(time.Now(), nil, f.dp, f.log)
		if f.ps.getPendingSA() != nil {
			t.Fatal("setup: the unauthenticated pending SA was not reaped")
		}
		if !f.ps.rebindRekeyOwed {
			t.Fatal("the reap dropped the owed rekey")
		}
		f.tick()
		f.answerRekey(t)
		f.wantReplacementOnMovedPort(t)
	})
}

// TestRebindOnARekeyResponseSaysGoodbyeAtOnce checks that a rebind raised by the answer
// to Ze's own Child SA rekey frees the request window before it closes the IKE SA, so
// the goodbye Delete leaves at once.
//
// Goal: RFC 7296 Section 2.3: "An IKE endpoint MUST wait for a response to each of its
// messages before sending a subsequent message unless it has received a Window Size
// Notify". The rekey response is that answer. A window still held after it made
// sayGoodbye wait goodbyeWindowWait for an answer already read, then send no Delete,
// against Section 2.4: the endpoint closing an IKE SA "SHOULD send a Delete payload
// indicating that it has closed the IKE SA unless the other endpoint is no longer
// responding".
//
// Method: a transport-mode Child SA. Ze sends its rekey request, then the peer answers
// it from another address (the old endpoint is moved to an address that is gone, so the
// real socket reads as the new one), which takes the transport-mode exit of
// rebindChildSAs. The answer MUST leave the window free and install nothing, and
// sayGoodbye MUST then deliver an IKE SA Delete well inside goodbyeWindowWait.
//
// MUTATION: move the response's answerAuthenticatedResponse back after the rebind block
// in handleOwnedInbound (inbound.go): the window stays held, sayGoodbye waits 2 s, and
// no Delete arrives.
func TestRebindOnARekeyResponseSaysGoodbyeAtOnce(t *testing.T) {
	f := rbOwner(t, rbOptions{migrator: true, transport: true})
	f.ps.rebindRekeyOwed = true
	f.tick()
	req := mbReceive(t, f.peerTr)
	msg := parseMsg(t, req.Data)
	if msg.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Fatalf("the owed rekey sent exchange %d, want CREATE_CHILD_SA", msg.Header.ExchangeType)
	}
	if !f.local.requestOutstanding {
		t.Fatal("setup: the rekey request holds no request window")
	}
	answer, _, err := respondChildRekey(f.peer, mbDecrypt(t, f.peer, req.Data),
		f.peerChild, msg.Header.MessageID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("peer answers the Child SA rekey: %v", err)
	}

	f.local.peerEndpoint = &net.UDPAddr{IP: rbAddressGone, Port: int(f.oldPort)}
	out := f.ps.handleOwnedInbound(f.local, transport.Packet{
		Data: answer, RemoteAddr: nttPeerAddr(t, f.peerTr), LocalAddr: nttPeerAddr(t, f.myTr), NATT: true,
	}, f.myTr, f.dp, f.log)
	if !out.reestablish {
		t.Fatal("a rekey answer from a new address did not close the transport-mode IKE SA")
	}
	if f.local.requestOutstanding {
		t.Fatal("the rekey answer that closed the IKE SA left its request window held")
	}
	if len(f.mock.sas) != 2 {
		t.Fatalf("the rekey answer of a closing IKE SA installed states: %d, want the first pair only", len(f.mock.sas))
	}

	start := time.Now()
	f.ps.sayGoodbye(f.local, f.myTr, f.dp, f.log)
	if waited := time.Since(start); waited >= goodbyeWindowWait/2 {
		t.Fatalf("sayGoodbye waited %s for a request already answered", waited)
	}
	goodbye := mbReceive(t, f.peerTr)
	for _, del := range lcyDeletes(mbDecrypt(t, f.peer, goodbye.Data)) {
		if del.ProtocolID == wire.ProtocolIKE {
			return
		}
	}
	t.Fatal("the closing IKE SA sent the peer no Delete for it")
}
