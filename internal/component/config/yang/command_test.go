package yang

import (
	"os"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
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

	show := entry.Dir["show"]
	require.NotNil(t, show)

	bgp := show.Dir["bgp"]
	require.NotNil(t, bgp)

	showPeer := bgp.Dir["peer"]
	require.NotNil(t, showPeer)
	assert.Equal(t, "ze-bgp:peer-list", GetCommandExtension(showPeer.Dir["list"]))
	assert.Equal(t, "ze-bgp:peer-detail", GetCommandExtension(showPeer.Dir["detail"]))
	assert.Equal(t, "ze-bgp:peer-capabilities", GetCommandExtension(showPeer.Dir["capabilities"]))
	assert.Equal(t, "ze-bgp:peer-statistics", GetCommandExtension(showPeer.Dir["statistics"]))
	assert.Equal(t, "ze-bgp:peer-history", GetCommandExtension(showPeer.Dir["history"]))
	assert.Equal(t, "ze-bgp:peer-rib", GetCommandExtension(showPeer.Dir["rib"]))

	peer := entry.Dir["peer"]
	require.NotNil(t, peer)
	assert.Equal(t, "", GetCommandExtension(peer), "peer grouping has no handler")
	assert.Equal(t, gyang.TSFalse, peer.Config)
	assert.Equal(t, "ze-bgp:peer-list", GetCommandExtension(peer.Dir["list"]))
	assert.Equal(t, "ze-bgp:peer-teardown", GetCommandExtension(peer.Dir["teardown"]))
	assert.Equal(t, "ze-bgp:peer-pause", GetCommandExtension(peer.Dir["pause"]))
	assert.Equal(t, "ze-bgp:peer-resume", GetCommandExtension(peer.Dir["resume"]))
	assert.Equal(t, "ze-bgp:peer-flush", GetCommandExtension(peer.Dir["flush"]))
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

	show := entry.Dir["show"]
	require.NotNil(t, show)
	bgp := show.Dir["bgp"]
	require.NotNil(t, bgp)
	rib := bgp.Dir["rib"]
	require.NotNil(t, rib)
	assert.Equal(t, "ze-rib-api:rpf", GetCommandExtension(rib.Dir["rpf"]))

	clear := entry.Dir["clear"]
	require.NotNil(t, clear)
	clearBGP := clear.Dir["bgp"]
	require.NotNil(t, clearBGP)
	ribClear := clearBGP.Dir["rib"]
	require.NotNil(t, ribClear)
	assert.Equal(t, "ze-rib-api:clear-in", GetCommandExtension(ribClear.Dir["in"]))
	assert.Equal(t, "ze-rib-api:clear-out", GetCommandExtension(ribClear.Dir["out"]))

	request := entry.Dir["request"]
	require.NotNil(t, request)
	requestBGP := request.Dir["bgp"]
	require.NotNil(t, requestBGP)
	ribRequest := requestBGP.Dir["rib"]
	require.NotNil(t, ribRequest)
	assert.Equal(t, "ze-rib-api:inject", GetCommandExtension(ribRequest.Dir["inject"]))
	assert.Equal(t, "ze-rib-api:withdraw", GetCommandExtension(ribRequest.Dir["withdraw"]))
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

// TestMetaCmdModule verifies ze-cli-meta-cmd.yang (introspection from cmd/meta plugin).
//
// VALIDATES: Meta command YANG module loads with help, command, event, plugin groups.
// PREVENTS: Introspection commands missing from the command tree.
func TestMetaCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"meta/schema/ze-cli-meta-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-meta-cmd")
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
		{"cache", cmdPluginBase + "cmd/cache/schema/ze-cli-cache-cmd.yang", "ze-cli-cache-cmd", "cache", "ze-bgp:cache"},
		{"commit", cmdPluginBase + "cmd/commit/schema/ze-cli-commit-cmd.yang", "ze-cli-commit-cmd", "commit", "ze-bgp:commit"},
		{"subscribe", cmdBase + "subscribe/schema/ze-cli-subscribe-cmd.yang", "ze-cli-subscribe-cmd", "subscribe", "ze-bgp:subscribe"},
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/commit/schema/ze-cli-commit-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-commit-cmd")
	require.NotNil(t, entry)
	assert.False(t, HasEditShortcutExtension(entry.Dir["commit"]), "ze-bgp:commit is NOT an edit shortcut")
}

