// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- `default-information originate`.
// Related: internal/plugins/ospf/redistribute -- the ExternalInjector seam reused here.
// RFC: rfc/short/rfc2328.md -- sec 12.4.4 AS-External-LSA origination (Type 5 default)

package ospf

import (
	"fmt"
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

// defaultV6Prefix is ::/0, the IPv6 default route advertised by `default-information
// originate` as an OSPFv3 AS-External-LSA (LS Type 0x4005, RFC 5340 Section 4.4.3.6).
var defaultV6Prefix = netip.PrefixFrom(netip.IPv6Unspecified(), 0)

// defaultRoute returns the default prefix this engine advertises and the Loc-RIB family the
// condition on `default-information originate` is evaluated against: 0.0.0.0/0 in IPv4
// unicast for OSPFv2, ::/0 in IPv6 unicast for OSPFv3. Every default-route decision in this
// file asks this question first, because an OSPFv3 engine that took the OSPFv2 answer would
// watch the wrong RIB table and key the wrong LSA.
func (e *engine) defaultRoute() (netip.Prefix, family.Family) {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return defaultV6Prefix, family.IPv6Unicast
	}
	return defaultV4Prefix, family.IPv4Unicast
}

// originateDefaultExternal originates or refreshes the one AS-External-LSA carrying the
// default route, in the LS Type and body format its address family requires. It reports
// whether the LSDB changed.
//
// RFC 5340 Section 4.4.3.6: "The LS type of an AS-external-LSA is set to the value 0x4005."
// RFC 5340 Appendix A.4.2.1 reads the OSPFv2 value 0x0005 as U=0 with S2S1=00, which is
// "Link-Local Scoping - Flooded only on originating link", so an OSPFv3 engine that
// originated the OSPFv2 Type 5 would put its default on the wire in a form that reaches no
// router past the first hop.
func (e *engine) originateDefaultExternal(router types.RouterID, type2 bool, metric, tag uint32) (bool, error) {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.v6OriginateDefaultExternal(router, type2, metric, tag)
	}
	e.mu.Lock()
	db := e.lsdb
	e.mu.Unlock()
	if db == nil {
		return false, errEngineNotReady
	}
	_, changed, err := db.OriginateExternal(router, [4]byte{}, [4]byte{}, types.OptionE, type2, metric, [4]byte{}, tag)
	return changed, err
}

// purgeDefaultExternal MaxAge-purges the default route's AS-External-LSA in this engine's
// address family and reports whether a live self LSA existed to purge.
func (e *engine) purgeDefaultExternal(router types.RouterID) bool {
	e.mu.Lock()
	db := e.lsdb
	lsid, hasLSID := e.redistV6[defaultV6Prefix]
	e.mu.Unlock()
	if db == nil {
		return false
	}
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		if !hasLSID {
			return false // no Link State ID was ever assigned, so no default was originated
		}
		_, purged := db.WithdrawSelf(types.BackboneArea, v6ExternalKey(router, lsid))
		return purged
	}
	return db.PurgeExternal(router, [4]byte{})
}

// injectDefaultExternal is the `redistribute` entry point for the default route, in either
// address family. It validates the prefix against the engine's family and hands the
// redistribution params to the serialized coordinator, which owns the shared default LSA.
func (e *engine) injectDefaultExternal(prefix netip.Prefix, source string, routeTag uint32) error {
	if err := e.checkDefaultFamily(prefix); err != nil {
		return err
	}
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return errEngineNotReady
	}
	type2, metric, tag := externalParams(cfg, source, routeTag)
	e.injectRedistDefault(cfg.RouterID, type2, metric, tag)
	return nil
}

// withdrawDefaultExternal is the `redistribute` withdrawal for the default route, in either
// address family. It drops the redistribute claim and purges the shared default LSA unless
// `default-information originate` still wants it.
func (e *engine) withdrawDefaultExternal(prefix netip.Prefix) (bool, error) {
	if err := e.checkDefaultFamily(prefix); err != nil {
		return false, err
	}
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return false, nil
	}
	return e.withdrawRedistDefault(cfg.RouterID), nil
}

// checkDefaultFamily rejects a default prefix from the wrong address family. An OSPFv2
// engine advertises 0.0.0.0/0 and an OSPFv3 engine ::/0, and a prefix from the other family
// would otherwise be encoded into an LSA body no peer in this family can read.
func (e *engine) checkDefaultFamily(prefix netip.Prefix) error {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		if prefix.Addr().Is4() {
			return fmt.Errorf("ospf: default prefix %q is not IPv6", prefix)
		}
		return nil
	}
	if !prefix.Addr().Is4() {
		return fmt.Errorf("ospf: default prefix %q is not IPv4", prefix)
	}
	return nil
}

