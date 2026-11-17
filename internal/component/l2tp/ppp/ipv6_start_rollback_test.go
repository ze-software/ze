// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 carrier acquisition and rollback ownership
// Related: ipv6_service_start.go -- startIPv6Service

package ppp

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// A failed carrier acquisition leaves no address when rollback succeeds. The
// recorder applies successful address operations to a live-state map, so this
// checks remaining state rather than only a cleanup call count.
func TestIPv6ServiceUnavailableCarrierRollsBackNetworkState(t *testing.T) {
	backend := &ipv6RollbackBackend{fakeBackend: &fakeBackend{}, addresses: make(map[string]string)}
	handle := newTestSessionHandle(backend, "created-ppp")
	failure := errors.New("IPv6 carrier is unavailable")
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return nil, failure }
	service, err := startIPv6Service(serviceHandleConfig(handle), testServerID(), nil, slog.New(slog.DiscardHandler))
	if service != nil {
		t.Fatal("completed rollback retained a service owner")
	}
	if !errors.Is(err, failure) {
		t.Fatalf("carrier failure was lost: %v", err)
	}
	if len(backend.addresses) != 0 {
		t.Fatalf("carrier refusal retained addresses: %+v", backend.addresses)
	}
	if len(backend.RouteAddCalls()) != 0 {
		t.Fatal("unavailable carrier installed a delegated route")
	}
}

// A failed rollback must return its owner and retain both causes. That owner
// must retry the installed address through the existing Stop lifecycle.
func TestIPv6ServiceFailedAcquisitionRetainsCleanupOwner(t *testing.T) {
	backend := &ipv6RollbackBackend{fakeBackend: &fakeBackend{}, addresses: make(map[string]string)}
	handle := newTestSessionHandle(backend, "created-ppp")
	acquisition := errors.New("IPv6 carrier acquisition failed")
	withdrawal := errors.New("link-local withdrawal failed")
	backend.removeAddrErr = withdrawal
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) { return nil, acquisition }
	service, err := startIPv6Service(serviceHandleConfig(handle), testServerID(), nil, slog.New(slog.DiscardHandler))
	if !errors.Is(err, acquisition) {
		t.Errorf("carrier failure was lost: %v", err)
	}
	if !errors.Is(err, withdrawal) {
		t.Errorf("withdrawal failure was lost: %v", err)
	}
	if service == nil {
		t.Fatal("failed rollback discarded the owner of its installed address")
	}
	if !service.linkLocalInstalled {
		t.Fatal("failed rollback cleared its installed-address marker")
	}
	if len(backend.addresses) != 1 {
		t.Fatalf("failed rollback's live address state = %+v", backend.addresses)
	}
	backend.removeAddrErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if service.linkLocalInstalled {
		t.Fatal("successful retry retained the installed-address marker")
	}
	if len(backend.addresses) != 0 {
		t.Fatalf("successful retry retained addresses: %+v", backend.addresses)
	}
}

// The real session startup boundary must keep a partial owner despite the
// nonfatal service refusal, so an LCP restart can retry before reusing the unit.
func TestIPv6ServiceStartFailureRetainedUntilNCPReset(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	session, _, events := newRFC1661Session(LCPStateOpened)
	handle := testHandleFor(session)
	backend, ok := handle.backend.(*fakeBackend)
	if !ok {
		t.Fatalf("session backend = %T, want *fakeBackend", handle.backend)
	}
	backend.removeAddrErr = errors.New("link-local withdrawal failed")
	handle.openIPv6Carrier = func(context.Context) (IPv6Carrier, error) {
		return nil, errors.New("IPv6 carrier acquisition failed")
	}
	session.localInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 1}
	session.peerInterfaceID = [8]byte{2, 0, 0, 0, 0, 0, 0, 2}
	session.peerInterfaceIDNegotiated = true
	session.afterLCPOpenIPv6Service()
	if session.ipv6Svc == nil {
		t.Fatal("session discarded the failed IPv6 cleanup owner")
	}
	if len(events) != 0 {
		t.Fatal("nonfatal carrier refusal published a session event")
	}
	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("failed startup rollback reports = %+v", warnings)
	}
	if warnings[0].Code != diagnostic.CodeDoctorPPPCleanup {
		t.Fatalf("failed startup rollback code = %q", warnings[0].Code)
	}
	if warnings[0].Detail["interface"] != handle.Identity().InterfaceName {
		t.Fatalf("failed startup rollback identity = %v", warnings[0].Detail)
	}
	backend.removeAddrErr = nil
	if !session.resetNCP() {
		t.Fatal("NCP reset did not retry the retained IPv6 cleanup")
	}
	if session.ipv6Svc != nil {
		t.Fatal("successful NCP reset retained the stopped IPv6 service")
	}
	if len(backend.AddrRemoveCalls()) != 2 {
		t.Fatalf("address withdrawals = %+v, want refusal and successful retry", backend.AddrRemoveCalls())
	}
}

// ipv6RollbackBackend tracks installed addresses in these serial failed-start
// tests. It changes live state only after the underlying operation succeeds.
type ipv6RollbackBackend struct {
	*fakeBackend
	addresses map[string]string
}

func (backend *ipv6RollbackBackend) AddAddressP2P(name, local, peer string) error {
	if err := backend.fakeBackend.AddAddressP2P(name, local, peer); err != nil {
		return err
	}
	backend.addresses[name+"/"+local] = peer
	return nil
}

func (backend *ipv6RollbackBackend) RemoveAddress(name, local string) error {
	if err := backend.fakeBackend.RemoveAddress(name, local); err != nil {
		return err
	}
	delete(backend.addresses, name+"/"+local)
	return nil
}
