//go:build zetest

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the forced-answer grammar and the probe that reads it
// Related: probe_force_shipped.go -- the shipped build, which reads nothing

package kernelcap

import "github.com/ze-software/ze/internal/core/env"

// The variable exists only in a zetest build, the functional-test daemon
// (internal/le/test/functional/binaries.go adds the tag). The tag is the same
// gate the test-only plugins use (cmd/ze/plugins_zetest.go), so no shipped ze
// registers or reads it.
var _ = env.MustRegister(env.EnvEntry{
	Key:         forceEnv,
	Type:        envTypeString,
	Description: "Force enrolled kernel capability probe answers: <subsystem>=<present|absent|unknown>[,...] (zetest builds only)",
	Private:     true,
})

// forcedAnswersFromEnv returns the variable's forced-answer list.
func forcedAnswersFromEnv() string {
	return env.Get(forceEnv)
}
