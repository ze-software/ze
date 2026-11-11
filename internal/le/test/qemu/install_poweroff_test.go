// Design: docs/architecture/testing/qemu-integration.md -- actual ISO poweroff.

package testqemu

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestISOCaptureWaitsForActualPoweroff(t *testing.T) {
	installer := installCaptureProcess(t, "hang")
	output, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 250*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("completion text from a still-running guest counted as poweroff: %v", err)
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("guest did not reach the completed-write state before its deadline: %q", output)
	}
}

func TestISOCaptureRejectsGuestFailureAfterCompletionText(t *testing.T) {
	installer := installCaptureProcess(t, "fail")
	output, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 5*time.Second)
	if err == nil {
		t.Fatal("guest failure after completion text counted as safe poweroff")
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("failure lost the guest's completed-write diagnostic: %q", output)
	}
}

func TestISOCaptureAcceptsCleanPoweroff(t *testing.T) {
	installer := installCaptureProcess(t, "success")
	output, err := installer.runCaptureUntilExit(context.Background(), []string{"fixture-qemu"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, InstallMarkISODone) {
		t.Fatalf("clean guest output lost: %q", output)
	}
}

func installCaptureProcess(t *testing.T, mode string) *Installer {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	installer := installTestInstaller(t, InstallKindISO)
	installer.ops.Start = func(command *exec.Cmd) error {
		command.Path = executable
		command.Args = []string{executable, "-test.run=^TestISOCaptureGuestProcess$"}
		command.Env = append(os.Environ(), "ZE_ISO_CAPTURE_TEST="+mode)
		// Replace the lookup result as well as the path: CommandContext
		// already tried to resolve the deliberately fake QEMU name.
		command.Err = nil
		return command.Start()
	}
	return installer
}

func TestISOCaptureGuestProcess(t *testing.T) {
	mode := os.Getenv("ZE_ISO_CAPTURE_TEST")
	if mode == "" {
		return
	}
	if _, err := fmt.Fprintln(os.Stdout, InstallMarkISODone); err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "success":
		return
	case "fail":
		os.Exit(7)
	case "hang":
		// This guest never exits within the proof's explicit deadline. The
		// parent asserts the deadline error, not a duration or scheduling gap.
		time.Sleep(time.Hour)
	default:
		t.Fatalf("unknown fixture mode %q", mode)
	}
}
