package testqemu

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// VALIDATES: the serial stream copies every byte QEMU prints into the log,
// and records that QEMU's output ended, so a boot failure can tell an exited
// QEMU from a guest that is still running without a login prompt.
// PREVENTS: a stalled boot whose serial output is kept nowhere.
func TestSerialStreamKeepsTheLogAndSeesQEMUExit(t *testing.T) {
	var log bytes.Buffer
	serial := newSerialStream(context.Background(), strings.NewReader("OpenRC 0.62 is starting up Linux\nno getty here\n"), &log)

	login, err := serial.expect(context.Background(), "login:", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if login {
		t.Fatal("a stream with no login prompt matched login:")
	}
	if !serial.exited {
		t.Fatal("the end of QEMU's output was not recorded as QEMU exiting")
	}
	if !strings.Contains(log.String(), "no getty here") {
		t.Fatalf("serial log %q misses the guest's output", log.String())
	}
}

// VALIDATES: a boot timeout names the timeout as the cause and the serial log
// with the command to read it, and does not quote QEMU's stderr, whose
// accelerator fallback warning is not the cause. An exited QEMU is quoted.
// PREVENTS: "VM did not reach a login prompt; QEMU said: invalid accelerator hvf".
func TestBootFailureNamesTheCauseAndTheSerialLog(t *testing.T) {
	const warning = "qemu-system-x86_64: invalid accelerator hvf"
	const logName = "/w/tmp/qemu/serial-1.log"

	timeout := bootFailure(false, 5*time.Minute, warning, logName).Error()
	for _, want := range []string{"no login prompt within 5m0s", "tail -50 " + logName} {
		if !strings.Contains(timeout, want) {
			t.Errorf("timeout message %q misses %q", timeout, want)
		}
	}
	if strings.Contains(timeout, warning) {
		t.Errorf("timeout message %q blames QEMU's warning", timeout)
	}

	exited := bootFailure(true, 5*time.Minute, "could not open kernel", logName).Error()
	for _, want := range []string{"QEMU exited", "could not open kernel", logName} {
		if !strings.Contains(exited, want) {
			t.Errorf("exit message %q misses %q", exited, want)
		}
	}
}
