// Design: docs/contributing/spec-workflow.md -- the spec template and the checks that judge it
// Related: specdoors.go -- writeSpecDoorsRead, the other pre-write gate on a spec edit
// Related: ../spec/mechanisms/specmechanisms.go -- Problem, the judgement this gate applies
package hookruntime

import (
	"errors"
	"io/fs"
	"os"
	"strings"

	specmechanisms "github.com/ze-software/ze/internal/le/spec/mechanisms"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// toolEdit and toolMultiEdit are the edit tools whose payload this gate replays.
const (
	toolEdit      = "Edit"
	toolMultiEdit = "MultiEdit"
)

// ze point: planning/spec-metadata-blocking/start-a-design-from-the-mechanisms-ze-already-has
// writeSpecMechanisms refuses a Write, Edit or MultiEdit of a spec under plan/
// (specpath.IsSpec) whose content after the edit claims a design (Status
// design, ready, in-progress or verification) and carries no Existing
// Mechanisms table row (specmechanisms.Problem). It judges the content the edit
// produces, so an untouched spec is never judged, and a spec that predates the
// rule owes the section the first time it is edited. An Edit whose old_string
// the file does not hold changes nothing and is the Edit tool's own failure, so
// it passes unjudged.
func writeSpecMechanisms(ctx context) *verdict {
	if !oneOf(ctx.tool, toolWrite, toolEdit, toolMultiEdit) {
		return nil
	}
	file, inside := checkoutPath(ctx)
	// refuse-by-default: guards checkoutPath -- a file outside the checkout is no spec of this repository
	if !inside {
		return nil
	}
	if !specpath.IsSpec(file) {
		return nil
	}
	current, err := os.ReadFile(absolutePath(ctx))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &verdict{2, "❌ BLOCKED: " + file + " cannot be read: " + err.Error() +
			"\n  Why: the Existing Mechanisms gate judges the spec as the edit leaves it, and is fail-closed.\n  Fix: make the file readable, then retry.\n  ai/rules/planning.md"}
	}
	proposed, applied := proposedSpec(ctx, string(current))
	// refuse-by-default: guards replaceEdit -- an old_string the file lacks fails the Edit tool, so no content lands to judge
	if !applied {
		return nil
	}
	problem := specmechanisms.Problem(file, proposed)
	if problem == "" {
		return nil
	}
	return &verdict{2, "❌ BLOCKED: " + problem + "\n  ai/rules/planning.md"}
}

// proposedSpec answers the file content the edit leaves, and false when an
// edit's old_string is not in the content it edits.
func proposedSpec(ctx context, current string) (string, bool) {
	switch ctx.tool {
	case toolWrite:
		return stringInput(ctx.input, "content"), true
	case toolEdit:
		all, _ := ctx.input["replace_all"].(bool)
		return replaceEdit(current, stringInput(ctx.input, "old_string"), stringInput(ctx.input, "new_string"), all)
	}
	edits, _ := ctx.input["edits"].([]any)
	text := current
	for _, raw := range edits {
		edit, _ := raw.(map[string]any)
		all, _ := edit["replace_all"].(bool)
		next, applied := replaceEdit(text, stringInput(edit, "old_string"), stringInput(edit, "new_string"), all)
		if !applied {
			return "", false
		}
		text = next
	}
	return text, true
}

// replaceEdit applies one old_string to new_string replacement the way the Edit
// tool does: the first occurrence, or every one under replace_all. An empty
// old_string on an empty file is the tool's create form.
func replaceEdit(text, old, replacement string, all bool) (string, bool) {
	if old == "" {
		return replacement + text, text == ""
	}
	if !strings.Contains(text, old) {
		return "", false
	}
	if all {
		return strings.ReplaceAll(text, old, replacement), true
	}
	return strings.Replace(text, old, replacement, 1), true
}
