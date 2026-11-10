// Design: docs/architecture/testing/interop.md -- which phase a scenario failed in
// Related: lab.go -- Suite.Run prints the phase and stops early; runScenario sets it.
package interoplab

import (
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ScenarioPhase names the step of a scenario run that failed, so a red that
// never reached the checker is not read as the feature's red: a helper that
// could not start fails every scenario in setup or readiness, and only a
// checker failure says anything about the behavior the scenario tests.
type ScenarioPhase string

const (
	// PhaseSetup covers everything before the first readiness wait and
	// between waits: pre-clean, network, prepare, validation, images, start
	// wrapping and each container run.
	PhaseSetup ScenarioPhase = "setup"
	// PhaseReadiness is a peer's declared readiness wait (PeerConfig.Ready).
	PhaseReadiness ScenarioPhase = "readiness"
	// PhaseChecker is the scenario's checker.
	PhaseChecker ScenarioPhase = "checker"
)

// setupFailuresBeforeStop is how many scenarios in a row may fail before their
// checker ran (in setup or readiness) before Suite.Run stops. A cause the
// whole lab shares, such as a helper binary that panics at start, fails every
// later scenario the same way, and the 2026-11-09 run lost four scenarios to
// one before anyone saw it. A checker failure or a pass resets the count, so a
// run of feature reds never stops early.
const setupFailuresBeforeStop = 3

// beforeChecker answers whether a failure in p happened before the checker
// ran, which is what Suite.Run counts toward its early stop.
func (p ScenarioPhase) beforeChecker() bool {
	switch p {
	case PhaseSetup, PhaseReadiness:
		return true
	case PhaseChecker:
		return false
	}
	panic("BUG: scenario phase '" + string(p) + "' is not one runScenario sets")
}

// inPhase appends " in <phase>" to a FAIL marker. A result built outside
// runScenario may carry no phase, and then the marker is left as it was.
func inPhase(line *textbuf.Buffer, phase ScenarioPhase) *textbuf.Buffer {
	if phase == "" {
		return line
	}
	return line.Str(" in ").Str(string(phase))
}

// stopReason says why Run stopped after failed, which failed in a row before
// their checker ran, and that notRun were skipped rather than passed. The fix
// line names the lab's own `./le` action when a registered catalog declares
// this suite's scenario directory (rerunCommand).
func (s Suite) stopReason(failed, notRun []string) string {
	var reason textbuf.Buffer
	reason.Int(int64(len(failed))).Str(" scenarios in a row failed before their checker ran, in setup or readiness: ").
		Join(failed, " ").
		Str(". A cause the whole lab shares, such as a helper that cannot start, fails every later scenario the same way, so ").
		Int(int64(len(notRun))).Str(" scenario(s) were not run and none of them passed: ").Join(notRun, " ").
		Str(".\nFix: correct the setup cause these failures name, then run the lab again:\n")
	command, known := s.rerunCommand()
	if known {
		return reason.Str(command).String()
	}
	return reason.Str("the same command that started this run").String()
}

// rerunCommand answers the `./le` command that runs this suite, read from the
// registered catalog whose scenario root holds the suite's first scenario, so
// the command is declared once by the lab (Catalog.Action) and never spelled
// here. It answers false when no registered catalog declares that directory,
// which a suite built by a test is.
func (s Suite) rerunCommand() (string, bool) {
	if len(s.Scenarios) == 0 {
		return "", false
	}
	scenarioRoot := filepath.Dir(s.Scenarios[0].Source.Directory)
	for _, name := range CatalogSuites() {
		catalog, held := CatalogNamed(name)
		if !held {
			continue
		}
		if filepath.Join(s.Checkout, filepath.FromSlash(catalog.ScenarioRoot)) != scenarioRoot {
			continue
		}
		return "./le " + strings.Replace(catalog.Action, "/", " ", 1), true
	}
	return "", false
}
