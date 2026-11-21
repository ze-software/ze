// Design: docs/research/l2tpv2-ze-integration.md -- route ACK and original retirement
// Related: session_guarded_mutations_integration_linux_test.go -- real PPP writer

//go:build linux

package kernel

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// This is an explicit ACK scheduling port over regular files. It uses the real
// same-unit WithDescriptor guard only for the short submission, then delays the
// successful reply. It grants no PPP/native authority and performs no syscall
// from a descriptor label. The unchanged integration test supplies native proof.
type sessionRouteACKPort struct {
	nativeOperationsModel
	original    *CreatedUnit
	descriptor  int
	awaitAdd    func(context.Context) error
	awaitRemove func(context.Context) error
}

func (port *sessionRouteACKPort) AddRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	if err := port.original.WithDescriptor(func(fd int) error {
		if fd != port.descriptor {
			return errors.New("route model received a different original descriptor")
		}
		return port.sessionTestMutationOperations.AddRoute(ctx, destination, gateway, metric, protocol)
	}); err != nil {
		return err
	}
	if port.awaitAdd != nil {
		return port.awaitAdd(ctx)
	}
	return nil
}

func (port *sessionRouteACKPort) RemoveRoute(ctx context.Context, destination netip.Prefix, gateway netip.Addr, metric int, protocol rtproto.Proto) error {
	if err := port.original.WithDescriptor(func(fd int) error {
		if fd != port.descriptor {
			return errors.New("route model received a different original descriptor")
		}
		return port.sessionTestMutationOperations.RemoveRoute(ctx, destination, gateway, metric, protocol)
	}); err != nil {
		return err
	}
	if port.awaitRemove != nil {
		return port.awaitRemove(ctx)
	}
	return nil
}

func sessionRouteACKOwner(t *testing.T) (*session, *CreatedUnit, *routeOwnershipBackend, *sessionRouteACKPort) {
	t.Helper()
	unit, channel, _ := capturedModelOwners(t)
	backend := &routeOwnershipBackend{routes: make(map[routeOwnershipTuple]bool)}
	unit.backend = backend
	unit.identity.InterfaceName = "created-ppp"
	cfg, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatal(err)
	}
	ops := productionSessionOps()
	owner := capturedModelSession(t, cfg, ops)
	port := &sessionRouteACKPort{original: unit, descriptor: owner.unitFD}
	port.sessionTestMutationOperations = sessionTestMutationOperations{backend: backend, identity: unit.identity, ops: &ops}
	// Only actual same-unit physical retirement may settle this local model.
	port.settle = unit.WaitNativePPPUnitRetired
	unit.mu.Lock()
	unit.nativeOperations.ownership = nativeUnitOperationsOwned{operations: port}
	unit.mu.Unlock()
	owner.mutations = port
	return owner, unit, backend, port
}

