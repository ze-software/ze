package yang

import (
	"os"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadCmdModule is a test helper that loads a -cmd.yang file by path into a loader.
// The loader must already have LoadEmbedded() called (for ze-extensions import).
func loadCmdModule(t *testing.T, loader *Loader, path string) {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err, "reading %s", path)
	err = loader.AddModuleFromText(path, string(content))
	require.NoError(t, err, "loading %s", path)
}

// TestCommandExtension verifies ze:command extension is parsed from YANG with its WireMethod argument.
//
// VALIDATES: goyang parses ze:command with handler argument on config false containers.
// PREVENTS: Command tree walker missing executable command nodes or losing WireMethod.
func TestCommandExtension(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-cmd {
    namespace "urn:test:cmd";
    prefix tc;

    import ze-extensions { prefix ze; }

    container peer {
        config false;
        description "Peer operations";

        container list {
            config false;
            ze:command "ze-bgp:peer-list";
            description "List all peers";
        }

        container add {
            config false;
            ze:command "ze-bgp:peer-add";
            description "Add a new peer";

            leaf address {
                type string;
                description "Peer address";
            }
        }

        container status {
            config false;
            description "Status grouping (not a command, just a branch)";
        }
    }
}
`

	err = loader.AddModuleFromText("test-cmd.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("test-cmd")
	require.NotNil(t, entry)

	peerEntry := entry.Dir["peer"]
	require.NotNil(t, peerEntry)
	assert.Equal(t, gyang.TSFalse, peerEntry.Config, "peer should be config false")
	assert.Equal(t, "", GetCommandExtension(peerEntry), "peer is a grouping, no handler")

	listEntry := peerEntry.Dir["list"]
	require.NotNil(t, listEntry)
	assert.Equal(t, "ze-bgp:peer-list", GetCommandExtension(listEntry))
	assert.True(t, HasCommandExtension(listEntry))

	addEntry := peerEntry.Dir["add"]
	require.NotNil(t, addEntry)
	assert.Equal(t, "ze-bgp:peer-add", GetCommandExtension(addEntry))
	assert.NotNil(t, addEntry.Dir["address"], "add should have address leaf child")

	statusEntry := peerEntry.Dir["status"]
	require.NotNil(t, statusEntry)
	assert.False(t, HasCommandExtension(statusEntry), "status has no ze:command")
}

// TestEditShortcutExtension verifies ze:edit-shortcut extension is parsed from YANG.
//
// VALIDATES: goyang parses ze:edit-shortcut on command containers.
// PREVENTS: Edit mode missing shortcut commands.
func TestEditShortcutExtension(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-shortcut {
    namespace "urn:test:shortcut";
    prefix ts;

    import ze-extensions { prefix ze; }

    container commit {
        config false;
        ze:command "ze-bgp:commit";
        ze:edit-shortcut;
        description "Apply config changes";
    }

    container save {
        config false;
        ze:command "ze-bgp:save";
        ze:edit-shortcut;
        description "Persist config";
    }

    container summary {
        config false;
        ze:command "ze-bgp:summary";
        description "Show summary (not an edit shortcut)";
    }
}
`

	err = loader.AddModuleFromText("test-shortcut.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("test-shortcut")
	require.NotNil(t, entry)

	commitEntry := entry.Dir["commit"]
	require.NotNil(t, commitEntry)
	assert.Equal(t, "ze-bgp:commit", GetCommandExtension(commitEntry))
	assert.True(t, HasEditShortcutExtension(commitEntry), "commit should have ze:edit-shortcut")

	saveEntry := entry.Dir["save"]
	require.NotNil(t, saveEntry)
	assert.True(t, HasEditShortcutExtension(saveEntry), "save should have ze:edit-shortcut")

	summaryEntry := entry.Dir["summary"]
	require.NotNil(t, summaryEntry)
	assert.Equal(t, "ze-bgp:summary", GetCommandExtension(summaryEntry))
	assert.False(t, HasEditShortcutExtension(summaryEntry), "summary should NOT have ze:edit-shortcut")
}

// TestExtensionNilEntry verifies extension functions handle nil safely.
//
// VALIDATES: No panic on nil entry.
// PREVENTS: NPE in tree walker when entry is nil.
func TestExtensionNilEntry(t *testing.T) {
	assert.Equal(t, "", GetCommandExtension(nil))
	assert.False(t, HasCommandExtension(nil))
	assert.False(t, HasEditShortcutExtension(nil))
}

// cmdPluginBase is the relative path from this test package to the BGP plugins directory.
const cmdPluginBase = "../../../component/bgp/plugins/"

// cmdBase is the relative path from this test package to the general command plugins directory.
const cmdBase = "../../../component/cmd/"

// TestPeerCmdModule verifies ze-peer-cmd.yang (peer operations from cmd/peer plugin).
//
// VALIDATES: Peer command YANG module loads with correct hierarchy and WireMethod refs.
// PREVENTS: Peer commands missing or mislinked in the command tree.
func TestPeerCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"cmd/peer/schema/ze-peer-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-peer-cmd")
	require.NotNil(t, entry)

	// summary is top-level
	assert.Equal(t, "ze-bgp:summary", GetCommandExtension(entry.Dir["summary"]))
	assert.False(t, HasEditShortcutExtension(entry.Dir["summary"]))

	// peer grouping
	peer := entry.Dir["peer"]
	require.NotNil(t, peer)
	assert.Equal(t, "", GetCommandExtension(peer), "peer grouping has no handler")
	assert.Equal(t, gyang.TSFalse, peer.Config)

	// peer commands
	assert.Equal(t, "ze-bgp:peer-list", GetCommandExtension(peer.Dir["list"]))
	assert.Equal(t, "ze-bgp:peer-detail", GetCommandExtension(peer.Dir["detail"]))
	assert.Equal(t, "ze-bgp:peer-add", GetCommandExtension(peer.Dir["add"]))
	assert.Equal(t, "ze-bgp:peer-remove", GetCommandExtension(peer.Dir["remove"]))
	assert.Equal(t, "ze-bgp:peer-teardown", GetCommandExtension(peer.Dir["teardown"]))
	assert.Equal(t, "ze-bgp:peer-pause", GetCommandExtension(peer.Dir["pause"]))
	assert.Equal(t, "ze-bgp:peer-resume", GetCommandExtension(peer.Dir["resume"]))
	assert.Equal(t, "ze-bgp:peer-save", GetCommandExtension(peer.Dir["save"]))
	assert.Equal(t, "ze-bgp:peer-capabilities", GetCommandExtension(peer.Dir["capabilities"]))
	assert.Equal(t, "ze-bgp:peer-statistics", GetCommandExtension(peer.Dir["statistics"]))

	// deep nesting: peer > plugin > session > ready
	assert.Equal(t, "ze-plugin:session-peer-ready",
		GetCommandExtension(peer.Dir["plugin"].Dir["session"].Dir["ready"]))
}

