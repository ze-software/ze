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
