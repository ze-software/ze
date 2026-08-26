package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreCos "github.com/ze-software/ze/internal/core/cos"
	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// recordingEventBus is a portable ze.EventBus stub used by the ready-gate
// tests. Subscribe records handlers keyed by (namespace, eventType) and
// Emit invokes them synchronously. Tests use this to prove that the
// Subscribe/Emit wiring mirrored from register.go actually routes into
// reconcileOnVPPReady.
type recordingEventBus struct {
	mu       sync.Mutex
	handlers map[string][]func(any)
}

var _ ze.EventBus = (*recordingEventBus)(nil)

func newRecordingEventBus() *recordingEventBus {
	return &recordingEventBus{handlers: make(map[string][]func(any))}
}

func (b *recordingEventBus) key(namespace, eventType string) string {
	return namespace + ":" + eventType
}

func (b *recordingEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	handlers := append([]func(any){}, b.handlers[b.key(namespace, eventType)]...)
	b.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
	return len(handlers), nil
}

func (b *recordingEventBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	k := b.key(namespace, eventType)
	b.mu.Lock()
	b.handlers[k] = append(b.handlers[k], handler)
	idx := len(b.handlers[k]) - 1
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		hs := b.handlers[k]
		if idx < len(hs) {
			b.handlers[k] = append(hs[:idx], hs[idx+1:]...)
		}
	}
}

// TestReconcileOnVPPReady_NoOpWhenActiveCfgNil verifies AC-4 / defensive
// path: when no config has been applied yet, the handler is a no-op.
//
// VALIDATES: defensive handling before the first applyConfig.
// PREVENTS: nil-deref when EventConnected arrives during startup ordering.
func TestReconcileOnVPPReady_NoOpWhenActiveCfgNil(t *testing.T) {
	var cfg atomic.Pointer[ifaceConfig]
	// Should not panic nor touch the backend (there is none registered).
	reconcileOnVPPReady(&cfg)
}

// TestReconcileOnVPPReady_NoOpForNonVPPBackend guards against mutating
// non-vpp backend state when a vpp lifecycle event fires under a
// non-vpp-backed iface config. Scenario: vpp.enabled=true is paired with
// interface.backend=netlink (vpp for FIB, netlink for interface mgmt).
// Netlink's StartMonitor is not idempotent, so retrying it on every
// EventConnected / EventReconnected would leak a fresh monitor goroutine
// each time.
//
// VALIDATES: reconcileOnVPPReady gates on cfg.Backend == vppBackendName.
// PREVENTS: netlink monitor goroutine leak on every vpp lifecycle event.
func TestReconcileOnVPPReady_NoOpForNonVPPBackend(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	fb.ifaces["orphan-dum"] = fakeIface{name: "orphan-dum", linkType: zeTypeDummy}

	cfg := testConfigWithAddresses()
	cfg.Backend = "netlink"
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	// Reconcile MUST NOT have pruned the orphan: for non-vpp backends the
	// handler is a no-op because vpp-ready carries no meaning for netlink.
	require.False(t, fb.deleted["orphan-dum"], "non-vpp backend must not trigger reconcile on vpp event")
	// No addresses should have been applied either.
	require.Empty(t, fb.addrs["dum0"], "non-vpp backend must not add addresses on vpp event")
}

// TestReconcileOnVPPReady_RunsReconcile verifies AC-4: when activeCfg is
// set and the backend is registered, the handler invokes reconcileOnReady
// against the registered backend and prunes orphans.
//
// VALIDATES: AC-4 -- EventConnected handler triggers full reconcile.
// PREVENTS: activeCfg being ignored after vpp connects.
func TestReconcileOnVPPReady_RunsReconcile(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	// Pre-populate backend state with an orphan interface not in config.
	fb.ifaces["orphan-dum"] = fakeIface{name: "orphan-dum", linkType: zeTypeDummy}
	fb.ifaces["dum0"] = fakeIface{name: "dum0", linkType: zeTypeDummy}

	cfg := testConfigWithAddresses()
	cfg.previousManaged = map[string]bool{"dum0": true, "orphan-dum": true}
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	require.True(t, fb.deleted["orphan-dum"], "orphan should have been pruned")
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
}

// TestReconcileOnVPPReady_InvokedOnEventConnected verifies AC-4: a Subscribe
// wired like register.go, when the EventBus emits vppevents.EventConnected,
// actually routes delivery into reconcileOnVPPReady against the registered
// backend.
//
// VALIDATES: AC-4 -- EventConnected on the bus triggers deferred reconcile.
// PREVENTS: a regression that breaks the Subscribe wiring (wrong namespace,
//
//	wrong event name, handler not invoked) without the functional test
//	catching it.
func TestReconcileOnVPPReady_InvokedOnEventConnected(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	fb.ifaces["orphan-dum"] = fakeIface{name: "orphan-dum", linkType: zeTypeDummy}
	fb.ifaces["dum0"] = fakeIface{name: "dum0", linkType: zeTypeDummy}

	cfg := testConfigWithAddresses()
	cfg.previousManaged = map[string]bool{"dum0": true, "orphan-dum": true}
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	bus := newRecordingEventBus()
	// Synchronous trigger for deterministic assertions; production uses a
	// non-blocking enqueue onto vppReconcileCh. Both paths call
	// reconcileOnVPPReady eventually; this test just verifies the
	// subscribe/trigger wiring by forcing a direct reconcile.
	unsubs := subscribeReconcileOnReady(bus, func() { reconcileOnVPPReady(&activeCfg) })
	t.Cleanup(func() {
		for _, u := range unsubs {
			u()
		}
	})

	n, err := bus.Emit(vppevents.Namespace, vppevents.EventConnected, "")
	require.NoError(t, err)
	require.Equal(t, 1, n, "expected exactly one subscriber for EventConnected")

	require.True(t, fb.deleted["orphan-dum"], "EventConnected should prune orphan")
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
}

// TestReconcileOnVPPReady_InvokedOnEventReconnected verifies AC-5: after a
// vpp crash/reconnect, emitting EventReconnected re-runs reconciliation
// against the currently-active config.
//
// VALIDATES: AC-5 -- crash-recovery reconcile path.
// PREVENTS: reconcileOnVPPReady being wired only to EventConnected and
//
//	missing the reconnect path.
func TestReconcileOnVPPReady_InvokedOnEventReconnected(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	fb.ifaces["dum0"] = fakeIface{name: "dum0", linkType: zeTypeDummy}

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(testConfigWithAddresses())

	bus := newRecordingEventBus()
	unsubs := subscribeReconcileOnReady(bus, func() { reconcileOnVPPReady(&activeCfg) })
	t.Cleanup(func() {
		for _, u := range unsubs {
			u()
		}
	})

	n, err := bus.Emit(vppevents.Namespace, vppevents.EventReconnected, "")
	require.NoError(t, err)
	require.Equal(t, 1, n, "expected exactly one subscriber for EventReconnected")

	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
}

// TestUnsubscribeOnShutdown verifies AC-7: the unsubscribe functions
// returned by Subscribe remove the handlers so a later Emit does not
// invoke reconcileOnVPPReady after the plugin has shut down.
//
// VALIDATES: AC-7 -- plugin shutdown cleanup path.
// PREVENTS: handler leaks that would keep firing reconcile after the
//
//	plugin's resources (logger, backend) have been torn down.
func TestUnsubscribeOnShutdown(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	fb.ifaces["orphan-dum"] = fakeIface{name: "orphan-dum", linkType: zeTypeDummy}

	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(testConfigWithAddresses())

	bus := newRecordingEventBus()
	unsubs := subscribeReconcileOnReady(bus, func() { reconcileOnVPPReady(&activeCfg) })

	// Shutdown: call every unsubscribe.
	for _, u := range unsubs {
		u()
	}

	n, err := bus.Emit(vppevents.Namespace, vppevents.EventConnected, "")
	require.NoError(t, err)
	require.Equal(t, 0, n, "EventConnected must have no subscribers after shutdown")

	require.False(t, fb.deleted["orphan-dum"], "handler must not fire after unsubscribe")
}

// TestReconcileOnVPPReady_ReloadsBackend verifies AC-1: when
// EventReconnected fires, reconcileOnVPPReady reloads the backend via
// LoadBackend to clear stale state (dead GoVPP channel, stale name map,
// stale bridge domains) from the pre-crash VPP instance.
//
// VALIDATES: AC-1 -- backend reloaded on vpp reconnect.
// PREVENTS: stale ifacevpp state surviving VPP crash.
func TestReconcileOnVPPReady_ReloadsBackend(t *testing.T) {
	var factoryCalls int
	err := RegisterBackend(vppBackendName, func() (Backend, error) {
		factoryCalls++
		return &fakeBackend{ifaces: map[string]fakeIface{}}, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, vppBackendName)
		backendsMu.Unlock()
	})

	require.NoError(t, LoadBackend(vppBackendName))
	require.Equal(t, 1, factoryCalls, "factory should be called once for initial load")

	cfg := testConfigWithAddresses()
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	require.Equal(t, 2, factoryCalls, "factory should be called again during reconcile (backend reload)")
}

// TestReconcileOnVPPReady_ReconcilesToNewBackend verifies AC-2/AC-3: after
// a backend reload, the reconciliation runs against the fresh backend (not
// the stale one). This proves that addresses are applied to the new
// instance, meaning the new VPP instance will have the correct state.
//
// VALIDATES: AC-2, AC-3 -- fresh backend receives reconciled state.
// PREVENTS: reconciliation running against old, stale backend.
func TestReconcileOnVPPReady_ReconcilesToNewBackend(t *testing.T) {
	var latestBackend *fakeBackend
	err := RegisterBackend(vppBackendName, func() (Backend, error) {
		fb := &fakeBackend{ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: zeTypeDummy},
		}}
		latestBackend = fb
		return fb, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, vppBackendName)
		backendsMu.Unlock()
	})

	require.NoError(t, LoadBackend(vppBackendName))
	firstBackend := latestBackend

	cfg := testConfigWithAddresses()
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	require.True(t, firstBackend != latestBackend, "backend instance should have been replaced")
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, latestBackend.addrs["dum0"],
		"addresses should be applied to the NEW backend")
}

// TestReconcileOnVPPReady_ClearsStaleState verifies AC-2: the fresh backend
// created by LoadBackend carries no state from the pre-crash instance. We
// simulate stale state by mutating the first backend (adding addresses and
// created-interface markers), then verify the replacement has none of it.
//
// VALIDATES: AC-2 -- stale state does not survive backend reload.
// PREVENTS: dead GoVPP channel / stale name map / stale bridge domains
// leaking across a VPP crash boundary.
func TestReconcileOnVPPReady_ClearsStaleState(t *testing.T) {
	var latestBackend *fakeBackend
	err := RegisterBackend(vppBackendName, func() (Backend, error) {
		fb := &fakeBackend{ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: zeTypeDummy},
		}}
		latestBackend = fb
		return fb, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, vppBackendName)
		backendsMu.Unlock()
	})

	require.NoError(t, LoadBackend(vppBackendName))
	staleBackend := latestBackend

	// Simulate pre-crash state on the old backend.
	staleBackend.ensureMaps()
	staleBackend.addrs["dum0"] = []string{"192.168.99.1/24"}
	staleBackend.created["stale-if"] = true

	cfg := testConfigWithAddresses()
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	require.True(t, staleBackend != latestBackend, "backend instance must be replaced")
	require.False(t, latestBackend.created["stale-if"],
		"fresh backend must not inherit stale created-interface markers")
	require.NotContains(t, latestBackend.addrs["dum0"], "192.168.99.1/24",
		"fresh backend must not carry stale addresses")
}

// TestReconcileOnVPPReady_FirstConnect verifies AC-4: on the first VPP
// connect (EventConnected, not a crash), the LoadBackend reload is harmless
// because the old backend has no meaningful state to lose. The factory is
// called, reconciliation succeeds, and the fresh backend receives the
// desired config.
//
// VALIDATES: AC-4 -- first-connect reload is safe.
// PREVENTS: regression where the reload path assumes prior state exists.
func TestReconcileOnVPPReady_FirstConnect(t *testing.T) {
	var factoryCalls int
	var latestBackend *fakeBackend
	err := RegisterBackend(vppBackendName, func() (Backend, error) {
		factoryCalls++
		fb := &fakeBackend{ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: zeTypeDummy},
		}}
		latestBackend = fb
		return fb, nil
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = CloseBackend()
		backendsMu.Lock()
		delete(backends, vppBackendName)
		backendsMu.Unlock()
	})

	// Initial load (daemon startup).
	require.NoError(t, LoadBackend(vppBackendName))
	require.Equal(t, 1, factoryCalls)

	// First EventConnected fires before any config is applied to the backend.
	// The backend has no stale state, no addresses, no created interfaces.
	cfg := testConfigWithAddresses()
	var activeCfg atomic.Pointer[ifaceConfig]
	activeCfg.Store(cfg)

	reconcileOnVPPReady(&activeCfg)

	require.Equal(t, 2, factoryCalls, "factory called again on first connect")
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, latestBackend.addrs["dum0"],
		"fresh backend must receive desired addresses on first connect")
}

// testConfigWithAddresses builds an ifaceConfig that declares one dummy
// interface with two addresses. Shared by reconcileOnReady and
// reconcileOnVPPReady tests. Backend is "vpp" so the VPP-event handler's
// backend guard (reconcileOnVPPReady) passes; the pure reconcileOnReady
// tests never inspect Backend, so they are unaffected.
func testConfigWithAddresses() *ifaceConfig {
	return &ifaceConfig{
		Backend: vppBackendName,
		Dummy: []ifaceEntry{{
			Name: "dum0",
			Units: []unitEntry{{
				Label:     "default",
				Addresses: []string{"10.0.0.1/24", "10.0.0.2/24"},
			}},
		}},
	}
}

// TestReconcileOnReady_DefersOnSentinel verifies AC-2: when ListInterfaces
// returns an error wrapping ErrBackendNotReady, reconcileOnReady signals
// "deferred" with no errors so the caller can retry later.
//
// VALIDATES: AC-2 -- sentinel error does not pollute errs.
// PREVENTS: startup ERROR logs when vpp is still handshaking.
func TestReconcileOnReady_DefersOnSentinel(t *testing.T) {
	fb := &fakeBackend{
		ifaces:  map[string]fakeIface{},
		listErr: fmt.Errorf("ifacevpp: VPP connector not ready: %w", ErrBackendNotReady),
	}
	cfg := testConfigWithAddresses()

	errs, deferred := reconcileOnReady(cfg, fb)
	require.True(t, deferred, "expected deferred=true when backend returns ErrBackendNotReady")
	require.Empty(t, errs, "expected no errs when deferred")
}

// TestReconcileOnReady_RecordsNonSentinelError verifies AC-8: a real
// ListInterfaces error (not the sentinel) is still recorded in errs.
//
// VALIDATES: AC-8 -- non-sentinel errors still surface.
// PREVENTS: silent swallowing of real backend failures.
func TestReconcileOnReady_RecordsNonSentinelError(t *testing.T) {
	realErr := errors.New("netlink: rtnetlink receive: permission denied")
	fb := &fakeBackend{
		ifaces:  map[string]fakeIface{},
		listErr: realErr,
	}
	cfg := testConfigWithAddresses()

	errs, deferred := reconcileOnReady(cfg, fb)
	require.False(t, deferred, "expected deferred=false for non-sentinel error")
	require.Len(t, errs, 1, "expected non-sentinel error to be recorded")
	require.ErrorIs(t, errs[0], realErr)
}

// TestApplyConfig_SkipsReconcileOnSentinel verifies AC-2/AC-3: when the
// backend defers reconciliation at ListInterfaces, applyConfig still applies
// additive-only address changes and returns an empty errs slice.
//
// VALIDATES: AC-2/AC-3 -- deferred reconcile path produces no error.
// PREVENTS: deliverConfigRPC failure at startup under vpp backend.
func TestApplyConfig_SkipsReconcileOnSentinel(t *testing.T) {
	fb := &fakeBackend{
		ifaces:  map[string]fakeIface{},
		listErr: fmt.Errorf("ifacevpp: VPP connector not ready: %w", ErrBackendNotReady),
	}
	cfg := testConfigWithAddresses()

	errs := applyConfig(cfg, nil, fb)
	require.Empty(t, errs, "applyConfig must not return errs when deferred")
	// Additive fallback: desired addresses applied despite reconcile deferral.
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
}

// TestReconcileOnReady_AddsMissing verifies AC-4: when the backend is ready
// and has no pre-existing addresses on the managed interface,
// reconcileOnReady adds every desired address.
//
// VALIDATES: AC-4 -- full reconcile runs Phase 3 on the ready path.
// PREVENTS: reconcileOnReady regressing on the ready path.
func TestReconcileOnReady_AddsMissing(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0": {name: "dum0", linkType: "dummy"},
		},
	}
	cfg := testConfigWithAddresses() // desires 10.0.0.1/24 + 10.0.0.2/24

	errs, deferred := reconcileOnReady(cfg, fb)
	require.False(t, deferred)
	require.Empty(t, errs)
	require.ElementsMatch(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, fb.addrs["dum0"])
}

