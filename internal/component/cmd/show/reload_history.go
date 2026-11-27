// Design: docs/architecture/diagnostics/production-diagnostics.md -- show system reload history handler
// Related: reload_status.go -- show system reload status handler
// Related: op_pipe.go -- command-owned | operation <id> pipe filter

package show

import (
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	defaultReloadHistoryLimit = 32
	maxReloadHistoryLimit     = 32
	reloadHistoryExample      = "ze show system reload history count 10"
	reloadHistoryOpExample    = "ze show system reload history | operation <id>"
)

// handleShowSystemReloadHistory returns the bounded circular history of recent
// configuration reload transactions, newest first, optionally limited by
// `count <1..32>` and filtered by `| operation <id>`.
func handleShowSystemReloadHistory(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	opID, _, refusal := extractOperationPipeTarget(validated, reloadHistoryOpExample)
	if refusal != nil {
		return refusal, nil
	}

	limit, countRefusal := parseReloadHistoryLimit(validated.Tokens())
	if countRefusal != nil {
		return countRefusal, nil
	}

	if ctx == nil {
		return emptyReloadHistoryResponse(), nil
	}
	if ctx.Server == nil {
		return emptyReloadHistoryResponse(), nil
	}

	entries := ctx.Server.ReloadHistory(limit, opID)
	if entries == nil {
		entries = []pluginserver.ReloadRecord{}
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			triageKeyEntries: entries,
			keyCount:         len(entries),
		},
	}, nil
}

func emptyReloadHistoryResponse() *plugin.Response {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			triageKeyEntries: []pluginserver.ReloadRecord{},
			keyCount:         0,
		},
	}
}

func parseReloadHistoryLimit(args []string) (int, *plugin.Response) {
	limit := defaultReloadHistoryLimit
	for i := 0; i < len(args); i++ {
		if args[i] != argCount {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system reload history: unknown option "+strconv.Quote(args[i]),
				"only the optional count filter is accepted on this command",
				reloadHistoryExample,
			))
		}
		if i+1 >= len(args) {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system reload history: count requires a value",
				"provide a positive integer between 1 and 32 after count",
				reloadHistoryExample,
			))
		}
		i++
		n, err := strconv.Atoi(args[i])
		if err != nil {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system reload history: invalid count "+strconv.Quote(args[i]),
				"count must be a positive integer between 1 and 32",
				reloadHistoryExample,
			))
		}
		if n < 1 {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system reload history: count "+strconv.Itoa(n)+" is below 1",
				"count must be a positive integer between 1 and 32",
				reloadHistoryExample,
			))
		}
		if n > maxReloadHistoryLimit {
			return 0, plugin.OperatorErrorResponse(opererr.New(
				"show system reload history: count "+strconv.Itoa(n)+" exceeds 32",
				"count must be a positive integer between 1 and 32",
				reloadHistoryExample,
			))
		}
		limit = n
	}
	return limit, nil
}
