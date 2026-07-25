// Design: plan/learned/965-ospf-11-stub-nssa.md -- engine NSSA default-route origination +
// translator election and Type 7 -> Type 5 translation.
// Related: internal/plugins/ospf/lsdb -- the Type 7 originator (OriginateNSSA).
// RFC: rfc/short/rfc3101.md -- sec 2.3 NSSA default; sec 3.5 translator election;
// sec 3.6 Type 7 -> Type 5 translation

package ospf

import (
	"bytes"
	"maps"
	"time"

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

// applyNSSADefaults originates or withdraws the per-area NSSA Type 7 default route
// (0.0.0.0/0) according to each attached NSSA's `default-originate` config. RFC 3101
// sec 2.3: an NSSA ABR may inject a Type 7 default so internal routers reach external
// destinations via the ABR; it carries P=0 (a default is never translated to Type 5)
// and the area `default-cost` as its metric. Only an ABR (a router that can reach the
// backbone) originates it. The redistribute path routes 0.0.0.0/0 to the Type 5
// coordinator, so the NSSA Type 7 default has a single owner here and is purged freely
// when the condition lapses.
func (e *engine) applyNSSADefaults() {
	e.nssaMu.Lock()
	defer e.nssaMu.Unlock()
	e.mu.Lock()
	cfg := e.cfg
	db := e.lsdb
	e.mu.Unlock()
	if db == nil || cfg.RouterID == (types.RouterID{}) {
		return
	}
	nssas, isABR := e.externalScope()
	attached := make(map[types.AreaID]bool, len(nssas))
	faByArea := make(map[types.AreaID][4]byte, len(nssas))
	for _, n := range nssas {
		attached[n.area] = true
		faByArea[n.area] = n.fa
	}
	changed := false
	for _, a := range cfg.Areas {
		if a.AreaType != areaTypeNSSA || !attached[a.AreaID] {
			continue
		}
		// RFC 3101 §2.3: a totally-stubby NSSA (no Summary-LSAs imported) leaves its internal
		// routers with no other path to AS-external destinations, so its ABR MUST originate a
		// Type 7 default automatically. For a regular NSSA the Type 7 default stays operator-
		// configurable (`default-originate`). The forwarding address is this ABR's intra-NSSA
		// interface address (RFC 3101 §2.3): a Type 7 with a zero FA is not usable for an
		// internal router's route calculation, so the default needs the same non-zero FA as
		// the redistributed Type 7s.
		if isABR && (a.NoSummary || a.NSSADefaultOriginate) {
			fa := faByArea[a.AreaID]
			if _, c := db.OriginateNSSA(a.AreaID, cfg.RouterID, [4]byte{}, [4]byte{}, false, a.DefaultCost, fa, 0, false); c {
				changed = true
			}
		} else if db.PurgeNSSA(a.AreaID, cfg.RouterID, [4]byte{}) {
			changed = true
		}
	}
	if changed {
		e.originateSelfLSAs()
		e.refreshExternalMetrics(db, cfg.RouterID)
	}
}

// electNSSATranslator reports whether self is the elected Type 7 -> Type 5 translator
// among the NSSA's ABRs. RFC 3101 Section 3.5: role `never` never translates; `always`
// always translates; `candidate` translates iff self has the highest Router ID among
// the candidate ABRs (the higher Router ID wins, analogous to DR election).
func electNSSATranslator(self types.RouterID, role string, abrs []types.RouterID) bool {
	switch role {
	case translateRoleNever:
		return false
	case translateRoleAlways:
		return true
	}
	for _, r := range abrs {
		if bytes.Compare(r[:], self[:]) > 0 {
			return false
		}
	}
	return true
}

// nssaABRs returns the Router IDs of the translator-candidate ABRs currently present in
// the NSSA area's link-state database -- the translator-election candidate set. RFC 3101
// §3.5: only routers advertising the Nt-bit are candidates, so a higher-Router-ID ABR
// configured `translate never` (Nt clear) does not wedge translation off for a willing
// lower-Router-ID candidate. The B-bit is also required (a translator is an ABR).
func (e *engine) nssaABRs(db *ospflsdb.LSDB, area types.AreaID) []types.RouterID {
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
		if err != nil || body.Flags&packet.RouterFlagB == 0 || body.Flags&packet.RouterFlagNt == 0 {
			continue
		}
		abrs = append(abrs, h.AdvertisingRouter)
	}
	return abrs
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
			if a.AreaType == areaTypeNSSA {
				policyByArea[a.AreaID] = areaPolicy{role: a.NSSATranslateRole, stability: time.Duration(a.NSSAStabilityInterval) * time.Second}
			}
		}
		for _, n := range nssas {
			p := policyByArea[n.area]
			elected := electNSSATranslator(self, p.role, e.nssaABRs(db, n.area))
			if !e.translatorEffective(n.area, elected, now, p.stability) {
				continue
			}
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
				// RFC 3101 §3.6: when an equivalent Type 5 for this network is already advertised
				// by a translator with a higher Router ID, suppress our translation so only the
				// highest-Router-ID translator injects the Type 5 (no duplicate, even while a
				// deposed translator's stability grace overlaps the newly-elected one).
				if db.HigherRIDType5Exists(network, self) {
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
			if a.AreaType == areaTypeNSSA {
				policyByArea[a.AreaID] = areaPolicy{role: a.NSSATranslateRole, stability: time.Duration(a.NSSAStabilityInterval) * time.Second}
			}
		}
		for _, n := range nssas {
			p := policyByArea[n.area]
			elected := electNSSATranslator(self, p.role, e.nssaABRsV6(db, n.area))
			if !e.translatorEffective(n.area, elected, now, p.stability) {
				continue
			}
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
				lsid := h.LinkStateID
				if redist[[4]byte(lsid)] {
					continue // RFC 3101 §3.6: keep the locally-redistributed Type 5; do not translate
				}
				if db.HigherRIDType5LSIDExists(types.LSType(ospfv3types.LSTypeASExternal), lsid, self) {
					continue
				}
				body.Prefix.Options &^= ospfv3types.OptPrefixP
				desired[[4]byte(lsid)] = nssaTranslationV6{lsid: lsid, body: body, area: n.area}
			}
		}
	}
	e.applyTranslationsV6(db, self, desired)
}

func (e *engine) nssaABRsV6(db *ospflsdb.LSDB, area types.AreaID) []types.RouterID {
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
		if err != nil || body.Flags&ospfv3packet.RouterFlagB == 0 || body.Flags&ospfv3packet.RouterFlagNt == 0 || !body.Options.NSSA() {
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
