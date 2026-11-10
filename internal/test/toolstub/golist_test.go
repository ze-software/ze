package toolstub

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

// TestGoListAnswersTheStdlibListing checks the stub's `go list` reply in the
// shape the stdpatch tree builder decodes: a JSON stream that holds at least
// one package under the GOROOT it was asked about. It also checks that every
// listing the stub cannot answer is refused rather than answered empty, since
// an empty answer is what made the builder refuse the stand-in GOROOT.
func TestGoListAnswersTheStdlibListing(t *testing.T) {
	goroot := filepath.Join(t.TempDir(), "goroot")
	answer, err := goListAnswer(goroot, []string{"-e", "-json=ImportPath,Dir,EmbedFiles,Error", "std", "cmd"})
	if err != nil {
		t.Fatalf("the stdlib listing was refused: %v", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(answer))
	var listed int
	for decoder.More() {
		var pkg struct {
			ImportPath string
			Dir        string
			EmbedFiles []string
			Error      *struct{ Err string }
		}
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("the listing is not a go list JSON stream: %v\n%s", err, answer)
		}
		if pkg.ImportPath == "" {
			t.Fatalf("a listed package has no import path: %s", answer)
		}
		if pkg.Dir != filepath.Join(goroot, "src", pkg.ImportPath) {
			t.Fatalf("package %s lists Dir %q, not under the GOROOT asked about", pkg.ImportPath, pkg.Dir)
		}
		if pkg.Error != nil {
			t.Fatalf("package %s carries a load error: %s", pkg.ImportPath, pkg.Error.Err)
		}
		listed++
	}
	if listed == 0 {
		t.Fatal("the stdlib listing holds no package")
	}

	for _, test := range []struct {
		name   string
		goroot string
		args   []string
	}{
		{"no GOROOT", "", []string{"-json", "std", "cmd"}},
		{"no JSON", goroot, []string{"-e", "std", "cmd"}},
		{"std alone", goroot, []string{"-json", "std"}},
		{"another pattern", goroot, []string{"-json", "./..."}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if answer, err := goListAnswer(test.goroot, test.args); err == nil {
				t.Fatalf("a listing the stub cannot answer was answered: %s", answer)
			}
		})
	}
}