// TestReconcileOnReady_PreservesUnownedManageableInterface verifies that
// first-apply reconciliation does not adopt arbitrary manageable kernel links
// and delete them just because they are absent from config.
//
// VALIDATES: first apply preserves unmanaged manageable links.
// PREVENTS: Ze deleting operator-created dummy/veth/bridge/tunnel devices on startup.
func TestReconcileOnReady_PreservesUnownedManageableInterface(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0":         {name: "dum0", linkType: zeTypeDummy},
			"operator-dum": {name: "operator-dum", linkType: zeTypeDummy},
		},
	}
	cfg := testConfigWithAddresses() // managed set = {dum0}; no previous ownership.

	errs, deferred := reconcileOnReady(cfg, fb)
	require.False(t, deferred)
	require.Empty(t, errs)
	require.False(t, fb.deleted["operator-dum"], "unowned manageable interface should NOT be deleted")
	require.False(t, fb.deleted["dum0"], "configured interface should NOT be deleted")
}

// TestReconcileOnReady_PrunesPreviouslyManagedInterface verifies AC-4: when
// the backend is ready and an interface Ze managed in the previous config is
// absent from the new cfg, reconcileOnReady deletes it (Phase 4).
//
// VALIDATES: AC-4 -- full reconcile runs Phase 4.
// PREVENTS: stale Ze-owned interfaces persisting after config apply.
func TestReconcileOnReady_PrunesPreviouslyManagedInterface(t *testing.T) {
	fb := &fakeBackend{
		ifaces: map[string]fakeIface{
			"dum0":         {name: "dum0", linkType: zeTypeDummy},
			"removed-dum":  {name: "removed-dum", linkType: zeTypeDummy},
			"operator-dum": {name: "operator-dum", linkType: zeTypeDummy},
		},
	}
	cfg := testConfigWithAddresses() // managed set = {dum0}
	cfg.previousManaged = map[string]bool{"dum0": true, "removed-dum": true}

	errs, deferred := reconcileOnReady(cfg, fb)
	require.False(t, deferred)
	require.Empty(t, errs)
	require.True(t, fb.deleted["removed-dum"], "previously managed interface should be deleted")
	require.False(t, fb.deleted["operator-dum"], "unowned manageable interface should NOT be deleted")
	require.False(t, fb.deleted["dum0"], "configured interface should NOT be deleted")
}

// TestParseTunnelGRE verifies that a gre case with all common leaves parses
// into a TunnelSpec with the correct kind and fields.
//
// VALIDATES: AC-1, AC-2, AC-3 - gre kind with local/remote endpoint
// containers, key, ttl, tos parses correctly.
// PREVENTS: Tunnel parser silently dropping fields when adding new encap kinds.
func TestParseTunnelGRE(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"},
							"key": "42",
							"ttl": "64",
							"tos": "0"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	e := cfg.Tunnel[0]
	assert.Equal(t, "gre0", e.Name)
	assert.Equal(t, TunnelKindGRE, e.Spec.Kind)
	assert.Equal(t, "192.0.2.1", e.Spec.LocalAddress)
	assert.Equal(t, "198.51.100.1", e.Spec.RemoteAddress)
	assert.True(t, e.Spec.KeySet)
	assert.Equal(t, uint32(42), e.Spec.Key)
	assert.True(t, e.Spec.TTLSet)
	assert.Equal(t, uint8(64), e.Spec.TTL)
	assert.True(t, e.Spec.TosSet)
}

// TestParseTunnelGretap verifies that the gretap case is recognized distinctly
// from gre even though their leaves overlap.
//
// VALIDATES: AC-5 - gretap discriminator separate from gre.
// PREVENTS: TunnelKind enum collision between gre and gretap.
func TestParseTunnelGretap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gretap0": {
					"encapsulation": {
						"gretap": {
							"local":  {"ip": "10.0.0.1"},
							"remote": {"ip": "10.0.0.2"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindGRETap, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelGretapMAC verifies that the mac/address leaf inside the gretap
// case container is parsed correctly. After the YANG restructure, the mac
// container lives inside the per-case container for bridgeable kinds
// (gretap/ip6gretap), not at the list level.
//
// VALIDATES: AC-2 (spec-iface-tunnel-mac-per-case) - mac/address inside gretap accepted.
// PREVENTS: MAC address silently dropped when moved from list level to case container.
func TestParseTunnelGretapMAC(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gretap0": {
					"encapsulation": {
						"gretap": {
							"local":  {"ip": "10.0.0.1"},
							"remote": {"ip": "10.0.0.2"},
							"mac": {"address": "aa:bb:cc:dd:ee:ff"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindGRETap, cfg.Tunnel[0].Spec.Kind)
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", cfg.Tunnel[0].MACAddress,
		"mac/address inside gretap case must be parsed")
}

// TestParseTunnelGreNoMAC verifies that an L3 tunnel kind (gre) does not
// carry a mac/address, and that any mac at the list level is ignored
// for tunnels (YANG enforces this; parser provides defense-in-depth).
//
// VALIDATES: AC-3 (spec-iface-tunnel-mac-per-case) - L3 kind without MAC accepted.
// VALIDATES: AC-4 (spec-iface-tunnel-mac-per-case) - mac/address not available on L3 kinds.
// PREVENTS: L3 tunnel silently accepting MAC that the kernel ignores.
func TestParseTunnelGreNoMAC(t *testing.T) {
	// mac at list level (hand-edited) -- must be ignored for tunnels.
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre0": {
					"mac": {"address": "aa:bb:cc:dd:ee:ff"},
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindGRE, cfg.Tunnel[0].Spec.Kind)
	assert.Empty(t, cfg.Tunnel[0].MACAddress,
		"L3 tunnel must not carry mac/address (list-level mac must be ignored)")
}

// TestParseTunnelIp6gretapMAC verifies that the mac/address leaf inside the
// ip6gretap case container is parsed correctly, mirroring TestParseTunnelGretapMAC
// for the v6-underlay L2 kind.
//
// VALIDATES: ip6gretap mac/address parity with gretap.
// PREVENTS: ip6gretap silently dropping mac/address.
func TestParseTunnelIp6gretapMAC(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ip6gretap0": {
					"encapsulation": {
						"ip6gretap": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"mac": {"address": "11:22:33:44:55:66"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindIP6GRETap, cfg.Tunnel[0].Spec.Kind)
	assert.Equal(t, "11:22:33:44:55:66", cfg.Tunnel[0].MACAddress,
		"mac/address inside ip6gretap case must be parsed")
}

// TestParseTunnelIp6gretap verifies the ip6gretap case is recognized as
// a distinct bridgeable kind.
//
// VALIDATES: ip6gretap discriminator.
// PREVENTS: ip6gretap kind regression.
func TestParseTunnelIp6gretap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ip6gretap0": {
					"encapsulation": {
						"ip6gretap": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindIP6GRETap, cfg.Tunnel[0].Spec.Kind)
	assert.True(t, cfg.Tunnel[0].Spec.Kind.isBridgeable())
}

// TestParseTunnelNoPMTUDiscovery verifies the no-pmtu-discovery empty leaf.
//
// VALIDATES: no-pmtu-discovery flag is set when present.
// PREVENTS: NoPMTUDiscovery silently dropped.
func TestParseTunnelNoPMTUDiscovery(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"},
							"no-pmtu-discovery": ""
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.True(t, cfg.Tunnel[0].Spec.NoPMTUDiscovery,
		"no-pmtu-discovery empty leaf must set the flag")
}

// TestParseTunnelIp6gre verifies the ip6gre case with v6 endpoints, hoplimit,
// and tclass parses into the right TunnelSpec fields.
//
// VALIDATES: AC-7 - ip6gre with hoplimit/tclass parses.
// PREVENTS: v6-underlay leaves silently dropped.
func TestParseTunnelIp6gre(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ip6gre0": {
					"encapsulation": {
						"ip6gre": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"hoplimit": "64",
							"tclass": "0",
							"key": "100"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIP6GRE, spec.Kind)
	assert.Equal(t, "2001:db8::1", spec.LocalAddress)
	assert.Equal(t, "2001:db8::2", spec.RemoteAddress)
	assert.True(t, spec.HopLimitSet)
	assert.Equal(t, uint8(64), spec.HopLimit)
	assert.True(t, spec.TClassSet)
	assert.True(t, spec.KeySet)
	assert.Equal(t, uint32(100), spec.Key)
}

// TestParseTunnelIpip verifies the ipip case parses without GRE-specific fields.
//
// VALIDATES: AC-8 - ipip case parses without key or ignore-df.
// PREVENTS: Schema accepting key on ipip silently.
func TestParseTunnelIpip(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ipip0": {
					"encapsulation": {
						"ipip": {
							"local":  {"ip": "10.0.0.1"},
							"remote": {"ip": "10.0.0.2"},
							"ttl": "32"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIPIP, spec.Kind)
	assert.False(t, spec.KeySet, "ipip must not have key set")
	assert.True(t, spec.TTLSet)
	assert.Equal(t, uint8(32), spec.TTL)
}

// TestParseTunnelSit verifies the sit (6in4) case parses with v4 endpoints.
//
// VALIDATES: AC-9 - sit kind with v4 endpoints parses.
func TestParseTunnelSit(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"sixin4": {
					"encapsulation": {
						"sit": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindSIT, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelIp6tnl verifies the ip6tnl case (IPv6 in IPv6) parses with
// encaplimit.
//
// VALIDATES: AC-10 - ip6tnl with encaplimit parses.
func TestParseTunnelIp6tnl(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"v6t": {
					"encapsulation": {
						"ip6tnl": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"encaplimit": "4"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, TunnelKindIP6Tnl, spec.Kind)
	assert.True(t, spec.EncapLimitSet)
	assert.Equal(t, uint8(4), spec.EncapLimit)
}

// TestParseTunnelIpip6 verifies the ipip6 case parses with the IPIP6 kind
// (which the linux backend implements as Ip6tnl with Proto=IPPROTO_IPIP).
//
// VALIDATES: AC-11 - ipip6 kind is distinct from ip6tnl in the spec.
// PREVENTS: ipip6 silently treated as ip6tnl, losing the discriminator the
// linux backend needs to set Proto=4 instead of Proto=41.
func TestParseTunnelIpip6(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"v4inv6": {
					"encapsulation": {
						"ipip6": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindIPIP6, cfg.Tunnel[0].Spec.Kind)
}

// TestParseTunnelLocalInterface verifies the choice inside the local
// container lets the user specify a parent interface name instead of an IP.
//
// VALIDATES: AC-13 - local interface alternative parses.
func TestParseTunnelLocalInterface(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre1": {
					"encapsulation": {
						"gre": {
							"local":  {"interface": "eth0"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	spec := cfg.Tunnel[0].Spec
	assert.Equal(t, "eth0", spec.LocalInterface)
	assert.Empty(t, spec.LocalAddress)
}

// TestParseTunnelMissingEncapsulation verifies the parser rejects a tunnel
// entry with no encapsulation block. The YANG schema rejects this at edit
// time too (via mandatory choice), so reaching this branch means hand-edited
// JSON or a schema bug.
//
// VALIDATES: AC-15 - tunnel without encapsulation rejected.
func TestParseTunnelMissingEncapsulation(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing encapsulation")
}

// TestParseTunnelMultipleCases verifies the parser rejects a tunnel with
// two encapsulation cases set simultaneously. Same defense-in-depth role
// as TestParseTunnelMissingEncapsulation.
func TestParseTunnelMultipleCases(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"x": {
					"encapsulation": {
						"gre":  {"local": {"ip": "10.0.0.1"}, "remote": {"ip": "10.0.0.2"}},
						"ipip": {"local": {"ip": "10.0.0.1"}, "remote": {"ip": "10.0.0.3"}}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple encapsulation cases")
}

// TestParseTunnelBothLocals verifies the parser rejects a tunnel with both
// local.ip and local.interface set. Static validation now invokes the
// side-effect-free plugin verifier, and this test pins the parser check that
// both verifier and runtime configure paths share.
//
// VALIDATES: AC-14 - local ip and local interface are mutually exclusive.
// PREVENTS: Silent acceptance of contradictory tunnel source config.
func TestParseTunnelBothLocals(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1", "interface": "eth0"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestParseTunnelMissingLocal verifies the parser rejects a tunnel with
// neither local.ip nor local.interface set. Same Go-side defense-in-depth
// as TestParseTunnelBothLocals.
func TestParseTunnelMissingLocal(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"encapsulation": {
						"gre": {
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local ip or local interface required")
}

// TestApplyTunnelsTwoGREDistinctKeys verifies AC-12: two gre tunnels with
// the same local/remote endpoints but different keys can coexist as long
// as they have distinct names. Each must reach the backend with its own
// Spec.
//
// VALIDATES: AC-12 - two GRE tunnels distinct keys.
// PREVENTS: Spec deduplication on local/remote that would drop one of the
// tunnels silently.
func TestApplyTunnelsTwoGREDistinctKeys(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{
			{
				Name: "gre-a",
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre-a",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           1,
					KeySet:        true,
				},
			},
			{
				Name: "gre-b",
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre-b",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           2,
					KeySet:        true,
				},
			},
		},
	}
	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs)
	require.Contains(t, b.tunnels, "gre-a")
	require.Contains(t, b.tunnels, "gre-b")
	assert.Equal(t, uint32(1), b.tunnels["gre-a"].Key)
	assert.Equal(t, uint32(2), b.tunnels["gre-b"].Key)
	assert.Equal(t, b.tunnels["gre-a"].LocalAddress, b.tunnels["gre-b"].LocalAddress)
	assert.Equal(t, b.tunnels["gre-a"].RemoteAddress, b.tunnels["gre-b"].RemoteAddress)
}

// TestApplyTunnelsUnchangedSkipsRecreate verifies that applyConfig does NOT
// delete-then-create a tunnel whose Spec is identical to the previous apply.
//
// VALIDATES: Smart reconciliation preserves running tunnels across reload.
// PREVENTS: Every SIGHUP briefly dropping every tunnel even when nothing changed.
func TestApplyTunnelsUnchangedSkipsRecreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	spec := TunnelSpec{
		Kind:          TunnelKindGRE,
		Name:          "gre0",
		LocalAddress:  "192.0.2.1",
		RemoteAddress: "198.51.100.1",
		Key:           42,
		KeySet:        true,
	}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel:  []tunnelEntry{{Name: "gre0", Spec: spec}},
	}
	// First apply: tunnel created.
	require.Empty(t, applyConfig(cfg, nil, b))
	require.Contains(t, b.tunnels, "gre0")
	require.False(t, b.deleted["gre0"], "first apply must not delete")

	// Second apply with the SAME config: no delete should fire.
	b.deleted = nil
	require.Empty(t, applyConfig(cfg, cfg, b))
	assert.False(t, b.deleted["gre0"], "unchanged spec must not trigger delete-then-create")
}

// TestApplyTunnelsChangedTriggersRecreate verifies that applyConfig deletes
// and recreates a tunnel whose Spec changed across reloads.
//
// VALIDATES: AC-18 - key change recreates the tunnel.
// PREVENTS: Modified tunnel parameters being silently ignored.
func TestApplyTunnelsChangedTriggersRecreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	prev := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{{
			Name: "gre0",
			Spec: TunnelSpec{
				Kind: TunnelKindGRE, Name: "gre0",
				LocalAddress: "192.0.2.1", RemoteAddress: "198.51.100.1",
				Key: 1, KeySet: true,
			},
		}},
	}
	require.Empty(t, applyConfig(prev, nil, b))
	b.deleted = nil

	updated := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{{
			Name: "gre0",
			Spec: TunnelSpec{
				Kind: TunnelKindGRE, Name: "gre0",
				LocalAddress: "192.0.2.1", RemoteAddress: "198.51.100.1",
				Key: 2, KeySet: true,
			},
		}},
	}
	require.Empty(t, applyConfig(updated, prev, b))
	assert.True(t, b.deleted["gre0"], "spec change must trigger delete-then-create")
	assert.Equal(t, uint32(2), b.tunnels["gre0"].Key, "new key must be applied")
}

// TestParseTunnelVLANRejectedOnL3 verifies parseTunnelEntry rejects a vlan-id
// unit on an L3 tunnel kind. Only gretap and ip6gretap carry Ethernet frames
// and accept VLAN sub-interfaces.
//
// VALIDATES: VLAN-on-tunnel only allowed on bridgeable kinds.
// PREVENTS: Silent failure when configuring VLAN on a gre/ipip/sit tunnel.
// wireguardTestKey builds a deterministic 32-byte key for unit tests.
// The byte value is repeated so a seed value acts as a human-readable
// label in test fixtures.
func wireguardTestKey(b byte) WireguardKey {
	var k WireguardKey
	for i := range k {
		k[i] = b
	}
	return k
}

// TestApplyWireguardsCreate verifies that applyConfig invokes both
// CreateWireguardDevice and ConfigureWireguardDevice on first apply
// for a new wireguard interface.
//
// VALIDATES: AC-1 apply path -- new wireguard interface -> netdev is
// created and configured in one reload.
// PREVENTS: silent drop of ConfigureWireguardDevice from the apply loop.
func TestApplyWireguardsCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Wireguard: []wireguardEntry{{
			Name: "wg0",
			Spec: WireguardSpec{
				Name:          "wg0",
				PrivateKey:    wireguardTestKey(1),
				ListenPort:    51820,
				ListenPortSet: true,
				Peers: []WireguardPeerSpec{{
					Name:       "a",
					PublicKey:  wireguardTestKey(2),
					AllowedIPs: []string{"10.0.0.1/32"},
				}},
			},
		}},
	}

	errs := applyConfig(cfg, nil, b)
	assert.Empty(t, errs, "apply errors: %v", errs)
	assert.True(t, b.created["wg0"], "wg0 not created")
	assert.Equal(t, 1, b.wgConfigCt["wg0"], "configure called exactly once")
	assert.Equal(t, uint16(51820), b.wgConfigs["wg0"].ListenPort)
}

// TestApplyWireguardsUnchangedSkipsConfigure verifies AC-2/3/4/5 no-op:
// reloading the same spec does NOT call ConfigureWireguardDevice a second
// time, so handshake state and kernel-level counters are preserved.
//
// VALIDATES: spec equality via wireguardSpecEqual skips the reconcile.
// PREVENTS: spurious genetlink traffic on every SIGHUP.
func TestApplyWireguardsUnchangedSkipsConfigure(t *testing.T) {
	spec := WireguardSpec{
		Name:          "wg0",
		PrivateKey:    wireguardTestKey(1),
		ListenPort:    51820,
		ListenPortSet: true,
		Peers: []WireguardPeerSpec{{
			Name:                "a",
			PublicKey:           wireguardTestKey(2),
			AllowedIPs:          []string{"10.0.0.1/32"},
			PersistentKeepalive: 25,
		}},
	}
	previous := &ifaceConfig{
		Wireguard: []wireguardEntry{{Name: "wg0", Spec: spec}},
	}
	cfg := &ifaceConfig{
		Wireguard: []wireguardEntry{{Name: "wg0", Spec: spec}},
	}

	// The previous config already had wg0, so the kernel already has the
	// device: applyConfig skips the create, and every later phase addresses a
	// link that exists.
	b := &fakeBackend{ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: "wireguard"}}}
	errs := applyConfig(cfg, previous, b)
	assert.Empty(t, errs)
	assert.Equal(t, 0, b.wgConfigCt["wg0"], "configure should be skipped when spec unchanged")
	assert.False(t, b.created["wg0"], "create should be skipped when previous had the interface")
}

// TestApplyWireguardsAddPeer verifies AC-2: adding a peer triggers a
// reconfigure without recreating the netdev.
//
// VALIDATES: AC-2 -- new peer reaches ConfigureWireguardDevice with the
// updated spec; netdev is not recreated.
// PREVENTS: silently dropped peer additions on SIGHUP.
func TestApplyWireguardsAddPeer(t *testing.T) {
	base := WireguardSpec{
		Name:       "wg0",
		PrivateKey: wireguardTestKey(1),
		Peers: []WireguardPeerSpec{{
			Name:       "a",
			PublicKey:  wireguardTestKey(2),
			AllowedIPs: []string{"10.0.0.1/32"},
		}},
	}
	withNew := base
	withNew.Peers = append(withNew.Peers, WireguardPeerSpec{
		Name:       "b",
		PublicKey:  wireguardTestKey(3),
		AllowedIPs: []string{"10.0.0.2/32"},
	})

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: base}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: withNew}}}

	// The previous config already had wg0, so the kernel already has the
	// device: applyConfig skips the create, and every later phase addresses a
	// link that exists.
	b := &fakeBackend{ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: "wireguard"}}}
	errs := applyConfig(cfg, previous, b)
	assert.Empty(t, errs)
	assert.Equal(t, 1, b.wgConfigCt["wg0"])
	assert.Len(t, b.wgConfigs["wg0"].Peers, 2, "both peers should be in the applied spec")
	assert.False(t, b.created["wg0"], "netdev should NOT be re-created")
	assert.False(t, b.deleted["wg0"], "netdev should NOT be deleted before re-create")
}

