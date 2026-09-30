// Design: docs/architecture/ospf/ospf-11-stub-nssa.md -- engine NSSA default-route origination +
// translator election and Type 7 -> Type 5 translation.
// Related: internal/plugins/ospf/lsdb -- the Type 7 originator (OriginateNSSA).
// RFC: rfc/short/rfc3101.md -- sec 2.3 NSSA default; sec 3.5 translator election;
// sec 3.6 Type 7 -> Type 5 translation

package ospf

import (
	"bytes"
	"maps"
	"net/netip"
	"time"

	ospfspf "github.com/ze-software/ze/internal/plugins/ospf/spf"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
	ospfv3types "github.com/ze-software/ze/internal/plugins/ospf/v3/types"
)

// translatorGrace is the per-NSSA translator-stability state (RFC 3101 §3.5): active
// reports whether this router currently performs translation; lostAt is when it last
// lost the election while still active (zero while elected), starting the grace timer.
type translatorGrace struct {
	active bool
	lostAt time.Time
}

// translatorEffective applies the RFC 3101 §3.5 stability grace to the raw election
// result for an NSSA: a router that loses the election keeps translating until the
// stability interval elapses (so a transient flap of the elected translator does not
// open a Type 5 gap); a router that wins translates immediately.
func (e *engine) translatorEffective(area types.AreaID, elected bool, now time.Time, stability time.Duration) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	st := e.translatorState[area]
	switch {
	case elected:
		st.active = true
		st.lostAt = time.Time{}
	case st.active:
		if st.lostAt.IsZero() {
			st.lostAt = now
		}
		if now.Sub(st.lostAt) >= stability {
			st.active = false
			st.lostAt = time.Time{}
		}
	}
	e.translatorState[area] = st
	return st.active
}

