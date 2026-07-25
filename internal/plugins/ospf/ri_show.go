// Design: plan/learned/1031-ospf-ext-3-router-information.md -- `show ospf database router-information`.
// RFC: rfc/short/rfc7770.md -- sec 2.4 (Informational Capabilities TLV), sec 2.5 (bit names),
// sec 3 (smallest Instance ID wins for an unspecified-multi-instance TLV).
//
// The RI database view decodes the stored RI LSA bodies (OSPFv2 opaque type 4 + OSPFv3
// function code 12) into capability bits + a TLV list, for both address families. It reads
// the LSDB (the authoritative store), never a separate cache, so a received RI LSA is
// rendered as stored (AC-13). The TLV walk is the bound-checked ext-1 iterator, so a
// truncated/malformed body renders what it can and never crashes (AC-14). Multiple instances
// of the same (af, scope, area, router) collapse to the smallest Instance ID for the
// effective capabilities (RFC 7770 sec 3), while every instance's TLVs are still listed.

package ospf

import (
	"sort"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// riInfoBitNames maps the RFC 7770 sec 2.5 informational capability bit indices to stable
// display names (bits 0-5; bits 6-31 are unassigned).
var riInfoBitNames = map[uint]string{
	packet.RIInfoBitGracefulRestart:       "graceful-restart-capable",
	packet.RIInfoBitGracefulRestartHelper: "graceful-restart-helper",
	packet.RIInfoBitStubRouter:            "stub-router",
	packet.RIInfoBitTrafficEngineering:    "traffic-engineering",
	4:                                     "point-to-point-over-lan",
	5:                                     "experimental-te",
}

// riObservation is one RI LSA read from an LSDB store, address-family neutral, ready to be
// grouped and decoded for display.
type riObservation struct {
	af        string
	scope     string
	area      string
	advRouter string
	instance  uint32
	body      []byte
}

// riDatabaseView is the `show ospf database router-information` payload: one entry per
// (af, scope, area, advertising-router) group.
type riDatabaseView struct {
	RouterInformation []riRouterEntry `json:"router-information"`
}

// riRouterEntry is one router's RI advertisement in one scope/area/af: the effective
// capabilities (from the smallest Instance ID, RFC 7770 sec 3) plus every instance's TLVs.
type riRouterEntry struct {
	AF                string          `json:"af"`
	Scope             string          `json:"scope"`
	Area              string          `json:"area,omitempty"`
	AdvertisingRouter string          `json:"advertising-router"`
	EffectiveInstance uint32          `json:"effective-instance"`
	Capabilities      []string        `json:"informational-capabilities"`
	Instances         []riInstanceRow `json:"instances"`
}

// riInstanceRow is one RI LSA instance's decoded TLV list.
type riInstanceRow struct {
	Instance  uint32     `json:"instance"`
	TLVs      []riTLVRow `json:"tlvs"`
	Malformed bool       `json:"malformed,omitempty"`
}

// riTLVRow is one decoded RI TLV: its type, value length (padding excluded), and, for the
// recognized capability TLVs, a name.
type riTLVRow struct {
	Type   uint16 `json:"type"`
	Length int    `json:"length"`
	Name   string `json:"name,omitempty"`
}

// riDatabaseSnapshot renders `show ospf database router-information` for both address
// families: OSPFv2 RI opaque LSAs (Opaque type 4) from the IPv4 engine and OSPFv3 native RI
// LSAs (function code 12) from the IPv6 engine. It returns the single-element wrapping the
// other `show ospf ...` commands use.
func riDatabaseSnapshot(v4, v6 *engine) []any {
	var obs []riObservation
	if v4 != nil && v4.lsdb != nil {
		for _, v := range v4.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
			obs = append(obs, riObservation{
				af: "v2", scope: OpaqueScope(v.Scope).String(), area: riAreaLabel(OpaqueScope(v.Scope), v.Area),
				advRouter: v.AdvertisingRouter.String(), instance: v.OpaqueID, body: v.Body,
			})
		}
	}
	if v6 != nil && v6.lsdb != nil {
		for _, sc := range riV3Scopes {
			for _, v := range v6.lsdb.LSAViewsByType(sc.typ) {
				obs = append(obs, riObsFromNativeView(v, sc.scope))
			}
		}
	}
	return []any{buildRIView(obs)}
}