// TestRibCmdModule verifies ze-rib-cmd.yang (RIB operations from cmd/rib plugin).
//
// VALIDATES: RIB command YANG module loads with correct hierarchy and WireMethod refs.
// PREVENTS: RIB commands missing from the command tree.
func TestRibCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"cmd/rib/schema/ze-rib-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-rib-cmd")
	require.NotNil(t, entry)

	rib := entry.Dir["rib"]
	require.NotNil(t, rib)
	assert.Equal(t, "ze-rib-api:status", GetCommandExtension(rib.Dir["status"]))
	assert.Equal(t, "ze-rib-api:routes", GetCommandExtension(rib.Dir["routes"]))

	best := rib.Dir["best"]
	require.NotNil(t, best)
	assert.Equal(t, "ze-rib-api:best", GetCommandExtension(best))
	assert.Equal(t, "ze-rib-api:best-status", GetCommandExtension(best.Dir["status"]))

	ribClear := rib.Dir["clear"]
	require.NotNil(t, ribClear)
	assert.Equal(t, "ze-rib-api:clear-in", GetCommandExtension(ribClear.Dir["in"]))
	assert.Equal(t, "ze-rib-api:clear-out", GetCommandExtension(ribClear.Dir["out"]))
}

// TestRefreshCmdModule verifies ze-refresh-cmd.yang (route refresh from route_refresh plugin).
//
// VALIDATES: Refresh command YANG module loads with correct hierarchy.
// PREVENTS: Route refresh commands missing from the command tree.
func TestRefreshCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"route_refresh/schema/ze-refresh-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-refresh-cmd")
	require.NotNil(t, entry)

	peer := entry.Dir["peer"]
	require.NotNil(t, peer)
	assert.Equal(t, "ze-bgp:peer-refresh", GetCommandExtension(peer.Dir["refresh"]))
	assert.Equal(t, "ze-bgp:peer-borr", GetCommandExtension(peer.Dir["borr"]))
	assert.Equal(t, "ze-bgp:peer-eorr", GetCommandExtension(peer.Dir["eorr"]))
	assert.Equal(t, "ze-bgp:peer-clear-soft", GetCommandExtension(peer.Dir["clear"].Dir["soft"]))
}

