// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 4724 Section 4.1 per-family route selection deferral
// Related: selection_deferral.go -- selectionDeferral, reselectDeferredFamily, handleSelectionDeferralText, parseSelectionDeferralConfig
// Related: config_tx.go -- parseRIBConfigSections, installConfig
// RFC 4724 Section 4.1 -- see rfc/short/rfc4724.md

package rib

import (
	"encoding/json"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/test/sim"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type rfc4724TestBus struct {
	mu       sync.Mutex
	nextID   uint64
	handlers map[string]map[uint64]func(any)
}

func newRFC4724TestBus() *rfc4724TestBus {
	return &rfc4724TestBus{handlers: make(map[string]map[uint64]func(any))}
}

func (b *rfc4724TestBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.mu.Lock()
	subs := make([]func(any), 0, len(b.handlers[namespace+"/"+eventType]))
	for _, fn := range b.handlers[namespace+"/"+eventType] {
		subs = append(subs, fn)
	}
	b.mu.Unlock()
	for _, fn := range subs {
		fn(payload)
	}
	return len(subs), nil
}

func (b *rfc4724TestBus) Subscribe(namespace, eventType string, handler func(any)) func() {
	key := namespace + "/" + eventType
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	if b.handlers[key] == nil {
		b.handlers[key] = make(map[uint64]func(any))
	}
	b.handlers[key][id] = handler
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		delete(b.handlers[key], id)
		b.mu.Unlock()
	}
}

