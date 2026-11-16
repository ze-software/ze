//go:build integration && linux

// Design: docs/research/l2tpv2-ze-integration.md -- retained native PPP unit and birth fence

package kernel

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A real unit cannot be allocated again while its pending release retains the
// owner file. Both real poll-backed readers must nevertheless finish. Confirmed
// release then permits the same unit number, with none of the old birth alias.
func TestKernelNativeReleasePinsRealUnitBirth(t *testing.T) {
	channelFD, err := openDevPPP()
	if err != nil {
		t.Fatal(err)
	}
	unitFD, err := openDevPPP()
	if err != nil {
		if closeErr := unix.Close(channelFD); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	transferred := false
	t.Cleanup(func() {
		if !transferred {
			if err := unix.Close(channelFD); err != nil {
				t.Error(err)
			}
			if err := unix.Close(unitFD); err != nil {
				t.Error(err)
			}
		}
	})
	unit, err := ioctlGetSetInt(unitFD, pppiocNewUnit, -1)
	if err != nil {
		t.Fatal(err)
	}
	// Linux ppp_read/ppp_poll return EOF for an unconnected unit unless demand
	// mode is set. PPPIOCSFLAGS is _IOW('t',89,int); SC_LOOP_TRAFFIC is 0x200.
	// Both values and that branch were read from the pinned Linux 7.3 source.
	if _, err := ioctlGetSetInt(unitFD, 0x40047459, 0x200); err != nil {
		t.Fatal(err)
	}
	cfg := sessionTestConfig()
	cfg.ChanFD, cfg.UnitFD, cfg.UnitNum = channelFD, unitFD, unit
	cfg.InterfaceName = fmt.Sprintf("ppp%d", unit)
	// The control /dev/ppp file is deliberately unattached, as in the native
	// quiesce proof. The unit is real; demand mode keeps its own read blocked.
	entered := make(chan int, 2)
	var physicalReaders []*interruptibleFDFile
	ops := productionSessionOps()
	wrap := ops.wrap
	ops.wrap = func(fd int, name string) (io.ReadWriteCloser, error) {
		stream, err := wrap(fd, name)
		if err != nil {
			return nil, err
		}
		reader, ok := stream.(*interruptibleFDFile)
		if !ok {
			t.Fatalf("native stream has type %T, want *interruptibleFDFile", stream)
		}
		reader.readFrame = func(channel, wake int, buffer []byte) (int, error) {
			entered <- fd
			return pollThenRead(channel, wake, buffer)
		}
		physicalReaders = append(physicalReaders, reader)
		return reader, nil
	}
	transferred = true
	frames, handle, err := newSession(cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	uncertain := errors.New("remote cleanup has no confirmed answer")
	seal := &birthTestSeal{tag: "kernel-release-proof-" + rand.Text()}
	seal.cause = uncertain
	var workers sync.WaitGroup
	var closeDone chan struct{}
	t.Cleanup(func() {
		// Wake/join even when a mutation deliberately breaks the close path.
		for _, reader := range physicalReaders {
			if err := reader.InterruptRead(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
		seal.mu.Lock()
		seal.cause = nil
		seal.mu.Unlock()
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
		if closeDone != nil {
			select {
			case <-closeDone:
			case <-time.After(3 * time.Second):
				t.Error("native Close worker did not stop during cleanup")
			}
		}
		workers.Wait()
		for _, reader := range physicalReaders {
			if err := reader.channel.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
			if err := reader.wake.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				t.Error(err)
			}
		}
	})
	release, ok := handle.(ppp.NativeReleaseHandle)
	if !ok {
		t.Fatal("actual kernel owner does not support its release seal")
	}
	identity := handle.Identity()
	if err := release.BindNativeReleaseSeal(seal); err != nil {
		t.Fatal(err)
	}
	link, err := netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil {
		t.Fatal(err)
	}
	if link.Attrs().Name != identity.InterfaceName || link.Attrs().Alias != seal.tag {
		t.Fatalf("native birth does not match the admitted unit: %+v", link.Attrs())
	}
	results := make(chan error, 2)
	for _, reader := range []io.Reader{frames.Control(), frames.Opened()} {
		workers.Go(func() {
			var frame [8]byte
			_, err := reader.Read(frame[:])
			results <- err
		})
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	seen := make(map[int]bool, 2)
	for range 2 {
		select {
		case fd := <-entered:
			seen[fd] = true
		case <-deadline.C:
			t.Fatal("real readers never acquired both descriptor leases")
		}
	}
	if !seen[channelFD] || !seen[unitFD] {
		t.Fatalf("read leases did not cover both distinct native streams: %v", seen)
	}
	closeResult := make(chan error, 1)
	closeDone = make(chan struct{})
	go func() {
		defer close(closeDone)
		closeResult <- handle.Close()
	}()
	select {
	case err := <-closeResult:
		if !errors.Is(err, uncertain) {
			t.Fatalf("pending release lost its cause: %v", err)
		}
	case <-deadline.C:
		t.Fatal("native Close did not wake and release both borrowed read leases")
	}
	for range 2 {
		select {
		case err := <-results:
			if !errors.Is(err, errReadInterrupted) {
				t.Errorf("native reader did not wake through eventfd: %v", err)
			}
		case <-deadline.C:
			t.Fatal("native reader remained blocked behind retained unit ownership")
		}
	}
	if !release.NativeReleasePending() {
		t.Fatal("pending native release discarded its owner")
	}
	contender, err := openDevPPP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unix.Close(contender); err != nil {
			t.Error(err)
		}
	})
	if _, err := ioctlGetSetInt(contender, pppiocNewUnit, unit); err == nil {
		t.Fatal("an unconfirmed remote cleanup allowed the PPP unit number to be reused")
	}
	link, err = netlink.LinkByIndex(int(identity.NativeIndex))
	if err != nil || link.Attrs().Alias != seal.tag || link.Attrs().Name != identity.InterfaceName {
		t.Fatalf("pending release lost its actual native unit or birth: link=%v, err=%v", link, err)
	}
	seal.mu.Lock()
	seal.cause = nil
	seal.mu.Unlock()
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
	if release.NativeReleasePending() {
		t.Fatal("confirmed native release still reports a pending unit")
	}
	reused, err := ioctlGetSetInt(contender, pppiocNewUnit, unit)
	if err != nil || reused != unit {
		t.Fatalf("confirmed cleanup did not permit exact unit reuse: unit=%d, err=%v", reused, err)
	}
	replacement, err := netlink.LinkByName(identity.InterfaceName)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Attrs().Alias == seal.tag {
		t.Fatal("replacement unit inherited the old transport's native birth")
	}
}
