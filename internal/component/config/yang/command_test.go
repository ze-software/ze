package yang

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"

	gyang "github.com/openconfig/goyang/pkg/yang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
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
	assert.True(t, hasCommandExtension(listEntry))

	addEntry := peerEntry.Dir["add"]
	require.NotNil(t, addEntry)
	assert.Equal(t, "ze-bgp:peer-add", GetCommandExtension(addEntry))
	assert.NotNil(t, addEntry.Dir["address"], "add should have address leaf child")

	statusEntry := peerEntry.Dir["status"]
	require.NotNil(t, statusEntry)
	assert.False(t, hasCommandExtension(statusEntry), "status has no ze:command")
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

    container overview {
        config false;
        ze:command "ze-bgp:overview";
        description "Show an overview (not an edit shortcut)";
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
	assert.True(t, hasEditShortcutExtension(commitEntry), "commit should have ze:edit-shortcut")

	saveEntry := entry.Dir["save"]
	require.NotNil(t, saveEntry)
	assert.True(t, hasEditShortcutExtension(saveEntry), "save should have ze:edit-shortcut")

	overviewEntry := entry.Dir["overview"]
	require.NotNil(t, overviewEntry)
	assert.Equal(t, "ze-bgp:overview", GetCommandExtension(overviewEntry))
	assert.False(t, hasEditShortcutExtension(overviewEntry), "overview should NOT have ze:edit-shortcut")
}

// TestExtensionNilEntry verifies extension functions handle nil safely.
//
// VALIDATES: No panic on nil entry.
// PREVENTS: NPE in tree walker when entry is nil.
func TestExtensionNilEntry(t *testing.T) {
	assert.Equal(t, "", GetCommandExtension(nil))
	assert.False(t, hasCommandExtension(nil))
	assert.False(t, hasEditShortcutExtension(nil))
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/peer/yang/ze-peer-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-peer-cmd")
	require.NotNil(t, entry)

	show := entry.Dir["show"]
	require.NotNil(t, show)

	bgp := show.Dir["bgp"]
	require.NotNil(t, bgp)

	assert.Equal(t, "ze-bgp:overview", GetCommandExtension(bgp), "`show bgp` carries the command itself")
	assert.Nil(t, bgp.Dir["summary"], "the `summary` container is retired")

	showPeer := bgp.Dir["peer"]
	require.NotNil(t, showPeer)
	assert.Equal(t, "ze-bgp:peer-list", GetCommandExtension(showPeer.Dir["list"]))
	assert.Equal(t, "ze-bgp:peer-detail", GetCommandExtension(showPeer.Dir["detail"]))
	assert.Equal(t, "ze-bgp:peer-capabilities", GetCommandExtension(showPeer.Dir["capabilities"]))
	assert.Equal(t, "ze-bgp:peer-statistics", GetCommandExtension(showPeer.Dir["statistics"]))
	assert.Equal(t, "ze-bgp:peer-history", GetCommandExtension(showPeer.Dir["history"]))
	assert.Equal(t, "ze-bgp:peer-rib", GetCommandExtension(showPeer.Dir["rib"]))

	request := entry.Dir["request"]
	require.NotNil(t, request)
	peer := request.Dir["peer"]
	require.NotNil(t, peer)
	assert.Equal(t, "", GetCommandExtension(peer), "peer grouping has no handler")
	assert.Equal(t, gyang.TSFalse, peer.Config)
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/rib/yang/ze-rib-cmd.yang")
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
	loadCmdModule(t, loader, cmdPluginBase+"route_refresh/yang/ze-refresh-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-refresh-cmd")
	require.NotNil(t, entry)

	request := entry.Dir["request"]
	require.NotNil(t, request)
	peer := request.Dir["peer"]
	require.NotNil(t, peer)
	assert.Equal(t, "ze-bgp:peer-refresh", GetCommandExtension(peer.Dir["refresh"]))
	assert.Equal(t, "ze-bgp:peer-borr", GetCommandExtension(peer.Dir["borr"]))
	assert.Equal(t, "ze-bgp:peer-eorr", GetCommandExtension(peer.Dir["eorr"]))
	assert.Equal(t, "ze-bgp:peer-clear-soft", GetCommandExtension(peer.Dir["clear"].Dir["soft"]))
}

