package peer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// savedConfig is the configuration file these tests start from. It declares one
// peer, which stands for every peer an operator wrote down rather than created
// at runtime.
const savedConfig = `bgp {
	peer peer1 {
		connection {
			remote {
				ip 127.0.0.1
			}
			local {
				ip 127.0.0.1
				accept false
			}
		}
		session {
			asn {
				local 1
				remote 1
			}
			router-id 1.2.3.4
			family {
				ipv4/unicast { prefix { maximum 10000; } }
			}
		}
	}
}
`

// runtimePeerTree is what `create bgp peer 192.0.2.7 asn 65002 family
// ipv4/unicast` leaves in the running configuration: the tree the command built
// (peerCreateTree, create.go) with the two leaves AddDynamicPeer fills in.
func runtimePeerTree() map[string]any {
	return map[string]any{
		"session": map[string]any{
			"asn": map[string]any{"remote": "65002"},
			"family": map[string]any{
				"ipv4/unicast": map[string]any{"prefix": map[string]any{"maximum": "1000"}},
			},
		},
		"connection": map[string]any{
			"remote": map[string]any{"ip": "192.0.2.7"},
			"local":  map[string]any{"ip": "auto"},
		},
	}
}

// configuredPeerTree is peer1 as the running configuration carries it, which is
// the file's own lowering.
func configuredPeerTree() map[string]any {
	return map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"ip": "127.0.0.1"},
			"local":  map[string]any{"ip": "127.0.0.1", "accept": "false"},
		},
		"session": map[string]any{
			"asn":       map[string]any{"local": "1", "remote": "1"},
			"router-id": "1.2.3.4",
		},
	}
}

// newSaveContext writes the configuration file, and answers the command context
// a handler runs in: a reactor whose running configuration holds the named
// peers, and the path of the file just written.
func newSaveContext(t *testing.T, running map[string]any) (*pluginserver.CommandContext, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ze-bgp.conf")
	require.NoError(t, os.WriteFile(path, []byte(savedConfig), 0o600))

	reactor := &mockReactor{configTree: map[string]any{"bgp": map[string]any{"peer": running}}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{ConfigPath: path}, reactor)
	require.NoError(t, err)

	return &pluginserver.CommandContext{Server: server}, path
}

// peersInFile answers the peers a configuration file declares, read back with
// the parser a starting daemon reads it with.
//
// The parse is asserted rather than assumed: the editor writes a file without
// reading it back, and a file it cannot parse is a peer saved and lost. That is
// the whole of "a daemon started on that file brings the peer up" this test can
// hold; the functional test holds the rest.
func peersInFile(t *testing.T, path string) map[string]map[string]any {
	t.Helper()

	content, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temp file
	require.NoError(t, err)

	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(string(content))
	require.NoError(t, err, "the saved configuration file parses")

	bgp := tree.GetContainer("bgp")
	require.NotNil(t, bgp, "the file declares a bgp block")

	peers := make(map[string]map[string]any)
	for key, entry := range bgp.GetList("peer") {
		peers[key] = entry.ToMap()
	}
	return peers
}

// TestPeerSaveWritesACreatedPeerToTheFile is AC-10.
//
// GOAL: a peer created at runtime survives a restart once the operator saves
// it. The file must carry the AS and every other value the create command
// stated, because a daemon started on that file builds the peer from the file
// alone.
// METHOD: run the handler with a running configuration holding the configured
// peer and one created at runtime, then read the file back through the editor.
//
// VALIDATES: the file declares the created peer, with its remote AS, its
// address and the family the command named; the answer names it under "added".
// PREVENTS: a save that writes a peer the file cannot bring back up, which is
// what writing the fields of a PeerInfo produces: it carries no family, no
// graceful restart time and no process binding.
func TestPeerSaveWritesACreatedPeerToTheFile(t *testing.T) {
	ctx, path := newSaveContext(t, map[string]any{
		"peer1":          configuredPeerTree(),
		"peer-192.0.2.7": runtimePeerTree(),
	})

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok, "the answer is structured data")
	assert.Equal(t, []string{"peer-192.0.2.7"}, data[fieldSaveAdded])
	assert.Empty(t, data[fieldSaveRemoved])

	peers := peersInFile(t, path)
	require.Contains(t, peers, "peer-192.0.2.7", "the file declares the created peer")
	require.Contains(t, peers, "peer1", "the configured peer is left alone")

	created := peers["peer-192.0.2.7"]
	assert.Equal(t, "65002", subtree(t, created, "session", "asn")["remote"])
	assert.Equal(t, "1000",
		subtree(t, created, "session", "family", "ipv4/unicast", "prefix")["maximum"],
		"every leaf the create command stated reaches the file")
	assert.Equal(t, "192.0.2.7", subtree(t, created, "connection", "remote")["ip"])
}

// subtree answers the container one config subtree holds at a path, and fails
// the test where any level of it is absent or holds something else.
func subtree(t *testing.T, tree map[string]any, path ...string) map[string]any {
	t.Helper()

	node := tree
	for i, key := range path {
		child, ok := node[key].(map[string]any)
		require.True(t, ok, "no container at %v", path[:i+1])
		node = child
	}
	return node
}

