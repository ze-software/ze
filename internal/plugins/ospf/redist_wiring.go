// Design: plan/learned/964-ospf-10-as-external-asbr.md -- engine side of OSPF redistribution.
// Related: internal/plugins/ospf/redistribute -- the RedistConsumer + producer Source.
// RFC: rfc/short/rfc2328.md -- sec 12.4.4 AS-External-LSA origination (Type 5)

package ospf

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var errEngineNotReady = errors.New("ospf: engine not ready for external origination")

// InjectExternal implements ospfredistribute.ExternalInjector: it originates a Type
// 5 AS-External-LSA for prefix learned from source, applying the per-source metric /
// metric-type / route-tag from the `ospf` container's `redistribute` config (or the
// code defaults), then re-originates the Router-LSA (E-bit) and re-floods.
func (e *engine) InjectExternal(prefix netip.Prefix, source string) error {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.v6InjectExternal(prefix, source)
	}
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return errEngineNotReady
	}
	if !prefix.IsValid() || !prefix.Addr().Is4() {
		return fmt.Errorf("ospf: external prefix %q is not IPv4", prefix)
	}
	type2, metric, tag := externalParams(cfg, source)
	prefix = prefix.Masked()
	// 0.0.0.0/0 shares its Type 5 LSA key with `default-information originate`; route it
	// through the serialized default-route coordinator so a withdraw from one intent
	// never drops a default the other still wants.
	if prefix.Bits() == 0 {
		e.injectRedistDefault(cfg.RouterID, type2, metric, tag)
		return nil
	}
	network := prefix.Addr().As4()
	mask := maskBytes(prefix.Bits())
	nssas, canType5 := e.externalScope()
	// RFC 3101 sec 2: a redistributed route is advertised as a Type 7 inside each
	// attached NSSA (Type 5 is blocked there). The P (propagate) bit is set only when
	// this router cannot inject a Type 5 directly AND it has a non-zero intra-NSSA
	// forwarding address -- a P=1 Type 7 with a zero FA is not translatable.
	for _, n := range nssas {
		propagate := !canType5 && n.fa != ([4]byte{})
		db.OriginateNSSA(n.area, cfg.RouterID, network, mask, type2, metric, n.fa, tag, propagate)
	}
	// RFC 2328 sec 12.4.4: a Type 5 (Forwarding Address 0.0.0.0, forward via this ASBR)
	// is originated AS-wide when this router can inject it directly (normal/backbone
	// attachment, or no NSSA attachment at all).
	if canType5 {
		if _, _, err := db.OriginateExternal(cfg.RouterID, network, mask, types.OptionE, type2, metric, [4]byte{}, tag); err != nil {
			// The Type 5 was not installed (AS-external store full). Drop any redistribute
			// claim for this network and surface the failure so the consumer logs it and
			// does NOT count the route as injected (ze_ospf_redist_injected_total).
			e.mu.Lock()
			delete(e.redistExternals, network)
			e.mu.Unlock()
			return err
		}
	}
	// Record (or clear) the redistribute claim on this Type 5 key so the NSSA translator
	// does not also translate/purge a network this router already redistributes (RFC 3101
	// §3.6 -- the locally-originated Type 5 wins; the translation is skipped).
	e.mu.Lock()
	if canType5 {
		e.redistExternals[network] = true
	} else {
		delete(e.redistExternals, network)
	}
	e.mu.Unlock()
	e.originateSelfLSAs()
	e.refreshExternalMetrics(db, cfg.RouterID)
	return nil
}

// nssaAttachment is an attached NSSA area plus this router's intra-NSSA forwarding
// address (its interface address in that area), used as the Type 7 Forwarding Address.
type nssaAttachment struct {
	area types.AreaID
	fa   [4]byte
}

// externalScope enumerates the redistribution origination scope for this router: the
// NSSA areas it must originate Type 7 into, and whether it can originate a Type 5
// AS-wide directly. canType5 is true when the router has a normal/backbone attachment
// (it can flood Type 5 into a non-stub/non-NSSA area) OR has no NSSA attachment at all
// (plain ASBR behavior, preserved for routers with no NSSA areas).
func (e *engine) externalScope() ([]nssaAttachment, bool) {
	e.mu.Lock()
	cfg := e.cfg
	running := make([]interfaceConfig, 0, len(e.running))
	for _, ic := range e.running {
		running = append(running, ic)
	}
	e.mu.Unlock()
	return e.externalScopeFor(cfg, running, nil)
}

