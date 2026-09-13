// Design: docs/architecture/api/commands.md — `update bgp config`
// Overview: peer.go — the BGP peer lifecycle handlers and their registration
// Related: create.go — `create bgp peer`, which puts a peer in the running configuration
// Related: internal/component/bgp/reactor/reactor_api.go — recordPeerConfig, which holds that peer's leaves

package peer

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/ze-software/ze/internal/component/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// cmdBgpConfigSave is the path an operator types to reach handleBgpPeerSave.
const cmdBgpConfigSave = "update bgp config"

// The keys this command answers with. Each is named twice, here and in the
// column order registerColumns declares, which is why they are constants
// (fields.go states the rule).
const (
	fieldSaveAdded   = "added"
	fieldSaveRemoved = "removed"
	fieldSaveConfig  = "config"
)

// The two levels of the configuration tree this command writes under.
//
// prefix_update.go keeps its own path segments literal, and fields.go states
// why: a schema path is a different vocabulary from a JSON key. These are named
// because ONE path is walked by four functions here, and a typo in one of them
// is silent in both directions: SetValue CREATES the path it is given, so a
// misspelled level writes a peer into a container the loader never reads.
const (
	configRootBGP  = "bgp"
	configListPeer = "peer"
)

var (
	errSaveTakesNoSelector = errors.New("update bgp config takes no selector")
	errSavePeerHasNoLeaf   = errors.New("the running configuration holds no leaf for this peer")
	errSaveNameDisagrees   = errors.New("the file and the running configuration name two different peers alike")
)

// handleBgpPeerSave handles `update bgp config`.
//
// It makes the configuration FILE state the running peer set. A peer an
// operator created at runtime is written into the file, and a peer the file
// declares that the running set no longer holds is taken out of it. Those two
// are one operation: after a `delete bgp peer` the peer is gone from the
// running set, so the ABSENCE is the fact being persisted and there is no peer
// left for a selector to name. That is why the command takes none.
//
// A peer both sides already hold is left exactly as the operator wrote it. Its
// leaves in the file and its leaves in the running configuration are the same
// leaves, because a commit is what changes a running peer's settings and a
// commit writes the file too. Rewriting it would replace the operator's text
// with a lowering of it and gain nothing.
//
// The running configuration is the reactor's, and it is true: the reload
// replaces it wholesale (SetConfigTree), and the two runtime commands maintain
// it as they change the peer set (recordPeerConfig and dropPeerConfig,
// internal/component/bgp/reactor/reactor_api.go). So the file and the daemon
// agree once this command returns, and a later commit of an unrelated leaf
// reconciles against a configuration that still names every running peer.
func handleBgpPeerSave(ctx *pluginserver.CommandContext, args []string) (*plugin.Response, error) {
	_, errResp, err := pluginserver.RequireReactor(ctx)
	if err != nil {
		return errResp, err
	}

	// Refused by name rather than ignored, and refused HERE rather than in the
	// schema alone: the command node declares no selector, so the dispatcher
	// turns an operator's trailing word away, while a plugin reaching this wire
	// method over the IPC transport sends arguments the CLI never saw.
	if len(args) > 0 {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error: tb.Str("update bgp config takes no selector: it writes the whole running peer set. ").
				Str("Got ").Quoted(args[0]).String(),
		}, fmt.Errorf("%w, got %q", errSaveTakesNoSelector, args[0])
	}

	configPath := ctx.Server.ConfigPath()
	if configPath == "" {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "no configuration file to write: this daemon was given its configuration on stdin",
		}, errConfigPathNotSet
	}

	running := runningPeerConfig(ctx.Reactor().GetConfigTree())

	ed, err := cli.NewEditor(configPath)
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("cannot open config: ").Err(err).String(),
		}, fmt.Errorf("open config: %w", err)
	}
	defer func() { _ = ed.Close() }()

	added, removed, err := savePeerSet(ed, running, filePeerConfig(ed))
	if err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{Status: plugin.StatusError, Error: tb.Err(err).String()}, err
	}

	if _, err := ed.Save(); err != nil {
		var tb textbuf.Buffer
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  tb.Str("cannot write config: ").Err(err).String(),
		}, fmt.Errorf("save config: %w", err)
	}

	var message textbuf.Buffer
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			fieldSaveAdded:   added,
			fieldSaveRemoved: removed,
			fieldSaveConfig:  configPath,
			// What the file says NOW, rather than what was written: a save
			// that changes nothing writes nothing, and the sentence is still
			// true (Editor.Save returns early on an unchanged tree).
			fieldMessage: message.Str(configPath).Str(" names the running peer set: ").
				Int(int64(len(added))).Str(" added, ").Int(int64(len(removed))).Str(" removed").String(),
		},
	}, nil
}

