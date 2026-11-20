// Design: docs/architecture/core-design.md -- native delegation hook policy
// Related: lifecycle.go -- runLifecycleHook, which serves mark-agent-spawned

package hookruntime

import (
	"encoding/json"
	"io"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/ai/tokens/contextsize"
)

// handoffContextTokens is the main thread's handoff threshold: "A main thread
// past 400k context MUST write its per-spec state file and hand off"
// (ai/rules/planning.md, the main thread supervises and verifies what comes
// back). The figure is decimal thousands, as the token report prints it.
// Thomas lowered it from 600k on 2026-11-20: over 2026-11-15..20 main threads
// averaged 315k per call, and 42% of their context was fed in calls at
// 400k to 600k.
const handoffContextTokens = 400_000

// hookNotice is text a hook that lets the call through gives the model as
// additional context, under the hook event that carries it. The zero value
// is no notice.
type hookNotice struct {
	event string
	text  string
}

// writeAdditionalContext writes the hook protocol answer that hands text to
// the model on the next request: the harness wraps it in a system reminder.
// The harness reads that answer only from a hook that exits 0, so a caller
// writes it only on a call it lets through. The text stays far under the
// 10,000-character cap the harness puts on additionalContext.
func writeAdditionalContext(out io.Writer, notice hookNotice) {
	response := map[string]any{"hookSpecificOutput": map[string]any{
		"hookEventName": notice.event, "additionalContext": notice.text,
	}}
	// A notice the harness cannot read is lost; the call it rides on still passes.
	_ = json.NewEncoder(out).Encode(response)
}

// hookAgentSpawned serves mark-agent-spawned, the after-hook of an Agent call.
// It writes the session's spawn marker, which the stop hook reads, and tells
// the main thread its context size when the size is past the handoff
// threshold. It never blocks: the agent has already been spawned.
func hookAgentSpawned(ctx context, out io.Writer) int {
	code := writeSessionMarker(ctx, ".agent-spawned-", "")
	if text := spawnContextNotice(ctx); text != "" {
		writeAdditionalContext(out, hookNotice{event: "PostToolUse", text: text})
	}
	return code
}

// spawnContextNotice answers the handoff notice for the main thread, or ""
// when none is owed. The threshold is the main thread's, so an agent spawning
// an agent (the payload carries its agent_id) is told nothing. A transcript
// that cannot be read, or whose tail holds no usage record, says nothing about
// the size, so the hook stays silent rather than print a figure it did not
// read: the size is one turn old at best, since the harness writes the
// transcript asynchronously, and a missed notice is caught at the next spawn.
func spawnContextNotice(ctx context) string {
	if ctx.payload.AgentID != "" {
		return ""
	}
	if ctx.payload.TranscriptPath == "" {
		return ""
	}
	tokens, err := contextsize.Last(ctx.payload.TranscriptPath)
	// refuse-by-default: guards hookAgentSpawned -- no notice means the size is unknown, never that the transcript fits the threshold
	if err != nil {
		return ""
	}
	if tokens <= handoffContextTokens {
		return ""
	}
	var text textbuf.Buffer
	text.Str("Context check: this session's last recorded context is ").Int(tokens / 1000).
		Str("k tokens, past the ").Int(handoffContextTokens / 1000).
		Str("k handoff threshold (ai/rules/planning.md).\n").
		Str("  -- Every call re-feeds that context, so the work left costs more here than in a fresh session.\n").
		Str("  -- Write the per-spec state file now: `./le spec state current` prints its path.\n").
		Str("  -- Then hand off. One spec per session: the next spec starts in a fresh session.")
	return text.String()
}