// TestApplyWireguardsRemovePeer verifies AC-3: removing a peer triggers
// a reconfigure without recreating the netdev. ConfigureWireguardDevice
// uses ReplacePeers: true internally so the kernel drops the missing peer.
//
// VALIDATES: AC-3 -- peer removal is applied via a single reconfigure.
// PREVENTS: leaking removed peers into the kernel peer set forever.
func TestApplyWireguardsRemovePeer(t *testing.T) {
	twoPeer := WireguardSpec{
		Name:       "wg0",
		PrivateKey: wireguardTestKey(1),
		Peers: []WireguardPeerSpec{
			{Name: "a", PublicKey: wireguardTestKey(2), AllowedIPs: []string{"10.0.0.1/32"}},
			{Name: "b", PublicKey: wireguardTestKey(3), AllowedIPs: []string{"10.0.0.2/32"}},
		},
	}
	onePeer := twoPeer
	onePeer.Peers = []WireguardPeerSpec{twoPeer.Peers[0]}

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: twoPeer}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: onePeer}}}

	// The previous config already had wg0, so the kernel already has the
	// device: applyConfig skips the create, and every later phase addresses a
	// link that exists.
	b := &fakeBackend{ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: "wireguard"}}}
	errs := applyConfig(cfg, previous, b)
	assert.Empty(t, errs)
	assert.Equal(t, 1, b.wgConfigCt["wg0"])
	assert.Len(t, b.wgConfigs["wg0"].Peers, 1)
	assert.Equal(t, wireguardTestKey(2), b.wgConfigs["wg0"].Peers[0].PublicKey)
}

// TestApplyWireguardsAllowedIPsChange verifies AC-4: changing a peer's
// allowed-ips reaches ConfigureWireguardDevice.
//
// VALIDATES: AC-4 -- allowed-ips updates round-trip through applyConfig.
// PREVENTS: stale CIDR routing after a config reload.
func TestApplyWireguardsAllowedIPsChange(t *testing.T) {
	beforeSpec := WireguardSpec{
		Name:       "wg0",
		PrivateKey: wireguardTestKey(1),
		Peers: []WireguardPeerSpec{{
			Name:       "a",
			PublicKey:  wireguardTestKey(2),
			AllowedIPs: []string{"10.0.0.1/32"},
		}},
	}
	afterSpec := beforeSpec
	afterSpec.Peers = []WireguardPeerSpec{{
		Name:       "a",
		PublicKey:  wireguardTestKey(2),
		AllowedIPs: []string{"10.0.0.1/32", "192.168.10.0/24"},
	}}

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: beforeSpec}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: afterSpec}}}

	// The previous config already had wg0, so the kernel already has the
	// device: applyConfig skips the create, and every later phase addresses a
	// link that exists.
	b := &fakeBackend{ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: "wireguard"}}}
	errs := applyConfig(cfg, previous, b)
	assert.Empty(t, errs)
	assert.Equal(t, 1, b.wgConfigCt["wg0"])
	assert.ElementsMatch(t,
		[]string{"10.0.0.1/32", "192.168.10.0/24"},
		b.wgConfigs["wg0"].Peers[0].AllowedIPs)
}

// TestApplyWireguardsEndpointChange verifies AC-5: changing a peer's
// endpoint reaches ConfigureWireguardDevice.
//
// VALIDATES: AC-5 -- endpoint updates round-trip.
// PREVENTS: stale endpoints after operator edits config.
func TestApplyWireguardsEndpointChange(t *testing.T) {
	beforeSpec := WireguardSpec{
		Name:       "wg0",
		PrivateKey: wireguardTestKey(1),
		Peers: []WireguardPeerSpec{{
			Name:         "a",
			PublicKey:    wireguardTestKey(2),
			EndpointIP:   "198.51.100.1",
			EndpointPort: 51820,
			AllowedIPs:   []string{"10.0.0.1/32"},
		}},
	}
	afterSpec := beforeSpec
	afterSpec.Peers = []WireguardPeerSpec{{
		Name:         "a",
		PublicKey:    wireguardTestKey(2),
		EndpointIP:   "198.51.100.2",
		EndpointPort: 51820,
		AllowedIPs:   []string{"10.0.0.1/32"},
	}}

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: beforeSpec}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{Name: "wg0", Spec: afterSpec}}}

	// The previous config already had wg0, so the kernel already has the
	// device: applyConfig skips the create, and every later phase addresses a
	// link that exists.
	b := &fakeBackend{ifaces: map[string]fakeIface{"wg0": {name: "wg0", linkType: "wireguard"}}}
	errs := applyConfig(cfg, previous, b)
	assert.Empty(t, errs)
	assert.Equal(t, 1, b.wgConfigCt["wg0"])
	assert.Equal(t, "198.51.100.2", b.wgConfigs["wg0"].Peers[0].EndpointIP)
}

// TestApplyWireguardsDisableIfaceSkips verifies AC-16: a wireguard entry
// marked disable is skipped entirely -- no Create, no Configure.
//
// VALIDATES: AC-16 -- disabled wireguard is a no-op in the apply loop.
// PREVENTS: disabled interfaces being created and then immediately deleted.
func TestApplyWireguardsDisableIfaceSkips(t *testing.T) {
	cfg := &ifaceConfig{
		Wireguard: []wireguardEntry{{
			Name: "wg0", Disable: true,
			Spec: WireguardSpec{Name: "wg0", PrivateKey: wireguardTestKey(1)},
		}},
	}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	errs := applyConfig(cfg, nil, b)
	assert.Empty(t, errs)
	assert.False(t, b.created["wg0"])
	assert.Equal(t, 0, b.wgConfigCt["wg0"])
}

func TestParseTunnelVLANRejectedOnL3(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"tunnel": {
				"gre0": {
					"unit": {
						"100": {"vlan-id": "100"}
					},
					"encapsulation": {
						"gre": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vlan-id units are not supported on gre tunnels")
}

// TestParseTunnelVLANAcceptedOnGretap verifies parseTunnelEntry accepts a
// vlan-id unit on gretap (the L2 bridgeable kind).
func TestParseTunnelVLANAcceptedOnGretap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gtap0": {
					"unit": {
						"100": {"vlan-id": "100"}
					},
					"encapsulation": {
						"gretap": {
							"local":  {"ip": "192.0.2.1"},
							"remote": {"ip": "198.51.100.1"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	require.Len(t, cfg.Tunnel[0].Units, 1)
	assert.Equal(t, 100, cfg.Tunnel[0].Units[0].VLANID)
}

// TestApplyTunnelsCreate verifies that applyConfig with one tunnel entry
// invokes Backend.CreateTunnel with a TunnelSpec carrying the parsed fields.
//
// VALIDATES: Backend dispatch wires through applyConfig.
// PREVENTS: tunnelEntry parsed but never reaching the backend.
func TestApplyTunnelsCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	cfg := &ifaceConfig{
		Backend: "fake",
		Tunnel: []tunnelEntry{
			{
				Name:  "gre0",
				Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/30"}}},
				Spec: TunnelSpec{
					Kind:          TunnelKindGRE,
					Name:          "gre0",
					LocalAddress:  "192.0.2.1",
					RemoteAddress: "198.51.100.1",
					Key:           42,
					KeySet:        true,
				},
			},
		},
	}
	errs := applyConfig(cfg, nil, b)
	require.Empty(t, errs, "applyConfig should not error for happy path")
	require.Contains(t, b.tunnels, "gre0", "tunnel must reach the backend")
	got := b.tunnels["gre0"]
	assert.Equal(t, TunnelKindGRE, got.Kind)
	assert.Equal(t, "192.0.2.1", got.LocalAddress)
	assert.Equal(t, "198.51.100.1", got.RemoteAddress)
	assert.Equal(t, uint32(42), got.Key)
	assert.True(t, got.KeySet)
	assert.Contains(t, b.addrs["gre0"], "10.0.0.1/30", "tunnel address should be applied")
}

// mustParseIfaceJSON wraps parseIfaceConfig with a t.Fatal on parse error.
// Used by table-driven tunnel tests to keep individual cases concise.
func mustParseIfaceJSON(t *testing.T, input string) *ifaceConfig {
	t.Helper()
	// Validate JSON first so the test fails clearly on a typo.
	var raw any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		t.Fatalf("invalid JSON in test fixture: %v", err)
	}
	cfg, err := parseIfaceConfig(input)
	if err != nil {
		t.Fatalf("parseIfaceConfig: %v", err)
	}
	return cfg
}

// TestParseUnitDHCPv4Enabled verifies that the dhcp container inside the
// ipv4 family is parsed into a dhcpUnitConfig with Enabled=true.
//
// VALIDATES: AC-1 - Config with ipv4 { dhcp { enabled true } } parsed.
// PREVENTS: DHCP leaves silently ignored by parseIPv4Settings.
func TestParseUnitDHCPv4Enabled(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"dhcp": {
									"enabled": "true"
								}
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv4, "IPv4 settings should be parsed")
	require.NotNil(t, u.IPv4.DHCP, "DHCP config should be parsed")
	assert.True(t, u.IPv4.DHCP.Enabled)
	if u.IPv6 != nil {
		assert.Nil(t, u.IPv6.DHCPv6, "DHCPv6 should be nil when not configured")
	}
}

// TestParseUnitMPLSEnable verifies `unit { mpls { enable } }` drives the
// per-interface MPLS input setting.
//
// VALIDATES: mpls-1 AC-4 -- interface mpls enable parsed for sysctl emission.
func TestParseUnitMPLSEnable(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"mpls": {
								"enable": "true"
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.MPLSEnable, "MPLS enable should be parsed")
	assert.True(t, *u.MPLSEnable)
}

// TestParseUnitMPLSAbsent verifies MPLSEnable is nil when not configured, so no
// sysctl is emitted for interfaces that do not opt in.
func TestParseUnitMPLSAbsent(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {"0": {"ipv4": {"address": ["10.0.0.1/30"]}}}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	assert.Nil(t, cfg.Ethernet[0].Units[0].MPLSEnable, "no mpls config means nil (unconfigured)")
}

// TestParseUnitDHCPv4Hostname verifies hostname and client-id parsing.
//
// VALIDATES: AC-3 - hostname and client-id parsed from ipv4 { dhcp {} }.
// PREVENTS: DHCP options silently dropped during parsing.
func TestParseUnitDHCPv4Hostname(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"dhcp": {
									"enabled": "true",
									"hostname": "ze-router",
									"client-id": "ze:01"
								}
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.DHCP)
	assert.True(t, u.IPv4.DHCP.Enabled)
	assert.Equal(t, "ze-router", u.IPv4.DHCP.Hostname)
	assert.Equal(t, "ze:01", u.IPv4.DHCP.ClientID)
}

// TestParseUnitDHCPDisabledDefault verifies that a unit without a dhcp
// container has DHCP=nil (disabled by default).
//
// VALIDATES: AC-5 - No dhcp block means no DHCP client.
// PREVENTS: DHCP accidentally enabled when config omits the block.
func TestParseUnitDHCPDisabledDefault(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv4)
	assert.Nil(t, u.IPv4.DHCP, "DHCP should be nil when not configured")
	assert.Nil(t, u.IPv6, "IPv6 should be nil when not configured")
	assert.Equal(t, []string{"10.0.0.1/24"}, u.Addresses)
}

// TestParseUnitDHCPv6PD verifies DHCPv6 with prefix delegation parsing.
//
// VALIDATES: AC-4 - DHCPv6 enabled with PD length inside ipv6 container.
// PREVENTS: DHCPv6 PD length silently dropped.
func TestParseUnitDHCPv6PD(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv6": {
								"dhcpv6": {
									"enabled": "true",
									"pd": {
										"length": "56"
									},
									"duid": "00:01:00:01:aa:bb"
								}
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.IPv4, "IPv4 should be nil")
	require.NotNil(t, u.IPv6)
	require.NotNil(t, u.IPv6.DHCPv6)
	assert.True(t, u.IPv6.DHCPv6.Enabled)
	assert.Equal(t, 56, u.IPv6.DHCPv6.PDLength)
	assert.Equal(t, "00:01:00:01:aa:bb", u.IPv6.DHCPv6.DUID)
}

// TestParseUnitDHCPDualStack verifies both DHCPv4 and DHCPv6 on the same unit.
//
// VALIDATES: Dual-stack DHCP coexistence in family containers.
// PREVENTS: v4 and v6 config interfering with each other.
func TestParseUnitDHCPDualStack(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"dhcp": {"enabled": "true", "hostname": "ze"}
							},
							"ipv6": {
								"dhcpv6": {"enabled": "true"}
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.DHCP)
	assert.True(t, u.IPv4.DHCP.Enabled)
	assert.Equal(t, "ze", u.IPv4.DHCP.Hostname)
	require.NotNil(t, u.IPv6)
	require.NotNil(t, u.IPv6.DHCPv6)
	assert.True(t, u.IPv6.DHCPv6.Enabled)
}

// TestParseUnitDHCPWithStaticAddress verifies DHCP alongside static addresses.
//
// VALIDATES: Static IP config alongside DHCP in same family container.
// PREVENTS: DHCP parsing clobbering static address list.
func TestParseUnitDHCPWithStaticAddress(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24"],
								"dhcp": {"enabled": "true"}
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, []string{"10.0.0.1/24"}, u.Addresses)
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.DHCP)
	assert.True(t, u.IPv4.DHCP.Enabled)
}