// v6OriginateDefaultExternal originates the OSPFv3 AS-External-LSA for ::/0. RFC 5340
// Section 4.4.3.6: "The Link State ID of an AS-external-LSA has lost all of its addressing
// semantics and simply serves to distinguish multiple AS-external-LSAs that are originated
// by the same router." Nothing in ::/0 derives that index, so the default's Link State ID is
// allocated once from the same redistV6 table the redistribution path uses: the two intents
// share the one default LSA, exactly as they share the 0.0.0.0/0 key in OSPFv2, and holding
// the entry also keeps the LSA inside the keep-set v6WithdrawExternal builds.
func (e *engine) v6OriginateDefaultExternal(router types.RouterID, type2 bool, metric, tag uint32) (bool, error) {
	wirePrefix, ok := netipToV6Prefix(defaultV6Prefix, 0)
	if !ok {
		return false, fmt.Errorf("ospf: default prefix %q is not a usable IPv6 prefix", defaultV6Prefix)
	}
	e.mu.Lock()
	db := e.lsdb
	lsid, assigned := e.redistV6[defaultV6Prefix]
	if !assigned {
		e.redistV6Next++
		lsid = v6SummaryLSID(e.redistV6Next)
		e.redistV6[defaultV6Prefix] = lsid
	}
	e.mu.Unlock()
	if db == nil {
		return false, errEngineNotReady
	}
	return e.v6OriginateExternalLSA(router, lsid, wirePrefix, type2, metric, tag), nil
}

// applyDefaultInformation evaluates `default-information originate` and originates or
// withdraws the default route accordingly, in this engine's address family: 0.0.0.0/0 as
// an OSPFv2 Type 5 AS-External-LSA, ::/0 as an OSPFv3 0x4005 AS-External-LSA. RFC 2328 sec
// 12.4.4 plus vendor semantics: `always` originates unconditionally; the bare form
// originates only while a NON-OSPF default exists in the Loc-RIB for that family. It is
// re-evaluated at config-apply (reconcile) and live on Loc-RIB default-prefix changes
// (watchDefaultRoute).
//
// Idempotent and loop-safe: both origination paths short-circuit when the LSA body is
// unchanged, so repeated calls neither re-flood nor re-trigger SPF. The engine tracks
// defaultInfoOriginated so a withdraw never purges a default LSA that a `redistribute`
// rule (not default-information) happens to own for the same prefix.
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
	prefix, af := e.defaultRoute()
	want := di.Originate && (di.Always || hasNonOSPFDefault(prefix, af))

	switch {
	case want:
		type2 := di.MetricType != metricType1
		changed, err := e.originateDefaultExternal(cfg.RouterID, type2, di.Metric, 0)
		if err != nil {
			slog.Warn("ospf default-information: default origination failed", "prefix", prefix, "error", err)
		}
		e.mu.Lock()
		e.defaultInfoOriginated = true
		e.mu.Unlock()
		if changed {
			e.originateSelfLSAs()
			e.refreshExternalMetrics(db, cfg.RouterID)
		}
	default:
		// default-information no longer wants the default. Purge the AS-External-LSA ONLY
		// if a `redistribute` rule does not also currently inject the default (the two
		// intents share the one default LSA key); otherwise just drop our claim and leave
		// the LSA for redistribution. The flag also stops a purge when we never originated.
		e.mu.Lock()
		owned := e.defaultInfoOriginated
		e.defaultInfoOriginated = false
		e.mu.Unlock()
		if owned && !redistOwns {
			if e.purgeDefaultExternal(cfg.RouterID) {
				e.originateSelfLSAs()
				e.refreshExternalMetrics(db, cfg.RouterID)
			}
		}
	}
}

// injectRedistDefault records that a `redistribute` rule injects the default route and
// (re)originates the shared AS-External default with the redistribution params, in this
// engine's address family. Serialized with applyDefaultInformation via defaultInfoMu so the
// two default-route intents (default-information and redistribute) never race on the one
// shared default LSA.
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
	changed, err := e.originateDefaultExternal(router, type2, metric, tag)
	if err != nil {
		slog.Warn("ospf default-information: redistributed default origination failed", "error", err)
	}
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, router)
	}
}

// withdrawRedistDefault drops the redistribute claim on the default route and purges the
// shared AS-External default UNLESS default-information still originates it. Returns whether
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
	if e.purgeDefaultExternal(router) {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, router)
		return true
	}
	return wasInjected
}

// hasNonOSPFDefault reports whether the Loc-RIB holds a valid default route for prefix in
// af, contributed by a protocol OTHER than OSPF. The caller passes the pair its address
// family owns (defaultRoute), because an OSPFv3 engine asking the IPv4 unicast table would
// answer about a route it can never advertise. The self-exclusion is essential: OSPF's own
// originated or learned default must not satisfy its own `default-information originate`
// condition, which would otherwise self-sustain. The scan runs under the Loc-RIB shard lock
// via Inspect, because a Lookup result's Paths slice shares the stored backing array and
// ranging it off-lock races writers.
func hasNonOSPFDefault(prefix netip.Prefix, af family.Family) bool {
	found := false
	locrib.Default().Inspect(af, prefix, func(g locrib.PathGroup) {
		for i := range g.Paths {
			if p := g.Paths[i]; p.Valid() && p.Source != ospfspf.ProtocolID() {
				found = true
				return
			}
		}
	})
	return found
}

// watchDefaultRoute subscribes to Loc-RIB best-path changes for this engine's own default
// route (0.0.0.0/0 in IPv4 unicast, ::/0 in IPv6 unicast)
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
		prefix, af := e.defaultRoute()
		unsub := loc.OnChange(func(c locrib.Change) {
			if c.Family != af || c.Prefix != prefix {
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
