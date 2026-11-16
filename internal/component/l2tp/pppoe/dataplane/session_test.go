// Design: docs/architecture/l2tp/bng-5-pppoe.md -- admitted session ownership

package dataplane

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestOpenedSessionBorrowsFrames checks that construction preserves the
// admitted endpoints and handle without closing any successful resource.
func TestOpenedSessionBorrowsFrames(t *testing.T) {
	frames, handle, transport := testSessionResources()
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	if opened.Dataplane() != handle {
		t.Error("data-plane owner changed")
	}
	if opened.Frames().Control() != frames.control {
		t.Error("control endpoint changed")
	}
	if opened.Frames().Opened() != nil {
		t.Error("invented an optional Opened reader")
	}
	if handle.closes != 0 {
		t.Error("construction closed the handle")
	}
	if transport.closes != 0 {
		t.Error("construction closed the transport")
	}
	closeTestSession(t, opened)
}

// TestOpenedSessionRetainsValidatedEndpoints proves a provider cannot replace
// the admitted streams with invalid values after construction. Interruption
// still reaches the actual channel owner and preserves its error.
func TestOpenedSessionRetainsValidatedEndpoints(t *testing.T) {
	frames, handle, transport := testSessionResources()
	control := frames.control
	openedReader := &bytes.Buffer{}
	frames.opened = openedReader
	frames.interruptErr = errors.New("interrupt failed")
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	frames.control = nil
	frames.opened = nil
	if opened.Frames().Control() != control {
		t.Error("validated control endpoint changed")
	}
	if opened.Frames().Opened() != openedReader {
		t.Error("validated Opened endpoint changed")
	}
	if err := opened.Frames().InterruptRead(); !errors.Is(err, frames.interruptErr) {
		t.Errorf("interrupt error lost: %v", err)
	}
	closeTestSession(t, opened)
}

// TestOpenedSessionRejectsPartialResources checks every invalid ownership
// boundary, including typed nils, and releases both supplied owners on failure.
func TestOpenedSessionRejectsPartialResources(t *testing.T) {
	tests := []struct {
		name   string
		change func(*testFrames, *ppp.FrameChannel, *ppp.SessionHandle, *io.Closer)
	}{
		{"nil frames", func(_ *testFrames, frames *ppp.FrameChannel, _ *ppp.SessionHandle, _ *io.Closer) { *frames = nil }},
		{"typed nil frames", func(_ *testFrames, frames *ppp.FrameChannel, _ *ppp.SessionHandle, _ *io.Closer) {
			*frames = (*testFrames)(nil)
		}},
		{"nil control", func(frames *testFrames, _ *ppp.FrameChannel, _ *ppp.SessionHandle, _ *io.Closer) {
			frames.control = nil
		}},
		{"typed nil control", func(frames *testFrames, _ *ppp.FrameChannel, _ *ppp.SessionHandle, _ *io.Closer) {
			frames.control = (*bytes.Buffer)(nil)
		}},
		{"typed nil opened", func(frames *testFrames, _ *ppp.FrameChannel, _ *ppp.SessionHandle, _ *io.Closer) {
			frames.opened = (*bytes.Buffer)(nil)
		}},
		{"nil handle", func(_ *testFrames, _ *ppp.FrameChannel, handle *ppp.SessionHandle, _ *io.Closer) { *handle = nil }},
		{"typed nil handle", func(_ *testFrames, _ *ppp.FrameChannel, handle *ppp.SessionHandle, _ *io.Closer) {
			*handle = (*testHandle)(nil)
		}},
		{"nil transport", func(_ *testFrames, _ *ppp.FrameChannel, _ *ppp.SessionHandle, transport *io.Closer) { *transport = nil }},
		{"typed nil transport", func(_ *testFrames, _ *ppp.FrameChannel, _ *ppp.SessionHandle, transport *io.Closer) {
			*transport = (*testCloser)(nil)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			frames, handle, transport := testSessionResources()
			var suppliedFrames ppp.FrameChannel = frames
			var suppliedHandle ppp.SessionHandle = handle
			var suppliedTransport io.Closer = transport
			test.change(frames, &suppliedFrames, &suppliedHandle, &suppliedTransport)
			opened, err := NewOpenedSession(suppliedFrames, suppliedHandle, suppliedTransport)
			if err == nil {
				t.Fatal("accepted partial resources")
			}
			if opened != nil {
				t.Fatal("failed construction returned a session")
			}
			wantHandleCloses := 1
			if suppliedHandle != handle {
				wantHandleCloses = 0
			}
			if handle.closes != wantHandleCloses {
				t.Errorf("handle closes = %d, want %d", handle.closes, wantHandleCloses)
			}
			wantTransportCloses := 1
			if suppliedTransport != transport {
				wantTransportCloses = 0
			}
			if transport.closes != wantTransportCloses {
				t.Errorf("transport closes = %d, want %d", transport.closes, wantTransportCloses)
			}
		})
	}
}

