// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- the childless IKE SA
// Related: create_child.go -- initiateChildCreate and applyChildCreateResponse
// Related: transport_mode.go -- wantsTransportMode and recordInitiatorTransportMode

package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// cctCarriesTransportNotify reports whether a payload chain holds USE_TRANSPORT_MODE.
func cctCarriesTransportNotify(inner []wire.PayloadEntry) bool {
	for i := range inner {
		notify, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if notify.NotifyMsgType == wire.NotifyUseTransportMode {
			return true
		}
	}
	return false
}

// VALIDATES: AC-4 under `mode transport`. A childless IKE SA initiator whose peer runs
// transport mode asks for its Child SA with USE_TRANSPORT_MODE in the CREATE_CHILD_SA
// request, and the Child SA the answer installs runs transport mode on both ends.
//
// Goal: the creation path makes the same mode request IKE_AUTH makes. RFC 7296 Section
// 1.3.1: "The USE_TRANSPORT_MODE notification MAY be included in a request message that
// also includes an SA payload requesting a Child SA." Method: both ends of one IKE SA run
// mode transport; the peer answers through its real respondNewChild. The request MUST
// carry the notify, and both Child SAs MUST carry modeTransport.
//
// PREVENTS: a creation that silently brings up a tunnel-mode Child SA for a peer the
// operator configured for transport mode.
func TestChildlessInitiatorCreatesTransportChildSA(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.sa.PeerCfg.Mode = dataplane.ModeTransport
	r.peer.PeerCfg.Mode = dataplane.ModeTransport
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}

	var peerChild *ChildSA
	req, out := r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if !cctCarriesTransportNotify(req) {
		t.Error("the creation request under mode transport carries no USE_TRANSPORT_MODE")
	}
	child := r.ps.getChildSA()
	if child == nil {
		t.Fatal("the answered transport-mode creation installed no Child SA")
	}
	if out.createdChild != child {
		t.Error("the owner loop was not handed the created Child SA")
	}
	if child.Mode != modeTransport {
		t.Errorf("the created Child SA runs mode %d, want transport (%d)", child.Mode, modeTransport)
	}
	if peerChild == nil {
		t.Fatal("the peer installed no Child SA")
	}
	if peerChild.Mode != modeTransport {
		t.Errorf("the peer's Child SA runs mode %d, want transport (%d)", peerChild.Mode, modeTransport)
	}
}

// VALIDATES: AC-4 and AC-6 under `transport-required`. A childless IKE SA initiator that
// requires transport mode, answered by a peer that declines it, installs nothing, deletes
// the pair the peer installed, keeps the IKE SA, and backs the creation off.
//
// Goal: the creation path enforces the operator's transport requirement as IKE_AUTH does.
// Method: ze runs mode transport with transport-required, the peer runs tunnel mode, and
// the peer answers through its real respondNewChild, which leaves USE_TRANSPORT_MODE out.
// ze MUST install no Child SA, MUST send an ESP Delete for the inbound SPI it offered, MUST
// keep the IKE SA established, and MUST double the creation wait.
//
// RFC requirement: RFC7296-1.3.1-2 positive -- "If the responder declines the request,
// the Child SA will be established in tunnel mode. If this is unacceptable to the
// initiator, the initiator MUST delete the SA" (RFC 7296 Section 1.3.1). On the
// CREATE_CHILD_SA creation path, applyChildCreateResponse (create_child.go) refuses the
// tunnel-mode answer and finishChildCreate deletes the pair the peer installed.
//
// PREVENTS: a transport-required peer running a tunnel-mode Child SA it refused.
func TestChildlessInitiatorDeletesDeclinedTransportChildSA(t *testing.T) {
	r := createRig(t, ipsec.PFSDisable)
	r.sa.PeerCfg.Mode = dataplane.ModeTransport
	r.sa.PeerCfg.TransportRequired = true
	r.ps.serviceChildCreate(r.sa, r.myTr, time.Now(), slogutil.DiscardLogger())
	if r.ps.pendingRekey == nil {
		t.Fatal("a due creation sent no creation request")
	}
	inSPI := r.ps.pendingRekey.newInboundSPI

	var peerChild *ChildSA
	_, out := r.relayCreate(t, r.peerAnswersCreate(t, &peerChild))
	if peerChild == nil {
		t.Fatal("setup: the peer did not accept the creation")
	}
	if peerChild.Mode != modeTunnel {
		t.Fatalf("setup: the peer installed mode %d, want the tunnel mode that declines transport", peerChild.Mode)
	}
	if out.createdChild != nil {
		t.Error("a declined transport-required creation was handed to the owner loop")
	}
	if r.ps.getChildSA() != nil {
		t.Fatal("a declined transport-required creation installed a Child SA")
	}
	expectDeleteOf(t, r, inSPI)
	if r.sa.State != StateEstablished {
		t.Errorf("the IKE SA is %v after the declined creation, want established", r.sa.State)
	}
	if r.ps.childCreate.wait != 2*childCreateFirst {
		t.Errorf("the creation wait is %v after the declined creation, want %v", r.ps.childCreate.wait, 2*childCreateFirst)
	}
}
