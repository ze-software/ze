// Related: stressbird.go -- the native half of this duplicate-then-swap proof.
//
// This file pins the producer contract beside its Go replacement. It reads the
// live Python scenario instead of copying its rounds into a second fixture, then
// compares those values with the rounds the native runner actually reports.
package integration

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

var stressBirdProducerRoundPattern = regexp.MustCompile(
	`\(([\d_]+),\s*"([^"]+)",\s*(\d+)\)`,
)

// VALIDATES: AC-11 for ze-stress-bird-test. The old scenario and native runner
// name the same four prefix bases, counts, and peer timeouts, in the same order.
// PREVENTS: a green native fixture whose copied round table drifted with check.py.
func TestStressBirdRoundsMatchThePythonProducer(t *testing.T) {
	producer := readStressBirdProducer(t, "test", "stress", "scenarios", stressBirdScenario, "check.py")
	matches := stressBirdProducerRoundPattern.FindAllStringSubmatch(producer, -1)
	if len(matches) != len(stressBirdRounds) {
		t.Fatalf("producer rounds = %d, native rounds = %d", len(matches), len(stressBirdRounds))
	}
	for index, match := range matches {
		count, err := strconv.Atoi(strings.ReplaceAll(match[1], "_", ""))
		if err != nil {
			t.Fatalf("parse producer round %d count: %v", index, err)
		}
		timeout, err := strconv.Atoi(match[3])
		if err != nil {
			t.Fatalf("parse producer round %d timeout: %v", index, err)
		}
		round := stressBirdRounds[index]
		if count != round.prefixes || match[2] != round.prefixBase || timeout != int(round.peerWait.Seconds()) {
			t.Fatalf(
				"round %d = (%d, %q, %d), native = (%d, %q, %d)",
				index, count, match[2], timeout,
				round.prefixes, round.prefixBase, int(round.peerWait.Seconds()),
			)
		}
	}
}

// VALIDATES: the native action retains the exact gate name, scenario, metadata
// reason, root requirement, and BIRD configuration selected by the producer.
// PREVENTS: wiring a callable runner under a similar but different gate contract.
func TestStressBirdGateMetadataAndConfigurationMatchTheProducer(t *testing.T) {
	application := readStressBirdProducer(t, "scripts", "le", "application", "integration.py")
	for _, fragment := range []string{
		"name='ze-stress-bird-test'",
		"argv=_sudo_stress('04-bulk-ipv4-bird')",
		"the BIRD baseline the ze bulk-IPv4 stress numbers are read against.",
		"Needs root, bird2 and network namespaces",
	} {
		if !strings.Contains(application, fragment) {
			t.Fatalf("integration.py no longer contains %q", fragment)
		}
	}
	if StressBirdGate != "ze-stress-bird-test" || stressBirdScenario != "04-bulk-ipv4-bird" {
		t.Fatalf("native identity = (%q, %q)", StressBirdGate, stressBirdScenario)
	}

	config := readStressBirdProducer(t, "test", "stress", "scenarios", stressBirdScenario, "bird.conf")
	wantConfig := `log stderr all;
router id 172.31.0.2;

protocol device {
}

protocol bgp injector {
    local 172.31.0.2 as 65001;
    neighbor 172.31.0.3 as 65100;
    passive;
    hold time 90;

    ipv4 {
        import all;
        export none;
        receive limit 2000000;
    };
}
`
	if config != wantConfig {
		t.Fatalf("BIRD producer configuration changed:\n%s", config)
	}
}

// VALIDATES: the recording fixture reaches each producer-owned external effect:
// namespace creation, BIRD, four peers, four queries, and final teardown.
// PREVENTS: parity that compares constants while the callable runner omits work.
func TestStressBirdRecordedEffectsCoverTheProducerLifecycle(t *testing.T) {
	recorder := newStressBirdRecorder()
	recorder.routeCounts = []int{100_000, 250_000, 500_000, 1_000_000}
	report, code := runRecordedStressBird(t, recorder)
	if code != 0 {
		t.Fatalf("recorded native run code = %d, failure = %#v", code, report.Failure)
	}

	producerEffects := map[string]string{
		`_run(["ip", "netns", "add", ZE_NS])`: "run ip netns add ze-stress-ze-fixture",
		`_run(["ip", "netns", "add", BB_NS])`: "run ip netns add ze-stress-bb-fixture",
		`"bird",`:                             "start ip netns exec ze-stress-ze-fixture bird -f",
		`ze_test,`:                            "start ip netns exec ze-stress-bb-fixture /repo/bin/ze-test peer",
		`["birdc", "-s", bird_sock, "show", "route", "count"]`: "run ip netns exec ze-stress-ze-fixture birdc -s",
		`_run(["ip", "netns", "del", ZE_NS], check=False)`:     "run ip netns del ze-stress-ze-fixture",
		`_run(["ip", "netns", "del", BB_NS], check=False)`:     "run ip netns del ze-stress-bb-fixture",
	}
	harness := readStressBirdProducer(t, "test", "stress", "harness.py")
	for source, effect := range producerEffects {
		if !strings.Contains(harness, source) {
			t.Fatalf("harness.py no longer contains producer effect %q", source)
		}
		if !anyStressBirdEvent(recorder.events, effect) {
			t.Fatalf("native recording omitted %q; events = %v", effect, recorder.events)
		}
	}
}

func readStressBirdProducer(t *testing.T, parts ...string) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("resolve checkout: %v", err)
	}
	path := filepath.Join(append([]string{root}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read producer %s: %v", filepath.Join(parts...), err)
	}
	return string(content)
}
