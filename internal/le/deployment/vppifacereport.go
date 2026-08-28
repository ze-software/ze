// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: vppiface.go -- the run that fills this in
// Related: report.go -- the L2TP proof's own payload
//
// vppifacereport.go contains the answer from the VPP interface proof. The answer
// is data, so `| json`, `| yaml` and `| table` render it with no code here. Text
// is a second rendering of the same data for a person who typed no operator.
//
// The plugin table is part of the ANSWER, not narration. A green run over an
// image that ships neither wireguard nor linux-cp proves two of the four
// features and skips the other two. The plugin table lets the reader tell that
// run from a full one.

package deployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Outcome is what one scenario did.
//
// The zero value is Unspecified. Thus, a scenario that the run never reached
// cannot read as a pass. This is why Outcome is a typed number instead of a pair
// of booleans. `passed` and `skipped` are two fields for one fact. Their
// false-false corner has no name.
type Outcome uint8

const (
	// OutcomeUnspecified is a scenario that was never run.
	OutcomeUnspecified Outcome = iota
	// OutcomePass is a scenario whose feature VPP confirmed.
	OutcomePass
	// OutcomeSkip is a scenario whose plugin this image does not ship. It is
	// evidence-backed rather than silent: the plugin table says which.
	OutcomeSkip
	// OutcomeFail is a scenario whose feature VPP never showed.
	OutcomeFail
)

// String answers the word a person and a JSON document both read. An outcome
// nobody set says so, rather than borrowing the name of a real one.
func (o Outcome) String() string {
	switch o {
	case OutcomePass:
		return "pass"
	case OutcomeSkip:
		return "skip"
	case OutcomeFail:
		return "fail"
	case OutcomeUnspecified:
		return reportValueUnspecified
	}
	return reportValueUnspecified
}

// MarshalJSON writes the word rather than the number, so `| json` and `| yaml`
// carry a value a reader and a script can both use.
func (o Outcome) MarshalJSON() ([]byte, error) {
	var tb textbuf.Buffer
	return []byte(tb.Quoted(o.String()).String()), nil
}

// PluginState is one VPP plugin and whether this image loaded it.
type PluginState struct {
	Name   string `json:"name"`
	Loaded bool   `json:"loaded"`
}

// ScenarioResult is one interface feature's verdict.
//
// Evidence contains what VPP itself said for a failure: the query's output. For
// the LCP pair, it contains the container's link listing. This listing is the
// half of that proof that VPP's own command line cannot show. LogTail contains
// the daemon's last lines. Only a failure fills LogTail because a scenario that
// passed has nothing to explain.
type ScenarioResult struct {
	Feature  string   `json:"feature"`
	Outcome  Outcome  `json:"outcome"`
	Detail   string   `json:"detail"`
	Evidence []string `json:"evidence,omitempty"`
	LogTail  []string `json:"log-tail,omitempty"`
}

// VPPIfaceReport is one run of the VPP interface-feature proof.
type VPPIfaceReport struct {
	Image     string           `json:"image"`
	Container string           `json:"container"`
	Version   string           `json:"vpp-version"`
	Plugins   []PluginState    `json:"plugins"`
	Scenarios []ScenarioResult `json:"scenarios"`
	Passed    bool             `json:"passed"`
}

// Text renders the run for a person in the shape that the Python original
// printed. It prints one PLUGIN line per probe, then one line per scenario. Under
// a failure, it prints the query's own output and the daemon's last lines.
func (r VPPIfaceReport) Text() string {
	var tb textbuf.Buffer

	for _, plugin := range r.Plugins {
		tb.Str("PLUGIN: ").Str(plugin.Name).Str(" loaded=").Bool(plugin.Loaded).Byte('\n')
	}

	for i := range r.Scenarios {
		one := &r.Scenarios[i]
		switch one.Outcome {
		case OutcomePass:
			tb.Str("OK: ").Str(one.Detail).Byte('\n')
		case OutcomeSkip:
			tb.Str("SKIP: ").Str(one.Detail).Byte('\n')
		case OutcomeFail, OutcomeUnspecified:
			tb.Str("FAIL: ").Str(one.Detail).Byte('\n')
		}
		if one.Outcome != OutcomeFail {
			continue
		}
		for _, line := range one.Evidence {
			tb.Str(line).Byte('\n')
		}
		if len(one.LogTail) > 0 {
			tb.Str("ze log tail:\n")
			for _, line := range one.LogTail {
				tb.Str(line).Byte('\n')
			}
		}
	}

	return tb.String()
}
