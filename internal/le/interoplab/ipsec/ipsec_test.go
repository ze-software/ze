// Design: checkers.go -- typed fixtures prove positive and negative handshake discrimination.
// Related: test/interop-ipsec/parity_test.go -- exact native fixture population.
package ipsec

import (
	"context"
	"encoding/base64"
	"errors"
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

type fakeCheckerLab struct {
	responses map[string][]interoplab.CommandResult
	indexes   map[string]int
	peerLogs  map[string]interoplab.LogResult
}

func (f *fakeCheckerLab) Exec(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	key := fakeCommandKey(peer, argv)
	responses := f.responses[key]
	if len(responses) == 0 {
		return interoplab.CommandResult{}, errors.New("unexpected command: " + key)
	}
	index := f.indexes[key]
	if index >= len(responses) {
		index = len(responses) - 1
	}
	f.indexes[key]++
	result := responses[index]
	if result.ExitCode != 0 {
		return result, errors.New("scripted command failure")
	}
	return result, nil
}

func (f *fakeCheckerLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	return nil
}

func (f *fakeCheckerLab) Query(ctx context.Context, peer string, argv []string, environ []interoplab.EnvironmentVariable) (string, error) {
	result, err := f.Exec(ctx, peer, argv, environ)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", errors.New("query returned no output")
	}
	return result.Stdout, nil
}

func (f *fakeCheckerLab) Logs(_ context.Context, peer string, _ int) (interoplab.LogResult, error) {
	result, ok := f.peerLogs[peer]
	if !ok {
		return interoplab.LogResult{}, errors.New("unexpected log read: " + peer)
	}
	return result, nil
}

func (*fakeCheckerLab) Signal(context.Context, string, string) error { return nil }
func (*fakeCheckerLab) Start(context.Context, string) error          { return nil }
func (*fakeCheckerLab) Stop(context.Context, string, int) error      { return nil }
func (*fakeCheckerLab) Pause(context.Context, string) error          { return nil }
func (*fakeCheckerLab) Unpause(context.Context, string) error        { return nil }
func (*fakeCheckerLab) PeerPID(context.Context, string) (int, error) {
	return 0, errors.New("unexpected peer PID query")
}

func fakeCommandKey(peer string, argv []string) string {
	return peer + "\x00" + strings.Join(argv, "\x00")
}

func scriptedLab() *fakeCheckerLab {
	return &fakeCheckerLab{
		responses: make(map[string][]interoplab.CommandResult),
		indexes:   make(map[string]int),
		peerLogs:  make(map[string]interoplab.LogResult),
	}
}

func (f *fakeCheckerLab) answer(peer string, argv []string, stdout ...string) {
	results := make([]interoplab.CommandResult, 0, len(stdout))
	for _, output := range stdout {
		results = append(results, interoplab.CommandResult{Stdout: output})
	}
	f.responses[fakeCommandKey(peer, argv)] = results
}

func checkerFixture(fake *fakeCheckerLab) *scenarioLab {
	check := &interoplab.CheckContext{
		Source: interoplab.ScenarioSource{Name: "fixture", Directory: "test/interop-ipsec/scenarios/fixture"},
		Lab:    fake,
	}
	return newScenarioLab(check, 5*time.Millisecond, &scenarioState{})
}

func xfrmCounter(spi string, bytes int) string {
	return "src 172.28.0.2 dst 172.28.0.3\n" +
		"\tproto esp spi " + spi + " reqid 1 mode tunnel\n" +
		"\tlifetime current:\n" +
		"\t\t" + strconv.Itoa(bytes) + "(bytes), 1(packets)\n" +
		"\tlifetime config:\n"
}