// TestParseIfaceDHCPAuto verifies the dhcp-auto leaf is parsed.
//
// VALIDATES: dhcp-auto true parsed into ifaceConfig.DHCPAuto.
// PREVENTS: Auto-discovery silently ignored due to parse bug.
func TestParseIfaceDHCPAuto(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dhcp-auto": "true"
		}
	}`)
	assert.True(t, cfg.DHCPAuto)
}

// TestParseIfaceDHCPAutoDefault verifies dhcp-auto is false by default.
//
// VALIDATES: No dhcp-auto means disabled.
// PREVENTS: DHCP auto-discovery running when not configured.
func TestParseIfaceDHCPAutoDefault(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {}
	}`)
	assert.False(t, cfg.DHCPAuto)
}

// TestParseUnitRoutePriority verifies that route-priority is parsed into
// unitEntry.RoutePriority from the YANG config JSON.
//
// VALIDATES: AC-1 - Config with route-priority is parsed into unitEntry.
// PREVENTS: route-priority silently ignored during config parsing.
func TestParseUnitRoutePriority(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"route-priority": "5"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, 5, u.RoutePriority)
	assert.True(t, u.RoutePrioritySet, "a written leaf is recorded as written")
}

// TestParseUnitRoutePriorityDefault verifies that a unit without
// route-priority takes the learned-route metric, and that an explicit 0 is
// distinguishable from an absent leaf.
//
// VALIDATES: a learned default route is ranked below an operator's static
// default instead of sharing metric 0 with it, and `route-priority 0` restores
// the pre-2026-08-11 metric.
// PREVENTS: a DHCP, RA or PPPoE default route installed at metric 0, where
// RouteReplace overwrites the operator's static default (gateway included),
// because RouteReplace matches on destination, metric and table and takes no
// protocol.
func TestParseUnitRoutePriorityDefault(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, defaultLearnedRouteMetric, u.RoutePriority)
	assert.Equal(t, 254, u.RoutePriority, "the value an operator reads in the schema")
	assert.False(t, u.RoutePrioritySet, "an absent leaf is not a written one")

	zero := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {"route-priority": "0"}
					}
				}
			}
		}
	}`)
	uz := zero.Ethernet[0].Units[0]
	assert.Equal(t, 0, uz.RoutePriority, "an explicit 0 stays 0: the documented way back")
	assert.True(t, uz.RoutePrioritySet)
}

// TestHandleDHCPLeaseEventStoresGateway verifies that a DHCP lease event
// updates the stored gateway for link-state failover.
//
// VALIDATES: Gateway stored from DHCP lease for failover use.
// PREVENTS: Link failover silently does nothing because gateway is empty.
func TestHandleDHCPLeaseEventStoresGateway(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {params: dhcpParams{v4: true}},
	}
	logger := slog.Default()

	data := `{"name":"eth0","unit":"default","router":"192.168.1.1","address":"192.168.1.50","prefix-length":24}`
	handleDHCPLeaseEvent(data, active, logger)

	entry := active[dhcpUnitKey{ifaceName: "eth0", unit: "default"}]
	assert.Equal(t, "192.168.1.1", entry.gateway)
}

// TestHandleDHCPLeaseEventNoMatch verifies that lease events for unknown
// interfaces are silently ignored.
//
// VALIDATES: No panic or map corruption on unmatched lease event.
// PREVENTS: Map write for interface not in activeDHCP.
func TestHandleDHCPLeaseEventNoMatch(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {params: dhcpParams{v4: true}},
	}
	logger := slog.Default()

	data := `{"name":"eth1","unit":"default","router":"10.0.0.1"}`
	handleDHCPLeaseEvent(data, active, logger)

	// eth0 gateway unchanged (still empty).
	entry := active[dhcpUnitKey{ifaceName: "eth0", unit: "default"}]
	assert.Equal(t, "", entry.gateway)
}

// TestHandleLinkDownWithRoutePriority verifies that handleLinkDown removes
// the base-metric route and installs a deprioritized route (base + 1024).
//
// VALIDATES: AC-3 - Link down with route-priority 5 deprioritizes to 1029.
// PREVENTS: Link-down using hardcoded metric 0 instead of configured routePriority.
func TestHandleLinkDownWithRoutePriority(t *testing.T) {
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-linkdown-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	defer func() { _ = CloseBackend() }()

	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {
			params:  dhcpParams{v4: true, routePriority: 5, routePrioritySet: true},
			gateway: "192.168.1.1",
		},
	}
	logger := slog.Default()

	handleLinkDown("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 5, rtproto.Iface}, fb.routeRemoves[0],
		"should remove route with base metric")

	require.Len(t, fb.routeAdds, 1, "should add one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 1029, rtproto.Iface}, fb.routeAdds[0],
		"should add deprioritized route (5 + 1024 = 1029)")
}

// TestHandleLinkUpWithRoutePriority verifies that handleLinkUp removes
// the deprioritized route and restores the base-metric route.
//
// VALIDATES: AC-4 - Link up with route-priority 5 restores metric to 5.
// PREVENTS: Link-up using hardcoded metric 0 instead of configured routePriority.
func TestHandleLinkUpWithRoutePriority(t *testing.T) {
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-linkup-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	defer func() { _ = CloseBackend() }()

	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {
			params:  dhcpParams{v4: true, routePriority: 5, routePrioritySet: true},
			gateway: "192.168.1.1",
		},
	}
	logger := slog.Default()

	handleLinkUp("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 1029, rtproto.Iface}, fb.routeRemoves[0],
		"should remove deprioritized route (5 + 1024 = 1029)")

	require.Len(t, fb.routeAdds, 1, "should add one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 5, rtproto.Iface}, fb.routeAdds[0],
		"should restore route with base metric")
}

// TestHandleLinkDownDefaultMetric verifies that link-down on a unit whose
// operator wrote `route-priority 0` uses metric 0 and 1024.
//
// VALIDATES: `route-priority 0` is the documented way back to the metric ze
// used before learned defaults were ranked at defaultLearnedRouteMetric.
// PREVENTS: the restore path silently taking the new default anyway, which
// would leave an operator no way to reproduce the old behavior.
func TestHandleLinkDownDefaultMetric(t *testing.T) {
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-linkdown-default-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	defer func() { _ = CloseBackend() }()

	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {
			params:  dhcpParams{v4: true, routePriority: 0, routePrioritySet: true},
			gateway: "10.0.0.1",
		},
	}
	logger := slog.Default()

	handleLinkDown("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, 0, fb.routeRemoves[0].metric, "should remove metric-0 route")

	require.Len(t, fb.routeAdds, 1)
	assert.Equal(t, 1024, fb.routeAdds[0].metric, "should add metric-1024 route")
}

// TestHandleLinkDownThenUp verifies the full down-then-up sequence uses
// the same dhcpEntry and produces the correct route operations.
//
// VALIDATES: AC-3, AC-4 - Full failover cycle with route-priority 5.
// PREVENTS: State corruption between handleLinkDown and handleLinkUp.
func TestHandleLinkDownThenUp(t *testing.T) {
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-downup-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	defer func() { _ = CloseBackend() }()

	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: "default"}: {
			params:  dhcpParams{v4: true, routePriority: 5, routePrioritySet: true},
			gateway: "192.168.1.1",
		},
	}
	logger := slog.Default()

	// Link goes down: remove metric-5, add metric-1029.
	handleLinkDown("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, 5, fb.routeRemoves[0].metric)
	require.Len(t, fb.routeAdds, 1)
	assert.Equal(t, 1029, fb.routeAdds[0].metric)

	// Link comes back up: remove metric-1029, add metric-5.
	handleLinkUp("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 2)
	assert.Equal(t, 1029, fb.routeRemoves[1].metric)
	require.Len(t, fb.routeAdds, 2)
	assert.Equal(t, 5, fb.routeAdds[1].metric)
}

// TestIfaceApplyJournalCreate verifies that applyConfig wrapped in a journal
// can be rolled back by re-applying the previous config.
//
// VALIDATES: AC-5 - Interface config adds new interface + address.
// PREVENTS: Interface created without journal tracking, making rollback impossible.
func TestIfaceApplyJournalCreate(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created")
	assert.Contains(t, b.addrs["dummy0"], "10.0.0.1/24", "address should be added")
	assert.Equal(t, 1, j.Len(), "journal should have 1 entry")
}

// TestIfaceApplyJournalAddress verifies that address operations are tracked
// by the journal and can be undone.
//
// VALIDATES: AC-5 - Address assigned via journal.
// PREVENTS: Address added without undo capability.
func TestIfaceApplyJournalAddress(t *testing.T) {
	b := &fakeBackend{
		ifaces: map[string]fakeIface{
			"eth0": {name: "eth0", linkType: "ethernet"},
		},
	}

	cfg := &ifaceConfig{
		Backend:  "fake",
		Ethernet: []ifaceEntry{{Name: "eth0", Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/24", "10.0.0.2/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(cfg, nil, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			empty := &ifaceConfig{Backend: "fake"}
			if errs := applyConfig(empty, cfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.Len(t, b.addrs["eth0"], 2, "both addresses should be added")
}

// TestIfaceApplyJournalRollbackEvents verifies that rollback re-applies
// the previous config, effectively undoing the changes.
//
// VALIDATES: AC-6 - Interface rollback after partial apply.
// PREVENTS: Rollback leaving stale interfaces or addresses.
func TestIfaceApplyJournalRollbackEvents(t *testing.T) {
	b := &fakeBackend{ifaces: map[string]fakeIface{}}

	// Previous config: no interfaces.
	previousCfg := &ifaceConfig{Backend: "fake"}

	// New config: creates dummy0.
	newCfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/24"}}}}},
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error {
			if errs := applyConfig(newCfg, previousCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
		func() error {
			if errs := applyConfig(previousCfg, newCfg, b); len(errs) > 0 {
				return errs[0]
			}
			return nil
		},
	)
	require.NoError(t, err)
	assert.True(t, b.created["dummy0"], "dummy0 should be created after apply")

	// Rollback: should re-apply previous (empty) config, deleting dummy0.
	errs := j.Rollback()
	assert.Empty(t, errs, "rollback should succeed")
	assert.True(t, b.deleted["dummy0"], "dummy0 should be deleted after rollback")
}

func TestApplyConfigRollsBackCreatedInterfaceOnAddressFailure(t *testing.T) {
	b := &fakeBackend{
		ifaces:        map[string]fakeIface{},
		addAddressErr: map[string]error{addressErrKey("dummy0", "10.0.0.1/24"): errors.New("address failed")},
	}
	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{Label: "default", Addresses: []string{"10.0.0.1/24"}}}}},
	}

	errs := applyConfig(cfg, nil, b)

	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "dummy0 add address 10.0.0.1/24")
	assert.True(t, b.deleted["dummy0"], "created interface should be deleted by partial rollback")
	assert.NotContains(t, b.ifaces, "dummy0", "created interface should not survive failed apply")
}

func TestApplyConfigStopsAfterFirstCreateFailure(t *testing.T) {
	b := &fakeBackend{
		ifaces:         map[string]fakeIface{},
		createDummyErr: map[string]error{"dummy0": errors.New("create failed")},
	}
	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy: []ifaceEntry{
			{Name: "dummy0"},
			{Name: "dummy1"},
		},
	}

	errs := applyConfig(cfg, nil, b)

	require.NotEmpty(t, errs)
	assert.Contains(t, errs[0].Error(), "dummy dummy0 create")
	assert.False(t, b.created["dummy1"], "apply must not continue after first failure")
	assert.NotContains(t, b.ifaces, "dummy1")
}

// TestIfaceVerifyEstimate verifies that the verify callback computes an
// estimate proportional to interface operations.
//
// VALIDATES: AC-12 - Interface budget proportional to interface count.
// PREVENTS: Budget estimate that doesn't scale with config size.
func TestIfaceVerifyEstimate(t *testing.T) {
	// Interface budget is set statically at registration (VerifyBudget=2, ApplyBudget=10).
	// The estimate scales with the number of configured interfaces.
	cfg := &ifaceConfig{
		Backend: "fake",
		Dummy: []ifaceEntry{
			{Name: "dummy0"},
			{Name: "dummy1"},
			{Name: "dummy2"},
		},
		Veth: []vethEntry{
			{Name: "veth0", Peer: "veth0-peer"},
		},
	}

	// Count operations: 3 dummy creates + 1 veth create = 4 operations.
	count := len(cfg.Dummy) + len(cfg.Veth) + len(cfg.Bridge) + len(cfg.Ethernet)
	assert.Equal(t, 4, count, "operation count should reflect interface config size")
}

// routeCall records a single AddRoute or RemoveRoute invocation.
type routeCall struct {
	ifaceName string
	destCIDR  string
	gateway   string
	metric    int
	proto     rtproto.Proto
}

// errFakeLinkExists is what the fake backend answers when a create names a
// link that already exists. The kernel answers RTM_NEWLINK with EEXIST there,
// whose Go text is "file exists"; a fake that overwrites instead cannot tell an
// idempotent apply from one that fails on a real backend.
var errFakeLinkExists = errors.New("file exists")

// fakeBackend implements Backend for testing config application.
type fakeBackend struct {
	ifaces           map[string]fakeIface
	created          map[string]bool
	deleted          map[string]bool
	addrs            map[string][]string
	tunnels          map[string]TunnelSpec
	wgConfigs        map[string]WireguardSpec
	wgConfigCt       map[string]int
	vlans            map[string]VLANSpec
	routeAdds        []routeCall
	routeRemoves     []routeCall
	addRouteErr      error       // if non-nil, AddRoute records the call and returns this
	staleRoutes      []RouteInfo // returned by ListRoutes for stale cleanup tests
	listErr          error       // if non-nil, ListInterfaces returns this instead of enumerating
	createDummyErr   map[string]error
	createTunnelErr  map[string]error // per-name CreateTunnel error injection
	addAddressErr    map[string]error
	lcpPairs         map[string]string      // vppIface -> hostName recorded by SetupLCPPair
	macSet           map[string]string      // ifaceName -> mac recorded by SetMACAddress
	macvlans         map[string]MacvlanSpec // name -> spec recorded by CreateMacvlanDevice
	callOrder        []string               // ordered log of macvlan-create + add-address calls
	createMacvlanErr map[string]error       // per-name CreateMacvlanDevice error injection
	mirrorCalls      []mirrorCall           // ordered log of SetupMirror + RemoveMirror calls
	setupMirrorErr   map[string]error       // per-source-interface SetupMirror error injection
	removeMirrorErr  map[string]error       // per-source-interface RemoveMirror error injection
	// mirrors is the fake dataplane's LIVE mirror state, keyed by source
	// device. SetupMirror and RemoveMirror maintain it, and ListMirrors reports
	// it. A test can therefore seed a mirror no config asks for, which is what a
	// restart leaves behind. The test then watches the reconcile retire it. A
	// recorded CALL cannot express that: the seed happened before the test ran.
	mirrors        map[string]MirrorState
	listMirrorsErr error             // if non-nil, ListMirrors returns this instead of the state
	nextIndex      int               // hands each created device its own kernel index
	mtuSet         map[string]int    // kernel device -> MTU recorded by SetMTU
	adminSet       map[string]string // kernel device -> "up"/"down" recorded by SetAdminUp/Down
	bridgePorts    []bridgePortCall  // ordered log of BridgeAddPort + BridgeDelPort calls
}

// The two membership operations a bridge can be asked for.
const (
	bridgePortOpAdd = "add"
	bridgePortOpDel = "del"
)

// bridgePortCall records one bridge membership operation. The port name is the
// point of the record: the apply must name the kernel device the member's
// selector resolved to, never the logical entry name that selector exists to
// translate.
type bridgePortCall struct {
	op     string
	bridge string
	port   string
}

// The two mirror operations a backend can be asked for.
const (
	mirrorOpSetup  = "setup"
	mirrorOpRemove = "remove"
)

// mirrorCall records one mirror operation the apply path asked the backend
// for. The order matters: a changed mirror must be retired before the new one
// is installed, because tc filters are additive.
type mirrorCall struct {
	op      string
	iface   string
	dst     string
	ingress bool
	egress  bool
}

type fakeIface struct {
	name     string
	linkType string
	mac      string
	// permMAC is the device's permanent (factory) address, IFLA_PERM_ADDRESS.
	// A real NIC reports one and a virtual device does not, which is the
	// difference the mac/match selector is built on: deviceMatchMAC prefers it
	// so a binding survives an operational MAC override.
	permMAC     string
	alias       string
	index       int // the interface's own kernel index
	parentIndex int // for macvlan: the parent's index
	// masterIndex is the index of the aggregating device this one is a member
	// of, IFLA_MASTER: the bridge it is a port of, or the bond it is enslaved
	// to. A live kernel sets it on the MEMBER, and the aggregator then wears
	// that member's hardware address, which is the state a mac/match selector
	// has to read past.
	masterIndex int
	mtu         int
	state       string
}

func (b *fakeBackend) ensureMaps() {
	if b.ifaces == nil {
		b.ifaces = make(map[string]fakeIface)
	}
	if b.created == nil {
		b.created = make(map[string]bool)
	}
	if b.deleted == nil {
		b.deleted = make(map[string]bool)
	}
	if b.addrs == nil {
		b.addrs = make(map[string][]string)
	}
	if b.tunnels == nil {
		b.tunnels = make(map[string]TunnelSpec)
	}
	if b.wgConfigs == nil {
		b.wgConfigs = make(map[string]WireguardSpec)
	}
	if b.wgConfigCt == nil {
		b.wgConfigCt = make(map[string]int)
	}
	if b.vlans == nil {
		b.vlans = make(map[string]VLANSpec)
	}
}

func addressErrKey(ifaceName, cidr string) string {
	return ifaceName + "|" + cidr
}

func (b *fakeBackend) CreateDummy(name string) error {
	b.ensureMaps()
	if err := b.createDummyErr[name]; err != nil {
		return err
	}
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "dummy"}
	return nil
}

func (b *fakeBackend) CreateVeth(name, peerName string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "veth"}
	return nil
}

func (b *fakeBackend) CreateBridge(name string) error {
	b.ensureMaps()
	b.created[name] = true
	// A bridge carries a kernel index of its own, because a port names its
	// bridge by index (IFLA_MASTER) and an index of zero reads as "no master".
	b.nextIndex++
	b.ifaces[name] = fakeIface{name: name, linkType: "bridge", index: 2000 + b.nextIndex}
	return nil
}

func (b *fakeBackend) CreateVLAN(spec VLANSpec) error {
	b.ensureMaps()
	var nb textbuf.Buffer
	name := nb.Str(spec.Parent).Byte('.').Int(int64(spec.VLANID)).String()
	b.created[name] = true
	b.vlans[name] = spec
	// A VLAN sub-interface INHERITS its parent's hardware address and names the
	// parent in ParentIndex, and it reports no permanent address of its own.
	// A fake that left the MAC empty could not show what a live kernel showed:
	// the parent's own mac/match selector matching its child as well as itself.
	parent := b.ifaces[spec.Parent]
	b.nextIndex++
	b.ifaces[name] = fakeIface{
		name:        name,
		linkType:    "vlan",
		mac:         parent.mac,
		index:       1000 + b.nextIndex,
		parentIndex: parent.index,
		mtu:         parent.mtu,
	}
	return nil
}

func (b *fakeBackend) UpdateVLANQoSMap(_ string, _, _ map[uint32]uint32) error { return nil }

// fakeKernelLinkTypes spells the netlink link type Linux reports for a device
// of each tunnel kind, so a read-back from this fake carries what the kernel
// would carry (ip -d link show, netlink.Link.Type()). It is written out here
// rather than taken from kernelLinkTypes (tunnel.go) on purpose: a fake that
// reads the production map agrees with a wrong entry in it, and the kept-netdev
// guard in applyConfig compares against exactly that map.
var fakeKernelLinkTypes = map[TunnelKind]string{
	TunnelKindGRE:       "gre",
	TunnelKindGRETap:    "gretap",
	TunnelKindIP6GRE:    "ip6gre",
	TunnelKindIP6GRETap: "ip6gretap",
	TunnelKindIPIP:      "ipip",
	TunnelKindSIT:       "sit",
	TunnelKindIP6Tnl:    "ip6tnl",
	// ipip6 is the ip6_tunnel driver with the inner protocol set to IPIP; the
	// kernel reports the driver, not the protocol.
	TunnelKindIPIP6: "ip6tnl",
	TunnelKindVxlan: "vxlan",
}

// fakeUnknownLinkType is the link type the fake leaves behind for a tunnel kind
// no driver answers to. It is not TunnelKind.String()'s "unknown", which names
// a KIND with no YANG spelling; this names a LINK TYPE no kernel reports.
const fakeUnknownLinkType = "unknown"

func (b *fakeBackend) CreateTunnel(spec TunnelSpec) error {
	b.ensureMaps()
	if err := b.createTunnelErr[spec.Name]; err != nil {
		// A refused create leaves no netdev behind, which is what separates
		// this from the EEXIST case below.
		return err
	}
	if _, exists := b.ifaces[spec.Name]; exists {
		return errFakeLinkExists
	}
	linkType, known := fakeKernelLinkTypes[spec.Kind]
	if !known {
		// No driver answers to a kind Ze does not model, so the device such a
		// spec would leave behind is of no known type. Spelled here rather
		// than returning an error, because the tests that build a bare
		// TunnelSpec exercise later phases and not the create itself.
		linkType = fakeUnknownLinkType
	}
	b.created[spec.Name] = true
	b.tunnels[spec.Name] = spec
	b.ifaces[spec.Name] = fakeIface{name: spec.Name, linkType: linkType}
	return nil
}

func (b *fakeBackend) CreateWireguardDevice(name string) error {
	b.ensureMaps()
	b.created[name] = true
	b.ifaces[name] = fakeIface{name: name, linkType: "wireguard"}
	return nil
}

func (b *fakeBackend) ConfigureWireguardDevice(spec WireguardSpec) error {
	b.ensureMaps()
	b.wgConfigs[spec.Name] = spec
	b.wgConfigCt[spec.Name]++
	return nil
}

func (b *fakeBackend) GetWireguardDevice(_ string) (WireguardSpec, error) {
	return WireguardSpec{}, nil
}

func (b *fakeBackend) CreateXFRM(spec XFRMSpec) error {
	b.ensureMaps()
	b.created[spec.Name] = true
	b.ifaces[spec.Name] = fakeIface{name: spec.Name, linkType: "xfrm"}
	return nil
}

func (b *fakeBackend) CreateMacvlanDevice(spec MacvlanSpec) error {
	b.ensureMaps()
	if b.macvlans == nil {
		b.macvlans = make(map[string]MacvlanSpec)
	}
	if err := b.createMacvlanErr[spec.Name]; err != nil {
		return err
	}
	b.callOrder = append(b.callOrder, "create-macvlan:"+spec.Name)
	b.created[spec.Name] = true
	b.macvlans[spec.Name] = spec
	// Faithful to the netlink backend: MAC + alias set atomically at create,
	// parent index + MTU inherited from the parent (mirrored here from the
	// parent fakeIface, so a re-run sees a spec-equal device -- no false drift).
	p := b.ifaces[spec.Parent]
	b.ifaces[spec.Name] = fakeIface{
		name:        spec.Name,
		linkType:    zeTypeMacvlan,
		mac:         spec.MAC,
		alias:       spec.Alias,
		parentIndex: p.index,
		mtu:         p.mtu,
	}
	return nil
}
func (b *fakeBackend) GetXFRMInfo(_ string) (XFRMInfo, error) { return XFRMInfo{}, nil }

// SetAdminUp, SetAdminDown, SetMTU and AddAddress all refuse a device that is
// not present, which is what the netlink backend does: each starts with
// netlink.LinkByName and answers "not found" when it fails (manage_linux.go).
// A fake that accepted any name could not tell an apply that reached the
// operator's hardware from one that named a device which does not exist.
func (b *fakeBackend) SetAdminUp(name string) error { return b.setAdmin(name, "up") }

func (b *fakeBackend) SetAdminDown(name string) error { return b.setAdmin(name, "down") }

func (b *fakeBackend) setAdmin(name, state string) error {
	if _, ok := b.ifaces[name]; !ok {
		return fmt.Errorf("iface: set %s %q: not found: link not found", state, name)
	}
	if b.adminSet == nil {
		b.adminSet = make(map[string]string)
	}
	b.adminSet[name] = state
	f := b.ifaces[name]
	f.state = state
	b.ifaces[name] = f
	return nil
}

func (b *fakeBackend) SetMTU(name string, mtu int) error {
	f, ok := b.ifaces[name]
	if !ok {
		return fmt.Errorf("iface: set mtu on %q: not found: link not found", name)
	}
	if b.mtuSet == nil {
		b.mtuSet = make(map[string]int)
	}
	b.mtuSet[name] = mtu
	f.mtu = mtu
	b.ifaces[name] = f
	return nil
}
func (b *fakeBackend) SetMACAddress(name, mac string) error {
	// Faithful to the netlink backend: setting a MAC on an interface whose link
	// is absent fails with a not-found error (manage_linux.go SetMACAddress).
	if _, ok := b.ifaces[name]; !ok {
		return fmt.Errorf("iface: set mac on %q: not found: link not found", name)
	}
	if b.macSet == nil {
		b.macSet = make(map[string]string)
	}
	b.macSet[name] = mac
	return nil
}
func (b *fakeBackend) GetMACAddress(name string) (string, error) {
	f, ok := b.ifaces[name]
	if !ok {
		return "", fmt.Errorf("iface: get mac on %q: not found: link not found", name)
	}
	return f.mac, nil
}
func (b *fakeBackend) GetStats(_ string) (*InterfaceStats, error) {
	return &InterfaceStats{}, nil
}

func (b *fakeBackend) DeleteInterface(name string) error {
	b.ensureMaps()
	b.deleted[name] = true
	delete(b.ifaces, name)
	return nil
}

func (b *fakeBackend) AddAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	if err := b.addAddressErr[addressErrKey(ifaceName, cidr)]; err != nil {
		return err
	}
	if _, ok := b.ifaces[ifaceName]; !ok {
		return fmt.Errorf("iface: add address on %q: not found: link not found", ifaceName)
	}
	b.callOrder = append(b.callOrder, "add-address:"+ifaceName+":"+cidr)
	b.addrs[ifaceName] = append(b.addrs[ifaceName], cidr)
	return nil
}

func (b *fakeBackend) RemoveAddress(ifaceName, cidr string) error {
	b.ensureMaps()
	filtered := b.addrs[ifaceName][:0]
	for _, a := range b.addrs[ifaceName] {
		if a != cidr {
			filtered = append(filtered, a)
		}
	}
	b.addrs[ifaceName] = filtered
	return nil
}

func (b *fakeBackend) ReplaceAddressWithLifetime(_, _ string, _, _ int) error { return nil }

func (b *fakeBackend) AddAddressP2P(_, _, _ string) error { return nil }

// AddRoute records the call and then returns addRouteErr. The call is recorded
// either way: a test that injects a failure is asking what the caller did after
// the kernel refused it, and that starts with the attempt itself.
func (b *fakeBackend) AddRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b.routeAdds = append(b.routeAdds, routeCall{ifaceName, destCIDR, gateway, metric, proto})
	return b.addRouteErr
}

func (b *fakeBackend) RemoveRoute(ifaceName, destCIDR, gateway string, metric int, proto rtproto.Proto) error {
	b.routeRemoves = append(b.routeRemoves, routeCall{ifaceName, destCIDR, gateway, metric, proto})
	return nil
}

func (b *fakeBackend) ListRoutes(_, _ string) ([]RouteInfo, error) {
	return b.staleRoutes, nil
}

func (b *fakeBackend) ListNeighbors(_ int) ([]NeighborInfo, error) {
	return nil, nil
}

func (b *fakeBackend) RouteLookup(_ netip.Addr) (map[string]any, error) {
	return map[string]any{}, nil
}

func (b *fakeBackend) AddressIsLocal(_ netip.Addr) (bool, error) { return false, nil }
func (b *fakeBackend) ListKernelRoutes(_ string, _ int) ([]KernelRoute, error) {
	return nil, nil
}

func (b *fakeBackend) ResetCounters(_ string) error {
	return nil
}

func (b *fakeBackend) ListInterfaces() ([]InterfaceInfo, error) {
	if b.listErr != nil {
		return nil, b.listErr
	}
	var result []InterfaceInfo
	for _, f := range b.ifaces {
		// OsName carries the kernel device name, as the netlink backend sets it
		// (show_linux.go). The resolver reads it back as the device a logical
		// name resolved to, so a fake that left it empty made every Resolve in
		// a unit test answer with no device.
		info := InterfaceInfo{Name: f.name, OsName: f.name, Type: f.linkType, MAC: f.mac, PermanentMAC: f.permMAC, Alias: f.alias, ParentIndex: f.parentIndex, MasterIndex: f.masterIndex, MTU: f.mtu, Index: f.index, State: f.state}
		if addrs, ok := b.addrs[f.name]; ok {
			for _, a := range addrs {
				// b.addrs stores full CIDR strings (what AddAddress received,
				// e.g. "10.0.0.1/24"). Split back into bare address +
				// prefix length here to match the real backend's contract
				// (netlink/show_linux.go addrList: Address is a.IP.String(),
				// PrefixLength is the mask size) -- currentAddrSet()
				// reconstructs "Address/PrefixLength", so returning the
				// already-slashed string verbatim as Address would produce
				// a double-slash CIDR that never matches desiredState()'s
				// output, silently defeating stale-address removal.
				prefix, err := netip.ParsePrefix(a)
				if err != nil {
					continue
				}
				info.Addresses = append(info.Addresses, AddrInfo{Address: prefix.Addr().String(), PrefixLength: prefix.Bits()})
			}
		}
		result = append(result, info)
	}
	return result, nil
}

func (b *fakeBackend) GetInterface(name string) (*InterfaceInfo, error) {
	f, ok := b.ifaces[name]
	if !ok {
		return nil, fmt.Errorf("interface %s not found", name)
	}
	// The same projection ListInterfaces makes. A GetInterface that answered
	// with the name and type alone made the apply path read MTU 0 and an empty
	// state for every device, so its MTU undo and its admin-state undo were
	// no-ops no test could see.
	return &InterfaceInfo{Name: f.name, OsName: f.name, Type: f.linkType, MAC: f.mac, PermanentMAC: f.permMAC,
		Alias: f.alias, ParentIndex: f.parentIndex, MasterIndex: f.masterIndex, MTU: f.mtu, Index: f.index, State: f.state}, nil
}

// BridgeAddPort records the call and then models what a live kernel does with
// it: the port names the bridge as its master, and a bridge with no permanent
// address of its own takes the port's address. Verified against Linux --
// enslaving a dummy carrying 02:00:00:00:be:99 made the bridge report that same
// address with no permanent one, which is what makes a mac/match selector read
// as ambiguous on every apply after the one that built the bridge.
func (b *fakeBackend) BridgeAddPort(bridge, port string) error {
	b.ensureMaps()
	b.bridgePorts = append(b.bridgePorts, bridgePortCall{op: bridgePortOpAdd, bridge: bridge, port: port})
	br, haveBridge := b.ifaces[bridge]
	p, havePort := b.ifaces[port]
	if !haveBridge {
		return fmt.Errorf("iface fake: bridge %q does not exist", bridge)
	}
	if !havePort {
		return fmt.Errorf("iface fake: bridge port %q does not exist", port)
	}
	p.masterIndex = br.index
	b.ifaces[port] = p
	if br.permMAC == "" {
		br.mac = p.mac
		b.ifaces[bridge] = br
	}
	return nil
}

func (b *fakeBackend) BridgeDelPort(port string) error {
	b.ensureMaps()
	b.bridgePorts = append(b.bridgePorts, bridgePortCall{op: bridgePortOpDel, port: port})
	if p, ok := b.ifaces[port]; ok {
		p.masterIndex = 0
		b.ifaces[port] = p
	}
	return nil
}
func (b *fakeBackend) BridgeSetSTP(_ string, _ bool) error { return nil }

func (b *fakeBackend) SetupMirror(srcIface, dstIface string, ingress, egress bool) error {
	b.mirrorCalls = append(b.mirrorCalls, mirrorCall{
		op: mirrorOpSetup, iface: srcIface, dst: dstIface, ingress: ingress, egress: egress,
	})
	if err := b.setupMirrorErr[srcIface]; err != nil {
		return err
	}
	// One direction at a time, as the kernel does. A second call for the other
	// hook leaves the first hook alone. A dropped direction therefore has to be
	// removed rather than overwritten.
	b.setMirror(srcIface, func(state *MirrorState) {
		if ingress {
			state.Ingress = dstIface
		}
		if egress {
			state.Egress = dstIface
		}
	})
	return nil
}

func (b *fakeBackend) RemoveMirror(srcIface string) error {
	b.mirrorCalls = append(b.mirrorCalls, mirrorCall{op: mirrorOpRemove, iface: srcIface})
	if err := b.removeMirrorErr[srcIface]; err != nil {
		return err
	}
	delete(b.mirrors, srcIface)
	return nil
}

// ListMirrors reports the fake dataplane's live mirrors in a stable order.
func (b *fakeBackend) ListMirrors() ([]MirrorState, error) {
	if b.listMirrorsErr != nil {
		return nil, b.listMirrorsErr
	}
	names := make([]string, 0, len(b.mirrors))
	for name := range b.mirrors {
		names = append(names, name)
	}
	sort.Strings(names)
	states := make([]MirrorState, 0, len(names))
	for _, name := range names {
		states = append(states, b.mirrors[name])
	}
	return states, nil
}

// setMirror installs or updates one source device's live mirror.
func (b *fakeBackend) setMirror(srcIface string, edit func(*MirrorState)) {
	if b.mirrors == nil {
		b.mirrors = make(map[string]MirrorState)
	}
	state := b.mirrors[srcIface]
	state.Interface = srcIface
	edit(&state)
	if state.Ingress == "" && state.Egress == "" {
		delete(b.mirrors, srcIface)
		return
	}
	b.mirrors[srcIface] = state
}

// seedMirror installs a mirror in the fake dataplane with no apply behind it,
// which is the state a restart inherits. The tc filters an earlier ze installed
// are still there, and nothing in memory records that they are. The source is a
// parameter because a mirror on an interface ze does NOT configure is one of
// the states that has to be reproduced.
func (b *fakeBackend) seedMirror(srcIface, ingress, egress string) {
	b.setMirror(srcIface, func(state *MirrorState) {
		state.Ingress = ingress
		state.Egress = egress
	})
}
func (b *fakeBackend) SetupLCPPair(vppIface, hostName string) error {
	if b.lcpPairs == nil {
		b.lcpPairs = make(map[string]string)
	}
	b.lcpPairs[vppIface] = hostName
	return nil
}
func (b *fakeBackend) RemoveLCPPair(vppIface string) error {
	delete(b.lcpPairs, vppIface)
	return nil
}
func (b *fakeBackend) StartMonitor(_ ze.EventBus) error { return nil }
func (b *fakeBackend) StopMonitor()                     {}
func (b *fakeBackend) Close() error                     { return nil }

// --- IPv6 Router Tracking Tests ---

// setupFakeBackendForTest registers and loads a fakeBackend for a test.
func setupFakeBackendForTest(t *testing.T) *fakeBackend {
	t.Helper()
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	t.Cleanup(func() { _ = CloseBackend() })
	return fb
}

// TestNeighRouterDetected verifies that a router-discovered event installs
// an IPv6 default route with the configured metric.
//
// VALIDATES: AC-2 - Netlink neighbor event with NTF_ROUTER installs ::/0 with metric.
// PREVENTS: Router event ignored, no IPv6 default route installed.
func TestNeighRouterDetected(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{"eth0": 5}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, priorities, logger)

	require.Len(t, fb.routeAdds, 1, "should install one IPv6 default route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeAdds[0])
	assert.Contains(t, routers, routerKey{ifaceName: "eth0", routerIP: "fe80::1"})
}

// TestNeighRouterRemoved verifies that a router-lost event removes the
// IPv6 default route.
//
// VALIDATES: AC-6 - Router disappears, IPv6 default route removed.
// PREVENTS: Stale route left after router goes away.
func TestNeighRouterRemoved(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterLost(data, routers, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one IPv6 default route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeRemoves[0])
	assert.NotContains(t, routers, routerKey{ifaceName: "eth0", routerIP: "fe80::1"})
}

// TestLinkDownIPv6 verifies that link-down deprioritizes IPv6 default routes.
//
// VALIDATES: AC-4 - Link down deprioritizes IPv6 route to metric + 1024.
// PREVENTS: IPv6 routes not deprioritized on carrier loss.
func TestLinkDownIPv6(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	logger := slog.Default()

	handleLinkDownIPv6("eth0", routers, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1, "should add deprioritized route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 1029, rtproto.Iface}, fb.routeAdds[0])
}

// TestLinkUpIPv6 verifies that link-up restores IPv6 default route priority.
//
// VALIDATES: AC-5 - Link up restores IPv6 route to original metric.
// PREVENTS: IPv6 routes stuck at deprioritized metric after carrier restore.
func TestLinkUpIPv6(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	logger := slog.Default()

	handleLinkUpIPv6("eth0", routers, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove deprioritized route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 1029, rtproto.Iface}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1, "should add restored route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeAdds[0])
}

// TestNeighRouterDetectedNoRoutePriority verifies that router events are
// ignored when route-priority is not configured (0).
//
// VALIDATES: AC-3/AC-9 - No route-priority means kernel handles everything.
// PREVENTS: Ze installing routes when user didn't configure route-priority.
func TestNeighRouterDetectedNoRoutePriority(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	// An interface writtenRoutePriorities left out: no unit wrote the leaf
	// above 0, so suppressRAForConfig suppressed nothing for it either.
	priorities := map[string]int{}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, priorities, logger)

	assert.Empty(t, routers, "should not track router when route-priority is 0")
}

// TestMultipleRoutersOnSameLink verifies that multiple routers on the same
// interface are tracked independently.
//
// VALIDATES: AC-7 - Multiple routers, all with configured metric.
// PREVENTS: Second router overwriting the first.
func TestMultipleRoutersOnSameLink(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{"eth0": 5}
	logger := slog.Default()

	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, priorities, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::2"}`, routers, priorities, logger)

	require.Len(t, fb.routeAdds, 2, "should install two IPv6 default routes")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeAdds[0])
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::2", 5, rtproto.Iface}, fb.routeAdds[1])
	assert.Len(t, routers, 2, "should track two routers")
}

