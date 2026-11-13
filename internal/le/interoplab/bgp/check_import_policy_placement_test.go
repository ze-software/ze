// Design: docs/architecture/testing/interop.md -- paired IGP import-policy proof.

package bgp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type importPolicyProofLab struct {
	recordingLab
	answers map[string]string
	calls   map[string]int
	images  map[string]int
	failAt  string
	stopAt  string
	stop    context.CancelFunc
}

func importPolicyProofKey(peer string, command []string) string {
	return peer + "\n" + strings.Join(command, "\x00")
}

func (lab *importPolicyProofLab) Query(_ context.Context, peer string, command []string, _ []interoplab.EnvironmentVariable) (string, error) {
	key := importPolicyProofKey(peer, command)
	lab.calls[key]++
	if key == lab.failAt {
		return "", errors.New("controlled native placement failure")
	}
	if key == lab.stopAt {
		defer lab.stop()
	}
	answer, found := lab.answers[key]
	if !found {
		return "", errors.New("unexpected proof query: " + key)
	}
	return answer, nil
}

func (lab *importPolicyProofLab) PeerImage(_ context.Context, peer string) (string, error) {
	lab.images[peer]++
	return "sha256:controlled-" + peer, nil
}

func newImportPolicyProofLab(protocol, placement string) *importPolicyProofLab {
	lab := &importPolicyProofLab{answers: make(map[string]string), calls: make(map[string]int), images: make(map[string]int)}
	put := func(peer string, command []string, answer string) {
		lab.answers[importPolicyProofKey(peer, command)] = answer
	}
	put("ze", zeCommand("plugin session ping"), `{"pid":4242}`)
	put("ze", zeCommand("system subsystem list"), `{"subsystems":[{"name":"sysrib-policy-owner","running":true,"stage":"Running","command-count":4}]}`)
	put("ze", []string{"le", "test", "fixture", "plugin/sysrib-process-placement", "4242", "rib", placement}, "OK: actual process placement")
	if protocol == "isis" {
		for _, peer := range []string{peerFRR, peerFRRSink} {
			put(peer, []string{cmdVtysh, "-c", frrShowVersion}, "FRRouting 10.7.1")
			put(peer, []string{cmdVtysh, "-c", frrShowISISNeighbor}, "Up")
		}
		put("ze", zeCommand("show rib"), isisImportControlPrefix)
		put("ze", zeCommand("show isis route"), isisImportHiddenPrefix+"\n"+isisImportControlPrefix)
		put("ze", []string{"ip", "-4", ipObjectRoute, ipActionShow}, isisImportControlPrefix+" via 10.101.0.3")
		put("ze", zeCommand("show rib candidates prefix "+isisImportHiddenPrefix), `[{"prefix":"10.77.1.0/24","paths":[{"source":"isis","hidden":true,"hidden-by":"source","chain":"isis import"}]}]`)
		put("ze", zeCommand("show rib candidates prefix "+isisImportControlPrefix), `[{"prefix":"10.77.2.0/24","paths":[{"source":"isis","selected":true}]}]`)
		for _, prefix := range []string{isisImportHiddenPrefix, isisImportControlPrefix} {
			answer := `{"PREFIX":[{"protocol":"isis","selected":true,"nexthops":[{"ip":"10.102.0.2"}]}]}`
			put(peerFRRSink, []string{cmdVtysh, "-c", "show ip route " + prefix + " json"}, strings.ReplaceAll(answer, "PREFIX", prefix))
		}
		return lab
	}
	put(peerBIRD, []string{cmdBirdc, "show ospf neighbors ospf0"}, ospfStateFull)
	put(peerBIRD, []string{cmdBirdc, "show status"}, "BIRD 3.4.0 ready.")
	put("ze", zeCommand("show rib candidates prefix "+ospfImportTaggedPrefix), `[{"prefix":"10.88.1.0/24","paths":[{"source":"ospf","distance":200},{"source":"static","distance":150,"selected":true}]}]`)
	put("ze", zeCommand("show rib candidates prefix "+ospfImportUntaggedPrefix), `[{"prefix":"10.88.2.0/24","paths":[{"source":"ospf","distance":110,"selected":true},{"source":"static","distance":150}]}]`)
	put("ze", zeCommand("show rib candidates prefix "+ospfImportCentralPrefix), `[{"prefix":"10.88.3.0/24","paths":[{"source":"ospf","distance":110,"hidden":true,"hidden-by":"central","chain":"rib import"}]}]`)
	put("ze", zeCommand("show ospf route"), ospfImportCentralPrefix+"\n"+ospfImportUntaggedPrefix)
	put("ze", zeCommand("show rib"), ospfImportUntaggedPrefix)
	put("ze", []string{"ip", "-4", ipObjectRoute, ipActionShow}, ospfImportUntaggedPrefix+" via 172.30.0.4")
	return lab
}

// The registered entry points must retain every original query and add native
// placement/identity evidence. Returning success without checking cannot pass.
func TestImportPolicyPairedCheckersRetainOriginalProof(t *testing.T) {
	for _, tc := range []struct{ scenario, protocol, placement string }{
		{isisImportPolicyScenario, "isis", "internal"},
		{isisImportPolicyForkedScenario, "isis", "forked"},
		{ospfImportPolicyScenario, "ospf", "internal"},
		{ospfImportPolicyForkedScenario, "ospf", "forked"},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			lab := newImportPolicyProofLab(tc.protocol, tc.placement)
			checker := specialCheckers[tc.scenario]
			if checker == nil {
				t.Fatal("paired checker is not registered")
			}
			if err := checker(t.Context(), &interoplab.CheckContext{Lab: lab}); err != nil {
				t.Fatal(err)
			}
			for key := range lab.answers {
				if lab.calls[key] == 0 {
					t.Errorf("proof did not query %q", key)
				}
			}
			peers := []string{peerBIRD}
			if tc.protocol == "isis" {
				peers = []string{peerFRR, peerFRRSink}
			}
			for _, peer := range peers {
				if lab.images[peer] == 0 {
					t.Errorf("proof did not identify consumed image for %s", peer)
				}
			}
		})
	}
}

