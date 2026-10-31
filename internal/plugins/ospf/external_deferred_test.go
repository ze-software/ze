// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- a MinLSInterval-deferred external LSA.

package ospf

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestOSPFDeferredExternalOriginatesWithNoInterface proves an AS-External-LSA
// whose origination RFC 2328 Section 12.4 MinLSInterval deferred is generated
// later even when no interface was ever enrolled. Method: run the real OSPF
// plugin over a pipe with no interface and a 100 ms MinLSInterval, offer a
// redistributed route, withdraw it (the flush bypasses MinLSInterval and
// leaves the copy at MaxAge), then offer it again at once, inside
// MinLSInterval, so the origination is deferred. The LSDB must hold the LSA
// live again within a few maintenance ticks. Before the fix only interface
// enrolment started the maintenance worker, so the MaxAge copy stayed until an
// interface opened.
func TestOSPFDeferredExternalOriginatesWithNoInterface(t *testing.T) {
	const cfg = `{"ospf":{"router-id":"10.0.0.1","timers":{"min-ls-interval-ms":100},"filter":{"export":{"final":"accept"}}}}`
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() { ospfredistribute.SetExportChain(nil) })
	previous, hadPrevious := configredist.LookupConsumer(ospfredistribute.ConsumerName)
	t.Cleanup(func() {
		if hadPrevious {
			configredist.ReregisterConsumer(previous)
		}
	})

	pluginEnd, engineEnd := net.Pipe()
	eng := &ospfRollbackEngine{mux: rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)), seen: make(chan string, 64)}
	go eng.serve(ctx)
	done := make(chan struct{})
	go func() {
		runOSPFEngine(pluginEnd)
		close(done)
	}()
	t.Cleanup(func() {
		_ = eng.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("OSPF engine did not exit after its transport closed")
		}
	})

	eng.waitFor(t, ctx, "ze-plugin-engine:declare-registration")
	eng.callback(t, ctx, "ze-plugin-callback:configure",
		rpc.ConfigureInput{Sections: []rpc.ConfigSection{{Root: Namespace, Data: cfg}}})
	eng.waitFor(t, ctx, "ze-plugin-engine:declare-capabilities")
	eng.callback(t, ctx, "ze-plugin-callback:share-registry", rpc.ShareRegistryInput{})
	eng.waitFor(t, ctx, "ze-plugin-engine:ready")

	var consumer configredist.RedistConsumer
	for consumer == nil {
		if c, ok := configredist.LookupConsumer(ospfredistribute.ConsumerName); ok && (!hadPrevious || c != previous) {
			consumer = c
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("OnStarted never registered the OSPF redistribution consumer")
		case <-time.After(10 * time.Millisecond):
		}
	}

	prefix := netip.MustParsePrefix("192.0.2.1/32")
	entry := configredist.RouteEntry{Prefix: prefix.String(), Source: "static"}
	entry.Route.Prefix = prefix
	consumer.InjectRoute(ctx, family.IPv4Unicast, entry)
	if !eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("the offered route was not originated")
	}
	consumer.WithdrawRoute(ctx, family.IPv4Unicast, prefix.String())
	if eng.advertises(t, ctx, prefix.Addr().String()) {
		t.Fatal("the withdrawn route is still advertised")
	}
	consumer.InjectRoute(ctx, family.IPv4Unicast, entry)

	// The maintenance worker ticks once a second; three ticks is the bound.
	deadline := time.Now().Add(3500 * time.Millisecond)
	for !eng.advertises(t, ctx, prefix.Addr().String()) {
		if time.Now().After(deadline) {
			t.Fatal("the deferred AS-External-LSA was never originated: the LSDB keeps the MaxAge copy")
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// TestOSPFv3DeferredExternalOriginatesWithNoInterface is the OSPFv3 twin of
// TestOSPFDeferredExternalOriginatesWithNoInterface: it proves v6InjectExternal
// generates an AS-External-LSA whose origination RFC 2328 Section 12.4
// MinLSInterval (kept for OSPFv3 by RFC 5340) deferred, with no interface ever
// enrolled. Method: configure an IPv6-unicast engine through the v6 engine set,
// open no interface, set a 100 ms MinLSInterval, then drive the redistribution
// injector the plugin wires (v6InjectorAF). Offer, withdraw and offer a prefix
// again: the OSPFv3 withdraw forgets the prefix's Link State ID, so the second
// offer takes a fresh one and goes live at once. Then offer it with a new
// metric inside MinLSInterval: that instance of the same LSA is deferred, and
// the LSDB must carry the new metric within a few maintenance ticks. Without
// the fix nothing starts the maintenance worker with no interface, so the
// stale metric stays until an interface opens.
func TestOSPFv3DeferredExternalOriginatesWithNoInterface(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1",`+
		`"address-family":{"ipv6":{"instance-id":0,"areas":{"area":{"0":{"area-id":"0"}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	families := cfg.v6Families()
	if len(families) != 1 || families[0].af != afIPv6Unicast {
		t.Fatalf("config produced families %v, want the IPv6-unicast one alone", families)
	}
	families[0].cfg.Timers.MinLSIntervalMS = 100
	v6set := newV6EngineSet()
	t.Cleanup(v6set.shutdownAll)
	v6set.configure(families, false)
	eng, ok := v6set.engineFor(afIPv6Unicast)
	if !ok {
		t.Fatal("the v6 engine set spawned no IPv6-unicast engine")
	}
	if n := len(eng.interfaceSnapshot()); n != 0 {
		t.Fatalf("the engine has %d interface(s), want none", n)
	}
	router := families[0].cfg.RouterID
	inj := v6InjectorAF{set: v6set, af: afIPv6Unicast}

	prefix := netip.MustParsePrefix("2001:db8:5::/64")
	first := ospfredistribute.External{Metric: 20, Type2: true}
	if err := inj.InjectExternal(prefix, "static", first); err != nil {
		t.Fatalf("first offer: %v", err)
	}
	if metric, live := liveV6ExternalMetric(t, eng, router, prefix); !live || metric != 20 {
		t.Fatalf("the offered route is live=%v with metric %d, want live with 20", live, metric)
	}
	if _, err := inj.WithdrawExternal(prefix); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if n := selfV6ExternalCount(eng, router); n != 0 {
		t.Fatalf("the withdrawn route still has %d live AS-External-LSA(s)", n)
	}
	if err := inj.InjectExternal(prefix, "static", first); err != nil {
		t.Fatalf("second offer: %v", err)
	}
	if metric, live := liveV6ExternalMetric(t, eng, router, prefix); !live || metric != 20 {
		t.Fatalf("the re-offered route is live=%v with metric %d, want live with 20", live, metric)
	}
	if err := inj.InjectExternal(prefix, "static", ospfredistribute.External{Metric: 30, Type2: true}); err != nil {
		t.Fatalf("changed offer: %v", err)
	}
	if metric, _ := liveV6ExternalMetric(t, eng, router, prefix); metric != 20 {
		t.Fatalf("the change inside MinLSInterval installed metric %d at once: nothing was deferred", metric)
	}

	// The maintenance worker ticks once a second; three ticks is the bound.
	deadline := time.Now().Add(3500 * time.Millisecond)
	for {
		metric, live := liveV6ExternalMetric(t, eng, router, prefix)
		if live && metric == 30 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the deferred OSPFv3 AS-External-LSA was never originated: live=%v metric %d, want 30", live, metric)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// liveV6ExternalMetric reads the metric of the OSPFv3 AS-External-LSA eng
// originated for prefix, and whether that instance is live (not MaxAge).
func liveV6ExternalMetric(t *testing.T, eng *engine, router types.RouterID, prefix netip.Prefix) (uint32, bool) {
	t.Helper()
	eng.mu.Lock()
	lsid, ok := eng.redistV6[prefix.Masked()]
	eng.mu.Unlock()
	if !ok {
		t.Fatalf("no Link State ID is assigned to %s", prefix)
	}
	lsa, found := eng.lsdb.LookupLSA(types.BackboneArea, v6ExternalKey(router, lsid))
	if !found {
		return 0, false
	}
	decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
	if err != nil {
		t.Fatalf("DecodeLSA: %v", err)
	}
	body, err := decoded.DecodeExternal()
	if err != nil {
		t.Fatalf("DecodeExternal: %v", err)
	}
	return body.Metric, !decoded.Header.Age.IsMaxAge()
}
