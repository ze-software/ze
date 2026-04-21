// Design: docs/architecture/core-design.md -- Traffic control config parsing
// Related: model.go -- Data model types produced by parser
// Related: backend.go -- Backend.Apply consumes parser output

package traffic

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ParseTrafficConfig parses the traffic-control config section JSON into a map
// of interface name to InterfaceQoS. The JSON is wrapped:
// {"traffic-control": {"interface": {...}}}.
// Returns nil, nil if no traffic-control section is present.
func ParseTrafficConfig(data string) (map[string]InterfaceQoS, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("traffic config: unmarshal: %w", err)
	}

	tcMap, ok := root["traffic-control"].(map[string]any)
	if !ok {
		return map[string]InterfaceQoS{}, nil
	}

	ifaceMap, ok := tcMap["interface"].(map[string]any)
	if !ok {
		return map[string]InterfaceQoS{}, nil
	}

	result := make(map[string]InterfaceQoS, len(ifaceMap))
	for name, v := range ifaceMap {
		im, ok := v.(map[string]any)
		if !ok {
			continue
		}
		qos, err := parseInterfaceQoS(name, im)
		if err != nil {
			return nil, fmt.Errorf("traffic interface %q: %w", name, err)
		}
		result[name] = qos
	}
	return result, nil
}

func parseInterfaceQoS(name string, m map[string]any) (InterfaceQoS, error) {
	qos := InterfaceQoS{Interface: name}

	qdiscMap, ok := m["qdisc"].(map[string]any)
	if !ok {
		return qos, nil
	}

	typeStr, _ := qdiscMap["type"].(string)
	qt, ok := ParseQdiscType(typeStr)
	if !ok {
		return InterfaceQoS{}, fmt.Errorf("invalid qdisc type %q", typeStr)
	}
	qos.Qdisc.Type = qt

	if dc, ok := qdiscMap["default-class"].(string); ok {
		qos.Qdisc.DefaultClass = dc
	}

	if classMap, ok := qdiscMap["class"].(map[string]any); ok {
		for cName, cv := range classMap {
			cm, ok := cv.(map[string]any)
			if !ok {
				continue
			}
			tc, err := parseTrafficClass(cName, cm)
			if err != nil {
				return InterfaceQoS{}, fmt.Errorf("class %q: %w", cName, err)
			}
			qos.Qdisc.Classes = append(qos.Qdisc.Classes, tc)
		}
	}

	return qos, nil
}

func parseTrafficClass(name string, m map[string]any) (TrafficClass, error) {
	tc := TrafficClass{Name: name}

	if rateStr, ok := m["rate"].(string); ok {
		rate, err := ParseRateBps(rateStr)
		if err != nil {
			return TrafficClass{}, fmt.Errorf("rate: %w", err)
		}
		tc.Rate = rate
	}

	if ceilStr, ok := m["ceil"].(string); ok {
		ceil, err := ParseRateBps(ceilStr)
		if err != nil {
			return TrafficClass{}, fmt.Errorf("ceil: %w", err)
		}
		tc.Ceil = ceil
	}

	if priStr, ok := m["priority"].(string); ok {
		pri, err := strconv.ParseUint(priStr, 10, 8)
		if err != nil {
			return TrafficClass{}, fmt.Errorf("priority: %w", err)
		}
		tc.Priority = uint8(pri)
	}

	if matchMap, ok := m["match"].(map[string]any); ok {
		for ftName, fv := range matchMap {
			fm, ok := fv.(map[string]any)
			if !ok {
				continue
			}
			ft, ok := ParseFilterType(ftName)
			if !ok {
				return TrafficClass{}, fmt.Errorf("unknown filter type %q", ftName)
			}
			valStr, _ := fm["value"].(string)
			val, err := parseFilterValue(ft, valStr)
			if err != nil {
				return TrafficClass{}, fmt.Errorf("filter %q: %w", ftName, err)
			}
			tc.Filters = append(tc.Filters, TrafficFilter{Type: ft, Value: val})
		}
	}

	return tc, nil
}

// rateSuffixes lists rate suffixes ordered longest-first to avoid substring
// collisions (e.g., "mbit" must match before "bit").
var rateSuffixes = []struct {
	suffix string
	mult   uint64
}{
	{"gbit", 1_000_000_000},
	{"mbit", 1_000_000},
	{"kbit", 1_000},
	{"gbps", 8_000_000_000},
	{"mbps", 8_000_000},
	{"kbps", 8_000},
	{"bit", 1},
	{"bps", 8},
}

func ParseRateBps(v string) (uint64, error) {
	for _, rs := range rateSuffixes {
		if strings.HasSuffix(v, rs.suffix) {
			numStr := v[:len(v)-len(rs.suffix)]
			n, err := strconv.ParseUint(numStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid rate %q: %w", v, err)
			}
			return n * rs.mult, nil
		}
	}
	return 0, fmt.Errorf("invalid rate %q (must end with bit/kbit/mbit/gbit/bps/kbps/mbps/gbps)", v)
}

func parseFilterValue(ft FilterType, v string) (uint32, error) {
	if v == "" {
		return 0, fmt.Errorf("empty filter value")
	}
	switch ft { //nolint:exhaustive // filterUnknown rejected by ParseFilterType before reaching here
	case FilterMark:
		return parseHexOrDec(v)
	case FilterDSCP, FilterProtocol:
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid value %q: %w", v, err)
		}
		return uint32(n), nil
	case filterUnknown:
		return 0, fmt.Errorf("unsupported filter type %v", ft)
	}
	return 0, fmt.Errorf("unsupported filter type %v", ft)
}

func parseHexOrDec(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	var n uint64
	var err error
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err = strconv.ParseUint(s[2:], 16, 32)
	} else {
		n, err = strconv.ParseUint(s, 10, 32)
	}
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}
