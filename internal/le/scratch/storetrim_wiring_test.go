// Related: storetrim.go -- storeTrimBeforeDispatch, the hook this test reaches through le
// Related: register.go -- the registration that hands the hook to le's root handler

package scratch_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/env"
	_ "github.com/ze-software/ze/internal/le"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	"github.com/ze-software/ze/internal/le/scratch"
)

// TestLeInvocationSpawnsStoreTrimWhenDue proves the le root handler every le
// command and hook passes through starts the store trim when the hourly stamp
// is due (AC-1 trigger), not when it is fresh (AC-2), and not under
// ze.le.store.trim=off (AC-15), and that the trigger changes neither the exit
// code nor the output of the command it rides on.
//
// Method: the root handler is the one internal/le registers, looked up by name
// as cmd/ze does, so the path under test is the composition's blank import of
// this package, its register.go hook, and the handler running it. ZE_REPO_ROOT
// names a throwaway checkout and the spawn seam records what it is handed, so
// no child starts. Each run is compared, exit code and captured stdout and
// stderr, with leroot.Dispatch of the same words, which is the handler without
// its before-dispatch hooks.
func TestLeInvocationSpawnsStoreTrimWhenDue(t *testing.T) {
	handler := registry.LookupRoot("le")
	if handler == nil {
		t.Fatal("internal/le registered no le root")
	}
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZE_REPO_ROOT", root)
	t.Setenv("ZE_LE_STORE_TRIM", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)

	var spawned []string
	var leadings [][]string
	original := *scratch.StoreTrimSpawn
	*scratch.StoreTrimSpawn = func(leading []string) scratch.TrimSpawn {
		return func(root string) error {
			leadings = append(leadings, leading)
			spawned = append(spawned, root)
			return nil
		}
	}
	t.Cleanup(func() { *scratch.StoreTrimSpawn = original })

	// The test binary is not named le, so the handler dispatches as `ze le`,
	// and the trim child's argv must lead with the root handler's name.
	words := []string{"scratch", "no-such-verb"}
	wantCode, wantOutput := captureOutput(t, func() int { return leroot.Dispatch("ze le", words) })

	runOnce := func(t *testing.T) {
		t.Helper()
		code, output := captureOutput(t, func() int { return handler(nil, words) })
		if code != wantCode {
			t.Errorf("exit code = %d, want %d (the trigger changed it)", code, wantCode)
		}
		if output != wantOutput {
			t.Errorf("output = %q, want %q (the trigger printed)", output, wantOutput)
		}
	}

	runOnce(t)
	if len(spawned) != 1 || spawned[0] != resolved {
		t.Fatalf("due stamp: spawned %q, want one trim of %s", spawned, resolved)
	}
	if !slices.Equal(leadings[0], []string{"le"}) {
		t.Errorf("trim child argv leads with %q, want [le] for a ze binary carrying le", leadings[0])
	}
	if _, statErr := os.Stat(filepath.Join(resolved, "tmp", "store-trim", "stamp")); statErr != nil {
		t.Fatalf("the trigger wrote no stamp: %v", statErr)
	}

	runOnce(t)
	if len(spawned) != 1 {
		t.Fatalf("fresh stamp: spawned %d trims, want still 1", len(spawned))
	}

	if removeErr := os.Remove(filepath.Join(resolved, "tmp", "store-trim", "stamp")); removeErr != nil {
		t.Fatal(removeErr)
	}
	t.Setenv("ZE_LE_STORE_TRIM", "off")
	env.ResetCache()
	runOnce(t)
	if len(spawned) != 1 {
		t.Fatalf("ze.le.store.trim=off: spawned %d trims, want still 1", len(spawned))
	}
}

// captureOutput runs call with stdout and stderr sent to one file, and answers
// its exit code and everything it wrote.
func captureOutput(t *testing.T, call func() int) (int, string) {
	t.Helper()
	sink, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close() //nolint:errcheck // read back below
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = sink, sink
	code := call()
	os.Stdout, os.Stderr = stdout, stderr
	data, err := os.ReadFile(sink.Name())
	if err != nil {
		t.Fatal(err)
	}
	return code, string(data)
}
