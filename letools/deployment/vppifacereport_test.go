package deployment

// The VPP interface proof's payload.
//
// Goal: prove that the answer is DATA and that its default rendering says the
// same thing the Python original printed. Because the answer is DATA, `| json`,
// `| yaml` and `| table` render it with no code in the tool. Method: marshal the
// report, read the document back and render the text for each outcome.

import (
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: the report encodes as a JSON document carrying every key an
// operator or a script would read.
// PREVENTS: a tool that answers finished text, which is what AC-7 forbids: the
// pipe operators would then have nothing to render.
func TestTheVPPIfaceReportIsStructuredData(t *testing.T) {
	report := VPPIfaceReport{
		Image:     VPPImage,
		Container: "ze-vpp-iface-1",
		Version:   "vpp v24.02",
		Plugins:   []PluginState{{Name: WireguardPlugin, Loaded: true}},
		Scenarios: []ScenarioResult{{Feature: "gre-tunnel", Outcome: OutcomePass, Detail: "created"}},
		Passed:    true,
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("the report does not encode: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("the encoded report does not parse: %v", err)
	}
	for _, key := range []string{"image", "container", "vpp-version", "plugins", "scenarios", "passed"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("the report answered no %q key: %v", key, parsed)
		}
	}
}

// VALIDATES: an outcome reaches JSON as its WORD, and a scenario nobody ran
// reaches it as "unspecified" rather than borrowing the name of a real outcome.
// PREVENTS: the zero value reading as a pass. That is the whole reason the
// outcome is a typed number instead of a `passed` and a `skipped` boolean.
func TestAnOutcomeRendersAsItsWord(t *testing.T) {
	cases := map[Outcome]string{
		OutcomeUnspecified: "unspecified",
		OutcomePass:        "pass",
		OutcomeSkip:        "skip",
		OutcomeFail:        "fail",
	}
	for outcome, want := range cases {
		raw, err := json.Marshal(outcome)
		if err != nil {
			t.Fatalf("marshal %v: %v", outcome, err)
		}
		var got string
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("the encoded outcome is not a string: %s", raw)
		}
		if got != want {
			t.Errorf("outcome %d rendered as %q, want %q", outcome, got, want)
		}
	}
}

// VALIDATES: the default rendering prints one PLUGIN line per probe and one
// prefixed line per scenario, and puts the query's output and the daemon's last
// lines under a failure alone.
// PREVENTS: a report that hides which features were skipped. A green run over an
// image shipping neither wireguard nor linux-cp has proven half the proof, and a
// reader who cannot see that cannot tell it from a full one.
func TestTheVPPIfaceReportRendersEveryOutcome(t *testing.T) {
	report := VPPIfaceReport{
		Plugins: []PluginState{
			{Name: WireguardPlugin, Loaded: false},
			{Name: LinuxCPPlugin, Loaded: true},
		},
		Scenarios: []ScenarioResult{
			{Feature: "gre-tunnel", Outcome: OutcomePass, Detail: "a tunnel exists"},
			{Feature: "wireguard", Outcome: OutcomeSkip, Detail: "no plugin"},
			{
				Feature: "lcp-pair", Outcome: OutcomeFail, Detail: "no pair",
				Evidence: []string{"lcp table is empty"},
				LogTail:  []string{"ze: apply refused"},
			},
		},
	}

	text := report.Text()
	for _, want := range []string{
		"PLUGIN: " + WireguardPlugin + " loaded=false",
		"PLUGIN: " + LinuxCPPlugin + " loaded=true",
		"OK: a tunnel exists",
		"SKIP: no plugin",
		"FAIL: no pair",
		"lcp table is empty",
		"ze log tail:",
		"ze: apply refused",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the rendering does not carry %q:\n%s", want, text)
		}
	}

	// The evidence and the log tail belong to the failure alone. A scenario that
	// passed has nothing to explain, and printing its query output would bury
	// the one line a reader is looking for.
	passOnly := VPPIfaceReport{Scenarios: []ScenarioResult{{
		Feature: "gre-tunnel", Outcome: OutcomePass, Detail: "a tunnel exists",
		Evidence: []string{"should not be printed"},
	}}}
	if strings.Contains(passOnly.Text(), "should not be printed") {
		t.Errorf("a passing scenario printed its evidence:\n%s", passOnly.Text())
	}
}
