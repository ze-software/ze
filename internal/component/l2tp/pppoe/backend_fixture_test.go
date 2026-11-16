// Design: docs/architecture/l2tp/bng-5-pppoe.md -- explicit provider selection for policy tests

package pppoe

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// Policy/reload fixtures create no native interfaces. Their named providers
// are explicitly selected through the production registries. An accidental
// session open refuses instead of pretending to construct a data plane.
type fixtureIfaceBackend struct{ iface.Backend }

func (*fixtureIfaceBackend) Close() error { return nil }

var fixtureBackendOnce sync.Once
var fixtureBackendErr error

// Policy fixtures resolve their existing synthetic bindings through the same
// registered preparation boundary that production providers implement.
var resolveInterfaceFn = resolveInterface

func fixturePrepareAccess(_ context.Context, name string) (dataplane.Access, error) {
	binding, err := resolveInterfaceFn(name)
	if err != nil {
		return nil, err
	}
	return dataplane.NewAccess(dataplane.AccessInfo{Device: binding.Device, ControlDevice: binding.Device,
		DiscoveryIfIndex: binding.Ifindex, HWAddr: binding.HWAddr, MTU: binding.MTU}, fixtureAccessOwner{})
}

type fixtureAccessOwner struct{}

func (fixtureAccessOwner) Close() error { return nil }

func fixturePrepareConfiguration(t *testing.T, interfaces []InterfaceConfig) (dataplane.Backend, *accessConfiguration) {
	t.Helper()
	backend, err := selectedSessionBackend()
	if err != nil {
		t.Fatal(err)
	}
	accesses, err := prepareAccessConfiguration(context.Background(), backend, interfaces, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := closeAccessConfiguration(accesses.accesses, nil); err != nil {
			t.Error(err)
		}
	})
	return backend, accesses
}

func fixtureSelectBackend(t *testing.T) {
	t.Helper()
	const name = "pppoe-policy-fixture"
	if iface.GetBackend() != nil || iface.ActiveBackendName() != "" {
		t.Fatal("policy fixture cannot replace an active interface backend")
	}
	fixtureBackendOnce.Do(func() {
		fixtureBackendErr = iface.RegisterBackend(name, func() (iface.Backend, error) { return &fixtureIfaceBackend{}, nil })
		if fixtureBackendErr != nil {
			return
		}
		fixtureBackendErr = dataplane.RegisterSessionDataplane(name, func(context.Context, dataplane.Request) (dataplane.OpenedSession, error) {
			return nil, errors.New("policy fixture unexpectedly opened a session")
		}, fixturePrepareAccess)
	})
	if fixtureBackendErr != nil {
		t.Fatal(fixtureBackendErr)
	}
	if err := iface.LoadBackend(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := iface.CloseBackend(); err != nil {
			t.Error(err)
		}
	})
}
