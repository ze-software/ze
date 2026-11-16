// Design: docs/research/l2tpv2-ze-integration.md -- native unit release after remote cleanup

package kernel

import (
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type releaseTestSeal struct {
	mu         sync.Mutex
	cause      error
	panicValue error
	calls      int
	entered    chan struct{}
	resume     <-chan struct{}
	once       sync.Once
}

func (s *releaseTestSeal) SealNativeRelease() error {
	s.mu.Lock()
	s.calls++
	cause, panicValue, entered, resume := s.cause, s.panicValue, s.entered, s.resume
	s.mu.Unlock()
	if entered != nil {
		s.once.Do(func() { close(entered) })
	}
	if resume != nil {
		<-resume
	}
	if panicValue != nil {
		panic(panicValue)
	}
	return cause
}

type releaseTestStream struct {
	*sessionTestStream
	entered chan<- struct{}
}

func (s *releaseTestStream) Read(buffer []byte) (int, error) {
	s.entered <- struct{}{}
	return s.sessionTestStream.Read(buffer)
}

type nativeReleaseRig struct {
	handle       ppp.SessionHandle
	release      ppp.NativeReleaseHandle
	frames       ppp.FrameChannel
	streams      map[int]*sessionTestStream
	seal         *releaseTestSeal
	duplicates   int
	duplicateFD  int
	duplicateErr error
	released     []int
	readEntered  chan struct{}
}

func newNativeReleaseRig(t *testing.T) *nativeReleaseRig {
	t.Helper()
	rig := &nativeReleaseRig{
		streams:     map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}},
		seal:        &releaseTestSeal{},
		readEntered: make(chan struct{}, 2),
	}
	ops := sessionTestOps(rig.streams, &rig.released)
	// The birth marker has separate exact-write/refusal tests. These cases
	// isolate release ordering and descriptor ownership after a confirmed stamp.
	ops.stampBirth = func(ppp.SessionIdentity, string) error { return nil }
	ops.wrap = func(fd int, _ string) (io.ReadWriteCloser, error) {
		return &releaseTestStream{sessionTestStream: rig.streams[fd], entered: rig.readEntered}, nil
	}
	ops.duplicateFD = func(fd int) (int, error) {
		if fd != 32 {
			t.Errorf("retained descriptor copied %d, want the unit 32", fd)
		}
		rig.duplicates++
		return rig.duplicateFD, rig.duplicateErr
	}
	frames, handle, err := newSession(sessionTestConfig(), ops)
	if err != nil {
		t.Fatal(err)
	}
	rig.frames, rig.handle = frames, handle
	t.Cleanup(func() {
		rig.seal.mu.Lock()
		rig.seal.cause, rig.seal.panicValue = nil, nil
		rig.seal.mu.Unlock()
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	release, ok := handle.(ppp.NativeReleaseHandle)
	if !ok {
		t.Fatal("kernel owner cannot bind a native release seal before a remote operation")
	}
	rig.release = release
	return rig
}

func TestKernelNativeReleaseRetainsUnitUntilConfirmation(t *testing.T) {
	rig := newNativeReleaseRig(t)
	uncertain := errors.New("remote reset result is unknown")
	rig.seal.cause = uncertain
	identity := rig.handle.Identity()
	if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
		t.Fatal(err)
	}
	if err := rig.handle.Close(); !errors.Is(err, uncertain) {
		t.Fatalf("unconfirmed close = %v, want the remote cause", err)
	}
	for fd, stream := range rig.streams {
		if stream.closes != 1 {
			t.Errorf("borrowed stream %d closed %d times, want one", fd, stream.closes)
		}
	}
	if !rig.release.NativeReleasePending() || len(rig.released) != 0 {
		t.Fatalf("unconfirmed close released the retained unit: pending=%v, releases=%v", rig.release.NativeReleasePending(), rig.released)
	}
	if err := rig.handle.SetMTU(1492); err == nil {
		t.Fatal("retired handle still permits a network mutation")
	}
	if rig.handle.Identity() != identity {
		t.Fatal("retention changed the native interface identity")
	}
	rig.seal.cause = nil
	if err := rig.handle.Close(); err != nil {
		t.Fatal(err)
	}
	if rig.release.NativeReleasePending() || len(rig.released) != 1 || rig.released[0] != 0 {
		t.Fatalf("confirmed close did not release descriptor zero exactly once: %v", rig.released)
	}
	if err := rig.handle.Close(); err != nil {
		t.Fatal(err)
	}
	if rig.duplicates != 1 || len(rig.released) != 1 || rig.seal.calls != 2 {
		t.Fatalf("repeated close repeated native work: copies=%d, releases=%v, seals=%d", rig.duplicates, rig.released, rig.seal.calls)
	}
}

func TestKernelNativeReleaseBoundOnlyWhilePreparedAndOpen(t *testing.T) {
	rig := newNativeReleaseRig(t)
	bound, ok := rig.handle.(interface{ NativeReleaseBound() bool })
	if !ok {
		t.Fatal("kernel owner cannot prove native release binding before RPC reservation")
	}
	if bound.NativeReleaseBound() {
		t.Fatal("an unbound native owner granted RPC reservation authority")
	}
	rig.seal.cause = errors.New("remote cleanup is unconfirmed")
	if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
		t.Fatal(err)
	}
	if !bound.NativeReleaseBound() {
		t.Fatal("confirmed birth and retained unit did not prepare the open owner")
	}
	if err := rig.handle.Close(); err == nil {
		t.Fatal("unconfirmed native release reported success")
	}
	if bound.NativeReleaseBound() || !rig.release.NativeReleasePending() {
		t.Fatal("a retired owner still grants new RPC reservation authority")
	}
	rig.seal.cause = nil
	if err := rig.handle.Close(); err != nil {
		t.Fatal(err)
	}
	if bound.NativeReleaseBound() || rig.release.NativeReleasePending() {
		t.Fatal("a released owner retained native RPC authority")
	}
}

