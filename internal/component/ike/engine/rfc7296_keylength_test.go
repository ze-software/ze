// VALIDATES: the initiator refuses an accepted offer whose encryption key is longer than
// the key it offered, and the responder reports every key it accepts above its own policy.
// PREVENTS: the silent upgrade. Ze offers one Key Length attribute of 128 for an aes128
// configuration. A responder that answers with 256 once passed the consistency check, and
// the key hierarchy was then derived at 32 octets with no record anywhere.
package engine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// klnIKEGroup is an IKE policy of one proposal that names a 128-bit key. It is what an
// operator configuration of aes128 produces.
func klnIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "kln-ike",
		Proposals: []ipsec.IKEProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES128,
			Hash:       ipsec.HashSHA256,
			DHGroup:    14,
		}},
	}
}

// klnESPGroup is the Child SA policy that matches klnIKEGroup.
func klnESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "kln-esp",
		Lifetime: 3600,
		Proposals: []ipsec.ESPProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES128,
			Hash:       ipsec.HashSHA256,
		}},
	}
}

// RFC requirement: RFC7296-3.3.6-3 negative -- RFC 7296 Section 3.3.6: "The initiator of an
// exchange MUST check that the accepted offer is consistent with one of its proposals".
// It MUST stop the exchange when the offer does not agree.
// Ze offers one Key Length attribute of 128 for an aes128 policy. An accepted offer that
// names 256 is a suite ze never sent. verifyAcceptedOffer refuses it for the IKE SA and
// for the Child SA alike.
// RFC requirement: RFC7296-3.3.6-3 positive -- the same policy accepts the answer that carries the
// 128 ze really sent. The refusal comes from the unsent key length rather than from a
// path that refuses every answer.
func TestKlnInitiatorRefusesLongerAcceptedKey(t *testing.T) {
	ike := &wire.PayloadSA{Proposals: buildWireIKEProposals(klnIKEGroup())}
	if _, err := verifyAcceptedOffer(ike, klnIKEGroup(), klnESPGroup()); err != nil {
		t.Fatalf("the offer ze really sent returned %v, want acceptance", err)
	}
	negSetKeyLength(ike, 256)
	if _, err := verifyAcceptedOffer(ike, klnIKEGroup(), klnESPGroup()); err == nil {
		t.Error("an IKE answer of 256 bits against a 128-bit offer was accepted")
	}

	esp := &wire.PayloadSA{Proposals: buildWireESPProposals(klnESPGroup(), 0x11223344, dhGroupNone)}
	if _, err := verifyAcceptedOffer(esp, klnIKEGroup(), klnESPGroup()); err != nil {
		t.Fatalf("the ESP offer ze really sent returned %v, want acceptance", err)
	}
	negSetKeyLength(esp, 256)
	if _, err := verifyAcceptedOffer(esp, klnIKEGroup(), klnESPGroup()); err == nil {
		t.Error("an ESP answer of 256 bits against a 128-bit offer was accepted")
	}
}

// VALIDATES: the responder states every key length it accepts above its own policy. RFC
// 7296 Section 3.3.5 lets it accept a key that supplies greater security, and
// ai/rules/protocol.md makes that choice visible to the operator.
// PREVENTS: an operator who configures aes128, runs AES-256, and reads nothing about it.
func TestKlnResponderReportsAcceptedLongerKey(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	logKeyLengthUpgrade(log, "branch", crypto.IKEProposal{
		Encryption:      crypto.EncryptionTransform{ID: crypto.ENCR_AES_CBC, KeyLength: 256},
		PolicyKeyLength: 128,
	})
	got := buf.String()
	if !strings.Contains(got, "configured-bits=128") || !strings.Contains(got, "accepted-bits=256") {
		t.Errorf("the upgrade log line is %q, want the configured and the accepted lengths", got)
	}

	buf.Reset()
	logKeyLengthUpgrade(log, "branch", crypto.IKEProposal{
		Encryption: crypto.EncryptionTransform{ID: crypto.ENCR_AES_CBC, KeyLength: 128},
	})
	if buf.Len() != 0 {
		t.Errorf("an accepted key of the configured length logged %q, want silence", buf.String())
	}
}
