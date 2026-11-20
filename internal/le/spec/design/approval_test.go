// VALIDATES: the design digest covers exactly the four sections Thomas
// approves: Task, Acceptance Criteria, Design documents, and Design
// presentation without its Choices for Thomas subsection
// (spec-design-needs-owner-approval, AC-24, C-8).
// PREVENTS: an approval that survives a change of scope, criteria, verdicts or
// presentation, and one that a Status move or his recorded answer stales.
package specdesign_test

import (
	"strings"
	"testing"

	specdesign "github.com/ze-software/ze/internal/le/spec/design"
)

// TestDesignDigestCoversFourSections is AC-24. Method: one edit inside each of
// the four sections changes the digest; an edit of Status, of the Choices
// subsection, of Files to Modify and of a later section leaves it as it was.
func TestDesignDigestCoversFourSections(t *testing.T) {
	base := specdesign.DesignDigest(completeSpecText)
	stales := map[string][2]string{
		"Task":                {"Render the widget in the surface.", "Render the gadget in the surface."},
		"Acceptance Criteria": {"the surface renders it |", "the surface refuses it |"},
		"Design documents":    {"| agrees | The surface renders |", "| departs | The surface renders |"},
		"Design presentation": {"The surface renders the widget.", "The widget renders itself."},
		"the presentation's alternatives": {"Rendering in the widget, which the decision forbids.",
			"Nothing set aside."},
	}
	for section, edit := range stales {
		changed := strings.Replace(completeSpecText, edit[0], edit[1], 1)
		if changed == completeSpecText {
			t.Fatalf("the %s edit did not apply", section)
		}
		if specdesign.DesignDigest(changed) == base {
			t.Errorf("an edit of %s left the digest unchanged", section)
		}
	}
	keeps := map[string][2]string{
		"Status":               {"| Status | design |", "| Status | ready |"},
		"Choices for Thomas":   {"### Choices for Thomas\n\nNone.", "### Choices for Thomas\n\nC-1: (a), Thomas, 2026-11-20."},
		"Files to Modify":      {"- render in the surface", "- render it in the surface"},
		"Implementation Steps": {"1. Render.", "1. Render, then test."},
	}
	for section, edit := range keeps {
		changed := strings.Replace(completeSpecText, edit[0], edit[1], 1)
		if changed == completeSpecText {
			t.Fatalf("the %s edit did not apply", section)
		}
		if specdesign.DesignDigest(changed) != base {
			t.Errorf("an edit of %s changed the digest", section)
		}
	}
}
