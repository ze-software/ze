// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- ipsec-unauthenticated-input
// Related: doctor_unauthenticated_input.go -- checkIPsecUnauthenticatedInput
// Related: rfc7296_unprotected_response_test.go -- the forgery and the handshake reused here
// VALIDATES: AC-11: `ze doctor` names a configured peer whose handshake in progress
// dropped messages that failed their integrity check, with the count and the state, and
// says nothing once that handshake established.
// PREVENTS: a tunnel held up by forgeries with no record above debug level.

package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// doctorForgeAuthResponses runs a Ze-to-Ze pre-shared-secret handshake to the
// responder's answer to IKE_AUTH, publishes the pair's table as the running engine's,
// and delivers forgeries copies of that answer with a broken checksum to the initiator
// through handleInbound. It answers the pair and the genuine answer, held back.
func doctorForgeAuthResponses(t *testing.T, forgeries int) (*maPair, []byte) {
	t.Helper()
	p := maSAInitPair(t, maAuth(nil, nil), maAuth(nil, nil))
	p.respondWith(bytes.Clone(p.ini.LastSentMsg), false)
	if p.resp.State != StateEstablished {
		t.Fatalf("responder state after the IKE_AUTH request = %v, want established", p.resp.State)
	}
	genuine := p.resp.LastSentMsg
	SetActiveTableForTest(p.table)
	t.Cleanup(func() { SetActiveTableForTest(nil) })
	for range forgeries {
		handleInbound(p.ini, transport.Packet{Data: unprotTampered(genuine), RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)
	}
	if p.ini.State != StateAuthSent {
		t.Fatalf("initiator state after the forgeries = %v, want IKE_AUTH sent", p.ini.State)
	}
	return p, genuine
}

// TestDoctorReportsHandshakeReceivingUnprotectedMessages delivers three forged IKE_AUTH
// responses to an initiator through the dispatch entry point and asks the check. With no
// engine running it reports nothing; with the handshake held in auth-sent it reports one
// warning, under the registered code, naming the peer, the three drops and the state,
// why Ze kept waiting, and the command that shows the counter.
func TestDoctorReportsHandshakeReceivingUnprotectedMessages(t *testing.T) {
	if got := checkIPsecUnauthenticatedInput(registry.DoctorCheckContext{}); got != nil {
		t.Fatalf("with no engine running the check reported %v, want nothing", got)
	}
	p, _ := doctorForgeAuthResponses(t, 3)

	diags := checkIPsecUnauthenticatedInput(registry.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("three forgeries on one handshake drew %d diagnostics, want one: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Code != diagnostic.CodeDoctorIPsecUnauthenticatedInput {
		t.Fatalf("code %q, want %q", d.Code, diagnostic.CodeDoctorIPsecUnauthenticatedInput)
	}
	if d.Severity != severityWarning {
		t.Fatalf("severity %q, want %q", d.Severity, severityWarning)
	}
	for _, want := range []string{
		"ipsec peer " + p.ini.PeerName + ":",
		"(state auth-sent)",
		"dropped 3 IKE messages that failed their integrity check",
		"Ze kept waiting for the peer's answer",
	} {
		if !strings.Contains(d.Message, want) {
			t.Fatalf("message %q does not say %q", d.Message, want)
		}
	}
	if d.Fix != "show metrics values | match handshake-unauthenticated" {
		t.Fatalf("fix %q, want the command that shows the handshake-unauthenticated series", d.Fix)
	}
}

// TestDoctorIsSilentOnceThePeerEstablished delivers one forgery, checks the warning is
// raised, then delivers the genuine answer: the same SA establishes and the check
// reports nothing, though its drop count is still one.
func TestDoctorIsSilentOnceThePeerEstablished(t *testing.T) {
	p, genuine := doctorForgeAuthResponses(t, 1)
	if got := len(checkIPsecUnauthenticatedInput(registry.DoctorCheckContext{})); got != 1 {
		t.Fatalf("the handshake under a forgery drew %d diagnostics, want one", got)
	}

	handleInbound(p.ini, transport.Packet{Data: genuine, RemoteAddr: p.peerAddr}, p.table, p.myTr, p.log)

	if p.ini.State != StateEstablished {
		t.Fatalf("the genuine answer left the initiator %v, want established", p.ini.State)
	}
	if got := p.ini.unauthenticatedDropCount(); got != 1 {
		t.Fatalf("the SA's drop count is %d, want the one forgery", got)
	}
	if got := checkIPsecUnauthenticatedInput(registry.DoctorCheckContext{}); len(got) != 0 {
		t.Fatalf("an established peer was reported: %v", got)
	}
}
