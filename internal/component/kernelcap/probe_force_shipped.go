//go:build !zetest

// Design: docs/architecture/doctor-and-health-checks.md -- the kernel capability tier
// Overview: probe.go -- the forced-answer grammar and the probe that reads it
// Related: probe_force_zetest.go -- the functional-test build, which reads the variable

package kernelcap

// forcedAnswersFromEnv answers no forced list in a shipped build. The owner
// ruled out an operator override (2026-08-14), so ze.test.kernelcap.force is
// neither registered nor read here, whatever the environment holds, and every
// probe runs.
func forcedAnswersFromEnv() string {
	return ""
}
