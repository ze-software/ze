// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared access ownership

package dataplane

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func testAccessInfo() AccessInfo {
	return AccessInfo{Device: "dp0", ControlDevice: "cp0", Namespace: "dataplane",
		DiscoveryIfIndex: 12, HWAddr: [6]byte{2, 3, 4, 5, 6, 7}, MTU: 1500}
}

type accessTestOwner struct {
	calls int
	err   error
}

func (o *accessTestOwner) Close() error { o.calls++; return o.err }

// TestAccessOwnsImmutableBinding proves admitted metadata is copied and the
// provider's resource lease has exactly one close outcome under concurrency.
func TestAccessOwnsImmutableBinding(t *testing.T) {
	info := testAccessInfo()
	want := info
	owner := &accessTestOwner{err: errors.New("map removal failed")}
	access, err := NewAccess(info, owner)
	if err != nil {
		t.Fatal(err)
	}
	info.Device = "changed"
	if access.Device() != want.Device {
		t.Fatal("session device changed")
	}
	if access.ControlDevice() != want.ControlDevice {
		t.Fatal("control device changed")
	}
	if access.Namespace() != want.Namespace {
		t.Fatal("namespace changed")
	}
	if access.DiscoveryIfIndex() != want.DiscoveryIfIndex {
		t.Fatal("discovery index changed")
	}
	if access.HWAddr() != want.HWAddr {
		t.Fatal("hardware address changed")
	}
	if access.MTU() != want.MTU {
		t.Fatal("MTU changed")
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			if !errors.Is(access.Close(), owner.err) {
				t.Error("lost first close result")
			}
		})
	}
	workers.Wait()
	if owner.calls != 1 {
		t.Fatalf("owner close calls=%d", owner.calls)
	}
}

// TestAccessRefusalConsumesOwner changes each required identity independently;
// every refusal closes the unpublished lease and preserves its cleanup error.
func TestAccessRefusalConsumesOwner(t *testing.T) {
	cases := map[string]func(*AccessInfo){
		"device":           func(i *AccessInfo) { i.Device = "" },
		"device NUL":       func(i *AccessInfo) { i.Device = "dp\x00bad" },
		"control":          func(i *AccessInfo) { i.ControlDevice = "" },
		"control length":   func(i *AccessInfo) { i.ControlDevice = strings.Repeat("x", 16) },
		"control NUL":      func(i *AccessInfo) { i.ControlDevice = "cp\x00bad" },
		"namespace path":   func(i *AccessInfo) { i.Namespace = "../blue" },
		"namespace space":  func(i *AccessInfo) { i.Namespace = "blue green" },
		"namespace length": func(i *AccessInfo) { i.Namespace = strings.Repeat("x", 256) },
		"index zero":       func(i *AccessInfo) { i.DiscoveryIfIndex = 0 },
		"index negative":   func(i *AccessInfo) { i.DiscoveryIfIndex = -1 },
		"MAC absent":       func(i *AccessInfo) { i.HWAddr = [6]byte{} },
		"MAC multicast":    func(i *AccessInfo) { i.HWAddr[0] = 1 },
		"MTU":              func(i *AccessInfo) { i.MTU = 0 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			info := testAccessInfo()
			change(&info)
			owner := &accessTestOwner{err: errors.New("cleanup failed")}
			access, err := NewAccess(info, owner)
			if access != nil {
				t.Fatal("invalid binding published")
			}
			if !errors.Is(err, owner.err) {
				t.Fatalf("cleanup error lost: %v", err)
			}
			if owner.calls != 1 {
				t.Fatalf("owner closes=%d", owner.calls)
			}
		})
	}
}

// TestAccessRequiresOwner refuses nil interfaces and typed nils before an
// access can escape without a defined resource lifetime.
func TestAccessRequiresOwner(t *testing.T) {
	var owner *accessTestOwner
	if access, err := NewAccess(testAccessInfo(), nil); err == nil || access != nil {
		t.Fatal("accepted missing owner")
	}
	if access, err := NewAccess(testAccessInfo(), owner); err == nil || access != nil {
		t.Fatal("accepted typed-nil owner")
	}
}

func testUnusedPrepare(context.Context, string) (Access, error) {
	return nil, errors.New("unused access preparer")
}

