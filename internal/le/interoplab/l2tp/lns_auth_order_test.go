// Design: docs/labs/l2tp-interop.md -- required-auth refusal before LCP Opened.
// Related: lns_policy.go -- one-attempt packet correlation and live checker.

package l2tp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck supplies the complete refusal
// trace that the old checker accepted, including a peer Ack before termination.
func TestLNSCHAPOnlyEvidenceRejectsEarlyConfAck(t *testing.T) {
	for _, insertion := range []struct {
		name   string
		marker string
		packet string
	}{
		{"before PAP Nak", "sent [LCP ConfNak ", "sent [LCP ConfAck id=0x7 <mru 1500> <auth chap MD5>]\n"},
		{"after PAP Nak", "rcvd [LCP TermReq ", "sent [LCP ConfAck id=0x7 <mru 1500> <auth chap MD5>]\n"},
		{"later Ze request", "rcvd [LCP TermReq ", "rcvd [LCP ConfReq id=0x9 <mru 1500> <auth chap MD5>]\nsent [LCP ConfAck id=0x9 <mru 1500> <auth chap MD5>]\n"},
	} {
		t.Run(insertion.name, func(t *testing.T) {
			trace := strings.Replace(lnsRefusalTrace, insertion.marker, insertion.packet+insertion.marker, 1)
			if err := lnsCHAPOnlyEvidence(trace); err == nil {
				t.Fatal("accepted peer acknowledgement before required-auth refusal")
			} else if !strings.Contains(err.Error(), "LCP could open") {
				t.Fatalf("rejected the complete false-positive trace for another reason: %v", err)
			}
		})
	}
}

// TestLNSCHAPOnlyEvidenceConfAckDirection keeps harmless half-negotiation and
// diagnostic text distinct from an Ack that can open this connection attempt.
func TestLNSCHAPOnlyEvidenceConfAckDirection(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
	}{
		{"valid refusal", lnsRefusalTrace},
		{"Ze acknowledges peer request", strings.Replace(lnsRefusalTrace, "rcvd [LCP TermReq ", "rcvd [LCP ConfAck id=0x1 <mru 1500>]\nrcvd [LCP TermReq ", 1)},
		{"uncorrelated peer Ack", strings.Replace(lnsRefusalTrace, "rcvd [LCP TermReq ", "sent [LCP ConfAck id=0x55 <mru 1500>]\nrcvd [LCP TermReq ", 1)},
		{"diagnostic mention", strings.Replace(lnsRefusalTrace, "rcvd [LCP TermReq ", "diagnostic: expected sent [LCP ConfAck id=0x7 <auth chap MD5>]\nrcvd [LCP TermReq ", 1)},
		{"packet before connection", "rcvd [LCP ConfReq id=0x7 <auth chap MD5>]\nsent [LCP ConfAck id=0x7 <auth chap MD5>]\n" + lnsRefusalTrace},
		{"peer Ack after termination", strings.Replace(lnsRefusalTrace, "sent [LCP TermAck ", "sent [LCP ConfAck id=0x7 <mru 1500> <auth chap MD5>]\nsent [LCP TermAck ", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := lnsCHAPOnlyEvidence(test.trace); err != nil {
				t.Fatalf("rejected supported refusal evidence: %v", err)
			}
		})
	}
	for _, packet := range []string{
		"sent [LCP ConfAck id=invalid]\n",
		"sent [LCP ConfAck id=0x7\n",
	} {
		trace := strings.Replace(lnsRefusalTrace, "rcvd [LCP TermReq ", packet+"rcvd [LCP TermReq ", 1)
		if err := lnsCHAPOnlyEvidence(trace); err == nil {
			t.Fatalf("accepted unread peer acknowledgement record: %q", packet)
		}
	}
}

type lnsAuthOrderLab struct {
	lnsPolicyLab
	trace string
}

func (l *lnsAuthOrderLab) Exec(ctx context.Context, peer string, args []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer == peerLAC && strings.Join(args, " ") == "cat "+lnsPolicyTrace {
		return interoplab.CommandResult{Stdout: l.trace}, nil
	}
	return l.lnsPolicyLab.Exec(ctx, peer, args, environment)
}

// TestLNSCHAPOnlyCheckerRejectsEarlyConfAck exercises the registered live
// checker, retaining the valid refusal control and all its teardown barriers.
func TestLNSCHAPOnlyCheckerRejectsEarlyConfAck(t *testing.T) {
	for _, earlyAck := range []bool{false, true} {
		trace := lnsRefusalTrace
		if earlyAck {
			trace = strings.Replace(trace, "rcvd [LCP TermReq ", "sent [LCP ConfAck id=0x7 <mru 1500> <auth chap MD5>]\nrcvd [LCP TermReq ", 1)
		}
		lab := &lnsAuthOrderLab{lnsPolicyLab: lnsPolicyLab{refused: true}, trace: trace}
		check := scenarioCheckerMap(time.Second)[scenarioLNSCHAPOnly]
		if check == nil {
			t.Fatal("CHAP-only live checker is not registered")
		}
		err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
		if earlyAck {
			if err == nil || !strings.Contains(err.Error(), "LCP could open") {
				t.Fatalf("live checker accepted or misclassified early Ack: %v", err)
			}
			if lab.stopped {
				t.Fatal("cleanup manufactured a refusal before rejecting early Ack")
			}
			continue
		}
		if err != nil {
			t.Fatalf("live checker rejected valid refusal: %v", err)
		}
		if !lab.stopped || !lnsRecordedCommand(lab.calls, "show l2tp sessions") {
			t.Fatal("valid refusal skipped session-removal evidence or cleanup")
		}
	}
}
