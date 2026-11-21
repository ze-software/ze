// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- following a dynamic WAN address
// Related: apply.go -- rebindListeners, mobileSessionsOn, repointSessions
// Related: mobike.go -- serviceMobike, mobikeSource

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// imFixture is an engine running one `vpn ipsec interface` peer on its primary pair at
// 127.0.0.1, whose session holds a keyed IKE SA with an installed Child SA.
type imFixture struct {
	state      *ikeEngineState
	wan        *fakeInterface
	ps         *PeerSession
	sa, peerSA *SA
	child      *ChildSA
	peerTr     *transport.UDPTransport
	oldIKE     *transport.UDPTransport
}

// imInterfaceSession builds the fixture. The engine shares one wildcard NAT-T socket
// between its pairs, as it does in production with live XFRM migration (shareNATTFn: the
// test port alone would give each pair its own). The peer starts through the real apply;
// its session is then replaced by one the test owns, placed exactly where the reconcile
// placed the real one, so no connect cycle races the SA the test installs. Its goroutine
// only waits for Stop. mobike says whether the SA negotiated MOBIKE (SA.mobikeUp).
func imInterfaceSession(t *testing.T, mobike bool) *imFixture {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("a wildcard UDP socket chooses its source per datagram only on Linux (IP_PKTINFO)")
	}
	if !canBind(t, "127.0.0.2") {
		t.Skip("this host binds only 127.0.0.1 on loopback; the move needs a second address")
	}
	previous := shareNATTFn
	shareNATTFn = func() bool { return true }
	t.Cleanup(func() { shareNATTFn = previous })

	state := applyTestState(t)
	wan := &fakeInterface{addr: "127.0.0.1"}
	wan.install(t)
	if err := state.applyConfig(interfaceConfig(applyTestPeer("")), applyReload); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if state.sharedNATT == nil || state.listeners.natt != state.sharedNATT {
		t.Fatal("setup: the primary pair does not hold the shared NAT-T socket")
	}
	started := runningPeer(state, "test-peer")
	if started == nil {
		t.Fatal("setup: the peer did not start")
	}
	stopPeerSession("test-peer", started, state.peers, state.table, dataplane.Get(), nil, state.log)

	placed := state.socketsFor(state.plan)("test-peer")
	if !placed.fromInterface || placed.host != "127.0.0.1" || placed.ike == nil {
		t.Fatalf("setup: the peer is placed %+v, want the interface pair at 127.0.0.1", placed)
	}
	peer := state.plan.cfg.Peers["test-peer"]
	ini, resp, _ := establishPSK(t)
	ini.bindSockets(placed.ike, placed.natt)
	ini.mobike.canMigrate = true
	ini.floatToNATTPort()
	ini.mobike.enabled = true
	ini.mobike.originalInitiator = true
	peerTr := mbTransport(t, "127.0.0.1:0", true)
	ini.peerEndpoint = nttPeerAddr(t, peerTr)
	ini.mobike.local = &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: nttPort(t, placed.natt)}
	if mobike {
		atomic.StoreInt32(&ini.mobikeUp, 1)
	}
	child, err := createFirstChildSA(ini, testESPGroup(), "127.0.0.1", ini.PeerCfg.RemoteAddress, 7,
		&mbDataplane{}, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("setup: install the Child SA: %v", err)
	}

	ps := &PeerSession{
		peerName: "test-peer",
		peerCfg:  peer,
		ikeGroup: state.plan.cfg.IKEGroups[peer.IKEGroup],
		espGroup: state.plan.cfg.ESPGroups[peer.ESPGroup],
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
	ps.sockets.Store(&placed)
	ps.setSA(ini)
	ps.setChildSA(child)
	go func() {
		<-ps.stopCh
		close(ps.done)
	}()
	peersMu.Lock()
	state.peers["test-peer"] = ps
	peersMu.Unlock()

	return &imFixture{
		state: state, wan: wan, ps: ps, sa: ini, peerSA: resp, child: child,
		peerTr: peerTr, oldIKE: state.listeners.ike,
	}
}

