// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- ipsec-peer-malformed-message
// Related: doctor_peer_malformed.go -- checkIPsecPeerMalformedMessage
// Related: inbound.go -- handleOwnedInbound, which counts the message on the SA
// VALIDATES: `ze doctor` names a peer whose live IKE SA received authenticated messages
// that do not parse, with the count, and says nothing for a peer that sent none.
// PREVENTS: a peer's defective IKE implementation recorded only in WARN lines.

package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// doctorSealMalformedInformationalResponse seals, under sender's keys, an INFORMATIONAL
// response at messageID whose one inner payload header gives a Length of 2, below the 4
// octets of the header itself. The checksum verifies, so the chain is the one fault.
func doctorSealMalformedInformationalResponse(t *testing.T, sender *SA, messageID uint32) []byte {
	t.Helper()
	inner := []byte{0, 0, 0, 2}
	flags := wire.FlagResponse | initiatorFlag(sender)
	var raw []byte
	var err error
	if sender.Proposal.Encryption.IsAEAD {
		raw, err = buildSKMessageAEADWithMsgID(sender, inner, wire.PayloadTypeNotify, messageID, wire.ExchangeInformational, flags)
	} else {
		raw, err = buildSKMessageCBCWithMsgID(sender, inner, wire.PayloadTypeNotify, messageID, wire.ExchangeInformational, flags)
	}
	if err != nil {
		t.Fatalf("seal the INFORMATIONAL response: %v", err)
	}
	return raw
}

// doctorPublishEstablished publishes a table holding sa as the running engine's.
func doctorPublishEstablished(t *testing.T, sa *SA) {
	t.Helper()
	table := NewSATable()
	table.Insert(sa)
	SetActiveTableForTest(table)
	t.Cleanup(func() { SetActiveTableForTest(nil) })
}

// TestDoctorReportsPeerSendingMalformedMessages delivers to an established SA, through
// handleOwnedInbound, two out-of-window INFORMATIONAL responses the peer sealed over a
// chain that does not parse. With no engine running the check reports nothing; then it
// reports one warning under the registered code naming the peer, the two messages, that
// the defect is the peer's, and the command that shows the SA.
func TestDoctorReportsPeerSendingMalformedMessages(t *testing.T) {
	if got := checkIPsecPeerMalformedMessage(registry.DoctorCheckContext{}); got != nil {
		t.Fatalf("with no engine running the check reported %v, want nothing", got)
	}
	log := slogutil.DiscardLogger()
	ini, peer, ps, _, myTr := osrSession(t)
	doctorPublishEstablished(t, ini)
	for i := range uint32(2) {
		raw := doctorSealMalformedInformationalResponse(t, peer, ini.NextMsgID+7+i)
		if out := ps.handleOwnedInbound(ini, transport.Packet{Data: raw}, myTr, nil, log); out.dpdResp || out.peerAlive {
			t.Fatalf("a response that does not parse was reported: %+v", out)
		}
	}
	if ini.State != StateEstablished {
		t.Fatalf("state after the malformed responses = %v, want established", ini.State)
	}

	diags := checkIPsecPeerMalformedMessage(registry.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("two malformed responses drew %d diagnostics, want one: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Code != diagnostic.CodeDoctorIPsecPeerMalformedMessage {
		t.Fatalf("code %q, want %q", d.Code, diagnostic.CodeDoctorIPsecPeerMalformedMessage)
	}
	if d.Severity != severityWarning {
		t.Fatalf("severity %q, want %q", d.Severity, severityWarning)
	}
	for _, want := range []string{
		"ipsec peer " + ini.PeerName + " sent 2 IKE messages",
		"integrity check verified",
		"the defect is in the peer's IKE implementation",
	} {
		if !strings.Contains(d.Message, want) {
			t.Fatalf("message %q does not say %q", d.Message, want)
		}
	}
	if d.Fix != "show vpn ipsec sa" {
		t.Fatalf("fix %q, want show vpn ipsec sa", d.Fix)
	}
}

// TestDoctorIsSilentForAPeerWithNoMalformedMessage delivers the same out-of-window
// INFORMATIONAL response sealed over an empty chain, which parses: it is reported as a
// probe answer, and the check reports nothing.
func TestDoctorIsSilentForAPeerWithNoMalformedMessage(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, _, myTr := osrSession(t)
	doctorPublishEstablished(t, ini)
	msgID := ini.NextMsgID + 7
	good, err := buildEncryptedMessageEx(peer, nil, msgID, wire.ExchangeInformational, initiatorFlag(peer)|wire.FlagResponse)
	if err != nil {
		t.Fatalf("build the peer response at id %d: %v", msgID, err)
	}
	if out := ps.handleOwnedInbound(ini, transport.Packet{Data: good}, myTr, nil, log); !out.dpdResp {
		t.Fatal("the well-formed response was not reported, so it may not have reached the arm")
	}
	if got := checkIPsecPeerMalformedMessage(registry.DoctorCheckContext{}); len(got) != 0 {
		t.Fatalf("a peer that sent no malformed message was reported: %v", got)
	}
}
