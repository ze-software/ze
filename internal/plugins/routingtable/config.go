// Design: docs/architecture/static-routes.md -- routing-table config parsing

package routingtable

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// parseRoutingTableConfig parses the routing-table config JSON (map format from Tree.ToMap).
// The tree shape is:
//
//	{"routing-table": {"table": {"<name>": {"id": <number>}}}}
//
// Lists are keyed maps (list key = map key), not arrays.
func parseRoutingTableConfig(jsonData string) (map[string]uint32, error) {
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonData), &tree); err != nil {
		return nil, fmt.Errorf("unmarshal routing-table config: %w", err)
	}

	rtTree, ok := tree["routing-table"].(map[string]any)
	if !ok {
		return map[string]uint32{}, nil
	}

	tableMap, ok := rtTree["table"].(map[string]any)
	if !ok {
		return map[string]uint32{}, nil
	}

	tables := make(map[string]uint32, len(tableMap))
	for name, value := range tableMap {
		if name == "" {
			return nil, fmt.Errorf("routing-table entry missing name")
		}
		if name == "default" {
			return nil, fmt.Errorf("routing-table %q: name is built-in (maps to table 0)", name)
		}

		entry, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("routing-table %q: invalid entry", name)
		}

		idFloat, ok := cfgFloat(entry["id"])
		if !ok {
			return nil, fmt.Errorf("routing-table %q: missing or invalid id", name)
		}
		if idFloat < 0 || idFloat > math.MaxUint32 {
			return nil, fmt.Errorf("routing-table %q: id %v out of uint32 range", name, idFloat)
		}
		id := uint32(idFloat)

		if _, err := ValidateTableID(id); err != nil {
			return nil, fmt.Errorf("routing-table %q: %w", name, err)
		}

		tables[name] = id
	}

	return tables, nil
}

// cfgFloat coerces a config value to float64. The plugin config framework
// delivers YANG leaf values as JSON strings (e.g. "50"), so the string form is
// accepted alongside the native JSON number. Without it a string-valued id
// leaf would be rejected as "missing or invalid".
func cfgFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}
