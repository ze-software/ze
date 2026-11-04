// Design: docs/contributing/spec-workflow.md -- only locked claims admit new WIP
// Related: specmechanisms.go -- proposedSpec replays the proposed tool edit
package hookruntime

import (
	"errors"
	"io/fs"
	"os"

	"github.com/ze-software/ze/internal/le/spec"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// ze point: planning/spec-metadata-blocking/update-spec-status-at-each-transition
// writeSpecWIP leaves every new in-progress transition to the locked Claim
// publisher, whether claimed or not. It MUST check before the tool publishes.
func writeSpecWIP(ctx context) *verdict {
	if !oneOf(ctx.tool, toolWrite, toolEdit, toolMultiEdit) {
		return nil
	}
	file, inside := checkoutPath(ctx)
	if !inside {
		return nil
	}
	if !specpath.IsSpec(file) {
		return nil
	}
	current, err := os.ReadFile(absolutePath(ctx))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &verdict{2, "❌ BLOCKED: WIP transition cannot be judged for " + file + ": " + err.Error() +
			"\n  Make the spec readable, then retry the edit."}
	}
	proposed, applied := proposedSpec(ctx, string(current))
	if !applied {
		return nil
	}
	if err := spec.CheckWIPTransition(file, string(current), proposed); err != nil {
		return &verdict{2, "❌ BLOCKED: " + file + ": " + err.Error() + "\n  Inspect active work with ./le spec wip."}
	}
	return nil
}