// TestLogCmdModule verifies ze-cli-log-cmd.yang (log operations from cmd/log plugin).
//
// VALIDATES: Log command YANG module loads with log > levels and log > set nodes.
// PREVENTS: Log commands missing from the command tree.
func TestLogCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"log/schema/ze-cli-log-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-log-cmd")
	require.NotNil(t, entry)

	log := entry.Dir["log"]
	require.NotNil(t, log)
	assert.Equal(t, "ze-bgp:log-levels", GetCommandExtension(log.Dir["levels"]))
	assert.Equal(t, "ze-bgp:log-set", GetCommandExtension(log.Dir["set"]))
}

// TestMetricsCmdModule verifies ze-cli-metrics-cmd.yang (metrics operations from cmd/metrics plugin).
//
// VALIDATES: Metrics command YANG module loads with metrics > values and metrics > list nodes.
// PREVENTS: Metrics commands missing from the command tree.
func TestMetricsCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"metrics/schema/ze-cli-metrics-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-metrics-cmd")
	require.NotNil(t, entry)

	metrics := entry.Dir["metrics"]
	require.NotNil(t, metrics)
	assert.Equal(t, "ze-bgp:metrics-values", GetCommandExtension(metrics.Dir["values"]))
	assert.Equal(t, "ze-bgp:metrics-list", GetCommandExtension(metrics.Dir["list"]))
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

// TestCliUpdateCmdModule verifies ze-cli-update-cmd.yang (update verb from cmd/update).
//
// VALIDATES: Update verb YANG module loads with update > bgp > peer > prefix hierarchy.
// PREVENTS: Update verb missing from the command tree.
func TestCliUpdateCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"update/schema/ze-cli-update-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-update-cmd")
	require.NotNil(t, entry)

	update := entry.Dir["update"]
	require.NotNil(t, update, "update container must exist")
	assert.Equal(t, "", GetCommandExtension(update), "update is a grouping, no handler")

	bgp := update.Dir["bgp"]
	require.NotNil(t, bgp, "update > bgp must exist")

	peer := bgp.Dir["peer"]
	require.NotNil(t, peer, "update > bgp > peer must exist")

	prefix := peer.Dir["prefix"]
	require.NotNil(t, prefix, "update > bgp > peer > prefix must exist")
	assert.Equal(t, "ze-update:bgp-peer-prefix", GetCommandExtension(prefix))
}

// TestCliSetCmdModule verifies ze-cli-set-cmd.yang (set verb from cmd/set).
//
// VALIDATES: Set verb YANG module loads with set > bgp > peer > add/save hierarchy.
// PREVENTS: Set verb commands missing from the command tree.
func TestCliSetCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"set/schema/ze-cli-set-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-set-cmd")
	require.NotNil(t, entry)

	set := entry.Dir["set"]
	require.NotNil(t, set, "set container must exist")
	assert.Equal(t, "", GetCommandExtension(set), "set is a grouping, no handler")

	bgp := set.Dir["bgp"]
	require.NotNil(t, bgp, "set > bgp must exist")

	peer := bgp.Dir["peer"]
	require.NotNil(t, peer, "set > bgp > peer must exist")

	with := peer.Dir["with"]
	require.NotNil(t, with, "set > bgp > peer > with must exist")
	assert.Equal(t, "ze-set:bgp-peer-with", GetCommandExtension(with))

	save := peer.Dir["save"]
	require.NotNil(t, save, "set > bgp > peer > save must exist")
	assert.Equal(t, "ze-set:bgp-peer-save", GetCommandExtension(save))
}