// applyNSSADefaults reconciles the per-area NSSA default route. RFC 3101
// Section 2.4 requires an NSSA border router to originate a default into every
// directly attached NSSA. A regular NSSA gets a P-clear Type-7 default. The
// summary originator gives a no-summary NSSA its required Type-3 default.
// An internal NSSA router originates a P-set Type-7 default only when
// `default-originate` is enabled and a non-zero forwarding address is usable.
//
// The decision is the same in both address families, and only the LSA it produces
// differs (RFC 5340 Section 4.4.3.7: "The procedure for originating NSSA-LSAs in IPv6
// is the same as the IPv4 procedure documented in [NSSA]"). So the family is bound once,
// into originate and purge, and the decision below is written once for both.
func (e *engine) applyNSSADefaults() {
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	running := make([]interfaceConfig, 0, len(e.running))
	for _, ic := range e.running {
		running = append(running, ic)
	}
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}
	topology := e.lsdbTopology()
	byArea := make(map[types.AreaID][]ospflsdb.InterfaceInfo)
	activeIfaces := make(map[string]bool, len(topology))
	for idx := range topology {
		iface := topology[idx]
		byArea[iface.AreaID] = append(byArea[iface.AreaID], iface)
		if ospflsdb.AreaHasAdvertisedLinks(topology[idx : idx+1]) {
			activeIfaces[iface.Name] = true
		}
	}
	activeAreas := make([]types.AreaID, 0, len(byArea))
	active := make(map[types.AreaID]bool, len(byArea))
	for area, ifaces := range byArea {
		if ospflsdb.AreaHasAdvertisedLinks(ifaces) {
			activeAreas = append(activeAreas, area)
			active[area] = true
		}
	}
	isABR := ospfspf.IsABR(activeAreas)
	self := cfg.RouterID
	attached := make(map[types.AreaID]bool, len(cfg.Areas))
	hasFA := make(map[types.AreaID]bool, len(cfg.Areas))
	// originate installs this router's NSSA default LSA in one area and reports whether the
	// area store changed; purge MaxAge-flushes it. Binding the pair here is the address-family
	// dispatch: an OSPFv3 engine that reached the OSPFv2 producer would key its default 0x0007,
	// which RFC 5340 Appendix A.4.2.1 reads as function code 7 at link-local flooding scope
	// rather than the NSSA-LSA (0x2007), so the NSSA's internal routers would see no default.
	var originate func(area types.AreaID, type2 bool, metric, tag uint32, propagate bool) bool
	var purge func(area types.AreaID) bool
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		nssas, _ := e.externalScopeV6For(cfg, running, activeIfaces)
		scope := make(map[types.AreaID]nssaAttachmentV6, len(nssas))
		for _, n := range nssas {
			attached[n.area] = active[n.area]
			hasFA[n.area] = n.hasFA
			scope[n.area] = n
		}
		originate = func(area types.AreaID, type2 bool, metric, tag uint32, propagate bool) bool {
			return e.v6OriginateNSSALSA(area, self, v6NSSADefaultLSID, ospfv3packet.Prefix{}, type2, metric, scope[area].fa, scope[area].hasFA, tag, propagate)
		}
		purge = func(area types.AreaID) bool {
			return db.PurgeNSSAKey(area, v6NSSAKey(self, v6NSSADefaultLSID))
		}
	} else {
		nssas, _ := e.externalScopeFor(cfg, running, activeIfaces)
		faByArea := make(map[types.AreaID][4]byte, len(nssas))
		for _, n := range nssas {
			attached[n.area] = active[n.area]
			hasFA[n.area] = n.fa != ([4]byte{})
			faByArea[n.area] = n.fa
		}
		originate = func(area types.AreaID, type2 bool, metric, tag uint32, propagate bool) bool {
			_, c := db.OriginateNSSA(area, self, [4]byte{}, [4]byte{}, type2, metric, faByArea[area], tag, propagate)
			return c
		}
		purge = func(area types.AreaID) bool {
			return db.PurgeNSSA(area, self, [4]byte{})
		}
	}
	desired := make(map[types.AreaID]struct{}, len(cfg.Areas))
	changed := false
	defaultPrefix, _ := e.defaultRoute()
	imported, hasImport := e.externalImports[defaultPrefix]
	for _, a := range cfg.Areas {
		if a.AreaType != types.AreaTypeNSSA || !attached[a.AreaID] {
			continue
		}
		wantType7 := wantsType7Default(isABR, a.NoSummary, a.NSSADefaultOriginate, hasFA[a.AreaID])
		type2, metric, tag := false, a.DefaultCost, uint32(0)
		propagate := !isABR
		if !isABR {
			if hasImport {
				type2, metric, tag = externalParams(cfg, imported.source, imported.tag)
				propagate = externalPropagate(cfg, imported.source, false)
				wantType7 = !a.NoSummary
				if propagate {
					wantType7 = wantType7 && hasFA[a.AreaID]
				}
			}
		}
		if wantType7 {
			desired[a.AreaID] = struct{}{}
			if originate(a.AreaID, type2, metric, tag, propagate) {
				changed = true
			}
		} else if purge(a.AreaID) {
			changed = true
		}
	}
	for area := range e.nssaDefaultAreas {
		if _, keep := desired[area]; !keep && purge(area) {
			changed = true
		}
	}
	e.nssaDefaultAreas = desired
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, cfg.RouterID)
	}
}

// wantsType7Default reports whether this router originates a Type-7 default LSA into one
// attached NSSA. The decision carries no address family: the caller binds the producer that
// encodes it (RFC 5340 Section 4.4.3.7: "The procedure for originating NSSA-LSAs in IPv6 is
// the same as the IPv4 procedure documented in [NSSA]").
//
// Four RFC 3101 rules decide it, and each one can only subtract:
//
// RFC 3101 Section 2.4: "NSSA border routers must originate an LSA for the default
// destination into all their directly attached NSSAs in order to support intra-AS routing
// and inter-AS routing." A border router therefore needs no operator leaf.
//
// RFC 3101 Section 2.7: "When OSPF's summary routes are not imported, the default LSA
// originated by an NSSA border router into the NSSA should be a Type-3 summary-LSA."
// A no-summary NSSA takes its default from the summary originator instead, so this
// producer stands down there.
//
// RFC 3101 Section 1.3: "The Type-7 default LSAs originated by NSSA internal routers and
// the no-summary option are mutually exclusive features." An internal router's
// `default-originate` is therefore inert in a no-summary NSSA, whose Type-3 default exists
// precisely to keep inter-area traffic off a Type-7 default.
//
// RFC 3101 Section 2.4: "The LSAs of these networks must have a valid non-zero forwarding
// address." An internal router's default is P-set, so it needs one.
func wantsType7Default(isABR, noSummary, defaultOriginate, hasForwardingAddr bool) bool {
	// RFC requirement: RFC3101-2.7-2 -- a no-summary NSSA takes its
	// border-router default as a summary-LSA, so none is originated here.
	if noSummary {
		return false
	}
	// RFC requirement: RFC3101-2.4-5 -- an NSSA border router MUST
	// originate a default into every attached regular NSSA, with no operator gate.
	if isABR {
		return true
	}
	// RFC requirement: RFC3101-2.4-2 -- an internal router's default is
	// P-set, so it MUST carry a non-zero forwarding address or not be originated.
	return defaultOriginate && hasForwardingAddr
}

