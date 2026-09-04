package l2tp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/le/lepath"
)

type recordedCall struct {
	operation string
	peer      string
	arguments []string
}

type recordingLab struct {
	calls   []recordedCall
	stopped bool
}

func (l *recordingLab) Exec(_ context.Context, peer string, arguments []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "exec", peer: peer, arguments: append([]string(nil), arguments...)})
	command := strings.Join(arguments, " ")
	switch {
	case command == "ip -o link show type ppp":
		if l.stopped {
			return interoplab.CommandResult{}, nil
		}
		return interoplab.CommandResult{Stdout: "7: ppp0: <POINTOPOINT,UP> mtu 1500\n"}, nil
	case command == "ip -o addr show dev ppp0":
		return interoplab.CommandResult{Stdout: "7: ppp0 inet 10.100.0.1 peer 10.100.0.2/32 scope global ppp0\n"}, nil
	case command == "ip l2tp show tunnel":
		if l.stopped {
			return interoplab.CommandResult{}, nil
		}
		return interoplab.CommandResult{Stdout: "Tunnel 1, encap UDP\n"}, nil
	case strings.HasPrefix(command, "ping -c 3 -W 3 10.100.0.1"):
		return interoplab.CommandResult{Stdout: "3 packets transmitted, 3 received\n"}, nil
	case strings.HasPrefix(command, "vtysh -c show bgp neighbor"):
		return interoplab.CommandResult{Stdout: "BGP state = Established\n"}, nil
	case strings.Contains(command, "show bgp ipv4 unicast 10.100.0.2/32 json"):
		if l.stopped {
			return interoplab.CommandResult{Stdout: "{}\n"}, nil
		}
		return interoplab.CommandResult{Stdout: `{"prefix":"10.100.0.2/32","paths":[{}]}` + "\n"}, nil
	case command == "wget -qO- --header=Authorization: Bearer secret --header=Content-Type: application/json --post-data={\"command\":\"request l2tp outgoing-call remote xl2tpd called 12345\"} http://127.0.0.1:17012/api/v1/execute":
		return interoplab.CommandResult{Stdout: `{"error":"peer cannot answer OCRQ"}` + "\n"}, nil
	case strings.Contains(command, `{"command":"clear l2tp session all"}`):
		return interoplab.CommandResult{Stdout: `{"result":"1 session cleared"}` + "\n"}, nil
	default:
		return interoplab.CommandResult{}, errors.New("unexpected exec: " + peer + " " + command)
	}
}

func (l *recordingLab) ExecDetached(_ context.Context, peer string, arguments []string, _ []interoplab.EnvironmentVariable) error {
	l.calls = append(l.calls, recordedCall{operation: "exec-detached", peer: peer, arguments: append([]string(nil), arguments...)})
	return nil
}

func (l *recordingLab) Query(ctx context.Context, peer string, arguments []string, environment []interoplab.EnvironmentVariable) (string, error) {
	result, err := l.Exec(ctx, peer, arguments, environment)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", errors.New("recorded query returned no output")
	}
	return result.Stdout, nil
}

func (l *recordingLab) Logs(_ context.Context, peer string, _ int) (interoplab.LogResult, error) {
	l.calls = append(l.calls, recordedCall{operation: "logs", peer: peer})
	var text string
	switch peer {
	case peerZe:
		text = strings.Join([]string{
			"L2TP listener bound",
			"session established",
			"PPP session up",
			"session IP assigned",
			"subscriber route inject",
			"subscriber routes withdrawn",
			"tunnel now established (initiator)",
		}, "\n")
	case peerLAC:
		text = "xl2tpd: Listening on IP address 0.0.0.0\nConnection established\nOutgoing-Call-Request\n"
	case peerRadius:
		// Event-Timestamp is read against the clock, so the fake stamps NOW.
		// A constant here would age into a failure. The records carry no
		// Calling-Station-Id because xl2tpd sends no Calling Number AVP, and
		// only the Stop names a cause: the checker asserts both absences.
		stamp := "Event-Timestamp=" + strconv.FormatInt(time.Now().Unix(), 10)
		text = strings.Join([]string{
			"radius-mock listening on 0.0.0.0:1812",
			"RADIUS-RX Access-Request User-Name=alice NAS-Port-Id=lns1:12.34",
			"RADIUS-RX Accounting-Request Acct-Status-Type=Start Framed-IP-Address=10.100.0.2 " +
				"NAS-Port-Id=lns1:12.34 " + stamp + " Acct-Delay-Time=0",
			"RADIUS-RX Accounting-Request Acct-Status-Type=Stop Framed-IP-Address=10.100.0.2 " +
				"NAS-Port-Id=lns1:12.34 " + stamp + " Acct-Delay-Time=0 Acct-Terminate-Cause=6",
		}, "\n")
	default:
		return interoplab.LogResult{}, errors.New("unexpected logs peer: " + peer)
	}
	return interoplab.LogResult{Text: text, Available: true}, nil
}

