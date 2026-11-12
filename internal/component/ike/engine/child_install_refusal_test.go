// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: established.go -- runEstablished keeps the IKE SA when its first Child SA is refused
// Related: create_child.go -- childCreateSchedule, finishChildCreate, childInstallRefusedFix

package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cirTUNFix is the fix the stand-in TUN refusal carries, the one an operator acts on.
const cirTUNFix = "in Docker: docker run --device /dev/net/tun"

// cirTUNRefusal builds the error the dataplane answered in the interop run: the ESP form
// receiver could not open /dev/net/tun, wrapped by the XFRM install as InstallSA does.
// The cause is an operator error, as espFormTUNMissingRefusal writes it.
func cirTUNRefusal() error {
	cause := opererr.Wrap(errors.New("open /dev/net/tun: no such file or directory"),
		"the TUN device node /dev/net/tun does not exist, and Ze could not create it",
		"Ze uses the TUN device to create the ze-espform interface", cirTUNFix)
	return fmt.Errorf("xfrm: spi=3303519475 cannot receive both ESP forms, so it was not installed: %w", cause)
}

// cirRunEstablished runs runEstablished on a session that is already stopped, so the
// owner loop returns at its first select, and answers what runEstablished returned.
func cirRunEstablished(t *testing.T, ps *PeerSession, peer ipsec.SiteToSitePeer, sa *SA, table *SATable,
	tr *transport.UDPTransport, log *slog.Logger,
) error {
	t.Helper()
	ps.stopCh = make(chan struct{})
	ps.supersede = make(chan struct{}, 1)
	close(ps.stopCh)
	done := make(chan error, 1)
	go func() {
		done <- ps.runEstablished(sa, peer, testIKEGroup(), table, tr, nil, log)
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(rtxArrive):
		t.Fatal("runEstablished did not return after the session was stopped")
		return nil
	}
}

