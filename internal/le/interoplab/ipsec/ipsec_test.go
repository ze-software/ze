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

// VALIDATES: A successful PSK handshake is accepted only after strongSwan's IKE
// and Child SAs, both kernels' XFRM state, and both peers' ESP counters are read.
// PREVENTS: a control-plane-only fixture passing when the peer cannot decrypt ESP.
func TestPSKCheckerRequiresSuccessfulHandshakeAndPeerESP(t *testing.T) {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\n")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))

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
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(10, 600))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))

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

// VALIDATES: The Nak scenario passes only when strongSwan's own log records both
// the method it offered and the Nak it parsed back, with no XFRM SA at either end.
// PREVENTS: an exchange that never reached EAP passing on an empty kernel, and a
// checker crediting ze with a Nak strongSwan never decoded.
func TestEAPNakCheckerRequiresStrongSwanToParseTheNak(t *testing.T) {
	checker, ok := scenarioCheckers["eap-nak-method-negotiation"]
	if !ok {
		t.Fatal("native integration registry does not execute eap-nak-method-negotiation")
	}
	newLab := func(logs, zeXFRM string) *scenarioLab {
		fake := scriptedLab()
		fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: logs}
		fake.answer(zePeer, []string{"ip", "xfrm", "state"}, zeXFRM)
		fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "")
		return checkerFixture(fake)
	}

	good := "initiating EAP_MD5 method (id 0x00)\nparsed IKE_AUTH request 2 [ EAP/RES/NAK ]\n"
	if err := checker(context.Background(), newLab(good, "")); err != nil {
		t.Fatalf("proven Nak refusal rejected: %v", err)
	}
	if err := checker(context.Background(), newLab("initiating EAP_MD5 method (id 0x00)\n", "")); err == nil {
		t.Fatal("refusal passed without strongSwan parsing the Nak")
	}
	if err := checker(context.Background(), newLab("parsed IKE_AUTH request 2 [ EAP/RES/NAK ]\n", "")); err == nil {
		t.Fatal("refusal passed without strongSwan offering a method")
	}
	if err := checker(context.Background(), newLab(good, "proto esp spi 0x1\n")); err == nil {
		t.Fatal("refusal passed with an XFRM SA installed at ze")
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

// VALIDATES: Two SAs that share one SPI in opposite directions stay distinct,
// and only lifetime-current bytes are counted.
// PREVENTS: rekeyed or configured lifetime bytes being mistaken for traffic, and
// the sender's outbound SA folding into the receiver's inbound SA, which carries
// the same SPI because the receiver chose it (RFC 4301 Section 4.1).
func TestParseXFRMCountersKeepsDirection(t *testing.T) {
	output := "src a dst b\n\tproto esp spi 0x1\n\tlifetime current:\n\t\t12(bytes), 1(packets)\n\tlifetime config:\n\t\t99(bytes)\n" +
		"src b dst a\n\tproto esp spi 0x1\n\tlifetime current:\n\t\t7(bytes), 1(packets)\n"
	want := map[saKey]uint64{
		{source: "a", target: "b", spi: "0x1"}: 12,
		{source: "b", target: "a", spi: "0x1"}: 7,
	}
	got, err := parseXFRMCounters(output)
	if err != nil {
		t.Fatalf("direction-correct dump refused: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("counters = %v, want %v", got, want)
	}
}

// VALIDATES: An SA record printed under no src/dst header is refused.
// PREVENTS: a zero-valued direction that compares equal to a real one.
func TestParseXFRMCountersRefusesADumpWithNoDirection(t *testing.T) {
	if _, err := parseXFRMCounters("\tproto esp spi 0x1\n\tlifetime current:\n\t\t12(bytes), 1(packets)\n"); err == nil {
		t.Fatal("headerless SA record accepted with no direction")
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

// VALIDATES: ESP acceptance compares counters only for SPIs that survive the
// observation window, and reads only the SAs the claimed direction names.
// PREVENTS: A normal rekey deletion failing traffic proof, disjoint snapshots
// passing it, and the reverse direction's SA satisfying the claim.
func TestAssertESPAdvancedUsesSurvivingSPIs(t *testing.T) {
	out := saKey{source: zeIP, target: swanIP, spi: "0xaaaa"}
	retired := saKey{source: zeIP, target: swanIP, spi: "0xbbbb"}
	in := saKey{source: swanIP, target: zeIP, spi: "0xaaaa"}
	if err := assertESPAdvanced(
		map[saKey]uint64{out: 100, retired: 900},
		map[saKey]uint64{out: 101},
		zeEncrypts,
	); err != nil {
		t.Fatalf("surviving SPI advance rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		before map[saKey]uint64
		after  map[saKey]uint64
	}{
		{"no counter advanced", map[saKey]uint64{out: 10}, map[saKey]uint64{out: 10}},
		{"disjoint SPIs", map[saKey]uint64{out: 10}, map[saKey]uint64{retired: 11}},
		{"only the reverse direction advanced", map[saKey]uint64{out: 10, in: 10}, map[saKey]uint64{out: 10, in: 99}},
		{"no SA in this direction at all", map[saKey]uint64{in: 10}, map[saKey]uint64{in: 99}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := assertESPAdvanced(test.before, test.after, zeEncrypts); err == nil {
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
	for index := range peers {
		if peers[index].Name == name {
			return peers[index]
		}
	}
	t.Fatalf("peer %s missing from %#v", name, peers)
	return interoplab.PeerConfig{}
}

func ipsecHasPeer(peers []interoplab.PeerConfig, name string) bool {
	for index := range peers {
		if peers[index].Name == name {
			return true
		}
	}
	return false
}

// xfrmSA renders one simplex SA record in the shape `ip -s xfrm state` prints,
// with the `src <addr> dst <addr>` header that names the direction it carries.
//
// RFC 4301 Section 4.1: "An SA is a simplex "connection" that affords security
// services to the traffic carried by it." A peer dump therefore holds one record
// per direction, and both peers name one direction by the SAME SPI, because the
// receiver chooses it.
func xfrmSA(source, target, spi string, bytes int) string {
	return "src " + source + " dst " + target + "\n" +
		"\tproto esp spi " + spi + " reqid 1 mode tunnel\n" +
		"\tlifetime current:\n" +
		"\t\t" + strconv.Itoa(bytes) + "(bytes), 1(packets)\n" +
		"\tlifetime config:\n" +
		"\t\t99(bytes)\n"
}

// espDump renders what one peer prints for a bidirectional Child SA: the SA
// carrying Ze to strongSwan under spiToSwan, and the SA carrying strongSwan to
// Ze under spiToZe. Both peers see both records and both SPIs.
func espDump(toSwanBytes, toZeBytes int) string {
	return xfrmSA(zeIP, swanIP, "0x1", toSwanBytes) + xfrmSA(swanIP, zeIP, "0x2", toZeBytes)
}

func losslessPing(count int) string {
	return strconv.Itoa(count) + " packets transmitted, " + strconv.Itoa(count) +
		" packets received, 0% packet loss\n"
}

func lostPing(count int) string {
	return strconv.Itoa(count) + " packets transmitted, 0 packets received, 100% packet loss\n"
}

// VALIDATES: The one direction strongSwan never encrypted is refused, which is
// the state measured in psk-site-to-site on 2026-08-30 with charon bypass-lan
// loaded: Ze encrypts the echo request, strongSwan decrypts it, and the reply
// leaves in the clear.
// PREVENTS: one stimulus satisfying two clauses, because Ze encrypting advances
// its own outbound SA and strongSwan inbound SA under one SPI.
func TestVerifyTunnelTrafficRejectsOneWayTraffic(t *testing.T) {
	fake := scriptedLab()
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 500))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 500))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, lostPing(4))

	err := checkerFixture(fake).verifyTunnelTraffic(context.Background(), "traffic did not flow through the XFRM tunnel")
	if err == nil {
		t.Fatal("one-way ESP accepted as a bidirectional tunnel proof")
	}
}

// VALIDATES: The directed proof is reached through the registered psk-site-to-site
// checker, not by calling the assertion directly.
// PREVENTS: a strengthened helper that no scenario consumes.
func TestPSKCheckerRejectsTrafficThatStrongSwanNeverEncrypted(t *testing.T) {
	fake := scriptedLab()
	fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
	fake.answer(zePeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(swanPeer, []string{"ip", "xfrm", "state"}, "proto esp spi 0x1\nproto esp spi 0x2\n")
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 500))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 500))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))

	err := checkPSKSiteToSite(context.Background(), checkerFixture(fake))
	if err == nil || !strings.Contains(err.Error(), "strongSwan encrypted nothing toward Ze") {
		t.Fatalf("non-discriminating verdict: %v", err)
	}
}