// TestMetaCmdModule verifies ze-command-meta-cmd.yang (introspection from command component).
//
// VALIDATES: Meta command YANG module loads with help, command, event, plugin groups.
// PREVENTS: Introspection commands missing from the command tree.
func TestMetaCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, "../../../plugins/meta/yang/ze-command-meta-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-command-meta-cmd")
	require.NotNil(t, entry)

	assert.Equal(t, "ze-bgp:help", GetCommandExtension(entry.Dir["help"]))
	assert.Equal(t, "ze-bgp:command-list", GetCommandExtension(entry.Dir["show"].Dir["command"].Dir["list"]))
	assert.Equal(t, "ze-bgp:event-list", GetCommandExtension(entry.Dir["show"].Dir["event"].Dir["list"]))
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
		{"commit", cmdPluginBase + "cmd/commit/yang/ze-cli-commit-cmd.yang", "ze-cli-commit-cmd", "request/commit", "ze-bgp:commit"},
		{"subscribe", cmdBase + "subscribe/yang/ze-cli-subscribe-cmd.yang", "ze-cli-subscribe-cmd", "request/subscribe", "ze-bgp:subscribe"},
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
			node := entry
			for seg := range strings.SplitSeq(tt.container, "/") {
				require.NotNil(t, node.Dir[seg], "missing segment %q in path %q", seg, tt.container)
				node = node.Dir[seg]
			}
			assert.Equal(t, tt.wireMethod, GetCommandExtension(node))
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/commit/yang/ze-cli-commit-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-commit-cmd")
	require.NotNil(t, entry)
	assert.False(t, hasEditShortcutExtension(entry.Dir["commit"]), "ze-bgp:commit is NOT an edit shortcut")
}

// TestLogCmdModule verifies the bare anchor and relocated log command module.
//
// VALIDATES: Central cmd/log schema is anchor-only and log commands live in ze-log-cmd.
// PREVENTS: Plugin self-containment regressions for log commands.
func TestLogCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"log/yang/ze-cli-log-cmd.yang")
	loadCmdModule(t, loader, "../../../plugins/log/yang/ze-log-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	anchor := loader.GetEntry("ze-cli-log-cmd")
	require.NotNil(t, anchor)
	assert.Empty(t, anchor.Dir, "ze-cli-log-cmd is a bare anchor after relocation")

	entry := loader.GetEntry("ze-log-cmd")
	require.NotNil(t, entry)

	show := entry.Dir["show"]
	require.NotNil(t, show)
	showLog := show.Dir["log"]
	require.NotNil(t, showLog)
	assert.Equal(t, "ze-bgp:log-levels", GetCommandExtension(showLog.Dir["levels"]))
	assert.Equal(t, "ze-bgp:log-recent", GetCommandExtension(showLog.Dir["recent"]))

	req := entry.Dir["request"]
	require.NotNil(t, req)
	reqLog := req.Dir["log"]
	require.NotNil(t, reqLog)
	assert.Equal(t, "ze-bgp:log-set", GetCommandExtension(reqLog.Dir["level"]))
}

// TestMetricsCmdModule verifies ze-cli-metrics-cmd.yang (metrics operations from cmd/metrics plugin).
//
// VALIDATES: Metrics command YANG module loads with metrics > values and metrics > list nodes.
// PREVENTS: Metrics commands missing from the command tree.
func TestMetricsCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"metrics/yang/ze-cli-metrics-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-metrics-cmd")
	require.NotNil(t, entry)

	showM := entry.Dir["show"]
	require.NotNil(t, showM)
	metrics := showM.Dir["metrics"]
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/raw/yang/ze-raw-cmd.yang")
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/update/yang/ze-update-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-update-cmd")
	require.NotNil(t, entry)
	assert.Equal(t, "ze-bgp:peer-update", GetCommandExtension(entry.Dir["peer"].Dir["update"]))
}