// TestRFC4724RIBSelectionDeferralPerFamilyAndTimer proves RFC 4724 Section 4.1
// per-family Loc-RIB route selection deferral, multi-peer convergence, peer
// exclusions (non-GR and Restart-State R-bit), route-less family release, and
// the configurable Selection_Deferral_Timer upper bound using real Ze BGP
// config JSON (named standalone peers, grouped peers, dynamic group exclusion,
// and string-serialized YANG leaves).
//
// VALIDATES:
//   - RFC4724-4.1-5: bgp-rib defers best-path route selection per address family
//     until ALL non-excluded GR peers (2 static GR peers: one standalone, one
//     grouped; excluding R=1, non-GR, and dynamic-group templates) send
//     End-of-RIB for that family, OR Selection_Deferral_Timer expires.
//     IPv4 stays deferred after the first GR peer's EoR and releases only after
//     the second GR peer's EoR; a route-less converged family (IPv4 multicast)
//     also marks released while IPv6 stays deferred; duplicate/unnegotiated EoR
//     markers never release early.
//   - RFC4724-4.1-8: configuring `bgp update-delay max-delay` as Ze's real YANG
//     string leaf ("45" vs "90") bounds route selection deferral to the exact
//     configured duration and emits SelectionComplete upon completion; "0"
//     disables deferral so routes select into Loc-RIB immediately.
//
// PREVENTS: selecting routes into Loc-RIB before all non-excluded GR peers send
// End-of-RIB, dropping real Ze config peers, deferring a route-less converged
// family forever, or lacking a configurable Selection_Deferral_Timer.
//
// RFC requirement: RFC4724-4.1-5 positive -- bgp-rib defers Loc-RIB route selection per address family until ALL expected non-excluded GR peers (standalone and grouped, excluding non-GR, R-bit, and dynamic peers) send End-of-RIB for that family, then selects and installs the best path.
// RFC requirement: RFC4724-4.1-5 negative -- partial End-of-RIB (1 of 2 GR peers), duplicate EoR, or unnegotiated-family EoR markers do not release deferred Loc-RIB route selection prematurely.
// RFC requirement: RFC4724-4.1-8 positive -- configured bgp update-delay max-delay string leaf ("45" and "90") bounds route selection deferral to each exact duration, selects deferred routes into Loc-RIB on expiry, and emits SelectionComplete.
// RFC requirement: RFC4724-4.1-8 negative -- max-delay "0" disables selection deferral so received routes are selected into Loc-RIB immediately.
func TestRFC4724RIBSelectionDeferralPerFamilyAndTimer(t *testing.T) {
	p1 := netip.MustParseAddr("192.0.2.1") // Standalone GR peer 1 (R=0, IPv4 + IPv6 + IPv4Multicast)
	p4 := netip.MustParseAddr("192.0.2.4") // Grouped GR peer 2 (R=0, IPv4 + IPv6 + IPv4Multicast)
	p2 := netip.MustParseAddr("192.0.2.2") // Restarting peer (R=1, excluded)
	p3 := netip.MustParseAddr("192.0.2.3") // Non-GR peer (excluded)
	v4Prefix := netip.MustParsePrefix("198.51.100.0/24")
	v6Prefix := netip.MustParsePrefix("2001:db8:1::/48")

	seedRoute := func(t *testing.T, r *RIBManager, peerOwner uint64, peerAddr netip.Addr, fam family.Family, wireNLRI, attrs []byte) {
		t.Helper()
		r.peerMu.Lock()
		pr := r.bgpPeers[peerOwner]
		if pr == nil {
			var err error
			pr, err = storage.NewAddressPeerRIB(peerAddr)
			if err != nil {
				r.peerMu.Unlock()
				t.Fatalf("NewAddressPeerRIB: %v", err)
			}
			r.bgpPeers[peerOwner] = pr
		}
		r.peerMeta[peerOwner] = &peerMetadata{
			Address:  peerAddr,
			PeerASN:  65001,
			LocalASN: 65000,
		}
		pr.Insert(fam, attrs, wireNLRI)
		r.peerMu.Unlock()
	}

	v4Attrs := []byte{
		0x40, 1, 1, 0, // ORIGIN = IGP
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, // AS_PATH = [65001] (4-byte)
		0x40, 3, 4, 192, 0, 2, 1, // NEXT_HOP = 192.0.2.1
	}
	v4NLRI := []byte{24, 198, 51, 100} // 198.51.100.0/24
	v6NH := netip.MustParseAddr("2001:db8::1").AsSlice()
	v6Attrs := append([]byte{
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
		0x80, 14, 21, 0, 2, 1, 16,
	}, v6NH...)
	v6Attrs = append(v6Attrs, 0) // reserved byte in MP_REACH
	v6NLRI := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}

	// Build REAL Ze BGP config JSON matching Tree.ToMap() (string YANG leaves,
	// named standalone peers under bgp.peer.<name>.connection.remote.ip, grouped
	// peers under bgp.group.<g>.peer.<name>.connection.remote.ip, and a dynamic
	// group template whose "dynamic" IP must not be counted as an expected peer).
	newDeferredRIB := func(t *testing.T, maxDelayStr string) (*RIBManager, *locrib.RIB, *sim.FakeClock, *atomic.Int32) {
		t.Helper()
		bus := newRFC4724TestBus()
		prevBus := getEventBus()
		SetEventBus(bus)
		var completeCount atomic.Int32
		unsubComplete := ribevents.SelectionComplete.Subscribe(bus, func(_ *ribevents.SelectionCompleteEvent) {
			completeCount.Add(1)
		})
		t.Cleanup(func() {
			unsubComplete()
			if prevBus != nil {
				SetEventBus(prevBus)
			}
		})

		clk := sim.NewFakeClock(time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC))
		r := newTestRIBManager(t)
		r.deferClk = clk
		loc := electionFixtureLocRIB(t, r, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1"))
		t.Cleanup(func() {
			r.selectionDefer.stop()
		})

		cfgMap := map[string]any{
			"bgp": map[string]any{
				"update-delay": map[string]any{
					"max-delay": maxDelayStr,
				},
				"peer": map[string]any{
					"edge-gr-1": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": p1.String()}},
					},
					"edge-restarting": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": p2.String()}},
					},
					"edge-nongr": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": p3.String()}},
					},
				},
				"group": map[string]any{
					"transit": map[string]any{
						"peer": map[string]any{
							"edge-gr-2": map[string]any{
								"connection": map[string]any{"remote": map[string]any{"ip": p4.String()}},
							},
						},
					},
					"dyn-clients": map[string]any{
						"connection": map[string]any{"remote": map[string]any{"ip": "dynamic"}},
					},
				},
			},
		}
		rawCfg, err := json.Marshal(cfgMap)
		if err != nil {
			t.Fatalf("marshal bgp config: %v", err)
		}
		parsed, _, err := parseRIBConfig([]sdk.ConfigSection{{Root: configRootBGP, Data: string(rawCfg)}})
		if err != nil {
			t.Fatalf("parseRIBConfig: %v", err)
		}
		if err := r.installConfig(parsed, false); err != nil {
			t.Fatalf("installConfig: %v", err)
		}
		return r, loc, clk, &completeCount
	}

	t.Run("multi-peer per-family deferral releases IPv4 after all GR peers send EoR and IPv6 on 45s timer", func(t *testing.T) {
		r, loc, clk, completeCount := newDeferredRIB(t, "45")

		// Deliver negotiated events for all 4 static peers:
		// p1 & p4 are GR (R=0, IPv4+IPv6+IPv4Multicast), p2 is GR (R=1, excluded), p3 is non-GR (excluded).
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},"peer":{"remote":{"address":"192.0.2.1"}},"negotiated":{"families":["ipv4/unicast","ipv6/unicast","ipv4/multicast"],"graceful-restart":{"restart-state":false,"restart-time":120}}}}`)
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},"peer":{"remote":{"address":"192.0.2.4"}},"negotiated":{"families":["ipv4/unicast","ipv6/unicast","ipv4/multicast"],"graceful-restart":{"restart-state":false,"restart-time":120}}}}`)
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},"peer":{"remote":{"address":"192.0.2.2"}},"negotiated":{"families":["ipv4/unicast","ipv6/unicast"],"graceful-restart":{"restart-state":true,"restart-time":120}}}}`)
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"negotiated"},"peer":{"remote":{"address":"192.0.2.3"}},"negotiated":{"families":["ipv4/unicast","ipv6/unicast"]}}}`)

		// Store IPv4 and IPv6 routes in Adj-RIB-In and drive reconcileReceived;
		// both families must stay deferred before EoR (nothing installed in Loc-RIB).
		seedRoute(t, r, 1, p1, family.IPv4Unicast, v4NLRI, v4Attrs)
		seedRoute(t, r, 1, p1, family.IPv6Unicast, v6NLRI, v6Attrs)
		r.reconcileReceived(&Event{
			RawNLRIBytes: map[family.Family][]byte{
				family.IPv4Unicast: v4NLRI,
				family.IPv6Unicast: v6NLRI,
			},
		})
		if _, found := loc.Best(family.IPv4Unicast, v4Prefix); found {
			t.Fatal("IPv4 route installed in Loc-RIB while deferred")
		}
		if _, found := loc.Best(family.IPv6Unicast, v6Prefix); found {
			t.Fatal("IPv6 route installed in Loc-RIB while deferred")
		}

		// Negative: unnegotiated family EoR must not release anything.
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.1"}},"eor":{"family":"ipv6/multicast"}}}`)

		// Negative: EoR from ONLY 1 of 2 non-excluded GR peers (p1) must keep IPv4 deferred.
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.1"}},"eor":{"family":"ipv4/unicast"}}}`)
		if !r.selectionDefer.isDeferred(family.IPv4Unicast) {
			t.Fatal("IPv4Unicast released after only 1 of 2 GR peers sent EoR")
		}
		if _, found := loc.Best(family.IPv4Unicast, v4Prefix); found {
			t.Fatal("IPv4 route installed in Loc-RIB before 2nd GR peer sent EoR")
		}

		// Negative: duplicate IPv4 EoR from p1 must still keep IPv4 deferred.
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.1"}},"eor":{"family":"ipv4/unicast"}}}`)
		if !r.selectionDefer.isDeferred(family.IPv4Unicast) {
			t.Fatal("duplicate IPv4 EoR from p1 prematurely released IPv4Unicast")
		}

		// Positive: IPv4 EoR from 2nd GR peer (p4) releases IPv4Unicast immediately
		// (without waiting for R=1 p2 or non-GR p3) while IPv6Unicast stays deferred.
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.4"}},"eor":{"family":"ipv4/unicast"}}}`)
		if r.selectionDefer.isDeferred(family.IPv4Unicast) {
			t.Fatal("IPv4Unicast stayed deferred after ALL non-excluded GR peers sent EoR")
		}
		if _, found := loc.Best(family.IPv4Unicast, v4Prefix); !found {
			t.Fatal("IPv4 route was not selected into Loc-RIB after all GR peers sent IPv4 EoR")
		}

		// Route-less family convergence: both p1 and p4 send IPv4Multicast EoR with
		// zero multicast routes received yet; IPv4Multicast must be marked released
		// immediately even while IPv6Unicast is still pending.
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.1"}},"eor":{"family":"ipv4/multicast"}}}`)
		r.dispatchText(`{"type":"bgp","bgp":{"message":{"type":"eor"},"peer":{"remote":{"address":"192.0.2.4"}},"eor":{"family":"ipv4/multicast"}}}`)
		if r.selectionDefer.isDeferred(family.IPv4Multicast) {
			t.Fatal("route-less converged family IPv4Multicast remained deferred after all GR peers sent EoR")
		}
		if _, found := loc.Best(family.IPv6Unicast, v6Prefix); found {
			t.Fatal("IPv6 route installed in Loc-RIB before IPv6 EoR or Selection_Deferral_Timer expiry")
		}
		if completeCount.Load() != 0 {
			t.Fatalf("SelectionComplete emitted early while IPv6 still deferred: %d", completeCount.Load())
		}

		// Selection_Deferral_Timer upper bound (RFC4724-4.1-8 positive, configured "45"):
		clk.Add(44 * time.Second)
		if !r.selectionDefer.isDeferred(family.IPv6Unicast) {
			t.Fatal("IPv6Unicast released before 45s Selection_Deferral_Timer expiry")
		}
		clk.Add(1 * time.Second)
		if r.selectionDefer.isDeferred(family.IPv6Unicast) {
			t.Fatal("IPv6Unicast stayed deferred after 45s Selection_Deferral_Timer expired")
		}
		if _, found := loc.Best(family.IPv6Unicast, v6Prefix); !found {
			t.Fatal("IPv6 route was not selected into Loc-RIB upon Selection_Deferral_Timer expiry")
		}
		if completeCount.Load() != 1 {
			t.Fatalf("SelectionComplete count = %d, want 1 after all families released", completeCount.Load())
		}
	})

	t.Run("second distinct configured duration (90s) proves timer is configurable", func(t *testing.T) {
		r, loc, clk, completeCount := newDeferredRIB(t, "90")
		seedRoute(t, r, 1, p1, family.IPv4Unicast, v4NLRI, v4Attrs)
		r.reconcileReceived(&Event{
			RawNLRIBytes: map[family.Family][]byte{family.IPv4Unicast: v4NLRI},
		})
		clk.Add(89 * time.Second)
		if !r.selectionDefer.isDeferred(family.IPv4Unicast) {
			t.Fatal("90s timer released prematurely at 89s")
		}
		clk.Add(1 * time.Second)
		if r.selectionDefer.isDeferred(family.IPv4Unicast) {
			t.Fatal("90s timer did not release at 90s")
		}
		if _, found := loc.Best(family.IPv4Unicast, v4Prefix); !found {
			t.Fatal("IPv4 route not selected at 90s timer expiry")
		}
		if completeCount.Load() != 1 {
			t.Fatalf("SelectionComplete count = %d, want 1", completeCount.Load())
		}
	})

	t.Run("max-delay 0 string leaf disables selection deferral immediately", func(t *testing.T) {
		r, loc, _, _ := newDeferredRIB(t, "0")
		if r.selectionDefer.isHolding() {
			t.Fatal("max-delay 0 armed selectionDeferral; want inactive")
		}
		seedRoute(t, r, 1, p1, family.IPv4Unicast, v4NLRI, v4Attrs)
		r.reconcileReceived(&Event{
			RawNLRIBytes: map[family.Family][]byte{
				family.IPv4Unicast: v4NLRI,
			},
		})
		if _, found := loc.Best(family.IPv4Unicast, v4Prefix); !found {
			t.Fatal("max-delay 0 did not select received route into Loc-RIB immediately")
		}
	})
}
