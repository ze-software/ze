// Related: job.go -- insideParent and parentRunning, the reading these cases drive
// Related: contention_test.go -- the fixtures: unset, writeEntry, admission

package job

import (
	"os"
	"os/exec"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// TestADeadParentMeansAFreshAdmission asks whether insideParent answers
// "inside" only while the parent entry's holder may still run. Its method:
// a parent entry named under ParentVariable, once for this test's own PID
// (alive), once for a child that has already been waited for (dead), once
// with no PID (the stand-in a QEMU guest names), and once for a live PID
// with no `ps` on PATH, so liveness cannot be read.
//
// VALIDATES: a parent le killed by SIGKILL, whose entry stays behind, no
// longer lets its child run uncounted inside a slot nobody holds; a stand-in
// parent and an unknown liveness still read as inside, as the registry
// counts both as an occupied slot.
// PREVENTS: a child of a dead parent answering "inside" and running heavy
// work outside every slot.
func TestADeadParentMeansAFreshAdmission(t *testing.T) {
	ended := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit 0")
	if err := ended.Run(); err != nil {
		t.Fatalf("run a child that ends: %v", err)
	}
	deadPID := ended.Process.Pid

	for _, row := range []struct {
		name   string
		body   string
		noPS   bool
		inside bool
	}{
		{"alive", pidBody("live", os.Getpid()), false, true},
		{"dead", pidBody("dead", deadPID), false, false},
		{"stand-in", "LABEL=qemu-held-by-host\n", false, true},
		{"unreadable pid", "LABEL=odd\nPID=x\n", false, true},
		{"unknown liveness", pidBody("unknown", os.Getpid()), true, true},
	} {
		t.Run(row.name, func(t *testing.T) {
			unset(t, ParentKey)
			root := t.TempDir()
			path := writeEntry(t, root, "parent", row.body)
			t.Setenv(ParentVariable, path)
			if row.noPS {
				t.Setenv("PATH", t.TempDir())
			}
			env.ResetCache()

			_, inside, err := admission(t, root).insideParent()
			if err != nil {
				t.Fatalf("insideParent: %v", err)
			}
			if inside != row.inside {
				t.Errorf("inside = %v, want %v", inside, row.inside)
			}
		})
	}
}

// pidBody answers a parent entry's body naming label and pid.
func pidBody(label string, pid int) string {
	var tb textbuf.Buffer
	return tb.Str("LABEL=").Str(label).Str("\nPID=").Int(int64(pid)).Byte('\n').String()
}
