//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- original-file PPP attachment.
// Related: session_pair_linux.go -- originalPairLive, the probe before every borrow.

package kernel

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Goal: a signal during the liveness probe never fails the unit operation that
// borrows the pair. Method: probe two idle descriptors on one locked thread
// while another goroutine signals that thread with SIGURG, the signal the Go
// runtime itself uses for preemption. With nothing ready, Linux poll checks for
// a pending signal before it honors a zero timeout and returns EINTR.
// MUTATION: return poll's EINTR from originalPairLive instead of polling again.
func TestOriginalPairLiveSurvivesSignals(t *testing.T) {
	var channel, unit [2]int
	for _, pair := range []*[2]int{&channel, &unit} {
		if err := unix.Pipe2(pair[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			for _, fd := range pair {
				if err := unix.Close(fd); err != nil {
					t.Error(err)
				}
			}
		})
	}
	thread := make(chan int)
	stop := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		thread <- unix.Gettid()
		deadline := time.Now().Add(2 * time.Second)
		var err error
		for err == nil && time.Now().Before(deadline) {
			err = originalPairLive(channel[0], unit[0])
		}
		close(stop)
		result <- err
	}()
	tid := <-thread
	var signaller sync.WaitGroup
	signaller.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if err := unix.Tgkill(unix.Getpid(), tid, unix.SIGURG); err != nil {
				if !errors.Is(err, unix.ESRCH) {
					t.Error(err)
				}
				return
			}
		}
	})
	err := <-result
	signaller.Wait()
	if err != nil {
		t.Fatalf("a signal during the idle pair's liveness probe failed it: %v", err)
	}
}
