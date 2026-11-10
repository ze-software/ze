// Design: docs/labs/l2tp-interop.md -- compiled proxy fixture in the native L2TP suite.

package l2tp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/linuxle"
)

func proxyFixtureTrace(peerMRU string) string {
	return "OK: CHAP Success received\n" +
		"OK: IPCP opened, ze assigned 10.99.8.10\n" +
		"OK: proxy peer MRU " + peerMRU + " installed kernel MTU 1492 on ppp0\n" +
		"OK: proxy peer MRU " + peerMRU + " passed an ICMP round trip with 1492 Information-field octets\n" +
		"OK: proxy PPP interface removed after StopCCN\n"
}

func TestProxyMRUFixtureEvidence(t *testing.T) {
	valid := proxyFixtureTrace("9000")
	for _, tc := range []struct {
		name   string
		output string
		valid  bool
	}{
		{"complete kernel and traffic proof", valid, true},
		{"no CHAP", strings.ReplaceAll(valid, "OK: CHAP Success received", "diagnostic"), false},
		{"no IPCP", strings.ReplaceAll(valid, "OK: IPCP opened, ze assigned", "diagnostic"), false},
		{"uncapped MTU", strings.ReplaceAll(valid, "kernel MTU 1492", "kernel MTU 1500"), false},
		{"wrong peer case", proxyFixtureTrace("1500"), false},
		{"no traffic", strings.ReplaceAll(valid, "passed an ICMP round trip", "diagnostic"), false},
		{"no cleanup", strings.ReplaceAll(valid, "OK: proxy PPP interface removed after StopCCN", "diagnostic"), false},
		{"observer failure", valid + "ZE-OBSERVER-FAIL\n", false},
		{"usage page", "usage: le test fixture <name>", false},
		{"empty output", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := proxyMRUFixtureEvidence(tc.output, "9000")
			if (err == nil) != tc.valid {
				t.Fatalf("fixture evidence error=%v, want valid=%t", err, tc.valid)
			}
		})
	}
}

type proxyFixtureLab struct {
	recordingLab
	failAt         string
	transportError bool
}

func (l *proxyFixtureLab) Exec(ctx context.Context, peer string, arguments []string, environment []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if len(arguments) != 6 || arguments[0] != "le" || arguments[1] != "test" || arguments[2] != "fixture" || arguments[3] != proxyMRUFixture || arguments[4] != "1701" || peer != peerZe {
		return l.recordingLab.Exec(ctx, peer, arguments, environment)
	}
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), arguments...)})
	peerMRU := arguments[5]
	if peerMRU == l.failAt {
		if l.transportError {
			return interoplab.CommandResult{}, errors.New("fixture process unavailable")
		}
		return interoplab.CommandResult{ExitCode: 1, Stdout: proxyFixtureTrace(peerMRU)}, nil
	}
	return interoplab.CommandResult{Stdout: proxyFixtureTrace(peerMRU)}, nil
}

func TestProxyMRUCheckerExecutesRegisteredFixture(t *testing.T) {
	for _, tc := range []struct {
		name           string
		failAt         string
		transportError bool
		calls          int
	}{
		{"all peer capacities", "", false, 3},
		{"nonzero fixture exit", "9000", false, 2},
		{"unavailable process", "1500", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lab := &proxyFixtureLab{failAt: tc.failAt, transportError: tc.transportError}
			check := scenarioCheckerMap(time.Second)[scenarioProxyMRU]
			if check == nil {
				t.Fatal("proxy MRU checker is not registered")
			}
			err := check(t.Context(), &interoplab.CheckContext{Lab: lab})
			if (err == nil) != (tc.failAt == "") {
				t.Fatalf("checker error=%v, failAt=%q", err, tc.failAt)
			}
			var executions []recordedCall
			for _, call := range lab.calls {
				if call.operation == "exec" {
					executions = append(executions, call)
				}
			}
			if len(executions) != tc.calls {
				t.Fatalf("fixture calls=%+v, want %d", executions, tc.calls)
			}
			for index, call := range executions {
				peerMRU := []string{"1500", "9000", "0"}[index]
				want := []string{"le", "test", "fixture", proxyMRUFixture, "1701", peerMRU}
				if call.peer != peerZe || !reflect.DeepEqual(call.arguments, want) {
					t.Fatalf("fixture call=%+v, want Ze %v", call, want)
				}
			}
		})
	}
}

func TestProxyMRUScenarioStagesFixtureHelper(t *testing.T) {
	root := l2tpCheckout(t)
	plans, kernelRequired, err := plansAt(root, scenarioProxyMRU, testEnvironment())
	if err != nil {
		t.Fatal(err)
	}
	if !kernelRequired || len(plans) != 1 {
		t.Fatalf("proxy plans=%d kernelRequired=%t", len(plans), kernelRequired)
	}
	prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
		Source: plans[0].Source, Network: (&recordingLab{}).Network(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Peers) != 1 || prepared.Peers[0].Name != peerZe {
		t.Fatalf("proxy peers=%+v, want the Ze kernel namespace", prepared.Peers)
	}
	helper := interoplab.LabBinary{Name: linuxle.Name, Base: linuxle.Base, Output: "test/interop-l2tp/le-linux",
		StartProbe: linuxle.StartProbe()}
	found := false
	for _, binary := range LabBinaries() {
		if reflect.DeepEqual(binary, helper) {
			found = true
		}
	}
	if !found {
		t.Fatal("native L2TP preflight does not stage the static fixture-capable le personality")
	}
	path := filepath.Join(root, "test", "interop-l2tp", "Dockerfile.ze")
	dockerfile, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dockerfile), "COPY "+helper.Output+" /usr/local/bin/le\n") {
		t.Fatal("Ze lab image does not install its declared helper as le")
	}
	if strings.Contains(string(dockerfile), "go build") {
		t.Fatal("fixture helper is compiled inside the container instead of staged on the host")
	}
	if _, err := os.Stat(filepath.Join(plans[0].Source.Directory, "ze.conf")); err != nil {
		t.Fatal(fmt.Errorf("proxy scenario config: %w", err))
	}
}