// riObsFromNativeView converts a stored OSPFv3 RI LSA (function code 12) into a display
// observation. RFC 7770 sec 2.2: the 32-bit Link State ID is the Instance ID.
func riObsFromNativeView(v ospflsdb.NativeLSAView, scope OpaqueScope) riObservation {
	return riObservation{
		af:        "v3",
		scope:     scope.String(),
		area:      riAreaLabel(scope, v.Area),
		advRouter: v.AdvertisingRouter.String(),
		instance:  riInstanceOf(v.LinkStateID),
		body:      v.Body,
	}
}

// riAreaLabel returns the area string for an area-scoped RI LSA; AS/link-scoped LSAs carry no
// meaningful area, so the field is left empty (omitted from the JSON).
func riAreaLabel(scope OpaqueScope, area types.AreaID) string {
	if scope == OpaqueScopeArea {
		return area.String()
	}
	return ""
}

// riInstanceOf returns the RI LSA Instance ID from an OSPFv3 RI LSA's 32-bit Link State ID
// (RFC 7770 sec 2.2: the whole Link State ID is the Instance ID).
func riInstanceOf(id types.LinkStateID) uint32 {
	return uint32(id[0])<<24 | uint32(id[1])<<16 | uint32(id[2])<<8 | uint32(id[3])
}

// riGroupKey groups RI observations for the same advertisement (RFC 7770 sec 3 multi-instance
// applies within one af/scope/area/router).
type riGroupKey struct {
	af, scope, area, advRouter string
}

// buildRIView groups observations and, per group, selects the smallest Instance ID for the
// effective capabilities (RFC 7770 sec 3) while listing every instance's TLVs. Groups and
// instances are sorted for a stable render.
func buildRIView(obs []riObservation) riDatabaseView {
	groups := map[riGroupKey][]riObservation{}
	for _, o := range obs {
		k := riGroupKey{af: o.af, scope: o.scope, area: o.area, advRouter: o.advRouter}
		groups[k] = append(groups[k], o)
	}
	view := riDatabaseView{}
	for k, list := range groups {
		sort.Slice(list, func(i, j int) bool { return list[i].instance < list[j].instance })
		entry := riRouterEntry{AF: k.af, Scope: k.scope, Area: k.area, AdvertisingRouter: k.advRouter}
		entry.EffectiveInstance = list[0].instance
		// RFC 7770 sec 3: the numerically smallest Instance ID wins for the informational
		// capabilities (an unspecified-multi-instance TLV); later instances are ignored for it.
		effCaps, _, _ := decodeRIBody(list[0].body)
		entry.Capabilities = riCapabilityNames(effCaps)
		for _, o := range list {
			_, tlvs, malformed := decodeRIBody(o.body)
			entry.Instances = append(entry.Instances, riInstanceRow{Instance: o.instance, TLVs: tlvs, Malformed: malformed})
		}
		view.RouterInformation = append(view.RouterInformation, entry)
	}
	sort.Slice(view.RouterInformation, func(i, j int) bool {
		a, b := view.RouterInformation[i], view.RouterInformation[j]
		if a.AF != b.AF {
			return a.AF < b.AF
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.Area != b.Area {
			return a.Area < b.Area
		}
		return a.AdvertisingRouter < b.AdvertisingRouter
	})
	return view
}

// decodeRIBody walks an RI LSA body's TLV stream (RFC 7770 sec 2.3) with the bound-checked
// iterator. It returns the informational capability word (from the first type-1 TLV), the
// decoded TLV rows, and whether the stream was malformed/truncated (AC-14: it never panics
// and returns what it could decode).
func decodeRIBody(body []byte) (infoCaps uint32, tlvs []riTLVRow, malformed bool) {
	decoded, err := packet.DecodeRITLVStream(body)
	for _, tlv := range decoded {
		row := riTLVRow{Type: tlv.Type, Length: len(tlv.Value)}
		switch tlv.Type {
		case packet.RITLVInformationalCapabilities:
			infoCaps = packet.RIReadCapabilities(tlv.Value)
			row.Name = "informational-capabilities"
		case packet.RITLVFunctionalCapabilities:
			row.Name = "functional-capabilities"
		}
		tlvs = append(tlvs, row)
	}
	return infoCaps, tlvs, err != nil
}

// riCapabilityNames decodes the informational capability word into the RFC 7770 sec 2.5 bit
// names that are set, in ascending bit order.
func riCapabilityNames(field uint32) []string {
	var out []string
	for bit := range uint(6) {
		if field&packet.RIInfoBitMask(bit) != 0 {
			out = append(out, riInfoBitNames[bit])
		}
	}
	return out
}
