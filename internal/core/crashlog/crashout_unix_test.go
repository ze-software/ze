//go:build unix

package crashlog

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// TestHarvestPendingCrashes proves the harvest keeps a dead process's runtime
// output, removes a clean exit's header, and leaves a live process's file alone.
//
// VALIDATES: an unlocked pending file with output after the marker becomes a
// crash-*.log; an unlocked header-only file is removed; the file this process
// armed, whose lock it holds, stays pending.
// PREVENTS: one ze process harvesting another's live crash file, and a crash file
// for every run that ended cleanly.
func TestHarvestPendingCrashes(t *testing.T) {
	dir := t.TempDir()
	if err := armCrashOutput(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
		crashOut.Close() //nolint:errcheck // test cleanup
		crashOut = nil
	})
	live := crashOut.Name()

	pending := filepath.Join(dir, pendingDirName)
	header := string(appendRuntimeHeader(nil))
	crashed := filepath.Join(pending, "run-1.log")
	if err := os.WriteFile(crashed, []byte(header+"fatal error: boom\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(pending, "run-2.log")
	if err := os.WriteFile(clean, []byte(header), 0o600); err != nil {
		t.Fatal(err)
	}

	harvestPendingCrashes(dir, 5)

	if _, err := os.Stat(live); err != nil {
		t.Errorf("the live pending file was taken: %v", err)
	}
	if _, err := os.Stat(clean); !os.IsNotExist(err) {
		t.Errorf("the clean exit's pending file was kept: %v", err)
	}
	names := listCrashFileNames(dir)
	if len(names) != 1 || !strings.HasSuffix(names[0], "-run-1.log") {
		t.Fatalf("crash files = %v, want the one harvested from run-1", names)
	}
	data, err := os.ReadFile(filepath.Join(dir, names[0]))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fatal error: boom") {
		t.Errorf("the crash file lost the runtime output:\n%s", data)
	}
}
