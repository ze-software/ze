package yang

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

// TestEveryShippedYANGPatternCompiles: every `pattern` statement in every
// YANG module under internal/ compiles through compilePattern, the one XSD
// translation both the config validator and the command argument definitions
// use. Method: parse each module's statements with goyang and compile each
// pattern argument. Resolve now refuses a module holding an uncompilable
// pattern, so a failure here is a module that no longer loads.
//
// VALIDATES: every shipped YANG pattern translates and compiles.
// PREVENTS: a module that the pattern check at Resolve would refuse.
func TestEveryShippedYANGPatternCompiles(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "internal")
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".yang") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if len(files) < 100 {
		t.Fatalf("found %d YANG modules under %s, expected the shipped set", len(files), root)
	}
	patterns := 0
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		statements, err := yang.Parse(string(text), file)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		pending := slices.Clone(statements)
		for len(pending) > 0 {
			statement := pending[len(pending)-1]
			pending = append(pending[:len(pending)-1], statement.SubStatements()...)
			if statement.Keyword != "pattern" {
				continue
			}
			patterns++
			if _, err := compilePattern(statement.Argument); err != nil {
				t.Errorf("%s: %v", statement.Location(), err)
			}
		}
	}
	if patterns == 0 {
		t.Fatal("no pattern statement found: the walk read nothing")
	}
	t.Logf("%d modules, %d patterns", len(files), patterns)
}

// TestResolveRefusesUncompilablePattern: a module whose pattern the XSD
// translation cannot compile fails Resolve with ErrUncompilablePattern, naming
// the module, the location and the pattern. Before this check the command
// argument builder dropped such a pattern and the argument accepted any string.
// Method: one probe module beside the embedded set; goyang alone accepts it.
//
// VALIDATES: Resolve refuses an uncompilable pattern.
// PREVENTS: a pattern dropped in silence, leaving its value unconstrained.
func TestResolveRefusesUncompilablePattern(t *testing.T) {
	err := loadExtensionProbe(t, Module{Name: "ze-probe-pattern.yang", Content: `module ze-probe-pattern {
    namespace "urn:ze:probe:pattern";
    prefix probe;
    leaf name {
        type string {
            pattern '[a-z';
        }
    }
}
`})
	if err == nil {
		t.Fatal("Resolve accepted a module whose pattern does not compile")
	}
	if !errors.Is(err, ErrUncompilablePattern) {
		t.Fatalf("error %q does not wrap ErrUncompilablePattern", err)
	}
	assertNames(t, err, "ze-probe-pattern", "[a-z")
}

// TestConfigStringLengthCountsCharacters: the config validator judges a YANG
// length by characters, as the command argument validator does. Method: a
// length 1..4 string type and values of multi-byte characters.
//
// VALIDATES: RFC 7950 Section 9.4.4 "restricts the number of Unicode
// characters in the string" on config leaves.
// PREVENTS: a value of short multi-byte text refused for its byte count.
func TestConfigStringLengthCountsCharacters(t *testing.T) {
	v := NewValidator(nil)
	yangType := &yang.YangType{Kind: yang.Ystring, Length: yang.YangRange{{
		Min: yang.FromInt(1), Max: yang.FromInt(4),
	}}}
	if err := v.validateString("probe", yangType, "éééé"); err != nil {
		t.Errorf("four characters refused by a 1..4 length: %v", err)
	}
	err := v.validateString("probe", yangType, "ééééé")
	if err == nil {
		t.Fatal("five characters accepted by a 1..4 length")
	}
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error %q is not a ValidationError", err)
	}
	if validation.Got != "5" {
		t.Errorf("length reported as %q, want 5 characters", validation.Got)
	}
}
