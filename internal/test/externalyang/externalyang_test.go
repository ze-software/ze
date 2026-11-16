package externalyang

import (
	"strings"
	"testing"
)

// TestAcceptedHoldsEveryModuleTheDaemonAcceptsAndNoOther proves the list the
// config sweep publishes is the modules the daemon is expected to accept.
// Method: read Accepted, then compare it with each module of the package.
//
// VALIDATES: Accepted answers the augmenting module and the valid module, in
// name order, each with the text that defines the module it names, and
// neither refused module.
// PREVENTS: the sweep joining a module written to be refused, which would
// fail every sweep at the join, or losing an accepted one, which would turn
// the configs of its functional test into findings.
func TestAcceptedHoldsEveryModuleTheDaemonAcceptsAndNoOther(t *testing.T) {
	accepted := Accepted()
	want := []Module{Augment(), Valid()}
	if len(accepted) != len(want) {
		t.Fatalf("Accepted holds %d modules, want %d: %v", len(accepted), len(want), accepted)
	}
	for i, module := range accepted {
		if module != want[i] {
			t.Fatalf("Accepted[%d] is module %s, want %s", i, module.Name, want[i].Name)
		}
		if module.Refused {
			t.Fatalf("Accepted holds %s, which the daemon is expected to refuse", module.Name)
		}
		if !strings.Contains(module.Text, "module "+module.Name+" {") {
			t.Fatalf("the text of %s does not define that module", module.Name)
		}
	}
	for _, refused := range []Module{Invalid(), Clash()} {
		if !refused.Refused {
			t.Fatalf("module %s is not marked refused", refused.Name)
		}
	}
	if RedeclaredText == Valid().Text {
		t.Fatal("the redeclared text is the valid module's text: a second start would declare nothing new")
	}
}
