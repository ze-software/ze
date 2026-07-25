// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: record_parse.go -- the needs-linux option that consumes this probe

//go:build linux

package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// capNetAdmin is CAP_NET_ADMIN's bit position in the Linux capability bitmask
// (include/uapi/linux/capability.h).
const capNetAdmin = 12

// probeNetAdmin reports whether this process may perform privileged network
// configuration: creating interfaces, bringing links up, programming netlink.
//
// It reads the effective capability set from /proc/self/status rather than
// testing for uid 0, because the two are not the same thing: a setcap'd binary
// or an ambient-capability environment has CAP_NET_ADMIN without being root,
// and a root process inside a restrictive container may have had it dropped.
//
// A test that applies interface config and does NOT have this capability cannot
// pass: the interface plugin fails its stage-3 handshake with "operation not
// permitted", exits, and the daemon never reaches the state the test asserts.
// Before this probe existed those tests did not fail cleanly -- they HUNG until
// the suite timeout, because the assertion was waiting on a BGP message from a
// daemon whose plugin had already died.
func probeNetAdmin() bool {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		// Unreadable procfs: assume the capability is present rather than skip.
		// Over-skipping silently removes coverage, which is the worse failure
		// (ai/rules/no-test-deletion.md). But a guard that cannot evaluate must
		// SAY so (ai/rules/fail-closed-guards.md): without this line a broken
		// probe is indistinguishable from a genuine failure, and the next agent
		// re-derives the root cause from a hang.
		capsWarn("cannot read /proc/self/status", err)
		return true
	}
	for line := range strings.Lines(string(data)) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "CapEff:")
		if !ok {
			continue
		}
		mask, perr := strconv.ParseUint(strings.TrimSpace(rest), 16, 64)
		if perr != nil {
			capsWarn("cannot parse CapEff", perr)
			return true
		}
		return mask&(1<<capNetAdmin) != 0
	}
	capsWarn("no CapEff line in /proc/self/status", nil)
	return true
}

// capsWarn reports that the capability probe could not evaluate, so a test that
// then runs and fails is not mistaken for a genuine product failure.
func capsWarn(what string, err error) {
	var tb textbuf.Buffer
	tb.Str("runner: CAP_NET_ADMIN probe: ").Str(what).Str("; assuming the capability IS present")
	if err != nil {
		tb.Str(": ").Err(err)
	}
	fmt.Fprintln(os.Stderr, tb.String())
}
