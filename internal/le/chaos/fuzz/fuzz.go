// Design: docs/guide/chaos-testing.md -- the chaos swarm fuzzer command
// Related: register.go -- the registration of this command

// Package chaosfuzz is `le chaos fuzz`, the VOPR-style multi-seed in-process
// swarm fuzzer of internal/chaos/fuzz.
package chaosfuzz

import "github.com/ze-software/ze/internal/chaos/fuzz"

// name is the command as a developer types it after `le`.
const name = "chaos fuzz"

// Answer hands args to the chaos swarm fuzzer and returns its exit code.
func Answer(args []string) (any, int) {
	return nil, fuzz.CLIRun(args)
}
