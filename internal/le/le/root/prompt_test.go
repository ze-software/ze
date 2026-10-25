// VALIDATES: AC-45 (optional values are never asked), AC-46 in le (a canceled prompt runs nothing, exit 2),
// AC-44's echo (the completed line is printed to stderr before it runs), A-11 (nothing is asked off a terminal)
// PREVENTS: a prompt that asks what the action can run without, or a line that runs half-filled

package leroot

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/cli/menu"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

const promptProbe = "prompt-probe"

var (
	promptProbeOnce sync.Once
	promptProbeRuns []leaction.Arguments
)

// registerPromptProbe registers, once per process, an area whose one action
// takes a Required positional path, a Required keyword and an Optional one,
// and records the arguments each run receives.
func registerPromptProbe() {
	promptProbeOnce.Do(func() {
		area := leaction.New(promptProbe, leaction.Action{
			Verb: "run", Why: "run the probe over one target",
			Parameters: []leaction.Parameter{
				{Value: "target", Requirement: leaction.Required, Kind: leaction.KindPath, Help: "the target"},
				{Keyword: "scope", Value: "packages", Requirement: leaction.Required, Kind: leaction.KindString, Help: "the scope"},
				{Keyword: "timeout", Value: "duration", Requirement: leaction.Optional, Kind: leaction.KindString, Help: "the bound"},
			},
			AnswerArgs: func(args leaction.Arguments) (any, int) {
				promptProbeRuns = append(promptProbeRuns, args)
				return nil, 0
			},
		})
		Register(area.Name(), GroupSuite, area.Answer, registry.Meta{
			ShortHelp: "an area whose action needs values",
			Mode:      "offline", Section: registry.SectionTest,
		})
		RegisterActions(area.Name(), area.Actions)
	})
}

// atTerminal makes this test a person at a terminal whose every prompt is
// answered by answer, and answers the labels le asked for, in order.
func atTerminal(t *testing.T, answer func(menu.Prompt) (string, error)) *[]string {
	t.Helper()
	savedInteractive, savedAsk := interactive, askValue
	t.Cleanup(func() { interactive, askValue = savedInteractive, savedAsk })
	var asked []string
	interactive = func() bool { return true }
	askValue = func(prompt menu.Prompt) (string, error) {
		asked = append(asked, prompt.Label)
		return answer(prompt)
	}
	promptProbeRuns = nil
	return &asked
}

// TestOptionalParametersAreNeverPrompted proves a terminal line that gives
// every Required value runs with no prompt and no echo, and a line that gives
// none asks for exactly the Required ones, positional first, never the
// Optional one, then echoes the completed line and runs it. Method: drive
// Dispatch on a probe area with the terminal and the prompt replaced.
func TestOptionalParametersAreNeverPrompted(t *testing.T) {
	registerPromptProbe()
	asked := atTerminal(t, func(prompt menu.Prompt) (string, error) { return "given-" + prompt.Label, nil })

	var code int
	echo := captureStderr(t, func() { code = Dispatch("le", []string{promptProbe, "run", "./x", "scope", "s"}) })
	if code != 0 || len(*asked) != 0 || echo != "" {
		t.Errorf("a complete line: code %d, asked %v, stderr %q; want 0, nothing asked, nothing echoed", code, *asked, echo)
	}
	if len(promptProbeRuns) != 1 || promptProbeRuns[0].Has("timeout") {
		t.Fatalf("a complete line ran %v, want one run without timeout", promptProbeRuns)
	}

	echo = captureStderr(t, func() { code = Dispatch("le", []string{promptProbe, "run"}) })
	if !slices.Equal(*asked, []string{"target", "packages"}) {
		t.Errorf("le asked %v, want [target packages]: the Required values only, positional first", *asked)
	}
	if code != 0 || len(promptProbeRuns) != 2 {
		t.Fatalf("the filled line answered %d with %d runs, want 0 and a second run", code, len(promptProbeRuns))
	}
	run := promptProbeRuns[1]
	if run.One("target") != "given-target" || run.One("scope") != "given-packages" || run.Has("timeout") {
		t.Errorf("the filled line ran with %v", run)
	}
	if want := "running: le prompt-probe run given-target scope given-packages\n"; echo != want {
		t.Errorf("echo = %q, want %q", echo, want)
	}
}

// TestAPromptCancelInLeRunsNothing proves the reader leaving a prompt runs
// nothing and answers 2, and asks for no value after the canceled one.
func TestAPromptCancelInLeRunsNothing(t *testing.T) {
	registerPromptProbe()
	asked := atTerminal(t, func(menu.Prompt) (string, error) { return "", menu.ErrCanceled })

	var code int
	message := captureStderr(t, func() { code = Dispatch("le", []string{promptProbe, "run"}) })
	if code != 2 {
		t.Errorf("a canceled prompt answered %d, want 2", code)
	}
	if len(promptProbeRuns) != 0 {
		t.Errorf("a canceled prompt ran the action %d time(s)", len(promptProbeRuns))
	}
	if len(*asked) != 1 {
		t.Errorf("le asked %v after the cancel, want only the first value", *asked)
	}
	if !strings.Contains(message, "nothing ran") {
		t.Errorf("the cancel says %q, which does not say nothing ran", message)
	}
}

// TestALineOffATerminalIsNeverPrompted proves a line on pipes asks nothing:
// the action runs and its body answers for the missing value, as before.
func TestALineOffATerminalIsNeverPrompted(t *testing.T) {
	registerPromptProbe()
	asked := atTerminal(t, func(menu.Prompt) (string, error) { return "x", nil })
	interactive = func() bool { return false }

	captureStderr(t, func() { Dispatch("le", []string{promptProbe, "run"}) })
	if len(*asked) != 0 {
		t.Errorf("le asked %v on a pipe", *asked)
	}
}

// TestPromptCandidatesAreTheCompletionCallbacks proves a prompt's Tab offers
// what `le completion words` offers for the same place, plus paths on disk for
// a path value. Method: ask the completer the prompt for the positional path
// receives, with the prefix of a file in this package.
func TestPromptCandidatesAreTheCompletionCallbacks(t *testing.T) {
	registerPromptProbe()
	var offered []string
	atTerminal(t, func(prompt menu.Prompt) (string, error) {
		if prompt.Label == "target" {
			offered = prompt.Complete("prompt_te")
		}
		return "x", nil
	})
	captureStderr(t, func() { Dispatch("le", []string{promptProbe, "run"}) })
	if !slices.Contains(offered, "prompt_test.go") {
		t.Errorf("the path prompt offered %v, want prompt_test.go", offered)
	}
}