// TestCliUpdateCmdModule verifies ze-cli-update-cmd.yang (update verb from cmd/update).
//
// VALIDATES: Update verb YANG module loads with update > system hierarchy (bgp peer removed).
// PREVENTS: Update verb missing from the command tree.
func TestCliUpdateCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"update/yang/ze-cli-update-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-update-cmd")
	require.NotNil(t, entry)

	update := entry.Dir["update"]
	require.NotNil(t, update, "update container must exist")
	assert.Equal(t, "", GetCommandExtension(update), "update is a grouping, no handler")

	assert.Nil(t, update.Dir["bgp"], "update > bgp must not exist (update bgp peer prefix removed)")
	assert.Nil(t, update.Dir["system"], "update anchor is bare after firmware command relocation")
}

// TestCliSetCmdModule verifies ze-cli-set-cmd.yang (set verb from cmd/set).
//
// VALIDATES: Set verb YANG module loads with set > system hierarchy (bgp peer removed).
// PREVENTS: Set verb missing from the command tree.
func TestCliSetCmdModule(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, cmdBase+"set/yang/ze-cli-set-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	entry := loader.GetEntry("ze-cli-set-cmd")
	require.NotNil(t, entry)

	set := entry.Dir["set"]
	require.NotNil(t, set, "set container must exist")
	assert.Equal(t, "", GetCommandExtension(set), "set is a grouping, no handler")

	assert.Nil(t, set.Dir["bgp"], "set > bgp must not exist (set bgp peer with/save removed)")
	assert.Nil(t, set.Dir["system"], "set anchor is bare after host command relocation")
}

// TestPeerCmdModuleOwnsDeleteBgpPeer verifies the BGP peer command owner declares
// delete > bgp > peer (ze-delete:bgp-peer), relocated out of the central delete
// schema, which is now a bare verb-root anchor.
func TestPeerCmdModuleOwnsDeleteBgpPeer(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	loadCmdModule(t, loader, "../../../component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang")
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-peer-cmd")
	require.NotNil(t, entry)

	peer := entry.Dir["delete"].Dir["bgp"].Dir["peer"]
	require.NotNil(t, peer, "delete > bgp > peer must exist in the peer owner module")
	assert.Equal(t, "ze-delete:bgp-peer", GetCommandExtension(peer))
}