// savePeerSet writes the running peer set into the editor's tree and answers
// what it added and what it removed, each sorted so two runs of the same save
// read the same way.
//
// A peer both sides hold is left alone. Its leaves in the file and its leaves
// in the running configuration are the same leaves, because a commit is what
// changes a running peer's settings and a commit writes the file too.
func savePeerSet(ed *cli.Editor, running, onFile map[string]map[string]any) (added, removed []string, err error) {
	added = make([]string, 0, len(running))
	for _, name := range slices.Sorted(maps.Keys(running)) {
		declared, onBothSides := onFile[name]
		if onBothSides {
			if err := checkOnePeer(name, running[name], declared); err != nil {
				return nil, nil, err
			}
			continue
		}
		if err := writePeerEntry(ed, name, running[name]); err != nil {
			return nil, nil, err
		}
		added = append(added, name)
	}

	removed = make([]string, 0, len(onFile))
	for _, name := range slices.Sorted(maps.Keys(onFile)) {
		if _, stillRunning := running[name]; stillRunning {
			continue
		}
		if err := ed.DeleteByPath([]string{configRootBGP, configListPeer, name}); err != nil {
			return nil, nil, fmt.Errorf("remove peer %s from the config: %w", name, err)
		}
		removed = append(removed, name)
	}

	return added, removed, nil
}

// checkOnePeer refuses a name the file and the running configuration give to
// two different peers.
//
// Leaving a name that is on both sides alone is right only while the two are
// one peer, and the name of a peer created at runtime is DERIVED from its
// address (peerConfigNameFor,
// internal/component/bgp/reactor/reactor_api.go), so an operator is free to
// have written that same name over another address. Skipping it in silence
// would then leave the created peer unsaved while the command answered that it
// saved everything.
func checkOnePeer(name string, running, onFile map[string]any) error {
	runningAddr := peerRemoteAddress(running)
	fileAddr := peerRemoteAddress(onFile)
	if runningAddr == "" || fileAddr == "" || runningAddr == fileAddr {
		return nil
	}
	return fmt.Errorf("peer %s: %w: the file gives that name to %s and the running configuration to %s",
		name, errSaveNameDisagrees, fileAddr, runningAddr)
}

// peerRemoteAddress answers the address a peer subtree dials, and the empty
// string where it states none. A peer with no address is refused by the config
// loader, so the empty answer names a subtree that was never a whole peer.
func peerRemoteAddress(peerTree map[string]any) string {
	connection, _ := peerTree["connection"].(map[string]any)
	remote, _ := connection["remote"].(map[string]any)
	address, _ := remote["ip"].(string)
	return address
}

// writePeerEntry writes one peer's configuration into the editor's tree, under
// bgp > peer > name.
//
// A peer that writes no leaf at all is refused rather than reported as saved:
// an empty entry is a peer the file declares and cannot build, and the caller
// would answer that it saved it.
func writePeerEntry(ed *cli.Editor, name string, peerTree map[string]any) error {
	leaves, err := writeTreeLeaves(ed, []string{configRootBGP, configListPeer, name}, peerTree)
	if err != nil {
		return fmt.Errorf("write peer %s to the config: %w", name, err)
	}
	if leaves == 0 {
		return fmt.Errorf("write peer %s to the config: %w", name, errSavePeerHasNoLeaf)
	}
	return nil
}

// writeTreeLeaves writes every leaf of one config subtree through the editor,
// which types each value against the schema, and answers how many it wrote.
//
// The recursion is over a config subtree, whose depth is the schema's own and
// not an operator's: the deepest peer leaf sits five levels under the peer
// (`session > family > <name> > prefix > maximum`). The keys are written in
// sorted order, so the file a save produces does not depend on map iteration.
func writeTreeLeaves(ed *cli.Editor, path []string, tree map[string]any) (int, error) {
	leaves := 0
	for _, key := range slices.Sorted(maps.Keys(tree)) {
		switch value := tree[key].(type) {
		case string:
			if err := ed.SetValue(path, key, value); err != nil {
				return 0, fmt.Errorf("%s: %w", key, err)
			}
			leaves++
		case map[string]any:
			written, err := writeTreeLeaves(ed, append(slices.Clone(path), key), value)
			if err != nil {
				return 0, fmt.Errorf("%s > %w", key, err)
			}
			leaves += written
		default:
			// Named rather than skipped: a value this function cannot write is
			// a leaf of the running configuration that would be missing from
			// the file, and the peer would come back up without it.
			return 0, fmt.Errorf("%s: the running configuration holds a %T, which is not a leaf", key, value)
		}
	}
	return leaves, nil
}

// runningPeerConfig answers the peers the running configuration declares, each
// with its own subtree.
func runningPeerConfig(tree map[string]any) map[string]map[string]any {
	peers := make(map[string]map[string]any)

	bgp, _ := tree[configRootBGP].(map[string]any)
	declared, _ := bgp[configListPeer].(map[string]any)
	for name, entry := range declared {
		subtree, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		peers[name] = subtree
	}
	return peers
}

// filePeerConfig answers the peers the configuration file declares, each with
// its own subtree.
func filePeerConfig(ed *cli.Editor) map[string]map[string]any {
	peers := make(map[string]map[string]any)

	bgp := ed.Tree().GetContainer(configRootBGP)
	if bgp == nil {
		return peers
	}
	for name, entry := range bgp.GetList(configListPeer) {
		peers[name] = entry.ToMap()
	}
	return peers
}
