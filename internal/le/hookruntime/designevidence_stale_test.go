// Design: docs/contributing/spec-workflow.md -- a missing path is judged where the proposed text cites it
// VALIDATES: a missing producer path git never deleted refuses only when the
// proposed spec text still cites it: an edit that replaces the stale citation
// passes, an edit that keeps it or adds one refuses with the existence message.
// PREVENTS: a stale citation in the selected spec refusing every edit of that
// spec, including the edit that corrects it.
package hookruntime

import (
	"strings"
	"testing"
)

// TestDesignEvidenceAcceptsEditDroppingMissingPath checks that a missing path
// cited only by the current text owes no existence: the edit that corrects or
// removes it, or moves it to Files to Create, is not refused.
//
// MUTATION: (internal/le/hookruntime/designevidence.go, "if !source.proposed {", "if !source.proposed && source.proposed {")
// turns it red: the current pass judges the stale path again and refuses it.
func TestDesignEvidenceAcceptsEditDroppingMissingPath(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		stale := "## " + heading + "\n`" + evidenceProbeA + "`\n"
		for _, test := range []struct {
			name     string
			proposed string
		}{
			{"corrected", "## " + heading + "\n`" + evidenceProbeB + "`\n"},
			{"removed", "## " + heading + "\nNone.\n"},
			{"moved-to-files-to-create", "## Files to Create\n`" + evidenceProbeA + "`\n"},
		} {
			t.Run(heading+"/"+test.name, func(t *testing.T) {
				root := shorthandFixture(t)
				writeHookFixture(t, root, evidenceProbeSpec, stale)
				observedDesignGate(t, root, test.proposed, 0)
			})
		}
	}
}

// TestDesignEvidenceRefusesEditKeepingMissingPath checks that a missing path
// git never deleted still refuses when the edit leaves it cited.
//
// MUTATION: (internal/le/hookruntime/designevidence.go, "if !source.proposed {", "if source.proposed || !source.proposed {")
// turns it red: the proposed pass skips the kept missing path as well.
func TestDesignEvidenceRefusesEditKeepingMissingPath(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := shorthandFixture(t)
			stale := "## " + heading + "\n`" + evidenceProbeA + "`\n"
			writeHookFixture(t, root, evidenceProbeSpec, stale)
			message := observedDesignGate(t, root, stale+"\nEdited.\n", 2)
			if !strings.Contains(message, evidenceProbeA+": inspect producer") || !strings.Contains(message, "git history never deleted it") {
				t.Fatalf("a kept missing path did not refuse for existence: %s", message)
			}
		})
	}
}

// TestDesignEvidenceRefusesEditAddingMissingPath checks that a missing path
// git never deleted refuses when the edit is the one that cites it.
//
// MUTATION: (internal/le/hookruntime/designevidence.go, "}{{current, false}, {proposed, true}} {", "}{{current, false}, {proposed, false}} {")
// turns it red: the proposed text's missing paths are skipped like the current text's.
func TestDesignEvidenceRefusesEditAddingMissingPath(t *testing.T) {
	for _, heading := range []string{"Current Behavior", "Files to Modify"} {
		t.Run(heading, func(t *testing.T) {
			root := shorthandFixture(t)
			live := "## Files to Create\n`internal/le/producer_new.go`\n" // <!-- doc-links: ignore (a synthetic path in a test fixture) -->
			writeHookFixture(t, root, evidenceProbeSpec, live)
			message := observedDesignGate(t, root, live+"## "+heading+"\n`"+evidenceProbeA+"`\n", 2)
			if !strings.Contains(message, evidenceProbeA+": inspect producer") || !strings.Contains(message, "git history never deleted it") {
				t.Fatalf("an added missing path did not refuse for existence: %s", message)
			}
		})
	}
}
