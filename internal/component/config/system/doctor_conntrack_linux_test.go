//go:build linux

// Design: docs/features/ai-first.md -- the conntrack readiness check this component owns
// Detail: doctor_conntrack_linux.go -- checkConntrackProcfs and its registration

package system

import (
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// withNetfilterTree stands in a kernel for one test: treeErr is what a stat of
// the netfilter directory answers, keyErr what a write-access probe of
// nf_conntrack_max answers.
func withNetfilterTree(t *testing.T, treeErr, keyErr error) {
	t.Helper()
	previousStat, previousAccess := conntrackStatPath, conntrackAccessPath
	// os.DevNull always exists, so it stands in a present directory.
	conntrackStatPath = func(string) (os.FileInfo, error) {
		if treeErr != nil {
			return nil, treeErr
		}
		return os.Stat(os.DevNull)
	}
	conntrackAccessPath = func(string, uint32) error { return keyErr }
	t.Cleanup(func() {
		conntrackStatPath = previousStat
		conntrackAccessPath = previousAccess
	})
}

// conntrackTree builds the smallest config that tunes conntrack.
func conntrackTree() *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("system").GetOrCreateContainer("conntrack").Set("table-size", "1024")
	return tree
}

// TestCheckConntrackProcfsReportsAnUnwritableKey drives the check over a
// conntrack config on a kernel whose nf_conntrack_max refuses the write.
//
// VALIDATES: doctor-conntrack-procfs at warning severity, naming the key.
// PREVENTS: conntrack procfs dependency gaps being missed during readiness
// checks.
func TestCheckConntrackProcfsReportsAnUnwritableKey(t *testing.T) {
	withNetfilterTree(t, nil, os.ErrNotExist)

	diags := checkConntrackProcfs(diagnostic.DoctorCheckContext{Tree: conntrackTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeConntrackProcfs || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %s/%s, want %s at warning", diags[0].Code, diags[0].Severity, codeConntrackProcfs)
	}
	if !strings.HasSuffix(diags[0].Path, "nf_conntrack_max") || !strings.Contains(diags[0].Message, "not writable") {
		t.Fatalf("diagnostic = %+v, want the key named as not writable", diags[0])
	}
}

// TestCheckConntrackProcfsReportsAMissingTree covers the first verdict: the
// netfilter directory itself is absent, which is reported before any key.
//
// VALIDATES: the diagnostic names the directory with the unavailable wording.
// PREVENTS: a kernel without netfilter being reported as a permission problem
// on one key.
func TestCheckConntrackProcfsReportsAMissingTree(t *testing.T) {
	withNetfilterTree(t, os.ErrNotExist, nil)

	diags := checkConntrackProcfs(diagnostic.DoctorCheckContext{Tree: conntrackTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if !strings.HasSuffix(diags[0].Path, "netfilter") || !strings.Contains(diags[0].Message, "unavailable") {
		t.Fatalf("diagnostic = %+v, want the directory named as unavailable", diags[0])
	}
}

// TestCheckConntrackProcfsIsSilentWithoutTuning is the negative half: no
// conntrack block, no config, and a writable key each name no dependency.
//
// VALIDATES: no diagnostic in each case.
// PREVENTS: a netfilter warning on every box that never tuned conntrack.
func TestCheckConntrackProcfsIsSilentWithoutTuning(t *testing.T) {
	withNetfilterTree(t, os.ErrNotExist, os.ErrNotExist)
	if diags := checkConntrackProcfs(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkConntrackProcfs(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}

	withNetfilterTree(t, nil, nil)
	if diags := checkConntrackProcfs(diagnostic.DoctorCheckContext{Tree: conntrackTree()}); len(diags) != 0 {
		t.Fatalf("writable key: diagnostics = %d, want 0", len(diags))
	}
}

// TestConntrackDoctorCheckRegistered asks the registry the doctor runner reads
// whether it holds this check, at the phase and order the declaration states,
// and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and
// the registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestConntrackDoctorCheckRegistered(t *testing.T) {
	want := conntrackDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeConntrackProcfs) == nil {
		t.Fatalf("diagnostic code %q is not registered", codeConntrackProcfs)
	}
}
