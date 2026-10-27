// Design: docs/contributing/gh-pages.md -- native commands refuse unsafe or incomplete requests.
package siteterminaldemo

import (
	"path/filepath"
	"testing"
)

// TestReadmeCarrierRequiresAnOutputDirectory prevents an incomplete command from rendering or updating README.
func TestReadmeCarrierRequiresAnOutputDirectory(t *testing.T) {
	for _, args := range [][]string{{"readme-carrier"}, {"readme-carrier", "output", ""}} {
		payload, code := Answer(args)
		if code != 2 {
			t.Fatalf("%v returned %d, expected usage refusal", args, code)
		}
		if payload != nil {
			t.Fatalf("incomplete command produced a carrier result: %#v", payload)
		}
	}
}

// TestPTYActionPreservesRecorderErrors checks real option parsing and file errors through the public action.
func TestPTYActionPreservesRecorderErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.tape")
	if _, code := Answer([]string{"pty", "--tape", missing}); code != 1 {
		t.Fatalf("recorder file error returned %d, expected 1", code)
	}
	if _, code := Answer([]string{"pty", "--not-a-recorder-option"}); code != 2 {
		t.Fatalf("recorder option error returned %d, expected 2", code)
	}
}
