// implicit_selector_test.go pins which leaf a bare token between two key tokens
// fills, for the shapes the command tree actually declares.

package server

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
)

// TestImplicitSelectorPrefersThePatternlessLeaf covers the shape every
// `request interface <name> mac <address>` style command has: an inline
// identifier slot beside a typed trailing value.
//
// VALIDATES: a leaf that states a pattern is a typed value, not the inline
// identifier slot, so declaring one leaves the identifier unambiguous.
// PREVENTS: the shipped rule, which counted both leaves as candidates, answered
// nil, and made matchCommandTokens fail the whole match. Declaring the MAC value
// the description spelled in prose would have turned `request interface zetest0
// mac 02:de:ad:be:ef:01` into an unknown command.
func TestImplicitSelectorPrefersThePatternlessLeaf(t *testing.T) {
	mac := regexp.MustCompile(`^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$`)
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("address", nil, []*regexp.Regexp{mac}, command.ArgOptions{Mandatory: true})),
	}

	def := implicitSelectorDef([]string{"request", "interface", "mac"}, defs, nil)
	require.NotNil(t, def, "the pattern-less leaf is the inline identifier")
	assert.Equal(t, "name", def.Name())
}

// TestImplicitSelectorRefusesTwoPatternlessLeaves keeps the widening narrow.
//
// VALIDATES: ambiguity is still refused, so no command that resolves today
// resolves differently.
// PREVENTS: reading "prefer the pattern-less one" as "take the first one".
func TestImplicitSelectorRefusesTwoPatternlessLeaves(t *testing.T) {
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("name", nil, nil, command.ArgOptions{Mandatory: true})),
		commandtest.Must(command.NewStringArg("peer", nil, nil, command.ArgOptions{Mandatory: true})),
	}

	assert.Nil(t, implicitSelectorDef([]string{"create", "interface", "veth"}, defs, nil))
}

// TestImplicitSelectorTakesALonePatternedLeaf pins the case where the only
// candidate states a pattern.
//
// VALIDATES: a sole patterned leaf still fills the inline slot, which is what
// the shipped rule did.
// PREVENTS: turning the preference into a ban and losing a command that
// resolves today.
func TestImplicitSelectorTakesALonePatternedLeaf(t *testing.T) {
	defs := []command.ArgDef{
		commandtest.Must(command.NewStringArg("selector", nil, []*regexp.Regexp{regexp.MustCompile(`^\S+$`)}, command.ArgOptions{Mandatory: true})),
	}

	def := implicitSelectorDef([]string{"show", "bgp", "detail"}, defs, nil)
	require.NotNil(t, def)
	assert.Equal(t, "selector", def.Name())
}