// TestMetaCmdModule verifies ze-meta-cmd.yang (introspection from cmd/meta plugin).
//
// VALIDATES: Meta command YANG module loads with help, command, event, plugin groups.
// PREVENTS: Introspection commands missing from the command tree.
func TestMetaCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"meta/schema/ze-meta-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-meta-cmd")
	require.NotNil(t, entry)

	assert.Equal(t, "ze-bgp:help", GetCommandExtension(entry.Dir["help"]))
	assert.Equal(t, "ze-bgp:command-list", GetCommandExtension(entry.Dir["command"].Dir["list"]))
	assert.Equal(t, "ze-bgp:event-list", GetCommandExtension(entry.Dir["event"].Dir["list"]))
	assert.Equal(t, "ze-bgp:plugin-encoding", GetCommandExtension(entry.Dir["plugin"].Dir["encoding"]))
}

// TestSimpleCmdModules verifies cache, commit, subscribe, log, metrics cmd modules.
//
// VALIDATES: Simple command YANG modules load and have correct WireMethod refs.
// PREVENTS: Simple commands missing from the command tree.
func TestSimpleCmdModules(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		module     string
		container  string
		wireMethod string
	}{
		{"cache", cmdBase + "cache/schema/ze-cache-cmd.yang", "ze-cache-cmd", "cache", "ze-bgp:cache"},
		{"commit", cmdBase + "commit/schema/ze-commit-cmd.yang", "ze-commit-cmd", "commit", "ze-bgp:commit"},
		{"subscribe", cmdBase + "subscribe/schema/ze-subscribe-cmd.yang", "ze-subscribe-cmd", "subscribe", "ze-bgp:subscribe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLoader()
			err := loader.LoadEmbedded()
			require.NoError(t, err)
			loadCmdModule(t, loader, tt.path)
			err = loader.Resolve()
			require.NoError(t, err)

			entry := loader.GetEntry(tt.module)
			require.NotNil(t, entry)
			assert.Equal(t, tt.wireMethod, GetCommandExtension(entry.Dir[tt.container]))
		})
	}
}

// TestCommitNoEditShortcut verifies ze-bgp:commit is NOT an edit shortcut.
// The editor's "commit" (config commit) is internal, not the ze-bgp:commit RPC (named route commits).
//
// VALIDATES: ze-bgp:commit does not have ze:edit-shortcut.
// PREVENTS: Confusion between editor commit and RPC commit.
func TestCommitNoEditShortcut(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"commit/schema/ze-commit-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-commit-cmd")
	require.NotNil(t, entry)
	assert.False(t, HasEditShortcutExtension(entry.Dir["commit"]), "ze-bgp:commit is NOT an edit shortcut")
}