// routerReachability is the completed-SPF view the translator election reads. The
// production source is ospfspf.ReachabilitySnapshot; tests substitute a fixed view.
type routerReachability interface {
	Ready() bool
	RouterReachable(area types.AreaID, id types.RouterID) bool
}

// spfReachability is the production nssaReachabilityFn: the last completed SPF run, or
// an empty (not Ready) snapshot when no SPF computer exists.
func (e *engine) spfReachability() routerReachability {
	if e.spf == nil {
		return ospfspf.ReachabilitySnapshot{}
	}
	return e.spf.Reachability()
}

// electNSSATranslator reports whether self is the elected Type 7 -> Type 5 translator
// among the NSSA's ABRs. RFC 3101 Section 3.5: role `never` never translates; `always`
// always translates; `candidate` translates iff self has the highest Router ID among
// the listed candidate ABRs (the higher Router ID wins, analogous to DR election).
// decided is false when the candidate's list cannot be built because no SPF run has
// completed for the current configuration: the caller then keeps the translator state
// it had, rather than electing itself over an empty list it never computed.
func (e *engine) electNSSATranslator(self types.RouterID, role string, area types.AreaID, abrs []types.RouterID) (elected, decided bool) {
	switch role {
	case translateRoleNever:
		return false, true
	case translateRoleAlways:
		return true, true
	}
	reach := e.nssaReachabilityFn()
	if !reach.Ready() {
		return false, false
	}
	for _, r := range abrs {
		// RFC 3101 Section 3.1: "An NSSA border router whose NSSA's NSSATranslatorRole is
		// set to Candidate must maintain a list of the NSSA's border routers that are
		// reachable both over the NSSA and as ASBRs over the AS's transit topology."
		// RFC requirement: RFC3101-3.1-2 -- a router the NSSA's SPF does not reach is not
		// on the list, so it cannot disable the candidate.
		if !reach.RouterReachable(area, r) {
			continue
		}
		if bytes.Compare(r[:], self[:]) > 0 {
			return false, true
		}
	}
	return true, true
}

// nssaTranslatorActive reports the translator state the last decided election left for
// area, used while the election cannot be decided.
func (e *engine) nssaTranslatorActive(area types.AreaID) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.translatorState[area].active
}

// nssaABRs returns the Router IDs of the NSSA area's routers whose Router-LSA carries every
// bit in flags. With B|Nt it is the translator-election candidate set, before the
// reachability filter electNSSATranslator applies. Requiring Nt is a disclosed deviation
// from RFC 3101 Section 3.1 (RFC3101-3.1-4, a {gap} row): the RFC disables a candidate for
// any listed router with a higher Router ID, so a higher-Router-ID ABR configured
// `translate never` (Nt clear) would leave the NSSA with no translator. With B alone it is the NSSA's border routers, the "NSSA translators" of RFC 3101
// Section 3.2 step (2) (owner decision D-12).
func (e *engine) nssaABRs(db *ospflsdb.LSDB, area types.AreaID, flags uint8) []types.RouterID {
	var abrs []types.RouterID
	for _, h := range db.Summary(area) {
		if h.Type != types.LSTypeRouter || h.Age.IsMaxAge() {
			continue
		}
		lsa, ok := db.LookupLSA(area, h.Key())
		if !ok {
			continue
		}
		body, err := lsa.DecodeRouter()
		if err != nil {
			continue
		}
		if body.Flags&flags != flags {
			continue
		}
		abrs = append(abrs, h.AdvertisingRouter)
	}
	return abrs
}

