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

// cfxExpectResponderLine checks that the captured log holds a line carrying msg whose fix
// attribute is fix. This node refused as the responder and only the peer asks again, so
// the line MUST NOT name a retry-after.
func cfxExpectResponderLine(t *testing.T, logged *bytes.Buffer, msg, fix string) {
	t.Helper()
	for line := range strings.Lines(logged.String()) {
		if !strings.Contains(line, msg) {
			continue
		}
		if strings.Contains(line, "retry-after=") {
			t.Errorf("the responder's %q line names a retry-after it never keeps: %s", msg, line)
		}
		if !strings.Contains(line, "fix=\""+fix+"\"") {
			t.Errorf("the %q line does not carry the fix %q: %s", msg, fix, line)
		}
		return
	}
	t.Fatalf("no %q line was logged:\n%s", msg, logged.String())
}

// cfxStubXFRMIfID makes every vti bind name an interface that is not an XFRM interface.
func cfxStubXFRMIfID(t *testing.T) {
	t.Helper()
	previous := xfrmIfIDOf
	xfrmIfIDOf = func(name string) (uint32, error) {
		return 0, errors.New("test: " + name + " is not an XFRM interface")
	}
	t.Cleanup(func() { xfrmIfIDOf = previous })
}

// VALIDATES: a peer's new Child SA request this node refuses for a local cause (a bad vti
// bind, a tunnel address that is not an IP address, a kernel that refuses the install)
// names the local fix, and the peer still reads the generic NO_PROPOSAL_CHOSEN.
//
// Goal: the operator is sent to the leaf on this node that caused the refusal. Method: a
// childless established responder receives a well-formed request through its real
// handleNewChildRequest with one local cause broken; the refusal line MUST carry that
// cause's responder fix and no retry-after, and the answer MUST carry NO_PROPOSAL_CHOSEN
// (RFC 7296 Section 3.10.1).
//
// PREVENTS: a local refusal blamed on the peer's proposals and traffic selectors.
func TestResponderChildRefusalNamesLocalFix(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, link errPair) dataplane.Dataplane
		fix   string
	}{
		{name: "bad vti bind", fix: childRefusedFixResponderBinding,
			setup: func(t *testing.T, link errPair) dataplane.Dataplane {
				cfxStubXFRMIfID(t)
				link.resp.PeerCfg.VTIBind = "eth0"
				return nil
			}},
		{name: "bad address", fix: childRefusedFixResponderAddress,
			setup: func(_ *testing.T, link errPair) dataplane.Dataplane {
				link.resp.PeerCfg.LocalAddress = "not-an-address"
				return nil
			}},
		{name: "kernel refusal", fix: childRefusedFixResponderInstall,
			setup: func(_ *testing.T, _ errPair) dataplane.Dataplane {
				return &bypassDP{installErr: errors.New("test: install refused")}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			link := errLink(t)
			ps := link.ps
			ps.peerName = "responder-refusal"
			ps.espGroup = testESPGroup()
			ps.setChildSA(nil)
			dp := tc.setup(t, link)
			inner := peerNewChildRequest(t, ps.espGroup, 0x0c0ffee1, testNonce(62), nil, false,
				"0.0.0.0/0", "0.0.0.0/0")
			msg := &wire.Message{Header: wire.Header{MessageID: link.resp.ExpectedMsgID}}
			log, logged := cfxLogger()
			ps.handleNewChildRequest(link.resp, msg, inner, link.myTr, dp, log)
			if ps.getChildSA() != nil {
				t.Fatal("a refused request installed a Child SA")
			}
			got := rtxRecv(t, link.peerTr)
			if got == nil {
				t.Fatal("a refused request drew no answer")
			}
			if types := errNotifyIn(t, link.ini, got); len(types) != 1 || types[0] != wire.NotifyNoProposalChosen {
				t.Errorf("the answer carries notifies %v, want exactly NO_PROPOSAL_CHOSEN", types)
			}
			cfxExpectResponderLine(t, logged, "ike: refused a peer request for a new Child SA", tc.fix)
		})
	}
}

// VALIDATES: the Child SA piggybacked on IKE_AUTH, refused because this node's vti bind
// names no usable XFRM interface, names the binding fix while the IKE SA is kept.
//
// Goal: the IKE_AUTH refusal line follows the same per-cause contract. Method: an
// established PSK responder rebuilds its IKE_AUTH answer through buildAuthResponse with a
// vti bind that resolveIfID refuses; the childless answer's line MUST carry the binding
// fix and no retry-after.
//
// PREVENTS: a bad vti bind on the responder reported as a proposal mismatch.
func TestResponderAuthChildRefusalNamesBindingFix(t *testing.T) {
	cfxStubXFRMIfID(t)
	_, resp, ps := establishPSK(t)
	resp.PeerCfg.VTIBind = "eth0"
	log, logged := cfxLogger()
	out, child, err := ps.buildAuthResponse(resp, 2, nil, nil, nil, false, log)
	if err != nil {
		t.Fatalf("the childless IKE_AUTH answer could not be built: %v", err)
	}
	if out == nil || child != nil {
		t.Fatal("a refused IKE_AUTH Child SA did not produce a childless answer")
	}
	cfxExpectResponderLine(t, logged, "ike: refused the IKE_AUTH Child SA", childRefusedFixResponderBinding)
}

// VALIDATES: the Child SA piggybacked on IKE_AUTH, refused because this node has no tunnel
// address or because the kernel refused the install, names that local cause's fix while the
// IKE SA is kept.
//
// Goal: createFirstChildSA marks both causes, so the IKE_AUTH refusal line follows the same
// per-cause contract as a refused CREATE_CHILD_SA request. Method: an established PSK
// responder rebuilds its IKE_AUTH answer through buildAuthResponse with one local cause
// broken; the childless answer's line MUST carry that cause's responder fix.
//
// PREVENTS: a bad tunnel address or a kernel refusal on the responder reported as a
// proposal mismatch.
func TestResponderAuthChildRefusalNamesLocalFix(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, resp *SA)
		fix   string
	}{
		{name: "bad address", fix: childRefusedFixResponderAddress,
			setup: func(_ *testing.T, resp *SA) {
				resp.PeerCfg.LocalAddress = "not-an-address"
			}},
		{name: "kernel refusal", fix: childRefusedFixResponderInstall,
			setup: func(t *testing.T, _ *SA) {
				loadFailClosedBackend(t, &bypassDP{installErr: errors.New("test: install refused")})
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, resp, ps := establishPSK(t)
			tc.setup(t, resp)
			log, logged := cfxLogger()
			out, child, err := ps.buildAuthResponse(resp, 2, nil, nil, nil, false, log)
			if err != nil {
				t.Fatalf("the childless IKE_AUTH answer could not be built: %v", err)
			}
			if out == nil || child != nil {
				t.Fatal("a refused IKE_AUTH Child SA did not produce a childless answer")
			}
			cfxExpectResponderLine(t, logged, "ike: refused the IKE_AUTH Child SA", tc.fix)
		})
	}
}
