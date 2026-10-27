package crashlog

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// useCrashDir points the package at dir with keep records for one test and
// restores the previous values after it.
func useCrashDir(t *testing.T, dir string, keep int) {
	t.Helper()
	previousDir, previousKeep := crashDir, crashKeep
	crashDir, crashKeep = dir, keep
	t.Cleanup(func() { crashDir, crashKeep = previousDir, previousKeep })
}

// TestRecordRecoveredPanicListed proves a recovered panic leaves a record that
// the `show crashes` listing returns with its own kind, and that the record
// carries the origin, the panic value and the stack the caller handed it.
func TestRecordRecoveredPanicListed(t *testing.T) {
	useCrashDir(t, t.TempDir(), 5)

	name, err := RecordRecoveredPanic("bgp read, peer 192.0.2.1, message UPDATE", "42", []byte("goroutine 7 [running]:\nmain.f()"))
	if err != nil {
		t.Fatalf("RecordRecoveredPanic: %v", err)
	}

	summaries := ListCrashes()
	if len(summaries) != 1 {
		t.Fatalf("ListCrashes returned %d entries, want 1: %+v", len(summaries), summaries)
	}
	if summaries[0].Name != name {
		t.Fatalf("listed name %q, want the returned %q", summaries[0].Name, name)
	}
	if summaries[0].Kind != KindRecovered {
		t.Fatalf("listed kind %q, want %q", summaries[0].Kind, KindRecovered)
	}

	report := ReadCrash(name)
	for _, want := range []string{
		"\n=== Panic ===\n42\n",
		"goroutine 7 [running]:",
		"Origin: bgp read, peer 192.0.2.1, message UPDATE",
		"The process kept running.",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}

// TestRecordRecoveredPanicNoDir proves a process with no crash directory is
// told so, rather than handed a name for a record that was never written.
func TestRecordRecoveredPanicNoDir(t *testing.T) {
	useCrashDir(t, "", 5)

	name, err := RecordRecoveredPanic("origin", "boom", nil)
	if !errors.Is(err, ErrNoCrashDir) {
		t.Fatalf("err = %v, want ErrNoCrashDir", err)
	}
	if name != "" {
		t.Fatalf("name = %q, want empty with the error", name)
	}
}

// TestRecordRecoveredPanicWriteError proves a failed write is returned to the
// caller. The directory does not exist, so the write fails.
func TestRecordRecoveredPanicWriteError(t *testing.T) {
	useCrashDir(t, filepath.Join(t.TempDir(), "absent"), 5)

	if _, err := RecordRecoveredPanic("origin", "boom", nil); err == nil {
		t.Fatal("a write into a missing directory reported no error")
	}
}

// TestRecoveredRotationSparesProcessCrash proves the two rotations are apart:
// recovered records beyond keep are removed oldest first, and neither they nor
// a later process crash evict the other kind.
func TestRecoveredRotationSparesProcessCrash(t *testing.T) {
	dir := t.TempDir()
	useCrashDir(t, dir, 2)

	fatal := "crash-20000101-000000.log"
	if err := os.WriteFile(filepath.Join(dir, fatal), []byte("fatal"), 0o600); err != nil {
		t.Fatal(err)
	}

	var names []string
	for range 4 {
		name, err := RecordRecoveredPanic("origin", "boom", nil)
		if err != nil {
			t.Fatalf("RecordRecoveredPanic: %v", err)
		}
		names = append(names, name)
	}

	if ReadCrash(fatal) != "fatal" {
		t.Fatal("recovered records evicted the process crash report")
	}
	for _, gone := range names[:2] {
		if ReadCrash(gone) != "" {
			t.Errorf("old recovered record %s survived rotation past keep=2", gone)
		}
	}
	for _, kept := range names[2:] {
		if ReadCrash(kept) == "" {
			t.Errorf("newest recovered record %s was removed", kept)
		}
	}

	rotateCrashFiles(dir, 1)
	if ReadCrash(fatal) != "fatal" {
		t.Fatal("process crash rotation counted recovered records and removed the only process crash")
	}
	if ReadCrash(names[3]) == "" {
		t.Fatal("process crash rotation removed a recovered record")
	}
}
