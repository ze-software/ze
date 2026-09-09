// VALIDATES: spec-per-protocol-fib-import AC-4 (vocabulary) -- the editor offers
// the REGISTERED protocols for `rib { fib-withhold [ ... ] }`, so a protocol that
// registers appears with no edit to any list.
// PREVENTS: the failure the six hand-written `rib { distance { } }` leaves have,
// where four registered protocols have no leaf and nothing says so.
package cli

import (
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"

	// The completer reads the registered YANG modules, and the live binary
	// registers this one through the generated all.go blank import.
	_ "github.com/ze-software/ze/internal/component/sysrib/yang"
)

func TestFibWithholdCompletionOffersRegisteredProtocols(t *testing.T) {
	// A protocol nothing else in this binary registers. It is the evidence: no
	// hand-written list can hold a name invented here, so a completion that
	// offers it read the registry.
	const late = "test-late-protocol"
	redistevents.RegisterProtocol(late)

	texts := completionTexts(NewCompleter().Complete("set fib-withhold ", []string{"rib"}))
	if len(texts) == 0 {
		t.Fatal("fib-withhold offers no completion at all")
	}

	offered := make(map[string]bool, len(texts))
	for _, text := range texts {
		offered[text] = true
	}
	for _, name := range redistevents.ProtocolNames() {
		if !offered[name] {
			t.Errorf("the registered protocol %q is not offered: %v", name, texts)
		}
	}
	if !offered[late] {
		t.Errorf("a protocol registered after the schema was written is not offered: %v", texts)
	}
}
