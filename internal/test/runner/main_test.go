package runner

import (
	"os"
	"testing"
)

// TestMain clears the ambient suite-selection environment before any test runs.
//
// ZE_QEMU_LINUX_ONLY is a RUNTIME FILTER for real .ci suite runs: parseAndAdd
// stamps SkipReason = "ZE_QEMU_LINUX_ONLY (not option=needs-linux)" on every
// record that is not option=needs-linux (record_parse.go:246). That is correct
// for `make ze-qemu-needs-linux-test`, and wrong for a unit test that builds a
// Record and asserts on its SkipReason -- the record is skipped before the
// assertion ever describes anything.
//
// The QEMU unit phase runs inside exactly that environment, so
// TestNetnsLinkRunsUnderNetnsMode and TestParseCIOptionSkipOS both failed there
// while passing everywhere else. It went unseen until the phase was repaired and
// executed for the first time (its GOCACHE pointed through a host-only symlink,
// so it had been dying on startup).
//
// A test that genuinely wants the filter can still opt in with t.Setenv, which
// takes precedence over this and is restored automatically.
func TestMain(m *testing.M) {
	os.Unsetenv("ZE_QEMU_LINUX_ONLY") //nolint:errcheck // clearing an unset var is not an error worth handling
	os.Exit(m.Run())
}
