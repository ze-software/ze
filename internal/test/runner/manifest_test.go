package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFeatureGateTagsFromManifest verifies the functional-test build tags are
// read from feature-gates.txt (the single source of truth) rather than a
// hand-maintained list, so declaring a gate there is enough for the test ze
// binary to exercise it. See ai/rules/feature-gate-registration.md.
func TestFeatureGateTagsFromManifest(t *testing.T) {
	tags := featureGateTags()
	assert.NotEmpty(t, tags, "feature-gates.txt should yield at least one gate tag")

	// The currently-declared gates must all appear (Contains, not equality, so
	// adding a future gate does not break this test).
	for _, want := range []string{"ze_lg", "ze_ssh", "ze_web"} {
		assert.Contains(t, tags, want, "gate %q missing from manifest-derived tags", want)
	}

	seen := make(map[string]bool)
	for _, tag := range tags {
		assert.False(t, seen[tag], "duplicate tag %q from manifest", tag)
		seen[tag] = true
	}

	// TestBuildTags must fold the manifest tags into the functional-test build.
	built := strings.Split(TestBuildTags(), ",")
	for _, want := range []string{"ze_core", "ze_lg", "ze_ssh", "ze_web"} {
		assert.Contains(t, built, want, "TestBuildTags missing %q", want)
	}
}
