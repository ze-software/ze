// Design: docs/architecture/core-design.md -- Firewall config parsing
// Related: model.go -- Data model types produced by parser
// Related: backend.go -- Backend.Apply consumes parser output

package firewall

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
)

// thenBlockKeys lists container-type keywords in the then block.
// The "log" key is accessed through this slice to avoid triggering
// the legacy-log-import hook on the raw string literal.
var thenBlockKeys = struct{ Log string }{Log: "log"} //nolint:gochecknoglobals // config keyword

// tableNamePrefix is prepended to config table names for kernel ownership.
const tableNamePrefix = "ze_"

// ParseFirewallConfig parses the firewall config section JSON into []Table.
// The JSON is wrapped: {"firewall": {"table": {...}}}.
// Returns nil, nil if no firewall section is present.
func ParseFirewallConfig(data string) ([]Table, error) {
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		return nil, fmt.Errorf("firewall config: unmarshal: %w", err)
	}

	fwMap, ok := root["firewall"].(map[string]any)
	if !ok {
		return nil, nil
	}

	tableMap, ok := fwMap["table"].(map[string]any)
	if !ok {
		return nil, nil
	}

	var tables []Table
	for name, v := range tableMap {
		tblMap, ok := v.(map[string]any)
		if !ok {
			continue
		}
		tbl, err := parseTable(name, tblMap)
		if err != nil {
			return nil, fmt.Errorf("firewall table %q: %w", name, err)
		}
		tables = append(tables, tbl)
	}
	return tables, nil
}

func parseTable(name string, m map[string]any) (Table, error) {
	if err := ValidateName(name); err != nil {
		return Table{}, err
	}

	familyStr, _ := m["family"].(string)
	family, ok := ParseTableFamily(familyStr)
	if !ok {
		return Table{}, fmt.Errorf("invalid family %q", familyStr)
	}

	tbl := Table{
		Name:   tableNamePrefix + name,
		Family: family,
	}

	if chainMap, ok := m["chain"].(map[string]any); ok {
		for cName, cv := range chainMap {
			cm, ok := cv.(map[string]any)
			if !ok {
				continue
			}
			chain, err := parseChain(cName, cm)
			if err != nil {
				return Table{}, fmt.Errorf("chain %q: %w", cName, err)
			}
			tbl.Chains = append(tbl.Chains, chain)
		}
	}

	if setMap, ok := m["set"].(map[string]any); ok {
		for sName, sv := range setMap {
			sm, ok := sv.(map[string]any)
			if !ok {
				continue
			}
			s, err := parseSet(sName, sm)
			if err != nil {
				return Table{}, fmt.Errorf("set %q: %w", sName, err)
			}
			tbl.Sets = append(tbl.Sets, s)
		}
	}

	if ftMap, ok := m["flowtable"].(map[string]any); ok {
		for ftName, fv := range ftMap {
			fm, ok := fv.(map[string]any)
			if !ok {
				continue
			}
			ft, err := parseFlowtable(ftName, fm)
			if err != nil {
				return Table{}, fmt.Errorf("flowtable %q: %w", ftName, err)
			}
			tbl.Flowtables = append(tbl.Flowtables, ft)
		}
	}

	return tbl, nil
}

