// Design: docs/architecture/core-design.md -- native delegation hook policy
// Related: agent.go -- the checks on the spawn; session.go -- the parent session identity
package hookruntime

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
)

const (
	// agentCallBudget is the number of hooked tool calls an editing agent makes
	// before it hands off (ai/rules/context-economy.md). The hook sees Bash,
	// Write, Edit, MultiEdit and NotebookEdit; Read and ToolSearch pass
	// unhooked, and they are under a tenth of an agent's calls.
	agentCallBudget = 100
	// budgetedAgentType is the one agent type the budget binds. A read-only
	// agent cut mid-review loses coverage, and the rule names the editing agent.
	budgetedAgentType = "ze-work"
)

var (
	handoffCommand = regexp.MustCompile(`session-state-|spec (session )?state`)
	safeAgentID    = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// ze point: context-economy/directives/an-editing-agent-hands-off-at-one-hundred-tool-calls
// bashCallBudget counts a Bash call against the editing agent's budget. The
// gate map binds one check to one dispatcher, so Bash and Write/Edit each
// carry a name over the one counter.
func bashCallBudget(ctx context) *verdict {
	return agentCallBudgetCheck(ctx)
}

// ze point: context-economy/directives/an-editing-agent-hands-off-at-one-hundred-tool-calls
// writeCallBudget counts a Write, Edit, MultiEdit or NotebookEdit call against
// the editing agent's budget.
func writeCallBudget(ctx context) *verdict {
	return agentCallBudgetCheck(ctx)
}

// agentCallBudgetCheck counts the hooked calls an editing agent has made and
// refuses the call past the budget, except a call that writes the handoff.
func agentCallBudgetCheck(ctx context) *verdict {
	if ctx.payload.AgentID == "" || ctx.payload.AgentType != budgetedAgentType {
		return nil
	}
	if !safeAgentID.MatchString(ctx.payload.AgentID) {
		return &verdict{1, "agent-call-budget: agent id " + strconv.Quote(ctx.payload.AgentID) + " is not a file name, so this agent is not counted"}
	}
	parent, present := payloadSessionID(ctx.payload)
	if !present || parent == "" {
		return &verdict{1, "agent-call-budget: no parent session id in the payload, so this agent is not counted"}
	}
	paths, err := lepath.SessionForID(ctx.root, parent)
	if err != nil {
		return &verdict{1, "agent-call-budget: " + err.Error()}
	}
	calls, err := bumpAgentCalls(filepath.Join(ctx.root, paths.Dir, "agents"), ctx.payload.AgentID)
	if err != nil {
		return &verdict{1, "agent-call-budget: " + err.Error()}
	}
	if calls <= agentCallBudget {
		return nil
	}
	if isHandoffCall(ctx) {
		return nil
	}
	var count [20]byte
	return &verdict{2, red + bold + "❌ Blocked: this agent has made " + string(strconv.AppendInt(count[:0], calls-1, 10)) + " tool calls; its budget is " + string(strconv.AppendInt(count[10:10], agentCallBudget, 10)) + " (ai/rules/context-economy.md)." + reset + "\n" +
		"  -- Every call now re-feeds more context than a successor's whole start costs.\n" +
		"  -- Append your handoff to the per-spec state file: `./le spec state current` prints its path, and /ze-implement, \"Phase handoff\", names its four parts.\n" +
		"  -- Then report to the main thread that the package needs a continuation, and stop. The continuation carries the SAME package: no stub, no trimmed criterion, no parked item.\n" +
		"  -- A call that names that state file still passes."}
}

// bumpAgentCalls adds one to the agent's counter file and returns the new
// count. The first call creates the file. A file holding anything but a
// number is an error, never a zero: a zero would let a corrupted counter
// reopen the budget.
func bumpAgentCalls(dir, agentID string) (int64, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return 0, err
	}
	path := filepath.Join(dir, agentID+".calls")
	calls := int64(0)
	body, err := os.ReadFile(path) //nolint:gosec // a counter under the checkout tmp directory, named by a validated agent id
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return 0, err
	default:
		calls, err = strconv.ParseInt(strings.TrimSpace(string(body)), 10, 64)
		if err != nil {
			return 0, errors.New("counter " + path + " does not hold a number: " + err.Error())
		}
	}
	calls++
	var count [20]byte
	if err := os.WriteFile(path, strconv.AppendInt(count[:0], calls, 10), 0o600); err != nil {
		return 0, err
	}
	return calls, nil
}

// isHandoffCall reports whether the call names the per-spec state file, or
// asks for its path. Those pass after the budget, because the handoff is
// what the refusal asks the agent to write.
func isHandoffCall(ctx context) bool {
	if ctx.tool == "Bash" {
		return handoffCommand.MatchString(stringInput(ctx.input, "command"))
	}
	return strings.Contains(ctx.path, "session-state-")
}