// A delayed route reply must leave Close free to retire the actual files. Both
// directions preserve intent/admission until retirement, fence the old handle,
// and keep a fresh same-label owner's useful route untouched by late completion.
// MUTATION: keeping s.mu over either route writer call fails lock availability.
// MUTATION: omitting AddRoute's receipt or beginNativeMutation's token fails the
// pending-intent or overlapping-admission assertion before Close is requested.
func TestKernelSessionRouteACKRetirementKeepsOriginalOwner(t *testing.T) {
	for _, withdrawal := range []bool{false, true} {
		name := "add"
		if withdrawal {
			name = "withdrawal"
		}
		t.Run(name, func(t *testing.T) {
			owner, unit, backend, port := sessionRouteACKOwner(t)
			destination := netip.MustParsePrefix("2001:db8:1::/64")
			gateway := netip.MustParseAddr("fe80::2")
			route, err := newSessionRoute(destination, gateway, 7, rtproto.Static)
			if err != nil {
				t.Fatal(err)
			}
			if withdrawal {
				if err := owner.AddRoute(destination, gateway, 7, rtproto.Static); err != nil {
					t.Fatal(err)
				}
			}
			entered, resume := make(chan struct{}), make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(resume) }) }
			await := func(ctx context.Context) error {
				close(entered)
				select {
				case <-resume:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if withdrawal {
				port.awaitRemove = await
			} else {
				port.awaitAdd = await
			}
			done := make(chan error, 1)
			closed := make(chan error, 1)
			joined, closeStarted, closeJoined := false, false, false
			t.Cleanup(func() {
				release()
				if !joined {
					select {
					case <-done:
					case <-time.After(3 * time.Second):
						t.Error("released route worker did not join")
					}
				}
				if closeStarted {
					if !closeJoined {
						select {
						case <-closed:
						case <-time.After(3 * time.Second):
							t.Error("released Close worker did not join")
						}
					}
				}
			})
			go func() {
				if withdrawal {
					done <- owner.RemoveRoute(destination, gateway, 7, rtproto.Static)
					return
				}
				done <- owner.AddRoute(destination, gateway, 7, rtproto.Static)
			}()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("actual route method did not submit the original tuple")
			}
			if !owner.mu.TryLock() {
				t.Fatal("route ACK wait holds the session mutex")
			}
			_, retained := owner.routes[route]
			admitted := owner.nativeMutation
			owner.mu.Unlock()
			if !retained {
				t.Fatal("pending route lost its exact original intent")
			}
			if !admitted {
				t.Fatal("pending route lost its mutation admission token")
			}
			if !unit.mu.TryLock() {
				t.Fatal("route ACK wait holds the original unit mutex")
			}
			unit.mu.Unlock()
			if !owner.pair.mu.TryLock() {
				t.Fatal("route ACK wait holds the original pair mutex")
			}
			owner.pair.mu.Unlock()
			if !owner.closeMu.TryLock() {
				t.Fatal("route ACK wait holds the physical Close mutex")
			}
			owner.closeMu.Unlock()
			overlap := netip.MustParsePrefix("2001:db8:2::/64")
			if err := owner.RemoveRoute(overlap, gateway, 7, rtproto.Static); err == nil {
				t.Fatal("overlapping route bypassed the pending admission token")
			} else if err.Error() != "ppp: an original native mutation is in progress" {
				t.Fatalf("overlap failed for the wrong reason: %v", err)
			}
			if len(backend.adds) != 1 {
				t.Fatalf("overlap reached the backend: %v", backend.adds)
			}
			closeStarted = true
			go func() { closed <- owner.Close() }()
			select {
			case err := <-closed:
				closeJoined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("original physical retirement waited for the route ACK")
			}
			capturedFileClosed(t, unit.file)
			if err := unit.WaitNativePPPUnitRetired(t.Context()); err != nil {
				t.Fatal(err)
			}
			called := false
			if err := unit.WithDescriptor(func(int) error { called = true; return nil }); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("retired original guard returned %v, want os.ErrClosed", err)
			}
			if called {
				t.Fatal("retired original lent a descriptor to a late native operation")
			}
			successor, freshUnit, freshBackend, _ := sessionRouteACKOwner(t)
			if freshUnit == unit {
				t.Fatal("positive successor reused the original owner")
			}
			if successor.identity != owner.identity {
				t.Fatal("successor did not reuse the original diagnostic labels")
			}
			if err := successor.AddRoute(destination, gateway, 7, rtproto.Static); err != nil {
				t.Fatalf("fresh original owner could not add its useful route: %v", err)
			}
			release()
			select {
			case err := <-done:
				joined = true
				if err != nil {
					t.Fatalf("delayed successful original ACK changed: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("released original ACK did not join")
			}
			want := routeOwnershipTuple{"created-ppp", destination.String(), gateway.String(), 7, rtproto.Static}
			if backend.adds[0] != want {
				t.Fatalf("original add changed tuple: %+v", backend.adds)
			}
			removeCount := 0
			if withdrawal {
				removeCount = 1
			}
			if len(backend.removes) != removeCount {
				t.Fatalf("original withdrawals = %v, want %d", backend.removes, removeCount)
			}
			if withdrawal {
				if backend.removes[0] != want {
					t.Fatalf("original withdrawal changed tuple: %+v", backend.removes)
				}
			}
			if !freshBackend.routes[want] {
				t.Fatal("late original completion deleted the successor's useful route")
			}
			if len(freshBackend.adds) != 1 {
				t.Fatal("late original completion repeated an add on the successor")
			}
			if len(freshBackend.removes) != 0 {
				t.Fatal("late original completion withdrew on the successor backend")
			}
			if owner.backend != backend {
				t.Fatal("late original completion replaced its captured backend")
			}
			if len(owner.routes) != 0 {
				t.Fatal("late ACK resurrected retired original route intent")
			}
			for _, operation := range []func() error{
				func() error { return owner.AddRoute(destination, gateway, 7, rtproto.Static) },
				func() error { return owner.RemoveRoute(destination, gateway, 7, rtproto.Static) },
			} {
				if err := operation(); !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("retired handle was not fenced by io.ErrClosedPipe: %v", err)
				}
			}
			if len(backend.adds) != 1 {
				t.Fatal("fenced original handle submitted another route add")
			}
			if len(backend.removes) != removeCount {
				t.Fatal("fenced original handle submitted another route withdrawal")
			}
			if err := successor.RemoveRoute(destination, gateway, 7, rtproto.Static); err != nil {
				t.Fatalf("fresh original owner could not withdraw its own route: %v", err)
			}
			if len(freshBackend.removes) != 1 {
				t.Fatalf("successor withdrawals = %v, want its exact tuple once", freshBackend.removes)
			}
			if freshBackend.removes[0] != want {
				t.Fatalf("successor withdrawal changed tuple: %+v", freshBackend.removes)
			}
			if len(freshBackend.routes) != 0 {
				t.Fatal("successor's own successful withdrawal left its route")
			}
		})
	}
}