// TestCliDeleteCmdModule verifies the central delete verb module is a bare
// verb-root anchor: the `delete` container exists with no handler and no bgp
// subtree (delete bgp peer moved to the BGP peer command owner).
func TestCliDeleteCmdModule(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	loadCmdModule(t, loader, cmdBase+"delete/yang/ze-cli-delete-cmd.yang")
	require.NoError(t, loader.Resolve())

	entry := loader.GetEntry("ze-cli-delete-cmd")
	require.NotNil(t, entry)

	deleteRoot := entry.Dir["delete"]
	require.NotNil(t, deleteRoot, "delete container must exist")
	assert.Equal(t, "", GetCommandExtension(deleteRoot), "delete is a bare verb-root anchor, no handler")
	assert.Nil(t, deleteRoot.Dir["bgp"], "delete > bgp must not exist in the central anchor (owned by the peer module)")
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/peer/yang/ze-peer-cmd.yang")
	loadCmdModule(t, loader, cmdPluginBase+"cmd/raw/yang/ze-raw-cmd.yang")
	loadCmdModule(t, loader, cmdPluginBase+"route_refresh/yang/ze-refresh-cmd.yang")
	// Load a non-overlapping module
	loadCmdModule(t, loader, cmdPluginBase+"cmd/cache/yang/ze-cli-cache-cmd.yang")

	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)

	// "cache" moved under show and request verbs from ze-cli-cache-cmd
	assert.Nil(t, tree.Children["cache"])
	showCache := tree.Children["show"].Children["cache"]
	require.NotNil(t, showCache, "show > cache should exist")
	assert.Equal(t, "ze-bgp:cache-list", showCache.WireMethod)

	// "peer" at top level from ze-raw-cmd
	peer := tree.Children["peer"]
	require.NotNil(t, peer, "peer should exist (from ze-raw-cmd)")
	require.NotNil(t, peer.Children["raw"], "peer.raw from ze-raw-cmd")
	assert.Equal(t, "ze-bgp:peer-raw", peer.Children["raw"].WireMethod)

	// Lifecycle actions moved under request > peer
	request := tree.Children["request"]
	require.NotNil(t, request, "request should exist")
	reqPeer := request.Children["peer"]
	require.NotNil(t, reqPeer, "request > peer should exist")

	// From ze-refresh-cmd
	require.NotNil(t, reqPeer.Children["refresh"], "request.peer.refresh from ze-refresh-cmd")
	assert.Equal(t, "ze-bgp:peer-refresh", reqPeer.Children["refresh"].WireMethod)
	assert.NotNil(t, reqPeer.Children["borr"], "request.peer.borr from ze-refresh-cmd")

	// Deep merge: request > peer > clear > soft from ze-refresh-cmd
	clearNode := reqPeer.Children["clear"]
	require.NotNil(t, clearNode, "request.peer.clear should exist")
	assert.Equal(t, "", clearNode.WireMethod, "request.peer.clear is grouping")
	require.NotNil(t, clearNode.Children["soft"], "request.peer.clear.soft from ze-refresh-cmd")
	assert.Equal(t, "ze-bgp:peer-clear-soft", clearNode.Children["soft"].WireMethod)

	assert.Nil(t, tree.Children["summary"])
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
	loadCmdModule(t, loader, cmdPluginBase+"cmd/rib/yang/ze-rib-cmd.yang")
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
	loadCmdModule(t, loader, "../../../core/ipc/yang/ze-system-cmd.yang")
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

	// process status under show, lifecycle actions under request
	show := entry.Dir["show"]
	require.NotNil(t, show)
	assert.Equal(t, "ze-system:daemon-status", GetCommandExtension(show.Dir["status"]))

	request := entry.Dir["request"]
	require.NotNil(t, request)
	assert.Equal(t, "ze-system:daemon-shutdown", GetCommandExtension(request.Dir["shutdown"]))
	assert.Equal(t, "ze-system:daemon-quit", GetCommandExtension(request.Dir["halt"]))
	assert.Equal(t, "ze-system:daemon-reload", GetCommandExtension(request.Dir["reload"]))
}