// equivalentType5 reports whether one of the higher-Router-ID translators' Type-5s is
// functionally equivalent to the Type-7 body describing network.
// RFC 3101 Section 3.2 step (2): "the calculating router has the highest router ID amongst NSSA
// translators that have originated a functionally equivalent Type-5 LSA (i.e. same destination,
// cost and non-zero forwarding address)". Destination is the masked Link State ID with the same
// mask; cost is the metric (owner decision D-12).
func equivalentType5(yieldTo []packet.LSA, network [4]byte, nssa packet.ExternalLSA) bool {
	if nssa.ForwardingAddr == ([4]byte{}) {
		return false // no non-zero forwarding address, so no Type-5 can be equivalent
	}
	for i := range yieldTo {
		external, err := yieldTo[i].DecodeExternal()
		if err != nil {
			continue // an undecodable Type-5 describes no destination
		}
		if external.NetworkMask != nssa.NetworkMask {
			continue
		}
		if maskIPv4([4]byte(yieldTo[i].Header.LinkStateID), nssa.NetworkMask) != maskIPv4(network, nssa.NetworkMask) {
			continue
		}
		if external.Metric != nssa.Metric {
			continue
		}
		if external.ForwardingAddr != nssa.ForwardingAddr {
			continue
		}
		return true
	}
	return false
}

// equivalentType5V6 is the OSPFv3 form of equivalentType5: the destination is the body's
// prefix, since an OSPFv3 Link State ID carries no addressing semantics (RFC 5340 Section 4.4.3).
// RFC 3101 Section 3.2 step (2): "functionally equivalent Type-5 LSA (i.e. same destination,
// cost and non-zero forwarding address)".
func equivalentType5V6(yieldTo []packet.LSA, nssa ospfv3packet.ExternalLSA) bool {
	if !nssa.HasForwardingAddr {
		return false
	}
	if nssa.ForwardingAddr == ([16]byte{}) {
		return false
	}
	for i := range yieldTo {
		decoded, err := ospfv3packet.DecodeLSA(yieldTo[i].RawBytes)
		if err != nil {
			continue
		}
		external, err := decoded.DecodeExternal()
		if err != nil {
			continue
		}
		if external.Prefix.Length != nssa.Prefix.Length {
			continue
		}
		if !bytes.Equal(external.Prefix.Address, nssa.Prefix.Address) {
			continue
		}
		if external.Metric != nssa.Metric {
			continue
		}
		if !external.HasForwardingAddr {
			continue
		}
		if external.ForwardingAddr != nssa.ForwardingAddr {
			continue
		}
		return true
	}
	return false
}

// maskIPv4 returns address with every bit outside mask cleared.
func maskIPv4(address, mask [4]byte) [4]byte {
	return [4]byte{address[0] & mask[0], address[1] & mask[1], address[2] & mask[2], address[3] & mask[3]}
}

// nssaTranslation is one desired Type 7 -> Type 5 translation.
type nssaTranslation struct {
	network [4]byte
	mask    [4]byte
	type2   bool
	metric  uint32
	fwd     [4]byte
	tag     uint32
	area    types.AreaID
}

