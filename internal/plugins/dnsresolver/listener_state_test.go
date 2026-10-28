package dnsresolver

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestListenerStateByPort proves AC-31's listener state is kept per bound
// socket: two listeners on one IP with different ports are reported apart.
// Method: install two listeners on 127.0.0.1, each reports udp and tcp up;
// reload with only the first, which rebinds, and the dropped one must read
// down while the kept one reads up (a state keyed by the IP alone reads up
// for both); stop, and both read down. A listener never bound reads unknown.
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
	want(dropped, "down", "down")

	in.srv.stop()
	want(kept, "down", "down")
}