// TestPluginCmdModuleLoads verifies ze-plugin-cmd.yang loads and has expected structure.
//
// VALIDATES: Plugin command YANG module loads with correct hierarchy and WireMethod refs.
// PREVENTS: Plugin commands missing from the unified tree.
func TestPluginCmdModuleLoads(t *testing.T) {
	loader := NewLoader()

	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, "../../../core/ipc/yang/ze-plugin-cmd.yang")
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
// VALIDATES: AC-1 -- enum leaf produces ArgDef with EnumValues, in the order
// the module declares them.
// PREVENTS: a generated usage line stating a value set in an order no module
// chose. `[import|export]` is the order handleShowPolicyChain
// (internal/component/bgp/plugins/cmd/policy/handler.go) documents and the
// alphabet inverts it.
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
	assert.Equal(t, []string{"summary", "blocked", "full"}, def.EnumValues)
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

	// Load all real -cmd YANG modules. The show route/neighbor/arp subtree
	// is owned by the iface component (it reads the kernel tables through the
	// iface backend), the show ping node is owned by the dedicated
	// ping feature module, the show traceroute / show probe-round nodes are
	// owned by the dedicated traceroute feature module, and the show dns
	// lookup/cache nodes are owned by the resolve component, so those command
	cmdFiles := []string{
		cmdBase + "show/yang/ze-cli-show-cmd.yang",
		cmdBase + "set/yang/ze-cli-set-cmd.yang",
		cmdBase + "log/yang/ze-cli-log-cmd.yang",
		"../../../component/iface/yang/ze-iface-show-cmd.yang",
		"../../../plugins/crashes/yang/ze-crashes-cmd.yang",
		"../../../plugins/diag/yang/ze-diag-cmd.yang",
		"../../../plugins/host-cmd/yang/ze-host-cmd.yang",
		"../../../plugins/host-cmd/yang/ze-host-set-cmd.yang",
		"../../../plugins/log/yang/ze-log-cmd.yang",
		"../../../plugins/ping-cmd/yang/ze-ping-cmd.yang",
		"../../../plugins/resolve-cmd/yang/ze-resolve-cmd.yang",
		"../../../plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang",
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
		"show ping":                    4, // dest, count, size, timeout
		"show traceroute":              4, // dest, max-hops, timeout, probes
		"show tcp-check":               4, // host, port, source, timeout
		"show probe-round":             4, // dest, probes, max-hops, timeout
		"show dns lookup":              2, // hostname, type
		"show dns cache record":        1, // name
		"clear dns cache record":       2, // name, type
		"show capture":                 4, // protocol, tunnel-id, count, peer
		"show capture raw":             4, // action, protocol, format, count
		"show capture interface":       6, // iface, count, duration, snap-len, format, protocol
		"show route":                   2, // prefix, limit
		"show neighbor":                1, // family
		"show crashes":                 1, // name
		"set system file-descriptors":  1, // limit
		"request log level":            2, // logger, level
		"show log recent":              3, // level, component, count
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

// TestBuildCommandTreeEnsureExists verifies ze:ensure-exists is extracted from the iface YANG.
//
// VALIDATES: BuildCommandTree propagates EnsureExists (rollback wire method) to command.Node,
// on the typed `name` selector node that carries the executable create command
// (create interface dummy name <name>), per cli.md R6.
// PREVENTS: Compound commands silently losing auto-ensure behavior.
func TestBuildCommandTreeEnsureExists(t *testing.T) {
	loader := NewLoader()
	err := loader.LoadEmbedded()
	require.NoError(t, err)
	loadCmdModule(t, loader, "../../../component/iface/yang/ze-iface-cmd.yang")
	err = loader.Resolve()
	require.NoError(t, err)

	tree := BuildCommandTree(loader)
	require.NotNil(t, tree)

	create := tree.Children["create"]
	require.NotNil(t, create)
	ifc := create.Children["interface"]
	require.NotNil(t, ifc)

	// Typed name selector (cli.md, R6): the executable create command lives
	// on the `name` selector node (`create interface dummy name <name>`), not on the
	// `dummy` grouping container. The ensure-exists rollback handler travels with it.
	dummy := ifc.Children["dummy"]
	require.NotNil(t, dummy, "dummy container must exist")
	dummyName := dummy.Children["name"]
	require.NotNil(t, dummyName, "dummy > name selector must exist")
	assert.Equal(t, "ze-iface:interface-delete", dummyName.EnsureExists, "dummy name must have ze:ensure-exists with rollback handler")
	assert.Equal(t, "ze-iface:interface-create-dummy", dummyName.WireMethod)

	bridge := ifc.Children["bridge"]
	require.NotNil(t, bridge, "bridge container must exist")
	bridgeName := bridge.Children["name"]
	require.NotNil(t, bridgeName, "bridge > name selector must exist")
	assert.Equal(t, "ze-iface:interface-delete", bridgeName.EnsureExists, "bridge name must have ze:ensure-exists with rollback handler")

	// Nested unit under the dummy name-selector carries its own wire method and has
	// no ensure-exists of its own (the rollback handler lives on the create node).
	dummyUnit := dummyName.Children["unit"]
	require.NotNil(t, dummyUnit, "dummy > name > unit must exist")
	assert.Equal(t, "ze-iface:interface-unit-add", dummyUnit.WireMethod)
	assert.Empty(t, dummyUnit.EnsureExists, "unit itself has no ensure-exists")

	// Flat unit (sibling of dummy) has no ensure-exists
	flatUnit := ifc.Children["unit"]
	require.NotNil(t, flatUnit, "flat unit must exist")
	assert.Empty(t, flatUnit.EnsureExists, "flat unit has no ensure-exists")

	// veth's create command exists but carries no ensure-exists (unlike dummy/bridge)
	veth := ifc.Children["veth"]
	require.NotNil(t, veth)
	vethName := veth.Children["name"]
	require.NotNil(t, vethName, "veth > name selector must exist")
	assert.Equal(t, "ze-iface:interface-create-veth", vethName.WireMethod)
	assert.Empty(t, vethName.EnsureExists, "veth has no ensure-exists")

	// Nested compound commands inherit backend restriction from parent
	assert.Equal(t, []string{"netlink"}, dummyUnit.Backend, "dummy > name > unit must have netlink backend")
	dummyAddr := dummyName.Children["address"]
	require.NotNil(t, dummyAddr, "dummy > name > address must exist")
	assert.Equal(t, []string{"netlink"}, dummyAddr.Backend, "dummy > name > address must have netlink backend")

	bridgeUnit := bridgeName.Children["unit"]
	require.NotNil(t, bridgeUnit, "bridge > name > unit must exist")
	assert.Equal(t, []string{"netlink"}, bridgeUnit.Backend, "bridge > name > unit must have netlink backend")
	bridgeAddr := bridgeName.Children["address"]
	require.NotNil(t, bridgeAddr, "bridge > name > address must exist")
	assert.Equal(t, []string{"netlink"}, bridgeAddr.Backend, "bridge > name > address must have netlink backend")
}

func TestValidateCommandTreeWarnsMissingDescription(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(old)

	root := &command.Node{Children: map[string]*command.Node{
		"show":    {Name: "show", Description: "Has description"},
		"request": {Name: "request"},
	}}

	validateCommandTree(root)

	assert.NotContains(t, buf.String(), "path=show", "described nodes produce no warning")
	assert.Contains(t, buf.String(), "YANG command node missing description")
	assert.Contains(t, buf.String(), "path=request")
}

func TestMergeYANGEntryWarnsOnDescriptionMismatch(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(old)

	root := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Description: "First description"},
	}}

	entry := &gyang.Entry{
		Dir: map[string]*gyang.Entry{
			"show": {
				Name:        "show",
				Description: "Different description",
				Config:      gyang.TSFalse,
			},
		},
	}

	mergeYANGEntry(root, entry)

	assert.Contains(t, buf.String(), "YANG command description mismatch")
	assert.Contains(t, buf.String(), "First description")
	assert.Contains(t, buf.String(), "Different description")
	assert.Equal(t, "First description", root.Children["show"].Description, "first description wins")
}

