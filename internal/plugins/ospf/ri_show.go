// Design: docs/architecture/ospf/ospf-ext-3-router-information.md -- `show ospf database router-information`.
// RFC: rfc/short/rfc7770.md -- sec 2.4 (Informational Capabilities TLV), sec 2.5 (bit names),
// sec 3 (smallest Instance ID wins for an unspecified-multi-instance TLV).
//
// The RI database view decodes the stored RI LSA bodies (OSPFv2 opaque type 4 + OSPFv3
// function code 12) into capability bits + a TLV list, for both address families. It reads
// the LSDB (the authoritative store), never a separate cache, so a received RI LSA is
// rendered as stored (AC-13). The TLV walk is the bound-checked ext-1 iterator, so a
// truncated/malformed body renders what it can and never crashes (AC-14). Multiple instances
// of the same (af, OSPFv3 instance, scope, area, router) collapse to the smallest RI Instance
// ID for the effective capabilities (RFC 7770 sec 3), while every instance's TLVs are still
// listed. The OSPFv3 part reads every running RFC 5838 address-family engine, or the one the
// address-family keyword names, and each OSPFv3 entry names its address-family and
// instance-id (docs/architecture/ospf/ospf-ext-15-multi-af.md).

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
// grouped and decoded for display. ospfv3 is the OSPFv3 instance whose LSDB held it, nil for
// an OSPFv2 LSA.
type riObservation struct {
	af        string
	ospfv3    *riOSPFv3Instance
	scope     string
	area      string
	advRouter string
	instance  uint32
	body      []byte
}

// riOSPFv3Instance names the RFC 5838 address-family instance an OSPFv3 RI entry came from.
// Its instance-id is the OSPFv3 Instance ID of that address family, not the RI LSA Instance
// ID of RFC 7770, which the entry carries as effective-instance and instances.
type riOSPFv3Instance struct {
	AddressFamily string `json:"address-family"`
	InstanceID    uint8  `json:"instance-id"`
}

// riDatabaseView is the payload of both RI views: one entry per (af, OSPFv3 instance, scope,
// area, advertising-router) group.
type riDatabaseView struct {
	RouterInformation []riRouterEntry `json:"router-information"`
}

