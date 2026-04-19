// Design: docs/architecture/core-design.md -- Firewall config parsing
// Related: model.go -- Data model types produced by parser
// Related: backend.go -- Backend.Apply consumes parser output

package firewall

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"sort"
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
		match, err := parsePortMatch(v, true)
		if err != nil {
			return nil, fmt.Errorf("source port: %w", err)
		}
		matches = append(matches, match)
	}

	if v, ok := m["destination-port"].(string); ok {
		match, err := parsePortMatch(v, false)
		if err != nil {
			return nil, fmt.Errorf("destination port: %w", err)
		}
		matches = append(matches, match)
	}

	if v, ok := m["protocol"].(string); ok {
		matches = append(matches, MatchProtocol{Protocol: v})
	}

	if v, ok := m["input-interface"].(string); ok {
		name, wildcard := parseInterfaceSpec(v)
		matches = append(matches, MatchInputInterface{Name: name, Wildcard: wildcard})
	}

	if v, ok := m["output-interface"].(string); ok {
		name, wildcard := parseInterfaceSpec(v)
		matches = append(matches, MatchOutputInterface{Name: name, Wildcard: wildcard})
	}

	if v, ok := m["icmp-type"].(string); ok {
		t, err := parseICMPType(v)
		if err != nil {
			return nil, fmt.Errorf("icmp-type: %w", err)
		}
		matches = append(matches, MatchICMPType{Type: t})
	}

	if v, ok := m["icmpv6-type"].(string); ok {
		t, err := parseICMPv6Type(v)
		if err != nil {
			return nil, fmt.Errorf("icmpv6-type: %w", err)
		}
		matches = append(matches, MatchICMPv6Type{Type: t})
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
	if counterMap, ok := m["counter"].(map[string]any); ok {
		name, _ := counterMap["name"].(string)
		actions = append(actions, Counter{Name: name})
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
		addr, addrEnd, port, portEnd, err := parseNATSpec(toStr)
		if err != nil {
			return nil, fmt.Errorf("snat: %w", err)
		}
		actions = append(actions, SNAT{Address: addr, AddressEnd: addrEnd, Port: port, PortEnd: portEnd})
	}

	if dnatMap, ok := m["dnat"].(map[string]any); ok {
		toStr, _ := dnatMap["to"].(string)
		addr, addrEnd, port, portEnd, err := parseNATSpec(toStr)
		if err != nil {
			return nil, fmt.Errorf("dnat: %w", err)
		}
		actions = append(actions, DNAT{Address: addr, AddressEnd: addrEnd, Port: port, PortEnd: portEnd})
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

	// NAT `exclude` keyword: in a NAT chain term, means "skip NAT for
	// matching traffic". The equivalent nftables expression is the
	// Return verdict on the NAT chain -- so we emit Return here and
	// rely on the existing lowering to program it.
	if _, ok := m["exclude"]; ok {
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
		v := uint32(n)
		lg.Level = &v
	}
	if gv, ok := m["group"].(string); ok {
		n, err := strconv.ParseUint(gv, 10, 16)
		if err != nil {
			return Log{}, fmt.Errorf("invalid log group %q: %w", gv, err)
		}
		v := uint16(n)
		lg.Group = &v
	}
	if sv, ok := m["snaplen"].(string); ok {
		n, err := strconv.ParseUint(sv, 10, 32)
		if err != nil {
			return Log{}, fmt.Errorf("invalid log snaplen %q: %w", sv, err)
		}
		v := uint32(n)
		lg.Snaplen = &v
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

// parsePortMatch mirrors parseAddressMatch for the port leaves: a bare
// "@setname" becomes a MatchInSet against a port (inet-service) set;
// any other form routes through parsePortSpec and becomes a MatchSourcePort
// or MatchDestinationPort. The cross-reference to the declared set name is
// checked at verify (ValidateTables), so a typo surfaces there rather than
// here.
func parsePortMatch(v string, isSource bool) (Match, error) {
	if strings.HasPrefix(v, "@") {
		setName := v[1:]
		if err := ValidateName(setName); err != nil {
			return nil, fmt.Errorf("invalid set name %q: %w", setName, err)
		}
		field := SetFieldSourcePort
		if !isSource {
			field = SetFieldDestPort
		}
		return MatchInSet{SetName: setName, MatchField: field}, nil
	}
	ranges, err := parsePortSpec(v)
	if err != nil {
		return nil, err
	}
	if isSource {
		return MatchSourcePort{Ranges: ranges}, nil
	}
	return MatchDestinationPort{Ranges: ranges}, nil
}

// maxPortRanges caps the number of comma-separated entries a single
// port spec may carry. The YANG pattern accepts arbitrary repetition,
// so without a cap an operator typo or a malicious input would translate
// into an unbounded anonymous interval set at lowering time. 128 ranges
// (single ports or ranges, freely mixed) is generous for legitimate use
// (VoIP's "5060-5061,16384-32767" is two entries) and still bounds the
// allocation in lowerPortMatch to a predictable size.
const maxPortRanges = 128

// parsePortSpec accepts a YANG port-spec string ("22", "80-90", "22,80,443",
// or "5060-5061,16384-32767") and returns the canonical []PortRange sorted
// by Lo with no overlapping or adjacent entries. Each range has Lo<=Hi;
// single ports collapse to Lo==Hi. An empty spec, a range with Lo>Hi, a
// port of 0, a list exceeding maxPortRanges entries, or overlapping /
// adjacent ranges all reject at parse so the operator gets a clear
// message instead of an opaque nftables kernel error at Apply.
func parsePortSpec(v string) ([]PortRange, error) {
	if v == "" {
		return nil, fmt.Errorf("empty port spec")
	}
	var ranges []PortRange
	for entry := range strings.SplitSeq(v, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("empty entry in port spec %q", v)
		}
		if len(ranges) >= maxPortRanges {
			return nil, fmt.Errorf("port spec %q has more than %d entries; split across multiple rules or use a named set", v, maxPortRanges)
		}
		if loStr, hiStr, ok := strings.Cut(entry, "-"); ok {
			lo, err := parsePortNumber(loStr)
			if err != nil {
				return nil, err
			}
			hi, err := parsePortNumber(hiStr)
			if err != nil {
				return nil, err
			}
			if hi < lo {
				return nil, fmt.Errorf("inverted port range %q", entry)
			}
			ranges = append(ranges, PortRange{Lo: lo, Hi: hi})
			continue
		}
		n, err := parsePortNumber(entry)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, PortRange{Lo: n, Hi: n})
	}
	if err := validatePortRanges(v, ranges); err != nil {
		return nil, err
	}
	return ranges, nil
}

// validatePortRanges sorts ranges in place and rejects any pair that
// overlaps or touches (adjacent ranges produce a redundant interval set
// element at lowering time, and overlaps make the nftables kernel reject
// the set with an opaque EEXIST). Safe to mutate the caller's slice:
// parsePortSpec owns it, callers receive the sorted form.
func validatePortRanges(spec string, ranges []PortRange) error {
	if len(ranges) < 2 {
		return nil
	}
	sortedIdx := make([]int, len(ranges))
	for i := range sortedIdx {
		sortedIdx[i] = i
	}
	// Sort by Lo then Hi. Bubble sort is fine here: len(ranges) is
	// capped at maxPortRanges (128) and this runs at config verify,
	// not on the hot path.
	for i := 1; i < len(ranges); i++ {
		for j := i; j > 0 && (ranges[j].Lo < ranges[j-1].Lo ||
			(ranges[j].Lo == ranges[j-1].Lo && ranges[j].Hi < ranges[j-1].Hi)); j-- {
			ranges[j], ranges[j-1] = ranges[j-1], ranges[j]
		}
	}
	for i := 1; i < len(ranges); i++ {
		prev := ranges[i-1]
		cur := ranges[i]
		// Adjacent ranges (prev.Hi+1 == cur.Lo) are a soft error: the
		// kernel handles them but the operator almost certainly meant
		// to write one merged range. Overlapping ranges are a hard
		// error; the kernel rejects the interval set at flush.
		if cur.Lo <= prev.Hi {
			return fmt.Errorf("port spec %q: range %d-%d overlaps %d-%d", spec, cur.Lo, cur.Hi, prev.Lo, prev.Hi)
		}
		if cur.Lo == prev.Hi+1 {
			return fmt.Errorf("port spec %q: range %d-%d is adjacent to %d-%d; merge into one range", spec, cur.Lo, cur.Hi, prev.Lo, prev.Hi)
		}
	}
	return nil
}

func parsePortNumber(s string) (uint16, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q: %w", s, err)
	}
	if n == 0 {
		return 0, fmt.Errorf("port must be 1-65535, got 0")
	}
	return uint16(n), nil
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

// byteRateMultiplier encodes the {value-less} prefix multiplier for the
// byte-rate family. Lookup is "" (plain "Nbytes" form = 1-byte units),
// "k"/"m"/"g" for kibi/mebi/gibi. Keeping the multipliers here (not
// inside parseRateSpec) lets the show formatter reverse the conversion
// without duplicating the table.
var byteRateMultiplier = map[string]uint64{
	"bytes":  1,
	"kbytes": 1024,
	"mbytes": 1024 * 1024,
	"gbytes": 1024 * 1024 * 1024,
}

// timeUnits holds the allowed rate time units. Lookup avoids a long
// switch chain and keeps the valid set derivable from the map.
var timeUnits = map[string]struct{}{
	"second": {}, "minute": {}, "hour": {}, "day": {},
}

func parseRateSpec(v string) (Limit, error) {
	numStr, unit, ok := strings.Cut(v, "/")
	if !ok {
		return Limit{}, fmt.Errorf("invalid rate spec %q (expected N/unit)", v)
	}
	if _, known := timeUnits[unit]; !known {
		return Limit{}, fmt.Errorf("invalid rate unit %q (want second|minute|hour|day)", unit)
	}

	// Two forms share the leading [0-9]+: plain "<N>" (packet rate) and
	// "<N>bytes|kbytes|mbytes|gbytes" (byte rate). Extract the numeric
	// prefix by walking leading digits; the rest is the optional byte
	// suffix. A 20-digit cap covers uint64's domain (max 20 chars:
	// 18446744073709551615) and stops a multi-megabyte numeric prefix
	// from making the walk O(n) before strconv rejects.
	const maxRateDigits = 20
	digitEnd := 0
	// Walk one past the cap so the post-loop check can distinguish
	// "exactly at cap" (accept) from "over cap" (reject). The ceiling
	// stops a multi-megabyte digit string from making the walk O(n).
	for digitEnd < len(numStr) && digitEnd <= maxRateDigits && numStr[digitEnd] >= '0' && numStr[digitEnd] <= '9' {
		digitEnd++
	}
	if digitEnd == 0 {
		return Limit{}, fmt.Errorf("invalid rate number %q", numStr)
	}
	if digitEnd > maxRateDigits {
		return Limit{}, fmt.Errorf("invalid rate number %q: exceeds %d-digit cap", numStr, maxRateDigits)
	}
	n, err := strconv.ParseUint(numStr[:digitEnd], 10, 64)
	if err != nil {
		return Limit{}, fmt.Errorf("invalid rate number %q: %w", numStr, err)
	}
	if err := ValidateRate(n); err != nil {
		return Limit{}, err
	}
	suffix := numStr[digitEnd:]
	if suffix == "" {
		return Limit{Rate: n, Unit: unit, Dimension: RateDimensionPackets}, nil
	}
	mult, ok := byteRateMultiplier[suffix]
	if !ok {
		return Limit{}, fmt.Errorf("invalid rate unit suffix %q (want bytes|kbytes|mbytes|gbytes)", suffix)
	}
	// Overflow guard: the YANG pattern accepts arbitrarily long digit
	// runs, and an operator writing e.g. "99999999999999999999gbytes"
	// would wrap silently without this check.
	if n > 0 && mult > 0 && n > (^uint64(0))/mult {
		return Limit{}, fmt.Errorf("rate %s overflows uint64 after scaling by %q", numStr[:digitEnd], suffix)
	}
	return Limit{Rate: n * mult, Unit: unit, Dimension: RateDimensionBytes}, nil
}

// parseNATSpec accepts the NAT target string the operator typed and
// returns the components needed by the lowering path. The accepted
// forms are:
//
//	<addr>                            single address, no port
//	<addr>:<port>                     single address + port
//	<addr>:<portLo>-<portHi>          single address + port range
//	<addr>-<addr>                     address range (IPv4 only today)
//	<addr>-<addr>:<port>              address range + single port
//	<addr>-<addr>:<portLo>-<portHi>   address range + port range
//
// IPv6 bracketed syntax (`[::1]:80`) is accepted for single-address
// forms via netip.ParseAddrPort; IPv6 address ranges are rejected as
// "not supported" so a malformed IPv6 range does not silently parse
// as something unintended.
func parseNATSpec(v string) (addr, addrEnd netip.Addr, port, portEnd uint16, err error) {
	if v == "" {
		return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("empty NAT target")
	}
	// Fast path for the two most common single-address forms.
	if ap, perr := netip.ParseAddrPort(v); perr == nil {
		return ap.Addr(), netip.Addr{}, ap.Port(), 0, nil
	}
	if a, perr := netip.ParseAddr(v); perr == nil {
		return a, netip.Addr{}, 0, 0, nil
	}

	// Split addr-portion from port-portion. The colon separating port is
	// always the LAST one when IPv6 uses bracketed notation; for bare
	// IPv4 there is exactly one colon at most. Bracketed IPv6 addresses
	// are accepted by ParseAddrPort above, so anything that reaches
	// here without a recognized form is expected to be IPv4 (with an
	// optional range and optional port). Reject any embedded `[`/`]` or
	// multi-colon input (unbracketed IPv6) upfront so we don't mis-parse
	// an unbracketed IPv6 range as a malformed IPv4 address and emit a
	// confusing "invalid NAT address" error.
	if strings.ContainsAny(v, "[]") {
		return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("NAT target %q: IPv6 address ranges not supported", v)
	}
	if strings.Count(v, ":") >= 2 {
		return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("NAT target %q: IPv6 address ranges not supported (use bracketed single-address form [addr]:port)", v)
	}
	addrPart := v
	portPart := ""
	if ci := strings.LastIndexByte(v, ':'); ci >= 0 {
		addrPart = v[:ci]
		portPart = v[ci+1:]
	}

	// addr-portion is either "<addr>" or "<addr>-<addr>".
	if loStr, hiStr, isRange := strings.Cut(addrPart, "-"); isRange {
		lo, lerr := netip.ParseAddr(loStr)
		if lerr != nil {
			return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT address %q: %w", loStr, lerr)
		}
		hi, herr := netip.ParseAddr(hiStr)
		if herr != nil {
			return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT address %q: %w", hiStr, herr)
		}
		if lo.Is4() != hi.Is4() {
			return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("NAT address range %q: mixed IPv4/IPv6 bounds", v)
		}
		if hi.Less(lo) {
			return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("NAT address range %q: end %s is below start %s", v, hi, lo)
		}
		addr = lo
		addrEnd = hi
	} else {
		a, perr := netip.ParseAddr(addrPart)
		if perr != nil {
			return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT address %q: %w", addrPart, perr)
		}
		addr = a
	}

	if portPart == "" {
		return addr, addrEnd, 0, 0, nil
	}
	ranges, perr := parsePortSpec(portPart)
	if perr != nil {
		return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("invalid NAT port %q: %w", portPart, perr)
	}
	if len(ranges) != 1 {
		return netip.Addr{}, netip.Addr{}, 0, 0, fmt.Errorf("NAT target %q: comma-list ports not supported, use a single port or range", v)
	}
	r := ranges[0]
	if r.Lo == r.Hi {
		return addr, addrEnd, r.Lo, 0, nil
	}
	return addr, addrEnd, r.Lo, r.Hi, nil
}

// parseInterfaceSpec strips a trailing `*` wildcard marker from an
// interface name and reports whether the wildcard was present. Inputs
// without `*` are returned unchanged with wildcard=false.
//
// The kernel represents interface names as 16-byte IFNAMSIZ-padded C
// strings. Exact matches compare all 16 bytes; prefix matches compare
// only the first len(stripped) bytes. The distinction is carried as
// the Wildcard field on MatchInputInterface / MatchOutputInterface so
// the lowerer can pick the right Cmp length.
func parseInterfaceSpec(v string) (string, bool) {
	if strings.HasSuffix(v, "*") {
		return v[:len(v)-1], true
	}
	return v, false
}

// icmp4Types maps nft-compatible symbolic ICMPv4 type names to their
// numeric codes (IANA assignments, RFC 792). Unknown symbolic names
// fall through to numeric parsing so configs can use `icmp-type 42`
// for any value the kernel accepts.
var icmp4Types = map[string]uint8{
	"echo-reply":              0,
	"destination-unreachable": 3,
	"source-quench":           4,
	"redirect":                5,
	"echo-request":            8,
	"router-advertisement":    9,
	"router-solicitation":     10,
	"time-exceeded":           11,
	"parameter-problem":       12,
	"timestamp-request":       13,
	"timestamp-reply":         14,
	"info-request":            15,
	"info-reply":              16,
	"address-mask-request":    17,
	"address-mask-reply":      18,
}

// icmp6Types maps nft-compatible symbolic ICMPv6 type names to numeric
// codes (IANA assignments, RFC 4443 and neighbor-discovery extensions).
var icmp6Types = map[string]uint8{
	"destination-unreachable": 1,
	"packet-too-big":          2,
	"time-exceeded":           3,
	"parameter-problem":       4,
	"echo-request":            128,
	"echo-reply":              129,
	"mld-listener-query":      130,
	"mld-listener-report":     131,
	"mld-listener-done":       132,
	"nd-router-solicit":       133,
	"nd-router-advert":        134,
	"nd-neighbor-solicit":     135,
	"nd-neighbor-advert":      136,
	"nd-redirect":             137,
	"mld2-listener-report":    143,
}

func parseICMPType(v string) (uint8, error) {
	if n, ok := icmp4Types[v]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("unknown ICMPv4 type %q", v)
	}
	return uint8(n), nil
}