func (l *recordingLab) Signal(_ context.Context, peer, signal string) error {
	l.calls = append(l.calls, recordedCall{operation: "signal " + signal, peer: peer})
	l.stopped = true
	return nil
}

func (l *recordingLab) PeerPID(_ context.Context, peer string) (int, error) {
	l.calls = append(l.calls, recordedCall{operation: "peer-pid", peer: peer})
	return 0, errors.New("unexpected peer PID query")
}

func (l *recordingLab) Pause(_ context.Context, peer string) error {
	l.calls = append(l.calls, recordedCall{operation: "pause", peer: peer})
	return nil
}

func (l *recordingLab) Unpause(_ context.Context, peer string) error {
	l.calls = append(l.calls, recordedCall{operation: "unpause", peer: peer})
	return nil
}

func (l *recordingLab) Start(_ context.Context, peer string) error {
	l.calls = append(l.calls, recordedCall{operation: "start", peer: peer})
	return nil
}

func (l *recordingLab) Stop(_ context.Context, peer string, code int) error {
	l.calls = append(l.calls, recordedCall{operation: "stop", peer: peer, arguments: []string{fmt.Sprint(code)}})
	return nil
}

func (l *recordingLab) Network() interoplab.Network {
	return interoplab.Network{Name: "ze-l2tp-test", IPv4: netip.MustParsePrefix("172.29.0.0/24")}
}

func l2tpCheckout(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	return root
}

// VALIDATES: every reviewed scenario has a typed callback in lexical order.
// PREVENTS: an inverse-role or three-peer branch disappearing from the native gate.
func TestPlansCoverEveryScenario(t *testing.T) {
	root := l2tpCheckout(t)
	plans, _, err := plansAt(root, "", testEnvironment())
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	got := make([]string, 0, len(plans))
	for _, plan := range plans {
		got = append(got, plan.Source.Name)
		if plan.Source.Checker == nil {
			t.Errorf("scenario %s has no checker", plan.Source.Name)
		}
	}
	want := []string{scenarioPPPIPv4, scenarioBGPRedistribute, scenarioInitiator, scenarioRadiusAttrs}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenario order mismatch: got %v, want %v", got, want)
	}
}

// VALIDATES: peer roles, image commands, configuration mounts, and privileged isolation match the native lifecycle.
// PREVENTS: a typed checker running without its independent xl2tpd, FRR, or RADIUS participant.
func TestPlanPeerCommandsAndConfigBytes(t *testing.T) {
	root := l2tpCheckout(t)
	plans, _, err := plansAt(root, "", testEnvironment())
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	wantPeers := [][]string{
		{peerZe, peerLAC},
		{peerZe, peerFRR, peerLAC},
		{peerLAC, peerZe},
		{peerRadius, peerZe, peerLAC},
	}
	network := interoplab.Network{Name: "n", IPv4: netip.MustParsePrefix("172.29.0.0/24")}
	prepared := make([]interoplab.PreparedScenario, len(plans))
	for index, plan := range plans {
		if len(plan.Containers) == 0 {
			t.Errorf("%s declares no pre-clean containers", plan.Source.Name)
		}
		if plan.Prepare == nil {
			t.Fatalf("%s has no typed preparer", plan.Source.Name)
		}
		value, err := plan.Prepare(t.Context(), interoplab.PrepareContext{Source: plan.Source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", plan.Source.Name, err)
		}
		prepared[index] = value
		if value.Cleanup != nil {
			cleanup := value.Cleanup
			t.Cleanup(func() {
				if err := cleanup(); err != nil {
					t.Errorf("cleanup rendered config: %v", err)
				}
			})
		}
		if got := peerNames(value.Peers); !reflect.DeepEqual(got, wantPeers[index]) {
			t.Errorf("%s participants = %v, want %v", plan.Source.Name, got, wantPeers[index])
		}
	}

	ze := peerByName(t, prepared[0].Peers, peerZe)
	if !reflect.DeepEqual(ze.Command, []string{"start", "/etc/ze/ze.conf"}) {
		t.Errorf("ze command = %v", ze.Command)
	}
	scenario01 := filepath.Join(root, "test", "interop-l2tp", "scenarios", scenarioPPPIPv4)
	assertMount(t, ze, filepath.Join(scenario01, "ze.conf"), "/etc/ze/ze.conf")
	lac := peerByName(t, prepared[0].Peers, peerLAC)
	assertMount(t, lac, filepath.Join(scenario01, "xl2tpd.conf"), "/etc/xl2tpd/xl2tpd.conf")
	assertMount(t, lac, filepath.Join(scenario01, "ppp-options"), "/etc/ppp/options.l2tpd.client")
	assertMount(t, lac, filepath.Join(scenario01, "l2tp-secrets"), "/etc/xl2tpd/l2tp-secrets")

	scenario02 := filepath.Join(root, "test", "interop-l2tp", "scenarios", scenarioBGPRedistribute)
	frr := peerByName(t, prepared[1].Peers, peerFRR)
	assertMount(t, frr, filepath.Join(scenario02, "frr.conf"), "/etc/frr/frr.conf")

	scenario03 := filepath.Join(root, "test", "interop-l2tp", "scenarios", scenarioInitiator)
	initiator := peerByName(t, prepared[2].Peers, peerZe)
	renderedPath := mountSource(t, initiator, "/etc/ze/ze.conf")
	rendered, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatalf("read rendered initiator config: %v", err)
	}
	fixture, err := os.ReadFile(filepath.Join(scenario03, "ze.conf"))
	if err != nil {
		t.Fatalf("read initiator config: %v", err)
	}
	wantRendered := strings.Replace(string(fixture), "\t\taddress 127.0.0.1", "\t\taddress 172.29.0.3", 1)
	if string(rendered) != wantRendered {
		t.Errorf("rendered initiator config bytes differ from the single peer-address substitution")
	}
	assertMount(t, peerByName(t, prepared[2].Peers, peerLAC), filepath.Join(scenario03, "xl2tpd.conf"), "/etc/xl2tpd/xl2tpd.conf")

	scenario04 := filepath.Join(root, "test", "interop-l2tp", "scenarios", scenarioRadiusAttrs)
	assertMount(t, peerByName(t, prepared[3].Peers, peerZe), filepath.Join(scenario04, "ze.conf"), "/etc/ze/ze.conf")
}

