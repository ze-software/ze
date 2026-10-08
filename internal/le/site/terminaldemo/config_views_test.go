package siteterminaldemo

import (
	"path/filepath"
	"testing"
)

// TestConfigViewsWaitsAreNotSatisfiedByTheTypedCommand proves that every wait
// in the config-views tape needs the command's output, not its echo. The
// terminal echoes each typed line onto the screen the wait reads, so a pattern
// that also matches the typed text passes before the command has answered.
// The published recording carried that defect: the round-trip `cmp` ran in a
// directory without the files, printed nothing, and the wait for
// "canonical output: identical" passed on the echoed `echo` argument.
// Method: parse the committed tape, and for each wait test its pattern against
// the prompt plus the text typed since the previous wait.
func TestConfigViewsWaitsAreNotSatisfiedByTheTypedCommand(t *testing.T) {
	// The recorder runs from the demo tree, where `Source common.tape` resolves.
	demoTreePath := filepath.Join("..", "..", "..", "..", "demos", "terminal")
	tapePath := filepath.Join(demoTreePath, demoConfigViews, "demo.tape")
	tape, err := parseTape(tapePath, demoTreePath)
	if err != nil {
		t.Fatal(err)
	}
	waitCount := 0
	typed := ""
	for _, action := range tape.actions {
		switch action.name {
		case tapeTypeCommand:
			typed += action.text
		case tapeWaitCommand:
			waitCount++
			echoed := "$ " + typed
			if action.pattern.MatchString(echoed) {
				t.Errorf("%s: wait %q is satisfied by the echoed input %q", action.where, action.pattern.String(), echoed)
			}
			typed = ""
		}
	}
	// The tape waits six times: the prepare banner, the intro card, and four
	// command outputs. Another count means the loop above judged a different
	// tape than the one this test describes, so its silence proves nothing.
	const waitCountWant = 6
	if waitCount != waitCountWant {
		t.Fatalf("parsed %d waits from %s, want %d", waitCount, tapePath, waitCountWant)
	}
}
