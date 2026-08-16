//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.True(t, hasSection([]string{name}, name),
			"positional %q must be recognized", name)
		assert.True(t, hasSection([]string{"--" + name}, name),
			"flag --%s must be recognized (legacy alias)", name)
	}

	// Section selection must not be confused by neighboring tokens.
	assert.True(t, hasSection([]string{"ai", "api", "--json"}, "api"))
	assert.False(t, hasSection([]string{"ai", "cli"}, "api"),
		"requesting cli must not select api")
	assert.False(t, hasSection([]string{"ai"}, "api"),
		"bare summary must not select any section")
	assert.False(t, hasSection(nil, "api"))

	// The --ai mode flag (legacy bare alias) selects no real section, so
	// "ze help --ai" stays a summary rather than dumping a section.
	for _, name := range []string{"cli", "api", "mcp", "dispatch", "all"} {
		assert.False(t, hasSection([]string{"--ai"}, name),
			"bare --ai must not select section %q", name)
	}
}

// TestAISectionGuess removed with the aiSectionGuess feature it
// covered. The self-correcting wrong-guess hint was reverted as redundant with
// the usage signpost and wrong-altitude (a per-command special case instead of
// the generic suggest.Command engine). No source defect; the feature is gone.

// TestAIHelpRequested verifies the deprecated --ai flag is still detected so the
// alias path in dispatchHelp keeps working.
func TestAIHelpRequested(t *testing.T) {
	assert.True(t, aiHelpRequested([]string{"--ai"}))
	assert.True(t, aiHelpRequested([]string{"--ai", "--api"}))
	assert.False(t, aiHelpRequested([]string{"ai"}),
		"the canonical 'ai' subcommand is handled by dispatchHelp, not aiHelpRequested")
	assert.False(t, aiHelpRequested([]string{"command"}))
	assert.False(t, aiHelpRequested(nil))
}
