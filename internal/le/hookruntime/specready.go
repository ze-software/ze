// Design: docs/contributing/spec-workflow.md -- a spec reaches ready only with Thomas's approval
// Related: specwip.go -- the sibling check for the edit that makes a spec in-progress
package hookruntime

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"time"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
)

// statusReadyWord is the Status value this check guards.
const statusReadyWord = "ready"

// ze point: planning/spec-metadata-blocking/a-spec-reaches-ready-only-with-thomass-approval
// writeSpecReady judges the edit that makes a spec ready, from another Status
// or as a new file: refused from a subagent (fromSubagent), since only the
// main session presents a design to Thomas, and from the main session unless
// specdesign.Admit admits the proposed text: Thomas's approval of it, or the
// away admission of a judged fix spec
// (plan/release/validation/spec-design-needs-owner-approval.md, AC-20, AC-21,
// C-5). An edit that leaves Status as it is, and any edit of a spec already
// ready, is never judged, so the 94 specs ready before this check stay
// editable. It MUST check before the tool publishes. A spec is found in the
// checkout it lies in (gatedCheckout), so a worktree's spec is judged against
// that checkout's records and history.
func writeSpecReady(ctx context) *verdict {
	if !oneOf(ctx.tool, toolWrite, toolEdit, toolMultiEdit) {
		return nil
	}
	checkout, file, inside := gatedCheckout(ctx, absolutePath(ctx))
	if !inside {
		return nil
	}
	if !specpath.IsSpec(file) {
		return nil
	}
	named := checkout.name(file)
	current, err := os.ReadFile(absolutePath(ctx))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &verdict{2, "❌ BLOCKED: the move of " + named + " to ready cannot be judged: " + err.Error() +
			"\n  fix: make the spec readable, then retry the edit"}
	}
	proposed, applied := proposedSpec(ctx, string(current))
	if !applied {
		return nil
	}
	if !movesToReady(string(current), proposed) {
		return nil
	}
	if fromSubagent(ctx) {
		return &verdict{2, "❌ BLOCKED: " + named + ": a subagent cannot move a spec to ready\n" +
			"  why: only the main session presents a design to Thomas, and he approves it\n" +
			"  fix: hand the design back to the main session and stop"}
	}
	admission, err := specdesign.Admit(checkout.root, file, proposed, time.Now())
	if err != nil {
		return &verdict{2, "❌ BLOCKED: the design approval of " + named + " cannot be read: " + err.Error() +
			"\n  why: an approval that cannot be read is never read as one Thomas gave" +
			"\n  fix: repair what the error names (the record under tmp/spec-design/, the git history, the spec's file name), then retry the edit"}
	}
	if admission.Admitted {
		return nil
	}
	return &verdict{2, "❌ BLOCKED: " + named + " cannot be marked ready: its design has no owner approval matching this text\n" +
		strings.Join(admission.Refusal(file), "\n")}
}

// movesToReady reports whether proposed makes a spec ready that current, ""
// for a new file, was not. A proposed text whose Status cannot be read is not
// judged here: writeSpecWIP refuses its metadata. A current text whose Status
// cannot be read was not ready, so its move is judged.
func movesToReady(current, proposed string) bool {
	after, err := specstatus.MetadataStatus(proposed)
	if err != nil {
		return false
	}
	if after != statusReadyWord {
		return false
	}
	if current == "" {
		return true
	}
	before, err := specstatus.MetadataStatus(current)
	if err != nil {
		return true
	}
	return before != statusReadyWord
}