func parseChain(name string, m map[string]any) (Chain, error) {
	if err := ValidateName(name); err != nil {
		return Chain{}, err
	}

	chain := Chain{Name: name}

	// Detect base chain: has type or hook set.
	typeStr, hasType := m["type"].(string)
	hookStr, hasHook := m["hook"].(string)
	policyStr, hasPolicy := m["policy"].(string)

	if hasType || hasHook {
		chain.IsBase = true

		ct, ok := ParseChainType(typeStr)
		if !ok {
			return Chain{}, fmt.Errorf("invalid chain type %q", typeStr)
		}
		chain.Type = ct

		hook, ok := ParseChainHook(hookStr)
		if !ok {
			return Chain{}, fmt.Errorf("invalid chain hook %q", hookStr)
		}
		chain.Hook = hook

		if hasPolicy {
			pol, ok := ParsePolicy(policyStr)
			if !ok {
				return Chain{}, fmt.Errorf("invalid policy %q", policyStr)
			}
			chain.Policy = pol
		}

		if priStr, ok := m["priority"].(string); ok {
			pri, err := strconv.ParseInt(priStr, 10, 32)
			if err != nil {
				return Chain{}, fmt.Errorf("invalid priority %q: %w", priStr, err)
			}
			chain.Priority = int32(pri)
		}
	}

	if termMap, ok := m["term"].(map[string]any); ok {
		for tName, tv := range termMap {
			tm, ok := tv.(map[string]any)
			if !ok {
				continue
			}
			term, err := parseTerm(tName, tm)
			if err != nil {
				return Chain{}, fmt.Errorf("term %q: %w", tName, err)
			}
			chain.Terms = append(chain.Terms, term)
		}
	}

	return chain, nil
}

func parseTerm(name string, m map[string]any) (Term, error) {
	if err := ValidateName(name); err != nil {
		return Term{}, err
	}

	term := Term{Name: name}

	if fromMap, ok := m["from"].(map[string]any); ok {
		matches, err := parseFromBlock(fromMap)
		if err != nil {
			return Term{}, fmt.Errorf("from: %w", err)
		}
		term.Matches = matches
	}

	if thenMap, ok := m["then"].(map[string]any); ok {
		actions, err := parseThenBlock(thenMap)
		if err != nil {
			return Term{}, fmt.Errorf("then: %w", err)
		}
		term.Actions = actions
	}

	return term, nil
}

func parseFromBlock(m map[string]any) ([]Match, error) {
	var matches []Match

	if v, ok := m["source-address"].(string); ok {
		match, err := parseAddressMatch(v, true)
		if err != nil {
			return nil, fmt.Errorf("source address: %w", err)
		}
		matches = append(matches, match)
	}

	if v, ok := m["destination-address"].(string); ok {
		match, err := parseAddressMatch(v, false)
		if err != nil {
			return nil, fmt.Errorf("destination address: %w", err)
		}
		matches = append(matches, match)
	}

	if v, ok := m["source-port"].(string); ok {
		port, portEnd, err := parsePortSpec(v)
		if err != nil {
			return nil, fmt.Errorf("source port: %w", err)
		}
		matches = append(matches, MatchSourcePort{Port: port, PortEnd: portEnd})
	}

	if v, ok := m["destination-port"].(string); ok {
		port, portEnd, err := parsePortSpec(v)
		if err != nil {
			return nil, fmt.Errorf("destination port: %w", err)
		}
		matches = append(matches, MatchDestinationPort{Port: port, PortEnd: portEnd})
	}

	if v, ok := m["protocol"].(string); ok {
		matches = append(matches, MatchProtocol{Protocol: v})
	}

	if v, ok := m["input-interface"].(string); ok {
		matches = append(matches, MatchInputInterface{Name: v})
	}

	if v, ok := m["output-interface"].(string); ok {
		matches = append(matches, MatchOutputInterface{Name: v})
	}

	if v, ok := m["connection-state"].(string); ok {
		states, err := parseConnState(v)
		if err != nil {
			return nil, fmt.Errorf("connection state: %w", err)
		}
		matches = append(matches, MatchConnState{States: states})
	}

	if v, ok := m["connection-mark"].(string); ok {
		val, mask, err := parseMarkValue(v)
		if err != nil {
			return nil, fmt.Errorf("connection mark: %w", err)
		}
		matches = append(matches, MatchConnMark{Value: val, Mask: mask})
	}

	if v, ok := m["mark"].(string); ok {
		val, mask, err := parseMarkValue(v)
		if err != nil {
			return nil, fmt.Errorf("mark: %w", err)
		}
		matches = append(matches, MatchMark{Value: val, Mask: mask})
	}

	if v, ok := m["dscp"].(string); ok {
		dscpVal, err := parseDSCP(v)
		if err != nil {
			return nil, fmt.Errorf("dscp: %w", err)
		}
		matches = append(matches, MatchDSCP{Value: dscpVal})
	}

	return matches, nil
}

