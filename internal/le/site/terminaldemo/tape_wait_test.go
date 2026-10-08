package siteterminaldemo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// selfSatisfiedWaits returns one finding for each Wait+Screen in the tape whose
// pattern already matches the screen the shell paints from the input alone:
// the prompt, then every line typed since the previous wait. waitFor reads the
// whole screen, typed lines included, so such a wait passes as soon as the
// command is echoed and proves nothing about what the command printed. The
// model paints a fresh prompt after each Enter, as the shell does once the
// command returns.
func selfSatisfiedWaits(tape terminalTape) (findings []string, waitCount int) {
	const prompt = "$ "
	echoed := prompt
	for _, action := range tape.actions {
		switch action.name {
		case tapeTypeCommand:
			echoed += action.text
		case "Enter":
			echoed += "\n" + prompt
		case tapeWaitCommand:
			waitCount++
			// A wait for the shell prompt follows leaving a program that owned
			// the screen, a TUI or `ze cli`. The input alone cannot say which
			// program painted the screen, so this model does not judge it.
			if action.pattern.MatchString(prompt) {
				echoed = prompt
				continue
			}
			if action.pattern.MatchString(echoed) {
				findings = append(findings, action.where+": wait /"+action.pattern.String()+"/ is satisfied by the echoed input "+strings.TrimSpace(echoed))
			}
			echoed = prompt
		}
	}
	return findings, waitCount
}

// TestTapeWaitsAreNotSatisfiedByTheTypedCommand proves that every wait in every
// published tape needs output its command produced, not the command's echo.
// The config-views recording shipped that defect: its round-trip `cmp` ran in a
// directory without the files and printed nothing, yet the wait for
// "canonical output: identical" passed on the echoed `echo` argument.
// Method: parse each demos/terminal/*/demo.tape the way the recorder does, and
// test each wait against the prompt plus the lines typed since the wait before.
func TestTapeWaitsAreNotSatisfiedByTheTypedCommand(t *testing.T) {
	// The recorder runs from the demo tree, where `Source common.tape` resolves.
	demoTreePath := filepath.Join("..", "..", "..", "..", "demos", "terminal")
	tapePaths, err := filepath.Glob(filepath.Join(demoTreePath, "*", "demo.tape"))
	if err != nil {
		t.Fatal(err)
	}
	// An empty glob means the path above no longer reaches the demo tree, and
	// a loop over nothing would pass in silence.
	if len(tapePaths) == 0 {
		t.Fatalf("no demo.tape under %s", demoTreePath)
	}
	for _, tapePath := range tapePaths {
		tape, err := parseTape(tapePath, demoTreePath)
		if err != nil {
			t.Errorf("%s: %v", tapePath, err)
			continue
		}
		findings, waitCount := selfSatisfiedWaits(tape)
		// Every published tape waits at least once, for its prepare banner, so
		// a tape with no wait was parsed into something this test cannot judge.
		if waitCount == 0 {
			t.Errorf("%s: parsed no Wait+Screen", tapePath)
		}
		for _, finding := range findings {
			t.Error(filepath.Base(filepath.Dir(tapePath)) + "/" + finding)
		}
	}
}

// TestSelfSatisfiedWaitsDiscriminates proves the detector above refuses the
// defect and accepts its fix, so a green run over the real tapes means they are
// clean rather than that the detector matches nothing.
// Method: one tape whose first wait repeats the typed text and whose second
// wait anchors on an output line the echo cannot paint.
func TestSelfSatisfiedWaitsDiscriminates(t *testing.T) {
	tapePath := filepath.Join(t.TempDir(), "demo.tape")
	body := strings.Join([]string{
		`Type "ze config cat ze.conf | grep AS-TEST"`,
		"Enter",
		"Wait+Screen /AS-TEST/",
		`Type "ze config validate ze.conf"`,
		"Enter",
		`Wait+Screen /(?m)^configuration valid/`,
	}, "\n") + "\n"
	if err := os.WriteFile(tapePath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tape, err := parseTape(tapePath, filepath.Dir(tapePath))
	if err != nil {
		t.Fatal(err)
	}
	findings, waitCount := selfSatisfiedWaits(tape)
	if waitCount != 2 {
		t.Fatalf("parsed %d waits, want 2", waitCount)
	}
	if len(findings) != 1 {
		t.Fatalf("findings %q, want exactly the AS-TEST wait", findings)
	}
	if !strings.Contains(findings[0], "AS-TEST") {
		t.Fatalf("finding %q does not name the AS-TEST wait", findings[0])
	}
}