// cirWarnLines answers the WARN lines of the captured log that contain text.
func cirWarnLines(logged *bytes.Buffer, text string) []string {
	var lines []string
	for line := range strings.Lines(logged.String()) {
		if !strings.Contains(line, "level=WARN") {
			continue
		}
		if !strings.Contains(line, text) {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// VALIDATES: an initiator whose own dataplane refuses the first Child SA of IKE_AUTH, or
// that cannot bind it, keeps the IKE SA, deletes the peer's half and asks again on the
// creation schedule, logging the refusal once.
//
// Goal: the defect seen in interop (mtu-nat-installed-endpoint, strongSwan): with no
// /dev/net/tun, InstallSA refused every Child SA, runEstablished returned the error, the
// IKE SA ended, and runInitiator, which clears connectFailures when the IKE SA
// establishes, re-ran the handshake after reconnectBase. Ze logged the same error once a
// second with no end. Method: a real PSK IKE_AUTH between two SAs, the far end's Child SA
// installed, then ze's dataplane refuses every install (or ze's vti bind names no XFRM
// interface), and runEstablished runs on a stopped session. It MUST return nil (the
// owner loop ran, the cycle did not fail), install no state, send an ESP Delete listing
// ze's inbound SPI, schedule the next creation childCreateFirst from now, and log exactly
// one WARN line carrying the cause and its fix.
//
// PREVENTS: a permanent dataplane refusal that re-runs IKE_SA_INIT, Diffie-Hellman and
// IKE_AUTH every second, and floods the log with one identical line per second.
// RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA to be
// deleted."
func TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA(t *testing.T) {
	cases := []struct {
		name    string
		vtiBind string
		refuse  error
		cause   string
		fix     string
	}{
		{name: "dataplane refuses the install", refuse: cirTUNRefusal(),
			cause: "open /dev/net/tun: no such file or directory", fix: cirTUNFix},
		{name: "vti bind names no XFRM interface", vtiBind: "ze-cir-no-such-xfrm",
			cause: "binds vti ze-cir-no-such-xfrm", fix: childResponseFixBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			quiet := slogutil.DiscardLogger()
			dp := &rkyDP{}
			loadFailClosedBackend(t, dp)
			peerTr, myTr := encapNATTPeerLink(t)
			iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "install-refusal-initiator")
			iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
			ini, resp, table := encapSAInit(t, iniPeer, respPeer)
			respNATT, _ := bindNATT(t, resp)
			bindNATT(t, ini)
			respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			respPS.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, respNATT, encapPeerAt(encapTestPeerPort), quiet)
			if resp.State != StateEstablished {
				t.Fatalf("setup: the far end did not establish (state %v)", resp.State)
			}
			// The far end installed its half; from here on ze's own installs are judged.
			dp.installed = nil
			dp.installErr = tc.refuse

			// The response arrives from the far end's NAT-T socket, so ze records that
			// endpoint and has an encapsulation port: the refusal is the dataplane's.
			pkt := transport.Packet{Data: resp.LastSentMsg, RemoteAddr: nttPeerAddr(t, peerTr), NATT: true}
			handleInbound(ini, pkt, table, myTr, quiet)
			if ini.State != StateEstablished || ini.IKEAuthChildless {
				t.Fatalf("setup: initiator state %v childless=%v, want established with a Child SA offered",
					ini.State, ini.IKEAuthChildless)
			}

			peer := iniPeer
			peer.VTIBind = tc.vtiBind
			if tc.vtiBind != "" {
				previous := xfrmIfIDOf
				xfrmIfIDOf = func(name string) (uint32, error) {
					return 0, errors.New("test: " + name + " is not an XFRM interface")
				}
				t.Cleanup(func() { xfrmIfIDOf = previous })
			}
			ps := &PeerSession{peerName: "ze", peerCfg: peer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			log, logged := cfxLogger()
			before := time.Now()
			if err := cirRunEstablished(t, ps, peer, ini, table, myTr, log); err != nil {
				t.Fatalf("runEstablished = %v, want nil: the IKE SA must outlive its refused Child SA", err)
			}
			after := time.Now()

			if len(dp.installed) != 0 {
				t.Errorf("%d states installed, want none", len(dp.installed))
			}
			if ps.getChildSA() != nil {
				t.Error("the session holds a Child SA its dataplane refused")
			}
			if !deleteListsSPI(t, resp, peerTr, ini.ChildInboundSPI) {
				t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", ini.ChildInboundSPI)
			}
			if !ps.childCreate.active {
				t.Fatal("no creation is scheduled: the IKE SA would stay childless forever")
			}
			if ps.childCreate.wait != childCreateFirst {
				t.Errorf("the first retry waits %v, want childCreateFirst %v", ps.childCreate.wait, childCreateFirst)
			}
			next := ps.childCreate.next
			if next.Before(before.Add(childCreateFirst)) || next.After(after.Add(childCreateFirst)) {
				t.Errorf("the first retry is at %v, want childCreateFirst after the refusal (%v .. %v)",
					next, before.Add(childCreateFirst), after.Add(childCreateFirst))
			}

			lines := cirWarnLines(logged, tc.cause)
			if len(lines) != 1 {
				t.Fatalf("%d WARN lines carry the cause, want exactly 1:\n%s", len(lines), logged.String())
			}
			if !strings.Contains(lines[0], "fix=\""+tc.fix+"\"") {
				t.Errorf("the refusal line does not carry the fix %q: %s", tc.fix, lines[0])
			}
			if !strings.Contains(lines[0], "retry-after="+childCreateFirst.String()) {
				t.Errorf("the refusal line does not name retry-after=%v: %s", childCreateFirst, lines[0])
			}
		})
	}
}

// cirEstablishInitiator runs a real PSK IKE_AUTH between two SAs, the far end's Child SA
// installed, and answers ze's initiator SA established with a Child SA offered, the
// far end's SA and transport, and ze's own transport and SA table. ze's installs are
// recorded in dp from here on.
func cirEstablishInitiator(t *testing.T, dp *rkyDP, label string) cirInitiator {
	t.Helper()
	quiet := slogutil.DiscardLogger()
	loadFailClosedBackend(t, dp)
	peerTr, myTr := encapNATTPeerLink(t)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, label)
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	ini, resp, table := encapSAInit(t, iniPeer, respPeer)
	respNATT, _ := bindNATT(t, resp)
	bindNATT(t, ini)
	respPS := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	respPS.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, respNATT, encapPeerAt(encapTestPeerPort), quiet)
	if resp.State != StateEstablished {
		t.Fatalf("setup: the far end did not establish (state %v)", resp.State)
	}
	dp.installed = nil
	pkt := transport.Packet{Data: resp.LastSentMsg, RemoteAddr: nttPeerAddr(t, peerTr), NATT: true}
	handleInbound(ini, pkt, table, myTr, quiet)
	if ini.State != StateEstablished || ini.IKEAuthChildless {
		t.Fatalf("setup: initiator state %v childless=%v, want established with a Child SA offered",
			ini.State, ini.IKEAuthChildless)
	}
	return cirInitiator{peer: iniPeer, ini: ini, resp: resp, peerTr: peerTr, myTr: myTr, table: table}
}

