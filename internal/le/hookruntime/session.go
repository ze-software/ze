// Design: plan/spec-le-is-a-ze-binary.md -- native session identity
package hookruntime

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/le/lepath"
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

func writeParentSessionPrefix(ctx context, out io.Writer) {
	if ctx.payload.AgentID == "" && os.Getenv("CLAUDE_CODE_FORK_SUBAGENT") == "" {
		return
	}
	id, present := payloadSessionID(ctx.payload)
	if !present || id == "" {
		return
	}
	command, ok := ctx.input["command"].(string)
	if !ok {
		return
	}
	updated := make(map[string]any, len(ctx.input))
	maps.Copy(updated, ctx.input)
	updated["command"] = "export CLAUDE_CODE_SESSION_ID=" + id + "; " + command
	response := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PreToolUse",
			"updatedInput":  updated,
		},
	}
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(response)
}

func sessionMarker(ctx context, prefix string) string {
	id := resolvedSessionID(ctx)
	if id == "" {
		return ""
	}
	return strings.Join([]string{ctx.root, "tmp", "session", prefix + id}, string(os.PathSeparator))
}