// TestPeerSaveTakesADeletedPeerOutOfTheFile is AC-11.
//
// GOAL: `delete bgp peer` on a configured peer is made permanent by the save.
// A daemon started on the saved file must not bring that peer back up.
// METHOD: run the handler with a running configuration that no longer names the
// peer the file declares.
//
// VALIDATES: the file declares no peer at all afterwards, and the answer names
// the peer under "removed".
// PREVENTS: a save that persists presence only, which would write a runtime
// creation to the file and silently lose a runtime deletion.
func TestPeerSaveTakesADeletedPeerOutOfTheFile(t *testing.T) {
	ctx, path := newSaveContext(t, map[string]any{})

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Equal(t, []string{"peer1"}, data[fieldSaveRemoved])
	assert.Empty(t, data[fieldSaveAdded])

	assert.NotContains(t, peersInFile(t, path), "peer1", "the deleted peer leaves the file")
}

// TestPeerSaveLeavesAnAgreeingFileAlone is AC-12.
//
// GOAL: once the running configuration and the file agree, the save is a no-op,
// so an operator can run it at any time and a later commit of an unrelated leaf
// removes no peer.
// METHOD: run the handler with a running configuration holding exactly the peer
// the file declares, and compare the file with what it was.
//
// VALIDATES: nothing is added, nothing is removed, and the file is unchanged
// byte for byte.
// PREVENTS: a save that rewrites a peer the file already declares, which would
// drop the leaves the operator wrote and the comments around them.
func TestPeerSaveLeavesAnAgreeingFileAlone(t *testing.T) {
	ctx, path := newSaveContext(t, map[string]any{"peer1": configuredPeerTree()})

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	assert.Empty(t, data[fieldSaveAdded])
	assert.Empty(t, data[fieldSaveRemoved])

	content, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temp file
	require.NoError(t, err)
	assert.Equal(t, savedConfig, string(content), "a save that changes nothing writes nothing")
}

// TestPeerSaveRefusesASelector is AC-13.
//
// GOAL: an operator who types a peer after the command is told the command
// takes no selector, rather than being given a save of a set they did not ask
// for.
// METHOD: call the handler with one argument, the way the dispatcher hands a
// trailing token on, and the way a plugin sends one over the IPC transport.
//
// VALIDATES: the call fails, the message says the command takes no selector and
// acts on the whole running set, and the file is untouched.
// PREVENTS: a selector read as "save this one peer", which cannot express the
// half of this command that persists an ABSENCE: after a delete the peer is
// gone from the running set, so a selector naming it selects nothing.
func TestPeerSaveRefusesASelector(t *testing.T) {
	ctx, path := newSaveContext(t, map[string]any{})

	resp, err := handleBgpPeerSave(ctx, []string{"192.0.2.7"})
	require.ErrorIs(t, err, errSaveTakesNoSelector)
	require.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "no selector")
	assert.Contains(t, resp.Error, "running peer set")

	content, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temp file
	require.NoError(t, err)
	assert.Equal(t, savedConfig, string(content), "a refused command writes nothing")
}

// TestPeerSaveRefusesWithNoConfigFile holds the guard on a daemon that was
// given its configuration on stdin.
//
// GOAL: the command says there is no file to write rather than reporting a save
// that reached nothing.
// METHOD: run the handler on a server with no config path.
//
// VALIDATES: the call fails and names the missing path.
// PREVENTS: a silent success, which is what an empty path opened as a file
// would produce.
func TestPeerSaveRefusesWithNoConfigFile(t *testing.T) {
	reactor := &mockReactor{}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: server}

	resp, err := handleBgpPeerSave(ctx, nil)
	require.ErrorIs(t, err, errConfigPathNotSet)
	assert.Equal(t, plugin.StatusError, resp.Status)
}

// TestPeerSaveRefusesANameTwoPeersShare holds the check that makes "a name on
// both sides is the same peer" true rather than assumed.
//
// GOAL: a created peer whose derived name an operator already gave to another
// address is refused, rather than left unsaved under an answer that says
// everything was saved.
// METHOD: the file declares peer1 at 127.0.0.1; the running configuration
// declares peer1 at another address, which is what an operator writing that
// name over a second peer produces.
//
// VALIDATES: the save fails, and the message names both addresses.
// PREVENTS: the silent skip, which would answer "0 added" for a peer the
// operator asked to persist.
func TestPeerSaveRefusesANameTwoPeersShare(t *testing.T) {
	running := configuredPeerTree()
	subtree(t, running, "connection", "remote")["ip"] = "192.0.2.9"

	ctx, path := newSaveContext(t, map[string]any{"peer1": running})

	resp, err := handleBgpPeerSave(ctx, nil)
	require.ErrorIs(t, err, errSaveNameDisagrees)
	require.Equal(t, plugin.StatusError, resp.Status)
	assert.Contains(t, resp.Error, "127.0.0.1")
	assert.Contains(t, resp.Error, "192.0.2.9")

	content, err := os.ReadFile(path) //nolint:gosec // the path is the test's own temp file
	require.NoError(t, err)
	assert.Equal(t, savedConfig, string(content), "a refused save writes nothing")
}
