//go:build linux

// Design: docs/research/l2tpv2-ze-integration.md -- consuming exact captured files
// These are original-file ownership models over regular files, not PPP units,
// protocol negotiation or native counter measurements. Genuine PPP tests are
// in session_created_integration_linux_test.go.

package kernel

import (
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

func capturedModelChannel(t *testing.T, file *os.File) *CreatedChannel {
	t.Helper()
	if file == nil {
		var err error
		file, err = os.CreateTemp(t.TempDir(), "captured-channel-")
		if err != nil {
			t.Fatal(err)
		}
	}
	namespace, err := captureSessionNamespace()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	channel := &CreatedChannel{file: file, namespace: namespace, ready: true, poll: func(*os.File) error { return nil }}
	channel.self = channel
	t.Cleanup(func() {
		if err := channel.Close(); err != nil {
			t.Error(err)
		}
	})
	return channel
}

func capturedModelOwners(t *testing.T) (*CreatedUnit, *CreatedChannel, SessionConfig) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "captured-unit-")
	if err != nil {
		t.Fatal(err)
	}
	namespace, err := captureSessionNamespace()
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			t.Error(closeErr)
		}
		t.Fatal(err)
	}
	unit := &CreatedUnit{file: file, namespace: namespace, ready: true,
		backend: &sessionTestBackend{}, number: 7,
		identity:     ppp.SessionIdentity{InterfaceName: "captured-unit", NativeIndex: 73},
		poll:         func(*os.File) error { return nil },
		readCounters: func() (ppp.SessionCounters, error) { return ppp.SessionCounters{RxBytes: 123}, nil }}
	unit.self = unit
	t.Cleanup(func() {
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
	})
	channel := capturedModelChannel(t, nil)
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	return unit, channel, cfg
}

func capturedModelSession(t *testing.T, cfg SessionConfig, ops sessionOps) *session {
	t.Helper()
	if unit, ok := cfg.Unit.(*CreatedUnit); ok {
		bindCapturedModelNativeOperations(unit, ops)
	}
	ops.preparePair = func(*claimedSessionPair) error { return nil }
	_, handle, err := newSession(cfg, ops)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("actual owner type %T", handle)
	}
	return owner
}

func capturedFileClosed(t *testing.T, file *os.File) {
	t.Helper()
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("original file not closed: %v", err)
	}
}

func capturedModelAdmission(t *testing.T, cfg SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
	t.Helper()
	frames, handle, err := NewSession(cfg)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	return frames, handle, err
}

