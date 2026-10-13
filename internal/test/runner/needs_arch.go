// Design: docs/architecture/testing/ci-format.md -- option=needs-arch host CPU gating
// Related: record_parse.go -- the option that consumes this decision
// Related: needs_path.go -- the needs-path prerequisite gate it sits beside

package runner

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var errOptionNeedsArchMissingValue = errors.New("option:needs-arch missing value=")

// needsArchSkipReason decides option=needs-arch:value=<goarch>[,<goarch>] for
// a host of arch host. It answers "" when host is in the list, and otherwise
// the skip reason, which names both the arch the test needs and the host's.
//
// Owner decision 2026-10-13: a test that builds or boots for a target arch runs
// only on a host whose CPU is that arch. Ze builds every kernel on a host of its
// own arch with no CPU emulation (internal/appliance/kernelbuilder CheckArch), so
// on any other host such a test cannot reach what it asserts, and the honest
// answer there is a visible SKIP, not a FAIL and not a pass.
//
// Every token is checked before the host is compared, so a malformed list fails
// the parse on every host rather than skipping, or running, by accident.
func needsArchSkipReason(value, host string) (string, error) {
	if value == "" {
		return "", errOptionNeedsArchMissingValue
	}
	matched := false
	for token := range strings.SplitSeq(value, ",") {
		arch := strings.TrimSpace(token)
		if err := checkNeedsArchToken(arch, value); err != nil {
			return "", err
		}
		if arch == host {
			matched = true
		}
	}
	if matched {
		return "", nil
	}
	var tb textbuf.Buffer
	return tb.Str("needs-arch=").Str(value).Str(", host GOARCH=").Str(host).
		Str(": a target arch is tested only on a CPU of that arch").String(), nil
}

// checkNeedsArchToken refuses a token outside the arches Ze builds a kernel
// for. The list is the kernel builder's own registry, so a typo (amd46) or a
// non-Go spelling (x86_64, aarch64) fails the parse instead of skipping the
// test on every host forever.
func checkNeedsArchToken(arch, value string) error {
	if arch == "" {
		return fmt.Errorf("option=needs-arch: value %q holds an empty arch", value)
	}
	supported := kernelbuilder.SupportedArches()
	if slices.Contains(supported, arch) {
		return nil
	}
	return fmt.Errorf("option=needs-arch: %q is not an arch Ze builds for (accepts %s)", arch, strings.Join(supported, ", "))
}