// VALIDATES: A ping that lost packets fails the tunnel proof even when every
// directed SA advanced.
// PREVENTS: the discarded ping verdict this spec exists to restore.
func TestVerifyTunnelTrafficRejectsLossyPing(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
	}{
		{"total loss", lostPing(4)},
		{"one percent, the first invalid value above zero", "100 packets transmitted, 99 packets received, 1% packet loss\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := scriptedLab()
			fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
			fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
			fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, test.output)

			err := checkerFixture(fake).verifyTunnelTraffic(context.Background(), "traffic did not flow through the XFRM tunnel")
			if err == nil || !strings.Contains(err.Error(), "packet loss") {
				t.Fatalf("lossy ping verdict = %v", err)
			}
		})
	}
}

// VALIDATES: A lossless ping over a clear path never passes on its own.
// PREVENTS: the bypass-lan signature, where an unprotected ping succeeds and no
// SA carries anything.
func TestVerifyTunnelTrafficRejectsPingWithNoESP(t *testing.T) {
	fake := scriptedLab()
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(10, 500))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(10, 500))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))

	err := checkerFixture(fake).verifyTunnelTraffic(context.Background(), "traffic did not flow through the XFRM tunnel")
	if err == nil || !strings.Contains(err.Error(), "did not advance") {
		t.Fatalf("clear-path ping verdict = %v", err)
	}
}

