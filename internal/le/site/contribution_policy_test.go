// Design: website/AI.md -- the approved AI contribution policy has one canonical source.
// Related: CONTRIBUTING.md -- the owner-approved wording and model examples.
package site

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestContributionAIPolicyPreservesOwnerApproval reads the canonical section
// and compares its complete wording with Thomas's approval. The recommendation
// must remain optional, including for models beyond the named examples.
func TestContributionAIPolicyPreservesOwnerApproval(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, "CONTRIBUTING.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, section, found := strings.Cut(string(source), "## AI-assisted contributions\n")
	if !found {
		t.Fatal("CONTRIBUTING.md has no canonical AI contribution policy")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	const approved = "AI-assisted contributions are welcome. You may use AI to research a problem, " +
		"develop and test a solution, and prepare a pull request. " +
		"We prefer recent, capable models such as Sol 6.1, Astra, Opus 5.5 or Fable 5.2, " +
		"and newer models of comparable capability. This is a recommendation, not a requirement. " +
		"Every contribution must meet the same specification, testing, documentation and independent review requirements."
	if got := strings.Join(strings.Fields(section), " "); got != approved {
		t.Errorf("AI contribution policy changed from the owner-approved wording:\n%s", section)
	}
}

// TestContributionPagesPointToCanonicalAIPolicy renders both contribution
// pages through the normal Markdown renderer. Each page must welcome AI
// assistance and recommend capable models, with examples kept at one link.
func TestContributionPagesPointToCanonicalAIPolicy(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	const policy = "https://github.com/ze-software/ze/blob/main/CONTRIBUTING.md#ai-assisted-contributions"
	for _, name := range []string{"contribute.md", "guide.md"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(root, "website", "contribute", name))
			if err != nil {
				t.Fatal(err)
			}
			body, _, err := renderMarkdown(source)
			if err != nil {
				t.Fatal(err)
			}
			_, paragraph, found := strings.Cut(body, "AI-assisted contributions are welcome.")
			if !found {
				t.Fatal("contribution page no longer welcomes AI-assisted contributions")
			}
			paragraph, _, found = strings.Cut(paragraph, "</p>")
			if !found {
				t.Fatal("the AI contribution introduction has no closing paragraph")
			}
			if !slices.Contains(linkTargets(paragraph), policy) {
				t.Error("the AI contribution introduction has no link to the canonical policy")
			}
			if !strings.Contains(strings.ToLower(paragraph), "recommend") {
				t.Error("contribution page no longer describes the model preference as a recommendation")
			}
			for _, model := range []string{"Sol 6.1", "Astra", "Opus 5.5", "Fable 5.2"} {
				if strings.Contains(body, model) {
					t.Errorf("contribution page repeats model example %q; the canonical policy owns those examples", model)
				}
			}
		})
	}
}
