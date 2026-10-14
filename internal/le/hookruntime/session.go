// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
package hookruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"strconv"
	"strings"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

func payloadSessionID(payload Payload) (string, bool) {
	if payload.SessionID == nil {
		return "", false
	}
	id, ok := payload.SessionID.(string)
	if !ok || !lepath.ValidSessionID(id) {
		return "", true
	}
	return id, true
}

func resolvedSessionID(ctx context) string {
	if id, present := payloadSessionID(ctx.payload); present {
		return id
	}
	paths, err := lepath.ResolveSession(ctx.root, false)
	if err != nil {
		return ""
	}
	return paths.ID
}

func runSessionID(ctx context, out, _ io.Writer) int {
	if id, present := payloadSessionID(ctx.payload); present {
		if id == "" {
			return 2
		}
		fmt.Fprintln(out, id) //nolint:errcheck // hook protocol
		return 0
	}
	return 1
}

// writeParentSessionPrefix makes a subagent's Bash call resolve the identity
// the harness gave it: the parent's session id, which keeps the session's
// directory and scratch shared, and the agent's own id, which keys everything
// one agent must own alone (its commit namespace, its weakened-test shard, its
// debt file and its spec claim, in internal/le/le/path). Without the agent id
// every sibling resolved one identity and shared all four
// (plan/journal/concurrent-session-corruption.md). An agent id that cannot
// name a file refuses the call with exit 2, because running it unprefixed
// would run it as the main thread, which is the sharing this exists to end.
// A payload with an agent id and no usable session id refuses the same way,
// for the same reason. A fork subagent carries no agent id, so it has no
// identity of its own to lose: with no usable session id it runs unprefixed.
// The answer is the hook's exit code.
func writeParentSessionPrefix(ctx context, out, errOut io.Writer) int {
	agent := ctx.payload.AgentID
	if agent == "" && os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") == "" {
		return 0
	}
	id, _ := payloadSessionID(ctx.payload)
	if id == "" {
		if agent == "" {
			return 0
		}
		fmt.Fprintln(errOut, red+bold+"BLOCKED: this subagent's hook payload carries no usable session id"+reset+ //nolint:errcheck // hook protocol
			"\n  agent "+strconv.Quote(agent)+" arrived with a session_id that is absent or not a file name."+
			"\n  Run unprefixed, the command would act as the main thread and share its commit namespace, weakened-test shard and spec claim."+
			"\n  Next: stop and report the payload's session_id to the main thread. An id holds only letters, digits, '.', '_' and '-'.")
		return 2
	}
	command, ok := ctx.input["command"].(string)
	if !ok {
		return 0
	}
	var prefix strings.Builder
	if agent := ctx.payload.AgentID; agent != "" {
		if _, err := lepath.AgentFingerprint(id, agent); err != nil {
			fmt.Fprintln(errOut, red+bold+"BLOCKED: this subagent's identity cannot name its own files"+reset+ //nolint:errcheck // hook protocol
				"\n  "+err.Error()+
				"\n  Run unprefixed, the command would act as the main thread and share its commit namespace, weakened-test shard and spec claim."+
				"\n  Next: stop and report the agent id to the main thread. An id holds only letters, digits, '.', '_' and '-'.")
			return 2
		}
		prefix.WriteString("export ZE_AGENT_ID=" + agent + "; ")
	}
	prefix.WriteString("export CLAUDE_CODE_SESSION_ID=" + id + "; ")
	updated := make(map[string]any, len(ctx.input))
	maps.Copy(updated, ctx.input)
	updated["command"] = prefix.String() + command
	response := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse",
			"updatedInput":  updated,
		},
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(response)
	return 0
}

// specClaim answers the spec the hook's identity has claimed: the agent's own
// claim when the payload names an agent that made one, else the session's,
// which is the claim the agent was spawned under. "" is no claim.
func specClaim(ctx context, id string) string {
	if agent := ctx.payload.AgentID; agent != "" {
		if marker, err := lepath.SpecClaimMarker(ctx.root, id, agent); err == nil {
			if claim := readFirstLine(marker); claim != "" {
				return claim
			}
		}
	}
	marker, err := lepath.SpecClaimMarker(ctx.root, id, "")
	if err != nil {
		return ""
	}
	return readFirstLine(marker)
}

func sessionMarker(ctx context, prefix string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	return strings.Join([]string{ctx.root, "tmp", "session", prefix + id}, string(os.PathSeparator))
}
