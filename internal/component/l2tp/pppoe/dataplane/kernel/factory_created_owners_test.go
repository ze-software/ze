// Design: docs/architecture/l2tp/bng-5-pppoe.md -- factory original-owner fault boundaries

package kernel

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestFactoryCreatorRefusalPrecedesTransport proves original backend capability
// refusal, including a nil success, reaches neither socket nor typed setup.
func TestFactoryCreatorRefusalPrecedesTransport(t *testing.T) {
	want := errors.New("original backend has no creation reader")
	for _, failure := range []error{want, nil} {
		f := newFactoryFixture()
		f.ops.creator = func(backend sessionBackend) (*pppkernel.UnitCreator, error) {
			if backend != f.backend {
				t.Fatal("creator received another backend")
			}
			return nil, failure
		}
		opened, err := openWithOps(t.Context(), factoryRequest(), f.ops)
		if opened != nil {
			t.Fatal("refused creator published a session")
		}
		if err == nil {
			t.Fatal("missing creator admitted native allocation")
		}
		if failure != nil {
			if !errors.Is(err, failure) {
				t.Fatalf("creator cause lost: %v", err)
			}
		}
		if !reflect.DeepEqual(f.calls, []string{"backend"}) {
			t.Fatalf("refused creator reached %v", f.calls)
		}
		if f.transportCloses != 0 {
			t.Fatal("refusal allocated a transport")
		}
	}
}

// TestFactoryConfigFailureClosesOriginalOwners proves non-consuming snapshot
// errors and panics retain exact unit/channel/transport custody and order.
func TestFactoryConfigFailureClosesOriginalOwners(t *testing.T) {
	for _, panics := range []bool{false, true} {
		f := newFactoryFixture()
		want := errors.New("original configuration failure")
		cleanup := errors.New("original unit close failed")
		var closes []string
		f.ops.config = func(unit *pppkernel.CreatedUnit, channel *pppkernel.CreatedChannel) (pppkernel.SessionConfig, error) {
			if unit != f.unit {
				t.Fatal("changed original unit")
			}
			if channel != f.channel {
				t.Fatal("changed original channel")
			}
			if panics {
				panic(want)
			}
			return pppkernel.SessionConfig{}, want
		}
		f.ops.closeUnit = func(unit *pppkernel.CreatedUnit) error {
			if unit != f.unit {
				t.Fatal("closed another unit")
			}
			closes = append(closes, "unit")
			return cleanup
		}
		f.ops.closeChannel = func(channel *pppkernel.CreatedChannel) error {
			if channel != f.channel {
				t.Fatal("closed another channel")
			}
			closes = append(closes, "channel")
			return nil
		}
		original := f.ops.openTransport
		f.ops.openTransport = func(ctx context.Context, request dataplane.Request) (*pppoxTransport, error) {
			transport, err := original(ctx, request)
			transport.closeFD = func(int) error { closes = append(closes, "transport"); return nil }
			return transport, err
		}
		var observed error
		var recovered error
		returned := false
		func() {
			defer func() {
				if failure := recover(); failure != nil {
					var ok bool
					recovered, ok = failure.(error)
					if !ok {
						t.Fatalf("panic payload changed: %v", failure)
					}
				}
			}()
			opened, err := openWithOps(t.Context(), factoryRequest(), f.ops)
			returned = true
			if opened != nil {
				t.Fatal("failed configuration published a session")
			}
			observed = err
		}()
		if panics {
			if returned {
				t.Fatal("configuration panic became an ordinary return")
			}
			if recovered == nil {
				t.Fatal("configuration panic did not propagate")
			}
			observed = recovered
		} else {
			if !returned {
				t.Fatal("configuration error did not return normally")
			}
			if recovered != nil {
				t.Fatalf("configuration error became a panic: %v", recovered)
			}
		}
		if !errors.Is(observed, want) {
			t.Fatalf("configuration cause lost: %v", observed)
		}
		if !errors.Is(observed, cleanup) {
			t.Fatalf("cleanup cause lost: %v", observed)
		}
		if !reflect.DeepEqual(closes, []string{"unit", "channel", "transport"}) {
			t.Fatalf("original cleanup order: %v", closes)
		}
		if f.handle.closes != 0 {
			t.Fatal("snapshot failure constructed a session")
		}
	}
}

