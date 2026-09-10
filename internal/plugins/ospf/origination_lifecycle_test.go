// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- deferred origination lifecycle.
// Related: docs/architecture/ospf/ospf-4-component-config.md -- interface enrollment and reload.

package ospf

import (
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// VALIDATES: passive-only and loopback-only engines publish a re-priced Router-LSA after
// MinLSInterval, whether their sole interface was present at startup or added by reload.
// PREVENTS: a rate-limited reload losing its new metric because no maintenance worker starts.
func TestReferenceBandwidthReloadPublishesAfterMinLSInterval(t *testing.T) {
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000})
	for _, mode := range []struct {
		name    string
		options string
	}{
		{name: "passive", options: `"passive":"true"`},
		{name: "loopback", options: `"network-type":"loopback"`},
	} {
		for _, enrollment := range []string{"initial", "reload-added"} {
			t.Run(mode.name+"/"+enrollment, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","reference-bandwidth":"100000","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0",`+mode.options+`}}}}}`), nil)
					if err != nil {
						t.Fatalf("parseOSPFConfig: %v", err)
					}
					eng := newEngine(transport.New(&fakeBackend{}))
					defer eng.shutdown()
					initial := cfg
					if enrollment == "reload-added" {
						initial.Interfaces = nil
					}
					eng.setConfig(initial)
					addressedTopology(eng)
					if err := eng.openInterfaces(); err != nil {
						t.Fatalf("openInterfaces: %v", err)
					}
					if enrollment == "reload-added" {
						eng.reconcile(cfg)
					}
					eng.originateSelfLSAs()
					synctest.Wait()
					if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 100 {
						t.Fatalf("initial Router-LSA metric = %d, want 100", got)
					}

					lowered := cfg
					lowered.ReferenceBandwidth = 10000
					eng.reconcile(lowered)
					synctest.Wait()
					if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 100 {
						t.Fatalf("Router-LSA metric immediately after reload = %d, want 100 during MinLSInterval", got)
					}
					// sleep(protocol): advance the virtual clock to just before RFC 2328's
					// five-second MinLSInterval, then let every ready worker finish.
					time.Sleep(5*time.Second - time.Nanosecond)
					synctest.Wait()
					if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 100 {
						t.Fatalf("Router-LSA metric before MinLSInterval = %d, want 100", got)
					}
					// sleep(protocol): cross MinLSInterval and allow one maintenance tick.
					// No test-triggered origination supplies the retry after the reload.
					time.Sleep(time.Second + time.Nanosecond)
					synctest.Wait()
					if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 10 {
						t.Errorf("Router-LSA metric after MinLSInterval = %d, want the re-priced 10", got)
					}
				})
			})
		}
	}
}

// selfRouterLinkCount returns the number of links in this router's own Router-LSA, and
// reports whether the LSDB holds that LSA at all.
func selfRouterLinkCount(t *testing.T, eng *engine, router types.RouterID) (int, bool) {
	t.Helper()
	key := types.LSAKey{Type: types.LSTypeRouter, LinkStateID: types.LinkStateID(router), AdvertisingRouter: router}
	lsa, ok := eng.lsdb.LookupLSA(types.BackboneArea, key)
	if !ok {
		return 0, false
	}
	body, err := lsa.DecodeRouter()
	if err != nil {
		t.Fatalf("DecodeRouter: %v", err)
	}
	return len(body.Links), true
}

// VALIDATES: shutdown waits for an active interface-down origination, then rejects late
// callbacks. A blocked topology read makes the join observable without inspecting workers.
// PREVENTS: a deferred origination outliving its engine and reading state its owner released.
func TestDeferredOriginationJoinsTheEngine(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","cost":"10"}}}}}`), nil)
		if err != nil {
			t.Fatalf("parseOSPFConfig: %v", err)
		}
		eng := newEngine(transport.New(&fakeBackend{}))
		eng.setConfig(cfg)
		eng.running["eth0"] = cfg.Interfaces[0]

		entered := make(chan struct{})
		release := make(chan struct{})
		releaseWork := sync.OnceFunc(func() { close(release) })
		var topologyCalls atomic.Uint64
		eng.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo {
			if topologyCalls.Add(1) == 1 {
				close(entered)
				<-release
			}
			topology := eng.lsdbTopology()
			for idx := range topology {
				topology[idx].Address = [4]byte{10, 0, 0, 1}
				topology[idx].NetworkMask = [4]byte{255, 255, 255, 0}
			}
			return topology
		})
		stop := make(chan struct{})
		stopped := make(chan struct{})
		startShutdown := sync.OnceFunc(func() { close(stop) })
		go func() {
			<-stop
			eng.shutdown()
			close(stopped)
		}()
		defer func() {
			releaseWork()
			startShutdown()
			<-stopped
		}()
		sink := nsmAdapter{table: eng.neighbors, onChange: eng.originateSelfLSAs, onChangeDeferred: eng.originateSelfLSAsDeferred, auth: eng.auth}

		sink.InterfaceDown("eth0")
		synctest.Wait()
		select {
		case <-entered:
		default:
			t.Fatal("InterfaceDown did not start a topology read")
		}
		startShutdown()
		<-eng.ctx.Done()
		synctest.Wait()
		select {
		case <-stopped:
			t.Fatal("shutdown returned while an origination still held a topology read")
		default:
		}

		releaseWork()
		<-stopped
		if links, ok := selfRouterLinkCount(t, eng, cfg.RouterID); !ok || links != 1 {
			t.Fatalf("Router-LSA after shutdown: present %v, links %d, want the completed origination's one link", ok, links)
		}
		if got := selfRouterLSAMetric(t, eng, cfg.RouterID); got != 10 {
			t.Errorf("completed Router-LSA metric = %d, want 10", got)
		}

		// Empty topology does not replace a Router-LSA, so its unchanged body alone cannot
		// prove suppression. Observe the topology callback itself after engine teardown.
		callsAtShutdown := topologyCalls.Load()
		delete(eng.running, "eth0")
		sink.InterfaceDown("eth0")
		synctest.Wait()
		if got := topologyCalls.Load(); got != callsAtShutdown {
			t.Errorf("topology reads after late InterfaceDown = %d, want %d at shutdown", got, callsAtShutdown)
		}
		if links, ok := selfRouterLinkCount(t, eng, cfg.RouterID); !ok || links != 1 {
			t.Errorf("Router-LSA after late InterfaceDown: present %v, links %d, want the retained one link", ok, links)
		}
	})
}
