// Design: docs/labs/l2tp-interop.md -- oversized MRU requests must be Nakked on wire.
// Related: lns_policy.go -- typed policy packet assertions.

package l2tp

import (
	"strings"
	"testing"
)

const lnsMRUNakTrace = `Connect: ppp0 <--> pppol2tp
sent [LCP ConfReq id=0x1 <mru 1493> <magic 0x1234>]
rcvd [LCP ConfReq id=0x7 <mru 1492> <auth chap MD5>]
sent [LCP ConfAck id=0x7 <mru 1492> <auth chap MD5>]
rcvd [LCP ConfNak id=0x1 <mru 1492>]
sent [LCP ConfReq id=0x2 <mru 1492> <magic 0x1234>]
rcvd [LCP ConfAck id=0x2 <mru 1492> <magic 0x1234>]
rcvd [CHAP Challenge id=0x1 <1234>, name = "ze"]
sent [CHAP Response id=0x1 <5678>, name = "alice"]
rcvd [CHAP Success id=0x1 "Welcome"]
`

const lnsPreviouslyAcceptedMRUTrace = `Connect: ppp0 <--> pppol2tp
sent [LCP ConfReq id=0x1 <mru 1492> <magic 0x1234>]
rcvd [LCP ConfReq id=0x7 <mru 1492> <auth chap MD5>]
sent [LCP ConfAck id=0x7 <mru 1492> <auth chap MD5>]
rcvd [LCP ConfAck id=0x1 <mru 1492> <magic 0x1234>]
rcvd [CHAP Challenge id=0x1 <1234>, name = "ze"]
sent [CHAP Response id=0x1 <5678>, name = "alice"]
rcvd [CHAP Success id=0x1 "Welcome"]
`

// TestLNSLocalMRUProofSurvivesNakStrengthening retains the previous input's
// local-policy and authentication proof without counting it as a Nak proof.
func TestLNSLocalMRUProofSurvivesNakStrengthening(t *testing.T) {
	lines, ackIndex, err := lnsLocalMRUAgreement(lnsPreviouslyAcceptedMRUTrace)
	if err != nil {
		t.Fatal(err)
	}
	if err := lnsCHAPSuccess(lines, ackIndex+1); err != nil {
		t.Fatal(err)
	}
	if err := lnsMRUEvidence(lnsPreviouslyAcceptedMRUTrace); err == nil {
		t.Fatal("the previous trace still passes without an oversized request or Nak")
	}
}

// TestLNSMRUEvidenceNakSequence rejects a successful session whose trace does
// not prove the promised oversized request and its correlated convergence.
func TestLNSMRUEvidenceNakSequence(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
		valid bool
	}{
		{"converged", lnsMRUNakTrace, true},
		{"no oversized request", strings.ReplaceAll(lnsMRUNakTrace, "<mru 1493>", "<mru 1492>"), false},
		{"no Nak", strings.ReplaceAll(lnsMRUNakTrace, "rcvd [LCP ConfNak ", "diagnostic "), false},
		{"wrong Nak identifier", strings.ReplaceAll(lnsMRUNakTrace, "ConfNak id=0x1", "ConfNak id=0x3"), false},
		{"wrong Nak MRU", strings.ReplaceAll(lnsMRUNakTrace, "ConfNak id=0x1 <mru 1492>", "ConfNak id=0x1 <mru 1493>"), false},
		{"no retry", strings.ReplaceAll(lnsMRUNakTrace, "sent [LCP ConfReq id=0x2", "diagnostic "), false},
		{"retry still oversized", strings.ReplaceAll(lnsMRUNakTrace, "ConfReq id=0x2 <mru 1492>", "ConfReq id=0x2 <mru 1493>"), false},
		{"wrong converged Ack identifier", strings.ReplaceAll(lnsMRUNakTrace, "ConfAck id=0x2", "ConfAck id=0x3"), false},
		{"CHAP before convergence", strings.ReplaceAll(lnsMRUNakTrace, "rcvd [LCP ConfAck id=0x2 <mru 1492> <magic 0x1234>]\n", "") + "rcvd [LCP ConfAck id=0x2 <mru 1492>]\n", false},
		{"no CHAP success", strings.ReplaceAll(lnsMRUNakTrace, "CHAP Success", "CHAP Failure"), false},
		{"PAP", lnsMRUNakTrace + "sent [PAP AuthReq id=0x1]\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := lnsMRUEvidence(test.trace)
			if (err == nil) != test.valid {
				t.Fatalf("evidence error=%v, want valid=%v", err, test.valid)
			}
		})
	}
}
