// Design: docs/architecture/cli/root-namespace-grammar.md -- an object takes its own token, and a bare token names what it holds
//
// The unit tests over leroot.Dispatch register probe commands and drive the
// resolver directly. This fixture drives the BUILT binary through argv, which
// is the path a developer is on: the words they type, the registry the linked
// binary holds, and the exit code their shell reads.
//
// Related: register_le_namespace_dispatch.go -- the registration.

package fixture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// leNamespaceAnswer is what one invocation of the le binary produced.
type leNamespaceAnswer struct {
	stdout string
	stderr string
	code   int
}

// leNamespaceDispatch drives the four answers a namespaced surface owes:
// a member resolves, a bare namespace token lists what it holds, an unknown
// token is still unknown, and a pipe operator survives the longer lookup.
func leNamespaceDispatch(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("le-namespace-dispatch: ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("le-namespace-dispatch: make ZE_REPO_ROOT absolute: %w", err)
	}
	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("le-namespace-dispatch: %w", err)
	}

	// `verify list mode full` is the member chosen because it reads the tree
	// and writes nothing, and .github/workflows/verify.yml reads its answer.
	member, err := leNamespaceRun(ctx, root, binary, "verify", "list", "mode", "full")
	if err != nil {
		return err
	}
	if member.code != 0 {
		return fmt.Errorf("le-namespace-dispatch: `le verify list mode full` exited %d: %s", member.code, member.stderr)
	}
	if !strings.Contains(member.stdout, "verify lint/run") {
		return fmt.Errorf("le-namespace-dispatch: `le verify list mode full` answered %q, want the stage population", member.stdout)
	}

	// A bare namespace token that is NOT a command of its own. It names what
	// the namespace holds, so a half-typed command teaches the rest of itself.
	//
	// The EXIT CODE is deliberately not asserted here. AC-11 of
	// spec-le-command-namespaces asks for 0, on the reading that a half-typed
	// command is not a typo; Dispatch answers 1, on the reading that nothing
	// ran. The two readings disagree about a contract a script reads, so the
	// spec stays open on it and this fixture asserts only what both agree on.
	bare, err := leNamespaceRun(ctx, root, binary, "spec")
	if err != nil {
		return err
	}
	listing := bare.stdout + bare.stderr
	for _, held := range []string{"citation", "session", "status"} {
		if !strings.Contains(listing, held) {
			return fmt.Errorf("le-namespace-dispatch: bare `le spec` did not name %q: %q", held, listing)
		}
	}

	// An unknown first word is still a typo, and still exits nonzero. Without
	// this case the listing above could be what every unrecognized word answers.
	unknown, err := leNamespaceRun(ctx, root, binary, "sepc")
	if err != nil {
		return err
	}
	if unknown.code == 0 {
		return fmt.Errorf("le-namespace-dispatch: `le sepc` exited 0; an unknown command must not read as a namespace: %q", unknown.stdout)
	}

	// The pipe word ends the candidate span, so a member still resolves with
	// an operator after it.
	piped, err := leNamespaceRun(ctx, root, binary, "verify", "list", "mode", "full", "|", "json")
	if err != nil {
		return err
	}
	if piped.code != 0 {
		return fmt.Errorf("le-namespace-dispatch: `le verify list mode full | json` exited %d: %s", piped.code, piped.stderr)
	}
	if !strings.HasPrefix(strings.TrimSpace(piped.stdout), "{") {
		return fmt.Errorf("le-namespace-dispatch: the json operator answered %q, want an object", piped.stdout)
	}

	fmt.Println("OK: a member resolves, a bare namespace token lists what it holds, an unknown token still fails, and a pipe survives the longer lookup") //nolint:forbidigo // the fixture protocol reads this line from stdout
	return nil
}

// leNamespaceRun runs the le binary once and answers what it produced. A
// nonzero exit is DATA here, because three of the four cases are about which
// code the developer's shell reads.
func leNamespaceRun(ctx context.Context, root, binary string, args ...string) (leNamespaceAnswer, error) {
	command := exec.CommandContext(ctx, binary, args...) // #nosec G204 -- every argv word is written in this file
	command.Dir = root
	var out, errOut bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errOut

	err := command.Run()
	answer := leNamespaceAnswer{stdout: out.String(), stderr: errOut.String()}
	if err == nil {
		return answer, nil
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return leNamespaceAnswer{}, fmt.Errorf("le-namespace-dispatch: run le %s: %w", strings.Join(args, " "), err)
	}
	answer.code = exit.ExitCode()
	return answer, nil
}
