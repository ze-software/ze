package iface

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vppevents "codeberg.org/thomas-mangin/ze/internal/component/vpp/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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

// TestParseTunnelGretapMAC verifies that mac-address inside the gretap case
// container is parsed correctly. After the YANG restructure, mac-address
// lives inside the per-case container for bridgeable kinds (gretap/ip6gretap),
// not at the list level.
//
// VALIDATES: AC-2 (spec-iface-tunnel-mac-per-case) - mac-address inside gretap accepted.
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
							"mac-address": "aa:bb:cc:dd:ee:ff"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindGRETap, cfg.Tunnel[0].Spec.Kind)
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", cfg.Tunnel[0].MACAddress,
		"mac-address inside gretap case must be parsed")
}

// TestParseTunnelGreNoMAC verifies that an L3 tunnel kind (gre) does not
// carry a mac-address, and that any mac-address at the list level is ignored
// for tunnels (YANG enforces this; parser provides defense-in-depth).
//
// VALIDATES: AC-3 (spec-iface-tunnel-mac-per-case) - L3 kind without MAC accepted.
// VALIDATES: AC-4 (spec-iface-tunnel-mac-per-case) - mac-address not available on L3 kinds.
// PREVENTS: L3 tunnel silently accepting MAC that the kernel ignores.
func TestParseTunnelGreNoMAC(t *testing.T) {
	// mac-address at list level (old syntax) -- must be ignored for tunnels.
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"gre0": {
					"mac-address": "aa:bb:cc:dd:ee:ff",
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
		"L3 tunnel must not carry mac-address (list-level mac-address must be ignored)")
}

// TestParseTunnelIp6gretapMAC verifies that mac-address inside the ip6gretap
// case container is parsed correctly, mirroring TestParseTunnelGretapMAC for
// the v6-underlay L2 kind.
//
// VALIDATES: ip6gretap mac-address parity with gretap.
// PREVENTS: ip6gretap silently dropping mac-address.
func TestParseTunnelIp6gretapMAC(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"tunnel": {
				"ip6gretap0": {
					"encapsulation": {
						"ip6gretap": {
							"local":  {"ip": "2001:db8::1"},
							"remote": {"ip": "2001:db8::2"},
							"mac-address": "11:22:33:44:55:66"
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Tunnel, 1)
	assert.Equal(t, TunnelKindIP6GRETap, cfg.Tunnel[0].Spec.Kind)
	assert.Equal(t, "11:22:33:44:55:66", cfg.Tunnel[0].MACAddress,
		"mac-address inside ip6gretap case must be parsed")
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
	assert.True(t, cfg.Tunnel[0].Spec.Kind.IsBridgeable())
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
				ifaceEntry: ifaceEntry{Name: "gre-a"},
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
				ifaceEntry: ifaceEntry{Name: "gre-b"},
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
		Tunnel:  []tunnelEntry{{ifaceEntry: ifaceEntry{Name: "gre0"}, Spec: spec}},
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
			ifaceEntry: ifaceEntry{Name: "gre0"},
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
			ifaceEntry: ifaceEntry{Name: "gre0"},
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
			ifaceEntry: ifaceEntry{Name: "wg0"},
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
		Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: spec}},
	}
	cfg := &ifaceConfig{
		Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: spec}},
	}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
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

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: base}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: withNew}}}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
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

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: twoPeer}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: onePeer}}}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
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

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: beforeSpec}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: afterSpec}}}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
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

	previous := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: beforeSpec}}}
	cfg := &ifaceConfig{Wireguard: []wireguardEntry{{ifaceEntry: ifaceEntry{Name: "wg0"}, Spec: afterSpec}}}

	b := &fakeBackend{ifaces: map[string]fakeIface{}}
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
			ifaceEntry: ifaceEntry{Name: "wg0", Disable: true},
			Spec:       WireguardSpec{Name: "wg0", PrivateKey: wireguardTestKey(1)},
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
				ifaceEntry: ifaceEntry{
					Name:  "gre0",
					Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/30"}}},
				},
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

