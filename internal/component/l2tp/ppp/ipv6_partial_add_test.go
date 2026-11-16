// Design: docs/research/l2tpv2-ze-integration.md -- attempted IPv6 address ownership
// Related: ipv6_service_start.go -- startup rollback

package ppp

import (
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/rtproto"
)

// A backend may install its address before returning an error. Startup must
// withdraw that attempted pair, preserving the independent IPv4 address.
func TestIPv6ServicePartialAddressFailureRollsBack(t *testing.T) {
	backend := &ipv6PartialAddBackend{fakeBackend: &fakeBackend{}, addresses: make(map[string]string)}
	owner := newTestSessionHandle(backend, "created-ppp")
	local4 := netip.MustParsePrefix("192.0.2.1/32")
	if err := owner.AddAddressP2P(local4, netip.MustParsePrefix("192.0.2.2/32")); err != nil {
		t.Fatal(err)
	}
	backend.addFailure = errors.New("address installed before reply failure")
	service, err := startIPv6Service(serviceHandleConfig(owner), testServerID(), nil, slog.New(slog.DiscardHandler))
	if !errors.Is(err, backend.addFailure) {
		t.Fatalf("startup error = %v, want address-add failure", err)
	}
	if service != nil {
		t.Fatal("successful rollback retained an IPv6 owner")
	}
	if len(backend.addresses) != 1 || backend.addresses["created-ppp/"+local4.String()] == "" {
		t.Fatalf("live addresses after failed IPv6 startup = %v, want only IPv4", backend.addresses)
	}
}

// Failed removal must return the attempted-install owner, including both
// errors, so Stop can retry the exact pair before any replacement lifetime.
func TestIPv6ServicePartialAddressFailureRetainsRetry(t *testing.T) {
	backend := &ipv6PartialAddBackend{fakeBackend: &fakeBackend{}, addresses: make(map[string]string)}
	owner := newTestSessionHandle(backend, "created-ppp")
	backend.addFailure = errors.New("partially installed address")
	backend.removeAddrErr = errors.New("withdrawal refused")
	service, err := startIPv6Service(serviceHandleConfig(owner), testServerID(), nil, slog.New(slog.DiscardHandler))
	if !errors.Is(err, backend.addFailure) || !errors.Is(err, backend.removeAddrErr) {
		t.Fatalf("startup error = %v, want add and withdrawal causes", err)
	}
	if service == nil {
		t.Fatal("partial installation lost its cleanup owner")
	}
	if len(backend.addresses) != 1 {
		t.Fatalf("partial installation state = %v", backend.addresses)
	}
	backend.removeAddrErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if len(backend.addresses) != 0 {
		t.Fatalf("successful retry retained addresses: %v", backend.addresses)
	}
	if service.linkLocalInstalled {
		t.Fatal("successful retry retained attempted-install ownership")
	}
}

// An uncertain delegated-route add still owns the prefix and its withdrawal.
// Failed Stop must retain both until the native path is actually removed.
func TestIPv6ServicePartialRouteFailureRetainsPrefix(t *testing.T) {
	backend := &ipv6PartialRouteBackend{fakeBackend: &fakeBackend{}, routes: make(map[string]string)}
	owner := newTestSessionHandle(backend, "created-ppp")
	cfg := serviceHandleConfig(owner)
	releases := 0
	cfg.ReleasePrefix = func(netip.Prefix) { releases++ }
	service := newIPv6Service(cfg)
	prefix := netip.MustParsePrefix("2001:db8:100::/56")
	service.delegatedPrefix = prefix
	failure := errors.New("route installed before reply failure")
	backend.addFailure = failure
	if err := service.installRoute(prefix); !errors.Is(err, failure) {
		t.Fatalf("route error = %v", err)
	}
	backend.removeRouteErr = errors.New("route withdrawal refused")
	if err := service.Stop(); !errors.Is(err, backend.removeRouteErr) {
		t.Fatalf("cleanup error = %v, want withdrawal refusal", err)
	}
	if releases != 0 || service.delegatedPrefix != prefix || len(backend.routes) != 1 {
		t.Fatalf("failed withdrawal lost retained prefix: releases=%d prefix=%s paths=%v", releases, service.delegatedPrefix, backend.routes)
	}
	backend.removeRouteErr = nil
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if releases != 1 || service.delegatedPrefix.IsValid() || len(backend.routes) != 0 {
		t.Fatalf("successful withdrawal did not release exactly once: releases=%d prefix=%s paths=%v", releases, service.delegatedPrefix, backend.routes)
	}
}

