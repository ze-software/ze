// Design: docs/architecture/core-design.md -- show firewall trace RPC handler
// Related: explain.go -- read-only firewall packet evaluator
// Related: yang/ze-firewall-cmd.yang -- YANG declaration for ze-firewall:show-trace

package firewall

import (
	"net/netip"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	traceLeafTable       = "table"
	traceLeafChain       = "chain"
	traceLeafProtocol    = "protocol"
	traceLeafSourceAddr  = "source-address"
	traceLeafDestAddr    = "destination-address"
	traceLeafSourcePort  = "source-port"
	traceLeafDestPort    = "destination-port"
	traceLeafInputIface  = "input-interface"
	traceLeafOutputIface = "output-interface"
	traceLeafConnState   = "connection-state"

	traceUsageFix     = "ze show firewall trace table <table-name> protocol tcp source-address <ip> destination-address <ip> destination-port <port>"
	traceErrPrefix    = "show firewall trace: "
	whyTracePortRange = "Transport ports must be whole numbers between 1 and 65535."

	traceProtoTCP    = "tcp"
	traceProtoUDP    = "udp"
	traceProtoICMP   = "icmp"
	traceProtoICMPv6 = "icmpv6"
)

func handleShowFirewallTrace(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	query, opErr := parseTraceQuery(validated)
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}

	result, opErr := EvaluatePacket(LastApplied(), query)
	if opErr != nil {
		return plugin.OperatorErrorResponse(opErr), nil
	}

	data := plugin.Map{
		"table":   result.Table,
		"family":  result.Family,
		"chain":   result.Chain,
		"verdict": result.Verdict,
		"steps":   result.Steps,
	}
	if result.MatchedChain != "" {
		data["matched-chain"] = result.MatchedChain
	}
	if result.MatchedTerm != "" {
		data["matched-term"] = result.MatchedTerm
	}
	if result.DefaultPolicy {
		data["default-policy"] = true
	}
	if result.Incomplete {
		data["incomplete"] = true
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   data,
	}, nil
}

func parseTraceQuery(validated command.ValidatedArgs) (TraceQuery, *opererr.Error) {
	var query TraceQuery
	var tb textbuf.Buffer

	table, ok := traceArg(validated, traceLeafTable)
	if !ok {
		return query, opererr.New(
			traceErrPrefix+"missing required table argument",
			"A table name is required to select which applied firewall ruleset to trace.",
			traceUsageFix,
		)
	}
	if table == "" {
		return query, opererr.New(
			traceErrPrefix+"table name must not be empty",
			"A table name is required to select which applied firewall ruleset to trace.",
			traceUsageFix,
		)
	}
	query.Table = table

	if chain, found := traceArg(validated, traceLeafChain); found {
		query.Chain = chain
	}

	if rawProto, found := traceArg(validated, traceLeafProtocol); found {
		proto := strings.ToLower(strings.TrimSpace(rawProto))
		switch proto {
		case traceProtoTCP, traceProtoUDP, traceProtoICMP, traceProtoICMPv6:
			query.Protocol = proto
		default:
			problem := tb.Reset().Str(traceErrPrefix).Str("invalid protocol ").Quoted(rawProto).String()
			return query, opererr.New(
				problem,
				"Supported trace protocols are tcp, udp, icmp, and icmpv6.",
				traceUsageFix,
			)
		}
	}

	if rawSrc, found := traceArg(validated, traceLeafSourceAddr); found {
		addr, err := netip.ParseAddr(strings.TrimSpace(rawSrc))
		if err != nil {
			problem := tb.Reset().Str(traceErrPrefix).Str("invalid source-address ").Quoted(rawSrc).String()
			return query, opererr.Wrap(
				err,
				problem,
				"The source-address must be a valid IPv4 or IPv6 host address.",
				traceUsageFix,
			)
		}
		query.SourceIP = addr
	}

	if rawDst, found := traceArg(validated, traceLeafDestAddr); found {
		addr, err := netip.ParseAddr(strings.TrimSpace(rawDst))
		if err != nil {
			problem := tb.Reset().Str(traceErrPrefix).Str("invalid destination-address ").Quoted(rawDst).String()
			return query, opererr.Wrap(
				err,
				problem,
				"The destination-address must be a valid IPv4 or IPv6 host address.",
				traceUsageFix,
			)
		}
		query.DestinationIP = addr
	}

	if rawSrcPort, found := traceArg(validated, traceLeafSourcePort); found {
		port, opErr := parseTracePort(traceLeafSourcePort, rawSrcPort)
		if opErr != nil {
			return query, opErr
		}
		query.SourcePort = port
	}

	if rawDstPort, found := traceArg(validated, traceLeafDestPort); found {
		port, opErr := parseTracePort(traceLeafDestPort, rawDstPort)
		if opErr != nil {
			return query, opErr
		}
		query.DestinationPort = port
	}

	if inIface, found := traceArg(validated, traceLeafInputIface); found {
		query.InInterface = inIface
	}
	if outIface, found := traceArg(validated, traceLeafOutputIface); found {
		query.OutInterface = outIface
	}

	if rawState, found := traceArg(validated, traceLeafConnState); found {
		stateName := strings.ToLower(strings.TrimSpace(rawState))
		cs, valid := connStateMap[stateName]
		if !valid {
			problem := tb.Reset().Str(traceErrPrefix).Str("invalid connection-state ").Quoted(rawState).String()
			return query, opererr.New(
				problem,
				"Supported connection-state values are new, established, related, and invalid.",
				traceUsageFix,
			)
		}
		query.ConnState = cs
	}

	return query, nil
}

func parseTracePort(leaf, raw string) (uint16, *opererr.Error) {
	var tb textbuf.Buffer
	trimmed := strings.TrimSpace(raw)
	n, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		problem := tb.Reset().Str(traceErrPrefix).Str("invalid ").Str(leaf).Byte(' ').Quoted(raw).String()
		return 0, opererr.Wrap(
			err,
			problem,
			whyTracePortRange,
			traceUsageFix,
		)
	}
	if n < 1 {
		problem := tb.Reset().Str(traceErrPrefix).Str(leaf).Byte(' ').Quoted(raw).Str(" is out of range").String()
		return 0, opererr.New(
			problem,
			whyTracePortRange,
			traceUsageFix,
		)
	}
	if n > 65535 {
		problem := tb.Reset().Str(traceErrPrefix).Str(leaf).Byte(' ').Quoted(raw).Str(" is out of range").String()
		return 0, opererr.New(
			problem,
			whyTracePortRange,
			traceUsageFix,
		)
	}
	return uint16(n), nil
}

func traceArg(validated command.ValidatedArgs, leaf string) (string, bool) {
	if value, found := validated.Keyed(leaf); found {
		return value, true
	}
	tokens := validated.Tokens()
	for i := 0; i+1 < len(tokens); i += 2 {
		if tokens[i] == leaf {
			return tokens[i+1], true
		}
	}
	return "", false
}
