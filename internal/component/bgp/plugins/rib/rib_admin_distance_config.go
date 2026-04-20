// Design: docs/architecture/plugin/rib-storage-design.md — admin-distance config extraction
// Related: rib.go — RIBManager state and OnConfigure wiring
// Related: rib_bestchange.go — checkBestPathChange stamps the configured distances on locrib mirrors
// Related: rib_multipath_config.go — sibling extractor pattern

package rib

import (
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
)

// extractAdminDistanceConfig extracts the bgp/admin-distance container
// from a Stage 2 config section. Returns the configured eBGP and iBGP
// distances (bounded to 1..255 per YANG).
//
// Returns (0, 0) when no admin-distance block is present, or the fields
// are out of range, so callers retain their existing defaults (20 / 200
// per Cisco/Juniper convention).
func extractAdminDistanceConfig(jsonStr string) (ebgp, ibgp uint8) {
	bgp, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return 0, 0
	}
	ad, ok := bgp["admin-distance"].(map[string]any)
	if !ok {
		return 0, 0
	}
	return extractAdminDistanceField(ad, "ebgp"), extractAdminDistanceField(ad, "ibgp")
}

// extractAdminDistanceField handles the three shapes a config tree may
// hand us: native JSON number (float64 after unmarshal), native Go int
// (direct tree delivery), or numeric string (legacy serialize-then-parse
// paths). Returns 0 for any value outside the YANG 1..255 range or any
// non-integer-valued float (e.g. 20.5). YANG's uint8 type forbids floats
// at validation time, so a fractional value reaching this extractor
// indicates an untrusted path (tests, raw plugin IPC) and is rejected
// rather than silently truncated.
func extractAdminDistanceField(m map[string]any, key string) uint8 {
	switch v := m[key].(type) {
	case float64:
		if v >= 1 && v <= 255 && v == float64(uint8(v)) {
			return uint8(v)
		}
	case int:
		if v >= 1 && v <= 255 {
			return uint8(v) //nolint:gosec // bounded to 1..255 above
		}
	case string:
		// ParseUint bitSize=8 already caps at 255; the >= 1 guard rejects "0".
		if n, err := strconv.ParseUint(v, 10, 8); err == nil && n >= 1 {
			return uint8(n)
		}
	}
	return 0
}
