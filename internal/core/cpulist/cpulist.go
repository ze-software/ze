// Design: docs/architecture/vpp-host-tuning.md -- the CPU list grammar Linux
// uses for isolcpus, for /sys/devices/system/cpu/{online,isolated}, and for the
// VPP corelist-workers directive.
//
// One grammar, one parser. The VPP component reads the kernel's isolated set
// and the operator's worker-cores leaf with it, and the appliance builder
// validates image.isolated-cpus with it before writing an isolcpus kernel
// argument. A second copy of this grammar would be a future disagreement with
// nothing to arbitrate it.
//
// Package cpulist parses and renders the CPU list syntax Linux uses for
// isolcpus, for /sys/devices/system/cpu and for VPP corelist-workers.
//
// The format is a comma-separated list of single ids and inclusive ranges:
// "3", "0-3", "0-1,4-6,9".

package cpulist

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// IDMax is the highest CPU id this package holds. Callers carry core ids as
	// uint8, so this is the type's ceiling rather than a policy limit.
	IDMax = 255
	// entriesMax bounds a list. At most IDMax+1 distinct ids exist and a
	// repeated id is refused, so a longer list cannot be valid and the parser
	// stops before walking it.
	entriesMax = IDMax + 1
)

var errTooLong = fmt.Errorf("core list has more than %d entries", entriesMax)

// ErrEmpty reports a list that parsed but named no CPU, for a caller whose
// source cannot legitimately be empty.
var ErrEmpty = errors.New("core list names no CPU")

// Parse parses a CPU list into ascending unique ids. An empty or blank string
// is an empty list, which is what /sys/devices/system/cpu/isolated holds on a
// host booted without isolcpus.
//
// A repeated id is an error, because a caller asking for the same CPU twice
// means one of the two entries is not what they intended.
func Parse(s string) ([]uint8, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	fields := strings.Split(s, ",")
	if len(fields) > entriesMax {
		return nil, errTooLong
	}

	var seen [IDMax + 1]bool
	ids := make([]uint8, 0, len(fields))
	for _, field := range fields {
		low, high, err := parseRange(strings.TrimSpace(field))
		if err != nil {
			return nil, err
		}
		for id := int(low); id <= int(high); id++ {
			if seen[id] {
				return nil, fmt.Errorf("core %d listed more than once", id)
			}
			seen[id] = true
			ids = append(ids, uint8(id))
		}
	}

	sortIDs(ids)
	return ids, nil
}

// parseRange parses one entry: either "7" or "0-3". The bounds are inclusive,
// and low is never above high.
func parseRange(field string) (low, high uint8, err error) {
	before, after, isRange := strings.Cut(field, "-")
	low, err = ParseID(before)
	if err != nil {
		return 0, 0, err
	}
	if !isRange {
		return low, low, nil
	}
	high, err = ParseID(after)
	if err != nil {
		return 0, 0, err
	}
	if high < low {
		return 0, 0, fmt.Errorf("core range %q counts down", field)
	}
	return low, high, nil
}

// ParseID parses a single CPU id and holds it to the uint8 range callers carry.
func ParseID(s string) (uint8, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("core id %q is not a number", s)
	}
	if n > IDMax {
		return 0, fmt.Errorf("core id %d is above the %d Ze supports", n, IDMax)
	}
	return uint8(n), nil
}

// sortIDs sorts core ids ascending with a counting pass over the 256 ids a
// uint8 holds, so the comparison sort and its closure are not needed here.
func sortIDs(ids []uint8) {
	var seen [IDMax + 1]bool
	for _, id := range ids {
		seen[id] = true
	}
	next := 0
	for id := 0; id <= IDMax; id++ {
		if !seen[id] {
			continue
		}
		ids[next] = uint8(id)
		next++
	}
}

// Format renders ascending core ids back into the list syntax, collapsing each
// run of consecutive ids into "low-high". An empty list renders as the empty
// string.
func Format(ids []uint8) string {
	if len(ids) == 0 {
		return ""
	}
	var tb textbuf.Buffer
	tb.Reset()
	for start := 0; start < len(ids); {
		end := start
		for end+1 < len(ids) && ids[end+1] == ids[end]+1 {
			end++
		}
		if start > 0 {
			tb.Byte(',')
		}
		tb.Int(int64(ids[start]))
		if end > start {
			tb.Byte('-').Int(int64(ids[end]))
		}
		start = end + 1
	}
	return tb.String()
}
