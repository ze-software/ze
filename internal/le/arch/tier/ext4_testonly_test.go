// Design: docs/architecture/module-tiers.md -- guest fixtures stay outside shipped code.
package archtier

import (
	"strings"
	"testing"
)

// TestExt4FixtureImportsFromShippedCodeAreRefused drives the real tier
// predicate with a shipped importer of the declared guest fixture package.
func TestExt4FixtureImportsFromShippedCodeAreRefused(t *testing.T) {
	const helper = "internal/test/ext4fixture"
	const importer = "internal/core/gptdisk/grow.go"
	result := testOnlyImportGate(fixtureModule, Edges{
		fixtureModule + "/" + helper: {importer},
	})
	if result.Code != 2 {
		t.Fatalf("shipped fixture importer: code %d, want 2; %s", result.Code, result.Diagnosis)
	}
	for _, want := range []string{importer, helper} {
		if !strings.Contains(result.Diagnosis, want) {
			t.Errorf("fixture refusal omits %q: %s", want, result.Diagnosis)
		}
	}
}

// TestExt4FixtureImportsFromTestsAndToolingAreAllowed retains the two actual
// consumer classes: guest tests and their host-side fixture producer.
func TestExt4FixtureImportsFromTestsAndToolingAreAllowed(t *testing.T) {
	result := testOnlyImportGate(fixtureModule, Edges{
		fixtureModule + "/internal/test/ext4fixture": {
			"internal/core/gptdisk/grow_integration_linux_test.go",
			"internal/le/test/qemu/guestbin.go",
		},
	})
	if result.Code != 0 {
		t.Fatalf("test and tooling fixture importers: code %d, want 0; %s", result.Code, result.Diagnosis)
	}
	if result.Diagnosis != "" {
		t.Errorf("allowed importers carry a refusal: %s", result.Diagnosis)
	}
}
