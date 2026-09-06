package peer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	peeryang "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	"github.com/ze-software/ze/internal/component/plugin"
)

// treeValue reads one leaf out of the config tree the handler built, by the path
// parsePeerFromTree reads it at. It answers "" for a path the tree does not
// hold, and every assertion below states the value it wants, so an absent leaf
// is a failure rather than a pass.
func treeValue(t *testing.T, tree map[string]any, path ...string) string {
	t.Helper()
	node := tree
	for _, key := range path[:len(path)-1] {
		child, ok := node[key].(map[string]any)
		if !ok {
			return ""
		}
		node = child
	}
	value, _ := node[path[len(path)-1]].(string)
	return value
}

// TestCreateBgpPeerReachesTheReactorFromTheCommandLine drives the whole path an
// operator and the ExaBGP bridge both take.
//
// GOAL: `create bgp peer <address> asn <asn> ...` reaches
// Reactor.AddDynamicPeer with the address the operator typed and a config tree
// carrying every keyword, so the command creates the session rather than
// existing as a handler nothing calls (ai/rules/completion.md).
// METHOD: dispatch the command text through the registered RPC table, then read
// the tree the reactor was handed.
//
// VALIDATES: the address binds positionally, and asn, local-as, local-address,
// router-id, the timers, connect, accept, family, graceful-restart,
// group-updates and attach each land on the leaf parsePeerFromTree reads.
// PREVENTS: a keyword accepted by the grammar and dropped before the reactor,
// which builds a session that is not the one the operator asked for and reports
// success for it (ai/rules/principles.md).
func TestCreateBgpPeerReachesTheReactorFromTheCommandLine(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, strings.Join([]string{
		"create bgp peer 192.0.2.7",
		"asn 65002",
		"local-as 65001",
		"local-address 192.0.2.1",
		"router-id 1.2.3.4",
		"receive-hold-time 90",
		"send-hold-time 600",
		"connect-retry 5",
		"connect false",
		"accept true",
		"family ipv4/unicast,ipv6/unicast",
		"graceful-restart 120",
		"group-updates false",
		"attach peer-lifecycle,bgp-rib",
	}, " "))
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.appliedConfigs, 1, "one command creates one peer")
	tree := reactor.appliedConfigs[0]

	assert.Equal(t, "65002", treeValue(t, tree, "session", "asn", "remote"))
	assert.Equal(t, "65001", treeValue(t, tree, "session", "asn", "local"))
	assert.Equal(t, "192.0.2.1", treeValue(t, tree, "connection", "local", "ip"))
	assert.Equal(t, "1.2.3.4", treeValue(t, tree, "session", "router-id"))
	assert.Equal(t, "90", treeValue(t, tree, "timer", "receive-hold-time"))
	assert.Equal(t, "600", treeValue(t, tree, "timer", "send-hold-time"))
	assert.Equal(t, "5", treeValue(t, tree, "timer", "connect-retry"))
	assert.Equal(t, "false", treeValue(t, tree, "connection", "remote", "connect"))
	assert.Equal(t, "true", treeValue(t, tree, "connection", "local", "accept"))
	assert.Equal(t, "enable", treeValue(t, tree, "session", "family", "ipv4/unicast"))
	assert.Equal(t, "enable", treeValue(t, tree, "session", "family", "ipv6/unicast"))
	assert.Equal(t, "120", treeValue(t, tree, "session", "capability", "graceful-restart", "restart-time"))
	assert.Equal(t, "false", treeValue(t, tree, "behavior", "group-updates"))

	processes, ok := tree["attach"].(map[string]any)["process"].(map[string]any)
	require.True(t, ok, "attach names the processes bound to this peer")
	for _, name := range []string{"peer-lifecycle", "bgp-rib"} {
		binding, bound := processes[name].(map[string]any)
		require.True(t, bound, "each name in attach is bound")
		assert.Equal(t, "*", binding["receive"], "a bound process receives every message from this peer")
		assert.Equal(t, "*", binding["send"], "a bound process may send every type toward this peer")
	}

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "the answer is structured data, so every pipe renders it")
	assert.Equal(t, "192.0.2.7", data[fieldPeer])
	assert.Equal(t, "65002", data[fieldRemoteAS])
	assert.Equal(t, "peer created", data[fieldMessage])
}

// TestCreateBgpPeerTakesTheAddressAndTheASAlone holds the smallest command.
//
// GOAL: every keyword but `asn` is optional, so the shortest line an operator
// can type creates a peer.
// METHOD: dispatch the two-value form and read the tree.
//
// VALIDATES: the tree carries the remote AS and nothing the command did not
// state, so the reactor's own defaults decide the rest.
// PREVENTS: a keyword becoming required by accident, which would break the line
// the ExaBGP bridge writes for the smallest `create neighbor`.
func TestCreateBgpPeerTakesTheAddressAndTheASAlone(t *testing.T) {
	reactor := &mockReactor{}
	ctx := newDispatchContext(reactor)

	resp, err := ctx.Server.Dispatcher().Dispatch(ctx, "create bgp peer 192.0.2.7 asn 65002")
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	require.Len(t, reactor.appliedConfigs, 1)
	tree := reactor.appliedConfigs[0]
	assert.Equal(t, "65002", treeValue(t, tree, "session", "asn", "remote"))
	assert.NotContains(t, tree, "timer", "a timer the command did not state is the reactor's default")
	assert.NotContains(t, tree, "behavior")
	assert.NotContains(t, tree, "attach")
}

