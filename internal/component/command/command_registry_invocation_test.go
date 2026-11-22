// Design: docs/architecture/api/commands.md -- shared declaration lookup and child floors.

package command

import "testing"

// TestCommandRegistryLookupUsesDecodedPathSegments proves all metadata readers
// share literal invocation boundaries, case folding and longest-prefix choice.
func TestCommandRegistryLookupUsesDecodedPathSegments(t *testing.T) {
	registry := newCommandRegistry[string]()
	registry.register([]string{"show probe"}, "parent")
	registry.register([]string{"show probe own"}, "child")
	for _, test := range []struct {
		command, value, path string
		found                bool
	}{
		{`show "probe" own content "two words"`, "child", "show probe own", true},
		{`"show" probe own`, "child", "show probe own", true},
		{`SHOW "PROBE" OWN`, "child", "show probe own", true},
		{`show "probe" content "two words"`, "parent", "show probe", true},
		{`show "probe" own | json compact`, "child", "show probe own", true},
		{`show "probe own"`, "", "", false},
		{`"show probe" own`, "", "", false},
		{`show probe "private`, "", "", false},
		{`show probe "private\q"`, "", "", false},
	} {
		t.Run(test.command, func(t *testing.T) {
			value, path, found := registry.lookupPath(test.command)
			if found != test.found {
				t.Errorf("found=%t, want %t", found, test.found)
			}
			if value != test.value {
				t.Errorf("value=%q, want %q", value, test.value)
			}
			if path != test.path {
				t.Errorf("path=%q, want %q", path, test.path)
			}
		})
	}
}

// TestCommandRegistryQuotedChildBoundaryPreservesTokenRules keeps one accepted
// argument distinct from a child branch, including when that argument has spaces.
func TestCommandRegistryQuotedChildBoundaryPreservesTokenRules(t *testing.T) {
	registry := newCommandRegistry[string]()
	registry.register([]string{"show probe"}, "parent")
	registry.registerChildBoundary("show probe", func(argument string) bool {
		switch argument {
		case "all", "all groups":
			return true
		default:
			return false
		}
	})
	registry.register([]string{"show probe own"}, "child")
	for _, test := range []struct {
		command, value, path string
		found                bool
	}{
		{`show "probe"`, "parent", "show probe", true},
		{`show "probe" "ALL"`, "parent", "show probe", true},
		{`show "probe" "ALL GROUPS"`, "parent", "show probe", true},
		{`show "probe" all detail`, "", "show probe all", true},
		{`show "probe" "other thing"`, "", `show probe "other thing"`, true},
		{`show "probe" "all groups" detail`, "", `show probe "all groups"`, true},
		{`show "probe" own`, "child", "show probe own", true},
		{`show "probe" own 192.0.2.1 detail`, "child", "show probe own", true},
		{`"show probe" all`, "", "", false},
	} {
		t.Run(test.command, func(t *testing.T) {
			value, path, found := registry.lookupPath(test.command)
			if found != test.found {
				t.Errorf("found=%t, want %t", found, test.found)
			}
			if value != test.value {
				t.Errorf("value=%q, want %q", value, test.value)
			}
			if path != test.path {
				t.Errorf("path=%q, want %q", path, test.path)
			}
		})
	}
}

// TestCommandRegistryLookupSkipsArgDefSourceWhenKeywordsAbsent proves that
// lookupPath does not invoke the YANG argument definition source for registered
// paths whose keywords do not even appear in the command words (such as `le`
// shell callbacks checking unrelated `le <area>` declarations).
func TestCommandRegistryLookupSkipsArgDefSourceWhenKeywordsAbsent(t *testing.T) {
	calls := 0
	prev := ReplaceArgDefSourceForTest(func(path string) ([]ArgDef, error) {
		calls++
		return nil, nil
	})
	t.Cleanup(func() { ReplaceArgDefSourceForTest(prev) })

	registry := newCommandRegistry[string]()
	registry.register([]string{"le spec"}, "spec")
	registry.register([]string{"le verify"}, "verify")
	registry.register([]string{"show bgp peer rib"}, "peer-rib")

	if _, _, found := registry.lookupPath("le completion words"); found {
		t.Fatal("unexpected match for le completion words")
	}
	if calls != 0 {
		t.Fatalf("argDefSource called %d times for unrelated keywords, want 0", calls)
	}
}