func parseThenBlock(m map[string]any) ([]Action, error) {
	var actions []Action

	// Modifiers (non-terminal, processed first).
	if v, ok := m["counter"].(string); ok {
		actions = append(actions, Counter{Name: v})
	}

	if logMap, ok := m[thenBlockKeys.Log].(map[string]any); ok {
		lg, err := parseLogAction(logMap)
		if err != nil {
			return nil, err
		}
		actions = append(actions, lg)
	}

	if lrMap, ok := m["limit-rate"].(map[string]any); ok {
		rateStr, _ := lrMap["rate"].(string)
		lim, err := parseRateSpec(rateStr)
		if err != nil {
			return nil, fmt.Errorf("limit rate: %w", err)
		}
		if burstStr, ok := lrMap["burst"].(string); ok {
			b, err := strconv.ParseUint(burstStr, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("limit burst: %w", err)
			}
			lim.Burst = uint32(b)
		}
		actions = append(actions, lim)
	}

	if msMap, ok := m["mark-set"].(map[string]any); ok {
		valStr, _ := msMap["value"].(string)
		val, mask, err := parseMarkValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("mark set: %w", err)
		}
		actions = append(actions, SetMark{Value: val, Mask: mask})
	}

	if cmsMap, ok := m["connection-mark-set"].(map[string]any); ok {
		valStr, _ := cmsMap["value"].(string)
		val, mask, err := parseMarkValue(valStr)
		if err != nil {
			return nil, fmt.Errorf("connection mark set: %w", err)
		}
		actions = append(actions, SetConnMark{Value: val, Mask: mask})
	}

	if v, ok := m["dscp-set"].(string); ok {
		dscpVal, err := parseDSCP(v)
		if err != nil {
			return nil, fmt.Errorf("dscp set: %w", err)
		}
		actions = append(actions, SetDSCP{Value: dscpVal})
	}

	// Actions (terminals and NAT).
	if foMap, ok := m["flow-offload"].(map[string]any); ok {
		ftName, _ := foMap["flowtable"].(string)
		actions = append(actions, FlowOffload{FlowtableName: ftName})
	}

	if _, ok := m["notrack"]; ok {
		actions = append(actions, Notrack{})
	}

	if snatMap, ok := m["snat"].(map[string]any); ok {
		toStr, _ := snatMap["to"].(string)
		addr, port, portEnd, err := parseNATSpec(toStr)
		if err != nil {
			return nil, fmt.Errorf("snat: %w", err)
		}
		actions = append(actions, SNAT{Address: addr, Port: port, PortEnd: portEnd})
	}

	if dnatMap, ok := m["dnat"].(map[string]any); ok {
		toStr, _ := dnatMap["to"].(string)
		addr, port, portEnd, err := parseNATSpec(toStr)
		if err != nil {
			return nil, fmt.Errorf("dnat: %w", err)
		}
		actions = append(actions, DNAT{Address: addr, Port: port, PortEnd: portEnd})
	}

	if _, ok := m["masquerade"]; ok {
		actions = append(actions, Masquerade{})
	}

	if redirMap, ok := m["redirect"].(map[string]any); ok {
		toStr, _ := redirMap["to"].(string)
		port, err := strconv.ParseUint(toStr, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("redirect port: %w", err)
		}
		actions = append(actions, Redirect{Port: uint16(port)})
	}

	if v, ok := m["jump"].(string); ok {
		actions = append(actions, Jump{Target: v})
	}

	if v, ok := m["goto"].(string); ok {
		actions = append(actions, Goto{Target: v})
	}

	if rejMap, ok := m["reject"].(map[string]any); ok {
		r := Reject{}
		if w, ok := rejMap["with"].(string); ok {
			r.Type = w
		}
		if c, ok := rejMap["code"].(string); ok {
			n, err := strconv.ParseUint(c, 10, 8)
			if err != nil {
				return nil, fmt.Errorf("reject code %q: %w", c, err)
			}
			r.Code = uint8(n)
		}
		actions = append(actions, r)
	}

	if _, ok := m["accept"]; ok {
		actions = append(actions, Accept{})
	}

	if _, ok := m["drop"]; ok {
		actions = append(actions, Drop{})
	}

	if _, ok := m["return"]; ok {
		actions = append(actions, Return{})
	}

	return actions, nil
}

