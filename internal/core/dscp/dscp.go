// Design: docs/architecture/core-design.md -- DSCP name-to-value map

package dscp

import (
	"fmt"
	"strconv"
	"strings"
)

const MaxValue = 63

var byName = map[string]uint8{
	"ef":   46,
	"af11": 10, "af12": 12, "af13": 14,
	"af21": 18, "af22": 20, "af23": 22,
	"af31": 26, "af32": 28, "af33": 30,
	"af41": 34, "af42": 36, "af43": 38,
	"cs0": 0, "cs1": 8, "cs2": 16, "cs3": 24,
	"cs4": 32, "cs5": 40, "cs6": 48, "cs7": 56,
}

// Parse accepts a named DSCP value (e.g. "cs6", "ef") or a decimal
// integer and returns the numeric DSCP value (0-63).
func Parse(v string) (uint8, error) {
	if n, ok := byName[strings.ToLower(v)]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid dscp %q", v)
	}
	if n > MaxValue {
		return 0, fmt.Errorf("dscp value %d out of range (0-%d)", n, MaxValue)
	}
	return uint8(n), nil
}
