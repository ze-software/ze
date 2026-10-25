package fixture

// Related: register_le_help_pages.go -- the registration.

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

// leHelpPagesHelpWord is the word that asks le for a page.
const leHelpPagesHelpWord = "help"

// leHelpPagesResult is what one invocation of the built program answered.
type leHelpPagesResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// leHelpPages builds the le personality once, under the names le and ze, and
// reads its help pages through pipes: the root page on stdout from every
// spelling, the namespace and keyword pages, a refusal naming one action, and
// the program name a ze_le build prints.
func leHelpPages(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-help-pages-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	lePath, zePath, err := leHelpPagesBuild(ctx, root, work)
	if err != nil {
		return err
	}
	run := func(program string, args ...string) (leHelpPagesResult, error) {
		cmd := exec.CommandContext(ctx, program, args...) //nolint:gosec // the fixture chooses the program and its arguments
		cmd.Dir = root
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

	if err := leHelpPagesRoot(run, lePath); err != nil {
		return err
	}
	if err := leHelpPagesNodes(run, lePath); err != nil {
		return err
	}
	if err := leHelpPagesRefusals(run, lePath, zePath); err != nil {
		return err
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// leHelpPagesBuild builds the le personality into work and links the ze name
// to the same binary, answering both paths.
func leHelpPagesBuild(ctx context.Context, root, work string) (string, string, error) {
	tags, err := uiLEFeatureTags(root)
	if err != nil {
		return "", "", err
	}
	lePath := filepath.Join(work, "le")
	build := exec.CommandContext(ctx, "go", "build", "-tags", strings.Join(tags, ","), "-o", lePath, "./cmd/ze") //nolint:gosec // the fixture chooses the program and its arguments
	build.Dir = root
	var buildOutput bytes.Buffer
	build.Stdout = &buildOutput
	build.Stderr = &buildOutput
	if err := build.Run(); err != nil {
		return "", "", fmt.Errorf("build the le personality: %w\n%s", err, buildOutput.String())
	}
	zePath := filepath.Join(work, "ze")
	if err := os.Link(lePath, zePath); err != nil {
		return "", "", fmt.Errorf("link the ze name: %w", err)
	}
	return lePath, zePath, nil
}

// leHelpPagesRun is one invocation of a built program.
type leHelpPagesRun func(program string, args ...string) (leHelpPagesResult, error)

// leHelpPagesRoot checks AC-3, AC-4 and AC-10: le, le help and le --help print
// one root page on stdout, laid out by tree, within 100 columns.
func leHelpPagesRoot(run leHelpPagesRun, lePath string) error {
	bare, err := run(lePath)
	if err != nil {
		return err
	}
	for _, word := range []string{leHelpPagesHelpWord, "--help"} {
		asked, err := run(lePath, word)
		if err != nil {
			return err
		}
		if asked.exitCode != 0 || asked.stderr != "" || asked.stdout != bare.stdout {
			return fmt.Errorf("le %s answered %d, stderr %q, and a page that differs from bare le", word, asked.exitCode, asked.stderr)
		}
	}
	// A namespace block is headed by what the namespace is for (AC-53,
	// D-P7-namespace), the sentence internal/le/spec/register.go registers.
	for _, want := range []string{"\nCommands:\n", "\nspec: the spec workflow: ", "  spec claim ", "[workflow] "} {
		if !strings.Contains(bare.stdout, want) {
			return fmt.Errorf("the root page lacks %q:\n%s", want, bare.stdout)
		}
	}
	if strings.Contains(bare.stdout, "Workflow (") {
		return fmt.Errorf("the root page still prints group headings:\n%s", bare.stdout)
	}
	for line := range strings.SplitSeq(bare.stdout, "\n") {
		token := strings.TrimSpace(line)
		group := strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]")
		if len(line) > 100 && strings.Contains(token, " ") && !group {
			return fmt.Errorf("a root page line of %d columns exceeds 100: %q", len(line), line)
		}
	}
	return nil
}

// leHelpPagesNodes checks AC-5, AC-8 and AC-13: the spec namespace page, the
// verify summary, and one keyword's page.
func leHelpPagesNodes(run leHelpPagesRun, lePath string) error {
	namespace, err := run(lePath, "spec", "--help")
	if err != nil {
		return err
	}
	if namespace.exitCode != 0 || !strings.Contains(namespace.stdout, "  claim ") || !strings.Contains(namespace.stdout, "[workflow]") {
		return fmt.Errorf("le spec --help answered %d:\n%s%s", namespace.exitCode, namespace.stdout, namespace.stderr)
	}
	verify, err := run(lePath, "verify", "--help")
	if err != nil {
		return err
	}
	if !strings.Contains(verify.stdout, "change set") {
		return fmt.Errorf("le verify --help does not describe the namespace:\n%s", verify.stdout)
	}
	keyword, err := run(lePath, leHelpPagesHelpWord, "site", "build", "partial")
	if err != nil {
		return err
	}
	if keyword.exitCode != 0 || !strings.Contains(keyword.stdout, "kind: switch") || !strings.Contains(keyword.stdout, "required: no") {
		return fmt.Errorf("le help site build partial answered %d:\n%s%s", keyword.exitCode, keyword.stdout, keyword.stderr)
	}
	unknown, err := run(lePath, leHelpPagesHelpWord, "site", "build", "nosuch")
	if err != nil {
		return err
	}
	if unknown.exitCode != 2 || !strings.Contains(unknown.stderr, "output <directory>") {
		return fmt.Errorf("an unknown keyword answered %d, stderr %q", unknown.exitCode, unknown.stderr)
	}
	return nil
}

// leHelpPagesRefusals checks AC-9 and AC-12: a spec claim with no spec is
// refused naming that one action, under the program that ran.
func leHelpPagesRefusals(run leHelpPagesRun, lePath, zePath string) error {
	for _, program := range []struct{ path, name string }{{lePath, "le"}, {zePath, programZeLe}} {
		args := []string{areaSpec, "claim"}
		if program.name == programZeLe {
			args = append([]string{"le"}, args...)
		}
		refused, err := run(program.path, args...)
		if err != nil {
			return err
		}
		if refused.exitCode != 2 || !strings.Contains(refused.stderr, "usage: "+program.name+" spec claim spec <spec>") {
			return fmt.Errorf("%s spec claim answered %d, stderr %q", program.name, refused.exitCode, refused.stderr)
		}
		if strings.Contains(refused.stderr, "release") {
			return fmt.Errorf("%s spec claim names another member: %q", program.name, refused.stderr)
		}
	}
	return nil
}