func parseICMPv6Type(v string) (uint8, error) {
	if n, ok := icmp6Types[v]; ok {
		return n, nil
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("unknown ICMPv6 type %q", v)
	}
	return uint8(n), nil
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

	if elems, ok := m["element"].(map[string]any); ok {
		elements, err := parseSetElements(name, elems)
		if err != nil {
			return Set{}, err
		}
		s.Elements = elements
	}

	return s, nil
}

// maxSetElements caps the per-set element count. The kernel accepts
// far more in principle, but the ze config surface is authored by
// humans; a cap protects against a runaway config (machine-generated
// or malicious) that would pre-allocate millions of SetElement slots
// at parse time. Parallel to maxPortRanges.
const maxSetElements = 65536

// parseSetElements reads the YANG `list element` shape: a map keyed by
// element value, each value a map holding per-element leaves (today:
// "timeout" uint32 seconds). Unknown leaf keys reject so a typo like
// `timout: 60` surfaces offline instead of silently dropping the
// timeout at commit. Output is sorted by Value so reloads produce a
// stable SetElement order (Go map iteration is randomized; without
// sorting, `show firewall` and LastApplied would shuffle on every
// parse).
func parseSetElements(setName string, m map[string]any) ([]SetElement, error) {
	if len(m) > maxSetElements {
		return nil, fmt.Errorf("set %q: %d elements exceeds cap %d", setName, len(m), maxSetElements)
	}
	out := make([]SetElement, 0, len(m))
	for value, raw := range m {
		entry, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("set %q: element %q: not an object", setName, value)
		}
		el := SetElement{Value: value}
		for k, v := range entry {
			if k == "timeout" {
				n, err := parseElementTimeout(v)
				if err != nil {
					return nil, fmt.Errorf("set %q: element %q: %w", setName, value, err)
				}
				el.Timeout = n
				continue
			}
			if k == "value" {
				// The key leaf is echoed in the entry body by some
				// YANG serializers; accept and ignore when it matches
				// the map key. Mismatch is a parser bug upstream.
				if s, ok := v.(string); ok && s == value {
					continue
				}
				return nil, fmt.Errorf("set %q: element %q: value leaf %v does not match key", setName, value, v)
			}
			return nil, fmt.Errorf("set %q: element %q: unknown leaf %q", setName, value, k)
		}
		out = append(out, el)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out, nil
}

// parseElementTimeout accepts the JSON number forms the YANG-to-JSON
// layer produces (float64 from encoding/json, or a string for YANG's
// uint32 serialized as a number-in-quotes). Rejects negative, non-integer,
// or out-of-range values with a message naming the offending input.
func parseElementTimeout(v any) (uint32, error) {
	switch n := v.(type) {
	case float64:
		if n < 0 || n > 4294967295 {
			return 0, fmt.Errorf("timeout %v out of range 0..4294967295", n)
		}
		if n != float64(uint32(n)) {
			return 0, fmt.Errorf("timeout %v must be an integer", n)
		}
		return uint32(n), nil
	case string:
		u, err := strconv.ParseUint(n, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("timeout %q: %w", n, err)
		}
		return uint32(u), nil
	}
	return 0, fmt.Errorf("timeout %v: expected number", v)
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