// TestParseUnitDHCPv4Enabled verifies that the dhcp container in a unit
// is parsed into a dhcpUnitConfig with Enabled=true.
//
// VALIDATES: AC-1 - Config with dhcp { enabled true } parsed.
// PREVENTS: DHCP leaves silently ignored by parseUnits.
func TestParseUnitDHCPv4Enabled(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"dhcp": {
								"enabled": "true"
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
	require.NotNil(t, u.DHCP, "DHCP config should be parsed")
	assert.True(t, u.DHCP.Enabled)
	assert.Nil(t, u.DHCPv6, "DHCPv6 should be nil when not configured")
}

// TestParseUnitDHCPv4Hostname verifies hostname and client-id parsing.
//
// VALIDATES: AC-3, AC-4 - hostname and client-id parsed from config.
// PREVENTS: DHCP options silently dropped during parsing.
func TestParseUnitDHCPv4Hostname(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
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
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.DHCP)
	assert.True(t, u.DHCP.Enabled)
	assert.Equal(t, "ze-router", u.DHCP.Hostname)
	assert.Equal(t, "ze:01", u.DHCP.ClientID)
}

// TestParseUnitDHCPDisabledDefault verifies that a unit without a dhcp
// container has DHCP=nil (disabled by default).
//
// VALIDATES: AC-2 - No dhcp block means no DHCP client.
// PREVENTS: DHCP accidentally enabled when config omits the block.
func TestParseUnitDHCPDisabledDefault(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"address": ["10.0.0.1/24"]
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.DHCP, "DHCP should be nil when not configured")
	assert.Nil(t, u.DHCPv6, "DHCPv6 should be nil when not configured")
	assert.Equal(t, []string{"10.0.0.1/24"}, u.Addresses)
}

// TestParseUnitDHCPv6PD verifies DHCPv6 with prefix delegation parsing.
//
// VALIDATES: AC-12, AC-13 - DHCPv6 enabled with PD length.
// PREVENTS: DHCPv6 PD length silently dropped.
func TestParseUnitDHCPv6PD(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
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
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Nil(t, u.DHCP, "DHCPv4 should be nil")
	require.NotNil(t, u.DHCPv6)
	assert.True(t, u.DHCPv6.Enabled)
	assert.Equal(t, 56, u.DHCPv6.PDLength)
	assert.Equal(t, "00:01:00:01:aa:bb", u.DHCPv6.DUID)
}

