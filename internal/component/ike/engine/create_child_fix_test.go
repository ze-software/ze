// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: create_child.go -- finishChildCreate and serviceChildCreate log lines

package engine

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cfxLogger returns a text logger writing into the returned buffer.
func cfxLogger() (*slog.Logger, *bytes.Buffer) {
	logged := &bytes.Buffer{}
	return slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug})), logged
}

// cfxExpectLine checks that the captured log holds a line carrying msg whose fix
// attribute is fix and which names a retry-after.
func cfxExpectLine(t *testing.T, logged *bytes.Buffer, msg, fix string) {
	t.Helper()
	for line := range strings.Lines(logged.String()) {
		if !strings.Contains(line, msg) {
			continue
		}
		if !strings.Contains(line, "retry-after=") {
			t.Errorf("the %q line names no retry-after: %s", msg, line)
		}
		if !strings.Contains(line, "fix=\""+fix+"\"") {
			t.Errorf("the %q line does not carry the fix %q: %s", msg, fix, line)
		}
		return
	}
	t.Fatalf("no %q line was logged:\n%s", msg, logged.String())
}

// VALIDATES: a creation response refused because the peer declined the required
// transport mode tells the operator the cause and the configuration that fixes it.
//
// Goal: the warning names the next step for this cause, not the generic one. Method: ze
// runs mode transport with transport-required, the peer answers in tunnel mode through
// its real respondNewChild, and the refused-response line MUST carry the transport fix
// and a retry-after.
//
// PREVENTS: an operator told the pair was deleted with no word on what to change.
func TestChildCreateDeclinedTransportNamesFix(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.sa.PeerCfg.Mode = dataplane.ModeTransport
	r.sa.PeerCfg.TransportRequired = true
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	var peerChild *ChildSA
	log, logged := cfxLogger()
	r.createLog = log
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	cfxExpectLine(t, logged, "child-sa: creation response refused", childResponseFixTransport)
}

// VALIDATES: a creation response refused because the answer breaks the exchange (a pfs
// request answered without KEr) names the proposal and selector fix.
//
// Goal: a peer-side defect gets its own next step. Method: a pfs creation is answered by
// a scripted success response with no KEr, and the refused-response line MUST carry the
// answer fix and a retry-after.
//
// PREVENTS: the transport or install fix shown for a malformed answer.
func TestChildCreateMalformedAnswerNamesFix(t *testing.T) {
	r := createRig(t, ipsec.PFSEnable)
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	answer, err := espProposalWire(r.ps.espGroup.Proposals[0], 0x0abc0001, 1, r.sa.Proposal.DHGroup.ID)
	if err != nil {
		t.Fatalf("espProposalWire: %v", err)
	}
	log, logged := cfxLogger()
	r.createLog = log
	r.relayCreate(t, r.peerScripted(t, []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: []wire.Proposal{answer}}},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(71)}},
		{Payload: tsPayload(t, wire.PayloadTypeTSi, "0.0.0.0/0")},
		{Payload: tsPayload(t, wire.PayloadTypeTSr, "0.0.0.0/0")},
	}))
	cfxExpectLine(t, logged, "child-sa: creation response refused", childResponseFixAnswer)
}

// VALIDATES: a creation response refused because this node's dataplane would not install
// the Child SA names the local fix.
//
// Goal: a local install failure is not blamed on the peer. Method: the rig's dataplane
// refuses every SA install, the peer answers through its real respondNewChild, and the
// refused-response line MUST carry the install fix and a retry-after.
//
// PREVENTS: an operator sent to change the peer for a refusal his own kernel made.
func TestChildCreateInstallFailureNamesFix(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	r.dp.installErr = errors.New("test: install refused")
	var peerChild *ChildSA
	log, logged := cfxLogger()
	r.createLog = log
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if r.ps.getChildSA() != nil {
		t.Fatal("a creation whose install failed holds a Child SA")
	}
	cfxExpectLine(t, logged, "child-sa: creation response refused", childResponseFixInstall)
}

// VALIDATES: a creation request this node cannot build names its next step and when ze
// tries again.
//
// Goal: the request-side warning follows the same contract as the response side. Method:
// the esp-group holds no proposal, so initiateChildCreate fails, and the request-failed
// line MUST carry the request fix and a retry-after.
//
// PREVENTS: a creation that silently stops with no word on what to change.
func TestChildCreateRequestFailureNamesFix(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.ps.espGroup.Proposals = nil
	log, logged := cfxLogger()
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), log)
	if r.ps.pendingRekey != nil {
		t.Fatal("an esp-group with no proposal sent a creation request")
	}
	cfxExpectLine(t, logged, "child-sa: creation request failed", childRequestFailedFix)
}

// VALIDATES: a creation response refused because the peer's vti bind names no usable
// XFRM interface names the binding fix, not a kernel refusal.
//
// Goal: the operator is sent to the leaf he configured. Method: after the request is
// sent, the peer's vti bind names an interface that is not an XFRM interface, so
// resolveIfID fails; the peer answers through its real respondNewChild, and the
// refused-response line MUST carry the binding fix and a retry-after.
//
// PREVENTS: a bad vti bind reported as something the kernel refused.
func TestChildCreateBadVTIBindNamesFix(t *testing.T) {
	previous := xfrmIfIDOf
	xfrmIfIDOf = func(name string) (uint32, error) {
		return 0, errors.New("test: " + name + " is not an XFRM interface")
	}
	t.Cleanup(func() { xfrmIfIDOf = previous })
	r := createRig(t, ipsec.PFSDisable)
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	r.sa.PeerCfg.VTIBind = "eth0"
	var peerChild *ChildSA
	log, logged := cfxLogger()
	r.createLog = log
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if r.ps.getChildSA() != nil {
		t.Fatal("a creation whose vti binding failed holds a Child SA")
	}
	cfxExpectLine(t, logged, "child-sa: creation response refused", childResponseFixBinding)
}

// VALIDATES: a creation response refused because this node's tunnel address is not an
// IP address names the address fix.
//
// Goal: the operator is sent to local-address and remote-address. Method: after the
// request is sent, the peer's local-address is not an IP address, so newChildSA fails;
// the peer answers through its real respondNewChild, and the refused-response line MUST
// carry the address fix and a retry-after.
//
// PREVENTS: an invalid address reported as something the kernel refused.
func TestChildCreateBadAddressNamesFix(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	r.sa.PeerCfg.LocalAddress = "not-an-address"
	var peerChild *ChildSA
	log, logged := cfxLogger()
	r.createLog = log
	r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if r.ps.getChildSA() != nil {
		t.Fatal("a creation whose tunnel address is invalid holds a Child SA")
	}
	cfxExpectLine(t, logged, "child-sa: creation response refused", childResponseFixAddress)
}
