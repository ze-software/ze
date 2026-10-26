package fixture

// Related: register_le_launcher_words.go -- the registration.
// Related: ui_fixture_le_help_pages.go -- leHelpPagesBuild and leHelpPagesResult, shared.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// leLauncherWordsBuild is the placed build a rollback would set aside, named in
// the store's <UTC commit time>-<sha> shape.
const leLauncherWordsBuild = "20260101T000000Z-0000000000"

// leLauncherWordsRollback is the launcher word under test: the one the
// refusal answers for, and one the root page must name.
const leLauncherWordsRollback = "rollback"

// leLauncherWordsSession is the value of --name the root page must name.
const leLauncherWordsSession = "session"

// leLauncherWords runs the repository's own le script from a copy beside a
// placed build, so a rollback that was not refused would move that build and
// the store listing would show it, and nothing in the shared checkout's store
// can be touched. It then reads the root page of the built le personality for
// the launcher's words.
func leLauncherWords(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-launcher-words-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	checkout := filepath.Join(work, "checkout")
	script, err := leLauncherWordsPlace(root, checkout)
	if err != nil {
		return err
	}
	before, err := leLauncherWordsStore(checkout)
	if err != nil {
		return err
	}
	for _, extra := range [][]string{{"newest"}, {"newest", "now"}, {leHelpPagesHelpWord, "now"}, {"|", "match"}} {
		if err := leLauncherWordsRefused(ctx, checkout, script, extra, before); err != nil {
			return err
		}
	}
	if err := leLauncherWordsInherited(ctx, checkout, script, before); err != nil {
		return err
	}
	for _, extra := range [][]string{{leHelpPagesHelpWord}, {"-h"}, {"--help"}, {leHelpPagesHelpWord, "|", renderJSON}} {
		if err := leLauncherWordsHelp(ctx, checkout, script, extra); err != nil {
			return err
		}
	}
	if err := leLauncherWordsPiped(ctx, checkout, script); err != nil {
		return err
	}

	build := filepath.Join(work, "build")
	if err := os.Mkdir(build, 0o750); err != nil {
		return fmt.Errorf("create build directory: %w", err)
	}
	lePath, _, err := leHelpPagesBuild(ctx, root, build)
	if err != nil {
		return err
	}
	page, err := leLauncherWordsRun(ctx, root, lePath, leHelpPagesHelpWord)
	if err != nil {
		return err
	}
	if page.exitCode != 0 {
		return fmt.Errorf("le help answered %d: %s", page.exitCode, page.stderr)
	}
	for _, word := range []string{"--name", leLauncherWordsSession, leLauncherWordsRollback} {
		if !strings.Contains(page.stdout, word) {
			return fmt.Errorf("le help does not name the launcher word %s:\n%s", word, page.stdout)
		}
	}
	var tb textbuf.Buffer
	tb.Str("refused: ./le rollback newest, ./le rollback newest now, ./le rollback help now, ./le rollback | match (exit 2, store unchanged: ").
		Int(int64(len(before))).Str(" entries)\n").
		Str("refused with ZE_LE_BUILD_NAME inherited: ./le rollback (exit 2, names the variable)\n").
		Str("passed to the build: ./le rollback help, -h, --help, help | json (exit 0, nothing moved)\n").
		Str("rolled back: ./le rollback | json (the build moved aside)\n").
		Str("le help names: --name, session, rollback\n").
		// The .ci reads this line; an exit code of 0 alone would also pass
		// a driver that returned before its checks ran.
		Str("OK\n").StdOut() //nolint:errcheck // fixture output
	return nil
}

// leLauncherWordsRefused runs `./le rollback <extra...>` through the copied
// script and requires exit 2, a refusal naming ./le rollback, and a build
// store equal to before.
func leLauncherWordsRefused(ctx context.Context, checkout, script string, extra, before []string) error {
	typed := strings.Join(extra, " ")
	result, err := leLauncherWordsRun(ctx, checkout, script, append([]string{leLauncherWordsRollback}, extra...)...)
	if err != nil {
		return err
	}
	if result.exitCode != 2 {
		return fmt.Errorf("./le rollback %s answered %d, want 2; stderr %q", typed, result.exitCode, result.stderr)
	}
	for _, want := range []string{"./le rollback takes no other word", typed, "Run: ./le rollback"} {
		if !strings.Contains(result.stderr, want) {
			return fmt.Errorf("./le rollback %s: stderr does not say %q: %q", typed, want, result.stderr)
		}
	}
	after, err := leLauncherWordsStore(checkout)
	if err != nil {
		return err
	}
	if !slices.Equal(before, after) {
		return fmt.Errorf("./le rollback %s changed the build store: before %q, after %q", typed, before, after)
	}
	return nil
}

