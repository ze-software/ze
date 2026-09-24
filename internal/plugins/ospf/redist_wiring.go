// Design: docs/architecture/ospf/ospf-10-as-external-asbr.md -- engine side of OSPF redistribution.
// Related: internal/plugins/ospf/redistribute -- the RedistConsumer + producer Source.
// RFC: rfc/short/rfc2328.md -- sec 12.4.4 AS-External-LSA origination (Type 5)

package ospf

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"

	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

var errEngineNotReady = errors.New("ospf: engine not ready for external origination")

// externalImport retains the source policy and originated NSSA scopes until
// withdrawal. The redistribution table bounds this state. Guarded by nssaMu.
type externalImport struct {
	source string
	tag    uint32
	areas  []types.AreaID
}

// InjectExternal implements ospfredistribute.ExternalInjector: it originates a Type
// 5 AS-External-LSA for prefix learned from source, applying the per-source metric /
// metric-type / route-tag from the `ospf` container's `redistribute` config (or the
// code defaults), then re-originates the Router-LSA (E-bit) and re-floods.
func (e *engine) InjectExternal(prefix netip.Prefix, source string, routeTag uint32) error {
	// The default route shares its one AS-External-LSA with `default-information originate`
	// in BOTH address families, so it goes through the serialized default-route coordinator
	// BEFORE the family split: a withdraw from one intent must never drop a default the
	// other still wants, and the coordinator picks the LS Type the family needs (0x0005 for
	// OSPFv2, 0x4005 for OSPFv3, RFC 5340 Section 4.4.3.6).
	if prefix.IsValid() && prefix.Bits() == 0 {
		return e.injectDefaultExternal(prefix, source, routeTag)
	}
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	return e.injectExternalLocked(prefix, source, routeTag)
}

// injectExternalLocked MUST be called with nssaMu held, including replay after
// an interface or source-policy change.
func (e *engine) injectExternalLocked(prefix netip.Prefix, source string, routeTag uint32) error {
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		return e.v6InjectExternal(prefix, source, routeTag)
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
	type2, metric, tag := externalParams(cfg, source, routeTag)
	prefix = prefix.Masked()
	network := prefix.Addr().As4()
	mask := maskBytes(prefix.Bits())
	nssas, canType5 := e.externalScope()
	areas := make([]types.AreaID, 0, len(nssas))
	propagate := externalPropagate(cfg, source, canType5)
	changed := false
	if !canType5 {
		changed = db.PurgeExternal(cfg.RouterID, network)
	}
	for _, n := range nssas {
		areas = append(areas, n.area)
		if _, originated := db.OriginateNSSA(n.area, cfg.RouterID, network, mask, type2, metric, n.fa, tag, propagate); originated {
			changed = true
		}
	}
	// RFC 2328 sec 12.4.4: a Type 5 (Forwarding Address 0.0.0.0, forward via this ASBR)
	// is originated AS-wide when this router can inject it directly (normal/backbone
	// attachment, or no NSSA attachment at all).
	if canType5 {
		_, originated, err := db.OriginateExternal(cfg.RouterID, network, mask, types.OptionE, type2, metric, [4]byte{}, tag)
		if err != nil {
			// The Type 5 was not installed (AS-external store full). Drop any redistribute
			// claim for this network and surface the failure so the consumer logs it and
			// does NOT count the route as injected (ze_ospf_redist_injected_total).
			e.mu.Lock()
			delete(e.redistExternals, network)
			e.mu.Unlock()
			return err
		}
		changed = changed || originated
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
	if e.rememberExternalImport(prefix, source, routeTag, areas,
		types.LSAKey{Type: types.LSTypeNSSA, LinkStateID: types.LinkStateID(network), AdvertisingRouter: cfg.RouterID}) {
		changed = true
	}
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, cfg.RouterID)
	}
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
	cfg, running := e.externalInterfaces()
	return e.externalScopeFor(cfg, running, nil)
}

// externalInterfaces excludes Down interfaces even while their configured
// runtime remains in running. Passive interfaces still advertise stub networks.
func (e *engine) externalInterfaces() (ospfConfig, []interfaceConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()
	running := make([]interfaceConfig, 0, len(e.running))
	for _, ic := range e.running {
		if !ic.Passive {
			if rt := e.interfaces[ic.Name]; rt != nil {
				if rt.State() == ospfiface.StateDown {
					continue
				}
			}
		}
		running = append(running, ic)
	}
	return e.cfg, running
}

// externalPropagate implements RFC 3101 Appendix D's explicit, P-clear-default
// source policy. A Type-5 twin takes precedence over the operator's P-bit request.
func externalPropagate(cfg ospfConfig, source string, canType5 bool) bool {
	if canType5 {
		return false
	}
	for _, rule := range cfg.Redistribute {
		if rule.Source == source {
			return rule.NSSAPropagate
		}
	}
	return false
}

// rememberExternalImport MUST run under nssaMu. Its caller has originated the
// current scopes; this removes copies in NSSAs no longer attached.
func (e *engine) rememberExternalImport(prefix netip.Prefix, source string, tag uint32, areas []types.AreaID, key types.LSAKey) bool {
	changed := false
	for _, previous := range e.externalImports[prefix].areas {
		found := false
		for _, area := range areas {
			if area == previous {
				found = true
				break
			}
		}
		if !found {
			if e.lsdb.PurgeNSSAKey(previous, key) {
				changed = true
			}
		}
	}
	if e.externalImports == nil {
		e.externalImports = make(map[netip.Prefix]externalImport)
	}
	e.externalImports[prefix] = externalImport{source: source, tag: tag, areas: areas}
	return changed
}

