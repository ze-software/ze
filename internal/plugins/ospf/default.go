// Design: plan/learned/964-ospf-10-as-external-asbr.md -- `default-information originate`.
// Related: internal/plugins/ospf/redistribute -- the ExternalInjector seam reused here.
// RFC: rfc/short/rfc2328.md -- sec 12.4.4 AS-External-LSA origination (Type 5 default)

package ospf

import (
	"log/slog"
	"net/netip"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// defaultV4Prefix is 0.0.0.0/0, the IPv4 default route advertised by
// `default-information originate` as a Type 5 AS-External-LSA.
var defaultV4Prefix = netip.PrefixFrom(netip.AddrFrom4([4]byte{}), 0)

// applyDefaultInformation evaluates `default-information originate` and originates or
// withdraws the Type 5 default route (0.0.0.0/0) accordingly. RFC 2328 sec 12.4.4 plus
// vendor semantics: `always` originates unconditionally; the bare form originates only
// while a NON-OSPF default exists in the Loc-RIB. It is re-evaluated at config-apply
// (reconcile) and live on Loc-RIB default-prefix changes (watchDefaultRoute).
//
// Idempotent and loop-safe: OriginateExternal short-circuits when the LSA body is
// unchanged, so repeated calls neither re-flood nor re-trigger SPF. The engine tracks
// defaultInfoOriginated so a withdraw never purges a Type 5 default that a
// `redistribute` rule (not default-information) happens to own for 0.0.0.0/0.
func (e *engine) applyDefaultInformation() {
	// Serialize so the reconcile caller and the watcher worker cannot interleave: a
	// stale worker run must not re-originate a default a concurrent config-disable just
	// withdrew. The cfg/flags are re-read fresh under e.mu inside this critical section,
	// so whichever caller runs last observes the latest config and converges correctly.
	e.defaultInfoMu.Lock()
	defer e.defaultInfoMu.Unlock()

	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	redistOwns := e.redistDefaultInjected
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}

	di := cfg.DefaultInformation
	want := di.Originate && (di.Always || hasNonOSPFDefault())

	switch {
	case want:
		type2 := di.MetricType != metricType1
		_, changed, err := db.OriginateExternal(cfg.RouterID, [4]byte{}, [4]byte{}, types.OptionE, type2, di.Metric, [4]byte{}, 0)
		if err != nil {
			slog.Warn("ospf default-information: Type 5 default origination failed", "error", err)
		}
		e.mu.Lock()
		e.defaultInfoOriginated = true
		e.mu.Unlock()
		if changed {
			e.originateSelfLSAs()
			e.refreshExternalMetrics(db, cfg.RouterID)
		}
	default:
		// default-information no longer wants the default. Purge the Type 5 ONLY if a
		// `redistribute` rule does not also currently inject 0.0.0.0/0 (the two intents
		// share the one default LSA key); otherwise just drop our claim and leave the
		// LSA for redistribution. The flag also stops a purge when we never originated.
		e.mu.Lock()
		owned := e.defaultInfoOriginated
		e.defaultInfoOriginated = false
		e.mu.Unlock()
		if owned && !redistOwns {
			if db.PurgeExternal(cfg.RouterID, [4]byte{}) {
				e.originateSelfLSAs()
				e.refreshExternalMetrics(db, cfg.RouterID)
			}
		}
	}
}

// injectRedistDefault records that a `redistribute` rule injects 0.0.0.0/0 and
// (re)originates the shared Type 5 default with the redistribution params. Serialized
// with applyDefaultInformation via defaultInfoMu so the two default-route intents
// (default-information and redistribute) never race on the one shared default LSA.
func (e *engine) injectRedistDefault(router types.RouterID, type2 bool, metric, tag uint32) {
	e.defaultInfoMu.Lock()
	defer e.defaultInfoMu.Unlock()
	e.mu.Lock()
	db := e.lsdb
	e.redistDefaultInjected = true
	e.mu.Unlock()
	if db == nil {
		return
	}
	_, changed, err := db.OriginateExternal(router, [4]byte{}, [4]byte{}, types.OptionE, type2, metric, [4]byte{}, tag)
	if err != nil {
		slog.Warn("ospf default-information: redistributed Type 5 default origination failed", "error", err)
	}
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, router)
	}
}

// withdrawRedistDefault drops the redistribute claim on 0.0.0.0/0 and purges the
// shared Type 5 default UNLESS default-information still originates it. Returns whether
// a redistribute claim existed (so the consumer can bump its withdrawn metric even when
// the LSA is kept alive for default-information).
func (e *engine) withdrawRedistDefault(router types.RouterID) bool {
	e.defaultInfoMu.Lock()
	defer e.defaultInfoMu.Unlock()
	e.mu.Lock()
	db := e.lsdb
	wasInjected := e.redistDefaultInjected
	e.redistDefaultInjected = false
	diOwns := e.defaultInfoOriginated
	e.mu.Unlock()
	if db == nil || diOwns {
		return wasInjected // default-information keeps the default LSA alive
	}
	if db.PurgeExternal(router, [4]byte{}) {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, router)
		return true
	}
	return wasInjected
}

// hasNonOSPFDefault reports whether the Loc-RIB holds a valid default route
// (0.0.0.0/0) contributed by a protocol OTHER than OSPF. The self-exclusion is
// essential: OSPF's own originated or learned default must not satisfy its own
// `default-information originate` condition, which would otherwise self-sustain.
// The scan runs under the Loc-RIB shard lock via Inspect, because a Lookup result's
// Paths slice shares the stored backing array and ranging it off-lock races writers.
func hasNonOSPFDefault() bool {
	found := false
	locrib.Default().Inspect(family.IPv4Unicast, defaultV4Prefix, func(g locrib.PathGroup) {
		for i := range g.Paths {
			if p := g.Paths[i]; p.Valid() && p.Source != ospfspf.ProtocolID() {
				found = true
				return
			}
		}
	})
	return found
}

// watchDefaultRoute subscribes to Loc-RIB best-path changes for the IPv4 default route
// so a conditional `default-information originate` reacts when a non-OSPF default
// appears or disappears, independently of OSPF topology changes. The OnChange handler
// runs under the Loc-RIB shard write lock and MUST NOT re-enter the RIB, so it only
// enqueues a coalesced re-evaluation token; a long-lived worker drains it and calls
// applyDefaultInformation outside the lock (mirrors the sysrib subscriber shape).
// Started once; the worker exits and unsubscribes on engine shutdown.
func (e *engine) watchDefaultRoute() {
	e.defaultWatchOnce.Do(func() {
		loc := locrib.Default()
		if loc == nil {
			return
		}
		// Buffer 1 + non-blocking send coalesces a burst of RIB changes into a single
		// pending re-evaluation (applyDefaultInformation always reads the current RIB).
		ch := make(chan struct{}, 1)
		unsub := loc.OnChange(func(c locrib.Change) {
			if c.Family != family.IPv4Unicast || c.Prefix != defaultV4Prefix {
				return
			}
			select {
			case ch <- struct{}{}:
			default: // a re-evaluation is already pending
			}
		})
		e.wg.Go(func() {
			defer unsub()
			for {
				select {
				case <-e.ctx.Done():
					return
				case <-ch:
					e.applyDefaultInformation()
				}
			}
		})
	})
}
