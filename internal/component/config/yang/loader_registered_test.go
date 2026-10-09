package yang_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // registers every compiled-in YANG module
)

// TestEveryRegisteredModuleResolves: the embedded modules plus every module
// the compiled-in plugins register load and resolve with no error, so no
// shipped module carries an undeclared or misspelled extension. Method: the
// external test package imports the generated composition root, which the
// yang package itself cannot import.
//
// VALIDATES: the shipped schema passes Resolve, extension check included.
// PREVENTS: a compiled-in extension typo making DefaultLoader fail at
// startup, where several callers discard its error and use a nil loader.
func TestEveryRegisteredModuleResolves(t *testing.T) {
	if len(yang.Modules()) == 0 {
		t.Fatal("no registered YANG module: the composition root registered nothing")
	}
	loader := yang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("LoadRegistered: %v", err)
	}
	if _, err := loader.Resolve(); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, err := yang.DefaultLoader(); err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
}