// VALIDATES: each typed checker reads positive state from every required peer and preserves teardown ordering.
// PREVENTS: a checker passing without an xl2tpd/FRR/RADIUS response or checking absence before the mechanism ran.
func TestCheckersRecordPeerParticipation(t *testing.T) {
	tests := []struct {
		name      string
		wantPeers []string
		wantCalls []recordedCall
	}{
		{
			name: scenarioPPPIPv4, wantPeers: []string{peerZe, peerLAC},
			wantCalls: []recordedCall{
				{operation: "exec", peer: peerLAC, arguments: []string{"ping", "-c", "3", "-W", "3", localPPPAddress}},
				{operation: "signal TERM", peer: peerLAC},
			},
		},
		{
			name:      scenarioBGPRedistribute,
			wantPeers: []string{peerZe, peerLAC, peerFRR},
			wantCalls: []recordedCall{
				{operation: "exec", peer: peerFRR, arguments: []string{commandVTYSH, "-c", commandShow + " bgp ipv4 unicast 10.100.0.2/32 json"}},
				{operation: "signal TERM", peer: peerLAC},
			},
		},
		{
			name: scenarioInitiator, wantPeers: []string{peerZe, peerLAC},
			wantCalls: []recordedCall{{
				operation: "exec", peer: peerZe,
				arguments: []string{
					"wget", "-qO-", "--header=Authorization: Bearer secret",
					"--header=Content-Type: application/json",
					"--post-data={\"command\":\"request l2tp outgoing-call remote xl2tpd called 12345\"}",
					"http://127.0.0.1:17012/api/v1/execute",
				},
			}},
		},
		{
			name:      scenarioRadiusAttrs,
			wantPeers: []string{peerZe, peerLAC, peerRadius},
			wantCalls: []recordedCall{
				{operation: "logs", peer: peerRadius},
				{
					operation: "exec", peer: peerZe,
					arguments: []string{
						"wget", "-qO-", "--header=Authorization: Bearer secret",
						"--header=Content-Type: application/json",
						"--post-data={\"command\":\"clear l2tp session all\"}",
						"http://127.0.0.1:17012/api/v1/execute",
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lab := &recordingLab{}
			check := scenarioCheckerMap(90 * time.Second)[test.name]
			state := &interoplab.CheckContext{Lab: lab}
			if err := check(t.Context(), state); err != nil {
				t.Fatalf("checker failed: %v", err)
			}
			seen := map[string]bool{}
			for _, call := range lab.calls {
				seen[call.peer] = true
			}
			for _, peer := range test.wantPeers {
				if !seen[peer] {
					t.Errorf("checker never read peer %s; calls: %#v", peer, lab.calls)
				}
			}
			for _, want := range test.wantCalls {
				if !containsCall(lab.calls, want) {
					t.Errorf("checker missing exact call %#v; calls: %#v", want, lab.calls)
				}
			}
		})
	}
}

// VALIDATES: both legacy kernel-probe bypass variables are refused before any Docker work.
// PREVENTS: the native gate reporting PPP coverage after deliberately skipping its host prerequisite.
func TestPreflightRefusesKernelProbeBypass(t *testing.T) {
	keys := []string{"ZE_L2TP_SKIP_KERNEL_PROBE", "ze.l2tp.skip-kernel-probe"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			restoreEnvironment(t, keys)
			if err := os.Setenv(key, "true"); err != nil {
				t.Fatalf("set %s: %v", key, err)
			}
			err := preflight("test")(t.Context(), nil)
			if err == nil || !strings.Contains(err.Error(), "refusing to run with "+key+" set") {
				t.Fatalf("preflight error = %v", err)
			}
		})
	}
}

