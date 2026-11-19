// Design: docs/architecture/l2tp/bng-5-pppoe.md -- transactional access preparation

package pppoe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

type accessConfigurationOwner struct {
	closes int
	open   dataplane.OpenFunc
}

func (o *accessConfigurationOwner) Close() error { o.closes++; return nil }

func (o *accessConfigurationOwner) Open(ctx context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
	if o.open == nil {
		return nil, errors.New("test does not open sessions")
	}
	return o.open(ctx, request)
}

var accessConfigurationTestID atomic.Uint64

func accessConfigurationBackend(t *testing.T, prepare dataplane.PrepareFunc) dataplane.Backend {
	t.Helper()
	name := fmt.Sprintf("access-configuration-%d", accessConfigurationTestID.Add(1))
	err := dataplane.RegisterSessionDataplane(name, prepare)
	if err != nil {
		t.Fatal(err)
	}
	backend, err := dataplane.LookupSessionDataplane(name)
	if err != nil {
		t.Fatal(err)
	}
	return backend
}

func accessConfigurationFixture(t *testing.T, owner *accessConfigurationOwner, name, namespace string, index int) dataplane.Access {
	t.Helper()
	access, err := dataplane.NewAccess(dataplane.AccessInfo{Device: "dp-" + name, ControlDevice: "cp-" + name,
		Namespace: namespace, DiscoveryIfIndex: index, HWAddr: [6]byte{2, 0, 0, 0, 0, byte(index)}, MTU: 1500}, owner)
	if err != nil {
		t.Fatal(err)
	}
	return access
}