// cirInitiator is what cirEstablishInitiator answers: ze's peer config and initiator SA,
// the far end's SA and transport, and ze's own transport and SA table.
type cirInitiator struct {
	peer         ipsec.SiteToSitePeer
	ini, resp    *SA
	peerTr, myTr *transport.UDPTransport
	table        *SATable
}

// VALIDATES: an initiator that cannot BUILD the first Child SA of IKE_AUTH keeps the IKE
// SA, as one that cannot install it does: it deletes the peer's half, records the refusal
// for the doctor, schedules the next creation childCreateFirst from now, and logs exactly
// one WARN line carrying the cause, retry-after, the fix and
// code=doctor-ipsec-child-install-refused. One row per cause createFirstChildSA answers
// after IKE_AUTH that is a failure of the Child SA: an esp-group with no proposal
// (errChildProposal), a key derivation that fails (errChildKeying, through a PRF ze does
// not know), and a peer SPI never recorded.
//
// Goal: before the fold, runEstablished's `case err != nil` returned each of these, the
// IKE SA ended after runInitiator had cleared connectFailures, so reconnectDelay answered
// reconnectBase and the handshake and its WARN line repeated every second. On that code
// every row fails at its first check, runEstablished answering the error.
//
// PREVENTS: a Child SA cause that re-runs IKE_SA_INIT, Diffie-Hellman and IKE_AUTH every
// second. RFC 7296 Section 2.21.2: "This failure does not automatically cause the IKE SA
// to be deleted."
func TestInitiatorKeepsIKESAWhenItCannotBuildTheFirstChildSA(t *testing.T) {
	cases := []struct {
		name    string
		breakSA func(sa *SA)
		cause   func(err error) bool
		fix     string
	}{
		{name: "esp-group with no proposal", breakSA: func(sa *SA) { sa.ESPGroup.Proposals = nil },
			cause: func(err error) bool { return errors.Is(err, errChildProposal) }, fix: childResponseFixProposal},
		{name: "key derivation fails", breakSA: func(sa *SA) { sa.Proposal.PRF.ID = 0 },
			cause: func(err error) bool { return errors.Is(err, errChildKeying) }, fix: childResponseFixKeying},
		{name: "no peer ESP SPI recorded", breakSA: func(sa *SA) { sa.ChildOutboundSPI = 0 },
			cause: func(err error) bool { return strings.Contains(err.Error(), "no peer ESP SPI recorded") },
			fix:   childResponseFixAnswer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dp := &rkyDP{}
			rig := cirEstablishInitiator(t, dp, "build-refusal-initiator")
			peer, ini, resp, peerTr, myTr, table := rig.peer, rig.ini, rig.resp, rig.peerTr, rig.myTr, rig.table
			tc.breakSA(ini)

			ps := &PeerSession{peerName: "ze", peerCfg: peer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
			log, logged := cfxLogger()
			before := time.Now()
			if err := cirRunEstablished(t, ps, peer, ini, table, myTr, log); err != nil {
				t.Fatalf("runEstablished = %v, want nil: the IKE SA must outlive its unbuilt Child SA", err)
			}
			after := time.Now()

			if len(dp.installed) != 0 {
				t.Errorf("%d states installed, want none", len(dp.installed))
			}
			if ps.getChildSA() != nil {
				t.Error("the session holds a Child SA it could not build")
			}
			if !deleteListsSPI(t, resp, peerTr, ini.ChildInboundSPI) {
				t.Errorf("no ESP Delete listing ze's inbound SPI %#x reached the peer", ini.ChildInboundSPI)
			}
			if !ps.childCreate.active {
				t.Fatal("no creation is scheduled: the IKE SA would stay childless forever")
			}
			next := ps.childCreate.next
			if next.Before(before.Add(childCreateFirst)) || next.After(after.Add(childCreateFirst)) {
				t.Errorf("the first retry is at %v, want childCreateFirst after the refusal (%v .. %v)",
					next, before.Add(childCreateFirst), after.Add(childCreateFirst))
			}
			refusal := ps.childRefused.Load()
			if refusal == nil {
				t.Fatal("the unbuilt first Child SA was not recorded for the doctor")
			}
			if !tc.cause(refusal.err) {
				t.Errorf("recorded %v, want the %s cause", refusal.err, tc.name)
			}

			lines := cirWarnLines(logged, "this node could not build it")
			if len(lines) != 1 {
				t.Fatalf("%d refusal lines, want exactly 1:\n%s", len(lines), logged.String())
			}
			for _, want := range []string{
				"fix=\"" + tc.fix + "\"",
				"retry-after=" + childCreateFirst.String(),
				"code=" + diagnostic.CodeDoctorIPsecChildInstallRefused,
			} {
				if !strings.Contains(lines[0], want) {
					t.Errorf("the refusal line does not carry %s: %s", want, lines[0])
				}
			}
		})
	}
}

