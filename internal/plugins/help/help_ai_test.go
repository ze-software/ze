package help_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/plugins/help"
)

// TestHasSection verifies the AI-help section selector accepts both the
// canonical positional form ("api", from "ze help ai api") and the legacy flag
// form ("--api", from "ze help --ai --api"), and that distinct section names do
// not collide.
//
// VALIDATES: ze help ai <section> grammar and its --ai flag alias select the
// same sections.
// PREVENTS: a future refactor silently dropping either the positional form
// (breaking the documented grammar) or the flag form (breaking shipped skills
// and agent prompts that learned "ze help --ai --json").
func TestHasSection(t *testing.T) {
	for _, name := range []string{"cli", "api", "mcp", "dispatch", "all"} {
		assert.True(t, help.HasSection([]string{name}, name),
			"positional %q must be recognized", name)
		assert.True(t, help.HasSection([]string{"--" + name}, name),
			"flag --%s must be recognized (legacy alias)", name)
	}

	// Section selection must not be confused by neighboring tokens.
	assert.True(t, help.HasSection([]string{"ai", "api", "--json"}, "api"))
	assert.False(t, help.HasSection([]string{"ai", "cli"}, "api"),
		"requesting cli must not select api")
	assert.False(t, help.HasSection([]string{"ai"}, "api"),
		"bare summary must not select any section")
	assert.False(t, help.HasSection(nil, "api"))

	// The --ai mode flag (legacy bare alias) selects no real section, so
	// "ze help --ai" stays a summary rather than dumping a section.
	for _, name := range []string{"cli", "api", "mcp", "dispatch", "all"} {
		assert.False(t, help.HasSection([]string{"--ai"}, name),
			"bare --ai must not select section %q", name)
	}
}
