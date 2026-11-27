// Design: docs/architecture/core-design.md -- read-only firewall packet decision trace
// Related: model.go -- firewall Table, Chain, Term, Match, and Action types
// Related: cmd_trace.go -- show firewall trace RPC handler

package firewall

import (
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// maxTraceDepth bounds jump/goto call depth to prevent cyclic chain loops.
	maxTraceDepth = 16

	verdictAccept             = "accept"
	verdictDrop               = "drop"
	verdictReject             = "reject"
	verdictReturn             = "return"
	verdictSNAT               = "snat"
	verdictDNAT               = "dnat"
	verdictMasquerade         = "masquerade"
	verdictRedirect           = "redirect"
	verdictDepthLimitExceeded = "depth-limit-exceeded"

	actionCounterKeyword = "counter"
	actionLogKeyword     = "log"
	actionNotrackKeyword = "notrack"

	reasonSourceAddrMissing = "source-address not provided"
	reasonDestAddrMissing   = "destination-address not provided"
	reasonSourcePortMissing = "source-port not provided"
	reasonDestPortMissing   = "destination-port not provided"
	reasonInIfaceMissing    = "input-interface not provided"
	reasonOutIfaceMissing   = "output-interface not provided"
	reasonProtocolMissing   = "protocol not provided"
	reasonConnStateMissing  = "connection-state not provided"
	reasonDSCPMissing       = "dscp not provided"
	reasonMarkMissing       = "mark not provided"

	whyTraceRequiresAppliedTable = "Packet tracing evaluates only tables currently applied by the firewall component."
	whyTraceRequiresChain        = "Packet tracing starts from a named chain or the lowest-priority base chain in the target table."
	fixShowFirewallConfig        = "ze show configuration firewall"

	msgDoesNotMatch = " does not match "
	msgNotInSet     = " not in set "
)

// TraceQuery describes a synthetic packet evaluated against applied firewall tables.
type TraceQuery struct {
	Table           string
	Chain           string
	Protocol        string
	SourceIP        netip.Addr
	DestinationIP   netip.Addr
	SourcePort      uint16
	DestinationPort uint16
	InInterface     string
	OutInterface    string
	ConnState       ConnState
	DSCP            *uint8
	Mark            *uint32
}