// TestCliDelCmdModule verifies ze-cli-del-cmd.yang (del verb from cmd/del).
//
// VALIDATES: Del verb YANG module loads with del > bgp > peer hierarchy.
// PREVENTS: Del verb command missing from the command tree.
func TestCliDelCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"del/schema/ze-cli-del-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-del-cmd")
	require.NotNil(t, entry)

	del := entry.Dir["del"]
	require.NotNil(t, del, "del container must exist")
	assert.Equal(t, "", GetCommandExtension(del), "del is a grouping, no handler")

	bgp := del.Dir["bgp"]
	require.NotNil(t, bgp, "del > bgp must exist")

	peer := bgp.Dir["peer"]
	require.NotNil(t, peer, "del > bgp > peer must exist")
	assert.Equal(t, "ze-del:bgp-peer", GetCommandExtension(peer))
}

// TestBuildCommandTree verifies BuildCommandTree merges multiple -cmd modules into one command.Node tree.
//
// VALIDATES: Multiple YANG modules with overlapping containers merge correctly into command.Node.
// PREVENTS: Commands missing or duplicated when multiple plugins contribute to the same tree branch.
func TestBuildCommandTree(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)

	// Load 3 modules that all contribute to the "peer" container
	loadCmdModule(t, loader, cmdPluginBase+"cmd/peer/schema/ze-peer-cmd.yang")
	loadCmdModule(t, loader, cmdPluginBase+"cmd/raw/schema/ze-raw-cmd.yang")
	loadCmdModule(t, loader, cmdPluginBase+"route_refresh/schema/ze-refresh-cmd.yang")
	// Load a non-overlapping module
	loadCmdModule(t, loader, cmdPluginBase+"cmd/cache/schema/ze-cli-cache-cmd.yang")

	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)

	// "cache" from ze-cli-cache-cmd
	cache := tree.Children["cache"]
	require.NotNil(t, cache, "cache should exist")
	assert.Equal(t, "Manage cached BGP UPDATE messages.\nActions: list (show cached entries), retain (hold a message),\nrelease (free a retained message), expire (force eviction),\nforward (send a cached message to peers). Grammar: cache <action>\n<id> [args].", cache.Description)
	assert.Equal(t, "ze-bgp:cache", cache.WireMethod)

	// "peer" merged from 3 modules
	peer := tree.Children["peer"]
	require.NotNil(t, peer, "peer should exist (merged)")
	assert.Equal(t, "Peer lifecycle and flow control operations", peer.Description, "peer grouping gets YANG description")
	assert.Equal(t, "", peer.WireMethod, "peer grouping has no WireMethod")

	// From ze-peer-cmd -- verify WireMethod on merged leaves
	require.NotNil(t, peer.Children["list"], "peer.list from ze-peer-cmd")
	assert.Equal(t, "ze-bgp:peer-list", peer.Children["list"].WireMethod)
	assert.Nil(t, peer.Children["add"], "peer.add moved to set verb")

	// From ze-raw-cmd
	require.NotNil(t, peer.Children["raw"], "peer.raw from ze-raw-cmd")
	assert.Equal(t, "ze-bgp:peer-raw", peer.Children["raw"].WireMethod)

	// From ze-refresh-cmd
	require.NotNil(t, peer.Children["refresh"], "peer.refresh from ze-refresh-cmd")
	assert.Equal(t, "ze-bgp:peer-refresh", peer.Children["refresh"].WireMethod)
	assert.NotNil(t, peer.Children["borr"], "peer.borr from ze-refresh-cmd")

	// Deep merge: peer > clear > soft from ze-refresh-cmd
	clearNode := peer.Children["clear"]
	require.NotNil(t, clearNode, "peer.clear should exist")
	assert.Equal(t, "", clearNode.WireMethod, "peer.clear is grouping")
	require.NotNil(t, clearNode.Children["soft"], "peer.clear.soft from ze-refresh-cmd")
	assert.Equal(t, "ze-bgp:peer-clear-soft", clearNode.Children["soft"].WireMethod)

	// "summary" from ze-peer-cmd (top-level, not under peer)
	summary := tree.Children["summary"]
	require.NotNil(t, summary, "summary should exist")
	assert.Equal(t, "ze-bgp:summary", summary.WireMethod)
}

