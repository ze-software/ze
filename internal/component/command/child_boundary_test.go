// Design: docs/architecture/api/commands.md -- parent answer inheritance boundaries

package command

import "testing"

// TestChildBoundaryResolvesAtTheFirstTypedKeyword checks a scoped boundary
// against ordinary inheritance, selector spellings and explicit child values.
func TestChildBoundaryResolvesAtTheFirstTypedKeyword(t *testing.T) {
	registry := newCommandRegistry[string]()
	registry.register([]string{"show probe"}, "parent")
	registry.register([]string{"show unrelated"}, "other-parent")
	registry.register([]string{"show probed"}, "similar-parent")
	registry.registerChildBoundary("show probe", nil)
	registry.register([]string{"show probe own"}, "child")
	registry.register([]string{"show probe branch detail"}, "leaf")

	cases := []struct {
		command string
		value   string
		path    string
	}{
		{"show probe", "parent", "show probe"},
		{"show probe fresh", "", "show probe fresh"},
		{"show probe fresh detail", "", "show probe fresh"},
		{"show probe fresh 192.0.2.1 detail", "", "show probe fresh"},
		{"show probe own", "child", "show probe own"},
		{"show probe own 192.0.2.1 detail", "child", "show probe own"},
		{"show probe branch detail", "leaf", "show probe branch detail"},
		{"show unrelated child", "other-parent", "show unrelated"},
		{"show probed child", "similar-parent", "show probed"},
	}
	for _, test := range cases {
		t.Run(test.command, func(t *testing.T) {
			value, path, found := registry.lookupPath(test.command)
			if !found {
				t.Fatal("declaration missing")
			}
			if value != test.value {
				t.Errorf("value = %q, want %q", value, test.value)
			}
			if path != test.path {
				t.Errorf("path = %q, want %q", path, test.path)
			}
		})
	}
}

// TestChildBoundarySurvivesValueRemoval proves a runtime child's removal
// exposes the boundary while a complete registry reset removes both.
func TestChildBoundarySurvivesValueRemoval(t *testing.T) {
	registry := newCommandRegistry[string]()
	registry.register([]string{"show probe"}, "parent")
	registry.registerChildBoundary("show probe", nil)
	registry.register([]string{"show probe child"}, "runtime")
	registry.remove("show probe child")
	value, path, found := registry.lookupPath("show probe child")
	if !found {
		t.Fatal("child floor missing after value removal")
	}
	if value != "" {
		t.Errorf("removed child inherited %q", value)
	}
	if path != "show probe child" {
		t.Errorf("floor = %q, want child path", path)
	}
	registry.reset()
	if _, _, found := registry.lookupPath("show probe child"); found {
		t.Fatal("reset retained child floor")
	}
}

// TestChildBoundaryPreservesOneOwnerArgument checks that the owner's accepted
// token inherits only when it is the entire tail, while explicit metadata wins.
func TestChildBoundaryPreservesOneOwnerArgument(t *testing.T) {
	registry := newCommandRegistry[string]()
	registry.register([]string{"show probe"}, "parent")
	registry.registerChildBoundary("show probe", func(argument string) bool {
		return argument == "all"
	})

	cases := []struct {
		command string
		value   string
		path    string
	}{
		{"show probe", "parent", "show probe"},
		{"show probe all", "parent", "show probe"},
		{"show probe ALL", "parent", "show probe"},
		{"show probe all detail", "", "show probe all"},
		{"show probe all 192.0.2.1 detail", "", "show probe all"},
		{"show probe other", "", "show probe other"},
	}
	for _, test := range cases {
		t.Run(test.command, func(t *testing.T) {
			value, path, found := registry.lookupPath(test.command)
			if !found {
				t.Fatal("declaration missing")
			}
			if value != test.value {
				t.Errorf("value = %q, want %q", value, test.value)
			}
			if path != test.path {
				t.Errorf("path = %q, want %q", path, test.path)
			}
		})
	}

	registry.register([]string{"show probe all"}, "explicit")
	value, path, found := registry.lookupPath("show probe all")
	if !found {
		t.Fatal("explicit declaration missing")
	}
	if value != "explicit" {
		t.Errorf("value = %q, want explicit", value)
	}
	if path != "show probe all" {
		t.Errorf("path = %q, want explicit path", path)
	}
}
