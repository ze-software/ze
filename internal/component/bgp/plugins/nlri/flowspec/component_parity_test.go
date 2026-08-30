// Design: docs/architecture/bgp/on-demand-origination.md -- announce states its grammar in the model
package flowspec

import (
	"os"
	"sort"
	"strings"
	"testing"

	flowspecyang "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec/yang"
)

const (
	// encoderSourceFile is the file isComponentKeyword lives in, so the set
	// this test reads and the function that accepts it cannot come from two
	// different places.
	encoderSourceFile = "plugin_encode_text.go"

	// encoderKeywordMarker opens the const block that declares the component
	// keywords, and it is the whole reason the block can be found by reading.
	encoderKeywordMarker = "// FlowSpec component keywords."

	// componentCountHint sizes the two slices this test builds. RFC 8955 and
	// RFC 8956 define thirteen component types between them, and Ze spells
	// several of them more than one way, so the count is a hint and never a
	// bound: both readers grow past it rather than truncate.
	componentCountHint = 19
)

// TestModelDeclaresEveryComponentKeyword holds the YANG this plugin publishes
// against the function that decides what its encoder accepts.
//
// VALIDATES: the component set `ze-flowspec-cmd.yang` augments onto
// `announce flowspec` is exactly the set `isComponentKeyword` recognises.
// PREVENTS: the second copy drifting from its producer. The keyword set has one
// producer, `isComponentKeyword` over the `kw*` constants, and declaring it in
// YANG makes a copy. A copy with no check is a future disagreement with nothing
// to arbitrate it (ai/rules/principles.md), and it misleads in both directions:
// a keyword in the encoder alone is a component an operator can type and cannot
// discover, and one in the model alone is a component the usage line offers and
// the encoder refuses.
func TestModelDeclaresEveryComponentKeyword(t *testing.T) {
	declared := augmentedContainerNames(t, flowspecyang.ZeFlowspecCmdYANG)
	if len(declared) == 0 {
		t.Fatal("no container names parsed out of the module, so this test could not discriminate")
	}

	for _, name := range declared {
		if !isComponentKeyword(name) {
			t.Errorf("the model declares %q and the encoder does not accept it", name)
		}
	}

	for _, name := range encoderComponentKeywords(t) {
		// The const block and the switch beside it disagreeing means the set
		// read below is not the set the encoder accepts, so every comparison
		// after this one would report noise. Stop rather than add to it.
		if !isComponentKeyword(name) {
			t.Fatalf("the const block declares %q and isComponentKeyword refuses it", name)
		}
		if !declaresComponent(declared, name) {
			t.Errorf("the encoder accepts %q and the model does not declare it", name)
		}
	}
}

// encoderComponentKeywords answers the keywords the encoder's own source
// declares, read out of the const block encoderKeywordMarker opens.
//
// It READS the set rather than restating it, because a list written here would
// be a THIRD copy and it would fail open in the one direction that matters. A
// keyword added to isComponentKeyword and to neither the list nor the model
// leaves both loops above with nothing to compare, so the drift this test
// exists to catch would pass unseen. Reading the block instead makes the new
// constant arrive on the encoder side by itself.
//
// The loop is bounded by the line count of one file this test compiles beside.
func encoderComponentKeywords(t *testing.T) []string {
	t.Helper()
	source, err := os.ReadFile(encoderSourceFile)
	if err != nil {
		t.Fatalf("read %s: %v", encoderSourceFile, err)
	}

	_, after, found := strings.Cut(string(source), encoderKeywordMarker)
	if !found {
		t.Fatalf("%s no longer carries %q, so the encoder's set could not be read", encoderSourceFile, encoderKeywordMarker)
	}
	block, _, closed := strings.Cut(after, "\n)")
	if !closed {
		t.Fatalf("the const block after %q in %s is not closed", encoderKeywordMarker, encoderSourceFile)
	}

	keywords := make([]string, 0, componentCountHint)
	for line := range strings.SplitSeq(block, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "kw") {
			continue
		}
		_, value, opened := strings.Cut(line, `= "`)
		if !opened {
			continue
		}
		keyword, _, terminated := strings.Cut(value, `"`)
		if !terminated {
			continue
		}
		keywords = append(keywords, keyword)
	}
	if len(keywords) == 0 {
		t.Fatalf("no keyword parsed out of the const block in %s, so this test could not discriminate", encoderSourceFile)
	}
	return keywords
}

func declaresComponent(sorted []string, want string) bool {
	index := sort.SearchStrings(sorted, want)
	return index < len(sorted) && sorted[index] == want
}

// augmentedContainerNames answers the container names the module's augment
// block declares, sorted.
//
// It reads the module text rather than the built command tree because the tree
// needs the whole loader and this test's subject is what THIS module states. A
// component container sits at the augment block's own indent; anything deeper
// belongs to one of them.
func augmentedContainerNames(t *testing.T, module string) []string {
	t.Helper()
	const componentIndent = "        container "
	names := make([]string, 0, componentCountHint)
	inAugment := false
	for line := range strings.SplitSeq(module, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "augment ") {
			inAugment = true
			continue
		}
		if !inAugment || !strings.HasPrefix(line, componentIndent) {
			continue
		}
		name := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "container ")), " {")
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
