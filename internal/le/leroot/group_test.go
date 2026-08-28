// VALIDATES: help splits the commands into the declared groups, in order, and
// still prints a command whose registration declared none.
// PREVENTS: a grouped help page that drops a command, or that renders the
// groups in whatever order a map iteration produced.
package leroot

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func metaFor(description string) registry.Meta {
	return registry.Meta{Description: description, Mode: "offline", Section: registry.SectionTest}
}

func TestUsageSectionsFollowGroupOrder(t *testing.T) {
	setGroup("probe-suite", GroupSuite)
	setGroup("probe-workflow", GroupWorkflow)
	setGroup("probe-gate", GroupGate)

	roots := []registry.RootCommand{
		{Name: "probe-suite", Meta: metaFor("a suite probe")},
		{Name: "probe-workflow", Meta: metaFor("a workflow probe")},
		{Name: "probe-gate", Meta: metaFor("a gate probe")},
	}

	sections := usageSections(roots)
	if len(sections) != 3 {
		t.Fatalf("sections = %d, want 3", len(sections))
	}
	want := []Group{GroupWorkflow, GroupGate, GroupSuite}
	for index, group := range want {
		if sections[index].Title != GroupTitle(group) {
			t.Errorf("section %d title = %q, want %q", index, sections[index].Title, GroupTitle(group))
		}
		if len(sections[index].Entries) != 1 {
			t.Fatalf("section %d holds %d entries, want 1", index, len(sections[index].Entries))
		}
	}
	if sections[0].Entries[0].Name != "probe-workflow" {
		t.Errorf("first section holds %q, want probe-workflow", sections[0].Entries[0].Name)
	}
}

// TestUsageSectionsPrintAnUngroupedCommand covers the command that reached the
// registry by some route other than Register. Filing it badly is recoverable;
// leaving it off the help page is not.
func TestUsageSectionsPrintAnUngroupedCommand(t *testing.T) {
	roots := []registry.RootCommand{{Name: "probe-no-group", Meta: metaFor("a probe with no group")}}

	sections := usageSections(roots)
	if len(sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(sections))
	}
	if sections[0].Title != "Ungrouped" {
		t.Errorf("title = %q, want Ungrouped", sections[0].Title)
	}
	if len(sections[0].Entries) != 1 || sections[0].Entries[0].Name != "probe-no-group" {
		t.Errorf("entries = %#v, want the one ungrouped command", sections[0].Entries)
	}
}

func TestRegisterRefusesAnUnknownGroup(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Error("Register accepted a group help has no title for")
		}
	}()
	Register("probe-bad-group", Group("invented"), func([]string) (any, int) { return nil, 0 }, metaFor("a probe"))
}