// translateNSSA performs the RFC 3101 Section 3.6 Type 7 -> Type 5 translation. For
// each attached NSSA where this router is the elected translator, every Type 7 with the
// P-bit set and a non-zero Forwarding Address is re-originated as a Type 5 AS-External-
// LSA (P cleared, Advertising Router = this router, Forwarding Address / metric / E-bit
// / tag preserved). Translations no longer backed by a P=1 Type 7 -- or all of them
// when this router is not an ABR / loses the role -- are MaxAge-purged. Only the
// elected translator translates, so no duplicate Type 5 is injected (trap #9).
func (e *engine) translateNSSA(now time.Time) {
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	if e.dispatch != nil && e.dispatch.codec.IsV6() {
		e.translateNSSAV6(now)
		return
	}
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	redist := maps.Clone(e.redistExternals) // networks this router redistributes as Type 5
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}
	self := cfg.RouterID
	nssas, isABR := e.externalScope()

	desired := make(map[[4]byte]nssaTranslation)
	if isABR {
		type areaPolicy struct {
			role      string
			stability time.Duration
		}
		policyByArea := make(map[types.AreaID]areaPolicy, len(cfg.Areas))
		for _, a := range cfg.Areas {
			if a.AreaType == types.AreaTypeNSSA {
				policyByArea[a.AreaID] = areaPolicy{role: a.NSSATranslateRole, stability: time.Duration(a.NSSAStabilityInterval) * time.Second}
			}
		}
		for _, n := range nssas {
			p := policyByArea[n.area]
			elected, decided := e.electNSSATranslator(self, p.role, n.area, e.nssaABRs(db, n.area, packet.RouterFlagB|packet.RouterFlagNt))
			effective := e.nssaTranslatorActive(n.area)
			if decided {
				effective = e.translatorEffective(n.area, elected, now, p.stability)
			}
			if !effective {
				continue
			}
			// RFC 3101 Section 3.2: the Type-5s that higher-Router-ID NSSA translators
			// originated, against which each Type-7 is checked for a functional equivalent.
			yieldTo := db.HigherRIDTranslatorExternals(types.LSTypeASExternal, e.nssaABRs(db, n.area, packet.RouterFlagB), self)
			for _, h := range db.Summary(n.area) {
				if h.Type != types.LSTypeNSSA || h.Age.IsMaxAge() {
					continue
				}
				lsa, ok := db.LookupLSA(n.area, h.Key())
				if !ok || !lsa.Header.Options.Has(types.OptionNP) {
					continue // P=0 Type 7 is not translated
				}
				body, err := lsa.DecodeExternal()
				if err != nil || body.ForwardingAddr == ([4]byte{}) {
					continue // a zero forwarding address is not translatable
				}
				network := [4]byte(h.LinkStateID)
				if redist[network] {
					continue // RFC 3101 §3.6: keep the locally-redistributed Type 5; do not translate
				}
				// RFC 3101 Section 3.2 step (2)
				if equivalentType5(yieldTo, network, body) {
					continue
				}
				desired[network] = nssaTranslation{network: network, mask: body.NetworkMask, type2: body.ExternalType2, metric: body.Metric, fwd: body.ForwardingAddr, tag: body.ExternalRouteTag, area: n.area}
			}
		}
	}
	e.applyTranslations(db, self, desired, redist)
}

// applyTranslations reconciles the set of translated Type 5 LSAs against desired,
// originating new translations (and bumping ze_ospf_nssa_translations_total{area}) and
// MaxAge-purging the ones that no longer apply.
func (e *engine) applyTranslations(db *ospflsdb.LSDB, self types.RouterID, desired map[[4]byte]nssaTranslation, redist map[[4]byte]bool) {
	e.mu.Lock()
	prev := e.translations
	counter := e.mNSSATranslations
	e.mu.Unlock()

	changed := false
	next := make(map[[4]byte]types.AreaID, len(desired))
	for network, tr := range desired {
		_, c, err := db.OriginateExternal(self, tr.network, tr.mask, types.OptionE, tr.type2, tr.metric, tr.fwd, tr.tag)
		if err != nil {
			// The AS-external store is at capacity: installOriginated already logged the drop.
			// Do NOT count a translation that never entered the backbone, and do NOT record it
			// as translated -- so a later tick re-attempts and counts it once the store frees.
			// Mirrors the redistribution path (redist_wiring.go), which surfaces the same error.
			continue
		}
		if c {
			changed = true
		}
		if _, existed := prev[network]; !existed {
			counter.With(tr.area.String()).Inc()
		}
		next[network] = tr.area
	}
	for network := range prev {
		if _, keep := desired[network]; keep || redist[network] {
			// Still translated, or now owned by this router's own redistribution: never
			// purge a Type 5 that redistribution owns (RFC 3101 §3.6, shared LSA key).
			continue
		}
		if db.PurgeExternal(self, network) {
			changed = true
		}
	}
	e.mu.Lock()
	e.translations = next
	e.mu.Unlock()
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, self)
	}
}

type nssaTranslationV6 struct {
	lsid types.LinkStateID
	body ospfv3packet.ExternalLSA
	area types.AreaID
}

