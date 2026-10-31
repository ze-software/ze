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
