package rfc

// VALIDATES: the exported members and requirement index another le package
// reads (ProfileMembers, RequirementStems).
// PREVENTS: roadmap members that disagree with `./le rfc tree`, and an id
// index that silently drops a summary that did not parse.

import (
	"slices"
	"testing"
)

// TestProfileMembersAndRequirementStems: the members of each tier equal the
// rows `tree profile` answers in that tier over the priority fixture, an
// aggregate has none, and the index maps ids to stems and refuses a duplicate
// id.
func TestProfileMembersAndRequirementStems(t *testing.T) {
	corpus := priorityFixture(t, map[string]bool{})
	members := ProfileMembers(corpus.profiles, corpus.metas)
	for _, name := range []string{"core", "edge", "lab"} {
		report, err := corpus.tree(name)
		if err != nil {
			t.Fatalf("tree %s: %v", name, err)
		}
		required, optional := rowStems(report.Rows), rowStems(report.Optional.Rows)
		slices.Sort(required)
		slices.Sort(optional)
		if !slices.Equal(members[name].Required, required) || !slices.Equal(members[name].Optional, optional) {
			t.Errorf("%s members %+v, tree rows %v optional %v", name, members[name], required, optional)
		}
	}
	if want := []string{"rfc1", "rfc2", "rfc3", "rfc4", "rfc5"}; !slices.Equal(members["edge"].Required, want) {
		t.Errorf("edge required %v, want %v", members["edge"].Required, want)
	}
	if _, held := members["base"]; held {
		t.Error("an aggregate has use-case members: only use-case profiles do")
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
