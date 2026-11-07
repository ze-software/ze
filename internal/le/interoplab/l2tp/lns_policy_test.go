// Design: docs/labs/l2tp-interop.md -- discriminating LNS policy evidence.
// Related: lns_policy.go -- registered live peer checkers.
package l2tp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const lnsMRUTrace = lnsMRUNakTrace

const lnsRefusalTrace = `Connect: ppp0 <--> pppol2tp
sent [LCP ConfReq id=0x1 <mru 1500>]
rcvd [LCP ConfReq id=0x7 <mru 1500> <auth chap MD5>]
sent [LCP ConfNak id=0x7 <auth pap>]
rcvd [LCP TermReq id=0x8 "authentication method refused"]
sent [LCP TermAck id=0x8]
Modem hangup
Connection terminated.
`

// TestLNSMRUEvidence refuses a default-1500 request even if the independently
// configured client MRU would still cap its MTU at 1492.
func TestLNSMRUEvidence(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
		valid bool
	}{
		{"negotiated", lnsMRUTrace, true},
		{"ignored Ze MRU leaf", strings.ReplaceAll(lnsMRUTrace, "<mru 1492> <auth", "<mru 1500> <auth"), false},
		{"missing request", strings.ReplaceAll(lnsMRUTrace, "rcvd [LCP ConfReq", "unrelated"), false},
		{"ack for another request", strings.ReplaceAll(lnsMRUTrace, "ConfAck id=0x7", "ConfAck id=0x9"), false},
		{"no CHAP success", strings.ReplaceAll(lnsMRUTrace, "CHAP Success", "CHAP Failure"), false},
		{"PAP authentication", lnsMRUTrace + "sent [PAP AuthReq id=0x1]\n", false},
		{"two attempts", lnsMRUTrace + "Connect: ppp1 <--> pppol2tp\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := lnsMRUEvidence(test.trace)
			if (err == nil) != test.valid {
				t.Fatalf("evidence verdict = %v, want valid=%v", err, test.valid)
			}
		})
	}
}

// TestLNSCHAPOnlyEvidence makes the refusal proof fail for PAP, missing or
// miscorrelated packets, wrong order, post-LCP progress and repeated dials.
func TestLNSCHAPOnlyEvidence(t *testing.T) {
	for _, test := range []struct {
		name  string
		trace string
		valid bool
	}{
		{"refused", lnsRefusalTrace, true},
		{"no protocol evidence", "Connect: ppp0 <--> pppol2tp\n", false},
		{"no PAP Nak", strings.ReplaceAll(lnsRefusalTrace, "<auth pap>", "<auth eap>"), false},
		{"Nak wrong identifier", strings.ReplaceAll(lnsRefusalTrace, "ConfNak id=0x7", "ConfNak id=0x9"), false},
		{"no Ze TermReq", strings.ReplaceAll(lnsRefusalTrace, "rcvd [LCP TermReq", "unrelated"), false},
		{"TermAck wrong identifier", strings.ReplaceAll(lnsRefusalTrace, "TermAck id=0x8", "TermAck id=0x9"), false},
		{"TermReq before Nak", "rcvd [LCP TermReq id=0x8]\n" + strings.ReplaceAll(lnsRefusalTrace, "rcvd [LCP TermReq", "unrelated"), false},
		{"Ze offers PAP", lnsRefusalTrace + "rcvd [LCP ConfReq id=0x9 <auth pap>]\n", false},
		{"PAP packet", lnsRefusalTrace + "sent [PAP AuthReq id=0x1]\n", false},
		{"CHAP handler reached", lnsRefusalTrace + "rcvd [CHAP Challenge id=0x1]\n", false},
		{"LCP opened", lnsRefusalTrace + "sent [IPCP ConfReq id=0x1]\n", false},
		{"redial", lnsRefusalTrace + "Connect: ppp1 <--> pppol2tp\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := lnsCHAPOnlyEvidence(test.trace)
			if (err == nil) != test.valid {
				t.Fatalf("evidence verdict = %v, want valid=%v", err, test.valid)
			}
		})
	}
}

