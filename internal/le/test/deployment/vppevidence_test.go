package testdeployment

import (
	"reflect"
	"strings"
	"testing"
)

// TestVPPScenarioTableNamesEveryProducerScenario validates the complete and
// ordered producer population.
//
// VALIDATES: the eight legacy effective-VPP scenarios, mpls-label-pop,
// fib-ecmp and fib-stale-sweep are represented as data, in execution order.
// PREVENTS: a scenario disappearing while the command still reports a pass.
func TestVPPScenarioTableNamesEveryProducerScenario(t *testing.T) {
	want := []string{
		VPPScenarioIPsec,
		VPPScenarioIPv4FIB,
		VPPScenarioMPLSFIB,
		VPPScenarioMPLSLabelPop,
		VPPScenarioFIBECMP,
		VPPScenarioFIBStaleSweep,
		VPPScenarioTrafficInterface,
		VPPScenarioTrafficProtocol,
		VPPScenarioTrafficDSCP,
		VPPScenarioTrafficMultiClass,
		VPPScenarioFirewall,
	}
	got := make([]string, 0, len(want))
	for _, step := range newVPP(t.TempDir()).scenarioRuns() {
		got = append(got, step.name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scenarioRuns() = %v, want %v", got, want)
	}
	if len(want) != 11 {
		t.Fatalf("the absolute scenario count is %d, want 11", len(want))
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
	run := newVPP(t.TempDir())
	run.Progress = nil
	report, err := run.Run(t.Context())
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

// TestVPPSelectedScenariosFilterAndOrder validates scenario filtering,
// declaration-order preservation, and conditional IPsec probe selection.
//
// VALIDATES: an empty selector runs all eleven scenarios; a non-empty selector
// runs only the named scenarios in declaration order and drops the IPsec probe
// build whenever ipsec is not selected.
// PREVENTS: scenario filtering reordering steps, duplicating runs, or forcing
// an unrelated ike/dataplane build during FIB/MPLS runs.
func TestVPPSelectedScenariosFilterAndOrder(t *testing.T) {
	v := newVPP(t.TempDir())
	all, err := v.selectedScenarioRuns()
	if err != nil {
		t.Fatalf("empty selection failed: %v", err)
	}
	if len(all) != 11 {
		t.Fatalf("empty selection returned %d scenarios, want 11", len(all))
	}
	if !needsIPsecProbe(all) {
		t.Fatal("default selection did not request the IPsec probe")
	}

	v.Scenarios = []string{VPPScenarioFIBStaleSweep, VPPScenarioIPv4FIB, VPPScenarioFIBECMP, VPPScenarioIPv4FIB}
	fibSteps, err := v.selectedScenarioRuns()
	if err != nil {
		t.Fatalf("FIB selection failed: %v", err)
	}
	gotFIB := make([]string, 0, len(fibSteps))
	for _, step := range fibSteps {
		gotFIB = append(gotFIB, step.name)
	}
	wantFIB := []string{VPPScenarioIPv4FIB, VPPScenarioFIBECMP, VPPScenarioFIBStaleSweep}
	if !reflect.DeepEqual(gotFIB, wantFIB) {
		t.Fatalf("FIB selection = %v, want %v", gotFIB, wantFIB)
	}
	if needsIPsecProbe(fibSteps) {
		t.Fatal("FIB selection requested the IPsec probe without ipsec selected")
	}

	v.Scenarios = []string{VPPScenarioFirewall, VPPScenarioIPsec}
	withIPsec, err := v.selectedScenarioRuns()
	if err != nil {
		t.Fatalf("ipsec selection failed: %v", err)
	}
	gotIPsec := make([]string, 0, len(withIPsec))
	for _, step := range withIPsec {
		gotIPsec = append(gotIPsec, step.name)
	}
	wantIPsec := []string{VPPScenarioIPsec, VPPScenarioFirewall}
	if !reflect.DeepEqual(gotIPsec, wantIPsec) {
		t.Fatalf("ipsec selection = %v, want %v", gotIPsec, wantIPsec)
	}
	if !needsIPsecProbe(withIPsec) {
		t.Fatal("selection containing ipsec did not request the IPsec probe")
	}
}

// TestVPPUnknownScenarioRefusedBeforeDocker validates that unknown scenario
// names are refused before looking up binaries, building daemons, or starting
// Docker, and that the CLI action exits with the usage code.
//
// VALIDATES: an unknown scenario name returns an error naming both the bad
// input and every declared scenario, and `test deployment vpp scenario <bad>`
// exits 2 without invoking Docker.
// PREVENTS: a typo burning minutes on a daemon build or silently running zero
// scenarios and reporting a pass.
func TestVPPUnknownScenarioRefusedBeforeDocker(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	run := newVPP(t.TempDir())
	run.Progress = nil
	run.Scenarios = []string{VPPScenarioIPv4FIB, "no-such-scenario"}

	report, err := run.Run(t.Context())
	if err == nil {
		t.Fatal("unknown scenario succeeded")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no-such-scenario") {
		t.Fatalf("error did not name the unknown scenario: %s", msg)
	}
	for _, step := range run.scenarioRuns() {
		if !strings.Contains(msg, step.name) {
			t.Fatalf("error did not list valid scenario %q: %s", step.name, msg)
		}
	}
	if len(report.Scenarios) != 0 {
		t.Fatalf("unknown scenario produced %d scenario verdicts", len(report.Scenarios))
	}

	_, code := Answer([]string{"vpp", vppScenarioKeyword, "no-such-scenario"})
	if code != composeUsageExit {
		t.Fatalf("Answer with unknown scenario exited %d, want %d", code, composeUsageExit)
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