// TestBuildCommandTreeEmpty verifies BuildCommandTree handles no -cmd modules.
//
// VALIDATES: Empty tree returned when no command modules are loaded.
// PREVENTS: Nil panic when no plugins are registered.
func TestBuildCommandTreeEmpty(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)
	assert.Empty(t, tree.Children)
}

// TestBuildCommandTreeCommandNodes verifies only ze:command nodes become leaves with descriptions.
//
// VALIDATES: Grouping containers (no ze:command) have no description; command nodes have their YANG description.
// PREVENTS: Grouping nodes showing up as executable commands in completions.
func TestBuildCommandTreeCommandNodes(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdPluginBase+"cmd/rib/schema/ze-rib-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)

	show := tree.Children["show"]
	require.NotNil(t, show)
	bgp := show.Children["bgp"]
	require.NotNil(t, bgp)
	rib := bgp.Children["rib"]
	require.NotNil(t, rib)
	// show bgp rib is the routes command, owned by the BGP rib plugin schema.
	assert.Equal(t, "ze-rib-api:routes", rib.WireMethod, "show bgp rib is the BGP-owned routes command")
	assert.Contains(t, rib.Description, "Query routes in the BGP RIB", "show bgp rib has the routes description")

	rpf := rib.Children["rpf"]
	require.NotNil(t, rpf)
	assert.Equal(t, "ze-rib-api:rpf", rpf.WireMethod)
	assert.Equal(t, "ze-rib-api:best", rib.Children["best"].WireMethod)
	assert.Equal(t, "ze-rib-api:best-status", rib.Children["best"].Children["status"].WireMethod)
	assert.Equal(t, "ze-rib-api:status", rib.Children["status"].WireMethod)

	clear := tree.Children["clear"]
	require.NotNil(t, clear)
	clearRIB := clear.Children["bgp"].Children["rib"]
	require.NotNil(t, clearRIB)
	assert.Equal(t, "Clear and re-advertise RIB entries", clearRIB.Description, "clear bgp rib grouping gets YANG description")
	assert.Equal(t, "", clearRIB.WireMethod, "clear bgp rib grouping has no WireMethod")
	assert.Equal(t, "ze-rib-api:clear-in", clearRIB.Children["in"].WireMethod)
	assert.Equal(t, "ze-rib-api:clear-out", clearRIB.Children["out"].WireMethod)

	request := tree.Children["request"]
	require.NotNil(t, request)
	requestRIB := request.Children["bgp"].Children["rib"]
	require.NotNil(t, requestRIB)
	assert.Equal(t, "ze-rib-api:inject", requestRIB.Children["inject"].WireMethod)
	assert.Equal(t, "ze-rib-api:withdraw", requestRIB.Children["withdraw"].WireMethod)
	// Verify command package is used (tree is *command.Node from BuildCommandTree)
	require.IsType(t, &command.Node{}, tree)
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
	assert.Equal(t, "ze-system:daemon-quit", GetCommandExtension(daemon.Dir["quit"]))
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

// TestGetBackendExtension_Absent verifies nil return when no ze:backend is present.
//
// VALIDATES: AC-8 — nil for entries without ze:backend.
// PREVENTS: false-positive backend restrictions on unannotated entries.
func TestGetBackendExtension_Absent(t *testing.T) {
	assert.Nil(t, GetBackendExtension(nil))

	entry := &gyang.Entry{Name: "test"}
	assert.Nil(t, GetBackendExtension(entry))
}

// TestGetBackendExtension_Single verifies single backend name extraction.
//
// VALIDATES: AC-9 — ["netlink"] for ze:backend "netlink".
// PREVENTS: backend annotation silently ignored.
func TestGetBackendExtension_Single(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-backend {
    namespace "urn:test:backend";
    prefix tb;
    import ze-extensions { prefix ze; }

    container tunnel {
        config false;
        ze:backend "netlink";
        ze:command "ze-test:tunnel";
        description "Tunnel operations";
    }
}
`
	err = loader.AddModuleFromText("test-backend.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("test-backend")
	require.NotNil(t, entry)

	tunnel := entry.Dir["tunnel"]
	require.NotNil(t, tunnel)
	assert.Equal(t, []string{"netlink"}, GetBackendExtension(tunnel))
}

// TestGetBackendExtension_Multi verifies multi-backend and deduplication.
//
// VALIDATES: AC-10 — ["netlink","vpp"] for ze:backend "netlink vpp", deduplicated.
// PREVENTS: duplicate backend entries or dropped backends.
func TestGetBackendExtension_Multi(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-backend-multi {
    namespace "urn:test:backend:multi";
    prefix tbm;
    import ze-extensions { prefix ze; }

    container shared {
        config false;
        ze:backend "netlink vpp";
        ze:command "ze-test:shared";
        description "Shared across backends";
    }

    container duped {
        config false;
        ze:backend "netlink netlink vpp";
        ze:command "ze-test:duped";
        description "Has duplicates";
    }
}
`
	err = loader.AddModuleFromText("test-backend-multi.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("test-backend-multi")
	require.NotNil(t, entry)

	shared := entry.Dir["shared"]
	require.NotNil(t, shared)
	assert.Equal(t, []string{"netlink", "vpp"}, GetBackendExtension(shared))

	duped := entry.Dir["duped"]
	require.NotNil(t, duped)
	assert.Equal(t, []string{"netlink", "vpp"}, GetBackendExtension(duped))
}

// TestBuildCommandTreeBackend verifies mergeYANGEntry stores Backend from ze:backend.
//
// VALIDATES: mergeYANGEntry reads ze:backend and populates command.Node.Backend.
// PREVENTS: command tree missing backend info for filtering.
func TestBuildCommandTreeBackend(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)

	yangText := `
module test-backend-tree-cmd {
    namespace "urn:test:backend:tree:cmd";
    prefix tbtc;
    import ze-extensions { prefix ze; }

    container vpp {
        config false;
        ze:backend "vpp";
        description "VPP operations";

        container trace {
            config false;
            ze:command "ze-test:vpp-trace";
            description "VPP trace";
        }
    }

    container general {
        config false;
        ze:command "ze-test:general";
        description "Works on all backends";
    }
}
`
	err = loader.AddModuleFromText("test-backend-tree-cmd.yang", yangText)
	require.NoError(t, err)
	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)

	vpp := tree.Children["vpp"]
	require.NotNil(t, vpp)
	assert.Equal(t, []string{"vpp"}, vpp.Backend)

	// Child without its own annotation does not inherit (narrowest-wins: parent annotated, child nil)
	trace := vpp.Children["trace"]
	require.NotNil(t, trace)
	assert.Nil(t, trace.Backend)

	general := tree.Children["general"]
	require.NotNil(t, general)
	assert.Nil(t, general.Backend)
}

