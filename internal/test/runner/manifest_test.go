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

// TestHelperBuildTagsCarryFeatureGates pins the ze-test helper to the SAME
// feature set as the daemon.
//
// PREVENTS: the helper being built with a bare `ze_test`, which compiles out
// every gated plugin's registering init(). `ze-test plugin-external as112` then
// exits 1 with "unknown registered plugin" (internal/test/cli/cmd_plugin_external.go
// registry.Lookup), the plugin's IsInternal() refusal is never emitted, and
// as112-external-refuses / flowexport-external-refuses wait out their
// await=stderr fence against a process that already died.
func TestHelperBuildTagsCarryFeatureGates(t *testing.T) {
	built := strings.Split(TestHelperBuildTags(), ",")

	// ze_test selects the helper's own CLI surface (the peer / plugin-external
	// subcommands); without it there is no helper at all.
	assert.Contains(t, built, "ze_test", "TestHelperBuildTags missing ze_test")

	// Every gate the daemon gets, the helper gets: plugin-external hands the
	// connection to the registry entry the DAEMON expects to be there.
	for _, want := range featureGateTags() {
		assert.Contains(t, built, want, "TestHelperBuildTags missing gate %q", want)
	}
}