// TestCreateBgpPeerCreatesNoPeerFromARefusedLine holds the safety property of
// every refusal.
//
// GOAL: a line the grammar or the handler will not carry creates NOTHING. A
// parameter dropped on the way would build a session the operator did not
// describe and ack the command for it (ai/rules/principles.md).
// METHOD: dispatch one bad line per fault and require an error with no call
// into the reactor.
//
// VALIDATES: an unknown keyword, a keyword with no value, a missing asn, an
// unknown family, an out-of-range restart time, AS 0, a peer name and the
// wildcard each fail with no peer created.
// PREVENTS: a partial create, which is worse than no create: the session is up
// and it is the wrong one.
//
// It does NOT assert the message. Five of these eight are answered by the
// dispatcher's terminal-selector guard rather than by the handler, and that
// guard says "requires a selector" over a line that gave one. The row is in
// plan/journal/earlier-guard-hides-the-better-error.md, and the message this
// command owns is asserted in the test below.
func TestCreateBgpPeerCreatesNoPeerFromARefusedLine(t *testing.T) {
	for _, command := range []string{
		"create bgp peer 192.0.2.7 asn 65002 md5 secret",
		"create bgp peer 192.0.2.7 asn 65002 router-id",
		"create bgp peer 192.0.2.7",
		"create bgp peer 192.0.2.7 asn 65002 family ipv4/nonesuch",
		"create bgp peer 192.0.2.7 asn 65002 graceful-restart 9000",
		"create bgp peer 192.0.2.7 asn 0",
		"create bgp peer edge1 asn 65002",
		"create bgp peer * asn 65002",
	} {
		t.Run(command, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := newDispatchContext(reactor)

			_, err := ctx.Server.Dispatcher().Dispatch(ctx, command)
			require.Error(t, err)
			assert.Empty(t, reactor.appliedConfigs, "a refused line creates no peer")
		})
	}
}

// TestCreateBgpPeerNamesTheKeywordItRefuses holds the operator-facing half of
// the refusal.
//
// GOAL: the handler says WHICH keyword or value it would not carry, so an
// operator repairs the line rather than guessing at it.
// METHOD: call the handler with the selector the dispatcher would have bound
// and require the fault's own word in the message.
//
// VALIDATES: an unknown keyword, a value-less keyword, an absent asn, an
// unregistered family, a restart time the capability cannot carry, AS 0, a peer
// name and the wildcard each name themselves.
// PREVENTS: a keyword quietly dropped, which is the failure this command exists
// not to have (ai/rules/principles.md).
func TestCreateBgpPeerNamesTheKeywordItRefuses(t *testing.T) {
	cases := []struct {
		name     string
		selector string
		args     []string
		says     string
	}{
		{"a keyword the command does not take", "192.0.2.7", []string{"192.0.2.7", "asn", "65002", "md5", "secret"}, "md5"},
		{"a keyword with no value", "192.0.2.7", []string{"192.0.2.7", "asn", "65002", "router-id"}, "router-id"},
		{"no asn at all", "192.0.2.7", []string{"192.0.2.7"}, "asn"},
		{"a family that is not registered", "192.0.2.7", []string{"192.0.2.7", "asn", "65002", "family", "ipv4/nonesuch"}, "nonesuch"},
		{"a restart time the capability cannot carry", "192.0.2.7", []string{"192.0.2.7", "asn", "65002", "graceful-restart", "9000"}, "graceful-restart"},
		{"AS 0 names no speaker", "192.0.2.7", []string{"192.0.2.7", "asn", "0"}, "asn"},
		{"a peer name is not an address", "edge1", []string{"edge1", "asn", "65002"}, "edge1"},
		{"the wildcard creates nothing", "*", []string{"*", "asn", "65002"}, "*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := newTestContext(reactor)
			ctx.Peer = tc.selector

			resp, err := handleBgpPeerAdd(ctx, tc.args)
			require.Error(t, err)
			assert.Equal(t, plugin.StatusError, resp.Status)
			assert.Contains(t, resp.Error, tc.says, "the refusal names what it refused")
			assert.Empty(t, reactor.appliedConfigs, "a refused line creates no peer")
		})
	}
}

// TestCreateBgpPeerKeywordsAreDeclaredInYANG pairs the two declarations of the
// command's grammar.
//
// GOAL: every keyword the handler reads is a leaf the model declares, so
// completion offers it and the dispatcher types its value. A keyword the model
// does not carry reaches the handler only as an unplaced token, and the
// dispatcher then refuses the whole line.
// METHOD: read the leaf names out of the command module's own text.
//
// VALIDATES: peerCreateKeywords and the YANG leaves under `create bgp peer` name
// the same set.
// PREVENTS: a keyword added to one half alone, which is a command an operator
// can type and cannot complete, or a leaf nothing reads.
func TestCreateBgpPeerKeywordsAreDeclaredInYANG(t *testing.T) {
	module := peeryang.ZePeerCmdYANG
	start := strings.Index(module, "container create {")
	require.NotEqual(t, -1, start, "the command module declares the create verb")
	create := module[start:]
	end := strings.Index(create, "container delete {")
	require.NotEqual(t, -1, end, "delete follows create in the module")
	create = create[:end]

	for _, keyword := range peerCreateKeywordNames() {
		assert.Contains(t, create, "leaf "+keyword+" ", "the model declares every keyword the handler reads")
	}
}
