// Design: docs/architecture/diagnostics/production-diagnostics.md -- show system bad-packets handler
// Related: show.go -- ze-cmd:show-system-bad-packets registration and pipe filter declaration
// Related: op_pipe.go -- shared single-text pipe filter extractor

package show

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/badpacket"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	pipeOpProtocol         = "protocol"
	badPacketsCountExample = "ze show system bad-packets count 16"
	badPacketsPipeExample  = "ze show system bad-packets | protocol bgp"
)

// protocolPipeFilter declares the command-owned `| protocol <name>` pipe
// operation on `show system bad-packets`.
var protocolPipeFilter = command.PipeFilter{
	Name:        pipeOpProtocol,
	Description: "Filter captured bad packets to one protocol",
	Argument:    command.ArgText,
}

func extractProtocolPipeTarget(validated command.ValidatedArgs) (protocol string, present bool, refusal *plugin.Response) {
	return extractSingleTextPipeTarget(
		validated,
		pipeOpProtocol,
		"duplicate | protocol pipe operation",
		"only one | protocol filter may be given on a command line",
		"| protocol requires a protocol name",
		"a protocol name must follow | protocol",
		"| protocol takes exactly one protocol name",
		"only one protocol name may follow | protocol",
		badPacketsPipeExample,
	)
}

// handleShowSystemBadPackets returns the bounded circular ring of captured
// offending wire frames, newest first, optionally limited by `count <1..16>`
// and filtered by `| protocol <name>`.
func handleShowSystemBadPackets(_ *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	protocol, _, refusal := extractProtocolPipeTarget(validated)
	if refusal != nil {
		return refusal, nil
	}

	limit, countRefusal := parseBadPacketsLimit(validated)
	if countRefusal != nil {
		return countRefusal, nil
	}

	records := badpacket.Snapshot(limit, protocol)
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"packets":        records,
			keyCount:         len(records),
			"total-recorded": badpacket.Count(),
			"capacity":       badpacket.MaxRecords,
		},
	}, nil
}

func parseBadPacketsLimit(validated command.ValidatedArgs) (int, *plugin.Response) {
	if raw, ok := validated.Keyed(argCount); ok {
		return parseBadPacketsCountValue(raw)
	}
	args := validated.Tokens()
	limit := badpacket.MaxRecords
	for i := 0; i < len(args); i++ {
		if args[i] != argCount {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system bad-packets: unknown option "+strconv.Quote(args[i]),
				"only the optional count filter is accepted on this command",
				badPacketsCountExample,
			))
		}
		if i+1 >= len(args) {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system bad-packets: count requires a value",
				"provide an integer between 1 and 16 after count",
				badPacketsCountExample,
			))
		}
		i++
		n, refusal := parseBadPacketsCountValue(args[i])
		if refusal != nil {
			return 0, refusal
		}
		limit = n
	}
	return limit, nil
}

func parseBadPacketsCountValue(raw string) (int, *plugin.Response) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, plugin.OperatorErrorResponse(opererr.New(
			"show system bad-packets: invalid count "+strconv.Quote(raw),
			"count must be an integer between 1 and 16",
			badPacketsCountExample,
		))
	}
	if n < 1 {
		return 0, plugin.OperatorErrorResponse(opererr.New(
			"show system bad-packets: count "+strconv.Itoa(n)+" is below 1",
			"count must be an integer between 1 and 16",
			badPacketsCountExample,
		))
	}
	if n > badpacket.MaxRecords {
		return 0, plugin.OperatorErrorResponse(opererr.New(
			"show system bad-packets: count "+strconv.Itoa(n)+" exceeds 16",
			"count must be an integer between 1 and 16",
			badPacketsCountExample,
		))
	}
	return n, nil
}