// riRouterEntry is one router's RI advertisement in one scope/area/af: the effective
// capabilities (from the smallest Instance ID, RFC 7770 sec 3) plus every instance's TLVs.
// An OSPFv3 entry follows af with its address-family and instance-id; the embedded pointer
// is nil for an OSPFv2 entry, so encoding/json writes neither key there.
type riRouterEntry struct {
	AF string `json:"af"`
	*riOSPFv3Instance
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

// routerInformationView answers `show ospf database router-information`: the OSPFv2 RI
// opaque LSAs of the base engine v2, whole, and the OSPFv3 RI LSAs of each address-family
// engine cmdArgs selects. The address-family keyword narrows only the OSPFv3 part.
func routerInformationView(set *v6EngineSet, v2 *engine, command string, cmdArgs []string) (any, error) {
	instances, err := set.selectInstances(command, cmdArgs)
	if err != nil {
		return nil, err
	}
	return riDatabaseSnapshot(v2, instances), nil
}

// v3RouterInformationView answers `show ospf ipv6 database router-information`: the OSPFv3
// RI LSAs of each address-family engine cmdArgs selects, and no OSPFv2 LSA.
func v3RouterInformationView(set *v6EngineSet, _ *engine, command string, cmdArgs []string) (any, error) {
	instances, err := set.selectInstances(command, cmdArgs)
	if err != nil {
		return nil, err
	}
	return riDatabaseSnapshot(nil, instances), nil
}

// riDatabaseSnapshot renders the RI database view: the OSPFv2 RI opaque LSAs (Opaque type 4)
// of the IPv4 engine v4 when v4 is not nil, and the OSPFv3 native RI LSAs (function code 12)
// of each OSPFv3 address-family instance in v3, each entry naming its instance. It returns
// the single-element wrapping the other `show ospf ...` commands use.
func riDatabaseSnapshot(v4 *engine, v3 []afInstance) []any {
	var obs []riObservation
	if v4 != nil && v4.lsdb != nil {
		for _, v := range v4.lsdb.OpaqueLSAsByType(packet.RIOpaqueType) {
			obs = append(obs, riObservation{
				af: "v2", scope: OpaqueScope(v.Scope).String(), area: riAreaLabel(OpaqueScope(v.Scope), v.Area),
				advRouter: v.AdvertisingRouter.String(), instance: v.OpaqueID, body: v.Body,
			})
		}
	}
	for _, inst := range v3 {
		if inst.eng.lsdb == nil {
			continue
		}
		// RFC 5838 Section 2: each Instance ID has its own link state database, so the RI
		// LSAs of one address family are read from its engine alone and named after it.
		identity := &riOSPFv3Instance{AddressFamily: inst.af.String(), InstanceID: inst.instanceID}
		for _, sc := range riV3Scopes {
			for _, v := range inst.eng.lsdb.LSAViewsByType(sc.typ) {
				obs = append(obs, riObsFromNativeView(v, sc.scope, identity))
			}
		}
	}
	return []any{buildRIView(obs)}
}

// riObsFromNativeView converts a stored OSPFv3 RI LSA (function code 12), read from the LSDB
// of the instance identity names, into a display observation. RFC 7770 sec 2.2: the 32-bit
// Link State ID is the Instance ID.
func riObsFromNativeView(v ospflsdb.NativeLSAView, scope OpaqueScope, identity *riOSPFv3Instance) riObservation {
	return riObservation{
		af:        "v3",
		ospfv3:    identity,
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
// applies within one af/OSPFv3 instance/scope/area/router). ospfv3 is the zero value for
// every OSPFv2 observation, which belongs to no OSPFv3 instance: it only groups and orders,
// is never rendered, and cannot collide with an OSPFv3 instance, whose address-family is
// never empty.
type riGroupKey struct {
	af          string
	ospfv3      riOSPFv3Instance
	scope, area string
	advRouter   string
}

// riGroupKeyOf answers the advertisement o belongs to.
func riGroupKeyOf(o riObservation) riGroupKey {
	k := riGroupKey{af: o.af, scope: o.scope, area: o.area, advRouter: o.advRouter}
	if o.ospfv3 != nil {
		k.ospfv3 = *o.ospfv3
	}
	return k
}

// riGroupKeyLess orders RI groups for a stable render: by af, then OSPFv3 Instance ID (RFC
// 5838 range order, so ipv6-unicast first), then address family, scope, area and router.
// The address family follows the Instance ID so the order is total over every field of the
// key, whatever Instance IDs the families carry. OSPFv2 groups all carry the zero OSPFv3
// identity, so among them the order falls through to scope, area and router.
func riGroupKeyLess(a, b riGroupKey) bool {
	if a.af != b.af {
		return a.af < b.af
	}
	if a.ospfv3.InstanceID != b.ospfv3.InstanceID {
		return a.ospfv3.InstanceID < b.ospfv3.InstanceID
	}
	if a.ospfv3.AddressFamily != b.ospfv3.AddressFamily {
		return a.ospfv3.AddressFamily < b.ospfv3.AddressFamily
	}
	if a.scope != b.scope {
		return a.scope < b.scope
	}
	if a.area != b.area {
		return a.area < b.area
	}
	return a.advRouter < b.advRouter
}

// buildRIView groups observations and, per group, selects the smallest Instance ID for the
// effective capabilities (RFC 7770 sec 3) while listing every instance's TLVs. Groups are
// rendered in riGroupKeyLess order and instances by Instance ID.
func buildRIView(obs []riObservation) riDatabaseView {
	groups := map[riGroupKey][]riObservation{}
	for _, o := range obs {
		k := riGroupKeyOf(o)
		groups[k] = append(groups[k], o)
	}
	keys := make([]riGroupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return riGroupKeyLess(keys[i], keys[j]) })
	view := riDatabaseView{}
	for _, k := range keys {
		list := groups[k]
		sort.Slice(list, func(i, j int) bool { return list[i].instance < list[j].instance })
		entry := riRouterEntry{
			AF: k.af, riOSPFv3Instance: list[0].ospfv3,
			Scope: k.scope, Area: k.area, AdvertisingRouter: k.advRouter,
		}
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
