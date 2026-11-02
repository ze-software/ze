//go:build ze_core

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