// This serial fixture models an uncertain address-add result by installing
// first, then failing its reply. RemoveAddress remains independently fallible.
type ipv6PartialAddBackend struct {
	*fakeBackend
	addresses  map[string]string
	addFailure error
}

// A retrying client cannot overwrite the single exact prefix retained after
// an uncertain route add. The allocator must not be called for a second lease.
func TestIPv6ServiceRetainedRouteSurvivesSolicitRetry(t *testing.T) {
	backend := &ipv6PartialRouteBackend{fakeBackend: &fakeBackend{}, routes: make(map[string]string), addFailure: errors.New("unknown route-add outcome")}
	owner := newTestSessionHandle(backend, "created-ppp")
	service := newIPv6Service(serviceHandleConfig(owner))
	retained := netip.MustParsePrefix("2001:db8:100::/56")
	if err := service.installRoute(retained); !errors.Is(err, backend.addFailure) {
		t.Fatalf("route-add error = %v", err)
	}
	client := DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: []byte{2, 0, 0, 0, 0, 1}}
	message, err := parseDHCPv6(buildSolicit([3]byte{1, 2, 3}, client, 1))
	if err != nil {
		t.Fatal(err)
	}
	allocations := 0
	response, err := service.handleSolicit(message, testServerID(), func() (netip.Prefix, bool) { allocations++; return netip.MustParsePrefix("2001:db8:200::/56"), true })
	if err != nil {
		t.Fatal(err)
	}
	reply, err := parseDHCPv6(response)
	if err != nil {
		t.Fatal(err)
	}
	if allocations != 0 || service.delegatedPrefix != retained {
		t.Fatalf("retry replaced cleanup owner: allocations=%d prefix=%s", allocations, service.delegatedPrefix)
	}
	if reply.IAPD == nil || reply.IAPD.Prefix == nil || reply.IAPD.Prefix.Prefix != retained {
		t.Fatalf("retry did not advertise retained prefix: %+v", reply.IAPD)
	}
	if err := service.Stop(); err != nil {
		t.Fatal(err)
	}
	if len(backend.routes) != 0 {
		t.Fatalf("cleanup forgot original uncertain route: %v", backend.routes)
	}
}

func (backend *ipv6PartialAddBackend) AddAddressP2P(name, local, peer string) error {
	if err := backend.fakeBackend.AddAddressP2P(name, local, peer); err != nil {
		return err
	}
	backend.addresses[name+"/"+local] = peer
	return backend.addFailure
}

func (backend *ipv6PartialAddBackend) RemoveAddress(name, local string) error {
	if err := backend.fakeBackend.RemoveAddress(name, local); err != nil {
		return err
	}
	delete(backend.addresses, name+"/"+local)
	return nil
}

type ipv6PartialRouteBackend struct {
	*fakeBackend
	routes     map[string]string
	addFailure error
}

func (backend *ipv6PartialRouteBackend) AddRoute(name, prefix, gateway string, metric int, protocol rtproto.Proto) error {
	if err := backend.fakeBackend.AddRoute(name, prefix, gateway, metric, protocol); err != nil {
		return err
	}
	backend.routes[name+"/"+prefix] = gateway
	return backend.addFailure
}

func (backend *ipv6PartialRouteBackend) RemoveRoute(name, prefix, gateway string, metric int, protocol rtproto.Proto) error {
	if err := backend.fakeBackend.RemoveRoute(name, prefix, gateway, metric, protocol); err != nil {
		return err
	}
	delete(backend.routes, name+"/"+prefix)
	return nil
}