// VALIDATES: A successful PSK handshake is accepted only after strongSwan's IKE
// and Child SAs, both kernels' XFRM state, and both peers' ESP counters are read.
// PREVENTS: a control-plane-only fixture passing when the peer cannot decrypt ESP.
func TestPSKCheckerRequiresSuccessfulHandshakeAndPeerESP(t *testing.T) {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\n")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, xfrmCounter("0x1", 10), xfrmCounter("0x1", 20))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, xfrmCounter("0x2", 30), xfrmCounter("0x2", 40))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, "4 packets transmitted\n")

	if err := checkPSKSiteToSite(context.Background(), checkerFixture(fake)); err != nil {
		t.Fatalf("successful handshake rejected: %v", err)
	}
}

// VALIDATES: The successful-handshake fixture is discriminating at the peer.
// PREVENTS: a checker that treats Ze's outbound counter as proof of interop.
func TestPSKCheckerRejectsPeerThatAcceptedNoESP(t *testing.T) {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\n")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, xfrmCounter("0x1", 10), xfrmCounter("0x1", 20))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, xfrmCounter("0x2", 30), xfrmCounter("0x2", 30))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, "4 packets transmitted\n")

	err := checkPSKSiteToSite(context.Background(), checkerFixture(fake))
	if err == nil || !strings.Contains(err.Error(), "strongSwan accepted no ESP") {
		t.Fatalf("non-discriminating verdict: %v", err)
	}
}

// VALIDATES: The negative TLS 1.2 handshake passes only after strongSwan proves
// EAP ran and Ze reports every attributed refusal fact with no XFRM SA installed.
// PREVENTS: an absence-only authentication test passing because no exchange ran.
func TestEAPTLSNegativeHandshakeIsProvenAndFailClosed(t *testing.T) {
	fake := scriptedLab()
	fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "negotiated TLS 1.2\nEAP method EAP_TLS succeeded, MSK established\n"}
	fake.peerLogs[zePeer] = interoplab.LogResult{Available: true, Text: strings.Join(eapTLSRefusalFacts[:], "\n")}
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "")
	if err := checkEAPTLS(context.Background(), checkerFixture(fake)); err != nil {
		t.Fatalf("proven negative handshake rejected: %v", err)
	}

	fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "negotiated TLS 1.2\n"}
	if err := checkEAPTLS(context.Background(), checkerFixture(fake)); err == nil {
		t.Fatal("negative handshake passed without proof that EAP succeeded")
	}
}

// VALIDATES: The native responder checker reads strongSwan's TLS 1.3 verdict,
// protected-success acceptance, missing-indication refusal, and both XFRM states.
// PREVENTS: an established IKE SA or a bare EAP-Success standing in for RFC 9190 Section 2.5.
func TestResponderEAPTLS13CheckerDiscriminatesProtectedSuccess(t *testing.T) {
	checker, ok := scenarioCheckers["responder-eap-tls13"]
	if !ok {
		t.Fatal("native integration registry does not execute responder-eap-tls13")
	}
	newLab := func(logs string) *scenarioLab {
		fake := scriptedLab()
		fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
		fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\n")
		fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x2\n")
		fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: logs}
		return checkerFixture(fake)
	}

	good := "negotiated TLS 1.3\nreceived protected success indication via TLS\n"
	if err := checker(context.Background(), newLab(good)); err != nil {
		t.Fatalf("accepted protected success rejected: %v", err)
	}
	if err := checker(context.Background(), newLab("negotiated TLS 1.3\n")); err == nil {
		t.Fatal("missing protected success indication passed")
	}
	if err := checker(context.Background(), newLab(good+"missing protected success indication\n")); err == nil {
		t.Fatal("strongSwan refusal passed")
	}
}

// VALIDATES: Every log-backed checker refuses an empty readable log.
// PREVENTS: an unmeasured empty Docker log satisfying a negative assertion.
func TestEmptyLogsFailClosed(t *testing.T) {
	fake := scriptedLab()
	fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: ""}
	_, err := checkerFixture(fake).logs(context.Background(), swanPeer)
	if err == nil || !strings.Contains(err.Error(), "read no peer output") {
		t.Fatalf("empty logs verdict = %v", err)
	}
}

