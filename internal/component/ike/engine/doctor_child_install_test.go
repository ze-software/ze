// Design: docs/architecture/doctor-and-health-checks.md -- the runtime phase: checks only the daemon runs
// Related: doctor_child_install.go -- recordChildRefused, childInstallFindings
// Related: child_install_refusal_test.go -- the refusal paths these tests record from

package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// dciTUNRefusal is the install error InstallSA answers when the ESP form receiver cannot
// create ze-espform: the receiver's operator error, wrapped by the XFRM install.
func dciTUNRefusal() error {
	return fmt.Errorf("xfrm: spi=3303519475 cannot receive both ESP forms, so it was not installed: %w",
		dciTUNCause())
}

// dciTUNProblem is the problem the receiver's operator error names for a missing
// /dev/net/tun.
const dciTUNProblem = "the TUN device node /dev/net/tun does not exist, and Ze could not create it"

// dciTUNCause is the receiver's operator error for a missing /dev/net/tun, the cause a
// recorded TUN refusal MUST carry.
func dciTUNCause() error {
	return opererr.Wrap(errors.New("open /dev/net/tun: no such file or directory"),
		dciTUNProblem, "Ze uses the TUN device to create the ze-espform interface", cirTUNFix)
}

// dciCarriesTUNCause reports whether err carries the TUN operator error dciTUNCause
// builds: its problem, reachable by errors.As through every wrap.
func dciCarriesTUNCause(err error) bool {
	cause, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		return false
	}
	return cause.Problem() == dciTUNProblem
}

// VALIDATES: recordChildRefused stores the refusal with its time and the next attempt the
// creation schedule names; setChildSA with a Child SA clears it, and setChildSA(nil),
// which a teardown calls, leaves it.
// PREVENTS: a refusal reported after a Child SA installed, and one dropped by a teardown
// while no Child SA exists.
func TestRecordChildRefusedClearsOnInstall(t *testing.T) {
	ps := &PeerSession{peerName: "site-b"}
	now := time.Now()
	ps.childCreate.start(now)
	cause := errors.New("netlink: operation not supported")
	ps.recordChildRefused(cause, now)

	refusal := ps.childRefused.Load()
	if refusal == nil {
		t.Fatal("no refusal recorded")
	}
	if !errors.Is(refusal.err, cause) {
		t.Errorf("recorded error %v, want %v", refusal.err, cause)
	}
	if !refusal.at.Equal(now) {
		t.Errorf("recorded at %v, want %v", refusal.at, now)
	}
	if want := now.Add(childCreateFirst); !refusal.retryAt.Equal(want) {
		t.Errorf("next attempt %v, want %v", refusal.retryAt, want)
	}

	ps.setChildSA(nil)
	if ps.childRefused.Load() == nil {
		t.Fatal("setChildSA(nil) cleared the refusal, though no Child SA exists")
	}
	ps.setChildSA(&ChildSA{InboundSPI: 1})
	if got := ps.childRefused.Load(); got != nil {
		t.Errorf("an installed Child SA left the refusal %+v", got)
	}
}

// VALIDATES: ipsec-child-install answers one error per peer whose refusal stands, in peer
// name order, naming the peer, the refusal time, the cause and the next attempt; the fix
// is the operator error's own when the cause carries one, the engine's warnings for
// errChildEncapPort (a Ze defect), and the reload otherwise, with the cause's prose in the
// message; a peer with no refusal and an empty peer set answer nothing.
// PREVENTS: an IKE SA that protects no traffic while doctor reads the box as healthy.
func TestChildInstallFindings(t *testing.T) {
	if got := childInstallFindings(nil); len(got) != 0 {
		t.Fatalf("no peers: %v, want none", got)
	}
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	retry := at.Add(childCreateFirst)
	refused := func(name string, err error, retryAt time.Time) *PeerSession {
		ps := &PeerSession{peerName: name}
		ps.childRefused.Store(&childRefusal{err: err, at: at, retryAt: retryAt})
		return ps
	}
	tun := fmt.Errorf("%w: %w", errChildInstall, dciTUNRefusal())
	encap := fmt.Errorf("%w: peer site-c has no authenticated IKE endpoint yet", errChildEncapPort)
	install := fmt.Errorf("%w: netlink: operation not supported", errChildInstall)
	peers := map[string]*PeerSession{
		"site-z": refused("site-z", install, time.Time{}),
		"site-a": refused("site-a", tun, retry),
		"site-c": refused("site-c", encap, retry),
		"site-m": {peerName: "site-m"},
	}

	diags := childInstallFindings(peers)
	code := "error:" + diagnostic.CodeDoctorIPsecChildInstallRefused
	if got, want := diagnosticCodes(diags), code+","+code+","+code; got != want {
		t.Fatalf("diagnostics = %q, want %q (%v)", got, want, diags)
	}
	for i, tc := range []struct {
		contains []string
		fix      string
	}{
		{
			contains: []string{"IPsec peer site-a has no Child SA", "2026-11-12T10:00:00Z",
				"the TUN device node /dev/net/tun does not exist", "no such file or directory",
				"no traffic of this peer is protected", "asks the peer again at 2026-11-12T10:00:30Z"},
			fix: cirTUNFix,
		},
		{
			contains: []string{"IPsec peer site-c has no Child SA", "no authenticated IKE endpoint", childResponseFixEncapPort},
			fix:      childInstallFixLog,
		},
		{
			contains: []string{"IPsec peer site-z has no Child SA", "operation not supported", childResponseFixInstall,
				"No further attempt is scheduled"},
			fix: childInstallFixReload,
		},
	} {
		assertChildInstallFinding(t, diags[i], tc.contains, tc.fix)
	}
}

