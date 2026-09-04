// Detail: completer.go -- mergeAugmentedEntries, mergeHelpExts

package cli

import (
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// helpEntry answers one declaration of a node: a container carrying its own
// ze:help and one child, which is the shape a plugin module writes when it
// attaches leaves to a node another module also declares.
func helpEntry(name, help, child string) *gyang.Entry {
	return &gyang.Entry{
		Name: name,
		Exts: []*gyang.Statement{{
			Keyword:     yang.HelpExtensionKeyword,
			HasArgument: true,
			Argument:    help,
		}},
		Dir: map[string]*gyang.Entry{child: {Name: child}},
	}
}

// TestMergeKeepsEveryDeclarationsHelp checks that a node several modules
// declare keeps every module's explanation. The goal is the operator's ? box:
// before this, the merged entry took its whole extension list from the first
// input, so the help of every later module was unreachable and the winner was
// decided by the alphabetical order of the module names.
func TestMergeKeepsEveryDeclarationsHelp(t *testing.T) {
	first := helpEntry("interface", "What an interface is.", "backend")
	second := helpEntry("interface", "What the QoS plugin attaches here.", "class-of-service")

	merged := mergeAugmentedEntries([]*gyang.Entry{first, second})
	help := entryLongHelp(merged)

	if !strings.Contains(help, "What an interface is.") {
		t.Errorf("merged help lost the first declaration: %q", help)
	}
	if !strings.Contains(help, "What the QoS plugin attaches here.") {
		t.Errorf("merged help lost the second declaration: %q", help)
	}
	if _, ok := merged.Dir["backend"]; !ok {
		t.Error("merged entry lost the first declaration's child")
	}
	if _, ok := merged.Dir["class-of-service"]; !ok {
		t.Error("merged entry lost the second declaration's child")
	}
	if got := entryLongHelp(first); got != "What an interface is." {
		t.Errorf("the input entry was mutated: %q", got)
	}
}

// TestMergeRepeatsNoHelpTwice checks the case where two modules carry the same
// sentence, which happens where one module copied the other's wording. The
// operator reads it once.
func TestMergeRepeatsNoHelpTwice(t *testing.T) {
	same := "One sentence, written twice."
	merged := mergeAugmentedEntries([]*gyang.Entry{
		helpEntry("interface", same, "backend"),
		helpEntry("interface", same, "class-of-service"),
	})

	if got := entryLongHelp(merged); got != same {
		t.Errorf("help = %q, want the sentence once", got)
	}
}

// TestMergeLeavesASingleDeclarationAlone checks that a node only one module
// declares keeps the entry the loader parsed, rather than a copy.
func TestMergeLeavesASingleDeclarationAlone(t *testing.T) {
	only := helpEntry("interface", "The only explanation.", "backend")

	if merged := mergeAugmentedEntries([]*gyang.Entry{only}); merged != only {
		t.Error("a single declaration was wrapped rather than returned unchanged")
	}
}

// TestMergeKeepsTheHelpOfTheDeclarationThatCarriesOne checks the common shape:
// one module writes the prose and another declares the node to attach a leaf.
// The silent declaration must not erase the written one, whichever order the
// module names sort in.
func TestMergeKeepsTheHelpOfTheDeclarationThatCarriesOne(t *testing.T) {
	silent := &gyang.Entry{Name: "interface", Dir: map[string]*gyang.Entry{"class-of-service": {Name: "class-of-service"}}}
	written := helpEntry("interface", "The explanation the operator needs.", "backend")

	merged := mergeAugmentedEntries([]*gyang.Entry{silent, written})
	if got := entryLongHelp(merged); got != "The explanation the operator needs." {
		t.Errorf("help = %q, want the written explanation", got)
	}
}
