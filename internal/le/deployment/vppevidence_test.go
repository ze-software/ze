package deployment

import (
	"reflect"
	"testing"
)

// TestVPPScenarioTableNamesEveryProducerScenario validates the complete and
// ordered producer population.
//
// VALIDATES: all eight effective-vpp.py scenarios are represented as data.
// PREVENTS: a scenario disappearing while the command still reports a pass.
func TestVPPScenarioTableNamesEveryProducerScenario(t *testing.T) {
	want := []string{
		VPPScenarioIPsec,
		VPPScenarioIPv4FIB,
		VPPScenarioMPLSFIB,
		VPPScenarioTrafficInterface,
		VPPScenarioTrafficProtocol,
		VPPScenarioTrafficDSCP,
		VPPScenarioTrafficMultiClass,
		VPPScenarioFirewall,
	}
	got := make([]string, 0, 8)
	for _, step := range NewVPP(t.TempDir()).scenarioRuns() {
		got = append(got, step.name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenarioRuns() = %v, want %v", got, want)
	}
	if len(want) != 8 {
		t.Fatalf("the absolute scenario count is %d, want 8", len(want))
	}
}

// TestVPPConstantsMatchTheProducer validates every value shared with the
// evidence producer.
//
// VALIDATES: routes, labels, classes, IPsec material and ACL tags are unchanged.
// PREVENTS: two internally consistent proofs exercising different data.
func TestVPPConstantsMatchTheProducer(t *testing.T) {
	got := []any{
		VPPFIBPrefix, VPPNextHop, VPPMPLSPrefix, VPPMPLSLabel,
		VPPTrafficPolicerClass, VPPTrafficProtocolClass, VPPTrafficProtocolNumber,
		VPPTrafficDSCPClass, VPPTrafficDSCPValue,
		VPPTrafficMultiClassA, VPPTrafficMultiProtocolA,
		VPPTrafficMultiClassB, VPPTrafficMultiProtocolB,
		VPPIPsecReportPrefix, VPPIPsecSPI, VPPIPsecInboundSPI, VPPIPsecSalt,
		VPPIPsecCipherKey, VPPFirewallACLTag,
	}
	want := []any{
		"10.20.0.0/24", "10.0.0.1", "10.30.0.0/24", 100,
		"default", "tcp", 6,
		"cs6", 48,
		"web", 6,
		"dns", 17,
		"ze-vpp-ipsec:", uint64(0x11223344), uint64(0x55667788), "0xdeadbeef",
		"000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", "ze/wan/input",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shared constants = %#v, want %#v", got, want)
	}
}

// TestParseVPPInterfaceAnswersTheCreatedLoopback validates both VPP output
// forms and the producer's fallback.
//
// VALIDATES: the created loopback is used for every later scenario.
// PREVENTS: a parser selecting an unrelated token from VPP output.
func TestParseVPPInterfaceAnswersTheCreatedLoopback(t *testing.T) {
	for _, tt := range []struct {
		name string
		text string
		want string
	}{
		{name: "named", text: "loop7\n", want: "loop7"},
		{name: "sentence", text: "create loopback interface loop12\n", want: "loop12"},
		{name: "fallback", text: "created\n", want: "loop0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVPPInterface(tt.text); got != tt.want {
				t.Fatalf("parseVPPInterface(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

// TestParseVPPInterfaceIndexRejectsMissingAndInvalidIndexes validates the
// boundary that feeds the IPsec probe.
//
// VALIDATES: only the exact interface row supplies its numeric index.
// PREVENTS: an absent or malformed index silently becoming zero.
func TestParseVPPInterfaceIndexRejectsMissingAndInvalidIndexes(t *testing.T) {
	if got, err := parseVPPInterfaceIndex("loop0 7 up\n", "loop0"); err != nil || got != 7 {
		t.Fatalf("parse index = %d, %v", got, err)
	}
	for _, text := range []string{"loop1 7 up\n", "loop0 nope up\n"} {
		if _, err := parseVPPInterfaceIndex(text, "loop0"); err == nil {
			t.Fatalf("parseVPPInterfaceIndex(%q) succeeded", text)
		}
	}
}

// TestParseIPsecProbeRequiresEveryCleanupAndStateID validates the probe report
// as one complete boundary.
//
// VALIDATES: the probe ran both the close-cleanup and persistent-state halves.
// PREVENTS: a partial probe output becoming valid IPsec evidence.
func TestParseIPsecProbeRequiresEveryCleanupAndStateID(t *testing.T) {
	valid := "ze-vpp-ipsec:spd-id=41\n" +
		"ze-vpp-ipsec:sad-id=42\n" +
		"ze-vpp-ipsec:close-removed-spi=43\n" +
		"ze-vpp-ipsec:close-removed-spd-id=44\n"
	ids, err := parseIPsecProbe(valid)
	if err != nil {
		t.Fatalf("parse valid output: %v", err)
	}
	if ids.SPD != "41" || ids.CloseSPD != "44" {
		t.Fatalf("parsed ids = %#v", ids)
	}

	for _, remove := range []string{
		"ze-vpp-ipsec:spd-id=41\n",
		"ze-vpp-ipsec:sad-id=42\n",
		"ze-vpp-ipsec:close-removed-spi=43\n",
		"ze-vpp-ipsec:close-removed-spd-id=44\n",
	} {
		broken := valid
		broken = removeOne(broken, remove)
		if _, err := parseIPsecProbe(broken); err == nil {
			t.Fatalf("probe without %q succeeded", remove)
		}
	}
	if _, err := parseIPsecProbe(valid + "--- SKIP: TestVPPRealDataplaneInstalls\n"); err == nil {
		t.Fatal("a skipped probe succeeded")
	}
}

// TestVPPQueryFailureIsAnOperatingError validates the fail-closed difference
// from the script.
//
// VALIDATES: failed VPP queries do not contribute payload evidence.
// PREVENTS: an error line that repeats the query needle becoming a pass.
func TestVPPQueryFailureIsAnOperatingError(t *testing.T) {
	_, err := requireVPPQuery("show ip fib 10.20.0.0/24: unknown input", false, "show ip fib")
	if err == nil {
		t.Fatal("a failed query was accepted")
	}
	if got, err := requireVPPQuery("10.20.0.0/24 via 10.0.0.1", true, "show ip fib"); err != nil || got == "" {
		t.Fatalf("a successful query returned %q, %v", got, err)
	}
}

// TestVPPFailureStopsAtTheFirstScenario validates the report branch used by
// every scenario runner.
//
// VALIDATES: later scenarios do not run after the shared VPP state is untrusted.
// PREVENTS: a final pass overwriting an earlier proof failure.
func TestVPPFailureStopsAtTheFirstScenario(t *testing.T) {
	if empty := finishVPPScenario(VPPScenarioIPsec, nil); empty.Verdict != VPPProofFail {
		t.Fatalf("a scenario with no checks answered %s", empty.Verdict.String())
	}
	report := VPPReport{}
	appendVPPScenario(&report, VPPScenarioReport{Scenario: VPPScenarioIPsec, Verdict: VPPProofPass})
	if !appendVPPScenario(&report, VPPScenarioReport{Scenario: VPPScenarioIPv4FIB, Verdict: VPPProofFail}) {
		t.Fatal("the failed scenario did not request a stop")
	}
	if len(report.Scenarios) != 2 || report.Passed {
		t.Fatalf("report after failure = %#v", report)
	}
}

// TestVPPBuildErrorsStayOperatingErrors validates that the action does not
// manufacture a report verdict for a missing prerequisite.
//
// VALIDATES: operating failures are errors, not proof failures.
// PREVENTS: a missing Docker binary producing a valid-looking failed scenario.
func TestVPPBuildErrorsStayOperatingErrors(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	run := NewVPP(t.TempDir())
	run.Progress = nil
	report, err := run.Run()
	if err == nil {
		t.Fatal("a run without docker returned no operating error")
	}
	if len(report.Scenarios) != 0 {
		t.Fatalf("an operating error manufactured %d scenario verdicts", len(report.Scenarios))
	}
	if vppExitCode(report.Passed, err) != 1 {
		t.Fatal("an operating error exited zero")
	}
}

func removeOne(text, part string) string {
	at := -1
	for i := 0; i+len(part) <= len(text); i++ {
		if text[i:i+len(part)] == part {
			at = i
			break
		}
	}
	if at < 0 {
		return text
	}
	return text[:at] + text[at+len(part):]
}
