package hub

// VALIDATES: the service construction registry (registerService / buildServices
// / registerBuiltService) builds registered factories, skips unconfigured ones
// (nil service), skips factories that error, and routes services into the
// listener migrator through per-service registration hooks.
// PREVENTS: a feature-gate regression where an optional service is silently not
// built, or where one service's build error aborts startup of the others.

import (
	"context"
	"errors"
	"testing"
)

// fakeService is a minimal Service used to exercise the construction registry
// without building a real (compile-out-able) service.
type fakeService struct {
	name    string
	addrs   []string
	stopped bool
}

func (f *fakeService) Name() string                                    { return f.name }
func (f *fakeService) Addresses() []string                             { return f.addrs }
func (f *fakeService) Reconfigure(_ context.Context, a []string) error { f.addrs = a; return nil }
func (f *fakeService) Shutdown(context.Context) error                  { f.stopped = true; return nil }

// registeredServiceName reports whether a factory is registered under name in
// the current build. Used by the build-tag present/absent tests.
func registeredServiceName(name string) bool {
	for _, nf := range serviceFactories {
		if nf.name == name {
			return true
		}
	}
	return false
}

// withCleanRegistry isolates the package-global factory list per test.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	saved := serviceFactories
	t.Cleanup(func() { serviceFactories = saved })
	serviceFactories = nil
}

func TestServiceRegistry_BuildsRegisteredFactory(t *testing.T) {
	withCleanRegistry(t)

	want := &fakeService{name: "fake", addrs: []string{"127.0.0.1:1"}}
	registerService("fake", func(ServiceDeps) (Service, error) { return want, nil }, nil)
	// A factory that returns (nil, nil) means "not configured" and is skipped.
	registerService("unconfigured", func(ServiceDeps) (Service, error) { return nil, nil }, nil) //nolint:nilnil // models the not-configured skip path

	got := buildServices(ServiceDeps{})
	if len(got) != 1 {
		t.Fatalf("buildServices: want 1 built service, got %d", len(got))
	}
	if got[0].Name() != "fake" {
		t.Fatalf("buildServices: want %q, got %q", "fake", got[0].Name())
	}
}

func TestServiceRegistry_AbsentFeatureNoOp(t *testing.T) {
	withCleanRegistry(t)
	if got := buildServices(ServiceDeps{}); len(got) != 0 {
		t.Fatalf("empty registry: want 0 services, got %d", len(got))
	}
}

func TestServiceRegistry_BuildErrorSkipped(t *testing.T) {
	withCleanRegistry(t)
	registerService("boom", func(ServiceDeps) (Service, error) { return nil, errors.New("boom") }, nil)
	ok := &fakeService{name: "ok"}
	registerService("ok", func(ServiceDeps) (Service, error) { return ok, nil }, nil)

	got := buildServices(ServiceDeps{})
	if len(got) != 1 || got[0].Name() != "ok" {
		t.Fatalf("build error should be skipped, surviving the rest: got %d services", len(got))
	}
}

func TestRegisterBuiltService_UsesRegisteredMigratorHook(t *testing.T) {
	withCleanRegistry(t)
	lm := newListenerMigrator()
	want := &fakeService{name: "fake", addrs: []string{"0.0.0.0:8443"}}
	registerService("fake", func(ServiceDeps) (Service, error) { return want, nil }, func(lm *ListenerMigrator, svc Service) {
		lm.SetLG(svc)
	})

	built := buildServices(ServiceDeps{})
	if len(built) != 1 {
		t.Fatalf("buildServices: want 1 built service, got %d", len(built))
	}
	registerBuiltService(lm, built[0])
	if lm.lg != want {
		t.Fatal("registered service hook was not used to wire the listener migrator")
	}
	registerBuiltService(lm, builtService{Service: &fakeService{name: "unknown"}})
}