func TestCapturedSessionKeepsBothOriginalFiles(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	owner := capturedModelSession(t, cfg, productionSessionOps())
	control, ok := owner.channel.(*ownedSessionChannelStream)
	if !ok {
		t.Fatalf("channel owner type %T", owner.channel)
	}
	physicalControl, ok := control.ReadWriteCloser.(*interruptibleFDFile)
	if !ok || physicalControl.channel != channel.file {
		t.Fatal("channel wrapper reconstructed the original file")
	}
	opened, ok := owner.unit.(*ownedSessionUnitStream)
	if !ok {
		t.Fatalf("unit owner type %T", owner.unit)
	}
	physicalUnit, ok := opened.ReadWriteCloser.(*interruptibleFDFile)
	if !ok || physicalUnit.channel != unit.file {
		t.Fatal("unit wrapper reconstructed the original file")
	}
	if got, err := owner.Counters(); err != nil || got.RxBytes != 123 {
		t.Fatalf("live model counters %+v, %v", got, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	capturedFileClosed(t, channel.file)
	capturedFileClosed(t, unit.file)
	capturedFileClosed(t, channel.namespace.file)
	capturedFileClosed(t, unit.namespace.file)
}

func TestCapturedSessionFreshChannelClosesWhenUnitReplayed(t *testing.T) {
	unit, original, cfg := capturedModelOwners(t)
	owner := capturedModelSession(t, cfg, productionSessionOps())
	fresh := capturedModelChannel(t, nil)
	bad := cfg
	bad.Channel = fresh
	if _, handle, err := capturedModelAdmission(t, bad); err == nil || handle != nil {
		t.Fatalf("replayed unit admitted: %v", err)
	}
	capturedFileClosed(t, fresh.file)
	if _, err := original.file.Stat(); err != nil {
		t.Fatalf("replay closed original channel: %v", err)
	}
	if got, err := owner.Counters(); err != nil || got.RxBytes != 123 {
		t.Fatalf("replay closed original unit %+v, %v", got, err)
	}
	if _, err := unit.file.Stat(); err != nil {
		t.Fatal(err)
	}
}

func TestCapturedSessionFreshUnitClosesWhenChannelReplayed(t *testing.T) {
	_, original, cfg := capturedModelOwners(t)
	owner := capturedModelSession(t, cfg, productionSessionOps())
	freshUnit, _, freshCfg := capturedModelOwners(t)
	freshCfg.Channel = original
	if _, handle, err := capturedModelAdmission(t, freshCfg); err == nil || handle != nil {
		t.Fatalf("replayed channel admitted: %v", err)
	}
	capturedFileClosed(t, freshUnit.file)
	if got, err := owner.Counters(); err != nil || got.RxBytes != 123 {
		t.Fatalf("replay harmed original %+v, %v", got, err)
	}
}

func TestCapturedSessionReusedChannelNumberStillClosesFreshOwner(t *testing.T) {
	_, original, cfg := capturedModelOwners(t)
	owner := capturedModelSession(t, cfg, productionSessionOps())
	oldFD, err := originalFileDescriptor(original.file)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := os.CreateTemp(t.TempDir(), "fresh-reused-channel-")
	if err != nil {
		t.Fatal(err)
	}
	if int(file.Fd()) != oldFD {
		fd, err := unix.FcntlInt(file.Fd(), unix.F_DUPFD_CLOEXEC, oldFD)
		if err != nil {
			if closeErr := file.Close(); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			if closeErr := unix.Close(fd); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal(err)
		}
		file = os.NewFile(uintptr(fd), "fresh-reused-channel")
		if fd != oldFD {
			if closeErr := file.Close(); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatalf("fixture allocated %d instead of freed original %d", fd, oldFD)
		}
	}
	fresh := capturedModelChannel(t, file)
	bad := cfg
	bad.Channel, bad.ChanFD = fresh, oldFD
	if _, handle, err := capturedModelAdmission(t, bad); err == nil || handle != nil {
		t.Fatalf("replayed closed unit admitted: %v", err)
	}
	capturedFileClosed(t, fresh.file)
}

func TestCapturedSessionCopiedUnitCannotReleaseOriginal(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	copy := &CreatedUnit{self: unit, file: unit.file, namespace: unit.namespace,
		ready: unit.ready, backend: unit.backend, number: unit.number,
		identity: unit.identity, poll: unit.poll, readCounters: unit.readCounters}
	cfg.Unit = copy
	if _, handle, err := capturedModelAdmission(t, cfg); err == nil || handle != nil {
		t.Fatalf("copied unit admitted: %v", err)
	}
	capturedFileClosed(t, channel.file)
	if _, err := unit.file.Stat(); err != nil {
		t.Fatalf("copied unit closed original: %v", err)
	}
}

func TestCapturedSessionCopiedChannelCannotReleaseOriginal(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	copy := &CreatedChannel{self: channel, file: channel.file, namespace: channel.namespace,
		ready: channel.ready, poll: channel.poll}
	cfg.Channel = copy
	if _, handle, err := capturedModelAdmission(t, cfg); err == nil || handle != nil {
		t.Fatalf("copied channel admitted: %v", err)
	}
	capturedFileClosed(t, unit.file)
	if _, err := channel.file.Stat(); err != nil {
		t.Fatalf("copied channel closed original: %v", err)
	}
}

func TestCapturedSessionDeadUnitAdmissionClosesBothOwners(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	dead := errors.New("original unit POLLHUP before admission")
	unit.poll = func(*os.File) error { return dead }
	frames, handle, err := capturedModelAdmission(t, cfg)
	if !errors.Is(err, dead) || frames != nil || handle != nil {
		t.Fatalf("dead original admission: %v", err)
	}
	capturedFileClosed(t, unit.file)
	capturedFileClosed(t, channel.file)
}

func TestCapturedSessionCloseDependencyPanicKeepsExactCustody(t *testing.T) {
	for _, afterClose := range []bool{false, true} {
		name := "before-close"
		if afterClose {
			name = "after-close"
		}
		t.Run(name, func(t *testing.T) {
			unit, channel, cfg := capturedModelOwners(t)
			panicCause := errors.New("close-unit dependency panicked")
			ops := productionSessionOps()
			ops.preparePair = func(*claimedSessionPair) error { return nil }
			wrap := ops.wrapUnit
			var counted *capturedCountedStream
			ops.wrapUnit = func(lease sessionUnitLease) (io.ReadWriteCloser, error) {
				stream, err := wrap(lease)
				if err != nil {
					return stream, err
				}
				counted = &capturedCountedStream{ReadWriteCloser: stream}
				return counted, nil
			}
			ops.closeUnit = func(_ sessionUnitLease, stream io.ReadWriteCloser) error {
				if afterClose {
					if err := stream.Close(); err != nil {
						panic(errors.Join(panicCause, err))
					}
				}
				panic(panicCause)
			}
			_, handle, err := newSession(cfg, ops)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := handle.Close(); err != nil && !errors.Is(err, panicCause) {
					t.Error(err)
				}
			})
			if err := handle.Close(); !errors.Is(err, panicCause) {
				t.Fatalf("close cause lost: %v", err)
			}
			capturedFileClosed(t, unit.file)
			capturedFileClosed(t, channel.file)
			if counted == nil || counted.closes != 1 {
				t.Fatalf("physical unit wrapper closes=%v, want1", counted)
			}
			if err := handle.Close(); !errors.Is(err, panicCause) {
				t.Fatalf("cached close cause lost: %v", err)
			}
			if counted.closes != 1 {
				t.Fatalf("repeated Close repeated physical wrapper %d", counted.closes)
			}
		})
	}
}

