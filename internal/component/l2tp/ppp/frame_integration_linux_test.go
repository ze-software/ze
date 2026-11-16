//go:build integration && linux

// Design: docs/guide/l2tp.md -- blocking PPP descriptor lifetime

package ppp

import (
	"errors"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// Queue delivery transfers ownership even when Stop prevents session dispatch.
func TestPPPStoppedDispatcherClosesQueuedDescriptors(t *testing.T) {
	channel, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := unix.Open(os.DevNull, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		_ = unix.Close(channel)
		t.Fatal(err)
	}
	driver := &Driver{
		sessionsIn:   make(chan StartSession, 1),
		stopCh:       make(chan struct{}),
		dispatchDone: make(chan struct{}),
		stopped:      true,
	}
	ops, _, _ := newFakeOps()
	driver.sessionsIn <- testSessionStart(StartSession{LCP: testLCPConfig()}, &fakeBackend{}, ops, channel, unit, 0)
	close(driver.stopCh)
	driver.dispatch()
	for _, fd := range []int{channel, unit} {
		if _, err := unix.Write(fd, []byte{1}); !errors.Is(err, unix.EBADF) {
			_ = unix.Close(fd)
			t.Errorf("stopped dispatcher retained owned descriptor %d: %v", fd, err)
		}
	}
}