func TestMergeYANGEntrySilentOnMatchingDescription(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(old)

	root := &command.Node{Children: map[string]*command.Node{
		"show": {Name: "show", Description: "Same description"},
	}}

	entry := &gyang.Entry{
		Dir: map[string]*gyang.Entry{
			"show": {
				Name:        "show",
				Description: "Same description",
				Config:      gyang.TSFalse,
			},
		},
	}

	mergeYANGEntry(root, entry)

	assert.Empty(t, buf.String(), "no warning when descriptions match")
}

// declarationOrderModule declares four leaves in an order the alphabet inverts,
// so a sort on name cannot be mistaken for the declared order.
const declarationOrderModule = `
module test-order-cmd {
    namespace "urn:test:order:cmd";
    prefix toc;
    import ze-extensions { prefix ze; }

    container request {
        config false;
        container outgoing-call {
            config false;
            ze:command "ze-test:outgoing-call";
            description "Place a call.";
            leaf remote { type string; mandatory true; description "Remote name"; }
            leaf called { type string; mandatory true; description "Called number"; }
            leaf zone { type string; description "Zone"; }
            leaf attempts { type uint8; description "Attempts"; }
        }
    }
}
`

// VALIDATES: an argument definition list follows the order the module declares,
// not the order the alphabet imposes.
// PREVENTS: `request outgoing-call called <called> remote <remote>`, which is
// what an alphabetical sort renders and what no operator reads in a document.
func TestArgDefsFollowDeclarationOrder(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())
	require.NoError(t, loader.AddModuleFromText("test-order-cmd.yang", declarationOrderModule))
	require.NoError(t, loader.Resolve())

	node := BuildCommandTree(loader).Children["request"].Children["outgoing-call"]
	require.NotNil(t, node)

	names := make([]string, 0, len(node.ArgDefs))
	for _, def := range node.ArgDefs {
		names = append(names, def.Name)
	}
	assert.Equal(t, []string{"remote", "called", "zone", "attempts"}, names)
}