// leLauncherWordsInherited runs a bare `./le rollback` with ZE_LE_BUILD_NAME
// set, as a parent `./le --name` call leaves it for its children, and requires
// exit 2, a refusal that names the variable and how to unset it, and a build
// store equal to before. A refusal that only said "run ./le rollback" would
// send the caller round the same loop, because the variable is still set.
func leLauncherWordsInherited(ctx context.Context, checkout, script string, before []string) error {
	environ := append(os.Environ(), "ZE_LE_BUILD_NAME=launcher-words-parent")
	result, err := leLauncherWordsRunWith(ctx, checkout, script, environ, leLauncherWordsRollback)
	if err != nil {
		return err
	}
	if result.exitCode != 2 {
		return fmt.Errorf("./le rollback with ZE_LE_BUILD_NAME set answered %d, want 2; stderr %q", result.exitCode, result.stderr)
	}
	for _, want := range []string{"ZE_LE_BUILD_NAME=launcher-words-parent", "env -u ZE_LE_BUILD_NAME ./le rollback"} {
		if !strings.Contains(result.stderr, want) {
			return fmt.Errorf("./le rollback with ZE_LE_BUILD_NAME set: stderr does not say %q: %q", want, result.stderr)
		}
	}
	after, err := leLauncherWordsStore(checkout)
	if err != nil {
		return err
	}
	if !slices.Equal(before, after) {
		return fmt.Errorf("./le rollback with ZE_LE_BUILD_NAME set changed the build store: before %q, after %q", before, after)
	}
	return nil
}

// leLauncherWordsHelp runs `./le rollback <help word...>` and requires exit 0,
// the placed build receiving the whole line (its stand-in echoes its
// arguments), and no build moved aside: a help word asks the binary for the
// page, and must neither be refused nor roll anything back.
func leLauncherWordsHelp(ctx context.Context, checkout, script string, extra []string) error {
	typed := strings.Join(extra, " ")
	result, err := leLauncherWordsRun(ctx, checkout, script, append([]string{leLauncherWordsRollback}, extra...)...)
	if err != nil {
		return err
	}
	if result.exitCode != 0 {
		return fmt.Errorf("./le rollback %s answered %d, want 0; stderr %q", typed, result.exitCode, result.stderr)
	}
	if want := "stand-in: rollback " + typed + "\n"; !strings.Contains(result.stdout, want) {
		return fmt.Errorf("./le rollback %s did not reach the build as %q: stdout %q, stderr %q", typed, want, result.stdout, result.stderr)
	}
	after, err := leLauncherWordsStore(checkout)
	if err != nil {
		return err
	}
	for _, entry := range after {
		if strings.Contains(entry, ".failed-") {
			return fmt.Errorf("./le rollback %s moved a build aside: %q", typed, after)
		}
	}
	return nil
}

// leLauncherWordsPiped runs `./le rollback | json`, the form the help page's
// usage line shows, and requires exit 0 and the placed build moved aside: the
// pipe chain shapes an answer and is not a reason to refuse the rollback.
func leLauncherWordsPiped(ctx context.Context, checkout, script string) error {
	result, err := leLauncherWordsRun(ctx, checkout, script, leLauncherWordsRollback, "|", renderJSON)
	if err != nil {
		return err
	}
	if result.exitCode != 0 {
		return fmt.Errorf("./le rollback | json answered %d, want 0; stderr %q", result.exitCode, result.stderr)
	}
	after, err := leLauncherWordsStore(checkout)
	if err != nil {
		return err
	}
	if !slices.Contains(after, "/.failed-"+leLauncherWordsBuild) {
		return fmt.Errorf("./le rollback | json did not move the build aside: store %q, stderr %q", after, result.stderr)
	}
	return nil
}

// leLauncherWordsPlace copies the repository's le script into checkout and
// places one build in its store, answering the script's path.
func leLauncherWordsPlace(root, checkout string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, "le")) //nolint:gosec // the repository's own launcher
	if err != nil {
		return "", fmt.Errorf("read the le script: %w", err)
	}
	placed := filepath.Join(checkout, "bin", "builds", leLauncherWordsBuild)
	if err := os.MkdirAll(placed, 0o750); err != nil {
		return "", fmt.Errorf("create the build store: %w", err)
	}
	if err := os.WriteFile(filepath.Join(placed, "le"), []byte("#!/bin/sh\necho \"stand-in: $*\"\nexit 0\n"), 0o750); err != nil { //nolint:gosec // an executable stand-in for a build
		return "", fmt.Errorf("place a build: %w", err)
	}
	script := filepath.Join(checkout, "le")
	if err := os.WriteFile(script, body, 0o750); err != nil { //nolint:gosec // the launcher is executable
		return "", fmt.Errorf("copy the le script: %w", err)
	}
	return script, nil
}

// leLauncherWordsStore answers every path under the checkout's build store,
// set-aside entries included, sorted.
func leLauncherWordsStore(checkout string) ([]string, error) {
	store := filepath.Join(checkout, "bin", "builds")
	var paths []string
	err := filepath.WalkDir(store, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, strings.TrimPrefix(path, store))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list the build store: %w", err)
	}
	slices.Sort(paths)
	return paths, nil
}

// leLauncherWordsRun runs program in dir, in this process's environment, and
// answers what a shell would read.
func leLauncherWordsRun(ctx context.Context, dir, program string, args ...string) (leHelpPagesResult, error) {
	return leLauncherWordsRunWith(ctx, dir, program, nil, args...)
}

// leLauncherWordsRunWith runs program in dir with environ, or this process's
// environment when environ is nil, and answers what a shell would read.
func leLauncherWordsRunWith(ctx context.Context, dir, program string, environ []string, args ...string) (leHelpPagesResult, error) {
	cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	cmd.Env = environ
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := leHelpPagesResult{stdout: stdout.String(), stderr: stderr.String()}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.exitCode = exitErr.ExitCode()
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("run %s %s: %w", filepath.Base(program), strings.Join(args, " "), err)
	}
	return result, nil
}
