// Related: writeedit.go -- the plan-location checks these cases drive
//
// VALIDATES: a Claude Code plan file is written to plan/drafts/ze-plan-<name>.
// A Write under .claude/plan/ or .claude/plans/ is refused with the plan/drafts/
// path to use, a misnamed file under plan/drafts/ is refused, and a draft is a
// design document the source-read evidence check judges.
// PREVENTS: a plan landing under .claude/, which the owner rule forbids (plans
// live only in subfolders under plan/), and a draft written before any source
// was read.
package hookruntime

import (
	"path/filepath"
	"strings"
	"testing"
)

// planWrite is a Write of a small plan body at a checkout-relative path.
func planWrite(root, relative string) context {
	path := filepath.Join(root, filepath.FromSlash(relative))
	return context{
		root: root, tool: toolWrite, path: path, content: "# Plan\n",
		input:   map[string]any{"file_path": path, "content": "# Plan\n"},
		payload: Payload{SessionID: "plan-sid"},
	}
}

func TestWriteFilePatternsRedirectsAPlanUnderClaudeToPlanDrafts(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{".claude/plan/ze-plan-x.md", ".claude/plans/x.md"} {
		refused := writeFilePatterns(planWrite(root, relative))
		if refused == nil || refused.code != 2 {
			t.Fatalf("%s = %#v, want the Write refused", relative, refused)
		}
		if !strings.Contains(refused.message, claudePlanDir+"ze-plan-<name>") {
			t.Fatalf("%s refusal does not name %s: %q", relative, claudePlanDir, refused.message)
		}
	}
}

func TestWriteFilePatternsRequiresTheZePlanPrefixUnderPlanDrafts(t *testing.T) {
	root := t.TempDir()
	refused := writeFilePatterns(planWrite(root, "plan/drafts/notes.md"))
	if refused == nil || refused.code != 2 || !strings.Contains(refused.message, "ze-plan-notes.md") {
		t.Fatalf("misnamed draft = %#v, want it refused with the ze-plan- name", refused)
	}
	if admitted := writeFilePatterns(planWrite(root, "plan/drafts/ze-plan-notes.md")); admitted != nil {
		t.Fatalf("a ze-plan- draft was refused: %#v", admitted)
	}
	if admitted := writeFilePatterns(planWrite(root, "plan/records/notes.md")); admitted != nil {
		t.Fatalf("a file in another plan/ folder met the draft naming rule: %#v", admitted)
	}
}

func TestWriteDesignEvidenceJudgesAPlanDraft(t *testing.T) {
	root := t.TempDir()
	refused := writeDesignEvidence(planWrite(root, "plan/drafts/ze-plan-notes.md"))
	if refused == nil || refused.code != 2 || !strings.Contains(refused.message, "read the producing source") {
		t.Fatalf("draft with no source read = %#v, want it refused", refused)
	}
	if admitted := writeDesignEvidence(planWrite(root, "plan/records/notes.md")); admitted != nil {
		t.Fatalf("a record outside plan/drafts/ was judged as a design: %#v", admitted)
	}
}