// TestArgDefFromEnumYANG verifies that enum leaves inside ze:command containers
// produce ArgDef entries with EnumValues populated.
//
// VALIDATES: AC-1 -- enum leaf produces ArgDef with EnumValues.
func TestArgDefFromEnumYANG(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	yangText := `
module test-enum-cmd {
    namespace "urn:test:enum:cmd";
    prefix tec;
    import ze-extensions { prefix ze; }

    container show {
        config false;
        container goroutines {
            config false;
            ze:command "ze-show:system-goroutines";
            description "Show goroutines";
            leaf mode {
                type enumeration {
                    enum summary;
                    enum blocked;
                    enum full;
                }
                description "Display mode";
            }
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("test-enum-cmd.yang", yangText))
	require.NoError(t, loader.Resolve())

	tree := BuildCommandTree(loader)
	goroutines := tree.Children["show"].Children["goroutines"]
	require.NotNil(t, goroutines)
	require.Len(t, goroutines.ArgDefs, 1)

	def := goroutines.ArgDefs[0]
	assert.Equal(t, "mode", def.Name)
	assert.Equal(t, command.ArgEnum, def.Kind)
	assert.Equal(t, []string{"blocked", "full", "summary"}, def.EnumValues)
}

// TestArgDefFromUnionYANG verifies that union(uint64, enum) leaves produce
// ArgDef with Kind=ArgUnion and enum values extracted.
//
// VALIDATES: AC-2 -- union leaf produces ArgDef with extracted enum values.
func TestArgDefFromUnionYANG(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	yangText := `
module test-union-cmd {
    namespace "urn:test:union:cmd";
    prefix tuc;
    import ze-extensions { prefix ze; }

    container set {
        config false;
        container file-descriptors {
            config false;
            ze:command "ze-set:system-file-descriptors";
            description "Set FD limit";
            leaf limit {
                type union {
                    type uint64;
                    type enumeration {
                        enum max;
                    }
                }
                description "New limit or max";
            }
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("test-union-cmd.yang", yangText))
	require.NoError(t, loader.Resolve())

	tree := BuildCommandTree(loader)
	fd := tree.Children["set"].Children["file-descriptors"]
	require.NotNil(t, fd)
	require.Len(t, fd.ArgDefs, 1)

	def := fd.ArgDefs[0]
	assert.Equal(t, "limit", def.Name)
	assert.Equal(t, command.ArgUnion, def.Kind)
	assert.Equal(t, []string{"max"}, def.EnumValues)
	require.Len(t, def.UnionDefs, 2)
	assert.Equal(t, command.ArgUint, def.UnionDefs[0].Kind)
	assert.Equal(t, command.ArgEnum, def.UnionDefs[1].Kind)
}

// TestArgDefFromUintRangeYANG verifies uint leaves with range constraints.
//
// VALIDATES: AC-3 -- uint leaf with range produces ArgDef with range metadata.
func TestArgDefFromUintRangeYANG(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	yangText := `
module test-uint-cmd {
    namespace "urn:test:uint:cmd";
    prefix turc;
    import ze-extensions { prefix ze; }

    container show {
        config false;
        container capture {
            config false;
            ze:command "ze-show:capture";
            description "Capture packets";
            leaf count {
                type uint32 {
                    range "1..10000";
                }
                description "Packet count";
            }
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("test-uint-cmd.yang", yangText))
	require.NoError(t, loader.Resolve())

	tree := BuildCommandTree(loader)
	capture := tree.Children["show"].Children["capture"]
	require.NotNil(t, capture)
	require.Len(t, capture.ArgDefs, 1)

	def := capture.ArgDefs[0]
	assert.Equal(t, "count", def.Name)
	assert.Equal(t, command.ArgUint, def.Kind)
	assert.Equal(t, 32, def.UintBits)
	require.Len(t, def.Ranges, 1)
	assert.Equal(t, uint64(1), def.Ranges[0].Min)
	assert.Equal(t, uint64(10000), def.Ranges[0].Max)
}

// TestArgDefFromPatternYANG verifies string leaves with pattern constraints.
//
// VALIDATES: AC-4 -- string leaf with pattern produces ArgDef with Pattern.
func TestArgDefFromPatternYANG(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	yangText := `
module test-pattern-cmd {
    namespace "urn:test:pattern:cmd";
    prefix tpc;
    import ze-extensions { prefix ze; }

    container show {
        config false;
        container ping {
            config false;
            ze:command "ze-show:ping";
            description "Ping host";
            leaf timeout {
                type string {
                    pattern '\d+[smh]?';
                }
                description "Timeout duration";
            }
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("test-pattern-cmd.yang", yangText))
	require.NoError(t, loader.Resolve())

	tree := BuildCommandTree(loader)
	ping := tree.Children["show"].Children["ping"]
	require.NotNil(t, ping)
	require.Len(t, ping.ArgDefs, 1)

	def := ping.ArgDefs[0]
	assert.Equal(t, "timeout", def.Name)
	assert.Equal(t, command.ArgString, def.Kind)
	require.NotNil(t, def.Pattern)
	assert.True(t, def.Pattern.MatchString("30s"))
	assert.False(t, def.Pattern.MatchString("abc"))
}

// TestArgDefsPopulated verifies that all commands with typed arguments have
// ArgDefs populated after BuildCommandTree using real YANG schemas.
//
// VALIDATES: AC-18 -- all commands with typed args have ArgDefs after BuildCommandTree.
func TestArgDefsPopulated(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	// Load all real -cmd YANG modules. The show ip/neighbors/kernel-routes
	// subtree is owned by the iface component (it reads the kernel tables
	// through the iface backend), and the show ping node is owned by the
	// dedicated ping feature module, so those command modules live next to
	// their owners rather than in the central show schema.
	cmdFiles := []string{
		cmdBase + "show/schema/ze-cli-show-cmd.yang",
		cmdBase + "set/schema/ze-cli-set-cmd.yang",
		cmdBase + "log/schema/ze-cli-log-cmd.yang",
		"../../../component/iface/schema/ze-iface-show-cmd.yang",
		"../../../component/ping/schema/ze-ping-cmd.yang",
	}
	for _, path := range cmdFiles {
		loadCmdModule(t, loader, path)
	}
	require.NoError(t, loader.Resolve())

	tree := BuildCommandTree(loader)

	// Commands that should have ArgDefs (from the Typed Argument Catalog).
	wantArgDefs := map[string]int{
		"show system goroutines":       1, // mode
		"show system file-descriptors": 1, // mode
		"show system sockets":          3, // protocol, state, port
		"show system kernel-log":       2, // level, count
		"show system profile":          2, // type, duration
		"show audit":                   6, // action, actor, surface, since, until, count
		"show ping":                    3, // dest, count, timeout
		"show traceroute":              4, // dest, max-hops, timeout, probes
		"show tcp-check":               4, // host, port, source, timeout
		"show probe-round":             4, // dest, probes, max-hops, timeout
		"show dns lookup":              2, // hostname, type
		"show dns cache":               2, // action, name
		"show capture":                 4, // protocol, tunnel-id, count, peer
		"show capture raw":             4, // action, protocol, format, count
		"show capture interface":       6, // iface, count, duration, snap-len, format, protocol
		"show ip arp":                  1, // family
		"show ip route":                2, // prefix, limit
		"show neighbors":               1, // family
		"show crashes":                 1, // name
		"set system file-descriptors":  1, // limit
		"log set":                      2, // logger, level
		"log recent":                   3, // level, component, count
	}

	for path, wantCount := range wantArgDefs {
		node := navigateTree(tree, path)
		if node == nil {
			t.Errorf("command %q not found in tree", path)
			continue
		}
		if len(node.ArgDefs) != wantCount {
			t.Errorf("command %q: want %d ArgDefs, got %d", path, wantCount, len(node.ArgDefs))
		}
	}
}

// navigateTree walks a command tree by space-separated path.
func navigateTree(root *command.Node, path string) *command.Node {
	current := root
	for name := range strings.FieldsSeq(path) {
		if current == nil || current.Children == nil {
			return nil
		}
		child, ok := current.Children[name]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}