// VALIDATES: Ping output with no packet-loss summary is a failure.
// PREVENTS: a regexp that found no match being read as success, which would make
// the check answer for a ping that never reported.
func TestVerifyTunnelTrafficRejectsUnparseablePing(t *testing.T) {
	fake := scriptedLab()
	fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 600))
	fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, "ping: bad address 172.28.0.3\n")

	err := checkerFixture(fake).verifyTunnelTraffic(context.Background(), "traffic did not flow through the XFRM tunnel")
	if err == nil || !strings.Contains(err.Error(), "printed no packet-loss summary") {
		t.Fatalf("unreadable ping verdict = %v", err)
	}
}

// VALIDATES: Each of the four simplex SAs names itself when it is the one that
// stalled, and a retired SA is worded apart from a stalled counter.
// PREVENTS: a reader who cannot tell "strongSwan sent nothing" from "Ze received
// nothing", and a rekey reading as a traffic failure.
func TestVerifyTunnelTrafficNamesTheDirectionThatStalled(t *testing.T) {
	rekeyed := xfrmSA(zeIP, swanIP, "0x1", 20) + xfrmSA(swanIP, zeIP, "0x3", 700)
	for _, test := range []struct {
		name                  string
		zeBefore, zeAfter     string
		swanBefore, swanAfter string
		want                  string
	}{
		{"ze outbound stalled", espDump(10, 500), espDump(10, 600), espDump(10, 500), espDump(20, 600),
			"Ze encrypted nothing toward strongSwan"},
		{"strongSwan inbound stalled", espDump(10, 500), espDump(20, 600), espDump(10, 500), espDump(10, 600),
			"strongSwan accepted no ESP from Ze"},
		{"strongSwan outbound stalled", espDump(10, 500), espDump(20, 600), espDump(10, 500), espDump(20, 500),
			"strongSwan encrypted nothing toward Ze"},
		{"ze inbound stalled", espDump(10, 500), espDump(20, 500), espDump(10, 500), espDump(20, 600),
			"Ze decrypted no ESP from strongSwan"},
		{"ze inbound SA retired between the snapshots", espDump(10, 500), rekeyed, espDump(10, 500), espDump(20, 600),
			"no surviving SA"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := scriptedLab()
			fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, test.zeBefore, test.zeAfter)
			fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, test.swanBefore, test.swanAfter)
			fake.answer(zePeer, []string{"ping", "-c", "4", "-W", "2", swanIP}, losslessPing(4))

			err := checkerFixture(fake).verifyTunnelTraffic(context.Background(), "traffic did not flow through the XFRM tunnel")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verdict = %v, want it to name %q", err, test.want)
			}
		})
	}
}

