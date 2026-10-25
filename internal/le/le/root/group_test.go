// VALIDATES: help splits the commands into the declared groups, in order, and
// still prints a command whose registration declared none.
// PREVENTS: a grouped help page that drops a command, or that renders the
// groups in whatever order a map iteration produced.
package leroot

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

func metaFor(description string) registry.Meta {
	return registry.Meta{ShortHelp: description, Mode: "offline", Section: registry.SectionTest}
}

// TestRootPageIsLaidOutByTree checks AC-4: the root page holds one block for
// the single-word commands and one per namespace, each line carries the purpose
// tag in brackets and the first sentence of the summary, no group heading line
// is left, and every area appears exactly once, a single word that is also a
// namespace token inside its namespace. Method: build the sections over a
// fixed command set and read each one.
func TestRootPageIsLaidOutByTree(t *testing.T) {
	setGroup("probe-suite", GroupSuite)
	setGroup("probe-ns", GroupWorkflow)
	setGroup("probe-ns alpha", GroupGate)
	setGroup("probe-ns beta", GroupReport)

	roots := []registry.RootCommand{
		{Name: "probe-ns", Meta: metaFor("the namespace's own command. More words")},
		{Name: "probe-ns alpha", Meta: metaFor("a gate probe")},
		{Name: "probe-ns beta", Meta: metaFor("a report probe")},
		{Name: "probe-suite", Meta: metaFor("a suite probe")},
	}

	sections := manifestFrom("le", roots).sections()
	if len(sections) != 2 {
		t.Fatalf("sections = %d, want 2 (single words, then probe-ns): %#v", len(sections), sections)
	}
	if sections[0].Title != "Commands" || len(sections[0].Entries) != 1 || sections[0].Entries[0].Name != "probe-suite" {
		t.Errorf("the single-word block is %#v", sections[0])
	}
	if sections[0].Entries[0].Desc != "[suite] a suite probe" {
		t.Errorf("a line reads %q, want the tag then the summary", sections[0].Entries[0].Desc)
	}
	if sections[1].Title != "probe-ns" || len(sections[1].Entries) != 3 {
		t.Fatalf("the namespace block is %#v", sections[1])
	}
	if got := sections[1].Entries[0]; got.Name != "probe-ns" || got.Desc != "[workflow] the namespace's own command." {
		t.Errorf("the namespace's own command reads %#v, want its first sentence in its namespace", got)
	}
	seen := map[string]int{}
	for _, section := range sections {
		for _, entry := range section.Entries {
			seen[entry.Name]++
		}
	}
	for _, root := range roots {
		if seen[root.Name] != 1 {
			t.Errorf("%s appears %d times, want once", root.Name, seen[root.Name])
		}
	}

	var page strings.Builder
	built := manifestFrom("le", roots).page()
	built.WriteTo(&page, false)
	for _, heading := range []string{"Workflow (", "Gates (", "Suites (", "Reports ("} {
		if strings.Contains(page.String(), heading) {
			t.Errorf("the page still prints the group heading %q:\n%s", heading, page.String())
		}
	}
}

// TestUsageSectionsPrintAnUngroupedCommand covers the command that reached the
// registry by some route other than Register. Tagging it ungrouped is
// recoverable; leaving it off the help page is not.
func TestUsageSectionsPrintAnUngroupedCommand(t *testing.T) {
	roots := []registry.RootCommand{{Name: "probe-no-group", Meta: metaFor("a probe with no group")}}

	sections := manifestFrom("le", roots).sections()
	if len(sections) != 1 {
		t.Fatalf("sections = %d, want 1", len(sections))
	}
	if len(sections[0].Entries) != 1 || sections[0].Entries[0].Name != "probe-no-group" {
		t.Errorf("entries = %#v, want the one ungrouped command", sections[0].Entries)
	}
	if desc := sections[0].Entries[0].Desc; desc != "[ungrouped] a probe with no group" {
		t.Errorf("the line reads %q, want the ungrouped tag", desc)
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