// TestFactoryPartialSetupCleanupSurvivesClosePanic proves every returned partial
// owner remains reachable when its sibling's private cleanup operation panics.
func TestFactoryPartialSetupCleanupSurvivesClosePanic(t *testing.T) {
	f := newFactoryFixture()
	want := errors.New("typed setup failed")
	cleanup := errors.New("unit cleanup panicked")
	f.ops.setup = func(*pppkernel.UnitCreator, *os.File, string) (*pppkernel.CreatedChannel, *pppkernel.CreatedUnit, error) {
		return f.channel, f.unit, want
	}
	f.ops.closeUnit = func(*pppkernel.CreatedUnit) error { f.unitCloses++; panic(cleanup) }
	var opened dataplane.OpenedSession
	var err error
	var escaped any
	func() {
		defer func() { escaped = recover() }()
		opened, err = openWithOps(t.Context(), factoryRequest(), f.ops)
	}()
	if escaped != nil {
		t.Fatalf("original cleanup panic escaped instead of returning all causes: %v", escaped)
	}
	if opened != nil {
		t.Fatal("partial typed setup published a session")
	}
	if !errors.Is(err, want) {
		t.Fatalf("setup cause lost: %v", err)
	}
	if !errors.Is(err, cleanup) {
		t.Fatalf("cleanup panic lost: %v", err)
	}
	if f.unitCloses != 1 {
		t.Fatalf("unit closes=%d", f.unitCloses)
	}
	if f.channelCloses != 1 {
		t.Fatalf("channel closes=%d", f.channelCloses)
	}
	if f.transportCloses != 1 {
		t.Fatalf("transport closes=%d", f.transportCloses)
	}
}

// TestFactoryPartialTransportClosesOnError proves a returned unpublished socket
// is retained even beside an error, before setup can return any typed owner.
func TestFactoryPartialTransportClosesOnError(t *testing.T) {
	f := newFactoryFixture()
	want := errors.New("socket publication failed")
	cleanup := errors.New("partial socket close failed")
	f.ops.openTransport = func(context.Context, dataplane.Request) (*pppoxTransport, error) {
		return &pppoxTransport{fd: 41, closeFD: func(int) error { f.transportCloses++; return cleanup }}, want
	}
	opened, err := openWithOps(t.Context(), factoryRequest(), f.ops)
	if opened != nil {
		t.Fatal("partial socket published a session")
	}
	if !errors.Is(err, want) {
		t.Fatalf("socket cause lost: %v", err)
	}
	if !errors.Is(err, cleanup) {
		t.Fatalf("cleanup cause lost: %v", err)
	}
	if f.transportCloses != 1 {
		t.Fatalf("partial socket closes=%d", f.transportCloses)
	}
	if f.channelCloses != 0 {
		t.Fatal("partial socket reached typed channel setup")
	}
	if f.unitCloses != 0 {
		t.Fatal("partial socket reached typed unit setup")
	}
}

// TestPreparedKernelCreatorRefusalClosesBackend proves capability preflight
// happens over the original acquired owner before resolution or publication.
func TestPreparedKernelCreatorRefusalClosesBackend(t *testing.T) {
	backend := &preparedOwnerBackend{}
	want := errors.New("no original creation counter capability")
	resolutions := 0
	access, err := prepareWithCreator(t.Context(), "logical", func() (iface.Backend, error) { return backend, nil },
		func(string) (iface.Binding, error) { resolutions++; return preparedOwnerBinding(), nil },
		func(actual pppkernel.InterfaceOperations) (*pppkernel.UnitCreator, error) {
			if actual != backend {
				t.Fatal("creator preflight switched the backend")
			}
			return nil, want
		})
	if access != nil {
		t.Fatal("unsupported original creator published access")
	}
	if !errors.Is(err, want) {
		t.Fatalf("creator refusal lost: %v", err)
	}
	if resolutions != 0 {
		t.Fatal("unsupported creator reached resolution")
	}
	if backend.closes != 1 {
		t.Fatalf("unsupported backend closes=%d", backend.closes)
	}
}