// VALIDATES: XFRM traffic counters are associated with their own SPI and only
// lifetime-current bytes are counted.
// PREVENTS: rekeyed or configured lifetime bytes being mistaken for traffic.
func TestParseXFRMCountersBySPI(t *testing.T) {
	output := "src a dst b\n\tproto esp spi 0x1\n\tlifetime current:\n\t\t12(bytes), 1(packets)\n\tlifetime config:\n\t\t99(bytes)\n" +
		"src c dst d\n\tproto esp spi 0x2\n\tlifetime current:\n\t\t7(bytes), 1(packets)\n"
	want := map[string]uint64{"0x1": 12, "0x2": 7}
	if got := parseXFRMCounters(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("counters = %v, want %v", got, want)
	}
}

// VALIDATES: PEM rendering preserves the one base64 DER body exactly.
// PREVENTS: encrypted, bundled, or malformed fixture material entering Ze config.
func TestPEMBase64DERFailsClosed(t *testing.T) {
	body, err := pemBase64DER("-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n", "fixture.pem")
	if err != nil || body != "YWJj" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if _, err := pemBase64DER("-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nZA==\n-----END CERTIFICATE-----\n", "bundle.pem"); err == nil {
		t.Fatal("PEM bundle accepted")
	}
}

// VALIDATES: ESP acceptance compares counters only for SPIs that survive the observation window.
// PREVENTS: A normal rekey deletion failing traffic proof or disjoint snapshots passing it.
func TestAssertESPAdvancedUsesSurvivingSPIs(t *testing.T) {
	if err := assertESPAdvanced(
		map[string]uint64{"0xaaaa": 100, "0xbbbb": 900},
		map[string]uint64{"0xaaaa": 101},
		"peer accepted no ESP",
	); err != nil {
		t.Fatalf("surviving SPI advance rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		before map[string]uint64
		after  map[string]uint64
	}{
		{"no counter advanced", map[string]uint64{"0x1": 10}, map[string]uint64{"0x1": 10}},
		{"disjoint SPIs", map[string]uint64{"0x1": 10}, map[string]uint64{"0x2": 11}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := assertESPAdvanced(test.before, test.after, "peer accepted no ESP"); err == nil {
				t.Fatal("non-advancing ESP snapshot accepted")
			}
		})
	}
}

// VALIDATES: PEM conversion skips an EC PARAMETERS prelude and rejects every
// encrypted, empty, bundled, or corrupt representation.
// PREVENTS: Rendering a path, ciphertext, or partial certificate chain as a pki leaf.
func TestPEMBase64DERInputPopulation(t *testing.T) {
	withParameters := "-----BEGIN EC PARAMETERS-----\nYWJj\n-----END EC PARAMETERS-----\n" +
		"-----BEGIN EC PRIVATE KEY-----\nZA==\n-----END EC PRIVATE KEY-----\n"
	if got, err := pemBase64DER(withParameters, "client-key.pem"); err != nil || got != "ZA==" {
		t.Fatalf("EC key body = %q, error = %v", got, err)
	}

	tests := []struct {
		name string
		pem  string
	}{
		{"no PEM block", "/etc/ze/pki/ca.pem"},
		{"empty block", "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----\n"},
		{"RFC 1421 encryption header", "-----BEGIN RSA PRIVATE KEY-----\nProc-Type: 4,ENCRYPTED\nYWJj\n-----END RSA PRIVATE KEY-----\n"},
		{"encrypted PKCS8 label", "-----BEGIN ENCRYPTED PRIVATE KEY-----\nYWJj\n-----END ENCRYPTED PRIVATE KEY-----\n"},
		{"certificate and key bundle", "-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n-----BEGIN PRIVATE KEY-----\nZA==\n-----END PRIVATE KEY-----\n"},
		{"invalid base64", "-----BEGIN CERTIFICATE-----\nnot/base64!\n-----END CERTIFICATE-----\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := pemBase64DER(test.pem, "fixture.pem"); err == nil {
				t.Fatal("invalid PEM accepted")
			}
		})
	}
}

