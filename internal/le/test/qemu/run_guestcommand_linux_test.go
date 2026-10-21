//go:build linux

// Overview: run_guestcommand.go -- the guest command's recorded process group
package testqemu

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// VALIDATES: the line records its process group at startup, and the interrupt
// line signals the group it reads from that file, ending the command through
// its own teardown (review N7).
// METHOD: startGuestCommandLine, then run guestInterruptRemote under /bin/sh.
// It reads /proc, which the Mac has not, so it runs on Linux and in the guest.
// PREVENTS: an interrupt that reads /proc of a pid at the moment it is sent,
// and a group file that names something other than the group.
func TestGuestInterruptLineSignalsTheRecordedGroup(t *testing.T) {
	groupFile := filepath.Join(t.TempDir(), "pgid")
	command, output := startGuestCommandLine(t, groupFile, guestTearsDownOnSIGINT)
	recorded, err := os.ReadFile(groupFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(recorded)), strconv.Itoa(command.Process.Pid); got != want {
		t.Errorf("the group file holds %q, want the line's process group %s", got, want)
	}
	sent, err := exec.CommandContext(t.Context(), "/bin/sh", "-c", guestInterruptRemote(groupFile)).Output()
	if err != nil {
		t.Fatalf("the interrupt line failed: %v", err)
	}
	if got := strings.TrimSpace(string(sent)); got != strconv.Itoa(command.Process.Pid) {
		t.Errorf("the interrupt line says it signaled %q, want group %d", got, command.Process.Pid)
	}
	err = command.Wait()
	exit, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exit.ExitCode() != 1 || !strings.Contains(output.String(), "torn down") {
		t.Errorf("the command did not tear down on the interrupt: %v, %q", err, output.String())
	}
}

// VALIDATES: QEMU and every ssh client a run starts die with le on Linux
// (review B1): the attributes carry Pdeathsig SIGKILL beside Setpgid.
// PREVENTS: a VM that outlives the le that started it, holding its port and
// its cores until somebody notices.
func TestChildProcAttrDiesWithLe(t *testing.T) {
	attr := childProcAttr()
	if !attr.Setpgid {
		t.Error("the child does not lead its own process group")
	}
	if attr.Pdeathsig != syscall.SIGKILL {
		t.Errorf("Pdeathsig = %v, want SIGKILL", attr.Pdeathsig)
	}
}
