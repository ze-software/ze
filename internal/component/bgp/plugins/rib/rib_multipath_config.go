// Design: docs/architecture/plugin/rib-storage-design.md — multipath config extraction
// Related: rib.go — RIBManager state and OnConfigure wiring
// Related: bestpath.go — best-path selection (consumer of maximumPaths/relaxASPath)

package rib

import (
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/configjson"
)

// maxMultipathPaths is the YANG-enforced upper bound on bgp/multipath/maximum-paths.
// Mirrored here so the extractor can refuse out-of-range values even on inputs
// that bypass full YANG validation (tests, raw JSON delivery via plugin IPC).
const maxMultipathPaths = 256

// extractMultipathConfig extracts the bgp/multipath container from a Stage 2
// config section. Returns the configured maximum-paths (bounded to 1..256 per
// YANG) and relax-as-path boolean.
//
// Returns (0, false) when no multipath block is present or when the values are
// out of range, so callers can apply the RFC 4271 default (single best-path,
// maximum-paths=1).
func extractMultipathConfig(jsonStr string) (maxPaths uint16, relax bool) {
	bgp, ok := configjson.ParseBGPSubtree(jsonStr)
	if !ok {
		return 0, false
	}
	mp, ok := bgp["multipath"].(map[string]any)
	if !ok {
		return 0, false
	}
	// maximum-paths (YANG uint16 range 1..256). The switch handles the three
	// shapes a config tree may hand us: native JSON number (float64 after
	// unmarshal), native Go int (direct tree delivery), or a numeric string
	// (legacy serialize-then-parse paths).
	switch v := mp["maximum-paths"].(type) {
	case float64:
		if v >= 1 && v <= maxMultipathPaths {
			maxPaths = uint16(v)
		}
	case int:
		if v >= 1 && v <= maxMultipathPaths {
			maxPaths = uint16(v) //nolint:gosec // bounded by maxMultipathPaths
		}
	case string:
		if n, err := strconv.ParseUint(v, 10, 16); err == nil && n >= 1 && n <= maxMultipathPaths {
			maxPaths = uint16(n)
		}
	}
	// relax-as-path (boolean).
	switch v := mp["relax-as-path"].(type) {
	case bool:
		relax = v
	case string:
		relax = v == "true"
	}
	return maxPaths, relax
}