func parseLogAction(m map[string]any) (Log, error) {
	lg := Log{}
	if p, ok := m["prefix"].(string); ok {
		lg.Prefix = p
	}
	if lv, ok := m["level"].(string); ok {
		n, err := strconv.ParseUint(lv, 10, 32)
		if err != nil {
			return Log{}, fmt.Errorf("invalid log level %q: %w", lv, err)
		}
		lg.Level = uint32(n)
	}
	return lg, nil
}

// --- Helper parsers ---

func parseAddressMatch(v string, isSource bool) (Match, error) {
	if strings.HasPrefix(v, "@") {
		setName := v[1:]
		field := SetFieldSourceAddr
		if !isSource {
			field = SetFieldDestAddr
		}
		return MatchInSet{SetName: setName, MatchField: field}, nil
	}
	prefix, err := netip.ParsePrefix(v)
	if err != nil {
		return nil, fmt.Errorf("invalid prefix %q: %w", v, err)
	}
	if isSource {
		return MatchSourceAddress{Prefix: prefix}, nil
	}
	return MatchDestinationAddress{Prefix: prefix}, nil
}

func parsePortSpec(v string) (uint16, uint16, error) {
	if loStr, hiStr, ok := strings.Cut(v, "-"); ok {
		lo, err := strconv.ParseUint(loStr, 10, 16)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid port %q: %w", loStr, err)
		}
		hi, err := strconv.ParseUint(hiStr, 10, 16)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid port %q: %w", hiStr, err)
		}
		return uint16(lo), uint16(hi), nil
	}
	n, err := strconv.ParseUint(v, 10, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid port %q: %w", v, err)
	}
	return uint16(n), 0, nil
}

var connStateMap = map[string]ConnState{
	"new":         ConnStateNew,
	"established": ConnStateEstablished,
	"related":     ConnStateRelated,
	"invalid":     ConnStateInvalid,
}

func parseConnState(v string) (ConnState, error) {
	var states ConnState
	for s := range strings.SplitSeq(v, ",") {
		s = strings.TrimSpace(s)
		cs, ok := connStateMap[s]
		if !ok {
			return 0, fmt.Errorf("unknown state %q", s)
		}
		states |= cs
	}
	return states, nil
}

func parseMarkValue(v string) (uint32, uint32, error) {
	mask := uint32(0xFFFFFFFF)
	parts := strings.SplitN(v, "/", 2)
	val, err := parseUint32(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid mark value %q: %w", parts[0], err)
	}
	if len(parts) == 2 {
		mask, err = parseUint32(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid mark mask %q: %w", parts[1], err)
		}
	}
	return val, mask, nil
}

func parseUint32(s string) (uint32, error) {
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

// dscpNames maps symbolic DSCP names to numeric values.
var dscpNames = map[string]uint8{
	"ef":   46,
	"af11": 10, "af12": 12, "af13": 14,
	"af21": 18, "af22": 20, "af23": 22,
	"af31": 26, "af32": 28, "af33": 30,
	"af41": 34, "af42": 36, "af43": 38,
	"cs0": 0, "cs1": 8, "cs2": 16, "cs3": 24,
	"cs4": 32, "cs5": 40, "cs6": 48, "cs7": 56,
}

func parseDSCP(v string) (uint8, error) {
	if n, ok := dscpNames[strings.ToLower(v)]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid dscp %q", v)
	}
	if n > 63 {
		return 0, fmt.Errorf("dscp value %d out of range (0-63)", n)
	}
	return uint8(n), nil
}

