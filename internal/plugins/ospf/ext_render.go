// Design: plan/learned/1039-ospf-ext-4-extended-link-prefix.md -- decoded `show ospf database
// opaque-area` / `opaque-as` view of the RFC 7684 Extended Prefix/Link Opaque LSAs.
// RFC: rfc/short/rfc7684.md -- sec 2.1 (Extended Prefix TLV), sec 3.1 (Extended Link TLV).
//
// This decorates the generic opaque-scope database view so Extended Prefix (Opaque Type 7) and
// Extended Link (Opaque Type 8) bodies render decoded (Route Type, prefix, flags, Link
// Type/ID/Data, sub-TLVs) rather than as raw hex (AC-15). It reads the LSDB (the authoritative
// store, self + received) and decodes on demand; a malformed body is left to the generic hex
// view. The resolved-attribute section reflects the receive-side dedup (lowest Opaque ID) and
// RFC 5250 sec 5 Type-11 usability. IPv4 rendering goes through netip (no fmt on the render path).

package ospf

import (
	"encoding/hex"
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// extSubTLVRow is one decoded sub-TLV: its type, value length (padding excluded), and either a
// registered codec's display string or the raw hex fallback.
type extSubTLVRow struct {
	Type   uint16 `json:"type"`
	Length int    `json:"length"`
	Value  string `json:"value,omitempty"`
	Hex    string `json:"hex,omitempty"`
}

// extPrefixRow is one decoded Extended Prefix TLV for display.
type extPrefixRow struct {
	RouteType string         `json:"route-type"`
	Prefix    string         `json:"prefix"`
	Flags     []string       `json:"flags,omitempty"`
	SubTLVs   []extSubTLVRow `json:"sub-tlvs,omitempty"`
}

// extPrefixDecodedLSA is one stored Extended Prefix Opaque LSA decoded for display.
type extPrefixDecodedLSA struct {
	AdvertisingRouter string         `json:"advertising-router"`
	Scope             string         `json:"scope"`
	OpaqueID          uint32         `json:"opaque-id"`
	Prefixes          []extPrefixRow `json:"prefixes"`
	PrefixRanges      int            `json:"prefix-ranges,omitempty"`
}

// extLinkDecodedLSA is one stored Extended Link Opaque LSA decoded for display.
type extLinkDecodedLSA struct {
	AdvertisingRouter string         `json:"advertising-router"`
	Scope             string         `json:"scope"`
	OpaqueID          uint32         `json:"opaque-id"`
	LinkType          string         `json:"link-type"`
	LinkID            string         `json:"link-id"`
	LinkData          string         `json:"link-data"`
	SubTLVs           []extSubTLVRow `json:"sub-tlvs,omitempty"`
}

// extResolvedRow is one resolved received-prefix attribute set: the winning Opaque ID (RFC
// 7684 sec 2 lowest-Opaque-ID dedup) and RFC 5250 sec 5 usability.
type extResolvedRow struct {
	AdvertisingRouter string `json:"advertising-router"`
	Prefix            string `json:"prefix"`
	RouteType         string `json:"route-type"`
	OpaqueID          uint32 `json:"opaque-id"`
	Usable            bool   `json:"usable"`
}

// extDecodedDatabase is the decoded Extended Prefix/Link section appended to a `show ospf
// database opaque-area` / `opaque-as` response.
type extDecodedDatabase struct {
	ExtendedPrefix   []extPrefixDecodedLSA `json:"extended-prefix,omitempty"`
	ExtendedLink     []extLinkDecodedLSA   `json:"extended-link,omitempty"`
	ResolvedPrefixes []extResolvedRow      `json:"resolved-prefixes,omitempty"`
}

// appendExtOpaqueDecode appends the decoded Extended Prefix/Link view for the scope to a
// `show ospf database opaque-*` response, when any Extended LSA is present (AC-15).
func (e *engine) appendExtOpaqueDecode(out []any, scope OpaqueScope) []any {
	decoded := e.extOpaqueDecode(scope)
	if len(decoded.ExtendedPrefix) > 0 || len(decoded.ExtendedLink) > 0 || len(decoded.ResolvedPrefixes) > 0 {
		out = append(out, decoded)
	}
	return out
}

// extOpaqueDecode decodes every stored Extended Prefix/Link Opaque LSA of the given scope, and
// gathers the resolved received-prefix attribute set for that scope.
func (e *engine) extOpaqueDecode(scope OpaqueScope) extDecodedDatabase {
	var db extDecodedDatabase
	if e.lsdb == nil {
		return db
	}
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.ExtPrefixOpaqueType) {
		if OpaqueScope(v.Scope) != scope {
			continue
		}
		lsa, err := packet.DecodeExtPrefixLSA(v.Body)
		if err != nil {
			continue // malformed: leave to the generic hex view
		}
		row := extPrefixDecodedLSA{
			AdvertisingRouter: v.AdvertisingRouter.String(),
			Scope:             OpaqueScope(v.Scope).String(),
			OpaqueID:          v.OpaqueID,
			PrefixRanges:      len(lsa.Ranges),
		}
		for i := range lsa.Prefixes {
			row.Prefixes = append(row.Prefixes, extPrefixRowFrom(lsa.Prefixes[i]))
		}
		db.ExtendedPrefix = append(db.ExtendedPrefix, row)
	}
	for _, v := range e.lsdb.OpaqueLSAsByType(packet.ExtLinkOpaqueType) {
		if OpaqueScope(v.Scope) != scope {
			continue
		}
		lsa, err := packet.DecodeExtLinkLSA(v.Body)
		if err != nil || !lsa.HasLink {
			continue
		}
		db.ExtendedLink = append(db.ExtendedLink, extLinkDecodedLSAFrom(lsa, v))
	}
	db.ResolvedPrefixes = e.extRecv.snapshot(scope)
	return db
}

