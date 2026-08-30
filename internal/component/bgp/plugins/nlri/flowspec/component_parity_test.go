// Design: docs/architecture/bgp/on-demand-origination.md -- announce states its grammar in the model
package flowspec

import (
	"sort"
	"strings"
	"testing"

	flowspecyang "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec/yang"
)

// encoderComponentKeywords is every keyword isComponentKeyword accepts, listed
// so the test can walk the set in both directions. A keyword added to the
// encoder without being added here fails the guard below rather than passing
// unnoticed.
var encoderComponentKeywords = []string{
	kwDestination, kwDestinationIPv4, kwDestinationIPv6,
	kwSource, kwSourceIPv4, kwSourceIPv6,
	kwProtocol, kwNextHeader, kwPort, kwDestPort, kwSourcePort,
	kwICMPType, kwICMPCode, kwTCPFlags, kwPacketLength, kwDSCP,
	kwFragment, kwFlowLabel, kwRD,
}

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

	for _, name := range encoderComponentKeywords {
		if !isComponentKeyword(name) {
			t.Fatalf("this test names %q, which isComponentKeyword refuses: the list here is wrong, not the model", name)
		}
		if !declaresComponent(declared, name) {
			t.Errorf("the encoder accepts %q and the model does not declare it", name)
		}
	}
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
	names := make([]string, 0, len(encoderComponentKeywords))
	inAugment := false
	for _, line := range strings.Split(module, "\n") {
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
