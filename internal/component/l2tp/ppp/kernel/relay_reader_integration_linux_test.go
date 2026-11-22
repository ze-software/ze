//go:build integration && linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay handoff (LAC role)
//
// The test runs unprivileged in a user, network and mount namespace of its own
// (userns.Enter). Linux ppp_open refuses with EPERM an opener without
// CAP_NET_ADMIN in its user namespace, whatever the device's mode, and root of
// that namespace holds it. The device's owner and group are unmapped there, so
// the open still needs the dip group grant `./le setup install` applies (its
// `ppp-access` step), hence the integration tag, as in
// internal/component/l2tp/bridge_integration_linux_test.go.

package kernel

import (
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestInterruptibleReaderWakesOnDevPPP proves the quiesce wake on the real
// /dev/ppp driver, which the unit tests replace with a pipe.
//
// VALIDATES: a Read waiting on a /dev/ppp descriptor returns
// errReadInterrupted once InterruptRead runs, and every later Read does too,
// so Driver.QuiesceSession can stop the channel reader before
// PPPIOCBRIDGECHAN (spec-home-cpe-14, step 7).
// METHOD: a /dev/ppp descriptor attached to no channel never polls readable
// (ppp_poll answers no events while file->private_data is unset), so the
// Read can only return through the wake. A blocking read(2) would wait
// forever, which is why the plain file cannot be quiesced.
// PREVENTS: QuiesceSession hanging on a real channel while it passes on a
// pipe.
func TestInterruptibleReaderWakesOnDevPPP(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	dev, err := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("cannot open /dev/ppp read-write: %v\n"+
			"This test runs as root of its own user namespace, which holds the CAP_NET_ADMIN "+
			"Linux requires to open /dev/ppp, so the device's group grant is missing: "+
			"/dev/ppp must be group dip, mode 0660, and you must be in dip.\n"+
			"Grant it, then log in again so the dip membership applies:\n"+
			"./le setup install", err)
	}
	fd, err := unix.Dup(int(dev.Fd()))
	if err != nil {
		t.Fatalf("dup /dev/ppp: %v", err)
	}
	if err := dev.Close(); err != nil {
		t.Fatalf("close the original /dev/ppp descriptor: %v", err)
	}

	file, err := newInterruptibleFDFile(fd, "ppp-test")
	if err != nil {
		t.Fatalf("newInterruptibleFDFile: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	reader, ok := file.(interface{ InterruptRead() error })
	if !ok {
		t.Fatal("the interruptible file has no InterruptRead")
	}

	type readResult struct {
		n   int
		err error
	}
	done := make(chan readResult, 1)
	go func() {
		buf := make([]byte, ppp.MaxFrameLen)
		n, rerr := file.Read(buf)
		done <- readResult{n: n, err: rerr}
	}()

	select {
	case got := <-done:
		t.Fatalf("Read returned (%d, %v) before the wake: nothing should be readable", got.n, got.err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := reader.InterruptRead(); err != nil {
		t.Fatalf("InterruptRead: %v", err)
	}
	select {
	case got := <-done:
		if !errors.Is(got.err, errReadInterrupted) {
			t.Fatalf("woken Read = (%d, %v), want errReadInterrupted", got.n, got.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Read on /dev/ppp did not return after InterruptRead")
	}

	buf := make([]byte, ppp.MaxFrameLen)
	if _, err := file.Read(buf); !errors.Is(err, errReadInterrupted) {
		t.Fatalf("Read after the wake = %v, want errReadInterrupted", err)
	}
}
