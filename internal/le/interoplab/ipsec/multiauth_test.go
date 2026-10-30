package ipsec

import (
	"context"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// multipleAuthScriptedLab scripts one run whose tunnel establishes and carries
// ESP both ways, with charon's and Ze's logs as the two variables a test moves.
func multipleAuthScriptedLab(swanLog, zeLog string) *fakeCheckerLab {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: swanLog}
	fake.peerLogs[zePeer] = interoplab.LogResult{Available: true, Text: zeLog}
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\n")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))
	return fake
}

// zeSecondRoundLine is the line Ze's slog text handler writes for
// zeSecondRoundVerified with the peer and id attributes.
func zeSecondRoundLine(identity string) string {
	return `time=2026-10-30T12:00:00Z level=INFO msg="` + zeSecondRoundVerified + `" peer=swan id=` + identity
}

// VALIDATES: the Ze-initiator checker passes only when charon verified Ze's
// certificate round and its pre-shared key round, built its own second round,
// and Ze logged verifying strongSwan's second round, each one removed in turn.
// PREVENTS: a tunnel that came up on one round per side passing as an RFC 4739
// proof, and Ze's log message drifting from the constant the checker reads.
func TestMultipleAuthZeInitiatorCheckerRequiresEveryRound(t *testing.T) {
	swanLines := []string{
		"authentication of 'ze-test-client' with ECDSA_WITH_SHA256_DER successful",
		swanVerifiedZeRoundTwo,
		swanBuiltRoundTwo,
	}
	zeLine := zeSecondRoundLine(multipleAuthSwanRoundTwoID)

	fake := multipleAuthScriptedLab(strings.Join(swanLines, "\n")+"\n", zeLine+"\n")
	if err := checkMultipleAuthZeInitiator(context.Background(), checkerFixture(fake)); err != nil {
		t.Fatalf("a run where both sides logged both rounds was rejected: %v", err)
	}

	for dropped := range swanLines {
		kept := make([]string, 0, len(swanLines)-1)
		for index, line := range swanLines {
			if index != dropped {
				kept = append(kept, line)
			}
		}
		fake := multipleAuthScriptedLab(strings.Join(kept, "\n")+"\n", zeLine+"\n")
		if err := checkMultipleAuthZeInitiator(context.Background(), checkerFixture(fake)); err == nil {
			t.Errorf("charon's log without %q was accepted", swanLines[dropped])
		}
	}

	fake = multipleAuthScriptedLab(strings.Join(swanLines, "\n")+"\n", "level=INFO msg=\"ike: IKE SA established\"\n")
	err := checkMultipleAuthZeInitiator(context.Background(), checkerFixture(fake))
	if err == nil || !strings.Contains(err.Error(), zeSecondRoundVerified) {
		t.Fatalf("Ze's log without its second-round line was accepted or not named: %v", err)
	}
}

// VALIDATES: Ze's second-round line counts only when its own line names the
// peer and the round-2 identity; the same words split over two lines, or the
// round-1 identity, fail.
// PREVENTS: an id attribute from another log line binding to the round-2 message.
func TestMultipleAuthZeSecondRoundLineBindsItsAttributes(t *testing.T) {
	swanLog := swanVerifiedZeCertificatePrefix + "ECDSA_WITH_SHA256_DER successful" + "\n" + swanVerifiedZeRoundTwo + "\n" +
		swanBuiltRoundTwo + "\n"

	for name, zeLog := range map[string]string{
		"split over two lines": `level=INFO msg="` + zeSecondRoundVerified + `" peer=swan` + "\nlevel=INFO msg=other id=" + multipleAuthSwanRoundTwoID + "\n",
		"another identity":     zeSecondRoundLine(zeIP) + "\n",
		"another peer":         `level=INFO msg="` + zeSecondRoundVerified + `" peer=branch id=` + multipleAuthSwanRoundTwoID + "\n",
	} {
		fake := multipleAuthScriptedLab(swanLog, zeLog)
		if err := checkMultipleAuthZeResponder(context.Background(), checkerFixture(fake)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	fake := multipleAuthScriptedLab(swanLog, zeSecondRoundLine(multipleAuthSwanRoundTwoID)+"\n")
	if err := checkMultipleAuthZeResponder(context.Background(), checkerFixture(fake)); err != nil {
		t.Fatalf("a run where both sides logged both rounds was rejected: %v", err)
	}
}

// VALIDATES: the Ze-responder checker refuses a charon log that verified only
// one of Ze's two rounds.
// PREVENTS: the round-1 certificate line standing in for round 2, or round 2
// standing in for round 1.
func TestMultipleAuthZeResponderCheckerRequiresBothOfZesRounds(t *testing.T) {
	zeLog := zeSecondRoundLine(multipleAuthSwanRoundTwoID) + "\n"
	for name, swanLog := range map[string]string{
		"round 1 only": swanVerifiedZeCertificatePrefix + "ECDSA_WITH_SHA256_DER successful" + "\n" +
			swanBuiltRoundTwo + "\n",
		"round 2 only": swanVerifiedZeRoundTwo + "\n" + swanBuiltRoundTwo + "\n",
		"no own round": swanVerifiedZeCertificatePrefix + "ECDSA_WITH_SHA256_DER successful" + "\n" +
			swanVerifiedZeRoundTwo + "\n",
	} {
		fake := multipleAuthScriptedLab(swanLog, zeLog)
		if err := checkMultipleAuthZeResponder(context.Background(), checkerFixture(fake)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