type capturedCountedStream struct {
	io.ReadWriteCloser
	closes int
}

func (stream *capturedCountedStream) Close() error {
	stream.closes++
	return stream.ReadWriteCloser.Close()
}

func TestCapturedSessionBackendCannotChangeOriginalCreator(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	cfg.Backend = &sessionTestBackend{}
	if _, handle, err := capturedModelAdmission(t, cfg); err == nil || handle != nil {
		t.Fatalf("foreign backend admitted: %v", err)
	}
	capturedFileClosed(t, unit.file)
	capturedFileClosed(t, channel.file)
}

func TestCapturedSessionPartialUnitWrapperKeepsBothOriginalFiles(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	primary := errors.New("actual partial unit wrapper refused")
	cleanup := errors.New("actual unit wrapper closer panicked")
	ops := productionSessionOps()
	ops.preparePair = func(*claimedSessionPair) error { return nil }
	wrap := ops.wrapUnit
	var physical *interruptibleFDFile
	ops.wrapUnit = func(lease sessionUnitLease) (io.ReadWriteCloser, error) {
		stream, err := wrap(lease)
		if err != nil {
			return stream, err
		}
		var ok bool
		physical, ok = stream.(*interruptibleFDFile)
		if !ok {
			return stream, errors.New("unit wrapper lost actual physical stream")
		}
		physical.closeChannel = func() error { panic(cleanup) }
		return physical, primary
	}
	frames, handle, err := newSession(cfg, ops)
	if handle != nil {
		t.Cleanup(func() {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) || frames != nil || handle != nil {
		t.Fatalf("partial actual wrapper receipt: %v", err)
	}
	if !errors.Is(SetupCleanupFailure(err), cleanup) {
		t.Fatalf("partial cleanup classification lost: %v", err)
	}
	capturedFileClosed(t, unit.file)
	capturedFileClosed(t, channel.file)
	capturedFileClosed(t, unit.namespace.file)
	capturedFileClosed(t, channel.namespace.file)
	if physical == nil {
		t.Fatal("partial physical wrapper was not reached")
	}
	capturedFileClosed(t, physical.wake)
}

func TestCapturedSessionUnitWrapperPanicClosesActualChannel(t *testing.T) {
	unit, channel, cfg := capturedModelOwners(t)
	primary := errors.New("unit wrapper panicked after actual channel")
	cleanup := errors.New("actual channel closer panicked")
	ops := productionSessionOps()
	ops.preparePair = func(*claimedSessionPair) error { return nil }
	wrap := ops.wrapChannel
	var physical *interruptibleFDFile
	ops.wrapChannel = func(lease sessionChannelLease) (io.ReadWriteCloser, error) {
		stream, err := wrap(lease)
		if err != nil {
			return stream, err
		}
		var ok bool
		physical, ok = stream.(*interruptibleFDFile)
		if !ok {
			return stream, errors.New("channel wrapper lost actual physical stream")
		}
		physical.closeChannel = func() error { panic(cleanup) }
		return physical, nil
	}
	ops.wrapUnit = func(sessionUnitLease) (io.ReadWriteCloser, error) { panic(primary) }
	var failure any
	func() {
		defer func() { failure = recover() }()
		_, handle, _ := newSession(cfg, ops)
		if handle != nil {
			if err := handle.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
	err, ok := failure.(error)
	if !ok || !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("actual wrapper panic/cleanup causes: %v", failure)
	}
	if !errors.Is(SetupCleanupFailure(err), cleanup) {
		t.Fatalf("panic cleanup classification lost: %v", err)
	}
	capturedFileClosed(t, unit.file)
	capturedFileClosed(t, channel.file)
	capturedFileClosed(t, unit.namespace.file)
	capturedFileClosed(t, channel.namespace.file)
	if physical == nil {
		t.Fatal("actual channel wrapper was not reached")
	}
	capturedFileClosed(t, physical.wake)
}

func TestCapturedSessionDiscardsChangedOriginalReceipt(t *testing.T) {
	for _, change := range []string{"identity", "birth", "lease"} {
		t.Run(change, func(t *testing.T) {
			unit, _, cfg := capturedModelOwners(t)
			owner := capturedModelSession(t, cfg, productionSessionOps())
			entered, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(resume) }) }
			unit.readCounters = func() (ppp.SessionCounters, error) {
				close(entered)
				<-resume
				return ppp.SessionCounters{RxBytes: 123}, nil
			}
			type answer struct {
				counters ppp.SessionCounters
				err      error
			}
			done := make(chan answer, 1)
			joined := false
			t.Cleanup(func() {
				release()
				if !joined {
					select {
					case <-done:
					case <-time.After(3 * time.Second):
						t.Error("changed-receipt reader did not join")
					}
				}
			})
			go func() { counters, err := owner.Counters(); done <- answer{counters, err} }()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("original receipt read did not enter")
			}
			owner.mu.Lock()
			switch change {
			case "identity":
				owner.identity.NativeIndex++
			case "birth":
				owner.birthTag = "different-original-birth"
			case "lease":
				owner.unitLease = &sessionTestUnitLease{source: newSessionTestUnit(79, 8, owner.identity)}
			}
			owner.mu.Unlock()
			release()
			select {
			case got := <-done:
				joined = true
				if !errors.Is(got.err, io.ErrClosedPipe) || got.counters != (ppp.SessionCounters{}) {
					t.Fatalf("changed original receipt returned %+v, %v", got.counters, got.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("released receipt reader did not join")
			}
		})
	}
}
