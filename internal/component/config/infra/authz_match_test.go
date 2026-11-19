package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
)

// storeWithEditMatches builds a store holding one profile whose edit section
// has one prefix entry per match, numbered from 10 in steps of 10.
func storeWithEditMatches(t *testing.T, matches ...string) *authz.Store {
	t.Helper()
	section := authz.Section{Default: authz.Deny}
	for i, match := range matches {
		section.Entries = append(section.Entries, authz.Entry{
			Number: uint32(10 * (i + 1)),
			Action: authz.Allow,
			Match:  match,
		})
	}
	store := authz.NewStore()
	store.AddProfile(authz.Profile{Name: "operator", Edit: section})
	return store
}

// TestUnknownMatchEntriesKnowsConfigPermissions proves an entry reaching a
// config permission string is not reported as unknown. Method: the entries of
// the documented operator sample and of a named-configuration session
// (`config`, `config set`, `config commit`, `config commit client-edge.conf`)
// are checked against the real YANG command set, where none of them is a
// command, and none may be reported.
func TestUnknownMatchEntriesKnowsConfigPermissions(t *testing.T) {
	store := storeWithEditMatches(t,
		"config", "config set", "config commit", "config commit client-edge.conf")

	unknown, err := unknownMatchEntries(store)

	require.NoError(t, err)
	assert.Empty(t, unknown)
}

// TestUnknownMatchEntriesReportsUnknownEntry proves an entry that reaches
// neither a command nor a config permission string is still reported. Method:
// a misspelled verb and an unknown config action are checked, and each must
// come back with its profile, section and match as written.
func TestUnknownMatchEntriesReportsUnknownEntry(t *testing.T) {
	store := storeWithEditMatches(t, "confg set", "config frobnicate")

	unknown, err := unknownMatchEntries(store)

	require.NoError(t, err)
	assert.ElementsMatch(t, []unknownMatchEntry{
		{profile: "operator", section: "edit", match: "confg set"},
		{profile: "operator", section: "edit", match: "config frobnicate"},
	}, unknown)
}

// TestUnknownMatchEntriesNeedsWordBoundary proves a config permission string
// counts only at a word boundary, the rule the authorizer matches with.
// Method: `config commits` starts with `config commit` but no authorizer
// question ever matches it, so it must be reported.
func TestUnknownMatchEntriesNeedsWordBoundary(t *testing.T) {
	store := storeWithEditMatches(t, "config commits")

	unknown, err := unknownMatchEntries(store)

	require.NoError(t, err)
	assert.Equal(t, []unknownMatchEntry{
		{profile: "operator", section: "edit", match: "config commits"},
	}, unknown)
}
