package fixture

// Related: register_le_help_ai.go -- the registration.
// Related: ui_fixture_le_help_pages.go -- leHelpPagesBuild and the run helper this driver shares.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// leHelpAiSections are the sections le help ai holds (AC-39).
var leHelpAiSections = []string{fieldCommands, "groups", "verbs", "exit-codes"}

// leHelpAiReference is the part of le help ai's JSON this driver reads.
type leHelpAiReference struct {
	Program  string `json:"program"`
	Commands []struct {
		Name    string `json:"name"`
		Actions []struct {
			Verb       string `json:"verb"`
			Parameters []struct {
				Keyword  string   `json:"keyword"`
				Value    string   `json:"value"`
				Kind     string   `json:"kind"`
				Help     string   `json:"help"`
				Required bool     `json:"required"`
				Values   []string `json:"values"`
			} `json:"parameters"`
		} `json:"actions"`
	} `json:"commands"`
}

// leHelpAi builds the le personality, under the names le and ze, and reads le
// help ai through pipes: the text summary, every command's typed parameters as
// JSON, every section as YAML, an unknown section and a rendering flag refused
// with the usage code, and the same payload under ze le naming that program.
func leHelpAi(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-help-ai-")
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

	if err := leHelpAiText(run, lePath); err != nil {
		return err
	}
	if err := leHelpAiRendered(run, lePath, zePath); err != nil {
		return err
	}
	if err := leHelpAiRefusals(run, lePath); err != nil {
		return err
	}
	// The .ci reads this line; an exit code of 0 alone would also pass a
	// driver that returned before its checks ran.
	fmt.Println("OK")
	return nil
}

// leHelpAiText checks AC-38: le help ai prints a text summary naming every
// section and the JSON form, exit 0.
func leHelpAiText(run leHelpPagesRun, lePath string) error {
	summary, err := run(lePath, leHelpPagesHelpWord, "ai")
	if err != nil {
		return err
	}
	if summary.exitCode != 0 || summary.stderr != "" {
		return fmt.Errorf("le help ai answered %d, stderr %q", summary.exitCode, summary.stderr)
	}
	for _, want := range append([]string{"le help ai | json"}, leHelpAiSections...) {
		if !strings.Contains(summary.stdout, want) {
			return fmt.Errorf("le help ai does not name %q:\n%s", want, summary.stdout)
		}
	}
	return nil
}

// leHelpAiRendered checks AC-39 and the help ai half of AC-48: commands as
// JSON carry each action's typed parameters, all as YAML holds every section,
// and ze le answers the same commands under its own program name.
func leHelpAiRendered(run leHelpPagesRun, lePath, zePath string) error {
	le, err := leHelpAiCommands(run, lePath)
	if err != nil {
		return err
	}
	if le.Program != "le" {
		return fmt.Errorf("le help ai commands names the program %q, want le", le.Program)
	}
	claim := false
	for _, command := range le.Commands {
		for _, action := range command.Actions {
			for _, parameter := range action.Parameters {
				if parameter.Kind == "" || parameter.Help == "" {
					return fmt.Errorf("%s %s %s%s carries no kind or no help", command.Name, action.Verb, parameter.Keyword, parameter.Value)
				}
				if command.Name == "spec claim" && parameter.Value == areaSpec && parameter.Kind == areaSpec && parameter.Required {
					claim = true
				}
			}
		}
	}
	if !claim {
		return errors.New("le help ai commands does not carry spec claim's required spec parameter")
	}
	ze, err := leHelpAiCommands(run, zePath, "le")
	if err != nil {
		return err
	}
	if ze.Program != programZeLe || len(ze.Commands) != len(le.Commands) {
		return fmt.Errorf("ze le help ai commands names %q with %d commands; le answered %d", ze.Program, len(ze.Commands), len(le.Commands))
	}
	all, err := run(lePath, leHelpPagesHelpWord, "ai", "all", "|", "yaml")
	if err != nil {
		return err
	}
	if all.exitCode != 0 {
		return fmt.Errorf("le help ai all | yaml answered %d: %s", all.exitCode, all.stderr)
	}
	for _, section := range leHelpAiSections {
		if !strings.Contains("\n"+all.stdout, "\n"+section+":") {
			return fmt.Errorf("le help ai all | yaml holds no %s section", section)
		}
	}
	return nil
}

// leHelpAiCommands runs `<program> [prefix...] help ai commands | json` and
// decodes it.
func leHelpAiCommands(run leHelpPagesRun, program string, prefix ...string) (leHelpAiReference, error) {
	args := slices.Concat(prefix, []string{leHelpPagesHelpWord, "ai", fieldCommands, "|", renderJSON})
	answer, err := run(program, args...)
	if err != nil {
		return leHelpAiReference{}, err
	}
	if answer.exitCode != 0 {
		return leHelpAiReference{}, fmt.Errorf("%s answered %d: %s", strings.Join(args, " "), answer.exitCode, answer.stderr)
	}
	var reference leHelpAiReference
	if err := json.Unmarshal([]byte(answer.stdout), &reference); err != nil {
		return leHelpAiReference{}, fmt.Errorf("%s is not JSON: %w", strings.Join(args, " "), err)
	}
	if len(reference.Commands) == 0 {
		return leHelpAiReference{}, fmt.Errorf("%s lists no command", strings.Join(args, " "))
	}
	return reference, nil
}

// leHelpAiRefusals checks AC-39's unknown section and AC-40: each is refused
// with the usage code, the first listing the sections, the flags naming the
// pipe.
func leHelpAiRefusals(run leHelpPagesRun, lePath string) error {
	unknown, err := run(lePath, leHelpPagesHelpWord, "ai", "nosuch")
	if err != nil {
		return err
	}
	if unknown.exitCode != 2 || !strings.Contains(unknown.stderr, strings.Join(leHelpAiSections, " | ")) {
		return fmt.Errorf("le help ai nosuch answered %d, stderr %q", unknown.exitCode, unknown.stderr)
	}
	for _, flag := range []string{flagJSON, "--ai"} {
		refused, err := run(lePath, leHelpPagesHelpWord, "ai", flag)
		if err != nil {
			return err
		}
		if refused.exitCode != 2 || !strings.Contains(refused.stderr, "le help ai | json") {
			return fmt.Errorf("le help ai %s answered %d, stderr %q", flag, refused.exitCode, refused.stderr)
		}
	}
	return nil
}
