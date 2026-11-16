// Design: docs/architecture/module-tiers.md -- test-only helper declarations.
package testonly

import "testing"

// TestExt4FixtureDeclarationNamesOnlyItsPackage checks the guest fixture's
// declared ownership without admitting neighboring or undeclared packages.
func TestExt4FixtureDeclarationNamesOnlyItsPackage(t *testing.T) {
	const helper = "internal/test/ext4fixture"
	for _, test := range []struct {
		path string
		held bool
	}{
		{path: helper, held: true},
		{path: helper + "/disk_linux.go", held: true},
		{path: "internal/test/ext4fixtureextra/disk_linux.go"},
		{path: "internal/test/unusedfixture/helper.go"},
		{path: "internal/core/gptdisk/grow.go"},
	} {
		t.Run(test.path, func(t *testing.T) {
			owner, held := Owner(test.path)
			if held != test.held {
				t.Errorf("Owner(%q) held = %v, want %v", test.path, held, test.held)
			}
			if test.held {
				if owner != helper {
					t.Errorf("Owner(%q) = %q, want %q", test.path, owner, helper)
				}
			} else if owner != "" {
				t.Errorf("undeclared path %q has owner %q", test.path, owner)
			}
		})
	}
}
