// Design: docs/contributing/running-commands.md -- the host preflight a verification runs first
// Related: lifecycle.go -- the worktree verification it guards
// Related: current.go -- the current-checkout verification it guards

package verify

import (
	"github.com/ze-software/ze/internal/le/setup"
	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// HostPreflight answers nil when this host can run every stage the options'
// piece of the worktree population holds, and otherwise the refusal that names
// what is missing and how to add it.
//
// A caller MUST call it before it starts any work for the run: a functional
// stage is among the last of the population, so a host it cannot run on would
// otherwise learn so after every other stage (two hours on 2026-10-13).
func (o Options) HostPreflight() error {
	part, err := o.part()
	if err != nil {
		return err
	}
	return hostPreflight(o.population(), part, setup.FunctionalHostPreflight)
}

// hostPreflight runs functionalHost only when the piece holds a functional
// stage, so a piece of a cut run that binds no loopback address starts on any
// host. The check is a parameter so a test can name a host without one.
func hostPreflight(mode string, part verifyengine.Part, functionalHost func() error) error {
	stages, err := verifyengine.Dealt(mode, part)
	if err != nil {
		return err
	}
	for index := range stages {
		if stages[index].Identity.Command == verifyengine.FunctionalCommand {
			return functionalHost()
		}
	}
	return nil
}