func (e *engine) translateNSSAV6(now time.Time) {
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	redist := make(map[[4]byte]bool, len(e.redistV6))
	for _, lsid := range e.redistV6 {
		redist[[4]byte(lsid)] = true
	}
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}
	self := cfg.RouterID
	nssas, isABR := e.externalScopeV6()

	desired := make(map[[4]byte]nssaTranslationV6)
	if isABR {
		type areaPolicy struct {
			role      string
			stability time.Duration
		}
		policyByArea := make(map[types.AreaID]areaPolicy, len(cfg.Areas))
		for _, a := range cfg.Areas {
			if a.AreaType == types.AreaTypeNSSA {
				policyByArea[a.AreaID] = areaPolicy{role: a.NSSATranslateRole, stability: time.Duration(a.NSSAStabilityInterval) * time.Second}
			}
		}
		for _, n := range nssas {
			p := policyByArea[n.area]
			elected, decided := e.electNSSATranslator(self, p.role, n.area, e.nssaABRsV6(db, n.area, ospfv3packet.RouterFlagB|ospfv3packet.RouterFlagNt))
			effective := e.nssaTranslatorActive(n.area)
			if decided {
				effective = e.translatorEffective(n.area, elected, now, p.stability)
			}
			if !effective {
				continue
			}
			// RFC 3101 Section 3.2 (RFC 5340 Section 4.4.3.6 carries it to OSPFv3).
			yieldTo := db.HigherRIDTranslatorExternals(types.LSType(ospfv3types.LSTypeASExternal), e.nssaABRsV6(db, n.area, ospfv3packet.RouterFlagB), self)
			for _, h := range db.Summary(n.area) {
				if !h.Type.NSSA() || h.Age.IsMaxAge() {
					continue
				}
				lsa, ok := db.LookupLSA(n.area, h.Key())
				if !ok || len(lsa.RawBytes) == 0 {
					continue
				}
				decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
				if err != nil {
					continue
				}
				body, err := decoded.DecodeExternal()
				if err != nil || !ospfv3packet.NSSAPropagate(body) {
					continue // P=0 Type 7 is not translated
				}
				if !body.HasForwardingAddr || body.ForwardingAddr == ([16]byte{}) {
					continue // a zero forwarding address is not translatable
				}
				// RFC 5340 Appendix A.4.7: the forwarding address "MUST NOT be set to the
				// IPv6 Unspecified Address (0:0:0:0:0:0:0:0) or an IPv6 Link-Local Address
				// (Prefix FE80/10)", and "an OSPFv3 implementation advertising a forwarding
				// address MUST advertise a global IPv6 address". The translated Type-5 copies
				// the Type-7's address (RFC 3101 Section 3.2), so a received NSSA-LSA whose
				// address is not global is not translated. On an IPv4 AF, RFC 5838 Section 2.6
				// governs the field instead.
				if !translatableForwardingAddressV6(body.ForwardingAddr, e.af) {
					continue
				}
				lsid := h.LinkStateID
				if redist[[4]byte(lsid)] {
					continue // RFC 3101 §3.6: keep the locally-redistributed Type 5; do not translate
				}
				// RFC 3101 Section 3.2 step (2)
				if equivalentType5V6(yieldTo, body) {
					continue
				}
				body.Prefix.Options &^= ospfv3types.OptPrefixP
				desired[[4]byte(lsid)] = nssaTranslationV6{lsid: lsid, body: body, area: n.area}
			}
		}
	}
	e.applyTranslationsV6(db, self, desired)
}