// VALIDATES: PKI placeholders resolve to the original base64 DER body and a
// config without placeholders is unchanged.
// PREVENTS: A missing PKI directory or unreadable fixture becoming a plausible value.
func TestResolvePKIPlaceholdersFailsClosed(t *testing.T) {
	pki := t.TempDir()
	if err := os.WriteFile(filepath.Join(pki, "ca.pem"), []byte(
		"-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}
	input := `certificate "%%PKI_B64:ca.pem%%";`
	if got, err := resolvePKIPlaceholders(input, pki); err != nil || got != `certificate "YWJj";` {
		t.Fatalf("resolved config = %q, error = %v", got, err)
	}
	plain := `certificate "already-inlined";`
	if got, err := resolvePKIPlaceholders(plain, ""); err != nil || got != plain {
		t.Fatalf("plain config = %q, error = %v", got, err)
	}
	if _, err := resolvePKIPlaceholders(input, ""); err == nil {
		t.Fatal("placeholder accepted without a PKI directory")
	}
	if _, err := resolvePKIPlaceholders(`certificate "%%PKI_B64:missing.pem%%";`, pki); err == nil {
		t.Fatal("missing PKI fixture accepted")
	}
}

// VALIDATES: Every rendered Ze config gains the native CLI account and listener
// without modifying the checked-in scenario input.
// PREVENTS: Scenarios without PKI placeholders starting without the CLI used by rekey checks.
func TestRenderZeConfigAlwaysAppendsCLIConfig(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.conf")
	original := "vpn {\n\tipsec {}\n}\n"
	if err := os.WriteFile(source, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "rendered.conf")
	if err := renderZeConfig(source, "", destination); err != nil {
		t.Fatal(err)
	}
	rendered, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimRight(original, "\n") + "\n" + zeCLIConfig()
	if string(rendered) != want {
		t.Fatalf("rendered config differs:\n%s", rendered)
	}
	sourceAfter, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(sourceAfter) != original {
		t.Fatal("rendering modified the checked-in source config")
	}
}

// VALIDATES: XFRM state reads preserve a failed peer command as an error.
// PREVENTS: A failed Docker query becoming an empty, apparently clean snapshot.
func TestXFRMStatePropagatesCommandFailure(t *testing.T) {
	fake := scriptedLab()
	key := fakeCommandKey(swanPeer, []string{"ip", "xfrm", "state"})
	fake.responses[key] = []interoplab.CommandResult{{ExitCode: 1, Stderr: "operation not permitted"}}
	if _, err := checkerFixture(fake).xfrmState(context.Background(), swanPeer); err == nil {
		t.Fatal("failed XFRM query returned no error")
	}
}

// VALIDATES: Clear and responder re-init both refuse an empty baseline SPI snapshot.
// PREVENTS: The already-installed SA satisfying the later new-SPI wait.
func TestReestablishCheckersRefuseEmptyBaseline(t *testing.T) {
	for _, test := range []struct {
		name  string
		check scenarioChecker
		want  string
	}{
		{"clear", checkClearReestablish, "empty ESP SPI snapshot before clear"},
		{"responder re-init", checkResponderAcceptsReinit, "empty ESP SPI snapshot before re-init"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := scriptedLab()
			fake.answer(swanPeer, []string{"swanctl", "--list-sas"},
				"ze: ESTABLISHED\nze-child: INSTALLED\n",
				"ze: ESTABLISHED\nze-child: INSTALLED\n")
			fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "")
			err := test.check(context.Background(), checkerFixture(fake))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("empty baseline error = %v, want %q", err, test.want)
			}
		})
	}
}

