package dnsresolver

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestListenerStateByPort proves AC-31's listener state is kept per bound
// socket: two listeners on one IP with different ports are reported apart.
// Method: install two listeners on 127.0.0.1, each reports udp and tcp up;
// reload with only the first, which rebinds, and the dropped one must read
// unknown while the kept one reads up (a state keyed by the IP alone reads up
// for both); stop, and the kept one reads down. A listener never bound reads
// unknown.
func TestListenerStateByPort(t *testing.T) {
	up := aUpstream(t, 0)
	cfg := testConfig(up)
	loopback := netip.MustParseAddr("127.0.0.1")
	kept := listenerEndpoint{Name: "kept", IP: loopback, Port: freeLoopbackPort(t)}
	dropped := listenerEndpoint{Name: "dropped", IP: loopback, Port: freeLoopbackPort(t)}
	for range 20 {
		if dropped.Port != kept.Port {
			break
		}
		dropped.Port = freeLoopbackPort(t)
	}
	if dropped.Port == kept.Port {
		t.Fatal("no second free port after 20 probes")
	}
	cfg.Listeners = []listenerEndpoint{kept, dropped}
	res := newTestResolver(t, cfg, newTestClock())
	log := slogutil.DiscardLogger()
	in := &resolverInstaller{srv: newResolverServer(log, res), res: res, log: log}
	t.Cleanup(in.srv.stop)

	state := func(l listenerEndpoint) plugin.Map {
		return res.listeners.show(netip.AddrPortFrom(l.IP, l.Port))
	}
	want := func(l listenerEndpoint, udp, tcp string) {
		t.Helper()
		got := state(l)
		if got["udp"] != udp {
			t.Errorf("%s udp = %v, want %s", l.Name, got["udp"], udp)
		}
		if got["tcp"] != tcp {
			t.Errorf("%s tcp = %v, want %s", l.Name, got["tcp"], tcp)
		}
	}

	want(kept, "unknown", "unknown")
	if err := in.install(cfg); err != nil {
		t.Fatalf("install: %v", err)
	}
	want(kept, "up", "up")
	want(dropped, "up", "up")

	next := cfg
	next.Listeners = []listenerEndpoint{kept}
	if err := in.install(next); err != nil {
		t.Fatalf("reload: %v", err)
	}
	want(kept, "up", "up")
	// The dropped listener was unbound for the last time: its state is
	// forgotten (unknown), never kept as down (a state keyed by the IP alone
	// reads up for it).
	want(dropped, "unknown", "unknown")

	in.srv.stop()
	want(kept, "down", "down")
}

// TestPruneDeletesGaugeSeries proves a listener the config no longer names
// leaves neither a state key nor a ze_dns_resolver_listener_up series, while
// a kept listener keeps both, down included. Method: record edges for a kept
// and a dropped socket under a Prometheus registry, prune to the kept one,
// then delete each series: the dropped one must already be gone.
func TestPruneDeletesGaugeSeries(t *testing.T) {
	setMetricsRegistry(metrics.NewPrometheusRegistry())
	t.Cleanup(func() { setMetricsRegistry(metrics.NopRegistry{}) })
	kept := netip.MustParseAddrPort("127.0.0.1:5301")
	dropped := netip.MustParseAddrPort("127.0.0.1:5302")
	var s listenerStates
	s.change(networkUDP, kept, false)
	s.change(networkUDP, dropped, true)
	s.change(networkTCP, dropped, false)

	s.prune([]listenerEndpoint{{Name: "kept", IP: kept.Addr(), Port: kept.Port()}})

	if got := s.show(kept)[networkUDP]; got != "down" {
		t.Errorf("kept udp = %v, want down", got)
	}
	for _, protocol := range []string{networkUDP, networkTCP} {
		if got := s.show(dropped)[protocol]; got != "unknown" {
			t.Errorf("dropped %s = %v, want unknown", protocol, got)
		}
		if rmetrics().listenerUp.Delete(protocol, dropped.String()) {
			t.Errorf("dropped %s gauge series survived prune", protocol)
		}
	}
	if !rmetrics().listenerUp.Delete(networkUDP, kept.String()) {
		t.Error("kept udp gauge series was deleted")
	}
}