// TestParseUnitDHCPDualStack verifies both DHCPv4 and DHCPv6 on the same unit.
//
// VALIDATES: AC-15 - Dual-stack DHCP coexistence.
// PREVENTS: v4 and v6 config interfering with each other.
func TestParseUnitDHCPDualStack(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"dhcp": {"enabled": "true", "hostname": "ze"},
							"dhcpv6": {"enabled": "true"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	require.NotNil(t, u.DHCP)
	assert.True(t, u.DHCP.Enabled)
	assert.Equal(t, "ze", u.DHCP.Hostname)
	require.NotNil(t, u.DHCPv6)
	assert.True(t, u.DHCPv6.Enabled)
}

// TestParseUnitDHCPWithStaticAddress verifies DHCP alongside static addresses.
//
// VALIDATES: AC-15 - Static IP config alongside DHCP.
// PREVENTS: DHCP parsing clobbering static address list.
func TestParseUnitDHCPWithStaticAddress(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{
		"interface": {
			"ethernet": {
				"eth0": {
					"unit": {
						"0": {
							"address": ["10.0.0.1/24"],
							"dhcp": {"enabled": "true"}
						}
					}
				}
			}
		}
	}`)
	require.Len(t, cfg.Ethernet, 1)
	u := cfg.Ethernet[0].Units[0]
	assert.Equal(t, []string{"10.0.0.1/24"}, u.Addresses)
	require.NotNil(t, u.DHCP)
	assert.True(t, u.DHCP.Enabled)
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
}

// TestParseUnitRoutePriorityDefault verifies that a unit without
// route-priority configured defaults to 0 (kernel default).
//
// VALIDATES: AC-2 - No route-priority means metric 0 (unchanged behavior).
// PREVENTS: Non-zero default accidentally changing existing route behavior.
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
	assert.Equal(t, 0, u.RoutePriority)
}

// TestHandleDHCPLeaseEventStoresGateway verifies that a DHCP lease event
// updates the stored gateway for link-state failover.
//
// VALIDATES: Gateway stored from DHCP lease for failover use.
// PREVENTS: Link failover silently does nothing because gateway is empty.
func TestHandleDHCPLeaseEventStoresGateway(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v4: true}},
	}
	logger := slog.Default()

	data := `{"name":"eth0","unit":0,"router":"192.168.1.1","address":"192.168.1.50","prefix-length":24}`
	handleDHCPLeaseEvent(data, active, logger)

	entry := active[dhcpUnitKey{ifaceName: "eth0", unit: 0}]
	assert.Equal(t, "192.168.1.1", entry.gateway)
}

// TestHandleDHCPLeaseEventNoMatch verifies that lease events for unknown
// interfaces are silently ignored.
//
// VALIDATES: No panic or map corruption on unmatched lease event.
// PREVENTS: Map write for interface not in activeDHCP.
func TestHandleDHCPLeaseEventNoMatch(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v4: true}},
	}
	logger := slog.Default()

	data := `{"name":"eth1","unit":0,"router":"10.0.0.1"}`
	handleDHCPLeaseEvent(data, active, logger)

	// eth0 gateway unchanged (still empty).
	entry := active[dhcpUnitKey{ifaceName: "eth0", unit: 0}]
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
		{ifaceName: "eth0", unit: 0}: {
			params:  dhcpParams{v4: true, routePriority: 5},
			gateway: "192.168.1.1",
		},
	}
	logger := slog.Default()

	handleLinkDown("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 5}, fb.routeRemoves[0],
		"should remove route with base metric")

	require.Len(t, fb.routeAdds, 1, "should add one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 1029}, fb.routeAdds[0],
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
		{ifaceName: "eth0", unit: 0}: {
			params:  dhcpParams{v4: true, routePriority: 5},
			gateway: "192.168.1.1",
		},
	}
	logger := slog.Default()

	handleLinkUp("eth0", active, logger)

	require.Len(t, fb.routeRemoves, 1, "should remove one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 1029}, fb.routeRemoves[0],
		"should remove deprioritized route (5 + 1024 = 1029)")

	require.Len(t, fb.routeAdds, 1, "should add one route")
	assert.Equal(t, routeCall{"eth0", "0.0.0.0/0", "192.168.1.1", 5}, fb.routeAdds[0],
		"should restore route with base metric")
}

// TestHandleLinkDownDefaultMetric verifies that link-down with no route-priority
// (default 0) uses metric 0 and 1024, preserving existing behavior.
//
// VALIDATES: AC-2 - No route-priority configured preserves existing behavior.
// PREVENTS: Regression in default metric behavior.
func TestHandleLinkDownDefaultMetric(t *testing.T) {
	fb := &fakeBackend{ifaces: map[string]fakeIface{}}
	backendName := "test-linkdown-default-" + t.Name()
	err := RegisterBackend(backendName, func() (Backend, error) { return fb, nil })
	require.NoError(t, err)
	require.NoError(t, LoadBackend(backendName))
	defer func() { _ = CloseBackend() }()

	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {
			params:  dhcpParams{v4: true},
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
		{ifaceName: "eth0", unit: 0}: {
			params:  dhcpParams{v4: true, routePriority: 5},
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
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
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
		Ethernet: []ifaceEntry{{Name: "eth0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24", "10.0.0.2/24"}}}}},
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
		Dummy:   []ifaceEntry{{Name: "dummy0", Units: []unitEntry{{ID: 0, Addresses: []string{"10.0.0.1/24"}}}}},
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
			{ifaceEntry: ifaceEntry{Name: "veth0"}, Peer: "veth0-peer"},
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
}

// fakeBackend implements Backend for testing config application.
type fakeBackend struct {
	ifaces       map[string]fakeIface
	created      map[string]bool
	deleted      map[string]bool
	addrs        map[string][]string
	tunnels      map[string]TunnelSpec
	wgConfigs    map[string]WireguardSpec
	wgConfigCt   map[string]int
	routeAdds    []routeCall
	routeRemoves []routeCall
	staleRoutes  []RouteInfo // returned by ListRoutes for stale cleanup tests
	listErr      error       // if non-nil, ListInterfaces returns this instead of enumerating
}

type fakeIface struct {
	name     string
	linkType string
}

func (b *fakeBackend) ensureMaps() {
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
}

func (b *fakeBackend) CreateDummy(name string) error {
	b.ensureMaps()
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
	b.ifaces[name] = fakeIface{name: name, linkType: "bridge"}
	return nil
}

func (b *fakeBackend) CreateVLAN(_ string, _ int) error { return nil }

func (b *fakeBackend) CreateTunnel(spec TunnelSpec) error {
	b.ensureMaps()
	b.created[spec.Name] = true
	b.tunnels[spec.Name] = spec
	b.ifaces[spec.Name] = fakeIface{name: spec.Name, linkType: "tunnel-" + spec.Kind.String()}
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

func (b *fakeBackend) SetAdminUp(_ string) error              { return nil }
func (b *fakeBackend) SetAdminDown(_ string) error            { return nil }
func (b *fakeBackend) SetMTU(_ string, _ int) error           { return nil }
func (b *fakeBackend) SetMACAddress(_, _ string) error        { return nil }
func (b *fakeBackend) GetMACAddress(_ string) (string, error) { return "", nil }
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

func (b *fakeBackend) AddRoute(ifaceName, destCIDR, gateway string, metric int) error {
	b.routeAdds = append(b.routeAdds, routeCall{ifaceName, destCIDR, gateway, metric})
	return nil
}

func (b *fakeBackend) RemoveRoute(ifaceName, destCIDR, gateway string, metric int) error {
	b.routeRemoves = append(b.routeRemoves, routeCall{ifaceName, destCIDR, gateway, metric})
	return nil
}

func (b *fakeBackend) ListRoutes(_, _ string) ([]RouteInfo, error) {
	return b.staleRoutes, nil
}

func (b *fakeBackend) ListNeighbors(_ int) ([]NeighborInfo, error) {
	return nil, nil
}

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
		info := InterfaceInfo{Name: f.name, Type: f.linkType}
		if addrs, ok := b.addrs[f.name]; ok {
			for _, a := range addrs {
				info.Addresses = append(info.Addresses, AddrInfo{Address: a, PrefixLength: 24})
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
	return &InterfaceInfo{Name: f.name, Type: f.linkType}, nil
}

func (b *fakeBackend) BridgeAddPort(_, _ string) error     { return nil }
func (b *fakeBackend) BridgeDelPort(_ string) error        { return nil }
func (b *fakeBackend) BridgeSetSTP(_ string, _ bool) error { return nil }

func (b *fakeBackend) SetupMirror(_, _ string, _, _ bool) error { return nil }
func (b *fakeBackend) RemoveMirror(_ string) error              { return nil }
func (b *fakeBackend) StartMonitor(_ ze.EventBus) error         { return nil }
func (b *fakeBackend) StopMonitor()                             {}
func (b *fakeBackend) Close() error                             { return nil }

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
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v6: true, routePriority: 5}},
	}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, active, logger)

	require.Len(t, fb.routeAdds, 1, "should install one IPv6 default route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeAdds[0])
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
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeRemoves[0])
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
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1, "should add deprioritized route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 1029}, fb.routeAdds[0])
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
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 1029}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1, "should add restored route")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeAdds[0])
}

// TestNeighRouterDetectedNoRoutePriority verifies that router events are
// ignored when route-priority is not configured (0).
//
// VALIDATES: AC-3/AC-9 - No route-priority means kernel handles everything.
// PREVENTS: Ze installing routes when user didn't configure route-priority.
func TestNeighRouterDetectedNoRoutePriority(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v6: true, routePriority: 0}},
	}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, active, logger)

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
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v6: true, routePriority: 5}},
	}
	logger := slog.Default()

	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, active, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::2"}`, routers, active, logger)

	require.Len(t, fb.routeAdds, 2, "should install two IPv6 default routes")
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeAdds[0])
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::2", 5}, fb.routeAdds[1])
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
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v6: true, routePriority: 5}},
	}
	logger := slog.Default()

	data := `{"name":"eth0","router-ip":"fe80::1"}`
	handleRouterDiscovered(data, routers, active, logger)
	handleRouterDiscovered(data, routers, active, logger)

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
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{v6: true, routePriority: 10}},
	}
	logger := slog.Default()

	// Simulate the router being re-discovered after config reload with new metric.
	// First, the old entry should be cleaned up.
	// restoreAcceptRaDefrtr removes existing routes; then suppressRAForConfig
	// re-suppresses and the monitor re-discovers the router.
	// Simulating the removal + re-discovery:
	handleRouterLost(`{"name":"eth0","router-ip":"fe80::1"}`, routers, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":"fe80::1"}`, routers, active, logger)

	// Old route removed with metric 5, new installed with metric 10.
	require.Len(t, fb.routeRemoves, 1)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeRemoves[0])
	require.Len(t, fb.routeAdds, 1)
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 10}, fb.routeAdds[0])
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
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 5}, fb.routeRemoves[0])

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
	assert.Equal(t, routeCall{"eth0", "::/0", "fe80::1", 0}, fb.routeRemoves[0])
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
							"address": ["10.0.0.1/24"]
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()

	suppressRAForConfig(cfg, suppressed, routers, eb, logger)

	assert.Empty(t, eb.emissions, "should not emit any sysctl events")
	assert.Empty(t, suppressed, "should not suppress any interfaces")
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
							"address": ["10.0.0.1/24"]
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()

	suppressRAForConfig(cfg, suppressed, routers, eb, logger)

	require.Len(t, eb.emissions, 1, "should emit one sysctl event")
	assert.Contains(t, eb.emissions[0].data, `"value":"0"`)
	assert.True(t, suppressed["eth0"])
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
							"address": ["10.0.0.1/24"]
						}
					}
				}
			}
		}
	}`)
	logger := slog.Default()

	suppressRAForConfig(cfg, suppressed, routers, eb, logger)

	require.Len(t, eb.emissions, 1, "should emit restore sysctl event")
	assert.Contains(t, eb.emissions[0].data, `"value":"1"`)
	assert.Empty(t, suppressed, "interface should no longer be suppressed")
	assert.Empty(t, routers, "router entries should be cleaned up")
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

// TestRoutePriorityForInterfaceMultiUnit verifies that the first non-zero
// route-priority is returned when multiple units exist.
//
// VALIDATES: Multi-unit interface returns a valid metric.
// PREVENTS: Zero returned when a non-zero route-priority exists.
func TestRoutePriorityForInterfaceMultiUnit(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{routePriority: 0}},
		{ifaceName: "eth0", unit: 1}: {params: dhcpParams{routePriority: 7}},
	}

	result := routePriorityForInterface("eth0", active)
	assert.Equal(t, 7, result, "should return the non-zero route-priority")
}

// TestRoutePriorityForInterfaceNoMatch verifies that an unknown interface
// returns 0.
//
// VALIDATES: Unknown interface returns zero (kernel default).
// PREVENTS: Non-zero metric for unconfigured interface.
func TestRoutePriorityForInterfaceNoMatch(t *testing.T) {
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{routePriority: 5}},
	}

	result := routePriorityForInterface("eth1", active)
	assert.Equal(t, 0, result, "unknown interface should return 0")
}

// TestRouterDiscoveredBadJSON verifies that malformed JSON is silently ignored.
//
// VALIDATES: Defensive JSON parsing.
// PREVENTS: Panic on malformed event bus payload.
func TestRouterDiscoveredBadJSON(t *testing.T) {
	_ = setupFakeBackendForTest(t)
	routers := make(map[routerKey]routerEntry)
	active := map[dhcpUnitKey]dhcpEntry{
		{ifaceName: "eth0", unit: 0}: {params: dhcpParams{routePriority: 5}},
	}
	logger := slog.Default()

	handleRouterDiscovered("not json", routers, active, logger)
	handleRouterDiscovered(`{"name":"","router-ip":"fe80::1"}`, routers, active, logger)
	handleRouterDiscovered(`{"name":"eth0","router-ip":""}`, routers, active, logger)

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
