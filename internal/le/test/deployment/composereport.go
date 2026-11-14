// Design: ai/rules/cli.md -- one payload, and the operator picks the rendering
// Overview: compose.go -- the run that fills this in

package testdeployment

import (
	"github.com/ze-software/ze/internal/core/textbuf"
)

// composePassLine is the verdict line of a run that proved persistence. The
// spec's AC-5 evidence quotes it.
const composePassLine = "DEPLOYMENT-COMPOSE: PASS authenticated edit retained across a new container on the same persistent volume"

// composeFailPrefix starts the verdict line of a run that did not.
const composeFailPrefix = "DEPLOYMENT-COMPOSE: FAIL "

// ComposeStep is one step of the journey and what it showed.
type ComposeStep struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

// ComposeReport is one run of the Compose persistence proof. HostKey and the
// two container ids are evidence a reader checks: one key across two
// different containers is one store across a recreation.
type ComposeReport struct {
	Project         string        `json:"project"`
	Image           string        `json:"image"`
	BreakVolume     bool          `json:"break-volume"`
	BreakCommit     bool          `json:"break-commit"`
	Steps           []ComposeStep `json:"steps"`
	FirstContainer  string        `json:"first-container,omitempty"`
	SecondContainer string        `json:"second-container,omitempty"`
	HostKey         string        `json:"host-key,omitempty"`
	Proven          bool          `json:"proven"`
	Reason          string        `json:"reason,omitempty"`
	LeftBehind      []string      `json:"left-behind,omitempty"`
	// EarlierVolumes names each store volume an interrupted earlier run left,
	// with its removal command. It never fails the run.
	EarlierVolumes []string `json:"earlier-volumes,omitempty"`
	EditorTail     []string `json:"editor-tail,omitempty"`
	LogTail        []string `json:"log-tail,omitempty"`
}

// Verdict is the one line a reader quotes.
func (r ComposeReport) Verdict() string {
	if r.Proven {
		return composePassLine
	}
	var tb textbuf.Buffer
	tb.Str(composeFailPrefix)
	if r.BreakVolume {
		tb.Str("(counterfactual: no volume mounted) ")
	}
	if r.BreakCommit {
		tb.Str("(counterfactual: committed change overwritten) ")
	}
	return tb.Str(r.Reason).String()
}

// Text renders the run for a person: the verdict, each step, then what a
// failure leaves to read.
func (r ComposeReport) Text() string {
	var tb textbuf.Buffer
	tb.Str(r.Verdict()).Byte('\n')
	tb.Str("project ").Str(r.Project).Str(", image ").Str(r.Image).Byte('\n')
	for _, step := range r.Steps {
		mark := "FAIL"
		if step.Passed {
			mark = "ok"
		}
		tb.Str("  ").Str(mark).Str(" ").Str(step.Name).Str(": ").Str(step.Detail).Byte('\n')
	}
	writeComposeLines(&tb, "left behind:", r.LeftBehind)
	writeComposeLines(&tb, "earlier runs left:", r.EarlierVolumes)
	if r.Proven {
		return tb.String()
	}
	writeComposeLines(&tb, "SSH editor screen:", r.EditorTail)
	writeComposeLines(&tb, "ze log tail:", r.LogTail)
	return tb.String()
}

func writeComposeLines(tb *textbuf.Buffer, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	tb.Str(title).Byte('\n')
	for _, line := range lines {
		tb.Str("  ").Str(line).Byte('\n')
	}
}