// TestOpenedSessionCloseTransportOnce proves repeated closure returns the
// original failure and invokes the separate transport owner only once.
func TestOpenedSessionCloseTransportOnce(t *testing.T) {
	frames, handle, transport := testSessionResources()
	transport.err = errors.New("transport close failed")
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := opened.CloseTransport(); !errors.Is(err, transport.err) {
			t.Errorf("close = %v, want %v", err, transport.err)
		}
	}
	if transport.closes != 1 {
		t.Errorf("transport closes = %d, want 1", transport.closes)
	}
	if handle.closes != 0 {
		t.Error("transport closure closed the separate data-plane owner")
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestOpenedSessionFailureKeepsCloseErrors proves validation cannot hide either
// resource's close failure behind its own refusal.
func TestOpenedSessionFailureKeepsCloseErrors(t *testing.T) {
	_, handle, transport := testSessionResources()
	handle.err = errors.New("handle close failed")
	transport.err = errors.New("transport close failed")
	_, err := NewOpenedSession(nil, handle, transport)
	for _, want := range []error{handle.err, transport.err} {
		if !errors.Is(err, want) {
			t.Errorf("construction error %v lost %v", err, want)
		}
	}
}

// TestOpenedSessionSharedOwner checks that an idempotent backend owning both
// roles releases its resource once on construction failure or normal teardown.
func TestOpenedSessionSharedOwner(t *testing.T) {
	for _, valid := range []bool{false, true} {
		owner := &testHandle{}
		var frames ppp.FrameChannel
		if valid {
			frames = &testFrames{control: &bytes.Buffer{}}
		}
		opened, err := NewOpenedSession(frames, owner, owner)
		if valid {
			if err != nil {
				t.Fatal(err)
			}
			closeTestSession(t, opened)
			closeTestSession(t, opened)
		} else if err == nil {
			t.Fatal("accepted missing frames")
		}
		if owner.closes != 1 {
			t.Errorf("valid %v: releases = %d, want 1", valid, owner.closes)
		}
	}
}

// TestOpenedSessionControlPanicConsumesOwners injects the first frame accessor
// panic and checks that rollback preserves it while releasing both owners.
func TestOpenedSessionControlPanicConsumesOwners(t *testing.T) {
	testOpenedSessionAccessorPanic(t, false)
}

// TestOpenedSessionOpenedPanicConsumesOwners injects the second frame accessor
// panic after a valid control stream, checking the same ownership boundary.
func TestOpenedSessionOpenedPanicConsumesOwners(t *testing.T) {
	testOpenedSessionAccessorPanic(t, true)
}

func testOpenedSessionAccessorPanic(t *testing.T, openedAccessor bool) {
	t.Helper()
	primary := errors.New("frame accessor panic")
	cleanup := errors.New("owner close failed")
	for _, payload := range []any{primary, "frame accessor payload"} {
		for _, cleanupFails := range []bool{false, true} {
			handleClose := &testConstructorCloser{}
			transport := &testConstructorCloser{}
			if cleanupFails {
				handleClose.err = cleanup
				transport.err = cleanup
			}
			handle := &testConstructorHandle{closer: handleClose}
			frames := &testConstructorFrames{failure: payload, openedAccessor: openedAccessor}
			var result OpenedSession
			var recovered any
			returned := false
			func() {
				defer func() { recovered = recover() }()
				var err error
				result, err = NewOpenedSession(frames, handle, transport)
				returned = true
				t.Errorf("accessor panic returned: %v", err)
			}()
			if returned || result != nil {
				t.Fatalf("panicked construction published result=%v returned=%v", result, returned)
			}
			if handleClose.calls != 1 || transport.calls != 1 {
				t.Fatalf("rollback closes: handle=%d transport=%d", handleClose.calls, transport.calls)
			}
			if !cleanupFails {
				if recovered != payload {
					t.Fatalf("original panic changed: got=%v want=%v", recovered, payload)
				}
				continue
			}
			failure, ok := recovered.(error)
			if !ok || !errors.Is(failure, cleanup) {
				t.Fatalf("cleanup failure lost: %v", recovered)
			}
			if payload == primary {
				if !errors.Is(failure, primary) {
					t.Fatalf("original error panic lost: %v", failure)
				}
			} else if !strings.Contains(failure.Error(), payload.(string)) {
				t.Fatalf("original panic payload lost: %v", failure)
			}
		}
	}
}

// TestOpenedSessionHandleClosePanicStillClosesTransport makes rejection cleanup
// panic in the first owner and checks that the second owner is still released.
func TestOpenedSessionHandleClosePanicStillClosesTransport(t *testing.T) {
	primary := errors.New("handle close panic")
	cleanup := errors.New("transport close failed")
	handleClose := &testConstructorCloser{failure: primary}
	transport := &testConstructorCloser{err: cleanup}
	result, err := NewOpenedSession(nil, &testConstructorHandle{closer: handleClose}, transport)
	if result != nil || err == nil {
		t.Fatalf("rejection result=%v error=%v", result, err)
	}
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("rejection lost cleanup failures: %v", err)
	}
	if !strings.Contains(err.Error(), "no frame channel") {
		t.Fatalf("rejection lost validation failure: %v", err)
	}
	if handleClose.calls != 1 || transport.calls != 1 {
		t.Fatalf("rollback closes: handle=%d transport=%d", handleClose.calls, transport.calls)
	}
}