func TestImportPolicyPlacementRefusesUnprovedOwner(t *testing.T) {
	for _, tc := range []struct{ name, command, answer, reason string }{
		{"missing PID", "plugin session ping", `{"pid":0}`, "no process identity"},
		{"bad PID reply", "plugin session ping", `invalid`, "decode daemon identity"},
		{"missing owner", "system subsystem list", `{"subsystems":[]}`, "owner is absent"},
		{"stopped owner", "system subsystem list", `{"subsystems":[{"name":"sysrib-policy-owner","running":false}]}`, "not running"},
		{"early owner", "system subsystem list", `{"subsystems":[{"name":"sysrib-policy-owner","running":true,"stage":"Config"}]}`, "finish startup"},
		{"no commands", "system subsystem list", `{"subsystems":[{"name":"sysrib-policy-owner","running":true,"stage":"Running","command-count":0}]}`, "no commands"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lab := newImportPolicyProofLab("ospf", "forked")
			lab.answers[importPolicyProofKey("ze", zeCommand(tc.command))] = tc.answer
			err := checkImportPolicyPlacement(t.Context(), &interoplab.CheckContext{Lab: lab}, "forked")
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("got %v, want %q", err, tc.reason)
			}
		})
	}
	lab := newImportPolicyProofLab("ospf", "forked")
	lab.failAt = importPolicyProofKey("ze", []string{"le", "test", "fixture", "plugin/sysrib-process-placement", "4242", "rib", "forked"})
	if err := checkImportPolicyPlacement(t.Context(), &interoplab.CheckContext{Lab: lab}, "forked"); err == nil || !strings.Contains(err.Error(), "actual system RIB placement") {
		t.Fatalf("native observer failure was not retained: %v", err)
	}
}

func TestOSPFImportCentralRequiresGateAndHealthyControl(t *testing.T) {
	for _, tc := range []struct{ name, command, answer, reason string }{
		{"central path visible", "show rib candidates prefix 10.88.3.0/24", `[{"prefix":"10.88.3.0/24","paths":[{"source":"ospf","distance":110,"selected":true}]}]`, "central OSPF hidden candidate"},
		{"wrong chain", "show rib candidates prefix 10.88.3.0/24", `[{"prefix":"10.88.3.0/24","paths":[{"source":"ospf","distance":110,"hidden":true,"hidden-by":"source","chain":"ospf import"}]}]`, "central OSPF hidden candidate"},
		{"missing SPF subject", "show ospf route", ospfImportUntaggedPrefix, "central OSPF assertion 1"},
		{"hidden subject forwarded", "show rib", ospfImportCentralPrefix + "\n" + ospfImportUntaggedPrefix, "central OSPF assertion 2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			lab := newImportPolicyProofLab("ospf", "internal")
			key := importPolicyProofKey("ze", zeCommand(tc.command))
			lab.answers[key], lab.stopAt, lab.stop = tc.answer, key, cancel
			err := checkOSPFImportCentral(ctx, &interoplab.CheckContext{Lab: lab})
			if err == nil {
				t.Fatal("unproved central gate was accepted")
			}
			if lab.calls[key] == 0 {
				t.Fatalf("guard did not inspect its subject: %v", err)
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("wrong guard failed: %v, want %q", err, tc.reason)
			}
		})
	}
	for _, kernel := range []string{ospfImportCentralPrefix + "\n" + ospfImportUntaggedPrefix, ""} {
		ctx, cancel := context.WithCancel(t.Context())
		lab := newImportPolicyProofLab("ospf", "internal")
		key := importPolicyProofKey("ze", []string{"ip", "-4", ipObjectRoute, ipActionShow})
		lab.answers[key] = kernel
		reason := "central OSPF assertion 4"
		if kernel == "" {
			lab.stopAt, lab.stop = key, cancel
			reason = "central OSPF assertion 3"
		}
		err := checkOSPFImportCentral(ctx, &interoplab.CheckContext{Lab: lab})
		cancel()
		if err == nil {
			t.Fatal("kernel violation or absent control was accepted")
		}
		if lab.calls[key] == 0 {
			t.Fatalf("kernel guard did not run: %v", err)
		}
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("wrong kernel guard failed: %v, want %q", err, reason)
		}
	}
}

func TestISISImportPeerIdentityRejectsUninspectedVersion(t *testing.T) {
	for _, peer := range []string{peerFRR, peerFRRSink} {
		lab := newImportPolicyProofLab("isis", "internal")
		key := importPolicyProofKey(peer, []string{cmdVtysh, "-c", frrShowVersion})
		lab.answers[key] = "FRRouting 10.6.2"
		err := checkISISImportPeerIdentity(t.Context(), &interoplab.CheckContext{Lab: lab})
		if err == nil || !strings.Contains(err.Error(), "differs from inspected FRR") {
			t.Fatalf("wrong version accepted or wrong guard failed: %v", err)
		}
		if lab.calls[key] == 0 {
			t.Fatalf("uninspected peer version was not read: %s", peer)
		}
	}
}