// assertChildInstallFinding checks one finding's message and its fix.
func assertChildInstallFinding(t *testing.T, d rpc.DoctorCheckDiagnostic, contains []string, fix string) {
	t.Helper()
	for _, text := range contains {
		if !strings.Contains(d.Message, text) {
			t.Errorf("message does not carry %q: %q", text, d.Message)
		}
	}
	if d.Fix != fix {
		t.Errorf("fix = %q, want %q", d.Fix, fix)
	}
}

// VALIDATES: a creation response this node refuses (finishChildCreate) is recorded with
// the next attempt the schedule names, its WARN line carries
// code=doctor-ipsec-child-install-refused, a TUN cause included, the doctor reports the
// peer, and the attempt after the cause is removed installs the Child SA and clears the
// record, so the doctor reports nothing.
// PREVENTS: a creation refusal logged with no doctor code (the doctor-coverage gate
// refuses it), and a refusal the doctor keeps reporting after the Child SA installed.
func TestChildCreateRefusalIsRecordedForTheDoctor(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.dp.installErr = dciTUNRefusal()
	log, logged := cfxLogger()
	r.createLog = log

	r.ps.serviceChildCreate(r.sa, r.myTr, r.ps.childCreate.next, log)
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no request")
	}
	inSPI := r.ps.pendingRekey.newInboundSPI
	var peerChild *ChildSA
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))

	refusal := r.ps.childRefused.Load()
	if refusal == nil {
		t.Fatal("the refused creation was not recorded")
	}
	if !dciCarriesTUNCause(refusal.err) {
		t.Errorf("recorded %v, want the TUN refusal", refusal.err)
	}
	if !refusal.retryAt.Equal(r.ps.childCreate.next) {
		t.Errorf("recorded next attempt %v, want the schedule's %v", refusal.retryAt, r.ps.childCreate.next)
	}
	lines := cirWarnLines(logged, "creation response refused")
	if len(lines) != 1 {
		t.Fatalf("%d refusal lines, want 1:\n%s", len(lines), logged.String())
	}
	if !strings.Contains(lines[0], "code="+diagnostic.CodeDoctorIPsecChildInstallRefused) {
		t.Errorf("the refusal line carries no doctor code: %s", lines[0])
	}
	diags := childInstallFindings(map[string]*PeerSession{r.ps.peerName: r.ps})
	if got, want := diagnosticCodes(diags), "error:"+diagnostic.CodeDoctorIPsecChildInstallRefused; got != want {
		t.Fatalf("after the refusal: diagnostics = %q, want %q", got, want)
	}
	assertChildInstallFinding(t, diags[0], []string{r.ps.peerName, "/dev/net/tun does not exist"}, cirTUNFix)
	cirAnswerDelete(t, r, inSPI)

	// The operator gives the container the device: the next attempt installs.
	r.dp.installErr = nil
	r.ps.serviceChildCreate(r.sa, r.myTr, r.ps.childCreate.next, log)
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if r.ps.getChildSA() == nil {
		t.Fatal("the attempt after the fix installed no Child SA")
	}
	if got := r.ps.childRefused.Load(); got != nil {
		t.Errorf("the installed Child SA left the refusal %+v", got)
	}
	if diags := childInstallFindings(map[string]*PeerSession{r.ps.peerName: r.ps}); len(diags) != 0 {
		t.Errorf("after the install: %v, want none", diags)
	}
}