// TestLNSMeasuredOutput refuses missing or configured-default state instead of
// treating it as actual negotiated state, and requires one measured client link.
func TestLNSMeasuredOutput(t *testing.T) {
	for _, body := range []string{
		`{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":1492,"auth-method":"chap-md5"}`,
		`{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":1500,"auth-method":"chap-md5"}`,
		`{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":null,"auth-method":null}`,
		`{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":1492,"auth-method":"pap"}`,
	} {
		var session lnsPolicySession
		if err := json.Unmarshal([]byte(body), &session); err != nil {
			t.Fatal(err)
		}
		valid := strings.Contains(body, `"mru":1492,"auth-method":"chap-md5"`)
		if err := lnsNegotiatedSession(session); (err == nil) != valid {
			t.Errorf("session %s verdict = %v", body, err)
		}
	}
	for _, body := range []string{`[]`, `[{}]`, `[{"mtu":1500}]`, `[{},{}]`, `not-json`} {
		if err := lnsClientMTU(body); err == nil {
			t.Errorf("accepted unmeasured/wrong client MTU: %s", body)
		}
	}
	if err := lnsClientMTU(`[{"mtu":1492}]`); err != nil {
		t.Fatal(err)
	}
}

// TestLNSNoCredentialRequest refuses unread backend evidence and any request,
// including a request whose eventual authentication result was a rejection.
func TestLNSNoCredentialRequest(t *testing.T) {
	for _, logs := range []interoplab.LogResult{
		{},
		{Available: true},
		{Available: true, Text: "radius-mock listening\nRADIUS-RX Access-Request User-Name=alice"},
		{Available: true, Text: "radius-mock listening\nRADIUS-RX Accounting-Request"},
	} {
		if err := lnsNoCredentialRequest(logs); err == nil {
			t.Errorf("accepted unavailable or reached backend: %+v", logs)
		}
	}
	if err := lnsNoCredentialRequest(interoplab.LogResult{Available: true, Text: "radius-mock listening"}); err != nil {
		t.Fatal(err)
	}
}

type lnsPolicyLab struct {
	recordingLab
	refused bool
	answer  string
}

func (l *lnsPolicyLab) Exec(ctx context.Context, peer string, args []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command := strings.Join(args, " ")
	if command == "cat "+lnsPolicyTrace {
		trace := lnsMRUTrace
		if l.refused {
			trace = lnsRefusalTrace
		}
		return interoplab.CommandResult{Stdout: trace}, nil
	}
	if strings.Contains(command, "pidof pppd") {
		return interoplab.CommandResult{Stdout: "exited\n"}, nil
	}
	if command == "ip -j link show dev ppp0" {
		return interoplab.CommandResult{Stdout: `[{"mtu":1492}]`}, nil
	}
	if strings.Contains(command, "/api/v1/execute") {
		l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: args})
		if l.answer != "" {
			return interoplab.CommandResult{Stdout: l.answer}, nil
		}
		var data string
		switch {
		case strings.Contains(command, "show l2tp config"):
			data = `{"mru":1492,"auth-method":"chap-md5","authentication-allow":["chap-md5"]}`
		case strings.Contains(command, "show l2tp sessions"):
			data = `[{"local-sid":7}]`
			if l.refused {
				data = `[]`
			}
		case strings.Contains(command, "show l2tp session id 7"):
			data = `{"local-sid":7,"username":"alice","assigned-addr":"10.100.0.2","mru":1492,"auth-method":"chap-md5"}`
		default:
			return interoplab.CommandResult{}, errors.New("unexpected daemon command: " + command)
		}
		return interoplab.CommandResult{Stdout: `{"status":"done","data":` + data + `}`}, nil
	}
	return l.recordingLab.Exec(ctx, peer, args, environment)
}

