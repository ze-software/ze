// Design: docs/contributing/gh-pages.md -- terminal demo scenarios
// Related: scenarios.go -- runScenario, the lab dispatch that reads this registry
// Related: validate_runtime.go -- validateDemoRuntime, the validation that reads it

package siteterminaldemo

import (
	"fmt"
	"io"
)

// scenarioRunner starts, drives or stops one demo's lab for the action a tape
// names through `ze-demo run <id> <action> [args]`.
type scenarioRunner func(action string, args []string, stdout io.Writer) error

// scenario is one demo's lab runner and its validator, registered under the
// id the manifest's `validate` field and the demo's tape name.
type scenario struct {
	// run is nil for a demo whose tape starts no lab of its own.
	run scenarioRunner
	// validate is required: no recording ships without a check of what it shows.
	validate func() error
}

// scenarioRegistry maps a demo id to its scenario. Each scenario registers
// itself from an init() in a register_*.go file, so a new demo edits no
// central switch or map.
type scenarioRegistry map[string]scenario

// scenarios is filled at init and only read afterwards, so it needs no lock.
var scenarios = scenarioRegistry{}

// add registers sc under id. A registration with no validator, or a second
// one under the same id, is a defect in this package and panics at init.
func (registry scenarioRegistry) add(id string, sc scenario) {
	if sc.validate == nil {
		panic("BUG: terminal demo scenario " + id + " registered without a validator")
	}
	if _, taken := registry[id]; taken {
		panic("BUG: terminal demo scenario " + id + " registered twice")
	}
	registry[id] = sc
}

// lookup returns the scenario registered under id, and refuses an unknown id
// by name.
func (registry scenarioRegistry) lookup(id string) (scenario, error) {
	sc, ok := registry[id]
	if !ok {
		return scenario{}, fmt.Errorf("unknown demo %q: no scenario is registered under that id", id)
	}
	return sc, nil
}