// TestOpenedSessionTransportClosePanicKeepsHandleCleanup checks that the second
// owner's panic cannot replace either the refusal or the first close failure.
func TestOpenedSessionTransportClosePanicKeepsHandleCleanup(t *testing.T) {
	primary := errors.New("transport close panic")
	cleanup := errors.New("handle close failed")
	handleClose := &testConstructorCloser{err: cleanup}
	transport := &testConstructorCloser{failure: primary}
	result, err := NewOpenedSession(nil, &testConstructorHandle{closer: handleClose}, transport)
	if result != nil || err == nil {
		t.Fatalf("rejection result=%v error=%v", result, err)
	}
	if !errors.Is(err, primary) || !errors.Is(err, cleanup) {
		t.Fatalf("rejection lost cleanup failures: %v", err)
	}
	if !strings.Contains(err.Error(), "no frame channel") {
		t.Fatalf("rejection lost validation failure: %v", err)
	}
	if handleClose.calls != 1 || transport.calls != 1 {
		t.Fatalf("rollback closes: handle=%d transport=%d", handleClose.calls, transport.calls)
	}
}

// TestOpenedSessionRejectsNoncomparableHandle checks the actual dynamic value,
// including a comparable struct type whose interface field holds a slice.
func TestOpenedSessionRejectsNoncomparableHandle(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		frames, handle, transport := testSessionResources()
		var supplied ppp.SessionHandle = testSliceSessionHandle{testHandle: handle, values: []byte{1}}
		if indirect {
			supplied = testInterfaceSessionHandle{testHandle: handle, value: []byte{1}}
		}
		opened, err := NewOpenedSession(frames, supplied, transport)
		if err == nil {
			t.Fatalf("indirect %v: accepted a noncomparable handle", indirect)
		}
		if opened != nil {
			t.Fatalf("indirect %v: rejected handle published a session", indirect)
		}
		if handle.closes != 1 || transport.closes != 1 {
			t.Fatalf("indirect %v: rejection closes handle=%d transport=%d", indirect, handle.closes, transport.closes)
		}
	}
}

type testSliceSessionHandle struct {
	*testHandle
	values []byte
}

type testInterfaceSessionHandle struct {
	*testHandle
	value any
}