// TraceStep records the evaluation outcome of one term during a packet trace.
type TraceStep struct {
	Chain       string   `json:"chain"`
	Term        string   `json:"term"`
	Matched     bool     `json:"matched"`
	Unevaluated bool     `json:"unevaluated,omitempty"`
	Actions     []string `json:"actions,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

// TraceResult is the structured decision trace for EvaluatePacket.
type TraceResult struct {
	Table         string      `json:"table"`
	Family        string      `json:"family"`
	Chain         string      `json:"chain"`
	Verdict       string      `json:"verdict"`
	MatchedChain  string      `json:"matched-chain,omitempty"`
	MatchedTerm   string      `json:"matched-term,omitempty"`
	DefaultPolicy bool        `json:"default-policy,omitempty"`
	Incomplete    bool        `json:"incomplete,omitempty"`
	Steps         []TraceStep `json:"steps"`
}

type traceOutcome uint8

const (
	outcomeFallthrough traceOutcome = iota
	outcomeReturn
	outcomeTerminal
	outcomeDepthExceeded
)

type matchOutcome uint8

const (
	matchHit matchOutcome = iota
	matchMiss
	matchUnevaluated
)

type traceState struct {
	chains map[string]*Chain
	sets   map[string]*Set
	query  TraceQuery
	result *TraceResult
}

// EvaluatePacket performs a read-only evaluation of a synthetic packet against
// applied firewall tables and returns the step-by-step decision trace.
func EvaluatePacket(tables []Table, query TraceQuery) (*TraceResult, *opererr.Error) {
	table, opErr := findTraceTable(tables, query.Table)
	if opErr != nil {
		return nil, opErr
	}

	chainsByName := make(map[string]*Chain, len(table.Chains))
	for i := range table.Chains {
		chainsByName[table.Chains[i].Name] = &table.Chains[i]
	}

	entryChain, opErr := selectEntryChain(table, chainsByName, query.Chain)
	if opErr != nil {
		return nil, opErr
	}

	setsByName := make(map[string]*Set, len(table.Sets))
	for i := range table.Sets {
		setsByName[table.Sets[i].Name] = &table.Sets[i]
	}

	bareTable := StripZeTablePrefix(table.Name)
	result := &TraceResult{
		Table:  bareTable,
		Family: table.Family.String(),
		Chain:  entryChain.Name,
		Steps:  []TraceStep{},
	}

	state := &traceState{
		chains: chainsByName,
		sets:   setsByName,
		query:  cloneTraceQuery(&query),
		result: result,
	}

	// Bounded internal recursion: evalChain enforces maxTraceDepth (16).
	outcome := evalChain(state, entryChain, 0)
	switch outcome {
	case outcomeTerminal, outcomeDepthExceeded:
		return result, nil
	case outcomeFallthrough, outcomeReturn:
		if entryChain.IsBase {
			result.Verdict = entryChain.Policy.String()
			result.DefaultPolicy = true
			return result, nil
		}
		result.Verdict = verdictReturn
		return result, nil
	}
	return result, nil
}

func cloneTraceQuery(src *TraceQuery) TraceQuery {
	dst := *src
	if src.DSCP != nil {
		v := *src.DSCP
		dst.DSCP = &v
	}
	if src.Mark != nil {
		v := *src.Mark
		dst.Mark = &v
	}
	return dst
}

func findTraceTable(tables []Table, wanted string) (*Table, *opererr.Error) {
	names := make([]string, 0, len(tables))
	for i := range tables {
		bare := StripZeTablePrefix(tables[i].Name)
		if bare == wanted {
			return &tables[i], nil
		}
		names = append(names, bare)
	}
	slices.Sort(names)

	var tb textbuf.Buffer
	if len(names) == 0 {
		problem := tb.Reset().Str("firewall table ").Quoted(wanted).Str(" not found; no firewall tables are applied").String()
		return nil, opererr.New(problem, whyTraceRequiresAppliedTable, fixShowFirewallConfig)
	}
	problem := tb.Reset().Str("firewall table ").Quoted(wanted).Str(" not found; applied tables: ").Join(names, ", ").String()
	fix := tb.Reset().Str("ze show firewall trace table ").Str(names[0]).String()
	return nil, opererr.New(problem, whyTraceRequiresAppliedTable, fix)
}

func selectEntryChain(table *Table, chainsByName map[string]*Chain, wantedChain string) (*Chain, *opererr.Error) {
	bareTable := StripZeTablePrefix(table.Name)
	var tb textbuf.Buffer
	if wantedChain != "" {
		if ch, ok := chainsByName[wantedChain]; ok {
			return ch, nil
		}
		names := make([]string, 0, len(table.Chains))
		for i := range table.Chains {
			names = append(names, table.Chains[i].Name)
		}
		slices.Sort(names)
		if len(names) == 0 {
			problem := tb.Reset().Str("chain ").Quoted(wantedChain).Str(" not found in table ").Quoted(bareTable).Str("; table has no chains").String()
			return nil, opererr.New(problem, whyTraceRequiresChain, fixShowFirewallConfig)
		}
		problem := tb.Reset().Str("chain ").Quoted(wantedChain).Str(" not found in table ").Quoted(bareTable).Str("; available chains: ").Join(names, ", ").String()
		fix := tb.Reset().Str("ze show firewall trace table ").Str(bareTable).Str(" chain ").Str(names[0]).String()
		return nil, opererr.New(problem, whyTraceRequiresChain, fix)
	}

	baseChains := make([]*Chain, 0, len(table.Chains))
	for i := range table.Chains {
		if table.Chains[i].IsBase {
			baseChains = append(baseChains, &table.Chains[i])
		}
	}
	if len(baseChains) == 0 {
		names := make([]string, 0, len(table.Chains))
		for i := range table.Chains {
			names = append(names, table.Chains[i].Name)
		}
		slices.Sort(names)
		if len(names) == 0 {
			problem := tb.Reset().Str("table ").Quoted(bareTable).Str(" has no base chains to trace").String()
			return nil, opererr.New(problem, whyTraceRequiresChain, fixShowFirewallConfig)
		}
		problem := tb.Reset().Str("table ").Quoted(bareTable).Str(" has no base chains; specify a chain from: ").Join(names, ", ").String()
		fix := tb.Reset().Str("ze show firewall trace table ").Str(bareTable).Str(" chain ").Str(names[0]).String()
		return nil, opererr.New(problem, whyTraceRequiresChain, fix)
	}

	slices.SortFunc(baseChains, func(a, b *Chain) int {
		if a.Priority < b.Priority {
			return -1
		}
		if a.Priority > b.Priority {
			return 1
		}
		return strings.Compare(a.Name, b.Name)
	})
	return baseChains[0], nil
}

// evalChain walks terms in chain up to maxTraceDepth (16) nested calls.
func evalChain(state *traceState, chain *Chain, depth int) traceOutcome {
	if depth >= maxTraceDepth {
		state.result.Verdict = verdictDepthLimitExceeded
		state.result.Incomplete = true
		return outcomeDepthExceeded
	}

	for i := range chain.Terms {
		term := &chain.Terms[i]
		outcome := evalTerm(state, chain, term, depth)
		switch outcome {
		case outcomeTerminal, outcomeDepthExceeded, outcomeReturn:
			return outcome
		case outcomeFallthrough:
		}
	}
	return outcomeFallthrough
}

func evalTerm(state *traceState, chain *Chain, term *Term, depth int) traceOutcome {
	step := TraceStep{
		Chain: chain.Name,
		Term:  term.Name,
	}

	matchRes, reason := evalTermMatches(term.Matches, &state.query, state.sets)
	switch matchRes {
	case matchMiss:
		step.Reason = reason
		state.result.Steps = append(state.result.Steps, step)
		return outcomeFallthrough
	case matchUnevaluated:
		step.Unevaluated = true
		step.Reason = reason
		state.result.Incomplete = true
		state.result.Steps = append(state.result.Steps, step)
		return outcomeFallthrough
	case matchHit:
	}

	step.Matched = true
	return evalTermActions(state, chain, term, &step, depth)
}

func evalTermMatches(matches []Match, query *TraceQuery, sets map[string]*Set) (matchOutcome, string) {
	unevaluatedReason := ""
	for _, m := range matches {
		outcome, reason := evalMatch(m, query, sets)
		switch outcome {
		case matchMiss:
			return matchMiss, reason
		case matchUnevaluated:
			if unevaluatedReason == "" {
				unevaluatedReason = reason
			}
		case matchHit:
		}
	}
	if unevaluatedReason != "" {
		return matchUnevaluated, unevaluatedReason
	}
	return matchHit, ""
}

func evalTermActions(state *traceState, chain *Chain, term *Term, step *TraceStep, depth int) traceOutcome {
	var tb textbuf.Buffer
	stepAppended := false

	flushStep := func() {
		if stepAppended {
			return
		}
		state.result.Steps = append(state.result.Steps, *step)
		stepAppended = true
	}

	for _, action := range term.Actions {
		if stepAppended {
			*step = TraceStep{
				Chain:   chain.Name,
				Term:    term.Name,
				Matched: true,
			}
			stepAppended = false
		}

		switch a := action.(type) {
		case Accept:
			step.Actions = append(step.Actions, verdictAccept)
			flushStep()
			return recordTerminalVerdict(state.result, verdictAccept, chain.Name, term.Name)
		case Drop:
			step.Actions = append(step.Actions, verdictDrop)
			flushStep()
			return recordTerminalVerdict(state.result, verdictDrop, chain.Name, term.Name)
		case Reject:
			step.Actions = append(step.Actions, formatTraceReject(a))
			flushStep()
			return recordTerminalVerdict(state.result, verdictReject, chain.Name, term.Name)
		case SNAT:
			step.Actions = append(step.Actions, formatTraceNAT(verdictSNAT, a.Address, a.AddressEnd, a.Port, a.PortEnd))
			flushStep()
			return recordTerminalVerdict(state.result, verdictSNAT, chain.Name, term.Name)
		case DNAT:
			step.Actions = append(step.Actions, formatTraceNAT(verdictDNAT, a.Address, a.AddressEnd, a.Port, a.PortEnd))
			flushStep()
			return recordTerminalVerdict(state.result, verdictDNAT, chain.Name, term.Name)
		case Masquerade:
			step.Actions = append(step.Actions, verdictMasquerade)
			flushStep()
			return recordTerminalVerdict(state.result, verdictMasquerade, chain.Name, term.Name)
		case Redirect:
			step.Actions = append(step.Actions, formatTraceRedirect(a))
			flushStep()
			return recordTerminalVerdict(state.result, verdictRedirect, chain.Name, term.Name)
		case Return:
			step.Actions = append(step.Actions, verdictReturn)
			flushStep()
			return outcomeReturn
		case Jump:
			step.Actions = append(step.Actions, tb.Reset().Str("jump ").Str(a.Target).String())
			target, ok := state.chains[a.Target]
			if !ok {
				step.Unevaluated = true
				step.Reason = tb.Reset().Str("jump target chain ").Quoted(a.Target).Str(" not found").String()
				state.result.Incomplete = true
				flushStep()
				continue
			}
			flushStep()
			sub := evalChain(state, target, depth+1)
			if sub == outcomeTerminal {
				return outcomeTerminal
			}
			if sub == outcomeDepthExceeded {
				return outcomeDepthExceeded
			}
		case Goto:
			step.Actions = append(step.Actions, tb.Reset().Str("goto ").Str(a.Target).String())
			target, ok := state.chains[a.Target]
			if !ok {
				step.Unevaluated = true
				step.Reason = tb.Reset().Str("goto target chain ").Quoted(a.Target).Str(" not found").String()
				state.result.Incomplete = true
				flushStep()
				return outcomeFallthrough
			}
			flushStep()
			sub := evalChain(state, target, depth+1)
			if sub == outcomeFallthrough {
				return outcomeReturn
			}
			return sub
		case SetMark:
			applyTraceSetMark(&state.query, a)
			step.Actions = append(step.Actions, tb.Reset().Str("set-mark 0x").Str(formatHexUint32(a.Value)).String())
		case SetConnMark:
			step.Actions = append(step.Actions, tb.Reset().Str("set-connection-mark 0x").Str(formatHexUint32(a.Value)).String())
		case SetDSCP:
			v := a.Value
			state.query.DSCP = &v
			step.Actions = append(step.Actions, textbuf.StrUint("set-dscp ", uint64(a.Value)))
		case SetTCPMSS:
			step.Actions = append(step.Actions, textbuf.StrUint("set-tcp-mss ", uint64(a.Size)))
		case ClampTCPMSSToPMTU:
			step.Actions = append(step.Actions, "clamp-tcp-mss-to-pmtu")
		case Counter:
			if a.Name == "" {
				step.Actions = append(step.Actions, actionCounterKeyword)
			} else {
				step.Actions = append(step.Actions, tb.Reset().Str("counter ").Str(a.Name).String())
			}
		case Log:
			if a.Prefix == "" {
				step.Actions = append(step.Actions, actionLogKeyword)
			} else {
				step.Actions = append(step.Actions, tb.Reset().Str(actionLogKeyword).Str(" prefix ").Quoted(a.Prefix).String())
			}
		case Limit:
			step.Actions = append(step.Actions, tb.Reset().Str("limit ").Uint(a.Rate).Byte('/').Str(a.Unit).String())
		case Notrack:
			step.Actions = append(step.Actions, actionNotrackKeyword)
		case FlowOffload:
			step.Actions = append(step.Actions, tb.Reset().Str("flow-offload ").Str(a.FlowtableName).String())
		case SetCtHelper:
			step.Actions = append(step.Actions, tb.Reset().Str("set-ct-helper ").Str(a.Name).String())
		}
	}

	flushStep()
	return outcomeFallthrough
}

func recordTerminalVerdict(result *TraceResult, verdict, chainName, termName string) traceOutcome {
	result.Verdict = verdict
	result.MatchedChain = chainName
	result.MatchedTerm = termName
	return outcomeTerminal
}

func applyTraceSetMark(query *TraceQuery, action SetMark) {
	var current uint32
	if query.Mark != nil {
		current = *query.Mark
	}
	mask := action.Mask
	if mask == 0 {
		mask = 0xFFFFFFFF
	}
	updated := (current & ^mask) | (action.Value & mask)
	query.Mark = &updated
}

func evalMatch(m Match, query *TraceQuery, sets map[string]*Set) (matchOutcome, string) {
	var tb textbuf.Buffer
	switch v := m.(type) {
	case MatchSourceAddress:
		if !query.SourceIP.IsValid() {
			return matchMiss, reasonSourceAddrMissing
		}
		if !v.Prefix.Contains(query.SourceIP) {
			return matchMiss, tb.Reset().Str("source-address ").Addr(query.SourceIP).Str(" not in ").Prefix(v.Prefix).String()
		}
		return matchHit, ""
	case MatchDestinationAddress:
		if !query.DestinationIP.IsValid() {
			return matchMiss, reasonDestAddrMissing
		}
		if !v.Prefix.Contains(query.DestinationIP) {
			return matchMiss, tb.Reset().Str("destination-address ").Addr(query.DestinationIP).Str(" not in ").Prefix(v.Prefix).String()
		}
		return matchHit, ""
	case MatchProtocol:
		if query.Protocol == "" {
			return matchMiss, reasonProtocolMissing
		}
		if !strings.EqualFold(query.Protocol, v.Protocol) {
			return matchMiss, tb.Reset().Str("protocol ").Str(query.Protocol).Str(" != ").Str(v.Protocol).String()
		}
		return matchHit, ""
	case MatchSourcePort:
		if query.SourcePort == 0 {
			return matchMiss, reasonSourcePortMissing
		}
		if !portInRanges(query.SourcePort, v.Ranges) {
			return matchMiss, tb.Reset().Str("source-port ").Uint16(query.SourcePort).Str(" not in matched ranges").String()
		}
		return matchHit, ""
	case MatchDestinationPort:
		if query.DestinationPort == 0 {
			return matchMiss, reasonDestPortMissing
		}
		if !portInRanges(query.DestinationPort, v.Ranges) {
			return matchMiss, tb.Reset().Str("destination-port ").Uint16(query.DestinationPort).Str(" not in matched ranges").String()
		}
		return matchHit, ""
	case MatchInputInterface:
		if query.InInterface == "" {
			return matchMiss, reasonInIfaceMissing
		}
		if !ifaceMatches(query.InInterface, v.Name, v.Wildcard) {
			return matchMiss, tb.Reset().Str("input-interface ").Str(query.InInterface).Str(msgDoesNotMatch).Str(v.Name).String()
		}
		return matchHit, ""
	case MatchOutputInterface:
		if query.OutInterface == "" {
			return matchMiss, reasonOutIfaceMissing
		}
		if !ifaceMatches(query.OutInterface, v.Name, v.Wildcard) {
			return matchMiss, tb.Reset().Str("output-interface ").Str(query.OutInterface).Str(msgDoesNotMatch).Str(v.Name).String()
		}
		return matchHit, ""
	case MatchConnState:
		if query.ConnState == 0 {
			return matchMiss, reasonConnStateMissing
		}
		if query.ConnState&v.States == 0 {
			return matchMiss, "connection-state mismatch"
		}
		return matchHit, ""
	case MatchDSCP:
		if query.DSCP == nil {
			return matchMiss, reasonDSCPMissing
		}
		if *query.DSCP != v.Value {
			return matchMiss, tb.Reset().Str("dscp ").Uint8(*query.DSCP).Str(" != ").Uint8(v.Value).String()
		}
		return matchHit, ""
	case MatchMark:
		if query.Mark == nil {
			return matchMiss, reasonMarkMissing
		}
		mask := v.Mask
		if mask == 0 {
			mask = 0xFFFFFFFF
		}
		if (*query.Mark & mask) != v.Value {
			return matchMiss, tb.Reset().Str("mark 0x").Str(formatHexUint32(*query.Mark)).Str(msgDoesNotMatch).Str("0x").Str(formatHexUint32(v.Value)).String()
		}
		return matchHit, ""
	case MatchInSet:
		return evalMatchInSet(v, query, sets)
	case MatchDestinationLocal:
		return matchUnevaluated, "unevaluated predicate: destination-local"
	case MatchUnicast:
		return matchUnevaluated, "unevaluated predicate: unicast"
	case MatchIPsecTransform:
		return matchUnevaluated, "unevaluated predicate: ipsec-transform"
	case MatchConnMark:
		return matchUnevaluated, "unevaluated predicate: connection-mark"
	case MatchICMPType:
		return matchUnevaluated, "unevaluated predicate: icmp-type"
	case MatchICMPv6Type:
		return matchUnevaluated, "unevaluated predicate: icmpv6-type"
	case MatchTCPFlags:
		return matchUnevaluated, "unevaluated predicate: tcp-flags"
	case MatchIPv4TTLBelow:
		return matchUnevaluated, "unevaluated predicate: ipv4-ttl-below"
	case MatchIPv6HopLimitBelow:
		return matchUnevaluated, "unevaluated predicate: ipv6-hop-limit-below"
	case MatchICMPErrorQuotedTCPPort:
		return matchUnevaluated, "unevaluated predicate: icmp-error-quoted-tcp-port"
	case MatchICMPErrorQuotedDestination:
		return matchUnevaluated, "unevaluated predicate: icmp-error-quoted-destination"
	case MatchARPOperation:
		return matchUnevaluated, "unevaluated predicate: arp-operation"
	case MatchARPSenderAddress:
		return matchUnevaluated, "unevaluated predicate: arp-sender-address"
	case MatchNDTargetAddress:
		return matchUnevaluated, "unevaluated predicate: nd-target-address"
	}
	return matchUnevaluated, "unevaluated predicate: unknown match type"
}

func evalMatchInSet(m MatchInSet, query *TraceQuery, sets map[string]*Set) (matchOutcome, string) {
	var tb textbuf.Buffer
	set, ok := sets[m.SetName]
	if !ok {
		return matchUnevaluated, tb.Reset().Str("set ").Quoted(m.SetName).Str(" not found in table").String()
	}

	switch set.Type {
	case SetTypeIPv4, SetTypeIPv6:
		return evalIPSetMatch(m, set, query)
	case SetTypeInetService:
		return evalPortSetMatch(m, set, query)
	case SetTypeIfname:
		return evalIfnameSetMatch(m, set, query)
	case SetTypeEther, SetTypeMark:
		return matchUnevaluated, tb.Reset().Str("unevaluated set type ").Str(set.Type.String()).Str(" for set ").Quoted(set.Name).String()
	default:
		// Open numeric SetType; unknown values cannot be evaluated offline.
		return matchUnevaluated, tb.Reset().Str("unevaluated set type ").Str(set.Type.String()).Str(" for set ").Quoted(set.Name).String()
	}
}

func evalIPSetMatch(m MatchInSet, set *Set, query *TraceQuery) (matchOutcome, string) {
	var ip netip.Addr
	var missingReason string
	switch m.MatchField {
	case SetFieldSourceAddr:
		ip = query.SourceIP
		missingReason = reasonSourceAddrMissing
	case SetFieldDestAddr:
		ip = query.DestinationIP
		missingReason = reasonDestAddrMissing
	case SetFieldSourcePort, SetFieldDestPort:
		return matchUnevaluated, "unevaluated set field for address set"
	default:
		// Open SetFieldType; unknown fields cannot be evaluated offline.
		return matchUnevaluated, "unevaluated set field for address set"
	}

	if !ip.IsValid() {
		return matchMiss, missingReason
	}
	if set.Type == SetTypeIPv4 {
		if !ip.Is4() {
			return matchMiss, "address family mismatch for ipv4 set"
		}
	}
	if set.Type == SetTypeIPv6 {
		if !ip.Is6() {
			return matchMiss, "address family mismatch for ipv6 set"
		}
		if ip.Is4In6() {
			return matchMiss, "address family mismatch for ipv6 set"
		}
	}

	if set.RawIntervals {
		return matchIPSetResult(ipInRawIntervals(ip, set.Elements), ip, set.Name)
	}
	if hasIntervalEnd(set.Elements) {
		return matchIPSetResult(ipInRawIntervals(ip, set.Elements), ip, set.Name)
	}
	return matchIPSetResult(ipInStandardElements(ip, set.Elements), ip, set.Name)
}

func matchIPSetResult(matched bool, ip netip.Addr, setName string) (matchOutcome, string) {
	if matched {
		return matchHit, ""
	}
	var tb textbuf.Buffer
	return matchMiss, tb.Str("address ").Addr(ip).Str(msgNotInSet).Quoted(setName).String()
}

func ipInRawIntervals(ip netip.Addr, elems []SetElement) bool {
	inRange := false
	var rangeStart netip.Addr
	for _, elem := range elems {
		bound, err := netip.ParseAddr(elem.Value)
		if err != nil {
			continue
		}
		if elem.IntervalEnd {
			if inRange {
				if ip.Compare(rangeStart) >= 0 {
					if ip.Compare(bound) < 0 {
						return true
					}
				}
				inRange = false
			}
			continue
		}
		rangeStart = bound
		inRange = true
	}
	if inRange {
		return ip.Compare(rangeStart) >= 0
	}
	return false
}

func ipInStandardElements(ip netip.Addr, elems []SetElement) bool {
	for _, elem := range elems {
		if strings.Contains(elem.Value, "/") {
			prefix, err := netip.ParsePrefix(elem.Value)
			if err != nil {
				continue
			}
			if prefix.Contains(ip) {
				return true
			}
			continue
		}
		addr, err := netip.ParseAddr(elem.Value)
		if err != nil {
			continue
		}
		if addr == ip {
			return true
		}
	}
	return false
}

func evalPortSetMatch(m MatchInSet, set *Set, query *TraceQuery) (matchOutcome, string) {
	var port uint16
	var missingReason string
	switch m.MatchField {
	case SetFieldSourcePort:
		port = query.SourcePort
		missingReason = reasonSourcePortMissing
	case SetFieldDestPort:
		port = query.DestinationPort
		missingReason = reasonDestPortMissing
	case SetFieldSourceAddr, SetFieldDestAddr:
		return matchUnevaluated, "unevaluated set field for port set"
	default:
		// Open SetFieldType; unknown fields cannot be evaluated offline.
		return matchUnevaluated, "unevaluated set field for port set"
	}

	if port == 0 {
		return matchMiss, missingReason
	}
	for _, elem := range set.Elements {
		ranges, err := ParsePortSpec(elem.Value)
		if err != nil {
			continue
		}
		if portInRanges(port, ranges) {
			return matchHit, ""
		}
	}
	var tb textbuf.Buffer
	return matchMiss, tb.Str("port ").Uint16(port).Str(msgNotInSet).Quoted(set.Name).String()
}

func evalIfnameSetMatch(m MatchInSet, set *Set, query *TraceQuery) (matchOutcome, string) {
	var iface string
	var missingReason string
	switch m.MatchField {
	case SetFieldSourceAddr, SetFieldSourcePort:
		iface = query.InInterface
		missingReason = reasonInIfaceMissing
	case SetFieldDestAddr, SetFieldDestPort:
		iface = query.OutInterface
		missingReason = reasonOutIfaceMissing
	default:
		// Open SetFieldType; unknown fields cannot be evaluated offline.
		return matchUnevaluated, "unevaluated set field for interface set"
	}

	if iface == "" {
		return matchMiss, missingReason
	}
	for _, elem := range set.Elements {
		if prefix, isWildcard := strings.CutSuffix(elem.Value, "*"); isWildcard {
			if strings.HasPrefix(iface, prefix) {
				return matchHit, ""
			}
			continue
		}
		if elem.Value == iface {
			return matchHit, ""
		}
	}
	var tb textbuf.Buffer
	return matchMiss, tb.Str("interface ").Str(iface).Str(msgNotInSet).Quoted(set.Name).String()
}

func portInRanges(port uint16, ranges []PortRange) bool {
	for _, r := range ranges {
		if port < r.Lo {
			continue
		}
		if port > r.Hi {
			continue
		}
		return true
	}
	return false
}

func ifaceMatches(actual, pattern string, wildcard bool) bool {
	if wildcard {
		return strings.HasPrefix(actual, pattern)
	}
	return actual == pattern
}

func formatTraceReject(a Reject) string {
	if a.Type == "" {
		return verdictReject
	}
	var tb textbuf.Buffer
	return tb.Str("reject with ").Str(a.Type).String()
}

func formatTraceRedirect(a Redirect) string {
	if a.Port == 0 {
		return verdictRedirect
	}
	return textbuf.StrUint("redirect to ", uint64(a.Port))
}

func formatTraceNAT(verb string, addr, addrEnd netip.Addr, port, portEnd uint16) string {
	if !addr.IsValid() {
		return verb
	}
	var tb textbuf.Buffer
	tb.Str(verb).Str(" to ").Addr(addr)
	if addrEnd.IsValid() {
		tb.Byte('-').Addr(addrEnd)
	}
	if port != 0 {
		tb.Byte(':').Uint16(port)
		if portEnd != 0 {
			tb.Byte('-').Uint16(portEnd)
		}
	}
	return tb.String()
}

func formatHexUint32(v uint32) string {
	var buf [8]byte
	return string(strconv.AppendUint(buf[:0], uint64(v), 16))
}
