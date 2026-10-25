// Design: docs/contributing/running-commands.md -- le's hidden commands
// Related: hidden.go -- the hidden set these tests pin

package leroot

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// hiddenProbe is a hidden area under the tree probe's namespace.
const hiddenProbe = "treeprobe hidden"

// registerHiddenProbe registers one forwarding area marked hidden, beside the
// tree probe, so the namespace holds a visible member and a hidden one.
func registerHiddenProbe(t *testing.T) {
	t.Helper()
	treeProbe(t)
	Register(hiddenProbe, GroupTool, func([]string) (any, int) { return nil, 0 }, registry.Meta{
		Description: "A hidden probe area for this test.", ShortHelp: "a hidden probe area",
		Mode: "offline", Section: registry.SectionTest,
	})
	RegisterForwarding(hiddenProbe, "the probe program")
	RegisterHidden(hiddenProbe)
}

// VALIDATES: AC-23 -- a hidden area is absent from the root page, its
// namespace's page and refusal, completion and the menu's tree, is listed by
// `le help ai commands` marked hidden, and still dispatches.
// PREVENTS: a spawned helper offered to a person as a command to type, or a
// hidden word le accepts that no help names.
func TestHiddenAreaIsOnlyInHelpAI(t *testing.T) {
	registerHiddenProbe(t)

	if !Hidden(hiddenProbe) {
		t.Fatalf("%q was registered hidden and Hidden answers false", hiddenProbe)
	}
	if Hidden("treeprobe area") {
		t.Error("the visible probe answers hidden")
	}

	if nodeAt(Tree(), []string{"treeprobe", "hidden"}) != nil {
		t.Error("the tree, which completion and the menu read, offers the hidden area")
	}
	if nodeAt(Tree(), []string{"treeprobe", "area"}) == nil {
		t.Error("the tree lost the visible area beside the hidden one")
	}

	candidates, err := Complete([]string{"treeprobe", ""})
	if err != nil {
		t.Fatalf("complete treeprobe: %v", err)
	}
	for _, candidate := range candidates {
		if candidate.Value == "hidden" {
			t.Error("completion offers the hidden area")
		}
	}

	for _, entry := range childEntries("treeprobe") {
		if entry.Name == "hidden" {
			t.Error("the namespace page lists the hidden area")
		}
	}
	if slices.Contains(members("treeprobe"), "hidden") {
		t.Error("the namespace refusal names the hidden area")
	}

	manifest := manifestOf("le")
	for _, section := range manifest.sections() {
		for _, entry := range section.Entries {
			if entry.Name == hiddenProbe {
				t.Error("the root page lists the hidden area")
			}
		}
	}
	marked := false
	for _, area := range manifest.Areas {
		if area.Name == hiddenProbe {
			marked = area.Hidden
		}
	}
	if !marked {
		t.Error("the root payload does not carry the hidden area marked hidden")
	}

	var ref reference.Reference
	fillCommands(&ref, "le")
	listed := false
	for _, entry := range ref.Commands {
		if entry.Name == hiddenProbe {
			listed = entry.Hidden
		}
	}
	if !listed {
		t.Error("le help ai commands does not list the hidden area marked hidden")
	}

	if name, handler, _ := resolve([]string{"treeprobe", "hidden"}); name != hiddenProbe || handler == nil {
		t.Errorf("the hidden area does not dispatch: resolved %q", name)
	}
}