func (l *lnsPolicyLab) Logs(ctx context.Context, peer string, lines int) (interoplab.LogResult, error) {
	if l.refused {
		switch peer {
		case peerZe:
			return interoplab.LogResult{Available: true,
				Text: "session established\nauth-method-refused: peer suggested Auth-Protocol 0xc023 (pap); allowed [chap-md5]"}, nil
		case peerRadius:
			return interoplab.LogResult{Available: true, Text: "radius-mock listening on 0.0.0.0:1812"}, nil
		}
	}
	return l.recordingLab.Logs(ctx, peer, lines)
}

// TestLNSPolicyCheckersReachDaemon exercises both complete typed checkers and
// ensures the positive path reads effective config and a specific live session.
func TestLNSPolicyCheckersReachDaemon(t *testing.T) {
	for _, refused := range []bool{false, true} {
		lab := &lnsPolicyLab{refused: refused}
		name := scenarioLNSMRU
		if refused {
			name = scenarioLNSCHAPOnly
		}
		check := scenarioCheckerMap(time.Second)[name]
		if check == nil {
			t.Fatalf("scenario %s is not registered", name)
		}
		if err := check(t.Context(), &interoplab.CheckContext{Lab: lab}); err != nil {
			t.Fatalf("refused=%v checker failed: %v", refused, err)
		}
		if !lab.stopped {
			t.Fatal("checker did not tear down LAC after assertions")
		}
		for _, command := range []string{"show l2tp sessions"} {
			if !lnsRecordedCommand(lab.calls, command) {
				t.Errorf("checker did not read daemon %s", command)
			}
		}
		if refused {
			continue
		}
		for _, command := range []string{"show l2tp config", "show l2tp session id 7"} {
			if !lnsRecordedCommand(lab.calls, command) {
				t.Errorf("positive checker did not read daemon %s", command)
			}
		}
	}
}

func lnsRecordedCommand(calls []recordedCall, command string) bool {
	for _, call := range calls {
		if call.peer == peerZe && strings.Contains(strings.Join(call.arguments, " "), command) {
			return true
		}
	}
	return false
}

// TestLNSPolicyRESTRefusesUnreadState keeps API transport and command failures
// separate from an empty session table, which is meaningful only after refusal.
func TestLNSPolicyRESTRefusesUnreadState(t *testing.T) {
	for _, answer := range []string{`{}`, `{"status":"error","error":"unavailable"}`, `{"status":"done"}`, `{"status":"done","data":null}`, `not-json`} {
		lab := &lnsPolicyLab{answer: answer}
		if _, err := lnsPolicySessions(t.Context(), lab); err == nil {
			t.Errorf("accepted unread daemon state: %s", answer)
		}
	}
}

// TestLNSPolicyScenarioPreparation joins each declarative directory to its
// registered checker and the existing Ze, real LAC and independent backend.
func TestLNSPolicyScenarioPreparation(t *testing.T) {
	root := l2tpCheckout(t)
	for _, name := range []string{scenarioLNSMRU, scenarioLNSCHAPOnly} {
		t.Run(name, func(t *testing.T) {
			environment := testEnvironment()
			plans, kernelRequired, err := plansAt(root, name, environment)
			if err != nil {
				t.Fatal(err)
			}
			if !kernelRequired {
				t.Fatal("policy scenario skipped PPPoL2TP preflight")
			}
			if len(plans) != 1 {
				t.Fatalf("selected %d plans, want one", len(plans))
			}
			prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
				Source: plans[0].Source, Network: (&recordingLab{}).Network(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Peers) != 3 {
				t.Fatalf("prepared %d peers, want independent backend, Ze and LAC", len(prepared.Peers))
			}
			for index, want := range []string{peerRadius, peerZe, peerLAC} {
				if prepared.Peers[index].Name != want {
					t.Errorf("startup peer %d = %s, want %s", index, prepared.Peers[index].Name, want)
				}
			}
			radiusBuilt := false
			for _, image := range imageBuilds(root, plans, environment.Image) {
				if image.Name == imageRadius {
					radiusBuilt = image.Required
				}
			}
			if !radiusBuilt {
				t.Fatal("policy plan did not require the independent RADIUS image")
			}
		})
	}
}
