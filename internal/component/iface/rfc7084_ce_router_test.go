// Design: docs/features/interfaces.md -- IPv6 customer edge router provisioning
//
// RFC 7084 Section 4.2 and 4.3 obligations an IPv6 CE router meets through the
// interface component: SLAAC turned on for the WAN unit through the sysctls Ze
// writes, and a ULA prefix taken from the configuration alone.
//
// VALIDATES: a unit that sets ipv6 autoconf and accept-ra emits both kernel
// sysctls for its device; a configured ULA prefix is advertised exactly as
// written, on every parse of the same configuration.
// PREVENTS: a WAN unit left without address autoconfiguration, and a ULA
// prefix that differs between two boots of one configuration.

package iface

import (
	"encoding/json"
	"log/slog"
	"net/netip"
	"testing"

	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	"github.com/ze-software/ze/internal/core/rtproto"
)

// TestRFC7084WANUnitTurnsOnSLAAC applies the sysctls of a WAN unit that asks
// for SLAAC and for Router Advertisements while forwarding.
func TestRFC7084WANUnitTurnsOnSLAAC(t *testing.T) {
	// RFC requirement: RFC7084-4.2-11 positive -- a unit with ipv6 autoconf true emits net.ipv6.conf.eth0.autoconf 1, the kernel switch for SLAAC on that device.
	// RFC requirement: RFC7084-4.2-1 positive -- the same unit with accept-ra 2 emits net.ipv6.conf.eth0.accept_ra 2, so the device keeps acting on Router Advertisements as a host while the router forwards.
	bus := &testEventBus{}
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(nil) })

	cfg := mustParseIfaceJSON(t, `{"interface": {"ethernet": {"eth0": {"unit": {"0": {
		"ipv6": {"autoconf": "true", "accept-ra": "2"}
	}}}}}}`)
	if len(cfg.Ethernet) != 1 || len(cfg.Ethernet[0].Units) != 1 {
		t.Fatalf("parsed %d ethernet entries, want one with one unit", len(cfg.Ethernet))
	}
	applySysctl("eth0", cfg.Ethernet[0].Units[0])

	got := make(map[string]string)
	for _, e := range bus.emissions {
		if e.namespace != sysctlevents.Namespace {
			continue
		}
		payload, ok := e.data.(string)
		if !ok {
			t.Fatalf("sysctl payload is %T, want the JSON string applySysctl emits", e.data)
		}
		var kv struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal([]byte(payload), &kv); err != nil {
			t.Fatalf("sysctl payload %q: %v", e.data, err)
		}
		got[kv.Key] = kv.Value
	}
	if v := got["net.ipv6.conf.eth0.autoconf"]; v != "1" {
		t.Errorf("autoconf = %q, want 1 (emitted %v)", v, got)
	}
	if v := got["net.ipv6.conf.eth0.accept_ra"]; v != "2" {
		t.Errorf("accept_ra = %q, want 2 (emitted %v)", v, got)
	}
}

// TestRFC7084ULAPrefixComesFromTheConfigAlone parses one configuration twice,
// as two boots load it, and compares the advertised ULA prefix.
func TestRFC7084ULAPrefixComesFromTheConfigAlone(t *testing.T) {
	// RFC requirement: RFC7084-4.3-1 positive -- a ULA prefix fd12:3456:789a:1::/64 configured on an advertising unit is the advertised prefix on each of two parses of that configuration, since nothing but the configuration supplies it.
	const body = `{"enabled": "true", "prefix": {"fd12:3456:789a:1::/64": {}}}`
	want := netip.MustParsePrefix("fd12:3456:789a:1::/64")
	for boot := range 2 {
		ra := mustParseRA(t, body)
		if len(ra.Prefixes) != 1 {
			t.Fatalf("boot %d: %d prefixes, want the one configured", boot, len(ra.Prefixes))
		}
		if got := ra.Prefixes[0].Prefix; got != want {
			t.Errorf("boot %d: advertised prefix %v, want %v", boot, got, want)
		}
	}
}

// TestRFC7084DiscoveredRouterBecomesTheDefaultRoute has ze own the IPv6
// default route of a unit with route-priority, then reports a router that
// Router Discovery found on it.
func TestRFC7084DiscoveredRouterBecomesTheDefaultRoute(t *testing.T) {
	// RFC requirement: RFC7084-4.2-4 positive -- a router discovered on eth0 makes ze install ::/0 via that router, fe80::1, at the unit's route-priority 100, once ze has taken the default route from the kernel.
	fb := setupFakeBackendForTest(t)
	cfg := slaacUnitJSON(t)
	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{}
	suppressRAForConfig(cfg, suppressed, routers, priorities, &testEventBus{}, slog.New(slog.DiscardHandler))
	if !suppressed["eth0"] {
		t.Fatal("ze did not take the IPv6 default route of eth0 from the kernel, so the case is not the one under test")
	}

	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, priorities, slog.New(slog.DiscardHandler))

	want := routeCall{"eth0", "::/0", "fe80::1", 100, rtproto.Iface}
	if len(fb.routeAdds) != 1 || fb.routeAdds[0] != want {
		t.Errorf("routes added %v, want exactly %v", fb.routeAdds, want)
	}
}
