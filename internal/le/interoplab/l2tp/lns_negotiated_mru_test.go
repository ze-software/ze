// Design: docs/labs/l2tp-interop.md -- configured values cannot pass as negotiation.
// Related: lns_negotiated_mru.go -- registered unequal-MRU live proof.

package l2tp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const lnsDistinctMRUTrace = `Connect: ppp0 <--> pppol2tp
sent [LCP ConfReq id=0x1 <mru 1400> <magic 0x1234>]
rcvd [LCP ConfReq id=0x7 <mru 1492> <auth chap MD5>]
sent [LCP ConfAck id=0x7 <mru 1492> <auth chap MD5>]
rcvd [LCP ConfAck id=0x1 <mru 1400> <magic 0x1234>]
rcvd [CHAP Challenge id=0x1 <1234>, name = "ze"]
sent [CHAP Response id=0x1 <5678>, name = "alice"]
rcvd [CHAP Success id=0x1 "Welcome"]
`

func TestLNSDistinctMRUEvidence(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
		valid bool
	}{
		{"unequal directions", lnsDistinctMRUTrace, true},
		{"equal configured value", strings.ReplaceAll(lnsDistinctMRUTrace, "<mru 1400>", "<mru 1492>"), false},
		{"wrong request acknowledgement", strings.ReplaceAll(lnsDistinctMRUTrace, "rcvd [LCP ConfAck id=0x1", "rcvd [LCP ConfAck id=0x2"), false},
		{"missing peer acknowledgement", strings.ReplaceAll(lnsDistinctMRUTrace, "rcvd [LCP ConfAck ", "diagnostic "), false},
		{"missing Ze policy", strings.ReplaceAll(lnsDistinctMRUTrace, "<mru 1492>", "<mru 1500>"), false},
		{"no authenticated subscriber", strings.ReplaceAll(lnsDistinctMRUTrace, "CHAP Success", "CHAP Failure"), false},
		{"PAP", lnsDistinctMRUTrace + "sent [PAP AuthReq id=0x1]\n", false},
		{"redial", lnsDistinctMRUTrace + "Connect: ppp1 <--> pppol2tp\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := lnsDistinctMRUEvidence(test.trace)
			if (err == nil) != test.valid {
				t.Fatalf("wire evidence error=%v, want valid=%v", err, test.valid)
			}
		})
	}
}

type lnsDistinctMRULab struct {
	lnsPolicyLab
	listMRU   uint16
	detailMRU uint16
	zeMTU     uint16
}

func (l *lnsDistinctMRULab) Exec(ctx context.Context, peer string, args []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command := strings.Join(args, " ")
	switch {
	case command == "cat "+lnsPolicyTrace:
		return interoplab.CommandResult{Stdout: lnsDistinctMRUTrace}, nil
	case command == "ip -j link show dev ppp0" && peer == peerZe:
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`[{"mtu":%d}]`, l.zeMTU)}, nil
	case strings.Contains(command, "/api/v1/execute") && strings.Contains(command, "show l2tp sessions"):
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`{"status":"done","data":[{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":%d,"auth-method":"chap-md5"}]}`, l.listMRU)}, nil
	case strings.Contains(command, "/api/v1/execute") && strings.Contains(command, "show l2tp session id 7"):
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`{"status":"done","data":{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":%d,"auth-method":"chap-md5"}}`, l.detailMRU)}, nil
	}
	return l.lnsPolicyLab.Exec(ctx, peer, args, environment)
}

// TestLNSMeasuredOutputUnequalMRUs drives the complete registered checker. A
// copied config value must fail in either session view despite usable traffic.
func TestLNSMeasuredOutputUnequalMRUs(t *testing.T) {
	for _, test := range []struct {
		name      string
		listMRU   uint16
		detailMRU uint16
		mtu       uint16
		valid     bool
	}{
		{"measured smaller peer", 1400, 1400, 1400, true},
		{"list copies configured value", 1492, 1400, 1400, false},
		{"detail copies configured value", 1400, 1492, 1400, false},
		{"both copy configured value", 1492, 1492, 1400, false},
		{"interface copies configured value", 1400, 1400, 1492, false},
		{"unknown negotiation", 0, 0, 1400, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lab := &lnsDistinctMRULab{listMRU: test.listMRU, detailMRU: test.detailMRU, zeMTU: test.mtu}
			check := scenarioCheckerMap(time.Second)[scenarioLNSNegotiatedMRU]
			if check == nil {
				t.Fatal("unequal-MRU checker is not registered")
			}
			err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
			if (err == nil) != test.valid {
				t.Fatalf("checker error=%v, want valid=%v", err, test.valid)
			}
			if !test.valid {
				return
			}
			for _, command := range []string{"show l2tp config", "show l2tp sessions", "show l2tp session id 7", "ip -j link show dev ppp0"} {
				if !lnsRecordedCommand(lab.calls, command) {
					t.Errorf("checker did not read Ze's %s", command)
				}
			}
			if !lab.stopped {
				t.Fatal("checker did not finish its cleanup")
			}
		})
	}
}

func TestLNSDistinctMRUScenarioPreparation(t *testing.T) {
	root := l2tpCheckout(t)
	environment := testEnvironment()
	plans, kernelRequired, err := plansAt(root, scenarioLNSNegotiatedMRU, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !kernelRequired || len(plans) != 1 {
		t.Fatalf("unequal-MRU plans=%d kernelRequired=%v", len(plans), kernelRequired)
	}
	prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
		Source: plans[0].Source, Network: (&recordingLab{}).Network(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Peers) != 3 {
		t.Fatalf("prepared %d peers, want backend/Ze/real LAC", len(prepared.Peers))
	}
	for index, want := range []string{peerRadius, peerZe, peerLAC} {
		if prepared.Peers[index].Name != want {
			t.Fatalf("startup peer %d=%s, want %s", index, prepared.Peers[index].Name, want)
		}
	}
	for _, image := range imageBuilds(root, plans, environment.Image) {
		if image.Name == peerRadius && image.Required {
			return
		}
	}
	t.Fatal("unequal-MRU plan does not require the independent RADIUS image")
}
