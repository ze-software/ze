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
// single best-path. A maximum-paths out of range or not a number, and a
// relax-as-path that is not a boolean, is an error: reading it as the default
// would tell the operator their edit took effect when it did not.
func extractMultipathConfig(bgp map[string]any) (maxPaths uint32, relax bool, err error) {
	mp, ok := bgp["multipath"].(map[string]any)
	if !ok {
		return defaultMultipathPaths, false, nil
	}
	maxPaths, err = multipathMaximumPaths(mp["maximum-paths"])
	if err != nil {
		return 0, false, err
	}
	relax, err = multipathRelaxASPath(mp["relax-as-path"])
	if err != nil {
		return 0, false, err
	}
	return maxPaths, relax, nil
}

// multipathRelaxASPath reads the relax-as-path leaf: a JSON boolean, or the
// string "true" or "false" a serialized tree carries. nil is the absent leaf,
// the YANG default false. Any other value is refused: reading it as false
// would keep strict AS_PATH matching while the commit reports the edit taken.
func multipathRelaxASPath(value any) (bool, error) {
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		}
		if v == "false" {
			return false, nil
		}
		return false, fmt.Errorf("bgp multipath relax-as-path %q is not a boolean: "+
			"the RIB cannot tell whether paths with different AS_PATHs may share a multipath set; "+
			"fix: set bgp multipath relax-as-path true (or false)", v)
	default:
		return false, fmt.Errorf("bgp multipath relax-as-path must be a boolean, got %T: "+
			"the RIB cannot tell whether paths with different AS_PATHs may share a multipath set; "+
			"fix: set bgp multipath relax-as-path true (or false)", value)
	}
}

// maximumPathsReason and maximumPathsFix close every maximum-paths refusal:
// why the value cannot be used, and the command that sets a usable one.
const (
	maximumPathsReason = "the RIB keeps between 1 and 256 equal-cost paths per prefix"
	maximumPathsFix    = "fix: set bgp multipath maximum-paths <1-256>"
)

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
			return 0, fmt.Errorf("bgp multipath maximum-paths %v is not a whole number: %s; %s", v, maximumPathsReason, maximumPathsFix)
		}
		paths = int64(v)
	case int:
		paths = int64(v)
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("bgp multipath maximum-paths %q is not a number: %s; %s", v, maximumPathsReason, maximumPathsFix)
		}
		paths = n
	default:
		return 0, fmt.Errorf("bgp multipath maximum-paths must be a number, got %T: %s; %s", value, maximumPathsReason, maximumPathsFix)
	}
	if paths < 1 {
		return 0, fmt.Errorf("bgp multipath maximum-paths %d is below 1: %s; %s", paths, maximumPathsReason, maximumPathsFix)
	}
	if paths > maxMultipathPaths {
		return 0, fmt.Errorf("bgp multipath maximum-paths %d is above %d: %s; %s", paths, maxMultipathPaths, maximumPathsReason, maximumPathsFix)
	}
	return uint32(paths), nil
}
