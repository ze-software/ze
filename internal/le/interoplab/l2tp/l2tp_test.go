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
	lepath "github.com/ze-software/ze/internal/le/le/path"
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
	case command == strings.Join(lacStopCommand, " "):
		l.stopped = true
		return interoplab.CommandResult{}, nil
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
	case command == "cat "+subscriberResolvConf:
		return interoplab.CommandResult{Stdout: "nameserver 10.100.0.1\nnameserver 10.100.0.1\n"}, nil
	case command == "ip route get 10.100.0.1":
		return interoplab.CommandResult{Stdout: "10.100.0.1 dev ppp0 src 10.100.0.2 uid 0\n"}, nil
	case command == "nslookup -type=a www.lab.example 10.100.0.1":
		return interoplab.CommandResult{Stdout: busyboxAnswer}, nil
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

// VALIDATES: the L2TP listener in every Ze config the lab mounts can reach the
// other containers: its `environment l2tp server ip` is the wildcard or an
// address on the lab bridge, never a loopback address.
// PREVENTS: the 2026-10-17 nightly failure of 03-ze-lac-xl2tpd-lns, whose
// listener stayed on 127.0.0.1 after the lab left host networking. Ze sends
// every tunnel from its listener socket, so the kernel refused each SCCRQ to
// 172.29.0.3 ("sendto: invalid argument") and the tunnel never established.
func TestZeL2TPListenerReachesTheLabBridge(t *testing.T) {
	root := l2tpCheckout(t)
	plans, _, err := plansAt(root, "", testEnvironment())
	if err != nil {
		t.Fatalf("build plans: %v", err)
	}
	network := interoplab.Network{Name: "n", IPv4: netip.MustParsePrefix("172.29.0.0/24")}
	for _, plan := range plans {
		value, err := plan.Prepare(t.Context(), interoplab.PrepareContext{Source: plan.Source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", plan.Source.Name, err)
		}
		if value.Cleanup != nil {
			t.Cleanup(func() {
				if err := value.Cleanup(); err != nil {
					t.Errorf("cleanup rendered config: %v", err)
				}
			})
		}
		config, err := os.ReadFile(mountSource(t, peerByName(t, value.Peers, peerZe), "/etc/ze/ze.conf"))
		if err != nil {
			t.Fatalf("read %s ze config: %v", plan.Source.Name, err)
		}
		listener, err := l2tpListenerAddress(string(config))
		if err != nil {
			t.Fatalf("%s: %v", plan.Source.Name, err)
		}
		if listener.IsUnspecified() {
			continue
		}
		if !network.IPv4.Contains(listener) {
			t.Errorf("%s binds the L2TP listener to %s, which is neither 0.0.0.0 nor on the lab bridge %s, so Ze cannot send to the other containers",
				plan.Source.Name, listener, network.IPv4)
		}
	}
}

// l2tpListenerAddress returns the `ip` of the first server under
// `environment { l2tp { ... } }` in a Ze config. It matches the four tokens in
// order rather than parsing blocks, which the four fixtures' formats (with or
// without a trailing semicolon) all satisfy. A layout it misreads can only
// yield an error or a wrong address, so its failure mode is a false red.
func l2tpListenerAddress(config string) (netip.Addr, error) {
	tokens := strings.Fields(config)
	path := []string{"environment", "l2tp", "server", "ip"}
	next := 0
	for index, token := range tokens {
		if token != path[next] {
			continue
		}
		next++
		if next < len(path) {
			continue
		}
		if index+1 >= len(tokens) {
			break
		}
		return netip.ParseAddr(strings.TrimSuffix(tokens[index+1], ";"))
	}
	return netip.Addr{}, errors.New("config has no environment l2tp server ip")
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
				{operation: "exec", peer: peerLAC, arguments: lacStopCommand},
			},
		},
		{
			name:      scenarioBGPRedistribute,
			wantPeers: []string{peerZe, peerLAC, peerFRR},
			wantCalls: []recordedCall{
				{operation: "exec", peer: peerFRR, arguments: []string{commandVTYSH, "-c", commandShow + " bgp ipv4 unicast 10.100.0.2/32 json"}},
				{operation: "exec", peer: peerLAC, arguments: lacStopCommand},
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
		{
			name:      scenarioDNSResolver,
			wantPeers: []string{peerZe, peerLAC},
			wantCalls: []recordedCall{
				{operation: "exec", peer: peerLAC, arguments: []string{"cat", subscriberResolvConf}},
				{operation: "exec", peer: peerLAC, arguments: []string{"ip", "route", "get", localPPPAddress}},
				{operation: "exec", peer: peerLAC, arguments: []string{"nslookup", "-type=a", subscriberQueryName, localPPPAddress}},
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
			err := preflight()(t.Context(), nil)
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

// TestPeerPreparationFailsClosed exercises errors before any participant starts.
// Invalid peer addresses and unreadable/ambiguous input must not yield peers.
func TestPeerPreparationFailsClosed(t *testing.T) {
	const validConfig = "\t\taddress 127.0.0.1\n"
	for _, test := range []struct {
		name     string
		scenario string
		config   string
		network  string
		want     string
	}{
		{"missing-config", scenarioInitiator, "", "172.29.0.0/24", "read initiator config"},
		{"missing-address", scenarioInitiator, "address absent\n", "172.29.0.0/24", "exactly one xl2tpd address"},
		{"duplicate-address", scenarioInitiator, validConfig + validConfig, "172.29.0.0/24", "exactly one xl2tpd address"},
		{"ipv6-peer", scenarioInitiator, validConfig, "fd00::/64", "requires an IPv4 /24"},
		{"wrong-prefix", scenarioInitiator, validConfig, "172.29.0.0/25", "requires an IPv4 /24"},
		{"ipv6-relay", scenarioUnknownMandatoryAVP, "", "fd00::/64", "requires an IPv4 /24"},
		{"unsupported-scenario", "unsupported", "", "172.29.0.0/24", "unsupported L2TP scenario"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.config != "" {
				if err := os.WriteFile(filepath.Join(root, "ze.conf"), []byte(test.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			source := interoplab.ScenarioSource{Name: test.scenario, Directory: root}
			plan := scenarioPlan(root, source, testEnvironment())
			prepared, err := plan.Prepare(t.Context(), interoplab.PrepareContext{
				Source: source, Network: interoplab.Network{IPv4: netip.MustParsePrefix(test.network)},
			})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("prepare error = %v, want %q", err, test.want)
			}
			if len(prepared.Peers) != 0 {
				t.Fatalf("failed preparation returned runnable peers: %+v", prepared.Peers)
			}
		})
	}
}

// TestInitiatorPreparationRendersSelectedPeer observes address substitution and
// cleanup using two allocated networks, not a checked-in configuration snapshot.
func TestInitiatorPreparationRendersSelectedPeer(t *testing.T) {
	root := t.TempDir()
	const config = "remote {\n\t\taddress 127.0.0.1\n}\n"
	if err := os.WriteFile(filepath.Join(root, "ze.conf"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, network := range []string{"172.29.0.0/24", "192.0.2.0/24"} {
		prefix := netip.MustParsePrefix(network)
		source := interoplab.ScenarioSource{Name: scenarioInitiator, Directory: root}
		plan := scenarioPlan(root, source, testEnvironment())
		prepared, err := plan.Prepare(t.Context(), interoplab.PrepareContext{
			Source: source, Network: interoplab.Network{IPv4: prefix},
		})
		if err != nil {
			t.Fatal(err)
		}
		if prepared.Cleanup == nil {
			t.Fatal("rendered configuration has no cleanup")
		}
		t.Cleanup(func() {
			if err := prepared.Cleanup(); err != nil {
				t.Error(err)
			}
		})
		path := mountSource(t, peerByName(t, prepared.Peers, peerZe), "/etc/ze/ze.conf")
		relative, err := filepath.Rel(root, path)
		if err != nil || !filepath.IsLocal(relative) {
			t.Fatalf("rendered path %q is not Docker-shared: %v", path, err)
		}
		rendered, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		peer := prefix.Addr().Next().Next().Next()
		want := strings.Replace(config, "127.0.0.1", peer.String(), 1)
		if string(rendered) != want {
			t.Fatalf("rendered peer config = %q, want %q", rendered, want)
		}
		if err := prepared.Cleanup(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rendered configuration survived cleanup: %v", err)
		}
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
	return interoplab.Environment{SessionTimeout: 90 * time.Second, Suffix: "test", Image: interoplab.FRRImage}
}
