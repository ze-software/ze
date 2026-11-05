// Design: docs/architecture/config/transaction-protocol.md -- an outer rejection compensates its applied tree
// Related: reactor_api.go -- VerifyConfig, ApplyConfigDiff, loadPeersFullOrTree
// Related: reactor_dynamic.go -- dynamicGroupsLoad

package reactor

import "errors"

// TreeReloadFunc builds the peers, the global defaults and the dynamic groups
// of the whole configuration tree it is handed, in the ToRunningMap shape the
// reload coordinator passes to ApplyConfigDiff. It runs the
// pipeline ReloadFunc runs over the file (PruneInactive, ResolveBGPTree,
// CheckRequiredFields, YANG defaults, routes), so a peer it builds compares
// equal to the one the startup load built.
//
// It MUST apply nothing, for the reason ReloadFunc MUST NOT: it returns what
// the reconcile then applies under its journal.
type TreeReloadFunc func(tree map[string]any) ([]*PeerSettings, Globals, []*DynamicGroupConfig, error)

// SetTreeReloadFunc sets the loader ApplyConfigDiff runs over the tree it is
// handed, and the builder of a created peer (createdPeerSettings). Every
// reactor CreateReactorFromTree builds holds it (../config/loader_create.go),
// whether its config came from a file or from stdin: it reads no file.
func (r *Reactor) SetTreeReloadFunc(fn TreeReloadFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.treeReloadFunc = fn
}

// loadPeersFromConfig loads the peers of the whole configuration tree the
// reload coordinator hands ApplyConfigDiff.
//
// VerifyConfig keeps the file route (loadPeersFullOrTree): the coordinator
// verifies the candidate it read from that file, and the file-backed verifier
// is what keeps the optional schemas selected at boot
// (TestReloadVerifierRetainsSelectedPluginSchema, ../config/loader_test.go).
//
// The tree is the answer, never the file. Reading the file instead re-read
// the staged candidate (storage.ReadReloadConfig prefers it), so an outer
// compensation that handed ApplyConfigDiff the prior tree re-applied the very
// candidate it was undoing, before the hub cleared it. The loaded dynamic
// groups are the tree's own (dynamicGroupsReadAs), so the dynamic step
// reconciles the population against the tree being applied.
//
// A reactor with no TreeReloadFunc is one CreateReactorFromTree did not
// build. It holds no pipeline that can build a peer from the tree, so it
// refuses: parsing the bgp block alone would skip group resolution, the YANG
// defaults and the dynamic groups, and the reconcile would read every peer as
// changed.
func (a *reactorAPIAdapter) loadPeersFromConfig(tree map[string]any) (loadedConfig, error) {
	a.r.mu.RLock()
	treeReloadFn := a.r.treeReloadFunc
	a.r.mu.RUnlock()

	if treeReloadFn == nil {
		return loadedConfig{}, errNoTreeReload
	}
	peers, globals, groups, err := treeReloadFn(tree)
	if err != nil {
		return loadedConfig{}, err
	}
	return loadedConfig{peers: peers, globals: globals, groups: dynamicGroupsReadAs(groups)}, nil
}

// errNoTreeReload refuses a config apply on a reactor that holds no
// configuration pipeline (SetTreeReloadFunc was never called).
var errNoTreeReload = errors.New("bgp: cannot apply the configuration: this reactor holds no configuration pipeline to build its peers (SetTreeReloadFunc was never called); build the reactor with CreateReactorFromTree")

// RebuildRunningPeers rebuilds the peers of the running configuration
// through the tree reload (loadPeersFromConfig) and reconciles the reactor
// against them, as a config apply of the unchanged tree does: a peer whose
// rebuilt settings are equal stays as it is, one that differs is replaced.
//
// The BGP plugin calls it before StartPeers when a third-party plugin
// declared filter insertions at Stage 1. The daemon built its peers from the
// config before any plugin reached Stage 1, so those members are absent from
// the first build, and only a rebuild after Stage 1 places them
// (spec-filter-wire-6-stage-filters A-12). Peers are still held by the start
// barrier then (Reactor.peersStarted), so a replaced peer opens no session
// twice.
//
// It also runs after startup, from the listener the BGP plugin installs with
// registry.SetFilterInsertionsChanged: a plugin respawned with a different
// declaration, or a reload stopped one that declared insertions. A peer
// whose member list changed is replaced then, as a reload replaces it. The
// plugin server calls that listener holding its config transaction lock, so
// no reload swaps r.configTree during the rebuild. A rebuild is no apply, so
// it keeps the operator's live `subscribe` overrides (reactorAPIAdapter
// rebuild). The caller MUST NOT hold r.mu.
//
// The current running map retains inactive member intent, including after
// reload or compensation. Every rebuild uses that same authority.
func (r *Reactor) RebuildRunningPeers() error {
	r.mu.RLock()
	running := r.configTree
	r.mu.RUnlock()

	return (&reactorAPIAdapter{r: r, rebuild: true}).ApplyConfigDiff(running)
}