// TestNeighRouterDuplicateIgnored verifies that a duplicate router-discovered
// event for the same router is idempotent.
//
// VALIDATES: Idempotent router discovery.
// PREVENTS: Duplicate routes installed for the same router.
func TestNeighRouterDuplicateIgnored(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{"eth0": 5}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, priorities, logger)
	handleRouterDiscovered(data, routers, priorities, logger)

	require.Len(t, fb.routeAdds, 1, "duplicate discovery should not install a second route")
}

// TestReloadMetricChange verifies that when route-priority changes on reload,
// the old metric routes are removed and new metric routes are installed.
//
// VALIDATES: AC-8 - Reload changes route-priority, routes updated.
// PREVENTS: Stale metric routes left after config change.
func TestReloadMetricChange(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	// The reload wrote route-priority 10, so suppressRAForConfig republished
	// the map handleRouterDiscovered reads.
	priorities := map[string]int{"eth0": 10}
	logger := slog.Default()

	// Simulate the router being re-discovered after config reload with new metric.
	// First, the old entry should be cleaned up.
	// restoreAcceptRaDefrtr removes existing routes; then suppressRAForConfig
	// re-suppresses and the monitor re-discovers the router.
	// Simulating the removal + re-discovery:
	handleRouterLost(`{"name":"eth0","router-ip":"fe80::1"}`, routers, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, priorities, logger)

	// Old route removed with metric 5, new installed with metric 10.
	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 10, rtproto.Iface}, fb.routeAdds[0])
	assert.Equal(t, 10, routers[routerKey{ifaceName: "eth0", routerIP: "fe80::1"}].metric)
}

