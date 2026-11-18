// Design: docs/research/l2tpv2-ze-integration.md -- acknowledged NCP family gates.

package kernel

import (
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"testing"
)

type familyTestBackend struct {
	*sessionTestBackend
	trace     *[]string
	addErr    error
	removeErr error
}

func (b *familyTestBackend) AddAddressP2P(name, local, peer string) error {
	*b.trace = append(*b.trace, "add:"+local)
	return b.addErr
}

func (b *familyTestBackend) RemoveAddress(name, local string) error {
	*b.trace = append(*b.trace, "remove:"+local)
	return b.removeErr
}

type familyTestSession struct {
	owner         *session
	backend       *familyTestBackend
	trace         []string
	modeErr       error
	connectErr    error
	disconnectErr error
}

func newFamilyTestSession(t *testing.T) *familyTestSession {
	t.Helper()
	f := &familyTestSession{}
	f.backend = &familyTestBackend{sessionTestBackend: &sessionTestBackend{}, trace: &f.trace}
	cfg := sessionTestConfig()
	cfg.Backend = f.backend
	sessionTestUnit(cfg).info.backend = f.backend
	streams := map[int]*sessionTestStream{31: {closed: make(chan struct{})}, 32: {closed: make(chan struct{})}}
	var released []int
	ops := sessionTestOps(streams, &released)
	mode := func(family string) func(int, NPMode) error {
		return func(fd int, mode NPMode) error {
			if fd != cfg.UnitFD {
				t.Errorf("%s mode descriptor %d, want unit %d", family, fd, cfg.UnitFD)
			}
			f.trace = append(f.trace, fmt.Sprintf("%s:%d", family, mode))
			return f.modeErr
		}
	}
	ops.setIPv4Mode = mode("v4")
	ops.setIPv6Mode = mode("v6")
	ops.connect = func(fd, unit int) error {
		if fd != cfg.ChanFD || unit != cfg.UnitNum {
			t.Errorf("physical attach %d/%d", fd, unit)
		}
		f.trace = append(f.trace, "attach")
		return f.connectErr
	}
	ops.disconnect = func(fd int) error {
		if fd != cfg.ChanFD {
			t.Errorf("physical detach descriptor %d", fd)
		}
		f.trace = append(f.trace, "detach")
		return f.disconnectErr
	}
	_, handle, err := sessionTestNewSession(cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	owner, ok := handle.(*session)
	if !ok {
		t.Fatalf("kernel owner has type %T, want *session", handle)
	}
	f.owner = owner
	t.Cleanup(func() {
		if err := handle.Close(); err != nil {
			t.Error(err)
		}
	})
	return f
}

func requireFamilyTrace(t *testing.T, f *familyTestSession, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(f.trace, want) {
		t.Fatalf("operations %v, want %v", f.trace, want)
	}
	f.trace = nil
}

func TestKernelNCPAddressStagingDoesNotGrantTraffic(t *testing.T) {
	f := newFamilyTestSession(t)
	local, peer := netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.2/32")
	if err := f.owner.AddAddressP2P(local, peer); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "add:192.0.2.1/32")
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:2", "attach", "v4:1", "v6:2")
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:1", "v6:2")
}

