// Design: docs/architecture/iface/logical-name-resolution.md -- captured backend readback

package iface

import "testing"

func TestResolveBackendReturnsProducingOwner(t *testing.T) {
	const selected = "test-resolve-producing-owner"
	owner := &exactResolveBackend{link: func(string) (*InterfaceInfo, error) {
		return exactResolveInfo(), nil
	}}
	withExactResolveBackends(t, map[string]func() (Backend, error){
		selected: func() (Backend, error) { return owner, nil },
	})
	if err := LoadBackend(selected); err != nil {
		t.Fatal(err)
	}
	actual, binding, err := ResolveBackend(selected, "actual0")
	if err != nil {
		t.Fatal(err)
	}
	if actual != owner {
		t.Fatal("binding was returned with a different backend owner")
	}
	if binding.Ifindex != 7 {
		t.Fatalf("binding index = %d", binding.Ifindex)
	}
}

func TestResolveBackendRefusesMissingReadback(t *testing.T) {
	const selected = "test-resolve-missing-readback"
	owner := &exactResolveBackend{link: func(string) (*InterfaceInfo, error) {
		return nil, nil
	}}
	withExactResolveBackends(t, map[string]func() (Backend, error){
		selected: func() (Backend, error) { return owner, nil },
	})
	if err := LoadBackend(selected); err != nil {
		t.Fatal(err)
	}
	actual, binding, err := ResolveBackend(selected, "actual0")
	if err == nil {
		t.Fatal("missing link readback was admitted")
	}
	if actual != nil {
		t.Fatal("failed resolution published an owner")
	}
	if binding != (Binding{}) {
		t.Fatalf("failed resolution published %+v", binding)
	}
}