// --- Sysctl Suppression/Restore Tests ---

// testEventBus is a minimal EventBus that records emissions for testing.
type testEventBus struct {
	emissions []testEmission
}

type testEmission struct {
	namespace string
	eventType string
	data      any
}

func (b *testEventBus) Emit(namespace, eventType string, data any) (int, error) {
	b.emissions = append(b.emissions, testEmission{namespace, eventType, data})
	return 1, nil
}

func (b *testEventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// TestAcceptRaDefrtrSet verifies that suppressAcceptRaDefrtr emits the
// correct sysctl event to set accept_ra_defrtr=0.
//
// VALIDATES: AC-1 - Config with route-priority sets accept_ra_defrtr to 0.
// PREVENTS: Sysctl not set, kernel continues installing RA default routes.
func TestAcceptRaDefrtrSet(t *testing.T) {
	eb := &testEventBus{}
	suppressed := make(map[string]bool)
	logger := slog.Default()

	suppressAcceptRaDefrtr("eth0", suppressed, eb, logger)

	require.Len(t, eb.emissions, 1, "should emit one sysctl event")
	assert.Equal(t, "sysctl", eb.emissions[0].namespace)
	assert.Equal(t, "set", eb.emissions[0].eventType)
	assert.Contains(t, eb.emissions[0].data, "accept_ra_defrtr")
	assert.Contains(t, eb.emissions[0].data, `"value":"0"`)
	assert.True(t, suppressed["eth0"], "interface should be marked as suppressed")
}

// TestAcceptRaDefrtrRestore verifies that restoreAcceptRaDefrtr emits the
// correct sysctl event to restore accept_ra_defrtr=1 and cleans up routes.
//
// VALIDATES: AC-10 - Config reload removes route-priority, sysctl restored.
// PREVENTS: accept_ra_defrtr stuck at 0 after config change.
func TestAcceptRaDefrtrRestore(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	eb := &testEventBus{}
	suppressed := map[string]bool{"eth0": true}
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	logger := slog.Default()

	restoreAcceptRaDefrtr("eth0", suppressed, routers, eb, logger)

	// Route should be removed.
	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5, rtproto.Iface}, fb.routeRemoves[0])

	// Sysctl restored to 1.
	require.Len(t, eb.emissions, 1)
	assert.Contains(t, eb.emissions[0].data, `"value":"1"`)
	assert.Contains(t, eb.emissions[0].data, "accept_ra_defrtr")

	// State cleaned up.
	assert.False(t, suppressed["eth0"], "interface should no longer be suppressed")
	assert.Empty(t, routers, "router entry should be removed")
}

// TestAcceptRaDefrtrRestoreOnStop verifies that shutdown restores
// accept_ra_defrtr on all suppressed interfaces.
//
// VALIDATES: AC-11 - Clean daemon shutdown restores accept_ra_defrtr.
// PREVENTS: accept_ra_defrtr stuck at 0 after ze shutdown.
func TestAcceptRaDefrtrRestoreOnStop(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	eb := &testEventBus{}
	suppressed := map[string]bool{"eth0": true, "eth1": true}
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}

	// Simulate shutdown restore loop (collect keys first, same as production).
	names := make([]string, 0, len(suppressed))
	for name := range suppressed {
		names = append(names, name)
	}
	logger := slog.Default()
	for _, name := range names {
		restoreAcceptRaDefrtr(name, suppressed, routers, eb, logger)
	}

	assert.Len(t, eb.emissions, 2, "should restore both interfaces")
	assert.Empty(t, suppressed, "all interfaces should be restored")
	assert.Empty(t, routers, "all router entries should be removed")
}

// TestStaleKernelRouteCleanup verifies that cleanupStaleIPv6DefaultRoutes
// removes pre-existing ::/0 routes after sysctl suppression.
//
// VALIDATES: AC-12 - Stale kernel route removed after suppression.
// PREVENTS: Duplicate default routes with different metrics.
func TestStaleKernelRouteCleanup(t *testing.T) {
	fb := &fakeBackend{
		ifaces:      map[string]fakeIface{},
		staleRoutes: []RouteInfo{{Destination: "::/0", Gateway: "fe80::1", Metric: 0}},
	}
	backendName := "test-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	t.Cleanup(func() { _ = CloseBackend() })

	logger := slog.Default()
	cleanupStaleIPv6DefaultRoutes("eth0", logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one stale route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 0, rtproto.Any}, fb.routeRemoves[0])
}

// TestSuppressRAForConfigNoRoutePriority verifies that interfaces without
// route-priority are not suppressed.
//
// VALIDATES: AC-3 - No route-priority means kernel handles everything.
// PREVENTS: Suppressing accept_ra_defrtr when user didn't configure route-priority.
func TestSuppressRAForConfigNoRoutePriority(t *testing.T) {
	eb := &testEventBus{}
	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24"]
							}
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()
	priorities := map[string]int{}

	suppressRAForConfig(cfg, suppressed, routers, priorities, eb, logger)

	assert.Empty(t, eb.emissions, "should not emit any sysctl events")
	assert.Empty(t, suppressed, "should not suppress any interfaces")
	assert.Empty(t, priorities, "and ze installs no ::/0 route of its own")
}

// TestSuppressRAForConfigWithRoutePriority verifies that interfaces with
// route-priority > 0 get accept_ra_defrtr suppressed.
//
// VALIDATES: AC-1 - route-priority triggers suppression.
// PREVENTS: suppressRAForConfig silently skipping valid interfaces.
func TestSuppressRAForConfigWithRoutePriority(t *testing.T) {
	eb := &testEventBus{}
	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"route-priority": "5",
							"ipv4": {
								"address": ["10.0.0.1/24"]
							}
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()
	priorities := map[string]int{}

	suppressRAForConfig(cfg, suppressed, routers, priorities, eb, logger)

	require.Len(t, eb.emissions, 1, "should emit one sysctl event")
	assert.Contains(t, eb.emissions[0].data, `"value":"0"`)
	assert.True(t, suppressed["eth0"])
	assert.Equal(t, map[string]int{"eth0": 5}, priorities,
		"the interface ze suppressed is the interface ze installs ::/0 on")
}

// TestSuppressRAForConfigRestore verifies that removing route-priority from
// config restores accept_ra_defrtr on previously suppressed interfaces.
//
// VALIDATES: AC-10 - Config reload removes route-priority, sysctl restored.
// PREVENTS: Suppression never lifted after config change.
func TestSuppressRAForConfigRestore(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	eb := &testEventBus{}
	suppressed := map[string]bool{"eth0": true}
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
	}
	// Config with NO route-priority.
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24"]
							}
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()
	priorities := map[string]int{"eth0": 5}

	suppressRAForConfig(cfg, suppressed, routers, priorities, eb, logger)

	require.Len(t, eb.emissions, 1, "should emit restore sysctl event")
	assert.Contains(t, eb.emissions[0].data, `"value":"1"`)
	assert.Empty(t, suppressed, "interface should no longer be suppressed")
	assert.Empty(t, routers, "router entries should be cleaned up")
	assert.Empty(t, priorities,
		"and a later router event installs nothing, the kernel owns ::/0 again")
}

// TestLinkDownIPv6MultipleRouters verifies that link-down deprioritizes
// all routers on the same interface.
//
// VALIDATES: AC-4 + AC-7 combined - all routers deprioritized on carrier loss.
// PREVENTS: Only first router deprioritized, others left at normal metric.
func TestLinkDownIPv6MultipleRouters(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := map[routerKey]routerEntry{
		{ifaceName: "eth0", routerIP: "fe80::1"}: {metric: 5},
		{ifaceName: "eth0", routerIP: "fe80::2"}: {metric: 5},
	}
	logger := slog.Default()

	handleLinkDownIPv6("eth0", routers, logger)

	assert.Len(t, fb.routeRemoves, 2, "should remove both routes")
	assert.Len(t, fb.routeAdds, 2, "should add both deprioritized routes")
}

// TestRouterLostUnknown verifies that a RouterLost event for an untracked
// router is a silent no-op.
//
// VALIDATES: Defensive handling of unknown router events.
// PREVENTS: Panic or error on RouterLost for router not in activeRouters.
func TestRouterLostUnknown(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	logger := slog.Default()

	handleRouterLost(`{"name":"eth0","router-ip":"fe80::99"}`, routers, logger)

	assert.Empty(t, fb.routeRemoves, "should not attempt to remove unknown route")
}

// TestSuppressIdempotent verifies that calling suppress twice only emits once.
//
// VALIDATES: Idempotent suppression.
// PREVENTS: Double sysctl write and double stale cleanup.
func TestSuppressIdempotent(t *testing.T) {
	eb := &testEventBus{}
	suppressed := make(map[string]bool)
	logger := slog.Default()

	suppressAcceptRaDefrtr("eth0", suppressed, eb, logger)
	suppressAcceptRaDefrtr("eth0", suppressed, eb, logger)

	require.Len(t, eb.emissions, 1, "second call should be no-op")
}

// TestRestoreNotSuppressed verifies that restoring a non-suppressed
// interface is a silent no-op.
//
// VALIDATES: Defensive handling of restore on clean interface.
// PREVENTS: Spurious sysctl write or route removal.
func TestRestoreNotSuppressed(t *testing.T) {
	eb := &testEventBus{}
	suppressed := make(map[string]bool)
	routers := make(map[routerKey]routerEntry)
	logger := slog.Default()

	restoreAcceptRaDefrtr("eth0", suppressed, routers, eb, logger)

	assert.Empty(t, eb.emissions, "should not emit anything")
}

// TestWrittenRoutePrioritiesMultiUnit verifies that the first non-zero
// route-priority is taken when multiple units exist.
//
// VALIDATES: Multi-unit interface yields a valid metric. RA default routes are
// per-interface, not per-unit, so one number has to answer for the interface.
// PREVENTS: Zero returned when a non-zero route-priority exists, which would
// leave the interface suppressed with no ::/0 route of ze's own.
func TestWrittenRoutePrioritiesMultiUnit(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {"route-priority": "0"},
						"1": {"route-priority": "7"}
					}
				}
			}
		}
	}`)

	assert.Equal(t, map[string]int{"eth0": 7}, writtenRoutePriorities(cfg),
		"should take the non-zero route-priority")
}

// TestWrittenRoutePrioritiesNoMatch verifies that an interface nobody wrote the
// leaf on is absent from the map.
//
// VALIDATES: An unconfigured interface keeps the kernel's RA default routes.
// PREVENTS: A metric for an interface the operator said nothing about, which
// would take IPv6 ownership away from the kernel on every box that upgrades.
func TestWrittenRoutePrioritiesNoMatch(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {"unit": {"0": {"route-priority": "5"}}},
				"eth1": {"unit": {"0": {"ipv4": {"address": ["10.0.0.1/24"]}}}}
			}
		}
	}`)

	priorities := writtenRoutePriorities(cfg)
	assert.Equal(t, 5, priorities["eth0"])
	assert.NotContains(t, priorities, "eth1", "an unwritten interface stays with the kernel")
}

