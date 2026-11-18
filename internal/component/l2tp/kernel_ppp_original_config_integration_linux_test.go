// Design: docs/research/l2tpv2-ze-integration.md -- exact original backend
// Related: kernel_ppp_owners_linux.go -- snapshot refuses backend substitution

//go:build integration && linux

package l2tp

import (
	"testing"

	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/test/userns"
)

func TestKernelOriginalConfigRejectsAnotherActualBackend(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	original := kernelCounterNativeBackend(t)
	other := kernelCounterNativeBackend(t)
	creator, err := pppkernel.NewUnitCreator(original)
	if err != nil {
		t.Fatal(err)
	}
	transport := kernelCounterTransport(t)
	channel, unit, err := pppkernel.DevPPPSetup(creator, transport, "zc-original")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unit.Close(); err != nil {
			t.Error(err)
		}
		if err := channel.Close(); err != nil {
			t.Error(err)
		}
	})
	owners := &kernelPPPOwners{channel: channel, unit: unit}
	if _, err := originalKernelPPPConfig(owners, other); err == nil {
		t.Fatal("actor preflight substituted another independently opened backend")
	}
	config, err := originalKernelPPPConfig(owners, original)
	if err != nil {
		t.Fatalf("refusal consumed the original pair: %v", err)
	}
	if config.Backend != original {
		t.Fatal("valid actor snapshot did not retain the actual creator backend")
	}
	again, err := unit.SessionConfig(channel)
	if err != nil {
		t.Fatalf("valid actor snapshot consumed the original pair: %v", err)
	}
	if config.Channel != again.Channel || config.Unit != again.Unit || config.ChanFD != again.ChanFD || config.UnitFD != again.UnitFD {
		t.Fatal("actor snapshot changed the actual original owners or checked labels")
	}
}
