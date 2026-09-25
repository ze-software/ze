// Design: docs/guide/chaos-testing.md -- the chaos orchestrator command
// Related: register.go -- the registration of this command

// Package chaosrun is `le chaos run`, the chaos orchestrator of
// internal/chaos/orchestrator.
package chaosrun

import "github.com/ze-software/ze/internal/chaos/orchestrator"

// name is the command as a developer types it after `le`.
const name = "chaos run"

// Answer hands args to the orchestrator unchanged and answers its exit code.
//
// The payload is nil because the orchestrator writes its own report, event log
// and generated config: a chaos run is a program, not a value for a pipe
// operator to render.
func Answer(args []string) (any, int) {
	return nil, orchestrator.CLIRun(args)
}