// VALIDATES: the owner decision of 2026-11-20 (ipsec-7-ikev2-engine.md, "Following a
// dynamic WAN address"): an interface-mode session whose SA negotiated MOBIKE survives an
// interface address move. The same session keeps the same IKE SA and Child SA, by SPI;
// it is re-pointed to the pair reopened at the new address while the old IKE socket
// closes; and its next MOBIKE tick sends UPDATE_SA_ADDRESSES from the new address.
//
// Method: the real address worker step (followAddress) moves the fake interface from
// 127.0.0.1 to 127.0.0.2. The owner loop's tick is serviceMobike, called once by the test
// with the socket the loop was started with; a loopback NAT-T socket stands in for the
// responder and decrypts what arrives with the responder's keys.
//
// RED before the change: rebindListeners stopped every session on the moving pair, so
// the session was stopped, its SA removed, and a fresh session started a new IKE_SA_INIT
// from the new address (the mobike-initiator interop log).
func TestInterfaceMoveKeepsMOBIKESession(t *testing.T) {
	f := imInterfaceSession(t, true)
	iSPI, rSPI := f.sa.InitiatorSPI, f.sa.ResponderSPI
	inSPI, outSPI := f.child.InboundSPI, f.child.OutboundSPI

	f.wan.set("127.0.0.2")
	f.state.followAddress()

	if f.ps.stopped() || runningPeer(f.state, "test-peer") != f.ps {
		t.Fatal("the MOBIKE session was stopped or replaced by the address move")
	}
	if sa := f.ps.getSA(); sa != f.sa || sa.InitiatorSPI != iSPI || sa.ResponderSPI != rSPI {
		t.Fatal("the IKE SA was replaced by the address move")
	}
	if child := f.ps.getChildSA(); child != f.child || child.InboundSPI != inSPI || child.OutboundSPI != outSPI {
		t.Fatal("the Child SA was replaced by the address move")
	}
	f.state.applyMu.Lock()
	placed := f.ps.currentSockets()
	newIKE, shared, host := f.state.listeners.ike, f.state.sharedNATT, f.state.listeners.host
	f.state.applyMu.Unlock()
	if host != "127.0.0.2" || newIKE == nil || newIKE == f.oldIKE {
		t.Fatalf("the primary pair is at %q, want reopened at 127.0.0.2", host)
	}
	if placed.host != "127.0.0.2" || placed.ike != newIKE || placed.natt != shared || !placed.fromInterface {
		t.Fatalf("the session was not re-pointed to the reopened pair: %+v", placed)
	}
	if err := sendOn(f.oldIKE); !errors.Is(err, transport.ErrClosed) {
		t.Fatalf("the IKE socket at the old address is still open: %v", err)
	}

	f.ps.serviceMobike(f.sa, f.oldIKE, time.Now(), slogutil.DiscardLogger())

	if f.sa.ikeSocket != newIKE {
		t.Fatal("the owner tick left the SA on the closed IKE socket")
	}
	wantLocal := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: nttPort(t, shared)}
	update := mbReceive(t, f.peerTr)
	mbEndpoint(t, update.RemoteAddr, wantLocal)
	inner := mbDecrypt(t, f.peerSA, update.Data)
	mbRequireNotify(t, inner, wire.NotifyUpdateSAAddresses)
	mbRequireNotify(t, inner, wire.NotifyCookie2)
	if pending := f.sa.mobike.pending; pending == nil || !pending.update || !pending.local.IP.Equal(wantLocal.IP) {
		t.Fatalf("no address update is outstanding from 127.0.0.2: %+v", pending)
	}
}

// VALIDATES: a MOBIKE session whose reopened pair does not take the shared NAT-T socket
// is stopped, and the restart is logged at Error and raised once on the report bus
// under doctor-ipsec-mobike-socket-not-shared, naming the peer, the cause (the socket
// sharing decision changed) and the restart that fixes it, never the kernel's
// migration support, which the session already had.
// METHOD: the fixture of TestInterfaceMoveKeepsMOBIKESession with SA.mobikeUp set, its
// socket sharing then turned off (shareNATTFn) before the real address worker step
// moves the interface; read the session, the log and the recorded issues.
// PREVENTS: the restart reported under doctor-ipsec-mobike-unavailable with a
// CONFIG_XFRM_MIGRATE fix, which sends the operator to rebuild a kernel that is fine.
func TestInterfaceMoveWithoutTheSharedSocketReportsTheCause(t *testing.T) {
	f := imInterfaceSession(t, true)
	var logged bytes.Buffer
	f.state.log = slog.New(slog.NewTextHandler(&logged, nil))
	raiser := &espFormRecordedIssues{}
	f.state.issueRaiser = raiser
	shareNATTFn = func() bool { return false }

	f.wan.set("127.0.0.2")
	f.state.followAddress()

	if !f.ps.stopped() {
		t.Fatal("a MOBIKE session survived a move whose reopened pair lacks the shared NAT-T socket")
	}
	out := logged.String()
	code := diagnostic.CodeDoctorIPsecMOBIKESocketNotShared
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "code="+code) ||
		!strings.Contains(out, "test-peer") || !strings.Contains(out, "ze.test.ike.port") {
		t.Fatalf("the restart is not logged at Error with its code, peer and cause:\n%s", out)
	}
	if strings.Contains(out, diagnostic.CodeDoctorIPsecMOBIKEUnavailable) || strings.Contains(out, "CONFIG_XFRM_MIGRATE") {
		t.Fatalf("the restart is reported as missing kernel migration support:\n%s", out)
	}
	if len(raiser.issues) != 1 {
		t.Fatalf("the restart raised %d report-bus issues, want 1: %+v", len(raiser.issues), raiser.issues)
	}
	issue := raiser.issues[0]
	if issue.Code != code || issue.Severity != rpc.ReportError || issue.Fix != spdInstallFixRestart {
		t.Fatalf("the raised issue is %s/%s with fix %q, want %s/%s with fix %q",
			issue.Severity, issue.Code, issue.Fix, rpc.ReportError, code, spdInstallFixRestart)
	}
}

// VALIDATES: the other side of the same decision: an interface-mode session whose SA did
// not negotiate MOBIKE is still stopped by the move, and the reconcile starts a new
// session on the reopened pair at the new address.
//
// Method: the fixture of TestInterfaceMoveKeepsMOBIKESession with SA.mobikeUp left 0.
func TestInterfaceMoveRestartsSessionWithoutMOBIKE(t *testing.T) {
	f := imInterfaceSession(t, false)

	f.wan.set("127.0.0.2")
	f.state.followAddress()

	if !f.ps.stopped() {
		t.Fatal("a session without MOBIKE survived the move of its socket pair")
	}
	now := runningPeer(f.state, "test-peer")
	if now == nil || now == f.ps {
		t.Fatal("no new session was started on the new address")
	}
	f.state.applyMu.Lock()
	newIKE := f.state.listeners.ike
	f.state.applyMu.Unlock()
	if placed := now.currentSockets(); placed.host != "127.0.0.2" || placed.ike != newIKE || now.peerCfg.LocalAddress != "127.0.0.2" {
		t.Fatalf("the new session runs from %q (local %q), want the pair at 127.0.0.2", placed.host, now.peerCfg.LocalAddress)
	}
}
