package yang

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// resolvedFixtureModule declares one config leaf, so a resolved set holds a
// module a reader can name.
const resolvedFixtureModule = `module ze-fixture-resolved-conf {
  namespace "urn:ze:fixture:resolved";
  prefix zfr;
  leaf name { type string; }
}`

// refusedPatternModule holds a pattern compilePattern refuses (the XSD \i
// escape), so Resolve fails on it.
const refusedPatternModule = `module ze-fixture-refused {
  namespace "urn:ze:fixture:refused";
  prefix zfx;
  leaf name { type string { pattern '\i+'; } }
}`

// TestResolveRefusesLoadAfterResolution: once Resolve ran, successful or not,
// every load and every further Resolve on the Loader is refused, and the
// Resolved a successful Resolve returned answers as it did before the attempt.
//
// VALIDATES: AC-7, the transition is one-way although Go leaves the Loader
// usable: the module set the Resolved reads refuses a module the checks never
// saw.
// PREVENTS: a module added after Resolve reaching BuildCommandTree, the
// validator or the PathTo maps unchecked.
func TestResolveRefusesLoadAfterResolution(t *testing.T) {
	file := filepath.Join(t.TempDir(), "late.yang")
	if err := os.WriteFile(file, []byte(resolvedFixtureModule), 0o600); err != nil {
		t.Fatalf("write the late module: %v", err)
	}
	loads := []struct {
		name string
		load func(*Loader) error
	}{
		{"AddModuleFromText", func(l *Loader) error { return l.AddModuleFromText("late.yang", resolvedFixtureModule) }},
		{"AddModuleFromFile", func(l *Loader) error { return l.AddModuleFromFile(file) }},
		{"LoadEmbedded", func(l *Loader) error { return l.LoadEmbedded() }},
		{"LoadRegistered", func(l *Loader) error { return l.LoadRegistered() }},
		{"Resolve", func(l *Loader) error { _, err := l.Resolve(); return err }},
	}

	t.Run("after a successful Resolve", func(t *testing.T) {
		loader := NewLoader()
		if err := loader.AddModuleFromText("resolved.yang", resolvedFixtureModule); err != nil {
			t.Fatalf("load the fixture module: %v", err)
		}
		schema, err := loader.Resolve()
		if err != nil {
			t.Fatalf("resolve the fixture module: %v", err)
		}
		before := slices.Sorted(slices.Values(schema.ModuleNames()))
		for _, tc := range loads {
			if err := tc.load(loader); !errors.Is(err, ErrLoaderResolved) {
				t.Errorf("%s after Resolve answered %v, want ErrLoaderResolved", tc.name, err)
			}
		}
		after := slices.Sorted(slices.Values(schema.ModuleNames()))
		if !slices.Equal(before, after) {
			t.Errorf("the Resolved changed under refused loads: %v, then %v", before, after)
		}
	})

	t.Run("after a failed Resolve", func(t *testing.T) {
		loader := NewLoader()
		if err := loader.AddModuleFromText("refused.yang", refusedPatternModule); err != nil {
			t.Fatalf("load the fixture module: %v", err)
		}
		schema, err := loader.Resolve()
		if !errors.Is(err, ErrUncompilablePattern) {
			t.Fatalf("Resolve over a refused pattern answered %v, want ErrUncompilablePattern", err)
		}
		if schema != nil {
			t.Fatalf("a failed Resolve answered a Resolved: %+v", schema)
		}
		for _, tc := range loads {
			if err := tc.load(loader); !errors.Is(err, ErrLoaderResolved) {
				t.Errorf("%s after a failed Resolve answered %v, want ErrLoaderResolved", tc.name, err)
			}
		}
	})
}
