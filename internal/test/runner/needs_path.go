// Design: docs/architecture/testing/ci-format.md -- option=needs-path prerequisite gating
// Related: record_parse.go -- the option that consumes this lookup
// Related: caps.go -- the needs-linux/netns-link prerequisite gates it sits beside

package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	errOptionNeedsPathMissingValue = errors.New("option:needs-path missing value=")
	errNeedsPathNoRepoRoot         = errors.New("could not find go.mod in any parent of the .ci file")
)

// needsPathSatisfied reports whether the declared repo-relative path exists,
// accepting a glob so a version-pinned artifact can be named precisely.
//
// Precision matters here because the gate fails OPEN when it is coarse: naming
// the directory `gokrazy/modcache/github.com/rtr7` was satisfied by the
// unrelated `rtr7/dhcp4@...` entries that live beside the kernel module, so a
// checkout without the kernel passed the gate and died on the same missing
// vmlinuz the gate exists to prevent (ai/rules/evidence.md). A glob
// lets the test name the FILE it actually reads --
// `gokrazy/modcache/github.com/rtr7/kernel@*/vmlinuz` -- without hardcoding the
// pinned version string, which would then need updating on every dep bump.
func needsPathSatisfied(root, value string) bool {
	full := filepath.Join(root, value)
	if !strings.ContainsAny(value, "*?[") {
		_, err := os.Stat(full)
		return err == nil
	}
	matches, err := filepath.Glob(full)
	return err == nil && len(matches) > 0
}

// repoRootFrom walks up from the directory holding a .ci file until it finds the
// module root (go.mod). The .ci file's own path is the anchor rather than the
// process working directory: each test runs in its own temp dir, so a
// relative-to-cwd lookup would resolve against the wrong tree.
func repoRootFrom(ciFile string) (string, error) {
	dir := filepath.Dir(ciFile)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errNeedsPathNoRepoRoot
		}
		dir = parent
	}
}
