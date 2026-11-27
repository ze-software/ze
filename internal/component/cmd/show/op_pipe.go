// Design: docs/architecture/api/commands.md -- command-owned | operation <id> pipe filter
// Related: show.go -- show event recent and show errors | operation <id>
// Related: audit.go -- show audit | operation <id>
// Related: reload_history.go -- show system reload history | operation <id>

package show

import (
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	pipeOpOperation = "operation"
	keyOperationID  = "operation-id"
)

// operationPipeFilter declares the command-owned `| operation <id>` pipe
// operation shared by show event recent, show errors, show audit, and show
// system reload history.
var operationPipeFilter = command.PipeFilter{
	Name:        pipeOpOperation,
	Description: "Filter entries by operation correlation ID",
	Argument:    command.ArgText,
}

func extractSingleTextPipeTarget(
	validated command.ValidatedArgs,
	pipeName, duplicateMsg, duplicateWhy, requiredMsg, requiredWhy, extraMsg, extraWhy, example string,
) (value string, present bool, refusal *plugin.Response) {
	for _, op := range validated.PipeOperations() {
		if !strings.EqualFold(op.Name, pipeName) {
			continue
		}
		if present {
			return "", true, plugin.OperatorErrorResponse(opererr.New(duplicateMsg, duplicateWhy, example))
		}
		present = true
		if len(op.Args) == 0 {
			return "", true, plugin.OperatorErrorResponse(opererr.New(requiredMsg, requiredWhy, example))
		}
		if len(op.Args) > 1 {
			return "", true, plugin.OperatorErrorResponse(opererr.New(extraMsg, extraWhy, example))
		}
		value = strings.TrimSpace(op.Args[0])
	}
	if !present {
		return "", false, nil
	}
	if value == "" {
		return "", true, plugin.OperatorErrorResponse(opererr.New(requiredMsg, requiredWhy, example))
	}
	return value, true, nil
}

func extractOperationPipeTarget(validated command.ValidatedArgs, example string) (opID string, present bool, refusal *plugin.Response) {
	return extractSingleTextPipeTarget(
		validated,
		pipeOpOperation,
		"duplicate | operation pipe operation",
		"only one | operation filter may be given on a command line",
		"| operation requires an operation ID",
		"an operation ID must follow | operation",
		"| operation takes exactly one operation ID",
		"only one operation ID may follow | operation",
		example,
	)
}
