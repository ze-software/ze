package rfc

// VALIDATES: the exported closure and requirement index another le package
// reads (ProfileClosures, RequirementStems).
// PREVENTS: a roadmap closure that disagrees with `./le rfc tree`, and an id
// index that silently drops a summary that did not parse.

import (
	"slices"
	"testing"
)

// TestProfileClosuresAndRequirementStems: the closures equal the tree's
// members over the priority fixture, and the index maps ids to stems and
// refuses a duplicate id.
func TestProfileClosuresAndRequirementStems(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{})
	closures := ProfileClosures(corpus.graph, corpus.profiles)
	if want := []string{"rfc1", "rfc2", "rfc3", "rfc4", "rfc5", "rfc6"}; !slices.Equal(closures["edge"], want) {
		t.Errorf("edge closure %v, want %v", closures["edge"], want)
	}
	if want := []string{"rfc1", "rfc2", "rfc4"}; !slices.Equal(closures["core"], want) {
		t.Errorf("core closure %v, want %v (rfc4 updates rfc1)", closures["core"], want)
	}
	if _, held := closures["base"]; held {
		t.Error("an aggregate has a closure: only use-case profiles do")
	}

	line := "- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2)\n"
	stems, err := RequirementStems(map[string]string{"rfc9999": line})
	if err != nil {
		t.Fatalf("RequirementStems: %v", err)
	}
	if stems["RFC9999-2-1"] != "rfc9999" {
		t.Errorf("index %v, want RFC9999-2-1 -> rfc9999", stems)
	}
	if _, err := RequirementStems(map[string]string{"rfc9999": line + line}); err == nil {
		t.Error("a summary with a duplicate id was indexed")
	}
}
