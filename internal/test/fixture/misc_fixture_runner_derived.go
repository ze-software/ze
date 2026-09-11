// Design: docs/architecture/core-design.md -- the derived-artifact lifecycle, through the real hooks
//
// misc_fixture_runner_derived.go drives the REAL `le hook-check` binary over a
// scratch checkout and judges the artifact the hooks leave behind.
//
// The subject is the registry (internal/le/derived), not one generator. The
// scenario rides ai/CODE-TO-DOCS.md, whose inputs are markdown under docs/,
// because the posttool-writeedit chain also formats and LINTS an edited Go
// file: a scenario that edited Go would spend a minute in golangci-lint and
// would report that linter's verdict as this loop's. The package map's own
// loop is proved in process by TestAGrepAfterAnEditReadsTheEditedPackage
// (internal/le/derived/derived_test.go), which drives the same two checks.
//
// Related: register_le_derived_artifact_lifecycle.go -- the registration.

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// derivedArtifactRel is the artifact this scenario follows, relative to the
// scratch checkout. derivedPageRel is the page its anchors are read from.
const (
	derivedArtifactRel = "ai/CODE-TO-DOCS.md"
	derivedPageRel     = "docs/architecture/thing.md"
)

// leDerivedArtifactLifecycleDriver walks one session's loop: render, edit an
// input, read the artifact.
//
// A tracked artifact answered the third step from whatever the last commit
// held, with nothing to say the tree had moved since. What is asserted here is
// that the read answers from the EDIT.
func leDerivedArtifactLifecycleDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("derived-artifact fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-derived-artifact-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	if err := gitFixture(ctx, repo, map[string]string{
		fileGoMod:        "module fixture/derived\n\ngo 1.24\n",
		fileFeatureGates: contentFeatureGate,
		fileGitIgnore:    contentGitIgnoreTmp,
		"ai/.keep":       "",
		"internal/core/thing/thing.go": "// Design: docs/architecture/thing.md -- thing\n" +
			"// Package thing does a thing.\npackage thing\n",
		"internal/core/thing/other.go": "package thing\n",
		derivedPageRel:                 "# Thing\n\n<!-- source: internal/core/thing/thing.go -- the first claim -->\n",
	}); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "derived-fixture-ready") //nolint:errcheck // progress output

	// A session start renders every registered artifact the tree does not hold,
	// which is what puts the first copy there.
	if _, _, err := derivedHook(ctx, repo, le, "session-start", "", nil); err != nil {
		return err
	}
	artifact := filepath.Join(repo, filepath.FromSlash(derivedArtifactRel))
	rendered, err := os.ReadFile(artifact) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return fmt.Errorf("the session hook built no %s: %w", derivedArtifactRel, err)
	}
	// The index maps a code path to the documents that cite it, so what the
	// edit below has to move is WHICH path an anchor names. Asserting on the
	// anchor's prose would pass whatever the edit did, because no prose from a
	// `<!-- source: -->` comment reaches the rendered index.
	if !strings.Contains(string(rendered), "thing.go") {
		return fmt.Errorf("the rendered artifact does not carry the path its anchor names:\n%s", rendered)
	}
	if strings.Contains(string(rendered), "other.go") {
		return fmt.Errorf("the rendered artifact names a path no anchor cites yet:\n%s", rendered)
	}
	fmt.Fprintln(os.Stdout, "session-start-rendered-the-artifact") //nolint:errcheck // progress output

	// The edit a session makes, then the write hook that sees it.
	page := filepath.Join(repo, filepath.FromSlash(derivedPageRel))
	edited := "# Thing\n\n<!-- source: internal/core/thing/thing.go -- the first claim -->\n" +
		"\n<!-- source: internal/core/thing/other.go -- the second claim -->\n"
	if err := os.WriteFile(page, []byte(edited), 0o600); err != nil {
		return err
	}
	code, said, err := derivedWriteHook(ctx, repo, le, page)
	if err != nil {
		return err
	}
	if code > 1 {
		return fmt.Errorf("the write hook refused the edit with %d:\n%s", code, said)
	}
	if _, statErr := os.Stat(artifact); !os.IsNotExist(statErr) {
		// statErr is nil in the failing case, which is why it is not reported:
		// the fact is that the file is still there, not why the stat answered.
		return errors.New(derivedArtifactRel + " survived an edit to the page it is built from")
	}
	fmt.Fprintln(os.Stdout, "input-write-removed-the-artifact") //nolint:errcheck // progress output

	// The read hook runs BEFORE the command, so the grep below reads a file
	// built from the tree as it now stands.
	code, said, err = derivedReadHook(ctx, repo, le, "grep -n 'internal/core/thing' "+derivedArtifactRel)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("the read hook refused a grep of a registered artifact with %d:\n%s", code, said)
	}
	fmt.Fprintln(os.Stdout, "read-materialized-the-artifact") //nolint:errcheck // progress output

	found, code, err := rawCommand(ctx, repo, os.Environ(), "grep", "-n", "internal/core/thing", derivedArtifactRel)
	if err != nil || code != 0 {
		return fmt.Errorf("the grep the hook prepared for exit=%d: %w\n%s", code, err, found)
	}
	after, err := os.ReadFile(artifact) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	if !strings.Contains(string(after), "other.go") {
		return fmt.Errorf("the read answered from before the edit:\n%s", after)
	}
	fmt.Fprintln(os.Stdout, "grep-read-the-edited-tree") //nolint:errcheck // progress output
	return nil
}

// derivedWriteHook runs the post-write chain over one file a scenario has just
// written, which is the half that INVALIDATES a derived artifact.
func derivedWriteHook(ctx context.Context, repo, le, path string) (int, string, error) {
	return derivedHook(ctx, repo, le, "posttool-writeedit", "Write", map[string]any{"file_path": path})
}

// derivedReadHook runs the pre-command chain over one shell command, which is
// the half that REBUILDS every artifact the command names.
func derivedReadHook(ctx context.Context, repo, le, command string) (int, string, error) {
	return derivedHook(ctx, repo, le, "pretool-bash", "Bash", map[string]any{"command": command})
}

// derivedHook runs one `le hook-check <kind>` over the scratch checkout, with
// the payload on stdin, and answers its exit code and everything it printed.
//
// The output is answered on every path, refusal included. A hook states WHICH
// check refused and why, and a caller that reported the exit code alone left
// the author with a number and no way to reach the reason.
//
// A tool name of "" sends an empty payload, which is what a session-start hook
// takes.
func derivedHook(ctx context.Context, repo, le, kind, tool string, input map[string]any) (int, string, error) {
	payload := map[string]any{}
	if tool != "" {
		payload["tool_name"] = tool
		payload["tool_input"] = input
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return -1, "", err
	}
	command := exec.CommandContext(ctx, le, "hook-check", kind) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = repo
	command.Env = envRootedAt(repo)
	command.Stdin = bytes.NewReader(body)
	output, runErr := command.CombinedOutput()
	if runErr == nil {
		return 0, string(output), nil
	}
	exit, ok := errors.AsType[*exec.ExitError](runErr)
	if !ok {
		return -1, string(output), fmt.Errorf("le hook-check %s: %w\n%s", kind, runErr, output)
	}
	return exit.ExitCode(), string(output), nil
}