// VALIDATES: an IKE SA that holds no SK_* keys when its first Child SA is built is ended,
// the one failure of runEstablished's build that is the IKE SA's own: runEstablished
// answers errChildNoIKEKeys, schedules no creation and records no Child SA refusal.
//
// PREVENTS: the fold above keeping an IKE SA that can protect no message, so could
// neither delete the peer's half nor ever ask again. The discriminator for that test.
func TestInitiatorEndsIKESAWithNoKeysForTheFirstChildSA(t *testing.T) {
	dp := &rkyDP{}
	rig := cirEstablishInitiator(t, dp, "no-keys-initiator")
	peer, ini, myTr, table := rig.peer, rig.ini, rig.myTr, rig.table
	ini.SKKeys = nil

	ps := &PeerSession{peerName: "ze", peerCfg: peer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	log, logged := cfxLogger()
	if err := cirRunEstablished(t, ps, peer, ini, table, myTr, log); !errors.Is(err, errChildNoIKEKeys) {
		t.Fatalf("runEstablished = %v, want errChildNoIKEKeys", err)
	}
	if ps.childCreate.active {
		t.Error("a creation is scheduled on an IKE SA that was ended")
	}
	if got := ps.childRefused.Load(); got != nil {
		t.Errorf("an IKE SA failure was recorded as a Child SA refusal: %+v", got)
	}
	lines := cirWarnLines(logged, "holds no keys")
	if len(lines) != 1 {
		t.Fatalf("%d lines name the keyless IKE SA, want 1:\n%s", len(lines), logged.String())
	}
	if !strings.Contains(lines[0], "code="+diagnostic.CodeDoctorInternalDefect) {
		t.Errorf("the line carries no internal-defect code: %s", lines[0])
	}
}

// cirAnswerDelete reads the ESP Delete ze sent the stand-in peer for spi, and frees ze's
// request window as the peer's empty INFORMATIONAL response would.
func cirAnswerDelete(t *testing.T, r *refusedRig, spi uint32) {
	t.Helper()
	raw := rtxRecv(t, r.peerTr)
	if raw == nil {
		t.Fatalf("no ESP Delete for %#x reached the peer", spi)
	}
	msg := parseMsg(t, raw)
	inner, err := decryptAndParse(r.peer, msg, raw)
	if err != nil {
		t.Fatalf("the peer could not decrypt what ze sent: %v", err)
	}
	for i := range inner {
		del, ok := inner[i].Payload.(*wire.PayloadDelete)
		if !ok || del.ProtocolID != wire.ProtocolESP {
			continue
		}
		for off := 0; off+4 <= len(del.SPIs); off += 4 {
			if binary.BigEndian.Uint32(del.SPIs[off:]) == spi {
				r.sa.answerAuthenticatedResponse(msg.Header.MessageID)
				return
			}
		}
	}
	t.Fatalf("the datagram ze sent is no ESP Delete listing %#x", spi)
}

// VALIDATES: successive creation attempts of a childless IKE SA whose dataplane refuses
// every install are spaced by a wait that doubles up to childCreateMax, log one WARN line
// each with the growing retry-after, and stop once an install succeeds, which resets the
// schedule.
//
// Goal: a permanent refusal (no TUN device, no capability) costs the peer and the log a
// bounded rate, and the operator's fix is picked up with no restart. Method: the clock
// seam is serviceChildCreate's now and the schedule's due: each attempt is made at the
// instant the schedule names, and the instant one nanosecond earlier MUST not be due.
// finishChildCreate stamps the refusal with the wall clock, so the next attempt is
// bounded between the refusal's start and end plus the wait. The peer answers through its
// real respondNewChild and ze's dataplane refuses every install; after the cap is reached
// twice the refusal is lifted and the next attempt MUST install the Child SA.
//
// PREVENTS: a retry with no maximum backoff, a log line per second, and a schedule that
// survives the success it was waiting for.
func TestChildInstallRefusalBacksOffToTheCapAndResetsOnSuccess(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.dp.installErr = cirTUNRefusal()
	log, logged := cfxLogger()
	r.createLog = log

	waits := []time.Duration{
		2 * childCreateFirst,
		4 * childCreateFirst,
		8 * childCreateFirst,
		childCreateMax,
		childCreateMax,
	}
	if waits[2] >= childCreateMax {
		t.Fatalf("the fixture assumes 8*childCreateFirst %v stays under childCreateMax %v", waits[2], childCreateMax)
	}
	for i, wait := range waits {
		due := r.ps.childCreate.next
		if r.ps.childCreate.due(due.Add(-time.Nanosecond)) {
			t.Fatalf("attempt %d is due before the instant its schedule names", i+1)
		}
		r.ps.serviceChildCreate(r.sa, r.myTr, due, log)
		if r.ps.pendingRekey == nil {
			t.Fatalf("attempt %d: a due creation sent no request", i+1)
		}
		inSPI := r.ps.pendingRekey.newInboundSPI
		var peerChild *ChildSA
		before := time.Now()
		r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
		after := time.Now()
		if r.ps.getChildSA() != nil {
			t.Fatalf("attempt %d: a refused install holds a Child SA", i+1)
		}
		if r.ps.childCreate.wait != wait {
			t.Errorf("attempt %d: the next wait is %v, want %v", i+1, r.ps.childCreate.wait, wait)
		}
		next := r.ps.childCreate.next
		if next.Before(before.Add(wait)) || next.After(after.Add(wait)) {
			t.Errorf("attempt %d: the next attempt is at %v, want %v after the refusal", i+1, next, wait)
		}
		cirAnswerDelete(t, r, inSPI)
	}

	lines := cirWarnLines(logged, "open /dev/net/tun: no such file or directory")
	if len(lines) != len(waits) {
		t.Fatalf("%d WARN lines carry the cause over %d attempts, want one per attempt:\n%s",
			len(lines), len(waits), logged.String())
	}
	for i, line := range lines {
		if !strings.Contains(line, "retry-after="+waits[i].String()) {
			t.Errorf("WARN line %d does not name retry-after=%v: %s", i+1, waits[i], line)
		}
		if !strings.Contains(line, "fix=\""+cirTUNFix+"\"") {
			t.Errorf("WARN line %d does not carry the operator error's fix: %s", i+1, line)
		}
	}

	// The operator gives the container the device: the next attempt installs.
	r.dp.installErr = nil
	r.ps.serviceChildCreate(r.sa, r.myTr, r.ps.childCreate.next, log)
	if r.ps.pendingRekey == nil {
		t.Fatal("the attempt after the fix sent no request")
	}
	var peerChild *ChildSA
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if r.ps.getChildSA() == nil {
		t.Fatal("the attempt after the fix installed no Child SA")
	}
	if r.ps.childCreate != (childCreateSchedule{}) {
		t.Errorf("the schedule survived the success: %+v", r.ps.childCreate)
	}
	// The next refusal, on a later IKE SA, starts over at the first wait.
	r.ps.childCreate.start(time.Now())
	if r.ps.childCreate.wait != childCreateFirst {
		t.Errorf("a schedule started after a success waits %v, want childCreateFirst %v",
			r.ps.childCreate.wait, childCreateFirst)
	}
}

// VALIDATES: the fix a refused Child SA logs is the operator error's own fix when the
// producer that refused it wrote one, and the cause's generic fix otherwise.
//
// PREVENTS: an operator whose container lacks /dev/net/tun told to change the esp-group
// algorithms.
func TestChildInstallRefusedFix(t *testing.T) {
	withOperatorError := fmt.Errorf("%w: %w", errChildInstall, cirTUNRefusal())
	if got := childInstallRefusedFix(withOperatorError); got != cirTUNFix {
		t.Errorf("fix for an operator error cause = %q, want %q", got, cirTUNFix)
	}
	plain := fmt.Errorf("%w: %w", errChildInstall, errors.New("netlink: operation not supported"))
	if got := childInstallRefusedFix(plain); got != childResponseFixInstall {
		t.Errorf("fix for a plain install refusal = %q, want childResponseFixInstall", got)
	}
}