// translatableForwardingAddressV6 reports whether a received NSSA-LSA's Forwarding Address
// field fa may be copied into the AS-external-LSA this router translates it into, at the
// instance's address family af. The Type-7 is not translated when it returns false.
func translatableForwardingAddressV6(fa [16]byte, af addressFamily) bool {
	if !af.isIPv4() {
		return v6UsableForwardingAddress(netip.AddrFrom16(fa))
	}
	// RFC 5838 Section 2.6: "For IPv4 unicast and IPv4 multicast AFs, the Forwarding Address
	// in AS-external-LSAs and NSSA-LSAs MUST encode an IPv4 address. To achieve this, the
	// IPv4 Forwarding Address is advertised by placing it in the first 32 bits of the
	// Forwarding Address field in AS-external-LSAs and NSSA-LSAs. The remaining bits MUST be
	// set to zero." The translated Type-5 copies the field, so a received field that is not
	// an IPv4 address followed by 96 zero bits is not translated.
	if [12]byte(fa[4:]) != ([12]byte{}) {
		return false
	}
	v4 := netip.AddrFrom4([4]byte(fa[:4]))
	return !v4.IsUnspecified() && !v4.IsLoopback() && !v4.IsMulticast() && !v4.IsLinkLocalUnicast()
}

// nssaABRsV6 is the OSPFv3 counterpart of nssaABRs; the Router-LSA MUST also carry the
// N option, so a router that is not attached to the NSSA as an NSSA is never counted.
func (e *engine) nssaABRsV6(db *ospflsdb.LSDB, area types.AreaID, flags uint8) []types.RouterID {
	var abrs []types.RouterID
	for _, h := range db.Summary(area) {
		if h.Type != types.LSType(ospfv3types.LSTypeRouter) || h.Age.IsMaxAge() {
			continue
		}
		lsa, ok := db.LookupLSA(area, h.Key())
		if !ok || len(lsa.RawBytes) == 0 {
			continue
		}
		decoded, err := ospfv3packet.DecodeLSA(lsa.RawBytes)
		if err != nil {
			continue
		}
		body, err := decoded.DecodeRouter()
		if err != nil {
			continue
		}
		if body.Flags&flags != flags {
			continue
		}
		if !body.Options.NSSA() {
			continue
		}
		abrs = append(abrs, h.AdvertisingRouter)
	}
	return abrs
}

func (e *engine) applyTranslationsV6(db *ospflsdb.LSDB, self types.RouterID, desired map[[4]byte]nssaTranslationV6) {
	e.mu.Lock()
	prev := e.translations
	counter := e.mNSSATranslations
	redistLSIDs := make([]types.LinkStateID, 0, len(e.redistV6))
	for _, lsid := range e.redistV6 {
		redistLSIDs = append(redistLSIDs, lsid)
	}
	e.mu.Unlock()

	changed := false
	next := make(map[[4]byte]types.AreaID, len(desired))
	keep := make(map[ospflsdb.SelfLSARef]struct{}, len(desired)+len(redistLSIDs))
	for _, lsid := range redistLSIDs {
		keep[ospflsdb.SelfLSARef{Area: types.BackboneArea, Key: v6ExternalKey(self, lsid)}] = struct{}{}
	}
	for key, tr := range desired {
		if e.v6OriginateTranslatedExternal(self, tr.lsid, tr.body) {
			changed = true
		}
		if _, existed := prev[key]; !existed {
			counter.With(tr.area.String()).Inc()
		}
		next[key] = tr.area
		keep[ospflsdb.SelfLSARef{Area: types.BackboneArea, Key: v6ExternalKey(self, tr.lsid)}] = struct{}{}
	}
	if n := db.FlushStaleSelfLSAs(self, map[types.LSType]struct{}{types.LSType(ospfv3types.LSTypeASExternal): {}}, keep); n > 0 {
		changed = true
	}
	e.mu.Lock()
	e.translations = next
	e.mu.Unlock()
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, self)
	}
}

func (e *engine) v6OriginateTranslatedExternal(router types.RouterID, lsid types.LinkStateID, body ospfv3packet.ExternalLSA) bool {
	body.Prefix.Options &^= ospfv3types.OptPrefixP
	bodyBytes := make([]byte, body.EncodedLen())
	body.WriteTo(bodyBytes, 0)
	id := lsid
	b := body
	key := v6ExternalKey(router, lsid)
	_, ok := e.lsdb.OriginateSelf(types.BackboneArea, key, bodyBytes, func(seq types.LSSequenceNumber, purge bool) packet.LSA {
		return v6SelfLSA(ospfv3packet.LSA{
			Header:   v6OriginHeader(ospfv3types.LSTypeASExternal, ospfv3types.LinkStateID(id), router, seq, purge),
			External: &b,
		})
	})
	return ok
}
