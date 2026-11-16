package fixture

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestExternalYANGLeavesAreSortedPathWords proves the line a .ci asserts on is
// stable: the leaves of a Stage 2 document come out as path=value words in
// sorted order, whatever order the JSON object held them in.
func TestExternalYANGLeavesAreSortedPathWords(t *testing.T) {
	leaves, err := externalYANGLeaves(`{"acme-monitor":{"target":"probe-a","interval":"5"}}`)
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	want := []string{"acme-monitor/interval=5", "acme-monitor/target=probe-a"}
	if !slices.Equal(leaves, want) {
		t.Fatalf("leaves = %v, want %v", leaves, want)
	}
}

// TestExternalYANGLeavesOfASubPathRootKeepTheFullPath proves a root below the
// top level is printed under its whole path, which is how the daemon wraps it.
func TestExternalYANGLeavesOfASubPathRootKeepTheFullPath(t *testing.T) {
	leaves, err := externalYANGLeaves(`{"service":{"chrony":{"server":"time.example.net"}}}`)
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	want := []string{"service/chrony/server=time.example.net"}
	if !slices.Equal(leaves, want) {
		t.Fatalf("leaves = %v, want %v", leaves, want)
	}
}

// TestExternalYANGLeavesOfARemovedRootAreEmpty proves the two shapes of "no
// config" print no leaf: an absent document and the {} a removed root sends.
func TestExternalYANGLeavesOfARemovedRootAreEmpty(t *testing.T) {
	for _, document := range []string{"", "{}"} {
		leaves, err := externalYANGLeaves(document)
		if err != nil {
			t.Fatalf("leaves of %q: %v", document, err)
		}
		if len(leaves) != 0 {
			t.Fatalf("leaves of %q = %v, want none", document, leaves)
		}
	}
}

// TestExternalYANGLeavesRefuseADocumentThatIsNotJSON proves a document the
// fixture cannot read is an error, never an empty answer a test would read as
// "no config".
func TestExternalYANGLeavesRefuseADocumentThatIsNotJSON(t *testing.T) {
	if _, err := externalYANGLeaves(`{"acme-monitor":`); err == nil {
		t.Fatal("a truncated document was accepted")
	}
}

// TestExternalYANGVerifyRefusesOnlyTheRefusedTarget proves the verify handler
// refuses the one target a test uses to abort a commit, with the plugin's own
// text, and accepts every other candidate.
func TestExternalYANGVerifyRefusesOnlyTheRefusedTarget(t *testing.T) {
	accepted := []sdk.ConfigSection{{Root: "acme-monitor", Data: `{"acme-monitor":{"target":"probe-a"}}`}}
	if err := externalYANGVerify(accepted); err != nil {
		t.Fatalf("an ordinary target was refused: %v", err)
	}
	refused := []sdk.ConfigSection{{Root: "acme-monitor", Data: `{"acme-monitor":{"target":"refuse-me"}}`}}
	err := externalYANGVerify(refused)
	if err == nil {
		t.Fatal("the refused target was accepted")
	}
	if err.Error() != "acme-monitor cannot probe target refuse-me" {
		t.Fatalf("refusal = %q", err.Error())
	}
}

// TestExternalYANGRedeclaringVariantChangesItsModuleOnTheSecondStart proves
// the redeclare variant declares the valid module first and a different text
// afterwards, under one module name, and asks to be started again.
func TestExternalYANGRedeclaringVariantChangesItsModuleOnTheSecondStart(t *testing.T) {
	variant, err := externalYANGVariantNamed("redeclare")
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	valid, err := externalYANGVariantNamed("valid")
	if err != nil {
		t.Fatalf("variant: %v", err)
	}
	first := variant.declaration(1)
	second := variant.declaration(2)
	if first.Schema.YANGText != valid.declaration(1).Schema.YANGText {
		t.Fatal("the first start does not declare the valid module")
	}
	if second.Schema.YANGText == first.Schema.YANGText {
		t.Fatal("the second start declares the same module text as the first")
	}
	if second.Schema.Module != first.Schema.Module {
		t.Fatalf("module name changed: %q then %q", first.Schema.Module, second.Schema.Module)
	}
	if first.FailurePolicy != sdk.FailureRestart {
		t.Fatalf("failure policy = %v, want restart", first.FailurePolicy)
	}
}

// TestExternalYANGEveryVariantDeclaresAModuleAndARoot proves no variant
// declares an empty schema or wants nothing, which would make its test pass
// for the wrong reason.
func TestExternalYANGEveryVariantDeclaresAModuleAndARoot(t *testing.T) {
	for name, variant := range externalYANGVariants {
		declaration := variant.declaration(1)
		if declaration.Schema == nil {
			t.Fatalf("variant %s declares no schema", name)
		}
		if !strings.Contains(declaration.Schema.YANGText, "module "+declaration.Schema.Module+" {") {
			t.Fatalf("variant %s: the text does not define module %s", name, declaration.Schema.Module)
		}
		if len(declaration.WantsConfig) == 0 {
			t.Fatalf("variant %s wants no config root", name)
		}
	}
}

// TestExternalYANGUnknownVariantIsRefusedWithTheAcceptedNames proves a
// mistyped run-line argument names the accepted variants.
func TestExternalYANGUnknownVariantIsRefusedWithTheAcceptedNames(t *testing.T) {
	_, err := externalYANGVariantNamed("no-such-variant")
	if err == nil {
		t.Fatal("an unknown variant was accepted")
	}
	for _, name := range []string{"augment", "clash", "invalid", "redeclare", "valid"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("refusal %q does not name %s", err.Error(), name)
		}
	}
}