// VALIDATES: The valid fixture body remains decodable after conversion.
// PREVENTS: Validation changing the exact text that is written into configuration.
func TestPEMBase64DEROutputIsDecodable(t *testing.T) {
	body, err := pemBase64DER("-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----\n", "fixture.pem")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(body)
	if err != nil || string(decoded) != "abc" {
		t.Fatalf("decoded body = %q, error = %v", decoded, err)
	}
}

// VALIDATES: All 20 scenarios retain the isolated /24, fixed peer addresses,
// image roles, config mounts, and Ze command consumed by the native suite.
// PREVENTS: A complete checker registry running against a weakened or missing peer topology.
func TestScenarioPlansPreserveTopologyAndInputs(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	environment := interoplab.Environment{
		Image:          defaultFRRImage,
		SessionTimeout: 90 * time.Second,
		Suffix:         "plan-test",
	}
	sources, err := interoplab.Discover(
		filepath.Join(root, "test", "interop-ipsec", "scenarios"),
		"",
		checkerAdapters(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != len(scenarioCheckers) {
		t.Fatalf("plans = %d, checkers = %d", len(sources), len(scenarioCheckers))
	}
	network := interoplab.Network{Name: "ze-ipsec-plan-test", IPv4: networkPrefix}
	for _, source := range sources {
		state := &scenarioState{}
		plan := scenarioPlan(root, environment, source, state)
		if len(plan.Network.Candidates) != 1 ||
			plan.Network.Candidates[0].IPv4 != networkPrefix {
			t.Errorf("%s network = %#v", source.Name, plan.Network)
		}
		wantContainers := []string{
			"ze-ipsec-ze-plan-test",
			"ze-ipsec-swan-plan-test",
			"ze-ipsec-frr-plan-test",
		}
		if !reflect.DeepEqual(plan.Containers, wantContainers) {
			t.Errorf("%s cleanup containers = %v", source.Name, plan.Containers)
		}
		prepared, err := plan.Prepare(t.Context(), interoplab.PrepareContext{
			Source:  source,
			Network: network,
		})
		if err != nil {
			t.Fatalf("prepare %s: %v", source.Name, err)
		}
		if prepared.Cleanup != nil {
			if err := prepared.Cleanup(); err != nil {
				t.Errorf("cleanup %s: %v", source.Name, err)
			}
		}
		ze := ipsecPeerByName(t, prepared.Peers, zePeer)
		if ze.Host != 2 || ze.Image != zePeer ||
			!reflect.DeepEqual(ze.Command, []string{"start", "/etc/ze/ze.conf"}) {
			t.Errorf("%s Ze peer = %#v", source.Name, ze)
		}
		swan := ipsecPeerByName(t, prepared.Peers, swanPeer)
		if swan.Host != 3 || swan.Image != swanPeer ||
			!reflect.DeepEqual(swan.Arguments, []string{"--privileged"}) {
			t.Errorf("%s strongSwan peer = %#v", source.Name, swan)
		}
		hasFRR := fileExists(filepath.Join(source.Directory, "frr.conf"))
		if hasFRR {
			frr := ipsecPeerByName(t, prepared.Peers, frrPeer)
			if frr.Host != 4 || frr.Image != frrPeer {
				t.Errorf("%s FRR peer = %#v", source.Name, frr)
			}
		} else if ipsecHasPeer(prepared.Peers, frrPeer) {
			t.Errorf("%s unexpectedly starts FRR", source.Name)
		}
	}
}

func ipsecPeerByName(t *testing.T, peers []interoplab.PeerConfig, name string) interoplab.PeerConfig {
	t.Helper()
	for _, peer := range peers {
		if peer.Name == name {
			return peer
		}
	}
	t.Fatalf("peer %s missing from %#v", name, peers)
	return interoplab.PeerConfig{}
}

func ipsecHasPeer(peers []interoplab.PeerConfig, name string) bool {
	for _, peer := range peers {
		if peer.Name == name {
			return true
		}
	}
	return false
}
