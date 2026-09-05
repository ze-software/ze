// Design: docs/architecture/cli/command-verbs.md -- the send verb and what it costs
//
// The send family is the one command family whose every member puts operator
// bytes on a wire, so two properties are asserted from the DISPATCHER rather
// than from a handler or a predicate: a send with no destination is refused, and
// a profile that denies writes denies every send.
//
// Both properties hold today by DEFAULT, which is why they are pinned here. No
// rule anywhere names `send`: IsReadOnlyPath allowlists the read verbs and
// answers false for a verb it has never heard of, and a read-only profile's Edit
// section denies by default. A later edit that adds `send` to either list would
// remove the property with no line that mentions it. The shape of the BUILT-IN
// read-only profile is pinned beside it, in
// TestBuiltinReadOnlyProfileDeniesEverySendByDefault
// (internal/component/authz/authz_test.go), which this package cannot reach
// because the profile's constructor is unexported.

package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"

	// Trigger every builtin RPC and YANG module init(), matching the composition
	// root the running daemon assembles (mirrors all_import_test.go).
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// sendRawPath is the CLI path the raw form answers at under the new grammar.
const sendRawPath = "send bgp raw"

// TestSendRefusesAMissingSelector asserts a send with no destination never
// reaches a handler.
//
// A send names WHERE the message goes, so an absent destination is not a
// defaultable value. Before this grammar, PeerSelector answered "*" for an
// unbound selector, which made "the operator asked for every peer" and "nothing
// bound" one value (ai/rules/principles.md). RequiresSelector plus a mandatory
// leaf is what removes that case for the send family.
//
// VALIDATES: `send bgp raw hex FF` is refused by Dispatch, by name, before the
// handler runs.
// PREVENTS: a send inheriting the wildcard default and putting operator bytes on
// every session.
func TestSendRefusesAMissingSelector(t *testing.T) {
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	cmd := server.Dispatcher().Lookup(sendRawPath)
	require.NotNil(t, cmd, "the send form must be registered at %q", sendRawPath)
	assert.True(t, cmd.RequiresSelector, "every send must carry the selector guard")

	ctx := &pluginserver.CommandContext{Server: server}
	_, err = server.Dispatcher().Dispatch(ctx, "send bgp raw hex FF")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a selector")
	assert.Empty(t, ctx.Peer, "a refused send must leave no destination bound")
}

// TestSendIsDeniedToAReadOnlyProfile drives the whole authorization path from
// the dispatcher for a read-only operator.
//
// The denial is reached by two defaults and by no entry. IsReadOnlyPath is an
// ALLOWLIST of the read verbs, so it answers false for `send` without naming it,
// and the command is then judged in the profile's Edit section, whose default is
// Deny. Asserting a read command SUCCEEDS under the same profile is what proves
// the refusal came from that section rather than from a profile that denies
// everything.
//
// VALIDATES: AC-16 -- a send is unauthorized for a read-only profile, through
// Dispatch, with no rule that names the verb.
// PREVENTS: `send` being added to the read-verb allowlist, which would make every
// send a read and hand it to any operator who can run `show`.
func TestSendIsDeniedToAReadOnlyProfile(t *testing.T) {
	require.False(t, pluginserver.IsReadOnlyPath(sendRawPath),
		"a send writes to a wire, so the read-verb allowlist must not name it")

	store := authz.NewStore()
	store.AddProfile(authz.Profile{
		Name: "read-only",
		Run:  authz.Section{Default: authz.Allow},
		Edit: authz.Section{Default: authz.Deny},
	})
	store.AssignProfiles("watcher", []string{"read-only"})

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	server.Dispatcher().SetAuthorizer(authz.StoreAuthorizer{Store: store})

	ctx := &pluginserver.CommandContext{Server: server, Username: "watcher"}
	resp, err := server.Dispatcher().Dispatch(ctx, "send bgp 192.0.2.1 raw hex FF")
	require.ErrorIs(t, err, pluginserver.ErrUnauthorized)
	require.NotNil(t, resp)
	assert.Equal(t, plugin.StatusError, resp.Status)

	// The same profile, a read command: allowed. The refusal above is therefore
	// the Edit section's default, not a profile that refuses every command.
	assert.Equal(t, authz.Allow, store.Authorize("watcher", "show bgp summary", true),
		"a read-only profile must still allow reads, or the assertion above proves nothing")
}

// TestSendIsAVerbTheGrammarGateAccepts pins the verb declaration itself.
//
// R1 of the grammar ruleset requires the first token of every command path to be
// a canonical verb, and CheckName reads command.Verbs for that set. So this
// assertion answers AC-17 at the gate rather than at the map: a `send` path is
// grammar-clean because the verb is declared, and it stops being so the moment
// the entry is removed.
//
// The path is exempt from the gate TODAY (ze-bgp:peer-raw is a bridgeSurface
// member), and that exemption ends in the phase that shrinks the map. Checking
// the name directly is what makes the verb's declaration load-bearing now rather
// than at the end of the move.
//
// VALIDATES: AC-17 -- `send` is a canonical verb and `send bgp raw` produces no
// grammar finding. The ROLE it carries is pinned by TestVerbRegistryCanonical,
// which reads the unexported type this package cannot name.
// PREVENTS: the verb being dropped from command.Verbs, which would make every
// send path a first-token violation the day its exemption is removed.
func TestSendIsAVerbTheGrammarGateAccepts(t *testing.T) {
	assert.True(t, command.IsVerb("send"), "send must be a canonical verb")
	assert.Empty(t, grammar.CheckName(sendRawPath),
		"a send path must be grammar-clean, R1 included")
}
