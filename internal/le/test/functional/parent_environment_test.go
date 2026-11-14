// Design: docs/contributing/running-commands.md -- a tree of le processes holds one job slot
// Related: binaries.go -- BinarySet.Environment, the builder this test drives

package testfunctional

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
	"github.com/ze-software/ze/internal/le/job"
)

// TestBinarySetKeepsTheJobParentInEverySpelling proves that a functional
// suite started under a job slot sees the entry of the job that holds it, so
// its own le runs inside that slot and never waits for it.
//
// VALIDATES: spec-native-action-job-admission AC-5, for both trees the AC
// names. `test functional gating` runs each suite under this builder
// (Execute), and `rfc record` runs its functional child under it too
// (testoverlay.PrepareFunctional answers set.Environment as the child's
// environment).
// METHOD: name a parent entry in the process environment under the dotted
// spelling and the shell spelling, build the child environment of a canonical
// and an isolated binary set, and read each spelling back from it.
// PREVENTS: a builder that drops the parent variable. The child suite would
// then ask for a slot of its own while its parent holds the last one, and the
// tree would wait on itself.
func TestBinarySetKeepsTheJobParentInEverySpelling(t *testing.T) {
	t.Cleanup(env.ResetCache)
	root := t.TempDir()
	entry := filepath.Join(root, "tmp", ".ze-jobs", "test-functional-gating.1.job")
	shell := strings.ToUpper(strings.ReplaceAll(job.ParentKey, ".", "_"))
	t.Setenv(job.ParentKey, entry)
	t.Setenv(shell, entry)

	for _, canonical := range []bool{false, true} {
		set := BinarySet{Dir: filepath.Join(root, "bin"), Canonical: canonical}
		environ := set.Environment(gotoolchain.Toolchain{Root: root})
		for _, spelling := range []string{job.ParentKey, shell} {
			value, found := lastValue(environ, spelling)
			if !found {
				t.Errorf("canonical=%v: the child environment drops %s, so the suite would ask for a second slot", canonical, spelling)
				continue
			}
			if value != entry {
				t.Errorf("canonical=%v: the child environment names %s=%q, want the parent entry %q", canonical, spelling, value, entry)
			}
		}
	}
}

// lastValue answers the value the last entry of environ named name gives, the
// one an executed process reads, and whether any entry names it.
func lastValue(environ []string, name string) (string, bool) {
	value := ""
	found := false
	for _, entry := range environ {
		key, rest, cut := strings.Cut(entry, "=")
		if cut && key == name {
			value = rest
			found = true
		}
	}
	return value, found
}
