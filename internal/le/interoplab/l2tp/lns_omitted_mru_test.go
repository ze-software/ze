// Design: docs/labs/l2tp-interop.md -- default peer MRU and Ze's transmit cap.
// Related: lns_omitted_mru.go -- real peer packet and kernel interface evidence.

package l2tp

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const lnsOmittedMRUTrace = `Connect: ppp0 <--> pppol2tp
sent [LCP ConfReq id=0x1 <magic 0x1234>]
rcvd [LCP ConfReq id=0x7 <mru 1492> <auth chap MD5>]
sent [LCP ConfAck id=0x7 <mru 1492> <auth chap MD5>]
rcvd [LCP ConfAck id=0x1 <magic 0x1234>]
rcvd [CHAP Challenge id=0x1 <1234>, name = "ze"]
sent [CHAP Response id=0x1 <5678>, name = "alice"]
rcvd [CHAP Success id=0x1 "Welcome"]
`

func TestLNSOmittedMRUEvidence(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
		valid bool
	}{
		{"omitted", lnsOmittedMRUTrace, true},
		{"explicit default", strings.ReplaceAll(lnsOmittedMRUTrace, "ConfReq id=0x1 ", "ConfReq id=0x1 <mru 1500> "), false},
		{"no peer request", strings.ReplaceAll(lnsOmittedMRUTrace, "sent [LCP ConfReq ", "diagnostic "), false},
		{"wrong peer Ack", strings.ReplaceAll(lnsOmittedMRUTrace, "ConfAck id=0x1", "ConfAck id=0x2"), false},
		{"no peer Ack", strings.ReplaceAll(lnsOmittedMRUTrace, "rcvd [LCP ConfAck ", "diagnostic "), false},
		{"Ack adds MRU", strings.ReplaceAll(lnsOmittedMRUTrace, "ConfAck id=0x1 ", "ConfAck id=0x1 <mru 1500> "), false},
		{"wrong Ze request", strings.ReplaceAll(lnsOmittedMRUTrace, "<mru 1492>", "<mru 1500>"), false},
		{"PAP", lnsOmittedMRUTrace + "sent [PAP AuthReq id=0x1]\n", false},
		{"no credentials", strings.ReplaceAll(lnsOmittedMRUTrace, "CHAP Success", "CHAP Failure"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := lnsOmittedMRUEvidence(test.trace)
			if (err == nil) != test.valid {
				t.Fatalf("evidence error=%v, want valid=%v", err, test.valid)
			}
		})
	}
}

type lnsOmittedMRULab struct {
	lnsPolicyLab
	zeMTU      uint16
	sessionMRU uint16
}

func (l *lnsOmittedMRULab) Exec(ctx context.Context, peer string, args []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command := strings.Join(args, " ")
	switch {
	case command == "cat "+lnsPolicyTrace:
		return interoplab.CommandResult{Stdout: lnsOmittedMRUTrace}, nil
	case command == "ip -j link show dev ppp0" && peer == peerZe:
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`[{"mtu":%d}]`, l.zeMTU)}, nil
	case strings.Contains(command, "/api/v1/execute") && strings.Contains(command, "show l2tp sessions"):
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`{"status":"done","data":[{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":%d,"auth-method":"chap-md5"}]}`, l.sessionMRU)}, nil
	case strings.Contains(command, "/api/v1/execute") && strings.Contains(command, "show l2tp session id 7"):
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		return interoplab.CommandResult{Stdout: fmt.Sprintf(`{"status":"done","data":{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":%d,"auth-method":"chap-md5"}}`, l.sessionMRU)}, nil
	}
	return l.lnsPolicyLab.Exec(ctx, peer, args, environment)
}

func TestLNSOmittedMRUCheckerMeasuresZe(t *testing.T) {
	for _, test := range []struct {
		name  string
		mtu   uint16
		mru   uint16
		valid bool
	}{
		{"capped with actual peer default", 1492, 1500, true},
		{"uncapped Ze despite capped client", 1500, 1500, false},
		{"copied configured MRU", 1492, 1492, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lab := &lnsOmittedMRULab{zeMTU: test.mtu, sessionMRU: test.mru}
			check := scenarioCheckerMap(time.Second)[scenarioLNSOmittedMRU]
			if check == nil {
				t.Fatal("omitted-MRU checker is not registered")
			}
			err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
			if (err == nil) != test.valid {
				t.Fatalf("checker error=%v, want valid=%v", err, test.valid)
			}
			if !test.valid {
				return
			}
			measuredZe := false
			for _, call := range lab.calls {
				if call.peer == peerZe && strings.Join(call.arguments, " ") == "ip -j link show dev ppp0" {
					measuredZe = true
				}
			}
			if !measuredZe || !lab.stopped {
				t.Fatal("checker did not measure Ze and complete its cleanup")
			}
		})
	}
}

func TestLNSOmittedMRUScenarioPreparation(t *testing.T) {
	root := l2tpCheckout(t)
	environment := testEnvironment()
	plans, kernelRequired, err := plansAt(root, scenarioLNSOmittedMRU, environment)
	if err != nil {
		t.Fatal(err)
	}
	if !kernelRequired || len(plans) != 1 {
		t.Fatalf("omitted-MRU plans=%d kernelRequired=%v", len(plans), kernelRequired)
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
		if image.Name == imageRadius && image.Required {
			return
		}
	}
	t.Fatal("omitted-MRU plan does not require the independent RADIUS image")
}
