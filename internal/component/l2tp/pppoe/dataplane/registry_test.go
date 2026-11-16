// Design: docs/architecture/l2tp/bng-5-pppoe.md -- session backend selection

package dataplane

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestSessionRegistrySelectsFactory proves exact selection by observing the
// request and context delivered to the selected factory, with another present.
func TestSessionRegistrySelectsFactory(t *testing.T) {
	var registry sessionRegistry
	ctx := context.WithValue(context.Background(), testContextKey{}, "selected")
	want := Request{AccessName: "access", Device: "eth0.10", Namespace: "blue",
		SessionID: 41, PeerMAC: [6]byte{2, 3, 4, 5, 6, 7}, CaptureProxyLCP: true}
	frames, handle, transport := testSessionResources()
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := registry.register("chosen", func(got context.Context, request Request) (OpenedSession, error) {
		calls++
		if got != ctx {
			t.Error("factory received another context")
		}
		if request != want {
			t.Errorf("request = %+v, want %+v", request, want)
		}
		return opened, nil
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	if err := registry.register("other", func(context.Context, Request) (OpenedSession, error) {
		t.Error("unselected factory called")
		return nil, errors.New("unselected")
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	if backend.Name() != "chosen" {
		t.Errorf("name = %q", backend.Name())
	}
	got, err := backend.Open(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if got != opened {
		t.Error("factory result replaced")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
	closeTestSession(t, got)
}

// TestSessionRegistryRefusesRegistration checks malformed names, nil factories
// and duplicate registrations without touching the process registry.
func TestSessionRegistryRefusesRegistration(t *testing.T) {
	for _, name := range []string{"", "bad name", "bad/name", "-bad", strings.Repeat("a", 64)} {
		t.Run(name, func(t *testing.T) {
			var registry sessionRegistry
			if err := registry.register(name, testUnusedOpen, testUnusedPrepare); err == nil {
				t.Fatalf("accepted name %q", name)
			}
		})
	}
	var registry sessionRegistry
	if err := registry.register("missing", nil, testUnusedPrepare); err == nil {
		t.Fatal("accepted nil factory")
	}
	if err := registry.register("chosen", testUnusedOpen, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	if err := registry.register("chosen", testUnusedOpen, testUnusedPrepare); err == nil {
		t.Fatal("accepted duplicate factory")
	}
}

// TestSessionRegistryUnknownNeverFallsBack proves a miss refuses without
// invoking any installed provider, and diagnostics derive the sorted names.
func TestSessionRegistryUnknownNeverFallsBack(t *testing.T) {
	var registry sessionRegistry
	for _, name := range []string{"zebra", "alpha"} {
		if err := registry.register(name, func(context.Context, Request) (OpenedSession, error) {
			t.Error("lookup called a provider")
			return nil, errors.New("unselected")
		}, testUnusedPrepare); err != nil {
			t.Fatal(err)
		}
	}
	backend, err := registry.lookup("absent")
	if backend != nil {
		t.Fatal("miss returned a backend")
	}
	if err == nil {
		t.Fatal("miss returned no error")
	}
	if !strings.Contains(err.Error(), "absent") {
		t.Errorf("error does not name selection: %v", err)
	}
	if !strings.Contains(err.Error(), "alpha, zebra") {
		t.Errorf("error does not derive available providers: %v", err)
	}
	if got := registry.names(); !reflect.DeepEqual(got, []string{"alpha", "zebra"}) {
		t.Errorf("names = %v", got)
	}
	registry.names()[0] = "changed"
	if got := registry.names()[0]; got != "alpha" {
		t.Errorf("caller mutated registry through names: %s", got)
	}
}

// TestSessionRegistryEmptyRefusesSelection proves a build with no provider has
// an explicit unavailable result rather than an empty or default backend.
func TestSessionRegistryEmptyRefusesSelection(t *testing.T) {
	var registry sessionRegistry
	backend, err := registry.lookup("chosen")
	if backend != nil {
		t.Fatal("empty registry returned a backend")
	}
	if err == nil {
		t.Fatal("empty registry accepted selection")
	}
	if !strings.Contains(err.Error(), "no session backends are registered") {
		t.Errorf("unclear empty registry error: %v", err)
	}
	if names := registry.names(); len(names) != 0 {
		t.Errorf("empty registry names = %v", names)
	}
}

// TestSessionBackendRefusesNilResult checks both nil interface forms at the
// factory boundary, where a successful error result cannot authorize PPP.
func TestSessionBackendRefusesNilResult(t *testing.T) {
	var typedNil *openedSession
	for _, result := range []OpenedSession{nil, typedNil} {
		var registry sessionRegistry
		if err := registry.register("empty", func(context.Context, Request) (OpenedSession, error) {
			return result, nil
		}, testUnusedPrepare); err != nil {
			t.Fatal(err)
		}
		backend, err := registry.lookup("empty")
		if err != nil {
			t.Fatal(err)
		}
		if opened, err := backend.Open(context.Background(), Request{}); err == nil {
			t.Fatalf("accepted empty factory result: %v", opened)
		}
	}
}

// TestSessionBackendCleansFailedResult proves ownership of a partial factory
// result is released and neither its original nor cleanup errors are lost.
func TestSessionBackendCleansFailedResult(t *testing.T) {
	frames, handle, transport := testSessionResources()
	factoryErr := errors.New("factory failed")
	handle.err = errors.New("handle close failed")
	transport.err = errors.New("transport close failed")
	opened, err := NewOpenedSession(frames, handle, transport)
	if err != nil {
		t.Fatal(err)
	}
	var registry sessionRegistry
	if err := registry.register("partial", func(context.Context, Request) (OpenedSession, error) {
		return opened, factoryErr
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("partial")
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Open(context.Background(), Request{})
	if got != nil {
		t.Fatal("failed open retained a session")
	}
	for _, want := range []error{factoryErr, handle.err, transport.err} {
		if !errors.Is(err, want) {
			t.Errorf("open error %v lost %v", err, want)
		}
	}
	if handle.closes != 1 {
		t.Errorf("handle closes = %d", handle.closes)
	}
	if transport.closes != 1 {
		t.Errorf("transport closes = %d", transport.closes)
	}
}

// TestSessionBackendKeepsFailureWithoutResult proves nil and typed-nil partial
// results preserve the original factory error without attempting cleanup.
func TestSessionBackendKeepsFailureWithoutResult(t *testing.T) {
	var typedNil *openedSession
	factoryErr := errors.New("open failed before resources")
	for _, result := range []OpenedSession{nil, typedNil} {
		var registry sessionRegistry
		if err := registry.register("failed", func(context.Context, Request) (OpenedSession, error) {
			return result, factoryErr
		}, testUnusedPrepare); err != nil {
			t.Fatal(err)
		}
		backend, err := registry.lookup("failed")
		if err != nil {
			t.Fatal(err)
		}
		opened, err := backend.Open(context.Background(), Request{})
		if opened != nil {
			t.Fatal("failure returned a session")
		}
		if !errors.Is(err, factoryErr) {
			t.Errorf("factory error lost: %v", err)
		}
	}
}

// TestSessionBackendRefusesNilContext proves an invalid caller context cannot
// reach the provider and create resources it cannot cancel.
func TestSessionBackendRefusesNilContext(t *testing.T) {
	var registry sessionRegistry
	if err := registry.register("chosen", func(context.Context, Request) (OpenedSession, error) {
		t.Fatal("factory called with nil context")
		return nil, errors.New("unreachable")
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Open(nil, Request{}); err == nil {
		t.Fatal("accepted nil context")
	}
}

// TestSessionBackendRefusesUnconstructedResult checks Go's interface-embedding
// loophole: inheriting the sealed marker cannot bypass the resource constructor.
func TestSessionBackendRefusesUnconstructedResult(t *testing.T) {
	_, handle, transport := testSessionResources()
	var registry sessionRegistry
	if err := registry.register("unconstructed", func(context.Context, Request) (OpenedSession, error) {
		return &testUnconstructed{handle: handle, transport: transport}, nil
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("unconstructed")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := backend.Open(context.Background(), Request{})
	if opened != nil {
		t.Fatal("accepted a session outside its constructor")
	}
	if err == nil {
		t.Fatal("unconstructed result returned no error")
	}
	if handle.closes != 1 {
		t.Errorf("partial handle closes = %d", handle.closes)
	}
	if transport.closes != 1 {
		t.Errorf("partial transport closes = %d", transport.closes)
	}
}

type testUnconstructed struct {
	OpenedSession
	handle    *testHandle
	transport *testCloser
}

func (s *testUnconstructed) Dataplane() ppp.SessionHandle { return s.handle }
func (s *testUnconstructed) CloseTransport() error        { return s.transport.Close() }

// TestSessionBackendRefusesBareEmbedding proves nil promoted cleanup methods
// cannot panic out of admission or hide the original factory error. Both
// ownership roles must be attempted and report their distinct cleanup failures.
func TestSessionBackendRefusesBareEmbedding(t *testing.T) {
	factoryErr := errors.New("factory failure with bare embedding")
	backend := testFailedResultBackend(t, &struct{ OpenedSession }{}, factoryErr)
	opened, err := backend.Open(context.Background(), Request{})
	if opened != nil {
		t.Fatal("bare embedding returned an admitted session")
	}
	if !errors.Is(err, factoryErr) {
		t.Errorf("factory failure lost: %v", err)
	}
	if !strings.Contains(err.Error(), "data-plane cleanup panicked") {
		t.Errorf("data-plane cleanup attempt missing: %v", err)
	}
	if !strings.Contains(err.Error(), "transport cleanup panicked") {
		t.Errorf("transport cleanup attempt missing: %v", err)
	}
}

// TestSessionBackendAccessorPanicStillClosesTransport proves an invalid result's
// panicking data-plane accessor does not prevent the other owner's cleanup.
func TestSessionBackendAccessorPanicStillClosesTransport(t *testing.T) {
	transport := &testCloser{}
	factoryErr := errors.New("factory failed before data-plane access")
	result := &testAccessorPanic{transport: transport}
	backend := testFailedResultBackend(t, result, factoryErr)
	opened, err := backend.Open(context.Background(), Request{})
	if opened != nil {
		t.Fatal("panicking accessor returned an admitted session")
	}
	if !errors.Is(err, factoryErr) {
		t.Errorf("factory failure lost: %v", err)
	}
	if !strings.Contains(err.Error(), "data-plane cleanup panicked: accessor panic") {
		t.Errorf("accessor panic missing: %v", err)
	}
	if transport.closes != 1 {
		t.Errorf("other owner closes = %d, want 1", transport.closes)
	}
}

// TestSessionBackendHandlePanicStillClosesTransport proves a reachable handle's
// panicking Close does not prevent the transport cleanup or lose either error.
func TestSessionBackendHandlePanicStillClosesTransport(t *testing.T) {
	transport := &testCloser{err: errors.New("transport cleanup failed")}
	handle := &testClosePanic{}
	factoryErr := errors.New("factory failed with panicking handle")
	result := &testCleanupResult{handle: handle, transport: transport}
	backend := testFailedResultBackend(t, result, factoryErr)
	opened, err := backend.Open(context.Background(), Request{})
	if opened != nil {
		t.Fatal("panicking handle returned an admitted session")
	}
	for _, want := range []error{factoryErr, transport.err} {
		if !errors.Is(err, want) {
			t.Errorf("error %v lost %v", err, want)
		}
	}
	if !strings.Contains(err.Error(), "data-plane cleanup panicked: handle close panic") {
		t.Errorf("handle panic missing: %v", err)
	}
	if handle.calls != 1 {
		t.Errorf("handle close attempts = %d, want 1", handle.calls)
	}
	if transport.closes != 1 {
		t.Errorf("other owner closes = %d, want 1", transport.closes)
	}
}

// TestSessionBackendTransportPanicKeepsHandleCleanup proves the transport's
// panic is returned alongside original and data-plane cleanup failures.
func TestSessionBackendTransportPanicKeepsHandleCleanup(t *testing.T) {
	handle := &testHandle{err: errors.New("data-plane cleanup failed")}
	transport := &testTransportPanic{}
	factoryErr := errors.New("factory failed with panicking transport")
	result := &testCleanupResult{handle: handle, transport: transport}
	backend := testFailedResultBackend(t, result, factoryErr)
	opened, err := backend.Open(context.Background(), Request{})
	if opened != nil {
		t.Fatal("panicking transport returned an admitted session")
	}
	for _, want := range []error{factoryErr, handle.err} {
		if !errors.Is(err, want) {
			t.Errorf("error %v lost %v", err, want)
		}
	}
	if !strings.Contains(err.Error(), "transport cleanup panicked: transport close panic") {
		t.Errorf("transport panic missing: %v", err)
	}
	if handle.closes != 1 {
		t.Errorf("other owner closes = %d, want 1", handle.closes)
	}
	if transport.calls != 1 {
		t.Errorf("transport close attempts = %d, want 1", transport.calls)
	}
}

func testFailedResultBackend(t *testing.T, result OpenedSession, failure error) Backend {
	t.Helper()
	var registry sessionRegistry
	if err := registry.register("invalid", func(context.Context, Request) (OpenedSession, error) {
		return result, failure
	}, testUnusedPrepare); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("invalid")
	if err != nil {
		t.Fatal(err)
	}
	return backend
}

type testAccessorPanic struct {
	OpenedSession
	transport *testCloser
}

func (*testAccessorPanic) Dataplane() ppp.SessionHandle { panic("accessor panic") }
func (s *testAccessorPanic) CloseTransport() error      { return s.transport.Close() }

type testCleanupResult struct {
	OpenedSession
	handle    ppp.SessionHandle
	transport io.Closer
}

func (s *testCleanupResult) Dataplane() ppp.SessionHandle { return s.handle }
func (s *testCleanupResult) CloseTransport() error        { return s.transport.Close() }

type testClosePanic struct {
	ppp.SessionHandle
	calls int
}

func (h *testClosePanic) Close() error {
	h.calls++
	panic("handle close panic")
}

type testTransportPanic struct{ calls int }

func (t *testTransportPanic) Close() error {
	t.calls++
	panic("transport close panic")
}

type testContextKey struct{}

func testUnusedOpen(context.Context, Request) (OpenedSession, error) {
	return nil, errors.New("unused factory")
}
