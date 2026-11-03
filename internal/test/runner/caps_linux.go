// Design: docs/architecture/testing/ci-format.md -- option=needs-linux capability gating
// Related: record_parse.go -- the needs-linux option that consumes this probe

//go:build linux

package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// probeCaps reports whether this process holds EVERY capability in bits.
// An error means the capability set is unknown, never present or absent.
// The bit positions themselves live in caps.go so every platform can parse the
// caps= vocabulary.
//
// It reads the effective capability set from /proc/self/status rather than
// testing for uid 0, because the two are not the same thing: a setcap'd binary
// or an ambient-capability environment has capabilities without being root, and
// a root process inside a restrictive container may have had them dropped.
//
// A test that needs a capability it does not have cannot pass: the owning
// plugin fails its stage-3 handshake with "operation not permitted", exits, and
// the daemon never reaches the state the test asserts. Before this probe existed
// those tests did not fail cleanly -- they HUNG until the suite timeout, because
// the assertion was waiting on a message from a daemon whose plugin had died.
func probeCaps(bits ...int) (bool, error) {
	return probeCapsFile("/proc/self/status", bits...)
}

// probeCapsFile reads the capability source at path, so tests can exercise read
// and format failures without changing the host's procfs.
func probeCapsFile(path string, bits ...int) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("capability probe: cannot read %s: %w", path, err)
	}
	for line := range strings.Lines(string(data)) {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "CapEff:")
		if !ok {
			continue
		}
		mask, perr := strconv.ParseUint(strings.TrimSpace(rest), 16, 64)
		if perr != nil {
			return false, fmt.Errorf("capability probe: cannot parse CapEff in %s: %w", path, perr)
		}
		for _, bit := range bits {
			if mask&(1<<uint(bit)) == 0 { //nolint:gosec // bit is a package const, 0..63
				return false, nil
			}
		}
		return true, nil
	}
	return false, fmt.Errorf("capability probe: no CapEff line in %s", path)
}