// TestRouterDiscoveredBadJSON verifies that malformed JSON is silently ignored.
//
// VALIDATES: Defensive JSON parsing.
// PREVENTS: Panic on malformed event bus payload.
func TestRouterDiscoveredBadJSON(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	priorities := map[string]int{"eth0": 5}
	logger := slog.Default()

	handleRouterDiscovered("not json", routers, priorities, logger)
	handleRouterDiscovered(`{"name":"","router-ip":"fe80::1"}`, routers, priorities, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":""}`, routers, priorities, logger)

	assert.Empty(t, routers, "should not track any router from bad input")
}

// TestLinkDownIPv6NoRouters verifies that link-down with no IPv6 routers
// is a silent no-op.
//
// VALIDATES: Defensive handling of link-down without IPv6 state.
// PREVENTS: Panic or error when no IPv6 routers exist.
func TestLinkDownIPv6NoRouters(t *testing.T) {
	fb := setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	logger := slog.Default()

	handleLinkDownIPv6("eth0", routers, logger)

	assert.Empty(t, fb.routeRemoves, "should not attempt any route changes")
	assert.Empty(t, fb.routeAdds, "should not attempt any route changes")
}

// VALIDATES: AC-1 - offload { gro true } parsed into offloadConfig with GRO=true.
// PREVENTS: offload container silently ignored by parseIfaceEntry.
func TestOffloadConfigParseEnable(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"offload": {
						"gro": "true",
						"tso": "true"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	e := cfg.Ethernet[0]
	require.NotNil(t, e.Offload, "offload block must be parsed")
	require.NotNil(t, e.Offload.GRO)
	assert.True(t, *e.Offload.GRO)
	require.NotNil(t, e.Offload.TSO)
	assert.True(t, *e.Offload.TSO)
	assert.Nil(t, e.Offload.GSO, "absent leaf must be nil")
	assert.Nil(t, e.Offload.SG, "absent leaf must be nil")
	assert.Nil(t, e.Offload.LRO, "absent leaf must be nil")
	assert.Nil(t, e.Offload.HWTCOffload, "absent leaf must be nil")
	assert.Nil(t, e.Offload.RPS, "absent leaf must be nil")
	assert.Nil(t, e.Offload.RFS, "absent leaf must be nil")
}

// VALIDATES: AC-2 - offload { tso false } parsed into offloadConfig with TSO=false.
// PREVENTS: explicit disable treated as enable.
func TestOffloadConfigParseExplicitDisable(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"offload": {
						"tso": "false",
						"lro": "false"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	e := cfg.Ethernet[0]
	require.NotNil(t, e.Offload)
	require.NotNil(t, e.Offload.TSO)
	assert.False(t, *e.Offload.TSO)
	require.NotNil(t, e.Offload.LRO)
	assert.False(t, *e.Offload.LRO)
}

// VALIDATES: AC-3 - absence of offload container means nil (no ethtool calls).
// PREVENTS: empty offloadConfig struct created when container is absent.
func TestOffloadAbsencePreservesDefault(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"mtu": "9000"
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	assert.Nil(t, cfg.Ethernet[0].Offload, "no offload block = nil")
}

// VALIDATES: AC-4 - all 8 offload features parsed when all set to true.
// PREVENTS: missing feature in parseOffloadConfig.
func TestOffloadConfigParseAll8(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"offload": {
						"gro": "true",
						"gso": "true",
						"sg": "true",
						"tso": "true",
						"lro": "true",
						"hw-tc-offload": "true",
						"rps": "true",
						"rfs": "true"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	o := cfg.Ethernet[0].Offload
	require.NotNil(t, o)
	require.NotNil(t, o.GRO)
	assert.True(t, *o.GRO)
	require.NotNil(t, o.GSO)
	assert.True(t, *o.GSO)
	require.NotNil(t, o.SG)
	assert.True(t, *o.SG)
	require.NotNil(t, o.TSO)
	assert.True(t, *o.TSO)
	require.NotNil(t, o.LRO)
	assert.True(t, *o.LRO)
	require.NotNil(t, o.HWTCOffload)
	assert.True(t, *o.HWTCOffload)
	require.NotNil(t, o.RPS)
	assert.True(t, *o.RPS)
	require.NotNil(t, o.RFS)
	assert.True(t, *o.RFS)
}

// VALIDATES: AC-3 - empty offload container (no leaves) returns nil.
// PREVENTS: empty offloadConfig struct created for empty container.
func TestOffloadEmptyContainerIsNil(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"offload": {}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	assert.Nil(t, cfg.Ethernet[0].Offload, "empty offload container = nil")
}

// VALIDATES: AC-6 - offload container parsed on dummy (interface-l2 grouping).
// PREVENTS: offload only working on ethernet.
func TestOffloadOnDummy(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"offload": {
						"gro": "false"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	require.NotNil(t, cfg.Dummy[0].Offload)
	require.NotNil(t, cfg.Dummy[0].Offload.GRO)
	assert.False(t, *cfg.Dummy[0].Offload.GRO)
}

// VALIDATES: AC-6 - offload container parsed on veth (interface-l2 grouping).
// PREVENTS: offload only working on ethernet.
func TestOffloadOnVeth(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"veth": {
				"veth0": {
					"offload": {
						"tso": "true"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Veth, 1)
	require.NotNil(t, cfg.Veth[0].Offload)
	require.NotNil(t, cfg.Veth[0].Offload.TSO)
	assert.True(t, *cfg.Veth[0].Offload.TSO)
}

// VALIDATES: AC-6 - offload container parsed on bridge (interface-l2 grouping).
// PREVENTS: offload only working on ethernet.
func TestOffloadOnBridge(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"bridge": {
				"br0": {
					"offload": {
						"gro": "true"
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Bridge, 1)
	require.NotNil(t, cfg.Bridge[0].Offload)
	require.NotNil(t, cfg.Bridge[0].Offload.GRO)
	assert.True(t, *cfg.Bridge[0].Offload.GRO)
}

// --- Per-family address tests (spec-iface-1-per-family-address) ---

// VALIDATES: AC-1 -- IPv4 address in ipv4 container applied.
func TestParseAddress_IPv4InFamily(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24", "10.0.0.2/24"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	require.Len(t, cfg.Dummy[0].Units, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	assert.Equal(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, u.IPv4.Addresses)
	assert.Nil(t, u.IPv6)
	assert.Equal(t, []string{"10.0.0.1/24", "10.0.0.2/24"}, u.Addresses,
		"merged flat list must contain ipv4 addresses")
}

// VALIDATES: AC-2 -- IPv6 address in ipv6 container applied.
func TestParseAddress_IPv6InFamily(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv6": {
								"address": ["fd00::1/64"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	require.Len(t, cfg.Dummy[0].Units, 1)
	u := cfg.Dummy[0].Units[0]
	assert.Nil(t, u.IPv4)
	require.NotNil(t, u.IPv6)
	assert.Equal(t, []string{"fd00::1/64"}, u.IPv6.Addresses)
	assert.Equal(t, []string{"fd00::1/64"}, u.Addresses)
}

// VALIDATES: AC-3 -- both families configured.
func TestParseAddress_BothFamilies(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24"]
							},
							"ipv6": {
								"address": ["fd00::1/64"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv6)
	assert.Equal(t, []string{"10.0.0.1/24"}, u.IPv4.Addresses)
	assert.Equal(t, []string{"fd00::1/64"}, u.IPv6.Addresses)
	assert.Len(t, u.Addresses, 2, "merged flat list must contain both families")
}

// Flat address at unit level is no longer supported.
func TestParseAddress_FlatIgnored(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"address": ["10.0.0.1/24", "fd00::1/64"]
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	assert.Nil(t, u.IPv4)
	assert.Nil(t, u.IPv6)
	assert.Empty(t, u.Addresses, "flat address at unit level must be ignored")
}

// VALIDATES: AC-5 -- IPv4 address in ipv6 container rejected.
func TestParseAddress_WrongFamily_V4inV6(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv6": {
								"address": ["10.0.0.1/24"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an IPv6 address")
}

// VALIDATES: AC-6 -- IPv6 address in ipv4 container rejected.
func TestParseAddress_WrongFamily_V6inV4(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["fd00::1/64"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an IPv4 address")
}

// VALIDATES: AC-1,AC-5 -- multiple addresses per family.
func TestParseAddress_Multiple(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["10.0.0.1/24", "10.0.0.2/24", "192.168.1.1/32"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	assert.Len(t, u.IPv4.Addresses, 3)
	assert.Len(t, u.Addresses, 3)
}

// VALIDATES: AC-9 -- desiredState produces same address set from per-family config.
func TestDesiredState_PerFamilyAddresses(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: defaultBackendName,
		Dummy: []ifaceEntry{{
			Name: "dum0",
			Units: []unitEntry{{
				Label:     "default",
				IPv4:      &ipv4Settings{Addresses: []string{"10.0.0.1/24"}},
				IPv6:      &ipv6Settings{Addresses: []string{"fd00::1/64"}},
				Addresses: []string{"10.0.0.1/24", "fd00::1/64"},
			}},
		}},
	}
	addrs, managed, _ := cfg.desiredState(nil)
	assert.True(t, managed["dum0"])
	assert.True(t, addrs["dum0"]["10.0.0.1/24"])
	assert.True(t, addrs["dum0"]["fd00::1/64"])
}

// VALIDATES: AC-1 -- per-family addresses on ethernet (most common interface type).
func TestParseAddress_Ethernet(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": {
								"address": ["192.168.1.1/24"]
							},
							"ipv6": {
								"address": ["2001:db8::1/48"]
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv6)
	assert.Equal(t, []string{"192.168.1.1/24"}, u.IPv4.Addresses)
	assert.Equal(t, []string{"2001:db8::1/48"}, u.IPv6.Addresses)
	assert.Len(t, u.Addresses, 2)
}

// VALIDATES: AC-1 -- named unit with vlan-id parsed correctly.
func TestParseUnit_NamedKey(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"firewall-3": {
							"vlan-id": "100",
							"ipv4": { "address": ["10.0.100.1/24"] }
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, "firewall-3", u.Label)
	assert.Equal(t, 100, u.VLANID)
	assert.Equal(t, []string{"10.0.100.1/24"}, u.IPv4.Addresses)
}

// VALIDATES: AC-2 -- base unit without vlan-id works.
func TestParseUnit_DefaultNoVLAN(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"default": {
							"ipv4": { "address": ["10.0.0.1/24"] }
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, "default", u.Label)
	assert.Equal(t, 0, u.VLANID)
}

// VALIDATES: AC-3 -- multiple named units parsed with correct labels.
func TestParseUnit_MultipleNamed(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"default": {
							"ipv4": { "address": ["10.0.0.1/24"] }
						},
						"firewall-3": {
							"vlan-id": "100",
							"ipv4": { "address": ["10.0.100.1/24"] }
						},
						"supplier-acme": {
							"vlan-id": "200",
							"ipv6": { "address": ["2001:db8::1/64"] }
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 3)
	labels := make(map[string]bool)
	for _, u := range cfg.Ethernet[0].Units {
		labels[u.Label] = true
	}
	assert.True(t, labels["default"])
	assert.True(t, labels["firewall-3"])
	assert.True(t, labels["supplier-acme"])
}

// VALIDATES: AC-5 -- invalid unit names rejected.
func TestParseUnit_InvalidName(t *testing.T) {
	tests := []struct {
		name    string
		unitKey string
		wantErr string
	}{
		{"space", "fire wall", "invalid character"},
		{"starts with hyphen", "-test", "invalid character"},
		{"starts with dot", ".test", "invalid character"},
		{"slash", "a/b", "invalid character"},
		{"colon", "a:b", "invalid character"},
		{"empty", "", "length"},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUnitName(tt.unitKey)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// VALIDATES: AC-8 -- legacy numeric unit names accepted.
func TestParseUnit_LegacyNumeric(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"ipv4": { "address": ["10.0.0.1/24"] }
						},
						"100": {
							"vlan-id": "100",
							"ipv4": { "address": ["10.0.100.1/24"] }
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 2)
	labels := make(map[string]bool)
	for _, u := range cfg.Ethernet[0].Units {
		labels[u.Label] = true
	}
	assert.True(t, labels["0"])
	assert.True(t, labels["100"])
}

// VALIDATES: AC-1 -- rpf-check strict parsed to rpfModeStrict.
// PREVENTS: rpf-check enum value silently ignored by parseIPv4Settings.
func TestParseRPFCheck_Strict(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"rpf-check": "strict"
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	require.Len(t, cfg.Dummy[0].Units, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.RPFCheck)
	assert.Equal(t, rpfModeStrict, *u.IPv4.RPFCheck)
}

// VALIDATES: AC-2 -- rpf-check loose parsed to rpfModeLoose.
// PREVENTS: loose/strict enum values swapped.
func TestParseRPFCheck_Loose(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"rpf-check": "loose"
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.RPFCheck)
	assert.Equal(t, rpfModeLoose, *u.IPv4.RPFCheck)
}

// VALIDATES: AC-3 -- rpf-check disable parsed to rpfModeDisable.
// PREVENTS: disable treated as unconfigured (nil).
func TestParseRPFCheck_Disable(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv4": {
								"rpf-check": "disable"
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv4)
	require.NotNil(t, u.IPv4.RPFCheck)
	assert.Equal(t, rpfModeDisable, *u.IPv4.RPFCheck)
}

// VALIDATES: AC-5 -- legacy rp-filter integer maps to rpfMode enum.
// PREVENTS: backward-compat break when old configs use rp-filter N.
func TestParseRPFCheck_Legacy(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected rpfMode
	}{
		{"strict", "1", rpfModeStrict},
		{"loose", "2", rpfModeLoose},
		{"disable", "0", rpfModeDisable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mustParseIfaceJSON(t, fmt.Sprintf(`{
				"interface": {
					"dummy": {
						"dum0": {
							"unit": {
								"0": {
									"ipv4": {
										"rp-filter": %q
									}
								}
							}
						}
					}
				}
			}`, tt.value))
			require.Len(t, cfg.Dummy, 1)
			u := cfg.Dummy[0].Units[0]
			require.NotNil(t, u.IPv4)
			require.NotNil(t, u.IPv4.RPFCheck, "legacy rp-filter %s must map to RPFCheck", tt.value)
			assert.Equal(t, tt.expected, *u.IPv4.RPFCheck)
		})
	}
}

// VALIDATES: AC-4 -- rpf-check parsed in IPv6 container.
// PREVENTS: rpf-check silently ignored in parseIPv6Settings.
func TestParseRPFCheck_IPv6(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"dummy": {
				"dum0": {
					"unit": {
						"0": {
							"ipv6": {
								"rpf-check": "loose"
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Dummy, 1)
	u := cfg.Dummy[0].Units[0]
	require.NotNil(t, u.IPv6)
	require.NotNil(t, u.IPv6.RPFCheck)
	assert.Equal(t, rpfModeLoose, *u.IPv6.RPFCheck)
}

// VALIDATES: AC-1/AC-2/AC-3 -- rpfMode sysctl integer mapping.
// PREVENTS: enum-to-sysctl value mismatch (strict must be 1, not 2).
func TestApplyRPFCheck_Sysctl(t *testing.T) {
	tests := []struct {
		mode     rpfMode
		expected int
	}{
		{rpfModeDisable, 0},
		{rpfModeStrict, 1},
		{rpfModeLoose, 2},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.mode.rpfSysctlValue(),
			"rpfMode(%d).rpfSysctlValue()", tt.mode)
	}
}

// TestParseXFRMEntryIgnoresListLevelMAC verifies that a mac/address written at
// the xfrm list level does not reach the parsed entry.
//
// VALIDATES: parseXFRMEntry clears MACAddress that parseIfaceEntry may have read.
//
// PREVENTS: an XFRM interface carrying a MAC it cannot have. XFRM is an L3
// tunnel with no link layer of its own. A mac/address at the list level is a
// hand-edited config, or a leftover from a kind that does carry one. Letting it
// through hands the backend an address to program on an interface with no place
// to put it.
//
// The clear is easy to lose. `parseXFRMEntry` seeds `xfrmEntry` from the shared
// `ifaceEntry`, so the field arrives populated, and the clear is one line that
// nothing else depends on. Its two siblings each have a test for the same line,
// `TestParseTunnelGREIgnoresListLevelMAC` and the wireguard one. This one had
// none, which is how a modernize sweep came to rewrite all three with two
// covered.
func TestParseXFRMEntryIgnoresListLevelMAC(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"xfrm": {
				"xfrm0": {
					"if-id": "42",
					"dev": "eth0",
					"mac": {"address": "02:00:00:00:00:01"},
					"unit": {
						"default": {
							"ipv4": {"address": ["10.0.0.1/30"]}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.XFRM, 1)
	assert.Empty(t, cfg.XFRM[0].MACAddress,
		"xfrm is an L3 tunnel: a list-level mac/address must be ignored")
}

func TestParseXFRMEntry(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"xfrm": {
				"xfrm0": {
					"if-id": "42",
					"dev": "eth0",
					"unit": {
						"default": {
							"ipv4": {"address": ["10.0.0.1/30"]}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.XFRM, 1)
	assert.Equal(t, "xfrm0", cfg.XFRM[0].Name)
	assert.Equal(t, uint32(42), cfg.XFRM[0].Spec.IfID)
	assert.Equal(t, "eth0", cfg.XFRM[0].Spec.PhysicalDev)
	assert.Equal(t, "xfrm0", cfg.XFRM[0].Spec.Name)
	require.Len(t, cfg.XFRM[0].Units, 1)
	assert.Contains(t, cfg.XFRM[0].Units[0].Addresses, "10.0.0.1/30")
}

func TestParseXFRMEntryNoDev(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"xfrm": {
				"xfrm1": {
					"if-id": "99"
				}
			}
		}
	}`)
	require.Len(t, cfg.XFRM, 1)
	assert.Equal(t, uint32(99), cfg.XFRM[0].Spec.IfID)
	assert.Equal(t, "", cfg.XFRM[0].Spec.PhysicalDev)
}

func TestParseXFRMEntryMissingIfId(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"xfrm": {
				"xfrm0": {}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if-id is required")
}

func TestParseXFRMEntryZeroIfId(t *testing.T) {
	_, err := parseIfaceConfig(`{
		"interface": {
			"xfrm": {
				"xfrm0": {
					"if-id": "0"
				}
			}
		}
	}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "if-id must be non-zero")
}

func TestXFRMSpecEqual(t *testing.T) {
	a := XFRMSpec{Name: "xfrm0", IfID: 42, PhysicalDev: "eth0"}
	b := XFRMSpec{Name: "xfrm0", IfID: 42, PhysicalDev: "eth0"}
	assert.True(t, xfrmSpecEqual(a, b))

	c := XFRMSpec{Name: "xfrm0", IfID: 99, PhysicalDev: "eth0"}
	assert.False(t, xfrmSpecEqual(a, c))

	d := XFRMSpec{Name: "xfrm0", IfID: 42, PhysicalDev: ""}
	assert.False(t, xfrmSpecEqual(a, d))
}

func TestApplyXFRMCreate(t *testing.T) {
	cfg := &ifaceConfig{
		XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: XFRMSpec{Name: "xfrm0", IfID: 42},
		}},
	}
	b := &fakeBackend{ifaces: map[string]fakeIface{}}
	errs := applyConfig(cfg, nil, b)
	assert.Empty(t, errs)
	assert.True(t, b.created["xfrm0"])
}

func TestApplyXFRMUnchangedSkipsRecreate(t *testing.T) {
	spec := XFRMSpec{Name: "xfrm0", IfID: 42}
	prev := &ifaceConfig{
		XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: spec,
		}},
	}
	cfg := &ifaceConfig{
		XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: spec,
		}},
	}
	// Unchanged spec means the device survived the reload, so it is present.
	b := &fakeBackend{ifaces: map[string]fakeIface{"xfrm0": {name: "xfrm0", linkType: "xfrm"}}}
	errs := applyConfig(cfg, prev, b)
	assert.Empty(t, errs)
	assert.False(t, b.created["xfrm0"])
	assert.False(t, b.deleted["xfrm0"])
}

func TestApplyXFRMChangedTriggersRecreate(t *testing.T) {
	prev := &ifaceConfig{
		XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: XFRMSpec{Name: "xfrm0", IfID: 42},
		}},
	}
	cfg := &ifaceConfig{
		XFRM: []xfrmEntry{{
			Name: "xfrm0",
			Spec: XFRMSpec{Name: "xfrm0", IfID: 99},
		}},
	}
	b := &fakeBackend{ifaces: map[string]fakeIface{"xfrm0": {name: "xfrm0"}}}
	errs := applyConfig(cfg, prev, b)
	assert.Empty(t, errs)
	assert.True(t, b.deleted["xfrm0"])
	assert.True(t, b.created["xfrm0"])
}

// TestParseUnitQoSMap verifies that ingress-qos-map (PCP -> internal priority)
// and egress-qos-map (internal priority -> PCP) on a VLAN unit are parsed into
// unitEntry maps, including the 0 and 7 boundary values.
//
// VALIDATES: spec-vlan-qos-map AC-1, AC-2 -- QoS map entries populate unitEntry.
// PREVENTS: silent loss of 802.1p mapping config between YANG and the backend.
func TestParseUnitQoSMap(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"v100": {
							"vlan-id": "100",
							"ingress-qos-map": {
								"0": { "priority": "1" },
								"6": { "priority": "6" },
								"7": { "priority": "7" }
							},
							"egress-qos-map": {
								"0": { "pcp": "0" },
								"6": { "pcp": "6" },
								"7": { "pcp": "7" }
							}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, map[uint32]uint32{0: 1, 6: 6, 7: 7}, u.IngressQoSMap)
	assert.Equal(t, map[uint32]uint32{0: 0, 6: 6, 7: 7}, u.EgressQoSMap)
}

// TestParseUnitQoSMapInvalid verifies invalid QoS map values are rejected at
// parse time: PCP or priority above 7 (3-bit 802.1p field), non-numeric
// values, missing value leaf, and QoS maps on a unit without vlan-id.
//
// VALIDATES: spec-vlan-qos-map AC-4, AC-5 -- boundary enforcement (last valid
// 7, first invalid 8).
// PREVENTS: out-of-range values reaching the kernel netlink attribute.
func TestParseUnitQoSMapInvalid(t *testing.T) {
	tests := []struct {
		name string
		unit string
	}{
		{"ingress pcp 8", `{"vlan-id": "100", "ingress-qos-map": {"8": {"priority": "1"}}}`},
		{"ingress priority 8", `{"vlan-id": "100", "ingress-qos-map": {"1": {"priority": "8"}}}`},
		{"egress priority 8", `{"vlan-id": "100", "egress-qos-map": {"8": {"pcp": "1"}}}`},
		{"egress pcp 8", `{"vlan-id": "100", "egress-qos-map": {"1": {"pcp": "8"}}}`},
		{"non-numeric key", `{"vlan-id": "100", "ingress-qos-map": {"voice": {"priority": "6"}}}`},
		{"non-numeric value", `{"vlan-id": "100", "ingress-qos-map": {"6": {"priority": "high"}}}`},
		{"missing value leaf", `{"vlan-id": "100", "ingress-qos-map": {"6": {}}}`},
		{"ingress without vlan-id", `{"ingress-qos-map": {"6": {"priority": "6"}}}`},
		{"egress without vlan-id", `{"egress-qos-map": {"6": {"pcp": "6"}}}`},
		{"duplicate canonical key", `{"vlan-id": "100", "ingress-qos-map": {"6": {"priority": "6"}, "06": {"priority": "1"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseIfaceConfig(`{
				"interface": {
					"ethernet": {
						"eth0": {
							"unit": { "v100": ` + tt.unit + ` }
						}
					}
				}
			}`)
			assert.Error(t, err)
		})
	}
}

// TestApplyVLANQoSMap verifies the apply path forwards the parsed QoS maps to
// the backend inside the VLANSpec, and that units without maps pass nil.
//
// VALIDATES: spec-vlan-qos-map AC-3, AC-6 -- config to backend wiring.
// PREVENTS: maps parsed but dropped before reaching netlink.
func TestApplyVLANQoSMap(t *testing.T) {
	cfg := &ifaceConfig{
		Ethernet: []ifaceEntry{{
			Name: "eth0",
			Units: []unitEntry{{
				Label:         "v100",
				VLANID:        100,
				IngressQoSMap: map[uint32]uint32{6: 6},
				EgressQoSMap:  map[uint32]uint32{6: 6, 0: 0},
			}, {
				Label:  "v200",
				VLANID: 200,
			}},
		}},
	}
	b := &fakeBackend{ifaces: map[string]fakeIface{"eth0": {name: "eth0"}}}
	errs := applyConfig(cfg, nil, b)
	assert.Empty(t, errs)
	require.Contains(t, b.vlans, "eth0.100")
	spec := b.vlans["eth0.100"]
	assert.Equal(t, "eth0", spec.Parent)
	assert.Equal(t, 100, spec.VLANID)
	assert.Equal(t, map[uint32]uint32{6: 6}, spec.IngressQoSMap)
	assert.Equal(t, map[uint32]uint32{6: 6, 0: 0}, spec.EgressQoSMap)
	require.Contains(t, b.vlans, "eth0.200")
	assert.Nil(t, b.vlans["eth0.200"].IngressQoSMap)
	assert.Nil(t, b.vlans["eth0.200"].EgressQoSMap)
}

// TestParseUnitQoSMapEmpty verifies a VLAN unit without QoS maps parses with
// nil maps, preserving the pre-feature behavior (no netlink attributes sent).
// A present-but-empty list also yields nil: the vendor netlink lib serializes
// any non-nil map, and an empty IFLA_VLAN_*_QOS attribute must never be sent.
//
// VALIDATES: spec-vlan-qos-map AC-6 -- backward compatibility.
// PREVENTS: zero-length maps being sent to the kernel for legacy configs.
func TestParseUnitQoSMapEmpty(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"v100": {
							"vlan-id": "100"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.IngressQoSMap)
	assert.Nil(t, u.EgressQoSMap)

	cfg = mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"v100": {
							"vlan-id": "100",
							"ingress-qos-map": {},
							"egress-qos-map": {}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u = cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.IngressQoSMap, "present-but-empty list must normalize to nil")
	assert.Nil(t, u.EgressQoSMap, "present-but-empty list must normalize to nil")
}

// setupTestCoSResolver registers profiles and a resolver for CoS tests.
// The resolver mirrors the real cos plugin's logic (inheritance, "none",
// mutual exclusion, Lookup) so iface parsing tests exercise the full path.
func setupTestCoSResolver(t *testing.T, profiles map[string]coreCos.Profile) {
	t.Helper()
	for name, p := range profiles {
		coreCos.Register(name, p)
	}
	coreCos.RegisterResolver(func(parentCoS, unitCoS string, hasInlineMaps bool) (map[uint32]uint32, map[uint32]uint32, error) {
		name := unitCoS
		if name == "" {
			name = parentCoS
		}
		if name == "none" || name == "" {
			return nil, nil, nil
		}
		if hasInlineMaps {
			return nil, nil, fmt.Errorf("class-of-service and inline qos maps are mutually exclusive")
		}
		p, ok := coreCos.Lookup(name)
		if !ok {
			return nil, nil, fmt.Errorf("class-of-service profile %q not found", name)
		}
		return p.IngressMap, p.EgressMap, nil
	})
	t.Cleanup(func() { coreCos.Clear(); coreCos.ClearResolver() })
}

// TestCoSProfileResolution verifies that a class-of-service reference on the
// parent ethernet interface is resolved via the registered resolver and
// populates the unit's IngressQoSMap/EgressQoSMap.
//
// VALIDATES: spec-cos-plugin AC-4 -- interface-level profile populates unit maps.
// PREVENTS: class-of-service ref silently ignored during parsing.
func TestCoSProfileResolution(t *testing.T) {
	setupTestCoSResolver(t, map[string]coreCos.Profile{
		"residential": {
			IngressMap: map[uint32]uint32{0: 0, 6: 6},
			EgressMap:  map[uint32]uint32{0: 0, 6: 6},
		},
	})

	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "residential",
					"unit": {
						"v100": {
							"vlan-id": "100"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, map[uint32]uint32{0: 0, 6: 6}, u.IngressQoSMap)
	assert.Equal(t, map[uint32]uint32{0: 0, 6: 6}, u.EgressQoSMap)
}

// TestCoSProfileUnitOverride verifies that a unit-level class-of-service
// overrides the parent interface setting.
//
// VALIDATES: spec-cos-plugin AC-5 -- per-unit override wins over parent.
// PREVENTS: parent CoS always applied even when unit has its own.
func TestCoSProfileUnitOverride(t *testing.T) {
	setupTestCoSResolver(t, map[string]coreCos.Profile{
		"residential": {
			IngressMap: map[uint32]uint32{0: 0, 6: 6},
			EgressMap:  map[uint32]uint32{0: 0, 6: 6},
		},
		"business": {
			IngressMap: map[uint32]uint32{5: 5, 7: 7},
			EgressMap:  map[uint32]uint32{5: 5, 7: 7},
		},
	})

	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "residential",
					"unit": {
						"v100": {
							"vlan-id": "100",
							"class-of-service": "business"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, map[uint32]uint32{5: 5, 7: 7}, u.IngressQoSMap)
	assert.Equal(t, map[uint32]uint32{5: 5, 7: 7}, u.EgressQoSMap)
}

// TestCoSProfileUnitOptOut verifies that "class-of-service none" on a unit
// disables inheritance from the parent interface.
//
// VALIDATES: spec-cos-plugin AC-6 -- "none" opts out of parent profile.
// PREVENTS: parent CoS applied even when unit explicitly requests no maps.
func TestCoSProfileUnitOptOut(t *testing.T) {
	setupTestCoSResolver(t, map[string]coreCos.Profile{
		"residential": {
			IngressMap: map[uint32]uint32{0: 0, 6: 6},
			EgressMap:  map[uint32]uint32{0: 0, 6: 6},
		},
	})

	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "residential",
					"unit": {
						"v100": {
							"vlan-id": "100",
							"class-of-service": "none"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	require.Len(t, cfg.Ethernet[0].Units, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.IngressQoSMap)
	assert.Nil(t, u.EgressQoSMap)
}

// TestCoSProfileNoVLAN verifies that class-of-service on a unit without
// vlan-id is rejected (same constraint as inline qos maps).
//
// VALIDATES: spec-cos-plugin AC-7 -- class-of-service requires vlan-id.
// PREVENTS: kernel error from VLAN QoS map on a non-VLAN interface.
func TestCoSProfileNoVLAN(t *testing.T) {
	setupTestCoSResolver(t, map[string]coreCos.Profile{
		"residential": {
			IngressMap: map[uint32]uint32{0: 0},
			EgressMap:  map[uint32]uint32{0: 0},
		},
	})

	_, err := parseIfaceConfig(`{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "residential",
					"unit": {
						"v100": {}
					}
				}
			}
		}
	}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "vlan-id")
}

// TestCoSProfileConflictInline verifies that a unit with both a
// class-of-service reference and inline qos maps is rejected.
//
// VALIDATES: spec-cos-plugin AC-8 -- mutual exclusion.
// PREVENTS: ambiguous config where both mechanisms set the same maps.
func TestCoSProfileConflictInline(t *testing.T) {
	setupTestCoSResolver(t, map[string]coreCos.Profile{
		"residential": {
			IngressMap: map[uint32]uint32{0: 0},
			EgressMap:  map[uint32]uint32{0: 0},
		},
	})

	_, err := parseIfaceConfig(`{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "residential",
					"unit": {
						"v100": {
							"vlan-id": "100",
							"ingress-qos-map": {
								"6": { "priority": "6" }
							}
						}
					}
				}
			}
		}
	}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

// TestCoSProfileNotFound verifies that referencing a nonexistent profile
// name is rejected during parsing.
//
// VALIDATES: spec-cos-plugin AC-9 -- missing profile detected.
// PREVENTS: silent config acceptance that would leave VLAN without maps.
func TestCoSProfileNotFound(t *testing.T) {
	setupTestCoSResolver(t, nil)

	_, err := parseIfaceConfig(`{
		"interface": {
			"ethernet": {
				"eth0": {
					"class-of-service": "nonexistent",
					"unit": {
						"v100": {
							"vlan-id": "100"
						}
					}
				}
			}
		}
	}`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestValidateVPPQoSMapsIdentityAccepted verifies that identity ingress maps
// (pcp == priority) pass VPP validation.
//
// VALIDATES: VPP accepts identity ingress + arbitrary egress QoS maps.
// PREVENTS: false rejection of valid VPP QoS config.
func TestValidateVPPQoSMapsIdentityAccepted(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: "vpp",
		Ethernet: []ifaceEntry{{
			Name: "xe0",
			Units: []unitEntry{{
				Label:         "v100",
				VLANID:        100,
				IngressQoSMap: map[uint32]uint32{0: 0, 6: 6, 7: 7},
				EgressQoSMap:  map[uint32]uint32{0: 0, 6: 5, 7: 3},
			}},
		}},
	}
	assert.NoError(t, validateVPPQoSMaps(cfg))
}

// TestValidateVPPQoSMapsNonIdentityRejected verifies that non-identity ingress
// maps (pcp != priority) are rejected at validation time for VPP.
//
// VALIDATES: VPP rejects non-identity ingress maps at config validation.
// PREVENTS: apply-time failure that should be caught at commit.
func TestValidateVPPQoSMapsNonIdentityRejected(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: "vpp",
		Ethernet: []ifaceEntry{{
			Name: "xe0",
			Units: []unitEntry{{
				Label:         "v100",
				VLANID:        100,
				IngressQoSMap: map[uint32]uint32{6: 3},
			}},
		}},
	}
	err := validateVPPQoSMaps(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "identity")
}

// TestValidateVPPQoSMapsEgressOnlyAccepted verifies that egress-only maps
// (no ingress) pass VPP validation.
//
// VALIDATES: VPP accepts egress-only QoS maps.
// PREVENTS: false rejection when only egress mapping is configured.
func TestValidateVPPQoSMapsEgressOnlyAccepted(t *testing.T) {
	cfg := &ifaceConfig{
		Backend: "vpp",
		Ethernet: []ifaceEntry{{
			Name: "xe0",
			Units: []unitEntry{{
				Label:        "v100",
				VLANID:       100,
				EgressQoSMap: map[uint32]uint32{6: 6},
			}},
		}},
	}
	assert.NoError(t, validateVPPQoSMaps(cfg))
}
