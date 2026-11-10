// Design: docs/architecture/testing/tracked-build-gate.md -- building le from what git holds
//
// Related: export.go -- Commit, which materializes the tree this file compiles

package export

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// BuildLe compiles the cmd/ze le personality of the exported tree at tree
// into binary, with the tree's own feature tags (DaemonBuildTags plus ze_le),
// so the binary is the le that commit would have run.
//
// tree MUST be a directory Commit filled: the build reads vendor/ and the
// tree's own feature-gates.txt, never the checkout's. binary is a path the
// caller chose; BuildLe creates no directory for it.
//
// The child inherits this process's environment, so it compiles against the
// patched GOROOT that le's root handler activated before dispatch
// (docs/contributing/patching-the-standard-library.md, "What a build sets"):
// GOROOT, GOTOOLCHAIN=local and the PATH entry are left as inherited, and
// `go` is looked up on that PATH. An unpatched toolchain cannot compile Ze,
// so a process that never activated the tree gets the compiler's refusal as
// this error. Only GOFLAGS and CGO_ENABLED are set, appended so they win over
// any ambient value. ctx bounds the compile.
func BuildLe(ctx context.Context, tree, binary string) error {
	tags, err := repofeaturetags.DaemonBuildTags(tree, "ze_le")
	if err != nil {
		return err
	}
	build := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", binary, "./cmd/ze") //nolint:gosec // binary is the caller's path; tags come from the exported tree's own declaration
	build.Dir = tree
	build.Env = append(os.Environ(), "GOFLAGS=-mod=vendor", "CGO_ENABLED=0")

	var complaint bytes.Buffer
	build.Stderr = &complaint
	if err := build.Run(); err != nil {
		// The exit error is kept beside the compiler's text: a build ctx
		// canceled, or a `go` missing from PATH, writes no stderr at all.
		var tb textbuf.Buffer
		return errors.New(tb.Str("go build ./cmd/ze le personality: ").Err(err).Str(": ").
			Str(strings.TrimSpace(complaint.String())).String())
	}
	return nil
}
