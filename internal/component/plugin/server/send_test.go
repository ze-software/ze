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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/authz"
	cliclient "github.com/ze-software/ze/internal/component/cli/client"
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

// declaredArgumentPaths are the commands the send spec found taking
// operator-typed tokens the model never stated, with the invocation form each
// one now generates.
//
// The line is the MODEL's answer, rendered by command.Usage. A leaf that stops
// being declared changes it, and so does a leaf declared in another order.
//
// Each expected line is written out rather than derived. A line derived from the
// same ArgDefs the renderer reads would agree with itself, whatever the model
// said.
var declaredArgumentPaths = []struct {
	path  string
	usage string
	reads string
}{
	{
		path:  "send bgp raw",
		usage: "send bgp <selector> raw <hex|b64> <data> [type <open|update|notification|keepalive|route-refresh>]",
		reads: "rawArguments reads the encoding, the data and the optional message type",
	},
	{
		path:  "send bgp update",
		usage: "send bgp <selector> update <text|hex|b64|cursor>",
		reads: "handleUpdate reads the encoding word and hands the rest to that encoding's parser",
	},
	{
		path:  "request cache forward",
		usage: "request cache forward <id> <selector>",
		reads: "handleCacheForwardRPC reads the cache id and the peer selector",
	},
}

// TestSendArgumentsAreDeclared holds each command's generated usage line to the
// arguments its handler reads.
//
// The commands in plan/journal/command-takes-an-untyped-positional-value.md
// declared a selector and nothing else. Completion then offered nothing after
// the form word, and the generated line stated a command that took no
// arguments. A path no operator has typed before must not ship with that
// grammar.
//
// The second half is what makes the declaration load-bearing rather than
// decorative. A word outside a declared enumeration is refused by the
// DISPATCHER, with the enumeration named, before any handler runs.
//
// The server carries no reactor here. A handler that ran would answer "BGP
// reactor not available" instead, and the assertion reads that difference.
//
// VALIDATES: every argument each handler reads is named and typed in the model,
// and a value outside an enumeration is refused before the handler runs.
// PREVENTS: the tail grammar living only in a handler's doc comment, where
// completion, the generated help and the command catalog cannot reach it.
func TestSendArgumentsAreDeclared(t *testing.T) {
	tree := cliclient.YANGCommandTree()
	require.NotNil(t, tree)

	for _, want := range declaredArgumentPaths {
		t.Run(want.path, func(t *testing.T) {
			path := strings.Fields(want.path)
			node := command.FindNode(tree, path)
			require.NotNil(t, node, "the model must carry a node at %q", want.path)
			assert.Equal(t, want.usage, command.UsageLine(command.Usage(path, node)),
				"the generated line must name every argument the handler reads: %s", want.reads)
		})
	}

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	refusals := []struct {
		command string
		says    string
	}{
		{command: "send bgp 192.0.2.1 raw gzip DEADBEEF", says: "expected one of: hex, b64"},
		{command: "send bgp 192.0.2.1 raw hex DEADBEEF type heartbeat", says: "expected one of: open, update, notification, keepalive, route-refresh"},
		{command: "send bgp 192.0.2.1 update json", says: "expected one of: text, hex, b64, cursor"},
		// The same bad encoding with a route expression behind it. The
		// dispatcher leaves more tokens over than it leaves definitions open.
		// It cannot say which token was typed for which leaf, so it names the
		// declaration instead (validateCommandArgs). The refusal is still the
		// model's, and the handler still does not run.
		{command: "send bgp 192.0.2.1 update json nlri ipv4/unicast add 10.0.0.0/24", says: "required argument missing: encoding"},
	}
	for _, refusal := range refusals {
		t.Run(refusal.command, func(t *testing.T) {
			ctx := &pluginserver.CommandContext{Server: server}
			_, dispatchErr := server.Dispatcher().Dispatch(ctx, refusal.command)
			require.Error(t, dispatchErr)
			assert.Contains(t, dispatchErr.Error(), refusal.says,
				"the refusal must come from the model, so it names the declared set or the declaration")
			assert.NotContains(t, dispatchErr.Error(), "reactor",
				"a refusal that reached the handler would answer about the missing reactor instead")
		})
	}
}

// movedSendPaths are the old command paths this grammar has already left, with
// the line an operator typed at each one.
//
// The list grows as each form moves. It is never a list of paths that were
// removed and replaced by an alias, because an alias is what would make it
// stale without a failure.
var movedSendPaths = []struct {
	path  string
	typed string
	nowAt string
}{
	{
		path:  "peer raw",
		typed: "peer 192.0.2.1 raw hex DEADBEEF",
		nowAt: "send bgp raw",
	},
	{
		path:  "peer update",
		typed: "peer 192.0.2.1 update text nlri ipv4/unicast add 10.0.0.0/24",
		nowAt: "send bgp update",
	},
}

// TestOldSendPathsMatchNothing holds each moved form to one path.
//
// ze is unreleased, so a grammar it replaces is deleted rather than deprecated
// (ai/rules/cli.md). Two paths for one wire method would leave the model saying
// a send names its destination in two places, which is the thing the move
// removes. The model half and the dispatcher half are both asserted, because a
// node deleted from the schema and a handler still reachable through some other
// match are different failures.
//
// VALIDATES: AC-6 for the forms that have moved -- the old node is gone from the
// merged tree and the old line is refused, while the new line reaches the model.
// PREVENTS: the old spelling surviving as an alias, which no gate would report
// and which every migrated sender would then hide.
func TestOldSendPathsMatchNothing(t *testing.T) {
	tree := cliclient.YANGCommandTree()
	require.NotNil(t, tree)

	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)

	for _, moved := range movedSendPaths {
		t.Run(moved.path, func(t *testing.T) {
			assert.Nil(t, command.FindNode(tree, strings.Fields(moved.path)),
				"the merged tree must carry no node at the path the form left")
			assert.NotNil(t, command.FindNode(tree, strings.Fields(moved.nowAt)),
				"the form must answer at the path it moved to, or this proves only a deletion")

			ctx := &pluginserver.CommandContext{Server: server}
			_, dispatchErr := server.Dispatcher().Dispatch(ctx, moved.typed)
			require.ErrorIs(t, dispatchErr, pluginserver.ErrUnknownCommand,
				"the old line must reach no handler")
		})
	}
}