// VALIDATES: an initiator whose dataplane refuses the first Child SA of IKE_AUTH records
// the refusal with the next attempt childCreateFirst from now, and its WARN line carries
// code=doctor-ipsec-child-install-refused for a TUN cause (the receiver's operator error).
// Method: the setup of TestInitiatorKeepsIKESAWhenItCannotInstallTheFirstChildSA, a real
// PSK IKE_AUTH between two SAs, then runEstablished on a stopped session.
// PREVENTS: the first Child SA's refusal logged with no doctor code, and an IKE SA kept
// childless that doctor never names.
func TestFirstChildSARefusalIsRecordedForTheDoctor(t *testing.T) {
	quiet := slogutil.DiscardLogger()
	dp := &rkyDP{}
	loadFailClosedBackend(t, dp)
	peerTr, myTr := encapNATTPeerLink(t)
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "install-refusal-doctor")
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
	dp.installErr = dciTUNRefusal()
	pkt := transport.Packet{Data: resp.LastSentMsg, RemoteAddr: nttPeerAddr(t, peerTr), NATT: true}
	handleInbound(ini, pkt, table, myTr, quiet)
	if ini.State != StateEstablished || ini.IKEAuthChildless {
		t.Fatalf("setup: initiator state %v childless=%v, want established with a Child SA offered",
			ini.State, ini.IKEAuthChildless)
	}

	ps := &PeerSession{peerName: "ze", peerCfg: iniPeer, ikeGroup: testIKEGroup(), espGroup: testESPGroup()}
	log, logged := cfxLogger()
	before := time.Now()
	if err := cirRunEstablished(t, ps, iniPeer, ini, table, myTr, log); err != nil {
		t.Fatalf("runEstablished = %v, want nil", err)
	}
	after := time.Now()

	refusal := ps.childRefused.Load()
	if refusal == nil {
		t.Fatal("the refused first Child SA was not recorded")
	}
	if !dciCarriesTUNCause(refusal.err) {
		t.Errorf("recorded %v, want the TUN refusal", refusal.err)
	}
	if refusal.at.Before(before) || refusal.at.After(after) {
		t.Errorf("recorded at %v, want within %v .. %v", refusal.at, before, after)
	}
	if want := refusal.at.Add(childCreateFirst); !refusal.retryAt.Equal(want) {
		t.Errorf("recorded next attempt %v, want %v", refusal.retryAt, want)
	}
	lines := cirWarnLines(logged, "this node could not install it")
	if len(lines) != 1 {
		t.Fatalf("%d refusal lines, want 1:\n%s", len(lines), logged.String())
	}
	if !strings.Contains(lines[0], "code="+diagnostic.CodeDoctorIPsecChildInstallRefused) {
		t.Errorf("the refusal line carries no doctor code: %s", lines[0])
	}
}

// VALIDATES: ipsec-child-install is declared in the engine's Stage 1 registration at the
// runtime phase with doctor-ipsec-child-install-refused, and the code has an explanation
// entry ze show diagnostic describes.
// PREVENTS: a Warn line naming a code no check declares, which the doctor-coverage gate
// refuses, and a check run in a process that holds no peer session.
func TestChildInstallCheckRegistered(t *testing.T) {
	var found bool
	for _, decl := range engineDoctorCheckDecls() {
		if decl.Name != childInstallCheckName {
			continue
		}
		found = true
		if decl.Phase != rpc.DoctorPhaseRuntime {
			t.Errorf("phase = %q, want %q", decl.Phase, rpc.DoctorPhaseRuntime)
		}
		if got := strings.Join(decl.Codes, ","); got != diagnostic.CodeDoctorIPsecChildInstallRefused {
			t.Errorf("codes = %q, want %q", got, diagnostic.CodeDoctorIPsecChildInstallRefused)
		}
	}
	if !found {
		t.Fatalf("the engine declares no %s doctor check", childInstallCheckName)
	}
	// The builtin table registers only when the binary's entry point asks, so the
	// lookup below finds the entry only after this call, as TestIPsecDoctorCodesRegistered
	// makes it.
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(diagnostic.CodeDoctorIPsecChildInstallRefused) == nil {
		t.Errorf("%s has no explanation entry", diagnostic.CodeDoctorIPsecChildInstallRefused)
	}
}