// VALIDATES: repeated builds of one module produce one order.
// PREVENTS: the entry directory being a Go map making the published grammar,
// the help line and the catalog differ between two runs of the same binary.
func TestArgDefsAreDeterministic(t *testing.T) {
	var first []string
	for run := range 12 {
		loader := NewLoader()
		require.NoError(t, loader.LoadEmbedded())
		require.NoError(t, loader.AddModuleFromText("test-order-cmd.yang", declarationOrderModule))
		require.NoError(t, loader.Resolve())

		node := BuildCommandTree(loader).Children["request"].Children["outgoing-call"]
		require.NotNil(t, node)
		names := make([]string, 0, len(node.ArgDefs))
		for _, def := range node.ArgDefs {
			names = append(names, def.Name)
		}
		if run == 0 {
			first = names
			continue
		}
		assert.Equal(t, first, names, "run %d produced a different order", run)
	}
}

// TestModifierExtensionBuildsAGroup reads a module that declares two modifier
// groups and one subcommand under the same command.
//
// VALIDATES: ze:modifier reaches the tree with its occurrence, its declared
// position and its own leaves, and a container carrying ze:command stays a
// subcommand whatever else it says.
// PREVENTS: a group whose leaves are never extracted, which renders as a bare
// keyword; and a group whose order comes from the Children map, which renders
// differently between two runs of the same binary.
func TestModifierExtensionBuildsAGroup(t *testing.T) {
	loader := NewLoader()
	require.NoError(t, loader.LoadEmbedded())

	yangText := `
module test-modifier-cmd {
    namespace "urn:test:modifier:cmd";
    prefix tmc;
    import ze-extensions { prefix ze; }

    container announce {
        config false;
        ze:command "ze-bgp:announce";
        description "Announce a route.";
        leaf selector { type string; mandatory true; description "Peer selector"; }

        container tag {
            config false;
            ze:modifier "once";
            description "A key and a value carried with the announcement.";
            leaf key { type string; mandatory true; description "Tag key"; }
            leaf value { type string; mandatory true; description "Tag value"; }
        }

        container label {
            config false;
            ze:modifier "repeat";
            description "A repeatable label.";
            leaf name { type string; mandatory true; description "Label name"; }
        }

        container detail {
            config false;
            ze:command "ze-bgp:announce-detail";
            ze:modifier "once";
            description "A subcommand, not a group.";
        }

        container typo {
            config false;
            ze:modifier "sometimes";
            description "An occurrence nobody declared.";
        }
    }
}
`
	require.NoError(t, loader.AddModuleFromText("test-modifier-cmd.yang", yangText))
	require.NoError(t, loader.Resolve())

	root := BuildCommandTree(loader)
	announce := root.Children["announce"]
	require.NotNil(t, announce)

	tag := announce.Children["tag"]
	require.NotNil(t, tag)
	assert.Equal(t, command.ModifierOnce, tag.Modifier)
	assert.Equal(t, 1, tag.ModifierOrder)
	require.Len(t, tag.ArgDefs, 2)
	assert.Equal(t, "key", tag.ArgDefs[0].Name)
	assert.Equal(t, "value", tag.ArgDefs[1].Name)

	label := announce.Children["label"]
	require.NotNil(t, label)
	assert.Equal(t, command.ModifierRepeat, label.Modifier)
	assert.Equal(t, 2, label.ModifierOrder)

	detail := announce.Children["detail"]
	require.NotNil(t, detail)
	assert.Equal(t, command.ModifierNone, detail.Modifier,
		"a container that runs a command of its own is a subcommand, never a group")
	assert.Equal(t, "ze-bgp:announce-detail", detail.WireMethod)

	typo := announce.Children["typo"]
	require.NotNil(t, typo)
	assert.Equal(t, command.ModifierNone, typo.Modifier,
		"an occurrence the vocabulary does not hold leaves a plain grouping node")

	assert.Equal(t, "announce <selector> [tag <key> <value>] [label <name> ...]",
		command.UsageLine(command.Usage([]string{"announce"}, announce)))
}
