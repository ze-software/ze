// Design: docs/architecture/l2tp/bng-5-pppoe.md -- prepared kernel backend custody
// Related: prepare.go -- acquisition, refusal cleanup and bound request identity

package kernel

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

// TestPreparedKernelAccessRejectsMissingBackendOwner proves plain and typed nil
// dependencies cannot publish an Access or reach resolution, with or without error.
func TestPreparedKernelAccessRejectsMissingBackendOwner(t *testing.T) {
	var absent *preparedOwnerBackend
	want := errors.New("backend acquisition refused")
	for name, backend := range map[string]iface.Backend{"nil": nil, "typed nil": absent} {
		t.Run(name, func(t *testing.T) {
			for _, failure := range []error{nil, want} {
				acquisitions, resolutions := 0, 0
				access, err := prepareModelWithResolver(t.Context(), func() (iface.Backend, error) {
					acquisitions++
					return backend, failure
				}, func(string) (iface.Binding, error) {
					resolutions++
					return preparedOwnerBinding(), nil
				})
				if access != nil {
					t.Fatal("missing backend published an access")
				}
				if err == nil {
					t.Fatal("missing backend was accepted")
				}
				if failure != nil {
					if !errors.Is(err, failure) {
						t.Fatalf("acquisition error lost: %v", err)
					}
				}
				if acquisitions != 1 {
					t.Fatalf("acquisitions=%d, want 1", acquisitions)
				}
				if resolutions != 0 {
					t.Fatal("missing backend reached resolution")
				}
			}
		})
	}
}

// TestPreparedKernelAccessConsumesPartialBackendFailure proves a backend returned
// beside an error is closed exactly once and both original causes remain reachable.
func TestPreparedKernelAccessConsumesPartialBackendFailure(t *testing.T) {
	wantAcquire := errors.New("backend acquisition incomplete")
	wantClose := errors.New("partial backend close failed")
	backend := &preparedOwnerBackend{closeFailure: wantClose}
	resolutions := 0
	access, err := prepareModelWithResolver(t.Context(), func() (iface.Backend, error) {
		return backend, wantAcquire
	}, func(string) (iface.Binding, error) {
		resolutions++
		return preparedOwnerBinding(), nil
	})
	if access != nil {
		t.Fatal("partial backend acquisition published an access")
	}
	if !errors.Is(err, wantAcquire) {
		t.Fatalf("original acquisition cause lost: %v", err)
	}
	if !errors.Is(err, wantClose) {
		t.Fatalf("partial cleanup cause lost: %v", err)
	}
	if backend.closes != 1 {
		t.Fatalf("partial backend closes=%d, want 1", backend.closes)
	}
	if resolutions != 0 {
		t.Fatal("partial acquisition reached resolution")
	}
}

// TestPreparedKernelAccessClosesBackendOnResolutionFailure checks the acquired
// dependency is consumed once when resolution refuses, preserving both causes.
func TestPreparedKernelAccessClosesBackendOnResolutionFailure(t *testing.T) {
	wantResolve := errors.New("logical access resolution failed")
	wantClose := errors.New("unpublished backend close failed")
	backend := &preparedOwnerBackend{closeFailure: wantClose}
	access, err := prepareModelWithResolver(t.Context(), func() (iface.Backend, error) {
		return backend, nil
	}, func(name string) (iface.Binding, error) {
		if name != "logical" {
			t.Errorf("resolver name=%q", name)
		}
		return iface.Binding{}, wantResolve
	})
	if access != nil {
		t.Fatal("failed resolution published an access")
	}
	if !errors.Is(err, wantResolve) {
		t.Fatalf("resolution cause lost: %v", err)
	}
	if !errors.Is(err, wantClose) {
		t.Fatalf("cleanup cause lost: %v", err)
	}
	if backend.closes != 1 {
		t.Fatalf("unpublished backend closes=%d, want 1", backend.closes)
	}
}

