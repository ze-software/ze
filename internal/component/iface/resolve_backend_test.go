// Design: docs/architecture/iface/logical-name-resolution.md -- exact backend resolution

package iface

import (
	"errors"
	"testing"
)

// exactResolveBackend exposes only the reads this test expects. An accidental
// call to another Backend method fails through the nil embedded interface.
type exactResolveBackend struct {
	Backend
	link func(string) (*InterfaceInfo, error)
	list func() ([]InterfaceInfo, error)
}

func (b *exactResolveBackend) GetLink(name string) (*InterfaceInfo, error) { return b.link(name) }
func (b *exactResolveBackend) ListInterfaces() ([]InterfaceInfo, error)    { return b.list() }
func (*exactResolveBackend) Close() error                                  { return nil }

func withExactResolveBackends(t *testing.T, factories map[string]func() (Backend, error)) {
	t.Helper()
	backendsMu.Lock()
	previous, previousName := activeBackend, activeBackendName
	for name := range factories {
		if _, exists := backends[name]; exists {
			backendsMu.Unlock()
			t.Fatalf("test backend %q already registered", name)
		}
	}
	activeBackend, activeBackendName = nil, ""
	activeBackendGeneration++
	for name, factory := range factories {
		backends[name] = factory
	}
	backendsMu.Unlock()
	previousResolver := globalResolver
	globalResolver = freshResolver()
	t.Cleanup(func() {
		if err := CloseBackend(); err != nil {
			t.Errorf("close test backend: %v", err)
		}
		backendsMu.Lock()
		for name := range factories {
			delete(backends, name)
		}
		activeBackend, activeBackendName = previous, previousName
		activeBackendGeneration++
		backendsMu.Unlock()
		globalResolver = previousResolver
	})
}

func exactResolveInfo() *InterfaceInfo {
	return &InterfaceInfo{Name: "actual0", OsName: "actual0", Index: 7,
		MAC: "02:03:04:05:06:07", MTU: 1500, State: LinkStateUp}
}

// TestResolveForBackendBypassesForeignCache proves an exact resolution reads
// the selected backend and preserves os-name policy despite a foreign cache.
func TestResolveForBackendBypassesForeignCache(t *testing.T) {
	const selected = "test-exact-resolve-cache"
	reads := 0
	backend := &exactResolveBackend{link: func(name string) (*InterfaceInfo, error) {
		reads++
		if name != "actual0" {
			t.Errorf("backend read %q, want selected OS name", name)
		}
		return exactResolveInfo(), nil
	}}
	withExactResolveBackends(t, map[string]func() (Backend, error){
		selected: func() (Backend, error) { return backend, nil },
	})
	if err := LoadBackend(selected); err != nil {
		t.Fatal(err)
	}
	globalResolver.setMapping(map[string]string{"logical": "actual0"}, nil, nil)
	globalResolver.cache["logical"] = Binding{OsName: "foreign0", Ifindex: 999}
	got, err := ResolveForBackend(selected, "logical")
	if err != nil {
		t.Fatal(err)
	}
	if got.OsName != "actual0" {
		t.Fatalf("device=%q", got.OsName)
	}
	if got.Ifindex != 7 {
		t.Fatalf("index=%d", got.Ifindex)
	}
	if reads != 1 {
		t.Fatalf("backend reads=%d", reads)
	}
	if got.OperMAC != "02:03:04:05:06:07" {
		t.Fatalf("MAC=%q", got.OperMAC)
	}
	if got.MTU != 1500 {
		t.Fatalf("MTU=%d", got.MTU)
	}
}

// TestResolveForBackendUsesCapturedMACListing proves mac/match still wins over
// os-name and reads the same captured backend's actual interface population.
func TestResolveForBackendUsesCapturedMACListing(t *testing.T) {
	const selected = "test-exact-resolve-mac"
	lists := 0
	backend := &exactResolveBackend{
		link: func(string) (*InterfaceInfo, error) {
			t.Fatal("MAC selection reached direct lookup")
			return nil, errors.New("unexpected read")
		},
		list: func() ([]InterfaceInfo, error) { lists++; return []InterfaceInfo{*exactResolveInfo()}, nil },
	}
	withExactResolveBackends(t, map[string]func() (Backend, error){
		selected: func() (Backend, error) { return backend, nil },
	})
	if err := LoadBackend(selected); err != nil {
		t.Fatal(err)
	}
	globalResolver.setMapping(map[string]string{"logical": "wrong0"}, map[string]string{"logical": "02:03:04:05:06:07"}, nil)
	got, err := ResolveForBackend(selected, "logical")
	if err != nil {
		t.Fatal(err)
	}
	if got.OsName != "actual0" {
		t.Fatalf("device=%q", got.OsName)
	}
	if got.Ifindex != 7 {
		t.Fatalf("index=%d", got.Ifindex)
	}
	if lists != 1 {
		t.Fatalf("interface listings=%d", lists)
	}
}

// TestResolveForBackendRejectsChangedGeneration performs real backend swaps
// during the captured read. Even A-to-B-to-the-same-A-owner cannot publish the
// earlier generation's result or learned binding.
func TestResolveForBackendRejectsChangedGeneration(t *testing.T) {
	for _, transition := range []string{"replace", "same owner again", "close"} {
		t.Run(transition, func(t *testing.T) {
			selected := "test-exact-resolve-generation-" + transition
			other := selected + "-other"
			backend := &exactResolveBackend{}
			backend.link = func(string) (*InterfaceInfo, error) {
				if transition == "close" {
					if err := CloseBackend(); err != nil {
						return nil, err
					}
					return exactResolveInfo(), nil
				}
				if err := LoadBackend(other); err != nil {
					return nil, err
				}
				if transition == "same owner again" {
					if err := LoadBackend(selected); err != nil {
						return nil, err
					}
				}
				return exactResolveInfo(), nil
			}
			withExactResolveBackends(t, map[string]func() (Backend, error){
				selected: func() (Backend, error) { return backend, nil },
				other:    func() (Backend, error) { return &exactResolveBackend{}, nil },
			})
			if err := LoadBackend(selected); err != nil {
				t.Fatal(err)
			}
			got, err := ResolveForBackend(selected, "logical")
			if err == nil {
				t.Fatal("published a result across backend replacement")
			}
			if got != (Binding{}) {
				t.Fatalf("failed read published %+v", got)
			}
			if _, exists := globalResolver.boundOS["logical"]; exists {
				t.Fatal("failed read recorded a binding")
			}
		})
	}
}

// TestResolveForBackendRefusesWrongSelection never reads an unavailable or
// differently named backend, and preserves a producing read's error identity.
func TestResolveForBackendRefusesWrongSelection(t *testing.T) {
	const selected = "test-exact-resolve-refusal"
	want := errors.New("link read failed")
	reads := 0
	backend := &exactResolveBackend{link: func(string) (*InterfaceInfo, error) { reads++; return nil, want }}
	withExactResolveBackends(t, map[string]func() (Backend, error){
		selected: func() (Backend, error) { return backend, nil },
	})
	if _, err := ResolveForBackend(selected, "logical"); err == nil {
		t.Fatal("accepted missing backend")
	}
	if err := LoadBackend(selected); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveForBackend("another", "logical"); err == nil {
		t.Fatal("accepted wrong backend")
	}
	if reads != 0 {
		t.Fatalf("refused selection performed %d reads", reads)
	}
	if _, err := ResolveForBackend(selected, "logical"); !errors.Is(err, want) {
		t.Fatalf("read failure lost: %v", err)
	}
}