func TestKernelNCPFamilyWithdrawalPrecedesAddressRemoval(t *testing.T) {
	f := newFamilyTestSession(t)
	v4, v4peer := netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.2/32")
	v6, v6peer := netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")
	for _, pair := range [][2]netip.Prefix{{v4, v4peer}, {v6, v6peer}} {
		if err := f.owner.AddAddressP2P(pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	f.trace = nil
	if err := f.owner.RemoveAddress(v4); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:2", "remove:192.0.2.1/32")
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:1")
}

func TestKernelNCPFailedWithdrawalCannotRegrantFamily(t *testing.T) {
	for _, failure := range []string{"mode", "address"} {
		t.Run(failure, func(t *testing.T) {
			f := newFamilyTestSession(t)
			local, peer := netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")
			if err := f.owner.AddAddressP2P(local, peer); err != nil {
				t.Fatal(err)
			}
			if err := f.owner.Connect(); err != nil {
				t.Fatal(err)
			}
			failed := errors.New("withdrawal failed")
			if failure == "mode" {
				f.modeErr = failed
			} else {
				f.backend.removeErr = failed
			}
			f.trace = nil
			if err := f.owner.RemoveAddress(local); !errors.Is(err, failed) {
				t.Fatalf("withdrawal: %v", err)
			}
			if failure == "mode" {
				requireFamilyTrace(t, f, "v6:2")
			} else {
				requireFamilyTrace(t, f, "v6:2", "remove:fe80::1/64")
			}
			f.modeErr, f.backend.removeErr = nil, nil
			if err := f.owner.AddAddressP2P(netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.2/32")); err != nil {
				t.Fatal(err)
			}
			f.trace = nil
			if err := f.owner.Connect(); err != nil {
				t.Fatal(err)
			}
			requireFamilyTrace(t, f, "v4:1", "v6:2")
			if err := f.owner.AddAddressP2P(local, peer); err == nil {
				t.Fatal("retiring address was silently reactivated")
			}
			if err := f.owner.RemoveAddress(local); err != nil {
				t.Fatal(err)
			}
			requireFamilyTrace(t, f, "v6:2", "remove:fe80::1/64")
		})
	}
}

func TestKernelNCPPartialAddressAddRemainsCleanupOnly(t *testing.T) {
	f := newFamilyTestSession(t)
	local, peer := netip.MustParsePrefix("fe80::1/64"), netip.MustParsePrefix("fe80::2/64")
	f.backend.addErr = errors.New("partial add")
	if err := f.owner.AddAddressP2P(local, peer); !errors.Is(err, f.backend.addErr) {
		t.Fatalf("add: %v", err)
	}
	f.trace = nil
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:2", "attach", "v4:2", "v6:2")
	if err := f.owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v6:2", "remove:fe80::1/64")
}

func TestKernelNCPAttachFailureAndDetachRetry(t *testing.T) {
	f := newFamilyTestSession(t)
	f.modeErr = errors.New("drop failed")
	if err := f.owner.Connect(); !errors.Is(err, f.modeErr) {
		t.Fatalf("mode error: %v", err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:2")
	f.modeErr = nil
	f.connectErr = errors.New("attach failed")
	if err := f.owner.Connect(); !errors.Is(err, f.connectErr) {
		t.Fatalf("attach error: %v", err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:2", "attach")
	f.connectErr = nil
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	f.trace = nil
	f.disconnectErr = errors.New("detach failed")
	if err := f.owner.Disconnect(); !errors.Is(err, f.disconnectErr) {
		t.Fatalf("detach error: %v", err)
	}
	requireFamilyTrace(t, f, "detach")
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "v4:2", "v6:2")
	f.disconnectErr = nil
	if err := f.owner.Disconnect(); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.Connect(); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "detach", "v4:2", "v6:2", "attach", "v4:2", "v6:2")
}

func TestKernelNCPMissingModeOperationsRefusesBeforeIO(t *testing.T) {
	for _, family := range []string{"v4", "v6"} {
		t.Run(family, func(t *testing.T) {
			cfg := sessionTestConfig()
			cfg.ChanFD, cfg.UnitFD = 0, 1
			sessionTestChannel(cfg).fd = 0
			sessionTestUnit(cfg).info.fd = 1
			var released []int
			ops := sessionTestOps(nil, &released)
			ops.setIPv4Mode = func(int, NPMode) error { return nil }
			ops.setIPv6Mode = func(int, NPMode) error { return nil }
			if family == "v4" {
				ops.setIPv4Mode = nil
			} else {
				ops.setIPv6Mode = nil
			}
			sessionTestUnit(cfg).beforeDescribe = func() { t.Fatal("missing mode callbacks reached original-unit I/O") }
			if _, _, err := sessionTestNewSession(cfg, ops); err == nil {
				t.Fatal("missing mode callback admitted")
			}
			if !reflect.DeepEqual(released, []int{0, 1}) {
				t.Fatalf("descriptor rollback %v", released)
			}
		})
	}
}

func TestKernelNCPAddressOwnershipIsBoundedAndExact(t *testing.T) {
	f := newFamilyTestSession(t)
	local, peer := netip.MustParsePrefix("192.0.2.1/32"), netip.MustParsePrefix("192.0.2.2/32")
	if err := f.owner.AddAddressP2P(local, netip.MustParsePrefix("fe80::2/64")); err == nil {
		t.Fatal("mixed-family pair admitted")
	}
	if len(f.trace) != 0 {
		t.Fatalf("invalid pair mutated backend: %v", f.trace)
	}
	if err := f.owner.AddAddressP2P(local, peer); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.AddAddressP2P(local, peer); err != nil {
		t.Fatal(err)
	}
	requireFamilyTrace(t, f, "add:192.0.2.1/32")
	if err := f.owner.AddAddressP2P(local, netip.MustParsePrefix("192.0.2.3/32")); err == nil {
		t.Fatal("owned local prefix accepted a different peer")
	}
	if err := f.owner.RemoveAddress(netip.MustParsePrefix("192.0.2.99/32")); err != nil {
		t.Fatalf("idempotent absent-address withdrawal: %v", err)
	}
	if len(f.trace) != 0 {
		t.Fatalf("ownership refusal mutated backend: %v", f.trace)
	}
	for i := range maxSessionAddresses - 1 {
		address := netip.AddrFrom4([4]byte{198, 18, byte(i / 256), byte(i % 256)})
		if err := f.owner.AddAddressP2P(netip.PrefixFrom(address, 32), peer); err != nil {
			t.Fatalf("pair %d: %v", i, err)
		}
	}
	f.trace = nil
	if err := f.owner.AddAddressP2P(netip.MustParsePrefix("203.0.113.1/32"), peer); err == nil {
		t.Fatal("ownership bound was exceeded")
	}
	if len(f.trace) != 0 {
		t.Fatalf("excess pair reached backend: %v", f.trace)
	}
	if err := f.owner.RemoveAddress(local); err != nil {
		t.Fatal(err)
	}
	if err := f.owner.AddAddressP2P(netip.MustParsePrefix("203.0.113.1/32"), peer); err != nil {
		t.Fatalf("released ownership slot was not reusable: %v", err)
	}
}
