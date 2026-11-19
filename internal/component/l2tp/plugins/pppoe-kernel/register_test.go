// Design: docs/architecture/l2tp/bng-5-pppoe.md -- discoverable kernel provider
// Related: register.go -- composition-root side effect

package pppoekernel

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/kernel"
)

// TestKernelProviderRegisters proves importing the composition wrapper installs
// the exact native provider without a parent transport import.
func TestKernelProviderRegisters(t *testing.T) {
	backend, err := dataplane.LookupSessionDataplane(kernel.Name)
	if err != nil {
		t.Fatal(err)
	}
	if backend.Name() != kernel.Name {
		t.Fatalf("provider=%q", backend.Name())
	}
	access, err := dataplane.NewAccess(dataplane.AccessInfo{
		Device: "eth0", ControlDevice: "eth0", DiscoveryIfIndex: 1,
		HWAddr: [6]byte{2, 0, 0, 0, 0, 1}, MTU: 1500,
	}, &registrationAccessOwner{test: t})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := access.Close(); err != nil {
			t.Error(err)
		}
	})
	if opened, err := access.Open(nil, dataplane.Request{}); err == nil || opened != nil { //nolint:staticcheck // SA1012: intentionally nil to prove refusal before the session owner is invoked.
		t.Fatalf("prepared access context validation bypassed: session=%v error=%v", opened, err)
	}
}

type registrationAccessOwner struct{ test *testing.T }

func (owner *registrationAccessOwner) Open(context.Context, dataplane.Request) (dataplane.OpenedSession, error) {
	owner.test.Fatal("nil context reached the session owner")
	return nil, errors.New("unreachable session owner")
}

func (*registrationAccessOwner) Close() error { return nil }
