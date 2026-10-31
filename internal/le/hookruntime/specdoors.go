// Design: docs/architecture/core-design.md -- native edit hook policy
// Related: writeedit.go -- writeDesignDoorsRead, the code-edit side of the door gate, and unreadDoors
// Related: readgate.go -- the door headers, coveringDoors and the read records
package hookruntime

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	specpath "github.com/ze-software/ze/internal/le/spec/path"
)

// specDoorReason is why a spec edit waits for the doors of the paths it names.
const specDoorReason = "a spec's design is written against\n" +
	"  the door of every subsystem it targets: each door covering a path the spec names (its Files to Modify\n" +
	"  and Files to Create, or any other repository path in it, before or after this edit). A design written\n" +
	"  before them drops the contracts they hold.\n"

// ze point: architecture/directives/read-the-design-doors-before-writing-a-spec
// writeSpecDoorsRead refuses a Write, Edit or MultiEdit of a spec under plan/
// (specpath.IsSpec) from a context with no record of reading ai/doors/system.md
// and every door covering a repository path the spec names, each in full. The
// paths are read from the spec on disk and from the text the edit proposes, so
// an edit owes the doors of the subsystems the spec targeted as well as those it
// now adds. A brand-new spec that names no path owes system.md alone. The doors
// come from their own headers, through the coverage the code gate uses
// (coveringDoors), so no list of subsystems exists here. A spec written through
// a Bash heredoc never reaches this check; bashGovernedWrite refuses that.
func writeSpecDoorsRead(ctx context) *verdict {
	if !oneOf(ctx.tool, toolWrite, "Edit", "MultiEdit") {
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
		return &verdict{2, "❌ BLOCKED: " + file + " cannot be read: " + err.Error() +
			"\n  Why: the design-door gate reads the spec to name the doors of the subsystems it targets, and is fail-closed.\n  Fix: make the file readable, then retry.\n  ai/rules/architecture.md"}
	}
	unread, refusal := unreadDoors(ctx, specRepoPaths(ctx.root, string(current)+"\n"+ctx.content))
	if refusal != nil {
		return refusal
	}
	if len(unread) == 0 {
		return nil
	}
	return &verdict{2, doorRefusal(file, unread, specDoorReason)}
}

// specRepoPaths answers every checkout-relative path the text names: each run
// of path characters that holds a slash, with a leading `./` or the checkout
// root dropped and trailing sentence punctuation trimmed. A run that only looks
// like a path (`and/or`) reaches no door base, so coveringDoors ignores it; a
// bare word such as `test` is never a path, because it holds no slash.
func specRepoPaths(root, text string) []string {
	rootPrefix := filepath.ToSlash(root) + "/"
	var paths []string
	for token := range strings.FieldsFuncSeq(text, notPathRune) {
		token = strings.TrimPrefix(token, rootPrefix)
		token = strings.TrimPrefix(token, "./")
		token = strings.TrimRight(token, ".")
		token = strings.TrimSuffix(token, "/")
		if !strings.Contains(token, "/") {
			continue
		}
		if strings.HasPrefix(token, "/") {
			continue
		}
		if slices.Contains(paths, token) {
			continue
		}
		paths = append(paths, token)
	}
	return paths
}

// notPathRune reports whether r ends a run of repository path characters.
func notPathRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return false
	}
	if r >= 'A' && r <= 'Z' {
		return false
	}
	if r >= '0' && r <= '9' {
		return false
	}
	return !strings.ContainsRune("_./-", r)
}