// VALIDATES: A caller that claims no direction is refused.
// PREVENTS: an empty claim set reading as a proof that everything moved.
func TestVerifyESPDirectionsRefusesAnEmptyClaim(t *testing.T) {
	err := checkerFixture(scriptedLab()).verifyESPDirections(context.Background(), "nothing claimed", nil)
	if err == nil || !strings.Contains(err.Error(), "claimed no ESP direction") {
		t.Fatalf("empty claim verdict = %v", err)
	}
}

// VALIDATES: pingLoss reads the boundary values ping emits and refuses output it
// cannot read.
// PREVENTS: an unreadable summary defaulting to zero loss.
func TestPingLossBoundaries(t *testing.T) {
	for _, test := range []struct {
		output string
		want   int
	}{
		{"4 packets transmitted, 4 packets received, 0% packet loss", 0},
		{"100 packets transmitted, 99 packets received, 1% packet loss", 1},
		{"4 packets transmitted, 0 packets received, 100% packet loss", 100},
	} {
		t.Run(test.output, func(t *testing.T) {
			got, err := pingLoss(test.output)
			if err != nil || got != test.want {
				t.Fatalf("loss = %d, error = %v, want %d", got, err, test.want)
			}
		})
	}
	if _, err := pingLoss("4 packets transmitted\n"); err == nil {
		t.Fatal("output with no summary read as zero loss")
	}
	if _, err := pingLoss(""); err == nil {
		t.Fatal("empty ping output read as zero loss")
	}
}

