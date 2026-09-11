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
	if _, err := derivedHook(ctx, repo, le, "session-start", map[string]any{}); err != nil {
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
	code, err := derivedHook(ctx, repo, le, "posttool-writeedit", map[string]any{
		"tool_name":  "Write",
		"tool_input": map[string]any{"file_path": page},
	})
	if err != nil {
		return err
	}
	if code > 1 {
		return fmt.Errorf("the write hook refused the edit with %d", code)
	}
	if _, statErr := os.Stat(artifact); !os.IsNotExist(statErr) {
		// statErr is nil in the failing case, which is why it is not reported:
		// the fact is that the file is still there, not why the stat answered.
		return errors.New(derivedArtifactRel + " survived an edit to the page it is built from")
	}
	fmt.Fprintln(os.Stdout, "input-write-removed-the-artifact") //nolint:errcheck // progress output

	// The read hook runs BEFORE the command, so the grep below reads a file
	// built from the tree as it now stands.
	code, err = derivedHook(ctx, repo, le, "pretool-bash", map[string]any{
		"tool_name":  "Bash",
		"tool_input": map[string]any{"command": "grep -n 'internal/core/thing' " + derivedArtifactRel},
	})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("the read hook refused a grep of a registered artifact with %d", code)
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

// derivedHook runs one `le hook-check <kind>` over the scratch checkout, with
// the payload on stdin, and answers its exit code.
//
// The environment is built here rather than by envRootedAt because the hook
// runtime resolves its root from CLAUDE_PROJECT_DIR BEFORE it consults
// ZE_REPO_ROOT (hookruntime.newContext). A harness that exports the first would
// otherwise point every check at the shared checkout, where the write hook
// would remove a live session's artifacts.
func derivedHook(ctx context.Context, repo, le, kind string, payload map[string]any) (int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return -1, err
	}
	command := exec.CommandContext(ctx, le, "hook-check", kind) //nolint:gosec // the fixture chooses the program and its arguments
	command.Dir = repo
	command.Env = derivedHookEnvironment(repo)
	command.Stdin = bytes.NewReader(body)
	output, runErr := command.CombinedOutput()
	if runErr == nil {
		return 0, nil
	}
	exit, ok := errors.AsType[*exec.ExitError](runErr)
	if !ok {
		return -1, fmt.Errorf("le hook-check %s: %w\n%s", kind, runErr, output)
	}
	return exit.ExitCode(), nil
}

// derivedHookEnvironment answers the child environment with every spelling of
// the two root variables dropped, and ZE_REPO_ROOT set to the scratch checkout.
func derivedHookEnvironment(repo string) []string {
	inherited := os.Environ()
	kept := make([]string, 0, len(inherited)+1)
	for _, entry := range inherited {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			kept = append(kept, entry)
			continue
		}
		// env.Get matches case-insensitively and reads a dot as an underscore,
		// so both spellings of each key are dropped.
		normalized := strings.ToUpper(strings.ReplaceAll(name, ".", "_"))
		if normalized == "ZE_REPO_ROOT" || normalized == "CLAUDE_PROJECT_DIR" {
			continue
		}
		kept = append(kept, entry)
	}
	return append(kept, "ZE_REPO_ROOT="+repo)
}