// TestPreparedKernelAccessClosesBackendOnSnapshotRefusal drives valid resolution
// through real NewAccess validation, which must consume the rejected parent once.
func TestPreparedKernelAccessClosesBackendOnSnapshotRefusal(t *testing.T) {
	wantClose := errors.New("invalid snapshot backend close failed")
	backend := &preparedOwnerBackend{closeFailure: wantClose}
	access, err := prepareModelWithResolver(t.Context(), func() (iface.Backend, error) {
		return backend, nil
	}, func(string) (iface.Binding, error) {
		binding := preparedOwnerBinding()
		binding.Ifindex = 0
		return binding, nil
	})
	if access != nil {
		t.Fatal("invalid snapshot published an access")
	}
	if !errors.Is(err, wantClose) {
		t.Fatalf("snapshot cleanup cause lost: %v", err)
	}
	if !strings.Contains(err.Error(), "discovery interface index") {
		t.Fatalf("expected snapshot validation, got %v", err)
	}
	if backend.closes != 1 {
		t.Fatalf("invalid snapshot backend closes=%d, want 1", backend.closes)
	}
}

// TestPreparedKernelAccessRetainsBackendUntilParentClose proves preparation owns
// one dependency until its parent closes. This fake is not native session proof.
func TestPreparedKernelAccessRetainsBackendUntilParentClose(t *testing.T) {
	backend := &preparedOwnerBackend{}
	acquisitions, resolutions := 0, 0
	access, err := prepareModelWithResolver(t.Context(), func() (iface.Backend, error) {
		acquisitions++
		return backend, nil
	}, func(string) (iface.Binding, error) {
		resolutions++
		return preparedOwnerBinding(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	if acquisitions != 1 {
		t.Fatalf("acquisitions=%d, want 1", acquisitions)
	}
	if resolutions != 1 {
		t.Fatalf("resolutions=%d, want 1", resolutions)
	}
	if backend.closes != 0 {
		t.Fatal("successful preparation released its backend")
	}
	for range 2 {
		if err := access.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if backend.closes != 1 {
		t.Fatalf("parent backend closes=%d, want 1", backend.closes)
	}
}

// TestPreparedKernelOwnerRejectsForeignAccessRequest checks each captured access
// coordinate before transport work. A matching request reaches SID validation.
func TestPreparedKernelOwnerRejectsForeignAccessRequest(t *testing.T) {
	if err := requirePlatform(); err != nil {
		t.Skip(err)
	}
	info := dataplane.AccessInfo{Device: "actual0.10", ControlDevice: "actual0.10",
		DiscoveryIfIndex: 42, HWAddr: [6]byte{2, 3, 4, 5, 6, 7}, MTU: 1500}
	backend := &preparedOwnerBackend{}
	owner := &kernelAccessOwner{name: "logical", info: info, backend: backend}
	// SID zero keeps even a broken identity guard away from native allocation.
	request := dataplane.Request{AccessName: "logical", Device: info.Device,
		ControlDevice: info.ControlDevice, PeerMAC: [6]byte{2, 7, 6, 5, 4, 3}}
	for name, change := range map[string]func(*dataplane.Request){
		"logical access":    func(r *dataplane.Request) { r.AccessName = "other" },
		"session device":    func(r *dataplane.Request) { r.Device = "replacement0" },
		"control device":    func(r *dataplane.Request) { r.ControlDevice = "replacement1" },
		"control namespace": func(r *dataplane.Request) { r.Namespace = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			foreign := request
			change(&foreign)
			opened, err := owner.Open(t.Context(), foreign)
			if opened != nil {
				t.Fatal("foreign access returned a session")
			}
			if err == nil {
				t.Fatal("foreign access was accepted")
			}
			if !strings.Contains(err.Error(), name+" differs from its prepared kernel access") {
				t.Fatalf("foreign coordinate reached another refusal: %v", err)
			}
		})
	}
	opened, err := owner.Open(t.Context(), request)
	if opened != nil {
		t.Fatal("zero SID returned a session")
	}
	if err == nil {
		t.Fatal("zero SID was accepted")
	}
	if !strings.Contains(err.Error(), "nonzero session ID") {
		t.Fatalf("matching access did not reach session validation: %v", err)
	}
	if backend.closes != 0 {
		t.Fatal("refused child released the prepared backend")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if backend.closes != 1 {
		t.Fatalf("owner backend closes=%d, want 1", backend.closes)
	}
}

type preparedOwnerBackend struct {
	iface.Backend
	closes       int
	closeFailure error
}

func (b *preparedOwnerBackend) Close() error {
	b.closes++
	return b.closeFailure
}

func preparedOwnerBinding() iface.Binding {
	return iface.Binding{OsName: "actual0.10", Ifindex: 42,
		OperMAC: "02:03:04:05:06:07", MTU: 1500}
}
