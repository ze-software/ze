// Design: docs/architecture/plugin/rib-storage-design.md — attribute formatting for show commands
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_commands.go — command handling and JSON responses
// Related: rib_nlri.go — NLRI wire format helpers
// Related: bestpath.go — best-path selection (asPathLength, firstASInPath shared concern)
// Related: rib_pipeline.go — iterator pipeline for show commands
package rib

import (
	"encoding/hex"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// communityList wraps a typed community slice for lazy JSON marshaling.
// MarshalJSON produces a JSON array of quoted strings identical to
// json.Marshal([]string{...}) but without per-element string allocations.
type communityList []attribute.Community

func (cl communityList) MarshalJSON() ([]byte, error) {
	if len(cl) == 0 {
		return []byte("[]"), nil
	}
	buf := make([]byte, 0, len(cl)*12)
	buf = append(buf, '[')
	for i, c := range cl {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = c.AppendText(buf)
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf, nil
}

// largeCommunityList wraps a typed large community slice for lazy JSON marshaling.
type largeCommunityList []attribute.LargeCommunity

func (cl largeCommunityList) MarshalJSON() ([]byte, error) {
	if len(cl) == 0 {
		return []byte("[]"), nil
	}
	buf := make([]byte, 0, len(cl)*20)
	buf = append(buf, '[')
	for i, lc := range cl {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = lc.AppendText(buf)
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf, nil
}

// extCommunityList wraps a typed extended community slice for lazy JSON marshaling.
type extCommunityList []attribute.ExtendedCommunity

func (cl extCommunityList) MarshalJSON() ([]byte, error) {
	if len(cl) == 0 {
		return []byte("[]"), nil
	}
	buf := make([]byte, 0, len(cl)*20)
	buf = append(buf, '[')
	for i, ec := range cl {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = hex.AppendEncode(buf, ec[:])
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf, nil
}

// communityByteList wraps raw community pool bytes for lazy JSON marshaling.
// The byte slice must be a copy of the pool data (not a reference into pool
// storage) because json.Marshal may run after the pool shard lock is released.
type communityByteList []byte

func (cl communityByteList) MarshalJSON() ([]byte, error) {
	n := len(cl) / 4
	if n == 0 {
		return []byte("[]"), nil
	}
	buf := make([]byte, 0, n*12)
	buf = append(buf, '[')
	for i := 0; i+4 <= len(cl); i += 4 {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		c := uint32(cl[i])<<24 | uint32(cl[i+1])<<16 | uint32(cl[i+2])<<8 | uint32(cl[i+3])
		buf = attribute.Community(c).AppendText(buf)
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf, nil
}

// enrichRouteMapFromEntry adds path attributes from a pool-based RouteEntry to a route map.
// Only adds attributes that are present (valid handle) — missing attributes are omitted.
// Each attribute value is wrapped with RFC 4271 flag booleans.
func enrichRouteMapFromEntry(routeMap map[string]any, entry storage.RouteEntry) {
	if entry.StaleLevel > storage.StaleLevelFresh {
		routeMap["stale"] = true
		routeMap["stale-level"] = entry.StaleLevel
	}
	b := entry.GetBundle()
	if b.HasNextHop() {
		if data, err := pool.NextHop.Get(b.NextHop); err == nil {
			routeMap["next-hop"] = formatNextHop(data)
		}
	}
	if b.HasOrigin() {
		if data, err := pool.Origin.Get(b.Origin); err == nil {
			if origin := formatOrigin(data); origin != "" {
				routeMap["origin"] = attrWithFlags(origin, attribute.FlagTransitive)
			}
		}
	}
	if entry.HasASPath() {
		if data, err := pool.ASPath.Get(entry.ASPath); err == nil {
			if asPath := formatASPath(data); asPath != nil {
				routeMap["as-path"] = attrWithFlags(asPath, attribute.FlagTransitive)
			}
		}
	}
	if b.HasMED() {
		if data, err := pool.MED.Get(b.MED); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				routeMap["med"] = attrWithFlags(v, attribute.FlagOptional)
			}
		}
	}
	if b.HasLocalPref() {
		if data, err := pool.LocalPref.Get(b.LocalPref); err == nil {
			if v, ok := formatUint32Attr(data); ok {
				routeMap["local-preference"] = attrWithFlags(v, attribute.FlagTransitive)
			}
		}
	}
	if b.HasCommunities() {
		if data, err := pool.Communities.Get(b.Communities); err == nil {
			if len(data) >= 4 && len(data)%4 == 0 {
				cp := make([]byte, len(data))
				copy(cp, data)
				routeMap["community"] = attrWithFlags(communityByteList(cp), attribute.FlagOptional|attribute.FlagTransitive)
			}
		}
	}
}

// enrichRouteMapFromRoute adds path attributes from a Route (Adj-RIB-Out) to a route map.
// Only non-empty/non-nil attributes are added. Each value is wrapped with RFC 4271 flags.
func enrichRouteMapFromRoute(routeMap map[string]any, rt *Route) {
	if rt.Origin != nil {
		if s := rt.Origin.LowerString(); s != "" {
			routeMap["origin"] = attrWithFlags(s, attribute.FlagTransitive)
		}
	}
	if len(rt.ASPath) > 0 {
		routeMap["as-path"] = attrWithFlags(rt.ASPath, attribute.FlagTransitive)
	}
	if rt.MED != nil {
		routeMap["med"] = attrWithFlags(*rt.MED, attribute.FlagOptional)
	}
	if rt.LocalPreference != nil {
		routeMap["local-preference"] = attrWithFlags(*rt.LocalPreference, attribute.FlagTransitive)
	}
	if len(rt.Communities) > 0 {
		routeMap["community"] = attrWithFlags(communityList(rt.Communities), attribute.FlagOptional|attribute.FlagTransitive)
	}
	if len(rt.LargeCommunities) > 0 {
		routeMap["large-community"] = attrWithFlags(largeCommunityList(rt.LargeCommunities), attribute.FlagOptional|attribute.FlagTransitive)
	}
	if len(rt.ExtendedCommunities) > 0 {
		routeMap["extended-community"] = attrWithFlags(extCommunityList(rt.ExtendedCommunities), attribute.FlagOptional|attribute.FlagTransitive)
	}
}

// attrWithFlags wraps an attribute value with RFC 4271 flag booleans.
func attrWithFlags(value any, flags attribute.AttributeFlags) map[string]any {
	return map[string]any{
		"value":      value,
		"optional":   flags.IsOptional(),
		"transitive": flags.IsTransitive(),
		"partial":    flags.IsPartial(),
	}
}

// originNames maps ORIGIN values to RFC 4271 names.
var originNames = map[byte]string{
	0: "igp",
	1: "egp",
	2: "incomplete",
}

// formatOrigin converts raw ORIGIN pool bytes to RFC 4271 name.
// ORIGIN is 1 byte: 0=IGP, 1=EGP, 2=INCOMPLETE.
func formatOrigin(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	if name, ok := originNames[data[0]]; ok {
		return name
	}
	var b textbuf.Buffer
	return b.Reset().Str("unknown(").Uint8(data[0]).Byte(')').String()
}

// formatASPath converts raw AS_PATH pool bytes to a flat ASN slice.
// RFC 4271 Section 4.3b: segments are [type(1)][count(1)][ASN(4)*count].
// AS_SEQUENCE (type 2) and AS_SET (type 1) are both flattened.
func formatASPath(data []byte) []uint32 {
	if len(data) == 0 {
		return nil
	}
	var result []uint32
	offset := 0
	for offset+2 <= len(data) {
		// segType := data[offset] — not needed for flat list
		count := int(data[offset+1])
		offset += 2
		for range count {
			if offset+4 > len(data) {
				return nil // truncated data — don't return partial results
			}
			asn := uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
				uint32(data[offset+2])<<8 | uint32(data[offset+3])
			result = append(result, asn)
			offset += 4
		}
	}
	return result
}

// formatUint32Attr converts 4 big-endian bytes to uint32.
// Used for MED (type 4) and LOCAL_PREF (type 5).
func formatUint32Attr(data []byte) (uint32, bool) {
	if len(data) < 4 {
		return 0, false
	}
	v := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	return v, true
}

// formatCommunities converts raw COMMUNITIES pool bytes to display strings.
// RFC 1997: each community is 4 bytes. Uses attribute.Community.String()
// which resolves well-known names (no-export, no-advertise, etc.).
func formatCommunities(data []byte) []string {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	result := make([]string, 0, len(data)/4)
	for i := 0; i+4 <= len(data); i += 4 {
		c := attribute.CommunityFrom4([4]byte(data[i : i+4]))
		result = append(result, c.String())
	}
	return result
}
