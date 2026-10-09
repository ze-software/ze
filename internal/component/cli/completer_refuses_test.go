package cli

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
)

// newTestCompleter builds the completer over every module this test binary
// registers, and FAILS the test when that set does not resolve.
func newTestCompleter(t testing.TB) *Completer {
	t.Helper()
	completer, err := NewCompleter()
	if err != nil {
		t.Fatalf("NewCompleter: %v", err)
	}
	return completer
}

// TestNewCompleterRefusesFailedResolution: a registered module set that does
// not resolve is an error from NewCompleter, never a completer that answers.
//
// VALIDATES: NewCompleter returns the resolution error to its caller.
// PREVENTS: the silent degrade, where a failed Resolve built a Completer with
// no schema that answered only the bare command words, so the operator saw a
// thinner completion and no line said why (ai/rules/principles.md).
func TestNewCompleterRefusesFailedResolution(t *testing.T) {
	restore := yang.RegisterModuleForTest("ze-fixture-completer-refused.yang", `module ze-fixture-completer-refused {
  namespace "urn:ze:fixture:completer-refused";
  prefix zfcr;
  leaf name { type string { pattern '\i+'; } }
}`)
	t.Cleanup(restore)

	completer, err := NewCompleter()
	if !errors.Is(err, yang.ErrUncompilablePattern) {
		t.Fatalf("a module set with a refused pattern answered %v, want ErrUncompilablePattern", err)
	}
	if completer != nil {
		t.Fatal("a failed resolution still built a completer")
	}
}