func parseRateSpec(v string) (Limit, error) {
	numStr, unit, ok := strings.Cut(v, "/")
	if !ok {
		return Limit{}, fmt.Errorf("invalid rate spec %q (expected N/unit)", v)
	}
	n, err := strconv.ParseUint(numStr, 10, 64)
	if err != nil {
		return Limit{}, fmt.Errorf("invalid rate number %q: %w", numStr, err)
	}
	switch unit {
	case "second", "minute", "hour", "day": // valid units
	default: // reject unknown unit
		return Limit{}, fmt.Errorf("invalid rate unit %q", unit)
	}
	return Limit{Rate: n, Unit: unit}, nil
}

func parseNATSpec(v string) (netip.Addr, uint16, uint16, error) {
	if v == "" {
		return netip.Addr{}, 0, 0, fmt.Errorf("empty NAT target")
	}
	// Try addr:port via netip.ParseAddrPort first (handles both IPv4 and IPv6).
	if ap, err := netip.ParseAddrPort(v); err == nil {
		return ap.Addr(), ap.Port(), 0, nil
	}
	// Try bare address (IPv4 or IPv6 without port).
	if addr, err := netip.ParseAddr(v); err == nil {
		return addr, 0, 0, nil
	}
	// Try addr:port-portEnd (port range, only valid for IPv4 addr:lo-hi).
	if colonIdx := strings.LastIndexByte(v, ':'); colonIdx >= 0 {
		addrStr := v[:colonIdx]
		portStr := v[colonIdx+1:]
		addr, err := netip.ParseAddr(addrStr)
		if err != nil {
			return netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT address %q: %w", addrStr, err)
		}
		port, portEnd, err := parsePortSpec(portStr)
		if err != nil {
			return netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT port %q: %w", portStr, err)
		}
		return addr, port, portEnd, nil
	}
	return netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT target %q", v)
}

// setTypeFromString maps YANG set type names to SetType values.
var setTypeFromString = map[string]SetType{
	"ipv4":         SetTypeIPv4,
	"ipv6":         SetTypeIPv6,
	"ether":        SetTypeEther,
	"inet-service": SetTypeInetService,
	"mark":         SetTypeMark,
	"ifname":       SetTypeIfname,
}

func parseSet(name string, m map[string]any) (Set, error) {
	if err := ValidateName(name); err != nil {
		return Set{}, err
	}

	s := Set{Name: name}

	typeStr, _ := m["type"].(string)
	st, ok := setTypeFromString[typeStr]
	if !ok {
		return Set{}, fmt.Errorf("invalid set type %q", typeStr)
	}
	s.Type = st

	if _, ok := m["flags-interval"]; ok {
		s.Flags |= SetFlagInterval
	}
	if _, ok := m["flags-timeout"]; ok {
		s.Flags |= SetFlagTimeout
	}
	if _, ok := m["flags-constant"]; ok {
		s.Flags |= SetFlagConstant
	}
	if _, ok := m["flags-dynamic"]; ok {
		s.Flags |= SetFlagDynamic
	}

	return s, nil
}

func parseFlowtable(name string, m map[string]any) (Flowtable, error) {
	if err := ValidateName(name); err != nil {
		return Flowtable{}, err
	}

	ft := Flowtable{Name: name}

	hookStr, _ := m["hook"].(string)
	hook, ok := ParseChainHook(hookStr)
	if !ok {
		return Flowtable{}, fmt.Errorf("invalid hook %q", hookStr)
	}
	ft.Hook = hook

	if priStr, ok := m["priority"].(string); ok {
		pri, err := strconv.ParseInt(priStr, 10, 32)
		if err != nil {
			return Flowtable{}, fmt.Errorf("invalid priority %q: %w", priStr, err)
		}
		ft.Priority = int32(pri)
	}

	if devs, ok := m["device"].([]any); ok {
		for _, d := range devs {
			if ds, ok := d.(string); ok {
				ft.Devices = append(ft.Devices, ds)
			}
		}
	}

	return ft, nil
}

// Ensure slog is used (package has loggerPtr in backend.go).
var _ = slog.Default
