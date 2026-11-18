// Design: docs/architecture/cli/error-surface.md -- checked operator-error wire text.
package opererr

import (
	"errors"
	"strings"
	"testing"
)

// TestFromRenderedKeepsTheWireText reconstructs actual native wire messages,
// including nested and multiline evidence, without changing one received byte.
func TestFromRenderedKeepsTheWireText(t *testing.T) {
	for _, test := range []struct {
		name string
		err  *Error
	}{
		{name: "three parts", err: New("The source is unavailable.", "No source was changed.", "ze show doctor")},
		{name: "native cause", err: Wrap(errors.New("storage read failed"), "The source is unavailable.", "No source was changed.", "ze show doctor")},
		{name: "multiline explanation", err: New("The source is unavailable.", "The read failed.\nThe source was retained.", "ze show doctor")},
		{name: "joined causes", err: Wrap(errors.Join(errors.New("storage read failed"), errors.New("storage release failed")), "The source is unavailable.", "No complete source was read.", "ze show doctor")},
		{name: "nested operator cause", err: Wrap(New("The source is unavailable.", "The source was retained.", "ze show doctor"), "The backup operation could not complete.", "The live configuration is unchanged.", "Open /backups/")},
		{name: "multiline correction", err: New("The source is unavailable.", "Both settings must be repaired.", "set first value\nset second value")},
		{name: "multiline problem", err: New("The source is unavailable.\nIts retained identity is known.", "No source was changed.", "ze show doctor")},
		{name: "literal edge whitespace", err: New(" The source is unavailable. ", " No source was changed. ", " ze show doctor ")},
	} {
		t.Run(test.name, func(t *testing.T) {
			wire := test.err.Error()
			got, err := FromRendered(wire)
			if err != nil {
				t.Fatalf("valid native wire refusal rejected: %v", err)
			}
			if got == nil {
				t.Fatal("valid refusal decoded to nil")
			}
			if got.Error() != wire {
				t.Fatalf("wire changed:\n%q\nwant:\n%q", got.Error(), wire)
			}
			if got.Problem() == "" || got.Why() == "" || got.Fix() == "" {
				t.Fatal("decoded refusal has an empty required part")
			}
			if got.Fix() != test.err.Fix() {
				t.Fatalf("final correction changed: %q, want %q", got.Fix(), test.err.Fix())
			}
			if got.Unwrap() != nil {
				t.Fatal("wire text invented an in-process cause identity")
			}
		})
	}
}

// TestFromRenderedRefusesIncompleteWireText rejects a malformed transport
// message without constructing an empty operator error or panicking on input.
func TestFromRenderedRefusesIncompleteWireText(t *testing.T) {
	for name, wire := range map[string]string{
		"empty":               "",
		"problem only":        "The source is unavailable.",
		"no correction":       "The source is unavailable.\nNo source was changed.",
		"empty problem":       "\nNo source was changed.\nfix: ze show doctor",
		"blank problem":       " \t\nNo source was changed.\nfix: ze show doctor",
		"empty explanation":   "The source is unavailable.\n\nfix: ze show doctor",
		"blank explanation":   "The source is unavailable.\n \t\nfix: ze show doctor",
		"empty correction":    "The source is unavailable.\nNo source was changed.\nfix: ",
		"blank correction":    "The source is unavailable.\nNo source was changed.\nfix:  \t",
		"wrong label":         "The source is unavailable.\nNo source was changed.\nrepair: ze show doctor",
		"label before reason": "The source is unavailable.\nfix: ze show doctor",
		"label in one line":   "The source is unavailable; no source was changed; fix: ze show doctor",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := FromRendered(wire)
			if err == nil {
				t.Fatalf("incomplete wire text accepted as %v", got)
			}
			if got != nil {
				t.Fatal("refused wire text published a usable operator error")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("wire refusal has no diagnosis")
			}
		})
	}
}