// TestBackendPrepareSelectsProvider proves lookup preserves the caller's
// context/name and returns the selected provider's admitted access unchanged.
func TestBackendPrepareSelectsProvider(t *testing.T) {
	owner := &accessTestOwner{}
	want, err := NewAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), testContextKey{}, "prepare")
	var registry sessionRegistry
	if err := registry.register("chosen", testUnusedOpen, func(got context.Context, name string) (Access, error) {
		if got != ctx {
			t.Error("context replaced")
		}
		if name != "access0" {
			t.Errorf("name=%q", name)
		}
		return want, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.register("other", testUnusedOpen, func(context.Context, string) (Access, error) {
		t.Error("unselected preparer called")
		return nil, errors.New("unselected")
	}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Prepare(ctx, "access0")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatal("selected access replaced")
	}
	if owner.calls != 0 {
		t.Fatal("live lease closed")
	}
	if err := got.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestBackendPrepareRefusalCleansLease proves a preparation failure cannot
// retain its map lease and that both producing and cleanup errors survive.
func TestBackendPrepareRefusalCleansLease(t *testing.T) {
	owner := &accessTestOwner{err: errors.New("cleanup")}
	access, err := NewAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("prepare failed")
	var registry sessionRegistry
	if err := registry.register("chosen", testUnusedOpen, func(context.Context, string) (Access, error) {
		return access, want
	}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Prepare(context.Background(), "access0")
	if got != nil {
		t.Fatal("failed preparation published")
	}
	if !errors.Is(err, want) {
		t.Fatalf("preparation error lost: %v", err)
	}
	if !errors.Is(err, owner.err) {
		t.Fatalf("cleanup error lost: %v", err)
	}
	if owner.calls != 1 {
		t.Fatalf("owner closes=%d", owner.calls)
	}
}

// TestBackendPrepareRefusesInvalidBoundary prevents incomplete registrations,
// nil contexts and nil factory results from admitting an access.
func TestBackendPrepareRefusesInvalidBoundary(t *testing.T) {
	var registry sessionRegistry
	if err := registry.register("missing", testUnusedOpen, nil); err == nil {
		t.Fatal("accepted missing preparer")
	}
	calls := 0
	if err := registry.register("chosen", testUnusedOpen, func(context.Context, string) (Access, error) {
		calls++
		return nil, nil //nolint:nilnil // Deliberately invalid provider result; this test must prove nil success is refused after exactly one call.
	}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backend.Prepare(nil, "access0"); err == nil { //nolint:staticcheck // SA1012: intentionally nil to prove refusal before the original preparer is invoked.
		t.Fatal("accepted nil context")
	}
	if calls != 0 {
		t.Fatal("nil context reached preparer")
	}
	if _, err := backend.Prepare(context.Background(), "access0"); err == nil {
		t.Fatal("accepted nil result")
	}
	if calls != 1 {
		t.Fatalf("preparer calls=%d", calls)
	}
}

type accessEmbedding struct {
	Access
	owner *accessTestOwner
}

func (a *accessEmbedding) Close() error { return a.owner.Close() }

// TestBackendPrepareRejectsUnconstructedAccess closes a forged owner instead
// of trusting an inherited sealed marker or exposing its unvalidated facts.
func TestBackendPrepareRejectsUnconstructedAccess(t *testing.T) {
	owner := &accessTestOwner{}
	var registry sessionRegistry
	if err := registry.register("chosen", testUnusedOpen, func(context.Context, string) (Access, error) {
		return &accessEmbedding{owner: owner}, nil
	}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Prepare(context.Background(), "access0")
	if got != nil {
		t.Fatal("forged access published")
	}
	if err == nil {
		t.Fatal("forged access accepted")
	}
	if owner.calls != 1 {
		t.Fatalf("forged owner closes=%d", owner.calls)
	}
}

// TestBackendPrepareCancellationReleasesAccess covers cancellation after the
// provider installs a map but before the wrapper publishes its lease.
func TestBackendPrepareCancellationReleasesAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	owner := &accessTestOwner{}
	var registry sessionRegistry
	if err := registry.register("chosen", testUnusedOpen, func(context.Context, string) (Access, error) {
		access, err := NewAccess(testAccessInfo(), owner)
		cancel()
		return access, err
	}); err != nil {
		t.Fatal(err)
	}
	backend, err := registry.lookup("chosen")
	if err != nil {
		t.Fatal(err)
	}
	got, err := backend.Prepare(ctx, "access0")
	if got != nil {
		t.Fatal("canceled access published")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if owner.calls != 1 {
		t.Fatalf("unpublished owner closes=%d", owner.calls)
	}
}

// TestBackendPrepareRejectsTypedNilResult and its error path preserve absence
// without invoking a promoted method on a typed-nil access owner.
func TestBackendPrepareRejectsTypedNilResult(t *testing.T) {
	for _, failure := range []error{nil, errors.New("provider refused")} {
		var registry sessionRegistry
		if err := registry.register("chosen", testUnusedOpen, func(context.Context, string) (Access, error) {
			var access *preparedAccess
			return access, failure
		}); err != nil {
			t.Fatal(err)
		}
		backend, err := registry.lookup("chosen")
		if err != nil {
			t.Fatal(err)
		}
		got, err := backend.Prepare(context.Background(), "access0")
		if got != nil {
			t.Fatal("typed-nil result published")
		}
		if err == nil {
			t.Fatal("typed-nil result accepted")
		}
		if failure != nil {
			if !errors.Is(err, failure) {
				t.Fatalf("provider failure lost: %v", err)
			}
		}
	}
}

type accessPanicOwner struct {
	calls   int
	failure error
}

func (o *accessPanicOwner) Close() error { o.calls++; panic(o.failure) }

// TestAccessCleanupPanicHasOneOutcome proves a failing map release neither
// escapes construction/close nor repeats when another caller ends the lease.
func TestAccessCleanupPanicHasOneOutcome(t *testing.T) {
	want := errors.New("map release panicked")
	owner := &accessPanicOwner{failure: want}
	access, err := NewAccess(testAccessInfo(), owner)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := access.Close(); !errors.Is(err, want) {
			t.Fatalf("panic cause lost: %v", err)
		}
	}
	if owner.calls != 1 {
		t.Fatalf("close attempts=%d", owner.calls)
	}
	invalid := testAccessInfo()
	invalid.Device = ""
	refused := &accessPanicOwner{failure: want}
	if got, err := NewAccess(invalid, refused); got != nil || !errors.Is(err, want) {
		t.Fatalf("construction panic cleanup: access=%v error=%v", got, err)
	}
	if refused.calls != 1 {
		t.Fatalf("refused owner close attempts=%d", refused.calls)
	}
}