// VALIDATES: esp-form-change asserts Ze outbound, strongSwan inbound and
// strongSwan outbound, and passes while Ze's inbound KERNEL SA stays still.
// PREVENTS: forcing the fourth direction on a scenario whose whole subject is Ze
// receiving that ESP in userspace, and letting strongSwan stop encrypting unseen.
func TestESPFormChangeClaimsThreeDirections(t *testing.T) {
	if len(espFormChangeDirections) != 3 {
		t.Fatalf("esp-form-change claims %d directions", len(espFormChangeDirections))
	}
	for _, want := range espFormChangeDirections {
		if want == zeDecrypts {
			t.Fatal("esp-form-change claims Ze's inbound kernel SA, which that scenario refuses by design")
		}
	}
	checker, ok := scenarioCheckers["esp-form-change"]
	if !ok {
		t.Fatal("native integration registry does not execute esp-form-change")
	}
	newLab := func(swanBefore, swanAfter string) *scenarioLab {
		fake := scriptedLab()
		fake.answer(swanPeer, []string{"swanctl", "--list-sas"}, "ze: ESTABLISHED\nze-child: INSTALLED\n")
		fake.answer(zePeer, []string{"ip", "xfrm", "state"},
			"src 172.28.0.3 dst 172.28.0.2\n\tproto esp spi 0x2 encap type espinudp sport 4500 dport 4500\n"+
				"src 172.28.0.2 dst 172.28.0.3\n\tproto esp spi 0x1\n")
		fake.answer(zePeer, []string{"cat", "/proc/net/raw"},
			"  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops\n"+
				"   1: 00000000:0032 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 12345 2 0000000000000000 0\n")
		fake.answer(zePeer, []string{"cat", "/proc/net/xfrm_stat"}, "XfrmInStateMismatch 5\n", "XfrmInStateMismatch 9\n")
		// Ze's inbound SA (src swan dst ze, 0x2) stays at 500 in Ze's own dump,
		// because the kernel refuses the form and userspace receives instead.
		fake.answer(zePeer, []string{"ip", "-s", "xfrm", "state"}, espDump(10, 500), espDump(20, 500))
		fake.answer(swanPeer, []string{"ip", "-s", "xfrm", "state"}, swanBefore, swanAfter)
		fake.answer(zePeer, []string{"ping", "-c", "3", "-W", "2", swanIP}, losslessPing(3))
		fake.answer(swanPeer, []string{"ping", "-c", "3", "-W", "2", zeIP}, losslessPing(3))
		fake.peerLogs[swanPeer] = interoplab.LogResult{Available: true, Text: "charon started\n"}
		return checkerFixture(fake)
	}

	if err := checker(context.Background(), newLab(espDump(10, 500), espDump(20, 600))); err != nil {
		t.Fatalf("form disagreement rejected while Ze's inbound kernel SA correctly stood still: %v", err)
	}
	err := checker(context.Background(), newLab(espDump(10, 500), espDump(20, 500)))
	if err == nil || !strings.Contains(err.Error(), "strongSwan encrypted nothing toward Ze") {
		t.Fatalf("verdict with strongSwan sending no ESP = %v", err)
	}
}

// VALIDATES: Every prepared scenario that starts a strongSwan peer mounts the
// lab-wide charon drop-in read-only under /etc/strongswan.d/.
// PREVENTS: a scenario running with charon's bypass-lan shunt still installed,
// where an unprotected ping succeeds and no SA carries anything.
func TestEveryStrongSwanPeerMountsTheSharedLabDropIn(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	sources, err := interoplab.Discover(
		filepath.Join(root, "test", "interop-ipsec", "scenarios"),
		"",
		checkerAdapters(),
	)
	if err != nil {
		t.Fatal(err)
	}
	environment := interoplab.Environment{
		Image:          defaultFRRImage,
		SessionTimeout: 90 * time.Second,
		Suffix:         "lab-dropin-test",
	}
	network := interoplab.Network{Name: "ze-ipsec-lab-dropin-test", IPv4: networkPrefix}
	wantSource := filepath.Join(root, "test", "interop-ipsec", "strongswan-lab.conf")
	if _, err := os.Stat(wantSource); err != nil {
		t.Fatalf("shared lab drop-in missing: %v", err)
	}
	for _, source := range sources {
		prepared, err := scenarioPlan(root, environment, source, &scenarioState{}).
			Prepare(t.Context(), interoplab.PrepareContext{Source: source, Network: network})
		if err != nil {
			t.Fatalf("prepare %s: %v", source.Name, err)
		}
		if prepared.Cleanup != nil {
			if err := prepared.Cleanup(); err != nil {
				t.Errorf("cleanup %s: %v", source.Name, err)
			}
		}
		if !ipsecHasPeer(prepared.Peers, swanPeer) {
			t.Errorf("%s starts no strongSwan peer", source.Name)
			continue
		}
		swan := ipsecPeerByName(t, prepared.Peers, swanPeer)
		found := false
		for _, mount := range swan.Mounts {
			if mount.Source != wantSource {
				continue
			}
			found = true
			if mount.Target != "/etc/strongswan.d/98-lab.conf" || !mount.ReadOnly {
				t.Errorf("%s lab drop-in mounted as %#v", source.Name, mount)
			}
		}
		if !found {
			t.Errorf("%s strongSwan peer does not mount the shared lab drop-in: %#v", source.Name, swan.Mounts)
		}
	}
}