// reconcileExternalImports MUST serialize with injection and withdrawal under
// nssaMu. Replaying retained intents also retries a MinLSInterval-delayed update.
func (e *engine) reconcileExternalImports() {
	e.nssaMu.Lock()
	for prefix, imported := range e.externalImports {
		if prefix.Bits() == 0 {
			continue // The default coordinator owns its Type-5 and Type-7 copies.
		}
		if err := e.injectExternalLocked(prefix, imported.source, imported.tag); err != nil {
			e.log.Warn("OSPF external re-origination failed", "prefix", prefix, "error", err)
		}
	}
	e.nssaMu.Unlock()
	e.applyDefaultInformation()
	e.applyNSSADefaults()
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
		case types.AreaTypeNSSA:
			fa := e.nssaIPv4Address(ic.Name)
			if idx, ok := seen[ic.AreaID]; ok {
				if nssas[idx].fa == ([4]byte{}) && fa != ([4]byte{}) {
					nssas[idx].fa = fa
				}
				continue
			}
			seen[ic.AreaID] = len(nssas)
			nssas = append(nssas, nssaAttachment{area: ic.AreaID, fa: fa})
		case types.AreaTypeStub:
			// stub areas carry no externals
		default:
			attachedNormal = true
		}
	}
	if attachedNormal {
		return nssas, true
	}
	for _, area := range cfg.Areas {
		if area.AreaType == types.AreaTypeNSSA {
			return nssas, false
		}
	}
	return nssas, len(nssas) == 0
}

// nssaIPv4Address is this router's OSPFv2 Type-7 forwarding address on name: its IPv4
// interface address, or the zero address when the interface carries none. RFC 3101
// Section 2.4 reads a zero forwarding address as "data traffic will be forwarded to the
// LSA's originator", so the caller uses it to decide whether a P-set LSA may be originated.
func (e *engine) nssaIPv4Address(name string) [4]byte {
	if e.forwardingAddress == nil {
		return interfaceIPv4Address(name)
	}
	addr, ok := e.forwardingAddress(name)
	if !ok || !addr.Is4() {
		return [4]byte{}
	}
	return addr.As4()
}

// WithdrawExternal implements ospfredistribute.ExternalInjector: MaxAge-purge the
// Type 5 for prefix and re-originate the Router-LSA (clearing the E-bit when the
// last external is gone, AC-6).
func (e *engine) WithdrawExternal(prefix netip.Prefix) (bool, error) {
	// The default route is coordinated before the family split, for the reason InjectExternal
	// states: both intents share the one AS-External default LSA in either address family.
	if prefix.IsValid() && prefix.Bits() == 0 {
		return e.withdrawDefaultExternal(prefix)
	}
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
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
	network := prefix.Addr().As4()
	// Drop the redistribute claim so the NSSA translator may again own this network if a
	// peer's Type 7 still describes it.
	e.mu.Lock()
	delete(e.redistExternals, network)
	e.mu.Unlock()
	removed := db.PurgeExternal(cfg.RouterID, network)
	for _, area := range e.externalImports[prefix].areas {
		if db.PurgeNSSA(area, cfg.RouterID, network) {
			removed = true
		}
	}
	delete(e.externalImports, prefix)
	if removed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, cfg.RouterID)
	}
	return removed, nil
}

// externalParams resolves the metric type (E1/E2), 24-bit metric, and External Route
// Tag for a redistributed route. The metric and the metric type come from the `ospf`
// container's per-source `redistribute` entry, falling back to the code defaults
// (metric 20, type-2) when the source is not enrolled. The tag is the route's own tag
// when it carries one, and the per-source `tag` otherwise; externalRouteTag owns that
// choice.
func externalParams(cfg ospfConfig, source string, routeTag uint32) (type2 bool, metric, tag uint32) {
	type2, metric, tag = true, DefaultExternalMetric, 0
	for _, r := range cfg.Redistribute {
		if r.Source == source {
			type2, metric, tag = r.MetricType != metricType1, r.Metric, r.Tag
			break
		}
	}
	return type2, metric, externalRouteTag(routeTag, tag)
}

// externalRouteTag chooses the External Route Tag for a redistributed route: the
// route's own tag when it carries one, and the per-source `tag` under
// `ospf { redistribute { source <src> } }` otherwise. RFC 2328 Appendix A.4.5 defines
// the field -- "A 32-bit field attached to each external route.  This is not used by
// the OSPF protocol itself." -- so the RFC settles the encoding and leaves the
// precedence to the implementation. Ze gives the more specific value the win: the
// per-source tag names a whole source, and a tag on one route names that route
// (owner decision, 2026-09-06; spec-static-route-tag-reaches-no-consumer D-2).
//
// A zero routeTag is a GUARD, not a value: it means the route carries no tag. Every
// producer already reads the two as one thing -- an absent `tag` leaf parses to zero,
// `show static route` omits a zero tag, and zero is the OSPF default external route
// tag -- so a route that says nothing and a route that says zero ask for the same LSA.
// The guard lives here, with a test on both sides of it, rather than inline at each
// caller (ai/rules/principles.md).
func externalRouteTag(routeTag, configured uint32) uint32 {
	if routeTag != 0 {
		return routeTag
	}
	return configured
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
