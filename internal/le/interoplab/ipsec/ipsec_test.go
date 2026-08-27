// Design: checkers.go -- typed fixtures prove positive and negative handshake discrimination.
// Related: test/interop-ipsec/parity_test.go -- live Python producer parity.
package ipsec

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
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
