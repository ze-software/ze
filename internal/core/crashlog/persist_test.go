// Design: docs/architecture/diagnostics/crash-capture.md -- retention tests
//
// VALIDATES: kernel artifacts rotate under the existing ze.crash.keep count,
//            in timestamp order, rather than getting a retention rule of their
//            own.
// PREVENTS:  a panic loop filling /perm, and the older failure that a name
//            sorting by kind rather than by time would delete every Go panic
//            report before touching the first kernel one.

package crashlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKernelArtifactRetentionSharesKeep(t *testing.T) {
	dir := t.TempDir()
	// Six artifacts, alternating kind, one hour apart. The three oldest are the
	// ones a keep of three must drop, and two of those three are Go panic
	// reports while the third is a kernel one.
	names := []string{
		"crash-20260905-100000.log",
		"crash-20260905-110000-dmesg-ramoops-0-kernel.log",
		"crash-20260905-120000.log",
		"crash-20260905-130000-dmesg-ramoops-1-kernel.log",
		"crash-20260905-140000.log",
		"crash-20260905-150000-dmesg-ramoops-2-kernel.log",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("report\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	rotateCrashFiles(dir, 3)

	kept := listCrashFileNames(dir)
	if len(kept) != 3 {
		t.Fatalf("kept %d artifacts, want 3: %v", len(kept), kept)
	}
	want := names[3:]
	for i := range want {
		if kept[i] != want[i] {
			t.Fatalf("kept = %v, want %v", kept, want)
		}
	}
}

func TestKernelArtifactRotatesOnWrite(t *testing.T) {
	// The harvest writes through the same rotation as a Go panic report, so a
	// box that faults repeatedly cannot fill the crash directory.
	dir := t.TempDir()
	for minute := range 4 {
		record := KernelRecord{
			ID:   "dmesg-ramoops-0",
			When: time.Date(2026, 9, 5, 10, minute, 0, 0, time.UTC),
			Text: "Kernel panic - not syncing\n",
		}
		if _, err := writeKernelArtifact(dir, 2, record); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}

	kept := listCrashFileNames(dir)
	if len(kept) != 2 {
		t.Fatalf("kept %d artifacts, want 2: %v", len(kept), kept)
	}
}