func (e *engine) externalScopeFor(cfg ospfConfig, running []interfaceConfig, activeIfaces map[string]bool) (nssas []nssaAttachment, canType5 bool) {
	sort.Slice(running, func(i, j int) bool { return running[i].Name < running[j].Name })
	attachedNormal := false
	seen := make(map[types.AreaID]int, len(running))
	for _, ic := range running {
		if activeIfaces != nil && !activeIfaces[ic.Name] {
			continue
		}
		switch areaTypeFor(cfg, ic.AreaID) {
		case areaTypeNSSA:
			fa := e.nssaIPv4Address(ic.Name)
			if idx, ok := seen[ic.AreaID]; ok {
				if nssas[idx].fa == ([4]byte{}) && fa != ([4]byte{}) {
					nssas[idx].fa = fa
				}
				continue
			}
			seen[ic.AreaID] = len(nssas)
			nssas = append(nssas, nssaAttachment{area: ic.AreaID, fa: fa})
		case areaTypeStub:
			// stub areas carry no externals
		default:
			attachedNormal = true
		}
	}
	return nssas, attachedNormal || len(nssas) == 0
}

func (e *engine) nssaIPv4Address(name string) [4]byte {
	if e.ipv4Address != nil {
		return e.ipv4Address(name)
	}
	return interfaceIPv4Address(name)
}

// WithdrawExternal implements ospfredistribute.ExternalInjector: MaxAge-purge the
// Type 5 for prefix and re-originate the Router-LSA (clearing the E-bit when the
// last external is gone, AC-6).
func (e *engine) WithdrawExternal(prefix netip.Prefix) (bool, error) {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.v6WithdrawExternal(prefix)
	}
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) || !prefix.IsValid() || !prefix.Addr().Is4() {
		return false, nil
	}
	prefix = prefix.Masked()
	// 0.0.0.0/0 is shared with `default-information originate`: purge only when
	// default-information does not also originate the default (serialized coordinator).
	if prefix.Bits() == 0 {
		return e.withdrawRedistDefault(cfg.RouterID), nil
	}
	network := prefix.Addr().As4()
	// Drop the redistribute claim so the NSSA translator may again own this network if a
	// peer's Type 7 still describes it.
	e.mu.Lock()
	delete(e.redistExternals, network)
	e.mu.Unlock()
	removed := db.PurgeExternal(cfg.RouterID, network)
	// Purge the Type 7 from every attached NSSA the inject may have originated into.
	nssas, _ := e.externalScope()
	for _, n := range nssas {
		if db.PurgeNSSA(n.area, cfg.RouterID, network) {
			removed = true
		}
	}
	if removed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, cfg.RouterID)
	}
	return removed, nil
}

// externalParams resolves the metric type (E1/E2), 24-bit metric, and route tag for
// a redistributed route from the `ospf` container's per-source `redistribute` entry,
// falling back to the code defaults (metric 20, type-2) when the source is not
// enrolled (assumption A-2: the generic RouteEntry carries no per-route metric).
func externalParams(cfg ospfConfig, source string) (type2 bool, metric, tag uint32) {
	type2, metric, tag = true, DefaultExternalMetric, 0
	for _, r := range cfg.Redistribute {
		if r.Source == source {
			return r.MetricType != metricType1, r.Metric, r.Tag
		}
	}
	return type2, metric, tag
}

// wireRedistProducer connects the redistribution producer Source to the SPF
// Computer's OnChange callback (export OSPF -> BGP). Called at OnStarted.
func (e *engine) wireRedistProducer(src *ospfredistribute.Source) {
	e.mu.Lock()
	computer := e.spf
	e.mu.Unlock()
	if computer != nil && src != nil {
		computer.SetOnChange(src.OnSPFChange)
	}
}

// refreshExternalMetrics updates the ASBR gauge and the self-external-LSA count
// gauge from the current AS-wide store.
func (e *engine) refreshExternalMetrics(db *ospflsdb.LSDB, router types.RouterID) {
	count := db.SelfExternalCount(router)
	e.mu.Lock()
	asbr := e.mASBR
	ext := e.mExternalLSAs
	e.mu.Unlock()
	if count > 0 {
		asbr.Set(1)
	} else {
		asbr.Set(0)
	}
	ext.Set(float64(count))
}

// maskBytes returns the 4-byte network mask for an IPv4 prefix length.
func maskBytes(bits int) [4]byte {
	var out [4]byte
	for i := 0; i < bits && i < 32; i++ {
		out[i/8] |= 1 << (7 - uint(i%8))
	}
	return out
}

// compile-time assertion: *engine satisfies the redistribution injector seam.
var _ ospfredistribute.ExternalInjector = (*engine)(nil)
