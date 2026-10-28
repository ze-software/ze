package cli

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestTreeDiffListFollowsWorkingOrder proves AC-8: after the operator adds
// `peer alpha` behind the existing `peer zulu`, `show | compare` prints the
// two peers in the order the working content holds them (zulu, then alpha),
// marks the `peer alpha` line as added, and maps that display line to the
// working line that holds `peer alpha`, so a validation error lands on it.
// The method builds the working content in operator order, runs the tree diff
// against a baseline holding only zulu, and reads the display and its mapping.
func TestTreeDiffListFollowsWorkingOrder(t *testing.T) {
	schema, err := config.YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}
	peer := func(name, ip string) string {
		return "\tpeer " + name + " {\n\t\tconnection {\n\t\t\tremote {\n\t\t\t\tip " + ip + "\n\t\t\t}\n\t\t}\n\t}\n"
	}
	original := "bgp {\n" + peer("zulu", "192.0.2.26") + "}\n"
	working := "bgp {\n" + peer("zulu", "192.0.2.26") + peer("alpha", "192.0.2.1") + "}\n"

	content, lineMapping := annotateContentWithTreeDiff(original, working, schema)
	display := strings.Split(content, "\n")

	zuluDisplay, alphaDisplay := 0, 0
	for i, line := range display {
		if strings.Contains(line, "peer zulu {") {
			zuluDisplay = i + 1
		}
		if strings.Contains(line, "peer alpha {") {
			alphaDisplay = i + 1
		}
	}
	if zuluDisplay == 0 {
		t.Fatalf("the compare view lacks `peer zulu {`:\n%s", content)
	}
	if alphaDisplay == 0 {
		t.Fatalf("the compare view lacks `peer alpha {`:\n%s", content)
	}
	if alphaDisplay < zuluDisplay {
		t.Errorf("the compare view prints peer alpha before peer zulu, want the working order:\n%s", content)
	}
	if !strings.HasPrefix(display[alphaDisplay-1], string(diffAdded)) {
		t.Errorf("display line %d %q is not marked added:\n%s", alphaDisplay, display[alphaDisplay-1], content)
	}
	if strings.HasPrefix(display[zuluDisplay-1], string(diffAdded)) {
		t.Errorf("display line %d %q is marked added, but peer zulu is unchanged:\n%s", zuluDisplay, display[zuluDisplay-1], content)
	}

	workingAlpha := 0
	for i, line := range strings.Split(working, "\n") {
		if strings.Contains(line, "peer alpha {") {
			workingAlpha = i + 1
		}
	}
	if got := lineMapping[alphaDisplay]; got != workingAlpha {
		t.Errorf("display line %d (`peer alpha {`) maps to working line %d, want %d\ndisplay:\n%s\nworking:\n%s",
			alphaDisplay, got, workingAlpha, content, working)
	}
}