// TestRawCmdModule verifies ze-raw-cmd.yang (peer raw from cmd/raw plugin).
//
// VALIDATES: Raw command YANG module loads with peer > raw node.
// PREVENTS: Raw command missing from the command tree.
func TestRawCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"cmd/raw/schema/ze-raw-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-raw-cmd")
	require.NotNil(t, entry)
	assert.Equal(t, "ze-bgp:peer-raw", GetCommandExtension(entry.Dir["peer"].Dir["raw"]))
}

// TestUpdateCmdModule verifies ze-update-cmd.yang (peer update from cmd/update plugin).
//
// VALIDATES: Update command YANG module loads with peer > update node.
// PREVENTS: Update command missing from the command tree.
func TestUpdateCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"cmd/update/schema/ze-update-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-update-cmd")
	require.NotNil(t, entry)
	assert.Equal(t, "ze-bgp:peer-update", GetCommandExtension(entry.Dir["peer"].Dir["update"]))
}

// TestSystemCmdModuleLoads verifies ze-system-cmd.yang loads and has expected structure.
//
// VALIDATES: System command YANG module loads with correct hierarchy and WireMethod refs.
// PREVENTS: System commands missing from the unified tree.
func TestSystemCmdModuleLoads(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, "../../../core/ipc/schema/ze-system-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-system-cmd")
	require.NotNil(t, entry, "ze-system-cmd module should be loadable")

	// system group
	sys := entry.Dir["system"]
	require.NotNil(t, sys)
	assert.Equal(t, "ze-system:help", GetCommandExtension(sys.Dir["help"]))
	assert.Equal(t, "ze-system:dispatch", GetCommandExtension(sys.Dir["dispatch"]))

	// system > version
	ver := sys.Dir["version"]
	require.NotNil(t, ver)
	assert.Equal(t, "ze-system:version-software", GetCommandExtension(ver.Dir["software"]))
	assert.Equal(t, "ze-system:version-api", GetCommandExtension(ver.Dir["api"]))

	// system > command
	cmd := sys.Dir["command"]
	require.NotNil(t, cmd)
	assert.Equal(t, "ze-system:command-list", GetCommandExtension(cmd.Dir["list"]))

	// daemon group
	daemon := entry.Dir["daemon"]
	require.NotNil(t, daemon)
	assert.Equal(t, "ze-system:daemon-shutdown", GetCommandExtension(daemon.Dir["shutdown"]))
	assert.Equal(t, "ze-system:daemon-status", GetCommandExtension(daemon.Dir["status"]))
	assert.Equal(t, "ze-system:daemon-reload", GetCommandExtension(daemon.Dir["reload"]))
}

// TestPluginCmdModuleLoads verifies ze-plugin-cmd.yang loads and has expected structure.
//
// VALIDATES: Plugin command YANG module loads with correct hierarchy and WireMethod refs.
// PREVENTS: Plugin commands missing from the unified tree.
func TestPluginCmdModuleLoads(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, "../../../core/ipc/schema/ze-plugin-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-plugin-cmd")
	require.NotNil(t, entry, "ze-plugin-cmd module should be loadable")

	plugin := entry.Dir["plugin"]
	require.NotNil(t, plugin)
	assert.Equal(t, "ze-plugin:help", GetCommandExtension(plugin.Dir["help"]))

	// command subgroup
	cmd := plugin.Dir["command"]
	require.NotNil(t, cmd)
	assert.Equal(t, "ze-plugin:command-list", GetCommandExtension(cmd.Dir["list"]))
	assert.Equal(t, "ze-plugin:command-help", GetCommandExtension(cmd.Dir["help"]))
	assert.Equal(t, "ze-plugin:command-complete", GetCommandExtension(cmd.Dir["complete"]))

	// session subgroup
	session := plugin.Dir["session"]
	require.NotNil(t, session)
	assert.Equal(t, "ze-plugin:session-ready", GetCommandExtension(session.Dir["ready"]))
	assert.Equal(t, "ze-plugin:session-ping", GetCommandExtension(session.Dir["ping"]))
	assert.Equal(t, "ze-plugin:session-bye", GetCommandExtension(session.Dir["bye"]))
}
