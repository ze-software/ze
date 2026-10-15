// Design: docs/architecture/plugin/rib-storage-design.md — multipath config extraction
// Related: config_tx.go — the config transaction that installs what this reads
// Related: bestpath.go — best-path selection (consumer of maximumPaths/relaxASPath)

package rib

import (
	"fmt"
	"strconv"
)

// maxMultipathPaths is the YANG-enforced upper bound on bgp/multipath/maximum-paths.
// Mirrored here so the extractor can refuse out-of-range values even on inputs
// that bypass full YANG validation (tests, raw JSON delivery via plugin IPC).
const maxMultipathPaths = 256

// defaultMultipathPaths is the YANG default of maximum-paths: RFC 4271
// single best-path selection.
const defaultMultipathPaths = 1

// extractMultipathConfig reads the bgp/multipath container of a parsed bgp
// subtree: maximum-paths (1..256) and relax-as-path.
//
// An absent container or leaf is the YANG default, maximum-paths 1 and
// relax-as-path false, so a reload that removes the leaf returns the RIB to
// single best-path. A value out of range, or one that is not a number, is an
// error: reading it as the default would tell the operator their edit took
// effect when it did not.
func extractMultipathConfig(bgp map[string]any) (maxPaths uint32, relax bool, err error) {
	mp, ok := bgp["multipath"].(map[string]any)
	if !ok {
		return defaultMultipathPaths, false, nil
	}
	maxPaths, err = multipathMaximumPaths(mp["maximum-paths"])
	if err != nil {
		return 0, false, err
	}
	// relax-as-path (boolean).
	switch v := mp["relax-as-path"].(type) {
	case bool:
		relax = v
	case string:
		relax = v == "true"
	}
	return maxPaths, relax, nil
}

// multipathMaximumPaths reads the maximum-paths leaf. The switch handles the
// three shapes a config tree may hand us: native JSON number (float64 after
// unmarshal), native Go int (direct tree delivery), or a numeric string
// (legacy serialize-then-parse paths). nil is the absent leaf.
func multipathMaximumPaths(value any) (uint32, error) {
	var paths int64
	switch v := value.(type) {
	case nil:
		return defaultMultipathPaths, nil
	case float64:
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("bgp multipath maximum-paths %v is not a whole number (allowed 1-%d)", v, maxMultipathPaths)
		}
		paths = int64(v)
	case int:
		paths = int64(v)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("bgp multipath maximum-paths %q is not a number (allowed 1-%d)", v, maxMultipathPaths)
		}
		paths = n
	default:
		return 0, fmt.Errorf("bgp multipath maximum-paths must be a number, got %T", value)
	}
	if paths < 1 {
		return 0, fmt.Errorf("bgp multipath maximum-paths %d is out of range (allowed 1-%d)", paths, maxMultipathPaths)
	}
	if paths > maxMultipathPaths {
		return 0, fmt.Errorf("bgp multipath maximum-paths %d is out of range (allowed 1-%d)", paths, maxMultipathPaths)
	}
	return uint32(paths), nil
}