// VALIDATES: an unknown selector and an unavailable core return the deployment gate's exact non-zero code.
// PREVENTS: native discovery or setup failures being reported as an empty successful run.
func TestFailureCodeIsOne(t *testing.T) {
	root := l2tpCheckout(t)
	if report := runAt(t.Context(), root, "does-not-exist", nil, testEnvironment()); report.Code != 1 {
		t.Fatalf("unknown selector code = %d, want 1", report.Code)
	}
	if report := runAt(t.Context(), root, scenarioPPPIPv4, nil, testEnvironment()); report.Code != 1 {
		t.Fatalf("unavailable Docker code = %d, want 1", report.Code)
	}
}

// VALIDATES: The deployment action can select one scenario through the native environment contract.
// PREVENTS: An empty action argument silently running the full population.
func TestRunAtReadsNativeScenarioSelector(t *testing.T) {
	t.Setenv("ZE_L2TP_INTEROP_SCENARIO", "does-not-exist")
	report := RunAt(t.Context(), l2tpCheckout(t), "")
	if report.Code != 1 || !strings.Contains(report.SetupError, "does-not-exist") {
		t.Fatalf("environment-selected missing scenario report = %#v", report)
	}
}

func restoreEnvironment(t *testing.T, keys []string) {
	t.Helper()
	type value struct {
		text   string
		exists bool
	}
	original := make(map[string]value, len(keys))
	for _, key := range keys {
		text, exists := os.LookupEnv(key)
		original[key] = value{text: text, exists: exists}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		for _, key := range keys {
			saved := original[key]
			if saved.exists {
				if err := os.Setenv(key, saved.text); err != nil {
					t.Errorf("restore %s: %v", key, err)
				}
				continue
			}
			if err := os.Unsetenv(key); err != nil {
				t.Errorf("clear %s: %v", key, err)
			}
		}
	})
}

func mountSource(t *testing.T, peer interoplab.PeerConfig, target string) string {
	t.Helper()
	for _, mount := range peer.Mounts {
		if mount.Target == target && mount.ReadOnly {
			return mount.Source
		}
	}
	t.Fatalf("peer %s missing read-only target %s", peer.Name, target)
	return ""
}

func peerByName(t *testing.T, peers []interoplab.PeerConfig, name string) interoplab.PeerConfig {
	t.Helper()
	for index := range peers {
		if peers[index].Name == name {
			return peers[index]
		}
	}
	t.Fatalf("peer %s not found in %#v", name, peers)
	return interoplab.PeerConfig{}
}

func peerNames(peers []interoplab.PeerConfig) []string {
	names := make([]string, 0, len(peers))
	for index := range peers {
		names = append(names, peers[index].Name)
	}
	return names
}

func assertMount(t *testing.T, peer interoplab.PeerConfig, source, target string) {
	t.Helper()
	for _, mount := range peer.Mounts {
		if mount.Source == source && mount.Target == target && mount.ReadOnly {
			return
		}
	}
	t.Errorf("peer %s missing read-only mount %s:%s; mounts: %#v", peer.Name, source, target, peer.Mounts)
}

func containsCall(calls []recordedCall, want recordedCall) bool {
	for _, call := range calls {
		if call.operation == want.operation && call.peer == want.peer &&
			reflect.DeepEqual(call.arguments, want.arguments) {
			return true
		}
	}
	return false
}

func testEnvironment() interoplab.Environment {
	return interoplab.Environment{SessionTimeout: 90 * time.Second, Suffix: "test", Image: "quay.io/frrouting/frr:10.3.1"}
}
