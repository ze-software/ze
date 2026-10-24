// GOAL: a multi-word leaf keeps the boundary of each word it was written in.
//
// METHOD: parse `run` statements whose words are bare and quoted, then read the
// words back with GetSlice and the mirror with Get.
//
// PREVENTS: a quoted word holding a space being split into two words by every
// consumer, which is what a joined string alone gives them; and a statement
// with no value reading as one never written.

package config

import (
	"slices"
	"testing"
)

// TestMultiLeafKeepsWordBoundaries checks GetSlice returns the written words
// and Get a mirror that quotes the word holding a space.
func TestMultiLeafKeepsWordBoundaries(t *testing.T) {
	schema := NewSchema()
	schema.Define("run", multiLeaf(TypeString))

	tree, err := NewParser(schema).Parse(`run python3 /opt/inject.py "two words" plain;`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"python3", "/opt/inject.py", "two words", "plain"}
	if got := tree.GetSlice("run"); !slices.Equal(got, want) {
		t.Errorf("GetSlice = %q, want %q", got, want)
	}
	mirror, ok := tree.Get("run")
	if !ok {
		t.Fatalf("Get(run) is absent")
	}
	if mirror != JoinLeafListMembers(want) {
		t.Errorf("Get = %q, want %q", mirror, JoinLeafListMembers(want))
	}
}

// TestMultiLeafRepeatedStatementReplaces checks the leaf stays single-valued.
func TestMultiLeafRepeatedStatementReplaces(t *testing.T) {
	schema := NewSchema()
	schema.Define("run", multiLeaf(TypeString))

	tree, err := NewParser(schema).Parse("run first one;\nrun second;")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := tree.GetSlice("run"); !slices.Equal(got, []string{"second"}) {
		t.Errorf("GetSlice = %q, want [second]", got)
	}
}

// TestMultiLeafRefusesNoValue checks `run;` is refused rather than dropped.
func TestMultiLeafRefusesNoValue(t *testing.T) {
	schema := NewSchema()
	schema.Define("run", multiLeaf(TypeString))

	if _, err := NewParser(schema).Parse("run;"); err == nil {
		t.Fatalf("parse accepted a multi-word leaf with no value")
	}
}
