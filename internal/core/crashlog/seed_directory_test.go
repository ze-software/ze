// Design: docs/architecture/diagnostics/crash-capture.md -- seed directory identity.
package crashlog

import (
	"os"
	"testing"
)

// TestSeedCrashRootReplacement swaps the entry between lstat and acquisition;
// the replacement must be refused before any capture file can be created.
func TestSeedCrashRootReplacement(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close() //nolint:errcheck // Test root.
	if err := root.Mkdir("crash", 0o700); err != nil {
		t.Fatal(err)
	}
	entry, err := root.Lstat("crash")
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("crash", "held"); err != nil {
		t.Fatal(err)
	}
	if err := root.Mkdir("crash", 0o700); err != nil {
		t.Fatal(err)
	}
	opened, err := openSeedCrashRoot(root, entry)
	if opened != nil {
		if closeErr := opened.Close(); closeErr != nil {
			t.Errorf("close unexpectedly admitted crash root: %v", closeErr)
		}
	}
	if err == nil {
		t.Fatal("accepted replacement crash directory")
	}
}