// Goal: failed preparation never publishes a partial map and releases every
// newly prepared lease, while a running interface retains its existing lease.
func TestAccessConfigurationFailureReleasesOnlyAdditions(t *testing.T) {
	oldOwner := &accessConfigurationOwner{}
	newOwner := &accessConfigurationOwner{}
	old := accessConfigurationFixture(t, oldOwner, "old", "dataplane", 7)
	added := accessConfigurationFixture(t, newOwner, "new", "dataplane", 8)
	cause := errors.New("third access unavailable")
	var names []string
	backend := accessConfigurationBackend(t, func(_ context.Context, name string) (dataplane.Access, error) {
		names = append(names, name)
		if name == "new" {
			return added, nil
		}
		return nil, cause
	})
	kept := map[string]dataplane.Access{"old": old}
	next, err := prepareAccessConfiguration(context.Background(), backend, []InterfaceConfig{{Name: "old"}, {Name: "new"}, {Name: "bad"}}, kept)
	if !errors.Is(err, cause) {
		t.Fatalf("error = %v, want %v", err, cause)
	}
	if next != nil {
		t.Fatal("failed preparation published a configuration")
	}
	if oldOwner.closes != 0 {
		t.Fatal("failed preparation released the running access")
	}
	if newOwner.closes != 1 {
		t.Fatalf("new access closes = %d, want 1", newOwner.closes)
	}
	if !reflect.DeepEqual(names, []string{"new", "bad"}) {
		t.Fatalf("prepared names = %v", names)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: a provider aliasing a running owner under an added name is refused
// without cleanup destroying the owner still retained by the live map.
func TestAccessConfigurationAliasFailureKeepsExistingLease(t *testing.T) {
	owner := &accessConfigurationOwner{}
	access := accessConfigurationFixture(t, owner, "old", "dataplane", 7)
	backend := accessConfigurationBackend(t, func(context.Context, string) (dataplane.Access, error) { return access, nil })
	next, err := prepareAccessConfiguration(context.Background(), backend, []InterfaceConfig{{Name: "old"}, {Name: "new"}}, map[string]dataplane.Access{"old": access})
	if err == nil {
		t.Fatal("provider alias was accepted")
	}
	if next != nil {
		t.Fatal("provider alias published a map")
	}
	if owner.closes != 0 {
		t.Fatal("refusing an alias destroyed the running mapping")
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
}

// Goal: one shared discovery socket is never used across different namespaces,
// and a Linux ifindex collision never overwrites another interface's server.
func TestAccessConfigurationRefusesAmbiguousDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, namespace string
		index           int
	}{
		{"namespace", "other", 8}, {"index", "dataplane", 7}, {"unrepresentable index", "dataplane", 65536},
	} {
		t.Run(tc.name, func(t *testing.T) {
			firstOwner, secondOwner := &accessConfigurationOwner{}, &accessConfigurationOwner{}
			first := accessConfigurationFixture(t, firstOwner, "a", "dataplane", 7)
			second := accessConfigurationFixture(t, secondOwner, "b", tc.namespace, tc.index)
			backend := accessConfigurationBackend(t, func(_ context.Context, name string) (dataplane.Access, error) {
				if name == "a" {
					return first, nil
				}
				return second, nil
			})
			next, err := prepareAccessConfiguration(context.Background(), backend, []InterfaceConfig{{Name: "a"}, {Name: "b"}}, nil)
			if err == nil {
				t.Fatal("ambiguous discovery was accepted")
			}
			if next != nil {
				t.Fatal("ambiguous discovery published a configuration")
			}
			if firstOwner.closes != 1 {
				t.Fatalf("first closes = %d, want 1", firstOwner.closes)
			}
			if secondOwner.closes != 1 {
				t.Fatalf("second closes = %d, want 1", secondOwner.closes)
			}
		})
	}
}

// Goal: a prepared replacement preserves existing leases and publishes both
// DP and Linux identities without resolving or replacing the kept interface.
func TestAccessConfigurationRetainsRunningLease(t *testing.T) {
	oldOwner, newOwner := &accessConfigurationOwner{}, &accessConfigurationOwner{}
	old := accessConfigurationFixture(t, oldOwner, "old", "dataplane", 7)
	added := accessConfigurationFixture(t, newOwner, "new", "dataplane", 8)
	backend := accessConfigurationBackend(t, func(_ context.Context, name string) (dataplane.Access, error) {
		if name != "new" {
			return nil, errors.New("kept access was re-prepared")
		}
		return added, nil
	})
	next, err := prepareAccessConfiguration(context.Background(), backend, []InterfaceConfig{{Name: "old"}, {Name: "new"}}, map[string]dataplane.Access{"old": old})
	if err != nil {
		t.Fatal(err)
	}
	if next.accesses["old"] != old {
		t.Fatal("kept access was replaced")
	}
	if next.namespace != "dataplane" {
		t.Fatalf("namespace = %q", next.namespace)
	}
	if next.accesses["new"].Device() != "dp-new" {
		t.Fatal("data-plane identity was lost")
	}
	if next.accesses["new"].ControlDevice() != "cp-new" {
		t.Fatal("control-plane identity was lost")
	}
	if oldOwner.closes != 0 {
		t.Fatal("published old lease was released")
	}
	if newOwner.closes != 0 {
		t.Fatal("published new lease was released")
	}
	if err := closeAccessConfiguration(next.accesses, nil); err != nil {
		t.Fatal(err)
	}
}

// Goal: the actual server factory captures the prepared CP/DP identities and
// overwrites raw admission fields, while preserving the peer's session data.
func TestInterfaceServerUsesPreparedAccessSnapshot(t *testing.T) {
	owner := &accessConfigurationOwner{}
	access := accessConfigurationFixture(t, owner, "uplink", "dataplane", 9)
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	var got dataplane.Request
	cause := errors.New("factory reached")
	owner.open = func(_ context.Context, request dataplane.Request) (dataplane.OpenedSession, error) {
		got = request
		return nil, cause
	}
	name := fmt.Sprintf("access-configuration-%d", accessConfigurationTestID.Add(1))
	err := dataplane.RegisterSessionDataplane(name, func(context.Context, string) (dataplane.Access, error) {
		return nil, errors.New("snapshot must not be resolved again")
	})
	if err != nil {
		t.Fatal(err)
	}
	backend, err := dataplane.LookupSessionDataplane(name)
	if err != nil {
		t.Fatal(err)
	}
	sub := NewSubsystem(Parameters{})
	sub.logger = slog.Default()
	srv, err := sub.newInterfaceServer(&InterfaceConfig{Name: "wan", MaxSessions: 1}, &Parameters{}, backend, access)
	if err != nil {
		t.Fatal(err)
	}
	peer := [6]byte{2, 0, 0, 0, 0, 42}
	_, err = srv.openSession(context.Background(), dataplane.Request{AccessName: "wan", Device: "wrong-dp", ControlDevice: "wrong-cp", Namespace: "wrong-ns", SessionID: 42, PeerMAC: peer, CaptureProxyLCP: true})
	if !errors.Is(err, cause) {
		t.Fatalf("factory error=%v", err)
	}
	if got.Device != "dp-uplink" {
		t.Fatalf("DP device=%q", got.Device)
	}
	if got.ControlDevice != "cp-uplink" {
		t.Fatalf("CP device=%q", got.ControlDevice)
	}
	if got.Namespace != "dataplane" {
		t.Fatalf("namespace=%q", got.Namespace)
	}
	if got.SessionID != 42 {
		t.Fatalf("session ID changed: %+v", got)
	}
	if got.PeerMAC != peer {
		t.Fatalf("peer MAC changed: %+v", got)
	}
	if !got.CaptureProxyLCP {
		t.Fatalf("proxy choice changed: %+v", got)
	}
	if srv.ifIndex != 9 {
		t.Fatal("discovery did not retain the Linux index")
	}
	if srv.hwAddr != access.HWAddr() {
		t.Fatal("discovery did not retain the Linux source MAC")
	}
	if srv.mtu != access.MTU() {
		t.Fatal("discovery did not retain the Linux MTU")
	}
	if owner.closes != 0 {
		t.Fatal("server constructor released its borrowed access")
	}
}

// Goal: an already canceled reload cannot publish even if every interface is
// kept and no provider call would otherwise observe the canceled context.
func TestAccessConfigurationCanceledKeptAccessIsNotReleased(t *testing.T) {
	owner := &accessConfigurationOwner{}
	access := accessConfigurationFixture(t, owner, "old", "dataplane", 7)
	backend := accessConfigurationBackend(t, func(context.Context, string) (dataplane.Access, error) {
		return nil, errors.New("kept access was re-prepared")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	next, err := prepareAccessConfiguration(ctx, backend, []InterfaceConfig{{Name: "old"}}, map[string]dataplane.Access{"old": access})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	if next != nil {
		t.Fatal("canceled preparation was published")
	}
	if owner.closes != 0 {
		t.Fatal("canceled preparation released the running access")
	}
	if err := access.Close(); err != nil {
		t.Fatal(err)
	}
}

func selectAccessConfigurationBackend(t *testing.T, backend dataplane.Backend) {
	t.Helper()
	if iface.GetBackend() != nil {
		t.Fatal("access test cannot replace an active interface backend")
	}
	if iface.ActiveBackendName() != "" {
		t.Fatal("access test cannot replace an active interface backend name")
	}
	if err := iface.RegisterBackend(backend.Name(), func() (iface.Backend, error) { return &fixtureIfaceBackend{}, nil }); err != nil {
		t.Fatal(err)
	}
	if err := iface.LoadBackend(backend.Name()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
}

// Goal: the real startup entrypoint prepares every access before attempting
// native discovery and releases an earlier mapping if a later prepare fails.
// No kernel operation is needed: the selected provider deliberately refuses.
func TestPPPoEStartPreparationFailureIsAtomic(t *testing.T) {
	owner := &accessConfigurationOwner{}
	access := accessConfigurationFixture(t, owner, "a", "dataplane", 7)
	cause := errors.New("second access refused")
	backend := accessConfigurationBackend(t, func(_ context.Context, name string) (dataplane.Access, error) {
		if name == "a" {
			return access, nil
		}
		return nil, cause
	})
	selectAccessConfigurationBackend(t, backend)
	sub := NewSubsystem(Parameters{Enabled: true, Interfaces: []InterfaceConfig{{Name: "a"}, {Name: "b"}}})
	err := sub.Start(context.Background(), nil, nil)
	if !errors.Is(err, cause) {
		t.Fatalf("Start error=%v,want selected provider refusal", err)
	}
	if owner.closes != 1 {
		t.Fatalf("earlier access closes=%d,want1", owner.closes)
	}
	if sub.started {
		t.Fatal("failed start marked subsystem started")
	}
	if sub.discFD != -1 {
		t.Fatal("failed start published a discovery descriptor")
	}
	if sub.readDone != nil {
		t.Fatal("failed start published a discovery worker")
	}
	if sub.pppDriver != nil {
		t.Fatal("failed start published a PPP driver")
	}
	if sub.accesses != nil {
		t.Fatal("failed start published access leases")
	}
}