// extLinkDecodedLSAFrom flattens a decoded Extended Link Opaque LSA into a display row with
// its stored identity (advertising router, scope, Opaque ID).
func extLinkDecodedLSAFrom(lsa packet.ExtLinkLSA, v ospflsdb.OpaqueLSAView) extLinkDecodedLSA {
	return extLinkDecodedLSA{
		AdvertisingRouter: v.AdvertisingRouter.String(),
		Scope:             OpaqueScope(v.Scope).String(),
		OpaqueID:          v.OpaqueID,
		LinkType:          extLinkTypeString(lsa.Link.LinkType),
		LinkID:            ipv4Str(lsa.Link.LinkID),
		LinkData:          ipv4Str(lsa.Link.LinkData),
		SubTLVs:           extSubTLVRows(linkSubTLVs, lsa.Link.SubTLVs),
	}
}

// extPrefixRowFrom flattens a decoded Extended Prefix TLV into a display row, normalizing the
// N-Flag (ignored on a non-host prefix, RFC 7684 sec 2.1) and rendering sub-TLVs.
func extPrefixRowFrom(p packet.ExtPrefixTLV) extPrefixRow {
	return extPrefixRow{
		RouteType: extRouteTypeString(p.RouteType),
		Prefix:    extPrefixString(p),
		Flags:     extFlagNames(extNormalizeFlags(p)),
		SubTLVs:   extSubTLVRows(prefixSubTLVs, p.SubTLVs),
	}
}

// extSubTLVRows renders decoded sub-TLVs: a registered codec's string when present, else the
// raw type/length/hex (RFC 7684 sec 2 forward-compatibility: unknown sub-TLVs are shown, not
// dropped).
func extSubTLVRows(reg map[uint16]extSubTLVCodec, subs []packet.ExtSubTLV) []extSubTLVRow {
	if len(subs) == 0 {
		return nil
	}
	out := make([]extSubTLVRow, 0, len(subs))
	for _, s := range subs {
		row := extSubTLVRow{Type: s.Type, Length: len(s.Value)}
		if str := renderExtSubTLV(reg, s); str != "" {
			row.Value = str
		} else {
			row.Hex = hex.EncodeToString(s.Value)
		}
		out = append(out, row)
	}
	return out
}

// extFlagNames returns the set RFC 7684 sec 2.1 flag names (A/N) in a stable order.
func extFlagNames(flags uint8) []string {
	var out []string
	if flags&packet.ExtPrefixFlagA != 0 {
		out = append(out, "attach")
	}
	if flags&packet.ExtPrefixFlagN != 0 {
		out = append(out, "node")
	}
	return out
}

// RFC 7684 sec 2.1 Route Type display names (stable strings for the decoded show/JSON view).
const (
	extRouteNameUnspecified  = "unspecified"
	extRouteNameIntraArea    = "intra-area"
	extRouteNameInterArea    = "inter-area"
	extRouteNameASExternal   = "as-external"
	extRouteNameNSSAExternal = "nssa-external"
)

// extRouteTypeString maps an RFC 7684 sec 2.1 Route Type to a stable display name.
func extRouteTypeString(rt uint8) string {
	switch rt {
	case packet.ExtRouteTypeUnspecified:
		return extRouteNameUnspecified
	case packet.ExtRouteTypeIntraArea:
		return extRouteNameIntraArea
	case packet.ExtRouteTypeInterArea:
		return extRouteNameInterArea
	case packet.ExtRouteTypeASExternal:
		return extRouteNameASExternal
	case packet.ExtRouteTypeNSSAExternal:
		return extRouteNameNSSAExternal
	default:
		return unknownInterface
	}
}

// extLinkTypeString maps an RFC 2328 A.4.2 Link Type to a stable display name.
func extLinkTypeString(lt uint8) string {
	switch lt {
	case packet.RouterLinkTypeP2P:
		return networkPointToPoint
	case packet.RouterLinkTypeTransit:
		return "transit"
	case packet.RouterLinkTypeStub:
		return "stub"
	case packet.RouterLinkTypeVirtual:
		return "virtual"
	default:
		return unknownInterface
	}
}

// snapshot returns the resolved received-prefix attribute rows for the given scope, sorted for
// a stable render.
func (r *extReceiver) snapshot(scope OpaqueScope) []extResolvedRow {
	r.mu.Lock()
	rows := make([]extResolvedRow, 0, len(r.prefixes))
	for k, e := range r.prefixes {
		if e.scope != scope {
			continue
		}
		rows = append(rows, extResolvedRow{
			AdvertisingRouter: k.adv.String(),
			Prefix:            resolvedPrefixString(k.prefix),
			RouteType:         extRouteTypeString(e.routeType),
			OpaqueID:          e.opaqueID,
			Usable:            e.usable,
		})
	}
	r.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AdvertisingRouter != rows[j].AdvertisingRouter {
			return rows[i].AdvertisingRouter < rows[j].AdvertisingRouter
		}
		return rows[i].Prefix < rows[j].Prefix
	})
	return rows
}