// TestOpenedSessionPublicationEndsRollback uses distinct close counters to prove
// publication leaves both owners live until their respective explicit closes.
func TestOpenedSessionPublicationEndsRollback(t *testing.T) {
	handleClose := &testConstructorCloser{}
	transport := &testConstructorCloser{}
	frames := &testFrames{control: &bytes.Buffer{}}
	opened, err := NewOpenedSession(frames, &testConstructorHandle{closer: handleClose}, transport)
	if err != nil {
		t.Fatal(err)
	}
	if handleClose.calls != 0 || transport.calls != 0 {
		t.Fatalf("published owners rolled back: handle=%d transport=%d", handleClose.calls, transport.calls)
	}
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if handleClose.calls != 1 || transport.calls != 0 {
		t.Fatalf("separate handle close: handle=%d transport=%d", handleClose.calls, transport.calls)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
	if handleClose.calls != 1 || transport.calls != 1 {
		t.Fatalf("explicit owner closes: handle=%d transport=%d", handleClose.calls, transport.calls)
	}
}

type testConstructorFrames struct {
	ppp.FrameChannel
	failure        any
	openedAccessor bool
}

func (f *testConstructorFrames) Control() io.ReadWriter {
	if !f.openedAccessor {
		panic(f.failure)
	}
	return &bytes.Buffer{}
}

func (f *testConstructorFrames) Opened() io.Reader { panic(f.failure) }

type testConstructorHandle struct {
	ppp.SessionHandle
	closer *testConstructorCloser
}

func (h *testConstructorHandle) Close() error { return h.closer.Close() }

type testConstructorCloser struct {
	calls   int
	failure any
	err     error
}

func (c *testConstructorCloser) Close() error {
	c.calls++
	if c.failure != nil {
		panic(c.failure)
	}
	return c.err
}

// TestOpenedSessionRelayDescriptor borrows the owner's actual descriptor and
// proves closure refuses later borrowing without calling the owner again.
func TestOpenedSessionRelayDescriptor(t *testing.T) {
	frames, handle, _ := testSessionResources()
	transport := &testRelayTransport{descriptor: 37}
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor, err := opened.RelayDescriptor(); err != nil {
		t.Fatal(err)
	} else if descriptor != 37 {
		t.Errorf("descriptor = %d, want 37", descriptor)
	}
	if transport.closes != 0 {
		t.Fatal("borrowing closed the owner")
	}
	closeTestSession(t, opened)
	if _, err := opened.RelayDescriptor(); err == nil {
		t.Fatal("borrowed a descriptor after close")
	}
	if transport.borrows != 1 {
		t.Errorf("owner borrows = %d, want 1", transport.borrows)
	}
}

// TestOpenedSessionRefusesUnsupportedRelay proves a provider without the relay
// capability cannot supply a guessed descriptor or lose its normal ownership.
func TestOpenedSessionRefusesUnsupportedRelay(t *testing.T) {
	frames, handle, transport := testSessionResources()
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.RelayDescriptor(); err == nil {
		t.Fatal("unsupported transport supplied a relay descriptor")
	}
	if transport.closes != 0 {
		t.Fatal("unsupported borrowing closed the transport")
	}
	closeTestSession(t, opened)
}

// TestOpenedSessionRelayKeepsOwnerFailure proves transport errors reach the
// relay caller and an invalid negative descriptor is never accepted as success.
func TestOpenedSessionRelayKeepsOwnerFailure(t *testing.T) {
	ownerErr := errors.New("relay unavailable")
	for _, test := range []struct {
		descriptor int
		err        error
	}{{37, ownerErr}, {-1, nil}} {
		frames, handle, _ := testSessionResources()
		transport := &testRelayTransport{descriptor: test.descriptor, descriptorErr: test.err}
		opened, err := NewOpenedSession(frames, handle, transport)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := opened.RelayDescriptor(); err == nil {
			t.Fatal("accepted unavailable descriptor")
		} else if test.err != nil {
			if !errors.Is(err, test.err) {
				t.Errorf("owner error lost: %v", err)
			}
		}
		closeTestSession(t, opened)
	}
}

// TestOpenedSessionRelaySerializesClose holds the owner at its borrowing
// boundary, checks the close mutex is held, and joins both lifecycle helpers.
func TestOpenedSessionRelaySerializesClose(t *testing.T) {
	frames, handle, _ := testSessionResources()
	transport := &testRelayTransport{descriptor: 37, entered: make(chan struct{}), release: make(chan struct{})}
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	borrowDone := make(chan error, 1)
	go func() {
		_, err := opened.RelayDescriptor()
		borrowDone <- err
	}()
	<-transport.entered
	if opened.(*openedSession).transportMu.TryLock() {
		opened.(*openedSession).transportMu.Unlock()
		t.Error("descriptor borrowing did not exclude transport close")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- opened.CloseTransport() }()
	close(transport.release)
	if err := <-borrowDone; err != nil {
		t.Error(err)
	}
	if err := <-closeDone; err != nil {
		t.Error(err)
	}
	if transport.closes != 1 {
		t.Errorf("transport closes = %d, want 1", transport.closes)
	}
	if err := handle.Close(); err != nil {
		t.Fatal(err)
	}
}

type testRelayTransport struct {
	testCloser
	descriptor    int
	descriptorErr error
	borrows       int
	entered       chan struct{}
	release       chan struct{}
}

func (r *testRelayTransport) RelayDescriptor() (int, error) {
	r.borrows++
	if r.entered != nil {
		close(r.entered)
		<-r.release
	}
	return r.descriptor, r.descriptorErr
}

type testFrames struct {
	control      io.ReadWriter
	opened       io.Reader
	interruptErr error
}

func (f *testFrames) Control() io.ReadWriter { return f.control }
func (f *testFrames) Opened() io.Reader      { return f.opened }
func (f *testFrames) InterruptRead() error   { return f.interruptErr }

type testHandle struct {
	ppp.SessionHandle
	once   sync.Once
	closes int
	err    error
}

func (h *testHandle) Close() error {
	h.once.Do(func() { h.closes++ })
	return h.err
}

type testCloser struct {
	closes int
	err    error
}

func (c *testCloser) Close() error {
	c.closes++
	return c.err
}

func testSessionResources() (*testFrames, *testHandle, *testCloser) {
	return &testFrames{control: &bytes.Buffer{}}, &testHandle{}, &testCloser{}
}

func closeTestSession(t *testing.T, opened OpenedSession) {
	t.Helper()
	if err := opened.Dataplane().Close(); err != nil {
		t.Fatal(err)
	}
	if err := opened.CloseTransport(); err != nil {
		t.Fatal(err)
	}
}