func TestKernelNativeReleaseRefusedDuplicateKeepsOriginalOwner(t *testing.T) {
	rig := newNativeReleaseRig(t)
	want := errors.New("duplicate unit: descriptor limit")
	rig.duplicateErr = want
	if err := rig.release.BindNativeReleaseSeal(rig.seal); !errors.Is(err, want) {
		t.Fatalf("binding = %v, want the duplicate cause", err)
	}
	for _, stream := range rig.streams {
		if stream.closes != 0 {
			t.Fatal("a refused release binding closed the original streams")
		}
	}
	if len(rig.released) != 0 || rig.release.NativeReleasePending() {
		t.Fatal("a failed duplicate invented a retained native owner")
	}
	rig.duplicateErr = nil
	if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
		t.Fatalf("failed binding consumed the only binding opportunity: %v", err)
	}
}

func TestKernelNativeReleaseRefusesInvalidBinding(t *testing.T) {
	for _, outcome := range []string{"nil", "typed nil", "unit alias", "channel alias", "negative", "second binding", "closed"} {
		t.Run(outcome, func(t *testing.T) {
			rig := newNativeReleaseRig(t)
			var seal ppp.NativeReleaseSeal = rig.seal
			switch outcome {
			case "nil":
				seal = nil
			case "typed nil":
				seal = (*releaseTestSeal)(nil)
			case "unit alias":
				rig.duplicateFD = 32
			case "channel alias":
				rig.duplicateFD = 31
			case "negative":
				rig.duplicateFD = -1
			case "second binding":
				if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
					t.Fatal(err)
				}
			case "closed":
				if err := rig.handle.Close(); err != nil {
					t.Fatal(err)
				}
			}
			before := rig.duplicates
			if err := rig.release.BindNativeReleaseSeal(seal); err == nil {
				t.Fatal("unsafe release binding was accepted")
			}
			if outcome == "nil" || outcome == "typed nil" || outcome == "second binding" || outcome == "closed" {
				if rig.duplicates != before {
					t.Fatal("a refused binding duplicated a native descriptor")
				}
			}
			if len(rig.released) != 0 {
				t.Fatal("a refused alias binding closed an original descriptor")
			}
		})
	}
}

func TestKernelNativeReleaseSealPanicRetainsUnit(t *testing.T) {
	rig := newNativeReleaseRig(t)
	want := errors.New("release seal panicked")
	rig.seal.panicValue = want
	if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
		t.Fatal(err)
	}
	if err := rig.handle.Close(); !errors.Is(err, want) {
		t.Fatalf("seal panic was not retained: %v", err)
	}
	if !rig.release.NativeReleasePending() || len(rig.released) != 0 {
		t.Fatal("seal panic released the native unit")
	}
}

func TestKernelNativeReleaseWakesBothReadersWhileUnitRetained(t *testing.T) {
	rig := newNativeReleaseRig(t)
	uncertain := errors.New("rate cleanup is unconfirmed")
	rig.seal.cause = uncertain
	rig.seal.entered = make(chan struct{})
	resume := make(chan struct{})
	rig.seal.resume = resume
	var resumeOnce sync.Once
	unblockSeal := func() { resumeOnce.Do(func() { close(resume) }) }
	t.Cleanup(unblockSeal)
	if err := rig.release.BindNativeReleaseSeal(rig.seal); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	var readers sync.WaitGroup
	for _, reader := range []io.Reader{rig.frames.Control(), rig.frames.Opened()} {
		readers.Go(func() {
			var buffer [8]byte
			_, err := reader.Read(buffer[:])
			done <- err
		})
	}
	t.Cleanup(func() {
		for _, stream := range rig.streams {
			if err := stream.InterruptRead(); err != nil {
				t.Error(err)
			}
		}
		readers.Wait()
	})
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for range 2 {
		select {
		case <-rig.readEntered:
		case <-deadline.C:
			t.Fatal("borrowed readers did not enter Read")
		}
	}
	closed := make(chan error, 1)
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		closed <- rig.handle.Close()
	}()
	t.Cleanup(func() {
		unblockSeal()
		select {
		case <-closeDone:
		case <-time.After(3 * time.Second):
			t.Error("Close worker did not stop during test cleanup")
		}
	})
	select {
	case <-rig.seal.entered:
	case <-deadline.C:
		t.Fatal("Close did not reach the release seal")
	}
	for fd, stream := range rig.streams {
		if stream.closes != 1 {
			t.Errorf("release seal ran before stream %d closed", fd)
		}
	}
	for range 2 {
		select {
		case err := <-done:
			if !errors.Is(err, io.ErrClosedPipe) {
				t.Errorf("retired reader result = %v", err)
			}
		case <-deadline.C:
			t.Fatal("a borrowed reader remained blocked behind the retained unit")
		}
	}
	if len(rig.released) != 0 {
		t.Fatal("reader shutdown released the retained native unit")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned before the release seal: %v", err)
	default:
	}
	unblockSeal()
	select {
	case err := <-closed:
		if !errors.Is(err, uncertain) {
			t.Fatalf("unconfirmed unit release lost its cause: %v", err)
		}
	case <-deadline.C:
		t.Fatal("Close did not return after its seal answered")
	}
}
